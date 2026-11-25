-- +goose Up
-- 常驻对话 inbox：主窗口用户输入先落库再唤醒 runner。
-- status 只允许条件更新推进：pending → delivering（安全点已交给 runtime）→ consumed；撤回只能 pending → withdrawn。
CREATE TABLE agent_inbox_events (
    event_id VARCHAR(64) NOT NULL PRIMARY KEY,
    owner_user_id VARCHAR(64) NOT NULL,
    inbox_key VARCHAR(512) NOT NULL,
    seq BIGINT NOT NULL,
    kind VARCHAR(32) NOT NULL,
    dedupe_key VARCHAR(128) NOT NULL,
    payload TEXT NOT NULL,
    status VARCHAR(16) NOT NULL,
    round_id VARCHAR(64),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT uq_agent_inbox_events_dedupe UNIQUE (inbox_key, dedupe_key),
    CONSTRAINT uq_agent_inbox_events_seq UNIQUE (inbox_key, seq),
    CONSTRAINT ck_agent_inbox_events_status CHECK (status IN ('pending', 'delivering', 'consumed', 'withdrawn'))
);
CREATE INDEX idx_agent_inbox_events_open ON agent_inbox_events (status, inbox_key, seq);

-- +goose Down
DROP INDEX IF EXISTS idx_agent_inbox_events_open;
DROP TABLE IF EXISTS agent_inbox_events;
