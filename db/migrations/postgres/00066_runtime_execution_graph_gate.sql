-- +goose Up

ALTER TABLE runtime_graph_node_runs
    DROP CONSTRAINT IF EXISTS ck_runtime_graph_node_kind;
ALTER TABLE runtime_graph_node_runs
    ADD CONSTRAINT ck_runtime_graph_node_kind
    CHECK (node_kind IN ('agent', 'subagent', 'tool', 'gate'));

ALTER TABLE runtime_graph_edge_runs
    DROP CONSTRAINT IF EXISTS ck_runtime_graph_edge_kind;
ALTER TABLE runtime_graph_edge_runs
    ADD CONSTRAINT ck_runtime_graph_edge_kind
    CHECK (edge_kind IN ('invoke', 'spawn', 'guard', 'loop_back'));

-- +goose Down

DELETE FROM runtime_graph_edge_runs
WHERE edge_kind NOT IN ('invoke', 'spawn')
   OR source_node_run_id IN (
       SELECT node_run_id FROM runtime_graph_node_runs WHERE node_kind = 'gate'
   )
   OR target_node_run_id IN (
       SELECT node_run_id FROM runtime_graph_node_runs WHERE node_kind = 'gate'
   );

DELETE FROM runtime_graph_node_runs
WHERE node_kind = 'gate';

ALTER TABLE runtime_graph_edge_runs
    DROP CONSTRAINT IF EXISTS ck_runtime_graph_edge_kind;
ALTER TABLE runtime_graph_edge_runs
    ADD CONSTRAINT ck_runtime_graph_edge_kind
    CHECK (edge_kind IN ('invoke', 'spawn'));

ALTER TABLE runtime_graph_node_runs
    DROP CONSTRAINT IF EXISTS ck_runtime_graph_node_kind;
ALTER TABLE runtime_graph_node_runs
    ADD CONSTRAINT ck_runtime_graph_node_kind
    CHECK (node_kind IN ('agent', 'subagent', 'tool'));
