-- The export (design doc "Export", item 59). SELECT only; it may read every table. Each table is
-- read in pages of 500 by id, all on the caller's one read-only REPEATABLE READ snapshot.

-- name: ExportClients :many
SELECT id, name, phone_e164, phone_digits, notes, attrs, created_at, updated_at, archived_at
FROM clients
WHERE id > @after
ORDER BY id
LIMIT 500;

-- name: ExportAppointments :many
SELECT a.id, a.client_id, c.name AS client_name, a.starts_at, a.ends_at, a.status,
       a.total_cents, a.paid_cents, a.attrs, a.notes, a.created_at, a.updated_at
FROM appointments a
JOIN clients c ON c.id = a.client_id
WHERE a.id > @after
ORDER BY a.id
LIMIT 500;

-- name: ExportAppointmentLines :many
SELECT appointment_id, service_id, name, price_cents, duration_min
FROM appointment_services
WHERE appointment_id = ANY(@ids::uuid[])
ORDER BY appointment_id, position;

-- name: ExportServices :many
SELECT id, name, duration_min, price_cents, color, position, archived_at, description
FROM services
WHERE id > @after
ORDER BY id
LIMIT 500;

-- name: ExportTemplates :many
SELECT id, name, body, archived_at
FROM message_templates
WHERE id > @after
ORDER BY id
LIMIT 500;

-- name: ExportRules :many
SELECT id, kind, service_id, return_service_id, offset_days, applies_from, template_id, archived_at, handover_from
FROM reminder_rules
WHERE id > @after
ORDER BY id
LIMIT 500;

-- name: ExportLog :many
SELECT l.id, l.rule_id, l.appointment_id, l.anchor_at, l.action, l.body, l.phone_digits, l.acted_at,
       r.kind, a.client_id, c.name AS client_name
FROM reminder_log l
JOIN reminder_rules r ON r.id = l.rule_id
JOIN appointments a ON a.id = l.appointment_id
JOIN clients c ON c.id = a.client_id
WHERE l.id > @after
ORDER BY l.id
LIMIT 500;

-- name: ExportQuickReplies :many
-- rank is the reply's place in her order, from 1 (respuestas.csv "orden"); it is counted over
-- every reply before the page is cut.
SELECT id, title, body, position, archived_at, rank
FROM (SELECT id, title, body, position, archived_at, row_number() OVER (ORDER BY position, id)::int AS rank
      FROM quick_replies) r
WHERE id > @after
ORDER BY id
LIMIT 500;
