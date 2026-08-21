-- +goose Up

CREATE TABLE workgraph_workflow_drafts (
    preview_id VARCHAR(64) NOT NULL PRIMARY KEY,
    owner_user_id VARCHAR(128) NOT NULL,
    source_execution_id VARCHAR(64) NOT NULL,
    source_session_key VARCHAR(512) NOT NULL,
    source_agent_id VARCHAR(64) NOT NULL,
    source_conversation_id VARCHAR(128) NOT NULL DEFAULT '',
    output_language VARCHAR(8) NOT NULL,
    head_revision BIGINT NOT NULL,
    selected_revision BIGINT NOT NULL,
    editor_id VARCHAR(64) NOT NULL DEFAULT '',
    editor_agent_id VARCHAR(64) NOT NULL DEFAULT '',
    editor_session_key VARCHAR(512) NOT NULL DEFAULT '',
    editor_display_after_unix_milli BIGINT NOT NULL DEFAULT 0,
    save_scheduled BOOLEAN NOT NULL DEFAULT FALSE,
    saved_workflow_id VARCHAR(64) NOT NULL DEFAULT '',
    saved_revision BIGINT NOT NULL DEFAULT 0,
    expires_at TIMESTAMP WITHOUT TIME ZONE NOT NULL,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    updated_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    CONSTRAINT uq_workgraph_workflow_draft_source
        UNIQUE (owner_user_id, source_session_key, source_execution_id),
    CONSTRAINT ck_workgraph_workflow_draft_revisions
        CHECK (head_revision > 0 AND selected_revision > 0 AND selected_revision <= head_revision
            AND saved_revision >= 0 AND saved_revision <= head_revision)
);
CREATE INDEX idx_workgraph_workflow_drafts_owner_session_updated
    ON workgraph_workflow_drafts (owner_user_id, source_session_key, updated_at DESC, preview_id);
CREATE INDEX idx_workgraph_workflow_drafts_owner_editor_session
    ON workgraph_workflow_drafts (owner_user_id, editor_session_key);

CREATE TABLE workgraph_workflow_draft_versions (
    preview_id VARCHAR(64) NOT NULL,
    revision BIGINT NOT NULL,
    preview_json JSONB NOT NULL,
    created_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL,
    PRIMARY KEY (preview_id, revision),
    CONSTRAINT ck_workgraph_workflow_draft_versions_revision CHECK (revision > 0),
    FOREIGN KEY(preview_id) REFERENCES workgraph_workflow_drafts(preview_id) ON DELETE CASCADE
);

-- +goose Down

DROP TABLE workgraph_workflow_draft_versions;
DROP TABLE workgraph_workflow_drafts;
