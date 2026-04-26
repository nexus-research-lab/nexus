package room_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	agentsvc "github.com/nexus-research-lab/nexus/internal/agent"
	authsvc "github.com/nexus-research-lab/nexus/internal/auth"
	"github.com/nexus-research-lab/nexus/internal/bootstrap"
	permissionctx "github.com/nexus-research-lab/nexus/internal/permission"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	providercfg "github.com/nexus-research-lab/nexus/internal/provider"
	roomsvc "github.com/nexus-research-lab/nexus/internal/room"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/session"
	workspace2 "github.com/nexus-research-lab/nexus/internal/storage/workspace"
	usagesvc "github.com/nexus-research-lab/nexus/internal/usage"

	_ "github.com/mattn/go-sqlite3"
	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

type ChatRequest = roomsvc.ChatRequest
type InterruptRequest = roomsvc.InterruptRequest

var NewRealtimeServiceWithFactory = roomsvc.NewRealtimeServiceWithFactory

type fakeRoomClient struct {
	mu             sync.Mutex
	sessionID      string
	messages       chan sdkprotocol.ReceivedMessage
	interruptCalls int
	sentContents   []string
	onQuery        func(context.Context, string) error
	onInterrupt    func(context.Context)
}

func newFakeRoomClient() *fakeRoomClient {
	return &fakeRoomClient{
		sessionID: "room-sdk-session",
		messages:  make(chan sdkprotocol.ReceivedMessage, 32),
	}
}

func (c *fakeRoomClient) Connect(context.Context) error { return nil }

func (c *fakeRoomClient) Query(ctx context.Context, prompt string) error {
	if c.onQuery != nil {
		return c.onQuery(ctx, prompt)
	}
	return nil
}

func (c *fakeRoomClient) ReceiveMessages(context.Context) <-chan sdkprotocol.ReceivedMessage {
	return c.messages
}

func (c *fakeRoomClient) SendContent(_ context.Context, content any, _ *string, _ string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if text, ok := content.(string); ok {
		c.sentContents = append(c.sentContents, text)
	}
	return nil
}

func (c *fakeRoomClient) Interrupt(ctx context.Context) error {
	c.mu.Lock()
	c.interruptCalls++
	callback := c.onInterrupt
	c.mu.Unlock()
	if callback != nil {
		callback(ctx)
	}
	return nil
}

func (c *fakeRoomClient) Disconnect(context.Context) error { return nil }

func (c *fakeRoomClient) Reconfigure(context.Context, agentclient.Options) error {
	return nil
}

func (c *fakeRoomClient) SessionID() string { return c.sessionID }

type fakeRoomFactory struct {
	mu      sync.Mutex
	clients []*fakeRoomClient
	index   int
	options []agentclient.Options
}

func (f *fakeRoomFactory) New(options agentclient.Options) runtimectx.Client {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.options = append(f.options, options)
	if f.index >= len(f.clients) {
		return newFakeRoomClient()
	}
	client := f.clients[f.index]
	f.index++
	return client
}

func (f *fakeRoomFactory) LastOptions() agentclient.Options {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.options) == 0 {
		return agentclient.Options{}
	}
	return f.options[len(f.options)-1]
}

func sendFakeAssistantResult(client *fakeRoomClient, messageID string, text string) {
	sendFakeAssistantResultWithUsage(client, messageID, text, nil)
}

func sendFakeAssistantResultWithUsage(client *fakeRoomClient, messageID string, text string, usage map[string]any) {
	client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeAssistant,
		SessionID: client.sessionID,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				ID:    messageID,
				Model: "sonnet",
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.TextBlock{Text: text},
				},
			},
		},
	}
	client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeResult,
		SessionID: client.sessionID,
		UUID:      messageID + "-result",
		Result: &sdkprotocol.ResultMessage{
			Subtype:    "success",
			DurationMS: 1,
			NumTurns:   1,
			Result:     "done",
			Usage:      usage,
		},
	}
}

type realtimeTestSender struct {
	key    string
	events chan protocol.EventMessage
}

func newRealtimeTestSender(key string) *realtimeTestSender {
	return &realtimeTestSender{
		key:    key,
		events: make(chan protocol.EventMessage, 64),
	}
}

func (s *realtimeTestSender) Key() string    { return s.key }
func (s *realtimeTestSender) IsClosed() bool { return false }
func (s *realtimeTestSender) SendEvent(_ context.Context, event protocol.EventMessage) error {
	s.events <- event
	return nil
}

func TestRealtimeServiceHandleChatWithDirectRoomFallbackTarget(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := bootstrap.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := bootstrap.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}

	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "user-room-usage",
		Username: "room-owner",
		Role:     authsvc.RoleOwner,
	})
	memberAgent := createTestAgent(t, agentService, ctx, "单聊助手")
	dmContext, err := roomService.EnsureDirectRoom(ctx, memberAgent.AgentID)
	if err != nil {
		t.Fatalf("创建直聊 room 失败: %v", err)
	}

	client := newFakeRoomClient()
	client.onQuery = func(_ context.Context, _ string) error {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeAssistant,
				SessionID: client.sessionID,
				Assistant: &sdkprotocol.AssistantMessage{
					Message: sdkprotocol.ConversationEnvelope{
						ID:    "assistant-sdk-1",
						Model: "sonnet",
						Content: []sdkprotocol.ContentBlock{
							sdkprotocol.TextBlock{Text: "已收到，正在处理。"},
						},
					},
				},
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-sdk-1",
				Result: &sdkprotocol.ResultMessage{
					Subtype:       "success",
					DurationMS:    15,
					DurationAPIMS: 11,
					NumTurns:      1,
					Result:        "done",
					Usage: map[string]any{
						"input_tokens":  3,
						"output_tokens": 5,
					},
				},
			}
		}()
		return nil
	}

	permission := permissionctx.NewContext()
	runtimeManager := runtimectx.NewManager()
	factory := &fakeRoomFactory{clients: []*fakeRoomClient{client}}
	service := roomsvc.NewRealtimeServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimeManager,
		permission,
		factory,
	)
	usageService := usagesvc.NewServiceWithDB(cfg, db)
	service.SetUsageRecorder(usageService)
	roomHistory := workspace2.NewRoomHistoryStore(cfg.WorkspacePath)

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(dmContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-1")
	permission.BindSession(sharedSessionKey, sender, "client-1", true)

	if err = service.HandleChat(ctx, roomsvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         dmContext.Room.ID,
		ConversationID: dmContext.Conversation.ID,
		Content:        "你好",
		RoundID:        "room-round-1",
		ReqID:          "room-round-1",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	events := collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	assertRoomEventTypes(t, events, []protocol.EventType{
		protocol.EventTypeMessage,
		protocol.EventTypeRoundStatus,
		protocol.EventTypeChatAck,
		protocol.EventTypeSessionStatus,
		protocol.EventTypeStreamStart,
		protocol.EventTypeMessage,
		protocol.EventTypeMessage,
		protocol.EventTypeStreamEnd,
		protocol.EventTypeRoundStatus,
	})

	pendingMsgID := ""
	for _, event := range events {
		if event.EventType == protocol.EventTypeChatAck {
			pending, _ := event.Data["pending"].([]map[string]any)
			if len(pending) == 0 {
				rawPending, _ := event.Data["pending"].([]any)
				if len(rawPending) > 0 {
					if payload, ok := rawPending[0].(map[string]any); ok {
						pendingMsgID = normalizePendingValue(payload["msg_id"])
					}
				}
			} else {
				pendingMsgID = normalizePendingValue(pending[0]["msg_id"])
			}
		}
		if event.EventType == protocol.EventTypeMessage && event.MessageID == "assistant-sdk-1" {
			if pendingMsgID == "" {
				t.Fatal("未拿到 pending slot msg_id")
			}
			if event.MessageID == pendingMsgID {
				t.Fatalf("assistant message_id 不应回退成 slot msg_id: %s", pendingMsgID)
			}
		}
	}

	privateSessionKey := protocol.BuildRoomAgentSessionKey(dmContext.Conversation.ID, memberAgent.AgentID, dmContext.Room.RoomType)
	roomTranscriptBaseTime := time.Now().Add(-2 * time.Second).UTC()
	writeRoomTranscriptFixture(t, memberAgent.WorkspacePath, client.sessionID, []map[string]any{
		{
			"type":      "user",
			"uuid":      "room-user-1",
			"sessionId": client.sessionID,
			"timestamp": roomTranscriptBaseTime.Format(time.RFC3339Nano),
			"message": map[string]any{
				"role":    "user",
				"content": "dispatch prompt",
			},
		},
		{
			"type":       "assistant",
			"uuid":       "assistant-sdk-1",
			"sessionId":  client.sessionID,
			"parentUuid": "room-user-1",
			"timestamp":  roomTranscriptBaseTime.Add(200 * time.Millisecond).Format(time.RFC3339Nano),
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "text", "text": "已收到，正在处理。"},
				},
			},
		},
	})
	sharedMessages, err := roomHistory.ReadMessages(dmContext.Conversation.ID, nil)
	if err != nil {
		t.Fatalf("读取共享 Room 消息失败: %v", err)
	}
	if len(sharedMessages) != 2 {
		t.Fatalf("共享消息数量不正确: got=%d want=2", len(sharedMessages))
	}
	if sharedMessages[1]["message_id"] != "assistant-sdk-1" {
		t.Fatalf("共享 assistant message_id 不正确: %+v", sharedMessages[1])
	}
	sharedSummary, ok := sharedMessages[1]["result_summary"].(map[string]any)
	if !ok || anyToString(sharedSummary["result"]) != "done" {
		t.Fatalf("共享 result 摘要应挂在 assistant 上: %+v", sharedMessages[1])
	}
	privateMessages := readRoomPrivateHistory(
		t,
		cfg.WorkspacePath,
		memberAgent.WorkspacePath,
		privateSessionKey,
		memberAgent.AgentID,
		client.sessionID,
	)
	if len(privateMessages) != 2 {
		t.Fatalf("私有 runtime 消息数量不正确: got=%d want=2", len(privateMessages))
	}
	if privateMessages[0]["role"] != "user" || privateMessages[1]["role"] != "assistant" {
		t.Fatalf("私有 runtime 消息顺序不正确: %+v", privateMessages)
	}
	privateUserContent := anyToString(privateMessages[0]["content"])
	for _, expected := range []string{
		"<public_feed>",
		"\"content\":\"你好\"",
		"\"trigger_type\":\"public_chat\"",
	} {
		if !strings.Contains(privateUserContent, expected) {
			t.Fatalf("私有 round marker 应记录实际 Room dispatch prompt，缺少 %q:\n%s", expected, privateUserContent)
		}
	}
	privateSummary, ok := privateMessages[1]["result_summary"].(map[string]any)
	if !ok || anyToInt(privateSummary["duration_ms"]) != 15 || anyToString(privateSummary["result"]) != "done" {
		t.Fatalf("私有 result 应保留 runtime 摘要: %+v", privateMessages[1])
	}
	usageSummary, err := usageService.Summary(ctx, "user-room-usage")
	if err != nil {
		t.Fatalf("读取 room token usage 失败: %v", err)
	}
	if usageSummary.InputTokens != 3 || usageSummary.OutputTokens != 5 || usageSummary.TotalTokens != 8 {
		t.Fatalf("room result usage 未写入 ledger: %+v", usageSummary)
	}
	if usageSummary.SessionCount != 1 || usageSummary.MessageCount != 1 {
		t.Fatalf("room usage 计数不正确: %+v", usageSummary)
	}
}

func TestRealtimeServiceKeepsThinkingDuringStreamingAndHistoryReplay(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := bootstrap.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := bootstrap.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}

	ctx := context.Background()
	memberAgent := createTestAgent(t, agentService, ctx, "流式思考助手")
	dmContext, err := roomService.EnsureDirectRoom(ctx, memberAgent.AgentID)
	if err != nil {
		t.Fatalf("创建直聊 room 失败: %v", err)
	}

	client := newFakeRoomClient()
	client.onQuery = func(_ context.Context, _ string) error {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeStreamEvent,
				SessionID: client.sessionID,
				Stream: &sdkprotocol.StreamEvent{
					Event: map[string]any{
						"type": "message_start",
						"message": map[string]any{
							"id":    "assistant-room-think-1",
							"model": "sonnet",
						},
					},
				},
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeStreamEvent,
				SessionID: client.sessionID,
				Stream: &sdkprotocol.StreamEvent{
					Event: map[string]any{
						"type":  "content_block_start",
						"index": 0,
						"content_block": map[string]any{
							"type":     "thinking",
							"thinking": "先分析",
						},
					},
				},
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeStreamEvent,
				SessionID: client.sessionID,
				Stream: &sdkprotocol.StreamEvent{
					Event: map[string]any{
						"type":  "content_block_delta",
						"index": 0,
						"delta": map[string]any{
							"type":     "thinking_delta",
							"thinking": " 再收口",
						},
					},
				},
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeStreamEvent,
				SessionID: client.sessionID,
				Stream: &sdkprotocol.StreamEvent{
					Event: map[string]any{
						"type":  "content_block_start",
						"index": 0,
						"content_block": map[string]any{
							"type": "text",
							"text": "今天天气",
						},
					},
				},
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeStreamEvent,
				SessionID: client.sessionID,
				Stream: &sdkprotocol.StreamEvent{
					Event: map[string]any{
						"type":  "content_block_delta",
						"index": 0,
						"delta": map[string]any{
							"type": "text_delta",
							"text": " 很不错",
						},
					},
				},
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeAssistant,
				SessionID: client.sessionID,
				Assistant: &sdkprotocol.AssistantMessage{
					Message: sdkprotocol.ConversationEnvelope{
						ID:    "assistant-room-think-1",
						Model: "sonnet",
						Content: []sdkprotocol.ContentBlock{
							sdkprotocol.TextBlock{Text: "今天天气 很不错"},
						},
					},
				},
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-room-think-1",
				Result: &sdkprotocol.ResultMessage{
					Subtype:       "success",
					DurationMS:    12,
					DurationAPIMS: 10,
					NumTurns:      1,
					Result:        "done",
				},
			}
		}()
		return nil
	}

	permission := permissionctx.NewContext()
	runtimeManager := runtimectx.NewManager()
	service := NewRealtimeServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimeManager,
		permission,
		&fakeRoomFactory{clients: []*fakeRoomClient{client}},
	)
	roomHistory := workspace2.NewRoomHistoryStore(cfg.WorkspacePath)

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(dmContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-think-stream")
	permission.BindSession(sharedSessionKey, sender, "room-client-think-stream", true)

	if err = service.HandleChat(ctx, ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         dmContext.Room.ID,
		ConversationID: dmContext.Conversation.ID,
		Content:        "今天天气怎么样呀",
		RoundID:        "room-round-think-stream",
		ReqID:          "room-round-think-stream",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	events := collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	assertRoomStreamBlockIndex(t, events, "assistant-room-think-1", "thinking", 0)
	assertRoomStreamBlockIndex(t, events, "assistant-room-think-1", "text", 1)

	assistantPayload := findRoomAssistantMessagePayload(t, events, "assistant-room-think-1")
	assistantBlocks := roomContentBlocksFromPayload(t, assistantPayload)
	if len(assistantBlocks) != 2 {
		t.Fatalf("Room durable assistant 内容块数量不正确: %+v", assistantPayload)
	}
	if assistantBlocks[0]["type"] != "thinking" || assistantBlocks[0]["thinking"] != "先分析 再收口" {
		t.Fatalf("Room durable assistant 未保留完整 thinking: %+v", assistantBlocks)
	}
	if assistantBlocks[1]["type"] != "text" || assistantBlocks[1]["text"] != "今天天气 很不错" {
		t.Fatalf("Room durable assistant 未保留 text: %+v", assistantBlocks)
	}

	privateSessionKey := protocol.BuildRoomAgentSessionKey(dmContext.Conversation.ID, memberAgent.AgentID, dmContext.Room.RoomType)
	roomThinkingTranscriptBaseTime := time.Now().Add(-2 * time.Second).UTC()
	writeRoomTranscriptFixture(t, memberAgent.WorkspacePath, client.sessionID, []map[string]any{
		{
			"type":      "user",
			"uuid":      "room-think-user-1",
			"sessionId": client.sessionID,
			"timestamp": roomThinkingTranscriptBaseTime.Format(time.RFC3339Nano),
			"message": map[string]any{
				"role":    "user",
				"content": "dispatch prompt",
			},
		},
		{
			"type":       "assistant",
			"uuid":       "assistant-room-think-1",
			"sessionId":  client.sessionID,
			"parentUuid": "room-think-user-1",
			"timestamp":  roomThinkingTranscriptBaseTime.Add(200 * time.Millisecond).Format(time.RFC3339Nano),
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "thinking", "thinking": "先分析 再收口"},
					{"type": "text", "text": "今天天气 很不错"},
				},
			},
		},
	})
	sharedMessages, err := roomHistory.ReadMessages(dmContext.Conversation.ID, nil)
	if err != nil {
		t.Fatalf("读取共享 Room 消息失败: %v", err)
	}
	if len(sharedMessages) != 2 {
		t.Fatalf("共享消息数量不正确: got=%d want=2", len(sharedMessages))
	}
	sharedBlocks := roomContentBlocksFromPayload(t, sharedMessages[1])
	if len(sharedBlocks) != 2 || sharedBlocks[0]["type"] != "thinking" || sharedBlocks[1]["type"] != "text" {
		t.Fatalf("共享历史 assistant 内容块不正确: %+v", sharedMessages[1])
	}
	if _, exists := sharedMessages[1]["stream_status"]; exists {
		t.Fatalf("共享历史 assistant 不应携带 stream_status: %+v", sharedMessages[1])
	}
	if _, ok := sharedMessages[1]["result_summary"].(map[string]any); !ok {
		t.Fatalf("共享历史 assistant 应挂载 result 摘要: %+v", sharedMessages[1])
	}
	privateMessages := readRoomPrivateHistory(
		t,
		cfg.WorkspacePath,
		memberAgent.WorkspacePath,
		privateSessionKey,
		memberAgent.AgentID,
		client.sessionID,
	)
	if len(privateMessages) != 2 {
		t.Fatalf("私有 runtime 消息数量不正确: got=%d want=2", len(privateMessages))
	}
	privateBlocks := roomContentBlocksFromPayload(t, privateMessages[1])
	if len(privateBlocks) != 2 || privateBlocks[0]["type"] != "thinking" || privateBlocks[1]["type"] != "text" {
		t.Fatalf("私有历史 assistant 内容块不正确: %+v", privateMessages[1])
	}
	if _, exists := privateMessages[1]["stream_status"]; exists {
		t.Fatalf("私有历史 assistant 不应携带 stream_status: %+v", privateMessages[1])
	}
	if _, ok := privateMessages[1]["result_summary"].(map[string]any); !ok {
		t.Fatalf("私有历史 assistant 应挂载 result 摘要: %+v", privateMessages[1])
	}
}

func TestRealtimeServiceForwardsProviderModelOption(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := bootstrap.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := bootstrap.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}
	providerService := providercfg.NewServiceWithDB(cfg, db)
	if _, err = providerService.Create(context.Background(), providercfg.CreateInput{
		Provider:    "glm",
		DisplayName: "GLM",
		AuthToken:   "glm-token",
		BaseURL:     "https://open.bigmodel.cn/api/anthropic",
		Model:       "glm-5.1",
		Enabled:     true,
		IsDefault:   true,
	}); err != nil {
		t.Fatalf("创建默认 provider 失败: %v", err)
	}

	ctx := context.Background()
	memberAgent := createTestAgent(t, agentService, ctx, "透传测试助手")
	maxThinkingTokens := 1024
	maxTurns := 4
	memberAgent, err = agentService.UpdateAgent(ctx, memberAgent.AgentID, agentsvc.UpdateRequest{
		Options: &agentsvc.Options{
			MaxThinkingTokens: &maxThinkingTokens,
			MaxTurns:          &maxTurns,
			SettingSources:    []string{"user"},
		},
	})
	if err != nil {
		t.Fatalf("更新 member agent 配置失败: %v", err)
	}
	dmContext, err := roomService.EnsureDirectRoom(ctx, memberAgent.AgentID)
	if err != nil {
		t.Fatalf("创建直聊 room 失败: %v", err)
	}

	client := newFakeRoomClient()
	client.onQuery = func(_ context.Context, _ string) error {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "room-result-no-model",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "success",
					DurationMS: 1,
					NumTurns:   1,
					Result:     "ok",
				},
			}
		}()
		return nil
	}

	permission := permissionctx.NewContext()
	runtimeManager := runtimectx.NewManager()
	factory := &fakeRoomFactory{clients: []*fakeRoomClient{client}}
	service := NewRealtimeServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimeManager,
		permission,
		factory,
	)
	service.SetProviderResolver(providerService)

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(dmContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-no-model")
	permission.BindSession(sharedSessionKey, sender, "client-no-model", true)

	if err = service.HandleChat(ctx, ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         dmContext.Room.ID,
		ConversationID: dmContext.Conversation.ID,
		Content:        "测试 room model 透传",
		RoundID:        "room-round-no-model",
		ReqID:          "room-round-no-model",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	options := factory.LastOptions()
	if options.Model != "glm-5.1" {
		t.Fatalf("room runtime 未向 SDK options 透传 provider model: %+v", options)
	}
	if options.Env["ANTHROPIC_MODEL"] != "glm-5.1" {
		t.Fatalf("room runtime 未注入 provider model: %+v", options.Env)
	}
	if options.Env["ANTHROPIC_DEFAULT_SONNET_MODEL"] != "glm-5.1" {
		t.Fatalf("room runtime 未注入默认 sonnet model: %+v", options.Env)
	}
	if options.Env["CLAUDE_CODE_SUBAGENT_MODEL"] != "glm-5.1" {
		t.Fatalf("room runtime 未注入 subagent model: %+v", options.Env)
	}
	if options.MaxThinkingTokens != maxThinkingTokens {
		t.Fatalf("room runtime 未向 SDK 透传 max thinking tokens: %+v", options)
	}
	if options.MaxTurns != maxTurns {
		t.Fatalf("room runtime 未向 SDK 透传 max turns: %+v", options)
	}
	if len(options.SettingSources) != 1 || options.SettingSources[0] != "user" {
		t.Fatalf("room runtime 未向 SDK 透传 setting_sources: %+v", options)
	}
	if !options.IncludePartialMessages {
		t.Fatalf("room runtime 未开启 partial messages: %+v", options)
	}
}

func TestRealtimeServiceBypassPermissionsDoesNotInstallPermissionHandler(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := bootstrap.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := bootstrap.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := context.Background()
	memberAgent := createTestAgent(t, agentService, ctx, "bypass 助手")
	memberAgent, err = agentService.UpdateAgent(ctx, memberAgent.AgentID, agentsvc.UpdateRequest{
		Options: &agentsvc.Options{
			PermissionMode: "bypassPermissions",
			SettingSources: []string{"project"},
		},
	})
	if err != nil || memberAgent == nil {
		t.Fatalf("更新 member agent 配置失败: value=%+v err=%v", memberAgent, err)
	}
	dmContext, err := roomService.EnsureDirectRoom(ctx, memberAgent.AgentID)
	if err != nil {
		t.Fatalf("创建直聊 room 失败: %v", err)
	}

	client := newFakeRoomClient()
	client.onQuery = func(_ context.Context, _ string) error {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "room-result-bypass",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "success",
					DurationMS: 1,
					NumTurns:   1,
					Result:     "ok",
				},
			}
		}()
		return nil
	}

	permission := permissionctx.NewContext()
	runtimeManager := runtimectx.NewManager()
	factory := &fakeRoomFactory{clients: []*fakeRoomClient{client}}
	service := NewRealtimeServiceWithFactory(cfg, roomService, agentService, runtimeManager, permission, factory)
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(dmContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-bypass")
	permission.BindSession(sharedSessionKey, sender, "client-bypass", true)

	if err = service.HandleChat(ctx, ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         dmContext.Room.ID,
		ConversationID: dmContext.Conversation.ID,
		Content:        "测试 room bypass 权限处理器",
		RoundID:        "room-round-bypass",
		ReqID:          "room-round-bypass",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}
	collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	options := factory.LastOptions()
	if options.PermissionMode != sdkprotocol.PermissionModeBypassPermissions {
		t.Fatalf("room bypass 权限模式未透传: %+v", options)
	}
	if options.PermissionHandler != nil {
		t.Fatalf("room bypass 权限模式不应安装 permission handler: %+v", options)
	}
}

func TestRealtimeServiceWakesMentionedAgentFromPublicAssistantReply(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := bootstrap.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := bootstrap.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}

	ctx := context.Background()
	amy := createTestAgent(t, agentService, ctx, "Amy")
	devin := createTestAgent(t, agentService, ctx, "Devin")
	roomContext, err := roomService.CreateRoom(ctx, CreateRoomRequest{
		AgentIDs: []string{amy.AgentID, devin.AgentID},
		Name:     "公区 @ 测试房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	amyClient := newFakeRoomClient()
	devinClient := newFakeRoomClient()
	devinPrompt := make(chan string, 1)
	factory := &fakeRoomFactory{clients: []*fakeRoomClient{amyClient, devinClient}}
	permission := permissionctx.NewContext()
	runtimeManager := runtimectx.NewManager()
	service := NewRealtimeServiceWithFactory(cfg, roomService, agentService, runtimeManager, permission, factory)

	amyClient.onQuery = func(_ context.Context, _ string) error {
		go sendFakeAssistantResult(amyClient, "amy-public-mention-1", "@Devin 请查询天气，并在公区回复。")
		return nil
	}
	devinClient.onQuery = func(_ context.Context, prompt string) error {
		devinPrompt <- prompt
		go sendFakeAssistantResult(devinClient, "devin-public-mention-1", "天气查询完成。")
		return nil
	}

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-public-mention")
	permission.BindSession(sharedSessionKey, sender, "client-public-mention", true)

	if err = service.HandleChat(ctx, ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@Amy 让 Devin 查下天气",
		RoundID:        "room-round-public-mention",
		ReqID:          "room-round-public-mention",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	events := collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		roundID, _ := event.Data["round_id"].(string)
		return event.EventType == protocol.EventTypeRoundStatus &&
			strings.HasPrefix(roundID, "room_mention_") &&
			event.Data["status"] == "finished"
	})
	select {
	case prompt := <-devinPrompt:
		if !strings.Contains(prompt, "public_mention") ||
			!strings.Contains(prompt, "@Devin 请查询天气") ||
			!strings.Contains(prompt, "agent_id="+devin.AgentID) {
			t.Fatalf("Devin prompt 缺少公区 @ 触发上下文: %s", prompt)
		}
	case <-time.After(time.Second):
		t.Fatal("Devin 未被公区 @ 唤醒")
	}
	if !hasChatAckPendingAgent(events, devin.AgentID) {
		t.Fatalf("事件流缺少 Devin 公区 @ 唤醒 slot: %+v", events)
	}
}

func TestRealtimeServiceQueuesPublicMentionWhenTargetRunning(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := bootstrap.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := bootstrap.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}

	ctx := context.Background()
	amy := createTestAgent(t, agentService, ctx, "Amy")
	devin := createTestAgent(t, agentService, ctx, "Devin")
	roomContext, err := roomService.CreateRoom(ctx, CreateRoomRequest{
		AgentIDs: []string{amy.AgentID, devin.AgentID},
		Name:     "公区 @ 排队房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	devinCurrentClient := newFakeRoomClient()
	amyClient := newFakeRoomClient()
	devinQueuedClient := newFakeRoomClient()
	devinQueuedPrompt := make(chan string, 1)
	devinCurrentClient.onQuery = func(_ context.Context, _ string) error {
		return nil
	}
	amyClient.onQuery = func(_ context.Context, _ string) error {
		go sendFakeAssistantResult(amyClient, "amy-public-mention-busy", "@Devin 当前天气任务交给你。")
		return nil
	}
	devinQueuedClient.onQuery = func(_ context.Context, prompt string) error {
		devinQueuedPrompt <- prompt
		go sendFakeAssistantResult(devinQueuedClient, "devin-public-mention-after-busy", "天气任务已处理。")
		return nil
	}

	permission := permissionctx.NewContext()
	service := NewRealtimeServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permission,
		&fakeRoomFactory{clients: []*fakeRoomClient{devinCurrentClient, amyClient, devinQueuedClient}},
	)

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-public-mention-queue")
	permission.BindSession(sharedSessionKey, sender, "client-public-mention-queue", true)

	if err = service.HandleChat(ctx, ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@Devin 先处理一个长任务",
		RoundID:        "room-round-devin-busy",
		ReqID:          "room-round-devin-busy",
	}); err != nil {
		t.Fatalf("启动 Devin 长任务失败: %v", err)
	}
	_ = collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeStreamStart && event.AgentID == devin.AgentID
	})

	if err = service.HandleChat(ctx, ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@Amy 让 Devin 查下天气",
		RoundID:        "room-round-amy-mentions-busy-devin",
		ReqID:          "room-round-amy-mentions-busy-devin",
	}); err != nil {
		t.Fatalf("启动 Amy 公区 @ 失败: %v", err)
	}

	var queuedItem protocol.InputQueueItem
	_ = collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		if event.EventType != protocol.EventTypeInputQueue {
			return false
		}
		for _, item := range inputQueueItemsFromEvent(event) {
			if item.Source == protocol.InputQueueSourceAgentPublicMention && item.AgentID == devin.AgentID {
				queuedItem = item
				return true
			}
		}
		return false
	})
	if queuedItem.SourceMessageID != "amy-public-mention-busy" ||
		queuedItem.SourceAgentID != amy.AgentID ||
		len(queuedItem.TargetAgentIDs) != 1 ||
		queuedItem.TargetAgentIDs[0] != devin.AgentID {
		t.Fatalf("公区 @ 队列项缺少来源或目标: %+v", queuedItem)
	}
	targetQueueLocation := workspace2.InputQueueLocation{
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  devin.WorkspacePath,
		SessionKey:     protocol.BuildRoomAgentSessionKey(roomContext.Conversation.ID, devin.AgentID, roomContext.Room.RoomType),
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
	}
	targetQueueItems, err := workspace2.NewInputQueueStore(cfg.WorkspacePath).Snapshot(targetQueueLocation)
	if err != nil {
		t.Fatalf("读取目标 agent session 队列失败: %v", err)
	}
	if len(targetQueueItems) != 1 || targetQueueItems[0].ID != queuedItem.ID {
		t.Fatalf("Room 队列未落到目标 agent session: event=%+v stored=%+v", queuedItem, targetQueueItems)
	}

	devinCurrentClient.mu.Lock()
	interruptCalls := devinCurrentClient.interruptCalls
	devinCurrentClient.mu.Unlock()
	if interruptCalls != 0 {
		t.Fatalf("公区 @ 不应中断正在工作的目标 agent: interruptCalls=%d", interruptCalls)
	}
	select {
	case prompt := <-devinQueuedPrompt:
		t.Fatalf("目标 agent 尚未空闲前不应启动 queued mention: %s", prompt)
	default:
	}

	go sendFakeAssistantResult(devinCurrentClient, "devin-current-task-done", "当前长任务完成。")
	select {
	case prompt := <-devinQueuedPrompt:
		if !strings.Contains(prompt, "public_mention") ||
			!strings.Contains(prompt, "@Devin 当前天气任务交给你。") {
			t.Fatalf("queued mention prompt 缺少公区 @ 触发上下文: %s", prompt)
		}
	case <-time.After(time.Second):
		t.Fatal("目标 agent 空闲后未派发 queued mention")
	}
}

func TestRealtimeServiceAcksPublicMessageWithoutMention(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := bootstrap.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := bootstrap.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}

	ctx := context.Background()
	amy := createTestAgent(t, agentService, ctx, "Amy")
	devin := createTestAgent(t, agentService, ctx, "Devin")
	roomContext, err := roomService.CreateRoom(ctx, CreateRoomRequest{
		AgentIDs: []string{amy.AgentID, devin.AgentID},
		Name:     "公区无 @ 测试房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	permission := permissionctx.NewContext()
	runtimeManager := runtimectx.NewManager()
	service := NewRealtimeServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimeManager,
		permission,
		&fakeRoomFactory{},
	)
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-no-mention")
	permission.BindSession(sharedSessionKey, sender, "client-no-mention", true)

	if err = service.HandleChat(ctx, ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "先记一下这个背景",
		RoundID:        "room-round-no-mention",
		ReqID:          "room-round-no-mention",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	events := collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})
	if countEventType(events, protocol.EventTypeChatAck) != 1 {
		t.Fatalf("公区无 @ 消息也必须 ack，否则前端发送队列会卡住: %+v", events)
	}
}

func TestRealtimeServiceSuppressesNoReplyMarkerProjection(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := bootstrap.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := bootstrap.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}

	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "user-room-no-reply",
		Username: "room-owner",
		Role:     authsvc.RoleOwner,
	})
	agentValue := createTestAgent(t, agentService, ctx, "Amy")
	roomContext, err := roomService.CreateRoom(ctx, CreateRoomRequest{
		AgentIDs: []string{agentValue.AgentID},
		Name:     "无需回复测试房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	client := newFakeRoomClient()
	client.onQuery = func(_ context.Context, _ string) error {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeStreamEvent,
				SessionID: client.sessionID,
				Stream: &sdkprotocol.StreamEvent{
					Event: map[string]any{
						"type":  "content_block_start",
						"index": 0,
						"content_block": map[string]any{
							"type": "text",
							"text": "",
						},
					},
				},
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeStreamEvent,
				SessionID: client.sessionID,
				Stream: &sdkprotocol.StreamEvent{
					Event: map[string]any{
						"type":  "content_block_delta",
						"index": 0,
						"delta": map[string]any{
							"type": "text_delta",
							"text": "<nexus_room_no_reply/>",
						},
					},
				},
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeStreamEvent,
				SessionID: client.sessionID,
				Stream: &sdkprotocol.StreamEvent{
					Event: map[string]any{
						"type":  "content_block_stop",
						"index": 0,
					},
				},
			}
			sendFakeAssistantResultWithUsage(client, "amy-no-reply", "<nexus_room_no_reply/>", map[string]any{
				"input_tokens":  7,
				"output_tokens": 2,
			})
		}()
		return nil
	}

	permission := permissionctx.NewContext()
	runtimeManager := runtimectx.NewManager()
	service := NewRealtimeServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimeManager,
		permission,
		&fakeRoomFactory{clients: []*fakeRoomClient{client}},
	)
	usageService := usagesvc.NewServiceWithDB(cfg, db)
	service.SetUsageRecorder(usageService)
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-no-reply")
	permission.BindSession(sharedSessionKey, sender, "client-no-reply", true)

	if err = service.HandleChat(ctx, ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@Amy 这条不用你回答",
		RoundID:        "room-round-no-reply",
		ReqID:          "room-round-no-reply",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	events := collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})
	if hasAgentPublicMessage(events, agentValue.AgentID) {
		t.Fatalf("无需回复标记不应投影到公区: %+v", events)
	}
	if hasStreamText(events, "<nexus_room_no_reply/>") {
		t.Fatalf("无需回复标记不应以流式文本暴露给前端: %+v", events)
	}
	usageSummary, err := usageService.Summary(ctx, "user-room-no-reply")
	if err != nil {
		t.Fatalf("读取 no-reply token usage 失败: %v", err)
	}
	if usageSummary.TotalTokens != 9 {
		t.Fatalf("no-reply result usage 也应写入 ledger: %+v", usageSummary)
	}
}

func TestRealtimeServiceHandleInterruptCancelsAllSlots(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := bootstrap.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := bootstrap.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}

	ctx := context.Background()
	agentA := createTestAgent(t, agentService, ctx, "助手甲")
	agentB := createTestAgent(t, agentService, ctx, "助手乙")
	roomContext, err := roomService.CreateRoom(ctx, CreateRoomRequest{
		AgentIDs: []string{agentA.AgentID, agentB.AgentID},
		Name:     "中断测试房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	clientA := newFakeRoomClient()
	clientA.onQuery = func(_ context.Context, _ string) error {
		return nil
	}
	clientA.onInterrupt = func(_ context.Context) {
		go func() {
			clientA.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: clientA.sessionID,
				UUID:      "room-interrupted-a",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "interrupted",
					DurationMS: 1,
					NumTurns:   1,
				},
			}
		}()
	}
	clientB := newFakeRoomClient()
	clientB.onQuery = func(_ context.Context, _ string) error {
		return nil
	}
	clientB.onInterrupt = func(_ context.Context) {
		go func() {
			clientB.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: clientB.sessionID,
				UUID:      "room-interrupted-b",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "interrupted",
					DurationMS: 1,
					NumTurns:   1,
				},
			}
		}()
	}

	permission := permissionctx.NewContext()
	runtimeManager := runtimectx.NewManager()
	service := NewRealtimeServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimeManager,
		permission,
		&fakeRoomFactory{clients: []*fakeRoomClient{clientA, clientB}},
	)
	roomHistory := workspace2.NewRoomHistoryStore(cfg.WorkspacePath)

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-2")
	permission.BindSession(sharedSessionKey, sender, "client-1", true)

	if err = service.HandleChat(ctx, ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@助手甲 @助手乙 处理一下",
		RoundID:        "room-round-2",
		ReqID:          "room-round-2",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	_ = collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return countEventType(events, protocol.EventTypeStreamStart) >= 2
	})

	if err = service.HandleInterrupt(ctx, InterruptRequest{SessionKey: sharedSessionKey}); err != nil {
		t.Fatalf("HandleInterrupt 失败: %v", err)
	}

	events := collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "interrupted"
	})
	if countRoomResultSubtype(events, "interrupted") < 2 {
		t.Fatalf("期望每个 slot 都产出 interrupted result: %+v", events)
	}

	clientA.mu.Lock()
	interruptA := clientA.interruptCalls
	clientA.mu.Unlock()
	clientB.mu.Lock()
	interruptB := clientB.interruptCalls
	clientB.mu.Unlock()
	if interruptA == 0 || interruptB == 0 {
		t.Fatalf("所有 slot 都应收到 interrupt: a=%d b=%d", interruptA, interruptB)
	}

	sharedMessages, err := roomHistory.ReadMessages(roomContext.Conversation.ID, nil)
	if err != nil {
		t.Fatalf("读取中断后的共享 Room 消息失败: %v", err)
	}
	sharedInterrupted := 0
	for _, message := range sharedMessages {
		summary, ok := message["result_summary"].(map[string]any)
		if ok && summary["subtype"] == "interrupted" {
			sharedInterrupted++
		}
	}
	if sharedInterrupted < 2 {
		t.Fatalf("共享日志未完整落 interrupted result: %+v", sharedMessages)
	}

	for _, agentValue := range []*agentsvc.Agent{agentA, agentB} {
		privateSessionKey := protocol.BuildRoomAgentSessionKey(roomContext.Conversation.ID, agentValue.AgentID, roomContext.Room.RoomType)
		writeRoomTranscriptFixture(t, agentValue.WorkspacePath, "room-sdk-session", []map[string]any{
			{
				"type":      "user",
				"uuid":      "interrupt-user-" + agentValue.AgentID,
				"sessionId": "room-sdk-session",
				"timestamp": "2026-04-19T18:20:00Z",
				"message": map[string]any{
					"role":    "user",
					"content": "dispatch prompt",
				},
			},
		})
		privateMessages := readRoomPrivateHistory(
			t,
			cfg.WorkspacePath,
			agentValue.WorkspacePath,
			privateSessionKey,
			agentValue.AgentID,
			"room-sdk-session",
		)
		foundInterrupted := false
		for _, message := range privateMessages {
			summary, ok := message["result_summary"].(map[string]any)
			if ok && summary["subtype"] == "interrupted" {
				foundInterrupted = true
				break
			}
		}
		if !foundInterrupted {
			t.Fatalf("私有日志未落 interrupted result: agent=%s messages=%+v", agentValue.AgentID, privateMessages)
		}
	}
}

func TestRealtimeServiceNewMessageKeepsOtherAgentRoundRunning(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := bootstrap.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := bootstrap.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}

	ctx := context.Background()
	agentA := createTestAgent(t, agentService, ctx, "助手甲")
	agentB := createTestAgent(t, agentService, ctx, "助手乙")
	roomContext, err := roomService.CreateRoom(ctx, CreateRoomRequest{
		AgentIDs: []string{agentA.AgentID, agentB.AgentID},
		Name:     "并行测试房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	clientA := newFakeRoomClient()
	clientA.onQuery = func(_ context.Context, _ string) error {
		return nil
	}
	clientB := newFakeRoomClient()
	clientB.onQuery = func(_ context.Context, _ string) error {
		go sendFakeAssistantResult(clientB, "assistant-b", "助手乙完成")
		return nil
	}

	permission := permissionctx.NewContext()
	service := NewRealtimeServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permission,
		&fakeRoomFactory{clients: []*fakeRoomClient{clientA, clientB}},
	)

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-parallel-agents")
	permission.BindSession(sharedSessionKey, sender, "client-1", true)

	if err = service.HandleChat(ctx, ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@助手甲 先处理",
		RoundID:        "room-round-agent-a",
		ReqID:          "room-round-agent-a",
	}); err != nil {
		t.Fatalf("HandleChat A 失败: %v", err)
	}
	_ = collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeStreamStart && event.AgentID == agentA.AgentID
	})

	if err = service.HandleChat(ctx, ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@助手乙 你也处理",
		RoundID:        "room-round-agent-b",
		ReqID:          "room-round-agent-b",
	}); err != nil {
		t.Fatalf("HandleChat B 失败: %v", err)
	}

	clientA.mu.Lock()
	interruptA := clientA.interruptCalls
	clientA.mu.Unlock()
	if interruptA != 0 {
		t.Fatalf("发给助手乙的新消息不应中断助手甲: interruptA=%d", interruptA)
	}

	events := collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus &&
			event.Data["round_id"] == "room-round-agent-b" &&
			event.Data["status"] == "finished"
	})
	if countRoomResultSubtype(events, "success") == 0 {
		t.Fatalf("助手乙 round 应正常完成: %+v", events)
	}

	if err = service.HandleInterrupt(ctx, InterruptRequest{SessionKey: sharedSessionKey}); err != nil {
		t.Fatalf("清理活跃 Room round 失败: %v", err)
	}
}

func TestRealtimeServiceAppendsRunningTargetByDefault(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := bootstrap.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := bootstrap.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}

	ctx := context.Background()
	agentValue := createTestAgent(t, agentService, ctx, "助手甲")
	roomContext, err := roomService.CreateRoom(ctx, CreateRoomRequest{
		AgentIDs: []string{agentValue.AgentID},
		Name:     "排队测试房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	client := newFakeRoomClient()
	client.onQuery = func(_ context.Context, _ string) error {
		return nil
	}
	client.onInterrupt = func(_ context.Context) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-room-queue-cleanup",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "interrupted",
					DurationMS: 1,
					NumTurns:   1,
				},
			}
		}()
	}

	permission := permissionctx.NewContext()
	service := NewRealtimeServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permission,
		&fakeRoomFactory{clients: []*fakeRoomClient{client}},
	)

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-queue-running")
	permission.BindSession(sharedSessionKey, sender, "client-queue-running", true)

	if err = service.HandleChat(ctx, ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@助手甲 先处理",
		RoundID:        "room-round-queue-1",
		ReqID:          "room-round-queue-1",
	}); err != nil {
		t.Fatalf("第一条 Room 消息失败: %v", err)
	}
	_ = collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeStreamStart && event.AgentID == agentValue.AgentID
	})

	if err = service.HandleChat(ctx, ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@助手甲 这是补充要求",
		RoundID:        "room-round-queue-2",
		ReqID:          "room-round-queue-2",
	}); err != nil {
		t.Fatalf("第二条 Room 排队消息失败: %v", err)
	}
	_ = collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeChatAck && event.Data["round_id"] == "room-round-queue-2"
	})

	client.mu.Lock()
	interruptCalls := client.interruptCalls
	sentContents := append([]string(nil), client.sentContents...)
	client.mu.Unlock()
	if interruptCalls != 0 {
		t.Fatalf("默认排队不应中断同一个 Room agent: interruptCalls=%d", interruptCalls)
	}
	if len(sentContents) != 1 || sentContents[0] != "@助手甲 这是补充要求" {
		t.Fatalf("Room 运行中 slot 未收到排队输入: %+v", sentContents)
	}

	if err = service.HandleInterrupt(ctx, InterruptRequest{SessionKey: sharedSessionKey}); err != nil {
		t.Fatalf("清理活跃 Room round 失败: %v", err)
	}
}

func TestRealtimeServiceTreatsClosedStreamAfterInterruptAsInterrupted(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := bootstrap.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := bootstrap.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}

	ctx := context.Background()
	agentValue := createTestAgent(t, agentService, ctx, "助手甲")
	roomContext, err := roomService.CreateRoom(ctx, CreateRoomRequest{
		AgentIDs: []string{agentValue.AgentID},
		Name:     "中断关流测试房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	client := newFakeRoomClient()
	client.onQuery = func(_ context.Context, _ string) error {
		return nil
	}
	var closeOnce sync.Once
	client.onInterrupt = func(_ context.Context) {
		closeOnce.Do(func() {
			close(client.messages)
		})
	}

	permission := permissionctx.NewContext()
	service := NewRealtimeServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permission,
		&fakeRoomFactory{clients: []*fakeRoomClient{client}},
	)
	roomHistory := workspace2.NewRoomHistoryStore(cfg.WorkspacePath)

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-interrupt-closed-stream")
	permission.BindSession(sharedSessionKey, sender, "client-1", true)

	if err = service.HandleChat(ctx, ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@助手甲 处理一下",
		RoundID:        "room-round-closed-stream",
		ReqID:          "room-round-closed-stream",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	_ = collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeStreamStart
	})

	if err = service.HandleInterrupt(ctx, InterruptRequest{SessionKey: sharedSessionKey}); err != nil {
		t.Fatalf("HandleInterrupt 失败: %v", err)
	}

	events := collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus &&
			(event.Data["status"] == "interrupted" || event.Data["status"] == "error")
	})
	terminalStatus := ""
	for _, event := range events {
		if event.EventType == protocol.EventTypeRoundStatus {
			terminalStatus = anyToString(event.Data["status"])
		}
	}
	if terminalStatus != "interrupted" {
		t.Fatalf("主动中断后的关流应归类为 interrupted，实际 status=%s events=%+v", terminalStatus, events)
	}
	if countRoomResultSubtype(events, "error") > 0 {
		t.Fatalf("主动中断后的关流不应广播 error result: %+v", events)
	}

	sharedMessages, err := roomHistory.ReadMessages(roomContext.Conversation.ID, nil)
	if err != nil {
		t.Fatalf("读取中断后的共享 Room 消息失败: %v", err)
	}
	foundInterrupted := false
	for _, message := range sharedMessages {
		summary, ok := message["result_summary"].(map[string]any)
		if !ok {
			continue
		}
		if summary["subtype"] == "error" {
			t.Fatalf("主动中断后的共享日志不应落 error summary: %+v", sharedMessages)
		}
		if summary["subtype"] == "interrupted" {
			foundInterrupted = true
			if strings.Contains(anyToString(summary["result"]), "round stream closed before terminal") {
				t.Fatalf("interrupted summary 不应暴露底层 stream 错误: %+v", summary)
			}
		}
	}
	if !foundInterrupted {
		t.Fatalf("共享日志未落 interrupted summary: %+v", sharedMessages)
	}
}

func TestRealtimeServiceUsesAndPersistsRoomSDKSessionID(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := bootstrap.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := bootstrap.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}

	ctx := context.Background()
	agentValue := createTestAgent(t, agentService, ctx, "助手甲")
	roomContext, err := roomService.CreateRoom(ctx, CreateRoomRequest{
		AgentIDs: []string{agentValue.AgentID},
		Name:     "Resume 房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}
	if len(roomContext.Sessions) != 1 {
		t.Fatalf("期望只有一个 room session: %+v", roomContext.Sessions)
	}

	db, err = sql.Open("sqlite3", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer db.Close()

	resumeID := "room-resume-1"
	if _, err = db.Exec(`UPDATE sessions SET sdk_session_id = ? WHERE id = ?`, resumeID, roomContext.Sessions[0].ID); err != nil {
		t.Fatalf("预写入 room sdk_session_id 失败: %v", err)
	}

	client := newFakeRoomClient()
	client.sessionID = "room-sdk-session-latest"
	client.onQuery = func(_ context.Context, _ string) error {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "room-result-resume",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "success",
					DurationMS: 1,
					NumTurns:   1,
					Result:     "ok",
				},
			}
		}()
		return nil
	}

	permission := permissionctx.NewContext()
	factory := &fakeRoomFactory{clients: []*fakeRoomClient{client}}
	runtimeManager := runtimectx.NewManager()
	service := NewRealtimeServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimeManager,
		permission,
		factory,
	)

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-resume-sender")
	permission.BindSession(sharedSessionKey, sender, "client-room-resume", true)

	if err = service.HandleChat(ctx, ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "测试 room resume",
		RoundID:        "room-round-resume",
		ReqID:          "room-round-resume",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	options := factory.LastOptions()
	if options.Resume != resumeID {
		t.Fatalf("room runtime 未将房间 sdk_session_id 作为 resume 透传: %+v", options)
	}

	updatedContext, err := roomService.GetConversationContext(ctx, roomContext.Conversation.ID)
	if err != nil {
		t.Fatalf("读取更新后的 room context 失败: %v", err)
	}
	if len(updatedContext.Sessions) != 1 {
		t.Fatalf("更新后的 room session 数量不正确: %+v", updatedContext.Sessions)
	}
	if updatedContext.Sessions[0].SDKSessionID != client.sessionID {
		t.Fatalf("room sdk_session_id 未回写数据库: %+v", updatedContext.Sessions[0])
	}
}

func collectRoomEventsUntil(
	t *testing.T,
	events <-chan protocol.EventMessage,
	stop func([]protocol.EventMessage, protocol.EventMessage) bool,
) []protocol.EventMessage {
	t.Helper()
	result := make([]protocol.EventMessage, 0, 16)
	timeout := time.After(3 * time.Second)
	for {
		select {
		case event := <-events:
			result = append(result, event)
			if stop(result, event) {
				return result
			}
		case <-timeout:
			t.Fatalf("等待 Room 事件超时，当前事件: %+v", result)
		}
	}
}

var roomTranscriptSanitizePattern = regexp.MustCompile(`[^a-zA-Z0-9]`)

func readRoomPrivateHistory(
	t *testing.T,
	root string,
	workspacePath string,
	sessionKey string,
	agentID string,
	sessionID string,
) []protocol.Message {
	t.Helper()
	historyStore := workspace2.NewAgentHistoryStore(root)
	rows, err := historyStore.ReadMessages(workspacePath, session.Session{
		SessionKey: sessionKey,
		AgentID:    agentID,
		SessionID:  stringPointer(sessionID),
		Options:    map[string]any{},
	}, nil)
	if err != nil {
		t.Fatalf("读取 room transcript 历史失败: %v", err)
	}
	return rows
}

func writeRoomTranscriptFixture(
	t *testing.T,
	workspacePath string,
	sessionID string,
	rows []map[string]any,
) {
	t.Helper()
	if strings.TrimSpace(sessionID) == "" {
		t.Fatal("session_id 为空，无法写入 room transcript fixture")
	}
	projectDir := filepath.Join(
		os.Getenv("NEXUS_CONFIG_DIR"),
		"projects",
		sanitizeRoomTranscriptPath(canonicalizeRoomTranscriptPath(workspacePath)),
	)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("创建 room transcript 目录失败: %v", err)
	}
	file, err := os.Create(filepath.Join(projectDir, sessionID+".jsonl"))
	if err != nil {
		t.Fatalf("创建 room transcript fixture 失败: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	for _, row := range rows {
		if err := encoder.Encode(row); err != nil {
			t.Fatalf("写入 room transcript fixture 失败: %v", err)
		}
	}
}

func canonicalizeRoomTranscriptPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	absolutePath, err := filepath.Abs(path)
	if err == nil {
		path = absolutePath
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	return path
}

func sanitizeRoomTranscriptPath(path string) string {
	const maxLength = 200
	sanitized := roomTranscriptSanitizePattern.ReplaceAllString(path, "-")
	if len(sanitized) <= maxLength {
		return sanitized
	}
	return sanitized[:maxLength] + "-" + roomTranscriptHash(path)
}

func roomTranscriptHash(value string) string {
	var hash int32
	for _, character := range value {
		hash = hash*31 + int32(character)
	}

	number := int64(hash)
	if number < 0 {
		number = -number
	}
	if number == 0 {
		return "0"
	}

	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	result := make([]byte, 0, 8)
	for number > 0 {
		result = append(result, digits[number%36])
		number /= 36
	}
	for left, right := 0, len(result)-1; left < right; left, right = left+1, right-1 {
		result[left], result[right] = result[right], result[left]
	}
	return string(result)
}

func anyToInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	default:
		return 0
	}
}

func anyToString(value any) string {
	if typed, ok := value.(string); ok {
		return typed
	}
	return ""
}

func assertRoomEventTypes(t *testing.T, events []protocol.EventMessage, expected []protocol.EventType) {
	t.Helper()
	if len(events) < len(expected) {
		t.Fatalf("Room 事件数量不足: got=%d want>=%d all=%+v", len(events), len(expected), events)
	}
	for index, eventType := range expected {
		if events[index].EventType != eventType {
			t.Fatalf("第 %d 个 Room 事件类型不正确: got=%s want=%s all=%+v", index, events[index].EventType, eventType, events)
		}
	}
}

func countEventType(events []protocol.EventMessage, target protocol.EventType) int {
	count := 0
	for _, event := range events {
		if event.EventType == target {
			count++
		}
	}
	return count
}

func countRoomResultSubtype(events []protocol.EventMessage, subtype string) int {
	count := 0
	for _, event := range events {
		if event.EventType != protocol.EventTypeMessage {
			continue
		}
		if event.Data["role"] == "result" && event.Data["subtype"] == subtype {
			count++
			continue
		}
		if event.Data["role"] == "assistant" {
			summary, ok := event.Data["result_summary"].(map[string]any)
			if ok && summary["subtype"] == subtype {
				count++
			}
		}
	}
	return count
}

func hasChatAckPendingAgent(events []protocol.EventMessage, agentID string) bool {
	for _, event := range events {
		if event.EventType != protocol.EventTypeChatAck {
			continue
		}
		pending, ok := event.Data["pending"].([]map[string]any)
		if !ok {
			continue
		}
		for _, item := range pending {
			if item["agent_id"] == agentID {
				return true
			}
		}
	}
	return false
}

func inputQueueItemsFromEvent(event protocol.EventMessage) []protocol.InputQueueItem {
	switch items := event.Data["items"].(type) {
	case []protocol.InputQueueItem:
		return items
	case []any:
		result := make([]protocol.InputQueueItem, 0, len(items))
		for _, item := range items {
			payload, err := json.Marshal(item)
			if err != nil {
				continue
			}
			var parsed protocol.InputQueueItem
			if err = json.Unmarshal(payload, &parsed); err != nil {
				continue
			}
			result = append(result, parsed)
		}
		return result
	default:
		return nil
	}
}

func hasAgentPublicMessage(events []protocol.EventMessage, agentID string) bool {
	for _, event := range events {
		if event.EventType != protocol.EventTypeMessage {
			continue
		}
		if event.Data["agent_id"] == agentID &&
			(event.Data["role"] == "assistant" || event.Data["role"] == "result") {
			return true
		}
	}
	return false
}

func hasStreamText(events []protocol.EventMessage, text string) bool {
	for _, event := range events {
		if event.EventType != protocol.EventTypeStream {
			continue
		}
		block, _ := event.Data["content_block"].(map[string]any)
		if strings.Contains(normalizePendingValue(block["text"]), text) {
			return true
		}
	}
	return false
}

func normalizePendingValue(value any) string {
	if typed, ok := value.(string); ok {
		return typed
	}
	return ""
}

func assertRoomStreamBlockIndex(t *testing.T, events []protocol.EventMessage, messageID string, blockType string, expectedIndex int) {
	t.Helper()
	for _, event := range events {
		if event.EventType != protocol.EventTypeStream || event.MessageID != messageID {
			continue
		}
		contentBlock, ok := event.Data["content_block"].(map[string]any)
		if !ok || contentBlock["type"] != blockType {
			continue
		}
		if event.Data["index"] != expectedIndex {
			t.Fatalf("Room %s stream index 不正确: got=%v want=%d event=%+v", blockType, event.Data["index"], expectedIndex, event)
		}
		return
	}
	t.Fatalf("未找到 Room block_type=%s message_id=%s 的 stream 事件: %+v", blockType, messageID, events)
}

func findRoomAssistantMessagePayload(t *testing.T, events []protocol.EventMessage, messageID string) protocol.Message {
	t.Helper()
	for _, event := range events {
		if event.EventType != protocol.EventTypeMessage || event.MessageID != messageID {
			continue
		}
		if event.Data["role"] != "assistant" {
			continue
		}
		return protocol.Message(event.Data)
	}
	t.Fatalf("未找到 Room assistant message_id=%s 的 durable 消息: %+v", messageID, events)
	return nil
}

func roomContentBlocksFromPayload(t *testing.T, payload map[string]any) []map[string]any {
	t.Helper()
	rawBlocks, ok := payload["content"]
	if !ok {
		t.Fatalf("Room 消息缺少 content: %+v", payload)
	}
	switch typed := rawBlocks.(type) {
	case []map[string]any:
		return typed
	case []any:
		result := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			block, ok := item.(map[string]any)
			if !ok {
				t.Fatalf("Room content block 类型不正确: %+v", payload)
			}
			result = append(result, block)
		}
		return result
	default:
		t.Fatalf("Room content 类型不正确: %+v", payload)
		return nil
	}
}
