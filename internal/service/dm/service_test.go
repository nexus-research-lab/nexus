package dm

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
	sqliterepo "github.com/nexus-research-lab/nexus/internal/storage/sqlite"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

type fakeDMClient struct {
	mu              sync.Mutex
	sessionID       string
	messages        chan sdkprotocol.ReceivedMessage
	interruptCalls  int
	disconnectCalls int
	disconnectErrs  []error
	connectErrors   []error
	queryErrors     []error
	sentContents    []string
	reconfigureOps  []agentclient.Options
	onQuery         func(context.Context, string)
	onInterrupt     func(context.Context)
}

func newFakeDMClient() *fakeDMClient {
	return &fakeDMClient{
		sessionID: "sdk-session-1",
		messages:  make(chan sdkprotocol.ReceivedMessage, 16),
	}
}

func (c *fakeDMClient) Connect(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.connectErrors) == 0 {
		return nil
	}
	err := c.connectErrors[0]
	c.connectErrors = c.connectErrors[1:]
	return err
}

func (c *fakeDMClient) Query(ctx context.Context, prompt string) error {
	if c.onQuery != nil {
		c.onQuery(ctx, prompt)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.queryErrors) > 0 {
		err := c.queryErrors[0]
		c.queryErrors = c.queryErrors[1:]
		return err
	}
	return ctx.Err()
}

func (c *fakeDMClient) ReceiveMessages(context.Context) <-chan sdkprotocol.ReceivedMessage {
	return c.messages
}

func (c *fakeDMClient) SendContent(_ context.Context, content any, _ *string, _ string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sentContents = append(c.sentContents, normalizeTestString(content))
	return nil
}

func (c *fakeDMClient) Interrupt(ctx context.Context) error {
	c.mu.Lock()
	c.interruptCalls++
	callback := c.onInterrupt
	c.mu.Unlock()
	if callback != nil {
		callback(ctx)
	}
	return nil
}

func (c *fakeDMClient) Disconnect(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.disconnectCalls++
	if len(c.disconnectErrs) > 0 {
		err := c.disconnectErrs[0]
		c.disconnectErrs = c.disconnectErrs[1:]
		return err
	}
	return nil
}

func (c *fakeDMClient) Reconfigure(_ context.Context, options agentclient.Options) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reconfigureOps = append(c.reconfigureOps, options)
	return nil
}

func (c *fakeDMClient) SessionID() string { return c.sessionID }

func normalizeTestString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}

type fakeDMFactory struct {
	mu      sync.Mutex
	client  *fakeDMClient
	clients []*fakeDMClient
	options []agentclient.Options
}

func (f *fakeDMFactory) New(options agentclient.Options) runtimectx.Client {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.options = append(f.options, options)
	if len(f.clients) > 0 {
		client := f.clients[0]
		f.clients = f.clients[1:]
		return client
	}
	return f.client
}

func (f *fakeDMFactory) LastOptions() agentclient.Options {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.options) == 0 {
		return agentclient.Options{}
	}
	return f.options[len(f.options)-1]
}

func (f *fakeDMFactory) OptionAt(index int) agentclient.Options {
	f.mu.Lock()
	defer f.mu.Unlock()
	if index < 0 || index >= len(f.options) {
		return agentclient.Options{}
	}
	return f.options[index]
}

type dmTestSender struct {
	key    string
	events chan protocol.EventMessage
}

func newDMTestSender(key string) *dmTestSender {
	return &dmTestSender{
		key:    key,
		events: make(chan protocol.EventMessage, 32),
	}
}

func (s *dmTestSender) Key() string    { return s.key }
func (s *dmTestSender) IsClosed() bool { return false }
func (s *dmTestSender) SendEvent(_ context.Context, event protocol.EventMessage) error {
	s.events <- event
	return nil
}

func newDMAgentService(t *testing.T, cfg config.Config) *agentsvc.Service {
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

func newDMProviderService(t *testing.T, cfg config.Config) *providercfg.Service {
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
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
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

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sender := newDMTestSender("sender-1")
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

	sessionValue, workspacePath := mustFindDMSession(t, service, cfg, sessionKey)
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
	messages := readDMSessionHistory(t, cfg, service, sessionKey)
	if len(messages) != 2 {
		t.Fatalf("期望 2 条消息，实际 %d", len(messages))
	}
	if messages[0]["role"] != "user" || messages[1]["role"] != "assistant" {
		t.Fatalf("消息角色顺序不正确: %+v", messages)
	}
	summary, ok := messages[1]["result_summary"].(map[string]any)
	if !ok || anyToString(summary["result"]) != "done" || anyToInt(summary["duration_ms"]) != 12 {
		t.Fatalf("result 摘要应挂在 assistant 上: %+v", messages[1])
	}
	usage, _ := summary["usage"].(map[string]any)
	outputTokens := anyToInt(usage["output_tokens"])
	if outputTokens != 5 {
		t.Fatalf("result usage 应保留: %+v", messages[1])
	}
}

func TestServiceEnsureClientInjectsRuntimePrompt(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	created, err := agentService.CreateAgent(context.Background(), protocol.CreateRequest{Name: "提示词助手"})
	if err != nil {
		t.Fatalf("创建测试 agent 失败: %v", err)
	}
	if err = os.WriteFile(
		filepath.Join(created.WorkspacePath, "AGENTS.md"),
		[]byte("# AGENTS.md\n\n执行规则：必须先加载工作区规则。\n"),
		0o644,
	); err != nil {
		t.Fatalf("写入 AGENTS.md 失败: %v", err)
	}

	db, err := sql.Open("sqlite3", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() {
		_ = db.Close()
	}()
	if _, err = db.Exec(`UPDATE profiles SET headline = ?, profile_markdown = ? WHERE agent_id = ?`,
		"擅长规则执行",
		"## 详细档案\n- 运行前先汇总 workspace 规则。",
		created.AgentID,
	); err != nil {
		t.Fatalf("更新 profile 失败: %v", err)
	}

	agentValue, err := agentService.GetAgent(context.Background(), created.AgentID)
	if err != nil {
		t.Fatalf("读取测试 agent 失败: %v", err)
	}

	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)

	sessionKey := protocol.BuildAgentSessionKey(created.AgentID, protocol.SessionChannelWebSocketSegment, "dm", "prompt-ref", "")
	parsed := protocol.ParseSessionKey(sessionKey)
	sessionItem, err := service.ensureSession(context.Background(), agentValue, parsed, sessionKey)
	if err != nil {
		t.Fatalf("初始化 session 失败: %v", err)
	}
	if _, _, _, err = service.ensureClient(context.Background(), sessionKey, agentValue, sessionItem, Request{
		SessionKey:     sessionKey,
		PermissionMode: sdkprotocol.PermissionModeDefault,
	}); err != nil {
		t.Fatalf("构建 runtime client 失败: %v", err)
	}

	appendSystemPrompt := factory.LastOptions().AppendSystemPrompt
	if !strings.Contains(appendSystemPrompt, "执行规则：必须先加载工作区规则") {
		t.Fatalf("runtime prompt 未注入 AGENTS.md 内容: %s", appendSystemPrompt)
	}
	if !strings.Contains(appendSystemPrompt, "擅长规则执行") {
		t.Fatalf("runtime prompt 未注入 Agent headline: %s", appendSystemPrompt)
	}
	if !strings.Contains(appendSystemPrompt, "运行前先汇总 workspace 规则") {
		t.Fatalf("runtime prompt 未注入 Agent profile_markdown: %s", appendSystemPrompt)
	}
}

func TestServiceHandleChatKeepsThinkingDuringStreamingAndHistoryReplay(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
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
							sdkprotocol.TextBlock{Text: "今天天气 很不错"},
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

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sender := newDMTestSender("sender-think-stream")
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

	sessionValue, workspacePath := mustFindDMSession(t, service, cfg, sessionKey)
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
	messages := readDMSessionHistory(t, cfg, service, sessionKey)
	if len(messages) != 2 {
		t.Fatalf("期望 2 条消息，实际 %d", len(messages))
	}
	historyBlocks := contentBlocksFromPayload(t, messages[1])
	if len(historyBlocks) != 2 || historyBlocks[0]["type"] != "thinking" || historyBlocks[1]["type"] != "text" {
		t.Fatalf("历史 assistant 内容块不正确: %+v", messages[1])
	}
	if _, exists := messages[1]["stream_status"]; exists {
		t.Fatalf("历史 assistant 不应携带 stream_status: %+v", messages[1])
	}
	if _, ok := messages[1]["result_summary"].(map[string]any); !ok {
		t.Fatalf("历史 assistant 应挂载 result 摘要: %+v", messages[1])
	}
}

func TestServiceHandleChatForwardsRuntimeOptions(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	maxThinkingTokens := 2048
	maxTurns := 6
	providerService := newDMProviderService(t, cfg)
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
	updatedAgent, err := agentService.UpdateAgent(context.Background(), cfg.DefaultAgentID, protocol.UpdateRequest{
		Options: &protocol.Options{
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
	client := newFakeDMClient()
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

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	service.SetProviderResolver(providerService)
	sender := newDMTestSender("sender-no-model")
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
	if options.Model != "glm-5.1" {
		t.Fatalf("runtime 未向 SDK options 透传 provider model: %+v", options)
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

func TestServiceHandleChatBypassPermissionsDoesNotInstallPermissionHandler(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	maxTurns := 4
	agentValue, err := agentService.UpdateAgent(context.Background(), cfg.DefaultAgentID, protocol.UpdateRequest{
		Options: &protocol.Options{
			PermissionMode: "bypassPermissions",
			MaxTurns:       &maxTurns,
			SettingSources: []string{"project"},
		},
	})
	if err != nil || agentValue == nil {
		t.Fatalf("更新 agent 失败: value=%+v err=%v", agentValue, err)
	}

	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, _ string) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-bypass",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "success",
					DurationMS: 1,
					NumTurns:   1,
					Result:     "ok",
				},
			}
		}()
	}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sender := newDMTestSender("sender-bypass")
	sessionKey := "agent:nexus:ws:dm:bypass"
	permission.BindSession(sessionKey, sender, "client-bypass", true)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "测试 bypass 权限处理器",
		RoundID:    "round-bypass",
		ReqID:      "round-bypass",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}
	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	options := factory.LastOptions()
	if options.PermissionMode != sdkprotocol.PermissionModeBypassPermissions {
		t.Fatalf("bypass 权限模式未透传: %+v", options)
	}
	if options.PermissionHandler != nil {
		t.Fatalf("bypass 权限模式不应安装 permission handler: %+v", options)
	}
}

func TestServiceHandleChatUsesExplicitProvider(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	providerService := newDMProviderService(t, cfg)
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

	created, err := agentService.CreateAgent(context.Background(), protocol.CreateRequest{
		Name: "显式 Provider 助手",
		Options: &protocol.Options{
			Provider: "kimi",
		},
	})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}

	permission := permissionctx.NewContext()
	client := newFakeDMClient()
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

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	service.SetProviderResolver(providerService)
	sessionKey := "agent:" + created.AgentID + ":ws:dm:explicit-provider"
	sender := newDMTestSender("sender-explicit-provider")
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
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
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

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sender := newDMTestSender("sender-resume")
	sessionKey := "agent:nexus:ws:dm:resume-chat"
	permission.BindSession(sessionKey, sender, "client-resume", true)

	resumeID := "sdk-resume-chat-1"
	now := time.Now().UTC()
	if _, err := service.files.UpsertSession(filepath.Join(cfg.WorkspacePath, cfg.DefaultAgentID), protocol.Session{
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
		Options:      map[string]any{},
		IsActive:     true,
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

func TestServiceHandleChatSkipsStaleSDKSessionWhenRuntimeModelFingerprintDiffers(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	providerService := newDMProviderService(t, cfg)
	if _, err := providerService.Create(context.Background(), providercfg.CreateInput{
		Provider:    "glm",
		DisplayName: "GLM",
		AuthToken:   "glm-token",
		BaseURL:     "https://open.bigmodel.cn/api/anthropic",
		Model:       "glm-5.1",
		Enabled:     true,
		IsDefault:   true,
	}); err != nil {
		t.Fatalf("创建 provider 失败: %v", err)
	}

	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.sessionID = "sdk-new-model"
	client.onQuery = func(_ context.Context, _ string) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-new-model",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "success",
					DurationMS: 1,
					NumTurns:   1,
					Result:     "ok",
				},
			}
		}()
	}
	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	service.SetProviderResolver(providerService)
	sender := newDMTestSender("sender-stale-model")
	sessionKey := "agent:nexus:ws:dm:stale-model"
	permission.BindSession(sessionKey, sender, "client-stale-model", true)

	staleResumeID := "sdk-old-model"
	now := time.Now().UTC()
	if _, err := service.files.UpsertSession(filepath.Join(cfg.WorkspacePath, cfg.DefaultAgentID), protocol.Session{
		SessionKey:   sessionKey,
		AgentID:      cfg.DefaultAgentID,
		SessionID:    &staleResumeID,
		ChannelType:  "websocket",
		ChatType:     "dm",
		Status:       "active",
		CreatedAt:    now,
		LastActivity: now,
		Title:        "Stale Model",
		Options: map[string]any{
			protocol.OptionRuntimeProvider: "glm",
			protocol.OptionRuntimeModel:    "old-model",
		},
		IsActive: true,
	}); err != nil {
		t.Fatalf("预写入会话 meta 失败: %v", err)
	}

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "测试过期模型 session 不 resume",
		RoundID:    "round-stale-model",
		ReqID:      "round-stale-model",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	options := factory.LastOptions()
	if options.Resume != "" {
		t.Fatalf("runtime 模型变更后不应 resume 过期 sdk session: %+v", options)
	}
	if options.Model != "glm-5.1" {
		t.Fatalf("runtime 应使用当前 provider model: %+v", options)
	}
	sessionValue, _ := mustFindDMSession(t, service, cfg, sessionKey)
	if stringPointer(t, sessionValue.SessionID) != "sdk-new-model" {
		t.Fatalf("新 sdk session_id 未回写: %+v", sessionValue)
	}
	if sessionValue.Options[protocol.OptionRuntimeModel] != "glm-5.1" {
		t.Fatalf("runtime model 指纹未回写: %+v", sessionValue.Options)
	}
}
func TestServiceHandleInterruptEmitsInterruptedRound(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, _ string) {}
	client.onInterrupt = func(_ context.Context) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-interrupted",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "interrupted",
					DurationMS: 1,
					NumTurns:   1,
				},
			}
		}()
	}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sender := newDMTestSender("sender-1")
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

	sessionValue, workspacePath := mustFindDMSession(t, service, cfg, sessionKey)
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
	messages := readDMSessionHistory(t, cfg, service, sessionKey)
	if len(messages) != 2 {
		t.Fatalf("中断后消息数量不正确: got=%d want=2 messages=%+v", len(messages), messages)
	}
	if messages[1]["role"] != "assistant" {
		t.Fatalf("中断后应返回合成 assistant: %+v", messages)
	}
	summary, ok := messages[1]["result_summary"].(map[string]any)
	if !ok || summary["subtype"] != "interrupted" {
		t.Fatalf("中断后未挂载 interrupted result_summary: %+v", messages)
	}
}

func TestServiceHandleChatQueuesRunningRoundByDefault(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, _ string) {}
	client.onInterrupt = func(_ context.Context) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-queue-cleanup",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "interrupted",
					DurationMS: 1,
					NumTurns:   1,
				},
			}
		}()
	}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sender := newDMTestSender("sender-queue")
	sessionKey := "agent:nexus:ws:dm:test-queue"
	permission.BindSession(sessionKey, sender, "client-queue", true)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "先做一个长任务",
		RoundID:    "round-queue-1",
		ReqID:      "round-queue-1",
	}); err != nil {
		t.Fatalf("第一轮 HandleChat 失败: %v", err)
	}
	waitForEvent(t, sender.events, protocol.EventTypeRoundStatus, "running")

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "这是补充要求",
		RoundID:    "round-queue-2",
		ReqID:      "round-queue-2",
	}); err != nil {
		t.Fatalf("第二条排队消息 HandleChat 失败: %v", err)
	}
	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeChatAck && event.Data["round_id"] == "round-queue-2"
	})

	client.mu.Lock()
	interruptCalls := client.interruptCalls
	sentContents := append([]string(nil), client.sentContents...)
	client.mu.Unlock()
	if interruptCalls != 0 {
		t.Fatalf("默认排队不应中断运行中 DM round: interruptCalls=%d", interruptCalls)
	}
	if len(sentContents) != 1 || sentContents[0] != "这是补充要求" {
		t.Fatalf("运行中 DM round 未收到排队输入: %+v", sentContents)
	}
	if len(factory.options) != 1 {
		t.Fatalf("排队输入不应创建新 runtime client: got=%d want=1", len(factory.options))
	}

	if err := service.HandleInterrupt(context.Background(), InterruptRequest{SessionKey: sessionKey}); err != nil {
		t.Fatalf("清理运行中 round 失败: %v", err)
	}
}

func TestServiceHandleChatGuidePolicyQueuesHookGuidance(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, _ string) {}
	client.onInterrupt = func(_ context.Context) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-guide-cleanup",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "interrupted",
					DurationMS: 1,
					NumTurns:   1,
				},
			}
		}()
	}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sender := newDMTestSender("sender-guide")
	sessionKey := "agent:nexus:ws:dm:test-guide"
	permission.BindSession(sessionKey, sender, "client-guide", true)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "先查一下项目结构",
		RoundID:    "round-guide-1",
		ReqID:      "round-guide-1",
	}); err != nil {
		t.Fatalf("第一轮 HandleChat 失败: %v", err)
	}
	waitForEvent(t, sender.events, protocol.EventTypeRoundStatus, "running")

	if err := service.HandleChat(context.Background(), Request{
		SessionKey:           sessionKey,
		Content:              "等工具结果回来后优先看错误日志",
		RoundID:              "round-guide-2",
		ReqID:                "round-guide-2",
		DeliveryPolicy:       protocol.ChatDeliveryPolicyGuide,
		BroadcastUserMessage: true,
	}); err != nil {
		t.Fatalf("引导消息 HandleChat 失败: %v", err)
	}
	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeChatAck && event.Data["round_id"] == "round-guide-2"
	})
	guidanceEvents := collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeMessage && event.Data["role"] == "system"
	})
	guidanceEvent := guidanceEvents[len(guidanceEvents)-1]
	if guidanceEvent.Data["round_id"] != "round-guide-1" || guidanceEvent.Data["message_id"] != "round-guide-2" {
		t.Fatalf("引导消息应归入运行中的 round: %+v", guidanceEvent.Data)
	}
	if guidanceEvent.DeliveryMode != "ephemeral" {
		t.Fatalf("引导消息只应作为实时展示事件广播: %+v", guidanceEvent)
	}
	metadata, _ := guidanceEvent.Data["metadata"].(map[string]any)
	if metadata["subtype"] != "guided_input" {
		t.Fatalf("引导消息缺少 typed metadata: %+v", guidanceEvent.Data)
	}

	client.mu.Lock()
	interruptCalls := client.interruptCalls
	sentContents := append([]string(nil), client.sentContents...)
	client.mu.Unlock()
	if interruptCalls != 0 {
		t.Fatalf("引导不应中断运行中 DM round: interruptCalls=%d", interruptCalls)
	}
	if len(sentContents) != 0 {
		t.Fatalf("引导不应走普通 streaming input: %+v", sentContents)
	}
	if count := runtimeManager.PendingGuidanceCount(sessionKey); count != 1 {
		t.Fatalf("引导输入未登记到运行时队列: count=%d", count)
	}
	if len(factory.options) != 1 {
		t.Fatalf("引导不应创建新 runtime client: got=%d want=1", len(factory.options))
	}
	if matchers := factory.options[0].Hooks[sdkprotocol.HookEventPostToolUse]; len(matchers) == 0 {
		t.Fatalf("runtime options 未挂载 PostToolUse 引导 hook: %+v", factory.options[0].Hooks)
	}
	rows := readDMSessionHistory(t, cfg, service, sessionKey)
	for _, row := range rows {
		if row["message_id"] == "round-guide-2" {
			t.Fatalf("引导消息不应直接写入 overlay 历史，历史回放应来自 Claude transcript: %+v", rows)
		}
	}

	if err := service.HandleInterrupt(context.Background(), InterruptRequest{SessionKey: sessionKey}); err != nil {
		t.Fatalf("清理运行中 round 失败: %v", err)
	}
}

func TestServiceInputQueueGuideWaitsForPostToolUse(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, _ string) {}
	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sender := newDMTestSender("sender-guide-input-queue")
	sessionKey := "agent:nexus:ws:dm:test-guide-input-queue"
	permission.BindSession(sessionKey, sender, "client-guide-input-queue", true)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "先查项目",
		RoundID:    "round-guide-input-queue-1",
		ReqID:      "round-guide-input-queue-1",
	}); err != nil {
		t.Fatalf("启动运行中 DM round 失败: %v", err)
	}
	waitForEvent(t, sender.events, protocol.EventTypeRoundStatus, "running")

	if err := service.HandleInputQueue(context.Background(), InputQueueRequest{
		SessionKey:     sessionKey,
		Action:         "enqueue",
		Content:        "路径发给我吧",
		DeliveryPolicy: protocol.ChatDeliveryPolicyQueue,
	}); err != nil {
		t.Fatalf("写入 DM 待发送队列失败: %v", err)
	}
	_, location, err := service.resolveInputQueueLocation(context.Background(), sessionKey, "")
	if err != nil {
		t.Fatalf("解析 DM 队列位置失败: %v", err)
	}
	items, err := service.inputQueue.Snapshot(location)
	if err != nil {
		t.Fatalf("读取 DM 待发送队列失败: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("待发送队列应保留一条消息: %+v", items)
	}
	itemID := items[0].ID

	if err := service.HandleInputQueue(context.Background(), InputQueueRequest{
		SessionKey: sessionKey,
		Action:     "guide",
		ItemID:     itemID,
	}); err != nil {
		t.Fatalf("标记 DM 引导队列失败: %v", err)
	}
	items, err = service.inputQueue.Snapshot(location)
	if err != nil {
		t.Fatalf("读取标记后的 DM 待发送队列失败: %v", err)
	}
	if len(items) != 1 ||
		items[0].ID != itemID ||
		items[0].DeliveryPolicy != protocol.ChatDeliveryPolicyGuide ||
		items[0].RootRoundID != "round-guide-input-queue-1" {
		t.Fatalf("点击引导后应保留队列项并绑定当前 round: %+v", items)
	}

	var additionalContext string
	for _, matcher := range factory.options[0].Hooks[sdkprotocol.HookEventPostToolUse] {
		for _, hook := range matcher.Hooks {
			output, hookErr := hook(context.Background(), sdkprotocol.HookInput{
				EventName: sdkprotocol.HookEventPostToolUse,
			}, "tool-1")
			if hookErr != nil {
				t.Fatalf("执行 PostToolUse hook 失败: %v", hookErr)
			}
			if text, _ := output.HookSpecificOutput["additionalContext"].(string); text != "" {
				additionalContext = text
			}
		}
	}
	if !strings.Contains(additionalContext, "路径发给我吧") ||
		!strings.Contains(additionalContext, "queue_"+itemID) {
		t.Fatalf("PostToolUse hook 未注入队列引导: %q", additionalContext)
	}
	items, err = service.inputQueue.Snapshot(location)
	if err != nil {
		t.Fatalf("读取消费后的 DM 待发送队列失败: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("PostToolUse 真正注入后才应消费队列项: %+v", items)
	}

	if err := service.HandleInterrupt(context.Background(), InterruptRequest{SessionKey: sessionKey}); err != nil {
		t.Fatalf("清理运行中 round 失败: %v", err)
	}
}

func TestServiceHandleChatInterruptPolicyStopsRunningRound(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, prompt string) {
		if strings.Contains(prompt, "第二轮") {
			go func() {
				client.messages <- sdkprotocol.ReceivedMessage{
					Type:      sdkprotocol.MessageTypeResult,
					SessionID: client.sessionID,
					UUID:      "result-interrupt-policy",
					Result: &sdkprotocol.ResultMessage{
						Subtype:    "success",
						DurationMS: 1,
						NumTurns:   1,
						Result:     "ok",
					},
				}
			}()
		}
	}
	client.onInterrupt = func(_ context.Context) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-interrupt-policy-old",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "interrupted",
					DurationMS: 1,
					NumTurns:   1,
				},
			}
		}()
	}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sender := newDMTestSender("sender-interrupt-policy")
	sessionKey := "agent:nexus:ws:dm:test-interrupt-policy"
	permission.BindSession(sessionKey, sender, "client-interrupt-policy", true)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "第一轮",
		RoundID:    "round-interrupt-policy-1",
		ReqID:      "round-interrupt-policy-1",
	}); err != nil {
		t.Fatalf("第一轮 HandleChat 失败: %v", err)
	}
	waitForEvent(t, sender.events, protocol.EventTypeRoundStatus, "running")

	if err := service.HandleChat(context.Background(), Request{
		SessionKey:     sessionKey,
		Content:        "第二轮",
		RoundID:        "round-interrupt-policy-2",
		ReqID:          "round-interrupt-policy-2",
		DeliveryPolicy: protocol.ChatDeliveryPolicyInterrupt,
	}); err != nil {
		t.Fatalf("打断策略 HandleChat 失败: %v", err)
	}

	events := collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus &&
			event.Data["round_id"] == "round-interrupt-policy-2" &&
			event.Data["status"] == "finished"
	})
	assertContainsRoundStatus(t, events, "finished")

	client.mu.Lock()
	interruptCalls := client.interruptCalls
	sentContents := append([]string(nil), client.sentContents...)
	client.mu.Unlock()
	if interruptCalls == 0 {
		t.Fatal("打断策略应中断运行中 DM round")
	}
	if len(sentContents) != 0 {
		t.Fatalf("打断策略不应走 streaming input: %+v", sentContents)
	}
}

func TestServiceHandleInterruptCoercesTerminalErrorIntoInterrupted(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.onQuery = func(_ context.Context, _ string) {}
	client.onInterrupt = func(_ context.Context) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-error-after-interrupt",
				Result: &sdkprotocol.ResultMessage{
					Subtype:       "error",
					DurationMS:    8,
					DurationAPIMS: 123,
					NumTurns:      2,
					Result:        "",
					IsError:       true,
				},
			}
		}()
	}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sender := newDMTestSender("sender-interrupt-error")
	sessionKey := "agent:nexus:ws:dm:test-interrupt-error"
	permission.BindSession(sessionKey, sender, "client-interrupt-error", true)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "停止测试",
		RoundID:    "round-interrupt-error",
		ReqID:      "round-interrupt-error",
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

	sessionValue, workspacePath := mustFindDMSession(t, service, cfg, sessionKey)
	writeTranscriptFixture(t, cfg, workspacePath, stringPointer(t, sessionValue.SessionID), []map[string]any{
		{
			"type":      "user",
			"uuid":      "interrupt-error-user-1",
			"sessionId": stringPointer(t, sessionValue.SessionID),
			"timestamp": time.Now().Add(-time.Second).UTC().Format(time.RFC3339Nano),
			"message": map[string]any{
				"role":    "user",
				"content": "停止测试",
			},
		},
	})
	messages := readDMSessionHistory(t, cfg, service, sessionKey)
	if len(messages) != 2 {
		t.Fatalf("中断错误收口后消息数量不正确: got=%d want=2 messages=%+v", len(messages), messages)
	}
	summary, ok := messages[1]["result_summary"].(map[string]any)
	if !ok {
		t.Fatalf("中断错误未挂载 result_summary: %+v", messages)
	}
	if summary["subtype"] != "interrupted" {
		t.Fatalf("中断错误应收口为 interrupted: %+v", summary)
	}
	if _, exists := summary["result"]; exists {
		t.Fatalf("中断错误不应再补默认文案: %+v", summary)
	}
}

func TestServiceHandleChatAfterInterruptKeepsSameClientAndConsumesExplicitStop(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()

	client := newFakeDMClient()
	client.sessionID = "sdk-interrupt-old"
	queryCount := 0
	client.onQuery = func(_ context.Context, _ string) {
		queryCount++
		if queryCount != 2 {
			return
		}
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-after-resume",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "success",
					DurationMS: 1,
					NumTurns:   1,
					Result:     "ok",
				},
			}
		}()
	}
	client.onInterrupt = func(_ context.Context) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-after-interrupt",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "interrupted",
					DurationMS: 1,
					NumTurns:   1,
				},
			}
		}()
	}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sender := newDMTestSender("sender-reconnect")
	sessionKey := "agent:nexus:ws:dm:test-interrupt-reconnect"
	permission.BindSession(sessionKey, sender, "client-reconnect", true)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "第一轮",
		RoundID:    "round-interrupt-1",
		ReqID:      "round-interrupt-1",
	}); err != nil {
		t.Fatalf("第一轮 HandleChat 失败: %v", err)
	}
	waitForEvent(t, sender.events, protocol.EventTypeRoundStatus, "running")

	if err := service.HandleInterrupt(context.Background(), InterruptRequest{SessionKey: sessionKey}); err != nil {
		t.Fatalf("HandleInterrupt 失败: %v", err)
	}
	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "interrupted"
	})

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "第二轮",
		RoundID:    "round-interrupt-2",
		ReqID:      "round-interrupt-2",
	}); err != nil {
		t.Fatalf("第二轮 HandleChat 失败: %v", err)
	}

	events := collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus &&
			event.Data["status"] == "finished" &&
			event.Data["round_id"] == "round-interrupt-2"
	})

	if len(factory.options) != 1 {
		t.Fatalf("只应创建一次 runtime client，第二轮应复用现有 client: got=%d want=1", len(factory.options))
	}
	if len(client.reconfigureOps) == 0 {
		t.Fatalf("第二轮应复用 client 并执行 reconfigure")
	}
	for _, event := range events {
		if event.EventType != protocol.EventTypeMessage {
			continue
		}
		if event.Data["round_id"] != "round-interrupt-2" {
			continue
		}
		summary, ok := event.Data["result_summary"].(map[string]any)
		if !ok {
			continue
		}
		if summary["subtype"] == "interrupted" {
			t.Fatalf("第二轮不应消费上一轮残留结果: %+v", events)
		}
	}
}

func TestServiceHandleChatPersistsStructuredChannelMetadata(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
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

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sender := newDMTestSender("sender-structured")
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
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
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

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	sender := newDMTestSender("sender-premature")
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

func newDMTestConfig(t *testing.T) config.Config {
	t.Helper()
	root := t.TempDir()
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(root, ".nexus"))
	return config.Config{
		Host:           "127.0.0.1",
		Port:           18032,
		ProjectName:    "nexus-dm-test",
		APIPrefix:      "/nexus/v1",
		WebSocketPath:  "/nexus/v1/chat/ws",
		DefaultAgentID: "nexus",
		WorkspacePath:  filepath.Join(root, "workspace"),
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(root, "nexus.db"),
	}
}

var dmTranscriptSanitizePattern = regexp.MustCompile(`[^a-zA-Z0-9]`)

func mustFindDMSession(
	t *testing.T,
	service *Service,
	cfg config.Config,
	sessionKey string,
) (protocol.Session, string) {
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

func readDMSessionHistory(
	t *testing.T,
	cfg config.Config,
	service *Service,
	sessionKey string,
) []protocol.Message {
	t.Helper()
	sessionValue, workspacePath := mustFindDMSession(t, service, cfg, sessionKey)
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
		sanitizeDMTranscriptPath(canonicalizeDMTranscriptPath(workspacePath)),
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

func canonicalizeDMTranscriptPath(path string) string {
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

func sanitizeDMTranscriptPath(path string) string {
	const maxLength = 200
	sanitized := dmTranscriptSanitizePattern.ReplaceAllString(path, "-")
	if len(sanitized) <= maxLength {
		return sanitized
	}
	return sanitized[:maxLength] + "-" + dmTranscriptHash(path)
}

func dmTranscriptHash(value string) string {
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

func migrateDMSQLite(t *testing.T, databaseURL string) {
	t.Helper()

	db, err := sql.Open("sqlite3", databaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer db.Close()

	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("设置 goose 方言失败: %v", err)
	}
	if err = goose.Up(db, dmMigrationDir(t)); err != nil {
		t.Fatalf("执行 migration 失败: %v", err)
	}
}

func dmMigrationDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations", "sqlite")
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
		if event.EventType != protocol.EventTypeMessage {
			continue
		}
		if event.Data["role"] == "result" && event.Data["subtype"] == subtype {
			return
		}
		if event.Data["role"] == "assistant" {
			summary, ok := event.Data["result_summary"].(map[string]any)
			if ok && summary["subtype"] == subtype {
				return
			}
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

func findAssistantMessagePayload(t *testing.T, events []protocol.EventMessage, messageID string) protocol.Message {
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

func anyToString(value any) string {
	if typed, ok := value.(string); ok {
		return typed
	}
	return ""
}
