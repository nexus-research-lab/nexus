// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：service_test.go
// @Date   ：2026/04/11 03:00:00
// @Author ：leemysw
// 2026/04/11 03:00:00   Create
// =====================================================

package chat

import (
	"context"
	"database/sql"
	agentsvc "github.com/nexus-research-lab/nexus-core/internal/agent"
	"github.com/nexus-research-lab/nexus-core/internal/config"
	permissionctx "github.com/nexus-research-lab/nexus-core/internal/permission"
	"github.com/nexus-research-lab/nexus-core/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus-core/internal/runtime"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

type fakeChatClient struct {
	mu             sync.Mutex
	sessionID      string
	messages       chan sdkprotocol.ReceivedMessage
	interruptCalls int
	reconfigureOps []agentclient.Options
	onQuery        func(context.Context, string)
}

func newFakeChatClient() *fakeChatClient {
	return &fakeChatClient{
		sessionID: "sdk-session-1",
		messages:  make(chan sdkprotocol.ReceivedMessage, 16),
	}
}

func (c *fakeChatClient) Connect(context.Context) error { return nil }

func (c *fakeChatClient) Query(ctx context.Context, prompt string) error {
	if c.onQuery != nil {
		c.onQuery(ctx, prompt)
	}
	return ctx.Err()
}

func (c *fakeChatClient) ReceiveMessages(context.Context) <-chan sdkprotocol.ReceivedMessage {
	return c.messages
}

func (c *fakeChatClient) Interrupt(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.interruptCalls++
	return nil
}

func (c *fakeChatClient) Disconnect(context.Context) error { return nil }

func (c *fakeChatClient) Reconfigure(_ context.Context, options agentclient.Options) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reconfigureOps = append(c.reconfigureOps, options)
	return nil
}

func (c *fakeChatClient) SetPermissionMode(context.Context, sdkprotocol.PermissionMode) error {
	return nil
}

func (c *fakeChatClient) SessionID() string { return c.sessionID }

type fakeChatFactory struct {
	mu      sync.Mutex
	client  *fakeChatClient
	options []agentclient.Options
}

func (f *fakeChatFactory) New(options agentclient.Options) runtimectx.Client {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.options = append(f.options, options)
	return f.client
}

func (f *fakeChatFactory) LastOptions() agentclient.Options {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.options) == 0 {
		return agentclient.Options{}
	}
	return f.options[len(f.options)-1]
}

type chatTestSender struct {
	key    string
	events chan protocol.EventMessage
}

func newChatTestSender(key string) *chatTestSender {
	return &chatTestSender{
		key:    key,
		events: make(chan protocol.EventMessage, 32),
	}
}

func (s *chatTestSender) Key() string    { return s.key }
func (s *chatTestSender) IsClosed() bool { return false }
func (s *chatTestSender) SendEvent(_ context.Context, event protocol.EventMessage) error {
	s.events <- event
	return nil
}

func TestServiceHandleChatPersistsMessages(t *testing.T) {
	cfg := newChatTestConfig(t)
	migrateChatSQLite(t, cfg.DatabaseURL)

	agentService, err := agentsvc.NewService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	permission := permissionctx.NewContext()
	client := newFakeChatClient()
	client.onQuery = func(_ context.Context, _ string) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeAssistant,
				SessionID: client.sessionID,
				Assistant: &sdkprotocol.AssistantMessage{
					Message: sdkprotocol.ConversationEnvelope{
						ID:    "assistant-1",
						Model: "sonnet",
						Content: []sdkprotocol.ContentBlock{
							sdkprotocol.TextBlock{Text: "你好，世界"},
						},
					},
				},
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-1",
				Result: &sdkprotocol.ResultMessage{
					Subtype:       "success",
					DurationMS:    12,
					DurationAPIMS: 10,
					NumTurns:      1,
					Result:        "done",
					Usage: map[string]any{
						"input_tokens":  3,
						"output_tokens": 5,
					},
				},
			}
		}()
	}

	factory := &fakeChatFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sender := newChatTestSender("sender-1")
	sessionKey := "agent:nexus:ws:dm:test-chat"
	permission.BindSession(sessionKey, sender, "client-1", true)

	if err = service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "你好",
		RoundID:    "round-1",
		ReqID:      "round-1",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	events := collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})
	assertEventTypes(t, events, []protocol.EventType{
		protocol.EventTypeChatAck,
		protocol.EventTypeRoundStatus,
		protocol.EventTypeSessionStatus,
		protocol.EventTypeMessage,
		protocol.EventTypeStreamEnd,
		protocol.EventTypeMessage,
		protocol.EventTypeRoundStatus,
	})

	messages, err := service.files.ReadSessionMessages([]string{filepath.Join(cfg.WorkspacePath, cfg.DefaultAgentID)}, sessionKey)
	if err != nil {
		t.Fatalf("读取会话消息失败: %v", err)
	}
	if len(messages) != 3 {
		t.Fatalf("期望 3 条消息，实际 %d", len(messages))
	}
	if messages[0]["role"] != "user" || messages[1]["role"] != "assistant" || messages[2]["role"] != "result" {
		t.Fatalf("消息角色顺序不正确: %+v", messages)
	}
}

func TestServiceHandleChatForwardsRuntimeOptions(t *testing.T) {
	cfg := newChatTestConfig(t)
	migrateChatSQLite(t, cfg.DatabaseURL)

	agentService, err := agentsvc.NewService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	maxThinkingTokens := 2048
	maxTurns := 6
	updatedAgent, err := agentService.UpdateAgent(context.Background(), cfg.DefaultAgentID, agentsvc.UpdateRequest{
		Options: &agentsvc.Options{
			Model:             "glm-5.1",
			MaxThinkingTokens: &maxThinkingTokens,
			MaxTurns:          &maxTurns,
			SettingSources:    []string{"user"},
		},
	})
	if err != nil {
		t.Fatalf("更新 agent 配置失败: %v", err)
	}
	if updatedAgent == nil {
		t.Fatal("更新 agent 后返回为空")
	}
	permission := permissionctx.NewContext()
	client := newFakeChatClient()
	client.onQuery = func(_ context.Context, _ string) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-no-model",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "success",
					DurationMS: 1,
					NumTurns:   1,
					Result:     "ok",
				},
			}
		}()
	}

	factory := &fakeChatFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sender := newChatTestSender("sender-no-model")
	sessionKey := "agent:nexus:ws:dm:no-model"
	permission.BindSession(sessionKey, sender, "client-no-model", true)

	if err = service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "测试 model 透传",
		RoundID:    "round-no-model",
		ReqID:      "round-no-model",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	options := factory.LastOptions()
	if options.Model != "glm-5.1" {
		t.Fatalf("runtime 未向 SDK 透传 model: %+v", options)
	}
	if options.MaxThinkingTokens != maxThinkingTokens {
		t.Fatalf("runtime 未向 SDK 透传 max thinking tokens: %+v", options)
	}
	if options.MaxTurns != maxTurns {
		t.Fatalf("runtime 未向 SDK 透传 max turns: %+v", options)
	}
	if len(options.SettingSources) != 1 || options.SettingSources[0] != "user" {
		t.Fatalf("runtime 未向 SDK 透传 setting_sources: %+v", options)
	}
}

func TestServiceHandleInterruptEmitsInterruptedRound(t *testing.T) {
	cfg := newChatTestConfig(t)
	migrateChatSQLite(t, cfg.DatabaseURL)

	agentService, err := agentsvc.NewService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	permission := permissionctx.NewContext()
	client := newFakeChatClient()
	client.onQuery = func(ctx context.Context, _ string) {
		<-ctx.Done()
	}

	factory := &fakeChatFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sender := newChatTestSender("sender-1")
	sessionKey := "agent:nexus:ws:dm:test-interrupt"
	permission.BindSession(sessionKey, sender, "client-1", true)

	if err = service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "你好",
		RoundID:    "round-2",
		ReqID:      "round-2",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}
	waitForEvent(t, sender.events, protocol.EventTypeRoundStatus, "running")

	if err = service.HandleInterrupt(context.Background(), InterruptRequest{SessionKey: sessionKey}); err != nil {
		t.Fatalf("HandleInterrupt 失败: %v", err)
	}

	events := collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "interrupted"
	})
	assertContainsRoundStatus(t, events, "interrupted")

	client.mu.Lock()
	interruptCalls := client.interruptCalls
	client.mu.Unlock()
	if interruptCalls == 0 {
		t.Fatal("期望 fake client 收到 interrupt")
	}
}

func TestServiceHandleChatPersistsStructuredChannelMetadata(t *testing.T) {
	cfg := newChatTestConfig(t)
	migrateChatSQLite(t, cfg.DatabaseURL)

	agentService, err := agentsvc.NewService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	permission := permissionctx.NewContext()
	client := newFakeChatClient()
	client.onQuery = func(_ context.Context, _ string) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-structured",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "success",
					DurationMS: 1,
					NumTurns:   1,
					Result:     "ok",
				},
			}
		}()
	}

	factory := &fakeChatFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sender := newChatTestSender("sender-structured")
	sessionKey := "agent:nexus:tg:group:-100123456:topic:12"
	permission.BindSession(sessionKey, sender, "client-structured", true)

	if err = service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "结构化入口",
		RoundID:    "round-structured",
		ReqID:      "round-structured",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	item, _, err := service.files.FindSession([]string{filepath.Join(cfg.WorkspacePath, cfg.DefaultAgentID)}, sessionKey)
	if err != nil {
		t.Fatalf("读取 session 元数据失败: %v", err)
	}
	if item == nil {
		t.Fatal("session 元数据不存在")
	}
	if item.ChannelType != "telegram" || item.ChatType != "group" {
		t.Fatalf("session 元数据不正确: %+v", *item)
	}
}

func newChatTestConfig(t *testing.T) config.Config {
	t.Helper()
	root := t.TempDir()
	return config.Config{
		Host:           "127.0.0.1",
		Port:           18032,
		ProjectName:    "nexus-chat-test",
		APIPrefix:      "/agent/v1",
		WebSocketPath:  "/agent/v1/chat/ws",
		DefaultAgentID: "nexus",
		WorkspacePath:  filepath.Join(root, "workspace"),
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(root, "nexus.db"),
	}
}

func migrateChatSQLite(t *testing.T, databaseURL string) {
	t.Helper()

	db, err := sql.Open("sqlite3", databaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer db.Close()

	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("设置 goose 方言失败: %v", err)
	}
	if err = goose.Up(db, chatMigrationDir(t)); err != nil {
		t.Fatalf("执行 migration 失败: %v", err)
	}
}

func chatMigrationDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "db", "migrations", "sqlite")
}

func waitForEvent(t *testing.T, events <-chan protocol.EventMessage, eventType protocol.EventType, status string) {
	t.Helper()
	_ = collectEventsUntil(t, events, func(event protocol.EventMessage) bool {
		if event.EventType != eventType {
			return false
		}
		if status == "" {
			return true
		}
		return event.Data["status"] == status
	})
}

func collectEventsUntil(
	t *testing.T,
	events <-chan protocol.EventMessage,
	stop func(protocol.EventMessage) bool,
) []protocol.EventMessage {
	t.Helper()
	result := make([]protocol.EventMessage, 0, 8)
	timeout := time.After(3 * time.Second)
	for {
		select {
		case event := <-events:
			result = append(result, event)
			if stop(event) {
				return result
			}
		case <-timeout:
			t.Fatalf("等待事件超时，当前事件: %+v", result)
		}
	}
}

func assertEventTypes(t *testing.T, events []protocol.EventMessage, expected []protocol.EventType) {
	t.Helper()
	if len(events) < len(expected) {
		t.Fatalf("事件数量不足: got=%d want>=%d", len(events), len(expected))
	}
	for index, eventType := range expected {
		if events[index].EventType != eventType {
			t.Fatalf("第 %d 个事件类型不正确: got=%s want=%s all=%+v", index, events[index].EventType, eventType, events)
		}
	}
}

func assertContainsRoundStatus(t *testing.T, events []protocol.EventMessage, status string) {
	t.Helper()
	for _, event := range events {
		if event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == status {
			return
		}
	}
	t.Fatalf("未找到 round_status=%s: %+v", status, events)
}
