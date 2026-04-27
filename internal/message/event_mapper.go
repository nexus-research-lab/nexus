package message

import (
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

// EventMapperOptions 描述 SDK 消息到 Nexus 事件的映射策略。
type EventMapperOptions struct {
	Context                MessageContext
	InitialSessionID       string
	CausedBy               string
	IncludeStreamLifecycle bool
}

// EventMapResult 表示一次 SDK 消息映射后的事件与持久消息。
type EventMapResult struct {
	Events          []protocol.EventMessage
	DurableMessages []protocol.Message
	TerminalStatus  string
	ResultSubtype   string
}

// EventMapper 基于统一 Processor 生成场景化 protocol event。
type EventMapper struct {
	ctx                    MessageContext
	causedBy               string
	includeStreamLifecycle bool
	processor              *Processor
	lastAssistantMessage   protocol.Message
}

// NewEventMapper 创建通用 SDK 消息映射器。
func NewEventMapper(options EventMapperOptions) *EventMapper {
	return &EventMapper{
		ctx:                    options.Context,
		causedBy:               options.CausedBy,
		includeStreamLifecycle: options.IncludeStreamLifecycle,
		processor:              NewProcessor(options.Context, options.InitialSessionID),
	}
}

// Map 将一条 SDK 消息映射为 protocol event 与 durable message。
func (m *EventMapper) Map(incoming sdkprotocol.ReceivedMessage, interruptReason ...string) (EventMapResult, error) {
	output := m.processor.Process(incoming)
	if output.Err != nil {
		return EventMapResult{}, output.Err
	}
	NormalizeInterruptedOutput(&output, firstNonEmpty(interruptReason...))

	events := make([]protocol.EventMessage, 0, len(output.StreamEvents)+len(output.DurableMessages)+len(output.EphemeralMessages)+2)
	durableMessages := make([]protocol.Message, 0, len(output.DurableMessages))
	if m.includeStreamLifecycle && output.StreamStarted {
		events = append(events, m.wrapEvent(protocol.EventTypeStreamStart, map[string]any{
			"msg_id":   m.processor.CurrentMessageID(),
			"round_id": m.ctx.RoundID,
		}, m.processor.CurrentMessageID()))
	}
	for _, streamEvent := range output.StreamEvents {
		events = append(events, m.wrapEvent(protocol.EventTypeStream, streamEvent.Data, streamEvent.MessageID))
	}
	for _, messageValue := range output.DurableMessages {
		copyValue := protocol.Clone(messageValue)
		durableMessages = append(durableMessages, copyValue)
		projectedValue := m.projectDurableMessage(copyValue)
		events = append(events, m.wrapMessageEvent(projectedValue, true))
		if m.includeStreamLifecycle && messageValue["role"] == "assistant" && messageValue["is_complete"] == true {
			events = append(events, m.wrapEvent(protocol.EventTypeStreamEnd, map[string]any{
				"msg_id":   messageValue["message_id"],
				"round_id": m.ctx.RoundID,
			}, normalizeString(messageValue["message_id"])))
		}
	}
	for _, messageValue := range output.EphemeralMessages {
		events = append(events, m.wrapMessageEvent(protocol.Clone(messageValue), false))
	}
	return EventMapResult{
		Events:          events,
		DurableMessages: durableMessages,
		TerminalStatus:  output.TerminalStatus,
		ResultSubtype:   output.ResultSubtype,
	}, nil
}

// CurrentMessageID 返回当前 assistant message_id。
func (m *EventMapper) CurrentMessageID() string {
	return m.processor.CurrentMessageID()
}

// SessionID 返回当前 SDK session_id。
func (m *EventMapper) SessionID() string {
	return m.processor.SessionID()
}

// LastAssistantMessage 返回最近一条 assistant 快照。
func (m *EventMapper) LastAssistantMessage() protocol.Message {
	if len(m.lastAssistantMessage) == 0 {
		return nil
	}
	return protocol.Clone(m.lastAssistantMessage)
}

// ProjectResultMessage 将 result 投影回最近一条 assistant 快照。
func (m *EventMapper) ProjectResultMessage(message protocol.Message) protocol.Message {
	projected := ProjectResultMessage(m.lastAssistantMessage, message)
	m.lastAssistantMessage = protocol.Clone(projected)
	return projected
}

func (m *EventMapper) projectDurableMessage(message protocol.Message) protocol.Message {
	if message["role"] == "assistant" {
		m.lastAssistantMessage = protocol.Clone(message)
		return message
	}
	if message["role"] != "result" {
		return message
	}
	return m.ProjectResultMessage(message)
}

func (m *EventMapper) wrapMessageEvent(message protocol.Message, durable bool) protocol.EventMessage {
	event := m.wrapEvent(protocol.EventTypeMessage, message, normalizeString(message["message_id"]))
	if durable {
		event.DeliveryMode = "durable"
	}
	return event
}

func (m *EventMapper) wrapEvent(eventType protocol.EventType, data map[string]any, messageID string) protocol.EventMessage {
	event := protocol.NewEvent(eventType, data)
	event.SessionKey = m.ctx.SessionKey
	event.RoomID = m.ctx.RoomID
	event.ConversationID = m.ctx.ConversationID
	event.AgentID = m.ctx.AgentID
	event.MessageID = normalizeString(messageID)
	event.CausedBy = m.causedBy
	if sessionID := normalizeString(data["session_id"]); sessionID != "" {
		event.SessionID = sessionID
	}
	if event.Timestamp == 0 {
		event.Timestamp = time.Now().UnixMilli()
	}
	return event
}
