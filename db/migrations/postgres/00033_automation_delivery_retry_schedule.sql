-- +goose Up
ALTER TABLE automation_cron_runs ADD COLUMN IF NOT EXISTS delivery_next_attempt_at TIMESTAMPTZ;
ALTER TABLE automation_cron_runs ADD COLUMN IF NOT EXISTS delivery_dead_letter_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE automation_cron_runs DROP COLUMN IF EXISTS delivery_dead_letter_at;
ALTER TABLE automation_cron_runs DROP COLUMN IF EXISTS delivery_next_attempt_at;
