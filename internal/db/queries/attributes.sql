-- name: ListAttributeDefinitions :many
SELECT id, key, label, scope, type, required, position, archived_at
FROM attribute_definitions
ORDER BY scope, position, id;

-- name: ListAttributeOptions :many
SELECT id, attribute_id, parent_id, label, position, archived_at
FROM attribute_options
WHERE attribute_id = COALESCE(sqlc.narg('attribute_id')::uuid, attribute_id)
ORDER BY attribute_id, position, id;

-- name: ListAttributeKeys :many
SELECT key FROM attribute_definitions;

-- name: MaxAttributePosition :one
SELECT COALESCE(MAX(position), -1)::int AS max_position
FROM attribute_definitions
WHERE scope = @scope;

-- name: InsertAttribute :exec
INSERT INTO attribute_definitions (id, key, label, scope, type, required, position, created_at)
VALUES (@id, @key, @label, @scope, @type, @required, @position, @now);

-- name: GetAttribute :one
SELECT id, key, label, scope, type, required, position, archived_at
FROM attribute_definitions
WHERE id = @id;

-- name: UpdateAttribute :exec
UPDATE attribute_definitions SET
    label       = @label,
    required    = @required,
    position    = @position,
    archived_at = sqlc.narg('archived_at')::timestamptz
WHERE id = @id;

-- name: InsertAttributeOption :exec
INSERT INTO attribute_options (id, attribute_id, parent_id, label, position)
VALUES (@id, @attribute_id, sqlc.narg('parent_id')::uuid, @label, @position);

-- name: GetAttributeOption :one
SELECT id, attribute_id, parent_id, label, position, archived_at
FROM attribute_options
WHERE id = @id;

-- name: MaxOptionPosition :one
SELECT COALESCE(MAX(position), -1)::int AS max_position
FROM attribute_options
WHERE attribute_id = @attribute_id
  AND parent_id IS NOT DISTINCT FROM sqlc.narg('parent_id')::uuid;

-- name: UpdateAttributeOption :exec
UPDATE attribute_options SET
    label       = @label,
    position    = @position,
    archived_at = sqlc.narg('archived_at')::timestamptz
WHERE id = @id;
