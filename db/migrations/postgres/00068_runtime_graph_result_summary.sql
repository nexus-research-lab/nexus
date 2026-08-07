-- +goose Up

ALTER TABLE runtime_graph_node_runs
    ADD COLUMN result_summary TEXT,
    ADD COLUMN error_code VARCHAR(128),
    ADD COLUMN error_summary TEXT,
    ADD COLUMN summary_truncated BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN duration_ms BIGINT NOT NULL DEFAULT 0;

ALTER TABLE runtime_graph_edge_runs
    DROP CONSTRAINT IF EXISTS ck_runtime_graph_edge_kind;
ALTER TABLE runtime_graph_edge_runs
    ADD CONSTRAINT ck_runtime_graph_edge_kind
    CHECK (edge_kind IN ('invoke', 'spawn', 'guard', 'loop_back', 'retry'));

-- +goose Down

DELETE FROM runtime_graph_edge_runs
WHERE edge_kind = 'retry';

ALTER TABLE runtime_graph_edge_runs
    DROP CONSTRAINT IF EXISTS ck_runtime_graph_edge_kind;
ALTER TABLE runtime_graph_edge_runs
    ADD CONSTRAINT ck_runtime_graph_edge_kind
    CHECK (edge_kind IN ('invoke', 'spawn', 'guard', 'loop_back'));

ALTER TABLE runtime_graph_node_runs
    DROP COLUMN duration_ms,
    DROP COLUMN summary_truncated,
    DROP COLUMN error_summary,
    DROP COLUMN error_code,
    DROP COLUMN result_summary;
