// INPUT: 已鉴权 owner、Room Agent member、目标参与状态与可选 configuration_version。
// OUTPUT: CAS 推进 Room 配置版本/权限世代并持久化 participation_paused 后的主 conversation 上下文。
// POS: Room member 参与闸门的跨方言持久化入口。
package roomrepo

import (
	"context"
	"database/sql"
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
	return r.setRoomMemberParticipation(ctx, ownerUserID, roomID, agentID, paused, nil)
}

// SetRoomMemberParticipationAtVersion 使用 Room configuration_version CAS
// 持久化成员参与状态。
func (r *SQLRepository) SetRoomMemberParticipationAtVersion(
	ctx context.Context,
	ownerUserID string,
	roomID string,
	agentID string,
	paused bool,
	expectedVersion int64,
) (*protocol.ConversationContextAggregate, error) {
	if expectedVersion < 1 {
		return nil, errors.New("expected Room configuration_version 必须大于 0")
	}
	return r.setRoomMemberParticipation(
		ctx,
		ownerUserID,
		roomID,
		agentID,
		paused,
		&expectedVersion,
	)
}

func (r *SQLRepository) setRoomMemberParticipation(
	ctx context.Context,
	ownerUserID string,
	roomID string,
	agentID string,
	paused bool,
	expectedVersion *int64,
) (*protocol.ConversationContextAggregate, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	roomValue, err := r.lockRoomForMutation(ctx, tx, ownerUserID, roomID)
	if err != nil || roomValue == nil {
		return nil, err
	}
	if err = validateExpectedConfigurationVersion(*roomValue, expectedVersion); err != nil {
		return nil, err
	}
	if roomValue.RoomType != protocol.RoomTypeGroup {
		return nil, errors.New("DM room does not support member participation controls")
	}
	var currentPaused bool
	err = tx.QueryRowContext(ctx, `
SELECT participation_paused
FROM members
WHERE room_id = `+r.dialect.Bind(1)+`
  AND member_type = 'agent'
  AND member_agent_id = `+r.dialect.Bind(2), roomID, agentID).Scan(&currentPaused)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
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
	versionQuery := `
UPDATE rooms
SET configuration_version = configuration_version + 1,
    updated_at = ` + r.dialect.CurrentTimestamp()
	if currentPaused != paused {
		versionQuery += `,
    authority_epoch = authority_epoch + 1`
	}
	versionQuery += `
WHERE id = ` + r.dialect.Bind(1) + ` AND owner_user_id = ` + r.dialect.Bind(2)
	result, err = tx.ExecContext(ctx, versionQuery, roomID, ownerUserID)
	if err != nil {
		return nil, err
	}
	rowsAffected, err = result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if rowsAffected != 1 {
		return nil, errors.New("Room 成员参与状态版本推进失败")
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
