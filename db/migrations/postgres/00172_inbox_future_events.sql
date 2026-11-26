-- +goose Up
-- 主窗口未来事件：available_at 为空即立即可取，非空时到点才对起轮与安全点可见；cancelled 表示到点前失效。
-- Echo 改为主窗口 inbox 的 echo 事件，echo_attempts 退役（未完成的尝试直接丢弃，开关保留在 preferences）。
ALTER TABLE agent_inbox_events ADD COLUMN available_at TIMESTAMPTZ;
ALTER TABLE agent_inbox_events DROP CONSTRAINT ck_agent_inbox_events_status;
ALTER TABLE agent_inbox_events ADD CONSTRAINT ck_agent_inbox_events_status
    CHECK (status IN ('pending', 'delivering', 'consumed', 'withdrawn', 'cancelled'));
CREATE INDEX idx_agent_inbox_events_due ON agent_inbox_events (status, available_at);
DROP TABLE IF EXISTS echo_attempts;

-- +goose Down
-- cancelled 并入 withdrawn，未到点事件丢失到点时间；echo_attempts 只恢复空表。
UPDATE agent_inbox_events SET status = 'withdrawn' WHERE status = 'cancelled';
DROP INDEX IF EXISTS idx_agent_inbox_events_due;
ALTER TABLE agent_inbox_events DROP CONSTRAINT ck_agent_inbox_events_status;
ALTER TABLE agent_inbox_events ADD CONSTRAINT ck_agent_inbox_events_status
    CHECK (status IN ('pending', 'delivering', 'consumed', 'withdrawn'));
ALTER TABLE agent_inbox_events DROP COLUMN available_at;
CREATE TABLE echo_attempts (
    attempt_id VARCHAR(64) NOT NULL PRIMARY KEY,
    owner_user_id VARCHAR(64) NOT NULL,
    agent_id VARCHAR(64) NOT NULL,
    session_key VARCHAR(255) NOT NULL,
    trigger_kind VARCHAR(32) NOT NULL,
    anchor_round_id VARCHAR(64) NOT NULL,
    anchor_message_id VARCHAR(64),
    anchor_finished_at TIMESTAMPTZ NOT NULL,
    due_at TIMESTAMPTZ NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    status VARCHAR(32) NOT NULL,
    runtime_round_id VARCHAR(64),
    decision_reason TEXT,
    focus TEXT,
    error_code VARCHAR(64),
    delivered_message_id VARCHAR(64),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT now() NOT NULL,
    updated_at TIMESTAMPTZ DEFAULT now() NOT NULL,
    CONSTRAINT ck_echo_attempts_status CHECK (status IN ('scheduled', 'evaluating', 'running', 'committing', 'delivered', 'suppressed', 'cancelled', 'failed')),
    CONSTRAINT uq_echo_attempt_anchor UNIQUE (owner_user_id, session_key, trigger_kind, anchor_round_id)
);
CREATE INDEX idx_echo_attempts_due ON echo_attempts (status, due_at);
CREATE INDEX idx_echo_attempts_session ON echo_attempts (owner_user_id, session_key, created_at);
CREATE INDEX idx_echo_attempts_agent ON echo_attempts (owner_user_id, agent_id, created_at);
