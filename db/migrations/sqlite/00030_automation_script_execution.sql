-- +goose Up
ALTER TABLE automation_cron_jobs ADD COLUMN execution_kind VARCHAR(32) NOT NULL DEFAULT 'agent';

-- +goose Down
ALTER TABLE automation_cron_jobs DROP COLUMN execution_kind;
