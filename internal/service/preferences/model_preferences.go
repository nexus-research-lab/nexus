package preferences

import (
	"fmt"
	"strings"
	"time"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimepermission "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	runtimeprovider "github.com/nexus-research-lab/nexus/internal/runtime/provider"
)

// Preferences 表示当前用户的界面与运行默认偏好。
type Preferences struct {
	ChatDefaultDeliveryPolicy       protocol.ChatDeliveryPolicy `json:"chat_default_delivery_policy"`
	AgentRuntimeKind                string                      `json:"agent_runtime_kind,omitempty"`
	AgentSDKDiagnosticsEnabled      bool                        `json:"agent_sdk_diagnostics_enabled,omitempty"`
	RuntimeSettings                 RuntimeSettings             `json:"runtime_settings"`
	WebSearch                       WebSearchSettings           `json:"web_search"`
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
	WebSearch                       *WebSearchSettings           `json:"web_search,omitempty"`
	WebSearchAPIKey                 *string                      `json:"web_search_api_key,omitempty"`
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

// WebSearchSettings 描述 SDK 内置 WebSearch 的可持久化配置。
// 原始 API key 不进入 JSON；仅返回安全的脱敏摘要，并在服务端内存中参与 runtime 装配。
type WebSearchSettings struct {
	Enabled             bool              `json:"enabled"`
	Provider            string            `json:"provider,omitempty"`
	BaseURL             string            `json:"base_url,omitempty"`
	AllowPrivateNetwork bool              `json:"allow_private_network,omitempty"`
	UseProviderExtract  bool              `json:"use_provider_extract,omitempty"`
	DefaultCount        int               `json:"default_count,omitempty"`
	TimeoutSeconds      int               `json:"timeout_seconds,omitempty"`
	CacheTTLSeconds     int               `json:"cache_ttl_seconds,omitempty"`
	Country             string            `json:"country,omitempty"`
	Language            string            `json:"language,omitempty"`
	SearchLanguage      string            `json:"search_language,omitempty"`
	Freshness           string            `json:"freshness,omitempty"`
	SearchDepth         string            `json:"search_depth,omitempty"`
	ExtractDepth        string            `json:"extract_depth,omitempty"`
	AnySearch           AnySearchSettings `json:"anysearch,omitempty"`
	APIKeyConfigured    bool              `json:"api_key_configured,omitempty"`
	APIKeyMasked        string            `json:"api_key_masked,omitempty"`
	apiKey              string
}

// AnySearchSettings 描述 AnySearch 的垂直搜索参数。
type AnySearchSettings struct {
	Domain       string         `json:"domain,omitempty"`
	Tag          string         `json:"tag,omitempty"`
	ContentTypes []string       `json:"content_types,omitempty"`
	Params       map[string]any `json:"params,omitempty"`
}

type webSearchProviderRequirement struct {
	requiresAPIKey  bool
	requiresBaseURL bool
	supportsAPIKey  bool
}

const defaultWebSearchProvider = "anysearch"

var webSearchProviderRequirements = map[string]webSearchProviderRequirement{
	"anysearch": {supportsAPIKey: true},
	"brave":     {requiresAPIKey: true, supportsAPIKey: true},
	"exa":       {requiresAPIKey: true, supportsAPIKey: true},
	"firecrawl": {requiresAPIKey: true, supportsAPIKey: true},
	"searxng":   {requiresBaseURL: true},
	"tavily":    {requiresAPIKey: true, supportsAPIKey: true},
}

// WebSearchAPIKey 返回当前用户的 WebSearch 凭据，仅供服务端 runtime 装配使用。
func (p Preferences) WebSearchAPIKey() string {
	return strings.TrimSpace(p.WebSearch.apiKey)
}

// WebSearchAPIKey 返回 WebSearchSettings 中的服务端凭据。
func (s WebSearchSettings) WebSearchAPIKey() string {
	return strings.TrimSpace(s.apiKey)
}

// WithWebSearchAPIKey 为服务端 runtime 装配构造带凭据的配置副本。
func (s WebSearchSettings) WithWebSearchAPIKey(apiKey string) WebSearchSettings {
	s.apiKey = ""
	if webSearchProviderAcceptsAPIKey(s.Provider) {
		s.apiKey = strings.TrimSpace(apiKey)
	}
	s.APIKeyConfigured = s.apiKey != ""
	s.APIKeyMasked = maskWebSearchAPIKey(s.apiKey)
	return s
}

func maskWebSearchAPIKey(apiKey string) string {
	trimmed := strings.TrimSpace(apiKey)
	if trimmed == "" {
		return ""
	}
	if len(trimmed) <= 10 {
		return strings.Repeat("*", len(trimmed))
	}
	return trimmed[:5] + strings.Repeat("*", 24) + trimmed[len(trimmed)-5:]
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
		WebSearch: WebSearchSettings{Enabled: true},
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
	options.PermissionMode = string(runtimepermission.NormalizeMode(sdkpermission.Mode(options.PermissionMode)))
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
	webSearch := normalizeWebSearchSettings(item.WebSearch)
	if webSearchProviderAcceptsAPIKey(webSearch.Provider) {
		webSearch.apiKey = strings.TrimSpace(item.WebSearch.apiKey)
	}
	webSearch.APIKeyConfigured = webSearch.apiKey != ""
	webSearch.APIKeyMasked = maskWebSearchAPIKey(webSearch.apiKey)
	return Preferences{
		ChatDefaultDeliveryPolicy:       policy,
		AgentRuntimeKind:                runtimeKind,
		AgentSDKDiagnosticsEnabled:      item.AgentSDKDiagnosticsEnabled,
		RuntimeSettings:                 normalizeRuntimeSettings(item.RuntimeSettings),
		WebSearch:                       webSearch,
		DefaultAgentOptions:             options,
		DefaultImageModelSelection:      normalizeModelSelection(item.DefaultImageModelSelection),
		DefaultVisionModelSelection:     normalizeModelSelection(item.DefaultVisionModelSelection),
		DefaultBackgroundModelSelection: normalizeModelSelection(item.DefaultBackgroundModelSelection),
		UpdatedAt:                       strings.TrimSpace(item.UpdatedAt),
	}
}

func normalizeWebSearchSettings(settings WebSearchSettings) WebSearchSettings {
	settings.Provider = strings.ToLower(strings.TrimSpace(settings.Provider))
	if settings.Provider == "" {
		settings.Provider = defaultWebSearchProvider
	}
	settings.BaseURL = strings.TrimSpace(settings.BaseURL)
	settings.Country = strings.TrimSpace(settings.Country)
	settings.Language = strings.TrimSpace(settings.Language)
	settings.SearchLanguage = strings.TrimSpace(settings.SearchLanguage)
	settings.Freshness = strings.ToLower(strings.TrimSpace(settings.Freshness))
	settings.SearchDepth = normalizeWebSearchDepth(settings.SearchDepth)
	settings.ExtractDepth = normalizeWebSearchDepth(settings.ExtractDepth)
	if settings.DefaultCount <= 0 {
		settings.DefaultCount = 5
	}
	settings.DefaultCount = minInt(maxInt(settings.DefaultCount, 1), 20)
	if settings.TimeoutSeconds <= 0 {
		settings.TimeoutSeconds = 20
	}
	if settings.CacheTTLSeconds < 0 {
		settings.CacheTTLSeconds = 0
	}
	settings.AnySearch = normalizeAnySearchSettings(settings.AnySearch)
	if settings.Provider == defaultWebSearchProvider || (settings.Provider == "searxng" && settings.BaseURL != "") {
		settings.Enabled = true
	}
	settings.APIKeyConfigured = false
	settings.APIKeyMasked = ""
	settings.apiKey = ""
	return settings
}

func normalizeWebSearchDepth(value string) string {
	if strings.EqualFold(strings.TrimSpace(value), "advanced") {
		return "advanced"
	}
	return "basic"
}

func normalizeAnySearchSettings(settings AnySearchSettings) AnySearchSettings {
	settings.Domain = strings.TrimSpace(settings.Domain)
	settings.Tag = strings.TrimSpace(settings.Tag)
	settings.ContentTypes = normalizeStringSlice(settings.ContentTypes)
	if settings.Params != nil {
		params := make(map[string]any, len(settings.Params))
		for key, value := range settings.Params {
			params[strings.TrimSpace(key)] = value
		}
		settings.Params = params
	}
	return settings
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}

func webSearchProviderRequiresAPIKey(provider string) bool {
	requirement, exists := webSearchProviderRequirements[strings.ToLower(strings.TrimSpace(provider))]
	return exists && requirement.requiresAPIKey
}

func webSearchProviderAcceptsAPIKey(provider string) bool {
	requirement, exists := webSearchProviderRequirements[strings.ToLower(strings.TrimSpace(provider))]
	return exists && requirement.supportsAPIKey
}

func validateWebSearchSettings(settings WebSearchSettings) error {
	if !settings.Enabled {
		return nil
	}
	requirement, exists := webSearchProviderRequirements[settings.Provider]
	if !exists {
		return fmt.Errorf("WebSearch provider 不支持: %s", settings.Provider)
	}
	if requirement.requiresAPIKey && settings.WebSearchAPIKey() == "" {
		return fmt.Errorf("WebSearch provider %s 缺少 API key", settings.Provider)
	}
	if requirement.requiresBaseURL && settings.BaseURL == "" {
		return fmt.Errorf("WebSearch provider %s 缺少 Base URL", settings.Provider)
	}
	return nil
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
