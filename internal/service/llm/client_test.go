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

func TestChatCompletionsUsesConfiguredTokenLimitField(t *testing.T) {
	t.Parallel()

	payload := requestPayload(GenerateTextRequest{
		Config: &clientopts.RuntimeConfig{
			Model:                  "production-chat",
			APIFormat:              provider.APIFormatChatCompletions,
			UseMaxCompletionTokens: true,
		},
		Messages:  []Message{{Role: "user", Content: "ping"}},
		MaxTokens: 32,
	})
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("编码 Chat Completions payload 失败: %v", err)
	}
	var decoded map[string]any
	if err = json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("解析 Chat Completions payload 失败: %v", err)
	}
	if decoded["max_completion_tokens"] != float64(32) {
		t.Fatalf("max_completion_tokens 不正确: %+v", decoded)
	}
	if _, exists := decoded["max_tokens"]; exists {
		t.Fatalf("配置 max_completion_tokens 后不应发送 max_tokens: %+v", decoded)
	}
}

func TestGenerateTextDisablesKimiThinkingForSupportedModel(t *testing.T) {
	t.Parallel()

	var receivedThinking map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		defer request.Body.Close()
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("解析请求失败: %v", err)
		}
		if thinking, ok := payload["thinking"].(map[string]any); ok {
			receivedThinking = thinking
		}
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
			Provider:  "kimi",
			AuthToken: "kimi-key",
			BaseURL:   server.URL + "/v1",
			Model:     "kimi-k2.6",
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
	if receivedThinking["type"] != "disabled" {
		t.Fatalf("Kimi 可关闭模型应关闭 thinking: %+v", receivedThinking)
	}
}

func TestGenerateTextSkipsKimiAlwaysThinkingModelDisable(t *testing.T) {
	t.Parallel()

	var hasThinking bool
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		defer request.Body.Close()
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("解析请求失败: %v", err)
		}
		_, hasThinking = payload["thinking"]
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": "代码任务",
					},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.Client())
	text, err := client.GenerateText(context.Background(), GenerateTextRequest{
		Config: &clientopts.RuntimeConfig{
			Provider:  "kimi-code",
			AuthToken: "kimi-key",
			BaseURL:   server.URL + "/coding/v1",
			Model:     "kimi-for-coding",
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
	if text != "代码任务" {
		t.Fatalf("文本不正确: %s", text)
	}
	if hasThinking {
		t.Fatal("Kimi Code always-thinking 模型不应发送 unsupported thinking.disabled")
	}
}

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

func TestGenerateTextDisablesKimiThinkingForAnthropicMessages(t *testing.T) {
	t.Parallel()

	// 复现事故：kimi-k2.6 走 anthropic_messages 时 thinking 从未关闭，
	// 128 token 全被推理吃光、正文为空触发 max_tokens 截断。
	var receivedThinking map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		defer request.Body.Close()
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("解析请求失败: %v", err)
		}
		if thinking, ok := payload["thinking"].(map[string]any); ok {
			receivedThinking = thinking
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": "问候"},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.Client())
	text, err := client.GenerateText(context.Background(), GenerateTextRequest{
		Config: &clientopts.RuntimeConfig{
			Provider:  "kimi-k2-6",
			AuthToken: "kimi-key",
			BaseURL:   server.URL,
			Model:     "kimi-k2.6",
			APIFormat: provider.APIFormatAnthropicMessages,
			Reasoning: true,
		},
		Messages:         []Message{{Role: "user", Content: "hey"}},
		MaxTokens:        1024,
		DisableReasoning: true,
	})
	if err != nil {
		t.Fatalf("生成文本失败: %v", err)
	}
	if text != "问候" {
		t.Fatalf("文本不正确: %s", text)
	}
	if receivedThinking["type"] != "disabled" {
		t.Fatalf("Kimi anthropic_messages 可关闭模型应关闭 thinking: %+v", receivedThinking)
	}
}

func TestGenerateTextDisablesQwenThinkingForAnthropicMessages(t *testing.T) {
	t.Parallel()

	// Qwen/DashScope 系即便走 anthropic_messages 兼容端点，也用 enable_thinking=false
	// 而非 thinking.type=disabled；关闭方式必须按 provider 家族分派。
	var receivedEnableThinking any
	var hasThinking bool
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		defer request.Body.Close()
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("解析请求失败: %v", err)
		}
		receivedEnableThinking = payload["enable_thinking"]
		_, hasThinking = payload["thinking"]
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": "问候"},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.Client())
	text, err := client.GenerateText(context.Background(), GenerateTextRequest{
		Config: &clientopts.RuntimeConfig{
			Provider:  "qwen-token-plan",
			AuthToken: "qwen-key",
			BaseURL:   server.URL + "/apps/anthropic",
			Model:     "qwen3-coder-plus",
			APIFormat: provider.APIFormatAnthropicMessages,
			Reasoning: true,
		},
		Messages:         []Message{{Role: "user", Content: "hey"}},
		MaxTokens:        1024,
		DisableReasoning: true,
	})
	if err != nil {
		t.Fatalf("生成文本失败: %v", err)
	}
	if text != "问候" {
		t.Fatalf("文本不正确: %s", text)
	}
	if enable, ok := receivedEnableThinking.(bool); !ok || enable {
		t.Fatalf("Qwen anthropic_messages 应发送 enable_thinking=false: %v", receivedEnableThinking)
	}
	if hasThinking {
		t.Fatal("Qwen anthropic_messages 不应发送 thinking.type=disabled")
	}
}

func TestGenerateTextDisablesDashScopeThinkingForChatCompletions(t *testing.T) {
	t.Parallel()

	var receivedEnableThinking any
	var hasThinking bool
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		defer request.Body.Close()
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("解析请求失败: %v", err)
		}
		receivedEnableThinking = payload["enable_thinking"]
		_, hasThinking = payload["thinking"]
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
			Provider:  "dashscope",
			AuthToken: "dashscope-key",
			BaseURL:   server.URL + "/compatible-mode/v1",
			Model:     "qwen3-235b-a22b",
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
	if receivedEnableThinking != false {
		t.Fatalf("DashScope 应使用 enable_thinking=false: %#v", receivedEnableThinking)
	}
	if hasThinking {
		t.Fatal("DashScope 不应发送 GLM/Kimi thinking 字段")
	}
}

func TestGenerateTextDisablesLocalQwenThinkingForChatCompletions(t *testing.T) {
	t.Parallel()

	var receivedKwargs map[string]any
	var hasEnableThinking bool
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		defer request.Body.Close()
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("解析请求失败: %v", err)
		}
		if kwargs, ok := payload["chat_template_kwargs"].(map[string]any); ok {
			receivedKwargs = kwargs
		}
		_, hasEnableThinking = payload["enable_thinking"]
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": "本地问候",
					},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.Client())
	text, err := client.GenerateText(context.Background(), GenerateTextRequest{
		Config: &clientopts.RuntimeConfig{
			Provider:  "vllm",
			AuthToken: "empty",
			BaseURL:   server.URL + "/v1",
			Model:     "Qwen/Qwen3-32B",
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
	if text != "本地问候" {
		t.Fatalf("文本不正确: %s", text)
	}
	if receivedKwargs["enable_thinking"] != false {
		t.Fatalf("本地 Qwen 应使用 chat_template_kwargs.enable_thinking=false: %+v", receivedKwargs)
	}
	if hasEnableThinking {
		t.Fatal("本地 Qwen 不应发送 DashScope enable_thinking 顶层字段")
	}
}

func TestGenerateTextDisablesOpenAIReasoningForChatCompletions(t *testing.T) {
	t.Parallel()

	var receivedReasoningEffort string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		defer request.Body.Close()
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("解析请求失败: %v", err)
		}
		receivedReasoningEffort = stringValue(payload["reasoning_effort"])
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
			Model:     "gpt-5.5",
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
	if receivedReasoningEffort != "none" {
		t.Fatalf("OpenAI GPT-5.1+ 应使用 reasoning_effort=none: %s", receivedReasoningEffort)
	}
}

func TestApplyHeadersIncludesAzureAPIKey(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodPost, "https://sample.openai.azure.com/openai/v1/responses", nil)
	applyHeaders(request, &clientopts.RuntimeConfig{
		AuthToken: "azure-key",
		BaseURL:   "https://sample.openai.azure.com/openai/v1",
		APIFormat: provider.APIFormatResponses,
	})
	if got := request.Header.Get("api-key"); got != "azure-key" {
		t.Fatalf("Azure api-key 不正确: %q", got)
	}
	if got := request.Header.Get("Authorization"); got != "Bearer azure-key" {
		t.Fatalf("Azure Authorization 不正确: %q", got)
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

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}
