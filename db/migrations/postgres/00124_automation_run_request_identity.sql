-- +goose Up
ALTER TABLE automation_task_runs
    ADD COLUMN IF NOT EXISTS client_request_id VARCHAR(128),
    ADD COLUMN IF NOT EXISTS client_intent_digest VARCHAR(64);

CREATE UNIQUE INDEX IF NOT EXISTS uq_automation_task_runs_owner_request
    ON automation_task_runs (owner_user_id, client_request_id)
    WHERE client_request_id IS NOT NULL AND client_request_id <> '';

-- +goose Down
DROP INDEX IF EXISTS uq_automation_task_runs_owner_request;
ALTER TABLE automation_task_runs DROP COLUMN IF EXISTS client_intent_digest;
ALTER TABLE automation_task_runs DROP COLUMN IF EXISTS client_request_id;
