-- +goose Up
-- Empty means unclassified: retain it until the versioned backfill proves detail-only.
ALTER TABLE runtime_graph_node_runs ADD COLUMN workgraph_read_class TEXT NOT NULL DEFAULT '' CHECK (workgraph_read_class IN ('', 'detail'));
ALTER TABLE runtime_graph_node_runs ADD COLUMN recovery_key TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_runtime_graph_read_class ON runtime_graph_node_runs(owner_user_id, session_key, execution_id, workgraph_read_class, updated_at DESC);
CREATE INDEX idx_runtime_graph_recovery_key ON runtime_graph_node_runs(owner_user_id, session_key, recovery_key, status, started_at);
CREATE INDEX idx_runtime_graph_parent ON runtime_graph_node_runs(owner_user_id, session_key, agent_round_id, parent_subject_id);

-- +goose Down
DROP INDEX idx_runtime_graph_parent;
DROP INDEX idx_runtime_graph_recovery_key;
DROP INDEX idx_runtime_graph_read_class;
ALTER TABLE runtime_graph_node_runs DROP COLUMN recovery_key;
ALTER TABLE runtime_graph_node_runs DROP COLUMN workgraph_read_class;
