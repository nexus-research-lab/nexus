package message

import (
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const internalTranscriptInterruptPromptPrefix = "[Request interrupted by user"

// AttachResultSummary 把 runtime result 摘要挂到 assistant 上。
func AttachResultSummary(assistant protocol.Message, result protocol.Message) (protocol.Message, bool) {
	if protocol.MessageRole(assistant) != "assistant" || protocol.MessageRole(result) != "result" {
		return nil, false
	}
	assistantRoundID := protocol.MessageRoundID(assistant)
	resultRoundID := protocol.MessageRoundID(result)
	if assistantRoundID == "" || resultRoundID == "" || assistantRoundID != resultRoundID {
		return nil, false
	}
	summary := BuildAssistantResultSummary(result, ExtractAssistantDisplayText(assistant))
	if len(summary) == 0 {
		return nil, false
	}
	merged := protocol.Clone(assistant)
	merged["result_summary"] = summary
	return merged, true
}

// ProjectResultMessage 把 result 投影成前端统一使用的 assistant 终态形态。
func ProjectResultMessage(assistant protocol.Message, result protocol.Message) protocol.Message {
	if merged, ok := AttachResultSummary(assistant, result); ok {
		return merged
	}
	return BuildSyntheticAssistantFromResult(result)
}

// BuildAssistantResultSummary 只保留 assistant 终态需要的结果摘要。
func BuildAssistantResultSummary(result protocol.Message, assistantText string) map[string]any {
	resultMessageID := normalizeString(result["message_id"])
	resultSubtype := normalizeString(result["subtype"])
	resultValue := normalizeString(result["result"])
	summary := map[string]any{
		"message_id":      resultMessageID,
		"timestamp":       messageTimestamp(result),
		"subtype":         resultSubtype,
		"duration_ms":     intFromAny(result["duration_ms"]),
		"duration_api_ms": intFromAny(result["duration_api_ms"]),
		"num_turns":       intFromAny(result["num_turns"]),
		"is_error":        boolFromAny(result["is_error"]),
	}

	if _, exists := result["total_cost_usd"]; exists {
		summary["total_cost_usd"] = floatFromAny(result["total_cost_usd"])
	}
	if usage, ok := result["usage"].(map[string]any); ok && len(usage) > 0 {
		summary["usage"] = usage
	}

	resultText := NormalizeDisplayText(resultValue)
	if resultText != "" {
		if NormalizeResultSubtype(resultSubtype) != "success" || resultText != assistantText {
			summary["result"] = resultValue
		}
	}
	return summary
}

// ExtractAssistantDisplayText 提取 assistant 主正文文本，用于去重 result 文本。
func ExtractAssistantDisplayText(message protocol.Message) string {
	blocks := normalizeMessageContentBlocks(message["content"])
	if len(blocks) == 0 {
		return ""
	}

	texts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if normalizeString(block["type"]) != "text" {
			continue
		}
		text := normalizeString(block["text"])
		if text == "" {
			continue
		}
		texts = append(texts, text)
	}
	return NormalizeDisplayText(strings.Join(texts, "\n\n"))
}

// NormalizeDisplayText 统一正文比较用的文本格式。
func NormalizeDisplayText(value string) string {
	normalized := strings.ReplaceAll(value, "\r\n", "\n")
	return strings.TrimSpace(normalized)
}

// NormalizeResultSubtype 统一 result subtype。
func NormalizeResultSubtype(subtype string) string {
	normalized := strings.TrimSpace(subtype)
	switch normalized {
	case "success", "error", "interrupted":
		return normalized
	default:
		return ""
	}
}

// IsInternalTranscriptInterruptPrompt 判断是否为 SDK 内部注入的中断哨兵文案。
func IsInternalTranscriptInterruptPrompt(content string) bool {
	trimmed := strings.TrimSpace(content)
	return strings.HasPrefix(trimmed, internalTranscriptInterruptPromptPrefix) &&
		strings.HasSuffix(trimmed, "]")
}

// BuildSyntheticAssistantFromResult 在没有 assistant 可挂时构造一个终态 assistant。
func BuildSyntheticAssistantFromResult(result protocol.Message) protocol.Message {
	synthetic := protocol.Message{
		"message_id":  buildSyntheticAssistantMessageID(result),
		"session_key": normalizeString(result["session_key"]),
		"agent_id":    normalizeString(result["agent_id"]),
		"round_id":    normalizeString(result["round_id"]),
		"role":        "assistant",
		"timestamp":   messageTimestamp(result),
		"is_complete": true,
	}
	if roomID := normalizeString(result["room_id"]); roomID != "" {
		synthetic["room_id"] = roomID
	}
	if conversationID := normalizeString(result["conversation_id"]); conversationID != "" {
		synthetic["conversation_id"] = conversationID
	}
	if sessionID := normalizeString(result["session_id"]); sessionID != "" {
		synthetic["session_id"] = sessionID
	}
	if parentID := normalizeString(result["parent_id"]); parentID != "" {
		synthetic["parent_id"] = parentID
	}
	switch NormalizeResultSubtype(normalizeString(result["subtype"])) {
	case "interrupted":
		synthetic["stop_reason"] = "cancelled"
	case "error":
		synthetic["stop_reason"] = "error"
	default:
		synthetic["stop_reason"] = "end_turn"
	}
	if resultText := normalizeString(result["result"]); resultText != "" {
		synthetic["content"] = []map[string]any{{
			"type": "text",
			"text": resultText,
		}}
	} else {
		synthetic["content"] = []map[string]any{}
	}
	if summary, ok := AttachResultSummary(synthetic, result); ok {
		return summary
	}
	return synthetic
}

func intFromAny(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float32:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func floatFromAny(value any) float64 {
	switch typed := value.(type) {
	case float32:
		return float64(typed)
	case float64:
		return typed
	case int:
		return float64(typed)
	case int32:
		return float64(typed)
	case int64:
		return float64(typed)
	default:
		return 0
	}
}

func boolFromAny(value any) bool {
	typed, ok := value.(bool)
	return ok && typed
}

func messageTimestamp(message protocol.Message) int64 {
	switch typed := message["timestamp"].(type) {
	case int:
		return int64(typed)
	case int32:
		return int64(typed)
	case int64:
		return typed
	case float32:
		return int64(typed)
	case float64:
		return int64(typed)
	default:
		return 0
	}
}

func normalizeMessageContentBlocks(raw any) []map[string]any {
	if typed, ok := raw.([]map[string]any); ok {
		if len(typed) == 0 {
			return nil
		}
		result := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			result = append(result, item)
		}
		return result
	}
	items, ok := raw.([]any)
	if !ok {
		return nil
	}
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		payload, ok := item.(map[string]any)
		if !ok {
			continue
		}
		result = append(result, payload)
	}
	return result
}

func buildSyntheticAssistantMessageID(result protocol.Message) string {
	if messageID := normalizeString(result["message_id"]); messageID != "" {
		return "assistant_" + messageID
	}
	if roundID := normalizeString(result["round_id"]); roundID != "" {
		return "assistant_result_" + roundID
	}
	return "assistant_result"
}
