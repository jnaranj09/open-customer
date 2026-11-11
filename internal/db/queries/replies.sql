-- Quick replies (reminders-and-replies design, "Quick replies"; internal/replies).

-- name: ListReplies :many
SELECT id, title, body, position, archived_at
FROM quick_replies
ORDER BY position, id;

-- name: MaxReplyPosition :one
SELECT COALESCE(MAX(position), -1)::int AS max_position
FROM quick_replies;

-- name: InsertReply :exec
INSERT INTO quick_replies (id, title, body, position)
VALUES (@id, @title, @body, @position);

-- name: GetReply :one
SELECT id, title, body, position, archived_at
FROM quick_replies
WHERE id = @id;

-- name: UpdateReply :exec
UPDATE quick_replies SET
    title       = @title,
    body        = @body,
    position    = @position,
    archived_at = sqlc.narg('archived_at')::timestamptz
WHERE id = @id;
