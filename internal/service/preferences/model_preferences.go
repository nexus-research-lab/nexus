package preferences

import (
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimeprovider "github.com/nexus-research-lab/nexus/internal/runtime/provider"
)

// Preferences 表示当前用户的界面与运行默认偏好。
type Preferences struct {
	ChatDefaultDeliveryPolicy       protocol.ChatDeliveryPolicy `json:"chat_default_delivery_policy"`
	AgentRuntimeKind                string                      `json:"agent_runtime_kind,omitempty"`
	AgentSDKDiagnosticsEnabled      bool                        `json:"agent_sdk_diagnostics_enabled,omitempty"`
	RuntimeSettings                 RuntimeSettings             `json:"runtime_settings"`
	DefaultAgentOptions             protocol.Options            `json:"default_agent_options"`
	DefaultImageModelSelection      ModelSelection              `json:"default_image_model_selection,omitempty"`
	DefaultVisionModelSelection     ModelSelection              `json:"default_vision_model_selection,omitempty"`
	DefaultBackgroundModelSelection ModelSelection              `json:"default_background_model_selection,omitempty"`
	UpdatedAt                       string                      `json:"updated_at,omitempty"`
}

// UpdateRequest 表示偏好更新请求。字段为 nil 时保留原值。
type UpdateRequest struct {
	ChatDefaultDeliveryPolicy       *protocol.ChatDeliveryPolicy `json:"chat_default_delivery_policy,omitempty"`
	AgentRuntimeKind                *string                      `json:"agent_runtime_kind,omitempty"`
	AgentSDKDiagnosticsEnabled      *bool                        `json:"agent_sdk_diagnostics_enabled,omitempty"`
	RuntimeSettings                 *RuntimeSettings             `json:"runtime_settings,omitempty"`
	DefaultAgentOptions             *protocol.Options            `json:"default_agent_options,omitempty"`
	DefaultImageModelSelection      *ModelSelection              `json:"default_image_model_selection,omitempty"`
	DefaultVisionModelSelection     *ModelSelection              `json:"default_vision_model_selection,omitempty"`
	DefaultBackgroundModelSelection *ModelSelection              `json:"default_background_model_selection,omitempty"`
}

// RuntimeSettings 保存按 runtime 隔离的设置。
// 新增 runtime 设置时只扩展对应 runtime 的字段，不把内核差异摊平成全局开关。
type RuntimeSettings map[string]RuntimeSettingsForKind

// RuntimeSettingsForKind 表示一个 runtime 当前可配置的选项。
type RuntimeSettingsForKind struct {
	ToolSearch bool `json:"tool_search"`
}

// ModelSelection 表示一个 Provider + Model 选择。
type ModelSelection struct {
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
}

// DefaultPreferences 返回系统默认偏好。
func DefaultPreferences() Preferences {
	return normalizePreferences(Preferences{
		ChatDefaultDeliveryPolicy: protocol.ChatDeliveryPolicyQueue,
		AgentRuntimeKind:          "nxs",
		RuntimeSettings: RuntimeSettings{
			runtimeprovider.RuntimeKindNXS: {},
		},
		DefaultAgentOptions: protocol.Options{
			PermissionMode:  "default",
			AllowedTools:    []string{},
			DisallowedTools: []string{},
			SettingSources:  []string{"project"},
		},
	})
}

func normalizePreferences(item Preferences) Preferences {
	policy := item.ChatDefaultDeliveryPolicy
	if policy == "" {
		policy = protocol.ChatDeliveryPolicyQueue
	}
	runtimeKind := runtimeprovider.NormalizeRuntimeKind(item.AgentRuntimeKind)
	options := item.DefaultAgentOptions
	if strings.TrimSpace(options.PermissionMode) == "" {
		options.PermissionMode = "default"
	}
	options.PermissionMode = strings.TrimSpace(options.PermissionMode)
	options.Provider = strings.TrimSpace(options.Provider)
	options.Model = strings.TrimSpace(options.Model)
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
		ChatDefaultDeliveryPolicy:       policy,
		AgentRuntimeKind:                runtimeKind,
		AgentSDKDiagnosticsEnabled:      item.AgentSDKDiagnosticsEnabled,
		RuntimeSettings:                 normalizeRuntimeSettings(item.RuntimeSettings),
		DefaultAgentOptions:             options,
		DefaultImageModelSelection:      normalizeModelSelection(item.DefaultImageModelSelection),
		DefaultVisionModelSelection:     normalizeModelSelection(item.DefaultVisionModelSelection),
		DefaultBackgroundModelSelection: normalizeModelSelection(item.DefaultBackgroundModelSelection),
		UpdatedAt:                       strings.TrimSpace(item.UpdatedAt),
	}
}

func normalizeRuntimeSettings(settings RuntimeSettings) RuntimeSettings {
	result := make(RuntimeSettings, len(settings)+1)
	result[runtimeprovider.RuntimeKindNXS] = RuntimeSettingsForKind{}
	for kind, item := range settings {
		normalizedKind := normalizeRuntimeSettingKind(kind)
		if normalizedKind == "" {
			continue
		}
		result[normalizedKind] = item
	}
	return result
}

func normalizeRuntimeSettingKind(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case runtimeprovider.RuntimeKindNXS:
		return runtimeprovider.RuntimeKindNXS
	case runtimeprovider.RuntimeKindClaude:
		return runtimeprovider.RuntimeKindClaude
	default:
		return ""
	}
}

// ToolSearchEnabledForRuntime 返回指定 runtime 是否启用 ToolSearch。
func (p Preferences) ToolSearchEnabledForRuntime(runtimeKind string) bool {
	if normalizeRuntimeSettingKind(runtimeKind) != runtimeprovider.RuntimeKindNXS {
		return false
	}
	return p.RuntimeSettings[runtimeprovider.RuntimeKindNXS].ToolSearch
}

func normalizeModelSelection(selection ModelSelection) ModelSelection {
	provider := strings.TrimSpace(selection.Provider)
	model := strings.TrimSpace(selection.Model)
	if provider == "" || model == "" {
		return ModelSelection{}
	}
	return ModelSelection{Provider: provider, Model: model}
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
