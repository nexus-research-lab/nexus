package roomrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage"
	"github.com/nexus-research-lab/nexus/internal/storage/jsoncodec"
)

type roomQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// SQLRepository 提供 Room 仓储的跨方言 SQL 实现。
type SQLRepository struct {
	db      *sql.DB
	dialect storage.SQLDialect
}

// NewSQLRepository 创建 Room 仓储。
func NewSQLRepository(driver string, db *sql.DB) *SQLRepository {
	return &SQLRepository{
		db:      db,
		dialect: storage.NewSQLDialect(driver),
	}
}

// LoadAgentRuntimeRefs 读取建房所需的 Agent 运行时信息。
func (r *SQLRepository) LoadAgentRuntimeRefs(ctx context.Context, ownerUserID string, agentIDs []string) ([]AgentRuntimeRef, error) {
	if len(agentIDs) == 0 {
		return nil, nil
	}

	query := fmt.Sprintf(`
SELECT
    a.id,
    a.name,
    COALESCE(p.display_name, ''),
    COALESCE(rt.id, ''),
    a.status
FROM agents a
LEFT JOIN profiles p ON p.agent_id = a.id
LEFT JOIN runtimes rt ON rt.agent_id = a.id
WHERE a.id IN (%s)`, r.dialect.BindList(len(agentIDs)))

	args := make([]any, 0, len(agentIDs))
	for _, agentID := range agentIDs {
		args = append(args, agentID)
	}
	if ownerUserID != "" {
		query += " AND a.owner_user_id = " + r.dialect.Bind(len(args)+1)
		args = append(args, ownerUserID)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]AgentRuntimeRef, 0, len(agentIDs))
	for rows.Next() {
		var item AgentRuntimeRef
		if err = rows.Scan(
			&item.AgentID,
			&item.Name,
			&item.DisplayName,
			&item.RuntimeID,
			&item.Status,
		); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

// ListRecentRooms 列出最近房间。
func (r *SQLRepository) ListRecentRooms(ctx context.Context, ownerUserID string, limit int) ([]protocol.RoomAggregate, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id FROM rooms WHERE owner_user_id = `+r.dialect.Bind(1)+` ORDER BY updated_at DESC, created_at DESC LIMIT `+r.dialect.Bind(2),
		ownerUserID,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	roomIDs := make([]string, 0)
	for rows.Next() {
		var roomID string
		if err = rows.Scan(&roomID); err != nil {
			return nil, err
		}
		roomIDs = append(roomIDs, roomID)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if len(roomIDs) == 0 {
		return nil, nil
	}
	roomByID, err := r.loadRoomsByIDs(ctx, r.db, ownerUserID, roomIDs)
	if err != nil {
		return nil, err
	}
	membersByRoomID, err := r.listMembersByRoomIDs(ctx, r.db, roomIDs)
	if err != nil {
		return nil, err
	}
	result := make([]protocol.RoomAggregate, 0, len(roomIDs))
	for _, roomID := range roomIDs {
		roomValue, ok := roomByID[roomID]
		if !ok {
			continue
		}
		result = append(result, protocol.RoomAggregate{
			Room:    roomValue,
			Members: membersByRoomID[roomID],
		})
	}
	return result, nil
}

// GetRoom 读取单个房间。
func (r *SQLRepository) GetRoom(ctx context.Context, ownerUserID string, roomID string) (*protocol.RoomAggregate, error) {
	roomValue, err := r.loadRoom(ctx, r.db, ownerUserID, roomID)
	if err != nil {
		return nil, err
	}
	if roomValue == nil {
		return nil, nil
	}
	members, err := r.listMembers(ctx, r.db, roomID)
	if err != nil {
		return nil, err
	}
	return &protocol.RoomAggregate{
		Room:    *roomValue,
		Members: members,
	}, nil
}

// GetRoomContexts 读取房间上下文。
func (r *SQLRepository) GetRoomContexts(ctx context.Context, ownerUserID string, roomID string) ([]protocol.ConversationContextAggregate, error) {
	roomAggregate, err := r.GetRoom(ctx, ownerUserID, roomID)
	if err != nil || roomAggregate == nil {
		return nil, err
	}
	memberAgents, err := r.listMemberAgents(ctx, r.db, roomID)
	if err != nil {
		return nil, err
	}

	conversations, err := r.listConversations(ctx, r.db, roomID)
	if err != nil {
		return nil, err
	}

	conversationIDs := make([]string, 0, len(conversations))
	for _, conversation := range conversations {
		conversationIDs = append(conversationIDs, conversation.ID)
	}
	sessionsByConversation, err := r.listSessionsByConversations(ctx, r.db, conversationIDs)
	if err != nil {
		return nil, err
	}

	contexts := make([]protocol.ConversationContextAggregate, 0, len(conversations))
	for _, conversation := range conversations {
		contexts = append(contexts, protocol.ConversationContextAggregate{
			Room:         roomAggregate.Room,
			Members:      roomAggregate.Members,
			MemberAgents: memberAgents,
			Conversation: conversation,
			Sessions:     sessionsByConversation[conversation.ID],
		})
	}
	return contexts, nil
}

// GetConversationContext 按 conversation_id 读取单条房间上下文。
func (r *SQLRepository) GetConversationContext(ctx context.Context, ownerUserID string, conversationID string) (*protocol.ConversationContextAggregate, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT c.room_id
FROM conversations c
JOIN rooms r ON r.id = c.room_id
WHERE c.id = `+r.dialect.Bind(1)+` AND r.owner_user_id = `+r.dialect.Bind(2)+`
LIMIT 1`, conversationID, ownerUserID)
	var roomID string
	if err := row.Scan(&roomID); errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return r.getContextByConversation(ctx, ownerUserID, roomID, conversationID)
}

// GetConversationContextForSystem 按 conversation_id 读取内部系统续跑所需上下文。
func (r *SQLRepository) GetConversationContextForSystem(ctx context.Context, conversationID string) (*protocol.ConversationContextAggregate, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT c.room_id, r.owner_user_id
FROM conversations c
JOIN rooms r ON r.id = c.room_id
WHERE c.id = `+r.dialect.Bind(1)+`
LIMIT 1`, conversationID)
	var roomID string
	var ownerUserID string
	if err := row.Scan(&roomID, &ownerUserID); errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return r.getContextByConversation(ctx, ownerUserID, roomID, conversationID)
}

// FindDMRoomContext 查找指定 Agent 的 DM 上下文。
func (r *SQLRepository) FindDMRoomContext(ctx context.Context, ownerUserID string, agentID string) (*protocol.ConversationContextAggregate, error) {
	rows, err := r.db.QueryContext(ctx, `
SELECT r.id
FROM rooms r
WHERE r.room_type = 'dm'
  AND r.owner_user_id = `+r.dialect.Bind(1)+`
  AND EXISTS (
      SELECT 1
      FROM members m
      WHERE m.room_id = r.id
        AND m.member_type = 'agent'
        AND m.member_agent_id = `+r.dialect.Bind(2)+`
  )
  AND (
      SELECT COUNT(1)
      FROM members m
      WHERE m.room_id = r.id AND m.member_type = 'agent'
  ) = 1`, ownerUserID, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	roomIDs := make([]string, 0)
	for rows.Next() {
		var roomID string
		if err = rows.Scan(&roomID); err != nil {
			return nil, err
		}
		roomIDs = append(roomIDs, roomID)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}

	contexts := make([]protocol.ConversationContextAggregate, 0)
	for _, roomID := range roomIDs {
		roomContexts, loadErr := r.GetRoomContexts(ctx, ownerUserID, roomID)
		if loadErr != nil {
			return nil, loadErr
		}
		contexts = append(contexts, roomContexts...)
	}
	return PickLatestConversationContext(contexts), nil
}

// CreateRoom 创建房间、主对话和初始会话。
func (r *SQLRepository) CreateRoom(ctx context.Context, bundle CreateRoomBundle) (*protocol.ConversationContextAggregate, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err = tx.ExecContext(ctx, fmt.Sprintf(`
INSERT INTO rooms (id, owner_user_id, room_type, name, description, avatar, skill_names, host_agent_id, host_auto_reply_enabled, private_messages_enabled)
VALUES (%s)`, r.dialect.BindList(10)),
		bundle.Room.ID,
		bundle.Room.OwnerUserID,
		bundle.Room.RoomType,
		NullIfEmpty(bundle.Room.Name),
		bundle.Room.Description,
		NullIfEmpty(bundle.Room.Avatar),
		jsoncodec.MarshalStringSlice(bundle.Room.SkillNames),
		NullIfEmpty(bundle.Room.HostAgentID),
		bundle.Room.HostAutoReplyEnabled,
		bundle.Room.PrivateMessagesEnabled,
	); err != nil {
		return nil, err
	}

	for _, member := range bundle.Members {
		if _, err = tx.ExecContext(ctx, fmt.Sprintf(`
INSERT INTO members (id, room_id, member_type, member_user_id, member_agent_id)
VALUES (%s)`, r.dialect.BindList(5)),
			member.ID,
			member.RoomID,
			member.MemberType,
			NullIfEmpty(member.MemberUserID),
			NullIfEmpty(member.MemberAgentID),
		); err != nil {
			return nil, err
		}
	}

	if _, err = tx.ExecContext(ctx, fmt.Sprintf(`
INSERT INTO conversations (id, room_id, conversation_type, title, last_activity_at)
VALUES (%s, %s)`, r.dialect.BindList(4), r.dialect.CurrentTimestamp()),
		bundle.Conversation.ID,
		bundle.Conversation.RoomID,
		bundle.Conversation.ConversationType,
		NullIfEmpty(bundle.Conversation.Title),
	); err != nil {
		return nil, err
	}

	for _, sessionValue := range bundle.Sessions {
		if _, err = tx.ExecContext(ctx, fmt.Sprintf(`
INSERT INTO sessions (
    id, conversation_id, agent_id, runtime_id, version_no, branch_key,
    is_primary, sdk_session_id, status
) VALUES (%s)`, r.dialect.BindList(9)),
			sessionValue.ID,
			sessionValue.ConversationID,
			sessionValue.AgentID,
			sessionValue.RuntimeID,
			sessionValue.VersionNo,
			sessionValue.BranchKey,
			sessionValue.IsPrimary,
			NullIfEmpty(sessionValue.SDKSessionID),
			sessionValue.Status,
		); err != nil {
			return nil, err
		}
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r.getContextByConversation(ctx, bundle.Room.OwnerUserID, bundle.Room.ID, bundle.Conversation.ID)
}

// UpdateRoom 更新房间及主对话标题。
func (r *SQLRepository) UpdateRoom(
	ctx context.Context,
	ownerUserID string,
	roomID string,
	patch UpdateRoomPatch,
) (*protocol.ConversationContextAggregate, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	roomValue, err := r.loadRoom(ctx, tx, ownerUserID, roomID)
	if err != nil || roomValue == nil {
		return nil, err
	}
	for _, update := range patch.RoomColumnUpdates() {
		query := fmt.Sprintf(`UPDATE rooms SET %s = %s, updated_at = %s WHERE id = %s AND owner_user_id = %s`,
			update.Column,
			r.dialect.Bind(1),
			r.dialect.CurrentTimestamp(),
			r.dialect.Bind(2),
			r.dialect.Bind(3),
		)
		if _, err = tx.ExecContext(ctx, query, update.Value, roomID, ownerUserID); err != nil {
			return nil, err
		}
	}

	mainConversation, err := r.pickMainConversation(ctx, tx, roomID)
	if err != nil {
		return nil, err
	}
	if mainConversation != nil && patch.Title != nil {
		if _, err = tx.ExecContext(ctx, `UPDATE conversations SET title = `+r.dialect.Bind(1)+`, updated_at = `+r.dialect.CurrentTimestamp()+` WHERE id = `+r.dialect.Bind(2), NullIfEmpty(*patch.Title), mainConversation.ID); err != nil {
			return nil, err
		}
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}
	if mainConversation == nil {
		contexts, getErr := r.GetRoomContexts(ctx, ownerUserID, roomID)
		if getErr != nil || len(contexts) == 0 {
			return nil, getErr
		}
		return &contexts[0], nil
	}
	return r.getContextByConversation(ctx, ownerUserID, roomID, mainConversation.ID)
}

// AddRoomMember 向房间追加成员。
func (r *SQLRepository) AddRoomMember(ctx context.Context, ownerUserID string, roomID string, agent AgentRuntimeRef) (*protocol.ConversationContextAggregate, error) {
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
		return nil, errors.New("DM room does not support adding members")
	}
	for _, member := range roomAggregate.Members {
		if member.MemberType == protocol.MemberTypeAgent && member.MemberAgentID == agent.AgentID {
			return nil, errors.New("Agent already exists in room")
		}
	}

	if _, err = tx.ExecContext(ctx, `
INSERT INTO members (id, room_id, member_type, member_user_id, member_agent_id)
VALUES (`+r.dialect.Bind(1)+`, `+r.dialect.Bind(2)+`, 'agent', NULL, `+r.dialect.Bind(3)+`)`,
		NewEntityID(),
		roomID,
		agent.AgentID,
	); err != nil {
		return nil, err
	}

	conversations, err := r.listConversations(ctx, tx, roomID)
	if err != nil {
		return nil, err
	}
	for _, conversation := range conversations {
		if _, err = tx.ExecContext(ctx, `
	`+r.dialect.InsertIgnoreInto("sessions")+` (
	    id, conversation_id, agent_id, runtime_id, version_no, branch_key,
	    is_primary, sdk_session_id, status
	) VALUES (`+r.dialect.Bind(1)+`, `+r.dialect.Bind(2)+`, `+r.dialect.Bind(3)+`, `+r.dialect.Bind(4)+`, 1, 'main', `+r.dialect.TrueValue()+`, NULL, 'active')`+r.dialect.InsertIgnoreSuffix(),
			NewEntityID(),
			conversation.ID,
			agent.AgentID,
			agent.RuntimeID,
		); err != nil {
			return nil, err
		}
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

// RemoveRoomMember 从房间移除成员。
func (r *SQLRepository) RemoveRoomMember(ctx context.Context, ownerUserID string, roomID string, agentID string) (*protocol.ConversationContextAggregate, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	roomAggregate, err := r.getRoomAggregate(ctx, tx, ownerUserID, roomID)
	if err != nil || roomAggregate == nil {
		return nil, err
	}
	removable, err := validateRoomAgentRemoval(*roomAggregate, agentID)
	if err != nil || !removable {
		return nil, err
	}
	if err = r.removeRoomAgent(ctx, tx, ownerUserID, *roomAggregate, agentID); err != nil {
		return nil, err
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

func validateRoomAgentRemoval(room protocol.RoomAggregate, agentID string) (bool, error) {
	if room.Room.RoomType != protocol.RoomTypeGroup {
		return false, errors.New("DM room does not support removing members")
	}
	agentIDs := make(map[string]struct{}, len(room.Members))
	for _, member := range room.Members {
		if member.MemberType == protocol.MemberTypeAgent && member.MemberAgentID != "" {
			agentIDs[member.MemberAgentID] = struct{}{}
		}
	}
	if _, exists := agentIDs[agentID]; !exists {
		return false, nil
	}
	if len(agentIDs) == 1 {
		return false, errors.New("Room 至少保留一个 agent 成员")
	}
	return true, nil
}

func (r *SQLRepository) removeRoomAgent(
	ctx context.Context,
	tx *sql.Tx,
	ownerUserID string,
	room protocol.RoomAggregate,
	agentID string,
) error {
	if _, err := tx.ExecContext(ctx, `
DELETE FROM members
WHERE room_id = `+r.dialect.Bind(1)+` AND member_type = 'agent' AND member_agent_id = `+r.dialect.Bind(2),
		room.Room.ID,
		agentID,
	); err != nil {
		return err
	}
	if room.Room.HostAgentID == agentID {
		if _, err := tx.ExecContext(ctx, `
UPDATE rooms
SET host_agent_id = NULL,
    host_auto_reply_enabled = `+r.dialect.FalseValue()+`,
    updated_at = `+r.dialect.CurrentTimestamp()+`
WHERE id = `+r.dialect.Bind(1)+` AND owner_user_id = `+r.dialect.Bind(2), room.Room.ID, ownerUserID); err != nil {
			return err
		}
	}
	return r.deleteRoomAgentSessionDependents(ctx, tx, room.Room.ID, agentID)
}

// DeleteRoom 删除房间。
func (r *SQLRepository) DeleteRoom(ctx context.Context, ownerUserID string, roomID string) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	roomValue, err := r.loadRoom(ctx, tx, ownerUserID, roomID)
	if err != nil || roomValue == nil {
		return false, err
	}
	if err = r.deleteRoomDependents(ctx, tx, roomID); err != nil {
		return false, err
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM rooms WHERE id = `+r.dialect.Bind(1)+` AND owner_user_id = `+r.dialect.Bind(2), roomID, ownerUserID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	return affected > 0, nil
}

// CreateConversation 创建房间话题。
func (r *SQLRepository) CreateConversation(ctx context.Context, bundle CreateConversationBundle) (*protocol.ConversationContextAggregate, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	ownerUserID, err := r.lookupRoomOwnerUserID(ctx, tx, bundle.RoomID)
	if err != nil {
		return nil, err
	}
	if ownerUserID == "" {
		return nil, nil
	}

	if _, err = tx.ExecContext(ctx, fmt.Sprintf(`
INSERT INTO conversations (id, room_id, conversation_type, title, last_activity_at)
VALUES (%s, %s)`, r.dialect.BindList(4), r.dialect.CurrentTimestamp()),
		bundle.Conversation.ID,
		bundle.Conversation.RoomID,
		bundle.Conversation.ConversationType,
		NullIfEmpty(bundle.Conversation.Title),
	); err != nil {
		return nil, err
	}
	for _, sessionValue := range bundle.Sessions {
		if _, err = tx.ExecContext(ctx, fmt.Sprintf(`
INSERT INTO sessions (
    id, conversation_id, agent_id, runtime_id, version_no, branch_key,
    is_primary, sdk_session_id, status
) VALUES (%s)`, r.dialect.BindList(9)),
			sessionValue.ID,
			sessionValue.ConversationID,
			sessionValue.AgentID,
			sessionValue.RuntimeID,
			sessionValue.VersionNo,
			sessionValue.BranchKey,
			sessionValue.IsPrimary,
			NullIfEmpty(sessionValue.SDKSessionID),
			sessionValue.Status,
		); err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r.getContextByConversation(ctx, ownerUserID, bundle.RoomID, bundle.Conversation.ID)
}

// UpdateConversation 更新话题标题。
func (r *SQLRepository) UpdateConversation(ctx context.Context, ownerUserID string, roomID string, conversationID string, title string) (*protocol.ConversationContextAggregate, error) {
	result, err := r.db.ExecContext(ctx, `
UPDATE conversations
SET title = `+r.dialect.Bind(1)+`, updated_at = `+r.dialect.CurrentTimestamp()+`
WHERE id = `+r.dialect.Bind(2)+` AND room_id = `+r.dialect.Bind(3)+` AND EXISTS (
    SELECT 1 FROM rooms WHERE id = `+r.dialect.Bind(4)+` AND owner_user_id = `+r.dialect.Bind(5)+`
)`,
		NullIfEmpty(title),
		conversationID,
		roomID,
		roomID,
		ownerUserID,
	)
	if err != nil {
		return nil, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, nil
	}
	return r.getContextByConversation(ctx, ownerUserID, roomID, conversationID)
}

// UpdateSessionSDKSessionID 更新房间会话记录上的 SDK session_id。
func (r *SQLRepository) UpdateSessionSDKSessionID(ctx context.Context, sessionID string, sdkSessionID string) error {
	result, err := r.db.ExecContext(ctx, `
UPDATE sessions
SET sdk_session_id = `+r.dialect.Bind(1)+`, updated_at = `+r.dialect.CurrentTimestamp()+`
WHERE id = `+r.dialect.Bind(2),
		NullIfEmpty(sdkSessionID),
		sessionID,
	)
	if err != nil {
		return err
	}
	_, err = result.RowsAffected()
	return err
}

// TouchConversationActivity 更新 conversation 级最近活动时间。
func (r *SQLRepository) TouchConversationActivity(ctx context.Context, conversationID string, activityAt time.Time) error {
	if conversationID == "" {
		return nil
	}
	if activityAt.IsZero() {
		activityAt = time.Now().UTC()
	}
	activityValue := r.dialect.TimestampValue(activityAt)
	_, err := r.db.ExecContext(ctx, `
UPDATE conversations
SET
    last_activity_at = CASE
        WHEN COALESCE(last_activity_at, created_at) < `+r.dialect.Bind(1)+` THEN `+r.dialect.Bind(2)+`
        ELSE COALESCE(last_activity_at, updated_at, created_at)
    END,
    updated_at = CASE
        WHEN updated_at < `+r.dialect.Bind(3)+` THEN `+r.dialect.Bind(4)+`
        ELSE updated_at
    END
WHERE id = `+r.dialect.Bind(5),
		activityValue,
		activityValue,
		activityValue,
		activityValue,
		conversationID,
	)
	return err
}
