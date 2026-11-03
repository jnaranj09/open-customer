-- name: UserExists :one
SELECT EXISTS (SELECT 1 FROM users);
