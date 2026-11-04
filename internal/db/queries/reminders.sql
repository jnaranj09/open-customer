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
