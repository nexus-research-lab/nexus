-- +goose Up
ALTER TABLE automation_cron_runs ADD COLUMN delivery_next_attempt_at DATETIME;
ALTER TABLE automation_cron_runs ADD COLUMN delivery_dead_letter_at DATETIME;

-- +goose Down
ALTER TABLE automation_cron_runs DROP COLUMN delivery_dead_letter_at;
ALTER TABLE automation_cron_runs DROP COLUMN delivery_next_attempt_at;
