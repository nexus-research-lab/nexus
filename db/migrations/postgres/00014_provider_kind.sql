-- +goose Up
ALTER TABLE provider ADD COLUMN provider_kind VARCHAR(64) NOT NULL DEFAULT 'llm';
CREATE INDEX idx_provider_kind_enabled ON provider (provider_kind, enabled);

-- +goose Down
DROP INDEX idx_provider_kind_enabled;
ALTER TABLE provider DROP COLUMN provider_kind;
