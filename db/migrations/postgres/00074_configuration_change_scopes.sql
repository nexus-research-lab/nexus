-- +goose Up
-- Bind every audit record to its trusted conversation and resource scope.
ALTER TABLE configuration_changes ADD COLUMN context_kind TEXT NOT NULL DEFAULT '';
ALTER TABLE configuration_changes ADD COLUMN context_id TEXT NOT NULL DEFAULT '';
ALTER TABLE configuration_changes ADD COLUMN scope_kind TEXT NOT NULL DEFAULT 'owner';
ALTER TABLE configuration_changes ADD COLUMN scope_id TEXT NOT NULL DEFAULT '';
ALTER TABLE configuration_changes ADD COLUMN authority TEXT NOT NULL DEFAULT 'owner_main';
ALTER TABLE configuration_changes ADD COLUMN intent_digest TEXT NOT NULL DEFAULT '';

CREATE INDEX idx_configuration_changes_scope_created
    ON configuration_changes(owner_user_id, scope_kind, scope_id, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_configuration_changes_scope_created;
ALTER TABLE configuration_changes DROP COLUMN intent_digest;
ALTER TABLE configuration_changes DROP COLUMN authority;
ALTER TABLE configuration_changes DROP COLUMN scope_id;
ALTER TABLE configuration_changes DROP COLUMN scope_kind;
ALTER TABLE configuration_changes DROP COLUMN context_id;
ALTER TABLE configuration_changes DROP COLUMN context_kind;
