-- +goose Up

-- Compatibility repair: local branches that already recorded the former
-- 00117/00118 WorkGraph migrations must still receive the upstream Echo table.
CREATE TABLE IF NOT EXISTS echo_attempts (
    attempt_id VARCHAR(64) NOT NULL PRIMARY KEY,
    owner_user_id VARCHAR(64) NOT NULL,
    agent_id VARCHAR(64) NOT NULL,
    session_key VARCHAR(255) NOT NULL,
    trigger_kind VARCHAR(32) NOT NULL,
    anchor_round_id VARCHAR(64) NOT NULL,
    anchor_message_id VARCHAR(64),
    anchor_finished_at DATETIME NOT NULL,
    due_at DATETIME NOT NULL,
    expires_at DATETIME NOT NULL,
    status VARCHAR(32) NOT NULL,
    runtime_round_id VARCHAR(64),
    decision_reason TEXT,
    focus TEXT,
    error_code VARCHAR(64),
    delivered_message_id VARCHAR(64),
    started_at DATETIME,
    finished_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_echo_attempts_status CHECK (status IN ('scheduled', 'evaluating', 'running', 'committing', 'delivered', 'suppressed', 'cancelled', 'failed')),
    CONSTRAINT uq_echo_attempt_anchor UNIQUE (owner_user_id, session_key, trigger_kind, anchor_round_id)
);
CREATE INDEX IF NOT EXISTS idx_echo_attempts_due ON echo_attempts (status, due_at);
CREATE INDEX IF NOT EXISTS idx_echo_attempts_session ON echo_attempts (owner_user_id, session_key, created_at);
CREATE INDEX IF NOT EXISTS idx_echo_attempts_agent ON echo_attempts (owner_user_id, agent_id, created_at);

CREATE TABLE IF NOT EXISTS workgraph_workflow_drafts (
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
CREATE INDEX IF NOT EXISTS idx_workgraph_workflow_drafts_owner_session_updated
    ON workgraph_workflow_drafts (owner_user_id, source_session_key, updated_at DESC, preview_id);
CREATE INDEX IF NOT EXISTS idx_workgraph_workflow_drafts_owner_editor_session
    ON workgraph_workflow_drafts (owner_user_id, editor_session_key);

CREATE TABLE IF NOT EXISTS workgraph_workflow_draft_versions (
    preview_id VARCHAR(64) NOT NULL,
    revision INTEGER NOT NULL,
    preview_json TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY (preview_id, revision),
    CONSTRAINT ck_workgraph_workflow_draft_versions_revision CHECK (revision > 0),
    FOREIGN KEY(preview_id) REFERENCES workgraph_workflow_drafts(preview_id) ON DELETE CASCADE
);

-- +goose Down

DROP TABLE IF EXISTS workgraph_workflow_draft_versions;
DROP TABLE IF EXISTS workgraph_workflow_drafts;
