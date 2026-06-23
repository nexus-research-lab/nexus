package titlegen

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
)

func TestShouldRetryTitleRequest(t *testing.T) {
	t.Parallel()

	if !shouldRetryTitleRequest(context.DeadlineExceeded) {
		t.Fatal("deadline exceeded 应判定为可重试")
	}
	if !shouldRetryTitleRequest(errors.New("Post timeout")) {
		t.Fatal("timeout 文本应判定为可重试")
	}
	if shouldRetryTitleRequest(errors.New("400 bad request")) {
		t.Fatal("业务错误不应判定为可重试")
	}
}

func TestScheduleRetriesTimeoutOnce(t *testing.T) {
	t.Parallel()

	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		attempts++
		if attempts == 1 {
			time.Sleep(1200 * time.Millisecond)
			writer.WriteHeader(http.StatusGatewayTimeout)
			_, _ = writer.Write([]byte(`timeout`))
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"content": []map[string]any{
				{
					"type": "text",
					"text": "重试标题",
				},
			},
		})
	}))
	defer server.Close()

	sessionStore := &fakeSessionService{
		sessions: map[string]*protocol.Session{
			"agent:a:ws:dm:conv_1": {
				SessionKey: "agent:a:ws:dm:conv_1",
				Title:      "New Chat",
			},
		},
	}
	service := NewService(
		&fakeProviderResolver{
			config: &clientopts.RuntimeConfig{
				Provider:  "glm",
				AuthToken: "token-1",
				BaseURL:   server.URL,
				Model:     "glm-5.1",
			},
		},
		sessionStore,
		nil,
		&fakeEventBroadcaster{},
	)
	service.runAsync = func(job func()) { job() }
	service.llmClient.HTTPClient.Timeout = 800 * time.Millisecond

	service.Schedule(context.Background(), Request{
		SessionKey:          "agent:a:ws:dm:conv_1",
		Content:             "给我起一个标题",
		SessionMessageCount: 0,
	})

	if attempts != 2 {
		t.Fatalf("期望重试两次，实际: %d", attempts)
	}
	if got := sessionStore.sessions["agent:a:ws:dm:conv_1"].Title; got != "重试标题" {
		t.Fatalf("重试后标题未更新: %s", got)
	}
}

func TestScheduleDoesNotWarnOnEmptyGeneratedTitle(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"content": []map[string]any{
				{
					"type": "text",
					"text": "   ",
				},
			},
		})
	}))
	defer server.Close()

	sessionStore := &fakeSessionService{
		sessions: map[string]*protocol.Session{
			"agent:a:ws:dm:conv_1": {
				SessionKey: "agent:a:ws:dm:conv_1",
				Title:      "New Chat",
			},
		},
	}
	service := NewService(
		&fakeProviderResolver{
			config: &clientopts.RuntimeConfig{
				Provider:  "glm",
				AuthToken: "token-1",
				BaseURL:   server.URL,
				Model:     "glm-5.1",
			},
		},
		sessionStore,
		nil,
		&fakeEventBroadcaster{},
	)
	var buffer bytes.Buffer
	service.SetLogger(slog.New(slog.NewJSONHandler(&buffer, &slog.HandlerOptions{Level: slog.LevelInfo})))
	service.runAsync = func(job func()) { job() }

	service.Schedule(context.Background(), Request{
		SessionKey:          "agent:a:ws:dm:conv_1",
		Provider:            "glm",
		Content:             "给我起一个标题",
		SessionMessageCount: 0,
	})

	if got := sessionStore.sessions["agent:a:ws:dm:conv_1"].Title; got != "New Chat" {
		t.Fatalf("空标题不应更新 session 标题: %s", got)
	}
	if output := buffer.String(); strings.Contains(output, "生成会话标题失败") {
		t.Fatalf("空标题不应写入 warn 日志: %s", output)
	}
}

func TestResolveRuntimeConfigUsesBackgroundPreference(t *testing.T) {
	providerResolver := &fakeProviderResolver{
		config: &clientopts.RuntimeConfig{
			Provider: "background-provider",
			Model:    "background-model",
		},
	}
	service := NewService(
		providerResolver,
		nil,
		nil,
		nil,
		fakePreferencesService{prefs: preferencessvc.Preferences{
			DefaultBackgroundModelSelection: preferencessvc.ModelSelection{
				Provider: "background-provider",
				Model:    "background-model",
			},
		}},
	)
	config, err := service.resolveLLMConfig(context.Background(), Request{
		OwnerUserID: "user-1",
		Provider:    "agent-provider",
		Model:       "agent-model",
	})
	if err != nil {
		t.Fatalf("解析标题模型失败: %v", err)
	}
	if config.Provider != "background-provider" || config.Model != "background-model" {
		t.Fatalf("未使用后台任务模型: %+v", config)
	}
	if providerResolver.provider != "background-provider" || providerResolver.model != "background-model" {
		t.Fatalf("provider resolver 参数不正确: provider=%s model=%s", providerResolver.provider, providerResolver.model)
	}
}

func TestGenerateTitleSupportsChatCompletions(t *testing.T) {
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

	service := NewService(&fakeProviderResolver{
		config: &clientopts.RuntimeConfig{
			Provider:  "openai",
			AuthToken: "openai-key",
			BaseURL:   server.URL + "/v1",
			Model:     "gpt-4.1-mini",
			APIFormat: "chat_completions",
		},
	}, nil, nil, nil)

	title, err := service.generateTitle(context.Background(), Request{
		Provider: "openai",
		Model:    "gpt-4.1-mini",
	}, "帮我安排一下项目排期")
	if err != nil {
		t.Fatalf("生成标题失败: %v", err)
	}
	if title != "项目排期" {
		t.Fatalf("标题不正确: %s", title)
	}
	if receivedPath != "/v1/chat/completions" {
		t.Fatalf("Chat Completions 请求路径不正确: %s", receivedPath)
	}
	if receivedAuth != "Bearer openai-key" {
		t.Fatalf("Chat Completions 鉴权头不正确: %s", receivedAuth)
	}
	if receivedSystem == "" {
		t.Fatal("Chat Completions 缺少 system prompt")
	}
}

func TestGenerateTitleSupportsResponses(t *testing.T) {
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

	service := NewService(&fakeProviderResolver{
		config: &clientopts.RuntimeConfig{
			Provider:  "openai",
			AuthToken: "openai-key",
			BaseURL:   server.URL + "/v1",
			Model:     "gpt-4.1-mini",
			APIFormat: "responses",
		},
	}, nil, nil, nil)

	title, err := service.generateTitle(context.Background(), Request{
		Provider: "openai",
		Model:    "gpt-4.1-mini",
	}, "整理一下用户需求")
	if err != nil {
		t.Fatalf("生成标题失败: %v", err)
	}
	if title != "需求总结" {
		t.Fatalf("标题不正确: %s", title)
	}
	if receivedPath != "/v1/responses" {
		t.Fatalf("Responses 请求路径不正确: %s", receivedPath)
	}
	if receivedInputCount != 2 {
		t.Fatalf("Responses input 不正确: %d", receivedInputCount)
	}
}
