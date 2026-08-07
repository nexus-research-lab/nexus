// INPUT: runtime 类型与服务层已投影的 WebSearch 配置。
// OUTPUT: nxs 可消费的 WebSearch 环境变量。
// POS: clientopts 内不依赖持久化偏好模型的搜索配置边界。
package clientopts

import (
	"encoding/json"
	"maps"
	"slices"
	"strings"
)

// WebSearchConfig 表示 runtime 启动时所需的 WebSearch 配置。
type WebSearchConfig struct {
	Enabled             bool
	Provider            string
	BaseURL             string
	AllowPrivateNetwork bool
	UseProviderExtract  bool
	DefaultCount        int
	TimeoutSeconds      int
	CacheTTLSeconds     int
	Country             string
	Language            string
	SearchLanguage      string
	Freshness           string
	SearchDepth         string
	ExtractDepth        string
	AnySearch           AnySearchConfig
	apiKey              string
}

// AnySearchConfig 表示 AnySearch 的垂直搜索参数。
type AnySearchConfig struct {
	Domain       string
	Tag          string
	ContentTypes []string
	Params       map[string]any
}

// WithAPIKey 返回带服务端搜索凭据的配置副本。
// 密钥保持私有，避免 WebSearchConfig 被通用序列化时泄漏。
func (c WebSearchConfig) WithAPIKey(apiKey string) WebSearchConfig {
	c.apiKey = strings.TrimSpace(apiKey)
	return c
}

// BuildWebSearchRuntimeEnv 将 WebSearch 配置投影到 nxs。
func BuildWebSearchRuntimeEnv(runtimeKind string, config WebSearchConfig) map[string]string {
	if !runtimeProfileForKind(runtimeKind).isNXS() {
		return nil
	}
	payload := struct {
		Enabled             bool             `json:"enabled"`
		Provider            string           `json:"provider,omitempty"`
		BaseURL             string           `json:"base_url,omitempty"`
		AllowPrivateNetwork bool             `json:"allow_private_network,omitempty"`
		UseProviderExtract  bool             `json:"use_provider_extract,omitempty"`
		DefaultCount        int              `json:"default_count,omitempty"`
		TimeoutSeconds      int              `json:"timeout_seconds,omitempty"`
		CacheTTLSeconds     int              `json:"cache_ttl_seconds"`
		Country             string           `json:"country,omitempty"`
		Language            string           `json:"language,omitempty"`
		SearchLanguage      string           `json:"search_language,omitempty"`
		Freshness           string           `json:"freshness,omitempty"`
		SearchDepth         string           `json:"search_depth,omitempty"`
		ExtractDepth        string           `json:"extract_depth,omitempty"`
		AnySearch           *AnySearchConfig `json:"anysearch,omitempty"`
	}{
		Enabled:             config.Enabled,
		Provider:            config.Provider,
		BaseURL:             config.BaseURL,
		AllowPrivateNetwork: config.AllowPrivateNetwork,
		UseProviderExtract:  config.UseProviderExtract,
		DefaultCount:        config.DefaultCount,
		TimeoutSeconds:      config.TimeoutSeconds,
		CacheTTLSeconds:     config.CacheTTLSeconds,
		Country:             config.Country,
		Language:            config.Language,
		SearchLanguage:      config.SearchLanguage,
		Freshness:           config.Freshness,
		SearchDepth:         config.SearchDepth,
		ExtractDepth:        config.ExtractDepth,
		AnySearch:           optionalAnySearchConfig(config.AnySearch),
	}
	rawConfig, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	return map[string]string{
		"NEXUS_WEBSEARCH_CONFIG":  string(rawConfig),
		"NEXUS_WEBSEARCH_API_KEY": config.apiKey,
	}
}

func optionalAnySearchConfig(config AnySearchConfig) *AnySearchConfig {
	if config.Domain == "" && config.Tag == "" && len(config.ContentTypes) == 0 && len(config.Params) == 0 {
		return nil
	}
	config.ContentTypes = slices.Clone(config.ContentTypes)
	config.Params = maps.Clone(config.Params)
	return &config
}
