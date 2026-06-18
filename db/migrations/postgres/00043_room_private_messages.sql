-- +goose Up
ALTER TABLE rooms ADD COLUMN IF NOT EXISTS private_messages_enabled BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE rooms DROP COLUMN IF EXISTS private_messages_enabled;
