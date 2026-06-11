package titlegen

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
)

var errTestNotFound = errors.New("not found")

func TestScheduleUpdatesSessionAndConversationTitle(t *testing.T) {
	t.Parallel()

	var receivedPath string
	var receivedModel string
	var receivedContent string
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		receivedPath = request.URL.Path
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

	sessionStore := &fakeSessionService{
		sessions: map[string]*protocol.Session{
			"agent:a:ws:dm:conv_1": {
				SessionKey: "agent:a:ws:dm:conv_1",
				Title:      "New Chat",
			},
		},
	}
	roomStore := &fakeRoomService{
		contexts: map[string]*protocol.ConversationContextAggregate{
			"conv_1": {
				Room: protocol.RoomRecord{
					ID:   "room_1",
					Name: "Amy",
				},
				Conversation: protocol.ConversationRecord{
					ID:    "conv_1",
					Title: "Amy",
				},
			},
		},
	}
	events := &fakeEventBroadcaster{}
	service := NewService(
		&fakeProviderResolver{
			config: &clientopts.RuntimeConfig{
				Provider:  "kimi",
				AuthToken: "token-1",
				BaseURL:   server.URL + "/anthropic",
				Model:     "kimi-k2.5",
			},
		},
		sessionStore,
		roomStore,
		events,
	)
	service.runAsync = func(job func()) {
		job()
	}

	service.Schedule(context.Background(), Request{
		SessionKey:               "agent:a:ws:dm:conv_1",
		Content:                  "今天天气怎么样呀",
		SessionTitle:             "New Chat",
		SessionMessageCount:      0,
		ConversationID:           "conv_1",
		ConversationRoomID:       "room_1",
		ConversationTitle:        "Amy",
		ConversationRoomName:     "Amy",
		ConversationMessageCount: 0,
	})

	if receivedPath != "/anthropic/v1/messages" {
		t.Fatalf("标题请求路径不正确: %s", receivedPath)
	}
	if receivedModel != "kimi-k2.5" {
		t.Fatalf("标题请求模型不正确: %s", receivedModel)
	}
	if receivedContent != "今天天气怎么样呀" {
		t.Fatalf("标题请求内容不正确: %s", receivedContent)
	}
	if got := sessionStore.sessions["agent:a:ws:dm:conv_1"].Title; got != "天气问答" {
		t.Fatalf("session 标题未更新: %s", got)
	}
	if got := roomStore.contexts["conv_1"].Conversation.Title; got != "天气问答" {
		t.Fatalf("conversation 标题未更新: %s", got)
	}
	if len(events.events) != 1 {
		t.Fatalf("期望广播 1 条 resync 事件，实际: %d", len(events.events))
	}
	if events.events[0].EventType != protocol.EventTypeSessionResyncRequired {
		t.Fatalf("事件类型不正确: %+v", events.events[0])
	}
}

func TestScheduleSkipsNonDefaultTitles(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"content": []map[string]any{
				{
					"type": "text",
					"text": "不会生效",
				},
			},
		})
	}))
	defer server.Close()

	sessionStore := &fakeSessionService{
		sessions: map[string]*protocol.Session{
			"agent:a:ws:dm:conv_1": {
				SessionKey: "agent:a:ws:dm:conv_1",
				Title:      "用户自定义标题",
			},
		},
	}
	roomStore := &fakeRoomService{
		contexts: map[string]*protocol.ConversationContextAggregate{
			"conv_1": {
				Room: protocol.RoomRecord{
					ID:   "room_1",
					Name: "Amy",
				},
				Conversation: protocol.ConversationRecord{
					ID:    "conv_1",
					Title: "用户自定义标题",
				},
			},
		},
	}
	events := &fakeEventBroadcaster{}
	service := NewService(
		&fakeProviderResolver{
			config: &clientopts.RuntimeConfig{
				Provider:  "glm",
				AuthToken: "token-2",
				BaseURL:   server.URL,
				Model:     "glm-5.1",
			},
		},
		sessionStore,
		roomStore,
		events,
	)
	service.runAsync = func(job func()) {
		job()
	}

	service.Schedule(context.Background(), Request{
		SessionKey:               "agent:a:ws:dm:conv_1",
		Content:                  "给这次聊天起个标题",
		SessionTitle:             "用户自定义标题",
		SessionMessageCount:      0,
		ConversationID:           "conv_1",
		ConversationRoomID:       "room_1",
		ConversationTitle:        "用户自定义标题",
		ConversationRoomName:     "Amy",
		ConversationMessageCount: 0,
	})

	if len(events.events) != 0 {
		t.Fatalf("非默认标题不应广播 resync: %+v", events.events)
	}
	if got := sessionStore.sessions["agent:a:ws:dm:conv_1"].Title; got != "用户自定义标题" {
		t.Fatalf("session 标题不应被覆盖: %s", got)
	}
	if got := roomStore.contexts["conv_1"].Conversation.Title; got != "用户自定义标题" {
		t.Fatalf("conversation 标题不应被覆盖: %s", got)
	}
}

func TestScheduleUpdatesDefaultSessionTitleAfterInitialMessage(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"content": []map[string]any{
				{
					"type": "text",
					"text": "微信问候",
				},
			},
		})
	}))
	defer server.Close()

	sessionStore := &fakeSessionService{
		sessions: map[string]*protocol.Session{
			"agent:a:weixin-personal:dm:wx-user-1": {
				SessionKey: "agent:a:weixin-personal:dm:wx-user-1",
				Title:      "New Chat",
			},
		},
	}
	events := &fakeEventBroadcaster{}
	service := NewService(
		&fakeProviderResolver{
			config: &clientopts.RuntimeConfig{
				Provider:  "glm",
				AuthToken: "token-2",
				BaseURL:   server.URL,
				Model:     "glm-5.1",
			},
		},
		sessionStore,
		nil,
		events,
	)
	service.runAsync = func(job func()) {
		job()
	}

	service.Schedule(context.Background(), Request{
		SessionKey:          "agent:a:weixin-personal:dm:wx-user-1",
		Content:             "你好",
		SessionTitle:        "New Chat",
		SessionMessageCount: 8,
	})

	if got := sessionStore.sessions["agent:a:weixin-personal:dm:wx-user-1"].Title; got != "微信问候" {
		t.Fatalf("默认标题的历史 session 应继续补生成标题: %s", got)
	}
	if len(events.events) == 0 {
		t.Fatal("标题更新后应广播 resync")
	}
}

func TestFillEmptyPreviewFromGoalUpdatesDefaultSessionTitle(t *testing.T) {
	t.Parallel()

	sessionStore := &fakeSessionService{
		sessions: map[string]*protocol.Session{
			"agent:a:ws:dm:conv_1": {
				SessionKey: "agent:a:ws:dm:conv_1",
				Title:      "New Chat",
			},
		},
	}
	events := &fakeEventBroadcaster{}
	service := NewService(nil, sessionStore, nil, events)

	if err := service.FillEmptyPreviewFromGoal(context.Background(), "agent:a:ws:dm:conv_1", "Ship Goal mode"); err != nil {
		t.Fatalf("FillEmptyPreviewFromGoal() error = %v", err)
	}

	if got := sessionStore.sessions["agent:a:ws:dm:conv_1"].Title; got != "Ship Goal mode" {
		t.Fatalf("session title = %q, want goal objective", got)
	}
	if len(events.events) != 1 || events.events[0].EventType != protocol.EventTypeSessionResyncRequired {
		t.Fatalf("events = %#v, want session_resync_required", events.events)
	}
}

func TestFillEmptyPreviewFromGoalUpdatesDefaultRoomConversationTitle(t *testing.T) {
	t.Parallel()

	roomStore := &fakeRoomService{
		contexts: map[string]*protocol.ConversationContextAggregate{
			"conv_1": {
				Room: protocol.RoomRecord{
					ID:   "room_1",
					Name: "协作房间",
				},
				Conversation: protocol.ConversationRecord{
					ID:    "conv_1",
					Title: "协作房间",
				},
			},
		},
	}
	events := &fakeEventBroadcaster{}
	service := NewService(nil, nil, roomStore, events)

	if err := service.FillEmptyPreviewFromGoal(context.Background(), "room:group:conv_1", "完成 Room Goal"); err != nil {
		t.Fatalf("FillEmptyPreviewFromGoal() error = %v", err)
	}

	if got := roomStore.contexts["conv_1"].Conversation.Title; got != "完成 Room Goal" {
		t.Fatalf("conversation title = %q, want goal objective", got)
	}
	if len(events.events) != 1 || events.events[0].Data["room_id"] != "room_1" || events.events[0].Data["conversation_id"] != "conv_1" {
		t.Fatalf("events = %#v, want room conversation resync", events.events)
	}
}

func TestFillEmptyPreviewFromGoalSkipsNonDefaultTitles(t *testing.T) {
	t.Parallel()

	sessionStore := &fakeSessionService{
		sessions: map[string]*protocol.Session{
			"agent:a:ws:dm:conv_1": {
				SessionKey: "agent:a:ws:dm:conv_1",
				Title:      "已有标题",
			},
		},
	}
	service := NewService(nil, sessionStore, nil, &fakeEventBroadcaster{})

	if err := service.FillEmptyPreviewFromGoal(context.Background(), "agent:a:ws:dm:conv_1", "Ship Goal mode"); err != nil {
		t.Fatalf("FillEmptyPreviewFromGoal() error = %v", err)
	}

	if got := sessionStore.sessions["agent:a:ws:dm:conv_1"].Title; got != "已有标题" {
		t.Fatalf("session title = %q, want unchanged non-default title", got)
	}
}

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

type fakePreferencesService struct {
	prefs preferencessvc.Preferences
}

func (f fakePreferencesService) Get(_ context.Context, _ string) (preferencessvc.Preferences, error) {
	return f.prefs, nil
}

type fakeProviderResolver struct {
	config   *clientopts.RuntimeConfig
	provider string
	model    string
}

func (f *fakeProviderResolver) ResolveLLMConfig(
	_ context.Context,
	provider string,
	model string,
) (*clientopts.RuntimeConfig, error) {
	f.provider = provider
	f.model = model
	return f.config, nil
}

type fakeSessionService struct {
	sessions map[string]*protocol.Session
}

func (f *fakeSessionService) GetSession(_ context.Context, sessionKey string) (*protocol.Session, error) {
	item := f.sessions[sessionKey]
	if item == nil {
		return nil, errTestNotFound
	}
	value := *item
	return &value, nil
}

func (f *fakeSessionService) UpdateSessionTitle(_ context.Context, sessionKey string, title string) (*protocol.Session, error) {
	item := f.sessions[sessionKey]
	if item == nil {
		return nil, errTestNotFound
	}
	item.Title = title
	value := *item
	return &value, nil
}

type fakeRoomService struct {
	contexts map[string]*protocol.ConversationContextAggregate
}

func (f *fakeRoomService) GetConversationContext(_ context.Context, conversationID string) (*protocol.ConversationContextAggregate, error) {
	item := f.contexts[conversationID]
	if item == nil {
		return nil, errTestNotFound
	}
	value := *item
	return &value, nil
}

func (f *fakeRoomService) UpdateConversationTitle(
	_ context.Context,
	_ string,
	conversationID string,
	title string,
) (*protocol.ConversationContextAggregate, error) {
	item := f.contexts[conversationID]
	if item == nil {
		return nil, errTestNotFound
	}
	item.Conversation.Title = title
	value := *item
	return &value, nil
}

type fakeEventBroadcaster struct {
	events []protocol.EventMessage
}

func (f *fakeEventBroadcaster) BroadcastEvent(_ context.Context, _ string, event protocol.EventMessage) []error {
	f.events = append(f.events, event)
	return nil
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	default:
		return ""
	}
}
