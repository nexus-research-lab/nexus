package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	"github.com/nexus-research-lab/nexus/internal/service/provider"
)

func TestGLM53UsesLowestSupportedReasoningAcrossAPIFormats(t *testing.T) {
	t.Parallel()

	config := &clientopts.RuntimeConfig{Provider: "glm", Model: "glm-5.3", Reasoning: true}
	request := GenerateTextRequest{DisableReasoning: true}
	chatPayload := chatCompletionsRequest{}
	responsesPayload := responsesRequest{}
	anthropicPayload := anthropicMessagesRequest{}
	applyChatCompletionsReasoningDisableOptions(&chatPayload, config, request)
	applyResponsesReasoningDisableOptions(&responsesPayload, config, request)
	applyAnthropicMessagesReasoningDisableOptions(&anthropicPayload, config, request)
	if chatPayload.Thinking["type"] != "enabled" || chatPayload.ReasoningEffort != "low" ||
		responsesPayload.Thinking != nil || responsesPayload.Reasoning != nil ||
		anthropicPayload.Thinking["type"] != "enabled" || anthropicPayload.ReasoningEffort != "low" {
		t.Fatalf("GLM-5.3 应保持思考并使用最低推理强度: chat=%+v responses=%+v anthropic=%+v",
			chatPayload, responsesPayload, anthropicPayload)
	}
}

func TestBuildEndpointNormalizesAzureResponses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		baseURL string
		want    string
		wantErr bool
	}{
		{
			name:    "resource v1",
			baseURL: "https://sample.openai.azure.com/openai/v1",
			want:    "https://sample.openai.azure.com/openai/v1/responses",
		},
		{
			name:    "foundry project",
			baseURL: "https://sample.services.ai.azure.com/api/projects/project-1",
			want:    "https://sample.services.ai.azure.com/api/projects/project-1/openai/v1/responses",
		},
		{
			name:    "chat operation rejected",
			baseURL: "https://sample.openai.azure.com/openai/v1/chat/completions",
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := buildEndpoint(test.baseURL, provider.APIFormatResponses)
			if test.wantErr {
				if err == nil {
					t.Fatalf("buildEndpoint() = %q, want error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("buildEndpoint() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("buildEndpoint() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestGenerateTextDisablesOpenAIReasoningForResponses(t *testing.T) {
	t.Parallel()

	var receivedReasoning map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		defer request.Body.Close()
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("解析请求失败: %v", err)
		}
		if reasoning, ok := payload["reasoning"].(map[string]any); ok {
			receivedReasoning = reasoning
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"output_text": "问候",
		})
	}))
	defer server.Close()

	client := NewClient(server.Client())
	text, err := client.GenerateText(context.Background(), GenerateTextRequest{
		Config: &clientopts.RuntimeConfig{
			Provider:  "openai",
			AuthToken: "openai-key",
			BaseURL:   server.URL + "/v1",
			Model:     "gpt-5.5",
			APIFormat: provider.APIFormatResponses,
			Reasoning: true,
		},
		Messages:         []Message{{Role: "user", Content: "hey"}},
		MaxTokens:        128,
		DisableReasoning: true,
	})
	if err != nil {
		t.Fatalf("生成文本失败: %v", err)
	}
	if text != "问候" {
		t.Fatalf("文本不正确: %s", text)
	}
	if receivedReasoning["effort"] != "none" {
		t.Fatalf("OpenAI Responses 应使用 reasoning.effort=none: %+v", receivedReasoning)
	}
}

func TestGenerateTextSkipsUnsupportedOpenAIReasoningNone(t *testing.T) {
	t.Parallel()

	var hasReasoningEffort bool
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		defer request.Body.Close()
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("解析请求失败: %v", err)
		}
		_, hasReasoningEffort = payload["reasoning_effort"]
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": "问候",
					},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.Client())
	text, err := client.GenerateText(context.Background(), GenerateTextRequest{
		Config: &clientopts.RuntimeConfig{
			Provider:  "openai",
			AuthToken: "openai-key",
			BaseURL:   server.URL + "/v1",
			Model:     "gpt-5-pro",
			APIFormat: provider.APIFormatChatCompletions,
			Reasoning: true,
		},
		Messages:         []Message{{Role: "user", Content: "hey"}},
		MaxTokens:        128,
		DisableReasoning: true,
	})
	if err != nil {
		t.Fatalf("生成文本失败: %v", err)
	}
	if text != "问候" {
		t.Fatalf("文本不正确: %s", text)
	}
	if hasReasoningEffort {
		t.Fatal("不支持 none 的 OpenAI pro 模型不应发送 reasoning_effort=none")
	}
}

func TestGenerateTextRejectsResponsesWithoutText(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": "wrong response shape",
					},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.Client())
	text, err := client.GenerateText(context.Background(), GenerateTextRequest{
		Config: &clientopts.RuntimeConfig{
			Provider:  "openai",
			AuthToken: "openai-key",
			BaseURL:   server.URL + "/v1",
			Model:     "gpt-4.1-mini",
			APIFormat: provider.APIFormatResponses,
		},
		Messages:  []Message{{Role: "user", Content: "整理一下用户需求"}},
		MaxTokens: 32,
	})
	if err == nil || !strings.Contains(err.Error(), "missing text") {
		t.Fatalf("Responses 空文本应失败: text=%q err=%v", text, err)
	}
}

func TestGenerateTextDoesNotExposeChatCompletionsBodyWithoutText(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": "",
					},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.Client())
	text, err := client.GenerateText(context.Background(), GenerateTextRequest{
		Config: &clientopts.RuntimeConfig{
			Provider:  "openai",
			AuthToken: "openai-key",
			BaseURL:   server.URL + "/v1",
			Model:     "gpt-4.1-mini",
			APIFormat: provider.APIFormatChatCompletions,
		},
		Messages:  []Message{{Role: "user", Content: "整理一下用户需求"}},
		MaxTokens: 32,
	})
	if err == nil || !strings.Contains(err.Error(), "chat_completions response missing text") || strings.Contains(err.Error(), `"choices"`) {
		t.Fatalf("Chat Completions 空文本不应泄露响应体: text=%q err=%v", text, err)
	}
}

func TestGenerateTextReportsSafeAnthropicMissingTextMetadata(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"content": []map[string]any{
				{"type": "thinking", "thinking": "private reasoning"},
			},
			"stop_reason": "max_tokens",
			"usage":       map[string]any{"output_tokens": 4096},
		})
	}))
	defer server.Close()

	client := NewClient(server.Client())
	text, err := client.GenerateText(context.Background(), GenerateTextRequest{
		Config: &clientopts.RuntimeConfig{
			Provider:  "glm",
			AuthToken: "glm-key",
			BaseURL:   server.URL,
			Model:     "glm-5.3-flash",
			APIFormat: provider.APIFormatAnthropicMessages,
		},
		Messages:  []Message{{Role: "user", Content: "extract"}},
		MaxTokens: 4096,
	})
	if err == nil ||
		!strings.Contains(err.Error(), `stop_reason="max_tokens"`) ||
		!strings.Contains(err.Error(), "output_tokens=4096") ||
		!strings.Contains(err.Error(), `content_types="thinking"`) ||
		strings.Contains(err.Error(), "private reasoning") {
		t.Fatalf("Anthropic 空文本诊断不安全或不完整: text=%q err=%v", text, err)
	}
}
