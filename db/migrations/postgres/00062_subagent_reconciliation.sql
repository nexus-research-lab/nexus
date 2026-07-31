-- +goose Up

ALTER TABLE execution_attempts
ADD COLUMN parent_round_exited_at TIMESTAMP WITHOUT TIME ZONE;

ALTER TABLE execution_attempts
ADD COLUMN reconcile_after TIMESTAMP WITHOUT TIME ZONE;

ALTER TABLE execution_attempts
ADD CONSTRAINT ck_execution_attempts_reconciliation
CHECK (
    (parent_round_exited_at IS NULL AND reconcile_after IS NULL)
    OR
    (
        parent_attempt_id IS NOT NULL
        AND parent_round_exited_at IS NOT NULL
        AND reconcile_after IS NOT NULL
        AND reconcile_after >= parent_round_exited_at
    )
);

CREATE INDEX idx_execution_attempts_reconcile
    ON execution_attempts (reconcile_after, status)
    WHERE reconcile_after IS NOT NULL AND status = 'running';

-- +goose Down

DROP INDEX IF EXISTS idx_execution_attempts_reconcile;

ALTER TABLE execution_attempts
DROP CONSTRAINT IF EXISTS ck_execution_attempts_reconciliation;

ALTER TABLE execution_attempts DROP COLUMN reconcile_after;

ALTER TABLE execution_attempts DROP COLUMN parent_round_exited_at;
