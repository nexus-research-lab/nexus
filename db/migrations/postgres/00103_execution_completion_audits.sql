-- +goose Up

-- execution_completion_audits closes the crash window between committing an
-- accepted review and running the authoritative Execution completion check.
-- The latest accepted review wakes one durable receipt; completion settles it
-- in the same transaction as the Execution terminal transition.
CREATE TABLE execution_completion_audits (
    execution_id VARCHAR(64) NOT NULL PRIMARY KEY
        REFERENCES executions(execution_id) ON DELETE CASCADE,
    trigger_acceptance_id VARCHAR(64) NOT NULL
        REFERENCES execution_acceptances(acceptance_id) ON DELETE CASCADE,
    state VARCHAR(16) NOT NULL DEFAULT 'pending',
    version BIGINT NOT NULL DEFAULT 1,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMP WITHOUT TIME ZONE,
    last_error TEXT,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    settled_at TIMESTAMP WITHOUT TIME ZONE,
    CONSTRAINT ck_execution_completion_audits_state
        CHECK (state IN ('pending', 'completed', 'discarded')),
    CONSTRAINT ck_execution_completion_audits_version
        CHECK (version > 0),
    CONSTRAINT ck_execution_completion_audits_attempt_count
        CHECK (attempt_count >= 0),
    CONSTRAINT ck_execution_completion_audits_terminal
        CHECK (
            (state = 'pending' AND next_attempt_at IS NOT NULL AND settled_at IS NULL)
            OR
            (state = 'completed' AND next_attempt_at IS NULL
                AND last_error IS NULL AND settled_at IS NOT NULL)
            OR
            (state = 'discarded' AND next_attempt_at IS NULL
                AND last_error IS NOT NULL AND settled_at IS NOT NULL)
        )
);

CREATE INDEX idx_execution_completion_audits_recoverable
    ON execution_completion_audits (state, next_attempt_at, updated_at);

-- Older binaries could commit Acceptance and crash before Complete without
-- leaving a retry identity. Recover only active managed graphs with an accepted
-- current-Plan item; the reconciler re-derives all blockers before completion.
INSERT INTO execution_completion_audits (
    execution_id, trigger_acceptance_id, state, version, attempt_count,
    next_attempt_at, last_error, created_at, updated_at, settled_at
)
SELECT
    execution.execution_id,
    (
        SELECT acceptance.acceptance_id
        FROM execution_acceptances AS acceptance
        WHERE acceptance.execution_id = execution.execution_id
          AND acceptance.plan_id = plan.plan_id
          AND acceptance.decision = 'accepted'
        ORDER BY acceptance.created_at DESC, acceptance.acceptance_id DESC
        LIMIT 1
    ),
    'pending', 1, 0,
    now(),
    'legacy accepted review recovered during migration',
    execution.updated_at,
    now(),
    NULL
FROM executions AS execution
JOIN execution_plan_revisions AS plan
  ON plan.execution_id = execution.execution_id
 AND plan.status = 'active'
WHERE execution.status IN ('active', 'waiting', 'paused')
  AND EXISTS (
      SELECT 1
      FROM execution_acceptances AS acceptance
      WHERE acceptance.execution_id = execution.execution_id
        AND acceptance.plan_id = plan.plan_id
        AND acceptance.decision = 'accepted'
  );

-- +goose Down

DROP INDEX IF EXISTS idx_execution_completion_audits_recoverable;
DROP TABLE IF EXISTS execution_completion_audits;
