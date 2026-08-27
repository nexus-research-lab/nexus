-- +goose Up
ALTER TABLE automation_scheduled_tasks ADD COLUMN deletion_state VARCHAR(32) NOT NULL DEFAULT '';
ALTER TABLE automation_scheduled_tasks ADD COLUMN deletion_token VARCHAR(128);
ALTER TABLE automation_scheduled_tasks ADD COLUMN deletion_claimed_at DATETIME;

-- +goose Down
ALTER TABLE automation_scheduled_tasks DROP COLUMN deletion_claimed_at;
ALTER TABLE automation_scheduled_tasks DROP COLUMN deletion_token;
ALTER TABLE automation_scheduled_tasks DROP COLUMN deletion_state;
