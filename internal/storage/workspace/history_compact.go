// INPUT: transcript、overlay 与 runtime 产生的重复 message_id 快照。
// OUTPUT: 保留消息与 Agent 执行身份、单调合并内容和终态字段的历史行。
// POS: workspace 历史 normalize 前的 message_id 压缩边界。
package workspace

import (
	"encoding/json"
	"maps"
	"strconv"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/message"
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
	currentRole := stringFromAny(current["role"])
	nextRole := stringFromAny(next["role"])
	if currentRole == "user" && nextRole == "user" {
		return mergeUserSnapshots(current, next)
	}
	if currentRole == "assistant" && nextRole == "assistant" {
		return mergeAssistantSnapshots(current, next)
	}
	if transcriptPositioned(current) && !transcriptPositioned(next) {
		next[message.TranscriptLineField] = current[message.TranscriptLineField]
		next["timestamp"] = current["timestamp"]
	}
	return next
}

// transcriptPositioned 表示该行由 transcript 回放产出。宿主写入 overlay 的同 ID 副本只带接收时间，
// 合并时由带位置的副本决定时间：所有 transcript 消息按同一个时钟定序，不随宿主延迟交错。
func transcriptPositioned(row protocol.Message) bool {
	_, positioned := row[message.TranscriptLineField]
	return positioned
}

func mergeUserSnapshots(current protocol.Message, next protocol.Message) protocol.Message {
	merged := protocol.Clone(next)
	if stringFromAny(merged["session_id"]) == "" && stringFromAny(current["session_id"]) != "" {
		merged["session_id"] = current["session_id"]
	}

	// 受理游标只在受理时铸造一次；后一份快照不带它时沿用已有的值。
	if _, positioned := merged[protocol.InputSeqField]; !positioned {
		if inputSeq, positioned := current[protocol.InputSeqField]; positioned {
			merged[protocol.InputSeqField] = inputSeq
		}
	}

	// 后一份 durable user 快照仍是完整真相，不能从旧的单 Agent 快照继承
	// agent_round_id 等已被清除的字段。只补 transcript 独有的 session_id，
	// 并保留最早的有效输入时间，保证 user 排在由它触发的 assistant 前面。
	currentTimestamp := messageTimestamp(current)
	nextTimestamp := messageTimestamp(next)
	if currentTimestamp > 0 && (nextTimestamp <= 0 || currentTimestamp < nextTimestamp) {
		merged["timestamp"] = current["timestamp"]
	}
	return merged
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
	switch currentPositioned, nextPositioned := transcriptPositioned(current), transcriptPositioned(next); {
	case currentPositioned && !nextPositioned:
	case nextPositioned && !currentPositioned:
		merged["timestamp"] = next["timestamp"]
	case messageTimestamp(next) >= messageTimestamp(current):
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
		result = message.UpsertAssistantContentBlock(result, block)
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
