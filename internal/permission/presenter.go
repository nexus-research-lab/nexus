package permission

import (
	"strings"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
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

func buildPermissionPayload(pending *PendingRequest) map[string]any {
	riskLevel, riskLabel := resolveRisk(pending.ToolName)
	return map[string]any{
		"request_id":       pending.RequestID,
		"tool_name":        pending.ToolName,
		"tool_input":       pending.ToolInput,
		"interaction_mode": resolveInteractionMode(pending.ToolName),
		"risk_level":       riskLevel,
		"risk_label":       riskLabel,
		"summary":          summarizeInput(pending.ToolName, pending.ToolInput),
		"suggestions":      serializePermissionUpdates(pending.Suggestions),
		"expires_at":       pending.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
	}
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
		return "question"
	}
	return "permission"
}

func summarizeInput(toolName string, input map[string]any) string {
	if toolName == "Bash" {
		if command := strings.TrimSpace(normalizeString(input["command"])); command != "" {
			return command
		}
	}
	for _, key := range []string{"file_path", "path", "target_file", "cwd", "url", "query"} {
		if value := strings.TrimSpace(normalizeString(input[key])); value != "" {
			return value
		}
	}
	if toolName == "AskUserQuestion" {
		if questions, ok := input["questions"].([]any); ok && len(questions) > 0 {
			if payload, ok := questions[0].(map[string]any); ok {
				if question := strings.TrimSpace(normalizeString(payload["question"])); question != "" {
					return question
				}
			}
		}
	}
	for _, key := range []string{"description", "task", "prompt"} {
		if value := strings.TrimSpace(normalizeString(input[key])); value != "" {
			return value
		}
	}
	return toolName
}

func serializePermissionUpdates(updates []sdkprotocol.PermissionUpdate) []map[string]any {
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
