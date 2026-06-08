-- +goose Up

CREATE TABLE IF NOT EXISTS session_goals (
    goal_id VARCHAR(64) NOT NULL PRIMARY KEY,
    session_key VARCHAR(512) NOT NULL,
    objective TEXT NOT NULL,
    status VARCHAR(32) NOT NULL,
    token_budget BIGINT,
    token_used_input BIGINT NOT NULL DEFAULT 0,
    token_used_output BIGINT NOT NULL DEFAULT 0,
    token_used_cache_creation BIGINT NOT NULL DEFAULT 0,
    token_used_cache_read BIGINT NOT NULL DEFAULT 0,
    token_used_reasoning BIGINT NOT NULL DEFAULT 0,
    token_used_total BIGINT NOT NULL DEFAULT 0,
    time_used_seconds BIGINT NOT NULL DEFAULT 0,
    continuation_count INTEGER NOT NULL DEFAULT 0,
    empty_progress_count INTEGER NOT NULL DEFAULT 0,
    version BIGINT NOT NULL DEFAULT 1,
    created_by VARCHAR(128),
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    completed_at TIMESTAMP WITHOUT TIME ZONE,
    blocked_at TIMESTAMP WITHOUT TIME ZONE,
    last_error TEXT,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    CONSTRAINT ck_session_goals_status CHECK (status IN ('active', 'paused', 'complete', 'blocked', 'budget_limited', 'usage_limited'))
);

CREATE INDEX IF NOT EXISTS idx_session_goals_session_key ON session_goals (session_key);
CREATE INDEX IF NOT EXISTS idx_session_goals_status ON session_goals (status);
CREATE UNIQUE INDEX IF NOT EXISTS uq_session_goals_current ON session_goals (session_key)
    WHERE status IN ('active', 'paused', 'blocked', 'budget_limited', 'usage_limited');

CREATE TABLE IF NOT EXISTS goal_events (
    event_id VARCHAR(64) NOT NULL PRIMARY KEY,
    goal_id VARCHAR(64) NOT NULL REFERENCES session_goals(goal_id) ON DELETE CASCADE,
    session_key VARCHAR(512) NOT NULL,
    event_type VARCHAR(64) NOT NULL,
    source VARCHAR(32) NOT NULL,
    round_id VARCHAR(128),
    payload_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_goal_events_goal_id ON goal_events (goal_id, created_at);
CREATE INDEX IF NOT EXISTS idx_goal_events_session_key ON goal_events (session_key, created_at);

-- +goose Down

-- Compatibility migration only; do not drop Goal data on rollback.
SELECT 1;
