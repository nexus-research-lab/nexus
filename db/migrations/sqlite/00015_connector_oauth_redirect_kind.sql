-- +goose Up
ALTER TABLE connector_oauth_states ADD COLUMN redirect_kind VARCHAR(32) NOT NULL DEFAULT 'web';
CREATE INDEX idx_connector_oauth_states_redirect_kind ON connector_oauth_states (redirect_kind);

-- +goose Down
DROP INDEX IF EXISTS idx_connector_oauth_states_redirect_kind;
ALTER TABLE connector_oauth_states DROP COLUMN redirect_kind;
