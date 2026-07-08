-- +goose Up
ALTER TABLE IF EXISTS imported_skills ADD COLUMN IF NOT EXISTS update_available BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down
ALTER TABLE IF EXISTS imported_skills DROP COLUMN IF EXISTS update_available;
