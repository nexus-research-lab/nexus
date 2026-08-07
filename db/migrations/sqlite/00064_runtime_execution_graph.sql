-- +goose Up

CREATE TABLE IF NOT EXISTS runtime_graph_node_runs (
    node_run_id VARCHAR(80) NOT NULL PRIMARY KEY,
    graph_id VARCHAR(160) NOT NULL,
    owner_user_id VARCHAR(128) NOT NULL,
    session_key VARCHAR(512) NOT NULL,
    execution_id VARCHAR(64),
    node_kind VARCHAR(16) NOT NULL,
    subject_id VARCHAR(256) NOT NULL,
    parent_subject_id VARCHAR(256),
    root_round_id VARCHAR(128) NOT NULL,
    runtime_round_id VARCHAR(128) NOT NULL,
    agent_round_id VARCHAR(128) NOT NULL,
    agent_id VARCHAR(128),
    name VARCHAR(512),
    description TEXT,
    status VARCHAR(32) NOT NULL,
    failed BOOLEAN NOT NULL DEFAULT FALSE,
    started_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    finished_at DATETIME,
    metadata_json TEXT NOT NULL DEFAULT '{}',
    CONSTRAINT ck_runtime_graph_node_kind
        CHECK (node_kind IN ('agent', 'subagent', 'tool', 'gate')),
    CONSTRAINT ck_runtime_graph_node_status
        CHECK (status IN ('running', 'succeeded', 'failed', 'cancelled', 'interrupted')),
    CONSTRAINT uq_runtime_graph_node_subject
        UNIQUE (owner_user_id, session_key, agent_round_id, node_kind, subject_id)
);

CREATE INDEX IF NOT EXISTS idx_runtime_graph_nodes_latest
    ON runtime_graph_node_runs (owner_user_id, session_key, updated_at DESC);
CREATE INDEX IF NOT EXISTS idx_runtime_graph_nodes_execution
    ON runtime_graph_node_runs (owner_user_id, execution_id, updated_at);
CREATE INDEX IF NOT EXISTS idx_runtime_graph_nodes_round
    ON runtime_graph_node_runs (owner_user_id, session_key, root_round_id, agent_round_id);

CREATE TABLE IF NOT EXISTS runtime_graph_edge_runs (
    edge_run_id VARCHAR(80) NOT NULL PRIMARY KEY,
    graph_id VARCHAR(160) NOT NULL,
    owner_user_id VARCHAR(128) NOT NULL,
    session_key VARCHAR(512) NOT NULL,
    source_node_run_id VARCHAR(80) NOT NULL,
    target_node_run_id VARCHAR(80) NOT NULL,
    edge_kind VARCHAR(24) NOT NULL,
    created_at DATETIME NOT NULL,
    CONSTRAINT ck_runtime_graph_edge_kind
        CHECK (edge_kind IN ('invoke', 'spawn', 'guard', 'loop_back')),
    CONSTRAINT uq_runtime_graph_edge
        UNIQUE (graph_id, source_node_run_id, target_node_run_id, edge_kind),
    FOREIGN KEY(source_node_run_id)
        REFERENCES runtime_graph_node_runs(node_run_id) ON DELETE CASCADE,
    FOREIGN KEY(target_node_run_id)
        REFERENCES runtime_graph_node_runs(node_run_id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_runtime_graph_edges_session
    ON runtime_graph_edge_runs (owner_user_id, session_key, graph_id, created_at);

-- +goose Down

DROP TABLE IF EXISTS runtime_graph_edge_runs;
DROP TABLE IF EXISTS runtime_graph_node_runs;
