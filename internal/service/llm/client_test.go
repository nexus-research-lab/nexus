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

func TestGenerateTextSupportsAnthropicMessages(t *testing.T) {
	t.Parallel()

	var receivedPath string
	var receivedKey string
	var receivedModel string
	var receivedContent string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedPath = request.URL.Path
		receivedKey = request.Header.Get("x-api-key")
		defer request.Body.Close()
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("解析请求失败: %v", err)
		}
		receivedModel = stringValue(payload["model"])
		messages, _ := payload["messages"].([]any)
		if len(messages) > 0 {
			if firstMessage, ok := messages[0].(map[string]any); ok {
				receivedContent = stringValue(firstMessage["content"])
			}
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"content": []map[string]any{
				{
					"type": "text",
					"text": "天气问答",
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.Client())
	text, err := client.GenerateText(context.Background(), GenerateTextRequest{
		Config: &clientopts.RuntimeConfig{
			Provider:  "kimi",
			AuthToken: "token-1",
			BaseURL:   server.URL + "/anthropic",
			Model:     "kimi-k2.5",
			APIFormat: provider.APIFormatAnthropicMessages,
		},
		System:    "system",
		Messages:  []Message{{Role: "user", Content: "今天天气怎么样呀"}},
		MaxTokens: 32,
	})
	if err != nil {
		t.Fatalf("生成文本失败: %v", err)
	}
	if text != "天气问答" {
		t.Fatalf("文本不正确: %s", text)
	}
	if receivedPath != "/anthropic/v1/messages" {
		t.Fatalf("Anthropic Messages 请求路径不正确: %s", receivedPath)
	}
	if receivedKey != "token-1" {
		t.Fatalf("Anthropic Messages 鉴权头不正确: %s", receivedKey)
	}
	if receivedModel != "kimi-k2.5" || receivedContent != "今天天气怎么样呀" {
		t.Fatalf("Anthropic Messages payload 不正确: model=%s content=%s", receivedModel, receivedContent)
	}
}

func TestGenerateTextSupportsChatCompletions(t *testing.T) {
	t.Parallel()

	var receivedPath string
	var receivedAuth string
	var receivedSystem string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedPath = request.URL.Path
		receivedAuth = request.Header.Get("Authorization")
		defer request.Body.Close()
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("解析请求失败: %v", err)
		}
		messages, _ := payload["messages"].([]any)
		if len(messages) > 0 {
			if firstMessage, ok := messages[0].(map[string]any); ok {
				receivedSystem = stringValue(firstMessage["content"])
			}
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": "项目排期",
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
		System:    "system",
		Messages:  []Message{{Role: "user", Content: "帮我安排一下项目排期"}},
		MaxTokens: 32,
	})
	if err != nil {
		t.Fatalf("生成文本失败: %v", err)
	}
	if text != "项目排期" {
		t.Fatalf("文本不正确: %s", text)
	}
	if receivedPath != "/v1/chat/completions" {
		t.Fatalf("Chat Completions 请求路径不正确: %s", receivedPath)
	}
	if receivedAuth != "Bearer openai-key" {
		t.Fatalf("Chat Completions 鉴权头不正确: %s", receivedAuth)
	}
	if receivedSystem != "system" {
		t.Fatalf("Chat Completions system prompt 不正确: %s", receivedSystem)
	}
}

func TestGenerateTextDisablesGLMThinkingForChatCompletions(t *testing.T) {
	t.Parallel()

	var receivedThinking map[string]any
	var receivedMaxTokens float64
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		defer request.Body.Close()
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("解析请求失败: %v", err)
		}
		if thinking, ok := payload["thinking"].(map[string]any); ok {
			receivedThinking = thinking
		}
		if value, ok := payload["max_tokens"].(float64); ok {
			receivedMaxTokens = value
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
			Provider:    "glm",
			DisplayName: "GLM Coding Plan",
			AuthToken:   "glm-key",
			BaseURL:     server.URL + "/api/coding/paas/v4",
			Model:       "glm-4.5-air",
			APIFormat:   provider.APIFormatChatCompletions,
			Reasoning:   true,
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
		t.Fatalf("GLM 标题请求应关闭 thinking: %+v", receivedThinking)
	}
	if receivedMaxTokens != 128 {
		t.Fatalf("max_tokens 不正确: %v", receivedMaxTokens)
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

func TestGenerateTextDoesNotSendThinkingForGenericChatCompletions(t *testing.T) {
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
						"content": "项目排期",
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
			AuthToken: "openai-key",
			BaseURL:   server.URL + "/v1",
			Model:     "local-model",
			APIFormat: provider.APIFormatChatCompletions,
		},
		Messages:         []Message{{Role: "user", Content: "帮我安排一下项目排期"}},
		MaxTokens:        128,
		DisableReasoning: true,
	})
	if err != nil {
		t.Fatalf("生成文本失败: %v", err)
	}
	if text != "项目排期" {
		t.Fatalf("文本不正确: %s", text)
	}
	if hasThinking {
		t.Fatal("普通 Chat Completions 请求不应带 GLM thinking 字段")
	}
}

func TestGenerateTextSupportsResponses(t *testing.T) {
	t.Parallel()

	var receivedPath string
	var receivedInputCount int
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedPath = request.URL.Path
		defer request.Body.Close()
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("解析请求失败: %v", err)
		}
		input, _ := payload["input"].([]any)
		receivedInputCount = len(input)
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"output_text": "需求总结",
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
		System:    "system",
		Messages:  []Message{{Role: "user", Content: "整理一下用户需求"}},
		MaxTokens: 32,
	})
	if err != nil {
		t.Fatalf("生成文本失败: %v", err)
	}
	if text != "需求总结" {
		t.Fatalf("文本不正确: %s", text)
	}
	if receivedPath != "/v1/responses" {
		t.Fatalf("Responses 请求路径不正确: %s", receivedPath)
	}
	if receivedInputCount != 2 {
		t.Fatalf("Responses input 不正确: %d", receivedInputCount)
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

func TestGenerateTextReportsChatCompletionsBodyWithoutText(t *testing.T) {
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
	if err == nil || !strings.Contains(err.Error(), "chat_completions response missing text") || !strings.Contains(err.Error(), `"choices"`) {
		t.Fatalf("Chat Completions 空文本应带响应体失败: text=%q err=%v", text, err)
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
