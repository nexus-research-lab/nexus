// INPUT: owner-scoped Room session 查询与 SDK session ID 回写。
// OUTPUT: Room session 视图及遵循 Room-first 锁顺序的 session mutation。
// POS: Session service 的 Room SQL 投影仓储；写入不得先锁 session 再访问 Room。
package sessionrepo

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage"
	"github.com/nexus-research-lab/nexus/internal/storage/jsoncodec"
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
	items, scanErr := scanRoomSessions(rows)
	closeErr := rows.Close()
	if scanErr != nil {
		return nil, scanErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return items, nil
}

// ListRoomSessionsByAgent 列出指定 Agent 的 Room 成员会话视图。
func (r *SQLRepository) ListRoomSessionsByAgent(ctx context.Context, agentID string) ([]protocol.Session, error) {
	rows, err := r.db.QueryContext(ctx, r.roomSessionSelect()+`
WHERE s.is_primary = `+r.dialect.TrueValue()+` AND s.agent_id = `+r.dialect.Bind(1)+`
ORDER BY s.last_activity_at DESC`, agentID)
	if err != nil {
		return nil, err
	}
	items, scanErr := scanRoomSessions(rows)
	closeErr := rows.Close()
	if scanErr != nil {
		return nil, scanErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return items, nil
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
	_, err := r.updateRoomSessionSDKSessionID(
		ctx,
		roomSessionID,
		sdkSessionID,
		nil,
	)
	return err
}

// UpdateRoomSessionSDKSessionIDAtConnectorSelection 仅在 SQL Session 的 Connector
// 选择仍等于后台预备快照时提交 fork identity。
func (r *SQLRepository) UpdateRoomSessionSDKSessionIDAtConnectorSelection(
	ctx context.Context,
	roomSessionID string,
	sdkSessionID string,
	expected protocol.SessionConnectorSelection,
) (bool, error) {
	return r.updateRoomSessionSDKSessionID(
		ctx,
		roomSessionID,
		sdkSessionID,
		&expected,
	)
}

func (r *SQLRepository) updateRoomSessionSDKSessionID(
	ctx context.Context,
	roomSessionID string,
	sdkSessionID string,
	expected *protocol.SessionConnectorSelection,
) (bool, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	var roomID string
	err = tx.QueryRowContext(ctx, `
SELECT c.room_id
FROM sessions s
JOIN conversations c ON c.id = s.conversation_id
	WHERE s.id = `+r.dialect.Bind(1), roomSessionID).Scan(&roomID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err = storage.LockRoomForMutation(ctx, tx, r.dialect, "", roomID); err != nil {
		return false, err
	}
	var current sql.NullString
	var optionsJSON string
	if err = tx.QueryRowContext(
		ctx,
		"SELECT sdk_session_id, options_json FROM sessions WHERE id = "+r.dialect.Bind(1),
		roomSessionID,
	).Scan(&current, &optionsJSON); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	currentOptions := jsoncodec.ParseMap(optionsJSON)
	if expected != nil &&
		!protocol.SessionConnectorSelectionFromOptions(currentOptions).Equal(*expected) {
		return false, nil
	}
	options := protocol.WithTranscriptSessionIDs(
		currentOptions,
		[]string{current.String, sdkSessionID},
	)
	if strings.TrimSpace(sdkSessionID) != "" {
		forkSourceSessionID, _ := options[protocol.OptionRuntimeForkSourceSessionID].(string)
		options = protocol.WithRetainedTranscriptSessionIDs(options, []string{forkSourceSessionID})
		delete(options, protocol.OptionRuntimeForkSourceSessionID)
		delete(options, protocol.OptionRuntimeForkMessageID)
	}
	optionsJSON, err = jsoncodec.MarshalMap(options)
	if err != nil {
		return false, err
	}
	_, err = tx.ExecContext(ctx, `
UPDATE sessions
SET sdk_session_id = `+r.dialect.Bind(1)+`,
    options_json = `+r.dialect.Bind(2)+`,
    updated_at = `+r.dialect.CurrentTimestamp()+`
WHERE id = `+r.dialect.Bind(3)+`
  AND conversation_id IN (
      SELECT id FROM conversations WHERE room_id = `+r.dialect.Bind(4)+`
  )`,
		nullableStringValue(sdkSessionID),
		optionsJSON,
		roomSessionID,
		roomID,
	)
	if err != nil {
		return false, err
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

// UpdateRoomConversationRuntimeSettings 更新目标 Agent 模型，并统一 Conversation 权限。
func (r *SQLRepository) UpdateRoomConversationRuntimeSettings(
	ctx context.Context,
	roomSessionID string,
	targetOptions map[string]any,
	permissionMode string,
) ([]protocol.Session, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	rows, err := tx.QueryContext(ctx, `
SELECT sibling.id, sibling.options_json
FROM sessions target
JOIN sessions sibling ON sibling.conversation_id = target.conversation_id
WHERE target.id = `+r.dialect.Bind(1)+`
  AND sibling.is_primary = `+r.dialect.TrueValue(),
		roomSessionID,
	)
	if err != nil {
		return nil, err
	}
	type roomSessionOptions struct {
		id      string
		options map[string]any
	}
	sessionOptions := make([]roomSessionOptions, 0)
	for rows.Next() {
		var id string
		var rawOptions string
		if scanErr := rows.Scan(&id, &rawOptions); scanErr != nil {
			_ = rows.Close()
			return nil, scanErr
		}
		sessionOptions = append(sessionOptions, roomSessionOptions{
			id:      id,
			options: jsoncodec.ParseMap(rawOptions),
		})
	}
	if err = rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}
	if len(sessionOptions) == 0 {
		return nil, sql.ErrNoRows
	}

	for _, item := range sessionOptions {
		options := item.options
		if item.id == roomSessionID {
			options = targetOptions
		}
		settings := protocol.SessionRuntimeSettingsFromOptions(options)
		settings.PermissionMode = permissionMode
		options = protocol.WithSessionRuntimeSettings(options, settings)
		optionsJSON, marshalErr := jsoncodec.MarshalMap(options)
		if marshalErr != nil {
			return nil, marshalErr
		}
		if _, err = tx.ExecContext(ctx, `
UPDATE sessions
SET options_json = `+r.dialect.Bind(1)+`,
    updated_at = `+r.dialect.CurrentTimestamp()+`
WHERE id = `+r.dialect.Bind(2),
			optionsJSON,
			item.id,
		); err != nil {
			return nil, err
		}
	}

	projectedRows, err := tx.QueryContext(ctx, r.roomSessionSelect()+`
WHERE s.is_primary = `+r.dialect.TrueValue()+`
  AND c.id = (
      SELECT conversation_id
      FROM sessions
      WHERE id = `+r.dialect.Bind(1)+`
  )
ORDER BY s.last_activity_at DESC`,
		roomSessionID,
	)
	if err != nil {
		return nil, err
	}
	items, scanErr := scanRoomSessions(projectedRows)
	closeErr := projectedRows.Close()
	if scanErr != nil {
		return nil, scanErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *SQLRepository) roomSessionSelect() string {
	// messages 是历史导入兼容表。这里保留 COUNT 作为下限，service 必须再与
	// owner workspace 的 runtime 进度合并，不能把它解释为实时消息真相。
	return `
SELECT
    s.id,
    s.agent_id,
    COALESCE(s.sdk_session_id, ''),
    s.options_json,
    s.status,
    s.last_activity_at,
    c.last_activity_at,
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
		roomSessionID        string
		agentID              string
		sdkSessionID         string
		optionsJSON          string
		status               string
		lastActivity         time.Time
		conversationActivity sql.NullTime
		createdAt            time.Time
		conversationID       string
		title                string
		roomID               string
		roomType             string
		roomName             string
		messageCount         int
	)
	if err := scanner.Scan(
		&roomSessionID,
		&agentID,
		&sdkSessionID,
		&optionsJSON,
		&status,
		&lastActivity,
		&conversationActivity,
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
	// sessions.last_activity_at 建行后没有写入方推进；conversation 级活跃时间才是持续维护的真相。
	if conversationActivity.Valid && conversationActivity.Time.After(lastActivity) {
		lastActivity = conversationActivity.Time
	}
	result := protocol.Session{
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
		Options:        jsoncodec.ParseMap(optionsJSON),
		IsActive:       status == "active",
	}
	if result.Options == nil {
		result.Options = map[string]any{}
	}
	result.TranscriptSessionIDs = protocol.TranscriptSessionIDsFromOptions(result.Options)
	return result, nil
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
