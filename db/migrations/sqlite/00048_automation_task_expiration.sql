-- +goose Up
ALTER TABLE automation_cron_jobs ADD COLUMN expires_at DATETIME;
CREATE INDEX idx_automation_cron_jobs_expires_at
    ON automation_cron_jobs (enabled, expires_at);

-- +goose Down
DROP INDEX IF EXISTS idx_automation_cron_jobs_expires_at;
ALTER TABLE automation_cron_jobs DROP COLUMN expires_at;
