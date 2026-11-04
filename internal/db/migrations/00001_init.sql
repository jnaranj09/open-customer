-- +goose Up
-- Release 1 schema (design doc "Data model"). Migrations only move forward: there is no Down section.
-- No extensions and no regional defaults.

CREATE TABLE business_settings (
    id              smallint PRIMARY KEY CHECK (id = 1),
    name            text NOT NULL,
    timezone        text NOT NULL,
    locale          text NOT NULL,
    currency        char(3) NOT NULL,
    country_code    text NOT NULL,
    accent_color    text NOT NULL,
    icon_updated_at timestamptz NULL,
    push_hour       smallint NOT NULL DEFAULT 8 CHECK (push_hour BETWEEN 0 AND 19),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id                   uuid PRIMARY KEY,
    username             text NOT NULL UNIQUE,
    password_hash        text NOT NULL,
    must_change_password boolean NOT NULL DEFAULT false,
    created_at           timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    token_hash   bytea PRIMARY KEY,
    user_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at   timestamptz NOT NULL DEFAULT now(),
    expires_at   timestamptz NOT NULL,
    last_seen_at timestamptz NULL
);
CREATE INDEX sessions_user_id_idx ON sessions (user_id);

CREATE TABLE setup_tokens (
    token_hash bytea PRIMARY KEY,
    expires_at timestamptz NOT NULL,
    used_at    timestamptz NULL
);

CREATE TABLE attribute_definitions (
    id          uuid PRIMARY KEY,
    key         text NOT NULL UNIQUE,
    label       text NOT NULL,
    scope       text NOT NULL CHECK (scope IN ('client', 'appointment')),
    type        text NOT NULL CHECK (type IN ('text', 'number', 'date', 'select', 'multiselect')),
    required    boolean NOT NULL DEFAULT false,
    position    int NOT NULL,
    archived_at timestamptz NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE attribute_options (
    id           uuid PRIMARY KEY,
    attribute_id uuid NOT NULL REFERENCES attribute_definitions (id),
    parent_id    uuid NULL REFERENCES attribute_options (id),
    label        text NOT NULL,
    position     int NOT NULL,
    archived_at  timestamptz NULL
);
CREATE INDEX attribute_options_attribute_id_idx ON attribute_options (attribute_id);

CREATE TABLE clients (
    id           uuid PRIMARY KEY,
    name         text NOT NULL,
    phone_e164   text NULL,
    phone_digits text NULL,
    notes        text NULL,
    attrs        jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(attrs) = 'object'),
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    archived_at  timestamptz NULL
);

CREATE TABLE services (
    id           uuid PRIMARY KEY,
    name         text NOT NULL,
    duration_min int NOT NULL CHECK (duration_min > 0),
    price_cents  bigint NULL CHECK (price_cents >= 0),
    color        text NOT NULL,
    position     int NOT NULL,
    archived_at  timestamptz NULL
);

CREATE TABLE appointments (
    id          uuid PRIMARY KEY,
    client_id   uuid NOT NULL REFERENCES clients (id),
    starts_at   timestamptz NOT NULL,
    ends_at     timestamptz NOT NULL,
    status      text NOT NULL DEFAULT 'scheduled'
        CHECK (status IN ('scheduled', 'confirmed', 'cancelled', 'no_show')),
    total_cents bigint NOT NULL DEFAULT 0 CHECK (total_cents >= 0),
    paid_cents  bigint NULL CHECK (paid_cents >= 0),
    attrs       jsonb NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(attrs) = 'object'),
    notes       text NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    CHECK (ends_at > starts_at)
);
CREATE INDEX appointments_starts_at_idx ON appointments (starts_at)
    WHERE status NOT IN ('cancelled', 'no_show');
CREATE INDEX appointments_client_id_starts_at_idx ON appointments (client_id, starts_at DESC);
CREATE INDEX appointments_ends_at_idx ON appointments (ends_at)
    WHERE status NOT IN ('cancelled', 'no_show');

CREATE TABLE appointment_services (
    appointment_id uuid NOT NULL REFERENCES appointments (id) ON DELETE CASCADE,
    position       smallint NOT NULL,
    service_id     uuid NOT NULL REFERENCES services (id),
    name           text NOT NULL,
    price_cents    bigint NULL CHECK (price_cents >= 0),
    duration_min   int NOT NULL CHECK (duration_min > 0),
    PRIMARY KEY (appointment_id, position)
);

CREATE TABLE media (
    id             uuid PRIMARY KEY,
    client_id      uuid NOT NULL REFERENCES clients (id),
    appointment_id uuid NULL REFERENCES appointments (id),
    storage_key    text NOT NULL,
    thumb_key      text NOT NULL,
    content_type   text NOT NULL,
    bytes          int NOT NULL,
    width          int NOT NULL,
    height         int NOT NULL,
    created_at     timestamptz NOT NULL DEFAULT now(),
    archived_at    timestamptz NULL
);
CREATE INDEX media_client_id_idx ON media (client_id);
CREATE INDEX media_appointment_id_idx ON media (appointment_id);

CREATE TABLE message_templates (
    id          uuid PRIMARY KEY,
    name        text NOT NULL,
    body        text NOT NULL,
    archived_at timestamptz NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE reminder_rules (
    id           uuid PRIMARY KEY,
    kind         text NOT NULL CHECK (kind IN ('before_appointment', 'come_back')),
    service_id   uuid NULL REFERENCES services (id),
    offset_days  int NOT NULL CHECK (offset_days >= 0),
    applies_from timestamptz NULL,
    template_id  uuid NOT NULL REFERENCES message_templates (id),
    archived_at  timestamptz NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    CHECK (kind <> 'come_back' OR offset_days > 0),
    CHECK ((kind = 'before_appointment') = (service_id IS NULL))
);

CREATE TABLE reminder_log (
    id             uuid PRIMARY KEY,
    rule_id        uuid NOT NULL REFERENCES reminder_rules (id),
    appointment_id uuid NOT NULL REFERENCES appointments (id),
    anchor_at      timestamptz NULL,
    action         text NOT NULL CHECK (action IN ('sent', 'skipped')),
    body           text NULL,
    phone_digits   text NULL,
    acted_at       timestamptz NOT NULL,
    created_at     timestamptz NOT NULL DEFAULT now(),
    UNIQUE NULLS NOT DISTINCT (rule_id, appointment_id, anchor_at)
);
CREATE INDEX reminder_log_appointment_id_idx ON reminder_log (appointment_id);

CREATE TABLE push_subscriptions (
    id         uuid PRIMARY KEY,
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    endpoint   text NOT NULL UNIQUE,
    p256dh     text NOT NULL,
    auth       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE server_keys (
    name       text PRIMARY KEY,
    value      text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE scheduler_state (
    name       text PRIMARY KEY,
    value      text NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);
