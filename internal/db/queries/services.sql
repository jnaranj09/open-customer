-- name: ListServices :many
SELECT id, name, duration_min, price_cents, color, position, archived_at
FROM services
ORDER BY position, id;

-- name: GetServicesByIDs :many
SELECT id, name, duration_min, price_cents, color, position, archived_at
FROM services
WHERE id = ANY(@ids::uuid[]);

-- name: MaxServicePosition :one
SELECT COALESCE(MAX(position), -1)::int AS max_position
FROM services;

-- name: InsertService :exec
INSERT INTO services (id, name, duration_min, price_cents, color, position)
VALUES (@id, @name, @duration_min, sqlc.narg('price_cents')::bigint, @color, @position);

-- name: GetService :one
SELECT id, name, duration_min, price_cents, color, position, archived_at
FROM services
WHERE id = @id;

-- name: UpdateService :exec
UPDATE services SET
    name         = @name,
    duration_min = @duration_min,
    price_cents  = sqlc.narg('price_cents')::bigint,
    color        = @color,
    position     = @position,
    archived_at  = sqlc.narg('archived_at')::timestamptz
WHERE id = @id;
