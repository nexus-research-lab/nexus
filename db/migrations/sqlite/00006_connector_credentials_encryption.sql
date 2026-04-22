-- +goose Up
ALTER TABLE connector_connections ADD COLUMN credentials_encrypted TEXT;

-- +goose Down
ALTER TABLE connector_connections DROP COLUMN credentials_encrypted;
