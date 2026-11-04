-- name: UpsertState :exec
INSERT INTO scheduler_state (name, value, updated_at)
VALUES (@name, @value, @now::timestamptz)
ON CONFLICT (name) DO UPDATE SET value = EXCLUDED.value, updated_at = EXCLUDED.updated_at;

-- name: ListState :many
SELECT name, value FROM scheduler_state ORDER BY name;
