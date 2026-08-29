package provider

import (
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

func TestKnownVisionCapability(t *testing.T) {
	t.Parallel()

	tests := []struct {
		modelID string
		want    bool
	}{
		{modelID: "gpt-5.4-mini-2026-03-17", want: true},
		{modelID: "claude-sonnet-4-6-20260217", want: true},
		{modelID: "models/gemini-3.1-pro-preview", want: true},
		{modelID: "grok-4.6", want: true},
		{modelID: "qwen3-vl-plus", want: true},
		{modelID: "qwen3.8-max", want: true},
		{modelID: "glm-5.3-flash", want: true},
		{modelID: "glm-5.3", want: false},
		{modelID: "glm-4v-plus", want: true},
		{modelID: "kimi-for-coding", want: true},
		{modelID: "kimi-k3", want: true},
		{modelID: "kimi-k2", want: false},
		{modelID: "MiniMax-M3", want: true},
		{modelID: "doubao-seed-2-0-pro-260215", want: true},
		{modelID: "ernie-5.0", want: true},
		{modelID: "step-3.7-flash", want: true},
		{modelID: "mistral-large-2512", want: true},
		{modelID: "meta-llama/Llama-4-Maverick-17B-128E-Instruct", want: true},
		{modelID: "command-a-plus-05-2026", want: true},
		{modelID: "us.amazon.nova-2-lite-v1:0", want: true},
		{modelID: "private-model-v1", want: false},
		{modelID: "deepseek-v4", want: false},
	}

	for _, test := range tests {
		t.Run(test.modelID, func(t *testing.T) {
			t.Parallel()
			got := knownVisionCapability(test.modelID)
			if test.want && (got == nil || !*got) {
				t.Fatalf("knownVisionCapability(%q) = %v, want true", test.modelID, got)
			}
			if !test.want && got != nil {
				t.Fatalf("knownVisionCapability(%q) = %v, want unknown", test.modelID, *got)
			}
		})
	}
}

func TestKnownMaxOutputTokens(t *testing.T) {
	t.Parallel()

	tests := map[string]int{
		"gpt-5.6-sol":                128_000,
		"gpt-oss-20b":                131_072,
		"claude-opus-5":              128_000,
		"gemini-3.7-flash":           65_536,
		"glm-5.3":                    131_072,
		"qwen3.8-max":                131_072,
		"ernie-5.1":                  65_536,
		"eu.amazon.nova-2-lite-v1:0": 65_536,
	}
	for modelID, want := range tests {
		if got := knownMaxOutputTokens(modelID); got == nil || *got != want {
			t.Fatalf("knownMaxOutputTokens(%q) = %v, want %d", modelID, got, want)
		}
	}
}

func TestKnownReasoningCapability(t *testing.T) {
	t.Parallel()

	for _, modelID := range []string{
		"gpt-oss-120b",
		"claude-sonnet-5",
		"gemini-3.7-flash",
		"grok-4.6",
		"glm-5.3-flash",
		"kimi-k3",
		"qwen3.8-max",
		"MiniMax-M3",
		"doubao-seed-2-0-lite-260215",
		"ernie-5.0",
		"hy3",
		"step-3.5-flash",
		"mistral-small-2603",
		"command-a-plus-05-2026",
	} {
		model := providerstore.ModelEntity{ModelID: modelID}
		if !modelHasReasoningCapability(model) {
			t.Fatalf("modelHasReasoningCapability(%q) = false, want true", modelID)
		}
	}
}

func TestRemoteModelCardPrefersProviderContextWindow(t *testing.T) {
	t.Parallel()

	providerValue := 321_000
	model := remoteModel{ID: "glm-5.2", ContextWindow: &providerValue}
	_, _, contextWindow, _ := model.modelCard(ProviderKindLLM)
	if contextWindow == nil || *contextWindow != providerValue {
		t.Fatalf("Provider context_window 应覆盖内置目录: %v", contextWindow)
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

func TestRemoteModelCardPrefersProviderVisionCapability(t *testing.T) {
	t.Parallel()

	unsupported := false
	model := remoteModel{
		ID:           "gpt-5.4",
		Capabilities: ModelCapabilities{Vision: &unsupported},
	}
	capabilities, _, _, _ := model.modelCard(ProviderKindLLM)
	if capabilities.Vision == nil || *capabilities.Vision {
		t.Fatalf("Provider vision=false 应覆盖内置目录: %v", capabilities.Vision)
	}
}

func TestModelVisionOverrideWinsKnownCatalog(t *testing.T) {
	t.Parallel()

	model := providerstore.ModelEntity{
		ModelID:                  "gpt-5.4",
		CapabilitiesAutoJSON:     `{"vision":true}`,
		CapabilitiesOverrideJSON: `{"vision":false}`,
	}
	if modelHasVisionCapability(model) {
		t.Fatal("用户 vision=false 覆盖应优先于 Provider 与内置模型卡")
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
	if category != "chat" || capabilities.Vision == nil || !*capabilities.Vision {
		t.Fatalf("手动添加模型未应用内置模型卡: category=%s capabilities=%+v", category, capabilities)
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
	if legacy.CapabilitiesAuto.Vision == nil || !*legacy.CapabilitiesAuto.Vision ||
		legacy.CapabilitiesAuto.Reasoning == nil || !*legacy.CapabilitiesAuto.Reasoning {
		t.Fatalf("模型列表未展示内置能力: %+v", legacy.CapabilitiesAuto)
	}
}

func TestStoredModelExplicitContextWinsKnownWindow(t *testing.T) {
	t.Parallel()

	explicit := 123_456
	model := providerstore.ModelEntity{ModelID: "deepseek-v4-pro", ContextWindow: &explicit}
	if got := modelContextWindow(&model); got != explicit {
		t.Fatalf("用户配置应覆盖内置上下文窗口: %d", got)
	}
}
