package titlegen

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
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
	service.client.Timeout = 800 * time.Millisecond

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

type fakeProviderResolver struct {
	config *clientopts.RuntimeConfig
}

func (f *fakeProviderResolver) ResolveRuntimeConfig(_ context.Context, _ string) (*clientopts.RuntimeConfig, error) {
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
