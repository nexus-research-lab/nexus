-- +goose Up
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS last_activity_at TIMESTAMP WITHOUT TIME ZONE DEFAULT now() NOT NULL;
UPDATE conversations
SET last_activity_at = COALESCE(last_activity_at, updated_at, created_at, now())
WHERE last_activity_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_conversations_room_activity ON conversations (room_id, last_activity_at DESC, created_at DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_conversations_room_activity;
ALTER TABLE conversations DROP COLUMN IF EXISTS last_activity_at;
