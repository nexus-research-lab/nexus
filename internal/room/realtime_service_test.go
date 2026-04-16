// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：realtime_service_test.go
// @Date   ：2026/04/11 06:02:00
// @Author ：leemysw
// 2026/04/11 06:02:00   Create
// =====================================================

package room

import (
	"context"
	agentsvc "github.com/nexus-research-lab/nexus-core/internal/agent"
	permissionctx "github.com/nexus-research-lab/nexus-core/internal/permission"
	"github.com/nexus-research-lab/nexus-core/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus-core/internal/runtime"
	"sync"
	"testing"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

type fakeRoomClient struct {
	mu             sync.Mutex
	sessionID      string
	messages       chan sdkprotocol.ReceivedMessage
	interruptCalls int
	onQuery        func(context.Context, string) error
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

func (c *fakeRoomClient) Interrupt(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.interruptCalls++
	return nil
}

func (c *fakeRoomClient) Disconnect(context.Context) error { return nil }

func (c *fakeRoomClient) Reconfigure(context.Context, agentclient.Options) error {
	return nil
}

func (c *fakeRoomClient) SetPermissionMode(context.Context, sdkprotocol.PermissionMode) error {
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

	agentService, err := agentsvc.NewService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService, err := NewService(cfg)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}

	ctx := context.Background()
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
	service := NewRealtimeServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimeManager,
		permission,
		factory,
	)

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(dmContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-1")
	permission.BindSession(sharedSessionKey, sender, "client-1", true)

	if err = service.HandleChat(ctx, ChatRequest{
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

	sharedMessages, err := service.files.ReadRoomMessages(service.files.RoomConversationMessagePath(dmContext.Conversation.ID))
	if err != nil {
		t.Fatalf("读取共享 Room 消息失败: %v", err)
	}
	if len(sharedMessages) != 3 {
		t.Fatalf("共享消息数量不正确: got=%d want=3", len(sharedMessages))
	}
	if sharedMessages[1]["message_id"] != "assistant-sdk-1" {
		t.Fatalf("共享 assistant message_id 不正确: %+v", sharedMessages[1])
	}

	privateSessionKey := protocol.BuildRoomAgentSessionKey(dmContext.Conversation.ID, memberAgent.AgentID, dmContext.Room.RoomType)
	privateMessages, err := service.files.ReadSessionMessages([]string{memberAgent.WorkspacePath}, privateSessionKey)
	if err != nil {
		t.Fatalf("读取私有 runtime 消息失败: %v", err)
	}
	if len(privateMessages) != 3 {
		t.Fatalf("私有 runtime 消息数量不正确: got=%d want=3", len(privateMessages))
	}
	if privateMessages[0]["role"] != "user" || privateMessages[1]["role"] != "assistant" || privateMessages[2]["role"] != "result" {
		t.Fatalf("私有 runtime 消息顺序不正确: %+v", privateMessages)
	}

	costSummary, err := service.files.ReadSessionCostSummary([]string{memberAgent.WorkspacePath}, privateSessionKey, memberAgent.AgentID)
	if err != nil {
		t.Fatalf("读取私有 runtime 成本失败: %v", err)
	}
	if costSummary.SessionKey != privateSessionKey {
		t.Fatalf("成本 session_key 不正确: got=%s want=%s", costSummary.SessionKey, privateSessionKey)
	}
	if costSummary.TotalOutputTokens != 5 {
		t.Fatalf("输出 token 统计不正确: %+v", costSummary)
	}
}

func TestRealtimeServiceDoesNotForwardModelOption(t *testing.T) {
	cfg := newRoomTestConfig(t)
	cfg.MainAgentModel = "glm-5.1"
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, err := agentsvc.NewService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService, err := NewService(cfg)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}

	ctx := context.Background()
	memberAgent := createTestAgent(t, agentService, ctx, "透传测试助手")
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

	if options := factory.LastOptions(); options.Model != "" {
		t.Fatalf("room runtime 不应向 SDK 透传 model: %+v", options)
	}
}

func TestRealtimeServiceHandleInterruptCancelsAllSlots(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, err := agentsvc.NewService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService, err := NewService(cfg)
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
	clientA.onQuery = func(ctx context.Context, _ string) error {
		<-ctx.Done()
		return ctx.Err()
	}
	clientB := newFakeRoomClient()
	clientB.onQuery = func(ctx context.Context, _ string) error {
		<-ctx.Done()
		return ctx.Err()
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
	if countEventType(events, protocol.EventTypeStreamCancelled) < 2 {
		t.Fatalf("期望至少 2 个 stream_cancelled 事件: %+v", events)
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

func normalizePendingValue(value any) string {
	if typed, ok := value.(string); ok {
		return typed
	}
	return ""
}
