-- +goose Up
CREATE TABLE IF NOT EXISTS automation_runtime_commands (
    owner_user_id VARCHAR(64) NOT NULL,
    request_id VARCHAR(128) NOT NULL,
    actor_agent_id VARCHAR(64) NOT NULL,
    operation VARCHAR(32) NOT NULL,
    intent_digest VARCHAR(64) NOT NULL,
    approval_request_id VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL,
    result_json TEXT,
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (owner_user_id, request_id)
);

CREATE INDEX IF NOT EXISTS idx_automation_runtime_commands_status
    ON automation_runtime_commands (owner_user_id, status, updated_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_automation_runtime_commands_status;
DROP TABLE IF EXISTS automation_runtime_commands;
