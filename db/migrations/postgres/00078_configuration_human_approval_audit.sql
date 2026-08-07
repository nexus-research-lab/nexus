-- +goose Up
-- Persist non-secret evidence for destructive conversational configuration approval.
ALTER TABLE configuration_changes ADD COLUMN human_approval_request_id TEXT NOT NULL DEFAULT '';
ALTER TABLE configuration_changes ADD COLUMN human_principal_user_id TEXT NOT NULL DEFAULT '';
ALTER TABLE configuration_changes ADD COLUMN human_principal_role TEXT NOT NULL DEFAULT '';
ALTER TABLE configuration_changes ADD COLUMN human_auth_method TEXT NOT NULL DEFAULT '';
ALTER TABLE configuration_changes ADD COLUMN human_approved_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE configuration_changes DROP COLUMN human_approved_at;
ALTER TABLE configuration_changes DROP COLUMN human_auth_method;
ALTER TABLE configuration_changes DROP COLUMN human_principal_role;
ALTER TABLE configuration_changes DROP COLUMN human_principal_user_id;
ALTER TABLE configuration_changes DROP COLUMN human_approval_request_id;
