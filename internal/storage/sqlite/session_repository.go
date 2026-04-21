// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：session_repository.go
// @Date   ：2026/04/11 00:08:00
// @Author ：leemysw
// 2026/04/11 00:08:00   Create
// =====================================================

package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/nexus-research-lab/nexus/internal/model/session"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// SessionRepository 提供 SQLite 的 Room Session 视图查询。
type SessionRepository struct {
	db *sql.DB
}

// NewSessionRepository 创建 SessionRepository。
func NewSessionRepository(db *sql.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

// ListRoomSessions 列出全部 Room 成员会话视图。
func (r *SessionRepository) ListRoomSessions(ctx context.Context, ownerUserID string) ([]session.Session, error) {
	rows, err := r.db.QueryContext(ctx, sqliteRoomSessionSelect+`
WHERE s.is_primary = 1 AND r.owner_user_id = ?
ORDER BY s.last_activity_at DESC`, ownerUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRoomSessions(rows)
}

// ListRoomSessionsByAgent 列出指定 Agent 的 Room 成员会话视图。
func (r *SessionRepository) ListRoomSessionsByAgent(ctx context.Context, agentID string) ([]session.Session, error) {
	rows, err := r.db.QueryContext(ctx, sqliteRoomSessionSelect+`
WHERE s.is_primary = 1 AND s.agent_id = ?
ORDER BY s.last_activity_at DESC`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRoomSessions(rows)
}

// GetRoomSessionByKey 按结构化 key 查找 Room 成员会话。
func (r *SessionRepository) GetRoomSessionByKey(ctx context.Context, ownerUserID string, key protocol.SessionKey) (*session.Session, error) {
	if key.Kind != protocol.SessionKeyKindAgent || key.AgentID == "" || key.Ref == "" {
		return nil, nil
	}

	row := r.db.QueryRowContext(ctx, sqliteRoomSessionSelect+`
WHERE s.is_primary = 1 AND r.owner_user_id = ? AND s.agent_id = ? AND c.id = ?
LIMIT 1`, ownerUserID, key.AgentID, key.Ref)
	item, err := scanRoomSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

// UpdateRoomSessionSDKSessionID 回写 Room 成员会话的 sdk_session_id。
func (r *SessionRepository) UpdateRoomSessionSDKSessionID(
	ctx context.Context,
	roomSessionID string,
	sdkSessionID string,
) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE sessions
SET sdk_session_id = ?, updated_at = CURRENT_TIMESTAMP
WHERE id = ?`,
		nullableStringValue(sdkSessionID),
		roomSessionID,
	)
	return err
}

const sqliteRoomSessionSelect = `
SELECT
    s.id,
    s.agent_id,
    COALESCE(s.sdk_session_id, ''),
    s.status,
    s.last_activity_at,
    s.created_at,
    c.id,
    COALESCE(c.title, ''),
    r.id,
    r.room_type,
    COALESCE(r.name, ''),
    COALESCE(mc.message_count, 0)
FROM sessions s
JOIN conversations c ON c.id = s.conversation_id
JOIN rooms r ON r.id = c.room_id
LEFT JOIN (
    SELECT conversation_id, COUNT(id) AS message_count
    FROM messages
    GROUP BY conversation_id
) mc ON mc.conversation_id = c.id
`

func scanRoomSessions(rows *sql.Rows) ([]session.Session, error) {
	result := make([]session.Session, 0)
	for rows.Next() {
		item, err := scanRoomSession(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func scanRoomSession(scanner interface{ Scan(...any) error }) (session.Session, error) {
	var (
		roomSessionID  string
		agentID        string
		sdkSessionID   string
		status         string
		lastActivity   time.Time
		createdAt      time.Time
		conversationID string
		title          string
		roomID         string
		roomType       string
		roomName       string
		messageCount   int
	)
	if err := scanner.Scan(
		&roomSessionID,
		&agentID,
		&sdkSessionID,
		&status,
		&lastActivity,
		&createdAt,
		&conversationID,
		&title,
		&roomID,
		&roomType,
		&roomName,
		&messageCount,
	); err != nil {
		return session.Session{}, err
	}
	resolvedTitle := title
	if resolvedTitle == "" {
		resolvedTitle = roomName
	}
	if resolvedTitle == "" {
		resolvedTitle = "New Chat"
	}
	return session.Session{
		SessionKey:     protocol.BuildRoomAgentSessionKey(conversationID, agentID, roomType),
		AgentID:        agentID,
		SessionID:      nullableStringPointer(sdkSessionID),
		RoomSessionID:  nullableStringPointer(roomSessionID),
		RoomID:         nullableStringPointer(roomID),
		ConversationID: nullableStringPointer(conversationID),
		ChannelType:    "ws",
		ChatType:       roomChatType(roomType),
		Status:         status,
		CreatedAt:      createdAt.UTC(),
		LastActivity:   lastActivity.UTC(),
		Title:          resolvedTitle,
		MessageCount:   messageCount,
		Options: map[string]any{
			session.OptionHistorySource: session.HistorySourceTranscript,
		},
		IsActive: status == "active",
	}, nil
}

func roomChatType(roomType string) string {
	if roomType == "dm" {
		return "dm"
	}
	return "group"
}

func nullableStringPointer(value string) *string {
	if value == "" {
		return nil
	}
	copyValue := value
	return &copyValue
}

func nullableStringValue(value string) any {
	if value == "" {
		return nil
	}
	return value
}
