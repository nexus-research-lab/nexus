-- +goose Up

CREATE TABLE workgraph_workflows (
    workflow_id VARCHAR(64) NOT NULL PRIMARY KEY,
    owner_user_id VARCHAR(128) NOT NULL,
    slash_name VARCHAR(64) NOT NULL,
    title VARCHAR(160) NOT NULL,
    description TEXT,
    source_execution_id VARCHAR(64) NOT NULL,
    source_session_key VARCHAR(512) NOT NULL,
    objective TEXT NOT NULL,
    completion_criteria_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    CONSTRAINT ck_workgraph_workflows_version CHECK (version > 0),
    CONSTRAINT uq_workgraph_workflows_slash UNIQUE (owner_user_id, slash_name)
);
CREATE INDEX idx_workgraph_workflows_owner_updated
    ON workgraph_workflows (owner_user_id, updated_at DESC, workflow_id);

CREATE TABLE workgraph_workflow_nodes (
    workflow_id VARCHAR(64) NOT NULL,
    logical_key VARCHAR(64) NOT NULL,
    source_work_item_id VARCHAR(64) NOT NULL,
    role VARCHAR(24) NOT NULL,
    kind VARCHAR(16) NOT NULL,
    subject VARCHAR(512) NOT NULL,
    objective TEXT NOT NULL,
    deliverable TEXT NOT NULL,
    acceptance_criteria_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    is_required BOOLEAN NOT NULL DEFAULT TRUE,
    is_terminal BOOLEAN NOT NULL DEFAULT FALSE,
    parent_logical_key VARCHAR(64),
    position INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (workflow_id, logical_key),
    CONSTRAINT ck_workgraph_workflow_nodes_role CHECK (role IN ('key', 'collaboration')),
    CONSTRAINT ck_workgraph_workflow_nodes_kind CHECK (kind IN ('produce', 'review', 'verify', 'integrate')),
    CONSTRAINT ck_workgraph_workflow_nodes_position CHECK (position >= 0),
    FOREIGN KEY(workflow_id) REFERENCES workgraph_workflows(workflow_id) ON DELETE CASCADE,
    FOREIGN KEY(workflow_id, parent_logical_key)
        REFERENCES workgraph_workflow_nodes(workflow_id, logical_key)
);

CREATE TABLE workgraph_workflow_dependencies (
    workflow_id VARCHAR(64) NOT NULL,
    logical_key VARCHAR(64) NOT NULL,
    depends_on_logical_key VARCHAR(64) NOT NULL,
    dependency_kind VARCHAR(16) NOT NULL DEFAULT 'hard',
    PRIMARY KEY (workflow_id, logical_key, depends_on_logical_key),
    CONSTRAINT ck_workgraph_workflow_dependencies_not_self CHECK (logical_key <> depends_on_logical_key),
    CONSTRAINT ck_workgraph_workflow_dependencies_kind CHECK (dependency_kind IN ('hard', 'soft')),
    FOREIGN KEY(workflow_id, logical_key)
        REFERENCES workgraph_workflow_nodes(workflow_id, logical_key) ON DELETE CASCADE,
    FOREIGN KEY(workflow_id, depends_on_logical_key)
        REFERENCES workgraph_workflow_nodes(workflow_id, logical_key) ON DELETE CASCADE
);

-- +goose Down

DROP TABLE workgraph_workflow_dependencies;
DROP TABLE workgraph_workflow_nodes;
DROP TABLE workgraph_workflows;
