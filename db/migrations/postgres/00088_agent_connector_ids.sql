-- +goose Up
ALTER TABLE runtimes ADD COLUMN IF NOT EXISTS connector_ids_json TEXT NOT NULL DEFAULT '[]';

-- +goose Down
ALTER TABLE runtimes DROP COLUMN IF EXISTS connector_ids_json;
