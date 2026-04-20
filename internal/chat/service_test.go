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
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	agentsvc "github.com/nexus-research-lab/nexus/internal/agent"
	"github.com/nexus-research-lab/nexus/internal/config"
	sessionmodel "github.com/nexus-research-lab/nexus/internal/model/session"
	permissionctx "github.com/nexus-research-lab/nexus/internal/permission"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	providercfg "github.com/nexus-research-lab/nexus/internal/provider"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/session"
	sqliterepo "github.com/nexus-research-lab/nexus/internal/storage/sqlite"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

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

func newChatAgentService(t *testing.T, cfg config.Config) *agentsvc.Service {
	t.Helper()
	db, err := sql.Open("sqlite3", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return agentsvc.NewService(cfg, sqliterepo.NewAgentRepository(db))
}

func newChatProviderService(t *testing.T, cfg config.Config) *providercfg.Service {
	t.Helper()
	db, err := sql.Open("sqlite3", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开 provider 测试数据库失败: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return providercfg.NewServiceWithDB(cfg, db)
}

func TestServiceHandleChatPersistsMessages(t *testing.T) {
	cfg := newChatTestConfig(t)
	migrateChatSQLite(t, cfg.DatabaseURL)

	agentService := newChatAgentService(t, cfg)
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
							{Type: "text", Text: "你好，世界"},
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

	if err := service.HandleChat(context.Background(), Request{
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
		protocol.EventTypeMessage,
		protocol.EventTypeRoundStatus,
	})

	sessionValue, workspacePath := mustFindChatSession(t, service, cfg, sessionKey)
	transcriptBaseTime := time.Now().Add(-2 * time.Second).UTC()
	writeTranscriptFixture(t, cfg, workspacePath, stringPointer(t, sessionValue.SessionID), []map[string]any{
		{
			"type":      "user",
			"uuid":      "transcript-user-1",
			"sessionId": stringPointer(t, sessionValue.SessionID),
			"timestamp": transcriptBaseTime.Format(time.RFC3339Nano),
			"message": map[string]any{
				"role":    "user",
				"content": "你好",
			},
		},
		{
			"type":       "assistant",
			"uuid":       "assistant-1",
			"sessionId":  stringPointer(t, sessionValue.SessionID),
			"parentUuid": "transcript-user-1",
			"timestamp":  transcriptBaseTime.Add(200 * time.Millisecond).Format(time.RFC3339Nano),
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "text", "text": "你好，世界"},
				},
			},
		},
	})
	messages := readChatSessionHistory(t, cfg, service, sessionKey)
	if len(messages) != 3 {
		t.Fatalf("期望 3 条消息，实际 %d", len(messages))
	}
	if messages[0]["role"] != "user" || messages[1]["role"] != "assistant" || messages[2]["role"] != "result" {
		t.Fatalf("消息角色顺序不正确: %+v", messages)
	}
	if messages[2]["result"] != "done" || anyToInt(messages[2]["duration_ms"]) != 12 {
		t.Fatalf("result 摘要应来自 overlay: %+v", messages[2])
	}
	usage, _ := messages[2]["usage"].(map[string]any)
	outputTokens := anyToInt(usage["output_tokens"])
	if outputTokens != 5 {
		t.Fatalf("result usage 应保留: %+v", messages[2])
	}
}

func TestServiceHandleChatKeepsThinkingDuringStreamingAndHistoryReplay(t *testing.T) {
	cfg := newChatTestConfig(t)
	migrateChatSQLite(t, cfg.DatabaseURL)

	agentService := newChatAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeChatClient()
	client.onQuery = func(_ context.Context, _ string) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeStreamEvent,
				SessionID: client.sessionID,
				Stream: &sdkprotocol.StreamEvent{
					Event: map[string]any{
						"type": "message_start",
						"message": map[string]any{
							"id":    "assistant-think-1",
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
						ID:    "assistant-think-1",
						Model: "sonnet",
						Content: []sdkprotocol.ContentBlock{
							{Type: "text", Text: "今天天气 很不错"},
						},
					},
				},
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-think-1",
				Result: &sdkprotocol.ResultMessage{
					Subtype:       "success",
					DurationMS:    12,
					DurationAPIMS: 10,
					NumTurns:      1,
					Result:        "done",
				},
			}
		}()
	}

	factory := &fakeChatFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sender := newChatTestSender("sender-think-stream")
	sessionKey := "agent:nexus:ws:dm:think-stream"
	permission.BindSession(sessionKey, sender, "client-think-stream", true)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "今天天气怎么样呀",
		RoundID:    "round-think-stream",
		ReqID:      "round-think-stream",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	events := collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	assertStreamBlockIndex(t, events, "thinking", 0)
	assertStreamBlockIndex(t, events, "text", 1)

	assistantPayload := findAssistantMessagePayload(t, events, "assistant-think-1")
	assistantBlocks := contentBlocksFromPayload(t, assistantPayload)
	if len(assistantBlocks) != 2 {
		t.Fatalf("durable assistant 内容块数量不正确: %+v", assistantPayload)
	}
	if assistantBlocks[0]["type"] != "thinking" || assistantBlocks[0]["thinking"] != "先分析 再收口" {
		t.Fatalf("durable assistant 未保留完整 thinking: %+v", assistantBlocks)
	}
	if assistantBlocks[1]["type"] != "text" || assistantBlocks[1]["text"] != "今天天气 很不错" {
		t.Fatalf("durable assistant 未保留 text: %+v", assistantBlocks)
	}

	sessionValue, workspacePath := mustFindChatSession(t, service, cfg, sessionKey)
	thinkingTranscriptBaseTime := time.Now().Add(-2 * time.Second).UTC()
	writeTranscriptFixture(t, cfg, workspacePath, stringPointer(t, sessionValue.SessionID), []map[string]any{
		{
			"type":      "user",
			"uuid":      "transcript-think-user-1",
			"sessionId": stringPointer(t, sessionValue.SessionID),
			"timestamp": thinkingTranscriptBaseTime.Format(time.RFC3339Nano),
			"message": map[string]any{
				"role":    "user",
				"content": "今天天气怎么样呀",
			},
		},
		{
			"type":       "assistant",
			"uuid":       "assistant-think-1",
			"sessionId":  stringPointer(t, sessionValue.SessionID),
			"parentUuid": "transcript-think-user-1",
			"timestamp":  thinkingTranscriptBaseTime.Add(200 * time.Millisecond).Format(time.RFC3339Nano),
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "thinking", "thinking": "先分析 再收口"},
					{"type": "text", "text": "今天天气 很不错"},
				},
			},
		},
	})
	messages := readChatSessionHistory(t, cfg, service, sessionKey)
	if len(messages) != 3 {
		t.Fatalf("期望 3 条消息，实际 %d", len(messages))
	}
	historyBlocks := contentBlocksFromPayload(t, messages[1])
	if len(historyBlocks) != 2 || historyBlocks[0]["type"] != "thinking" || historyBlocks[1]["type"] != "text" {
		t.Fatalf("历史 assistant 内容块不正确: %+v", messages[1])
	}
	if messages[1]["stream_status"] != "done" {
		t.Fatalf("历史 assistant stream_status 未收口: %+v", messages[1])
	}
}

func TestServiceHandleChatForwardsRuntimeOptions(t *testing.T) {
	cfg := newChatTestConfig(t)
	migrateChatSQLite(t, cfg.DatabaseURL)

	agentService := newChatAgentService(t, cfg)
	maxThinkingTokens := 2048
	maxTurns := 6
	providerService := newChatProviderService(t, cfg)
	if _, err := providerService.Create(context.Background(), providercfg.CreateInput{
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
	updatedAgent, err := agentService.UpdateAgent(context.Background(), cfg.DefaultAgentID, agentsvc.UpdateRequest{
		Options: &agentsvc.Options{
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
	service.SetProviderResolver(providerService)
	sender := newChatTestSender("sender-no-model")
	sessionKey := "agent:nexus:ws:dm:no-model"
	permission.BindSession(sessionKey, sender, "client-no-model", true)

	if err := service.HandleChat(context.Background(), Request{
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
	if options.Model != "" {
		t.Fatalf("runtime 不应向 SDK 透传 agent model: %+v", options)
	}
	if options.Env["ANTHROPIC_MODEL"] != "glm-5.1" {
		t.Fatalf("runtime 未注入 provider model: %+v", options.Env)
	}
	if options.Env["ANTHROPIC_DEFAULT_SONNET_MODEL"] != "glm-5.1" {
		t.Fatalf("runtime 未注入默认 sonnet model: %+v", options.Env)
	}
	if options.Env["CLAUDE_CODE_SUBAGENT_MODEL"] != "glm-5.1" {
		t.Fatalf("runtime 未注入 subagent model: %+v", options.Env)
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
	if !options.IncludePartialMessages {
		t.Fatalf("runtime 未开启 partial messages: %+v", options)
	}
}

func TestServiceHandleChatUsesExplicitProvider(t *testing.T) {
	cfg := newChatTestConfig(t)
	migrateChatSQLite(t, cfg.DatabaseURL)

	agentService := newChatAgentService(t, cfg)
	providerService := newChatProviderService(t, cfg)
	if _, err := providerService.Create(context.Background(), providercfg.CreateInput{
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
	if _, err := providerService.Create(context.Background(), providercfg.CreateInput{
		Provider:    "kimi",
		DisplayName: "Kimi",
		AuthToken:   "kimi-token",
		BaseURL:     "https://api.moonshot.cn/anthropic",
		Model:       "kimi-k2.5",
		Enabled:     true,
		IsDefault:   false,
	}); err != nil {
		t.Fatalf("创建显式 provider 失败: %v", err)
	}

	created, err := agentService.CreateAgent(context.Background(), agentsvc.CreateRequest{
		Name: "显式 Provider 助手",
		Options: &agentsvc.Options{
			Provider: "kimi",
		},
	})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}

	permission := permissionctx.NewContext()
	client := newFakeChatClient()
	client.onQuery = func(_ context.Context, _ string) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-explicit-provider",
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
	service.SetProviderResolver(providerService)
	sessionKey := "agent:" + created.AgentID + ":ws:dm:explicit-provider"
	sender := newChatTestSender("sender-explicit-provider")
	permission.BindSession(sessionKey, sender, "client-explicit-provider", true)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		AgentID:    created.AgentID,
		Content:    "测试显式 provider",
		RoundID:    "round-explicit-provider",
		ReqID:      "round-explicit-provider",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	options := factory.LastOptions()
	if options.Env["ANTHROPIC_MODEL"] != "kimi-k2.5" {
		t.Fatalf("显式 provider 未命中新 provider model: %+v", options.Env)
	}
	if options.Env["ANTHROPIC_BASE_URL"] != "https://api.moonshot.cn/anthropic" {
		t.Fatalf("显式 provider 未命中新 provider base_url: %+v", options.Env)
	}
	if !options.IncludePartialMessages {
		t.Fatalf("显式 provider runtime 未开启 partial messages: %+v", options)
	}
}

func TestServiceHandleChatUsesPersistedSessionIDAsResume(t *testing.T) {
	cfg := newChatTestConfig(t)
	migrateChatSQLite(t, cfg.DatabaseURL)

	agentService := newChatAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeChatClient()
	client.onQuery = func(_ context.Context, _ string) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-resume",
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
	sender := newChatTestSender("sender-resume")
	sessionKey := "agent:nexus:ws:dm:resume-chat"
	permission.BindSession(sessionKey, sender, "client-resume", true)

	resumeID := "sdk-resume-chat-1"
	now := time.Now().UTC()
	if _, err := service.files.UpsertSession(filepath.Join(cfg.WorkspacePath, cfg.DefaultAgentID), session.Session{
		SessionKey:   sessionKey,
		AgentID:      cfg.DefaultAgentID,
		SessionID:    &resumeID,
		ChannelType:  "websocket",
		ChatType:     "dm",
		Status:       "active",
		CreatedAt:    now,
		LastActivity: now,
		Title:        "Resume Chat",
		MessageCount: 0,
		Options: map[string]any{
			sessionmodel.OptionHistorySource: sessionmodel.HistorySourceTranscript,
		},
		IsActive: true,
	}); err != nil {
		t.Fatalf("预写入会话 meta 失败: %v", err)
	}

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "测试 resume",
		RoundID:    "round-resume",
		ReqID:      "round-resume",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	options := factory.LastOptions()
	if options.Resume != resumeID {
		t.Fatalf("runtime 未将持久化 session_id 作为 resume 透传: %+v", options)
	}
}

func TestServiceHandleChatRejectsLegacySessionHistory(t *testing.T) {
	cfg := newChatTestConfig(t)
	migrateChatSQLite(t, cfg.DatabaseURL)

	agentService := newChatAgentService(t, cfg)
	permission := permissionctx.NewContext()
	runtimeManager := runtimectx.NewManagerWithFactory(&fakeChatFactory{client: newFakeChatClient()})
	service := NewService(cfg, agentService, runtimeManager, permission)

	sessionKey := "agent:nexus:ws:dm:legacy-chat"
	now := time.Now().UTC()
	if _, err := service.files.UpsertSession(filepath.Join(cfg.WorkspacePath, cfg.DefaultAgentID), session.Session{
		SessionKey:   sessionKey,
		AgentID:      cfg.DefaultAgentID,
		ChannelType:  "websocket",
		ChatType:     "dm",
		Status:       "active",
		CreatedAt:    now,
		LastActivity: now,
		Title:        "Legacy Chat",
		MessageCount: 0,
		Options:      map[string]any{},
		IsActive:     true,
	}); err != nil {
		t.Fatalf("预写入 legacy 会话失败: %v", err)
	}

	err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "测试 legacy",
		RoundID:    "round-legacy",
		ReqID:      "round-legacy",
	})
	if !errors.Is(err, sessionmodel.ErrLegacyHistoryUnsupported) {
		t.Fatalf("期望 legacy 会话被拒绝，实际错误: %v", err)
	}
}

func TestServiceHandleInterruptEmitsInterruptedRound(t *testing.T) {
	cfg := newChatTestConfig(t)
	migrateChatSQLite(t, cfg.DatabaseURL)

	agentService := newChatAgentService(t, cfg)
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

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "你好",
		RoundID:    "round-2",
		ReqID:      "round-2",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}
	waitForEvent(t, sender.events, protocol.EventTypeRoundStatus, "running")

	if err := service.HandleInterrupt(context.Background(), InterruptRequest{SessionKey: sessionKey}); err != nil {
		t.Fatalf("HandleInterrupt 失败: %v", err)
	}

	events := collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "interrupted"
	})
	assertContainsRoundStatus(t, events, "interrupted")
	assertContainsResultSubtype(t, events, "interrupted")

	client.mu.Lock()
	interruptCalls := client.interruptCalls
	client.mu.Unlock()
	if interruptCalls == 0 {
		t.Fatal("期望 fake client 收到 interrupt")
	}

	sessionValue, workspacePath := mustFindChatSession(t, service, cfg, sessionKey)
	writeTranscriptFixture(t, cfg, workspacePath, stringPointer(t, sessionValue.SessionID), []map[string]any{
		{
			"type":      "user",
			"uuid":      "interrupt-user-1",
			"sessionId": stringPointer(t, sessionValue.SessionID),
			"timestamp": time.Now().Add(-time.Second).UTC().Format(time.RFC3339Nano),
			"message": map[string]any{
				"role":    "user",
				"content": "你好",
			},
		},
	})
	messages := readChatSessionHistory(t, cfg, service, sessionKey)
	if len(messages) != 2 {
		t.Fatalf("中断后消息数量不正确: got=%d want=2 messages=%+v", len(messages), messages)
	}
	if messages[1]["role"] != "result" || messages[1]["subtype"] != "interrupted" {
		t.Fatalf("中断后未落 interrupted result: %+v", messages)
	}
}

func TestServiceHandleChatPersistsStructuredChannelMetadata(t *testing.T) {
	cfg := newChatTestConfig(t)
	migrateChatSQLite(t, cfg.DatabaseURL)

	agentService := newChatAgentService(t, cfg)
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

	if err := service.HandleChat(context.Background(), Request{
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

func TestServiceHandleChatFailsRoundWhenStreamEndsWithoutTerminalResult(t *testing.T) {
	cfg := newChatTestConfig(t)
	migrateChatSQLite(t, cfg.DatabaseURL)

	agentService := newChatAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeChatClient()
	client.onQuery = func(_ context.Context, _ string) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeStreamEvent,
				SessionID: client.sessionID,
				Stream: &sdkprotocol.StreamEvent{
					Event: map[string]any{
						"type": "message_start",
						"message": map[string]any{
							"id":    "assistant-premature",
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
			close(client.messages)
		}()
	}

	factory := &fakeChatFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sender := newChatTestSender("sender-premature")
	sessionKey := "agent:nexus:ws:dm:premature-close"
	permission.BindSession(sessionKey, sender, "client-premature", true)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "测试提前结束",
		RoundID:    "round-premature",
		ReqID:      "round-premature",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	events := collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "error"
	})

	assertContainsRoundStatus(t, events, "error")
	assertContainsStreamEventType(t, events, "message_start")
	assertContainsStreamEventType(t, events, "content_block_delta")
	assertContainsResultSubtype(t, events, "error")
	assertContainsErrorEventForMessage(t, events, "assistant-premature")
}

func newChatTestConfig(t *testing.T) config.Config {
	t.Helper()
	root := t.TempDir()
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(root, ".nexus"))
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

var chatTranscriptSanitizePattern = regexp.MustCompile(`[^a-zA-Z0-9]`)

func mustFindChatSession(
	t *testing.T,
	service *Service,
	cfg config.Config,
	sessionKey string,
) (session.Session, string) {
	t.Helper()
	item, workspacePath, err := service.files.FindSession([]string{filepath.Join(cfg.WorkspacePath, cfg.DefaultAgentID)}, sessionKey)
	if err != nil {
		t.Fatalf("读取 session 元数据失败: %v", err)
	}
	if item == nil {
		t.Fatalf("session 元数据不存在: %s", sessionKey)
	}
	return *item, workspacePath
}

func readChatSessionHistory(
	t *testing.T,
	cfg config.Config,
	service *Service,
	sessionKey string,
) []session.Message {
	t.Helper()
	sessionValue, workspacePath := mustFindChatSession(t, service, cfg, sessionKey)
	historyStore := workspacestore.NewAgentHistoryStore(cfg.WorkspacePath)
	rows, err := historyStore.ReadMessages(workspacePath, sessionValue, nil)
	if err != nil {
		t.Fatalf("读取 transcript 历史失败: %v", err)
	}
	return rows
}

func writeTranscriptFixture(
	t *testing.T,
	cfg config.Config,
	workspacePath string,
	sessionID string,
	rows []map[string]any,
) {
	t.Helper()
	if strings.TrimSpace(sessionID) == "" {
		t.Fatal("session_id 为空，无法写入 transcript fixture")
	}
	projectDir := filepath.Join(
		os.Getenv("NEXUS_CONFIG_DIR"),
		"projects",
		sanitizeChatTranscriptPath(canonicalizeChatTranscriptPath(workspacePath)),
	)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("创建 transcript 目录失败: %v", err)
	}
	file, err := os.Create(filepath.Join(projectDir, sessionID+".jsonl"))
	if err != nil {
		t.Fatalf("创建 transcript fixture 失败: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	for _, row := range rows {
		if err := encoder.Encode(row); err != nil {
			t.Fatalf("写入 transcript fixture 失败: %v", err)
		}
	}
}

func canonicalizeChatTranscriptPath(path string) string {
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

func sanitizeChatTranscriptPath(path string) string {
	const maxLength = 200
	sanitized := chatTranscriptSanitizePattern.ReplaceAllString(path, "-")
	if len(sanitized) <= maxLength {
		return sanitized
	}
	return sanitized[:maxLength] + "-" + chatTranscriptHash(path)
}

func chatTranscriptHash(value string) string {
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

func stringPointer(t *testing.T, value *string) string {
	t.Helper()
	if value == nil || strings.TrimSpace(*value) == "" {
		t.Fatal("session_id 未持久化")
	}
	return strings.TrimSpace(*value)
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

func assertContainsStreamEventType(t *testing.T, events []protocol.EventMessage, streamType string) {
	t.Helper()
	for _, event := range events {
		if event.EventType == protocol.EventTypeStream && event.Data["type"] == streamType {
			return
		}
	}
	t.Fatalf("未找到 stream.type=%s: %+v", streamType, events)
}

func assertContainsResultSubtype(t *testing.T, events []protocol.EventMessage, subtype string) {
	t.Helper()
	for _, event := range events {
		if event.EventType == protocol.EventTypeMessage &&
			event.Data["role"] == "result" &&
			event.Data["subtype"] == subtype {
			return
		}
	}
	t.Fatalf("未找到 result.subtype=%s: %+v", subtype, events)
}

func assertContainsErrorEventForMessage(t *testing.T, events []protocol.EventMessage, messageID string) {
	t.Helper()
	for _, event := range events {
		if event.EventType == protocol.EventTypeError && event.MessageID == messageID {
			return
		}
	}
	t.Fatalf("未找到绑定消息 %s 的 error 事件: %+v", messageID, events)
}

func assertStreamBlockIndex(t *testing.T, events []protocol.EventMessage, blockType string, expectedIndex int) {
	t.Helper()
	for _, event := range events {
		if event.EventType != protocol.EventTypeStream {
			continue
		}
		contentBlock, ok := event.Data["content_block"].(map[string]any)
		if !ok || contentBlock["type"] != blockType {
			continue
		}
		if event.Data["index"] != expectedIndex {
			t.Fatalf("%s stream index 不正确: got=%v want=%d event=%+v", blockType, event.Data["index"], expectedIndex, event)
		}
		return
	}
	t.Fatalf("未找到 block_type=%s 的 stream 事件: %+v", blockType, events)
}

func findAssistantMessagePayload(t *testing.T, events []protocol.EventMessage, messageID string) session.Message {
	t.Helper()
	for _, event := range events {
		if event.EventType != protocol.EventTypeMessage || event.MessageID != messageID {
			continue
		}
		if event.Data["role"] != "assistant" {
			continue
		}
		return session.Message(event.Data)
	}
	t.Fatalf("未找到 assistant message_id=%s 的 durable 消息: %+v", messageID, events)
	return nil
}

func contentBlocksFromPayload(t *testing.T, payload map[string]any) []map[string]any {
	t.Helper()
	rawBlocks, ok := payload["content"]
	if !ok {
		t.Fatalf("消息缺少 content: %+v", payload)
	}
	switch typed := rawBlocks.(type) {
	case []map[string]any:
		return typed
	case []any:
		result := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			block, ok := item.(map[string]any)
			if !ok {
				t.Fatalf("content block 类型不正确: %+v", payload)
			}
			result = append(result, block)
		}
		return result
	default:
		t.Fatalf("content 类型不正确: %+v", payload)
		return nil
	}
}
