// INPUT: owner-scoped Room/conversation 删除意图。
// OUTPUT: Room-first 锁顺序下完成的依赖清理、删除结果与回退 conversation。
// POS: Room 删除事务编排；取得 Room 锁后才允许触碰 rounds/messages/sessions/conversations。
package roomrepo

import (
	"context"
	"database/sql"
	"errors"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// DeleteConversation 删除对话并返回回退上下文。
func (r *SQLRepository) DeleteConversation(ctx context.Context, ownerUserID string, roomID string, conversationID string) (*protocol.ConversationContextAggregate, error) {
	return r.deleteConversation(ctx, ownerUserID, roomID, conversationID, nil)
}

// DeleteConversationAtVersion 仅在 Room configuration_version 匹配时删除对话。
func (r *SQLRepository) DeleteConversationAtVersion(
	ctx context.Context,
	ownerUserID string,
	roomID string,
	conversationID string,
	expectedVersion int64,
) (*protocol.ConversationContextAggregate, error) {
	return r.deleteConversation(
		ctx,
		ownerUserID,
		roomID,
		conversationID,
		&expectedVersion,
	)
}

func (r *SQLRepository) deleteConversation(
	ctx context.Context,
	ownerUserID string,
	roomID string,
	conversationID string,
	expectedVersion *int64,
) (*protocol.ConversationContextAggregate, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	deletion := conversationDeletion{
		repository:      r,
		ctx:             ctx,
		tx:              tx,
		ownerUserID:     ownerUserID,
		roomID:          roomID,
		conversationID:  conversationID,
		expectedVersion: expectedVersion,
	}
	return deletion.run()
}

type conversationDeletion struct {
	repository      *SQLRepository
	ctx             context.Context
	tx              *sql.Tx
	ownerUserID     string
	roomID          string
	conversationID  string
	expectedVersion *int64
	fallbackID      string
	promotionType   string
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
	roomValue, err := d.repository.lockRoomForMutation(d.ctx, d.tx, d.ownerUserID, d.roomID)
	if err != nil || roomValue == nil {
		return false, err
	}
	if err = validateExpectedConfigurationVersion(*roomValue, d.expectedVersion); err != nil {
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
	d.promotionType = plan.promotionType
	return true, nil
}

type conversationDeletionPlan struct {
	targetFound   bool
	fallbackID    string
	promotionType string
}

func planConversationDeletion(conversations []protocol.ConversationRecord, targetID string) (conversationDeletionPlan, error) {
	if len(conversations) <= 1 {
		return conversationDeletionPlan{}, errors.New("room 至少保留一个对话")
	}
	var target protocol.ConversationRecord
	for _, conversation := range conversations {
		if conversation.ID == targetID {
			target = conversation
			break
		}
	}
	if target.ID == "" {
		return conversationDeletionPlan{}, nil
	}

	fallback := firstFallbackConversation(conversations, targetID)
	plan := conversationDeletionPlan{
		targetFound: true,
		fallbackID:  fallback.ID,
	}
	if isPrimaryConversation(target) && !isPrimaryConversation(fallback) {
		plan.promotionType = target.ConversationType
	}
	return plan, nil
}

func isPrimaryConversation(conversation protocol.ConversationRecord) bool {
	return conversation.ConversationType == protocol.ConversationTypeMain ||
		conversation.ConversationType == protocol.ConversationTypeDM
}

func firstFallbackConversation(conversations []protocol.ConversationRecord, targetID string) protocol.ConversationRecord {
	var fallback protocol.ConversationRecord
	for _, conversation := range conversations {
		if conversation.ID == targetID {
			continue
		}
		if fallback.ID == "" {
			fallback = conversation
		}
		if isPrimaryConversation(conversation) {
			return conversation
		}
	}
	return fallback
}

func (d *conversationDeletion) delete() (bool, error) {
	if d.promotionType != "" {
		result, err := d.tx.ExecContext(
			d.ctx,
			`UPDATE conversations
SET conversation_type = `+d.repository.dialect.Bind(1)+`, updated_at = `+d.repository.dialect.CurrentTimestamp()+`
WHERE id = `+d.repository.dialect.Bind(2)+` AND room_id = `+d.repository.dialect.Bind(3),
			d.promotionType,
			d.fallbackID,
			d.roomID,
		)
		if err != nil {
			return false, err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return false, err
		}
		if affected != 1 {
			return false, errors.New("回退对话提升失败")
		}
	}
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
	if err != nil || affected == 0 {
		return false, err
	}
	if err = d.repository.advanceRoomConfigurationVersion(
		d.ctx,
		d.tx,
		d.ownerUserID,
		d.roomID,
	); err != nil {
		return false, err
	}
	return true, nil
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
