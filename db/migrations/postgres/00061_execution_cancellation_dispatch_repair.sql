-- +goose Up

-- 00060 was applied to some local databases before the cancellation outbox
-- table was present in the migration file. Keep this repair idempotent so it
-- is harmless for databases where 00060 already created the table.
CREATE TABLE IF NOT EXISTS execution_cancellation_dispatches (
    cancellation_dispatch_id VARCHAR(64) NOT NULL PRIMARY KEY,
    execution_id VARCHAR(64) NOT NULL,
    plan_id VARCHAR(64) NOT NULL,
    work_item_id VARCHAR(64) NOT NULL,
    spec_id VARCHAR(64) NOT NULL,
    assignment_id VARCHAR(64) NOT NULL,
    attempt_id VARCHAR(64) NOT NULL,
    runtime_attempt_id VARCHAR(64) NOT NULL,
    dispatch_id VARCHAR(64),
    command_id VARCHAR(128) NOT NULL,
    dedupe_key VARCHAR(256) NOT NULL,
    scope_kind VARCHAR(16) NOT NULL,
    scope_session_key VARCHAR(512) NOT NULL,
    room_id VARCHAR(64),
    conversation_id VARCHAR(64),
    executor_kind VARCHAR(16) NOT NULL,
    target_kind VARCHAR(32) NOT NULL,
    target_agent_id VARCHAR(128),
    runtime_session_key VARCHAR(512),
    room_session_id VARCHAR(128),
    sdk_session_id VARCHAR(128),
    runtime_round_id VARCHAR(128),
    root_round_id VARCHAR(128),
    agent_round_id VARCHAR(128),
    child_session_id VARCHAR(128),
    sdk_task_id VARCHAR(128),
    tool_use_id VARCHAR(128),
    status VARCHAR(32) NOT NULL,
    reason TEXT NOT NULL,
    limitation_code VARCHAR(64),
    outcome VARCHAR(32),
    receipt TEXT,
    delivery_attempts INTEGER NOT NULL DEFAULT 0,
    version BIGINT NOT NULL DEFAULT 1,
    available_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    lease_owner VARCHAR(128),
    lease_expires_at TIMESTAMP WITHOUT TIME ZONE,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    claimed_at TIMESTAMP WITHOUT TIME ZONE,
    delivered_at TIMESTAMP WITHOUT TIME ZONE,
    last_error TEXT,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    CONSTRAINT ck_execution_cancellation_dispatches_scope
        CHECK (scope_kind IN ('dm', 'room')),
    CONSTRAINT ck_execution_cancellation_dispatches_executor
        CHECK (executor_kind IN ('agent', 'subagent')),
    CONSTRAINT ck_execution_cancellation_dispatches_target
        CHECK (target_kind IN (
            'room_slot', 'runtime_round', 'not_started', 'unavailable'
        )),
    CONSTRAINT ck_execution_cancellation_dispatches_status
        CHECK (status IN (
            'pending', 'claimed', 'delivered', 'not_required',
            'unsupported', 'cancelled', 'failed'
        )),
    CONSTRAINT ck_execution_cancellation_dispatches_outcome
        CHECK (
            outcome IS NULL OR outcome IN (
                'provider_interrupted', 'local_round_cancelled',
                'already_ended', 'stale_target', 'not_started', 'unsupported'
            )
        ),
    CONSTRAINT ck_execution_cancellation_dispatches_attempts
        CHECK (delivery_attempts >= 0),
    CONSTRAINT ck_execution_cancellation_dispatches_version
        CHECK (version > 0),
    CONSTRAINT uq_execution_cancellation_dispatches_attempt
        UNIQUE (attempt_id),
    CONSTRAINT uq_execution_cancellation_dispatches_dedupe
        UNIQUE (execution_id, dedupe_key),
    FOREIGN KEY(attempt_id, assignment_id, execution_id, plan_id, work_item_id, spec_id)
        REFERENCES execution_attempts(
            attempt_id, assignment_id, execution_id, plan_id, work_item_id, spec_id
        ) ON DELETE CASCADE,
    FOREIGN KEY(runtime_attempt_id, assignment_id)
        REFERENCES execution_attempts(attempt_id, assignment_id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_execution_cancellation_dispatches_due
    ON execution_cancellation_dispatches (
        status, available_at, lease_expires_at, created_at
    );
CREATE INDEX IF NOT EXISTS idx_execution_cancellation_dispatches_execution
    ON execution_cancellation_dispatches (execution_id, created_at);

-- +goose Down

DROP TABLE IF EXISTS execution_cancellation_dispatches;
