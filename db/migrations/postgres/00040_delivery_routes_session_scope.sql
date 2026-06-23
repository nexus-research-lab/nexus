-- +goose Up
CREATE TABLE IF NOT EXISTS automation_delivery_routes (
    route_id VARCHAR(64) NOT NULL PRIMARY KEY,
    agent_id VARCHAR(64) NOT NULL,
    mode VARCHAR(32) NOT NULL,
    channel VARCHAR(64),
    "to" VARCHAR(255),
    account_id VARCHAR(64),
    thread_id VARCHAR(255),
    enabled BOOLEAN NOT NULL,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_automation_delivery_routes_mode CHECK (mode IN ('none', 'last', 'explicit'))
);

ALTER TABLE automation_delivery_routes ADD COLUMN IF NOT EXISTS session_key VARCHAR(512) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_automation_delivery_routes_agent_session_updated
ON automation_delivery_routes (agent_id, session_key, updated_at DESC, route_id DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_automation_delivery_routes_agent_session_updated;
ALTER TABLE automation_delivery_routes DROP COLUMN IF EXISTS session_key;
