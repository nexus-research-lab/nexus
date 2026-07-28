-- +goose Up

CREATE TABLE goal_usage_source_checkpoints (
    owner_user_id VARCHAR(128) NOT NULL,
    runtime_session_key VARCHAR(512) NOT NULL,
    source_kind VARCHAR(32) NOT NULL,
    source_id VARCHAR(256) NOT NULL,
    cumulative_actual_tokens INTEGER NOT NULL DEFAULT 0
        CHECK (cumulative_actual_tokens >= 0),
    last_attributed_goal_id VARCHAR(64),
    last_attributed_round_id VARCHAR(128),
    last_observed_at DATETIME NOT NULL,
    PRIMARY KEY (
        owner_user_id,
        runtime_session_key,
        source_kind,
        source_id
    )
);

-- +goose Down

DROP TABLE IF EXISTS goal_usage_source_checkpoints;
