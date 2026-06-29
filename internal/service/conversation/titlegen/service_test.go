package titlegen

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
)

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

func TestScheduleReplacesGoalFallbackTitle(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"content": []map[string]any{
				{
					"type": "text",
					"text": "Knip 清理",
				},
			},
		})
	}))
	defer server.Close()

	roomStore := &fakeRoomService{
		contexts: map[string]*protocol.ConversationContextAggregate{
			"conv_1": {
				Room: protocol.RoomRecord{
					ID:   "room_1",
					Name: "协作房间",
				},
				Conversation: protocol.ConversationRecord{
					ID:    "conv_1",
					Title: "Knip Until Clean",
				},
			},
		},
	}
	service := NewService(
		&fakeProviderResolver{
			config: &clientopts.RuntimeConfig{
				Provider:  "kimi",
				AuthToken: "token-1",
				BaseURL:   server.URL,
				Model:     "kimi-k2.5",
			},
		},
		nil,
		roomStore,
		&fakeEventBroadcaster{},
	)
	service.runAsync = func(job func()) {
		job()
	}

	service.Schedule(context.Background(), Request{
		SessionKey:               "room:group:conv_1",
		Content:                  "按 Loop「Knip Until Clean」推进这个 Room Goal。",
		FallbackTitle:            "Knip Until Clean",
		SessionMessageCount:      -1,
		ConversationID:           "conv_1",
		ConversationRoomID:       "room_1",
		ConversationMessageCount: 0,
	})

	if got := roomStore.contexts["conv_1"].Conversation.Title; got != "Knip 清理" {
		t.Fatalf("conversation title = %q, want generated title", got)
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

func TestScheduleUpdatesSessionWhenConversationTitleIsCustom(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"content": []map[string]any{
				{
					"type": "text",
					"text": "会议纪要",
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
					Name: "协作房间",
				},
				Conversation: protocol.ConversationRecord{
					ID:    "conv_1",
					Title: "用户自定义话题",
				},
			},
		},
	}
	service := NewService(
		&fakeProviderResolver{
			config: &clientopts.RuntimeConfig{
				Provider:  "kimi",
				AuthToken: "token-1",
				BaseURL:   server.URL,
				Model:     "kimi-k2.5",
			},
		},
		sessionStore,
		roomStore,
		&fakeEventBroadcaster{},
	)
	service.runAsync = func(job func()) {
		job()
	}

	service.Schedule(context.Background(), Request{
		SessionKey:               "agent:a:ws:dm:conv_1",
		Content:                  "帮我整理这次产品会议的纪要",
		SessionTitle:             "New Chat",
		SessionMessageCount:      0,
		ConversationID:           "conv_1",
		ConversationRoomID:       "room_1",
		ConversationTitle:        "用户自定义话题",
		ConversationRoomName:     "协作房间",
		ConversationMessageCount: 0,
	})

	if got := sessionStore.sessions["agent:a:ws:dm:conv_1"].Title; got != "会议纪要" {
		t.Fatalf("session title = %q, want generated title", got)
	}
	if got := roomStore.contexts["conv_1"].Conversation.Title; got != "用户自定义话题" {
		t.Fatalf("conversation title should keep custom value: %q", got)
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

func TestScheduleUpdatesLocalizedDefaultSessionTitle(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"content": []map[string]any{
				{
					"type": "text",
					"text": "钉钉答疑",
				},
			},
		})
	}))
	defer server.Close()

	const sessionKey = "agent:a:dingtalk:dm:dt-user-1"
	sessionStore := &fakeSessionService{
		sessions: map[string]*protocol.Session{
			sessionKey: {
				SessionKey: sessionKey,
				Title:      "未命名会话",
			},
		},
	}
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
		&fakeEventBroadcaster{},
	)
	service.runAsync = func(job func()) {
		job()
	}

	service.Schedule(context.Background(), Request{
		SessionKey:          sessionKey,
		Content:             "怎么配置钉钉",
		SessionTitle:        "未命名会话",
		SessionMessageCount: 3,
	})

	if got := sessionStore.sessions[sessionKey].Title; got != "钉钉答疑" {
		t.Fatalf("中文默认标题应被模型标题覆盖: %s", got)
	}
}
