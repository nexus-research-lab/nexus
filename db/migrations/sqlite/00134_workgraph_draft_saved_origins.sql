-- +goose Up

-- Source extraction remains unique; each historical named graph can restore its own Draft.
CREATE TABLE IF NOT EXISTS workgraph_workflow_drafts_next (
    preview_id VARCHAR(64) NOT NULL PRIMARY KEY,
    owner_user_id VARCHAR(128) NOT NULL,
    source_execution_id VARCHAR(64) NOT NULL,
    source_session_key VARCHAR(512) NOT NULL,
    source_agent_id VARCHAR(64) NOT NULL,
    source_conversation_id VARCHAR(128) NOT NULL DEFAULT '',
    output_language VARCHAR(8) NOT NULL,
    head_revision INTEGER NOT NULL,
    selected_revision INTEGER NOT NULL,
    editor_id VARCHAR(64) NOT NULL DEFAULT '',
    editor_agent_id VARCHAR(64) NOT NULL DEFAULT '',
    editor_session_key VARCHAR(512) NOT NULL DEFAULT '',
    editor_display_after_unix_milli INTEGER NOT NULL DEFAULT 0,
    save_scheduled BOOLEAN NOT NULL DEFAULT FALSE,
    saved_workflow_id VARCHAR(64) NOT NULL DEFAULT '',
    saved_revision INTEGER NOT NULL DEFAULT 0,
    expires_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    origin_workflow_id VARCHAR(64) NOT NULL DEFAULT '',
    CONSTRAINT uq_workgraph_workflow_draft_source
        UNIQUE (owner_user_id, source_session_key, source_execution_id, origin_workflow_id),
    CONSTRAINT ck_workgraph_workflow_draft_revisions
        CHECK (head_revision > 0 AND selected_revision > 0 AND selected_revision <= head_revision
            AND saved_revision >= 0 AND saved_revision <= head_revision)
);
INSERT INTO workgraph_workflow_drafts_next (preview_id, owner_user_id, source_execution_id, source_session_key, source_agent_id, source_conversation_id, output_language, head_revision, selected_revision, editor_id, editor_agent_id, editor_session_key, editor_display_after_unix_milli, save_scheduled, saved_workflow_id, saved_revision, expires_at, created_at, updated_at, origin_workflow_id)
SELECT preview_id, owner_user_id, source_execution_id, source_session_key, source_agent_id, source_conversation_id, output_language, head_revision, selected_revision, editor_id, editor_agent_id, editor_session_key, editor_display_after_unix_milli, save_scheduled, saved_workflow_id, saved_revision, expires_at, created_at, updated_at, '' FROM workgraph_workflow_drafts;

CREATE TABLE IF NOT EXISTS workgraph_workflow_draft_versions_next (
    preview_id VARCHAR(64) NOT NULL,
    revision INTEGER NOT NULL,
    preview_json TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY (preview_id, revision),
    CONSTRAINT ck_workgraph_workflow_draft_versions_revision CHECK (revision > 0),
    FOREIGN KEY(preview_id) REFERENCES workgraph_workflow_drafts_next(preview_id) ON DELETE CASCADE
);

INSERT INTO workgraph_workflow_draft_versions_next SELECT * FROM workgraph_workflow_draft_versions;
DROP TABLE workgraph_workflow_draft_versions;
DROP TABLE workgraph_workflow_drafts;
ALTER TABLE workgraph_workflow_drafts_next RENAME TO workgraph_workflow_drafts;
ALTER TABLE workgraph_workflow_draft_versions_next RENAME TO workgraph_workflow_draft_versions;

CREATE INDEX IF NOT EXISTS idx_workgraph_workflow_drafts_owner_session_updated
    ON workgraph_workflow_drafts (owner_user_id, source_session_key, updated_at DESC, preview_id);
CREATE INDEX IF NOT EXISTS idx_workgraph_workflow_drafts_owner_editor_session
    ON workgraph_workflow_drafts (owner_user_id, editor_session_key);

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

-- Refuse downgrade atomically if multiple restored graphs share a source; never discard them.
CREATE TABLE IF NOT EXISTS workgraph_workflow_drafts_next (
    preview_id VARCHAR(64) NOT NULL PRIMARY KEY,
    owner_user_id VARCHAR(128) NOT NULL,
    source_execution_id VARCHAR(64) NOT NULL,
    source_session_key VARCHAR(512) NOT NULL,
    source_agent_id VARCHAR(64) NOT NULL,
    source_conversation_id VARCHAR(128) NOT NULL DEFAULT '',
    output_language VARCHAR(8) NOT NULL,
    head_revision INTEGER NOT NULL,
    selected_revision INTEGER NOT NULL,
    editor_id VARCHAR(64) NOT NULL DEFAULT '',
    editor_agent_id VARCHAR(64) NOT NULL DEFAULT '',
    editor_session_key VARCHAR(512) NOT NULL DEFAULT '',
    editor_display_after_unix_milli INTEGER NOT NULL DEFAULT 0,
    save_scheduled BOOLEAN NOT NULL DEFAULT FALSE,
    saved_workflow_id VARCHAR(64) NOT NULL DEFAULT '',
    saved_revision INTEGER NOT NULL DEFAULT 0,
    expires_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT uq_workgraph_workflow_draft_source
        UNIQUE (owner_user_id, source_session_key, source_execution_id),
    CONSTRAINT ck_workgraph_workflow_draft_revisions
        CHECK (head_revision > 0 AND selected_revision > 0 AND selected_revision <= head_revision
            AND saved_revision >= 0 AND saved_revision <= head_revision)
);
INSERT INTO workgraph_workflow_drafts_next (preview_id, owner_user_id, source_execution_id, source_session_key, source_agent_id, source_conversation_id, output_language, head_revision, selected_revision, editor_id, editor_agent_id, editor_session_key, editor_display_after_unix_milli, save_scheduled, saved_workflow_id, saved_revision, expires_at, created_at, updated_at)
SELECT preview_id, owner_user_id, source_execution_id, source_session_key, source_agent_id, source_conversation_id, output_language, head_revision, selected_revision, editor_id, editor_agent_id, editor_session_key, editor_display_after_unix_milli, save_scheduled, saved_workflow_id, saved_revision, expires_at, created_at, updated_at FROM workgraph_workflow_drafts;

CREATE TABLE IF NOT EXISTS workgraph_workflow_draft_versions_next (
    preview_id VARCHAR(64) NOT NULL,
    revision INTEGER NOT NULL,
    preview_json TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY (preview_id, revision),
    CONSTRAINT ck_workgraph_workflow_draft_versions_revision CHECK (revision > 0),
    FOREIGN KEY(preview_id) REFERENCES workgraph_workflow_drafts_next(preview_id) ON DELETE CASCADE
);

INSERT INTO workgraph_workflow_draft_versions_next SELECT * FROM workgraph_workflow_draft_versions;
DROP TABLE workgraph_workflow_draft_versions;
DROP TABLE workgraph_workflow_drafts;
ALTER TABLE workgraph_workflow_drafts_next RENAME TO workgraph_workflow_drafts;
ALTER TABLE workgraph_workflow_draft_versions_next RENAME TO workgraph_workflow_draft_versions;

CREATE INDEX IF NOT EXISTS idx_workgraph_workflow_drafts_owner_session_updated
    ON workgraph_workflow_drafts (owner_user_id, source_session_key, updated_at DESC, preview_id);
CREATE INDEX IF NOT EXISTS idx_workgraph_workflow_drafts_owner_editor_session
    ON workgraph_workflow_drafts (owner_user_id, editor_session_key);
