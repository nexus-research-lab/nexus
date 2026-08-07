// INPUT: 已鉴权 owner、Room Agent member 与目标参与状态。
// OUTPUT: 原子持久化 participation_paused 后的主 conversation 上下文。
// POS: Room member 参与闸门的跨方言持久化入口。
package roomrepo

import (
	"context"
	"errors"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// SetRoomMemberParticipation 持久化 Room Agent 的参与暂停状态。
func (r *SQLRepository) SetRoomMemberParticipation(
	ctx context.Context,
	ownerUserID string,
	roomID string,
	agentID string,
	paused bool,
) (*protocol.ConversationContextAggregate, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	roomAggregate, err := r.getRoomAggregate(ctx, tx, ownerUserID, roomID)
	if err != nil || roomAggregate == nil {
		return nil, err
	}
	if roomAggregate.Room.RoomType != protocol.RoomTypeGroup {
		return nil, errors.New("DM room does not support member participation controls")
	}
	result, err := tx.ExecContext(ctx, `
UPDATE members
SET participation_paused = `+r.dialect.Bind(1)+`
WHERE room_id = `+r.dialect.Bind(2)+`
  AND member_type = 'agent'
  AND member_agent_id = `+r.dialect.Bind(3), paused, roomID, agentID)
	if err != nil {
		return nil, err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rowsAffected == 0 {
		return nil, nil
	}

	conversations, err := r.listConversations(ctx, tx, roomID)
	if err != nil {
		return nil, err
	}
	mainConversation := PickMainConversation(conversations)
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	if mainConversation == nil {
		return nil, nil
	}
	return r.getContextByConversation(ctx, ownerUserID, roomID, mainConversation.ID)
}
