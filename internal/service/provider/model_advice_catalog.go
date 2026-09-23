// INPUT: 精确模型 ID 与可选 Provider 预设，别名逐一列出。
// OUTPUT: 带版本的模型能力与 Provider 专属选型建议。
// POS: 人工核对的产品目录，不承诺价格、性能或账户权限。
package provider

import (
	"reflect"
	"strings"
)

const modelAdviceVersion = "2026-09-23.2"

type modelAdvice struct {
	Presets         []string
	IDs             []string
	Capabilities    ModelCapabilities
	Recommendations map[string]string
	Evidence        ModelAdviceEvidence
	TextOnly        bool
}

// 能力只匹配精确模型 ID，不推断模型族；资料与核对边界见 docs/testing/provider-model-evidence.md。
var modelAdviceCatalog = []modelAdvice{
	chatAdvice(presetOpenAI, []string{"gpt-6-astra"}, true, "flagship", "", openAIModels),
	chatAdvice(presetOpenAI, []string{"gpt-5.6-terra"}, true, "balanced", "", openAIModels),
	chatAdvice(presetOpenAI, []string{"gpt-5.6-luna"}, true, "", "", openAIModels),
	chatAdvice(presetAnthropic, []string{"claude-opus-5"}, true, "flagship", "", anthropicModels),
	chatAdvice(presetAnthropic, []string{"claude-sonnet-5"}, true, "balanced", "", anthropicModels),
	chatAdvice(presetAnthropic, []string{"claude-fable-5-1"}, true, "", "", anthropicModels),
	chatAdvice(presetAnthropic, []string{"claude-haiku-4-5", "claude-haiku-4-5-20251001"}, true, "", "", anthropicModels),
	chatAdvice(presetDeepSeek, []string{"deepseek-flash"}, true, "balanced", "", deepSeekModels),
	chatAdvice(presetDeepSeek, []string{"deepseek-v4-pro"}, false, "", "", deepSeekModels),
	chatAdvice(presetGLMCodingPlan, []string{"glm-5.3"}, false, "flagship", "coding_plan", glmModels, glmOverview),
	chatAdvice(presetGLMCodingPlan, []string{"glm-5.3-flash"}, true, "balanced", "coding_plan", glmModels, glmOverview),
	// FlashX 仅补模型能力，官方尚未将其加入 Coding Plan。
	{IDs: []string{"glm-5.3-flashx"}, Capabilities: ModelCapabilities{
		TextOutput: adviceBool(true), Vision: adviceBool(true), ImageOutput: adviceBool(false),
		ToolCalling: adviceBool(true), Reasoning: adviceBool(true),
	}, Evidence: ModelAdviceEvidence{ReviewedAt: "2026-09-23", URLs: []string{"https://docs.bigmodel.cn/cn/guide/models/vlm/glm-5.3-flash"}}},
	chatAdvice(presetGLMCodingPlan, []string{"glm-5.2", "glm-5.1"}, false, "", "glm_redirect", glmOverview),
	chatAdvice(presetKimiCode, []string{"kimi-for-coding"}, true, "balanced", "kimi_standard", kimiModels),
	chatAdvice(presetKimiCode, []string{"k3", "k3-256k"}, true, "flagship", "kimi_k3", kimiModels),
	chatAdvice(presetKimiCode, []string{"kimi-for-coding-highspeed"}, true, "", "kimi_highspeed", kimiModels),
	chatAdvice(presetMiniMaxToken, []string{"MiniMax-M3"}, true, "flagship", "minimax_plan", miniMaxModels, miniMaxPlan),
	chatAdvice(presetMiniMaxToken, []string{"MiniMax-M2.7", "MiniMax-M2.7-highspeed", "MiniMax-M2.5", "MiniMax-M2.5-highspeed", "MiniMax-M2.1", "MiniMax-M2.1-highspeed", "MiniMax-M2"}, false, "", "minimax_plan", miniMaxModels, miniMaxPlan),
	chatAdvice(presetDashScope, []string{"qwen3.8-max"}, true, "flagship", "region_access", dashScopeModels, "https://help.aliyun.com/zh/model-studio/text-generation-model"),
	chatAdvice(presetDashScope, []string{"qwen3.7-plus"}, true, "balanced", "region_access", dashScopeModels, "https://help.aliyun.com/zh/model-studio/text-generation-model"),
	{Presets: []string{presetDashScope}, IDs: []string{"qwen-vl-plus"}, Capabilities: ModelCapabilities{
		TextOutput: adviceBool(true), Vision: adviceBool(true), ImageOutput: adviceBool(false), ToolCalling: adviceBool(false),
	}, Evidence: ModelAdviceEvidence{ReviewedAt: "2026-09-23", URLs: []string{"https://help.aliyun.com/zh/model-studio/qwen-vl-plus"}}},
	// The Token Plan team endpoint is NOT the older coding.dashscope endpoint.
	chatAdvice(presetQwenTokenPlan, []string{"qwen3.8-max"}, true, "flagship", "qwen_team", qwenPlan),
	chatAdvice(presetQwenTokenPlan, []string{"qwen3.8-flash"}, true, "balanced", "qwen_team", qwenPlan),
	chatAdvice(presetQwenTokenPlan, []string{"qwen3.7-plus", "qwen3.6-plus", "qwen3.6-flash", "deepseek-v4.1-flash", "kimi-k2.7-code", "kimi-k2.6", "kimi-k2.5"}, true, "", "qwen_team", qwenPlan),
	chatAdvice(presetQwenTokenPlan, []string{"qwen3.7-max", "deepseek-v4-pro", "deepseek-v4-pro-0813", "deepseek-v4-flash", "deepseek-v4-flash-0731", "deepseek-v3.2", "glm-5.3", "glm-5.2", "glm-5.1", "glm-5", "MiniMax-M2.5"}, false, "", "qwen_team", qwenPlan),
	chatAdvice(presetVolcengine, []string{"glm-5.3-flash", "minimax-m3"}, true, "balanced", "coding_plan", volcPlan),
	chatAdvice(presetVolcengine, []string{"doubao-seed-2.1-turbo", "doubao-seed-2.0-lite", "kimi-k2.7-code"}, true, "", "coding_plan", volcPlan),
	chatAdvice(presetVolcengine, []string{"kimi-k3"}, true, "", "volc_k3", volcPlan),
	chatAdvice(presetVolcengine, []string{"glm-5.3", "glm-latest"}, false, "flagship", "volc_heavy", volcPlan, glmModels),
	chatAdvice(presetDoubao, []string{"doubao-seed-2-1-pro-260915"}, true, "flagship", "region_access", doubaoModels),
	chatAdvice(presetDoubao, []string{"doubao-seed-2-1-turbo-260628"}, true, "balanced", "region_access", doubaoModels),
	chatAdvice(presetDoubao, []string{"doubao-seed-evolving", "doubao-seed-2-1-pro-260628"}, true, "", "region_access", doubaoModels),
	imageAdvice(presetDoubao, []string{"doubao-seedream-5-0-pro-260628"}, true, "image_generation", doubaoModels),
	imageAdvice(presetDoubao, []string{"doubao-seedream-5-0-260128", "doubao-seedream-5-0-lite-260128", "doubao-seedream-4-5-251128", "doubao-seedream-4-0-250828"}, true, "", doubaoModels),
	// API examples establish capabilities, not a latest/best recommendation.
	chatAdvice(presetModelScope, []string{"Qwen/Qwen3.5-35B-A3B"}, true, "", "modelscope_dynamic", modelScopeDocs),
	imageAdvice(presetOpenAI, []string{"gpt-image-2.5-flare"}, true, "image_generation", openAIImages),
	imageAdvice(presetOpenAI, []string{"gpt-image-2.5-sunburst"}, true, "image_editing", openAIImages),
	imageAdvice(presetDashScope, []string{"wan2.7-image-pro"}, true, "image_generation", dashScopeImages),
	imageAdvice(presetModelScope, []string{"Qwen/Qwen-Image"}, false, "", modelScopeDocs),
	// Plan image entitlement is separate from the text endpoint. No implicit image route.
	imageAdvice(presetQwenTokenPlan, []string{"qwen-image-2.0", "qwen-image-2.0-pro", "qwen-image-3.0-pro", "wan2.7-image", "wan2.7-image-pro"}, false, "", qwenPlan),
}

const (
	doubaoModels    = "https://docs.volcengine.com/docs/82379/1330310?lang=zh"
	openAIModels    = "https://platform.openai.com/docs/models"
	openAIImages    = "https://platform.openai.com/docs/guides/image-generation"
	anthropicModels = "https://docs.anthropic.com/en/docs/about-claude/models/overview.md"
	deepSeekModels  = "https://api-docs.deepseek.com/quick_start/pricing"
	glmModels       = "https://docs.bigmodel.cn/cn/coding-plan/latest-model"
	glmOverview     = "https://docs.bigmodel.cn/cn/coding-plan/overview"
	kimiModels      = "https://www.kimi.com/code/docs/kimi-code/models.html"
	miniMaxModels   = "https://platform.minimaxi.com/docs/api-reference/text-anthropic-api"
	miniMaxPlan     = "https://platform.minimaxi.com/docs/token-plan/other-tools"
	dashScopeModels = "https://help.aliyun.com/zh/model-studio/getting-started/models"
	dashScopeImages = "https://help.aliyun.com/zh/model-studio/image-model"
	qwenPlan        = "https://help.aliyun.com/zh/model-studio/token-plan-team-overview"
	volcPlan        = "https://docs.volcengine.com/docs/82379/1925114?lang=zh"
	modelScopeDocs  = "https://modelscope.cn/docs/model-service/API-Inference/intro"
)

func chatAdvice(preset string, ids []string, vision bool, reason, notice string, urls ...string) modelAdvice {
	c := ModelCapabilities{TextOutput: adviceBool(true), Vision: adviceBool(vision), ImageOutput: adviceBool(false)}
	if preset != presetModelScope {
		c.Reasoning = adviceBool(true)
	}
	recommendations := map[string]string{}
	if reason != "" {
		recommendations[PurposeChat] = reason
		if vision {
			recommendations[PurposeVision] = reason
		}
	}
	return modelAdvice{Presets: []string{preset}, IDs: ids, Capabilities: c, Recommendations: recommendations,
		Evidence: ModelAdviceEvidence{ReviewedAt: "2026-09-17", URLs: urls, Notice: notice}, TextOnly: !vision}
}

func imageAdvice(preset string, ids []string, editing bool, reason string, urls ...string) modelAdvice {
	c := ModelCapabilities{TextOutput: adviceBool(false), ImageOutput: adviceBool(true)}
	if editing {
		c.ImageEditing = adviceBool(true)
	}
	recommendations := map[string]string{}
	if reason != "" {
		recommendations[PurposeImage] = reason
	}
	if editing && reason == "image_editing" {
		recommendations[PurposeEdit] = reason
	}
	return modelAdvice{Presets: []string{preset}, IDs: ids, Capabilities: c, Recommendations: recommendations,
		Evidence: ModelAdviceEvidence{ReviewedAt: "2026-09-17", URLs: urls}}
}

func lookupModelAdvice(preset, id string) *modelAdvice {
	for i := range modelAdviceCatalog {
		entry := &modelAdviceCatalog[i]
		for _, scope := range entry.Presets {
			if scope == preset && adviceMatchesID(entry, id) {
				return entry
			}
		}
	}
	return nil
}

func lookupModelAdviceByID(id string) *modelAdvice {
	var match *modelAdvice
	for i := range modelAdviceCatalog {
		entry := &modelAdviceCatalog[i]
		if !adviceMatchesID(entry, id) {
			continue
		}
		if match != nil && !reflect.DeepEqual(match.Capabilities, entry.Capabilities) {
			return nil
		}
		match = entry
	}
	return match
}

func adviceMatchesID(entry *modelAdvice, id string) bool {
	for _, alias := range entry.IDs {
		if strings.EqualFold(strings.TrimSpace(id), alias) {
			return true
		}
	}
	return false
}

func adviceBool(value bool) *bool { return &value }
