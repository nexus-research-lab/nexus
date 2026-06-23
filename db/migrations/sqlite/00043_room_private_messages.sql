-- +goose Up
ALTER TABLE rooms ADD COLUMN private_messages_enabled BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE rooms DROP COLUMN private_messages_enabled;
