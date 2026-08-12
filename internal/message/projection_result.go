// INPUT: runtime result、同一 Agent 执行轮的 assistant 快照与终态元数据。
// OUTPUT: 保留执行身份的 assistant result_summary 或 result-only 合成 assistant。
// POS: runtime result 到前端统一 assistant 终态形态的投影边界。
package message

import (
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

const internalTranscriptInterruptPromptPrefix = "[Request interrupted by user"

// 这是 SDK 在输出预算耗尽后注入的续跑哨兵，不属于用户可见对话。
const internalTranscriptMaxOutputRecoveryPrompt = "Output token limit hit. Resume directly — no apology, no recap of what you were doing. Pick up mid-thought if that is where the cut happened. Break remaining work into smaller pieces."

const (
	internalExplicitSkillContextOpenTag  = `<internal_context source="explicit_skill">`
	internalExplicitSkillContextCloseTag = "</internal_context>"
)

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
	if !sameOptionalIdentity(assistant, result, "agent_round_id") ||
		!sameOptionalIdentity(assistant, result, "agent_id") {
		return nil, false
	}
	summary := BuildAssistantResultSummary(result, ExtractAssistantDisplayText(assistant))
	if len(summary) == 0 {
		return nil, false
	}
	merged := protocol.Clone(assistant)
	merged["result_summary"] = summary
	// result 是 round 终态信号，挂载摘要时也要同步 assistant 终态，
	// 否则实时前端会继续把该 assistant 当作 streaming 消息。
	merged["is_complete"] = true
	if stopReason := stopReasonFromResult(result); stopReason != "" {
		merged["stop_reason"] = stopReason
	}
	return merged, true
}

// BuildGoalCompletionReceipt 从 complete Goal 的稳定聚合报告构造完成收据。
// report 缺失时仍保留完成事实；只有 usage_finalized 才公开 actual token。
func BuildGoalCompletionReceipt(
	goalID string,
	roundID string,
	report *protocol.GoalUsageReport,
) protocol.GoalCompletionReceipt {
	receipt := protocol.GoalCompletionReceipt{
		GoalID:  strings.TrimSpace(goalID),
		RoundID: strings.TrimSpace(roundID),
	}
	if report == nil {
		return receipt
	}
	if report.TimeUsedSeconds > 0 {
		seconds := report.TimeUsedSeconds
		receipt.TimeUsedSeconds = &seconds
	}
	if report.UsageFinalized {
		tokens := report.Usage.ActualTokens()
		receipt.ActualTokens = &tokens
	}
	return receipt
}

// AttachGoalCompletionReceipt 把收据挂到同一条最终 assistant 快照。
func AttachGoalCompletionReceipt(
	assistant protocol.Message,
	receipt protocol.GoalCompletionReceipt,
) (protocol.Message, bool) {
	if protocol.MessageRole(assistant) != "assistant" ||
		strings.TrimSpace(receipt.GoalID) == "" ||
		strings.TrimSpace(receipt.RoundID) == "" ||
		strings.TrimSpace(normalizeString(assistant["message_id"])) == "" {
		return nil, false
	}
	messageRoundID := protocol.MessageRoundID(assistant)
	agentRoundID := strings.TrimSpace(normalizeString(assistant["agent_round_id"]))
	if receipt.RoundID != messageRoundID && receipt.RoundID != agentRoundID {
		return nil, false
	}
	merged := protocol.Clone(assistant)
	merged[protocol.GoalCompletionReceiptField] = receipt
	return merged, true
}

// ProjectResultMessage 把 result 投影成前端统一使用的 assistant 终态形态。
func ProjectResultMessage(assistant protocol.Message, result protocol.Message) protocol.Message {
	if merged, ok := AttachResultSummary(assistant, result); ok {
		if hasPublicAssistantContent(normalizeMessageContentBlocks(merged["content"])) || resultNeedsAssistantProjection(result) {
			return merged
		}
		return nil
	}
	if !resultNeedsAssistantProjection(result) {
		return nil
	}
	return BuildSyntheticAssistantFromResult(result)
}

func resultNeedsAssistantProjection(result protocol.Message) bool {
	if boolFromAny(result["is_error"]) || NormalizeResultSubtype(normalizeString(result["subtype"])) == "error" {
		return true
	}
	// Room 的空 interrupted result 是 Agent 槽位的终态卡片，不是聊天气泡。
	// 保留它的身份，让实时态和历史态都能稳定展示“已停止”，同时不回显
	// runtime 的默认停止文案。
	if NormalizeResultSubtype(normalizeString(result["subtype"])) == "interrupted" &&
		normalizeString(result["room_id"]) != "" &&
		normalizeString(result["agent_round_id"]) != "" {
		return true
	}
	return NormalizeDisplayText(resultProjectionText(result)) != ""
}

// BuildAssistantResultSummary 只保留 assistant 终态需要的结果摘要。
func BuildAssistantResultSummary(result protocol.Message, assistantText string) map[string]any {
	resultMessageID := normalizeString(result["message_id"])
	resultSubtype := normalizeString(result["subtype"])
	resultValue := resultProjectionText(result)
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
	copyNonEmptyResultField(summary, result, "model_usage")
	copyNonEmptyResultField(summary, result, "structured_output")
	copyNonEmptyResultField(summary, result, "fast_mode_state")
	copyNonEmptyResultField(summary, result, "runtime_subtype")
	copyNonEmptyResultField(summary, result, "permission_denials")
	copyNonEmptyResultField(summary, result, "errors")
	copyNonEmptyResultField(summary, result, "terminal_reason")
	copyNonEmptyResultField(summary, result, "stop_reason")

	resultText := NormalizeDisplayText(resultValue)
	if resultText != "" {
		if NormalizeResultSubtype(resultSubtype) != "success" || resultText != assistantText {
			summary["result"] = resultValue
		}
	}
	return summary
}

func resultProjectionText(result protocol.Message) string {
	resultText := normalizeString(result["result"])
	if NormalizeResultSubtype(normalizeString(result["subtype"])) == "interrupted" {
		return NormalizeInterruptDisplayText(resultText)
	}
	return resultText
}

func copyNonEmptyResultField(target map[string]any, source protocol.Message, key string) {
	value, exists := source[key]
	if !exists || value == nil {
		return
	}
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return
		}
	case []string:
		if len(typed) == 0 {
			return
		}
	case []any:
		if len(typed) == 0 {
			return
		}
	case []map[string]any:
		if len(typed) == 0 {
			return
		}
	case map[string]any:
		if len(typed) == 0 {
			return
		}
	}
	target[key] = value
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

// IsInternalTranscriptContinuationPrompt 判断是否为 SDK 输出预算耗尽后的内部续跑提示。
func IsInternalTranscriptContinuationPrompt(content string) bool {
	return strings.TrimSpace(content) == internalTranscriptMaxOutputRecoveryPrompt
}

// IsInternalExplicitSkillPrompt 判断是否为旧版宿主注入的显式 Skill 正文。
//
// 新版由 runtime 以 isMeta user 承载；这里仅用于读取已经落盘的旧 transcript，
// 让它继续充当 round 边界，但绝不作为用户正文展示。
func IsInternalExplicitSkillPrompt(content string) bool {
	trimmed := strings.TrimSpace(content)
	return strings.HasPrefix(trimmed, "<system-reminder>") &&
		strings.Contains(trimmed, internalExplicitSkillContextOpenTag) &&
		strings.Contains(trimmed, internalExplicitSkillContextCloseTag) &&
		strings.HasSuffix(trimmed, "</system-reminder>")
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
	copySyntheticAssistantIdentity(synthetic, result, "agent_round_id")
	copySyntheticAssistantIdentity(synthetic, result, "model")
	if stopReason := stopReasonFromResult(result); stopReason != "" {
		synthetic["stop_reason"] = stopReason
	} else {
		synthetic["stop_reason"] = "end_turn"
	}
	if resultText := resultProjectionText(result); resultText != "" {
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

func sameOptionalIdentity(left protocol.Message, right protocol.Message, key string) bool {
	leftValue := normalizeString(left[key])
	rightValue := normalizeString(right[key])
	return leftValue == "" || rightValue == "" || leftValue == rightValue
}

func copySyntheticAssistantIdentity(target protocol.Message, source protocol.Message, key string) {
	if value := normalizeString(source[key]); value != "" {
		target[key] = value
	}
}

func stopReasonFromResult(result protocol.Message) string {
	if stopReason := normalizeString(result["stop_reason"]); stopReason != "" {
		return stopReason
	}
	switch NormalizeResultSubtype(normalizeString(result["subtype"])) {
	case "interrupted":
		return "cancelled"
	case "error":
		return "error"
	case "success":
		return "end_turn"
	default:
		return ""
	}
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
