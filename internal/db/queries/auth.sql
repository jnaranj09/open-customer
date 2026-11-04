-- name: UserExists :one
SELECT EXISTS (SELECT 1 FROM users);

-- name: InsertUser :exec
INSERT INTO users (id, username, password_hash, created_at)
VALUES (@id, @username, @password_hash, @created_at);

-- name: InsertSession :exec
INSERT INTO sessions (token_hash, user_id, created_at, expires_at, last_seen_at)
VALUES (@token_hash, @user_id, @now, @expires_at, @now);

-- name: GetSessionWithUser :one
SELECT s.token_hash, s.user_id, s.expires_at, s.last_seen_at, u.username, u.must_change_password
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.token_hash = @token_hash;

-- name: TouchSession :execrows
UPDATE sessions SET last_seen_at = @now::timestamptz, expires_at = @expires_at
WHERE token_hash = @token_hash;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE token_hash = @token_hash;

-- name: DeleteOtherSessions :exec
DELETE FROM sessions WHERE user_id = @user_id AND token_hash <> @keep_token_hash;

-- name: DeleteUserSessions :exec
DELETE FROM sessions WHERE user_id = @user_id;

-- name: DeleteSetupTokens :exec
DELETE FROM setup_tokens;

-- name: InsertSetupToken :exec
INSERT INTO setup_tokens (token_hash, expires_at) VALUES (@token_hash, @expires_at);

-- name: GetSetupToken :one
SELECT expires_at, used_at FROM setup_tokens WHERE token_hash = @token_hash;

-- name: LockSetup :exec
SELECT pg_advisory_xact_lock(@key::bigint);

-- name: GetUserByUsername :one
SELECT id, username, password_hash, must_change_password FROM users WHERE username = @username;

-- name: GetUserPasswordHash :one
SELECT password_hash FROM users WHERE id = @id;

-- name: SetUserPassword :exec
UPDATE users SET password_hash = @password_hash, must_change_password = @must_change_password
WHERE id = @id;
