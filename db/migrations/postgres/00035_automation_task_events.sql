-- +goose Up
CREATE TABLE IF NOT EXISTS automation_task_events (
    event_id VARCHAR(64) NOT NULL PRIMARY KEY,
    job_id VARCHAR(64) NOT NULL,
    owner_user_id VARCHAR(64) NOT NULL,
    agent_id VARCHAR(64) NOT NULL,
    action VARCHAR(32) NOT NULL,
    actor_user_id VARCHAR(64),
    actor_agent_id VARCHAR(64),
    run_id VARCHAR(64),
    detail_json TEXT NOT NULL DEFAULT '{}',
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_automation_task_events_job_created ON automation_task_events (job_id, created_at DESC, event_id DESC);
CREATE INDEX IF NOT EXISTS idx_automation_task_events_owner_created ON automation_task_events (owner_user_id, created_at DESC, event_id DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_automation_task_events_owner_created;
DROP INDEX IF EXISTS idx_automation_task_events_job_created;
DROP TABLE IF EXISTS automation_task_events;
