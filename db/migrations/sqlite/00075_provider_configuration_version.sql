-- +goose Up
-- Provider mutations share one monotonic configuration version.
ALTER TABLE provider ADD COLUMN configuration_version INTEGER NOT NULL DEFAULT 1;

-- +goose Down
ALTER TABLE provider DROP COLUMN configuration_version;
