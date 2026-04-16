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
	"encoding/json"
	"fmt"
	"github.com/nexus-research-lab/nexus-core/internal/protocol"
	"github.com/nexus-research-lab/nexus-core/internal/sessiondomain"
	"strings"
	"time"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

type streamState struct {
	messageID  string
	model      string
	stopReason string
	usage      map[string]any
	blocks     map[int]map[string]any
}

type messageMapper struct {
	sessionKey string
	agentID    string
	roundID    string
	stream     streamState
}

func newMessageMapper(sessionKey string, agentID string, roundID string) *messageMapper {
	return &messageMapper{
		sessionKey: sessionKey,
		agentID:    agentID,
		roundID:    roundID,
		stream: streamState{
			blocks: map[int]map[string]any{},
			usage:  map[string]any{},
		},
	}
}

func (m *messageMapper) Map(message sdkprotocol.ReceivedMessage) ([]protocol.EventMessage, string, string) {
	switch message.Type {
	case sdkprotocol.MessageTypeStreamEvent:
		return m.mapStreamEvent(message), "", ""
	case sdkprotocol.MessageTypeAssistant:
		return m.mapAssistantMessage(message), "", ""
	case sdkprotocol.MessageTypeResult:
		subtype := normalizeResultSubtype(message.Result)
		return m.mapResultMessage(message, subtype), statusFromResultSubtype(subtype), subtype
	case sdkprotocol.MessageTypeToolProgress:
		return m.mapToolProgress(message), "", ""
	default:
		return nil, "", ""
	}
}

func (m *messageMapper) mapStreamEvent(message sdkprotocol.ReceivedMessage) []protocol.EventMessage {
	if message.Stream == nil {
		return nil
	}
	payload, ok := message.Stream.Event.(map[string]any)
	if !ok {
		payload = message.Stream.Data
	}
	eventType := strings.TrimSpace(normalizeString(payload["type"]))
	if eventType == "" {
		return nil
	}

	switch eventType {
	case "message_start":
		messagePayload, _ := payload["message"].(map[string]any)
		m.stream.messageID = firstNonEmpty(normalizeString(messagePayload["id"]), m.stream.messageID, fmt.Sprintf("assistant_%s", m.roundID))
		m.stream.model = normalizeString(messagePayload["model"])
		if usage, ok := messagePayload["usage"].(map[string]any); ok {
			m.stream.usage = usage
		}
		return []protocol.EventMessage{
			m.wrapEvent(protocol.EventTypeStreamStart, map[string]any{
				"msg_id":   m.stream.messageID,
				"round_id": m.roundID,
			}, m.stream.messageID),
		}
	case "content_block_start":
		index := normalizeIntValue(payload["index"])
		block := normalizeContentBlock(payload["content_block"])
		if block["type"] == "tool_use" {
			m.stream.blocks[index] = block
			return nil
		}
		m.stream.blocks[index] = block
		return []protocol.EventMessage{m.wrapStreamEvent("content_block_start", index, block)}
	case "content_block_delta":
		index := normalizeIntValue(payload["index"])
		block := m.stream.blocks[index]
		if len(block) == 0 {
			return nil
		}
		applyDelta(block, payload["delta"])
		if block["type"] == "tool_use" {
			return nil
		}
		return []protocol.EventMessage{m.wrapStreamEvent("content_block_delta", index, block)}
	case "message_delta":
		delta, _ := payload["delta"].(map[string]any)
		if stopReason := normalizeString(delta["stop_reason"]); stopReason != "" {
			m.stream.stopReason = stopReason
		}
		if usage, ok := payload["usage"].(map[string]any); ok {
			m.stream.usage = usage
		}
		return []protocol.EventMessage{m.wrapEvent(protocol.EventTypeStream, map[string]any{
			"message_id":  m.stream.messageID,
			"session_key": m.sessionKey,
			"agent_id":    m.agentID,
			"round_id":    m.roundID,
			"type":        "message_delta",
			"message": map[string]any{
				"stop_reason": emptyToNil(m.stream.stopReason),
			},
			"usage":     m.stream.usage,
			"timestamp": time.Now().UnixMilli(),
		}, m.stream.messageID)}
	case "message_stop":
		return nil
	default:
		return nil
	}
}

func (m *messageMapper) mapAssistantMessage(message sdkprotocol.ReceivedMessage) []protocol.EventMessage {
	if message.Assistant == nil {
		return nil
	}
	msgID := firstNonEmpty(message.Assistant.Message.ID, m.stream.messageID, fmt.Sprintf("assistant_%s", m.roundID))
	payload := sessiondomain.Message{
		"message_id":  msgID,
		"session_key": m.sessionKey,
		"agent_id":    m.agentID,
		"round_id":    m.roundID,
		"session_id":  firstNonEmpty(message.SessionID, ""),
		"parent_id":   m.roundID,
		"role":        "assistant",
		"timestamp":   time.Now().UnixMilli(),
		"content":     normalizeConversationContent(message.Assistant.Message.Content),
		"model":       firstNonEmpty(message.Assistant.Message.Model, m.stream.model),
		"usage":       firstNonNilMap(message.Assistant.Message.Usage, m.stream.usage),
		"is_complete": true,
	}
	if reason := normalizeAny(message.Assistant.Message.StopReason); reason != nil {
		payload["stop_reason"] = reason
	}

	return []protocol.EventMessage{
		m.wrapDurableMessage(payload),
		m.wrapEvent(protocol.EventTypeStreamEnd, map[string]any{
			"msg_id":   msgID,
			"round_id": m.roundID,
		}, msgID),
	}
}

func (m *messageMapper) mapResultMessage(message sdkprotocol.ReceivedMessage, subtype string) []protocol.EventMessage {
	if message.Result == nil {
		return nil
	}
	msgID := firstNonEmpty(message.UUID, "result_"+m.roundID)
	payload := sessiondomain.Message{
		"message_id":      msgID,
		"session_key":     m.sessionKey,
		"agent_id":        m.agentID,
		"round_id":        m.roundID,
		"session_id":      firstNonEmpty(message.SessionID, ""),
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
	return []protocol.EventMessage{m.wrapDurableMessage(payload)}
}

func (m *messageMapper) mapToolProgress(message sdkprotocol.ReceivedMessage) []protocol.EventMessage {
	if message.ToolProgress == nil {
		return nil
	}
	payload := sessiondomain.Message{
		"message_id":  fmt.Sprintf("progress_%s_%d", m.roundID, time.Now().UnixMilli()),
		"session_key": m.sessionKey,
		"agent_id":    m.agentID,
		"round_id":    m.roundID,
		"role":        "system",
		"timestamp":   time.Now().UnixMilli(),
		"content":     message.ToolProgress.ToolName + " 正在执行",
		"metadata": map[string]any{
			"subtype":        "task_progress",
			"tool_use_id":    message.ToolProgress.ToolUseID,
			"last_tool_name": message.ToolProgress.ToolName,
			"task_id":        message.ToolProgress.TaskID,
		},
	}
	return []protocol.EventMessage{m.wrapDurableMessage(payload)}
}

func (m *messageMapper) wrapDurableMessage(payload sessiondomain.Message) protocol.EventMessage {
	event := m.wrapEvent(protocol.EventTypeMessage, payload, normalizeString(payload["message_id"]))
	event.DeliveryMode = "durable"
	return event
}

func (m *messageMapper) wrapStreamEvent(streamType string, index int, block map[string]any) protocol.EventMessage {
	return m.wrapEvent(protocol.EventTypeStream, map[string]any{
		"message_id":    m.stream.messageID,
		"session_key":   m.sessionKey,
		"agent_id":      m.agentID,
		"round_id":      m.roundID,
		"type":          streamType,
		"index":         index,
		"content_block": block,
		"timestamp":     time.Now().UnixMilli(),
	}, m.stream.messageID)
}

func (m *messageMapper) wrapEvent(eventType protocol.EventType, data map[string]any, messageID string) protocol.EventMessage {
	event := protocol.NewEvent(eventType, data)
	event.SessionKey = m.sessionKey
	event.AgentID = m.agentID
	event.MessageID = emptyString(messageID)
	return event
}

func applyDelta(block map[string]any, raw any) {
	delta, ok := raw.(map[string]any)
	if !ok {
		return
	}
	switch block["type"] {
	case "text":
		block["text"] = normalizeString(block["text"]) + normalizeString(delta["text"])
	case "thinking":
		block["thinking"] = normalizeString(block["thinking"]) + normalizeString(delta["thinking"])
	case "tool_use":
		// 中文注释：tool_use 的增量是局部 JSON patch，这一轮先留给 assistant 快照收口，
		// 避免前端拿到不完整参数后误渲染。
	default:
	}
}

func normalizeConversationContent(blocks []sdkprotocol.ContentBlock) []map[string]any {
	result := make([]map[string]any, 0, len(blocks))
	for _, block := range blocks {
		payload := cloneBlockPayload(block)
		if len(payload) == 0 {
			payload = map[string]any{}
		}
		payload["type"] = string(block.Type())
		mergeNormalizedBlockPayload(payload, block)
		result = append(result, payload)
	}
	return result
}

func normalizeContentBlock(raw any) map[string]any {
	payload, ok := raw.(map[string]any)
	if !ok {
		return map[string]any{}
	}
	result := map[string]any{
		"type": normalizeString(payload["type"]),
	}
	for key, value := range payload {
		result[key] = value
	}
	return result
}

func normalizeResultSubtype(result *sdkprotocol.ResultMessage) string {
	if result == nil {
		return "error"
	}
	subtype := strings.TrimSpace(result.Subtype)
	switch subtype {
	case "success", "error", "interrupted":
		return subtype
	default:
		if result.IsError {
			return "error"
		}
		return "success"
	}
}

func statusFromResultSubtype(subtype string) string {
	switch subtype {
	case "interrupted":
		return "interrupted"
	case "error":
		return "error"
	default:
		return "finished"
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

func normalizeAny(value any) any {
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return nil
		}
		return strings.TrimSpace(typed)
	default:
		return value
	}
}

func emptyToNil(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.TrimSpace(value)
}

func emptyString(value string) string {
	return strings.TrimSpace(value)
}

func cloneBlockPayload(block sdkprotocol.ContentBlock) map[string]any {
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

func mergeNormalizedBlockPayload(payload map[string]any, block sdkprotocol.ContentBlock) {
	switch typed, ok := sdkprotocol.AsTextBlock(block); {
	case ok:
		payload["text"] = typed.Text
		return
	}
	switch typed, ok := sdkprotocol.AsThinkingBlock(block); {
	case ok:
		payload["thinking"] = typed.Thinking
		payload["signature"] = emptyToNil(typed.Signature)
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
		if content := decodeRawJSON(typed.Content); content != nil {
			payload["content"] = content
		}
		payload["is_error"] = typed.IsError
		payload["mime_type"] = emptyToNil(typed.MimeType)
		return
	}
}

func decodeRawJSON(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	var result any
	if err := json.Unmarshal(raw, &result); err != nil {
		return strings.TrimSpace(string(raw))
	}
	return result
}
