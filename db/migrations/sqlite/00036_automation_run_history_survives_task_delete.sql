-- +goose Up
PRAGMA foreign_keys = OFF;

DROP INDEX IF EXISTS idx_automation_cron_runs_owner_job_created;
DROP INDEX IF EXISTS idx_automation_cron_runs_job_created;
DROP INDEX IF EXISTS idx_automation_cron_runs_status;
DROP INDEX IF EXISTS idx_automation_cron_runs_job;

ALTER TABLE automation_cron_runs RENAME TO automation_cron_runs_old;

CREATE TABLE automation_cron_runs (
    run_id VARCHAR(64) NOT NULL PRIMARY KEY,
    job_id VARCHAR(64) NOT NULL,
    owner_user_id VARCHAR(64) NOT NULL DEFAULT '__system__',
    status VARCHAR(32) NOT NULL,
    trigger_kind VARCHAR(32) NOT NULL DEFAULT '',
    session_key VARCHAR(255),
    round_id VARCHAR(64),
    session_id VARCHAR(255),
    message_count INTEGER NOT NULL DEFAULT 0,
    delivery_mode VARCHAR(32),
    delivery_to VARCHAR(255),
    delivery_status VARCHAR(32),
    delivery_error TEXT,
    delivered_at DATETIME,
    delivery_attempts INTEGER NOT NULL DEFAULT 0,
    delivery_next_attempt_at DATETIME,
    delivery_dead_letter_at DATETIME,
    scheduled_for DATETIME,
    started_at DATETIME,
    finished_at DATETIME,
    attempts INTEGER NOT NULL,
    error_message TEXT,
    result_summary TEXT,
    assistant_text TEXT,
    result_text TEXT,
    artifact_path VARCHAR(512),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_automation_cron_runs_status CHECK (status IN ('pending', 'running', 'succeeded', 'failed', 'cancelled', 'queued_to_main_session', 'skipped'))
);

INSERT INTO automation_cron_runs (
    run_id, job_id, owner_user_id, status, trigger_kind, session_key, round_id,
    session_id, message_count, delivery_mode, delivery_to, delivery_status,
    delivery_error, delivered_at, delivery_attempts, delivery_next_attempt_at,
    delivery_dead_letter_at, scheduled_for, started_at, finished_at, attempts,
    error_message, result_summary, assistant_text, result_text, artifact_path,
    created_at, updated_at
)
SELECT
    run_id, job_id, owner_user_id, status, trigger_kind, session_key, round_id,
    session_id, message_count, delivery_mode, delivery_to, delivery_status,
    delivery_error, delivered_at, delivery_attempts, delivery_next_attempt_at,
    delivery_dead_letter_at, scheduled_for, started_at, finished_at, attempts,
    error_message, result_summary, assistant_text, result_text, artifact_path,
    created_at, updated_at
FROM automation_cron_runs_old;

DROP TABLE automation_cron_runs_old;

CREATE INDEX idx_automation_cron_runs_job ON automation_cron_runs (job_id);
CREATE INDEX idx_automation_cron_runs_status ON automation_cron_runs (status);
CREATE INDEX IF NOT EXISTS idx_automation_cron_runs_job_created ON automation_cron_runs (job_id, created_at DESC, run_id DESC);
CREATE INDEX IF NOT EXISTS idx_automation_cron_runs_owner_job_created ON automation_cron_runs (owner_user_id, job_id, created_at DESC, run_id DESC);

PRAGMA foreign_keys = ON;

-- +goose Down
PRAGMA foreign_keys = OFF;

DELETE FROM automation_cron_runs
WHERE NOT EXISTS (
    SELECT 1
    FROM automation_cron_jobs
    WHERE automation_cron_jobs.job_id = automation_cron_runs.job_id
);

DROP INDEX IF EXISTS idx_automation_cron_runs_owner_job_created;
DROP INDEX IF EXISTS idx_automation_cron_runs_job_created;
DROP INDEX IF EXISTS idx_automation_cron_runs_status;
DROP INDEX IF EXISTS idx_automation_cron_runs_job;

ALTER TABLE automation_cron_runs RENAME TO automation_cron_runs_old;

CREATE TABLE automation_cron_runs (
    run_id VARCHAR(64) NOT NULL PRIMARY KEY,
    job_id VARCHAR(64) NOT NULL,
    owner_user_id VARCHAR(64) NOT NULL DEFAULT '__system__',
    status VARCHAR(32) NOT NULL,
    trigger_kind VARCHAR(32) NOT NULL DEFAULT '',
    session_key VARCHAR(255),
    round_id VARCHAR(64),
    session_id VARCHAR(255),
    message_count INTEGER NOT NULL DEFAULT 0,
    delivery_mode VARCHAR(32),
    delivery_to VARCHAR(255),
    delivery_status VARCHAR(32),
    delivery_error TEXT,
    delivered_at DATETIME,
    delivery_attempts INTEGER NOT NULL DEFAULT 0,
    delivery_next_attempt_at DATETIME,
    delivery_dead_letter_at DATETIME,
    scheduled_for DATETIME,
    started_at DATETIME,
    finished_at DATETIME,
    attempts INTEGER NOT NULL,
    error_message TEXT,
    result_summary TEXT,
    assistant_text TEXT,
    result_text TEXT,
    artifact_path VARCHAR(512),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_automation_cron_runs_status CHECK (status IN ('pending', 'running', 'succeeded', 'failed', 'cancelled', 'queued_to_main_session', 'skipped')),
    FOREIGN KEY(job_id) REFERENCES automation_cron_jobs (job_id) ON DELETE CASCADE
);

INSERT INTO automation_cron_runs (
    run_id, job_id, owner_user_id, status, trigger_kind, session_key, round_id,
    session_id, message_count, delivery_mode, delivery_to, delivery_status,
    delivery_error, delivered_at, delivery_attempts, delivery_next_attempt_at,
    delivery_dead_letter_at, scheduled_for, started_at, finished_at, attempts,
    error_message, result_summary, assistant_text, result_text, artifact_path,
    created_at, updated_at
)
SELECT
    run_id, job_id, owner_user_id, status, trigger_kind, session_key, round_id,
    session_id, message_count, delivery_mode, delivery_to, delivery_status,
    delivery_error, delivered_at, delivery_attempts, delivery_next_attempt_at,
    delivery_dead_letter_at, scheduled_for, started_at, finished_at, attempts,
    error_message, result_summary, assistant_text, result_text, artifact_path,
    created_at, updated_at
FROM automation_cron_runs_old;

DROP TABLE automation_cron_runs_old;

CREATE INDEX idx_automation_cron_runs_job ON automation_cron_runs (job_id);
CREATE INDEX idx_automation_cron_runs_status ON automation_cron_runs (status);
CREATE INDEX IF NOT EXISTS idx_automation_cron_runs_job_created ON automation_cron_runs (job_id, created_at DESC, run_id DESC);
CREATE INDEX IF NOT EXISTS idx_automation_cron_runs_owner_job_created ON automation_cron_runs (owner_user_id, job_id, created_at DESC, run_id DESC);

PRAGMA foreign_keys = ON;
