package provider

import "time"

// Entity 表示 provider 表的一行持久化记录。
type Entity struct {
	ID             string
	OwnerUserID    string
	Visibility     string
	ProviderKind   string
	Provider       string
	PresetKey      string
	APIFormat      string
	DisplayName    string
	AuthToken      string
	BaseURL        string
	ModelsPath     string
	Enabled        bool
	LastTestStatus string
	LastTestError  string
	LastTestAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// ModelEntity 表示 provider_models 表的一行模型卡记录。
type ModelEntity struct {
	ID                       string
	ProviderID               string
	ModelID                  string
	DisplayName              string
	Category                 string
	Enabled                  bool
	IsDefault                bool
	CapabilitiesAutoJSON     string
	CapabilitiesOverrideJSON string
	ContextWindow            *int
	MaxOutputTokens          *int
	ProviderOptionsJSON      string
	LastSeenAt               time.Time
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

// UsageAgentEntity 表示正在使用某个 Provider 的 Agent 摘要。
type UsageAgentEntity struct {
	Provider    string
	AgentID     string
	Name        string
	DisplayName string
	Avatar      string
	IsMain      bool
}

// RuntimeBindingEntity 表示 Agent 当前持久化的显式模型绑定。
// 空 provider/model 表示跟随用户默认模型。
type RuntimeBindingEntity struct {
	AgentID  string
	Provider string
	Model    string
	IsMain   bool
}
