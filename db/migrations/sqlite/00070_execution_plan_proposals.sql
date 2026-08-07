-- +goose Up

-- ExecutionPlanProposal 是独立于 executions / execution_plan_revisions 的
-- 非权威 sealed document。它持有创建时已知的 trusted Goal objective fence，
-- 但先于 Execution identity materialization 存在，因此不向权威表建立外键。
CREATE TABLE execution_plan_proposals (
    proposal_id VARCHAR(64) NOT NULL PRIMARY KEY,
    owner_user_id VARCHAR(128) NOT NULL,
    session_key VARCHAR(512) NOT NULL,
    scope_kind VARCHAR(16) NOT NULL,
    room_id VARCHAR(64),
    conversation_id VARCHAR(64),
    coordinator_agent_id VARCHAR(128) NOT NULL,
    root_round_id VARCHAR(128) NOT NULL,
    runtime_round_id VARCHAR(128),
    agent_round_id VARCHAR(128),

    target_execution_id VARCHAR(64),
    target_execution_version INTEGER NOT NULL DEFAULT 0,
    base_plan_id VARCHAR(64),

    goal_id VARCHAR(64),
    goal_objective_revision INTEGER NOT NULL DEFAULT 0,
    goal_activation_origin VARCHAR(32),
    goal_activation_reason VARCHAR(32),
    goal_reserved_execution_id VARCHAR(64),
    replaces_execution_id VARCHAR(64),

    document_json TEXT NOT NULL,
    content_digest VARCHAR(64) NOT NULL,
    status VARCHAR(24) NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,

    reserved_execution_id VARCHAR(64),
    materialization_command_id VARCHAR(128),
    materialized_execution_id VARCHAR(64),
    materialized_plan_id VARCHAR(64),

    confirmation_state VARCHAR(16) NOT NULL DEFAULT 'none',
    attempt_count INTEGER NOT NULL DEFAULT 0,
    next_attempt_at DATETIME,
    last_error TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    materialized_at DATETIME,

    CONSTRAINT ck_execution_plan_proposals_scope_kind
        CHECK (scope_kind IN ('dm', 'room')),
    CONSTRAINT ck_execution_plan_proposals_scope_identity
        CHECK (
            (scope_kind = 'dm' AND room_id IS NULL AND conversation_id IS NULL)
            OR
            (scope_kind = 'room' AND room_id IS NOT NULL AND conversation_id IS NOT NULL)
        ),
    CONSTRAINT ck_execution_plan_proposals_root_round
        CHECK (length(trim(root_round_id)) > 0),
    CONSTRAINT ck_execution_plan_proposals_target_version
        CHECK (target_execution_version >= 0),
    CONSTRAINT ck_execution_plan_proposals_goal_revision
        CHECK (
            (goal_id IS NULL AND goal_objective_revision = 0
                AND goal_activation_origin IS NULL AND goal_activation_reason IS NULL
                AND goal_reserved_execution_id IS NULL)
            OR
            (goal_id IS NOT NULL AND goal_objective_revision > 0
                AND goal_activation_origin IS NOT NULL
                AND goal_activation_reason IS NOT NULL)
        ),
    CONSTRAINT ck_execution_plan_proposals_goal_activation_origin
        CHECK (
            goal_activation_origin IS NULL
            OR goal_activation_origin IN ('user_explicit', 'adaptive_initial', 'adaptive_promoted')
        ),
    CONSTRAINT ck_execution_plan_proposals_goal_activation_reason
        CHECK (
            goal_activation_reason IS NULL
            OR goal_activation_reason IN (
                'persistence_requested', 'observed_boundary', 'room_dependency_chain',
                'external_wait', 'scheduled_retry', 'context_boundary',
                'recovery_required', 'substantial_complexity'
            )
        ),
    CONSTRAINT ck_execution_plan_proposals_status
        CHECK (status IN ('sealed', 'materializing', 'materialized', 'blocked', 'discarded')),
    CONSTRAINT ck_execution_plan_proposals_document_json
        CHECK (json_valid(document_json)),
    CONSTRAINT ck_execution_plan_proposals_content_digest
        CHECK (length(content_digest) = 64),
    CONSTRAINT ck_execution_plan_proposals_confirmation
        CHECK (confirmation_state IN ('none', 'pending', 'confirmed')),
    CONSTRAINT ck_execution_plan_proposals_version
        CHECK (version > 0),
    CONSTRAINT ck_execution_plan_proposals_attempt_count
        CHECK (
            (status = 'sealed' AND attempt_count = 0)
            OR (status IN ('materializing', 'materialized') AND attempt_count >= 1)
            OR (status IN ('blocked', 'discarded') AND attempt_count >= 0)
        ),
    CONSTRAINT ck_execution_plan_proposals_materialization_reservation
        CHECK (
            (status = 'sealed'
                AND reserved_execution_id IS NULL
                AND materialization_command_id IS NULL)
            OR
            (status IN ('materializing', 'materialized')
                AND reserved_execution_id IS NOT NULL
                AND materialization_command_id IS NOT NULL)
            OR
            (status = 'blocked' AND (
                (reserved_execution_id IS NULL AND materialization_command_id IS NULL)
                OR
                (reserved_execution_id IS NOT NULL AND materialization_command_id IS NOT NULL)
            ))
            OR
            (status = 'discarded'
                AND reserved_execution_id IS NULL
                AND materialization_command_id IS NULL)
        ),
    CONSTRAINT ck_execution_plan_proposals_materialized_receipt
        CHECK (
            (status = 'materialized'
                AND materialized_execution_id IS NOT NULL
                AND materialized_execution_id = reserved_execution_id
                AND materialized_plan_id IS NOT NULL
                AND materialized_at IS NOT NULL)
            OR
            (status <> 'materialized'
                AND materialized_execution_id IS NULL
                AND materialized_plan_id IS NULL
                AND materialized_at IS NULL)
        ),
    CONSTRAINT ck_execution_plan_proposals_goal_confirmation
        CHECK (
            (status = 'sealed' AND confirmation_state = 'none')
            OR
            (status = 'materializing' AND (
                (goal_id IS NULL AND confirmation_state = 'none')
                OR (goal_id IS NOT NULL AND confirmation_state = 'pending')
            ))
            OR
            (status = 'materialized' AND (
                (goal_id IS NULL AND confirmation_state = 'none')
                OR (goal_id IS NOT NULL AND confirmation_state IN ('pending', 'confirmed'))
            ))
            OR
            (status = 'blocked' AND (
                (goal_id IS NULL AND confirmation_state = 'none')
                OR (goal_id IS NOT NULL AND confirmation_state IN ('none', 'pending'))
            ))
            OR (status = 'discarded' AND confirmation_state = 'none')
        ),
    CONSTRAINT ck_execution_plan_proposals_retry_state
        CHECK (
            (status = 'sealed' AND next_attempt_at IS NULL AND last_error IS NULL)
            OR (status = 'blocked' AND next_attempt_at IS NULL AND last_error IS NOT NULL)
            OR (status = 'discarded' AND next_attempt_at IS NULL)
            OR status IN ('materializing', 'materialized')
        ),
    CONSTRAINT ck_execution_plan_proposals_confirmed_retry_state
        CHECK (
            confirmation_state <> 'confirmed'
            OR (next_attempt_at IS NULL AND last_error IS NULL)
        )
);

CREATE INDEX idx_execution_plan_proposals_session
    ON execution_plan_proposals (owner_user_id, session_key, status, updated_at);
CREATE INDEX idx_execution_plan_proposals_recoverable
    ON execution_plan_proposals (status, confirmation_state, next_attempt_at, updated_at);
CREATE UNIQUE INDEX uq_execution_plan_proposals_materialization_command
    ON execution_plan_proposals (materialization_command_id)
    WHERE materialization_command_id IS NOT NULL;

-- +goose Down

DROP TABLE IF EXISTS execution_plan_proposals;
