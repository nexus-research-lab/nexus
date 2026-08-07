-- +goose Up

CREATE TABLE IF NOT EXISTS runtime_graph_artifact_refs (
    artifact_ref_id VARCHAR(80) NOT NULL PRIMARY KEY,
    graph_id VARCHAR(160) NOT NULL,
    owner_user_id VARCHAR(128) NOT NULL,
    session_key VARCHAR(512) NOT NULL,
    execution_id VARCHAR(64),
    root_round_id VARCHAR(128) NOT NULL,
    agent_round_id VARCHAR(128) NOT NULL,
    tool_use_id VARCHAR(256) NOT NULL,
    artifact_json TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_runtime_graph_artifacts_exact_tool
    ON runtime_graph_artifact_refs (
        owner_user_id,
        session_key,
        agent_round_id,
        tool_use_id,
        updated_at
    );
CREATE INDEX IF NOT EXISTS idx_runtime_graph_artifacts_graph
    ON runtime_graph_artifact_refs (owner_user_id, session_key, graph_id, updated_at);

-- +goose Down

DROP TABLE IF EXISTS runtime_graph_artifact_refs;
