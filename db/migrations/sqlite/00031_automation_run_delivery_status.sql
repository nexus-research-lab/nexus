-- +goose Up
ALTER TABLE automation_cron_runs ADD COLUMN delivery_status VARCHAR(32);

-- +goose Down
ALTER TABLE automation_cron_runs DROP COLUMN delivery_status;
