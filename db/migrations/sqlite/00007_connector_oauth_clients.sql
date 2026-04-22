-- +goose Up
CREATE TABLE connector_oauth_clients (
    owner_user_id VARCHAR(64) NOT NULL,
    connector_id VARCHAR(128) NOT NULL,
    client_id VARCHAR(512) NOT NULL,
    client_secret_encrypted TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY (owner_user_id, connector_id)
);
CREATE INDEX idx_connector_oauth_clients_connector ON connector_oauth_clients (connector_id);

-- +goose Down
DROP INDEX IF EXISTS idx_connector_oauth_clients_connector;
DROP TABLE IF EXISTS connector_oauth_clients;
