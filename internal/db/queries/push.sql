-- name: GetServerKey :one
SELECT value FROM server_keys WHERE name = @name;

-- name: InsertServerKeyIfMissing :exec
INSERT INTO server_keys (name, value, created_at)
VALUES (@name, @value, @now::timestamptz)
ON CONFLICT (name) DO NOTHING;

-- name: UpsertSubscription :one
INSERT INTO push_subscriptions (id, user_id, endpoint, p256dh, auth, created_at)
VALUES (@id, @user_id, @endpoint, @p256dh, @auth, @now::timestamptz)
ON CONFLICT (endpoint) DO UPDATE SET p256dh = EXCLUDED.p256dh, auth = EXCLUDED.auth
RETURNING id, (xmax = 0)::boolean AS created;

-- name: ListSubscriptions :many
SELECT id, endpoint, p256dh, auth FROM push_subscriptions ORDER BY id;

-- name: DeleteSubscription :exec
DELETE FROM push_subscriptions WHERE id = @id;

-- name: CountSubscriptions :one
SELECT count(*) FROM push_subscriptions;
