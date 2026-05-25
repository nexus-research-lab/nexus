-- +goose Up
ALTER TABLE automation_cron_jobs ADD COLUMN IF NOT EXISTS next_run_at TIMESTAMPTZ;
ALTER TABLE automation_cron_jobs ADD COLUMN IF NOT EXISTS running_run_id VARCHAR(64);
ALTER TABLE automation_cron_jobs ADD COLUMN IF NOT EXISTS running_started_at TIMESTAMPTZ;
ALTER TABLE automation_cron_jobs ADD COLUMN IF NOT EXISTS last_run_at TIMESTAMPTZ;
ALTER TABLE automation_cron_jobs ADD COLUMN IF NOT EXISTS last_run_status VARCHAR(32);
ALTER TABLE automation_cron_jobs ADD COLUMN IF NOT EXISTS failure_streak INTEGER NOT NULL DEFAULT 0;
ALTER TABLE automation_cron_jobs ADD COLUMN IF NOT EXISTS last_error TEXT;
ALTER TABLE automation_cron_jobs ADD COLUMN IF NOT EXISTS last_delivery_status VARCHAR(32);

CREATE INDEX IF NOT EXISTS idx_automation_cron_jobs_runtime_due ON automation_cron_jobs (enabled, next_run_at);
CREATE INDEX IF NOT EXISTS idx_automation_cron_jobs_runtime_running ON automation_cron_jobs (running_run_id);

-- +goose Down
DROP INDEX IF EXISTS idx_automation_cron_jobs_runtime_running;
DROP INDEX IF EXISTS idx_automation_cron_jobs_runtime_due;

ALTER TABLE automation_cron_jobs DROP COLUMN IF EXISTS last_delivery_status;
ALTER TABLE automation_cron_jobs DROP COLUMN IF EXISTS last_error;
ALTER TABLE automation_cron_jobs DROP COLUMN IF EXISTS failure_streak;
ALTER TABLE automation_cron_jobs DROP COLUMN IF EXISTS last_run_status;
ALTER TABLE automation_cron_jobs DROP COLUMN IF EXISTS last_run_at;
ALTER TABLE automation_cron_jobs DROP COLUMN IF EXISTS running_started_at;
ALTER TABLE automation_cron_jobs DROP COLUMN IF EXISTS running_run_id;
ALTER TABLE automation_cron_jobs DROP COLUMN IF EXISTS next_run_at;
