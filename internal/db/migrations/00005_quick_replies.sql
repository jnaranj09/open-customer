-- +goose Up
-- Quick replies (reminders-and-replies design, "Quick replies"): her own messages to copy.
CREATE TABLE quick_replies (
    id          uuid PRIMARY KEY,
    title       text NOT NULL CHECK (char_length(title) BETWEEN 1 AND 60),
    body        text NOT NULL CHECK (char_length(body) BETWEEN 1 AND 2000),
    position    int NOT NULL,
    archived_at timestamptz NULL
);
