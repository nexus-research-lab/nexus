-- +goose Up
ALTER TABLE provider ADD COLUMN preset_key VARCHAR(128) NOT NULL DEFAULT 'custom';
ALTER TABLE provider ADD COLUMN api_format VARCHAR(64) NOT NULL DEFAULT 'anthropic_messages';
ALTER TABLE provider ADD COLUMN models_path TEXT NOT NULL DEFAULT '/v1/models';
ALTER TABLE provider ADD COLUMN last_test_status VARCHAR(32) NOT NULL DEFAULT '';
ALTER TABLE provider ADD COLUMN last_test_error TEXT NOT NULL DEFAULT '';
ALTER TABLE provider ADD COLUMN last_test_at TIMESTAMP WITHOUT TIME ZONE;
ALTER TABLE runtimes ADD COLUMN model VARCHAR(255);

CREATE INDEX idx_provider_preset_format ON provider (preset_key, api_format);

CREATE TABLE provider_models (
    id VARCHAR(64) NOT NULL PRIMARY KEY,
    provider_id VARCHAR(64) NOT NULL,
    model_id VARCHAR(255) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    category VARCHAR(64) NOT NULL DEFAULT 'chat',
    enabled BOOLEAN NOT NULL DEFAULT false,
    capabilities_auto_json TEXT NOT NULL DEFAULT '{}',
    capabilities_override_json TEXT NOT NULL DEFAULT '{}',
    context_window INTEGER,
    max_output_tokens INTEGER,
    provider_options_json TEXT NOT NULL DEFAULT '{}',
    is_default BOOLEAN NOT NULL DEFAULT false,
    last_seen_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
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
    TRUE,
    '{}',
    '{}',
    NULL,
    NULL,
    '{}',
    CASE WHEN p.is_default THEN TRUE ELSE FALSE END,
    now(),
    now(),
    now()
FROM provider p
WHERE TRIM(COALESCE(p.model, '')) <> '';

UPDATE runtimes
SET model = TRIM(p.model)
FROM provider p
WHERE p.provider = runtimes.provider
  AND p.provider_kind = 'llm'
  AND p.is_default = FALSE
  AND TRIM(COALESCE(p.model, '')) <> ''
  AND runtimes.model IS NULL;

UPDATE runtimes
SET provider = NULL,
    model = NULL
FROM provider p
WHERE p.provider = runtimes.provider
  AND p.provider_kind = 'llm'
  AND p.is_default = TRUE;

ALTER TABLE provider DROP COLUMN is_default;
ALTER TABLE provider DROP COLUMN model;

ALTER TABLE automation_cron_jobs ADD COLUMN IF NOT EXISTS next_run_at TIMESTAMPTZ;
ALTER TABLE automation_cron_jobs ADD COLUMN IF NOT EXISTS running_run_id VARCHAR(64);
ALTER TABLE automation_cron_jobs ADD COLUMN IF NOT EXISTS running_started_at TIMESTAMPTZ;
ALTER TABLE automation_cron_jobs ADD COLUMN IF NOT EXISTS last_run_at TIMESTAMPTZ;
ALTER TABLE automation_cron_jobs ADD COLUMN IF NOT EXISTS last_run_status VARCHAR(32);
ALTER TABLE automation_cron_jobs ADD COLUMN IF NOT EXISTS failure_streak INTEGER NOT NULL DEFAULT 0;
ALTER TABLE automation_cron_jobs ADD COLUMN IF NOT EXISTS last_error TEXT;
ALTER TABLE automation_cron_jobs ADD COLUMN IF NOT EXISTS last_delivery_status VARCHAR(32);
ALTER TABLE automation_cron_jobs ADD COLUMN IF NOT EXISTS execution_kind VARCHAR(32) NOT NULL DEFAULT 'agent';

CREATE INDEX IF NOT EXISTS idx_automation_cron_jobs_runtime_due ON automation_cron_jobs (enabled, next_run_at);
CREATE INDEX IF NOT EXISTS idx_automation_cron_jobs_runtime_running ON automation_cron_jobs (running_run_id);

ALTER TABLE automation_cron_runs ADD COLUMN IF NOT EXISTS assistant_text TEXT;
ALTER TABLE automation_cron_runs ADD COLUMN IF NOT EXISTS result_text TEXT;
ALTER TABLE automation_cron_runs ADD COLUMN IF NOT EXISTS artifact_path VARCHAR(512);
ALTER TABLE automation_cron_runs ADD COLUMN IF NOT EXISTS delivery_status VARCHAR(32);
ALTER TABLE automation_cron_runs ADD COLUMN IF NOT EXISTS delivery_error TEXT;
ALTER TABLE automation_cron_runs ADD COLUMN IF NOT EXISTS delivered_at TIMESTAMPTZ;
ALTER TABLE automation_cron_runs ADD COLUMN IF NOT EXISTS delivery_attempts INTEGER NOT NULL DEFAULT 0;
ALTER TABLE automation_cron_runs ADD COLUMN IF NOT EXISTS delivery_next_attempt_at TIMESTAMPTZ;
ALTER TABLE automation_cron_runs ADD COLUMN IF NOT EXISTS delivery_dead_letter_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS im_ingress_messages (
    owner_user_id VARCHAR(64) NOT NULL,
    channel_type VARCHAR(32) NOT NULL,
    req_id VARCHAR(255) NOT NULL,
    agent_id VARCHAR(64) NOT NULL,
    session_key VARCHAR(512) NOT NULL,
    round_id VARCHAR(255) NOT NULL,
    status VARCHAR(32) NOT NULL,
    error_message TEXT,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    completed_at TIMESTAMP WITHOUT TIME ZONE,
    PRIMARY KEY (owner_user_id, channel_type, req_id),
    CONSTRAINT ck_im_ingress_messages_status CHECK (status IN ('processing', 'accepted', 'failed'))
);
CREATE INDEX IF NOT EXISTS idx_im_ingress_messages_updated ON im_ingress_messages (updated_at DESC);

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

DO $$
DECLARE
    constraint_name TEXT;
BEGIN
    SELECT con.conname
    INTO constraint_name
    FROM pg_constraint AS con
    JOIN pg_class AS rel ON rel.oid = con.conrelid
    JOIN pg_class AS ref ON ref.oid = con.confrelid
    JOIN pg_attribute AS att ON att.attrelid = rel.oid AND att.attnum = ANY(con.conkey)
    WHERE con.contype = 'f'
      AND rel.relname = 'automation_cron_runs'
      AND ref.relname = 'automation_cron_jobs'
      AND att.attname = 'job_id'
    LIMIT 1;

    IF constraint_name IS NOT NULL THEN
        EXECUTE format('ALTER TABLE automation_cron_runs DROP CONSTRAINT %I', constraint_name);
    END IF;
END $$;

-- +goose Down
DELETE FROM automation_cron_runs AS run
WHERE NOT EXISTS (
    SELECT 1
    FROM automation_cron_jobs AS job
    WHERE job.job_id = run.job_id
);

DO $$
DECLARE
    constraint_exists BOOLEAN;
BEGIN
    SELECT EXISTS (
        SELECT 1
        FROM pg_constraint AS con
        JOIN pg_class AS rel ON rel.oid = con.conrelid
        JOIN pg_class AS ref ON ref.oid = con.confrelid
        JOIN pg_attribute AS att ON att.attrelid = rel.oid AND att.attnum = ANY(con.conkey)
        WHERE con.contype = 'f'
          AND rel.relname = 'automation_cron_runs'
          AND ref.relname = 'automation_cron_jobs'
          AND att.attname = 'job_id'
    )
    INTO constraint_exists;

    IF NOT constraint_exists THEN
        ALTER TABLE automation_cron_runs
            ADD CONSTRAINT automation_cron_runs_job_id_fkey
            FOREIGN KEY (job_id) REFERENCES automation_cron_jobs (job_id) ON DELETE CASCADE;
    END IF;
END $$;

DROP INDEX IF EXISTS idx_automation_task_events_owner_created;
DROP INDEX IF EXISTS idx_automation_task_events_job_created;
DROP TABLE IF EXISTS automation_task_events;

DROP INDEX IF EXISTS idx_im_ingress_messages_updated;
DROP TABLE IF EXISTS im_ingress_messages;

ALTER TABLE automation_cron_runs DROP COLUMN IF EXISTS delivery_dead_letter_at;
ALTER TABLE automation_cron_runs DROP COLUMN IF EXISTS delivery_next_attempt_at;
ALTER TABLE automation_cron_runs DROP COLUMN IF EXISTS delivery_attempts;
ALTER TABLE automation_cron_runs DROP COLUMN IF EXISTS delivered_at;
ALTER TABLE automation_cron_runs DROP COLUMN IF EXISTS delivery_error;
ALTER TABLE automation_cron_runs DROP COLUMN IF EXISTS delivery_status;
ALTER TABLE automation_cron_runs DROP COLUMN IF EXISTS artifact_path;
ALTER TABLE automation_cron_runs DROP COLUMN IF EXISTS result_text;
ALTER TABLE automation_cron_runs DROP COLUMN IF EXISTS assistant_text;

DROP INDEX IF EXISTS idx_automation_cron_jobs_runtime_running;
DROP INDEX IF EXISTS idx_automation_cron_jobs_runtime_due;

ALTER TABLE automation_cron_jobs DROP COLUMN IF EXISTS execution_kind;
ALTER TABLE automation_cron_jobs DROP COLUMN IF EXISTS last_delivery_status;
ALTER TABLE automation_cron_jobs DROP COLUMN IF EXISTS last_error;
ALTER TABLE automation_cron_jobs DROP COLUMN IF EXISTS failure_streak;
ALTER TABLE automation_cron_jobs DROP COLUMN IF EXISTS last_run_status;
ALTER TABLE automation_cron_jobs DROP COLUMN IF EXISTS last_run_at;
ALTER TABLE automation_cron_jobs DROP COLUMN IF EXISTS running_started_at;
ALTER TABLE automation_cron_jobs DROP COLUMN IF EXISTS running_run_id;
ALTER TABLE automation_cron_jobs DROP COLUMN IF EXISTS next_run_at;

ALTER TABLE provider ADD COLUMN model VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE provider ADD COLUMN is_default BOOLEAN NOT NULL DEFAULT false;
UPDATE provider
SET model = COALESCE((
    SELECT pm.model_id
    FROM provider_models pm
    WHERE pm.provider_id = provider.id
      AND pm.enabled = TRUE
    ORDER BY pm.is_default DESC, pm.updated_at DESC
    LIMIT 1
), '');
UPDATE provider
SET is_default = EXISTS (
    SELECT 1
    FROM provider_models pm
    WHERE pm.provider_id = provider.id
      AND pm.is_default = TRUE
);
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
