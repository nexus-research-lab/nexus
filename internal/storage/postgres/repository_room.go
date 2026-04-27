package postgres

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage/jsoncodec"
	"github.com/nexus-research-lab/nexus/internal/storage/roomrepo"
)

type roomQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// RoomRepository 提供 PostgreSQL 的 Room 仓储实现。
type RoomRepository struct {
	db *sql.DB
}

// NewRoomRepository 创建 Room 仓储。
func NewRoomRepository(db *sql.DB) *RoomRepository {
	return &RoomRepository{db: db}
}

// LoadAgentRuntimeRefs 读取建房所需的 Agent 运行时信息。
func (r *RoomRepository) LoadAgentRuntimeRefs(ctx context.Context, ownerUserID string, agentIDs []string) ([]roomrepo.AgentRuntimeRef, error) {
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
WHERE a.id IN (%s)`, joinPostgresPlaceholders(1, len(agentIDs)))

	args := make([]any, 0, len(agentIDs))
	for _, agentID := range agentIDs {
		args = append(args, agentID)
	}
	if ownerUserID != "" {
		query += fmt.Sprintf(" AND a.owner_user_id = $%d", len(args)+1)
		args = append(args, ownerUserID)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]roomrepo.AgentRuntimeRef, 0, len(agentIDs))
	for rows.Next() {
		var item roomrepo.AgentRuntimeRef
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
func (r *RoomRepository) ListRecentRooms(ctx context.Context, ownerUserID string, limit int) ([]protocol.RoomAggregate, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id FROM rooms WHERE owner_user_id = $1 ORDER BY updated_at DESC, created_at DESC LIMIT $2`,
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
func (r *RoomRepository) GetRoom(ctx context.Context, ownerUserID string, roomID string) (*protocol.RoomAggregate, error) {
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
func (r *RoomRepository) GetRoomContexts(ctx context.Context, ownerUserID string, roomID string) ([]protocol.ConversationContextAggregate, error) {
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

	contexts := make([]protocol.ConversationContextAggregate, 0, len(conversations))
	for _, conversation := range conversations {
		sessions, sessionErr := r.listSessionsByConversation(ctx, r.db, conversation.ID)
		if sessionErr != nil {
			return nil, sessionErr
		}
		contexts = append(contexts, protocol.ConversationContextAggregate{
			Room:         roomAggregate.Room,
			Members:      roomAggregate.Members,
			MemberAgents: memberAgents,
			Conversation: conversation,
			Sessions:     sessions,
		})
	}
	return contexts, nil
}

// GetConversationContext 按 conversation_id 读取单条房间上下文。
func (r *RoomRepository) GetConversationContext(ctx context.Context, ownerUserID string, conversationID string) (*protocol.ConversationContextAggregate, error) {
	row := r.db.QueryRowContext(ctx, `
SELECT c.room_id
FROM conversations c
JOIN rooms r ON r.id = c.room_id
WHERE c.id = $1 AND r.owner_user_id = $2
LIMIT 1`, conversationID, ownerUserID)
	var roomID string
	if err := row.Scan(&roomID); errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return r.getContextByConversation(ctx, ownerUserID, roomID, conversationID)
}

// FindDMRoomContext 查找指定 Agent 的 DM 上下文。
func (r *RoomRepository) FindDMRoomContext(ctx context.Context, ownerUserID string, agentID string) (*protocol.ConversationContextAggregate, error) {
	var roomID string
	err := r.db.QueryRowContext(ctx, `
SELECT r.id
FROM rooms r
JOIN members m ON m.room_id = r.id
WHERE r.room_type = 'dm' AND r.owner_user_id = $1
GROUP BY r.id
HAVING SUM(CASE WHEN m.member_type = 'agent' AND m.member_agent_id = $2 THEN 1 ELSE 0 END) = 1
   AND SUM(CASE WHEN m.member_type = 'agent' THEN 1 ELSE 0 END) = 1
LIMIT 1`, ownerUserID, agentID).Scan(&roomID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	contexts, err := r.GetRoomContexts(ctx, ownerUserID, roomID)
	if err != nil || len(contexts) == 0 {
		return nil, err
	}
	for _, contextValue := range contexts {
		if contextValue.Conversation.ConversationType == protocol.ConversationTypeDM {
			return &contextValue, nil
		}
	}
	return &contexts[0], nil
}

// CreateRoom 创建房间、主对话和初始会话。
func (r *RoomRepository) CreateRoom(ctx context.Context, bundle roomrepo.CreateRoomBundle) (*protocol.ConversationContextAggregate, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err = tx.ExecContext(ctx, `
INSERT INTO rooms (id, owner_user_id, room_type, name, description, avatar)
VALUES ($1, $2, $3, $4, $5, $6)`,
		bundle.Room.ID,
		bundle.Room.OwnerUserID,
		bundle.Room.RoomType,
		nullIfEmpty(bundle.Room.Name),
		bundle.Room.Description,
		nullIfEmpty(bundle.Room.Avatar),
	); err != nil {
		return nil, err
	}

	for _, member := range bundle.Members {
		if _, err = tx.ExecContext(ctx, `
INSERT INTO members (id, room_id, member_type, member_user_id, member_agent_id)
VALUES ($1, $2, $3, $4, $5)`,
			member.ID,
			member.RoomID,
			member.MemberType,
			nullIfEmpty(member.MemberUserID),
			nullIfEmpty(member.MemberAgentID),
		); err != nil {
			return nil, err
		}
	}

	if _, err = tx.ExecContext(ctx, `
INSERT INTO conversations (id, room_id, conversation_type, title)
VALUES ($1, $2, $3, $4)`,
		bundle.Conversation.ID,
		bundle.Conversation.RoomID,
		bundle.Conversation.ConversationType,
		nullIfEmpty(bundle.Conversation.Title),
	); err != nil {
		return nil, err
	}

	for _, sessionValue := range bundle.Sessions {
		if _, err = tx.ExecContext(ctx, `
INSERT INTO sessions (
    id, conversation_id, agent_id, runtime_id, version_no, branch_key,
    is_primary, sdk_session_id, status
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			sessionValue.ID,
			sessionValue.ConversationID,
			sessionValue.AgentID,
			sessionValue.RuntimeID,
			sessionValue.VersionNo,
			sessionValue.BranchKey,
			sessionValue.IsPrimary,
			nullIfEmpty(sessionValue.SDKSessionID),
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
func (r *RoomRepository) UpdateRoom(
	ctx context.Context,
	ownerUserID string,
	roomID string,
	name *string,
	description *string,
	title *string,
	avatar *string,
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
	if name != nil {
		if _, err = tx.ExecContext(ctx, `UPDATE rooms SET name = $1, updated_at = now() WHERE id = $2 AND owner_user_id = $3`, nullIfEmpty(*name), roomID, ownerUserID); err != nil {
			return nil, err
		}
	}
	if description != nil {
		if _, err = tx.ExecContext(ctx, `UPDATE rooms SET description = $1, updated_at = now() WHERE id = $2 AND owner_user_id = $3`, *description, roomID, ownerUserID); err != nil {
			return nil, err
		}
	}
	if avatar != nil {
		if _, err = tx.ExecContext(ctx, `UPDATE rooms SET avatar = $1, updated_at = now() WHERE id = $2 AND owner_user_id = $3`, nullIfEmpty(*avatar), roomID, ownerUserID); err != nil {
			return nil, err
		}
	}

	mainConversation, err := r.pickMainConversation(ctx, tx, roomID)
	if err != nil {
		return nil, err
	}
	if mainConversation != nil && title != nil {
		if _, err = tx.ExecContext(ctx, `UPDATE conversations SET title = $1, updated_at = now() WHERE id = $2`, nullIfEmpty(*title), mainConversation.ID); err != nil {
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
func (r *RoomRepository) AddRoomMember(ctx context.Context, ownerUserID string, roomID string, agent roomrepo.AgentRuntimeRef) (*protocol.ConversationContextAggregate, error) {
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
VALUES ($1, $2, 'agent', NULL, $3)`,
		newRoomEntityID(),
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
		var existingID string
		queryErr := tx.QueryRowContext(ctx, `
SELECT id FROM sessions
WHERE conversation_id = $1 AND agent_id = $2 AND is_primary = true
LIMIT 1`, conversation.ID, agent.AgentID).Scan(&existingID)
		if queryErr != nil && !errors.Is(queryErr, sql.ErrNoRows) {
			return nil, queryErr
		}
		if existingID != "" {
			continue
		}
		if _, err = tx.ExecContext(ctx, `
INSERT INTO sessions (
    id, conversation_id, agent_id, runtime_id, version_no, branch_key,
    is_primary, sdk_session_id, status
) VALUES ($1, $2, $3, $4, 1, 'main', true, NULL, 'active')`,
			newRoomEntityID(),
			conversation.ID,
			agent.AgentID,
			agent.RuntimeID,
		); err != nil {
			return nil, err
		}
	}

	mainConversation := pickMainConversation(conversations)
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	if mainConversation == nil {
		return nil, nil
	}
	return r.getContextByConversation(ctx, ownerUserID, roomID, mainConversation.ID)
}

// RemoveRoomMember 从房间移除成员。
func (r *RoomRepository) RemoveRoomMember(ctx context.Context, ownerUserID string, roomID string, agentID string) (*protocol.ConversationContextAggregate, error) {
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
		return nil, errors.New("DM room does not support removing members")
	}

	agentCount := 0
	memberFound := false
	for _, member := range roomAggregate.Members {
		if member.MemberType == protocol.MemberTypeAgent && member.MemberAgentID != "" {
			agentCount++
		}
		if member.MemberType == protocol.MemberTypeAgent && member.MemberAgentID == agentID {
			memberFound = true
		}
	}
	if !memberFound {
		return nil, nil
	}
	if agentCount <= 1 {
		return nil, errors.New("Room 至少保留一个 agent 成员")
	}

	if _, err = tx.ExecContext(ctx, `
DELETE FROM members
WHERE room_id = $1 AND member_type = 'agent' AND member_agent_id = $2`,
		roomID,
		agentID,
	); err != nil {
		return nil, err
	}

	if _, err = tx.ExecContext(ctx, `
DELETE FROM sessions
WHERE conversation_id IN (SELECT id FROM conversations WHERE room_id = $1)
  AND agent_id = $2`,
		roomID,
		agentID,
	); err != nil {
		return nil, err
	}

	conversations, err := r.listConversations(ctx, tx, roomID)
	if err != nil {
		return nil, err
	}
	mainConversation := pickMainConversation(conversations)
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	if mainConversation == nil {
		return nil, nil
	}
	return r.getContextByConversation(ctx, ownerUserID, roomID, mainConversation.ID)
}

// DeleteRoom 删除房间。
func (r *RoomRepository) DeleteRoom(ctx context.Context, ownerUserID string, roomID string) (bool, error) {
	result, err := r.db.ExecContext(ctx, `DELETE FROM rooms WHERE id = $1 AND owner_user_id = $2`, roomID, ownerUserID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

// CreateConversation 创建房间话题。
func (r *RoomRepository) CreateConversation(ctx context.Context, bundle roomrepo.CreateConversationBundle) (*protocol.ConversationContextAggregate, error) {
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

	if _, err = tx.ExecContext(ctx, `
INSERT INTO conversations (id, room_id, conversation_type, title)
VALUES ($1, $2, $3, $4)`,
		bundle.Conversation.ID,
		bundle.Conversation.RoomID,
		bundle.Conversation.ConversationType,
		nullIfEmpty(bundle.Conversation.Title),
	); err != nil {
		return nil, err
	}
	for _, sessionValue := range bundle.Sessions {
		if _, err = tx.ExecContext(ctx, `
INSERT INTO sessions (
    id, conversation_id, agent_id, runtime_id, version_no, branch_key,
    is_primary, sdk_session_id, status
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			sessionValue.ID,
			sessionValue.ConversationID,
			sessionValue.AgentID,
			sessionValue.RuntimeID,
			sessionValue.VersionNo,
			sessionValue.BranchKey,
			sessionValue.IsPrimary,
			nullIfEmpty(sessionValue.SDKSessionID),
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
func (r *RoomRepository) UpdateConversation(ctx context.Context, ownerUserID string, roomID string, conversationID string, title string) (*protocol.ConversationContextAggregate, error) {
	result, err := r.db.ExecContext(ctx, `
UPDATE conversations
SET title = $1, updated_at = now()
WHERE id = $2 AND room_id = $3 AND EXISTS (
    SELECT 1 FROM rooms WHERE id = $4 AND owner_user_id = $5
)`,
		nullIfEmpty(title),
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

// UpdateSessionSDKSessionID 更新房间会话记录上的 Claude session_id。
func (r *RoomRepository) UpdateSessionSDKSessionID(ctx context.Context, sessionID string, sdkSessionID string) error {
	result, err := r.db.ExecContext(ctx, `
UPDATE sessions
SET sdk_session_id = $1, updated_at = now()
WHERE id = $2`,
		nullIfEmpty(sdkSessionID),
		sessionID,
	)
	if err != nil {
		return err
	}
	_, err = result.RowsAffected()
	return err
}

// DeleteConversation 删除话题并返回回退上下文。
func (r *RoomRepository) DeleteConversation(ctx context.Context, ownerUserID string, roomID string, conversationID string) (*protocol.ConversationContextAggregate, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	roomValue, err := r.loadRoom(ctx, tx, ownerUserID, roomID)
	if err != nil || roomValue == nil {
		return nil, err
	}
	conversations, err := r.listConversations(ctx, tx, roomID)
	if err != nil {
		return nil, err
	}
	if len(conversations) <= 1 {
		return nil, errors.New("room 至少保留一个对话")
	}

	var (
		targetFound            bool
		targetIsTopic          bool
		fallbackConversationID string
	)
	for _, conversation := range conversations {
		if conversation.ID == conversationID {
			targetFound = true
			targetIsTopic = conversation.ConversationType == protocol.ConversationTypeTopic
			continue
		}
		if fallbackConversationID == "" && (conversation.ConversationType == protocol.ConversationTypeMain || conversation.ConversationType == protocol.ConversationTypeDM) {
			fallbackConversationID = conversation.ID
		}
	}
	if !targetFound {
		return nil, nil
	}
	if !targetIsTopic {
		return nil, errors.New("主对话不支持删除")
	}
	if fallbackConversationID == "" {
		for _, conversation := range conversations {
			if conversation.ID != conversationID {
				fallbackConversationID = conversation.ID
				break
			}
		}
	}

	result, err := tx.ExecContext(ctx, `DELETE FROM conversations WHERE id = $1 AND room_id = $2`, conversationID, roomID)
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

	if err = tx.Commit(); err != nil {
		return nil, err
	}
	if fallbackConversationID == "" {
		return nil, nil
	}
	return r.getContextByConversation(ctx, ownerUserID, roomID, fallbackConversationID)
}

func (r *RoomRepository) getRoomAggregate(ctx context.Context, querier roomQueryer, ownerUserID string, roomID string) (*protocol.RoomAggregate, error) {
	roomValue, err := r.loadRoom(ctx, querier, ownerUserID, roomID)
	if err != nil || roomValue == nil {
		return nil, err
	}
	members, err := r.listMembers(ctx, querier, roomID)
	if err != nil {
		return nil, err
	}
	return &protocol.RoomAggregate{
		Room:    *roomValue,
		Members: members,
	}, nil
}

func (r *RoomRepository) loadRoom(ctx context.Context, querier roomQueryer, ownerUserID string, roomID string) (*protocol.RoomRecord, error) {
	query := `
SELECT id, owner_user_id, room_type, COALESCE(name, ''), description, COALESCE(avatar, ''), created_at, updated_at
FROM rooms
WHERE id = $1`
	args := []any{roomID}
	if ownerUserID != "" {
		query += ` AND owner_user_id = $2`
		args = append(args, ownerUserID)
	}
	row := querier.QueryRowContext(ctx, query, args...)
	roomValue, err := scanRoomRecord(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &roomValue, nil
}

func (r *RoomRepository) lookupRoomOwnerUserID(ctx context.Context, querier roomQueryer, roomID string) (string, error) {
	row := querier.QueryRowContext(ctx, `SELECT owner_user_id FROM rooms WHERE id = $1 LIMIT 1`, roomID)
	var ownerUserID string
	if err := row.Scan(&ownerUserID); errors.Is(err, sql.ErrNoRows) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	return ownerUserID, nil
}

func (r *RoomRepository) listMembers(ctx context.Context, querier roomQueryer, roomID string) ([]protocol.MemberRecord, error) {
	rows, err := querier.QueryContext(ctx, `
SELECT id, room_id, member_type, COALESCE(member_user_id, ''), COALESCE(member_agent_id, ''), joined_at
FROM members
WHERE room_id = $1
ORDER BY joined_at ASC`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]protocol.MemberRecord, 0)
	for rows.Next() {
		item, scanErr := scanMemberRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *RoomRepository) loadRoomsByIDs(
	ctx context.Context,
	querier roomQueryer,
	ownerUserID string,
	roomIDs []string,
) (map[string]protocol.RoomRecord, error) {
	if len(roomIDs) == 0 {
		return map[string]protocol.RoomRecord{}, nil
	}
	query := fmt.Sprintf(`
SELECT id, owner_user_id, room_type, COALESCE(name, ''), description, COALESCE(avatar, ''), created_at, updated_at
FROM rooms
WHERE id IN (%s)`, joinPostgresPlaceholders(1, len(roomIDs)))
	args := make([]any, 0, len(roomIDs))
	for _, roomID := range roomIDs {
		args = append(args, roomID)
	}
	query += fmt.Sprintf(" AND owner_user_id = $%d", len(args)+1)
	args = append(args, ownerUserID)
	rows, err := querier.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]protocol.RoomRecord, len(roomIDs))
	for rows.Next() {
		item, scanErr := scanRoomRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result[item.ID] = item
	}
	return result, rows.Err()
}

func (r *RoomRepository) listMembersByRoomIDs(
	ctx context.Context,
	querier roomQueryer,
	roomIDs []string,
) (map[string][]protocol.MemberRecord, error) {
	if len(roomIDs) == 0 {
		return map[string][]protocol.MemberRecord{}, nil
	}
	query := fmt.Sprintf(`
SELECT id, room_id, member_type, COALESCE(member_user_id, ''), COALESCE(member_agent_id, ''), joined_at
FROM members
WHERE room_id IN (%s)
ORDER BY joined_at ASC`, joinPostgresPlaceholders(1, len(roomIDs)))
	args := make([]any, 0, len(roomIDs))
	for _, roomID := range roomIDs {
		args = append(args, roomID)
	}
	rows, err := querier.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]protocol.MemberRecord, len(roomIDs))
	for rows.Next() {
		item, scanErr := scanMemberRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result[item.RoomID] = append(result[item.RoomID], item)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	for _, roomID := range roomIDs {
		if _, exists := result[roomID]; !exists {
			result[roomID] = []protocol.MemberRecord{}
		}
	}
	return result, nil
}

func (r *RoomRepository) listMemberAgents(
	ctx context.Context,
	querier roomQueryer,
	roomID string,
) ([]protocol.Agent, error) {
	rows, err := querier.QueryContext(ctx, `
SELECT
    a.id,
    a.name,
    a.owner_user_id,
    a.workspace_path,
    a.status,
    a.is_main,
    COALESCE(a.avatar, ''),
    COALESCE(a.description, ''),
    COALESCE(a.vibe_tags::text, '[]'),
    a.created_at,
    COALESCE(rt.provider, ''),
    COALESCE(rt.permission_mode, ''),
    COALESCE(rt.allowed_tools_json, '[]'),
    COALESCE(rt.disallowed_tools_json, '[]'),
    COALESCE(rt.mcp_servers_json, '{}'),
    rt.max_turns,
    rt.max_thinking_tokens,
    COALESCE(rt.setting_sources_json, '[]')
FROM members m
JOIN agents a ON a.id = m.member_agent_id
LEFT JOIN runtimes rt ON rt.agent_id = a.id
WHERE m.room_id = $1 AND m.member_type = 'agent'
ORDER BY m.joined_at ASC`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]protocol.Agent, 0)
	for rows.Next() {
		item, scanErr := scanRoomMemberAgent(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *RoomRepository) listConversations(ctx context.Context, querier roomQueryer, roomID string) ([]protocol.ConversationRecord, error) {
	rows, err := querier.QueryContext(ctx, `
SELECT
    c.id,
    c.room_id,
    c.conversation_type,
    COALESCE(c.title, ''),
    COALESCE(mc.message_count, 0),
    c.created_at,
    c.updated_at
FROM conversations c
LEFT JOIN (
    SELECT conversation_id, COUNT(id) AS message_count
    FROM messages
    GROUP BY conversation_id
) mc ON mc.conversation_id = c.id
WHERE room_id = $1
ORDER BY created_at ASC`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]protocol.ConversationRecord, 0)
	for rows.Next() {
		item, scanErr := scanConversationRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *RoomRepository) listSessionsByConversation(ctx context.Context, querier roomQueryer, conversationID string) ([]protocol.SessionRecord, error) {
	rows, err := querier.QueryContext(ctx, `
SELECT
    id, conversation_id, agent_id, runtime_id, version_no, branch_key,
    is_primary, COALESCE(sdk_session_id, ''), status, last_activity_at, created_at, updated_at
FROM sessions
WHERE conversation_id = $1
ORDER BY last_activity_at DESC`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]protocol.SessionRecord, 0)
	for rows.Next() {
		item, scanErr := scanSessionRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *RoomRepository) pickMainConversation(ctx context.Context, querier roomQueryer, roomID string) (*protocol.ConversationRecord, error) {
	conversations, err := r.listConversations(ctx, querier, roomID)
	if err != nil || len(conversations) == 0 {
		return nil, err
	}
	item := pickMainConversation(conversations)
	return item, nil
}

func (r *RoomRepository) getContextByConversation(ctx context.Context, ownerUserID string, roomID string, conversationID string) (*protocol.ConversationContextAggregate, error) {
	contexts, err := r.GetRoomContexts(ctx, ownerUserID, roomID)
	if err != nil {
		return nil, err
	}
	for _, contextValue := range contexts {
		if contextValue.Conversation.ID == conversationID {
			return &contextValue, nil
		}
	}
	if len(contexts) == 0 {
		return nil, nil
	}
	return &contexts[0], nil
}

func scanRoomRecord(scanner interface{ Scan(...any) error }) (protocol.RoomRecord, error) {
	var (
		item      protocol.RoomRecord
		createdAt time.Time
		updatedAt time.Time
	)
	err := scanner.Scan(
		&item.ID,
		&item.OwnerUserID,
		&item.RoomType,
		&item.Name,
		&item.Description,
		&item.Avatar,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return protocol.RoomRecord{}, err
	}
	item.CreatedAt = createdAt
	item.UpdatedAt = updatedAt
	return item, nil
}

func scanMemberRecord(scanner interface{ Scan(...any) error }) (protocol.MemberRecord, error) {
	var (
		item     protocol.MemberRecord
		joinedAt time.Time
	)
	err := scanner.Scan(
		&item.ID,
		&item.RoomID,
		&item.MemberType,
		&item.MemberUserID,
		&item.MemberAgentID,
		&joinedAt,
	)
	if err != nil {
		return protocol.MemberRecord{}, err
	}
	item.JoinedAt = joinedAt
	return item, nil
}

func scanRoomMemberAgent(scanner interface{ Scan(...any) error }) (protocol.Agent, error) {
	var (
		item                protocol.Agent
		vibeTagsJSON        string
		allowedToolsJSON    string
		disallowedToolsJSON string
		mcpServersJSON      string
		settingSourcesJSON  string
		maxTurns            sql.NullInt64
		maxThinkingTokens   sql.NullInt64
		createdAt           time.Time
	)

	err := scanner.Scan(
		&item.AgentID,
		&item.Name,
		&item.OwnerUserID,
		&item.WorkspacePath,
		&item.Status,
		&item.IsMain,
		&item.Avatar,
		&item.Description,
		&vibeTagsJSON,
		&createdAt,
		&item.Options.Provider,
		&item.Options.PermissionMode,
		&allowedToolsJSON,
		&disallowedToolsJSON,
		&mcpServersJSON,
		&maxTurns,
		&maxThinkingTokens,
		&settingSourcesJSON,
	)
	if err != nil {
		return protocol.Agent{}, err
	}

	item.CreatedAt = createdAt
	item.VibeTags = jsoncodec.ParseStringSlice(vibeTagsJSON)
	item.Options.AllowedTools = jsoncodec.ParseStringSlice(allowedToolsJSON)
	item.Options.DisallowedTools = jsoncodec.ParseStringSlice(disallowedToolsJSON)
	item.Options.MCPServers = jsoncodec.ParseMap(mcpServersJSON)
	item.Options.SettingSources = jsoncodec.ParseStringSlice(settingSourcesJSON)
	if maxTurns.Valid {
		value := int(maxTurns.Int64)
		item.Options.MaxTurns = &value
	}
	if maxThinkingTokens.Valid {
		value := int(maxThinkingTokens.Int64)
		item.Options.MaxThinkingTokens = &value
	}
	return item, nil
}

func scanConversationRecord(scanner interface{ Scan(...any) error }) (protocol.ConversationRecord, error) {
	var (
		item      protocol.ConversationRecord
		createdAt time.Time
		updatedAt time.Time
	)
	err := scanner.Scan(
		&item.ID,
		&item.RoomID,
		&item.ConversationType,
		&item.Title,
		&item.MessageCount,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return protocol.ConversationRecord{}, err
	}
	item.CreatedAt = createdAt
	item.UpdatedAt = updatedAt
	return item, nil
}

func scanSessionRecord(scanner interface{ Scan(...any) error }) (protocol.SessionRecord, error) {
	var (
		item           protocol.SessionRecord
		lastActivityAt time.Time
		createdAt      time.Time
		updatedAt      time.Time
	)
	err := scanner.Scan(
		&item.ID,
		&item.ConversationID,
		&item.AgentID,
		&item.RuntimeID,
		&item.VersionNo,
		&item.BranchKey,
		&item.IsPrimary,
		&item.SDKSessionID,
		&item.Status,
		&lastActivityAt,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return protocol.SessionRecord{}, err
	}
	item.LastActivityAt = lastActivityAt
	item.CreatedAt = createdAt
	item.UpdatedAt = updatedAt
	return item, nil
}

func pickMainConversation(conversations []protocol.ConversationRecord) *protocol.ConversationRecord {
	for _, conversation := range conversations {
		if conversation.ConversationType == protocol.ConversationTypeMain || conversation.ConversationType == protocol.ConversationTypeDM {
			item := conversation
			return &item
		}
	}
	if len(conversations) == 0 {
		return nil
	}
	item := conversations[0]
	return &item
}

func joinPostgresPlaceholders(start int, count int) string {
	parts := make([]string, 0, count)
	for index := 0; index < count; index++ {
		parts = append(parts, fmt.Sprintf("$%d", start+index))
	}
	return strings.Join(parts, ", ")
}

func newRoomEntityID() string {
	buffer := make([]byte, 6)
	if _, err := rand.Read(buffer); err == nil {
		return hex.EncodeToString(buffer)
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
