package roomrepo

import (
	"context"
	"database/sql"
)

func (r *SQLRepository) deleteRoomDependents(ctx context.Context, tx *sql.Tx, roomID string) error {
	if _, err := tx.ExecContext(ctx, `
DELETE FROM rounds
WHERE session_id IN (
    SELECT s.id FROM sessions s
    JOIN conversations c ON c.id = s.conversation_id
    WHERE c.room_id = `+r.dialect.Bind(1)+`
)
OR trigger_message_id IN (
    SELECT m.id FROM messages m
    JOIN conversations c ON c.id = m.conversation_id
    WHERE c.room_id = `+r.dialect.Bind(2)+`
)`, roomID, roomID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
DELETE FROM messages
WHERE conversation_id IN (SELECT id FROM conversations WHERE room_id = `+r.dialect.Bind(1)+`)`, roomID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
DELETE FROM sessions
WHERE conversation_id IN (SELECT id FROM conversations WHERE room_id = `+r.dialect.Bind(1)+`)`, roomID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM conversations WHERE room_id = `+r.dialect.Bind(1), roomID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `DELETE FROM members WHERE room_id = `+r.dialect.Bind(1), roomID)
	return err
}

func (r *SQLRepository) deleteConversationDependents(ctx context.Context, tx *sql.Tx, conversationID string) error {
	if _, err := tx.ExecContext(ctx, `
DELETE FROM rounds
WHERE session_id IN (SELECT id FROM sessions WHERE conversation_id = `+r.dialect.Bind(1)+`)
OR trigger_message_id IN (SELECT id FROM messages WHERE conversation_id = `+r.dialect.Bind(2)+`)`, conversationID, conversationID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM messages WHERE conversation_id = `+r.dialect.Bind(1), conversationID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE conversation_id = `+r.dialect.Bind(1), conversationID)
	return err
}

func (r *SQLRepository) deleteRoomAgentSessionDependents(ctx context.Context, tx *sql.Tx, roomID string, agentID string) error {
	if _, err := tx.ExecContext(ctx, `
DELETE FROM rounds
WHERE session_id IN (
    SELECT s.id FROM sessions s
    JOIN conversations c ON c.id = s.conversation_id
    WHERE c.room_id = `+r.dialect.Bind(1)+` AND s.agent_id = `+r.dialect.Bind(2)+`
)`, roomID, agentID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE messages
SET session_id = NULL
WHERE session_id IN (
    SELECT s.id FROM sessions s
    JOIN conversations c ON c.id = s.conversation_id
    WHERE c.room_id = `+r.dialect.Bind(1)+` AND s.agent_id = `+r.dialect.Bind(2)+`
)`, roomID, agentID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
DELETE FROM sessions
WHERE conversation_id IN (SELECT id FROM conversations WHERE room_id = `+r.dialect.Bind(1)+`)
  AND agent_id = `+r.dialect.Bind(2), roomID, agentID)
	return err
}
