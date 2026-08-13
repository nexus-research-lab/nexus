-- +goose Up

-- execution_goal_confirmations is the durable cross-domain receipt for the
-- interval after the Execution-side Goal binding commits and before the Goal
-- aggregate confirms the reverse binding. The exact criteria snapshot makes
-- retry independent from an in-memory request or one plan proposal row.
CREATE TABLE execution_goal_confirmations (
    execution_id VARCHAR(64) NOT NULL PRIMARY KEY
        REFERENCES executions(execution_id) ON DELETE CASCADE,
    goal_id VARCHAR(64) NOT NULL,
    goal_objective_revision BIGINT NOT NULL,
    completion_criteria_json JSONB NOT NULL,
    state VARCHAR(16) NOT NULL DEFAULT 'pending',
    version BIGINT NOT NULL DEFAULT 1,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMP WITHOUT TIME ZONE,
    last_error TEXT,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    confirmed_at TIMESTAMP WITHOUT TIME ZONE,
    CONSTRAINT ck_execution_goal_confirmations_revision
        CHECK (goal_objective_revision > 0),
    CONSTRAINT ck_execution_goal_confirmations_state
        CHECK (state IN ('pending', 'confirmed')),
    CONSTRAINT ck_execution_goal_confirmations_version
        CHECK (version > 0),
    CONSTRAINT ck_execution_goal_confirmations_attempt_count
        CHECK (attempt_count >= 0),
    CONSTRAINT ck_execution_goal_confirmations_terminal
        CHECK (
            (state = 'pending' AND next_attempt_at IS NOT NULL AND confirmed_at IS NULL)
            OR
            (state = 'confirmed' AND next_attempt_at IS NULL AND last_error IS NULL
                AND confirmed_at IS NOT NULL)
        )
);

CREATE INDEX idx_execution_goal_confirmations_recoverable
    ON execution_goal_confirmations (state, next_attempt_at, updated_at);

-- Only import legacy rows whose Goal truth explicitly says that this exact
-- Execution/revision is pending. Confirmed or stale historical bindings do not
-- need a recovery receipt and must not be guessed from Execution truth alone.
INSERT INTO execution_goal_confirmations (
    execution_id, goal_id, goal_objective_revision,
    completion_criteria_json, state, version, attempt_count,
    next_attempt_at, last_error, created_at, updated_at, confirmed_at
)
SELECT
    execution.execution_id,
    execution.goal_id,
    execution.goal_objective_revision,
    execution.completion_criteria_json,
    'pending', 1, 0,
    now(),
    'legacy pending Goal confirmation recovered during migration',
    execution.updated_at,
    now(),
    NULL
FROM executions AS execution
JOIN session_goals AS goal
  ON goal.goal_id = execution.goal_id
WHERE execution.goal_id IS NOT NULL
  AND goal.metadata_json ->> 'execution_binding_state' = 'pending'
  AND goal.metadata_json ->> 'execution_id' = execution.execution_id
  AND COALESCE(
        CASE
          WHEN goal.metadata_json ->> 'objective_revision' ~ '^[1-9][0-9]*$'
          THEN (goal.metadata_json ->> 'objective_revision')::BIGINT
        END,
        1
      ) = execution.goal_objective_revision;

-- +goose Down

DROP INDEX IF EXISTS idx_execution_goal_confirmations_recoverable;
DROP TABLE IF EXISTS execution_goal_confirmations;
