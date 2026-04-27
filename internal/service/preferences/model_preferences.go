package preferences

import (
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// Preferences 表示当前用户的界面与运行默认偏好。
type Preferences struct {
	ChatDefaultDeliveryPolicy protocol.ChatDeliveryPolicy `json:"chat_default_delivery_policy"`
	DefaultAgentOptions       protocol.Options            `json:"default_agent_options"`
	UpdatedAt                 string                      `json:"updated_at,omitempty"`
}

// UpdateRequest 表示偏好更新请求。字段为 nil 时保留原值。
type UpdateRequest struct {
	ChatDefaultDeliveryPolicy *string           `json:"chat_default_delivery_policy,omitempty"`
	DefaultAgentOptions       *protocol.Options `json:"default_agent_options,omitempty"`
}

// DefaultAllowedTools 返回新建 Agent 默认启用的工具集合。
func DefaultAllowedTools() []string {
	return []string{
		"Task",
		"TaskOutput",
		"Bash",
		"Glob",
		"Grep",
		"LS",
		"ExitPlanMode",
		"Read",
		"Edit",
		"Write",
		"NotebookEdit",
		"WebFetch",
		"TodoWrite",
		"WebSearch",
		"KillShell",
		"AskUserQuestion",
		"Skill",
		"EnterPlanMode",
	}
}

// DefaultPreferences 返回系统默认偏好。
func DefaultPreferences() Preferences {
	return normalizePreferences(Preferences{
		ChatDefaultDeliveryPolicy: protocol.ChatDeliveryPolicyQueue,
		DefaultAgentOptions: protocol.Options{
			PermissionMode:  "bypassPermissions",
			AllowedTools:    DefaultAllowedTools(),
			DisallowedTools: []string{},
			SettingSources:  []string{"project"},
		},
	})
}

func normalizePreferences(item Preferences) Preferences {
	policy := protocol.NormalizeChatDeliveryPolicy(string(item.ChatDefaultDeliveryPolicy))
	options := item.DefaultAgentOptions
	if strings.TrimSpace(options.PermissionMode) == "" {
		options.PermissionMode = "bypassPermissions"
	}
	options.PermissionMode = strings.TrimSpace(options.PermissionMode)
	options.Provider = strings.TrimSpace(options.Provider)
	options.AllowedTools = normalizeStringSlice(options.AllowedTools)
	options.DisallowedTools = normalizeStringSlice(options.DisallowedTools)
	if options.AllowedTools == nil {
		options.AllowedTools = []string{}
	}
	if options.DisallowedTools == nil {
		options.DisallowedTools = []string{}
	}
	if len(options.SettingSources) == 0 {
		options.SettingSources = []string{"project"}
	} else {
		options.SettingSources = normalizeStringSlice(options.SettingSources)
	}
	return Preferences{
		ChatDefaultDeliveryPolicy: policy,
		DefaultAgentOptions:       options,
		UpdatedAt:                 strings.TrimSpace(item.UpdatedAt),
	}
}

func normalizeStringSlice(values []string) []string {
	if values == nil {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, item := range values {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}
