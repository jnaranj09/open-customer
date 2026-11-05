-- name: ListRules :many
SELECT id, kind, service_id, offset_days, applies_from, template_id, archived_at, created_at, return_service_id
FROM reminder_rules
ORDER BY created_at, id;

-- name: GetRule :one
SELECT id, kind, service_id, offset_days, applies_from, template_id, archived_at, created_at, return_service_id
FROM reminder_rules
WHERE id = @id;

-- name: InsertRule :exec
INSERT INTO reminder_rules (id, kind, service_id, return_service_id, offset_days, template_id, applies_from, created_at)
VALUES (@id, @kind, @service_id, @return_service_id, @offset_days, @template_id,
        sqlc.narg('applies_from')::timestamptz, @now::timestamptz);

-- name: UpdateRule :exec
UPDATE reminder_rules SET
    offset_days       = @offset_days,
    template_id       = @template_id,
    return_service_id = @return_service_id,
    applies_from      = sqlc.narg('applies_from')::timestamptz,
    archived_at       = sqlc.narg('archived_at')::timestamptz
WHERE id = @id;

-- name: GetTemplateState :one
SELECT archived_at FROM message_templates WHERE id = @id;

-- name: GetServiceState :one
SELECT archived_at FROM services WHERE id = @id;

-- The due-list loads (design doc "Due-list computation", steps 1-2). DueList runs them one after
-- another on one read-only REPEATABLE READ snapshot.

-- name: GetDueSettings :one
SELECT name, timezone, locale FROM business_settings WHERE id = 1;

-- name: ListActiveRulesForDue :many
-- The return service is the effective one: return_service_id, or service_id when it is empty. A
-- "before" rule has neither, so its return columns are NULL.
SELECT r.id, r.kind, r.service_id, r.offset_days, r.applies_from,
       t.body AS template_body, t.archived_at AS template_archived_at,
       s.archived_at AS service_archived_at,
       rs.id AS return_service_id, rs.name AS return_service_name, rs.archived_at AS return_archived_at
FROM reminder_rules r
JOIN message_templates t ON t.id = r.template_id
LEFT JOIN services s ON s.id = r.service_id
LEFT JOIN services rs ON rs.id = COALESCE(r.return_service_id, r.service_id)
WHERE r.archived_at IS NULL
ORDER BY r.created_at, r.id;

-- name: ListBeforeCandidates :many
SELECT a.id, a.client_id, c.name AS client_name, c.phone_digits, c.attrs AS client_attrs,
       a.starts_at, a.ends_at, a.created_at, a.status, a.attrs
FROM appointments a JOIN clients c ON c.id = a.client_id
WHERE a.starts_at > @now::timestamptz AND a.starts_at < @until::timestamptz
  AND a.status IN ('scheduled', 'confirmed') AND c.archived_at IS NULL
ORDER BY a.starts_at, a.id;

-- name: ListComeBackCandidates :many
SELECT a.id, a.client_id, c.name AS client_name, c.phone_digits, c.attrs AS client_attrs,
       a.starts_at, a.ends_at, a.created_at, a.status, a.attrs
FROM appointments a JOIN clients c ON c.id = a.client_id
WHERE a.ends_at >= @since::timestamptz AND a.ends_at <= @now::timestamptz
  AND a.status IN ('scheduled', 'confirmed') AND c.archived_at IS NULL
ORDER BY a.ends_at, a.id;

-- name: ListCandidateLines :many
SELECT appointment_id, service_id, name
FROM appointment_services
WHERE appointment_id = ANY(@ids::uuid[])
ORDER BY appointment_id, position;

-- name: ListLaterAppointments :many
SELECT id, client_id, starts_at
FROM appointments
WHERE client_id = ANY(@client_ids::uuid[]) AND starts_at > @after::timestamptz
  AND status NOT IN ('cancelled', 'no_show')
ORDER BY starts_at, id;

-- name: ListLogKeys :many
SELECT rule_id, appointment_id, anchor_at
FROM reminder_log
WHERE appointment_id = ANY(@ids::uuid[]);

-- The reminder log (design doc item 55, log.go).

-- name: GetRuleKind :one
SELECT kind FROM reminder_rules WHERE id = @id;

-- name: GetAppointmentExists :one
SELECT EXISTS (SELECT 1 FROM appointments WHERE id = @id);

-- name: GetClientExists :one
SELECT EXISTS (SELECT 1 FROM clients WHERE id = @id);

-- name: InsertLog :one
INSERT INTO reminder_log (id, rule_id, appointment_id, anchor_at, action, body, phone_digits, acted_at)
VALUES (@id, @rule_id, @appointment_id, sqlc.narg('anchor_at')::timestamptz, @action,
        sqlc.narg('body')::text, sqlc.narg('phone_digits')::text, @acted_at::timestamptz)
ON CONFLICT (rule_id, appointment_id, anchor_at) DO NOTHING
RETURNING id;

-- name: GetLogByKey :one
SELECT id FROM reminder_log
WHERE rule_id = @rule_id AND appointment_id = @appointment_id
  AND anchor_at IS NOT DISTINCT FROM sqlc.narg('anchor_at')::timestamptz;

-- name: GetLogItem :one
SELECT l.id, l.rule_id, l.appointment_id, l.anchor_at, l.action, l.body, l.phone_digits, l.acted_at,
       r.kind, a.client_id, c.name AS client_name
FROM reminder_log l
JOIN reminder_rules r ON r.id = l.rule_id
JOIN appointments a ON a.id = l.appointment_id
JOIN clients c ON c.id = a.client_id
WHERE l.id = @id;

-- name: ListLogForClient :many
SELECT l.id, l.rule_id, l.appointment_id, l.anchor_at, l.action, l.body, l.phone_digits, l.acted_at,
       r.kind, a.client_id, c.name AS client_name
FROM reminder_log l
JOIN reminder_rules r ON r.id = l.rule_id
JOIN appointments a ON a.id = l.appointment_id
JOIN clients c ON c.id = a.client_id
WHERE a.client_id = @client_id
ORDER BY l.acted_at DESC, l.id DESC;

-- name: ListLogInRange :many
SELECT l.id, l.rule_id, l.appointment_id, l.anchor_at, l.action, l.body, l.phone_digits, l.acted_at,
       r.kind, a.client_id, c.name AS client_name
FROM reminder_log l
JOIN reminder_rules r ON r.id = l.rule_id
JOIN appointments a ON a.id = l.appointment_id
JOIN clients c ON c.id = a.client_id
WHERE l.acted_at >= @from_at::timestamptz AND l.acted_at < @to_at::timestamptz
ORDER BY l.acted_at DESC, l.id DESC;

-- The template preview (design doc "Message templates", preview.go). An archived client is
-- allowed.

-- name: GetPreviewClient :one
SELECT name, attrs FROM clients WHERE id = @id;

-- name: GetLatestAppointmentForPreview :one
-- The client's latest appointment that is not cancelled or no-show; its lines come from
-- ListCandidateLines.
SELECT id, starts_at, attrs
FROM appointments
WHERE client_id = @client_id AND status IN ('scheduled', 'confirmed')
ORDER BY starts_at DESC, id DESC
LIMIT 1;
