-- +goose Up
ALTER TABLE automation_cron_runs ADD COLUMN IF NOT EXISTS delivery_error TEXT;
ALTER TABLE automation_cron_runs ADD COLUMN IF NOT EXISTS delivered_at TIMESTAMPTZ;
ALTER TABLE automation_cron_runs ADD COLUMN IF NOT EXISTS delivery_attempts INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE automation_cron_runs DROP COLUMN IF EXISTS delivery_attempts;
ALTER TABLE automation_cron_runs DROP COLUMN IF EXISTS delivered_at;
ALTER TABLE automation_cron_runs DROP COLUMN IF EXISTS delivery_error;
