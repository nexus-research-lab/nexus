-- +goose Up

ALTER TABLE workgraph_workflows
    ADD COLUMN artifact_contract_json JSONB NOT NULL DEFAULT 'null'::jsonb;

-- +goose Down

ALTER TABLE workgraph_workflows
    DROP COLUMN artifact_contract_json;
