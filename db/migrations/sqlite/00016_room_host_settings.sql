-- +goose Up
ALTER TABLE rooms ADD COLUMN host_agent_id VARCHAR(64);
ALTER TABLE rooms ADD COLUMN host_auto_reply_enabled BOOLEAN NOT NULL DEFAULT FALSE;
CREATE INDEX idx_rooms_host_agent ON rooms (host_agent_id);

-- +goose Down
DROP INDEX idx_rooms_host_agent;
ALTER TABLE rooms DROP COLUMN host_auto_reply_enabled;
ALTER TABLE rooms DROP COLUMN host_agent_id;
