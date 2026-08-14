-- +goose Up
ALTER TABLE token_usage_records ADD COLUMN goal_scope VARCHAR(16) NOT NULL DEFAULT 'unknown';
ALTER TABLE token_usage_records ADD COLUMN execution_scope VARCHAR(16) NOT NULL DEFAULT 'unknown';
ALTER TABLE token_usage_records ADD COLUMN responsibility_lane VARCHAR(32) NOT NULL DEFAULT 'unknown';
ALTER TABLE token_usage_records ADD COLUMN runtime_kind VARCHAR(16) NOT NULL DEFAULT 'unknown';
ALTER TABLE token_usage_records ADD COLUMN provider_fingerprint VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE token_usage_records ADD COLUMN model_fingerprint VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE token_usage_records ADD COLUMN goal_tool_surface VARCHAR(16) NOT NULL DEFAULT 'unknown';
ALTER TABLE token_usage_records ADD COLUMN execution_tool_surface VARCHAR(16) NOT NULL DEFAULT 'unknown';
ALTER TABLE token_usage_records ADD COLUMN host_tool_surface_complete BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE token_usage_records ADD COLUMN tool_policy_fingerprint VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE token_usage_records ADD COLUMN mcp_servers_fingerprint VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE token_usage_records ADD COLUMN tool_surface_fingerprint VARCHAR(64) NOT NULL DEFAULT '';

CREATE INDEX idx_token_usage_records_cache_segment
    ON token_usage_records (owner_user_id, goal_scope, execution_scope, responsibility_lane, occurred_at);

-- +goose Down
DROP INDEX IF EXISTS idx_token_usage_records_cache_segment;
ALTER TABLE token_usage_records DROP COLUMN tool_surface_fingerprint;
ALTER TABLE token_usage_records DROP COLUMN mcp_servers_fingerprint;
ALTER TABLE token_usage_records DROP COLUMN tool_policy_fingerprint;
ALTER TABLE token_usage_records DROP COLUMN host_tool_surface_complete;
ALTER TABLE token_usage_records DROP COLUMN execution_tool_surface;
ALTER TABLE token_usage_records DROP COLUMN goal_tool_surface;
ALTER TABLE token_usage_records DROP COLUMN model_fingerprint;
ALTER TABLE token_usage_records DROP COLUMN provider_fingerprint;
ALTER TABLE token_usage_records DROP COLUMN runtime_kind;
ALTER TABLE token_usage_records DROP COLUMN responsibility_lane;
ALTER TABLE token_usage_records DROP COLUMN execution_scope;
ALTER TABLE token_usage_records DROP COLUMN goal_scope;
