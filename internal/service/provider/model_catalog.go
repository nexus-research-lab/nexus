package provider

import (
	"net/url"
	"strings"
)

// knownModelContext 描述官方文档明确给出的模型上下文窗口。
//
// 目录只负责补齐 Provider 模型列表缺失的字段。远端返回值和用户显式配置始终优先，
// 未列出的模型保持未知，避免通过相似名称猜测运行时限制。
type knownModelContext struct {
	Family string
	Tokens int
}

var knownModelContexts = []knownModelContext{
	// OpenAI。
	{Family: "gpt-5.6-terra", Tokens: 1_050_000},
	{Family: "gpt-5.6-luna", Tokens: 1_050_000},
	{Family: "gpt-5.6-sol", Tokens: 1_050_000},
	{Family: "gpt-5.6", Tokens: 1_050_000},
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
	{Family: "claude-fable-5", Tokens: 1_000_000},
	{Family: "claude-sonnet-5", Tokens: 1_000_000},
	{Family: "claude-opus-4-8", Tokens: 1_000_000},
	{Family: "claude-opus-4-7", Tokens: 1_000_000},
	{Family: "claude-opus-4-6", Tokens: 1_000_000},
	{Family: "claude-sonnet-4-6", Tokens: 1_000_000},
	{Family: "claude-opus-4-5", Tokens: 200_000},
	{Family: "claude-sonnet-4-5", Tokens: 200_000},
	{Family: "claude-haiku-4-5", Tokens: 200_000},
	{Family: "claude-opus-4-1", Tokens: 200_000},
	{Family: "claude-opus-4", Tokens: 200_000},
	{Family: "claude-sonnet-4", Tokens: 200_000},
	{Family: "claude-3-7-sonnet", Tokens: 200_000},
	{Family: "claude-3-5-sonnet", Tokens: 200_000},
	{Family: "claude-3-5-haiku", Tokens: 200_000},
	{Family: "claude-3-opus", Tokens: 200_000},

	// Google Gemini。
	{Family: "gemini-3.1-pro-preview", Tokens: 1_048_576},
	{Family: "gemini-3.1-flash-lite-preview", Tokens: 1_048_576},
	{Family: "gemini-3-flash-preview", Tokens: 1_048_576},
	{Family: "gemini-2.5-pro", Tokens: 1_048_576},
	{Family: "gemini-2.5-flash", Tokens: 1_048_576},
	{Family: "gemini-2.5-flash-lite", Tokens: 1_048_576},

	// DeepSeek。
	{Family: "deepseek-v4-pro", Tokens: 1_000_000},
	{Family: "deepseek-v4-flash", Tokens: 1_000_000},
	{Family: "deepseek-chat", Tokens: 1_000_000},
	{Family: "deepseek-reasoner", Tokens: 1_000_000},

	// 智谱 GLM。Coding Plan 以接口精确 token 上限为准，而不是文档中的 200K 简写。
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
	{Family: "kimi-for-coding", Tokens: 262_144},
	{Family: "kimi-k2.7-code-highspeed", Tokens: 262_144},
	{Family: "kimi-k2.7-code", Tokens: 262_144},
	{Family: "kimi-k2.6", Tokens: 262_144},
	{Family: "kimi-k2.5", Tokens: 262_144},
	{Family: "kimi-k2-thinking", Tokens: 262_144},
	{Family: "kimi-k2", Tokens: 131_072},

	// 阿里云百炼与 Coding Plan。
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
		modelIDMatchesGeneration(normalized, "claude-sonnet-5"),
		modelIDMatchesGeneration(normalized, "gemini"),
		modelIDMatchesGeneration(normalized, "kimi-for-coding"):
		return boolPointer(true)
	case strings.Contains(normalized, "qwen") && strings.Contains(normalized, "vl"):
		return boolPointer(true)
	case modelIDMatchesGeneration(normalized, "glm-4v"),
		modelIDMatchesGeneration(normalized, "pixtral"),
		modelIDMatchesGeneration(normalized, "llava"):
		return boolPointer(true)
	default:
		return nil
	}
}

func knownContextWindow(modelID string) *int {
	normalized := normalizeCatalogModelID(modelID)
	for _, item := range knownModelContexts {
		if modelIDMatchesFamily(normalized, item.Family) {
			value := item.Tokens
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

func normalizeCatalogModelID(modelID string) string {
	normalized := strings.ToLower(strings.TrimSpace(modelID))
	if decoded, err := url.PathUnescape(normalized); err == nil {
		normalized = decoded
	}
	normalized = strings.Trim(normalized, "/")
	if index := strings.LastIndex(normalized, "/"); index >= 0 {
		normalized = normalized[index+1:]
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
