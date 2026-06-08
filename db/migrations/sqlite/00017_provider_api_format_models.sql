-- +goose Up
ALTER TABLE provider ADD COLUMN preset_key VARCHAR(128) NOT NULL DEFAULT 'custom';
ALTER TABLE provider ADD COLUMN api_format VARCHAR(64) NOT NULL DEFAULT 'anthropic_messages';
ALTER TABLE provider ADD COLUMN models_path TEXT NOT NULL DEFAULT '/v1/models';
ALTER TABLE provider ADD COLUMN last_test_status VARCHAR(32) NOT NULL DEFAULT '';
ALTER TABLE provider ADD COLUMN last_test_error TEXT NOT NULL DEFAULT '';
ALTER TABLE provider ADD COLUMN last_test_at DATETIME;
ALTER TABLE runtimes ADD COLUMN model VARCHAR(255);

CREATE INDEX idx_provider_preset_format ON provider (preset_key, api_format);

CREATE TABLE provider_models (
    id VARCHAR(64) NOT NULL PRIMARY KEY,
    provider_id VARCHAR(64) NOT NULL,
    model_id VARCHAR(255) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    category VARCHAR(64) NOT NULL DEFAULT 'chat',
    enabled BOOLEAN NOT NULL DEFAULT 0,
    capabilities_auto_json TEXT NOT NULL DEFAULT '{}',
    capabilities_override_json TEXT NOT NULL DEFAULT '{}',
    context_window INTEGER,
    max_output_tokens INTEGER,
    provider_options_json TEXT NOT NULL DEFAULT '{}',
    is_default BOOLEAN NOT NULL DEFAULT 0,
    last_seen_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    FOREIGN KEY(provider_id) REFERENCES provider (id) ON DELETE CASCADE,
    UNIQUE(provider_id, model_id)
);
CREATE INDEX idx_provider_models_provider_enabled ON provider_models (provider_id, enabled);
CREATE INDEX idx_provider_models_last_seen ON provider_models (provider_id, last_seen_at);
CREATE INDEX idx_provider_models_default ON provider_models (is_default, provider_id);

INSERT INTO provider_models (
    id, provider_id, model_id, display_name, category, enabled,
    capabilities_auto_json, capabilities_override_json, context_window,
    max_output_tokens, provider_options_json, is_default,
    last_seen_at, created_at, updated_at
)
SELECT
    'provider_model_' || p.id || '_legacy',
    p.id,
    TRIM(p.model),
    TRIM(p.model),
    'chat',
    1,
    '{}',
    '{}',
    NULL,
    NULL,
    '{}',
    CASE WHEN p.is_default THEN 1 ELSE 0 END,
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP,
    CURRENT_TIMESTAMP
FROM provider p
WHERE TRIM(COALESCE(p.model, '')) <> '';

UPDATE runtimes
SET model = (
    SELECT TRIM(p.model)
    FROM provider p
    WHERE p.provider = runtimes.provider
      AND p.provider_kind = 'llm'
      AND p.is_default = 0
      AND TRIM(COALESCE(p.model, '')) <> ''
    LIMIT 1
)
WHERE TRIM(COALESCE(provider, '')) <> ''
  AND model IS NULL
  AND EXISTS (
    SELECT 1
    FROM provider p
    WHERE p.provider = runtimes.provider
      AND p.provider_kind = 'llm'
      AND p.is_default = 0
      AND TRIM(COALESCE(p.model, '')) <> ''
  );

UPDATE runtimes
SET provider = NULL,
    model = NULL
WHERE EXISTS (
    SELECT 1
    FROM provider p
    WHERE p.provider = runtimes.provider
      AND p.provider_kind = 'llm'
      AND p.is_default = 1
);

ALTER TABLE provider DROP COLUMN is_default;
ALTER TABLE provider DROP COLUMN model;

ALTER TABLE automation_cron_jobs ADD COLUMN next_run_at DATETIME;
ALTER TABLE automation_cron_jobs ADD COLUMN running_run_id VARCHAR(64);
ALTER TABLE automation_cron_jobs ADD COLUMN running_started_at DATETIME;
ALTER TABLE automation_cron_jobs ADD COLUMN last_run_at DATETIME;
ALTER TABLE automation_cron_jobs ADD COLUMN last_run_status VARCHAR(32);
ALTER TABLE automation_cron_jobs ADD COLUMN failure_streak INTEGER NOT NULL DEFAULT 0;
ALTER TABLE automation_cron_jobs ADD COLUMN last_error TEXT;
ALTER TABLE automation_cron_jobs ADD COLUMN last_delivery_status VARCHAR(32);
ALTER TABLE automation_cron_jobs ADD COLUMN execution_kind VARCHAR(32) NOT NULL DEFAULT 'agent';

CREATE INDEX IF NOT EXISTS idx_automation_cron_jobs_runtime_due ON automation_cron_jobs (enabled, next_run_at);
CREATE INDEX IF NOT EXISTS idx_automation_cron_jobs_runtime_running ON automation_cron_jobs (running_run_id);

ALTER TABLE automation_cron_runs ADD COLUMN assistant_text TEXT;
ALTER TABLE automation_cron_runs ADD COLUMN result_text TEXT;
ALTER TABLE automation_cron_runs ADD COLUMN artifact_path VARCHAR(512);
ALTER TABLE automation_cron_runs ADD COLUMN delivery_status VARCHAR(32);
ALTER TABLE automation_cron_runs ADD COLUMN delivery_error TEXT;
ALTER TABLE automation_cron_runs ADD COLUMN delivered_at DATETIME;
ALTER TABLE automation_cron_runs ADD COLUMN delivery_attempts INTEGER NOT NULL DEFAULT 0;
ALTER TABLE automation_cron_runs ADD COLUMN delivery_next_attempt_at DATETIME;
ALTER TABLE automation_cron_runs ADD COLUMN delivery_dead_letter_at DATETIME;

CREATE TABLE im_ingress_messages (
    owner_user_id VARCHAR(64) NOT NULL,
    channel_type VARCHAR(32) NOT NULL,
    req_id VARCHAR(255) NOT NULL,
    agent_id VARCHAR(64) NOT NULL,
    session_key VARCHAR(512) NOT NULL,
    round_id VARCHAR(255) NOT NULL,
    status VARCHAR(32) NOT NULL,
    error_message TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    completed_at DATETIME,
    PRIMARY KEY (owner_user_id, channel_type, req_id),
    CONSTRAINT ck_im_ingress_messages_status CHECK (status IN ('processing', 'accepted', 'failed'))
);
CREATE INDEX idx_im_ingress_messages_updated ON im_ingress_messages (updated_at DESC);

CREATE TABLE automation_task_events (
    event_id VARCHAR(64) NOT NULL PRIMARY KEY,
    job_id VARCHAR(64) NOT NULL,
    owner_user_id VARCHAR(64) NOT NULL,
    agent_id VARCHAR(64) NOT NULL,
    action VARCHAR(32) NOT NULL,
    actor_user_id VARCHAR(64),
    actor_agent_id VARCHAR(64),
    run_id VARCHAR(64),
    detail_json TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL
);
CREATE INDEX idx_automation_task_events_job_created ON automation_task_events (job_id, created_at DESC, event_id DESC);
CREATE INDEX idx_automation_task_events_owner_created ON automation_task_events (owner_user_id, created_at DESC, event_id DESC);

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

DROP INDEX IF EXISTS idx_automation_task_events_owner_created;
DROP INDEX IF EXISTS idx_automation_task_events_job_created;
DROP TABLE IF EXISTS automation_task_events;

DROP INDEX IF EXISTS idx_im_ingress_messages_updated;
DROP TABLE IF EXISTS im_ingress_messages;

ALTER TABLE automation_cron_runs DROP COLUMN delivery_dead_letter_at;
ALTER TABLE automation_cron_runs DROP COLUMN delivery_next_attempt_at;
ALTER TABLE automation_cron_runs DROP COLUMN delivery_attempts;
ALTER TABLE automation_cron_runs DROP COLUMN delivered_at;
ALTER TABLE automation_cron_runs DROP COLUMN delivery_error;
ALTER TABLE automation_cron_runs DROP COLUMN delivery_status;
ALTER TABLE automation_cron_runs DROP COLUMN artifact_path;
ALTER TABLE automation_cron_runs DROP COLUMN result_text;
ALTER TABLE automation_cron_runs DROP COLUMN assistant_text;

DROP INDEX IF EXISTS idx_automation_cron_jobs_runtime_running;
DROP INDEX IF EXISTS idx_automation_cron_jobs_runtime_due;

ALTER TABLE automation_cron_jobs DROP COLUMN execution_kind;
ALTER TABLE automation_cron_jobs DROP COLUMN last_delivery_status;
ALTER TABLE automation_cron_jobs DROP COLUMN last_error;
ALTER TABLE automation_cron_jobs DROP COLUMN failure_streak;
ALTER TABLE automation_cron_jobs DROP COLUMN last_run_status;
ALTER TABLE automation_cron_jobs DROP COLUMN last_run_at;
ALTER TABLE automation_cron_jobs DROP COLUMN running_started_at;
ALTER TABLE automation_cron_jobs DROP COLUMN running_run_id;
ALTER TABLE automation_cron_jobs DROP COLUMN next_run_at;

ALTER TABLE provider ADD COLUMN model VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE provider ADD COLUMN is_default BOOLEAN NOT NULL DEFAULT 0;
UPDATE provider
SET model = COALESCE((
    SELECT pm.model_id
    FROM provider_models pm
    WHERE pm.provider_id = provider.id
      AND pm.enabled = 1
    ORDER BY pm.is_default DESC, pm.updated_at DESC
    LIMIT 1
), '');
UPDATE provider
SET is_default = CASE
    WHEN EXISTS (
        SELECT 1
        FROM provider_models pm
        WHERE pm.provider_id = provider.id
          AND pm.is_default = 1
    ) THEN 1
    ELSE 0
END;
ALTER TABLE runtimes DROP COLUMN model;
DROP INDEX idx_provider_models_default;
DROP INDEX idx_provider_models_last_seen;
DROP INDEX idx_provider_models_provider_enabled;
DROP TABLE provider_models;
DROP INDEX idx_provider_preset_format;
ALTER TABLE provider DROP COLUMN last_test_at;
ALTER TABLE provider DROP COLUMN last_test_error;
ALTER TABLE provider DROP COLUMN last_test_status;
ALTER TABLE provider DROP COLUMN models_path;
ALTER TABLE provider DROP COLUMN api_format;
ALTER TABLE provider DROP COLUMN preset_key;
