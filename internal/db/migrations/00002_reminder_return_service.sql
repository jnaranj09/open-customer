-- +goose Up
ALTER TABLE reminder_rules ADD COLUMN return_service_id uuid NULL REFERENCES services (id);
ALTER TABLE reminder_rules ADD CONSTRAINT reminder_rules_return_service_check CHECK (kind = 'come_back' OR return_service_id IS NULL);
