-- +goose Up
ALTER TABLE workgraph_workflows ADD COLUMN parameters_json TEXT NOT NULL DEFAULT '[]';
CREATE TABLE execution_workflow_usage (
 execution_id TEXT PRIMARY KEY REFERENCES executions(execution_id) ON DELETE CASCADE,
 owner_user_id TEXT NOT NULL,
 workflow_id TEXT NOT NULL,
 workflow_version BIGINT NOT NULL CHECK (workflow_version > 0),
 parameters_json TEXT NOT NULL,
 created_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX idx_execution_workflow_usage_owner ON execution_workflow_usage(owner_user_id, workflow_id, workflow_version);
-- +goose Down
DROP TABLE execution_workflow_usage;
ALTER TABLE workgraph_workflows DROP COLUMN parameters_json;
