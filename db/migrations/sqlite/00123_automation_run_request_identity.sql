-- +goose Up
ALTER TABLE automation_task_runs ADD COLUMN client_request_id VARCHAR(128);
ALTER TABLE automation_task_runs ADD COLUMN client_intent_digest VARCHAR(64);

CREATE UNIQUE INDEX uq_automation_task_runs_owner_request
    ON automation_task_runs (owner_user_id, client_request_id)
    WHERE client_request_id IS NOT NULL AND client_request_id <> '';

-- +goose Down
DROP INDEX IF EXISTS uq_automation_task_runs_owner_request;
ALTER TABLE automation_task_runs DROP COLUMN client_intent_digest;
ALTER TABLE automation_task_runs DROP COLUMN client_request_id;
