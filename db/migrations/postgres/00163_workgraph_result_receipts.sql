-- +goose Up
CREATE TABLE execution_result_receipts (
    receipt_id TEXT PRIMARY KEY,
    execution_id TEXT NOT NULL REFERENCES executions(execution_id) ON DELETE CASCADE,
    owner_user_id TEXT NOT NULL,
    kind TEXT NOT NULL CHECK (kind IN ('delivery', 'review')),
    target_id TEXT NOT NULL,
    actor_json TEXT NOT NULL,
    result_json TEXT NOT NULL,
    state TEXT NOT NULL DEFAULT 'pending' CHECK (state IN ('pending', 'applied', 'rejected')),
    reason TEXT NOT NULL DEFAULT '',
    next_attempt_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    UNIQUE (execution_id, kind, target_id)
);
CREATE INDEX idx_execution_results_pending ON execution_result_receipts(state, next_attempt_at, receipt_id);

-- +goose Down
DROP TABLE execution_result_receipts;
