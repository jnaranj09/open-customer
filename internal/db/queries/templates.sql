-- name: ListTemplates :many
SELECT id, name, body, archived_at
FROM message_templates
ORDER BY name, id;

-- name: InsertTemplate :exec
INSERT INTO message_templates (id, name, body, created_at, updated_at)
VALUES (@id, @name, @body, @now::timestamptz, @now::timestamptz);

-- name: GetTemplate :one
SELECT id, name, body, archived_at
FROM message_templates
WHERE id = @id;

-- name: UpdateTemplate :exec
UPDATE message_templates SET
    name        = @name,
    body        = @body,
    archived_at = sqlc.narg('archived_at')::timestamptz,
    updated_at  = @now::timestamptz
WHERE id = @id;
