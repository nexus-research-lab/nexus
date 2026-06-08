-- +goose Up
PRAGMA foreign_keys = OFF;

DROP INDEX IF EXISTS uq_session_goals_current;
DROP INDEX IF EXISTS idx_session_goals_status;
DROP INDEX IF EXISTS idx_session_goals_session_key;

CREATE TABLE session_goals_new (
    goal_id VARCHAR(64) NOT NULL PRIMARY KEY,
    session_key VARCHAR(512) NOT NULL,
    objective TEXT NOT NULL,
    status VARCHAR(32) NOT NULL,
    token_budget INTEGER,
    token_used_input INTEGER NOT NULL DEFAULT 0,
    token_used_output INTEGER NOT NULL DEFAULT 0,
    token_used_cache_creation INTEGER NOT NULL DEFAULT 0,
    token_used_cache_read INTEGER NOT NULL DEFAULT 0,
    token_used_reasoning INTEGER NOT NULL DEFAULT 0,
    token_used_total INTEGER NOT NULL DEFAULT 0,
    time_used_seconds INTEGER NOT NULL DEFAULT 0,
    continuation_count INTEGER NOT NULL DEFAULT 0,
    empty_progress_count INTEGER NOT NULL DEFAULT 0,
    version INTEGER NOT NULL DEFAULT 1,
    created_by VARCHAR(128),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    completed_at DATETIME,
    blocked_at DATETIME,
    cleared_at DATETIME,
    last_error TEXT,
    metadata_json TEXT NOT NULL DEFAULT '{}',
    CONSTRAINT ck_session_goals_status CHECK (status IN ('active', 'paused', 'complete', 'blocked', 'budget_limited', 'usage_limited', 'cleared'))
);

INSERT INTO session_goals_new (
    goal_id, session_key, objective, status, token_budget,
    token_used_input, token_used_output, token_used_cache_creation, token_used_cache_read,
    token_used_reasoning, token_used_total, time_used_seconds,
    continuation_count, empty_progress_count, version, created_by,
    created_at, updated_at, completed_at, blocked_at, cleared_at, last_error, metadata_json
)
SELECT
    goal_id, session_key, objective, status, token_budget,
    token_used_input, token_used_output, token_used_cache_creation, token_used_cache_read,
    token_used_reasoning, token_used_total, 0,
    continuation_count, empty_progress_count, version, created_by,
    created_at, updated_at, completed_at, blocked_at, cleared_at, last_error, metadata_json
FROM session_goals;

DROP TABLE session_goals;
ALTER TABLE session_goals_new RENAME TO session_goals;

CREATE INDEX idx_session_goals_session_key ON session_goals (session_key);
CREATE INDEX idx_session_goals_status ON session_goals (status);
CREATE UNIQUE INDEX uq_session_goals_current ON session_goals (session_key) WHERE status IN ('active', 'paused', 'blocked', 'budget_limited', 'usage_limited');

PRAGMA foreign_keys = ON;

-- +goose Down
PRAGMA foreign_keys = OFF;

DROP INDEX IF EXISTS uq_session_goals_current;
DROP INDEX IF EXISTS idx_session_goals_status;
DROP INDEX IF EXISTS idx_session_goals_session_key;

CREATE TABLE session_goals_new (
    goal_id VARCHAR(64) NOT NULL PRIMARY KEY,
    session_key VARCHAR(512) NOT NULL,
    objective TEXT NOT NULL,
    status VARCHAR(32) NOT NULL,
    token_budget INTEGER,
    token_used_input INTEGER NOT NULL DEFAULT 0,
    token_used_output INTEGER NOT NULL DEFAULT 0,
    token_used_cache_creation INTEGER NOT NULL DEFAULT 0,
    token_used_cache_read INTEGER NOT NULL DEFAULT 0,
    token_used_reasoning INTEGER NOT NULL DEFAULT 0,
    token_used_total INTEGER NOT NULL DEFAULT 0,
    continuation_count INTEGER NOT NULL DEFAULT 0,
    empty_progress_count INTEGER NOT NULL DEFAULT 0,
    version INTEGER NOT NULL DEFAULT 1,
    created_by VARCHAR(128),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    completed_at DATETIME,
    blocked_at DATETIME,
    cleared_at DATETIME,
    last_error TEXT,
    metadata_json TEXT NOT NULL DEFAULT '{}',
    CONSTRAINT ck_session_goals_status CHECK (status IN ('active', 'paused', 'complete', 'blocked', 'cleared'))
);

INSERT INTO session_goals_new (
    goal_id, session_key, objective, status, token_budget,
    token_used_input, token_used_output, token_used_cache_creation, token_used_cache_read,
    token_used_reasoning, token_used_total,
    continuation_count, empty_progress_count, version, created_by,
    created_at, updated_at, completed_at, blocked_at, cleared_at, last_error, metadata_json
)
SELECT
    goal_id, session_key, objective,
    CASE WHEN status IN ('budget_limited', 'usage_limited') THEN 'paused' ELSE status END,
    token_budget,
    token_used_input, token_used_output, token_used_cache_creation, token_used_cache_read,
    token_used_reasoning, token_used_total,
    continuation_count, empty_progress_count, version, created_by,
    created_at, updated_at, completed_at, blocked_at, cleared_at, last_error, metadata_json
FROM session_goals;

DROP TABLE session_goals;
ALTER TABLE session_goals_new RENAME TO session_goals;

CREATE INDEX idx_session_goals_session_key ON session_goals (session_key);
CREATE INDEX idx_session_goals_status ON session_goals (status);
CREATE UNIQUE INDEX uq_session_goals_current ON session_goals (session_key) WHERE status IN ('active', 'paused', 'blocked');

PRAGMA foreign_keys = ON;
