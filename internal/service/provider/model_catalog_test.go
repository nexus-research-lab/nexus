// model_catalog_test.go 覆盖模型目录、能力卡声明、建议与变更测试。
package provider

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
)

func TestKnownContextWindow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		modelID string
		want    int
	}{
		{name: "OpenAI snapshot", modelID: "gpt-5.4-2026-03-05", want: 1_050_000},
		{name: "OpenAI current generation", modelID: "gpt-5.6-terra", want: 1_050_000},
		{name: "OpenAI smaller variant", modelID: "gpt-5.4-mini-2026-03-17", want: 400_000},
		{name: "OpenAI chat alias", modelID: "gpt-5.2-chat-latest", want: 128_000},
		{name: "OpenAI open weight", modelID: "gpt-oss-120b", want: 131_072},
		{name: "OpenAI variant does not inherit parent", modelID: "gpt-5.4-micro", want: 0},
		{name: "Claude 5", modelID: "claude-opus-5", want: 1_000_000},
		{name: "Claude current generation", modelID: "claude-opus-4-8", want: 1_000_000},
		{name: "Claude 4.6 snapshot", modelID: "claude-sonnet-4-6-20260217", want: 1_000_000},
		{name: "Claude snapshot", modelID: "claude-sonnet-4-5-20250929", want: 200_000},
		{name: "Gemini namespace", modelID: "models/gemini-3.1-pro-preview", want: 1_048_576},
		{name: "Gemini stable", modelID: "gemini-3.7-flash", want: 1_048_576},
		{name: "Grok", modelID: "grok-4.6", want: 500_000},
		{name: "Mistral open weight", modelID: "mistral-medium-3-5", want: 262_144},
		{name: "Llama namespace", modelID: "meta-llama/Llama-4-Scout-17B-16E-Instruct", want: 10_000_000},
		{name: "Cohere", modelID: "command-a-plus-05-2026", want: 128_000},
		{name: "Amazon regional ID", modelID: "global.amazon.nova-2-lite-v1:0", want: 1_000_000},
		{name: "DeepSeek V4", modelID: "deepseek-v4-flash", want: 1_000_000},
		{name: "GLM 5.3", modelID: "glm-5.3", want: 1_000_000},
		{name: "GLM 5.3 Flash", modelID: "glm-5.3-flash", want: 1_000_000},
		{name: "GLM 5.3 FlashX", modelID: "glm-5.3-flashx", want: 1_000_000},
		{name: "GLM 5.2", modelID: "glm-5.2", want: 1_000_000},
		{name: "GLM coding plan", modelID: "glm-5.1", want: 202_752},
		{name: "Kimi coding alias", modelID: "kimi-for-coding", want: 262_144},
		{name: "Kimi K3", modelID: "kimi-k3[1m]", want: 1_048_576},
		{name: "Kimi namespaced", modelID: "kimi%2Fkimi-k2.6", want: 262_144},
		{name: "Qwen 3.8", modelID: "qwen3.8-max", want: 1_000_000},
		{name: "Qwen snapshot", modelID: "qwen3.7-plus-2026-05-26", want: 1_000_000},
		{name: "Qwen old max", modelID: "qwen3-max-2026-01-23", want: 262_144},
		{name: "MiniMax", modelID: "MiniMax-M2.5", want: 196_608},
		{name: "ERNIE", modelID: "ernie-5.1", want: 131_072},
		{name: "Tencent Hy", modelID: "hy3", want: 262_144},
		{name: "StepFun", modelID: "step-3.5-flash", want: 262_144},
		{name: "Unknown", modelID: "private-model-v1", want: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got := knownContextWindow(test.modelID)
			if test.want == 0 {
				if got != nil {
					t.Fatalf("knownContextWindow(%q) = %d, want nil", test.modelID, *got)
				}
				return
			}
			if got == nil || *got != test.want {
				t.Fatalf("knownContextWindow(%q) = %v, want %d", test.modelID, got, test.want)
			}
		})
	}
}

func TestRemoteModelCardPrefersProviderMaxOutputTokens(t *testing.T) {
	t.Parallel()

	providerValue := 64_000
	model := remoteModel{ID: "deepseek-v4-flash", MaxOutputTokens: &providerValue}
	_, _, _, maxOutputTokens := model.modelCard(ProviderKindLLM)
	if maxOutputTokens == nil || *maxOutputTokens != providerValue {
		t.Fatalf("Provider max_output_tokens 应覆盖内置目录: %v", maxOutputTokens)
	}
}

func TestRemoteModelFromCardReadsCamelCaseTokenLimits(t *testing.T) {
	t.Parallel()

	model := remoteModelFromCard(map[string]any{
		"id":               "vendor-model",
		"inputTokenLimit":  float64(777_000),
		"outputTokenLimit": "32000",
	})
	if model.ContextWindow == nil || *model.ContextWindow != 777_000 {
		t.Fatalf("未识别 Provider 的 inputTokenLimit: %v", model.ContextWindow)
	}
	if model.MaxOutputTokens == nil || *model.MaxOutputTokens != 32_000 {
		t.Fatalf("未识别 Provider 的 outputTokenLimit: %v", model.MaxOutputTokens)
	}
}

func TestDefaultModelCardFillsCatalogAndRuntimeDefaults(t *testing.T) {
	t.Parallel()

	capabilities, category, contextWindow, maxOutputTokens := defaultModelCard("kimi-for-coding", ProviderKindLLM)
	if contextWindow == nil || *contextWindow != 262_144 {
		t.Fatalf("手动添加模型未应用内置上下文窗口: %v", contextWindow)
	}
	if maxOutputTokens == nil || *maxOutputTokens != defaultModelMaxOutputTokens {
		t.Fatalf("模型缺少输出上限时未应用运行时默认值: %v", maxOutputTokens)
	}
	if category != "chat" || capabilities.Vision != nil {
		t.Fatalf("手动添加模型不得持久化目录能力猜测: category=%s capabilities=%+v", category, capabilities)
	}

	_, _, contextWindow, maxOutputTokens = defaultModelCard("private-model-v1", ProviderKindLLM)
	if contextWindow == nil || *contextWindow != defaultModelContextWindow ||
		maxOutputTokens == nil || *maxOutputTokens != defaultModelMaxOutputTokens {
		t.Fatalf("未知 LLM 未应用运行时默认值: context=%v output=%v", contextWindow, maxOutputTokens)
	}

	_, category, contextWindow, maxOutputTokens = defaultModelCard("image-model-v1", ProviderKindImageGeneration)
	if category != "image" || contextWindow != nil || maxOutputTokens != nil {
		t.Fatalf("生图模型不应应用 LLM token 默认值: category=%s context=%v output=%v", category, contextWindow, maxOutputTokens)
	}
}

func TestStoredModelWithoutLimitsUsesKnownCatalog(t *testing.T) {
	t.Parallel()

	model := providerstore.ModelEntity{ModelID: "deepseek-v4-pro"}
	if got := modelContextWindow(&model); got != 1_000_000 {
		t.Fatalf("历史模型卡未应用内置上下文窗口: %d", got)
	}
	record := toModelRecord(model)
	if record.ContextWindow == nil || *record.ContextWindow != 1_000_000 {
		t.Fatalf("模型列表未展示内置上下文窗口: %v", record.ContextWindow)
	}
	if got := modelMaxOutputTokens(&model); got != 384_000 {
		t.Fatalf("历史模型卡未应用内置输出上限: %d", got)
	}
	if record.MaxOutputTokens == nil || *record.MaxOutputTokens != 384_000 {
		t.Fatalf("模型列表未展示内置输出上限: %v", record.MaxOutputTokens)
	}
	legacy := toModelRecord(providerstore.ModelEntity{
		ModelID:              "gpt-5.6-sol",
		CapabilitiesAutoJSON: "{}",
	})
	if legacy.CapabilitiesAuto.Vision != nil || legacy.CapabilitiesAuto.Reasoning != nil {
		t.Fatalf("原始模型卡不应包含名称推断能力: %+v", legacy.CapabilitiesAuto)
	}
}

func TestModelNameFallbackKeepsProviderPolicyScoped(t *testing.T) {
	item := providerstore.Entity{PresetKey: presetCustom, ProviderKind: ProviderKindLLM}
	model := providerstore.ModelEntity{ModelID: "qwen3.8-max"}
	g := projectModelGuidance(item, model)
	if !g.Eligibility[PurposeVision].Available || g.Recommendations[PurposeChat] != "" || g.Evidence == nil || g.Evidence.Notice != "" {
		t.Fatalf("模型名能力与 Provider 专属建议未分离: %+v", g)
	}
	if lookupModelAdviceByID("wan2.7-image-pro") != nil {
		t.Fatal("冲突的同名模型能力不应自动推断")
	}
	model.CapabilitiesAutoJSON = encodeModelAutoCapabilities(ModelCapabilities{Vision: adviceBool(false)})
	if projectModelGuidance(item, model).Eligibility[PurposeVision].Available {
		t.Fatal("Provider 显式能力否定未覆盖模型名默认值")
	}
	model.CapabilitiesOverrideJSON = `{"vision":true}`
	if !projectModelGuidance(item, model).Eligibility[PurposeVision].Available {
		t.Fatal("用户显式能力覆盖未生效")
	}
	model.ModelID = "glm-5.3"
	model.CapabilitiesOverrideJSON = "{}"
	model.CapabilitiesAutoJSON = encodeModelAutoCapabilities(ModelCapabilities{Vision: adviceBool(true)})
	if !projectModelGuidance(item, model).Eligibility[PurposeVision].Available {
		t.Fatal("远端事实应能修正跨 Provider 的同名默认值")
	}
}

func TestAdviceAutomaticDenialsAndUserOverride(t *testing.T) {
	item := providerstore.Entity{PresetKey: presetGLMCodingPlan, ProviderKind: ProviderKindLLM}
	model := providerstore.ModelEntity{ModelID: "glm-5.3", CapabilitiesAutoJSON: encodeModelAutoCapabilities(ModelCapabilities{Vision: adviceBool(true)})}
	if projectModelGuidance(item, model).Eligibility[PurposeVision].Available {
		t.Fatal("automatic positive overrode documented denial")
	}
	model.CapabilitiesOverrideJSON = `{"vision":true}`
	if !projectModelGuidance(item, model).Eligibility[PurposeVision].Available {
		t.Fatal("explicit user override lost")
	}
	model.ModelID = "glm-5.3-flash"
	model.CapabilitiesOverrideJSON = "{}"
	model.CapabilitiesAutoJSON = encodeModelAutoCapabilities(ModelCapabilities{Vision: adviceBool(false)})
	g := projectModelGuidance(item, model)
	if g.Eligibility[PurposeVision].Available || g.Sources["vision"] != "provider_record" || g.TextOnly {
		t.Fatalf("remote denial lost or false text-only label: %+v", g)
	}
}

func TestMultimodalTextOutputWinsOverEmbeddingFlag(t *testing.T) {
	item := providerstore.Entity{PresetKey: presetKimiCode, ProviderKind: ProviderKindLLM}
	model := providerstore.ModelEntity{
		ModelID:                  "k3-256k",
		CapabilitiesAutoJSON:     `{"vision":true,"reasoning":true}`,
		CapabilitiesOverrideJSON: `{"vision":true,"image_output":true,"tool_calling":true,"reasoning":true,"embedding":true}`,
	}

	guidance := projectModelGuidance(item, model)
	if !guidance.Eligibility[PurposeChat].Available || !guidance.Eligibility[PurposeVision].Available {
		t.Fatalf("text-capable multimodal model must remain eligible for chat and vision: %+v", guidance)
	}
}

func TestDocumentedImageCapabilityStillRequiresTransport(t *testing.T) {
	for _, tc := range []struct {
		preset, id     string
		generate, edit bool
	}{
		{presetOpenAI, "gpt-image-2.5-sunburst", true, true},
		{presetDashScope, "wan2.7-image-pro", true, true},
		{presetDoubao, "doubao-seedream-5-0-pro-260628", true, false},
		{presetQwenTokenPlan, "qwen-image-3.0-pro", false, false},
		{presetModelScope, "Qwen/Qwen-Image", true, false},
	} {
		g := projectModelGuidance(providerstore.Entity{PresetKey: tc.preset, ProviderKind: ProviderKindLLM}, providerstore.ModelEntity{ModelID: tc.id})
		if g.Eligibility[PurposeImage].Available != tc.generate || g.Eligibility[PurposeEdit].Available != tc.edit || g.Eligibility[PurposeChat].Available {
			t.Fatalf("%s: %+v", tc.id, g)
		}
	}
}

func TestModelCardCapabilityDeclarations(t *testing.T) {
	for _, tt := range []struct {
		name, card string
		want       ModelCapabilities
	}{
		{"all independent", `{"capabilities":{"text_output":true,"vision":true,"image_output":true,"image_editing":false,"tool_calling":true,"reasoning":false,"embedding":true}}`,
			ModelCapabilities{TextOutput: adviceBool(true), Vision: adviceBool(true), ImageOutput: adviceBool(true), ImageEditing: adviceBool(false), ToolCalling: adviceBool(true), Reasoning: adviceBool(false), Embedding: adviceBool(true)}},
		{"nested modalities", `{"architecture":{"input_modalities":["text","image"],"output_modalities":["text","image"]},"supported_parameters":["tools","reasoning_effort"]}`,
			ModelCapabilities{TextOutput: adviceBool(true), Vision: adviceBool(true), ImageOutput: adviceBool(true), ToolCalling: adviceBool(true), Reasoning: adviceBool(true), Embedding: adviceBool(false)}},
		{"explicit denial wins", `{"vision":false,"reasoning":false,"architecture":{"input_modalities":["image"]},"supported_parameters":["reasoning"]}`,
			ModelCapabilities{Vision: adviceBool(false), Reasoning: adviceBool(false)}},
		{"name alone is unknown", `{"id":"kimi-k2.6"}`, ModelCapabilities{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var card map[string]any
			if err := json.Unmarshal([]byte(tt.card), &card); err != nil {
				t.Fatal(err)
			}
			got := modelCapabilitiesFromCard(card)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %+v; want %+v", got, tt.want)
			}
		})
	}
}

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
	if created.ContextWindow == nil || *created.ContextWindow != defaultModelContextWindow ||
		created.MaxOutputTokens == nil || *created.MaxOutputTokens != defaultModelMaxOutputTokens {
		t.Fatalf("手动模型缺少卡片信息时未应用默认值: %+v", created)
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

	deleted, err := service.DeleteModel(ctx, record.Provider, created.ModelID)
	if err != nil {
		t.Fatalf("删除手动模型失败: %v", err)
	}
	if deleted.Provider != record.Provider || deleted.Model != created.ModelID {
		t.Fatalf("模型删除结果不正确: %+v", deleted)
	}
	updatedRecord, err := service.Get(ctx, record.Provider)
	if err != nil {
		t.Fatalf("删除后读取 provider 失败: %v", err)
	}
	if len(updatedRecord.Models) != 0 {
		t.Fatalf("手动模型删除后仍然存在: %+v", updatedRecord.Models)
	}
}
