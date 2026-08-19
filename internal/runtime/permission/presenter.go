package permission

import (
	"strings"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

var (
	readOnlyTools = map[string]struct{}{
		"Read": {}, "Glob": {}, "Grep": {}, "LS": {}, "WebFetch": {}, "WebSearch": {}, "Skill": {},
	}
	editTools = map[string]struct{}{
		"Edit": {}, "Write": {}, "NotebookEdit": {}, "TodoWrite": {},
	}
	executeTools = map[string]struct{}{
		"Bash": {}, "KillShell": {}, "Task": {}, "TaskOutput": {},
	}
	interactiveTools = map[string]struct{}{
		"AskUserQuestion": {}, "EnterPlanMode": {}, "ExitPlanMode": {},
	}
)

const (
	interactionModeApproval = "permission"
	interactionModeQuestion = "question"
)

func buildPermissionPayload(pending *PendingRequest) map[string]any {
	riskLevel, riskLabel := resolveRisk(pending.ToolName)
	payload := map[string]any{
		"request_id":       pending.RequestID,
		"round_id":         strings.TrimSpace(pending.Route.RoundID),
		"agent_round_id":   strings.TrimSpace(pending.Route.AgentRoundID),
		"agent_id":         strings.TrimSpace(pending.Route.AgentID),
		"message_id":       strings.TrimSpace(pending.Route.MessageID),
		"tool_use_id":      strings.TrimSpace(pending.ToolUseID),
		"tool_name":        pending.ToolName,
		"tool_input":       pending.ToolInput,
		"interaction_mode": resolveInteractionMode(pending.ToolName),
		"risk_level":       riskLevel,
		"risk_label":       riskLabel,
		"summary":          summarizeInput(pending.ToolName, pending.ToolInput),
		"suggestions":      serializePermissionUpdates(pending.Suggestions),
	}
	if len(pending.ConfigurationSecretSlots) > 0 {
		payload["configuration_secret_slots"] = pending.ConfigurationSecretSlots
	}
	return payload
}

func resolveRisk(toolName string) (string, string) {
	if _, ok := readOnlyTools[toolName]; ok {
		return "low", "只读"
	}
	if _, ok := editTools[toolName]; ok {
		return "medium", "写入"
	}
	if _, ok := executeTools[toolName]; ok {
		return "high", "执行"
	}
	if _, ok := interactiveTools[toolName]; ok {
		return "medium", "交互"
	}
	return "high", "敏感"
}

func resolveInteractionMode(toolName string) string {
	if toolName == "AskUserQuestion" {
		return interactionModeQuestion
	}
	// SDK 的权限回调就是 runtime 等待用户响应的统一入口。未知工具也必须
	// 保留为可批准/拒绝的人工交互，不能因为缺少专用 UI 而只能去 Thread 处理。
	return interactionModeApproval
}

func summarizeInput(toolName string, input map[string]any) string {
	if toolName == "Bash" {
		if command := normalizeString(input["command"]); command != "" {
			return command
		}
	}
	for _, key := range []string{
		"file_path",
		"path",
		"target_file",
		"cwd",
		"url",
		"query",
		"objective",
		"logical_key",
		"result_summary",
		"reason",
	} {
		if value := normalizeString(input[key]); value != "" {
			return value
		}
	}
	if toolName == "AskUserQuestion" {
		if questions, ok := input["questions"].([]any); ok && len(questions) > 0 {
			if payload, ok := questions[0].(map[string]any); ok {
				if question := normalizeString(payload["question"]); question != "" {
					return question
				}
			}
		}
	}
	for _, key := range []string{"description", "task", "prompt"} {
		if value := normalizeString(input[key]); value != "" {
			return value
		}
	}
	return toolName
}

func serializePermissionUpdates(updates []sdkpermission.Update) []map[string]any {
	result := make([]map[string]any, 0, len(updates))
	for _, update := range updates {
		payload := map[string]any{
			"type": update.Type,
		}
		if update.Behavior != "" {
			payload["behavior"] = string(update.Behavior)
		}
		if update.Mode != "" {
			payload["mode"] = string(update.Mode)
		}
		if update.Destination != "" {
			payload["destination"] = string(update.Destination)
		}
		if len(update.Directories) > 0 {
			payload["directories"] = update.Directories
		}
		if len(update.Rules) > 0 {
			rules := make([]map[string]any, 0, len(update.Rules))
			for _, rule := range update.Rules {
				rules = append(rules, map[string]any{
					"toolName":    rule.ToolName,
					"ruleContent": rule.RuleContent,
				})
			}
			payload["rules"] = rules
		}
		result = append(result, payload)
	}
	return result
}
