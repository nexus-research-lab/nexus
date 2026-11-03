-- +goose Up
-- Freeze the upgrade's existing executions. New executions never read transcripts.
CREATE TABLE runtime_graph_history_backfills (
 execution_id TEXT PRIMARY KEY REFERENCES executions(execution_id) ON DELETE CASCADE,
 completed_at TIMESTAMP
);
INSERT INTO runtime_graph_history_backfills (execution_id)
SELECT execution_id FROM executions;
CREATE INDEX idx_runtime_graph_history_pending ON runtime_graph_history_backfills(execution_id) WHERE completed_at IS NULL;

-- +goose Down
DROP TABLE runtime_graph_history_backfills;
