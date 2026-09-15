-- +goose Up
ALTER TABLE team_relay_messages ADD COLUMN author_agent_id TEXT NOT NULL DEFAULT '';
ALTER TABLE team_relay_messages ADD COLUMN delivery_id TEXT NOT NULL DEFAULT '';
ALTER TABLE team_relay_messages ADD COLUMN output_kind TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE team_relay_messages DROP COLUMN output_kind;
ALTER TABLE team_relay_messages DROP COLUMN delivery_id;
ALTER TABLE team_relay_messages DROP COLUMN author_agent_id;
