-- +goose Up
ALTER TABLE connector_connections
ADD COLUMN credentials_key_id VARCHAR(80);

-- +goose Down
ALTER TABLE connector_connections DROP COLUMN credentials_key_id;
