// INPUT: transcript、overlay 与 runtime 产生的重复 message_id 快照。
// OUTPUT: 保留消息与 Agent 执行身份、单调合并内容和终态字段的历史行。
// POS: workspace 历史 normalize 前的 message_id 压缩边界。
package workspace

import (
	"encoding/json"
	"maps"
	"strconv"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func compactMessages(rows []protocol.Message) []protocol.Message {
	latestByID := make(map[string]protocol.Message, len(rows))
	order := make([]string, 0, len(rows))
	for _, row := range rows {
		messageID := stringFromAny(row["message_id"])
		if messageID == "" {
			continue
		}
		if current, exists := latestByID[messageID]; exists {
			latestByID[messageID] = mergeCompactedMessage(current, protocol.Clone(row))
			continue
		}
		if _, exists := latestByID[messageID]; !exists {
			order = append(order, messageID)
		}
		latestByID[messageID] = protocol.Clone(row)
	}

	compacted := make([]protocol.Message, 0, len(order))
	for _, messageID := range order {
		compacted = append(compacted, latestByID[messageID])
	}
	sortHistoryRows(compacted)
	return compacted
}

func mergeCompactedMessage(current protocol.Message, next protocol.Message) protocol.Message {
	if stringFromAny(current["role"]) != "assistant" || stringFromAny(next["role"]) != "assistant" {
		return next
	}
	return mergeAssistantSnapshots(current, next)
}

func mergeAssistantSnapshots(current protocol.Message, next protocol.Message) protocol.Message {
	merged := protocol.Clone(current)

	// assistant 快照属于同一条消息的增量物化，身份字段一旦建立就不应被后续快照改写。
	identityKeys := []string{
		"message_id",
		"session_key",
		"room_id",
		"conversation_id",
		"agent_id",
		"round_id",
		"agent_round_id",
		"parent_id",
		"session_id",
		"role",
	}
	for _, key := range identityKeys {
		if stringFromAny(merged[key]) == "" && stringFromAny(next[key]) != "" {
			merged[key] = next[key]
		}
	}

	if content, ok := mergeAssistantContentBlocks(merged["content"], next["content"]); ok {
		merged["content"] = content
	} else if next["content"] != nil {
		merged["content"] = next["content"]
	}
	if value := stringFromAny(next["model"]); value != "" {
		merged["model"] = value
	}
	if value := stringFromAny(next["stop_reason"]); value != "" {
		merged["stop_reason"] = value
	}
	if usage := normalizeMapValue(next["usage"]); len(usage) > 0 {
		merged["usage"] = usage
	}
	if boolFromAny(current["is_complete"]) || boolFromAny(next["is_complete"]) {
		merged["is_complete"] = true
	}
	if status := stringFromAny(next["stream_status"]); status != "" {
		merged["stream_status"] = status
	}
	if ts := messageTimestamp(next); ts >= messageTimestamp(current) {
		merged["timestamp"] = next["timestamp"]
	}

	// 其它非身份字段以后者为准，避免成本、终止原因、补充元数据被前序快照覆盖。
	for key, value := range next {
		switch key {
		case "content",
			"message_id",
			"session_key",
			"room_id",
			"conversation_id",
			"agent_id",
			"round_id",
			"agent_round_id",
			"parent_id",
			"session_id",
			"role",
			"model",
			"stop_reason",
			"usage",
			"is_complete",
			"stream_status",
			"timestamp":
			continue
		default:
			merged[key] = value
		}
	}
	return merged
}

func mergeAssistantContentBlocks(current any, next any) ([]map[string]any, bool) {
	result := normalizeMessageContentBlocks(current)
	incoming := normalizeMessageContentBlocks(next)
	if result == nil && incoming == nil {
		return nil, false
	}
	if len(result) == 0 {
		return incoming, true
	}
	if len(incoming) == 0 {
		return result, true
	}
	for _, block := range incoming {
		result = upsertAssistantContentBlock(result, block)
	}
	return result, true
}

func normalizeMessageContentBlocks(raw any) []map[string]any {
	switch typed := raw.(type) {
	case []map[string]any:
		return cloneMessageContentBlocks(typed)
	case []any:
		result := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			payload, ok := item.(map[string]any)
			if !ok {
				continue
			}
			result = append(result, cloneMessageMap(payload))
		}
		return result
	default:
		return nil
	}
}

func cloneMessageContentBlocks(blocks []map[string]any) []map[string]any {
	if len(blocks) == 0 {
		return nil
	}
	result := make([]map[string]any, 0, len(blocks))
	for _, block := range blocks {
		result = append(result, cloneMessageMap(block))
	}
	return result
}

func cloneMessageMap(payload map[string]any) map[string]any {
	if len(payload) == 0 {
		return nil
	}
	return maps.Clone(payload)
}

func upsertAssistantContentBlock(blocks []map[string]any, incoming map[string]any) []map[string]any {
	block := cloneMessageMap(incoming)
	if len(block) == 0 {
		return blocks
	}
	incomingType := stringFromAny(block["type"])
	for index, current := range blocks {
		currentType := stringFromAny(current["type"])
		if currentType != incomingType {
			continue
		}
		switch incomingType {
		case "thinking":
			blocks[index] = block
			return blocks
		case "text":
			blocks[index] = block
			return blocks
		case "tool_use":
			if stringFromAny(current["id"]) == stringFromAny(block["id"]) {
				blocks[index] = block
				return blocks
			}
		case "tool_result":
			if stringFromAny(current["tool_use_id"]) == stringFromAny(block["tool_use_id"]) {
				blocks[index] = block
				return blocks
			}
		case "task_progress":
			if stringFromAny(current["task_id"]) == stringFromAny(block["task_id"]) {
				blocks[index] = block
				return blocks
			}
		default:
			blocks[index] = block
			return blocks
		}
	}
	return append(blocks, block)
}

func normalizeMapValue(value any) map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneMessageMap(typed)
	default:
		return nil
	}
}

func boolFromAny(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	default:
		return false
	}
}

func messageTimestamp(row protocol.Message) int64 {
	value := row["timestamp"]
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case float32:
		return int64(typed)
	case int:
		return int64(typed)
	case int64:
		return typed
	case json.Number:
		parsed, _ := typed.Int64()
		return parsed
	case string:
		parsed, _ := strconv.ParseInt(strings.TrimSpace(typed), 10, 64)
		return parsed
	default:
		return 0
	}
}
