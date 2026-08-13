// INPUT: Processor 产出的流事件、durable/ephemeral 消息与 runtime 状态。
// OUTPUT: 带完整会话身份的协议事件、持久消息和 round 终态。
// POS: SDK 消息投影到 Nexus 事件协议的统一编排边界。
package message

import (
	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

// MessageDecorator 为 durable 消息及其最终 assistant 投影补充场景字段。
// 装饰器原地修改消息且必须非 nil，避免返回值表达含糊的丢弃语义。
type MessageDecorator func(protocol.Message)

func noopMessageDecorator(protocol.Message) {}

// EventMapperOptions 描述 SDK 消息到 Nexus 事件的映射策略。
type EventMapperOptions struct {
	Context                MessageContext
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
	includeStreamLifecycle bool
	processor              *Processor
	lastAssistantMessage   protocol.Message
	decorateMessage        MessageDecorator
}

// NewEventMapper 创建通用 SDK 消息映射器。
func NewEventMapper(options EventMapperOptions) *EventMapper {
	return &EventMapper{
		ctx:                    options.Context,
		includeStreamLifecycle: options.IncludeStreamLifecycle,
		processor:              NewProcessor(options.Context, ""),
		decorateMessage:        noopMessageDecorator,
	}
}

// SetMessageDecorator 设置 durable 消息及其最终 assistant 投影的场景装饰器。
func (m *EventMapper) SetMessageDecorator(decorator MessageDecorator) {
	m.decorateMessage = decorator
}

// Map 将一条 SDK 消息映射为 protocol event 与 durable message。
func (m *EventMapper) Map(incoming sdkprotocol.ReceivedMessage, interruptReason ...string) (EventMapResult, error) {
	output := m.processor.Process(incoming)
	if output.Err != nil {
		return EventMapResult{}, output.Err
	}
	NormalizeInterruptedOutput(&output, firstNonEmpty(interruptReason...))
	if output.ResultSubtype == "interrupted" {
		if partial := m.processor.FinalizeInterruptedAssistant(); len(partial) > 0 {
			output.DurableMessages = append([]protocol.Message{partial}, output.DurableMessages...)
		}
	}

	result := EventMapResult{
		Events:          make([]protocol.EventMessage, 0, len(output.StreamEvents)+len(output.DurableMessages)+len(output.EphemeralMessages)+2),
		DurableMessages: make([]protocol.Message, 0, len(output.DurableMessages)),
		TerminalStatus:  output.TerminalStatus,
		ResultSubtype:   output.ResultSubtype,
	}
	m.appendRuntimeStatus(&result, incoming.System)
	m.appendStreamEvents(&result, output)
	m.appendDurableMessages(&result, output.DurableMessages)
	m.appendEphemeralMessages(&result, output.EphemeralMessages)
	return result, nil
}

func (m *EventMapper) appendRuntimeStatus(result *EventMapResult, message *sdkprotocol.SystemMessage) {
	status, observed := projectRuntimeStatus(message)
	if !observed {
		return
	}
	result.Events = append(result.Events, m.wrapEvent(
		protocol.EventTypeRuntimeStatus,
		runtimeStatusEventData(status),
		"",
	))
}

func (m *EventMapper) appendStreamEvents(result *EventMapResult, output Output) {
	if m.includeStreamLifecycle && output.StreamStarted {
		messageID := m.processor.CurrentMessageID()
		result.Events = append(result.Events, m.wrapStreamLifecycleEvent(protocol.EventTypeStreamStart, messageID))
	}
	for _, streamEvent := range output.StreamEvents {
		result.Events = append(result.Events, m.wrapEvent(
			protocol.EventTypeStream,
			streamEvent.Data,
			streamEvent.MessageID,
		))
	}
}

func (m *EventMapper) appendDurableMessages(result *EventMapResult, messages []protocol.Message) {
	for _, messageValue := range messages {
		m.appendDurableMessage(result, messageValue)
	}
}

func (m *EventMapper) appendDurableMessage(result *EventMapResult, messageValue protocol.Message) {
	durable := protocol.Clone(messageValue)
	m.decorateMessage(durable)
	result.DurableMessages = append(result.DurableMessages, durable)

	projected := m.projectDurableMessage(durable)
	if len(projected) > 0 {
		result.Events = append(result.Events, m.wrapMessageEvent(projected, protocol.DeliveryModeDurable))
	}
	if m.includeStreamLifecycle && isCompletedAssistantMessage(durable) {
		result.Events = append(result.Events, m.wrapStreamLifecycleEvent(
			protocol.EventTypeStreamEnd,
			normalizeString(durable["message_id"]),
		))
	}
}

func (m *EventMapper) appendEphemeralMessages(result *EventMapResult, messages []protocol.Message) {
	for _, messageValue := range messages {
		result.Events = append(result.Events, m.wrapMessageEvent(
			protocol.Clone(messageValue),
			protocol.DeliveryModeEphemeral,
		))
	}
}

func isCompletedAssistantMessage(message protocol.Message) bool {
	return protocol.MessageRole(message) == "assistant" && message["is_complete"] == true
}

var runtimeStatusEventValues = map[protocol.RuntimeStatus]any{
	"":                               nil,
	protocol.RuntimeStatusCompacting: protocol.RuntimeStatusCompacting,
}

func runtimeStatusEventData(status protocol.RuntimeStatus) map[string]any {
	return map[string]any{"status": runtimeStatusEventValues[status]}
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

// FinalizeInterruptedAssistant 返回中断前已流出但尚无终态快照的 assistant。
func (m *EventMapper) FinalizeInterruptedAssistant() protocol.Message {
	partial := m.processor.FinalizeInterruptedAssistant()
	if len(partial) == 0 {
		return nil
	}
	m.decorateMessage(partial)
	return m.rememberAssistantMessage(partial)
}

// ProjectResultMessage 将 result 投影回最近一条 assistant 快照。
func (m *EventMapper) ProjectResultMessage(message protocol.Message) protocol.Message {
	projected := ProjectResultMessage(m.lastAssistantMessage, message)
	if len(projected) == 0 {
		return nil
	}
	m.decorateMessage(projected)
	return m.rememberAssistantMessage(projected)
}

func (m *EventMapper) projectDurableMessage(message protocol.Message) protocol.Message {
	switch protocol.MessageRole(message) {
	case "assistant":
		return m.rememberAssistantMessage(message)
	case "result":
		return m.ProjectResultMessage(message)
	default:
		return message
	}
}

func (m *EventMapper) rememberAssistantMessage(message protocol.Message) protocol.Message {
	m.lastAssistantMessage = protocol.Clone(message)
	return message
}

func (m *EventMapper) wrapStreamLifecycleEvent(eventType protocol.EventType, messageID string) protocol.EventMessage {
	return m.wrapEvent(eventType, map[string]any{
		"msg_id":   messageID,
		"round_id": m.ctx.RoundID,
	}, messageID)
}

func (m *EventMapper) wrapMessageEvent(
	message protocol.Message,
	deliveryMode string,
) protocol.EventMessage {
	event := m.wrapEvent(protocol.EventTypeMessage, message, normalizeString(message["message_id"]))
	event.DeliveryMode = deliveryMode
	return event
}

func (m *EventMapper) wrapEvent(
	eventType protocol.EventType,
	data map[string]any,
	messageID string,
) protocol.EventMessage {
	event := protocol.NewEvent(eventType, data)
	event.SessionKey = m.ctx.SessionKey
	event.RoomID = m.ctx.RoomID
	event.ConversationID = m.ctx.ConversationID
	event.AgentID = m.ctx.AgentID
	event.MessageID = normalizeString(messageID)
	event.RoundID = m.ctx.RoundID
	event.AgentRoundID = m.ctx.AgentRoundID
	event.SessionID = normalizeString(data["session_id"])
	return event
}
