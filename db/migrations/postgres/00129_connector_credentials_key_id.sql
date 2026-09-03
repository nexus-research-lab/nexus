-- +goose Up
ALTER TABLE connector_connections
ADD COLUMN IF NOT EXISTS credentials_key_id VARCHAR(80);

-- +goose Down
ALTER TABLE connector_connections DROP COLUMN IF EXISTS credentials_key_id;
