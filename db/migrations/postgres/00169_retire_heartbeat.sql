-- +goose Up
-- 周期 Heartbeat 已退役：删除配置/运行态表与 wake outbox 行。
-- 00125 新增的列和部分索引保留：遗留 migration ledger 修复会探测 00122-00126 结构的连续性，删除它们会让启动 fail-closed。
DELETE FROM automation_system_events WHERE event_type = 'heartbeat.wake';
DROP INDEX IF EXISTS idx_automation_heartbeat_states_enabled_agent;
DROP TABLE IF EXISTS automation_heartbeat_states;
-- 主会话任务一律在事件落库后立即唤醒；wake_mode 只保留满足 NOT NULL/CHECK 的遗留值。
UPDATE automation_scheduled_tasks SET wake_mode = 'now' WHERE wake_mode <> 'now';

-- +goose Down
-- 只恢复空表结构；已删除的 heartbeat 配置与 wake 行不可恢复，next-heartbeat 也不回填。
CREATE TABLE automation_heartbeat_states (
    state_id VARCHAR(64) NOT NULL PRIMARY KEY,
    agent_id VARCHAR(64) NOT NULL,
    enabled BOOLEAN NOT NULL,
    every_seconds INTEGER NOT NULL,
    target_mode VARCHAR(32) NOT NULL,
    ack_max_chars INTEGER NOT NULL,
    last_heartbeat_at TIMESTAMP WITHOUT TIME ZONE,
    last_ack_at TIMESTAMP WITHOUT TIME ZONE,
    configuration_version INTEGER NOT NULL DEFAULT 1 CHECK (configuration_version >= 1),
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    CONSTRAINT ck_automation_heartbeat_states_target_mode CHECK (target_mode IN ('none', 'last', 'explicit')),
    CONSTRAINT uq_automation_heartbeat_states_agent UNIQUE (agent_id),
    FOREIGN KEY(agent_id) REFERENCES agents (id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_automation_heartbeat_states_enabled_agent ON automation_heartbeat_states (enabled, agent_id);
