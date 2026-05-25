-- +goose Up
ALTER TABLE provider ADD COLUMN preset_key VARCHAR(128) NOT NULL DEFAULT 'custom';
ALTER TABLE provider ADD COLUMN api_format VARCHAR(64) NOT NULL DEFAULT 'anthropic_messages';
ALTER TABLE provider ADD COLUMN models_path TEXT NOT NULL DEFAULT '/v1/models';
ALTER TABLE provider ADD COLUMN last_test_status VARCHAR(32) NOT NULL DEFAULT '';
ALTER TABLE provider ADD COLUMN last_test_error TEXT NOT NULL DEFAULT '';
ALTER TABLE provider ADD COLUMN last_test_at DATETIME;
ALTER TABLE runtimes ADD COLUMN model VARCHAR(255);

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
    is_default BOOLEAN NOT NULL DEFAULT 0,
    last_seen_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    FOREIGN KEY(provider_id) REFERENCES provider (id) ON DELETE CASCADE,
    UNIQUE(provider_id, model_id)
);
CREATE INDEX idx_provider_models_provider_enabled ON provider_models (provider_id, enabled);
CREATE INDEX idx_provider_models_last_seen ON provider_models (provider_id, last_seen_at);
CREATE INDEX idx_provider_models_default ON provider_models (is_default, provider_id);

INSERT INTO provider_models (
    id, provider_id, model_id, display_name, category, enabled,
    capabilities_auto_json, capabilities_override_json, context_window,
    max_output_tokens, provider_options_json, is_default,
    last_seen_at, created_at, updated_at
)
SELECT
    'provider_model_' || p.id || '_legacy',
    p.id,
    TRIM(p.model),
    TRIM(p.model),
    'chat',
    1,
    '{}',
    '{}',
    NULL,
    NULL,
    '{}',
    CASE WHEN p.is_default THEN 1 ELSE 0 END,
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
FROM provider p
WHERE TRIM(COALESCE(p.model, '')) <> '';

UPDATE runtimes
SET model = (
    SELECT p.model
    FROM provider p
    WHERE p.provider = runtimes.provider
      AND TRIM(COALESCE(p.model, '')) <> ''
    LIMIT 1
)
WHERE TRIM(COALESCE(provider, '')) <> ''
  AND model IS NULL;

ALTER TABLE provider DROP COLUMN is_default;
ALTER TABLE provider DROP COLUMN model;

-- +goose Down
ALTER TABLE provider ADD COLUMN model VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE provider ADD COLUMN is_default BOOLEAN NOT NULL DEFAULT 0;
UPDATE provider
SET model = COALESCE((
    SELECT pm.model_id
    FROM provider_models pm
    WHERE pm.provider_id = provider.id
      AND pm.enabled = 1
    ORDER BY pm.is_default DESC, pm.updated_at DESC
    LIMIT 1
), '');
UPDATE provider
SET is_default = CASE
    WHEN EXISTS (
        SELECT 1
        FROM provider_models pm
        WHERE pm.provider_id = provider.id
          AND pm.is_default = 1
    ) THEN 1
    ELSE 0
END;
ALTER TABLE runtimes DROP COLUMN model;
DROP INDEX idx_provider_models_default;
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
