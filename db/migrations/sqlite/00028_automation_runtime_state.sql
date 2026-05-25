-- +goose Up
ALTER TABLE automation_cron_jobs ADD COLUMN next_run_at DATETIME;
ALTER TABLE automation_cron_jobs ADD COLUMN running_run_id VARCHAR(64);
ALTER TABLE automation_cron_jobs ADD COLUMN running_started_at DATETIME;
ALTER TABLE automation_cron_jobs ADD COLUMN last_run_at DATETIME;
ALTER TABLE automation_cron_jobs ADD COLUMN last_run_status VARCHAR(32);
ALTER TABLE automation_cron_jobs ADD COLUMN failure_streak INTEGER NOT NULL DEFAULT 0;
ALTER TABLE automation_cron_jobs ADD COLUMN last_error TEXT;
ALTER TABLE automation_cron_jobs ADD COLUMN last_delivery_status VARCHAR(32);

CREATE INDEX IF NOT EXISTS idx_automation_cron_jobs_runtime_due ON automation_cron_jobs (enabled, next_run_at);
CREATE INDEX IF NOT EXISTS idx_automation_cron_jobs_runtime_running ON automation_cron_jobs (running_run_id);

-- +goose Down
DROP INDEX IF EXISTS idx_automation_cron_jobs_runtime_running;
DROP INDEX IF EXISTS idx_automation_cron_jobs_runtime_due;

ALTER TABLE automation_cron_jobs DROP COLUMN last_delivery_status;
ALTER TABLE automation_cron_jobs DROP COLUMN last_error;
ALTER TABLE automation_cron_jobs DROP COLUMN failure_streak;
ALTER TABLE automation_cron_jobs DROP COLUMN last_run_status;
ALTER TABLE automation_cron_jobs DROP COLUMN last_run_at;
ALTER TABLE automation_cron_jobs DROP COLUMN running_started_at;
ALTER TABLE automation_cron_jobs DROP COLUMN running_run_id;
ALTER TABLE automation_cron_jobs DROP COLUMN next_run_at;
