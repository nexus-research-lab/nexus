-- +goose Up

ALTER TABLE workgraph_workflows
    ADD COLUMN artifact_contract_json TEXT NOT NULL DEFAULT 'null';

-- +goose Down

ALTER TABLE workgraph_workflows DROP COLUMN artifact_contract_json;
