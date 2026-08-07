-- +goose Up
-- Connector configuration versions are scoped by owner and connector.
CREATE TABLE connector_configuration_versions (
    owner_user_id VARCHAR(64) NOT NULL,
    connector_id VARCHAR(128) NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (owner_user_id, connector_id)
);

-- +goose Down
DROP TABLE IF EXISTS connector_configuration_versions;
