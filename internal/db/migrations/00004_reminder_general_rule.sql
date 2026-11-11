-- +goose Up
-- A general come-back rule has no service (reminders-and-replies design, "Data model").
-- 00001 has an unnamed CHECK ((kind = 'before_appointment') = (service_id IS NULL)).
-- Find it by its definition and fail unless exactly one matches.
-- +goose StatementBegin
DO $$
DECLARE names text[];
BEGIN
  SELECT array_agg(conname) INTO names FROM pg_constraint
  WHERE conrelid = 'reminder_rules'::regclass AND contype = 'c'
    AND pg_get_constraintdef(oid) LIKE '%before_appointment%service_id IS NULL%';
  IF coalesce(array_length(names, 1), 0) <> 1 THEN
    RAISE EXCEPTION 'expected one service CHECK on reminder_rules, found %', names;
  END IF;
  EXECUTE format('ALTER TABLE reminder_rules DROP CONSTRAINT %I', names[1]);
END $$;
-- +goose StatementEnd
ALTER TABLE reminder_rules ADD CONSTRAINT reminder_rules_service_check
  CHECK (kind = 'come_back' OR service_id IS NULL);
ALTER TABLE reminder_rules ADD CONSTRAINT reminder_rules_general_return_check
  CHECK (service_id IS NOT NULL OR return_service_id IS NULL);
-- Her answer to the handover question on a specific rule that stopped sending.
ALTER TABLE reminder_rules ADD COLUMN handover_from timestamptz NULL;
ALTER TABLE reminder_rules ADD CONSTRAINT reminder_rules_handover_check
  CHECK (handover_from IS NULL OR (kind = 'come_back' AND service_id IS NOT NULL));
