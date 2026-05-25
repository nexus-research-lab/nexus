-- +goose Up
ALTER TABLE automation_cron_jobs ADD COLUMN IF NOT EXISTS execution_kind VARCHAR(32) NOT NULL DEFAULT 'agent';

-- +goose Down
ALTER TABLE automation_cron_jobs DROP COLUMN IF EXISTS execution_kind;
