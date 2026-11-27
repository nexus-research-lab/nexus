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
