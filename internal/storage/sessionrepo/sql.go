package sessionrepo

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

// SQLRepository 提供 Room Session 视图查询。
type SQLRepository struct {
	db      *sql.DB
	dialect storage.SQLDialect
}

// NewSQLRepository 创建 SessionRepository。
func NewSQLRepository(driver string, db *sql.DB) *SQLRepository {
	return &SQLRepository{
		db:      db,
		dialect: storage.NewSQLDialect(driver),
	}
}

// ListRoomSessions 列出全部 Room 成员会话视图。
func (r *SQLRepository) ListRoomSessions(ctx context.Context, ownerUserID string) ([]protocol.Session, error) {
	rows, err := r.db.QueryContext(ctx, r.roomSessionSelect()+`
WHERE s.is_primary = `+r.dialect.TrueValue()+` AND r.owner_user_id = `+r.dialect.Bind(1)+`
ORDER BY s.last_activity_at DESC`, ownerUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRoomSessions(rows)
}

// ListRoomSessionsByAgent 列出指定 Agent 的 Room 成员会话视图。
func (r *SQLRepository) ListRoomSessionsByAgent(ctx context.Context, agentID string) ([]protocol.Session, error) {
	rows, err := r.db.QueryContext(ctx, r.roomSessionSelect()+`
WHERE s.is_primary = `+r.dialect.TrueValue()+` AND s.agent_id = `+r.dialect.Bind(1)+`
ORDER BY s.last_activity_at DESC`, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRoomSessions(rows)
}

// GetRoomSessionByKey 按结构化 key 查找 Room 成员会话。
func (r *SQLRepository) GetRoomSessionByKey(ctx context.Context, ownerUserID string, key protocol.SessionKey) (*protocol.Session, error) {
	if key.Kind != protocol.SessionKeyKindAgent || key.AgentID == "" || key.Ref == "" {
		return nil, nil
	}

	row := r.db.QueryRowContext(ctx, r.roomSessionSelect()+`
WHERE s.is_primary = `+r.dialect.TrueValue()+` AND r.owner_user_id = `+r.dialect.Bind(1)+` AND s.agent_id = `+r.dialect.Bind(2)+` AND c.id = `+r.dialect.Bind(3)+`
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
func (r *SQLRepository) UpdateRoomSessionSDKSessionID(
	ctx context.Context,
	roomSessionID string,
	sdkSessionID string,
) error {
	_, err := r.db.ExecContext(ctx, `
UPDATE sessions
SET sdk_session_id = `+r.dialect.Bind(1)+`, updated_at = `+r.dialect.CurrentTimestamp()+`
WHERE id = `+r.dialect.Bind(2),
		nullableStringValue(sdkSessionID),
		roomSessionID,
	)
	return err
}

func (r *SQLRepository) roomSessionSelect() string {
	return `
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
    (
        SELECT COUNT(1)
        FROM messages m
        WHERE m.conversation_id = c.id
    )
FROM sessions s
JOIN conversations c ON c.id = s.conversation_id
JOIN rooms r ON r.id = c.room_id
`
}

func scanRoomSessions(rows *sql.Rows) ([]protocol.Session, error) {
	result := make([]protocol.Session, 0)
	for rows.Next() {
		item, err := scanRoomSession(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func scanRoomSession(scanner interface{ Scan(...any) error }) (protocol.Session, error) {
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
		return protocol.Session{}, err
	}
	resolvedTitle := firstNonEmptyString(title, roomName, "New Chat")
	return protocol.Session{
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
		Options:        map[string]any{},
		IsActive:       status == "active",
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

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
