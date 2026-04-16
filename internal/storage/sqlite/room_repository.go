// =====================================================
// @File   ：room_repository.go
// @Date   ：2026/04/10 22:52:00
// @Author ：leemysw
// 2026/04/10 22:52:00   Create
// =====================================================

package sqlite

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/room"
)

type roomQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// RoomRepository 提供 SQLite 的 Room 仓储实现。
type RoomRepository struct {
	db *sql.DB
}

// NewRoomRepository 创建 Room 仓储。
func NewRoomRepository(db *sql.DB) *RoomRepository {
	return &RoomRepository{db: db}
}

// LoadAgentRuntimeRefs 读取建房所需的 Agent 运行时信息。
func (r *RoomRepository) LoadAgentRuntimeRefs(ctx context.Context, agentIDs []string) ([]room.AgentRuntimeRef, error) {
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
WHERE a.id IN (%s)`, joinPlaceholders("?", len(agentIDs)))

	args := make([]any, 0, len(agentIDs))
	for _, agentID := range agentIDs {
		args = append(args, agentID)
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]room.AgentRuntimeRef, 0, len(agentIDs))
	for rows.Next() {
		var item room.AgentRuntimeRef
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
func (r *RoomRepository) ListRecentRooms(ctx context.Context, limit int) ([]room.RoomAggregate, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id FROM rooms ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]room.RoomAggregate, 0)
	for rows.Next() {
		var roomID string
		if err = rows.Scan(&roomID); err != nil {
			return nil, err
		}
		item, getErr := r.GetRoom(ctx, roomID)
		if getErr != nil {
			return nil, getErr
		}
		if item != nil {
			result = append(result, *item)
		}
	}
	return result, rows.Err()
}

// GetRoom 读取单个房间。
func (r *RoomRepository) GetRoom(ctx context.Context, roomID string) (*room.RoomAggregate, error) {
	roomValue, err := r.loadRoom(ctx, r.db, roomID)
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
	return &room.RoomAggregate{
		Room:    *roomValue,
		Members: members,
	}, nil
}

// GetRoomContexts 读取房间上下文。
func (r *RoomRepository) GetRoomContexts(ctx context.Context, roomID string) ([]room.ConversationContextAggregate, error) {
	roomAggregate, err := r.GetRoom(ctx, roomID)
	if err != nil || roomAggregate == nil {
		return nil, err
	}

	conversations, err := r.listConversations(ctx, r.db, roomID)
	if err != nil {
		return nil, err
	}

	contexts := make([]room.ConversationContextAggregate, 0, len(conversations))
	for _, conversation := range conversations {
		sessions, sessionErr := r.listSessionsByConversation(ctx, r.db, conversation.ID)
		if sessionErr != nil {
			return nil, sessionErr
		}
		contexts = append(contexts, room.ConversationContextAggregate{
			Room:         roomAggregate.Room,
			Members:      roomAggregate.Members,
			Conversation: conversation,
			Sessions:     sessions,
		})
	}
	return contexts, nil
}

// GetConversationContext 按 conversation_id 读取单条房间上下文。
func (r *RoomRepository) GetConversationContext(ctx context.Context, conversationID string) (*room.ConversationContextAggregate, error) {
	row := r.db.QueryRowContext(ctx, `SELECT room_id FROM conversations WHERE id = ? LIMIT 1`, conversationID)
	var roomID string
	if err := row.Scan(&roomID); errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	return r.getContextByConversation(ctx, roomID, conversationID)
}

// FindDMRoomContext 查找指定 Agent 的 DM 上下文。
func (r *RoomRepository) FindDMRoomContext(ctx context.Context, agentID string) (*room.ConversationContextAggregate, error) {
	var roomID string
	err := r.db.QueryRowContext(ctx, `
SELECT r.id
FROM rooms r
JOIN members m ON m.room_id = r.id
WHERE r.room_type = 'dm'
GROUP BY r.id
HAVING SUM(CASE WHEN m.member_type = 'agent' AND m.member_agent_id = ? THEN 1 ELSE 0 END) = 1
   AND SUM(CASE WHEN m.member_type = 'agent' THEN 1 ELSE 0 END) = 1
LIMIT 1`, agentID).Scan(&roomID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	contexts, err := r.GetRoomContexts(ctx, roomID)
	if err != nil || len(contexts) == 0 {
		return nil, err
	}
	for _, contextValue := range contexts {
		if contextValue.Conversation.ConversationType == room.ConversationTypeDM {
			return &contextValue, nil
		}
	}
	return &contexts[0], nil
}

// CreateRoom 创建房间、主对话和初始会话。
func (r *RoomRepository) CreateRoom(ctx context.Context, bundle room.CreateRoomBundle) (*room.ConversationContextAggregate, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err = tx.ExecContext(ctx, `
INSERT INTO rooms (id, room_type, name, description, avatar)
VALUES (?, ?, ?, ?, ?)`,
		bundle.Room.ID,
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
VALUES (?, ?, ?, ?, ?)`,
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
VALUES (?, ?, ?, ?)`,
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
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
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
	return r.getContextByConversation(ctx, bundle.Room.ID, bundle.Conversation.ID)
}

// UpdateRoom 更新房间及主对话标题。
func (r *RoomRepository) UpdateRoom(
	ctx context.Context,
	roomID string,
	name *string,
	description *string,
	title *string,
	avatar *string,
) (*room.ConversationContextAggregate, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	roomValue, err := r.loadRoom(ctx, tx, roomID)
	if err != nil || roomValue == nil {
		return nil, err
	}
	if name != nil {
		if _, err = tx.ExecContext(ctx, `UPDATE rooms SET name = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, nullIfEmpty(*name), roomID); err != nil {
			return nil, err
		}
	}
	if description != nil {
		if _, err = tx.ExecContext(ctx, `UPDATE rooms SET description = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, *description, roomID); err != nil {
			return nil, err
		}
	}
	if avatar != nil {
		if _, err = tx.ExecContext(ctx, `UPDATE rooms SET avatar = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, nullIfEmpty(*avatar), roomID); err != nil {
			return nil, err
		}
	}

	mainConversation, err := r.pickMainConversation(ctx, tx, roomID)
	if err != nil {
		return nil, err
	}
	if mainConversation != nil && title != nil {
		if _, err = tx.ExecContext(ctx, `UPDATE conversations SET title = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, nullIfEmpty(*title), mainConversation.ID); err != nil {
			return nil, err
		}
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	if mainConversation == nil {
		contexts, getErr := r.GetRoomContexts(ctx, roomID)
		if getErr != nil || len(contexts) == 0 {
			return nil, getErr
		}
		return &contexts[0], nil
	}
	return r.getContextByConversation(ctx, roomID, mainConversation.ID)
}

// AddRoomMember 向房间追加成员。
func (r *RoomRepository) AddRoomMember(ctx context.Context, roomID string, agent room.AgentRuntimeRef) (*room.ConversationContextAggregate, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	roomAggregate, err := r.getRoomAggregate(ctx, tx, roomID)
	if err != nil || roomAggregate == nil {
		return nil, err
	}
	if roomAggregate.Room.RoomType != room.RoomTypeGroup {
		return nil, errors.New("DM room does not support adding members")
	}
	for _, member := range roomAggregate.Members {
		if member.MemberType == room.MemberTypeAgent && member.MemberAgentID == agent.AgentID {
			return nil, errors.New("Agent already exists in room")
		}
	}

	if _, err = tx.ExecContext(ctx, `
INSERT INTO members (id, room_id, member_type, member_user_id, member_agent_id)
VALUES (?, ?, 'agent', NULL, ?)`,
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
WHERE conversation_id = ? AND agent_id = ? AND is_primary = 1
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
) VALUES (?, ?, ?, ?, 1, 'main', 1, NULL, 'active')`,
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
	return r.getContextByConversation(ctx, roomID, mainConversation.ID)
}

// RemoveRoomMember 从房间移除成员。
func (r *RoomRepository) RemoveRoomMember(ctx context.Context, roomID string, agentID string) (*room.ConversationContextAggregate, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	roomAggregate, err := r.getRoomAggregate(ctx, tx, roomID)
	if err != nil || roomAggregate == nil {
		return nil, err
	}
	if roomAggregate.Room.RoomType != room.RoomTypeGroup {
		return nil, errors.New("DM room does not support removing members")
	}

	agentCount := 0
	memberFound := false
	for _, member := range roomAggregate.Members {
		if member.MemberType == room.MemberTypeAgent && member.MemberAgentID != "" {
			agentCount++
		}
		if member.MemberType == room.MemberTypeAgent && member.MemberAgentID == agentID {
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
WHERE room_id = ? AND member_type = 'agent' AND member_agent_id = ?`,
		roomID,
		agentID,
	); err != nil {
		return nil, err
	}

	if _, err = tx.ExecContext(ctx, `
DELETE FROM sessions
WHERE conversation_id IN (SELECT id FROM conversations WHERE room_id = ?)
  AND agent_id = ?`,
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
	return r.getContextByConversation(ctx, roomID, mainConversation.ID)
}

// DeleteRoom 删除房间。
func (r *RoomRepository) DeleteRoom(ctx context.Context, roomID string) (bool, error) {
	result, err := r.db.ExecContext(ctx, `DELETE FROM rooms WHERE id = ?`, roomID)
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
func (r *RoomRepository) CreateConversation(ctx context.Context, bundle room.CreateConversationBundle) (*room.ConversationContextAggregate, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err = tx.ExecContext(ctx, `
INSERT INTO conversations (id, room_id, conversation_type, title)
VALUES (?, ?, ?, ?)`,
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
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
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
	return r.getContextByConversation(ctx, bundle.RoomID, bundle.Conversation.ID)
}

// UpdateConversation 更新话题标题。
func (r *RoomRepository) UpdateConversation(ctx context.Context, roomID string, conversationID string, title string) (*room.ConversationContextAggregate, error) {
	result, err := r.db.ExecContext(ctx, `
UPDATE conversations
SET title = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ? AND room_id = ?`,
		nullIfEmpty(title),
		conversationID,
		roomID,
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
	return r.getContextByConversation(ctx, roomID, conversationID)
}

// UpdateSessionSDKSessionID 更新房间会话记录上的 Claude session_id。
func (r *RoomRepository) UpdateSessionSDKSessionID(ctx context.Context, sessionID string, sdkSessionID string) error {
	result, err := r.db.ExecContext(ctx, `
UPDATE sessions
SET sdk_session_id = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?`,
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
func (r *RoomRepository) DeleteConversation(ctx context.Context, roomID string, conversationID string) (*room.ConversationContextAggregate, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

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
			targetIsTopic = conversation.ConversationType == room.ConversationTypeTopic
			continue
		}
		if fallbackConversationID == "" && (conversation.ConversationType == room.ConversationTypeMain || conversation.ConversationType == room.ConversationTypeDM) {
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

	result, err := tx.ExecContext(ctx, `DELETE FROM conversations WHERE id = ? AND room_id = ?`, conversationID, roomID)
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
	return r.getContextByConversation(ctx, roomID, fallbackConversationID)
}

func (r *RoomRepository) getRoomAggregate(ctx context.Context, querier roomQueryer, roomID string) (*room.RoomAggregate, error) {
	roomValue, err := r.loadRoom(ctx, querier, roomID)
	if err != nil || roomValue == nil {
		return nil, err
	}
	members, err := r.listMembers(ctx, querier, roomID)
	if err != nil {
		return nil, err
	}
	return &room.RoomAggregate{
		Room:    *roomValue,
		Members: members,
	}, nil
}

func (r *RoomRepository) loadRoom(ctx context.Context, querier roomQueryer, roomID string) (*room.RoomRecord, error) {
	row := querier.QueryRowContext(ctx, `
SELECT id, room_type, COALESCE(name, ''), description, COALESCE(avatar, ''), created_at, updated_at
FROM rooms
WHERE id = ?`, roomID)
	roomValue, err := scanRoomRecord(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &roomValue, nil
}

func (r *RoomRepository) listMembers(ctx context.Context, querier roomQueryer, roomID string) ([]room.MemberRecord, error) {
	rows, err := querier.QueryContext(ctx, `
SELECT id, room_id, member_type, COALESCE(member_user_id, ''), COALESCE(member_agent_id, ''), joined_at
FROM members
WHERE room_id = ?
ORDER BY joined_at ASC`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]room.MemberRecord, 0)
	for rows.Next() {
		item, scanErr := scanMemberRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *RoomRepository) listConversations(ctx context.Context, querier roomQueryer, roomID string) ([]room.ConversationRecord, error) {
	rows, err := querier.QueryContext(ctx, `
SELECT id, room_id, conversation_type, COALESCE(title, ''), created_at, updated_at
FROM conversations
WHERE room_id = ?
ORDER BY created_at ASC`, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]room.ConversationRecord, 0)
	for rows.Next() {
		item, scanErr := scanConversationRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *RoomRepository) listSessionsByConversation(ctx context.Context, querier roomQueryer, conversationID string) ([]room.SessionRecord, error) {
	rows, err := querier.QueryContext(ctx, `
SELECT
    id, conversation_id, agent_id, runtime_id, version_no, branch_key,
    is_primary, COALESCE(sdk_session_id, ''), status, last_activity_at, created_at, updated_at
FROM sessions
WHERE conversation_id = ?
ORDER BY last_activity_at DESC`, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]room.SessionRecord, 0)
	for rows.Next() {
		item, scanErr := scanSessionRecord(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (r *RoomRepository) pickMainConversation(ctx context.Context, querier roomQueryer, roomID string) (*room.ConversationRecord, error) {
	conversations, err := r.listConversations(ctx, querier, roomID)
	if err != nil || len(conversations) == 0 {
		return nil, err
	}
	item := pickMainConversation(conversations)
	return item, nil
}

func (r *RoomRepository) getContextByConversation(ctx context.Context, roomID string, conversationID string) (*room.ConversationContextAggregate, error) {
	contexts, err := r.GetRoomContexts(ctx, roomID)
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

func scanRoomRecord(scanner interface{ Scan(...any) error }) (room.RoomRecord, error) {
	var (
		item      room.RoomRecord
		createdAt time.Time
		updatedAt time.Time
	)
	err := scanner.Scan(
		&item.ID,
		&item.RoomType,
		&item.Name,
		&item.Description,
		&item.Avatar,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return room.RoomRecord{}, err
	}
	item.CreatedAt = createdAt
	item.UpdatedAt = updatedAt
	return item, nil
}

func scanMemberRecord(scanner interface{ Scan(...any) error }) (room.MemberRecord, error) {
	var (
		item     room.MemberRecord
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
		return room.MemberRecord{}, err
	}
	item.JoinedAt = joinedAt
	return item, nil
}

func scanConversationRecord(scanner interface{ Scan(...any) error }) (room.ConversationRecord, error) {
	var (
		item      room.ConversationRecord
		createdAt time.Time
		updatedAt time.Time
	)
	err := scanner.Scan(
		&item.ID,
		&item.RoomID,
		&item.ConversationType,
		&item.Title,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return room.ConversationRecord{}, err
	}
	item.CreatedAt = createdAt
	item.UpdatedAt = updatedAt
	return item, nil
}

func scanSessionRecord(scanner interface{ Scan(...any) error }) (room.SessionRecord, error) {
	var (
		item           room.SessionRecord
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
		return room.SessionRecord{}, err
	}
	item.LastActivityAt = lastActivityAt
	item.CreatedAt = createdAt
	item.UpdatedAt = updatedAt
	return item, nil
}

func pickMainConversation(conversations []room.ConversationRecord) *room.ConversationRecord {
	for _, conversation := range conversations {
		if conversation.ConversationType == room.ConversationTypeMain || conversation.ConversationType == room.ConversationTypeDM {
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

func joinPlaceholders(token string, count int) string {
	return strings.TrimRight(strings.Repeat(token+",", count), ",")
}

func newRoomEntityID() string {
	buffer := make([]byte, 6)
	if _, err := rand.Read(buffer); err == nil {
		return hex.EncodeToString(buffer)
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
