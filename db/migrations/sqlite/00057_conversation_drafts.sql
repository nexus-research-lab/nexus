-- +goose Up

ALTER TABLE conversations
ADD COLUMN is_draft BOOLEAN NOT NULL DEFAULT FALSE;

-- Existing conversations stay non-drafts. Canonical Room input history is
-- file-backed, so SQL migration data cannot safely infer legacy draft state.
CREATE UNIQUE INDEX uq_conversations_room_draft
ON conversations (room_id)
WHERE is_draft = TRUE;

-- +goose Down

DROP INDEX IF EXISTS uq_conversations_room_draft;
ALTER TABLE conversations DROP COLUMN is_draft;
