-- +goose Up
ALTER TABLE automation_scheduled_tasks
    ADD COLUMN IF NOT EXISTS configuration_version BIGINT NOT NULL DEFAULT 1
    CHECK (configuration_version >= 1);

ALTER TABLE automation_heartbeat_states
    ADD COLUMN IF NOT EXISTS configuration_version BIGINT NOT NULL DEFAULT 1
    CHECK (configuration_version >= 1);

CREATE TABLE IF NOT EXISTS automation_task_create_requests (
    owner_user_id VARCHAR(64) NOT NULL,
    request_id VARCHAR(128) NOT NULL,
    job_id VARCHAR(64) NOT NULL,
    agent_id VARCHAR(64) NOT NULL,
    intent_digest VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (owner_user_id, request_id)
);

CREATE INDEX IF NOT EXISTS idx_automation_task_create_requests_job
    ON automation_task_create_requests (job_id);

-- +goose Down
DROP INDEX IF EXISTS idx_automation_task_create_requests_job;
DROP TABLE IF EXISTS automation_task_create_requests;
ALTER TABLE automation_heartbeat_states DROP COLUMN IF EXISTS configuration_version;
ALTER TABLE automation_scheduled_tasks DROP COLUMN IF EXISTS configuration_version;
