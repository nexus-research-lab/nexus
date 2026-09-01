-- +goose Up
ALTER TABLE automation_scheduled_tasks
    ADD COLUMN IF NOT EXISTS deletion_state VARCHAR(32) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS deletion_token VARCHAR(128),
    ADD COLUMN IF NOT EXISTS deletion_claimed_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE automation_scheduled_tasks DROP COLUMN IF EXISTS deletion_claimed_at;
ALTER TABLE automation_scheduled_tasks DROP COLUMN IF EXISTS deletion_token;
ALTER TABLE automation_scheduled_tasks DROP COLUMN IF EXISTS deletion_state;
