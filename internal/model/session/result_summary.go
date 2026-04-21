package session

import "strings"

const internalTranscriptInterruptPromptPrefix = "[Request interrupted by user"

// AttachResultSummary 把 runtime result 摘要挂到 assistant 上。
func AttachResultSummary(assistant Message, result Message) (Message, bool) {
	if MessageRole(assistant) != "assistant" || MessageRole(result) != "result" {
		return nil, false
	}
	assistantRoundID := MessageRoundID(assistant)
	resultRoundID := MessageRoundID(result)
	if assistantRoundID == "" || resultRoundID == "" || assistantRoundID != resultRoundID {
		return nil, false
	}
	summary := BuildAssistantResultSummary(result, ExtractAssistantDisplayText(assistant))
	if len(summary) == 0 {
		return nil, false
	}
	merged := Clone(assistant)
	merged["result_summary"] = summary
	return merged, true
}

// ProjectResultMessage 把 result 投影成前端统一使用的 assistant 终态形态。
func ProjectResultMessage(assistant Message, result Message) Message {
	if merged, ok := AttachResultSummary(assistant, result); ok {
		return merged
	}
	return BuildSyntheticAssistantFromResult(result)
}

// BuildAssistantResultSummary 只保留 assistant 终态需要的结果摘要。
func BuildAssistantResultSummary(result Message, assistantText string) map[string]any {
	summary := map[string]any{
		"message_id":      stringFromAny(result["message_id"]),
		"timestamp":       messageTimestamp(result),
		"subtype":         stringFromAny(result["subtype"]),
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

	resultText := NormalizeDisplayText(stringFromAny(result["result"]))
	if resultText != "" {
		if NormalizeResultSubtype(stringFromAny(result["subtype"])) != "success" || resultText != assistantText {
			summary["result"] = stringFromAny(result["result"])
		}
	}
	return summary
}

// ExtractAssistantDisplayText 提取 assistant 主正文文本，用于去重 result 文本。
func ExtractAssistantDisplayText(message Message) string {
	blocks := normalizeMessageContentBlocks(message["content"])
	if len(blocks) == 0 {
		return ""
	}

	texts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if strings.TrimSpace(stringFromAny(block["type"])) != "text" {
			continue
		}
		text := strings.TrimSpace(stringFromAny(block["text"]))
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
	switch strings.TrimSpace(subtype) {
	case "success", "error", "interrupted":
		return strings.TrimSpace(subtype)
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
func BuildSyntheticAssistantFromResult(result Message) Message {
	synthetic := Message{
		"message_id":  buildSyntheticAssistantMessageID(result),
		"session_key": stringFromAny(result["session_key"]),
		"agent_id":    stringFromAny(result["agent_id"]),
		"round_id":    stringFromAny(result["round_id"]),
		"role":        "assistant",
		"timestamp":   messageTimestamp(result),
		"is_complete": true,
	}
	if roomID := stringFromAny(result["room_id"]); roomID != "" {
		synthetic["room_id"] = roomID
	}
	if conversationID := stringFromAny(result["conversation_id"]); conversationID != "" {
		synthetic["conversation_id"] = conversationID
	}
	if sessionID := stringFromAny(result["session_id"]); sessionID != "" {
		synthetic["session_id"] = sessionID
	}
	if parentID := stringFromAny(result["parent_id"]); parentID != "" {
		synthetic["parent_id"] = parentID
	}
	switch NormalizeResultSubtype(stringFromAny(result["subtype"])) {
	case "interrupted":
		synthetic["stop_reason"] = "cancelled"
	case "error":
		synthetic["stop_reason"] = "error"
	default:
		synthetic["stop_reason"] = "end_turn"
	}
	if resultText := stringFromAny(result["result"]); resultText != "" {
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

func stringFromAny(value any) string {
	typed, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(typed)
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

func messageTimestamp(message Message) int64 {
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

func buildSyntheticAssistantMessageID(result Message) string {
	if messageID := stringFromAny(result["message_id"]); messageID != "" {
		return "assistant_" + messageID
	}
	if roundID := stringFromAny(result["round_id"]); roundID != "" {
		return "assistant_result_" + roundID
	}
	return "assistant_result"
}
