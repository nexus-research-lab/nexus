-- +goose Up
-- 定时任务 main 目标改为写入主窗口 inbox：主会话事件表退役；遗留的排队 run 收口为 cancelled，任务释放运行占用。
UPDATE automation_scheduled_tasks SET running_run_id = NULL, running_started_at = NULL
WHERE running_run_id IN (SELECT run_id FROM automation_task_runs WHERE status = 'queued_to_main_session');
UPDATE automation_task_runs
SET status = 'cancelled',
    finished_at = COALESCE(finished_at, CURRENT_TIMESTAMP),
    error_message = COALESCE(error_message, 'main session retired: scheduled tasks now run in the main window')
WHERE status = 'queued_to_main_session';
DROP INDEX IF EXISTS idx_automation_system_events_status_created;
DROP INDEX IF EXISTS idx_automation_system_events_created;
DROP INDEX IF EXISTS idx_automation_system_events_status;
DROP INDEX IF EXISTS idx_automation_system_events_type;
DROP TABLE IF EXISTS automation_system_events;

-- +goose Down
-- 只恢复空表结构；已删除的事件行与被收口的 run 不可恢复。
CREATE TABLE automation_system_events (
    event_id VARCHAR(64) NOT NULL PRIMARY KEY,
    event_type VARCHAR(64) NOT NULL,
    source_type VARCHAR(64),
    source_id VARCHAR(64),
    payload JSON NOT NULL,
    status VARCHAR(32) NOT NULL,
    processed_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_automation_system_events_status CHECK (status IN ('new', 'processing', 'processed', 'failed'))
);
CREATE INDEX idx_automation_system_events_type ON automation_system_events (event_type);
CREATE INDEX idx_automation_system_events_status ON automation_system_events (status);
CREATE INDEX idx_automation_system_events_created ON automation_system_events (created_at);
CREATE INDEX IF NOT EXISTS idx_automation_system_events_status_created ON automation_system_events (status, created_at ASC, event_id ASC);
