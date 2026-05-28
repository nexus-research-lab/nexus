package provider

import (
	"strings"

	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
)

const (
	presetAnthropic     = "anthropic"
	presetOpenAI        = "openai"
	presetDeepSeek      = "deepseek"
	presetQwenTokenPlan = "qwen-token-plan"
	presetMiniMaxToken  = "minimax-token-plan"
	presetGLMCodingPlan = "glm-coding-plan"
	presetKimiCode      = "kimi-code"
	presetVolcengine    = "volcengine-coding-plan"
	presetCustom        = "custom"
)

var providerPresets = []Preset{
	{
		PresetKey:     presetAnthropic,
		DisplayName:   "Anthropic",
		Description:   "Claude models through Anthropic Messages API.",
		KeyURL:        "https://console.anthropic.com/settings/keys",
		DefaultFormat: APIFormatAnthropicMessages,
		Formats: []PresetFormat{
			{
				APIFormat:  APIFormatAnthropicMessages,
				BaseURL:    "https://api.anthropic.com",
				ModelsPath: "/v1/models",
			},
		},
	},
	{
		PresetKey:     presetOpenAI,
		DisplayName:   "OpenAI",
		Description:   "OpenAI models through Chat Completions or Responses API.",
		KeyURL:        "https://platform.openai.com/api-keys",
		DefaultFormat: APIFormatChatCompletions,
		Formats: []PresetFormat{
			{
				APIFormat:  APIFormatChatCompletions,
				BaseURL:    "https://api.openai.com/v1",
				ModelsPath: "/models",
			},
			{
				APIFormat:  APIFormatResponses,
				BaseURL:    "https://api.openai.com/v1",
				ModelsPath: "/models",
			},
		},
	},
	{
		PresetKey:     presetDeepSeek,
		DisplayName:   "DeepSeek",
		Description:   "DeepSeek models through Anthropic-compatible Messages API.",
		KeyURL:        "https://platform.deepseek.com/api_keys",
		DefaultFormat: APIFormatAnthropicMessages,
		Formats: []PresetFormat{
			{
				APIFormat:  APIFormatAnthropicMessages,
				BaseURL:    "https://api.deepseek.com/anthropic",
				ModelsPath: "https://api.deepseek.com/models",
			},
		},
	},
	{
		PresetKey:     presetQwenTokenPlan,
		DisplayName:   "Qwen Token Plan",
		Description:   "Alibaba Cloud Model Studio Token Plan for coding tools through Anthropic-compatible Messages API.",
		KeyURL:        "https://tokenplan-enterprise.bailian.console.aliyun.com/",
		DefaultFormat: APIFormatAnthropicMessages,
		Formats: []PresetFormat{
			{
				APIFormat:  APIFormatAnthropicMessages,
				BaseURL:    "https://token-plan.cn-beijing.maas.aliyuncs.com/apps/anthropic",
				ModelsPath: "https://token-plan.cn-beijing.maas.aliyuncs.com/compatible-mode/v1/models",
			},
			{
				APIFormat:  APIFormatChatCompletions,
				BaseURL:    "https://token-plan.cn-beijing.maas.aliyuncs.com/compatible-mode/v1",
				ModelsPath: "/models",
			},
		},
	},
	{
		PresetKey:     presetMiniMaxToken,
		DisplayName:   "MiniMax Token Plan",
		Description:   "MiniMax Token Plan for M2.7 coding models through Anthropic-compatible or OpenAI-compatible APIs.",
		KeyURL:        "https://platform.minimaxi.com/user-center/payment/token-plan",
		DefaultFormat: APIFormatAnthropicMessages,
		Formats: []PresetFormat{
			{
				APIFormat:  APIFormatAnthropicMessages,
				BaseURL:    "https://api.minimaxi.com/anthropic",
				ModelsPath: "https://api.minimaxi.com/v1/models",
			},
			{
				APIFormat:  APIFormatChatCompletions,
				BaseURL:    "https://api.minimaxi.com/v1",
				ModelsPath: "/models",
			},
		},
	},
	{
		PresetKey:     presetGLMCodingPlan,
		DisplayName:   "GLM Coding Plan",
		Description:   "GLM Coding Plan 是专为 AI 编码打造的订阅套餐，仅需少量投入，即可为您带来智能、高速、稳定的编码体验。",
		KeyURL:        "https://bigmodel.cn/usercenter/proj-mgmt/apikeys",
		DefaultFormat: APIFormatAnthropicMessages,
		Formats: []PresetFormat{
			{
				APIFormat:  APIFormatAnthropicMessages,
				BaseURL:    "https://open.bigmodel.cn/api/anthropic",
				ModelsPath: "https://open.bigmodel.cn/api/coding/paas/v4/models",
			},
			{
				APIFormat:  APIFormatChatCompletions,
				BaseURL:    "https://open.bigmodel.cn/api/coding/paas/v4",
				ModelsPath: "https://open.bigmodel.cn/api/coding/paas/v4/models",
			},
		},
	},
	{
		PresetKey:     presetKimiCode,
		DisplayName:   "Kimi Code",
		Description:   "Kimi Code 是 Kimi 会员权益中专为开发者提供的智能编程服务，基于 Kimi 最新旗舰模型，通过 CLI、VS Code 扩展插件等产品形态，为开发者提供代码阅读、文件编辑、命令执行等 AI 辅助能力。",
		KeyURL:        "https://www.kimi.com/code/console",
		DefaultFormat: APIFormatAnthropicMessages,
		Formats: []PresetFormat{
			{
				APIFormat:  APIFormatAnthropicMessages,
				BaseURL:    "https://api.kimi.com/coding/",
				ModelsPath: "https://api.kimi.com/coding/v1/models",
			},
			{
				APIFormat:  APIFormatChatCompletions,
				BaseURL:    "https://api.kimi.com/coding/v1",
				ModelsPath: "https://api.kimi.com/coding/v1/models",
			},
		},
	},
	{
		PresetKey:     presetVolcengine,
		DisplayName:   "Volcengine Coding Plan",
		Description:   "火山方舟 Coding Plan 面向 AI 编码场景，支持通过 Anthropic-compatible Messages API 接入 Claude Code 类工具。",
		KeyURL:        "https://console.volcengine.com/ark/region:ark+cn-beijing/apikey",
		DefaultFormat: APIFormatAnthropicMessages,
		Formats: []PresetFormat{
			{
				APIFormat:  APIFormatAnthropicMessages,
				BaseURL:    "https://ark.cn-beijing.volces.com/api/coding",
				ModelsPath: "https://ark.cn-beijing.volces.com/api/coding/v3/models",
			},
			{
				APIFormat:  APIFormatChatCompletions,
				BaseURL:    "https://ark.cn-beijing.volces.com/api/coding/v3",
				ModelsPath: "/models",
			},
		},
	},
	{
		PresetKey:     presetCustom,
		DisplayName:   "Custom Provider",
		Description:   "Custom model API provider using one of the supported API formats.",
		DefaultFormat: APIFormatChatCompletions,
		Formats: []PresetFormat{
			{APIFormat: APIFormatChatCompletions, ModelsPath: "/models"},
			{APIFormat: APIFormatResponses, ModelsPath: "/models"},
			{APIFormat: APIFormatAnthropicMessages, ModelsPath: "/v1/models"},
		},
	},
}

// ListPresets 返回内置 Provider 模板。
func (s *Service) ListPresets() []Preset {
	result := make([]Preset, 0, len(providerPresets))
	for _, preset := range providerPresets {
		result = append(result, clonePreset(preset))
	}
	return result
}

func resolvePreset(presetKey string) Preset {
	key := strings.TrimSpace(presetKey)
	for _, preset := range providerPresets {
		if preset.PresetKey == key {
			return clonePreset(preset)
		}
	}
	return clonePreset(providerPresets[len(providerPresets)-1])
}

func (p Preset) Format(apiFormat string) PresetFormat {
	format := normalizeAPIFormat(apiFormat)
	for _, item := range p.Formats {
		if item.APIFormat == format {
			return item
		}
	}
	if len(p.Formats) == 0 {
		return PresetFormat{APIFormat: format}
	}
	return p.Formats[0]
}

func clonePreset(preset Preset) Preset {
	preset.Formats = append([]PresetFormat(nil), preset.Formats...)
	return preset
}

func normalizeAPIFormat(apiFormat string) string {
	switch strings.TrimSpace(apiFormat) {
	case APIFormatChatCompletions, APIFormatResponses, APIFormatAnthropicMessages:
		return strings.TrimSpace(apiFormat)
	case "":
		return ""
	default:
		return APIFormatChatCompletions
	}
}

func isAgentRuntimeAPIFormat(apiFormat string) bool {
	return normalizeAPIFormat(apiFormat) == APIFormatAnthropicMessages
}

func isAgentRuntimeProvider(item providerstore.Entity) bool {
	return item.ProviderKind == ProviderKindLLM && isAgentRuntimeAPIFormat(item.APIFormat)
}
