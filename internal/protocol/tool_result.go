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
	GoalStatusMetadataKey         = "_nexus_goal_status"
	GoalIDMetadataKey             = "_nexus_goal_id"
)

// ParseGoalIDResult 从 Goal 工具结果中读取服务端返回的精确 Goal identity。
// 只接受顶层 goalId/goal_id 或 goal 对象内部的 identity 字段，避免把普通
// 工具结果中的任意 id 误当成 Goal。
func ParseGoalIDResult(values ...any) (string, bool) {
	for _, value := range values {
		if goalID, ok := parseGoalIDResult(value, 0); ok {
			return goalID, true
		}
	}
	return "", false
}

func parseGoalIDResult(value any, depth int) (string, bool) {
	if value == nil || depth > 3 {
		return "", false
	}
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range []string{"goalId", "goal_id"} {
			if goalID := mutationResultString(typed[key]); goalID != "" {
				return goalID, true
			}
		}
		if goal, ok := typed["goal"].(map[string]any); ok {
			for _, key := range []string{"id", "goalId", "goal_id"} {
				if goalID := mutationResultString(goal[key]); goalID != "" {
					return goalID, true
				}
			}
		}
		for _, key := range []string{
			"structured_output",
			"structured_content",
			"structuredContent",
			"content",
			"text",
		} {
			if goalID, ok := parseGoalIDResult(typed[key], depth+1); ok {
				return goalID, true
			}
		}
	case []any:
		for _, item := range typed {
			if goalID, ok := parseGoalIDResult(item, depth+1); ok {
				return goalID, true
			}
		}
	case json.RawMessage:
		return parseGoalIDJSON([]byte(typed), depth+1)
	case []byte:
		return parseGoalIDJSON(typed, depth+1)
	case string:
		return parseGoalIDJSON([]byte(strings.TrimSpace(typed)), depth+1)
	}
	return "", false
}

func parseGoalIDJSON(raw []byte, depth int) (string, bool) {
	if len(raw) == 0 || len(raw) > mutationResultJSONLimit {
		return "", false
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return "", false
	}
	return parseGoalIDResult(decoded, depth)
}

// ParseGoalStatusResult 从 Goal 工具的 structured/text result 中读取权威状态。
// 调用方仍须先确认该结果属于成功的 update_goal，避免把其他工具的 status
// 字段误当成 Goal 生命周期。
func ParseGoalStatusResult(values ...any) (GoalStatus, bool) {
	for _, value := range values {
		if status, ok := parseGoalStatusResult(value, 0); ok {
			return status, true
		}
	}
	return "", false
}

func parseGoalStatusResult(value any, depth int) (GoalStatus, bool) {
	if value == nil || depth > 3 {
		return "", false
	}
	switch typed := value.(type) {
	case map[string]any:
		if goal, ok := typed["goal"].(map[string]any); ok {
			if status, ok := terminalGoalStatus(goal["status"]); ok {
				return status, true
			}
		}
		for _, key := range []string{
			"structured_output",
			"structured_content",
			"structuredContent",
			"content",
			"text",
		} {
			if status, ok := parseGoalStatusResult(typed[key], depth+1); ok {
				return status, true
			}
		}
	case []any:
		for _, item := range typed {
			if status, ok := parseGoalStatusResult(item, depth+1); ok {
				return status, true
			}
		}
	case json.RawMessage:
		return parseGoalStatusJSON([]byte(typed), depth+1)
	case []byte:
		return parseGoalStatusJSON(typed, depth+1)
	case string:
		return parseGoalStatusJSON([]byte(strings.TrimSpace(typed)), depth+1)
	}
	return "", false
}

func parseGoalStatusJSON(raw []byte, depth int) (GoalStatus, bool) {
	if len(raw) == 0 || len(raw) > mutationResultJSONLimit {
		return "", false
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return "", false
	}
	return parseGoalStatusResult(decoded, depth)
}

func terminalGoalStatus(value any) (GoalStatus, bool) {
	status := NormalizeGoalStatus(GoalStatus(mutationResultString(value)))
	switch status {
	case GoalStatusComplete, GoalStatusBlocked:
		return status, true
	default:
		return "", false
	}
}

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
