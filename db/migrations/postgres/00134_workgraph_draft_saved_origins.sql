-- +goose Up

ALTER TABLE workgraph_workflow_drafts ADD COLUMN origin_workflow_id VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE workgraph_workflow_drafts DROP CONSTRAINT uq_workgraph_workflow_draft_source;
ALTER TABLE workgraph_workflow_drafts ADD CONSTRAINT uq_workgraph_workflow_draft_source
    UNIQUE (owner_user_id, source_session_key, source_execution_id, origin_workflow_id);

-- UI saves no longer use a background scheduling claim.
UPDATE workgraph_workflow_drafts SET save_scheduled = FALSE;

-- Retain drafts of commands deleted by older versions, with no stale saved binding.
UPDATE workgraph_workflow_drafts SET saved_workflow_id = '', saved_revision = 0
WHERE saved_workflow_id <> '' AND NOT EXISTS (
    SELECT 1 FROM workgraph_workflows w
    WHERE w.workflow_id = workgraph_workflow_drafts.saved_workflow_id
      AND w.owner_user_id = workgraph_workflow_drafts.owner_user_id
);

-- +goose Down

-- Refuse downgrade if restored graphs share a source; never discard them.
ALTER TABLE workgraph_workflow_drafts DROP CONSTRAINT uq_workgraph_workflow_draft_source;
ALTER TABLE workgraph_workflow_drafts ADD CONSTRAINT uq_workgraph_workflow_draft_source
    UNIQUE (owner_user_id, source_session_key, source_execution_id);
ALTER TABLE workgraph_workflow_drafts DROP COLUMN origin_workflow_id;
