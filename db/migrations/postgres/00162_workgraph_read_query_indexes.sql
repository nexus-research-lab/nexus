-- +goose Up
-- WorkGraph reads ordinary edges ending at retained nodes in bounded identity batches; control edges remain complete.
CREATE INDEX idx_runtime_graph_edges_target ON runtime_graph_edge_runs(owner_user_id, session_key, target_node_run_id, graph_id);
-- Match lexicographic recovery ranges, including equal-timestamp run IDs.
DROP INDEX idx_runtime_graph_recovery_key;
CREATE INDEX idx_runtime_graph_recovery_key ON runtime_graph_node_runs(owner_user_id, session_key, recovery_key, status, started_at, node_run_id);

-- +goose Down
DROP INDEX idx_runtime_graph_recovery_key;
CREATE INDEX idx_runtime_graph_recovery_key ON runtime_graph_node_runs(owner_user_id, session_key, recovery_key, status, started_at);
DROP INDEX idx_runtime_graph_edges_target;
