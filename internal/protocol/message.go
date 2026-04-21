// =====================================================
// @File   ：message.go
// @Date   ：2026/04/10 21:22:41
// @Author ：leemysw
// 2026/04/10 21:22:41   Create
// =====================================================

package protocol

import (
	"encoding/json"
	"os"
	"strings"
	"time"
)

// EventType 表示统一事件类型。
type EventType string

// ChatAckTimeoutMS 是客户端等待 chat_ack 的上限（毫秒）。
// 服务端不强制该窗口，但承诺在此之前回 ack；
// 前端据此设置本地超时，两侧同源避免漂移。
const ChatAckTimeoutMS = 10000

const (
	EventTypeMessage               EventType = "message"
	EventTypeStream                EventType = "stream"
	EventTypeChatAck               EventType = "chat_ack"
	EventTypeRoundStatus           EventType = "round_status"
	EventTypeSessionStatus         EventType = "session_status"
	EventTypePermissionRequest     EventType = "permission_request"
	EventTypeAgentRuntimeEvent     EventType = "agent_runtime_event"
	EventTypeWorkspaceEvent        EventType = "workspace_event"
	EventTypeRoomMemberAdded       EventType = "room_member_added"
	EventTypeRoomMemberRemoved     EventType = "room_member_removed"
	EventTypeRoomDeleted           EventType = "room_deleted"
	EventTypeRoomResyncRequired    EventType = "room_resync_required"
	EventTypeSessionResyncRequired EventType = "session_resync_required"
	EventTypeStreamStart           EventType = "stream_start"
	EventTypeStreamEnd             EventType = "stream_end"
	EventTypeStreamCancelled       EventType = "stream_cancelled"
	EventTypeError                 EventType = "error"
	EventTypePong                  EventType = "pong"
)

// EventMessage 对齐前后端统一 envelope。
type EventMessage struct {
	EnvelopeID      string         `json:"envelope_id,omitempty"`
	ProtocolVersion int            `json:"protocol_version"`
	DeliveryMode    string         `json:"delivery_mode,omitempty"`
	EventType       EventType      `json:"event_type"`
	SessionKey      string         `json:"session_key,omitempty"`
	SessionSeq      *int64         `json:"session_seq,omitempty"`
	RoomID          string         `json:"room_id,omitempty"`
	RoomSeq         *int64         `json:"room_seq,omitempty"`
	ConversationID  string         `json:"conversation_id,omitempty"`
	AgentID         string         `json:"agent_id,omitempty"`
	MessageID       string         `json:"message_id,omitempty"`
	SessionID       string         `json:"session_id,omitempty"`
	CausedBy        string         `json:"caused_by,omitempty"`
	Data            map[string]any `json:"data"`
	Timestamp       int64          `json:"timestamp"`
}

// InboundWebSocketMessage 表示前端发送给服务端的基础消息。
type InboundWebSocketMessage struct {
	Type       string `json:"type"`
	SessionKey string `json:"session_key,omitempty"`
	ClientID   string `json:"client_id,omitempty"`
}

// RoundStatusData 表示 round 生命周期事件。
type RoundStatusData struct {
	RoundID       string `json:"round_id"`
	Status        string `json:"status"`
	IsTerminal    bool   `json:"is_terminal"`
	ResultSubtype string `json:"result_subtype,omitempty"`
}

// SessionStatusData 表示 session 生命周期事件。
type SessionStatusData struct {
	IsGenerating     bool     `json:"is_generating"`
	RunningRoundIDs  []string `json:"running_round_ids,omitempty"`
	ControllerClient string   `json:"controller_client_id,omitempty"`
	ObserverCount    int      `json:"observer_count,omitempty"`
	BoundClientCount int      `json:"bound_client_count,omitempty"`
}

// NewEvent 构造通用事件。
func NewEvent(eventType EventType, data map[string]any) EventMessage {
	return EventMessage{
		ProtocolVersion: 2,
		DeliveryMode:    "ephemeral",
		EventType:       eventType,
		Data:            data,
		Timestamp:       time.Now().UnixMilli(),
	}
}

// NewErrorEvent 构造错误事件。
func NewErrorEvent(sessionKey string, message string) EventMessage {
	event := NewEvent(EventTypeError, map[string]any{
		"message": message,
	})
	event.SessionKey = sessionKey
	return event
}

// NewPongEvent 构造 pong 事件。
func NewPongEvent(sessionKey string) EventMessage {
	event := NewEvent(EventTypePong, map[string]any{})
	event.SessionKey = sessionKey
	return event
}

// TypeScriptDefinitions 返回生成前端类型定义所需内容。
func TypeScriptDefinitions() string {
	const definitions = `/**
 * 由 cmd/protocol-tsgen 自动生成，请勿手改。
 */

export type EventType =
  | 'message'
  | 'stream'
  | 'chat_ack'
  | 'round_status'
  | 'session_status'
  | 'permission_request'
  | 'agent_runtime_event'
  | 'workspace_event'
  | 'room_member_added'
  | 'room_member_removed'
  | 'room_deleted'
  | 'session_resync_required'
  | 'room_resync_required'
  | 'stream_start'
  | 'stream_end'
  | 'stream_cancelled'
  | 'error'
  | 'pong';

export interface EventMessage {
  envelope_id?: string;
  protocol_version: number;
  delivery_mode?: string;
  event_type: EventType;
  session_key?: string;
  session_seq?: number;
  room_id?: string;
  room_seq?: number;
  conversation_id?: string;
  agent_id?: string;
  message_id?: string;
  session_id?: string;
  caused_by?: string;
  data: Record<string, unknown>;
  timestamp: number;
}

export interface RoundStatusData {
  round_id: string;
  status: string;
  is_terminal: boolean;
  result_subtype?: string;
}

export interface SessionStatusData {
  is_generating: boolean;
  running_round_ids?: string[];
  controller_client_id?: string;
  observer_count?: number;
  bound_client_count?: number;
}
`
	return definitions
}

// NewRoundStatusEvent 构造 round_status 事件。
func NewRoundStatusEvent(sessionKey string, roundID string, status string, resultSubtype string) EventMessage {
	data := map[string]any{
		"round_id":    roundID,
		"status":      status,
		"is_terminal": status == "finished" || status == "interrupted" || status == "error",
	}
	if strings.TrimSpace(resultSubtype) != "" {
		data["result_subtype"] = strings.TrimSpace(resultSubtype)
	}
	event := NewEvent(EventTypeRoundStatus, data)
	event.SessionKey = sessionKey
	return event
}

// NewChatAckEvent 构造 chat_ack 事件。
func NewChatAckEvent(sessionKey string, reqID string, roundID string, pending []map[string]any) EventMessage {
	event := NewEvent(EventTypeChatAck, map[string]any{
		"req_id":         reqID,
		"round_id":       roundID,
		"pending":        pending,
		"ack_timeout_ms": ChatAckTimeoutMS,
	})
	event.SessionKey = sessionKey
	return event
}

// MigrationDirName 返回 migration 目录名。
func MigrationDirName(driver string) string {
	switch strings.ToLower(driver) {
	case "postgres", "postgresql", "pg":
		return "postgres"
	default:
		return "sqlite"
	}
}

// GooseDialect 返回 goose 识别的方言名。
func GooseDialect(driver string) string {
	switch strings.ToLower(driver) {
	case "postgres", "postgresql", "pg":
		return "postgres"
	default:
		return "sqlite3"
	}
}

// NormalizeSQLDriver 把配置里的数据库驱动名规范化为 database/sql 名称。
func NormalizeSQLDriver(driver string) string {
	switch strings.ToLower(driver) {
	case "postgres", "postgresql", "pg":
		return "pgx"
	default:
		return "sqlite3"
	}
}

// NormalizeDatabaseURL 把旧配置格式转为 Go SQL 驱动可识别的 DSN。
func NormalizeDatabaseURL(raw string) string {
	normalized := strings.TrimSpace(raw)
	switch {
	case strings.HasPrefix(normalized, "sqlite:///"):
		return strings.TrimPrefix(normalized, "sqlite:///")
	case strings.HasPrefix(normalized, "~/"):
		home, err := os.UserHomeDir()
		if err == nil {
			return strings.Replace(normalized, "~/", home+"/", 1)
		}
	}
	return normalized
}

// MustJSON 将任意对象编码成紧凑 JSON 字符串。
func MustJSON(value any) string {
	payload, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(payload)
}
