-- +goose Up

ALTER TABLE execution_attempts
ADD COLUMN parent_round_exited_at DATETIME;

ALTER TABLE execution_attempts
ADD COLUMN reconcile_after DATETIME;

CREATE INDEX idx_execution_attempts_reconcile
    ON execution_attempts (reconcile_after, status)
    WHERE reconcile_after IS NOT NULL AND status = 'running';

-- +goose Down

DROP INDEX IF EXISTS idx_execution_attempts_reconcile;

ALTER TABLE execution_attempts DROP COLUMN reconcile_after;

ALTER TABLE execution_attempts DROP COLUMN parent_round_exited_at;
