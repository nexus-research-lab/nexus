package chat

import (
	"time"

	messagepkg "github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

type messageMapper struct {
	sessionKey string
	agentID    string
	roundID    string
	processor  *messagepkg.Processor

	lastAssistantMessage protocol.Message
}

func newMessageMapper(sessionKey string, agentID string, roundID string) *messageMapper {
	return &messageMapper{
		sessionKey: sessionKey,
		agentID:    agentID,
		roundID:    roundID,
		processor: messagepkg.NewProcessor(messagepkg.MessageContext{
			SessionKey: sessionKey,
			AgentID:    agentID,
			RoundID:    roundID,
			ParentID:   roundID,
		}, ""),
	}
}

func (m *messageMapper) Map(incoming sdkprotocol.ReceivedMessage, interruptReason ...string) ([]protocol.EventMessage, []protocol.Message, string, string, error) {
	output := m.processor.Process(incoming)
	if output.Err != nil {
		return nil, nil, "", "", output.Err
	}
	messagepkg.NormalizeInterruptedOutput(&output, firstNonEmpty(interruptReason...))
	events := make([]protocol.EventMessage, 0, len(output.StreamEvents)+len(output.DurableMessages)+len(output.EphemeralMessages)+2)
	durableMessages := make([]protocol.Message, 0, len(output.DurableMessages))
	if output.StreamStarted {
		events = append(events, m.wrapEvent(protocol.EventTypeStreamStart, map[string]any{
			"msg_id":   m.processor.CurrentMessageID(),
			"round_id": m.roundID,
		}, m.processor.CurrentMessageID()))
	}
	for _, streamEvent := range output.StreamEvents {
		events = append(events, m.wrapEvent(protocol.EventTypeStream, streamEvent.Data, streamEvent.MessageID))
	}
	for _, messageValue := range output.DurableMessages {
		copyValue := protocol.Clone(messageValue)
		durableMessages = append(durableMessages, copyValue)
		projectedValue := m.projectDurableMessage(copyValue)
		events = append(events, m.wrapDurableMessage(projectedValue))
		if messageValue["role"] == "assistant" && messageValue["is_complete"] == true {
			events = append(events, m.wrapEvent(protocol.EventTypeStreamEnd, map[string]any{
				"msg_id":   messageValue["message_id"],
				"round_id": m.roundID,
			}, mapperString(messageValue["message_id"])))
		}
	}
	for _, messageValue := range output.EphemeralMessages {
		events = append(events, m.wrapEphemeralMessage(protocol.Clone(messageValue)))
	}
	return events, durableMessages, output.TerminalStatus, output.ResultSubtype, nil
}

func (m *messageMapper) CurrentMessageID() string {
	return m.processor.CurrentMessageID()
}

func (m *messageMapper) SessionID() string {
	return m.processor.SessionID()
}

func (m *messageMapper) LastAssistantMessage() protocol.Message {
	if len(m.lastAssistantMessage) == 0 {
		return nil
	}
	return protocol.Clone(m.lastAssistantMessage)
}

func (m *messageMapper) ProjectResultMessage(message protocol.Message) protocol.Message {
	projected := messagepkg.ProjectResultMessage(m.lastAssistantMessage, message)
	m.lastAssistantMessage = protocol.Clone(projected)
	return projected
}

func (m *messageMapper) wrapDurableMessage(payload protocol.Message) protocol.EventMessage {
	event := m.wrapEvent(protocol.EventTypeMessage, payload, mapperString(payload["message_id"]))
	event.DeliveryMode = "durable"
	return event
}

func (m *messageMapper) wrapEphemeralMessage(payload protocol.Message) protocol.EventMessage {
	return m.wrapEvent(protocol.EventTypeMessage, payload, mapperString(payload["message_id"]))
}

func (m *messageMapper) wrapEvent(eventType protocol.EventType, data map[string]any, messageID string) protocol.EventMessage {
	event := protocol.NewEvent(eventType, data)
	event.SessionKey = m.sessionKey
	event.AgentID = m.agentID
	event.MessageID = mapperString(messageID)
	if sessionID := mapperString(data["session_id"]); sessionID != "" {
		event.SessionID = sessionID
	}
	if event.Timestamp == 0 {
		event.Timestamp = time.Now().UnixMilli()
	}
	return event
}

func (m *messageMapper) projectDurableMessage(message protocol.Message) protocol.Message {
	if message["role"] == "assistant" {
		m.lastAssistantMessage = protocol.Clone(message)
		return message
	}
	if message["role"] != "result" {
		return message
	}
	return m.ProjectResultMessage(message)
}

func mapperString(value any) string {
	typed, ok := value.(string)
	if !ok {
		return ""
	}
	return typed
}
