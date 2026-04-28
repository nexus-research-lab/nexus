-- +goose Up
ALTER TABLE automation_cron_jobs ADD COLUMN IF NOT EXISTS owner_user_id VARCHAR(64) NOT NULL DEFAULT '__system__';
ALTER TABLE automation_cron_jobs ADD COLUMN IF NOT EXISTS overlap_policy VARCHAR(32) NOT NULL DEFAULT 'skip';

UPDATE automation_cron_jobs AS job
SET owner_user_id = COALESCE(agent.owner_user_id, '__system__')
FROM agents AS agent
WHERE agent.id = job.agent_id;

ALTER TABLE automation_cron_jobs DROP CONSTRAINT IF EXISTS ck_automation_cron_jobs_source_context_type;
ALTER TABLE automation_cron_jobs ADD CONSTRAINT ck_automation_cron_jobs_source_context_type
    CHECK (source_context_type IS NULL OR source_context_type IN ('agent', 'room', 'chat'));
ALTER TABLE automation_cron_jobs DROP CONSTRAINT IF EXISTS ck_automation_cron_jobs_overlap_policy;
ALTER TABLE automation_cron_jobs ADD CONSTRAINT ck_automation_cron_jobs_overlap_policy
    CHECK (overlap_policy IN ('skip', 'allow'));

ALTER TABLE automation_cron_runs ADD COLUMN IF NOT EXISTS owner_user_id VARCHAR(64) NOT NULL DEFAULT '__system__';
ALTER TABLE automation_cron_runs ADD COLUMN IF NOT EXISTS trigger_kind VARCHAR(32) NOT NULL DEFAULT '';
ALTER TABLE automation_cron_runs ADD COLUMN IF NOT EXISTS session_key VARCHAR(255);
ALTER TABLE automation_cron_runs ADD COLUMN IF NOT EXISTS round_id VARCHAR(64);
ALTER TABLE automation_cron_runs ADD COLUMN IF NOT EXISTS session_id VARCHAR(255);
ALTER TABLE automation_cron_runs ADD COLUMN IF NOT EXISTS message_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE automation_cron_runs ADD COLUMN IF NOT EXISTS delivery_mode VARCHAR(32);
ALTER TABLE automation_cron_runs ADD COLUMN IF NOT EXISTS delivery_to VARCHAR(255);
ALTER TABLE automation_cron_runs ADD COLUMN IF NOT EXISTS result_summary TEXT;

UPDATE automation_cron_runs AS run
SET owner_user_id = COALESCE(job.owner_user_id, '__system__')
FROM automation_cron_jobs AS job
WHERE job.job_id = run.job_id;

ALTER TABLE automation_cron_runs DROP CONSTRAINT IF EXISTS ck_automation_cron_runs_status;
ALTER TABLE automation_cron_runs ADD CONSTRAINT ck_automation_cron_runs_status
    CHECK (status IN ('pending', 'running', 'succeeded', 'failed', 'cancelled', 'queued_to_main_session', 'skipped'));

CREATE INDEX IF NOT EXISTS idx_automation_cron_jobs_owner_created ON automation_cron_jobs (owner_user_id, created_at DESC, job_id DESC);
CREATE INDEX IF NOT EXISTS idx_automation_cron_jobs_owner_agent_created ON automation_cron_jobs (owner_user_id, agent_id, created_at DESC, job_id DESC);
CREATE INDEX IF NOT EXISTS idx_automation_cron_jobs_owner_enabled_agent ON automation_cron_jobs (owner_user_id, enabled, agent_id);
CREATE INDEX IF NOT EXISTS idx_automation_cron_runs_owner_job_created ON automation_cron_runs (owner_user_id, job_id, created_at DESC, run_id DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_automation_cron_runs_owner_job_created;
DROP INDEX IF EXISTS idx_automation_cron_jobs_owner_enabled_agent;
DROP INDEX IF EXISTS idx_automation_cron_jobs_owner_agent_created;
DROP INDEX IF EXISTS idx_automation_cron_jobs_owner_created;

ALTER TABLE automation_cron_runs DROP CONSTRAINT IF EXISTS ck_automation_cron_runs_status;
ALTER TABLE automation_cron_runs ADD CONSTRAINT ck_automation_cron_runs_status
    CHECK (status IN ('pending', 'running', 'succeeded', 'failed', 'cancelled'));

ALTER TABLE automation_cron_jobs DROP CONSTRAINT IF EXISTS ck_automation_cron_jobs_overlap_policy;
ALTER TABLE automation_cron_jobs DROP CONSTRAINT IF EXISTS ck_automation_cron_jobs_source_context_type;
ALTER TABLE automation_cron_jobs ADD CONSTRAINT ck_automation_cron_jobs_source_context_type
    CHECK (source_context_type IS NULL OR source_context_type IN ('agent', 'room'));

ALTER TABLE automation_cron_runs DROP COLUMN IF EXISTS result_summary;
ALTER TABLE automation_cron_runs DROP COLUMN IF EXISTS delivery_to;
ALTER TABLE automation_cron_runs DROP COLUMN IF EXISTS delivery_mode;
ALTER TABLE automation_cron_runs DROP COLUMN IF EXISTS message_count;
ALTER TABLE automation_cron_runs DROP COLUMN IF EXISTS session_id;
ALTER TABLE automation_cron_runs DROP COLUMN IF EXISTS round_id;
ALTER TABLE automation_cron_runs DROP COLUMN IF EXISTS session_key;
ALTER TABLE automation_cron_runs DROP COLUMN IF EXISTS trigger_kind;
ALTER TABLE automation_cron_runs DROP COLUMN IF EXISTS owner_user_id;

ALTER TABLE automation_cron_jobs DROP COLUMN IF EXISTS overlap_policy;
ALTER TABLE automation_cron_jobs DROP COLUMN IF EXISTS owner_user_id;
