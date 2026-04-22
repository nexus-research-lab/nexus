-- +goose Up
CREATE TABLE connector_oauth_states (
    state VARCHAR(64) NOT NULL PRIMARY KEY,
    connector_id VARCHAR(128) NOT NULL,
    code_verifier VARCHAR(128),
    redirect_uri VARCHAR(512) NOT NULL,
    shop_domain VARCHAR(255),
    extra_json TEXT,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX idx_connector_oauth_states_connector ON connector_oauth_states (connector_id);
CREATE INDEX idx_connector_oauth_states_expires ON connector_oauth_states (expires_at);

-- +goose Down
DROP INDEX IF EXISTS idx_connector_oauth_states_expires;
DROP INDEX IF EXISTS idx_connector_oauth_states_connector;
DROP TABLE IF EXISTS connector_oauth_states;
