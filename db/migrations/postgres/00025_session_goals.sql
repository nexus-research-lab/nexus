-- +goose Up

CREATE TABLE session_goals (
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
    continuation_count INTEGER NOT NULL DEFAULT 0,
    empty_progress_count INTEGER NOT NULL DEFAULT 0,
    version BIGINT NOT NULL DEFAULT 1,
    created_by VARCHAR(128),
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    completed_at TIMESTAMP WITHOUT TIME ZONE,
    blocked_at TIMESTAMP WITHOUT TIME ZONE,
    cleared_at TIMESTAMP WITHOUT TIME ZONE,
    last_error TEXT,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    CONSTRAINT ck_session_goals_status CHECK (status IN ('active', 'paused', 'complete', 'blocked', 'cleared'))
);
CREATE INDEX idx_session_goals_session_key ON session_goals (session_key);
CREATE INDEX idx_session_goals_status ON session_goals (status);
CREATE UNIQUE INDEX uq_session_goals_current ON session_goals (session_key) WHERE status IN ('active', 'paused', 'blocked');

CREATE TABLE goal_events (
    event_id VARCHAR(64) NOT NULL PRIMARY KEY,
    goal_id VARCHAR(64) NOT NULL REFERENCES session_goals(goal_id) ON DELETE CASCADE,
    session_key VARCHAR(512) NOT NULL,
    event_type VARCHAR(64) NOT NULL,
    source VARCHAR(32) NOT NULL,
    round_id VARCHAR(128),
    payload_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL
);
CREATE INDEX idx_goal_events_goal_id ON goal_events (goal_id, created_at);
CREATE INDEX idx_goal_events_session_key ON goal_events (session_key, created_at);

CREATE TABLE goal_checkpoints (
    checkpoint_id VARCHAR(64) NOT NULL PRIMARY KEY,
    goal_id VARCHAR(64) NOT NULL REFERENCES session_goals(goal_id) ON DELETE CASCADE,
    session_key VARCHAR(512) NOT NULL,
    summary TEXT NOT NULL,
    continuation_count INTEGER NOT NULL DEFAULT 0,
    usage_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL
);
CREATE INDEX idx_goal_checkpoints_goal_id ON goal_checkpoints (goal_id, created_at);

-- +goose Down

DROP TABLE IF EXISTS goal_checkpoints;
DROP TABLE IF EXISTS goal_events;
DROP INDEX IF EXISTS uq_session_goals_current;
DROP INDEX IF EXISTS idx_session_goals_status;
DROP INDEX IF EXISTS idx_session_goals_session_key;
DROP TABLE IF EXISTS session_goals;
