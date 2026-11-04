-- name: GetSettings :one
SELECT name, timezone, locale, currency, country_code, accent_color, icon_updated_at, push_hour
FROM business_settings
WHERE id = 1;

-- name: InsertSettings :exec
INSERT INTO business_settings (id, name, timezone, locale, currency, country_code, accent_color, push_hour, updated_at)
VALUES (1, @name, @timezone, @locale, @currency, @country_code, @accent_color, @push_hour, @now);

-- name: UpdateSettings :exec
UPDATE business_settings SET
    name         = COALESCE(sqlc.narg('name')::text, name),
    timezone     = COALESCE(sqlc.narg('timezone')::text, timezone),
    accent_color = COALESCE(sqlc.narg('accent_color')::text, accent_color),
    push_hour    = COALESCE(sqlc.narg('push_hour')::smallint, push_hour),
    updated_at   = @now
WHERE id = 1;

-- name: SetIconUpdatedAt :execrows
UPDATE business_settings SET icon_updated_at = @now::timestamptz, updated_at = @now WHERE id = 1;
