package provider

import (
	"time"
)

const (
	// APIFormatChatCompletions 表示 OpenAI Chat Completions 协议。
	APIFormatChatCompletions = "chat_completions"
	// APIFormatResponses 表示 OpenAI Responses 协议。
	APIFormatResponses = "responses"
	// APIFormatAnthropicMessages 表示 Anthropic Messages 协议。
	APIFormatAnthropicMessages = "anthropic_messages"
)

const (
	// TestStatusSuccess 表示最近一次 Provider 连通性测试成功。
	TestStatusSuccess = "success"
	// TestStatusFailed 表示最近一次 Provider 连通性测试失败。
	TestStatusFailed = "failed"
)

// Record 表示对外暴露的 Provider 配置。
type Record struct {
	ID                    string        `json:"id"`
	ProviderKind          string        `json:"provider_kind"`
	Provider              string        `json:"provider"`
	PresetKey             string        `json:"preset_key"`
	APIFormat             string        `json:"api_format"`
	DisplayName           string        `json:"display_name"`
	AuthTokenMasked       string        `json:"auth_token_masked"`
	BaseURL               string        `json:"base_url"`
	ModelsPath            string        `json:"models_path"`
	Enabled               bool          `json:"enabled"`
	UsageCount            int           `json:"usage_count"`
	UsedByAgents          []UsageAgent  `json:"used_by_agents"`
	LastTestStatus        string        `json:"last_test_status"`
	LastTestError         string        `json:"last_test_error"`
	LastTestAt            *time.Time    `json:"last_test_at,omitempty"`
	AgentRuntimeSupported bool          `json:"agent_runtime_supported"`
	Models                []ModelRecord `json:"models"`
	CreatedAt             *time.Time    `json:"created_at,omitempty"`
	UpdatedAt             *time.Time    `json:"updated_at,omitempty"`
}

// UsageAgent 表示正在使用某个 Provider 的 Agent 摘要。
type UsageAgent struct {
	AgentID     string `json:"agent_id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Avatar      string `json:"avatar,omitempty"`
	IsMain      bool   `json:"is_main,omitempty"`
}

// ModelOption 表示可供 Agent 选择的单个模型。
type ModelOption struct {
	ModelID     string `json:"model_id"`
	DisplayName string `json:"display_name"`
	IsDefault   bool   `json:"is_default"`
}

// ModelSelection 表示完整运行模型选择。
type ModelSelection struct {
	Provider            string `json:"provider"`
	ProviderDisplayName string `json:"provider_display_name"`
	Model               string `json:"model"`
	ModelDisplayName    string `json:"model_display_name"`
}

// Option 表示可供 Agent 选择的 Provider 选项。
type Option struct {
	Provider    string        `json:"provider"`
	DisplayName string        `json:"display_name"`
	Models      []ModelOption `json:"models"`
}

// OptionsResponse 表示 Provider 下拉选项响应。
type OptionsResponse struct {
	DefaultProvider  *string         `json:"default_provider"`
	DefaultModel     *string         `json:"default_model"`
	DefaultSelection *ModelSelection `json:"default_selection"`
	Items            []Option        `json:"items"`
}

// CreateInput 表示新增 Provider 配置的输入。
type CreateInput struct {
	ProviderKind string `json:"provider_kind"`
	Provider     string `json:"provider"`
	PresetKey    string `json:"preset_key"`
	APIFormat    string `json:"api_format"`
	DisplayName  string `json:"display_name"`
	AuthToken    string `json:"auth_token"`
	BaseURL      string `json:"base_url"`
	ModelsPath   string `json:"models_path"`
	Enabled      bool   `json:"enabled"`
}

// UpdateInput 表示更新 Provider 配置的输入。
type UpdateInput struct {
	ProviderKind string  `json:"provider_kind"`
	PresetKey    string  `json:"preset_key"`
	APIFormat    string  `json:"api_format"`
	DisplayName  string  `json:"display_name"`
	AuthToken    *string `json:"auth_token,omitempty"`
	BaseURL      string  `json:"base_url"`
	ModelsPath   string  `json:"models_path"`
	Enabled      bool    `json:"enabled"`
}

// DeleteInput 表示删除 Provider 的行为选项。
type DeleteInput struct {
	Force bool `json:"force"`
}

// DeleteResult 表示 Provider 删除结果。
type DeleteResult struct {
	Provider               string `json:"provider"`
	ReplacementProvider    string `json:"replacement_provider,omitempty"`
	ReplacementModel       string `json:"replacement_model,omitempty"`
	ReassignedRuntimeCount int    `json:"reassigned_runtime_count,omitempty"`
}

// Preset 表示内置 Provider 模板。
type Preset struct {
	PresetKey     string         `json:"preset_key"`
	DisplayName   string         `json:"display_name"`
	Description   string         `json:"description"`
	KeyURL        string         `json:"key_url"`
	DefaultFormat string         `json:"default_api_format"`
	Formats       []PresetFormat `json:"formats"`
}

// PresetFormat 表示预置模板在某个 API Format 下的默认 endpoint。
type PresetFormat struct {
	APIFormat  string `json:"api_format"`
	BaseURL    string `json:"base_url"`
	ModelsPath string `json:"models_path"`
}

// ModelRecord 表示单个 Provider 下的模型卡。
type ModelRecord struct {
	ID                   string            `json:"id"`
	ProviderID           string            `json:"provider_id"`
	ModelID              string            `json:"model_id"`
	DisplayName          string            `json:"display_name"`
	Category             string            `json:"category"`
	Enabled              bool              `json:"enabled"`
	IsDefault            bool              `json:"is_default"`
	CapabilitiesAuto     ModelCapabilities `json:"capabilities_auto"`
	CapabilitiesOverride ModelCapabilities `json:"capabilities_override"`
	ContextWindow        *int              `json:"context_window,omitempty"`
	MaxOutputTokens      *int              `json:"max_output_tokens,omitempty"`
	ProviderOptions      map[string]any    `json:"provider_options"`
	LastSeenAt           *time.Time        `json:"last_seen_at,omitempty"`
	CreatedAt            *time.Time        `json:"created_at,omitempty"`
	UpdatedAt            *time.Time        `json:"updated_at,omitempty"`
}

// ModelCapabilities 描述模型能力。
type ModelCapabilities struct {
	Vision      *bool `json:"vision,omitempty"`
	ImageOutput *bool `json:"image_output,omitempty"`
	ToolCalling *bool `json:"tool_calling,omitempty"`
	Reasoning   *bool `json:"reasoning,omitempty"`
	Embedding   *bool `json:"embedding,omitempty"`
}

// UpdateModelInput 表示模型卡更新输入。
type UpdateModelInput struct {
	Enabled              bool              `json:"enabled"`
	IsDefault            bool              `json:"is_default"`
	CapabilitiesOverride ModelCapabilities `json:"capabilities_override"`
	ContextWindow        *int              `json:"context_window,omitempty"`
	MaxOutputTokens      *int              `json:"max_output_tokens,omitempty"`
	ProviderOptions      map[string]any    `json:"provider_options"`
}

// FetchModelsResult 表示模型拉取结果。
type FetchModelsResult struct {
	Provider string        `json:"provider"`
	Models   []ModelRecord `json:"models"`
	Count    int           `json:"count"`
}

// TestResult 表示 Provider 或模型连通性测试结果。
type TestResult struct {
	Provider string     `json:"provider"`
	Model    string     `json:"model,omitempty"`
	Success  bool       `json:"success"`
	Status   string     `json:"status"`
	Error    string     `json:"error,omitempty"`
	TestedAt *time.Time `json:"tested_at,omitempty"`
}

// ImageConfig 表示图片生成要使用的 Provider 运行时配置。
type ImageConfig struct {
	Provider    string
	DisplayName string
	AuthToken   string
	BaseURL     string
	Model       string
}
