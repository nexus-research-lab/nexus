-- +goose Up
PRAGMA foreign_keys = OFF;

DROP INDEX IF EXISTS idx_automation_cron_runs_job_created;
DROP INDEX IF EXISTS idx_automation_cron_runs_status;
DROP INDEX IF EXISTS idx_automation_cron_runs_job;
DROP INDEX IF EXISTS idx_automation_cron_jobs_enabled_agent;
DROP INDEX IF EXISTS idx_automation_cron_jobs_agent_created;
DROP INDEX IF EXISTS idx_automation_cron_jobs_created;
DROP INDEX IF EXISTS idx_automation_cron_jobs_agent;

ALTER TABLE automation_cron_jobs RENAME TO automation_cron_jobs_old;

CREATE TABLE automation_cron_jobs (
    job_id VARCHAR(64) NOT NULL PRIMARY KEY,
    owner_user_id VARCHAR(64) NOT NULL DEFAULT '__system__',
    name VARCHAR(255) NOT NULL,
    agent_id VARCHAR(64) NOT NULL,
    schedule_kind VARCHAR(32) NOT NULL,
    run_at VARCHAR(32),
    interval_seconds INTEGER,
    cron_expression VARCHAR(255),
    timezone VARCHAR(64) NOT NULL,
    instruction TEXT NOT NULL,
    session_target_kind VARCHAR(32) NOT NULL,
    bound_session_key VARCHAR(255),
    named_session_key VARCHAR(255),
    wake_mode VARCHAR(32) NOT NULL,
    delivery_mode VARCHAR(32) NOT NULL,
    delivery_channel VARCHAR(64),
    delivery_to VARCHAR(255),
    delivery_account_id VARCHAR(64),
    delivery_thread_id VARCHAR(255),
    source_kind VARCHAR(32) NOT NULL DEFAULT 'system',
    source_creator_agent_id VARCHAR(64),
    source_context_type VARCHAR(32),
    source_context_id VARCHAR(255),
    source_context_label VARCHAR(255),
    source_session_key VARCHAR(255),
    source_session_label VARCHAR(255),
    overlap_policy VARCHAR(32) NOT NULL DEFAULT 'skip',
    enabled BOOLEAN NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_automation_cron_jobs_schedule_kind CHECK (schedule_kind IN ('every', 'cron', 'at')),
    CONSTRAINT ck_automation_cron_jobs_session_target_kind CHECK (session_target_kind IN ('isolated', 'main', 'bound', 'named')),
    CONSTRAINT ck_automation_cron_jobs_wake_mode CHECK (wake_mode IN ('now', 'next-heartbeat')),
    CONSTRAINT ck_automation_cron_jobs_delivery_mode CHECK (delivery_mode IN ('none', 'last', 'explicit')),
    CONSTRAINT ck_automation_cron_jobs_source_kind CHECK (source_kind IN ('user_page', 'agent', 'cli', 'system')),
    CONSTRAINT ck_automation_cron_jobs_source_context_type CHECK (source_context_type IS NULL OR source_context_type IN ('agent', 'room', 'chat')),
    CONSTRAINT ck_automation_cron_jobs_overlap_policy CHECK (overlap_policy IN ('skip', 'allow')),
    FOREIGN KEY(agent_id) REFERENCES agents (id) ON DELETE CASCADE
);

INSERT INTO automation_cron_jobs (
    job_id, owner_user_id, name, agent_id, schedule_kind, run_at, interval_seconds, cron_expression,
    timezone, instruction, session_target_kind, bound_session_key, named_session_key, wake_mode,
    delivery_mode, delivery_channel, delivery_to, delivery_account_id, delivery_thread_id,
    source_kind, source_creator_agent_id, source_context_type, source_context_id, source_context_label,
    source_session_key, source_session_label, overlap_policy, enabled, created_at, updated_at
)
SELECT
    old.job_id,
    COALESCE(agent.owner_user_id, '__system__'),
    old.name,
    old.agent_id,
    old.schedule_kind,
    old.run_at,
    old.interval_seconds,
    old.cron_expression,
    old.timezone,
    old.instruction,
    old.session_target_kind,
    old.bound_session_key,
    old.named_session_key,
    old.wake_mode,
    old.delivery_mode,
    old.delivery_channel,
    old.delivery_to,
    old.delivery_account_id,
    old.delivery_thread_id,
    old.source_kind,
    old.source_creator_agent_id,
    old.source_context_type,
    old.source_context_id,
    old.source_context_label,
    old.source_session_key,
    old.source_session_label,
    'skip',
    old.enabled,
    old.created_at,
    old.updated_at
FROM automation_cron_jobs_old AS old
LEFT JOIN agents AS agent ON agent.id = old.agent_id;

DROP TABLE automation_cron_jobs_old;

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
    scheduled_for DATETIME,
    started_at DATETIME,
    finished_at DATETIME,
    attempts INTEGER NOT NULL,
    error_message TEXT,
    result_summary TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_automation_cron_runs_status CHECK (status IN ('pending', 'running', 'succeeded', 'failed', 'cancelled', 'queued_to_main_session', 'skipped')),
    FOREIGN KEY(job_id) REFERENCES automation_cron_jobs (job_id) ON DELETE CASCADE
);

INSERT INTO automation_cron_runs (
    run_id, job_id, owner_user_id, status, trigger_kind, session_key, round_id, session_id,
    message_count, delivery_mode, delivery_to, scheduled_for, started_at, finished_at,
    attempts, error_message, result_summary, created_at, updated_at
)
SELECT
    old.run_id,
    old.job_id,
    COALESCE(job.owner_user_id, '__system__'),
    old.status,
    '',
    NULL,
    NULL,
    NULL,
    0,
    NULL,
    NULL,
    old.scheduled_for,
    old.started_at,
    old.finished_at,
    old.attempts,
    old.error_message,
    NULL,
    old.created_at,
    old.updated_at
FROM automation_cron_runs_old AS old
LEFT JOIN automation_cron_jobs AS job ON job.job_id = old.job_id;

DROP TABLE automation_cron_runs_old;

CREATE INDEX idx_automation_cron_jobs_agent ON automation_cron_jobs (agent_id);
CREATE INDEX IF NOT EXISTS idx_automation_cron_jobs_created ON automation_cron_jobs (created_at DESC, job_id DESC);
CREATE INDEX IF NOT EXISTS idx_automation_cron_jobs_agent_created ON automation_cron_jobs (agent_id, created_at DESC, job_id DESC);
CREATE INDEX IF NOT EXISTS idx_automation_cron_jobs_enabled_agent ON automation_cron_jobs (enabled, agent_id);
CREATE INDEX IF NOT EXISTS idx_automation_cron_jobs_owner_created ON automation_cron_jobs (owner_user_id, created_at DESC, job_id DESC);
CREATE INDEX IF NOT EXISTS idx_automation_cron_jobs_owner_agent_created ON automation_cron_jobs (owner_user_id, agent_id, created_at DESC, job_id DESC);
CREATE INDEX IF NOT EXISTS idx_automation_cron_jobs_owner_enabled_agent ON automation_cron_jobs (owner_user_id, enabled, agent_id);
CREATE INDEX idx_automation_cron_runs_job ON automation_cron_runs (job_id);
CREATE INDEX idx_automation_cron_runs_status ON automation_cron_runs (status);
CREATE INDEX IF NOT EXISTS idx_automation_cron_runs_job_created ON automation_cron_runs (job_id, created_at DESC, run_id DESC);
CREATE INDEX IF NOT EXISTS idx_automation_cron_runs_owner_job_created ON automation_cron_runs (owner_user_id, job_id, created_at DESC, run_id DESC);

PRAGMA foreign_keys = ON;

-- +goose Down
PRAGMA foreign_keys = OFF;

DROP INDEX IF EXISTS idx_automation_cron_runs_owner_job_created;
DROP INDEX IF EXISTS idx_automation_cron_runs_job_created;
DROP INDEX IF EXISTS idx_automation_cron_runs_status;
DROP INDEX IF EXISTS idx_automation_cron_runs_job;
DROP INDEX IF EXISTS idx_automation_cron_jobs_owner_enabled_agent;
DROP INDEX IF EXISTS idx_automation_cron_jobs_owner_agent_created;
DROP INDEX IF EXISTS idx_automation_cron_jobs_owner_created;
DROP INDEX IF EXISTS idx_automation_cron_jobs_enabled_agent;
DROP INDEX IF EXISTS idx_automation_cron_jobs_agent_created;
DROP INDEX IF EXISTS idx_automation_cron_jobs_created;
DROP INDEX IF EXISTS idx_automation_cron_jobs_agent;

ALTER TABLE automation_cron_jobs RENAME TO automation_cron_jobs_new;

CREATE TABLE automation_cron_jobs (
    job_id VARCHAR(64) NOT NULL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    agent_id VARCHAR(64) NOT NULL,
    schedule_kind VARCHAR(32) NOT NULL,
    run_at VARCHAR(32),
    interval_seconds INTEGER,
    cron_expression VARCHAR(255),
    timezone VARCHAR(64) NOT NULL,
    instruction TEXT NOT NULL,
    session_target_kind VARCHAR(32) NOT NULL,
    bound_session_key VARCHAR(255),
    named_session_key VARCHAR(255),
    wake_mode VARCHAR(32) NOT NULL,
    delivery_mode VARCHAR(32) NOT NULL,
    delivery_channel VARCHAR(64),
    delivery_to VARCHAR(255),
    delivery_account_id VARCHAR(64),
    delivery_thread_id VARCHAR(255),
    source_kind VARCHAR(32) NOT NULL DEFAULT 'system',
    source_creator_agent_id VARCHAR(64),
    source_context_type VARCHAR(32),
    source_context_id VARCHAR(255),
    source_context_label VARCHAR(255),
    source_session_key VARCHAR(255),
    source_session_label VARCHAR(255),
    enabled BOOLEAN NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_automation_cron_jobs_schedule_kind CHECK (schedule_kind IN ('every', 'cron', 'at')),
    CONSTRAINT ck_automation_cron_jobs_session_target_kind CHECK (session_target_kind IN ('isolated', 'main', 'bound', 'named')),
    CONSTRAINT ck_automation_cron_jobs_wake_mode CHECK (wake_mode IN ('now', 'next-heartbeat')),
    CONSTRAINT ck_automation_cron_jobs_delivery_mode CHECK (delivery_mode IN ('none', 'last', 'explicit')),
    CONSTRAINT ck_automation_cron_jobs_source_kind CHECK (source_kind IN ('user_page', 'agent', 'cli', 'system')),
    CONSTRAINT ck_automation_cron_jobs_source_context_type CHECK (source_context_type IS NULL OR source_context_type IN ('agent', 'room')),
    FOREIGN KEY(agent_id) REFERENCES agents (id) ON DELETE CASCADE
);

INSERT INTO automation_cron_jobs (
    job_id, name, agent_id, schedule_kind, run_at, interval_seconds, cron_expression,
    timezone, instruction, session_target_kind, bound_session_key, named_session_key, wake_mode,
    delivery_mode, delivery_channel, delivery_to, delivery_account_id, delivery_thread_id,
    source_kind, source_creator_agent_id, source_context_type, source_context_id, source_context_label,
    source_session_key, source_session_label, enabled, created_at, updated_at
)
SELECT
    job_id, name, agent_id, schedule_kind, run_at, interval_seconds, cron_expression,
    timezone, instruction, session_target_kind, bound_session_key, named_session_key, wake_mode,
    delivery_mode, delivery_channel, delivery_to, delivery_account_id, delivery_thread_id,
    source_kind, source_creator_agent_id,
    CASE WHEN source_context_type IN ('agent', 'room') THEN source_context_type ELSE NULL END,
    source_context_id, source_context_label, source_session_key, source_session_label,
    enabled, created_at, updated_at
FROM automation_cron_jobs_new;

DROP TABLE automation_cron_jobs_new;

ALTER TABLE automation_cron_runs RENAME TO automation_cron_runs_new;

CREATE TABLE automation_cron_runs (
    run_id VARCHAR(64) NOT NULL PRIMARY KEY,
    job_id VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL,
    scheduled_for DATETIME,
    started_at DATETIME,
    finished_at DATETIME,
    attempts INTEGER NOT NULL,
    error_message TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_automation_cron_runs_status CHECK (status IN ('pending', 'running', 'succeeded', 'failed', 'cancelled')),
    FOREIGN KEY(job_id) REFERENCES automation_cron_jobs (job_id) ON DELETE CASCADE
);

INSERT INTO automation_cron_runs (
    run_id, job_id, status, scheduled_for, started_at, finished_at, attempts, error_message, created_at, updated_at
)
SELECT
    run_id, job_id,
    CASE WHEN status IN ('pending', 'running', 'succeeded', 'failed', 'cancelled') THEN status ELSE 'cancelled' END,
    scheduled_for, started_at, finished_at, attempts, error_message, created_at, updated_at
FROM automation_cron_runs_new;

DROP TABLE automation_cron_runs_new;

CREATE INDEX idx_automation_cron_jobs_agent ON automation_cron_jobs (agent_id);
CREATE INDEX IF NOT EXISTS idx_automation_cron_jobs_created ON automation_cron_jobs (created_at DESC, job_id DESC);
CREATE INDEX IF NOT EXISTS idx_automation_cron_jobs_agent_created ON automation_cron_jobs (agent_id, created_at DESC, job_id DESC);
CREATE INDEX IF NOT EXISTS idx_automation_cron_jobs_enabled_agent ON automation_cron_jobs (enabled, agent_id);
CREATE INDEX idx_automation_cron_runs_job ON automation_cron_runs (job_id);
CREATE INDEX idx_automation_cron_runs_status ON automation_cron_runs (status);
CREATE INDEX IF NOT EXISTS idx_automation_cron_runs_job_created ON automation_cron_runs (job_id, created_at DESC, run_id DESC);

PRAGMA foreign_keys = ON;
