-- +goose Up
ALTER TABLE provider ADD COLUMN preset_key VARCHAR(128) NOT NULL DEFAULT 'custom';
ALTER TABLE provider ADD COLUMN api_format VARCHAR(64) NOT NULL DEFAULT 'anthropic_messages';
ALTER TABLE provider ADD COLUMN models_path TEXT NOT NULL DEFAULT '/v1/models';
ALTER TABLE provider ADD COLUMN last_test_status VARCHAR(32) NOT NULL DEFAULT '';
ALTER TABLE provider ADD COLUMN last_test_error TEXT NOT NULL DEFAULT '';
ALTER TABLE provider ADD COLUMN last_test_at DATETIME;

CREATE INDEX idx_provider_preset_format ON provider (preset_key, api_format);

CREATE TABLE provider_models (
    id VARCHAR(64) NOT NULL PRIMARY KEY,
    provider_id VARCHAR(64) NOT NULL,
    model_id VARCHAR(255) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    category VARCHAR(64) NOT NULL DEFAULT 'chat',
    enabled BOOLEAN NOT NULL DEFAULT 0,
    capabilities_auto_json TEXT NOT NULL DEFAULT '{}',
    capabilities_override_json TEXT NOT NULL DEFAULT '{}',
    context_window INTEGER,
    max_output_tokens INTEGER,
    provider_options_json TEXT NOT NULL DEFAULT '{}',
    last_seen_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    FOREIGN KEY(provider_id) REFERENCES provider (id) ON DELETE CASCADE,
    UNIQUE(provider_id, model_id)
);
CREATE INDEX idx_provider_models_provider_enabled ON provider_models (provider_id, enabled);
CREATE INDEX idx_provider_models_last_seen ON provider_models (provider_id, last_seen_at);

-- +goose Down
DROP INDEX idx_provider_models_last_seen;
DROP INDEX idx_provider_models_provider_enabled;
DROP TABLE provider_models;
DROP INDEX idx_provider_preset_format;
ALTER TABLE provider DROP COLUMN last_test_at;
ALTER TABLE provider DROP COLUMN last_test_error;
ALTER TABLE provider DROP COLUMN last_test_status;
ALTER TABLE provider DROP COLUMN models_path;
ALTER TABLE provider DROP COLUMN api_format;
ALTER TABLE provider DROP COLUMN preset_key;
