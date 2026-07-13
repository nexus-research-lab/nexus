package roomrepo

import (
	"context"
	"database/sql"
	"errors"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// DeleteConversation 删除话题并返回回退上下文。
func (r *SQLRepository) DeleteConversation(ctx context.Context, ownerUserID string, roomID string, conversationID string) (*protocol.ConversationContextAggregate, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	deletion := conversationDeletion{
		repository:     r,
		ctx:            ctx,
		tx:             tx,
		ownerUserID:    ownerUserID,
		roomID:         roomID,
		conversationID: conversationID,
	}
	return deletion.run()
}

type conversationDeletion struct {
	repository     *SQLRepository
	ctx            context.Context
	tx             *sql.Tx
	ownerUserID    string
	roomID         string
	conversationID string
	fallbackID     string
}

func (d *conversationDeletion) run() (*protocol.ConversationContextAggregate, error) {
	ready, err := d.prepare()
	if err != nil || !ready {
		return nil, err
	}
	deleted, err := d.delete()
	if err != nil || !deleted {
		return nil, err
	}
	if err = d.tx.Commit(); err != nil {
		return nil, err
	}
	if d.fallbackID == "" {
		return nil, nil
	}
	return d.repository.getContextByConversation(d.ctx, d.ownerUserID, d.roomID, d.fallbackID)
}

func (d *conversationDeletion) prepare() (bool, error) {
	roomValue, err := d.repository.loadRoom(d.ctx, d.tx, d.ownerUserID, d.roomID)
	if err != nil || roomValue == nil {
		return false, err
	}
	conversations, err := d.repository.listConversations(d.ctx, d.tx, d.roomID)
	if err != nil {
		return false, err
	}
	plan, err := planConversationDeletion(conversations, d.conversationID)
	if err != nil || !plan.targetFound {
		return false, err
	}
	d.fallbackID = plan.fallbackID
	return true, nil
}

type conversationDeletionPlan struct {
	targetFound bool
	targetTopic bool
	fallbackID  string
}

func planConversationDeletion(conversations []protocol.ConversationRecord, targetID string) (conversationDeletionPlan, error) {
	if len(conversations) <= 1 {
		return conversationDeletionPlan{}, errors.New("room 至少保留一个对话")
	}
	plan := conversationDeletionPlan{}
	for _, conversation := range conversations {
		plan.inspect(conversation, targetID)
	}
	if !plan.targetFound {
		return plan, nil
	}
	if !plan.targetTopic {
		return conversationDeletionPlan{}, errors.New("主对话不支持删除")
	}
	if plan.fallbackID == "" {
		plan.fallbackID = firstOtherConversationID(conversations, targetID)
	}
	return plan, nil
}

func (p *conversationDeletionPlan) inspect(conversation protocol.ConversationRecord, targetID string) {
	if conversation.ID == targetID {
		p.targetFound = true
		p.targetTopic = conversation.ConversationType == protocol.ConversationTypeTopic
		return
	}
	if p.fallbackID == "" && isPrimaryConversation(conversation) {
		p.fallbackID = conversation.ID
	}
}

func isPrimaryConversation(conversation protocol.ConversationRecord) bool {
	return conversation.ConversationType == protocol.ConversationTypeMain ||
		conversation.ConversationType == protocol.ConversationTypeDM
}

func firstOtherConversationID(conversations []protocol.ConversationRecord, targetID string) string {
	for _, conversation := range conversations {
		if conversation.ID != targetID {
			return conversation.ID
		}
	}
	return ""
}

func (d *conversationDeletion) delete() (bool, error) {
	if err := d.repository.deleteConversationDependents(d.ctx, d.tx, d.conversationID); err != nil {
		return false, err
	}
	result, err := d.tx.ExecContext(
		d.ctx,
		`DELETE FROM conversations WHERE id = `+d.repository.dialect.Bind(1)+` AND room_id = `+d.repository.dialect.Bind(2),
		d.conversationID,
		d.roomID,
	)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}

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
