-- +goose Up
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

-- +goose Down
DROP TABLE IF EXISTS echo_attempts;
