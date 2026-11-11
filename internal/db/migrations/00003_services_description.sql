-- +goose Up
ALTER TABLE services ADD COLUMN description text NULL
  CHECK (description IS NULL OR char_length(description) BETWEEN 1 AND 200);
