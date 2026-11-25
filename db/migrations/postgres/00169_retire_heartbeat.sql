-- +goose Up
-- 周期 Heartbeat 已退役：删除配置/运行态表、wake outbox 行，以及 00125 为 wake outbox 增加的列和部分索引。
DELETE FROM automation_system_events WHERE event_type = 'heartbeat.wake';
DROP INDEX IF EXISTS idx_automation_heartbeat_wake_due;
DROP INDEX IF EXISTS uq_automation_heartbeat_wake_request;
ALTER TABLE automation_system_events DROP COLUMN claim_expires_at;
ALTER TABLE automation_system_events DROP COLUMN claim_token;
ALTER TABLE automation_system_events DROP COLUMN accepted_configuration_version;
ALTER TABLE automation_system_events DROP COLUMN intent_digest;
ALTER TABLE automation_system_events DROP COLUMN request_id;
ALTER TABLE automation_system_events DROP COLUMN owner_user_id;
DROP INDEX IF EXISTS idx_automation_heartbeat_states_enabled_agent;
DROP TABLE IF EXISTS automation_heartbeat_states;
-- 主会话任务一律在事件落库后立即唤醒；wake_mode 只保留满足 NOT NULL/CHECK 的遗留值。
UPDATE automation_scheduled_tasks SET wake_mode = 'now' WHERE wake_mode <> 'now';

-- +goose Down
-- 只恢复空结构；已删除的 heartbeat 配置与 wake 行不可恢复，next-heartbeat 也不回填。
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
ALTER TABLE automation_system_events ADD COLUMN owner_user_id VARCHAR(64);
ALTER TABLE automation_system_events ADD COLUMN request_id VARCHAR(128);
ALTER TABLE automation_system_events ADD COLUMN intent_digest VARCHAR(64);
ALTER TABLE automation_system_events ADD COLUMN accepted_configuration_version INTEGER;
ALTER TABLE automation_system_events ADD COLUMN claim_token VARCHAR(128);
ALTER TABLE automation_system_events ADD COLUMN claim_expires_at TIMESTAMPTZ;
CREATE UNIQUE INDEX uq_automation_heartbeat_wake_request
    ON automation_system_events (owner_user_id, request_id)
    WHERE event_type = 'heartbeat.wake' AND request_id IS NOT NULL;
CREATE INDEX idx_automation_heartbeat_wake_due
    ON automation_system_events (status, claim_expires_at, created_at, event_id)
    WHERE event_type = 'heartbeat.wake' AND status IN ('new', 'processing');
