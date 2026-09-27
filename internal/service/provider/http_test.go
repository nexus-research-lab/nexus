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

func TestAzureResponsesRejectsLegacyOperationURL(t *testing.T) {
	t.Parallel()

	err := validateModelEndpoint(providerstore.Entity{
		APIFormat: APIFormatResponses,
		BaseURL:   "https://sample.cognitiveservices.azure.com/openai/deployments/gpt-image/images/generations?api-version=2024-02-01",
	})
	if err == nil || !strings.Contains(err.Error(), "/deployments/") {
		t.Fatalf("validateModelEndpoint() error = %v, want Azure legacy operation hint", err)
	}
}

func TestAzureResponsesAcceptsResourceRoot(t *testing.T) {
	t.Parallel()

	err := validateModelEndpoint(providerstore.Entity{
		APIFormat: APIFormatResponses,
		BaseURL:   "https://sample.openai.azure.com/openai/",
	})
	if err != nil {
		t.Fatalf("validateModelEndpoint() error = %v, want resource root accepted", err)
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
				if body["model"] != "model-1" || body["max_tokens"] != float64(providerTestTextMaxTokens) || body["messages"] == nil {
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
				if body["model"] != "model-1" || body["max_output_tokens"] != float64(providerTestResponsesMaxTokens) || body["input"] != "ping" || body["store"] != false {
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
				if body["model"] != "model-1" || body["max_tokens"] != float64(providerTestTextMaxTokens) || body["messages"] == nil {
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
				_ = json.NewEncoder(writer).Encode(probeTestResponse(tc.apiFormat, "pong", ""))
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
			result, err := service.TestModelCapability(ctx, record.Provider, "model-1", "text_output", nil, false)
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
