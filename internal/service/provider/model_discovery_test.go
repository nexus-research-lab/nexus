package provider

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

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

func TestFetchModelsAutoSelectsDefaultRuntimeModel(t *testing.T) {
	ctx := context.Background()
	service, _ := newTestService(t)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/models" {
			t.Fatalf("模型列表路径不正确: %s", request.URL.Path)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"data":[{"id":"model-b"},{"id":"model-a"}]}`))
	}))
	defer server.Close()

	record, err := service.Create(ctx, CreateInput{
		Provider:    "runtime-default",
		PresetKey:   presetCustom,
		APIFormat:   APIFormatAnthropicMessages,
		AuthToken:   "runtime-key",
		BaseURL:     server.URL,
		ModelsPath:  "/models",
		Enabled:     true,
		DisplayName: "Runtime Default",
	})
	if err != nil {
		t.Fatalf("创建 provider 失败: %v", err)
	}
	if _, err = service.FetchModels(ctx, record.Provider); err != nil {
		t.Fatalf("FetchModels 失败: %v", err)
	}
	options, err := service.ListOptions(ctx)
	if err != nil {
		t.Fatalf("读取 provider options 失败: %v", err)
	}
	if options.DefaultProvider == nil || *options.DefaultProvider != record.Provider ||
		options.DefaultModel == nil || *options.DefaultModel != "model-b" {
		t.Fatalf("未自动选择默认模型: %+v", options)
	}
	runtimeConfig, err := service.ResolveRuntimeConfig(ctx, record.Provider, "")
	if err != nil {
		t.Fatalf("显式 provider 缺省 model 应回落到默认模型: %v", err)
	}
	if runtimeConfig.Provider != record.Provider || runtimeConfig.Model != "model-b" {
		t.Fatalf("runtime config 默认模型不正确: %+v", runtimeConfig)
	}
}

func TestFetchModelsKeepsExistingDefaultRuntimeModel(t *testing.T) {
	ctx := context.Background()
	service, _ := newTestService(t)
	first, err := service.Create(ctx, CreateInput{
		Provider:  "first-default",
		PresetKey: presetCustom,
		APIFormat: APIFormatAnthropicMessages,
		AuthToken: "first-key",
		BaseURL:   "https://first.example.com",
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("创建首个 provider 失败: %v", err)
	}
	if _, err = service.UpdateModel(ctx, first.Provider, "first-model", UpdateModelInput{
		Enabled:   true,
		IsDefault: true,
	}); err != nil {
		t.Fatalf("设置首个默认模型失败: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"data":[{"id":"second-model"}]}`))
	}))
	defer server.Close()
	second, err := service.Create(ctx, CreateInput{
		Provider:   "second-default",
		PresetKey:  presetCustom,
		APIFormat:  APIFormatAnthropicMessages,
		AuthToken:  "second-key",
		BaseURL:    server.URL,
		ModelsPath: "/models",
		Enabled:    true,
	})
	if err != nil {
		t.Fatalf("创建第二个 provider 失败: %v", err)
	}
	if _, err = service.FetchModels(ctx, second.Provider); err != nil {
		t.Fatalf("FetchModels 失败: %v", err)
	}
	options, err := service.ListOptions(ctx)
	if err != nil {
		t.Fatalf("读取 provider options 失败: %v", err)
	}
	if options.DefaultProvider == nil || *options.DefaultProvider != first.Provider ||
		options.DefaultModel == nil || *options.DefaultModel != "first-model" {
		t.Fatalf("已有默认模型不应被覆盖: %+v", options)
	}
}

func TestFetchModelsFillsKnownContextWithoutInferringCapabilities(t *testing.T) {
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
		if model.ModelID == "kimi-for-coding" {
			if model.ContextWindow == nil || *model.ContextWindow != 262_144 {
				t.Fatalf("已知模型应补齐 context_window: %+v", model)
			}
		} else if model.ContextWindow != nil {
			t.Fatalf("未知上下文窗口不应被猜测: %+v", model)
		}
		if model.MaxOutputTokens != nil {
			t.Fatalf("不应根据模型名推断 max_output_tokens: %+v", model)
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
	if _, ok := success.attrs["body_preview"]; ok {
		t.Fatalf("成功日志不应记录完整响应预览: %+v", success.attrs)
	}
	modelIDs, _ := success.attrs["model_ids"].([]string)
	if len(modelIDs) != 1 || modelIDs[0] != "model-a" {
		t.Fatalf("日志 model_ids 不正确: %+v", success.attrs["model_ids"])
	}
}
