// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：message_mapper.go
// @Date   ：2026/04/11 03:46:00
// @Author ：leemysw
// 2026/04/11 03:46:00   Create
// =====================================================

package room

import (
	"encoding/json"
	"fmt"
	"github.com/nexus-research-lab/nexus-core/internal/protocol"
	"github.com/nexus-research-lab/nexus-core/internal/sessiondomain"
	"strings"
	"time"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

type slotMessageMapper struct {
	sessionKey      string
	roomID          string
	conversationID  string
	agentID         string
	slotMessageID   string
	agentRoundID    string
	streamModel     string
	streamUsage     map[string]any
	lastAssistantID string
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
		streamUsage:    map[string]any{},
	}
}

func (m *slotMessageMapper) Map(message sdkprotocol.ReceivedMessage) ([]protocol.EventMessage, *sessiondomain.Message, *sessiondomain.Message) {
	switch message.Type {
	case sdkprotocol.MessageTypeStreamEvent:
		return m.mapStream(message), nil, nil
	case sdkprotocol.MessageTypeAssistant:
		assistant := m.mapAssistant(message)
		return []protocol.EventMessage{m.wrapMessageEvent(assistant)}, &assistant, nil
	case sdkprotocol.MessageTypeResult:
		result := m.mapResult(message)
		return []protocol.EventMessage{m.wrapMessageEvent(result)}, nil, &result
	case sdkprotocol.MessageTypeToolProgress:
		system := m.mapToolProgress(message)
		return []protocol.EventMessage{m.wrapMessageEvent(system)}, &system, nil
	default:
		return nil, nil, nil
	}
}

func (m *slotMessageMapper) mapStream(message sdkprotocol.ReceivedMessage) []protocol.EventMessage {
	if message.Stream == nil {
		return nil
	}
	payload, ok := message.Stream.Event.(map[string]any)
	if !ok {
		payload = message.Stream.Data
	}
	eventType := strings.TrimSpace(anyString(payload["type"]))
	if eventType == "" {
		return nil
	}

	switch eventType {
	case "message_start":
		messagePayload, _ := payload["message"].(map[string]any)
		m.lastAssistantID = firstNonEmpty(anyString(messagePayload["id"]), m.lastAssistantID, "assistant_"+m.agentRoundID)
		m.streamModel = anyString(messagePayload["model"])
		if usage, ok := messagePayload["usage"].(map[string]any); ok {
			m.streamUsage = usage
		}
		return nil
	case "content_block_start":
		block := normalizeRoomContentBlock(payload["content_block"])
		if block["type"] == "tool_use" {
			return nil
		}
		return []protocol.EventMessage{m.wrapStreamEvent("content_block_start", intValue(payload["index"]), block)}
	case "content_block_delta":
		delta, _ := payload["delta"].(map[string]any)
		blockType := anyString(delta["type"])
		if blockType == "input_json_delta" {
			return nil
		}
		block := map[string]any{}
		if text := strings.TrimSpace(anyString(delta["text"])); text != "" {
			block["type"] = "text"
			block["text"] = text
		} else if thinking := strings.TrimSpace(anyString(delta["thinking"])); thinking != "" {
			block["type"] = "thinking"
			block["thinking"] = thinking
		}
		if len(block) == 0 {
			return nil
		}
		return []protocol.EventMessage{m.wrapStreamEvent("content_block_delta", intValue(payload["index"]), block)}
	case "message_delta":
		if usage, ok := payload["usage"].(map[string]any); ok {
			m.streamUsage = usage
		}
		return []protocol.EventMessage{m.wrapEvent(protocol.EventTypeStream, map[string]any{
			"message_id":      m.slotMessageID,
			"session_key":     m.sessionKey,
			"room_id":         m.roomID,
			"conversation_id": m.conversationID,
			"agent_id":        m.agentID,
			"round_id":        m.agentRoundID,
			"type":            "message_delta",
			"usage":           m.streamUsage,
			"timestamp":       time.Now().UnixMilli(),
		}, m.slotMessageID)}
	default:
		return nil
	}
}

func (m *slotMessageMapper) mapAssistant(message sdkprotocol.ReceivedMessage) sessiondomain.Message {
	messageID := firstNonEmpty(message.Assistant.Message.ID, m.lastAssistantID, "assistant_"+m.agentRoundID)
	payload := sessiondomain.Message{
		"message_id":      messageID,
		"session_key":     m.sessionKey,
		"room_id":         m.roomID,
		"conversation_id": m.conversationID,
		"agent_id":        m.agentID,
		"round_id":        m.agentRoundID,
		"session_id":      firstNonEmpty(message.SessionID, ""),
		"parent_id":       m.slotMessageID,
		"role":            "assistant",
		"timestamp":       time.Now().UnixMilli(),
		"content":         normalizeRoomConversationContent(message.Assistant.Message.Content),
		"model":           firstNonEmpty(message.Assistant.Message.Model, m.streamModel),
		"usage":           firstNonNilMap(message.Assistant.Message.Usage, m.streamUsage),
		"is_complete":     true,
	}
	if stopReason := strings.TrimSpace(fmt.Sprint(message.Assistant.Message.StopReason)); stopReason != "" && stopReason != "<nil>" {
		payload["stop_reason"] = stopReason
	}
	return payload
}

func (m *slotMessageMapper) mapResult(message sdkprotocol.ReceivedMessage) sessiondomain.Message {
	subtype := roomResultSubtype(message.Result)
	return sessiondomain.Message{
		"message_id":      "result_" + m.agentRoundID,
		"session_key":     m.sessionKey,
		"room_id":         m.roomID,
		"conversation_id": m.conversationID,
		"agent_id":        m.agentID,
		"round_id":        m.agentRoundID,
		"session_id":      firstNonEmpty(message.SessionID, ""),
		"parent_id":       m.slotMessageID,
		"role":            "result",
		"timestamp":       time.Now().UnixMilli(),
		"subtype":         subtype,
		"duration_ms":     message.Result.DurationMS,
		"duration_api_ms": message.Result.DurationAPIMS,
		"num_turns":       message.Result.NumTurns,
		"total_cost_usd":  message.Result.TotalCostUSD,
		"usage":           firstNonNilMap(message.Result.Usage, map[string]any{}),
		"result":          message.Result.Result,
		"is_error":        subtype == "error",
	}
}

func (m *slotMessageMapper) mapToolProgress(message sdkprotocol.ReceivedMessage) sessiondomain.Message {
	return sessiondomain.Message{
		"message_id":      fmt.Sprintf("progress_%s_%d", m.slotMessageID, time.Now().UnixMilli()),
		"session_key":     m.sessionKey,
		"room_id":         m.roomID,
		"conversation_id": m.conversationID,
		"agent_id":        m.agentID,
		"round_id":        m.agentRoundID,
		"role":            "system",
		"timestamp":       time.Now().UnixMilli(),
		"content":         firstNonEmpty(message.ToolProgress.ToolName, "后台任务") + " 正在执行",
		"metadata": map[string]any{
			"subtype":        "task_progress",
			"tool_use_id":    message.ToolProgress.ToolUseID,
			"last_tool_name": message.ToolProgress.ToolName,
			"task_id":        message.ToolProgress.TaskID,
		},
	}
}

func (m *slotMessageMapper) wrapMessageEvent(message sessiondomain.Message) protocol.EventMessage {
	event := m.wrapEvent(protocol.EventTypeMessage, message, anyString(message["message_id"]))
	event.DeliveryMode = "durable"
	return event
}

func (m *slotMessageMapper) wrapStreamEvent(streamType string, index int, block map[string]any) protocol.EventMessage {
	return m.wrapEvent(protocol.EventTypeStream, map[string]any{
		"message_id":      m.slotMessageID,
		"session_key":     m.sessionKey,
		"room_id":         m.roomID,
		"conversation_id": m.conversationID,
		"agent_id":        m.agentID,
		"round_id":        m.agentRoundID,
		"type":            streamType,
		"index":           index,
		"content_block":   block,
		"timestamp":       time.Now().UnixMilli(),
	}, m.slotMessageID)
}

func (m *slotMessageMapper) wrapEvent(eventType protocol.EventType, data map[string]any, messageID string) protocol.EventMessage {
	event := protocol.NewEvent(eventType, data)
	event.SessionKey = m.sessionKey
	event.RoomID = m.roomID
	event.ConversationID = m.conversationID
	event.AgentID = m.agentID
	event.MessageID = messageID
	event.CausedBy = m.agentRoundID
	return event
}

func normalizeRoomConversationContent(blocks []sdkprotocol.ContentBlock) []map[string]any {
	result := make([]map[string]any, 0, len(blocks))
	for _, block := range blocks {
		payload := cloneRoomBlockPayload(block)
		if len(payload) == 0 {
			payload = map[string]any{}
		}
		payload["type"] = string(block.Type())
		mergeNormalizedRoomBlockPayload(payload, block)
		result = append(result, payload)
	}
	return result
}

func normalizeRoomContentBlock(raw any) map[string]any {
	payload, ok := raw.(map[string]any)
	if !ok {
		return map[string]any{}
	}
	result := make(map[string]any, len(payload))
	for key, value := range payload {
		result[key] = value
	}
	if value := anyString(result["type"]); value != "" {
		result["type"] = value
	}
	return result
}

func roomResultSubtype(result *sdkprotocol.ResultMessage) string {
	if result == nil {
		return "error"
	}
	switch strings.TrimSpace(result.Subtype) {
	case "success", "error", "interrupted":
		return strings.TrimSpace(result.Subtype)
	default:
		if result.IsError {
			return "error"
		}
		return "success"
	}
}

func firstNonNilMap(values ...map[string]any) map[string]any {
	for _, value := range values {
		if len(value) > 0 {
			return value
		}
	}
	return map[string]any{}
}

func anyString(value any) string {
	typed, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(typed)
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
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func cloneRoomBlockPayload(block sdkprotocol.ContentBlock) map[string]any {
	if block == nil {
		return map[string]any{}
	}
	payload := block.RawPayload()
	if len(payload) == 0 {
		return map[string]any{}
	}
	result := make(map[string]any, len(payload))
	for key, value := range payload {
		result[key] = value
	}
	return result
}

func mergeNormalizedRoomBlockPayload(payload map[string]any, block sdkprotocol.ContentBlock) {
	switch typed, ok := sdkprotocol.AsTextBlock(block); {
	case ok:
		payload["text"] = typed.Text
		return
	}
	switch typed, ok := sdkprotocol.AsThinkingBlock(block); {
	case ok:
		payload["thinking"] = typed.Thinking
		if strings.TrimSpace(typed.Signature) != "" {
			payload["signature"] = typed.Signature
		}
		return
	}
	switch typed, ok := sdkprotocol.AsToolUseBlock(block); {
	case ok:
		payload["id"] = typed.ID
		payload["name"] = typed.Name
		payload["input"] = firstNonNilMap(typed.InputMap(), map[string]any{})
		return
	}
	switch typed, ok := sdkprotocol.AsToolResultBlock(block); {
	case ok:
		payload["tool_use_id"] = typed.ToolUseID
		if content := decodeRoomRawJSON(typed.Content); content != nil {
			payload["content"] = content
		}
		payload["is_error"] = typed.IsError
		if strings.TrimSpace(typed.MimeType) != "" {
			payload["mime_type"] = typed.MimeType
		}
		return
	}
}

func decodeRoomRawJSON(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var result any
	if err := json.Unmarshal(raw, &result); err != nil {
		return strings.TrimSpace(string(raw))
	}
	return result
}
