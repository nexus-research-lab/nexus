package provider

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
)

func newTestService(t *testing.T) (*Service, *sql.DB) {
	t.Helper()

	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)
	db := handlertest.OpenSQLite(t, cfg.DatabaseURL)
	t.Cleanup(func() { _ = db.Close() })
	return NewServiceWithDB(cfg, db), db
}

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
		t.Fatalf("OpenAI chat_completions 应被 Agent runtime 拒绝: %v", err)
	}
	if _, err = service.UpdateModel(ctx, "openai", "gpt-4o", UpdateModelInput{
		Enabled: true,
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

	entity, err := service.repository.GetByProvider(ctx, "openai")
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

func TestProviderListIncludesUsageAgents(t *testing.T) {
	ctx := context.Background()
	service, db := newTestService(t)
	record, err := service.Create(ctx, CreateInput{
		Provider:    "blocked",
		PresetKey:   presetCustom,
		APIFormat:   APIFormatAnthropicMessages,
		AuthToken:   "blocked-key",
		BaseURL:     "https://api.example.com",
		ModelsPath:  "/models",
		DisplayName: "Blocked",
	})
	if err != nil {
		t.Fatalf("创建 provider 失败: %v", err)
	}
	insertProviderUsageAgent(t, db, "agent-main", "main", "main", "主助手", true, record.Provider, "active")
	insertProviderUsageAgent(t, db, "agent-worker", "worker", "worker", "", false, record.Provider, "active")
	insertProviderUsageAgent(t, db, "agent-archived", "archived", "archived", "归档助手", false, record.Provider, "archived")

	records, err := service.List(ctx)
	if err != nil {
		t.Fatalf("读取 provider 列表失败: %v", err)
	}
	var target *Record
	for index := range records {
		if records[index].Provider == record.Provider {
			target = &records[index]
			break
		}
	}
	if target == nil {
		t.Fatalf("未找到 provider: %+v", records)
	}
	if target.UsageCount != 2 {
		t.Fatalf("usage_count 应只统计 active Agent: %+v", target)
	}
	if len(target.UsedByAgents) != 2 {
		t.Fatalf("used_by_agents 数量不正确: %+v", target.UsedByAgents)
	}
	if target.UsedByAgents[0].AgentID != "agent-main" || target.UsedByAgents[0].DisplayName != "主助手" || !target.UsedByAgents[0].IsMain {
		t.Fatalf("主 Agent 摘要不正确: %+v", target.UsedByAgents[0])
	}
	if target.UsedByAgents[1].AgentID != "agent-worker" || target.UsedByAgents[1].DisplayName != "worker" {
		t.Fatalf("普通 Agent 摘要不正确: %+v", target.UsedByAgents[1])
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

func TestForceDeleteProviderReassignsRuntimeProviders(t *testing.T) {
	ctx := context.Background()
	service, db := newTestService(t)
	fallback, err := service.Create(ctx, CreateInput{
		Provider:    "fallback-provider",
		PresetKey:   presetCustom,
		APIFormat:   APIFormatAnthropicMessages,
		AuthToken:   "fallback-key",
		BaseURL:     "https://api.fallback.example.com",
		ModelsPath:  "/models",
		Enabled:     true,
		DisplayName: "Fallback",
	})
	if err != nil {
		t.Fatalf("创建 fallback provider 失败: %v", err)
	}
	if _, err = service.UpdateModel(ctx, fallback.Provider, "fallback-model", UpdateModelInput{
		Enabled:   true,
		IsDefault: true,
	}); err != nil {
		t.Fatalf("设置 fallback 默认模型失败: %v", err)
	}
	target, err := service.Create(ctx, CreateInput{
		Provider:    "delete-target",
		PresetKey:   presetCustom,
		APIFormat:   APIFormatAnthropicMessages,
		AuthToken:   "target-key",
		BaseURL:     "https://api.target.example.com",
		ModelsPath:  "/models",
		Enabled:     true,
		DisplayName: "Target",
	})
	if err != nil {
		t.Fatalf("创建待删除 provider 失败: %v", err)
	}
	insertProviderUsageAgent(t, db, "agent-force-a", "force-a", "Force A", "", false, target.Provider, "active")
	insertProviderUsageAgent(t, db, "agent-force-b", "force-b", "Force B", "", false, target.Provider, "active")
	if _, err = service.Delete(ctx, target.Provider, DeleteInput{}); err == nil {
		t.Fatalf("普通删除应被正在使用的 provider 阻止")
	}
	result, err := service.Delete(ctx, target.Provider, DeleteInput{Force: true})
	if err != nil {
		t.Fatalf("强制删除 provider 失败: %v", err)
	}
	if result.ReplacementProvider != fallback.Provider || result.ReplacementModel != "fallback-model" || result.ReassignedRuntimeCount != 2 {
		t.Fatalf("强制删除结果不正确: %+v", result)
	}
	if _, err = service.Get(ctx, target.Provider); err == nil {
		t.Fatalf("待删除 provider 应已移除")
	}
	runtimes := runtimeSelectionsByAgent(t, db, "agent-force-a", "agent-force-b")
	if runtimes["agent-force-a"].provider != fallback.Provider ||
		runtimes["agent-force-a"].model != "fallback-model" ||
		runtimes["agent-force-b"].provider != fallback.Provider ||
		runtimes["agent-force-b"].model != "fallback-model" {
		t.Fatalf("runtime provider/model 未切换到默认模型: %+v", runtimes)
	}
	options, err := service.ListOptions(ctx)
	if err != nil {
		t.Fatalf("读取 provider options 失败: %v", err)
	}
	if options.DefaultProvider == nil || *options.DefaultProvider != fallback.Provider ||
		options.DefaultModel == nil || *options.DefaultModel != "fallback-model" {
		t.Fatalf("默认 provider 不正确: %+v", options)
	}
}

func TestFetchModelsMergesCardsAndPreservesOverride(t *testing.T) {
	ctx := context.Background()
	service, _ := newTestService(t)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/models" {
			t.Fatalf("模型列表路径不正确: %s", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer fetch-key" {
			t.Fatalf("Authorization header 未写入")
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"data":[{"id":"gpt-old","display_name":"GPT Old Updated","context_length":131072,"max_output_tokens":8192,"supports_reasoning":true,"supports_image_in":true,"supports_function_calling":true},{"id":"gpt-new"}]}`))
	}))
	defer server.Close()

	record, err := service.Create(ctx, CreateInput{
		Provider:    "fetcher",
		PresetKey:   presetCustom,
		APIFormat:   APIFormatChatCompletions,
		AuthToken:   "fetch-key",
		BaseURL:     server.URL,
		ModelsPath:  "/models",
		Enabled:     true,
		DisplayName: "Fetcher",
	})
	if err != nil {
		t.Fatalf("创建 provider 失败: %v", err)
	}
	if _, err = service.UpdateModel(ctx, record.Provider, "gpt-old", UpdateModelInput{
		Enabled: true,
		CapabilitiesOverride: ModelCapabilities{
			Vision: boolPointer(true),
		},
		ProviderOptions: map[string]any{"temperature": 0},
	}); err != nil {
		t.Fatalf("更新模型 override 失败: %v", err)
	}

	result, err := service.FetchModels(ctx, record.Provider)
	if err != nil {
		t.Fatalf("FetchModels 失败: %v", err)
	}
	if result.Count != 2 {
		t.Fatalf("模型数量不正确: %+v", result)
	}
	var oldModel *ModelRecord
	var newModel *ModelRecord
	for index := range result.Models {
		switch result.Models[index].ModelID {
		case "gpt-old":
			oldModel = &result.Models[index]
		case "gpt-new":
			newModel = &result.Models[index]
		}
	}
	if oldModel == nil || newModel == nil {
		t.Fatalf("模型合并结果不完整: %+v", result.Models)
	}
	if oldModel.DisplayName != "GPT Old Updated" {
		t.Fatalf("模型 display_name 未更新: %+v", oldModel)
	}
	if oldModel.CapabilitiesOverride.Vision == nil || !*oldModel.CapabilitiesOverride.Vision {
		t.Fatalf("用户能力覆盖不应被 fetch 覆盖: %+v", oldModel.CapabilitiesOverride)
	}
	if oldModel.ProviderOptions["temperature"] == nil {
		t.Fatalf("用户 provider options 不应被 fetch 覆盖: %+v", oldModel.ProviderOptions)
	}
	if oldModel.ContextWindow == nil || *oldModel.ContextWindow != 131072 {
		t.Fatalf("远端 context_length 未写入模型卡: %+v", oldModel)
	}
	if oldModel.MaxOutputTokens == nil || *oldModel.MaxOutputTokens != 8192 {
		t.Fatalf("远端 max_output_tokens 未写入模型卡: %+v", oldModel)
	}
	auto := oldModel.CapabilitiesAuto
	if auto.Reasoning == nil || !*auto.Reasoning ||
		auto.Vision == nil || !*auto.Vision ||
		auto.ToolCalling == nil || !*auto.ToolCalling {
		t.Fatalf("远端模型能力未写入 capabilities_auto: %+v", auto)
	}
}

func TestFetchModelsDoesNotInferModelCardsFromNames(t *testing.T) {
	ctx := context.Background()
	service, _ := newTestService(t)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/models" {
			t.Fatalf("模型列表路径不正确: %s", request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"data":[{"id":"text-embedding-3-small"},{"id":"gpt-image-1"},{"id":"kimi-for-coding"}]}`))
	}))
	defer server.Close()

	record, err := service.Create(ctx, CreateInput{
		Provider:   "no-infer",
		PresetKey:  presetCustom,
		APIFormat:  APIFormatChatCompletions,
		AuthToken:  "fetch-key",
		BaseURL:    server.URL,
		ModelsPath: "/models",
	})
	if err != nil {
		t.Fatalf("创建 provider 失败: %v", err)
	}
	result, err := service.FetchModels(ctx, record.Provider)
	if err != nil {
		t.Fatalf("FetchModels 失败: %v", err)
	}
	for _, model := range result.Models {
		if model.Category != "chat" {
			t.Fatalf("不应根据模型名推断 category: %+v", model)
		}
		if model.ContextWindow != nil || model.MaxOutputTokens != nil {
			t.Fatalf("不应根据模型名推断 token 窗口: %+v", model)
		}
		capabilities := model.CapabilitiesAuto
		if capabilities.Vision != nil ||
			capabilities.ImageOutput != nil ||
			capabilities.ToolCalling != nil ||
			capabilities.Reasoning != nil ||
			capabilities.Embedding != nil {
			t.Fatalf("不应根据模型名推断能力: %+v", model)
		}
	}
}

func TestParseModelListReadsProviderModelCard(t *testing.T) {
	models, err := parseModelList([]byte(`{"data":[{"id":"kimi-for-coding","created":1761264000,"created_at":"2025-10-24T00:00:00Z","object":"model","display_name":"Kimi-k2.6","type":"model","context_length":262144,"supports_reasoning":true,"supports_image_in":true,"supports_video_in":true}],"object":"list"}`))
	if err != nil {
		t.Fatalf("解析模型列表失败: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("模型数量不正确: %+v", models)
	}
	model := models[0]
	if model.ID != "kimi-for-coding" || model.DisplayName != "Kimi-k2.6" {
		t.Fatalf("基础模型字段解析不正确: %+v", model)
	}
	if model.Category != "chat" {
		t.Fatalf("type=model 不应被当成具体 category: %+v", model)
	}
	if model.ContextWindow == nil || *model.ContextWindow != 262144 {
		t.Fatalf("context_length 未解析: %+v", model)
	}
	if model.Capabilities.Reasoning == nil || !*model.Capabilities.Reasoning {
		t.Fatalf("supports_reasoning 未解析: %+v", model.Capabilities)
	}
	if model.Capabilities.Vision == nil || !*model.Capabilities.Vision {
		t.Fatalf("supports_image_in 未解析: %+v", model.Capabilities)
	}
}

func TestFetchModelsLogsModelsResponseData(t *testing.T) {
	ctx := context.Background()
	service, _ := newTestService(t)
	handler := &captureSlogHandler{}
	service.SetLogger(slog.New(handler))
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/models" {
			t.Fatalf("模型列表路径不正确: %s", request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"data":[{"id":"model-a","display_name":"Model A"}],"note":"secret-log-key"}`))
	}))
	defer server.Close()

	record, err := service.Create(ctx, CreateInput{
		Provider:   "log-fetch",
		PresetKey:  presetCustom,
		APIFormat:  APIFormatChatCompletions,
		AuthToken:  "secret-log-key",
		BaseURL:    server.URL,
		ModelsPath: "/models",
	})
	if err != nil {
		t.Fatalf("创建 provider 失败: %v", err)
	}
	if _, err = service.FetchModels(ctx, record.Provider); err != nil {
		t.Fatalf("FetchModels 失败: %v", err)
	}
	success := handler.find("Provider 模型列表请求成功")
	if success == nil {
		t.Fatalf("未输出模型列表成功日志: %+v", handler.messages())
	}
	if success.attrs["provider"] != "log-fetch" {
		t.Fatalf("日志 provider 不正确: %+v", success.attrs)
	}
	bodyPreview, _ := success.attrs["body_preview"].(string)
	if !strings.Contains(bodyPreview, "model-a") {
		t.Fatalf("日志 body_preview 未包含模型数据: %s", bodyPreview)
	}
	if strings.Contains(bodyPreview, "secret-log-key") {
		t.Fatalf("日志 body_preview 泄漏密钥: %s", bodyPreview)
	}
	modelIDs, _ := success.attrs["model_ids"].([]string)
	if len(modelIDs) != 1 || modelIDs[0] != "model-a" {
		t.Fatalf("日志 model_ids 不正确: %+v", success.attrs["model_ids"])
	}
}

func insertProviderUsageAgent(
	t *testing.T,
	db *sql.DB,
	agentID string,
	slug string,
	name string,
	displayName string,
	isMain bool,
	provider string,
	status string,
) {
	t.Helper()
	_, err := db.Exec(`
INSERT INTO agents (
    id, slug, name, description, definition, status, workspace_path, owner_user_id, is_main
) VALUES (?, ?, ?, '', '', ?, ?, 'owner-1', ?)`,
		agentID,
		slug,
		name,
		status,
		"/tmp/"+slug,
		isMain,
	)
	if err != nil {
		t.Fatalf("插入 agent 失败: %v", err)
	}
	_, err = db.Exec(`
INSERT INTO profiles (
    id, agent_id, display_name, headline, profile_markdown
) VALUES (?, ?, ?, '', '')`,
		"profile-"+agentID,
		agentID,
		displayName,
	)
	if err != nil {
		t.Fatalf("插入 profile 失败: %v", err)
	}
	_, err = db.Exec(`
INSERT INTO runtimes (
    id, agent_id, provider, permission_mode, allowed_tools_json, disallowed_tools_json,
    mcp_servers_json, setting_sources_json, runtime_version
) VALUES (?, ?, ?, '', '[]', '[]', '{}', '[]', 1)`,
		"runtime-"+agentID,
		agentID,
		provider,
	)
	if err != nil {
		t.Fatalf("插入 runtime 失败: %v", err)
	}
}

type runtimeSelection struct {
	provider string
	model    string
}

func runtimeSelectionsByAgent(t *testing.T, db *sql.DB, agentIDs ...string) map[string]runtimeSelection {
	t.Helper()
	result := map[string]runtimeSelection{}
	for _, agentID := range agentIDs {
		row := db.QueryRow(`SELECT COALESCE(provider, ''), COALESCE(model, '') FROM runtimes WHERE agent_id = ? LIMIT 1`, agentID)
		var item runtimeSelection
		if err := row.Scan(&item.provider, &item.model); err != nil {
			t.Fatalf("读取 runtime provider/model 失败: %v", err)
		}
		result[agentID] = item
	}
	return result
}

func TestUpdateModelCreatesManualModel(t *testing.T) {
	ctx := context.Background()
	service, _ := newTestService(t)

	record, err := service.Create(ctx, CreateInput{
		Provider:    "manual-model",
		PresetKey:   presetCustom,
		APIFormat:   APIFormatAnthropicMessages,
		AuthToken:   "manual-key",
		BaseURL:     "https://api.example.com",
		ModelsPath:  "/models",
		Enabled:     true,
		DisplayName: "Manual Model",
	})
	if err != nil {
		t.Fatalf("创建 provider 失败: %v", err)
	}
	created, err := service.UpdateModel(ctx, record.Provider, "claude-manual-1", UpdateModelInput{
		Enabled: true,
		CapabilitiesOverride: ModelCapabilities{
			Reasoning: boolPointer(true),
		},
		ProviderOptions: map[string]any{"thinking": map[string]any{"type": "enabled"}},
	})
	if err != nil {
		t.Fatalf("手动添加模型失败: %v", err)
	}
	if created.ModelID != "claude-manual-1" || !created.Enabled {
		t.Fatalf("手动模型记录不正确: %+v", created)
	}
	if created.CapabilitiesOverride.Reasoning == nil || !*created.CapabilitiesOverride.Reasoning {
		t.Fatalf("手动模型能力覆盖未保存: %+v", created.CapabilitiesOverride)
	}
	if created.ProviderOptions["thinking"] == nil {
		t.Fatalf("手动模型 provider options 未保存: %+v", created.ProviderOptions)
	}

	records, err := service.List(ctx)
	if err != nil {
		t.Fatalf("读取 provider 列表失败: %v", err)
	}
	if len(records) != 1 || len(records[0].Models) != 1 || records[0].Models[0].ModelID != "claude-manual-1" {
		t.Fatalf("手动模型未出现在 provider 模型列表: %+v", records)
	}
	if records[0].Models[0].IsDefault {
		t.Fatalf("手动启用模型不应自动成为默认模型: %+v", records[0].Models[0])
	}
}

func TestProviderTestPayloadsForSupportedAPIFormats(t *testing.T) {
	cases := []struct {
		name         string
		apiFormat    string
		expectedPath string
		assertBody   func(t *testing.T, body map[string]any)
	}{
		{
			name:         "chat",
			apiFormat:    APIFormatChatCompletions,
			expectedPath: "/chat/completions",
			assertBody: func(t *testing.T, body map[string]any) {
				t.Helper()
				if body["model"] != "model-1" || body["max_tokens"] != float64(1) || body["messages"] == nil {
					t.Fatalf("chat payload 不正确: %+v", body)
				}
			},
		},
		{
			name:         "responses",
			apiFormat:    APIFormatResponses,
			expectedPath: "/responses",
			assertBody: func(t *testing.T, body map[string]any) {
				t.Helper()
				if body["model"] != "model-1" || body["max_output_tokens"] != float64(1) || body["input"] != "ping" {
					t.Fatalf("responses payload 不正确: %+v", body)
				}
			},
		},
		{
			name:         "anthropic",
			apiFormat:    APIFormatAnthropicMessages,
			expectedPath: "/v1/messages",
			assertBody: func(t *testing.T, body map[string]any) {
				t.Helper()
				if body["model"] != "model-1" || body["max_tokens"] != float64(1) || body["messages"] == nil {
					t.Fatalf("anthropic payload 不正确: %+v", body)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			service, _ := newTestService(t)
			var calledPath string
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				if request.URL.Path == "/models" {
					writer.Header().Set("Content-Type", "application/json")
					_, _ = writer.Write([]byte(`{"data":[{"id":"model-1"}]}`))
					return
				}
				calledPath = request.URL.Path
				var body map[string]any
				if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
					t.Fatalf("解析请求 payload 失败: %v", err)
				}
				tc.assertBody(t, body)
				writer.Header().Set("Content-Type", "application/json")
				_, _ = writer.Write([]byte(`{}`))
			}))
			defer server.Close()

			record, err := service.Create(ctx, CreateInput{
				Provider:   "provider-" + tc.name,
				PresetKey:  presetCustom,
				APIFormat:  tc.apiFormat,
				AuthToken:  "token-1",
				BaseURL:    server.URL,
				ModelsPath: "/models",
				Enabled:    true,
			})
			if err != nil {
				t.Fatalf("创建 provider 失败: %v", err)
			}
			result, err := service.TestProvider(ctx, record.Provider)
			if err != nil {
				t.Fatalf("TestProvider 返回错误: %v", err)
			}
			if !result.Success {
				t.Fatalf("测试应成功: %+v", result)
			}
			if calledPath != tc.expectedPath {
				t.Fatalf("请求路径不正确: got=%s want=%s", calledPath, tc.expectedPath)
			}
		})
	}
}

func TestProviderTestRedactsSensitiveErrors(t *testing.T) {
	ctx := context.Background()
	service, _ := newTestService(t)
	var mu sync.Mutex
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		mu.Lock()
		requestCount++
		mu.Unlock()
		if request.URL.Path == "/models" {
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"data":[{"id":"model-1"}]}`))
			return
		}
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte(`{"error":"Bearer secret-token Authorization x-api-key"}`))
	}))
	defer server.Close()

	record, err := service.Create(ctx, CreateInput{
		Provider:   "redact",
		PresetKey:  presetCustom,
		APIFormat:  APIFormatChatCompletions,
		AuthToken:  "secret-token",
		BaseURL:    server.URL,
		ModelsPath: "/models",
		Enabled:    true,
	})
	if err != nil {
		t.Fatalf("创建 provider 失败: %v", err)
	}
	result, err := service.TestProvider(ctx, record.Provider)
	if err != nil {
		t.Fatalf("TestProvider 不应返回 transport 错误: %v", err)
	}
	if result.Success {
		t.Fatalf("测试应失败: %+v", result)
	}
	for _, leaked := range []string{"secret-token", "Authorization", "x-api-key"} {
		if strings.Contains(result.Error, leaked) {
			t.Fatalf("错误信息泄漏敏感内容 %q: %s", leaked, result.Error)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if requestCount != 2 {
		t.Fatalf("Provider 测试应先 /models 再模型请求: got=%d", requestCount)
	}
}

type capturedLogRecord struct {
	message string
	attrs   map[string]any
}

type captureSlogHandler struct {
	mu      sync.Mutex
	records []capturedLogRecord
}

func (h *captureSlogHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *captureSlogHandler) Handle(_ context.Context, record slog.Record) error {
	attrs := map[string]any{}
	record.Attrs(func(attr slog.Attr) bool {
		attrs[attr.Key] = attr.Value.Any()
		return true
	})
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, capturedLogRecord{
		message: record.Message,
		attrs:   attrs,
	})
	return nil
}

func (h *captureSlogHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h *captureSlogHandler) WithGroup(string) slog.Handler {
	return h
}

func (h *captureSlogHandler) find(message string) *capturedLogRecord {
	h.mu.Lock()
	defer h.mu.Unlock()
	for index := range h.records {
		if h.records[index].message == message {
			record := h.records[index]
			return &record
		}
	}
	return nil
}

func (h *captureSlogHandler) messages() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	result := make([]string, 0, len(h.records))
	for _, record := range h.records {
		result = append(result, record.message)
	}
	return result
}

func hasOptionProvider(items []Option, provider string) bool {
	for _, item := range items {
		if item.Provider == provider {
			return true
		}
	}
	return false
}
