-- +goose Up
ALTER TABLE automation_scheduled_tasks ADD COLUMN permission_policy_json TEXT NOT NULL DEFAULT '{}';
ALTER TABLE automation_scheduled_tasks ADD COLUMN permission_policy_revision INTEGER NOT NULL DEFAULT 0;
ALTER TABLE automation_scheduled_tasks ADD COLUMN permission_state VARCHAR(32) NOT NULL DEFAULT 'uninitialized';
ALTER TABLE automation_scheduled_tasks ADD COLUMN pending_permission_request_id VARCHAR(64);

ALTER TABLE automation_task_runs ADD COLUMN permission_policy_revision INTEGER NOT NULL DEFAULT 0;
ALTER TABLE automation_task_runs ADD COLUMN block_state VARCHAR(32) NOT NULL DEFAULT '';
ALTER TABLE automation_task_runs ADD COLUMN blocked_request_id VARCHAR(64);
ALTER TABLE automation_task_runs ADD COLUMN effect_started BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE automation_permission_requests (
    request_id VARCHAR(64) NOT NULL PRIMARY KEY,
    owner_user_id VARCHAR(64) NOT NULL,
    job_id VARCHAR(64) NOT NULL,
    run_id VARCHAR(64),
    policy_revision INTEGER NOT NULL,
    kind VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL,
    decision VARCHAR(32),
    tool_name VARCHAR(255) NOT NULL,
    connector_id VARCHAR(64),
    effect VARCHAR(32) NOT NULL,
    resource_scope TEXT,
    input_fingerprint VARCHAR(64) NOT NULL,
    capability_json TEXT NOT NULL,
    input_summary_json TEXT NOT NULL DEFAULT '{}',
    title VARCHAR(255),
    description TEXT,
    reason TEXT,
    session_key VARCHAR(255),
    round_id VARCHAR(64),
    tool_use_id VARCHAR(255),
    resume_safe BOOLEAN NOT NULL DEFAULT TRUE,
    resolved_by_user_id VARCHAR(64),
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_automation_permission_requests_kind CHECK (
        kind IN ('tool', 'script', 'connector_reauth', 'human_input')
    ),
    CONSTRAINT ck_automation_permission_requests_status CHECK (
        status IN ('pending', 'approved', 'denied', 'superseded', 'cancelled')
    )
);

CREATE INDEX idx_automation_permission_requests_owner_status_created
    ON automation_permission_requests (owner_user_id, status, created_at DESC, request_id DESC);
CREATE INDEX idx_automation_permission_requests_job_created
    ON automation_permission_requests (job_id, created_at DESC, request_id DESC);
CREATE INDEX idx_automation_permission_requests_run_created
    ON automation_permission_requests (run_id, created_at DESC, request_id DESC);
CREATE UNIQUE INDEX uq_automation_permission_requests_pending_capability
    ON automation_permission_requests (owner_user_id, job_id, run_id, kind, input_fingerprint)
    WHERE status = 'pending';
CREATE INDEX idx_automation_scheduled_tasks_permission_state
    ON automation_scheduled_tasks (owner_user_id, permission_state, updated_at DESC);
CREATE INDEX idx_automation_task_runs_block_state
    ON automation_task_runs (owner_user_id, block_state, updated_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_automation_task_runs_block_state;
DROP INDEX IF EXISTS idx_automation_scheduled_tasks_permission_state;
DROP INDEX IF EXISTS uq_automation_permission_requests_pending_capability;
DROP INDEX IF EXISTS idx_automation_permission_requests_run_created;
DROP INDEX IF EXISTS idx_automation_permission_requests_job_created;
DROP INDEX IF EXISTS idx_automation_permission_requests_owner_status_created;
DROP TABLE IF EXISTS automation_permission_requests;

ALTER TABLE automation_task_runs DROP COLUMN effect_started;
ALTER TABLE automation_task_runs DROP COLUMN blocked_request_id;
ALTER TABLE automation_task_runs DROP COLUMN block_state;
ALTER TABLE automation_task_runs DROP COLUMN permission_policy_revision;

ALTER TABLE automation_scheduled_tasks DROP COLUMN pending_permission_request_id;
ALTER TABLE automation_scheduled_tasks DROP COLUMN permission_state;
ALTER TABLE automation_scheduled_tasks DROP COLUMN permission_policy_revision;
ALTER TABLE automation_scheduled_tasks DROP COLUMN permission_policy_json;
