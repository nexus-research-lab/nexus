-- +goose Up
ALTER TABLE automation_scheduled_tasks ADD COLUMN IF NOT EXISTS delivery_session_key VARCHAR(255);
ALTER TABLE automation_scheduled_tasks ADD COLUMN IF NOT EXISTS permission_mode VARCHAR(32) NOT NULL DEFAULT 'default';
ALTER TABLE automation_task_runs ADD COLUMN IF NOT EXISTS delivery_target_json TEXT;
ALTER TABLE automation_delivery_routes ADD COLUMN IF NOT EXISTS context_token TEXT;
ALTER TABLE automation_permission_requests ADD COLUMN IF NOT EXISTS delivery_session_key VARCHAR(255);

UPDATE automation_delivery_routes
SET context_token = thread_id,
    thread_id = NULL
WHERE channel = 'weixin-personal'
  AND context_token IS NULL
  AND thread_id IS NOT NULL;

-- +goose Down
UPDATE automation_delivery_routes
SET thread_id = context_token
WHERE channel = 'weixin-personal'
  AND thread_id IS NULL
  AND context_token IS NOT NULL;

ALTER TABLE automation_permission_requests DROP COLUMN IF EXISTS delivery_session_key;
ALTER TABLE automation_delivery_routes DROP COLUMN IF EXISTS context_token;
ALTER TABLE automation_task_runs DROP COLUMN IF EXISTS delivery_target_json;
ALTER TABLE automation_scheduled_tasks DROP COLUMN IF EXISTS permission_mode;
ALTER TABLE automation_scheduled_tasks DROP COLUMN IF EXISTS delivery_session_key;
