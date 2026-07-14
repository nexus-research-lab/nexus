// [INPUT]: 依赖会话/运行时跨边界状态与时间戳。
// [OUTPUT]: 对外提供统一事件类型、普通 chat ack 与权威 pending slot 快照事件。
// [POS]: protocol 包的 WebSocket 事件真相源。
package protocol

import (
	"errors"
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
	EventTypeMessage                     EventType = "message"
	EventTypeStream                      EventType = "stream"
	EventTypeChatAck                     EventType = "chat_ack"
	EventTypeInputQueue                  EventType = "input_queue"
	EventTypeRoundStatus                 EventType = "round_status"
	EventTypeAgentRoundStatus            EventType = "agent_round_status"
	EventTypeSessionStatus               EventType = "session_status"
	EventTypeRuntimeStatus               EventType = "runtime_status"
	EventTypeGoalCreated                 EventType = "goal_created"
	EventTypeGoalUpdated                 EventType = "goal_updated"
	EventTypeGoalStatusChanged           EventType = "goal_status_changed"
	EventTypeGoalProgress                EventType = "goal_progress"
	EventTypeGoalContinuation            EventType = "goal_continuation"
	EventTypeGoalCleared                 EventType = "goal_cleared"
	EventTypePermissionRequest           EventType = "permission_request"
	EventTypePermissionRequestResolved   EventType = "permission_request_resolved"
	EventTypeAgentRuntimeEvent           EventType = "agent_runtime_event"
	EventTypeWorkspaceEvent              EventType = "workspace_event"
	EventTypeDirectoryChanged            EventType = "directory_changed"
	EventTypeScheduledTaskChanged        EventType = "scheduled_task_changed"
	EventTypeRoomMemberAdded             EventType = "room_member_added"
	EventTypeRoomMemberRemoved           EventType = "room_member_removed"
	EventTypeRoomDeleted                 EventType = "room_deleted"
	EventTypeRoomDirectedMessage         EventType = "room_directed_message"
	EventTypeRoomDirectedMessageConsumed EventType = "room_directed_message_consumed"
	EventTypeRoomResyncRequired          EventType = "room_resync_required"
	EventTypeSessionResyncRequired       EventType = "session_resync_required"
	EventTypeStreamStart                 EventType = "stream_start"
	EventTypeStreamEnd                   EventType = "stream_end"
	EventTypeStreamCancelled             EventType = "stream_cancelled"
	EventTypeError                       EventType = "error"
	EventTypePong                        EventType = "pong"
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
	RoundID         string         `json:"round_id,omitempty"`
	AgentRoundID    string         `json:"agent_round_id,omitempty"`
	Data            map[string]any `json:"data"`
	Timestamp       int64          `json:"timestamp"`
}

// InboundWebSocketMessage 表示前端发送给服务端的基础消息。
type InboundWebSocketMessage struct {
	Type       string `json:"type"`
	SessionKey string `json:"session_key,omitempty"`
}

type clientMessageError interface {
	ClientMessage() string
}

// ClientErrorMessage 只读取业务错误显式声明的安全文案，未知内部错误不得穿透到客户端。
func ClientErrorMessage(err error) (string, bool) {
	var target clientMessageError
	if !errors.As(err, &target) {
		return "", false
	}
	message := strings.TrimSpace(target.ClientMessage())
	return message, message != ""
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
	IsGenerating    bool     `json:"is_generating"`
	RunningRoundIDs []string `json:"running_round_ids,omitempty"`
}

// RuntimeStatus 表示当前会话内由 Agent runtime 主动上报的瞬时阶段。
type RuntimeStatus string

const (
	RuntimeStatusCompacting RuntimeStatus = "compacting"
)

// RuntimeStatusData 使用 nil 明确结束上一状态，避免客户端依赖轮次事件猜测。
type RuntimeStatusData struct {
	Status *RuntimeStatus `json:"status"`
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

// ChatAckPendingSlot 表示 chat_ack 中一个 agent slot 的占位信息。
type ChatAckPendingSlot struct {
	AgentID      string `json:"agent_id"`
	AgentRoundID string `json:"agent_round_id"`
	MsgID        string `json:"msg_id"`
	Status       string `json:"status"`
	Timestamp    int64  `json:"timestamp"`
	Index        int    `json:"index"`
}

// NewChatAckEvent 构造 chat_ack 事件。round_id / user_message_id 由后端 mint，
// client_request_id / client_message_id 原样回传供前端关联。
func NewChatAckEvent(sessionKey string, clientRequestID string, clientMessageID string, roundID string, userMessageID string, pending []ChatAckPendingSlot) EventMessage {
	if pending == nil {
		pending = []ChatAckPendingSlot{}
	}
	event := NewEvent(EventTypeChatAck, map[string]any{
		"client_request_id": clientRequestID,
		"client_message_id": clientMessageID,
		"round_id":          roundID,
		"user_message_id":   userMessageID,
		"pending":           pending,
		"pending_snapshot":  false,
		"ack_timeout_ms":    ChatAckTimeoutMS,
	})
	event.SessionKey = sessionKey
	return event
}

// NewChatPendingSnapshotEvent 构造订阅恢复时的权威 Room slot 快照。
// 前端必须用 pending 整体替换本地恢复值；空数组同样有意义，用于清除陈旧占位。
func NewChatPendingSnapshotEvent(sessionKey string, roundID string, pending []ChatAckPendingSlot) EventMessage {
	event := NewChatAckEvent(sessionKey, "", "", roundID, "", pending)
	event.Data["pending_snapshot"] = true
	return event
}

// IsTerminalRoundStatus 判断 round / slot 状态是否终态。
func IsTerminalRoundStatus(status string) bool {
	return status == "finished" || status == "interrupted" || status == "error"
}

// NewAgentRoundStatusEvent 构造 agent_round_status 事件（Room slot 生命周期）。
func NewAgentRoundStatusEvent(sessionKey string, roundID string, agentRoundID string, agentID string, status string) EventMessage {
	event := NewEvent(EventTypeAgentRoundStatus, map[string]any{
		"round_id":       roundID,
		"agent_round_id": agentRoundID,
		"agent_id":       agentID,
		"status":         status,
		"is_terminal":    IsTerminalRoundStatus(status),
	})
	event.SessionKey = sessionKey
	return event
}

// NewPermissionRequestResolvedEvent 构造权限请求结束事件。
func NewPermissionRequestResolvedEvent(sessionKey string, requestID string, status string) EventMessage {
	event := NewEvent(EventTypePermissionRequestResolved, map[string]any{
		"request_id": requestID,
		"status":     strings.TrimSpace(status),
	})
	event.SessionKey = sessionKey
	return event
}
