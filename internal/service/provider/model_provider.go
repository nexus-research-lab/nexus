package provider

import "time"

// Record 表示对外暴露的 Provider 配置。
type Record struct {
	ID              string     `json:"id"`
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
	Provider    string `json:"provider"`
	DisplayName string `json:"display_name"`
	AuthToken   string `json:"auth_token"`
	BaseURL     string `json:"base_url"`
	Model       string `json:"model"`
	Enabled     bool   `json:"enabled"`
	IsDefault   bool   `json:"is_default"`
}

// UpdateInput 表示更新 Provider 配置的输入。
type UpdateInput struct {
	DisplayName string  `json:"display_name"`
	AuthToken   *string `json:"auth_token,omitempty"`
	BaseURL     string  `json:"base_url"`
	Model       string  `json:"model"`
	Enabled     bool    `json:"enabled"`
	IsDefault   bool    `json:"is_default"`
}

// RuntimeConfig 表示运行时使用的 Provider 解析结果。
type RuntimeConfig struct {
	Provider    string
	DisplayName string
	AuthToken   string
	BaseURL     string
	Model       string
}

type entity struct {
	ID          string
	Provider    string
	DisplayName string
	AuthToken   string
	BaseURL     string
	Model       string
	Enabled     bool
	IsDefault   bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
