package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
)

func TestUpdateModelNormalizesEscapedSlashModelID(t *testing.T) {
	ctx := context.Background()
	service, _ := newTestService(t)
	record, err := service.Create(ctx, CreateInput{
		ProviderKind: ProviderKindImageGeneration,
		Provider:     "modelscope-escaped",
		PresetKey:    presetCustom,
		APIFormat:    APIFormatModelScopeImageGeneration,
		AuthToken:    "image-key",
		BaseURL:      "https://api-inference.modelscope.cn/v1",
		ModelsPath:   "",
		Enabled:      true,
		DisplayName:  "ModelScope",
	})
	if err != nil {
		t.Fatalf("创建 ModelScope provider 失败: %v", err)
	}

	const decodedModelID = "Tongyi-MAI/Z-Image-Turbo"
	const escapedModelID = "Tongyi-MAI%2FZ-Image-Turbo"
	now := service.now()
	err = service.repository.UpsertModels(ctx, []providerstore.ModelEntity{
		{
			ID:                       service.idFactory("provider_model"),
			ProviderID:               record.ID,
			ModelID:                  escapedModelID,
			DisplayName:              escapedModelID,
			Category:                 "image",
			CapabilitiesAutoJSON:     "{}",
			CapabilitiesOverrideJSON: "{}",
			ProviderOptionsJSON:      "{}",
			LastSeenAt:               now,
			CreatedAt:                now,
			UpdatedAt:                now,
		},
	})
	if err != nil {
		t.Fatalf("写入旧转义模型失败: %v", err)
	}

	renamed, err := service.UpdateModel(ctx, record.Provider, escapedModelID, UpdateModelInput{
		Enabled:   true,
		IsDefault: true,
	})
	if err != nil {
		t.Fatalf("写入转义模型失败: %v", err)
	}
	if renamed.ModelID != decodedModelID || renamed.DisplayName != decodedModelID {
		t.Fatalf("模型 ID 未归一化: %+v", renamed)
	}

	imageConfig, err := service.ResolveImageModelConfig(ctx, record.Provider, escapedModelID)
	if err != nil {
		t.Fatalf("解析转义模型失败: %v", err)
	}
	if imageConfig.Model != decodedModelID {
		t.Fatalf("生图配置模型 ID 未归一化: %+v", imageConfig)
	}

	updated, err := service.UpdateModel(ctx, record.Provider, decodedModelID, UpdateModelInput{
		Enabled:   true,
		IsDefault: true,
	})
	if err != nil {
		t.Fatalf("用真实模型 ID 更新失败: %v", err)
	}
	if updated.ModelID != decodedModelID {
		t.Fatalf("真实模型 ID 更新后不正确: %+v", updated)
	}

	listed, err := service.Get(ctx, record.Provider)
	if err != nil {
		t.Fatalf("读取 provider 失败: %v", err)
	}
	count := 0
	for _, model := range listed.Models {
		if model.ModelID == escapedModelID {
			t.Fatalf("模型列表不应返回转义 ID: %+v", listed.Models)
		}
		if model.ModelID == decodedModelID {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("模型列表应只保留一个真实 ID: count=%d models=%+v", count, listed.Models)
	}
}

func TestTestProviderAutoSelectsTestedModel(t *testing.T) {
	ctx := context.Background()
	service, _ := newTestService(t)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/models":
			writer.Header().Set("Content-Type", "application/json")
			_, _ = writer.Write([]byte(`{"data":[{"id":"model-b"},{"id":"model-a"}]}`))
		case "/v1/messages":
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write([]byte(`{}`))
		default:
			t.Fatalf("未预期的测试请求路径: %s", request.URL.Path)
		}
	}))
	defer server.Close()

	record, err := service.Create(ctx, CreateInput{
		Provider:   "test-provider-default",
		PresetKey:  presetCustom,
		APIFormat:  APIFormatAnthropicMessages,
		AuthToken:  "test-key",
		BaseURL:    server.URL,
		ModelsPath: "/models",
		Enabled:    true,
	})
	if err != nil {
		t.Fatalf("创建 provider 失败: %v", err)
	}
	result, err := service.TestProvider(ctx, record.Provider)
	if err != nil {
		t.Fatalf("测试 provider 失败: %v", err)
	}
	if !result.Success || result.Model != "model-b" {
		t.Fatalf("测试结果不正确: %+v", result)
	}
	options, err := service.ListOptions(ctx)
	if err != nil {
		t.Fatalf("读取 provider options 失败: %v", err)
	}
	if options.DefaultProvider == nil || *options.DefaultProvider != record.Provider ||
		options.DefaultModel == nil || *options.DefaultModel != "model-b" {
		t.Fatalf("provider 测试成功后未自动设置默认模型: %+v", options)
	}
}

func TestTestModelAutoSelectsTestedModel(t *testing.T) {
	ctx := context.Background()
	service, _ := newTestService(t)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/v1/messages" {
			t.Fatalf("未预期的测试请求路径: %s", request.URL.Path)
		}
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(`{}`))
	}))
	defer server.Close()

	record, err := service.Create(ctx, CreateInput{
		Provider:  "test-model-default",
		PresetKey: presetCustom,
		APIFormat: APIFormatAnthropicMessages,
		AuthToken: "model-key",
		BaseURL:   server.URL,
		Enabled:   true,
	})
	if err != nil {
		t.Fatalf("创建 provider 失败: %v", err)
	}
	result, err := service.TestModel(ctx, record.Provider, "manual-model")
	if err != nil {
		t.Fatalf("测试模型失败: %v", err)
	}
	if !result.Success || result.Model != "manual-model" {
		t.Fatalf("模型测试结果不正确: %+v", result)
	}
	runtimeConfig, err := service.ResolveRuntimeConfig(ctx, record.Provider, "")
	if err != nil {
		t.Fatalf("测试模型成功后应可解析 runtime config: %v", err)
	}
	if runtimeConfig.Model != "manual-model" {
		t.Fatalf("测试模型未成为默认模型: %+v", runtimeConfig)
	}
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
