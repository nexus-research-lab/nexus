-- +goose Up
ALTER TABLE automation_cron_runs ADD COLUMN IF NOT EXISTS delivery_status VARCHAR(32);

-- +goose Down
ALTER TABLE automation_cron_runs DROP COLUMN IF EXISTS delivery_status;
