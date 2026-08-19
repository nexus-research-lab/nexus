// [INPUT]: 依赖会话/运行时跨边界状态与时间戳。
// [OUTPUT]: 对外提供统一事件类型、消息恢复边界、请求 ACK、执行/待确认活动快照与 WorkGraph/Subagent 失效事件。
// [POS]: protocol 包的 WebSocket 事件真相源。
package protocol

import (
	"errors"
	"strings"
	"time"
)

// EventType 表示统一事件类型。
type EventType string

// RequestAckTimeoutMS 是客户端等待请求 ACK 的上限（毫秒）。
// 服务端不强制该窗口，但承诺在此之前回 ack；
// 前端据此设置本地超时，两侧同源避免漂移。
const RequestAckTimeoutMS = 10000

// ChatAckTimeoutMS 保留 chat_ack 的兼容名称；所有请求 ACK 共用同一超时契约。
const ChatAckTimeoutMS = RequestAckTimeoutMS

const (
	EventTypeMessage                     EventType = "message"
	EventTypeStream                      EventType = "stream"
	EventTypeChatAck                     EventType = "chat_ack"
	EventTypeInputQueue                  EventType = "input_queue"
	EventTypeInputQueueAck               EventType = "input_queue_ack"
	EventTypeInterruptAck                EventType = "interrupt_ack"
	EventTypeRoundStatus                 EventType = "round_status"
	EventTypeAgentRoundStatus            EventType = "agent_round_status"
	EventTypeSessionStatus               EventType = "session_status"
	EventTypeRuntimeStatus               EventType = "runtime_status"
	EventTypeCommandCatalog              EventType = "command_catalog"
	EventTypeContextUsage                EventType = "context_usage"
	EventTypeGoalCreated                 EventType = "goal_created"
	EventTypeGoalUpdated                 EventType = "goal_updated"
	EventTypeGoalStatusChanged           EventType = "goal_status_changed"
	EventTypeGoalProgress                EventType = "goal_progress"
	EventTypeGoalContinuation            EventType = "goal_continuation"
	EventTypeGoalCleared                 EventType = "goal_cleared"
	EventTypeExecutionInvalidated        EventType = "execution_invalidated"
	EventTypePermissionRequest           EventType = "permission_request"
	EventTypePermissionRequestResolved   EventType = "permission_request_resolved"
	EventTypeChannelAuthorization        EventType = "channel_authorization"
	EventTypeChannelAuthorizationResult  EventType = "channel_authorization_result"
	EventTypeAgentRuntimeEvent           EventType = "agent_runtime_event"
	EventTypeWorkspaceEvent              EventType = "workspace_event"
	EventTypeDirectoryChanged            EventType = "directory_changed"
	EventTypeScheduledTaskChanged        EventType = "scheduled_task_changed"
	EventTypeSubagentTaskChanged         EventType = "subagent_task_changed"
	EventTypeRoomMemberAdded             EventType = "room_member_added"
	EventTypeRoomMemberRemoved           EventType = "room_member_removed"
	EventTypeRoomMemberParticipation     EventType = "room_member_participation_changed"
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

const (
	RoundStatusRunning     = "running"
	RoundStatusFinished    = "finished"
	RoundStatusInterrupted = "interrupted"
	RoundStatusError       = "error"
)

const (
	DeliveryModeDurable   = "durable"
	DeliveryModeEphemeral = "ephemeral"
	DeliveryModeTransient = "transient"
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
	Message       string `json:"message,omitempty"`
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

// ExecutionInvalidationData 告知同 owner/session 的读取方重新拉取最新
// managed WorkGraph。ExecutionID 可在撤销或尚无图时为空；Version 为 0
// 表示调用方不能提供单个 Execution 的版本，但事件仍使目标 session 失效。
type ExecutionInvalidationData struct {
	ExecutionID string `json:"execution_id"`
	Version     int64  `json:"version"`
}

// ChannelAuthorizationData 只承载原生 UI 所需的人类展示材料。
// principal、Agent、session、round 与 runtime lease 绑定永不进入 wire。
type ChannelAuthorizationData struct {
	FlowID            string    `json:"flow_id"`
	PresentationToken string    `json:"presentation_token"`
	Kind              string    `json:"kind"`
	ChannelType       string    `json:"channel_type"`
	AccountBinding    string    `json:"account_binding"`
	QRPayload         string    `json:"qr_payload,omitempty"`
	QRPayloadType     string    `json:"qr_payload_type,omitempty"`
	Prompt            string    `json:"prompt"`
	ExpiresAt         time.Time `json:"expires_at"`
}

// ChannelAuthorizationResultData 是验证码控制提交的无敏感值 ACK。
type ChannelAuthorizationResultData struct {
	FlowID   string `json:"flow_id"`
	Accepted bool   `json:"accepted"`
	Status   string `json:"status,omitempty"`
	Message  string `json:"message"`
}

// CommandCatalogStatus 表示当前 session 命令目录的加载状态。
type CommandCatalogStatus string

const (
	CommandCatalogStatusCold        CommandCatalogStatus = "cold"
	CommandCatalogStatusReady       CommandCatalogStatus = "ready"
	CommandCatalogStatusUnavailable CommandCatalogStatus = "unavailable"
)

// CommandExecution 表示命令由 Nexus 宿主还是 runtime 解释。
type CommandExecution string

const (
	CommandExecutionHost        CommandExecution = "host"
	CommandExecutionRuntime     CommandExecution = "runtime"
	CommandExecutionUnsupported CommandExecution = "unsupported"
)

// CommandDescriptor 是浏览器可见的命令描述。
// runtime 字段严格对齐 Claude Agent SDK，Execution 仅描述 Nexus 的发送路径。
type CommandDescriptor struct {
	Name           string           `json:"name"`
	Description    string           `json:"description,omitempty"`
	ArgumentHint   string           `json:"argument_hint,omitempty"`
	Execution      CommandExecution `json:"execution"`
	Enabled        bool             `json:"enabled"`
	DisabledReason string           `json:"disabled_reason,omitempty"`
}

// CommandCatalogData 表示一个 Agent runtime 的命令目录快照。
type CommandCatalogData struct {
	Revision    string               `json:"revision,omitempty"`
	Generation  uint64               `json:"generation,omitempty"`
	RuntimeKind string               `json:"runtime_kind,omitempty"`
	Status      CommandCatalogStatus `json:"status"`
	AgentID     string               `json:"agent_id,omitempty"`
	Commands    []CommandDescriptor  `json:"commands"`
}

// NewEvent 构造通用事件。
func NewEvent(eventType EventType, data map[string]any) EventMessage {
	return EventMessage{
		ProtocolVersion: 2,
		DeliveryMode:    DeliveryModeEphemeral,
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

// NewExecutionInvalidatedEvent 构造 session-scoped WorkGraph 失效通知。
func NewExecutionInvalidatedEvent(
	sessionKey string,
	data ExecutionInvalidationData,
) EventMessage {
	event := NewEvent(EventTypeExecutionInvalidated, map[string]any{
		"execution_id": strings.TrimSpace(data.ExecutionID),
		"version":      data.Version,
	})
	event.SessionKey = strings.TrimSpace(sessionKey)
	return event
}

// NewChannelAuthorizationEvent 构造只投递给原始认证会话的原生授权卡事件。
func NewChannelAuthorizationEvent(
	sessionKey string,
	data ChannelAuthorizationData,
) EventMessage {
	event := NewEvent(EventTypeChannelAuthorization, map[string]any{
		"flow_id":            data.FlowID,
		"presentation_token": data.PresentationToken,
		"kind":               data.Kind,
		"channel_type":       data.ChannelType,
		"account_binding":    data.AccountBinding,
		"prompt":             data.Prompt,
		"expires_at":         data.ExpiresAt,
	})
	if strings.TrimSpace(data.QRPayload) != "" {
		event.Data["qr_payload"] = data.QRPayload
	}
	if strings.TrimSpace(data.QRPayloadType) != "" {
		event.Data["qr_payload_type"] = data.QRPayloadType
	}
	event.SessionKey = strings.TrimSpace(sessionKey)
	return event
}

// NewChannelAuthorizationResultEvent 构造不回显验证码的控制提交结果。
func NewChannelAuthorizationResultEvent(
	sessionKey string,
	data ChannelAuthorizationResultData,
) EventMessage {
	event := NewEvent(EventTypeChannelAuthorizationResult, map[string]any{
		"flow_id":  data.FlowID,
		"accepted": data.Accepted,
		"message":  data.Message,
	})
	if strings.TrimSpace(data.Status) != "" {
		event.Data["status"] = strings.TrimSpace(data.Status)
	}
	event.SessionKey = strings.TrimSpace(sessionKey)
	return event
}

// NewCommandCatalogEvent 构造 session 作用域的命令目录事件。
func NewCommandCatalogEvent(sessionKey string, data CommandCatalogData) EventMessage {
	data.Revision = strings.TrimSpace(data.Revision)
	data.RuntimeKind = strings.TrimSpace(data.RuntimeKind)
	data.AgentID = strings.TrimSpace(data.AgentID)
	commands := data.Commands
	if commands == nil {
		commands = []CommandDescriptor{}
	}
	payload := map[string]any{
		"status":   data.Status,
		"commands": commands,
	}
	if data.Revision != "" {
		payload["revision"] = data.Revision
	}
	if data.Generation > 0 {
		payload["generation"] = data.Generation
	}
	if data.RuntimeKind != "" {
		payload["runtime_kind"] = data.RuntimeKind
	}
	if data.AgentID != "" {
		payload["agent_id"] = data.AgentID
	}
	event := NewEvent(EventTypeCommandCatalog, payload)
	event.SessionKey = sessionKey
	event.AgentID = data.AgentID
	return event
}

// NewContextUsageEvent 构造 Agent session 作用域的上下文占用事件。
func NewContextUsageEvent(
	sessionKey string,
	agentID string,
	data ContextUsageData,
) EventMessage {
	payload := map[string]any{
		"total_tokens": data.TotalTokens,
		"max_tokens":   data.MaxTokens,
		"percentage":   data.Percentage,
	}
	if model := strings.TrimSpace(data.Model); model != "" {
		payload["model"] = model
	}
	event := NewEvent(EventTypeContextUsage, payload)
	event.SessionKey = strings.TrimSpace(sessionKey)
	event.AgentID = strings.TrimSpace(agentID)
	return event
}

// NewRoundStatusEvent 构造 round_status 事件。
func NewRoundStatusEvent(sessionKey string, roundID string, status string, resultSubtype string) EventMessage {
	data := map[string]any{
		"round_id":    roundID,
		"status":      status,
		"is_terminal": IsTerminalRoundStatus(status),
	}
	if strings.TrimSpace(resultSubtype) != "" {
		data["result_subtype"] = strings.TrimSpace(resultSubtype)
	}
	event := NewEvent(EventTypeRoundStatus, data)
	event.SessionKey = sessionKey
	return event
}

// NewRoundStatusErrorEvent 构造带可展示错误原因的失败轮次事件。
// error 事件本身是瞬时通知；把原因同时放进 round_status，客户端即使错过前一个事件，
// 仍能在轮次收口时给用户一个明确反馈。
func NewRoundStatusErrorEvent(sessionKey string, roundID string, message string) EventMessage {
	event := NewRoundStatusEvent(
		sessionKey,
		roundID,
		RoundStatusError,
		"error",
	)
	if trimmed := strings.TrimSpace(message); trimmed != "" {
		event.Data["message"] = trimmed
	}
	return event
}

// ChatAckPendingSlot 表示 chat_ack 中一个 agent slot 的占位信息。
// RoundID 在跨多个 root 的权威快照中必填；普通单 root ACK 可沿用事件 round_id。
type ChatAckPendingSlot struct {
	AgentID        string `json:"agent_id"`
	AgentRoundID   string `json:"agent_round_id"`
	MsgID          string `json:"msg_id"`
	RoundID        string `json:"round_id,omitempty"`
	HandoffID      string `json:"handoff_id,omitempty"`
	HiddenFromUser bool   `json:"hidden_from_user,omitempty"`
	Status         string `json:"status"`
	Timestamp      int64  `json:"timestamp"`
	Index          int    `json:"index"`
}

// NewChatAckEvent 构造 chat_ack 事件。round_id / user_message_id 由后端 mint，
// client_request_id / client_message_id 原样回传供前端关联。
func NewChatAckEvent(
	sessionKey string,
	clientRequestID string,
	clientMessageID string,
	roundID string,
	userMessageID string,
	userMessageCommitted bool,
	pending []ChatAckPendingSlot,
) EventMessage {
	if pending == nil {
		pending = []ChatAckPendingSlot{}
	}
	event := NewEvent(EventTypeChatAck, map[string]any{
		"client_request_id":      clientRequestID,
		"client_message_id":      clientMessageID,
		"round_id":               roundID,
		"user_message_id":        userMessageID,
		"user_message_committed": userMessageCommitted,
		"pending":                pending,
		"pending_snapshot":       false,
		"ack_timeout_ms":         ChatAckTimeoutMS,
	})
	event.SessionKey = sessionKey
	return event
}

// NewTransientChatAckEvent 确认一条只保留在当前时间线的用户输入。
//
// Host Slash 不进入 runtime 历史，但仍应保留用户实际执行的指令，避免 ACK
// 把 optimistic 消息误删后只剩一条无来源的宿主确认。
func NewTransientChatAckEvent(
	sessionKey string,
	clientRequestID string,
	clientMessageID string,
	roundID string,
	userMessageID string,
) EventMessage {
	event := NewChatAckEvent(
		sessionKey,
		clientRequestID,
		clientMessageID,
		roundID,
		userMessageID,
		false,
		nil,
	)
	event.Data["user_message_delivery_mode"] = DeliveryModeTransient
	return event
}

// NewInterruptAckEvent 确认一个精确停止请求已完成。ACK 只回显客户端请求身份
// 与停止目标；agent_round_status 仍是 Room slot 生命周期的权威事件。
func NewInterruptAckEvent(
	sessionKey string,
	clientRequestID string,
	roundID string,
	agentRoundID string,
) EventMessage {
	event := NewEvent(EventTypeInterruptAck, map[string]any{
		"accepted":          true,
		"client_request_id": strings.TrimSpace(clientRequestID),
		"round_id":          strings.TrimSpace(roundID),
		"agent_round_id":    strings.TrimSpace(agentRoundID),
		"ack_timeout_ms":    RequestAckTimeoutMS,
	})
	event.SessionKey = sessionKey
	event.RoundID = strings.TrimSpace(roundID)
	event.AgentRoundID = strings.TrimSpace(agentRoundID)
	return event
}

// NewChatPendingSnapshotEvent 构造订阅恢复时的权威 Room 执行与人工交互快照。
// 前端必须整体替换本地恢复值；空数组同样有意义，用于清除陈旧状态。
func NewChatPendingSnapshotEvent(
	sessionKey string,
	roundID string,
	pending []ChatAckPendingSlot,
	pendingInteractionRequestIDs []string,
) EventMessage {
	event := NewChatAckEvent(sessionKey, "", "", roundID, "", false, pending)
	event.Data["pending_snapshot"] = true
	if pendingInteractionRequestIDs == nil {
		pendingInteractionRequestIDs = []string{}
	}
	event.Data["pending_interaction_request_ids"] = pendingInteractionRequestIDs
	return event
}

// NewChatPendingInteractionSnapshotEvent 构造 Room 全局订阅恢复所需的人工交互快照。
// 它不声称拥有任何具体 conversation 的执行 slot，避免误清其他会话的工作态。
func NewChatPendingInteractionSnapshotEvent(pendingInteractionRequestIDs []string) EventMessage {
	if pendingInteractionRequestIDs == nil {
		pendingInteractionRequestIDs = []string{}
	}
	event := NewEvent(EventTypeChatAck, map[string]any{
		"pending_interaction_snapshot":    true,
		"pending_interaction_request_ids": pendingInteractionRequestIDs,
	})
	return event
}

// IsTerminalRoundStatus 判断 round / slot 状态是否终态。
func IsTerminalRoundStatus(status string) bool {
	switch status {
	case RoundStatusFinished, RoundStatusInterrupted, RoundStatusError:
		return true
	default:
		return false
	}
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
