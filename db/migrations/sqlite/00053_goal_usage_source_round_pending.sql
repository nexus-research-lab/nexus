-- +goose Up

CREATE TABLE goal_usage_scope_bindings (
    owner_user_id VARCHAR(128) NOT NULL,
    goal_session_key VARCHAR(512) NOT NULL,
    source_kind VARCHAR(32) NOT NULL
        CHECK (source_kind = 'nxs_task'),
    scope_round_id VARCHAR(128) NOT NULL,
    state VARCHAR(16) NOT NULL
        CHECK (state IN ('open', 'bound', 'closed')),
    goal_id VARCHAR(64),
    bound_at DATETIME,
    closed_at DATETIME,
    usage_event_id VARCHAR(64),
    PRIMARY KEY (
        owner_user_id,
        goal_session_key,
        source_kind,
        scope_round_id
    ),
    CHECK (
        (state = 'open' AND goal_id IS NULL AND bound_at IS NULL AND closed_at IS NULL)
        OR (state = 'bound' AND goal_id IS NOT NULL AND bound_at IS NOT NULL AND closed_at IS NULL)
        OR (state = 'closed' AND goal_id IS NULL AND bound_at IS NOT NULL AND closed_at IS NOT NULL)
    )
);

CREATE INDEX idx_goal_usage_scope_bindings_goal
ON goal_usage_scope_bindings(goal_id)
WHERE goal_id IS NOT NULL;

CREATE TABLE goal_usage_source_pending (
    owner_user_id VARCHAR(128) NOT NULL,
    runtime_session_key VARCHAR(512) NOT NULL,
    source_kind VARCHAR(32) NOT NULL
        CHECK (source_kind = 'nxs_task'),
    source_id VARCHAR(256) NOT NULL,
    goal_session_key VARCHAR(512) NOT NULL,
    scope_round_id VARCHAR(128) NOT NULL,
    source_round_id VARCHAR(128) NOT NULL,
    pending_actual_tokens INTEGER NOT NULL
        CHECK (pending_actual_tokens > 0),
    last_observed_at DATETIME NOT NULL,
    PRIMARY KEY (
        owner_user_id,
        runtime_session_key,
        source_kind,
        source_id,
        goal_session_key,
        scope_round_id,
        source_round_id
    )
);

CREATE INDEX idx_goal_usage_source_pending_scope
ON goal_usage_source_pending(
    owner_user_id,
    goal_session_key,
    source_kind,
    scope_round_id
);

CREATE TABLE goal_usage_source_evidence (
    owner_user_id VARCHAR(128) NOT NULL,
    runtime_session_key VARCHAR(512) NOT NULL,
    source_kind VARCHAR(32) NOT NULL
        CHECK (source_kind = 'nxs_task'),
    source_id VARCHAR(256) NOT NULL,
    goal_session_key VARCHAR(512) NOT NULL,
    scope_round_id VARCHAR(128) NOT NULL,
    source_round_id VARCHAR(128) NOT NULL,
    terminal_observed BOOLEAN NOT NULL DEFAULT 0,
    token_usage_observed BOOLEAN NOT NULL DEFAULT 0,
    discarded BOOLEAN NOT NULL DEFAULT 0,
    observed_at DATETIME NOT NULL,
    PRIMARY KEY (
        owner_user_id,
        runtime_session_key,
        source_kind,
        source_id,
        goal_session_key,
        scope_round_id,
        source_round_id
    ),
    CHECK (token_usage_observed = 0 OR terminal_observed = 1)
);

CREATE INDEX idx_goal_usage_source_evidence_scope
ON goal_usage_source_evidence(
    owner_user_id,
    goal_session_key,
    source_kind,
    scope_round_id,
    discarded,
    terminal_observed,
    token_usage_observed
);

CREATE TABLE goal_usage_parent_ledger (
    owner_user_id VARCHAR(128) NOT NULL,
    goal_session_key VARCHAR(512) NOT NULL,
    scope_round_id VARCHAR(128) NOT NULL,
    source_round_id VARCHAR(128) NOT NULL,
    token_used_input INTEGER NOT NULL DEFAULT 0
        CHECK (token_used_input >= 0),
    token_used_output INTEGER NOT NULL DEFAULT 0
        CHECK (token_used_output >= 0),
    token_used_cache_creation INTEGER NOT NULL DEFAULT 0
        CHECK (token_used_cache_creation >= 0),
    token_used_cache_read INTEGER NOT NULL DEFAULT 0
        CHECK (token_used_cache_read >= 0),
    token_used_reasoning INTEGER NOT NULL DEFAULT 0
        CHECK (token_used_reasoning >= 0),
    token_used_total INTEGER NOT NULL DEFAULT 0
        CHECK (token_used_total >= 0),
    token_used_actual_total INTEGER NOT NULL DEFAULT 0
        CHECK (token_used_actual_total >= 0),
    token_used_actual_estimated BOOLEAN NOT NULL DEFAULT 0,
    runtime_seconds INTEGER NOT NULL DEFAULT 0
        CHECK (runtime_seconds >= 0),
    token_usage_observed BOOLEAN NOT NULL,
    usage_attributed BOOLEAN NOT NULL DEFAULT 0,
    discarded BOOLEAN NOT NULL DEFAULT 0,
    attributed_goal_id VARCHAR(64),
    attributed_at DATETIME,
    observed_at DATETIME NOT NULL,
    PRIMARY KEY (
        owner_user_id,
        goal_session_key,
        scope_round_id,
        source_round_id
    ),
    CHECK (
        (usage_attributed = 0 AND discarded = 0 AND attributed_goal_id IS NULL AND attributed_at IS NULL)
        OR (usage_attributed = 1 AND discarded = 0 AND attributed_goal_id IS NOT NULL AND attributed_at IS NOT NULL)
        OR (usage_attributed = 0 AND discarded = 1 AND attributed_goal_id IS NULL AND attributed_at IS NULL)
    )
);

CREATE INDEX idx_goal_usage_parent_ledger_scope
ON goal_usage_parent_ledger(
    owner_user_id,
    goal_session_key,
    scope_round_id,
    discarded,
    usage_attributed
);

CREATE INDEX idx_goal_usage_parent_ledger_goal
ON goal_usage_parent_ledger(attributed_goal_id)
WHERE attributed_goal_id IS NOT NULL;

-- +goose Down

DROP TABLE IF EXISTS goal_usage_parent_ledger;
DROP TABLE IF EXISTS goal_usage_source_evidence;
DROP TABLE IF EXISTS goal_usage_source_pending;
DROP TABLE IF EXISTS goal_usage_scope_bindings;
