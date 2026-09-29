-- +goose Up
-- 每个 owner/Room 只保存当前摘要；来源 conversation 删除后摘要一起删除。
CREATE TABLE room_reply_previews (
 owner_user_id TEXT NOT NULL,
 room_id VARCHAR(64) NOT NULL REFERENCES rooms(id) ON DELETE CASCADE,
 conversation_id VARCHAR(64) NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
 session_key TEXT NOT NULL,
 message_id TEXT NOT NULL,
 preview TEXT NOT NULL,
 reply_timestamp BIGINT NOT NULL,
 invalidated_before BIGINT NOT NULL DEFAULT 0,
 PRIMARY KEY (owner_user_id, room_id)
);
CREATE INDEX ix_room_reply_preview_source ON room_reply_previews(owner_user_id, session_key);

-- +goose Down
DROP TABLE room_reply_previews;
