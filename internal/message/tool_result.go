// INPUT: assistant tool_use/tool_result 内容块。
// OUTPUT: 规范化的工具观察与 Goal 进展判定。
// POS: runtime 消息到产品进展语义的统一投影。
package message

import (
	"strings"
	"unicode"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// ToolResultObservation 表示 assistant 快照中一次已物化的工具结果。
type ToolResultObservation struct {
	ToolUseID string
	ToolName  string
	ErrorCode string
	IsError   bool
}

// AssistantToolResults 从 assistant 快照里提取 tool_result，并用同快照中的 tool_use 补齐工具名。
func AssistantToolResults(message protocol.Message) []ToolResultObservation {
	if protocol.MessageRole(message) != "assistant" {
		return nil
	}
	blocks := messageContentBlocks(message["content"])
	if len(blocks) == 0 {
		return nil
	}
	toolNames := make(map[string]string)
	for _, block := range blocks {
		if normalizeString(block["type"]) != "tool_use" {
			continue
		}
		toolUseID := normalizeString(block["id"])
		if toolUseID == "" {
			continue
		}
		toolNames[toolUseID] = normalizeString(block["name"])
	}
	observations := make([]ToolResultObservation, 0)
	for _, block := range blocks {
		if normalizeString(block["type"]) != "tool_result" {
			continue
		}
		toolUseID := normalizeString(block["tool_use_id"])
		if toolUseID == "" {
			continue
		}
		observations = append(observations, ToolResultObservation{
			ToolUseID: toolUseID,
			ToolName:  toolNames[toolUseID],
			ErrorCode: normalizeString(block["error_code"]),
			IsError:   boolValue(block["is_error"]),
		})
	}
	return observations
}

// AssistantHasCountedToolProgress 判断 assistant 快照里是否包含应计为 Goal 进展的工具完成。
func AssistantHasCountedToolProgress(message protocol.Message) bool {
	for _, observation := range AssistantToolResults(message) {
		if toolResultCountsForGoalProgress(observation) {
			return true
		}
	}
	return false
}

// AssistantMissedGoalCompletionTool 判断 assistant 是否声称目标已完成，但把 Goal 完成工具误判为不可用。
func AssistantMissedGoalCompletionTool(message protocol.Message) bool {
	if protocol.MessageRole(message) != "assistant" {
		return false
	}
	if assistantHasSuccessfulGoalUpdateTool(message) {
		return false
	}
	text := normalizeCompletionSignalText(ExtractAssistantDisplayText(message))
	if text == "" {
		return false
	}
	if claimsPartialGoalWorkOrContinuation(text) {
		return false
	}
	if claimsFinalGoalWorkComplete(text) {
		return true
	}
	return mentionsGoalUpdateTool(text) &&
		claimsGoalUpdateToolUnavailable(text) &&
		claimsGoalWorkComplete(text)
}

func toolResultCountsForGoalProgress(observation ToolResultObservation) bool {
	switch CanonicalToolName(observation.ToolName) {
	case "", "update_goal":
		return false
	case "retarget_goal":
		return !observation.IsError
	}
	switch normalizeString(observation.ErrorCode) {
	case askUserQuestionTimeoutErrorCode, askUserQuestionChannelUnavailableCode:
		return false
	default:
		return true
	}
}

func assistantHasSuccessfulGoalUpdateTool(message protocol.Message) bool {
	for _, observation := range AssistantToolResults(message) {
		if observation.IsError {
			continue
		}
		if CanonicalToolName(observation.ToolName) == "update_goal" {
			return true
		}
	}
	return false
}

// CanonicalToolName 把 SDK/MCP 展示名规整为模型工具短名。
func CanonicalToolName(name string) string {
	name = normalizeString(name)
	if name == "" {
		return ""
	}
	if strings.HasPrefix(name, "mcp__") {
		parts := strings.Split(name, "__")
		if len(parts) >= 3 {
			return strings.TrimSpace(parts[len(parts)-1])
		}
	}
	return name
}

func normalizeCompletionSignalText(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return ""
	}
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		return r
	}, text)
}

func mentionsGoalUpdateTool(text string) bool {
	for _, marker := range []string{
		"mcp__nexus_goal__update_goal",
		"update_goal",
		"nexus_goal",
		"goal update tool",
		"更新目标",
		"停止目标",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func claimsGoalUpdateToolUnavailable(text string) bool {
	for _, marker := range []string{
		"not available",
		"unavailable",
		"not exposed",
		"not visible",
		"not in the tool list",
		"cannot call",
		"can't call",
		"could not call",
		"unable to call",
		"no access",
		"don't see",
		"do not see",
		"missing",
		"找不到",
		"没找到",
		"没有看到",
		"没看到",
		"没有权限",
		"无法调用",
		"不能调用",
		"不可用",
		"没有这个工具",
		"没有这样的工具",
		"工具不存在",
		"未暴露",
		"没暴露",
		"不在工具",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func claimsPartialGoalWorkOrContinuation(text string) bool {
	for _, marker := range []string{
		"下一步",
		"下一个步骤",
		"下一阶段",
		"还需要",
		"仍需要",
		"仍需",
		"剩余",
		"未完成",
		"没完成",
		"尚未完成",
		"后续",
		"需要继续",
		"next step",
		"next phase",
		"remaining",
		"still need",
		"still needs",
		"not complete",
		"not completed",
		"not done",
		"unfinished",
		"follow-up",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	if strings.Contains(text, "阶段") && !strings.Contains(text, "所有阶段") && !strings.Contains(text, "全部阶段") {
		return true
	}
	if strings.Contains(text, "phase") && !strings.Contains(text, "all phases") {
		return true
	}
	if strings.Contains(text, "stage") && !strings.Contains(text, "all stages") {
		return true
	}
	return false
}

func claimsGoalWorkComplete(text string) bool {
	for _, marker := range []string{
		"goal is complete",
		"goal has been completed",
		"task is complete",
		"task has been completed",
		"work is complete",
		"work has been completed",
		"deliverable is complete",
		"deliverable has been completed",
		"all requirements are satisfied",
		"no required work remains",
		"already complete",
		"already completed",
		"目标已经完成",
		"目标已完成",
		"任务已经完成",
		"任务已完成",
		"工作已经完成",
		"工作已完成",
		"交付成果已经完成",
		"交付成果已完成",
		"所有要求都已满足",
		"所有要求已经满足",
		"已经完成",
		"已完成",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func claimsFinalGoalWorkComplete(text string) bool {
	for _, marker := range []string{
		"goal is complete",
		"goal has been completed",
		"task is complete",
		"task has been completed",
		"work is complete",
		"work has been completed",
		"deliverable is complete",
		"deliverable has been completed",
		"all requirements are satisfied",
		"no required work remains",
		"completed and verified",
		"done and verified",
		"目标已经完成",
		"目标已完成",
		"任务已经完成",
		"任务已完成",
		"工作已经完成",
		"工作已完成",
		"交付成果已经完成",
		"交付成果已完成",
		"所有要求都已满足",
		"所有要求已经满足",
		"所有阶段已完成",
		"全部阶段已完成",
		"已完成并验证",
		"完成并验证",
		"已完成并可用",
		"完成并可用",
		"已交付",
	} {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return false
}

func messageContentBlocks(value any) []map[string]any {
	switch typed := value.(type) {
	case []map[string]any:
		return cloneBlockSlice(typed)
	case []any:
		blocks := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			block, ok := item.(map[string]any)
			if !ok {
				continue
			}
			blocks = append(blocks, cloneMap(block))
		}
		return blocks
	default:
		return nil
	}
}
