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

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}
