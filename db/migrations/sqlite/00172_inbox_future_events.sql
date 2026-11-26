-- +goose Up
-- 主窗口未来事件：available_at 为空即立即可取，非空时到点才对起轮与安全点可见；cancelled 表示到点前失效。
-- Echo 改为主窗口 inbox 的 echo 事件，echo_attempts 退役（未完成的尝试直接丢弃，开关保留在 preferences）。
CREATE TABLE agent_inbox_events_v2 (
    event_id VARCHAR(64) NOT NULL PRIMARY KEY,
    owner_user_id VARCHAR(64) NOT NULL,
    inbox_key VARCHAR(512) NOT NULL,
    seq BIGINT NOT NULL,
    kind VARCHAR(32) NOT NULL,
    dedupe_key VARCHAR(128) NOT NULL,
    payload TEXT NOT NULL,
    status VARCHAR(16) NOT NULL,
    round_id VARCHAR(64),
    available_at DATETIME,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    CONSTRAINT uq_agent_inbox_events_dedupe UNIQUE (inbox_key, dedupe_key),
    CONSTRAINT uq_agent_inbox_events_seq UNIQUE (inbox_key, seq),
    CONSTRAINT ck_agent_inbox_events_status CHECK (status IN ('pending', 'delivering', 'consumed', 'withdrawn', 'cancelled'))
);
INSERT INTO agent_inbox_events_v2 (event_id, owner_user_id, inbox_key, seq, kind, dedupe_key, payload, status, round_id, created_at, updated_at) SELECT event_id, owner_user_id, inbox_key, seq, kind, dedupe_key, payload, status, round_id, created_at, updated_at FROM agent_inbox_events;
DROP INDEX IF EXISTS idx_agent_inbox_events_open;
DROP TABLE agent_inbox_events;
ALTER TABLE agent_inbox_events_v2 RENAME TO agent_inbox_events;
CREATE INDEX idx_agent_inbox_events_open ON agent_inbox_events (status, inbox_key, seq);
CREATE INDEX idx_agent_inbox_events_due ON agent_inbox_events (status, available_at);
DROP INDEX IF EXISTS idx_echo_attempts_due;
DROP INDEX IF EXISTS idx_echo_attempts_session;
DROP INDEX IF EXISTS idx_echo_attempts_agent;
DROP TABLE IF EXISTS echo_attempts;

-- +goose Down
-- cancelled 并入 withdrawn，未到点事件丢失到点时间；echo_attempts 只恢复空表。
CREATE TABLE agent_inbox_events_v1 (
    event_id VARCHAR(64) NOT NULL PRIMARY KEY,
    owner_user_id VARCHAR(64) NOT NULL,
    inbox_key VARCHAR(512) NOT NULL,
    seq BIGINT NOT NULL,
    kind VARCHAR(32) NOT NULL,
    dedupe_key VARCHAR(128) NOT NULL,
    payload TEXT NOT NULL,
    status VARCHAR(16) NOT NULL,
    round_id VARCHAR(64),
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    CONSTRAINT uq_agent_inbox_events_dedupe UNIQUE (inbox_key, dedupe_key),
    CONSTRAINT uq_agent_inbox_events_seq UNIQUE (inbox_key, seq),
    CONSTRAINT ck_agent_inbox_events_status CHECK (status IN ('pending', 'delivering', 'consumed', 'withdrawn'))
);
INSERT INTO agent_inbox_events_v1 (event_id, owner_user_id, inbox_key, seq, kind, dedupe_key, payload, status, round_id, created_at, updated_at) SELECT event_id, owner_user_id, inbox_key, seq, kind, dedupe_key, payload,
    CASE WHEN status = 'cancelled' THEN 'withdrawn' ELSE status END, round_id, created_at, updated_at FROM agent_inbox_events;
DROP INDEX IF EXISTS idx_agent_inbox_events_due;
DROP INDEX IF EXISTS idx_agent_inbox_events_open;
DROP TABLE agent_inbox_events;
ALTER TABLE agent_inbox_events_v1 RENAME TO agent_inbox_events;
CREATE INDEX idx_agent_inbox_events_open ON agent_inbox_events (status, inbox_key, seq);
CREATE TABLE echo_attempts (
    attempt_id VARCHAR(64) NOT NULL PRIMARY KEY,
    owner_user_id VARCHAR(64) NOT NULL,
    agent_id VARCHAR(64) NOT NULL,
    session_key VARCHAR(255) NOT NULL,
    trigger_kind VARCHAR(32) NOT NULL,
    anchor_round_id VARCHAR(64) NOT NULL,
    anchor_message_id VARCHAR(64),
    anchor_finished_at DATETIME NOT NULL,
    due_at DATETIME NOT NULL,
    expires_at DATETIME NOT NULL,
    status VARCHAR(32) NOT NULL,
    runtime_round_id VARCHAR(64),
    decision_reason TEXT,
    focus TEXT,
    error_code VARCHAR(64),
    delivered_message_id VARCHAR(64),
    started_at DATETIME,
    finished_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_echo_attempts_status CHECK (status IN ('scheduled', 'evaluating', 'running', 'committing', 'delivered', 'suppressed', 'cancelled', 'failed')),
    CONSTRAINT uq_echo_attempt_anchor UNIQUE (owner_user_id, session_key, trigger_kind, anchor_round_id)
);
CREATE INDEX idx_echo_attempts_due ON echo_attempts (status, due_at);
CREATE INDEX idx_echo_attempts_session ON echo_attempts (owner_user_id, session_key, created_at);
CREATE INDEX idx_echo_attempts_agent ON echo_attempts (owner_user_id, agent_id, created_at);
