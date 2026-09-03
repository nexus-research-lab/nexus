-- +goose Up
ALTER TABLE connector_connections
ADD COLUMN IF NOT EXISTS enabled BOOLEAN NOT NULL DEFAULT TRUE;

-- Preserve the availability state written by development builds that already
-- exposed the custom MCP switch before this schema became canonical.
UPDATE connector_connections
SET enabled = FALSE
WHERE auth_type = 'custom_mcp' AND state = 'disconnected';

-- +goose Down
ALTER TABLE connector_connections DROP COLUMN IF EXISTS enabled;
