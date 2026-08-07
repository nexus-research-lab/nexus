-- +goose Up
-- Preserve both the business Room/DM identity and the runtime execution lease
-- that authorized each conversational configuration mutation.
ALTER TABLE configuration_changes ADD COLUMN round_id TEXT NOT NULL DEFAULT '';
ALTER TABLE configuration_changes ADD COLUMN lease_session_key TEXT NOT NULL DEFAULT '';
ALTER TABLE configuration_changes ADD COLUMN lease_round_id TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE configuration_changes DROP COLUMN lease_round_id;
ALTER TABLE configuration_changes DROP COLUMN lease_session_key;
ALTER TABLE configuration_changes DROP COLUMN round_id;
