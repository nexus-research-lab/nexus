// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：mapper.go
// @Date   ：2026/04/11 02:42:00
// @Author ：leemysw
// 2026/04/11 02:42:00   Create
// =====================================================

package chat

import (
	"time"

	"github.com/nexus-research-lab/nexus/internal/message"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/session"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

type messageMapper struct {
	sessionKey string
	agentID    string
	roundID    string
	processor  *message.Processor
}

func newMessageMapper(sessionKey string, agentID string, roundID string) *messageMapper {
	return &messageMapper{
		sessionKey: sessionKey,
		agentID:    agentID,
		roundID:    roundID,
		processor: message.NewProcessor(message.MessageContext{
			SessionKey: sessionKey,
			AgentID:    agentID,
			RoundID:    roundID,
			ParentID:   roundID,
		}, ""),
	}
}

func (m *messageMapper) Map(message sdkprotocol.ReceivedMessage) ([]protocol.EventMessage, string, string) {
	output := m.processor.Process(message)
	events := make([]protocol.EventMessage, 0, len(output.StreamEvents)+len(output.DurableMessages)+2)
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
		events = append(events, m.wrapDurableMessage(messageValue))
		if messageValue["role"] == "assistant" && messageValue["is_complete"] == true {
			events = append(events, m.wrapEvent(protocol.EventTypeStreamEnd, map[string]any{
				"msg_id":   messageValue["message_id"],
				"round_id": m.roundID,
			}, mapperString(messageValue["message_id"])))
		}
	}
	return events, output.TerminalStatus, output.ResultSubtype
}

func (m *messageMapper) CurrentMessageID() string {
	return m.processor.CurrentMessageID()
}

func (m *messageMapper) SessionID() string {
	return m.processor.SessionID()
}

func (m *messageMapper) wrapDurableMessage(payload session.Message) protocol.EventMessage {
	event := m.wrapEvent(protocol.EventTypeMessage, payload, mapperString(payload["message_id"]))
	event.DeliveryMode = "durable"
	return event
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

func mapperString(value any) string {
	typed, ok := value.(string)
	if !ok {
		return ""
	}
	return typed
}
