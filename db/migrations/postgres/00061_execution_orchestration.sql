-- +goose Up

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
    completion_criteria_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    goal_id VARCHAR(64),
    goal_objective_revision BIGINT NOT NULL DEFAULT 0,
    goal_activation_origin VARCHAR(32),
    goal_activation_reason VARCHAR(32),
    recovery_of_execution_id VARCHAR(64),
    replaces_execution_id VARCHAR(64),
    root_round_id VARCHAR(128),
    trigger_message_id VARCHAR(128),
    status VARCHAR(32) NOT NULL,
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    completed_at TIMESTAMP WITHOUT TIME ZONE,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
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
                'external_wait', 'scheduled_retry', 'context_boundary', 'recovery_required'
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

CREATE TABLE goal_execution_identity_claims (
    execution_id VARCHAR(64) NOT NULL PRIMARY KEY,
    goal_id VARCHAR(64) NOT NULL,
    goal_objective_revision BIGINT NOT NULL,
    owner_user_id VARCHAR(128),
    claim_state VARCHAR(16) NOT NULL,
    command_id VARCHAR(128) NOT NULL,
    successor_execution_id VARCHAR(64),
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    CONSTRAINT ck_goal_execution_identity_claims_revision
        CHECK (goal_objective_revision > 0),
    CONSTRAINT ck_goal_execution_identity_claims_state
        CHECK (claim_state IN ('materialized', 'fenced')),
    CONSTRAINT ck_goal_execution_identity_claims_successor
        CHECK (
            (claim_state = 'materialized' AND successor_execution_id IS NULL)
            OR
            (claim_state = 'fenced' AND successor_execution_id IS NOT NULL)
        ),
    CONSTRAINT uq_goal_execution_identity_claims_revision
        UNIQUE (goal_id, goal_objective_revision)
);

CREATE TABLE execution_plan_revisions (
    plan_id VARCHAR(64) NOT NULL PRIMARY KEY,
    execution_id VARCHAR(64) NOT NULL,
    revision BIGINT NOT NULL,
    status VARCHAR(32) NOT NULL,
    base_plan_id VARCHAR(64),
    created_by_agent_id VARCHAR(128),
    revision_reason TEXT,
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    activated_at TIMESTAMP WITHOUT TIME ZONE,
    superseded_at TIMESTAMP WITHOUT TIME ZONE,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    CONSTRAINT ck_execution_plan_revisions_revision
        CHECK (revision > 0),
    CONSTRAINT ck_execution_plan_revisions_version
        CHECK (version > 0),
    CONSTRAINT ck_execution_plan_revisions_status
        CHECK (status IN ('proposed', 'active', 'superseded', 'cancelled')),
    CONSTRAINT uq_execution_plan_revisions_revision
        UNIQUE (execution_id, revision),
    CONSTRAINT uq_execution_plan_revisions_chain
        UNIQUE (plan_id, execution_id),
    FOREIGN KEY(execution_id) REFERENCES executions(execution_id) ON DELETE CASCADE,
    FOREIGN KEY(base_plan_id, execution_id)
        REFERENCES execution_plan_revisions(plan_id, execution_id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX uq_execution_plan_revisions_active
    ON execution_plan_revisions (execution_id)
    WHERE status = 'active';

CREATE TABLE execution_work_items (
    work_item_id VARCHAR(64) NOT NULL PRIMARY KEY,
    execution_id VARCHAR(64) NOT NULL,
    logical_key VARCHAR(256) NOT NULL,
    kind VARCHAR(16) NOT NULL,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    CONSTRAINT ck_execution_work_items_kind
        CHECK (kind IN ('produce', 'review', 'verify', 'integrate')),
    CONSTRAINT uq_execution_work_items_logical_key
        UNIQUE (execution_id, logical_key),
    CONSTRAINT uq_execution_work_items_chain
        UNIQUE (work_item_id, execution_id),
    FOREIGN KEY(execution_id) REFERENCES executions(execution_id) ON DELETE CASCADE
);

CREATE TABLE execution_work_item_specs (
    spec_id VARCHAR(64) NOT NULL PRIMARY KEY,
    work_item_id VARCHAR(64) NOT NULL,
    execution_id VARCHAR(64) NOT NULL,
    spec_version BIGINT NOT NULL,
    subject VARCHAR(512) NOT NULL,
    objective TEXT NOT NULL,
    deliverable TEXT NOT NULL,
    acceptance_criteria_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    input_refs_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    spec_hash VARCHAR(128) NOT NULL,
    created_by_agent_id VARCHAR(128),
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    CONSTRAINT ck_execution_work_item_specs_version
        CHECK (spec_version > 0),
    CONSTRAINT uq_execution_work_item_specs_version
        UNIQUE (work_item_id, spec_version),
    CONSTRAINT uq_execution_work_item_specs_chain
        UNIQUE (spec_id, work_item_id, execution_id),
    FOREIGN KEY(work_item_id, execution_id)
        REFERENCES execution_work_items(work_item_id, execution_id) ON DELETE CASCADE
);

CREATE TABLE execution_plan_items (
    plan_id VARCHAR(64) NOT NULL,
    execution_id VARCHAR(64) NOT NULL,
    work_item_id VARCHAR(64) NOT NULL,
    spec_id VARCHAR(64) NOT NULL,
    parent_work_item_id VARCHAR(64),
    is_required BOOLEAN NOT NULL DEFAULT TRUE,
    is_terminal BOOLEAN NOT NULL DEFAULT FALSE,
    position INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    PRIMARY KEY (plan_id, work_item_id),
    CONSTRAINT ck_execution_plan_items_not_parent_self
        CHECK (parent_work_item_id IS NULL OR parent_work_item_id <> work_item_id),
    CONSTRAINT ck_execution_plan_items_position
        CHECK (position >= 0),
    CONSTRAINT uq_execution_plan_items_chain
        UNIQUE (plan_id, execution_id, work_item_id),
    CONSTRAINT uq_execution_plan_items_spec_chain
        UNIQUE (plan_id, execution_id, work_item_id, spec_id),
    FOREIGN KEY(plan_id, execution_id)
        REFERENCES execution_plan_revisions(plan_id, execution_id) ON DELETE CASCADE,
    FOREIGN KEY(spec_id, work_item_id, execution_id)
        REFERENCES execution_work_item_specs(spec_id, work_item_id, execution_id) ON DELETE CASCADE,
    FOREIGN KEY(plan_id, execution_id, parent_work_item_id)
        REFERENCES execution_plan_items(plan_id, execution_id, work_item_id) ON DELETE CASCADE
);
CREATE INDEX idx_execution_plan_items_work_item
    ON execution_plan_items (work_item_id, plan_id);

CREATE TABLE execution_plan_dependencies (
    plan_id VARCHAR(64) NOT NULL,
    execution_id VARCHAR(64) NOT NULL,
    work_item_id VARCHAR(64) NOT NULL,
    depends_on_work_item_id VARCHAR(64) NOT NULL,
    dependency_kind VARCHAR(16) NOT NULL DEFAULT 'hard',
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    PRIMARY KEY (plan_id, work_item_id, depends_on_work_item_id),
    CONSTRAINT ck_execution_plan_dependencies_not_self
        CHECK (work_item_id <> depends_on_work_item_id),
    CONSTRAINT ck_execution_plan_dependencies_kind
        CHECK (dependency_kind IN ('hard', 'soft')),
    FOREIGN KEY(plan_id, execution_id, work_item_id)
        REFERENCES execution_plan_items(plan_id, execution_id, work_item_id) ON DELETE CASCADE,
    FOREIGN KEY(plan_id, execution_id, depends_on_work_item_id)
        REFERENCES execution_plan_items(plan_id, execution_id, work_item_id) ON DELETE CASCADE
);
CREATE INDEX idx_execution_plan_dependencies_upstream
    ON execution_plan_dependencies (plan_id, depends_on_work_item_id, work_item_id);

CREATE TABLE execution_plan_output_claims (
    plan_id VARCHAR(64) NOT NULL,
    execution_id VARCHAR(64) NOT NULL,
    work_item_id VARCHAR(64) NOT NULL,
    spec_id VARCHAR(64) NOT NULL,
    claim_key VARCHAR(512) NOT NULL,
    mode VARCHAR(16) NOT NULL,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    PRIMARY KEY (plan_id, work_item_id, claim_key),
    CONSTRAINT ck_execution_plan_output_claims_mode
        CHECK (mode IN ('exclusive', 'shared')),
    FOREIGN KEY(plan_id, execution_id, work_item_id, spec_id)
        REFERENCES execution_plan_items(plan_id, execution_id, work_item_id, spec_id) ON DELETE CASCADE
);
CREATE INDEX idx_execution_plan_output_claims_key
    ON execution_plan_output_claims (plan_id, claim_key, mode);
CREATE UNIQUE INDEX uq_execution_plan_exclusive_output_claim
    ON execution_plan_output_claims (plan_id, claim_key)
    WHERE mode = 'exclusive';

CREATE TABLE execution_work_item_states (
    work_item_id VARCHAR(64) NOT NULL PRIMARY KEY,
    execution_id VARCHAR(64) NOT NULL,
    current_spec_id VARCHAR(64) NOT NULL,
    status VARCHAR(32) NOT NULL,
    block_reason TEXT,
    needed_input TEXT,
    version BIGINT NOT NULL DEFAULT 1,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    CONSTRAINT ck_execution_work_item_states_status
        CHECK (status IN ('open', 'waiting_input', 'cancelled', 'superseded')),
    CONSTRAINT ck_execution_work_item_states_version
        CHECK (version > 0),
    CONSTRAINT uq_execution_work_item_states_chain
        UNIQUE (work_item_id, execution_id, current_spec_id),
    FOREIGN KEY(current_spec_id, work_item_id, execution_id)
        REFERENCES execution_work_item_specs(spec_id, work_item_id, execution_id) ON DELETE CASCADE
);
CREATE INDEX idx_execution_work_item_states_execution
    ON execution_work_item_states (execution_id, status, updated_at);

CREATE TABLE execution_work_assignments (
    assignment_id VARCHAR(64) NOT NULL PRIMARY KEY,
    execution_id VARCHAR(64) NOT NULL,
    plan_id VARCHAR(64) NOT NULL,
    work_item_id VARCHAR(64) NOT NULL,
    spec_id VARCHAR(64) NOT NULL,
    owner_agent_id VARCHAR(128) NOT NULL,
    assigned_by_agent_id VARCHAR(128),
    return_to_agent_id VARCHAR(128) NOT NULL,
    strategy VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL,
    assignment_reason TEXT,
    takeover_reason TEXT,
    version BIGINT NOT NULL DEFAULT 1,
    assigned_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    activated_at TIMESTAMP WITHOUT TIME ZONE,
    released_at TIMESTAMP WITHOUT TIME ZONE,
    completed_at TIMESTAMP WITHOUT TIME ZONE,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    CONSTRAINT ck_execution_work_assignments_strategy
        CHECK (strategy IN ('self', 'room_member')),
    CONSTRAINT ck_execution_work_assignments_status
        CHECK (status IN ('assigned', 'active', 'released', 'completed', 'cancelled', 'revoked')),
    CONSTRAINT ck_execution_work_assignments_version
        CHECK (version > 0),
    CONSTRAINT uq_execution_work_assignments_chain
        UNIQUE (assignment_id, execution_id, plan_id, work_item_id, spec_id),
    CONSTRAINT uq_execution_work_assignments_owner_chain
        UNIQUE (assignment_id, execution_id, owner_agent_id),
    CONSTRAINT uq_execution_work_assignments_return_chain
        UNIQUE (assignment_id, execution_id, return_to_agent_id),
    FOREIGN KEY(plan_id, execution_id, work_item_id, spec_id)
        REFERENCES execution_plan_items(plan_id, execution_id, work_item_id, spec_id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX uq_execution_work_assignments_current
    ON execution_work_assignments (work_item_id)
    WHERE status IN ('assigned', 'active');
CREATE INDEX idx_execution_work_assignments_owner
    ON execution_work_assignments (owner_agent_id, status, assigned_at);

CREATE TABLE execution_dispatches (
    dispatch_id VARCHAR(64) NOT NULL PRIMARY KEY,
    execution_id VARCHAR(64) NOT NULL,
    plan_id VARCHAR(64) NOT NULL,
    work_item_id VARCHAR(64) NOT NULL,
    spec_id VARCHAR(64) NOT NULL,
    assignment_id VARCHAR(64) NOT NULL,
    command_id VARCHAR(128) NOT NULL,
    dedupe_key VARCHAR(256) NOT NULL,
    target_agent_id VARCHAR(128) NOT NULL,
    kind VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL,
    instruction TEXT NOT NULL,
    handoff_id VARCHAR(128),
    queue_item_id VARCHAR(128),
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
    CONSTRAINT ck_execution_dispatches_kind
        CHECK (kind IN ('room_public', 'room_directed', 'subagent')),
    CONSTRAINT ck_execution_dispatches_status
        CHECK (status IN ('pending', 'claimed', 'delivered', 'cancelled', 'failed')),
    CONSTRAINT ck_execution_dispatches_attempts
        CHECK (delivery_attempts >= 0),
    CONSTRAINT ck_execution_dispatches_version
        CHECK (version > 0),
    CONSTRAINT uq_execution_dispatches_dedupe
        UNIQUE (execution_id, dedupe_key),
    CONSTRAINT uq_execution_dispatches_chain
        UNIQUE (dispatch_id, assignment_id, execution_id),
    CONSTRAINT uq_execution_dispatches_full_chain
        UNIQUE (dispatch_id, assignment_id, execution_id, plan_id, work_item_id, spec_id),
    FOREIGN KEY(assignment_id, execution_id, plan_id, work_item_id, spec_id)
        REFERENCES execution_work_assignments(
            assignment_id, execution_id, plan_id, work_item_id, spec_id
        ) ON DELETE CASCADE,
    FOREIGN KEY(assignment_id, execution_id, target_agent_id)
        REFERENCES execution_work_assignments(
            assignment_id, execution_id, owner_agent_id
        ) ON DELETE CASCADE
);
CREATE UNIQUE INDEX uq_execution_dispatches_current
    ON execution_dispatches (assignment_id)
    WHERE status IN ('pending', 'claimed');
CREATE INDEX idx_execution_dispatches_due
    ON execution_dispatches (status, available_at, lease_expires_at, created_at);

CREATE TABLE execution_attempts (
    attempt_id VARCHAR(64) NOT NULL PRIMARY KEY,
    execution_id VARCHAR(64) NOT NULL,
    plan_id VARCHAR(64) NOT NULL,
    work_item_id VARCHAR(64) NOT NULL,
    spec_id VARCHAR(64) NOT NULL,
    assignment_id VARCHAR(64) NOT NULL,
    dispatch_id VARCHAR(64),
    parent_attempt_id VARCHAR(64),
    executor_kind VARCHAR(16) NOT NULL,
    executor_agent_id VARCHAR(128),
    parent_agent_id VARCHAR(128),
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
    failure_reason TEXT,
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    started_at TIMESTAMP WITHOUT TIME ZONE,
    finished_at TIMESTAMP WITHOUT TIME ZONE,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    CONSTRAINT ck_execution_attempts_executor_kind
        CHECK (executor_kind IN ('agent', 'subagent')),
    CONSTRAINT ck_execution_attempts_subagent_identity
        CHECK (
            executor_kind = 'agent'
            OR (parent_attempt_id IS NOT NULL AND parent_agent_id IS NOT NULL)
        ),
    CONSTRAINT ck_execution_attempts_child_session
        CHECK (child_session_id IS NULL OR executor_kind = 'subagent'),
    CONSTRAINT ck_execution_attempts_sdk_task_identity
        CHECK (
            sdk_task_id IS NULL
            OR (runtime_session_key IS NOT NULL AND sdk_session_id IS NOT NULL)
        ),
    CONSTRAINT ck_execution_attempts_status
        CHECK (status IN (
            'pending', 'running', 'succeeded', 'failed',
            'interrupted', 'cancelled', 'timed_out'
        )),
    CONSTRAINT ck_execution_attempts_version
        CHECK (version > 0),
    CONSTRAINT uq_execution_attempts_assignment_chain
        UNIQUE (attempt_id, assignment_id),
    CONSTRAINT uq_execution_attempts_full_chain
        UNIQUE (attempt_id, assignment_id, execution_id, plan_id, work_item_id, spec_id),
    FOREIGN KEY(assignment_id, execution_id, plan_id, work_item_id, spec_id)
        REFERENCES execution_work_assignments(
            assignment_id, execution_id, plan_id, work_item_id, spec_id
        ) ON DELETE CASCADE,
    FOREIGN KEY(dispatch_id, assignment_id, execution_id)
        REFERENCES execution_dispatches(dispatch_id, assignment_id, execution_id) ON DELETE CASCADE,
    FOREIGN KEY(parent_attempt_id, assignment_id)
        REFERENCES execution_attempts(attempt_id, assignment_id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX uq_execution_attempts_current_root
    ON execution_attempts (assignment_id)
    WHERE parent_attempt_id IS NULL AND status IN ('pending', 'running');
CREATE UNIQUE INDEX uq_execution_attempts_sdk_task
    ON execution_attempts (runtime_session_key, sdk_session_id, sdk_task_id)
    WHERE runtime_session_key IS NOT NULL
      AND sdk_session_id IS NOT NULL
      AND sdk_task_id IS NOT NULL;
CREATE UNIQUE INDEX uq_execution_attempts_room_round
    ON execution_attempts (runtime_session_key, runtime_round_id, agent_round_id)
    WHERE runtime_session_key IS NOT NULL
      AND runtime_round_id IS NOT NULL
      AND agent_round_id IS NOT NULL;
CREATE UNIQUE INDEX uq_execution_attempts_tool_use
    ON execution_attempts (runtime_session_key, tool_use_id)
    WHERE runtime_session_key IS NOT NULL AND tool_use_id IS NOT NULL;
CREATE INDEX idx_execution_attempts_runtime
    ON execution_attempts (runtime_session_key, sdk_session_id, status);
CREATE INDEX idx_execution_attempts_root_round
    ON execution_attempts (root_round_id, status);

CREATE TABLE execution_cancellation_dispatches (
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
CREATE INDEX idx_execution_cancellation_dispatches_due
    ON execution_cancellation_dispatches (
        status, available_at, lease_expires_at, created_at
    );
CREATE INDEX idx_execution_cancellation_dispatches_execution
    ON execution_cancellation_dispatches (execution_id, created_at);

CREATE TABLE execution_submissions (
    submission_id VARCHAR(64) NOT NULL PRIMARY KEY,
    execution_id VARCHAR(64) NOT NULL,
    plan_id VARCHAR(64) NOT NULL,
    work_item_id VARCHAR(64) NOT NULL,
    spec_id VARCHAR(64) NOT NULL,
    assignment_id VARCHAR(64) NOT NULL,
    attempt_id VARCHAR(64) NOT NULL,
    submission_sequence BIGINT NOT NULL,
    submitter_agent_id VARCHAR(128) NOT NULL,
    result_summary TEXT NOT NULL,
    result_refs_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    evidence_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    CONSTRAINT ck_execution_submissions_sequence
        CHECK (submission_sequence > 0),
    CONSTRAINT uq_execution_submissions_sequence
        UNIQUE (work_item_id, submission_sequence),
    CONSTRAINT uq_execution_submissions_chain
        UNIQUE (
            submission_id, execution_id, plan_id, work_item_id, spec_id, assignment_id
        ),
    FOREIGN KEY(assignment_id, execution_id, plan_id, work_item_id, spec_id)
        REFERENCES execution_work_assignments(
            assignment_id, execution_id, plan_id, work_item_id, spec_id
        ) ON DELETE CASCADE,
    FOREIGN KEY(attempt_id, assignment_id, execution_id, plan_id, work_item_id, spec_id)
        REFERENCES execution_attempts(
            attempt_id, assignment_id, execution_id, plan_id, work_item_id, spec_id
        ) ON DELETE CASCADE
);
CREATE UNIQUE INDEX uq_execution_submissions_attempt
    ON execution_submissions (attempt_id);
CREATE INDEX idx_execution_submissions_work_item
    ON execution_submissions (work_item_id, spec_id, created_at);

CREATE TABLE execution_review_dispatches (
    review_dispatch_id VARCHAR(64) NOT NULL PRIMARY KEY,
    execution_id VARCHAR(64) NOT NULL,
    plan_id VARCHAR(64) NOT NULL,
    work_item_id VARCHAR(64) NOT NULL,
    spec_id VARCHAR(64) NOT NULL,
    assignment_id VARCHAR(64) NOT NULL,
    submission_id VARCHAR(64) NOT NULL,
    command_id VARCHAR(128) NOT NULL,
    dedupe_key VARCHAR(256) NOT NULL,
    target_agent_id VARCHAR(128) NOT NULL,
    status VARCHAR(32) NOT NULL,
    instruction TEXT NOT NULL,
    handoff_id VARCHAR(128),
    queue_item_id VARCHAR(128),
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
    CONSTRAINT ck_execution_review_dispatches_status
        CHECK (status IN ('pending', 'claimed', 'delivered', 'cancelled', 'failed')),
    CONSTRAINT ck_execution_review_dispatches_attempts
        CHECK (delivery_attempts >= 0),
    CONSTRAINT ck_execution_review_dispatches_version
        CHECK (version > 0),
    CONSTRAINT uq_execution_review_dispatches_dedupe
        UNIQUE (execution_id, dedupe_key),
    CONSTRAINT uq_execution_review_dispatches_submission
        UNIQUE (submission_id),
    CONSTRAINT uq_execution_review_dispatches_chain
        UNIQUE (
            review_dispatch_id, submission_id, assignment_id, execution_id,
            plan_id, work_item_id, spec_id, target_agent_id
        ),
    FOREIGN KEY(
        submission_id, execution_id, plan_id, work_item_id, spec_id, assignment_id
    ) REFERENCES execution_submissions(
        submission_id, execution_id, plan_id, work_item_id, spec_id, assignment_id
    ) ON DELETE CASCADE,
    FOREIGN KEY(assignment_id, execution_id, target_agent_id)
        REFERENCES execution_work_assignments(
            assignment_id, execution_id, return_to_agent_id
        ) ON DELETE CASCADE
);
CREATE INDEX idx_execution_review_dispatches_due
    ON execution_review_dispatches (
        status, available_at, lease_expires_at, created_at
    );

CREATE TABLE execution_acceptances (
    acceptance_id VARCHAR(64) NOT NULL PRIMARY KEY,
    execution_id VARCHAR(64) NOT NULL,
    plan_id VARCHAR(64) NOT NULL,
    work_item_id VARCHAR(64) NOT NULL,
    spec_id VARCHAR(64) NOT NULL,
    assignment_id VARCHAR(64) NOT NULL,
    submission_id VARCHAR(64) NOT NULL,
    decision VARCHAR(32) NOT NULL,
    reviewer_kind VARCHAR(16) NOT NULL,
    reviewer_id VARCHAR(128) NOT NULL,
    criteria_results_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    feedback TEXT,
    decision_round_id VARCHAR(128),
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    CONSTRAINT ck_execution_acceptances_decision
        CHECK (decision IN ('accepted', 'rejected', 'changes_requested')),
    CONSTRAINT ck_execution_acceptances_reviewer_kind
        CHECK (reviewer_kind IN ('agent', 'user', 'system', 'policy')),
    CONSTRAINT uq_execution_acceptances_submission
        UNIQUE (submission_id),
    CONSTRAINT uq_execution_acceptances_chain
        UNIQUE (
            acceptance_id, execution_id, plan_id, work_item_id,
            spec_id, assignment_id, submission_id
        ),
    FOREIGN KEY(
        submission_id, execution_id, plan_id, work_item_id, spec_id, assignment_id
    ) REFERENCES execution_submissions(
        submission_id, execution_id, plan_id, work_item_id, spec_id, assignment_id
    ) ON DELETE CASCADE
);
CREATE UNIQUE INDEX uq_execution_acceptances_work_spec
    ON execution_acceptances (work_item_id, spec_id)
    WHERE decision = 'accepted';
CREATE INDEX idx_execution_acceptances_work_item
    ON execution_acceptances (work_item_id, spec_id, decision, created_at);

CREATE TABLE execution_events (
    event_id VARCHAR(64) NOT NULL PRIMARY KEY,
    execution_id VARCHAR(64) NOT NULL,
    sequence BIGINT NOT NULL,
    command_id VARCHAR(128) NOT NULL,
    event_type VARCHAR(64) NOT NULL,
    entity_type VARCHAR(32) NOT NULL,
    entity_id VARCHAR(64) NOT NULL,
    entity_version BIGINT NOT NULL,
    actor_kind VARCHAR(16) NOT NULL,
    actor_id VARCHAR(128),
    goal_id VARCHAR(64),
    plan_id VARCHAR(64),
    work_item_id VARCHAR(64),
    spec_id VARCHAR(64),
    assignment_id VARCHAR(64),
    dispatch_id VARCHAR(64),
    attempt_id VARCHAR(64),
    submission_id VARCHAR(64),
    review_dispatch_id VARCHAR(64),
    acceptance_id VARCHAR(64),
    root_round_id VARCHAR(128),
    runtime_round_id VARCHAR(128),
    agent_round_id VARCHAR(128),
    payload_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    CONSTRAINT ck_execution_events_sequence
        CHECK (sequence > 0),
    CONSTRAINT ck_execution_events_entity_version
        CHECK (entity_version > 0),
    CONSTRAINT ck_execution_events_entity_type
        CHECK (entity_type IN (
            'execution', 'plan', 'work_item', 'assignment',
            'dispatch', 'attempt', 'submission', 'review_dispatch', 'acceptance'
        )),
    CONSTRAINT ck_execution_events_actor_kind
        CHECK (actor_kind IN ('user', 'agent', 'runtime', 'system')),
    CONSTRAINT uq_execution_events_sequence
        UNIQUE (execution_id, sequence),
    CONSTRAINT uq_execution_events_command
        UNIQUE (execution_id, command_id),
    FOREIGN KEY(execution_id) REFERENCES executions(execution_id) ON DELETE CASCADE
);
CREATE INDEX idx_execution_events_execution
    ON execution_events (execution_id, sequence);
CREATE INDEX idx_execution_events_entity
    ON execution_events (execution_id, entity_type, entity_id, sequence);

-- +goose Down

DROP TABLE IF EXISTS execution_events;
DROP TABLE IF EXISTS execution_acceptances;
DROP TABLE IF EXISTS execution_review_dispatches;
DROP TABLE IF EXISTS execution_submissions;
DROP TABLE IF EXISTS execution_cancellation_dispatches;
DROP TABLE IF EXISTS execution_attempts;
DROP TABLE IF EXISTS execution_dispatches;
DROP TABLE IF EXISTS execution_work_assignments;
DROP TABLE IF EXISTS execution_work_item_states;
DROP TABLE IF EXISTS execution_plan_output_claims;
DROP TABLE IF EXISTS execution_plan_dependencies;
DROP TABLE IF EXISTS execution_plan_items;
DROP TABLE IF EXISTS execution_work_item_specs;
DROP TABLE IF EXISTS execution_work_items;
DROP TABLE IF EXISTS execution_plan_revisions;
DROP TABLE IF EXISTS goal_execution_identity_claims;
DROP TABLE IF EXISTS executions;
