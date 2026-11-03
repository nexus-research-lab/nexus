-- +goose Up
CREATE TABLE execution_work_checkpoints (
 checkpoint_id TEXT PRIMARY KEY,
 execution_id TEXT NOT NULL REFERENCES executions(execution_id) ON DELETE CASCADE,
 work_item_id TEXT NOT NULL REFERENCES execution_work_items(work_item_id) ON DELETE CASCADE,
 spec_id TEXT NOT NULL,
 attempt_id TEXT NOT NULL REFERENCES execution_attempts(attempt_id) ON DELETE CASCADE,
 payload_json TEXT NOT NULL,
 execution_version BIGINT NOT NULL CHECK (execution_version > 0),
 created_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX idx_execution_checkpoint_current ON execution_work_checkpoints(execution_id, work_item_id, spec_id, execution_version DESC);
-- +goose Down
DROP TABLE execution_work_checkpoints;
