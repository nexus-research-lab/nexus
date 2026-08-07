// INPUT: Provider/MCP 工具结果中的结构化对象、JSON 文本或兼容包装层。
// OUTPUT: 跨消息、Goal loop 与 WorkGraph 共用的 mutation 语义结果。
// POS: 工具传输成功与业务 mutation 结果之间的协议真相源；只识别显式 envelope，不推断 Agent 路线。
package protocol

import (
	"encoding/json"
	"strings"
)

const mutationResultJSONLimit = 64 * 1024

// MutationResultOutcome 表示一次业务 mutation 是否改变了权威状态。
type MutationResultOutcome string

const (
	MutationResultApplied  MutationResultOutcome = "applied"
	MutationResultNoOp     MutationResultOutcome = "no_op"
	MutationResultRejected MutationResultOutcome = "rejected"
)

// MutationResultEnvelope 是模型工具结果里可稳定投影到 UI 与 loop guard 的紧凑语义。
type MutationResultEnvelope struct {
	Outcome    MutationResultOutcome
	Message    string
	ReasonCode string
}

// ToolResult metadata key 让新消息无需重复解析展示文本；历史消息仍可由
// ParseMutationResultEnvelope 从原始 content 恢复同一语义。
const (
	MutationOutcomeMetadataKey    = "_nexus_mutation_outcome"
	MutationMessageMetadataKey    = "_nexus_mutation_message"
	MutationReasonCodeMetadataKey = "_nexus_mutation_reason_code"
)

// ParseMutationResultEnvelope 按候选优先级识别显式 mutation envelope。
// 它接受 structured output、被 JSON 字符串包裹的 text result，以及常见包装字段。
func ParseMutationResultEnvelope(values ...any) (MutationResultEnvelope, bool) {
	for _, value := range values {
		if result, ok := parseMutationResultEnvelope(value, 0); ok {
			return result, true
		}
	}
	return MutationResultEnvelope{}, false
}

func parseMutationResultEnvelope(value any, depth int) (MutationResultEnvelope, bool) {
	if value == nil || depth > 3 {
		return MutationResultEnvelope{}, false
	}
	switch typed := value.(type) {
	case map[string]any:
		if outcome, ok := mutationResultOutcome(typed["outcome"]); ok {
			return MutationResultEnvelope{
				Outcome:    outcome,
				Message:    mutationResultString(typed["message"]),
				ReasonCode: mutationResultString(typed["reason_code"]),
			}, true
		}
		for _, key := range []string{
			"structured_output",
			"structured_content",
			"structuredContent",
			"content",
			"text",
		} {
			if result, ok := parseMutationResultEnvelope(typed[key], depth+1); ok {
				return result, true
			}
		}
	case []any:
		for _, item := range typed {
			if result, ok := parseMutationResultEnvelope(item, depth+1); ok {
				return result, true
			}
		}
	case json.RawMessage:
		return parseMutationResultJSON([]byte(typed), depth+1)
	case []byte:
		return parseMutationResultJSON(typed, depth+1)
	case string:
		return parseMutationResultJSON([]byte(strings.TrimSpace(typed)), depth+1)
	}
	return MutationResultEnvelope{}, false
}

func parseMutationResultJSON(raw []byte, depth int) (MutationResultEnvelope, bool) {
	if len(raw) == 0 || len(raw) > mutationResultJSONLimit {
		return MutationResultEnvelope{}, false
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return MutationResultEnvelope{}, false
	}
	return parseMutationResultEnvelope(decoded, depth)
}

func mutationResultOutcome(value any) (MutationResultOutcome, bool) {
	outcome := MutationResultOutcome(mutationResultString(value))
	switch outcome {
	case MutationResultApplied, MutationResultNoOp, MutationResultRejected:
		return outcome, true
	default:
		return "", false
	}
}

func mutationResultString(value any) string {
	typed, _ := value.(string)
	return strings.TrimSpace(typed)
}
