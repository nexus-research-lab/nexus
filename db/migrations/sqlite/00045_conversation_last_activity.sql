-- +goose Up
ALTER TABLE conversations ADD COLUMN last_activity_at DATETIME;
UPDATE conversations
SET last_activity_at = COALESCE(updated_at, created_at, CURRENT_TIMESTAMP)
WHERE last_activity_at IS NULL;
CREATE INDEX idx_conversations_room_activity ON conversations (room_id, last_activity_at DESC, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_conversations_room_activity;
ALTER TABLE conversations DROP COLUMN last_activity_at;
