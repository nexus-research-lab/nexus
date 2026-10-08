package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	providerstore "github.com/nexus-research-lab/nexus/internal/storage/provider"
)

func TestProviderEndpointURLPreservesOperationPathAndQuery(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		baseURL   string
		apiFormat string
		want      string
	}{
		{
			name:      "Azure Responses base with query",
			baseURL:   "https://sample.openai.azure.com/openai/v1?api-version=preview",
			apiFormat: APIFormatResponses,
			want:      "https://sample.openai.azure.com/openai/v1/responses?api-version=preview",
		},
		{
			name:      "Azure OpenAI resource root",
			baseURL:   "https://sample.openai.azure.com/openai/",
			apiFormat: APIFormatResponses,
			want:      "https://sample.openai.azure.com/openai/v1/responses",
		},
		{
			name:      "Azure Foundry project root",
			baseURL:   "https://sample.services.ai.azure.com/api/projects/project-1",
			apiFormat: APIFormatResponses,
			want:      "https://sample.services.ai.azure.com/api/projects/project-1/openai/v1/responses",
		},
		{
			name:      "full Responses operation URL",
			baseURL:   "https://api.example.com/v1/responses",
			apiFormat: APIFormatResponses,
			want:      "https://api.example.com/v1/responses",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			item := providerstore.Entity{
				ProviderKind: ProviderKindLLM,
				APIFormat:    test.apiFormat,
				BaseURL:      test.baseURL,
			}
			if got := endpointURL(item, test.apiFormat); got != test.want {
				t.Fatalf("endpointURL() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestAzureProviderHeadersIncludeAPIKey(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodPost, "https://sample.openai.azure.com/openai/v1/responses", nil)
	applyProviderHeaders(request, providerstore.Entity{
		APIFormat: APIFormatResponses,
		AuthToken: "azure-key",
		BaseURL:   "https://sample.openai.azure.com/openai/v1",
	})
	if got := request.Header.Get("api-key"); got != "azure-key" {
		t.Fatalf("api-key = %q, want azure-key", got)
	}
	if got := request.Header.Get("Authorization"); got != "Bearer azure-key" {
		t.Fatalf("Authorization = %q, want Bearer azure-key", got)
	}
}

func TestAzureChatCompletionsPayloadUsesMaxCompletionTokens(t *testing.T) {
	items := []providerstore.Entity{
		{
			PresetKey: presetAzure,
			APIFormat: APIFormatChatCompletions,
		},
		{
			PresetKey: "custom",
			BaseURL:   "https://resource-name.openai.azure.com/openai/v1",
			APIFormat: APIFormatChatCompletions,
		},
		{
			PresetKey: "custom",
			BaseURL:   "https://resource-name.cognitiveservices.azure.com/openai/v1",
			APIFormat: APIFormatChatCompletions,
		},
	}
	for _, item := range items {
		payload, err := minimalPayload(item, "production-chat")
		if err != nil {
			t.Fatalf("生成 Azure 测试 payload 失败: %v", err)
		}
		var body map[string]any
		if err = json.Unmarshal(payload, &body); err != nil {
			t.Fatalf("解析 Azure 测试 payload 失败: %v", err)
		}
		if body["max_completion_tokens"] != float64(azureModelCheckMaxCompletionTokens) {
			t.Fatalf("Azure payload 缺少 max_completion_tokens: %+v", body)
		}
		if _, exists := body["max_tokens"]; exists {
			t.Fatalf("Azure payload 不应发送 max_tokens: %+v", body)
		}
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
	if requestCount != 6 {
		t.Fatalf("Provider 测试应读取目录后独立验证五类可用协议: got=%d", requestCount)
	}
}
