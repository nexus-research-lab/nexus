-- +goose Up
ALTER TABLE automation_cron_runs ADD COLUMN delivery_error TEXT;
ALTER TABLE automation_cron_runs ADD COLUMN delivered_at DATETIME;
ALTER TABLE automation_cron_runs ADD COLUMN delivery_attempts INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE automation_cron_runs DROP COLUMN delivery_attempts;
ALTER TABLE automation_cron_runs DROP COLUMN delivered_at;
ALTER TABLE automation_cron_runs DROP COLUMN delivery_error;
