-- name: ListTemplates :many
SELECT id, name, body, archived_at
FROM message_templates
ORDER BY name, id;

-- name: InsertTemplate :exec
INSERT INTO message_templates (id, name, body)
VALUES (@id, @name, @body);

-- name: GetTemplate :one
SELECT id, name, body, archived_at
FROM message_templates
WHERE id = @id;

-- name: UpdateTemplate :exec
UPDATE message_templates SET
    name        = @name,
    body        = @body,
    archived_at = sqlc.narg('archived_at')::timestamptz,
    updated_at  = now()
WHERE id = @id;
