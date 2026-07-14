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
		{name: "OpenAI variant does not inherit parent", modelID: "gpt-5.4-micro", want: 0},
		{name: "Claude current generation", modelID: "claude-opus-4-8", want: 1_000_000},
		{name: "Claude 4.6 snapshot", modelID: "claude-sonnet-4-6-20260217", want: 1_000_000},
		{name: "Claude snapshot", modelID: "claude-sonnet-4-5-20250929", want: 200_000},
		{name: "Gemini namespace", modelID: "models/gemini-3.1-pro-preview", want: 1_048_576},
		{name: "DeepSeek V4", modelID: "deepseek-v4-flash", want: 1_000_000},
		{name: "GLM 5.2", modelID: "glm-5.2", want: 1_000_000},
		{name: "GLM coding plan", modelID: "glm-5.1", want: 202_752},
		{name: "Kimi coding alias", modelID: "kimi-for-coding", want: 262_144},
		{name: "Kimi namespaced", modelID: "kimi%2Fkimi-k2.6", want: 262_144},
		{name: "Qwen snapshot", modelID: "qwen3.7-plus-2026-05-26", want: 1_000_000},
		{name: "Qwen old max", modelID: "qwen3-max-2026-01-23", want: 262_144},
		{name: "MiniMax", modelID: "MiniMax-M2.5", want: 196_608},
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

func TestRemoteModelCardPrefersProviderContextWindow(t *testing.T) {
	t.Parallel()

	providerValue := 321_000
	model := remoteModel{ID: "glm-5.2", ContextWindow: &providerValue}
	_, _, contextWindow, _ := model.modelCard()
	if contextWindow == nil || *contextWindow != providerValue {
		t.Fatalf("Provider context_window 应覆盖内置目录: %v", contextWindow)
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

func TestDefaultModelCardFillsKnownContextWindow(t *testing.T) {
	t.Parallel()

	_, _, contextWindow, _ := defaultModelCard("kimi-for-coding")
	if contextWindow == nil || *contextWindow != 262_144 {
		t.Fatalf("手动添加模型未应用内置上下文窗口: %v", contextWindow)
	}
}

func TestStoredModelWithoutContextUsesKnownWindow(t *testing.T) {
	t.Parallel()

	model := providerstore.ModelEntity{ModelID: "deepseek-v4-pro"}
	if got := modelContextWindow(&model); got != 1_000_000 {
		t.Fatalf("历史模型卡未应用内置上下文窗口: %d", got)
	}
	record := toModelRecord(model)
	if record.ContextWindow == nil || *record.ContextWindow != 1_000_000 {
		t.Fatalf("模型列表未展示内置上下文窗口: %v", record.ContextWindow)
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
