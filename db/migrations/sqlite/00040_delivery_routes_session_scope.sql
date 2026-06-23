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
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_automation_delivery_routes_mode CHECK (mode IN ('none', 'last', 'explicit'))
);

ALTER TABLE automation_delivery_routes ADD COLUMN session_key VARCHAR(512) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_automation_delivery_routes_agent_session_updated
ON automation_delivery_routes (agent_id, session_key, updated_at DESC, route_id DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_automation_delivery_routes_agent_session_updated;

CREATE TABLE automation_delivery_routes_new (
    route_id VARCHAR(64) NOT NULL PRIMARY KEY,
    agent_id VARCHAR(64) NOT NULL,
    mode VARCHAR(32) NOT NULL,
    channel VARCHAR(64),
    "to" VARCHAR(255),
    account_id VARCHAR(64),
    thread_id VARCHAR(255),
    enabled BOOLEAN NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_automation_delivery_routes_mode CHECK (mode IN ('none', 'last', 'explicit')),
    FOREIGN KEY(agent_id) REFERENCES agents (id) ON DELETE CASCADE
);

INSERT INTO automation_delivery_routes_new (
    route_id, agent_id, mode, channel, "to", account_id, thread_id, enabled, created_at, updated_at
)
SELECT
    route_id, agent_id, mode, channel, "to", account_id, thread_id, enabled, created_at, updated_at
FROM automation_delivery_routes;

DROP TABLE automation_delivery_routes;
ALTER TABLE automation_delivery_routes_new RENAME TO automation_delivery_routes;

CREATE INDEX idx_automation_delivery_routes_agent ON automation_delivery_routes (agent_id);
CREATE INDEX IF NOT EXISTS idx_automation_delivery_routes_agent_updated
ON automation_delivery_routes (agent_id, updated_at DESC, route_id DESC);
