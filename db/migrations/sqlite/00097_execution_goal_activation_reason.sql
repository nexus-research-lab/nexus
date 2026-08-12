-- +goose Up
PRAGMA foreign_keys = OFF;
PRAGMA legacy_alter_table = ON;

DROP INDEX IF EXISTS uq_executions_current_goal_revision;
DROP INDEX IF EXISTS uq_executions_trigger_message;
DROP INDEX IF EXISTS idx_executions_replaces;
DROP INDEX IF EXISTS idx_executions_root_round;
DROP INDEX IF EXISTS idx_executions_goal;
DROP INDEX IF EXISTS uq_executions_current_session;
DROP INDEX IF EXISTS idx_executions_session;

ALTER TABLE executions RENAME TO executions_before_activation_reason_expansion;

CREATE TABLE executions (
    execution_id VARCHAR(64) NOT NULL PRIMARY KEY,
    owner_user_id VARCHAR(128) NOT NULL,
    session_key VARCHAR(512) NOT NULL,
    scope_kind VARCHAR(16) NOT NULL,
    room_id VARCHAR(64),
    conversation_id VARCHAR(64),
    coordinator_agent_id VARCHAR(128),
    origin VARCHAR(32) NOT NULL,
    objective TEXT NOT NULL,
    completion_criteria_json TEXT NOT NULL DEFAULT '[]',
    goal_id VARCHAR(64),
    goal_objective_revision INTEGER NOT NULL DEFAULT 0,
    goal_activation_origin VARCHAR(32),
    goal_activation_reason VARCHAR(32),
    recovery_of_execution_id VARCHAR(64),
    replaces_execution_id VARCHAR(64),
    root_round_id VARCHAR(128),
    trigger_message_id VARCHAR(128),
    status VARCHAR(32) NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    completed_at DATETIME,
    metadata_json TEXT NOT NULL DEFAULT '{}',
    CONSTRAINT ck_executions_scope_kind
        CHECK (scope_kind IN ('dm', 'room')),
    CONSTRAINT ck_executions_scope_identity
        CHECK (
            (scope_kind = 'dm' AND room_id IS NULL AND conversation_id IS NULL)
            OR
            (scope_kind = 'room' AND room_id IS NOT NULL AND conversation_id IS NOT NULL)
        ),
    CONSTRAINT ck_executions_origin
        CHECK (origin IN ('user_request', 'goal_continuation', 'recovery', 'system')),
    CONSTRAINT ck_executions_goal_activation_origin
        CHECK (
            goal_activation_origin IS NULL
            OR goal_activation_origin IN ('user_explicit', 'adaptive_initial', 'adaptive_promoted')
        ),
    CONSTRAINT ck_executions_goal_activation_reason
        CHECK (
            goal_activation_reason IS NULL
            OR goal_activation_reason IN (
                'persistence_requested', 'observed_boundary', 'room_dependency_chain',
                'external_wait', 'scheduled_retry', 'context_boundary', 'recovery_required',
                'substantial_complexity'
            )
        ),
    CONSTRAINT ck_executions_goal_binding
        CHECK (
            (
                goal_id IS NULL
                AND goal_objective_revision = 0
                AND goal_activation_origin IS NULL
                AND goal_activation_reason IS NULL
            )
            OR
            (
                goal_id IS NOT NULL
                AND goal_objective_revision > 0
                AND goal_activation_origin IS NOT NULL
                AND goal_activation_reason IS NOT NULL
            )
        ),
    CONSTRAINT ck_executions_status
        CHECK (status IN ('active', 'waiting', 'paused', 'completed', 'failed', 'cancelled', 'superseded')),
    CONSTRAINT ck_executions_version
        CHECK (version > 0),
    FOREIGN KEY(goal_id) REFERENCES session_goals(goal_id) ON DELETE CASCADE,
    FOREIGN KEY(recovery_of_execution_id) REFERENCES executions(execution_id) ON DELETE SET NULL,
    FOREIGN KEY(replaces_execution_id) REFERENCES executions(execution_id) ON DELETE SET NULL,
    CONSTRAINT ck_executions_not_replace_self
        CHECK (replaces_execution_id IS NULL OR replaces_execution_id <> execution_id)
);

INSERT INTO executions (
    execution_id, owner_user_id, session_key, scope_kind, room_id, conversation_id,
    coordinator_agent_id, origin, objective, completion_criteria_json, goal_id,
    goal_objective_revision, goal_activation_origin, goal_activation_reason,
    recovery_of_execution_id, replaces_execution_id, root_round_id,
    trigger_message_id, status, version, created_at, updated_at, completed_at,
    metadata_json
)
SELECT
    execution_id, owner_user_id, session_key, scope_kind, room_id, conversation_id,
    coordinator_agent_id, origin, objective, completion_criteria_json, goal_id,
    goal_objective_revision, goal_activation_origin, goal_activation_reason,
    recovery_of_execution_id, replaces_execution_id, root_round_id,
    trigger_message_id, status, version, created_at, updated_at, completed_at,
    metadata_json
FROM executions_before_activation_reason_expansion;

DROP TABLE executions_before_activation_reason_expansion;

CREATE INDEX idx_executions_session
    ON executions (owner_user_id, session_key, status, updated_at);
CREATE UNIQUE INDEX uq_executions_current_session
    ON executions (owner_user_id, session_key)
    WHERE status IN ('active', 'waiting', 'paused');
CREATE INDEX idx_executions_goal
    ON executions (goal_id, goal_objective_revision, status);
CREATE INDEX idx_executions_root_round
    ON executions (owner_user_id, root_round_id);
CREATE INDEX idx_executions_replaces
    ON executions (replaces_execution_id);
CREATE UNIQUE INDEX uq_executions_trigger_message
    ON executions (owner_user_id, session_key, trigger_message_id)
    WHERE trigger_message_id IS NOT NULL;
CREATE UNIQUE INDEX uq_executions_current_goal_revision
    ON executions (goal_id, goal_objective_revision)
    WHERE goal_id IS NOT NULL AND status IN ('active', 'waiting', 'paused');

PRAGMA foreign_keys = ON;
PRAGMA legacy_alter_table = OFF;

-- +goose Down
-- The new enum value may already be durable data, so narrowing this CHECK is unsafe.
SELECT 1;
