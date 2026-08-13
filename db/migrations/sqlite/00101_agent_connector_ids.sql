-- +goose Up
ALTER TABLE runtimes ADD COLUMN connector_ids_json TEXT NOT NULL DEFAULT '[]';

-- +goose Down
ALTER TABLE runtimes DROP COLUMN connector_ids_json;
