// INPUT: Room 或其 conversation/session 子资源标识、owner scope 与可选期望版本。
// OUTPUT: 已在当前事务中取得 mutation 锁的 Room 行及稳定的版本/host 不变量校验。
// POS: 所有 Room 持久化 mutation 的首个写锁入口；锁顺序固定为 Room 后再子资源。
package roomrepo

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

// lockRoomForMutation 通过无语义 UPDATE 在 SQLite/PostgreSQL 上统一先取得 Room 写锁。
func (r *SQLRepository) lockRoomForMutation(
	ctx context.Context,
	tx *sql.Tx,
	ownerUserID string,
	roomID string,
) (*protocol.RoomRecord, error) {
	if err := storage.LockRoomForMutation(ctx, tx, r.dialect, ownerUserID, roomID); err != nil {
		return nil, err
	}
	return r.loadRoom(ctx, tx, ownerUserID, roomID)
}

func (r *SQLRepository) getLockedRoomAggregate(
	ctx context.Context,
	tx *sql.Tx,
	ownerUserID string,
	roomID string,
) (*protocol.RoomAggregate, error) {
	roomValue, err := r.lockRoomForMutation(ctx, tx, ownerUserID, roomID)
	if err != nil || roomValue == nil {
		return nil, err
	}
	members, err := r.listMembers(ctx, tx, roomID)
	if err != nil {
		return nil, err
	}
	return &protocol.RoomAggregate{
		Room:    *roomValue,
		Members: members,
	}, nil
}

func validateExpectedConfigurationVersion(roomValue protocol.RoomRecord, expectedVersion *int64) error {
	if expectedVersion == nil || roomValue.ConfigurationVersion == *expectedVersion {
		return nil
	}
	return ErrConfigurationVersionConflict
}

func (r *SQLRepository) advanceRoomConfigurationVersion(
	ctx context.Context,
	tx *sql.Tx,
	ownerUserID string,
	roomID string,
) error {
	result, err := tx.ExecContext(ctx, `
UPDATE rooms
SET configuration_version = configuration_version + 1,
    updated_at = `+r.dialect.CurrentTimestamp()+`
WHERE id = `+r.dialect.Bind(1)+` AND owner_user_id = `+r.dialect.Bind(2),
		roomID,
		ownerUserID,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return errors.New("room configuration version advance failed")
	}
	return nil
}

func (r *SQLRepository) validateLockedRoomHostPatch(
	ctx context.Context,
	tx *sql.Tx,
	roomValue protocol.RoomRecord,
	patch UpdateRoomPatch,
) error {
	nextHostAgentID := roomValue.HostAgentID
	if patch.HostAgentID != nil {
		nextHostAgentID = strings.TrimSpace(*patch.HostAgentID)
	}
	nextAutoReplyEnabled := roomValue.HostAutoReplyEnabled
	if patch.HostAutoReplyEnabled != nil {
		nextAutoReplyEnabled = *patch.HostAutoReplyEnabled
	}
	if roomValue.RoomType == protocol.RoomTypeDM && nextHostAgentID != "" {
		return errors.New("DM room 不支持设置群主")
	}
	if nextHostAgentID == "" {
		if nextAutoReplyEnabled {
			return errors.New("启用群主接管前必须设置群主")
		}
		return nil
	}

	var isMember bool
	err := tx.QueryRowContext(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM members
    WHERE room_id = `+r.dialect.Bind(1)+`
      AND member_type = 'agent'
      AND member_agent_id = `+r.dialect.Bind(2)+`
)`, roomValue.ID, nextHostAgentID).Scan(&isMember)
	if err != nil {
		return err
	}
	if !isMember {
		return errors.New("群主必须是当前 Room 的 Agent 成员")
	}
	return nil
}

func (r *SQLRepository) lookupConversationRoomID(
	ctx context.Context,
	tx *sql.Tx,
	conversationID string,
) (string, error) {
	var roomID string
	err := tx.QueryRowContext(ctx, `
SELECT room_id
FROM conversations
WHERE id = `+r.dialect.Bind(1), conversationID).Scan(&roomID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return roomID, err
}

func (r *SQLRepository) lookupSessionRoomID(
	ctx context.Context,
	tx *sql.Tx,
	sessionID string,
) (string, error) {
	var roomID string
	err := tx.QueryRowContext(ctx, `
SELECT c.room_id
FROM sessions s
JOIN conversations c ON c.id = s.conversation_id
WHERE s.id = `+r.dialect.Bind(1), sessionID).Scan(&roomID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return roomID, err
}
