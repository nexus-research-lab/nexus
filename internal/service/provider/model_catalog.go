package provider

import (
	"net/url"
	"strings"
)

// knownModelLimit 描述官方文档明确给出的模型 token 上限。
//
// 目录只负责补齐 Provider 模型列表缺失的字段。远端返回值和用户显式配置始终优先，
// 未列出的 LLM 使用与运行时一致的保守基线，能力仍保持未知。
type knownModelLimit struct {
	Family          string
	Tokens          int
	MaxOutputTokens int
}

const (
	// 缺少模型卡时沿用 nxs 的运行时基线，避免设置页与实际执行分叉。
	defaultModelContextWindow   = 200_000
	defaultModelMaxOutputTokens = 32_000
)

var knownModelLimits = []knownModelLimit{
	// OpenAI。
	{Family: "gpt-5.6-terra", Tokens: 1_050_000, MaxOutputTokens: 128_000},
	{Family: "gpt-5.6-luna", Tokens: 1_050_000, MaxOutputTokens: 128_000},
	{Family: "gpt-5.6-sol", Tokens: 1_050_000, MaxOutputTokens: 128_000},
	{Family: "gpt-5.6", Tokens: 1_050_000, MaxOutputTokens: 128_000},
	{Family: "gpt-5.5-pro", Tokens: 1_050_000},
	{Family: "gpt-5.5", Tokens: 1_000_000},
	{Family: "gpt-5.4-mini", Tokens: 400_000},
	{Family: "gpt-5.4-nano", Tokens: 400_000},
	{Family: "gpt-5.4-pro", Tokens: 1_050_000},
	{Family: "gpt-5.4", Tokens: 1_050_000},
	{Family: "gpt-5.3-codex", Tokens: 400_000},
	{Family: "gpt-5.3-chat-latest", Tokens: 128_000},
	{Family: "gpt-5.3-chat", Tokens: 128_000},
	{Family: "gpt-5.2-codex", Tokens: 400_000},
	{Family: "gpt-5.2-chat-latest", Tokens: 128_000},
	{Family: "gpt-5.2-chat", Tokens: 128_000},
	{Family: "gpt-5.2-pro", Tokens: 400_000},
	{Family: "gpt-5.2", Tokens: 400_000},
	{Family: "gpt-5.1-chat-latest", Tokens: 128_000},
	{Family: "gpt-5.1-chat", Tokens: 128_000},
	{Family: "gpt-5.1-codex-max", Tokens: 400_000},
	{Family: "gpt-5.1-codex-mini", Tokens: 400_000},
	{Family: "gpt-5.1-codex", Tokens: 400_000},
	{Family: "gpt-5.1", Tokens: 400_000},
	{Family: "gpt-5-chat-latest", Tokens: 128_000},
	{Family: "gpt-5-chat", Tokens: 128_000},
	{Family: "gpt-5-codex", Tokens: 400_000},
	{Family: "gpt-5-pro", Tokens: 400_000},
	{Family: "gpt-5-mini", Tokens: 400_000},
	{Family: "gpt-5-nano", Tokens: 400_000},
	{Family: "gpt-5", Tokens: 400_000},
	{Family: "gpt-4.1-mini", Tokens: 1_047_576},
	{Family: "gpt-4.1-nano", Tokens: 1_047_576},
	{Family: "gpt-4.1", Tokens: 1_047_576},
	{Family: "gpt-4o-mini", Tokens: 128_000},
	{Family: "gpt-4o", Tokens: 128_000},
	{Family: "gpt-oss-120b", Tokens: 131_072, MaxOutputTokens: 131_072},
	{Family: "gpt-oss-20b", Tokens: 131_072, MaxOutputTokens: 131_072},
	{Family: "o4-mini", Tokens: 200_000},
	{Family: "o3-pro", Tokens: 200_000},
	{Family: "o3-mini", Tokens: 200_000},
	{Family: "o3", Tokens: 200_000},
	{Family: "o1-pro", Tokens: 200_000},
	{Family: "o1-mini", Tokens: 128_000},
	{Family: "o1-preview", Tokens: 128_000},
	{Family: "o1", Tokens: 200_000},

	// Anthropic。新一代模型默认提供 1M 窗口，旧型号保持各自的标准窗口。
	{Family: "claude-mythos-5", Tokens: 1_000_000},
	{Family: "claude-fable-5", Tokens: 1_000_000, MaxOutputTokens: 128_000},
	{Family: "claude-opus-5", Tokens: 1_000_000, MaxOutputTokens: 128_000},
	{Family: "claude-sonnet-5", Tokens: 1_000_000, MaxOutputTokens: 128_000},
	{Family: "claude-opus-4-8", Tokens: 1_000_000},
	{Family: "claude-opus-4-7", Tokens: 1_000_000},
	{Family: "claude-opus-4-6", Tokens: 1_000_000},
	{Family: "claude-sonnet-4-6", Tokens: 1_000_000},
	{Family: "claude-opus-4-5", Tokens: 200_000},
	{Family: "claude-sonnet-4-5", Tokens: 200_000},
	{Family: "claude-haiku-4-5", Tokens: 200_000, MaxOutputTokens: 64_000},
	{Family: "claude-opus-4-1", Tokens: 200_000},
	{Family: "claude-opus-4", Tokens: 200_000},
	{Family: "claude-sonnet-4", Tokens: 200_000},
	{Family: "claude-3-7-sonnet", Tokens: 200_000},
	{Family: "claude-3-5-sonnet", Tokens: 200_000},
	{Family: "claude-3-5-haiku", Tokens: 200_000},
	{Family: "claude-3-opus", Tokens: 200_000},

	// Google Gemini。
	{Family: "gemini-3.7-flash", Tokens: 1_048_576, MaxOutputTokens: 65_536},
	{Family: "gemini-3.5-flash-lite", Tokens: 1_048_576, MaxOutputTokens: 65_536},
	{Family: "gemini-3.5-flash", Tokens: 1_048_576, MaxOutputTokens: 65_536},
	{Family: "gemini-3.1-flash-lite", Tokens: 1_048_576, MaxOutputTokens: 65_536},
	{Family: "gemini-3.1-pro-preview", Tokens: 1_048_576},
	{Family: "gemini-3.1-flash-lite-preview", Tokens: 1_048_576},
	{Family: "gemini-3-flash-preview", Tokens: 1_048_576},
	{Family: "gemini-2.5-pro", Tokens: 1_048_576},
	{Family: "gemini-2.5-flash", Tokens: 1_048_576},
	{Family: "gemini-2.5-flash-lite", Tokens: 1_048_576},

	// xAI。
	{Family: "grok-4.6", Tokens: 500_000},

	// Mistral 开放权重模型。
	{Family: "mistral-medium-3-5", Tokens: 262_144},
	{Family: "mistral-small-2603", Tokens: 262_144},
	{Family: "mistral-large-2512", Tokens: 262_144},
	{Family: "ministral-14b-2512", Tokens: 262_144},
	{Family: "ministral-8b-2512", Tokens: 262_144},
	{Family: "ministral-3b-2512", Tokens: 262_144},

	// Meta Llama 开放权重模型。
	{Family: "llama-4-scout", Tokens: 10_000_000},
	{Family: "llama-4-maverick", Tokens: 1_000_000},

	// Cohere。
	{Family: "command-a-plus-05-2026", Tokens: 128_000, MaxOutputTokens: 64_000},

	// Amazon Nova。Bedrock 的区域前缀属于模型 ID 的一部分。
	{Family: "amazon.nova-2-lite-v1:0", Tokens: 1_000_000, MaxOutputTokens: 65_536},
	{Family: "global.amazon.nova-2-lite-v1:0", Tokens: 1_000_000, MaxOutputTokens: 65_536},
	{Family: "us.amazon.nova-2-lite-v1:0", Tokens: 1_000_000, MaxOutputTokens: 65_536},
	{Family: "eu.amazon.nova-2-lite-v1:0", Tokens: 1_000_000, MaxOutputTokens: 65_536},
	{Family: "jp.amazon.nova-2-lite-v1:0", Tokens: 1_000_000, MaxOutputTokens: 65_536},

	// DeepSeek。
	{Family: "deepseek-v4-pro", Tokens: 1_000_000, MaxOutputTokens: 384_000},
	{Family: "deepseek-v4-flash", Tokens: 1_000_000, MaxOutputTokens: 384_000},
	{Family: "deepseek-chat", Tokens: 1_000_000},
	{Family: "deepseek-reasoner", Tokens: 1_000_000},

	// 智谱 GLM。Coding Plan 以接口精确 token 上限为准，而不是文档中的 200K 简写。
	{Family: "glm-5.3-flash", Tokens: 1_000_000, MaxOutputTokens: 131_072},
	{Family: "glm-5.3", Tokens: 1_000_000, MaxOutputTokens: 131_072},
	{Family: "glm-5.2", Tokens: 1_000_000},
	{Family: "glm-5.1", Tokens: 202_752},
	{Family: "glm-5-turbo", Tokens: 202_752},
	{Family: "glm-5", Tokens: 202_752},
	{Family: "glm-4-7", Tokens: 202_752},
	{Family: "glm-4.7", Tokens: 202_752},
	{Family: "glm-4-6", Tokens: 202_752},
	{Family: "glm-4.6", Tokens: 202_752},
	{Family: "glm-4.5-air", Tokens: 131_072},
	{Family: "glm-4.5", Tokens: 131_072},
	{Family: "glm-4-long", Tokens: 1_000_000},

	// Moonshot Kimi。
	{Family: "kimi-k3", Tokens: 1_048_576},
	{Family: "kimi-for-coding", Tokens: 262_144},
	{Family: "kimi-k2.7-code-highspeed", Tokens: 262_144},
	{Family: "kimi-k2.7-code", Tokens: 262_144},
	{Family: "kimi-k2.6", Tokens: 262_144},
	{Family: "kimi-k2.5", Tokens: 262_144},
	{Family: "kimi-k2-thinking", Tokens: 262_144},
	{Family: "kimi-k2", Tokens: 131_072},

	// 阿里云百炼与 Coding Plan。
	{Family: "qwen3.8-max", Tokens: 1_000_000, MaxOutputTokens: 131_072},
	{Family: "qwen3.7-flash", Tokens: 1_000_000},
	{Family: "qwen3.7-max", Tokens: 1_000_000},
	{Family: "qwen3.7-plus", Tokens: 1_000_000},
	{Family: "qwen3.6-plus", Tokens: 1_000_000},
	{Family: "qwen3.6-flash", Tokens: 1_000_000},
	{Family: "qwen3.6-max-preview", Tokens: 262_144},
	{Family: "qwen3.5-plus", Tokens: 1_000_000},
	{Family: "qwen3-max-preview", Tokens: 262_144},
	{Family: "qwen3-max", Tokens: 262_144},
	{Family: "qwen3-coder-plus", Tokens: 1_000_000},
	{Family: "qwen3-coder-next", Tokens: 262_144},
	{Family: "qwen-plus", Tokens: 1_000_000},
	{Family: "qwen-flash", Tokens: 1_000_000},
	{Family: "minimax-m3", Tokens: 196_608},
	{Family: "minimax-m2.7", Tokens: 196_608},
	{Family: "minimax-m2.5", Tokens: 196_608},
	{Family: "minimax-m2.1", Tokens: 196_608},
	{Family: "mimo-v2.5-pro", Tokens: 1_000_000},

	// 百度文心。
	{Family: "ernie-5.1", Tokens: 131_072, MaxOutputTokens: 65_536},
	{Family: "ernie-5.0", Tokens: 131_072, MaxOutputTokens: 65_536},

	// 腾讯混元与阶跃星辰。
	{Family: "hy3", Tokens: 262_144},
	{Family: "step-3.5-flash", Tokens: 262_144},
}

// knownVisionCapability 只补齐已经稳定支持图片输入的常见模型族。
//
// 返回 nil 表示未知，而不是不支持。远端模型卡和用户覆盖值始终优先。
func knownVisionCapability(modelID string) *bool {
	normalized := normalizeCatalogModelID(modelID)
	switch {
	case modelIDMatchesGeneration(normalized, "gpt-5"),
		modelIDMatchesGeneration(normalized, "gpt-4.1"),
		modelIDMatchesGeneration(normalized, "gpt-4o"),
		modelIDMatchesGeneration(normalized, "claude-3"),
		modelIDMatchesGeneration(normalized, "claude-opus-4"),
		modelIDMatchesGeneration(normalized, "claude-sonnet-4"),
		modelIDMatchesGeneration(normalized, "claude-haiku-4"),
		modelIDMatchesGeneration(normalized, "claude-mythos-5"),
		modelIDMatchesGeneration(normalized, "claude-fable-5"),
		modelIDMatchesGeneration(normalized, "claude-opus-5"),
		modelIDMatchesGeneration(normalized, "claude-sonnet-5"),
		modelIDMatchesGeneration(normalized, "gemini"),
		modelIDMatchesGeneration(normalized, "grok-4.6"),
		modelIDMatchesGeneration(normalized, "kimi-for-coding"),
		modelIDMatchesGeneration(normalized, "kimi-k3"),
		modelIDMatchesGeneration(normalized, "kimi-k2.6"),
		modelIDMatchesGeneration(normalized, "qwen3.8-max"),
		modelIDMatchesGeneration(normalized, "qwen3.7-plus"),
		modelIDMatchesGeneration(normalized, "qwen3.7-flash"),
		modelIDMatchesGeneration(normalized, "minimax-m3"),
		modelIDMatchesGeneration(normalized, "doubao-seed-2-0"),
		modelIDMatchesGeneration(normalized, "ernie-5.0"),
		modelIDMatchesGeneration(normalized, "step-3.7-flash"),
		modelIDMatchesGeneration(normalized, "mistral-medium-3-5"),
		modelIDMatchesGeneration(normalized, "mistral-small-2603"),
		modelIDMatchesGeneration(normalized, "mistral-large-2512"),
		modelIDMatchesGeneration(normalized, "ministral-14b-2512"),
		modelIDMatchesGeneration(normalized, "ministral-8b-2512"),
		modelIDMatchesGeneration(normalized, "ministral-3b-2512"),
		modelIDMatchesGeneration(normalized, "llama-4-scout"),
		modelIDMatchesGeneration(normalized, "llama-4-maverick"),
		modelIDMatchesGeneration(normalized, "command-a-plus"),
		strings.Contains(normalized, "amazon.nova-2-lite"):
		return boolPointer(true)
	case strings.Contains(normalized, "qwen") && strings.Contains(normalized, "vl"):
		return boolPointer(true)
	case modelIDMatchesGeneration(normalized, "glm-5.3-flash"),
		modelIDMatchesGeneration(normalized, "glm-4v"),
		modelIDMatchesGeneration(normalized, "pixtral"),
		modelIDMatchesGeneration(normalized, "llava"):
		return boolPointer(true)
	default:
		return nil
	}
}

// knownReasoningCapability 只补齐官方明确标记为推理模型的模型族。
func knownReasoningCapability(modelID string) *bool {
	normalized := normalizeCatalogModelID(modelID)
	switch {
	case modelIDMatchesGeneration(normalized, "gpt-5.6"),
		modelIDMatchesGeneration(normalized, "gpt-oss"),
		modelIDMatchesGeneration(normalized, "claude-fable-5"),
		modelIDMatchesGeneration(normalized, "claude-opus-5"),
		modelIDMatchesGeneration(normalized, "claude-sonnet-5"),
		modelIDMatchesGeneration(normalized, "gemini-3.7"),
		modelIDMatchesGeneration(normalized, "gemini-3.5"),
		modelIDMatchesGeneration(normalized, "gemini-3.1-flash-lite"),
		modelIDMatchesGeneration(normalized, "grok-4.6"),
		modelIDMatchesGeneration(normalized, "glm-5.3"),
		modelIDMatchesGeneration(normalized, "kimi-k3"),
		modelIDMatchesGeneration(normalized, "kimi-k2.6"),
		modelIDMatchesGeneration(normalized, "qwen3.8"),
		modelIDMatchesGeneration(normalized, "qwen3.7"),
		modelIDMatchesGeneration(normalized, "minimax-m3"),
		modelIDMatchesGeneration(normalized, "doubao-seed-2-0"),
		modelIDMatchesGeneration(normalized, "ernie-5.0"),
		modelIDMatchesGeneration(normalized, "hy3"),
		modelIDMatchesGeneration(normalized, "step-3.7-flash"),
		modelIDMatchesGeneration(normalized, "step-3.5-flash"),
		modelIDMatchesGeneration(normalized, "mistral-small-2603"),
		modelIDMatchesGeneration(normalized, "command-a-plus"):
		return boolPointer(true)
	default:
		return nil
	}
}

func knownContextWindow(modelID string) *int {
	normalized := normalizeCatalogModelID(modelID)
	for _, item := range knownModelLimits {
		if modelIDMatchesFamily(normalized, item.Family) {
			value := item.Tokens
			return &value
		}
	}
	return nil
}

func knownMaxOutputTokens(modelID string) *int {
	normalized := normalizeCatalogModelID(modelID)
	for _, item := range knownModelLimits {
		if item.MaxOutputTokens > 0 && modelIDMatchesFamily(normalized, item.Family) {
			value := item.MaxOutputTokens
			return &value
		}
	}
	return nil
}

func contextWindowOrKnown(modelID string, contextWindow *int) *int {
	if contextWindow != nil {
		return contextWindow
	}
	return knownContextWindow(modelID)
}

func maxOutputTokensOrKnown(modelID string, maxOutputTokens *int) *int {
	if maxOutputTokens != nil {
		return maxOutputTokens
	}
	return knownMaxOutputTokens(modelID)
}

func contextWindowOrDefault(modelID string, contextWindow *int) *int {
	if value := contextWindowOrKnown(modelID, contextWindow); value != nil {
		return value
	}
	value := defaultModelContextWindow
	return &value
}

func maxOutputTokensOrDefault(modelID string, maxOutputTokens *int) *int {
	if value := maxOutputTokensOrKnown(modelID, maxOutputTokens); value != nil {
		return value
	}
	value := defaultModelMaxOutputTokens
	return &value
}

func normalizeCatalogModelID(modelID string) string {
	normalized := strings.ToLower(strings.TrimSpace(modelID))
	if decoded, err := url.PathUnescape(normalized); err == nil {
		normalized = decoded
	}
	normalized = strings.Trim(normalized, "/")
	if index := strings.LastIndex(normalized, "/"); index >= 0 {
		normalized = normalized[index+1:]
	}
	if index := strings.Index(normalized, "["); index >= 0 {
		normalized = strings.TrimSpace(normalized[:index])
	}
	return normalized
}

func modelIDMatchesFamily(modelID string, family string) bool {
	if modelID == family {
		return true
	}
	suffix, matched := strings.CutPrefix(modelID, family)
	if !matched {
		return false
	}
	if len(suffix) < 2 || suffix[0] != '-' {
		return false
	}
	version := suffix[1:]
	return version == "latest" || version[0] >= '0' && version[0] <= '9'
}

// modelIDMatchesGeneration 匹配一个稳定代际及其点版本、变体和日期快照。
func modelIDMatchesGeneration(modelID string, generation string) bool {
	if modelID == generation {
		return true
	}
	suffix, matched := strings.CutPrefix(modelID, generation)
	if !matched || suffix == "" {
		return false
	}
	return suffix[0] == '-' || suffix[0] == '.'
}
