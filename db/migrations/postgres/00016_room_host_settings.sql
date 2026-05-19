-- +goose Up
ALTER TABLE rooms ADD COLUMN IF NOT EXISTS host_agent_id VARCHAR(64);
ALTER TABLE rooms ADD COLUMN IF NOT EXISTS host_auto_reply_enabled BOOLEAN NOT NULL DEFAULT FALSE;
CREATE INDEX IF NOT EXISTS idx_rooms_host_agent ON rooms (host_agent_id);

-- +goose Down
DROP INDEX IF EXISTS idx_rooms_host_agent;
ALTER TABLE rooms DROP COLUMN IF EXISTS host_auto_reply_enabled;
ALTER TABLE rooms DROP COLUMN IF EXISTS host_agent_id;
