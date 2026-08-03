-- +goose Up
ALTER TABLE members
ADD COLUMN participation_paused BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE members DROP COLUMN participation_paused;
