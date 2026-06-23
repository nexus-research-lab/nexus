package provider

import (
	"context"
	"strings"
	"testing"
)

func TestMaskTokenShowsPrefixAndSuffix(t *testing.T) {
	tests := []struct {
		name  string
		token string
		want  string
	}{
		{name: "empty", token: "", want: ""},
		{name: "short", token: "short-key", want: "*********"},
		{name: "long", token: "sk-1234567890abcdef", want: "sk-12************************bcdef"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := maskToken(tt.token); got != tt.want {
				t.Fatalf("maskToken()=%q, want=%q", got, tt.want)
			}
		})
	}
}

func TestProviderPresetDefaultsAndRuntimeGate(t *testing.T) {
	ctx := context.Background()
	service, _ := newTestService(t)

	openai, err := service.Create(ctx, CreateInput{
		Provider:  "openai",
		PresetKey: presetOpenAI,
		AuthToken: "openai-key",
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("创建 OpenAI provider 失败: %v", err)
	}
	if openai.APIFormat != APIFormatChatCompletions {
		t.Fatalf("OpenAI 默认 API format 不正确: got=%s", openai.APIFormat)
	}
	if openai.BaseURL != "https://api.openai.com/v1" || openai.ModelsPath != "/models" {
		t.Fatalf("OpenAI 预置 endpoint 不正确: %+v", openai)
	}
	if openai.AgentRuntimeSupported {
		t.Fatalf("chat_completions 暂不应成为 Agent runtime provider: %+v", openai)
	}
	if _, err = service.ResolveRuntimeConfig(ctx, "openai", "gpt-4o"); err == nil || !strings.Contains(err.Error(), "暂不可用于 Agent runtime") {
		t.Fatalf("OpenAI chat_completions 应被 Claude runtime 拒绝: %v", err)
	}
	if _, err = service.UpdateModel(ctx, "openai", "gpt-4o", UpdateModelInput{
		Enabled:   true,
		IsDefault: true,
	}); err != nil {
		t.Fatalf("启用 OpenAI 模型失败: %v", err)
	}
	llmConfig, err := service.ResolveLLMConfig(ctx, "openai", "gpt-4o")
	if err != nil {
		t.Fatalf("OpenAI chat_completions 应可用于后端 LLM 任务: %v", err)
	}
	if llmConfig.APIFormat != APIFormatChatCompletions || llmConfig.Model != "gpt-4o" {
		t.Fatalf("OpenAI LLM config 不正确: %+v", llmConfig)
	}
	nxsRuntimeConfig, err := service.ResolveRuntimeConfigForRuntime(ctx, "openai", "gpt-4o", "nxs")
	if err != nil {
		t.Fatalf("OpenAI chat_completions 应可用于 nxs runtime: %v", err)
	}
	if nxsRuntimeConfig.APIFormat != APIFormatChatCompletions || nxsRuntimeConfig.Model != "gpt-4o" {
		t.Fatalf("OpenAI nxs runtime config 不正确: %+v", nxsRuntimeConfig)
	}
	openAINXSOptions, err := service.ListOptionsForRuntime(ctx, "nxs")
	if err != nil {
		t.Fatalf("读取 OpenAI nxs provider options 失败: %v", err)
	}
	if openAINXSOptions.DefaultProvider == nil || *openAINXSOptions.DefaultProvider != "openai" ||
		openAINXSOptions.DefaultModel == nil || *openAINXSOptions.DefaultModel != "gpt-4o" {
		t.Fatalf("OpenAI 应可成为 nxs 默认模型: %+v", openAINXSOptions)
	}
	openAINXSOption := optionByProvider(openAINXSOptions.Items, "openai")
	if openAINXSOption == nil || !hasModelOption(openAINXSOption.Models, "gpt-4o") {
		t.Fatalf("OpenAI nxs 默认模型下拉缺少已启用模型: %+v", openAINXSOptions.Items)
	}

	deepseek, err := service.Create(ctx, CreateInput{
		Provider:  "deepseek",
		PresetKey: presetDeepSeek,
		AuthToken: "deepseek-key",
	})
	if err != nil {
		t.Fatalf("创建 DeepSeek provider 失败: %v", err)
	}
	if deepseek.APIFormat != APIFormatAnthropicMessages ||
		deepseek.BaseURL != "https://api.deepseek.com/anthropic" ||
		deepseek.ModelsPath != "https://api.deepseek.com/models" {
		t.Fatalf("DeepSeek 默认配置不正确: %+v", deepseek)
	}
	if !deepseek.AgentRuntimeSupported {
		t.Fatalf("DeepSeek Anthropic format 应可用于 Agent runtime: %+v", deepseek)
	}

	qwenTokenPlan, err := service.Create(ctx, CreateInput{
		Provider:  "qwen-token-plan",
		PresetKey: presetQwenTokenPlan,
		AuthToken: "qwen-token-plan-key",
	})
	if err != nil {
		t.Fatalf("创建 Qwen Token Plan provider 失败: %v", err)
	}
	if qwenTokenPlan.APIFormat != APIFormatAnthropicMessages ||
		qwenTokenPlan.BaseURL != "https://token-plan.cn-beijing.maas.aliyuncs.com/apps/anthropic" ||
		qwenTokenPlan.ModelsPath != "https://token-plan.cn-beijing.maas.aliyuncs.com/compatible-mode/v1/models" {
		t.Fatalf("Qwen Token Plan 默认配置不正确: %+v", qwenTokenPlan)
	}
	if !qwenTokenPlan.AgentRuntimeSupported {
		t.Fatalf("Qwen Token Plan Anthropic format 应可用于 Agent runtime: %+v", qwenTokenPlan)
	}
	qwenPreset := resolvePreset(presetQwenTokenPlan)
	if format := qwenPreset.Format(APIFormatChatCompletions); format.BaseURL != "https://token-plan.cn-beijing.maas.aliyuncs.com/compatible-mode/v1" ||
		format.ModelsPath != "/models" {
		t.Fatalf("Qwen Token Plan OpenAI 兼容 endpoint 不正确: %+v", format)
	}

	miniMaxTokenPlan, err := service.Create(ctx, CreateInput{
		Provider:  "minimax-token-plan",
		PresetKey: presetMiniMaxToken,
		AuthToken: "minimax-token-plan-key",
	})
	if err != nil {
		t.Fatalf("创建 MiniMax Token Plan provider 失败: %v", err)
	}
	if miniMaxTokenPlan.APIFormat != APIFormatAnthropicMessages ||
		miniMaxTokenPlan.BaseURL != "https://api.minimaxi.com/anthropic" ||
		miniMaxTokenPlan.ModelsPath != "https://api.minimaxi.com/v1/models" {
		t.Fatalf("MiniMax Token Plan 默认配置不正确: %+v", miniMaxTokenPlan)
	}
	if !miniMaxTokenPlan.AgentRuntimeSupported {
		t.Fatalf("MiniMax Token Plan Anthropic format 应可用于 Agent runtime: %+v", miniMaxTokenPlan)
	}
	miniMaxPreset := resolvePreset(presetMiniMaxToken)
	if format := miniMaxPreset.Format(APIFormatChatCompletions); format.BaseURL != "https://api.minimaxi.com/v1" ||
		format.ModelsPath != "/models" {
		t.Fatalf("MiniMax Token Plan OpenAI 兼容 endpoint 不正确: %+v", format)
	}

	kimi, err := service.Create(ctx, CreateInput{
		Provider:  "kimi-code",
		PresetKey: presetKimiCode,
		AuthToken: "kimi-key",
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("创建 Kimi Code provider 失败: %v", err)
	}
	if kimi.APIFormat != APIFormatAnthropicMessages {
		t.Fatalf("Kimi Code 默认配置不正确: %+v", kimi)
	}
	if _, err = service.ResolveRuntimeConfig(ctx, "kimi-code", ""); err == nil || !strings.Contains(err.Error(), "model") {
		t.Fatalf("未设置模型的 Kimi Code 应被 Agent runtime 拒绝: %v", err)
	}
	if _, err = service.UpdateModel(ctx, "kimi-code", "kimi-for-coding", UpdateModelInput{
		Enabled:   true,
		IsDefault: true,
		CapabilitiesOverride: ModelCapabilities{
			Reasoning: boolPointer(true),
		},
	}); err != nil {
		t.Fatalf("设置 Kimi Code 默认模型失败: %v", err)
	}
	runtimeConfig, err := service.ResolveRuntimeConfig(ctx, "kimi-code", "kimi-for-coding")
	if err != nil {
		t.Fatalf("Kimi Code 应可用于 Agent runtime: %v", err)
	}
	if runtimeConfig.Model != "kimi-for-coding" {
		t.Fatalf("runtime model 未透传显式配置: %+v", runtimeConfig)
	}
	if runtimeConfig.APIFormat != APIFormatAnthropicMessages {
		t.Fatalf("runtime api_format 未透传: %+v", runtimeConfig)
	}
	if !runtimeConfig.Reasoning {
		t.Fatalf("runtime reasoning 能力未透传: %+v", runtimeConfig)
	}
	runtimeConfig, err = service.ResolveRuntimeConfig(ctx, "kimi-code", "")
	if err != nil {
		t.Fatalf("Kimi Code 显式 provider 应回退到默认模型: %v", err)
	}
	if runtimeConfig.Model != "kimi-for-coding" {
		t.Fatalf("runtime model 未回退到 provider 默认模型: %+v", runtimeConfig)
	}
	if !runtimeConfig.Reasoning {
		t.Fatalf("runtime 默认模型 reasoning 能力未透传: %+v", runtimeConfig)
	}

	volcengine, err := service.Create(ctx, CreateInput{
		Provider:  "volcengine-coding-plan",
		PresetKey: presetVolcengine,
		AuthToken: "volcengine-key",
	})
	if err != nil {
		t.Fatalf("创建 Volcengine Coding Plan provider 失败: %v", err)
	}
	if volcengine.APIFormat != APIFormatAnthropicMessages ||
		volcengine.BaseURL != "https://ark.cn-beijing.volces.com/api/coding" ||
		volcengine.ModelsPath != "https://ark.cn-beijing.volces.com/api/coding/v3/models" {
		t.Fatalf("Volcengine Coding Plan 默认配置不正确: %+v", volcengine)
	}
	if !volcengine.AgentRuntimeSupported {
		t.Fatalf("Volcengine Coding Plan Anthropic format 应可用于 Agent runtime: %+v", volcengine)
	}
	volcenginePreset := resolvePreset(presetVolcengine)
	if format := volcenginePreset.Format(APIFormatChatCompletions); format.BaseURL != "https://ark.cn-beijing.volces.com/api/coding/v3" ||
		format.ModelsPath != "/models" {
		t.Fatalf("Volcengine Coding Plan OpenAI 兼容 endpoint 不正确: %+v", format)
	}
	volcengineCompletions, err := service.Create(ctx, CreateInput{
		Provider:     "volcengine-completions",
		PresetKey:    presetVolcengine,
		ProviderKind: ProviderKindImageGeneration,
		APIFormat:    APIFormatChatCompletions,
		AuthToken:    "volcengine-key",
	})
	if err != nil {
		t.Fatalf("创建 Volcengine Completions 分支失败: %v", err)
	}
	if volcengineCompletions.ProviderKind != ProviderKindLLM ||
		volcengineCompletions.BaseURL != "https://ark.cn-beijing.volces.com/api/coding/v3" {
		t.Fatalf("Volcengine 内置 format 应忽略错误 provider_kind: %+v", volcengineCompletions)
	}

	doubao, err := service.Create(ctx, CreateInput{
		Provider:  "doubao",
		PresetKey: presetDoubao,
		AuthToken: "volcengine-key",
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("创建 Doubao provider 失败: %v", err)
	}
	if doubao.ProviderKind != ProviderKindLLM ||
		doubao.APIFormat != APIFormatChatCompletions ||
		doubao.BaseURL != "https://ark.cn-beijing.volces.com/api/v3" ||
		doubao.ModelsPath != "/models" ||
		doubao.DisplayName != "Doubao" {
		t.Fatalf("Doubao 默认配置不正确: %+v", doubao)
	}
	if doubao.AgentRuntimeSupported {
		t.Fatalf("Doubao Chat Completions 不应成为 Agent runtime provider: %+v", doubao)
	}
	doubaoPreset := resolvePreset(presetDoubao)
	if format := doubaoPreset.Format(APIFormatChatCompletions); format.ProviderKind != ProviderKindLLM ||
		format.BaseURL != "https://ark.cn-beijing.volces.com/api/v3" ||
		format.ModelsPath != "/models" {
		t.Fatalf("Doubao Chat Completions 分支配置不正确: %+v", format)
	}
	if format := doubaoPreset.Format(APIFormatResponses); format.ProviderKind != ProviderKindLLM ||
		format.BaseURL != "https://ark.cn-beijing.volces.com/api/v3" ||
		format.ModelsPath != "/models" {
		t.Fatalf("Doubao Responses 分支配置不正确: %+v", format)
	}
	if format := doubaoPreset.Format(APIFormatOpenAIImageGeneration); format.ProviderKind != ProviderKindImageGeneration ||
		format.BaseURL != "https://ark.cn-beijing.volces.com/api/v3" ||
		format.ModelsPath != "/models" {
		t.Fatalf("Doubao Seedream 生图分支配置不正确: %+v", format)
	}
	if _, err = service.UpdateModel(ctx, "doubao", "doubao-seedream-5-0-260128", UpdateModelInput{
		Enabled: true,
		CapabilitiesOverride: ModelCapabilities{
			ImageOutput: boolPointer(true),
		},
		IsDefault: true,
	}); err != nil {
		t.Fatalf("设置 Doubao Seedream 默认生图模型失败: %v", err)
	}
	if _, err = service.UpdateModel(ctx, "doubao", "doubao-1-5-pro-32k-250115", UpdateModelInput{
		Enabled: true,
	}); err != nil {
		t.Fatalf("启用 Doubao 文本模型失败: %v", err)
	}
	doubaoLLMConfig, err := service.ResolveLLMConfig(ctx, "doubao", "doubao-1-5-pro-32k-250115")
	if err != nil {
		t.Fatalf("Doubao 文本模型应可用于后端 LLM 任务: %v", err)
	}
	if doubaoLLMConfig.APIFormat != APIFormatChatCompletions ||
		doubaoLLMConfig.Model != "doubao-1-5-pro-32k-250115" {
		t.Fatalf("Doubao LLM config 不正确: %+v", doubaoLLMConfig)
	}
	imageConfig, err := service.ResolveImageModelConfig(ctx, "doubao", "doubao-seedream-5-0-260128")
	if err != nil {
		t.Fatalf("Doubao 应可解析 Seedream 生图配置: %v", err)
	}
	if imageConfig.APIFormat != APIFormatOpenAIImageGeneration ||
		imageConfig.BaseURL != "https://ark.cn-beijing.volces.com/api/v3" ||
		imageConfig.Model != "doubao-seedream-5-0-260128" {
		t.Fatalf("Doubao Seedream 生图配置不正确: %+v", imageConfig)
	}
	if _, err = service.ResolveImageModelConfig(ctx, "doubao", "doubao-1-5-pro-32k-250115"); err == nil ||
		!strings.Contains(err.Error(), "不是图片生成模型") {
		t.Fatalf("Doubao 文本模型不应被解析为生图模型: %v", err)
	}

	dashscope, err := service.Create(ctx, CreateInput{
		Provider:  "dashscope",
		PresetKey: presetDashScope,
		AuthToken: "dashscope-key",
	})
	if err != nil {
		t.Fatalf("创建 DashScope provider 失败: %v", err)
	}
	if dashscope.ProviderKind != ProviderKindLLM ||
		dashscope.APIFormat != APIFormatAnthropicMessages ||
		dashscope.BaseURL != "https://dashscope.aliyuncs.com/apps/anthropic" ||
		dashscope.DisplayName != "DashScope" ||
		dashscope.ModelsPath != "" {
		t.Fatalf("DashScope 默认配置不正确: %+v", dashscope)
	}
	if !dashscope.AgentRuntimeSupported {
		t.Fatalf("DashScope Anthropic 分支应可成为 Agent runtime provider: %+v", dashscope)
	}
	dashscopePreset := resolvePreset(presetDashScope)
	if format := dashscopePreset.Format(APIFormatDashScopeImageGeneration); format.ProviderKind != ProviderKindImageGeneration ||
		format.BaseURL != "https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation" {
		t.Fatalf("DashScope 生图分支配置不正确: %+v", format)
	}
	if format := dashscopePreset.Format(APIFormatResponses); format.ProviderKind != ProviderKindLLM ||
		format.BaseURL != "https://dashscope.aliyuncs.com/compatible-mode/v1" {
		t.Fatalf("DashScope Responses 分支配置不正确: %+v", format)
	}
	if format := dashscopePreset.Format(APIFormatChatCompletions); format.ProviderKind != ProviderKindLLM ||
		format.BaseURL != "https://dashscope.aliyuncs.com/compatible-mode/v1" {
		t.Fatalf("DashScope Chat Completions 分支配置不正确: %+v", format)
	}

	modelscope, err := service.Create(ctx, CreateInput{
		Provider:  "modelscope",
		PresetKey: presetModelScope,
		AuthToken: "modelscope-key",
	})
	if err != nil {
		t.Fatalf("创建 ModelScope provider 失败: %v", err)
	}
	if modelscope.ProviderKind != ProviderKindLLM ||
		modelscope.APIFormat != APIFormatChatCompletions ||
		modelscope.BaseURL != "https://api-inference.modelscope.cn/v1" ||
		modelscope.DisplayName != "ModelScope" ||
		modelscope.ModelsPath != "" {
		t.Fatalf("ModelScope 默认配置不正确: %+v", modelscope)
	}
	if modelscope.AgentRuntimeSupported {
		t.Fatalf("ModelScope Chat Completions 分支不应成为 Agent runtime provider: %+v", modelscope)
	}
	modelscopePreset := resolvePreset(presetModelScope)
	if format := modelscopePreset.Format(APIFormatModelScopeImageGeneration); format.ProviderKind != ProviderKindImageGeneration ||
		format.BaseURL != "https://api-inference.modelscope.cn/v1" {
		t.Fatalf("ModelScope 生图分支配置不正确: %+v", format)
	}

	options, err := service.ListOptions(ctx)
	if err != nil {
		t.Fatalf("读取 provider options 失败: %v", err)
	}
	if hasOptionProvider(options.Items, "openai") {
		t.Fatalf("OpenAI 不应出现在默认对话模型选项: %+v", options.Items)
	}
	if !hasOptionProvider(options.BackgroundItems, "openai") {
		t.Fatalf("OpenAI 应出现在后台任务模型选项: %+v", options.BackgroundItems)
	}
	doubaoBackground := optionByProvider(options.BackgroundItems, "doubao")
	if doubaoBackground == nil || hasModelOption(doubaoBackground.Models, "doubao-seedream-5-0-260128") ||
		!hasModelOption(doubaoBackground.Models, "doubao-1-5-pro-32k-250115") {
		t.Fatalf("Doubao 背景模型选项不正确: %+v", doubaoBackground)
	}
	doubaoImage := optionByProvider(options.ImageItems, "doubao")
	if doubaoImage == nil || !hasModelOption(doubaoImage.Models, "doubao-seedream-5-0-260128") ||
		hasModelOption(doubaoImage.Models, "doubao-1-5-pro-32k-250115") {
		t.Fatalf("Doubao 生图模型选项不正确: %+v", doubaoImage)
	}
	nxsOptions, err := service.ListOptionsForRuntime(ctx, "nxs")
	if err != nil {
		t.Fatalf("读取 nxs provider options 失败: %v", err)
	}
	if !hasOptionProvider(nxsOptions.Items, "openai") {
		t.Fatalf("OpenAI 应出现在 nxs 默认对话模型选项: %+v", nxsOptions.Items)
	}
	doubaoRuntime := optionByProvider(nxsOptions.Items, "doubao")
	if doubaoRuntime == nil || !hasModelOption(doubaoRuntime.Models, "doubao-1-5-pro-32k-250115") ||
		hasModelOption(doubaoRuntime.Models, "doubao-seedream-5-0-260128") {
		t.Fatalf("Doubao nxs runtime 模型选项不正确: %+v", doubaoRuntime)
	}
}

func TestBuiltinProviderEndpointUsesCatalog(t *testing.T) {
	ctx := context.Background()
	service, _ := newTestService(t)

	openai, err := service.Create(ctx, CreateInput{
		Provider:   "openai",
		PresetKey:  presetOpenAI,
		APIFormat:  APIFormatResponses,
		AuthToken:  "openai-key",
		BaseURL:    "https://proxy.example.com/v1",
		ModelsPath: "/proxy-models",
	})
	if err != nil {
		t.Fatalf("创建 OpenAI provider 失败: %v", err)
	}
	if openai.BaseURL != "https://api.openai.com/v1" || openai.ModelsPath != "/models" {
		t.Fatalf("内置 provider create 应忽略自定义 endpoint: %+v", openai)
	}

	updated, err := service.Update(ctx, "openai", UpdateInput{
		PresetKey:  presetOpenAI,
		APIFormat:  APIFormatResponses,
		BaseURL:    "https://another-proxy.example.com/v1",
		ModelsPath: "/another-models",
		Enabled:    true,
	})
	if err != nil {
		t.Fatalf("更新 OpenAI provider 失败: %v", err)
	}
	if updated.BaseURL != "https://api.openai.com/v1" || updated.ModelsPath != "/models" {
		t.Fatalf("内置 provider update 应忽略自定义 endpoint: %+v", updated)
	}

	entity, err := service.repository.GetVisibleByProvider(ctx, ownerUserIDFromContext(ctx), "openai")
	if err != nil || entity == nil {
		t.Fatalf("读取 OpenAI provider 失败: entity=%+v err=%v", entity, err)
	}
	entity.BaseURL = "https://dirty.example.com/v1"
	entity.ModelsPath = "/dirty-models"
	if err = service.repository.Update(ctx, *entity); err != nil {
		t.Fatalf("写入脏 endpoint 失败: %v", err)
	}
	records, err := service.List(ctx)
	if err != nil {
		t.Fatalf("读取 provider 列表失败: %v", err)
	}
	var listed *Record
	for index := range records {
		if records[index].Provider == "openai" {
			listed = &records[index]
			break
		}
	}
	if listed == nil || listed.BaseURL != "https://api.openai.com/v1" || listed.ModelsPath != "/models" {
		t.Fatalf("内置 provider list 应按 catalog 展示 endpoint: %+v", listed)
	}

	custom, err := service.Create(ctx, CreateInput{
		Provider:   "custom-openai",
		PresetKey:  presetCustom,
		APIFormat:  APIFormatChatCompletions,
		AuthToken:  "custom-key",
		BaseURL:    "https://proxy.example.com/v1",
		ModelsPath: "/proxy-models",
	})
	if err != nil {
		t.Fatalf("创建 custom provider 失败: %v", err)
	}
	if custom.BaseURL != "https://proxy.example.com/v1" || custom.ModelsPath != "/proxy-models" {
		t.Fatalf("custom provider 应保留自定义 endpoint: %+v", custom)
	}
}

func TestBuiltinMultiBranchProviderKindDerivedFromFormat(t *testing.T) {
	ctx := context.Background()
	service, _ := newTestService(t)

	dashscopeImage, err := service.Create(ctx, CreateInput{
		Provider:     "dashscope-image-branch",
		PresetKey:    presetDashScope,
		ProviderKind: ProviderKindImageGeneration,
		APIFormat:    APIFormatDashScopeImageGeneration,
		AuthToken:    "dashscope-key",
	})
	if err != nil {
		t.Fatalf("创建 DashScope 生图分支失败: %v", err)
	}
	if dashscopeImage.ProviderKind != ProviderKindImageGeneration ||
		dashscopeImage.APIFormat != APIFormatDashScopeImageGeneration ||
		dashscopeImage.BaseURL != "https://dashscope.aliyuncs.com/api/v1/services/aigc/multimodal-generation/generation" {
		t.Fatalf("DashScope 生图分支未按 format 解析: %+v", dashscopeImage)
	}

	modelscopeImage, err := service.Create(ctx, CreateInput{
		Provider:     "modelscope-image-branch",
		PresetKey:    presetModelScope,
		ProviderKind: ProviderKindImageGeneration,
		APIFormat:    APIFormatModelScopeImageGeneration,
		AuthToken:    "modelscope-key",
	})
	if err != nil {
		t.Fatalf("创建 ModelScope 生图分支失败: %v", err)
	}
	if modelscopeImage.ProviderKind != ProviderKindImageGeneration ||
		modelscopeImage.APIFormat != APIFormatModelScopeImageGeneration ||
		modelscopeImage.BaseURL != "https://api-inference.modelscope.cn/v1" {
		t.Fatalf("ModelScope 生图分支未按 format 解析: %+v", modelscopeImage)
	}

	dashscopeLLM, err := service.Create(ctx, CreateInput{
		Provider:     "dashscope-llm-branch",
		PresetKey:    presetDashScope,
		ProviderKind: ProviderKindImageGeneration,
		APIFormat:    APIFormatAnthropicMessages,
		AuthToken:    "dashscope-key",
	})
	if err != nil {
		t.Fatalf("DashScope LLM format 应按 format 推导 provider_kind: %v", err)
	}
	if dashscopeLLM.ProviderKind != ProviderKindLLM ||
		dashscopeLLM.APIFormat != APIFormatAnthropicMessages ||
		dashscopeLLM.BaseURL != "https://dashscope.aliyuncs.com/apps/anthropic" {
		t.Fatalf("DashScope LLM format 未按 format 推导 provider_kind: %+v", dashscopeLLM)
	}
}

func TestProviderImageOptionsIncludeDefaultModel(t *testing.T) {
	ctx := context.Background()
	service, _ := newTestService(t)
	imageProvider, err := service.Create(ctx, CreateInput{
		ProviderKind: ProviderKindImageGeneration,
		Provider:     "image-default",
		PresetKey:    presetCustom,
		APIFormat:    APIFormatChatCompletions,
		AuthToken:    "image-key",
		BaseURL:      "https://image.example.com/v1/images",
		ModelsPath:   "/models",
		Enabled:      true,
		DisplayName:  "Image Default",
	})
	if err != nil {
		t.Fatalf("创建生图 provider 失败: %v", err)
	}
	if _, err = service.UpdateModel(ctx, imageProvider.Provider, "image-model", UpdateModelInput{Enabled: true, IsDefault: true}); err != nil {
		t.Fatalf("设置生图默认模型失败: %v", err)
	}
	imageConfig, err := service.ResolveImageConfig(ctx, "")
	if err != nil {
		t.Fatalf("解析生图默认模型失败: %v", err)
	}
	if imageConfig.Provider != imageProvider.Provider || imageConfig.Model != "image-model" {
		t.Fatalf("生图默认模型不正确: %+v", imageConfig)
	}
	options, err := service.ListOptions(ctx)
	if err != nil {
		t.Fatalf("读取 provider options 失败: %v", err)
	}
	if options.DefaultImageProvider == nil || *options.DefaultImageProvider != imageProvider.Provider ||
		len(options.ImageItems) != 1 {
		t.Fatalf("生图默认模型未暴露到 options: %+v", options)
	}
}
