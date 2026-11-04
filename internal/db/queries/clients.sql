-- name: ListClients :many
SELECT id, name, phone_e164, phone_digits, notes, attrs, created_at, updated_at, archived_at
FROM clients
WHERE (archived_at IS NOT NULL) = @archived::boolean
ORDER BY lower(name), name, id;

-- name: GetClient :one
SELECT id, name, phone_e164, phone_digits, notes, attrs, created_at, updated_at, archived_at
FROM clients
WHERE id = @id;

-- name: InsertClient :exec
INSERT INTO clients (id, name, phone_e164, phone_digits, notes, attrs, created_at, updated_at)
VALUES (@id, @name, sqlc.narg('phone_e164')::text, sqlc.narg('phone_digits')::text,
        sqlc.narg('notes')::text, @attrs::jsonb, @now::timestamptz, @now::timestamptz);

-- name: UpdateClient :exec
UPDATE clients SET
    name         = @name,
    phone_e164   = sqlc.narg('phone_e164')::text,
    phone_digits = sqlc.narg('phone_digits')::text,
    notes        = sqlc.narg('notes')::text,
    attrs        = @attrs::jsonb,
    updated_at   = @now::timestamptz,
    archived_at  = sqlc.narg('archived_at')::timestamptz
WHERE id = @id;

-- name: ListActiveClientsByPhone :many
SELECT id, name, phone_e164, phone_digits, notes, attrs, created_at, updated_at, archived_at
FROM clients
WHERE phone_e164 = @phone_e164::text AND archived_at IS NULL AND id <> @id::uuid
ORDER BY lower(name), name, id;
