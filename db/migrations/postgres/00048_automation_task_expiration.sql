-- +goose Up
ALTER TABLE automation_cron_jobs ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ;
CREATE INDEX IF NOT EXISTS idx_automation_cron_jobs_expires_at
    ON automation_cron_jobs (enabled, expires_at);

-- +goose Down
DROP INDEX IF EXISTS idx_automation_cron_jobs_expires_at;
ALTER TABLE automation_cron_jobs DROP COLUMN IF EXISTS expires_at;
