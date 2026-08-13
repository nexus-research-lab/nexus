-- +goose Up
-- Session 是任务的可替换依赖：删除 Session 时保留任务定义，但持久标记失效绑定，
-- 使任务在重启后仍保持暂停，直到执行/投递目标都被重新分配。
ALTER TABLE automation_scheduled_tasks
    ADD COLUMN session_binding_state TEXT NOT NULL DEFAULT 'ready';
ALTER TABLE automation_scheduled_tasks
    ADD COLUMN invalidated_session_keys_json TEXT NOT NULL DEFAULT '[]';

CREATE INDEX idx_automation_scheduled_tasks_session_binding_state
    ON automation_scheduled_tasks(owner_user_id, session_binding_state);

-- +goose Down
DROP INDEX IF EXISTS idx_automation_scheduled_tasks_session_binding_state;
ALTER TABLE automation_scheduled_tasks DROP COLUMN invalidated_session_keys_json;
ALTER TABLE automation_scheduled_tasks DROP COLUMN session_binding_state;
