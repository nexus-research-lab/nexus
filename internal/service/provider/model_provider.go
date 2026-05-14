package provider

import (
	"time"
)

// Record 表示对外暴露的 Provider 配置。
type Record struct {
	ID              string     `json:"id"`
	ProviderKind    string     `json:"provider_kind"`
	Provider        string     `json:"provider"`
	DisplayName     string     `json:"display_name"`
	AuthTokenMasked string     `json:"auth_token_masked"`
	BaseURL         string     `json:"base_url"`
	Model           string     `json:"model"`
	Enabled         bool       `json:"enabled"`
	IsDefault       bool       `json:"is_default"`
	UsageCount      int        `json:"usage_count"`
	CreatedAt       *time.Time `json:"created_at,omitempty"`
	UpdatedAt       *time.Time `json:"updated_at,omitempty"`
}

// Option 表示可供 Agent 选择的 Provider 选项。
type Option struct {
	Provider    string `json:"provider"`
	DisplayName string `json:"display_name"`
	IsDefault   bool   `json:"is_default"`
}

// OptionsResponse 表示 Provider 下拉选项响应。
type OptionsResponse struct {
	DefaultProvider *string  `json:"default_provider"`
	Items           []Option `json:"items"`
}

// CreateInput 表示新增 Provider 配置的输入。
type CreateInput struct {
	ProviderKind string `json:"provider_kind"`
	Provider     string `json:"provider"`
	DisplayName  string `json:"display_name"`
	AuthToken    string `json:"auth_token"`
	BaseURL      string `json:"base_url"`
	Model        string `json:"model"`
	Enabled      bool   `json:"enabled"`
	IsDefault    bool   `json:"is_default"`
}

// UpdateInput 表示更新 Provider 配置的输入。
type UpdateInput struct {
	ProviderKind string  `json:"provider_kind"`
	DisplayName  string  `json:"display_name"`
	AuthToken    *string `json:"auth_token,omitempty"`
	BaseURL      string  `json:"base_url"`
	Model        string  `json:"model"`
	Enabled      bool    `json:"enabled"`
	IsDefault    bool    `json:"is_default"`
}

// ImageConfig 表示图片生成要使用的 Provider 运行时配置。
type ImageConfig struct {
	Provider    string
	DisplayName string
	AuthToken   string
	BaseURL     string
	Model       string
}
