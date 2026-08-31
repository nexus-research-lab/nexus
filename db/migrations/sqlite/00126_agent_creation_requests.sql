-- +goose Up
CREATE TABLE agent_creation_requests (
    owner_user_id VARCHAR(64) NOT NULL,
    creation_request_id VARCHAR(128) NOT NULL,
    intent_digest VARCHAR(64) NOT NULL,
    agent_id VARCHAR(64) NOT NULL,
    workspace_path VARCHAR(512) NOT NULL,
    status VARCHAR(32) NOT NULL,
    stage VARCHAR(32) NOT NULL DEFAULT 'reserved',
    claim_token VARCHAR(128),
    lease_expires_at_ms INTEGER NOT NULL DEFAULT 0,
    failure_code VARCHAR(128) NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (owner_user_id, creation_request_id),
    UNIQUE (owner_user_id, agent_id),
    CONSTRAINT ck_agent_creation_requests_status
        CHECK (status IN ('pending', 'committed', 'deleted', 'failed')),
    CONSTRAINT ck_agent_creation_requests_stage
        CHECK (stage IN ('reserved', 'workspace_prepared'))
);

CREATE INDEX idx_agent_creation_requests_agent
    ON agent_creation_requests (owner_user_id, agent_id);

-- +goose Down
DROP INDEX IF EXISTS idx_agent_creation_requests_agent;
DROP TABLE IF EXISTS agent_creation_requests;
