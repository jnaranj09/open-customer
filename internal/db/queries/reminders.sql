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
