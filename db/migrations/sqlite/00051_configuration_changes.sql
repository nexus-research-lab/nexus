-- +goose Up
CREATE TABLE configuration_changes (
    request_id TEXT NOT NULL,
    owner_user_id TEXT NOT NULL,
    actor_agent_id TEXT NOT NULL,
    session_key TEXT NOT NULL DEFAULT '',
    domain TEXT NOT NULL,
    operation TEXT NOT NULL,
    target TEXT NOT NULL DEFAULT '',
    request_json TEXT NOT NULL DEFAULT '{}',
    result_json TEXT NOT NULL DEFAULT '{}',
    revision_before TEXT NOT NULL DEFAULT '',
    revision_after TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    error_message TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (owner_user_id, request_id)
);

CREATE INDEX idx_configuration_changes_owner_created
    ON configuration_changes(owner_user_id, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_configuration_changes_owner_created;
DROP TABLE IF EXISTS configuration_changes;
