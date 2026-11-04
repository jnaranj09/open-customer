-- name: InsertMedia :exec
INSERT INTO media (id, client_id, appointment_id, storage_key, thumb_key, content_type, bytes, width, height, created_at)
VALUES (@id, @client_id, sqlc.narg('appointment_id'), @storage_key, @thumb_key, @content_type, @bytes, @width, @height, @created_at);

-- name: GetMedia :one
SELECT id, client_id, appointment_id, storage_key, thumb_key, content_type, bytes, width, height, created_at, archived_at
FROM media
WHERE id = @id;

-- name: ListMediaForClient :many
SELECT id, client_id, appointment_id, storage_key, thumb_key, content_type, bytes, width, height, created_at, archived_at
FROM media
WHERE client_id = @client_id AND archived_at IS NULL
ORDER BY created_at DESC, id DESC;

-- name: SetMediaArchived :one
UPDATE media SET archived_at = sqlc.narg('archived_at')::timestamptz
WHERE id = @id
RETURNING id, client_id, appointment_id, storage_key, thumb_key, content_type, bytes, width, height, created_at, archived_at;

-- name: GetAppointmentOwner :one
SELECT client_id FROM appointments WHERE id = $1;

-- name: GetClientArchivedAt :one
SELECT archived_at FROM clients WHERE id = $1;
