package room

import (
	"time"

	"github.com/nexus-research-lab/nexus/internal/message"
	sessionmodel "github.com/nexus-research-lab/nexus/internal/model/session"
	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

type slotMessageMapper struct {
	sessionKey     string
	roomID         string
	conversationID string
	agentID        string
	slotMessageID  string
	agentRoundID   string
	processor      *message.Processor

	lastAssistantMessage sessionmodel.Message
}

func newSlotMessageMapper(
	sessionKey string,
	roomID string,
	conversationID string,
	agentID string,
	slotMessageID string,
	agentRoundID string,
) *slotMessageMapper {
	return &slotMessageMapper{
		sessionKey:     sessionKey,
		roomID:         roomID,
		conversationID: conversationID,
		agentID:        agentID,
		slotMessageID:  slotMessageID,
		agentRoundID:   agentRoundID,
		processor: message.NewProcessor(message.MessageContext{
			SessionKey:     sessionKey,
			RoomID:         roomID,
			ConversationID: conversationID,
			AgentID:        agentID,
			RoundID:        agentRoundID,
			ParentID:       slotMessageID,
		}, ""),
	}
}

func (m *slotMessageMapper) Map(
	incoming sdkprotocol.ReceivedMessage,
	interruptReason ...string,
) ([]protocol.EventMessage, []sessionmodel.Message, string, error) {
	output := m.processor.Process(incoming)
	if output.Err != nil {
		return nil, nil, "", output.Err
	}
	message.NormalizeInterruptedOutput(&output, firstNonEmpty(interruptReason...))
	events := make([]protocol.EventMessage, 0, len(output.StreamEvents)+len(output.DurableMessages)+len(output.EphemeralMessages))
	messages := make([]sessionmodel.Message, 0, len(output.DurableMessages))
	for _, streamEvent := range output.StreamEvents {
		events = append(events, m.wrapEvent(protocol.EventTypeStream, streamEvent.Data, streamEvent.MessageID))
	}
	for _, messageValue := range output.DurableMessages {
		copyValue := sessionmodel.Clone(messageValue)
		messages = append(messages, copyValue)
		projectedValue := m.projectDurableMessage(copyValue)
		events = append(events, m.wrapMessageEvent(projectedValue))
	}
	for _, messageValue := range output.EphemeralMessages {
		events = append(events, m.wrapEphemeralMessage(sessionmodel.Clone(messageValue)))
	}
	return events, messages, output.TerminalStatus, nil
}

func (m *slotMessageMapper) projectDurableMessage(message sessionmodel.Message) sessionmodel.Message {
	if message["role"] == "assistant" {
		m.lastAssistantMessage = sessionmodel.Clone(message)
		return message
	}
	if message["role"] != "result" {
		return message
	}
	return m.ProjectResultMessage(message)
}

func (m *slotMessageMapper) CurrentMessageID() string {
	return m.processor.CurrentMessageID()
}

func (m *slotMessageMapper) SessionID() string {
	return m.processor.SessionID()
}

func (m *slotMessageMapper) LastAssistantMessage() sessionmodel.Message {
	if len(m.lastAssistantMessage) == 0 {
		return nil
	}
	return sessionmodel.Clone(m.lastAssistantMessage)
}

func (m *slotMessageMapper) ProjectResultMessage(message sessionmodel.Message) sessionmodel.Message {
	projected := sessionmodel.ProjectResultMessage(m.lastAssistantMessage, message)
	m.lastAssistantMessage = sessionmodel.Clone(projected)
	return projected
}

func (m *slotMessageMapper) wrapMessageEvent(message sessionmodel.Message) protocol.EventMessage {
	event := m.wrapEvent(protocol.EventTypeMessage, message, anyString(message["message_id"]))
	event.DeliveryMode = "durable"
	return event
}

func (m *slotMessageMapper) wrapEphemeralMessage(message sessionmodel.Message) protocol.EventMessage {
	return m.wrapEvent(protocol.EventTypeMessage, message, anyString(message["message_id"]))
}

func (m *slotMessageMapper) wrapEvent(eventType protocol.EventType, data map[string]any, messageID string) protocol.EventMessage {
	event := protocol.NewEvent(eventType, data)
	event.SessionKey = m.sessionKey
	event.RoomID = m.roomID
	event.ConversationID = m.conversationID
	event.AgentID = m.agentID
	event.MessageID = messageID
	event.CausedBy = m.agentRoundID
	if sessionID := anyString(data["session_id"]); sessionID != "" {
		event.SessionID = sessionID
	}
	if event.Timestamp == 0 {
		event.Timestamp = time.Now().UnixMilli()
	}
	return event
}

func anyString(value any) string {
	typed, ok := value.(string)
	if !ok {
		return ""
	}
	return typed
}

func intValue(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
