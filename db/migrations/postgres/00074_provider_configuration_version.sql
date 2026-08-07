-- +goose Up
-- Provider mutations share one monotonic configuration version.
ALTER TABLE provider ADD COLUMN IF NOT EXISTS configuration_version BIGINT NOT NULL DEFAULT 1;

-- +goose Down
ALTER TABLE provider DROP COLUMN IF EXISTS configuration_version;
