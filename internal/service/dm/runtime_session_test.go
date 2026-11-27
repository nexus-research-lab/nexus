// runtime_session_test.go 覆盖 DM 运行时会话、恢复、配置准备集成与临时会话测试。
package dm

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
	usagesvc "github.com/nexus-research-lab/nexus/internal/service/usage"
	_ "modernc.org/sqlite"
)

type fakeDMClient struct {
	mu               sync.Mutex
	sessionID        string
	messages         chan sdkprotocol.ReceivedMessage
	interruptCalls   int
	interruptReasons []string
	connectCalls     int
	disconnectCalls  int
	interruptErrors  []error
	disconnectErrs   []error
	connectErrors    []error
	queryErrors      []error
	queryPrompts     []string
	removeMessages   [][]string
	sentContents     []string
	queryOptions     []sdkprotocol.OutboundMessageOptions
	reconfigureOps   []agentclient.Options
	hookResponseAck  bool
	runningInput     bool
	runningInputs    []fakeRunningInput
	onConnect        func(context.Context)
	onQuery          func(context.Context, string)
	onInterrupt      func(context.Context)
}

type fakeTokenUsageRecorder struct {
	inputs []usagesvc.RecordInput
}

func (r *fakeTokenUsageRecorder) RecordMessageUsage(_ context.Context, input usagesvc.RecordInput) error {
	r.inputs = append(r.inputs, input)
	return nil
}

type externalReplyCall struct {
	agentID string
	text    string
	target  ExternalReplyTarget
}

type fakeExternalReplyDispatcher struct {
	mu          sync.Mutex
	calls       []externalReplyCall
	typingCalls []externalTypingCall
	result      ExternalReplyResult
}

type externalTypingCall struct {
	agentID string
	target  ExternalReplyTarget
	active  bool
}

func (d *fakeExternalReplyDispatcher) DeliverExternalReply(
	_ context.Context,
	agentID string,
	text string,
	target ExternalReplyTarget,
) (ExternalReplyResult, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, externalReplyCall{
		agentID: agentID,
		text:    text,
		target:  target,
	})
	return d.result, nil
}

func (d *fakeExternalReplyDispatcher) callsSnapshot() []externalReplyCall {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]externalReplyCall(nil), d.calls...)
}

func (d *fakeExternalReplyDispatcher) SetExternalTyping(
	_ context.Context,
	agentID string,
	target ExternalReplyTarget,
	active bool,
) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.typingCalls = append(d.typingCalls, externalTypingCall{
		agentID: agentID,
		target:  target,
		active:  active,
	})
	return nil
}

func (d *fakeExternalReplyDispatcher) typingCallsSnapshot() []externalTypingCall {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]externalTypingCall(nil), d.typingCalls...)
}

func newFakeDMClient() *fakeDMClient {
	return &fakeDMClient{
		sessionID: "sdk-session-1",
		messages:  make(chan sdkprotocol.ReceivedMessage, 16),
	}
}

func (c *fakeDMClient) Connect(ctx context.Context) error {
	if c.onConnect != nil {
		c.onConnect(ctx)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connectCalls++
	if len(c.connectErrors) == 0 {
		return nil
	}
	err := c.connectErrors[0]
	c.connectErrors = c.connectErrors[1:]
	return err
}

func (c *fakeDMClient) Query(ctx context.Context, prompt string) error {
	return c.QueryWithOptions(ctx, prompt, sdkprotocol.OutboundMessageOptions{})
}

func (c *fakeDMClient) QueryWithOptions(ctx context.Context, prompt string, options sdkprotocol.OutboundMessageOptions) error {
	if c.onQuery != nil {
		c.onQuery(ctx, prompt)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.queryPrompts = append(c.queryPrompts, prompt)
	c.queryOptions = append(c.queryOptions, options)
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
	return c.interrupt(ctx, "")
}

func (c *fakeDMClient) InterruptWithReason(ctx context.Context, reason string) error {
	return c.interrupt(ctx, reason)
}

func (c *fakeDMClient) interrupt(ctx context.Context, reason string) error {
	c.mu.Lock()
	c.interruptCalls++
	c.interruptReasons = append(c.interruptReasons, reason)
	if len(c.interruptErrors) > 0 {
		err := c.interruptErrors[0]
		c.interruptErrors = c.interruptErrors[1:]
		c.mu.Unlock()
		return err
	}
	callback := c.onInterrupt
	c.mu.Unlock()
	if callback != nil {
		callback(ctx)
	}
	return nil
}

func (c *fakeDMClient) StopTask(context.Context, string) error { return nil }

func (c *fakeDMClient) SendTaskMessage(context.Context, string, string, string) error { return nil }

func (c *fakeDMClient) RemoveMessages(_ context.Context, uuids []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.removeMessages = append(c.removeMessages, append([]string(nil), uuids...))
	return nil
}

func (c *fakeDMClient) SetPermissionMode(context.Context, sdkpermission.Mode) error { return nil }

func (c *fakeDMClient) Retire() {}

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

func (c *fakeDMClient) Supports(capability agentclient.Capability) bool {
	return (c.hookResponseAck && capability == agentclient.CapabilityHookResponseAck) ||
		(c.runningInput && capability == agentclient.CapabilityRunningInput)
}

type fakeRunningInput struct {
	id      string
	content any
}

func (c *fakeDMClient) SendRunningInput(_ context.Context, inputID string, content any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.runningInputs = append(c.runningInputs, fakeRunningInput{id: inputID, content: content})
	return nil
}

func (c *fakeDMClient) SessionID() string { return c.sessionID }

// sendResultAsync 异步推送一条 runtime 终态 result，模拟模型一轮结束。
func (c *fakeDMClient) sendResultAsync(sessionID, uuid, subtype, text string) {
	go func() {
		c.messages <- sdkprotocol.ReceivedMessage{
			Type:      sdkprotocol.MessageTypeResult,
			SessionID: sessionID,
			UUID:      uuid,
			Result:    &sdkprotocol.ResultMessage{Subtype: subtype, DurationMS: 1, NumTurns: 1, Result: text},
		}
	}()
}

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

type fakeDMRoomSessionStore struct {
	mu       sync.Mutex
	updates  []fakeDMRoomSessionUpdate
	sessions map[string]protocol.Session
}

type fakeDMRoomSessionUpdate struct {
	roomSessionID string
	sdkSessionID  string
}

func (s *fakeDMRoomSessionStore) GetRoomSessionByKey(
	_ context.Context,
	_ string,
	key protocol.SessionKey,
) (*protocol.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.sessions[key.Raw]
	if !ok {
		return nil, nil
	}
	copy := item
	return &copy, nil
}

func (s *fakeDMRoomSessionStore) UpdateRoomSessionRuntimeIdentityAtConnectorSelection(
	_ context.Context,
	roomSessionID string,
	sdkSessionID string,
	toolSurfaceFingerprint string,
	expected protocol.SessionConnectorSelection,
) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, item := range s.sessions {
		if item.RoomSessionID == nil || strings.TrimSpace(*item.RoomSessionID) != strings.TrimSpace(roomSessionID) {
			continue
		}
		if !protocol.SessionConnectorSelectionFromOptions(item.Options).Equal(expected) {
			return false, nil
		}
		item.SessionID = &sdkSessionID
		item.TranscriptSessionIDs = protocol.MergeTranscriptSessionIDs(
			item.TranscriptSessionIDs,
			[]string{sdkSessionID},
		)
		if item.Options == nil {
			item.Options = map[string]any{}
		}
		item.Options[protocol.OptionRuntimeToolSurfaceFingerprint] = strings.TrimSpace(toolSurfaceFingerprint)
		s.sessions[key] = item
		s.updates = append(s.updates, fakeDMRoomSessionUpdate{
			roomSessionID: strings.TrimSpace(roomSessionID),
			sdkSessionID:  strings.TrimSpace(sdkSessionID),
		})
		return true, nil
	}
	return false, nil
}

func (s *fakeDMRoomSessionStore) UpdateRoomSessionRuntimeIdentity(
	_ context.Context,
	roomSessionID string,
	sdkSessionID string,
	_ string,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updates = append(s.updates, fakeDMRoomSessionUpdate{
		roomSessionID: strings.TrimSpace(roomSessionID),
		sdkSessionID:  strings.TrimSpace(sdkSessionID),
	})
	return nil
}

func (s *fakeDMRoomSessionStore) Updates() []fakeDMRoomSessionUpdate {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]fakeDMRoomSessionUpdate(nil), s.updates...)
}

func TestServiceHandleChatDoesNotPersistSDKSessionIDWithoutTranscript(t *testing.T) {
	cfg := newDMTestConfig(t)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.sessionID = "11111111-2222-4222-8222-111111111111"
	client.onQuery = func(_ context.Context, _ string) {
		client.sendResultAsync(client.sessionID, "result-without-transcript", "success", "ok")
	}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	service.SetInboxStore(newDMInboxStore(t, cfg))
	sender := newDMTestSender("sender-without-transcript")
	sessionKey := "agent:nexus:ws:dm:without-transcript"
	permission.BindSession(sessionKey, sender)
	cleanupDMRuntimeSession(t, runtimeManager, sessionKey)

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "测试 transcript 未落盘不写 resume",
		RoundID:    "round-without-transcript",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	options := factory.LastOptions()
	if strings.TrimSpace(options.Session.ResumeID) != "" {
		t.Fatalf("新 DM 会话不应携带 resume: %+v", options)
	}
	sessionValue, _ := mustFindDMSession(t, service, cfg, sessionKey)
	if sessionValue.SessionID != nil && strings.TrimSpace(*sessionValue.SessionID) != "" {
		t.Fatalf("transcript 未落盘时不应写入 sdk session_id: %+v", sessionValue)
	}
}

type connectorToolSurfaceTestServer struct{}

func (connectorToolSurfaceTestServer) HandleMessage(_ context.Context, request map[string]any) (map[string]any, error) {
	if request["method"] != "tools/list" {
		return map[string]any{}, nil
	}
	return map[string]any{
		"result": map[string]any{
			"tools": []map[string]any{{
				"name":        "read",
				"description": "read Feishu document",
				"inputSchema": map[string]any{"type": "object"},
			}},
		},
	}, nil
}

func TestServiceHandleChatRetriesWithoutStaleSDKSessionWhenResumeConnectFails(t *testing.T) {
	cfg := newDMTestConfig(t)

	agentService := newDMAgentService(t, cfg)
	providerService := newDMProviderService(t, cfg)
	createDMProviderWithModel(t, providerService, providercfg.CreateInput{
		Provider:    "glm",
		DisplayName: "GLM",
		AuthToken:   "glm-token",
		BaseURL:     "https://open.bigmodel.cn/api/anthropic",
		Enabled:     true,
	}, "glm-5.1", true)

	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.sessionID = "sdk-fresh-after-stale"
	client.connectErrors = []error{agentclient.ErrNotConnected}
	client.onQuery = func(_ context.Context, _ string) {
		client.sendResultAsync(client.sessionID, "result-stale-resume-retry", "success", "ok")
	}
	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	service.SetInboxStore(newDMInboxStore(t, cfg))
	service.SetProviderResolver(providerService)
	roomStore := &fakeDMRoomSessionStore{}
	service.SetRoomSessionStore(roomStore)
	sender := newDMTestSender("sender-stale-resume-retry")
	sessionKey := "agent:nexus:ws:dm:stale-resume-retry"
	permission.BindSession(sessionKey, sender)

	staleResumeID := "22222222-2222-4222-8222-222222222222"
	roomSessionID := "room-session-stale-resume-1"
	workspacePath := dmMainWorkspacePath(cfg)
	writeTranscriptFixture(t, workspacePath, staleResumeID, []map[string]any{
		{
			"type":      "user",
			"uuid":      "22000000-0000-4000-8000-000000000001",
			"sessionId": staleResumeID,
			"timestamp": "2026-06-09T00:00:00Z",
			"cwd":       workspacePath,
			"message": map[string]any{
				"role":    "user",
				"content": "旧会话消息",
			},
		},
	})
	now := time.Now().UTC()
	if _, err := service.Files.UpsertSession(workspacePath, protocol.Session{
		SessionKey:    sessionKey,
		AgentID:       cfg.DefaultAgentID,
		SessionID:     &staleResumeID,
		RoomSessionID: &roomSessionID,
		ChannelType:   "websocket",
		ChatType:      "dm",
		Status:        "active",
		CreatedAt:     now,
		LastActivity:  now,
		Title:         "Stale Resume Retry",
		Options: map[string]any{
			protocol.OptionRuntimeProvider: "glm",
			protocol.OptionRuntimeModel:    "glm-5.1",
		},
		IsActive: true,
	}); err != nil {
		t.Fatalf("预写入 stale resume 会话 meta 失败: %v", err)
	}

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "测试 stale resume 自动恢复",
		RoundID:    "round-stale-resume-retry",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	firstOptions := factory.OptionAt(0)
	if firstOptions.Session.ResumeID != staleResumeID {
		t.Fatalf("首次 runtime connect 应携带持久化 resume: %+v", firstOptions)
	}
	retryOptions := factory.OptionAt(1)
	if retryOptions.Session.ResumeID != "" {
		t.Fatalf("stale resume 连接失败后重试不应继续携带 resume: %+v", retryOptions)
	}

	client.mu.Lock()
	connectCalls := client.connectCalls
	disconnectCalls := client.disconnectCalls
	client.mu.Unlock()
	if connectCalls != 2 {
		t.Fatalf("stale resume 应触发一次无 resume 重试，connectCalls=%d", connectCalls)
	}
	if disconnectCalls == 0 {
		t.Fatal("重试前应清理 runtime manager 中的旧 client")
	}

	sessionValue, _ := mustFindDMSession(t, service, cfg, sessionKey)
	if stringPointer(t, sessionValue.SessionID) != client.sessionID {
		t.Fatalf("新 sdk session_id 未回写: %+v", sessionValue)
	}
	updates := roomStore.Updates()
	if len(updates) != 2 {
		t.Fatalf("room sdk_session_id 应先清空再回写新值: %+v", updates)
	}
	if updates[0].roomSessionID != roomSessionID || updates[0].sdkSessionID != "" {
		t.Fatalf("首次 room sdk_session_id 更新应清空 stale 值: %+v", updates)
	}
	if updates[1].roomSessionID != roomSessionID || updates[1].sdkSessionID != client.sessionID {
		t.Fatalf("第二次 room sdk_session_id 更新应写入新值: %+v", updates)
	}
}

func TestServiceHandleChatForksSDKSessionWhenSelectedConnectorChangesToolSurface(t *testing.T) {
	cfg := newDMTestConfig(t)

	agentService := newDMAgentService(t, cfg)
	providerService := newDMProviderService(t, cfg)
	createDMProviderWithModel(t, providerService, providercfg.CreateInput{
		Provider:    "glm",
		DisplayName: "GLM",
		AuthToken:   "glm-token",
		BaseURL:     "https://open.bigmodel.cn/api/anthropic",
		Enabled:     true,
	}, "glm-5.1", true)

	oldSessionID := "99999999-9999-4999-8999-999999999999"
	newSessionID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	sessionKey := "agent:nexus:ws:dm:connector-tool-surface-fork"
	permission := permissionctx.NewContext()
	staleClient := newFakeDMClient()
	staleClient.sessionID = oldSessionID
	client := newFakeDMClient()
	// Claude Code 在 Connect 后尚未公布 fork identity，直到首条 query 才通过 init 返回。
	client.sessionID = ""
	client.onQuery = func(_ context.Context, _ string) {
		client.sessionID = newSessionID
		client.sendResultAsync(newSessionID, "result-connector-tool-surface-fork", "success", "ok")
	}
	factory := &fakeDMFactory{clients: []*fakeDMClient{staleClient, client}}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	warmClient, err := runtimeManager.GetOrCreate(context.Background(), sessionKey, agentclient.Options{
		Env: map[string]string{"NEXUS_RUNTIME_USER_ID": "__system__"},
	})
	if err != nil {
		t.Fatalf("预热旧 runtime 失败: %v", err)
	}
	if err = runtimeManager.Connect(context.Background(), sessionKey, warmClient); err != nil {
		t.Fatalf("连接旧 runtime 失败: %v", err)
	}
	service := NewService(cfg, agentService, runtimeManager, permission)
	service.SetInboxStore(newDMInboxStore(t, cfg))
	service.SetProviderResolver(providerService)
	service.SetPreferences(fakeDMPreferencesService{prefs: preferencessvc.Preferences{
		AgentRuntimeKind: "claude",
		DefaultAgentOptions: protocol.Options{
			Provider: "glm",
			Model:    "glm-5.1",
		},
	}})
	service.SetConnectorRuntimeStateLoader(func(context.Context, string) ([]ConnectorRuntimeState, error) {
		return []ConnectorRuntimeState{
			{ConnectorID: "feishu-docx", Configured: true},
			{ConnectorID: "github", Configured: true},
		}, nil
	})
	service.SetMCPServerBuilder(func(
		ctx context.Context,
		_ *protocol.Agent,
		_ string,
		_ string,
		_ string,
		_ string,
		_ string,
		_ *atomic.Int64,
		_ sdkpermission.Mode,
	) map[string]sdkmcp.ServerConfig {
		enabled := runtimectx.EnabledConnectorIDs(ctx)
		if len(enabled) != 1 || enabled[0] != "feishu-docx" {
			t.Fatalf("runtime connector selection = %+v, want feishu-docx", enabled)
		}
		return map[string]sdkmcp.ServerConfig{
			"nexus_feishu_docx": sdkmcp.SDKServerConfig{
				Name:     "nexus_feishu_docx",
				Instance: connectorToolSurfaceTestServer{},
			},
		}
	})
	sender := newDMTestSender("sender-connector-tool-surface-fork")
	permission.BindSession(sessionKey, sender)

	workspacePath := dmMainWorkspacePath(cfg)
	writeTranscriptFixture(t, workspacePath, oldSessionID, []map[string]any{
		{
			"type":      "user",
			"uuid":      "99000000-0000-4000-8000-000000000001",
			"sessionId": oldSessionID,
			"timestamp": "2026-08-17T00:00:00Z",
			"cwd":       workspacePath,
			"message": map[string]any{
				"role":    "user",
				"content": "旧工具面消息",
			},
		},
	})
	writeTranscriptFixture(t, workspacePath, newSessionID, []map[string]any{
		{
			"type":      "user",
			"uuid":      "aa000000-0000-4000-8000-000000000000",
			"sessionId": newSessionID,
			"timestamp": "2026-08-17T00:00:00Z",
			"cwd":       workspacePath,
			"message": map[string]any{
				"role":    "user",
				"content": "旧工具面消息",
			},
		},
		{
			"type":       "assistant",
			"uuid":       "aa000000-0000-4000-8000-000000000001",
			"parentUuid": "aa000000-0000-4000-8000-000000000000",
			"sessionId":  newSessionID,
			"timestamp":  "2026-08-17T00:00:01Z",
			"cwd":        workspacePath,
			"message": map[string]any{
				"role": "assistant",
				"content": []any{map[string]any{
					"type": "text",
					"text": "fork 工具面回复",
				}},
			},
		},
	})
	now := time.Now().UTC()
	connectorIDs := []string{"feishu-docx"}
	sessionOptions := protocol.WithSessionRuntimeSettings(map[string]any{
		protocol.OptionRuntimeKind:     "claude",
		protocol.OptionRuntimeProvider: "glm",
		protocol.OptionRuntimeModel:    "glm-5.1",
	}, protocol.SessionRuntimeSettings{ConnectorIDs: &connectorIDs})
	if _, err := service.Files.UpsertSession(workspacePath, protocol.Session{
		SessionKey:   sessionKey,
		AgentID:      cfg.DefaultAgentID,
		SessionID:    &oldSessionID,
		ChannelType:  "websocket",
		ChatType:     "dm",
		Status:       "active",
		CreatedAt:    now,
		LastActivity: now,
		Title:        "Connector Tool Surface Fork",
		Options:      sessionOptions,
		IsActive:     true,
	}); err != nil {
		t.Fatalf("预写入 legacy session 失败: %v", err)
	}

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "验证新的 Connector 工具面",
		RoundID:    "round-connector-tool-surface-fork",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}
	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	options := factory.LastOptions()
	// Connector 事实不进 system prompt（否则配置变化会让整段 prompt cache 失效），只在变化时随本轮上下文追加。
	if strings.Contains(options.System.AppendDynamic, "connector") {
		t.Fatalf("Connector 事实不应进入 system prompt: %s", options.System.AppendDynamic)
	}
	client.mu.Lock()
	currentTurnPrompt := strings.Join(client.queryPrompts, "\n")
	client.mu.Unlock()
	for _, want := range []string{
		`"connector_id":"feishu-docx","configuration_state":"configured","selected_in_current_session":true`,
		`"connector_id":"github","configuration_state":"configured","selected_in_current_session":false`,
		`<current_turn_connector_tools>`,
		`"connector_id":"feishu-docx","server_alias":"nexus_feishu_docx"`,
		`mcp__nexus_feishu_docx__read`,
	} {
		if strings.Count(currentTurnPrompt, want) != 1 {
			t.Fatalf("本轮上下文应恰好携带一次 Connector 事实 %q: %s", want, currentTurnPrompt)
		}
	}
	feishuConfig, ok := options.MCP.Servers["nexus_feishu_docx"].(sdkmcp.SDKServerConfig)
	if !ok || feishuConfig.Instance == nil {
		t.Fatalf("fork options 缺少飞书 MCP: %+v", options.MCP.Servers)
	}
	toolList, err := feishuConfig.Instance.HandleMessage(context.Background(), map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	})
	if err != nil || !strings.Contains(fmt.Sprint(toolList), "read") {
		t.Fatalf("fork 飞书 MCP 缺少 read schema: result=%+v err=%v", toolList, err)
	}
	if options.Session.ResumeID != oldSessionID || !options.Session.Fork || options.Session.ID != "" {
		t.Fatalf("工具面变化应从旧 transcript fork: %+v", options.Session)
	}
	staleClient.mu.Lock()
	staleDisconnectCalls := staleClient.disconnectCalls
	staleClient.mu.Unlock()
	if staleDisconnectCalls == 0 {
		t.Fatal("工具面 fork 前未退休 warm client")
	}
	sessionValue, _ := mustFindDMSession(t, service, cfg, sessionKey)
	if stringPointer(t, sessionValue.SessionID) != newSessionID {
		t.Fatalf("换代后新 SDK session_id = %q, want %q; session=%+v", stringPointer(t, sessionValue.SessionID), newSessionID, sessionValue)
	}
	if sessionValue.Options[protocol.OptionRuntimeToolSurfaceFingerprint] == "" {
		t.Fatalf("换代后工具面指纹未写回: %+v", sessionValue.Options)
	}
	if sessionValue.SessionKey != sessionKey || sessionValue.Title != "Connector Tool Surface Fork" {
		t.Fatalf("Nexus Session identity/title 不应随 SDK 换代改变: %+v", sessionValue)
	}
	if segmented, _ := sessionValue.Options[protocol.OptionRuntimeSegmentedTranscript].(bool); segmented {
		t.Fatalf("复制 transcript 的 fork 不应标记为非复制分段历史: %+v", sessionValue.Options)
	}
	wantLineage := protocol.MergeTranscriptSessionIDs([]string{oldSessionID, newSessionID})
	if got := protocol.SessionTranscriptIDs(sessionValue); len(got) != len(wantLineage) || got[0] != wantLineage[0] || got[1] != wantLineage[1] {
		t.Fatalf("换代 transcript lineage = %+v, want %+v", got, wantLineage)
	}
	rows, err := service.History.ForOwner("__system__").ReadMessages(workspacePath, sessionValue, nil)
	if err != nil {
		t.Fatalf("读取换代后的统一历史失败: %v", err)
	}
	joinedRows := fmt.Sprint(rows)
	if !strings.Contains(joinedRows, "旧工具面消息") || !strings.Contains(joinedRows, "验证新的 Connector 工具面") {
		t.Fatalf("fork 后历史不完整: %+v", rows)
	}
}

func TestPrepareConnectorRuntimeMaterializesNXSForkBeforeUserQuery(t *testing.T) {
	cfg := newDMTestConfig(t)
	agentService := newDMAgentService(t, cfg)
	providerService := newDMProviderService(t, cfg)
	createDMProviderWithModel(t, providerService, providercfg.CreateInput{
		Provider:    "glm",
		DisplayName: "GLM",
		AuthToken:   "glm-token",
		BaseURL:     "https://open.bigmodel.cn/api/anthropic",
		Enabled:     true,
	}, "glm-5.1", true)

	oldSessionID := "19191919-1919-4919-8919-191919191919"
	newSessionID := "29292929-2929-4929-8929-292929292929"
	roomSessionID := "room-session-eager-connector-fork"
	sessionKey := "agent:nexus:ws:dm:eager-connector-tool-surface-fork"
	client := newFakeDMClient()
	client.sessionID = newSessionID
	client.onQuery = func(_ context.Context, _ string) {
		client.sendResultAsync(newSessionID, "result-eager-connector-tool-surface-fork", "success", "ok")
	}
	factory := &fakeDMFactory{client: client}
	permission := permissionctx.NewContext()
	service := NewService(
		cfg,
		agentService,
		runtimectx.NewManagerWithFactory(factory),
		permission,
	)
	service.SetInboxStore(newDMInboxStore(t, cfg))
	service.SetProviderResolver(providerService)
	roomStore := &fakeDMRoomSessionStore{}
	service.SetRoomSessionStore(roomStore)
	service.SetPreferences(fakeDMPreferencesService{prefs: preferencessvc.Preferences{
		AgentRuntimeKind: "nxs",
		DefaultAgentOptions: protocol.Options{
			Provider: "glm",
			Model:    "glm-5.1",
		},
	}})
	service.SetMCPServerBuilder(func(
		ctx context.Context,
		_ *protocol.Agent,
		_ string,
		_ string,
		_ string,
		_ string,
		_ string,
		_ *atomic.Int64,
		_ sdkpermission.Mode,
	) map[string]sdkmcp.ServerConfig {
		enabled := runtimectx.EnabledConnectorIDs(ctx)
		if len(enabled) != 1 || enabled[0] != "feishu-docx" {
			t.Fatalf("runtime connector selection=%v", enabled)
		}
		return map[string]sdkmcp.ServerConfig{
			"nexus_feishu_docx": sdkmcp.SDKServerConfig{
				Name:     "nexus_feishu_docx",
				Instance: connectorToolSurfaceTestServer{},
			},
		}
	})

	workspacePath := dmMainWorkspacePath(cfg)
	writeTranscriptFixture(t, workspacePath, oldSessionID, []map[string]any{{
		"type":      "user",
		"uuid":      "19191919-0000-4000-8000-000000000001",
		"sessionId": oldSessionID,
		"timestamp": "2026-08-18T00:00:00Z",
		"cwd":       workspacePath,
		"message": map[string]any{
			"role":    "user",
			"content": "开启飞书前的消息",
		},
	}})
	// fake nxs Connect 不会真的复制 transcript；fixture 模拟 Connect 已物化目标分支。
	writeTranscriptFixture(t, workspacePath, newSessionID, []map[string]any{{
		"type":      "user",
		"uuid":      "29292929-0000-4000-8000-000000000001",
		"sessionId": newSessionID,
		"timestamp": "2026-08-18T00:00:00Z",
		"cwd":       workspacePath,
		"message": map[string]any{
			"role":    "user",
			"content": "开启飞书前的消息",
		},
	}})
	connectorIDs := []string{"feishu-docx"}
	options := protocol.WithSessionRuntimeSettings(map[string]any{
		protocol.OptionRuntimeKind:                   "nxs",
		protocol.OptionRuntimeProvider:               "glm",
		protocol.OptionRuntimeModel:                  "glm-5.1",
		protocol.OptionRuntimeToolSurfaceFingerprint: "surface-before-connector-selection",
	}, protocol.SessionRuntimeSettings{ConnectorIDs: &connectorIDs})
	now := time.Now().UTC()
	roomOptions := protocol.WithSessionRuntimeSettings(nil, protocol.SessionRuntimeSettings{
		ConnectorIDs: &connectorIDs,
	})
	roomSnapshot := protocol.Session{
		SessionKey:           sessionKey,
		AgentID:              cfg.DefaultAgentID,
		SessionID:            &oldSessionID,
		RoomSessionID:        &roomSessionID,
		ChannelType:          "websocket",
		ChatType:             protocol.RoomTypeDM,
		Status:               "closed",
		CreatedAt:            now,
		LastActivity:         now,
		Title:                "Eager Connector Fork",
		Options:              roomOptions,
		TranscriptSessionIDs: []string{oldSessionID},
	}
	roomStore.sessions = map[string]protocol.Session{sessionKey: roomSnapshot}
	workspaceSnapshot := roomSnapshot
	workspaceSnapshot.Options = options
	stored, err := service.Files.UpsertSession(workspacePath, workspaceSnapshot)
	if err != nil || stored == nil {
		t.Fatalf("seed Session: item=%+v err=%v", stored, err)
	}
	ctx := runtimehost.ContextWithExactOwner(context.Background(), "__system__")
	// Session 设置事务通知的是 SQL projection（configuration_version 为零）；
	// 预备器必须先与 workspace runtime projection 合并后再建立版本栅栏。
	if err = service.prepareConnectorRuntime(ctx, roomSnapshot); err != nil {
		t.Fatalf("prepareConnectorRuntime() error=%v", err)
	}

	client.mu.Lock()
	queryCountBeforeSend := len(client.queryPrompts)
	client.mu.Unlock()
	if queryCountBeforeSend != 0 {
		t.Fatalf("background preparation sent %d hidden queries", queryCountBeforeSend)
	}
	prepared, _ := mustFindDMSession(t, service, cfg, sessionKey)
	if got := stringPointer(t, prepared.SessionID); got != newSessionID {
		t.Fatalf("prepared SDK session_id=%q want=%q", got, newSessionID)
	}
	if prepared.Options[protocol.OptionRuntimeToolSurfaceFingerprint] == "surface-before-connector-selection" {
		t.Fatalf("prepared tool surface was not committed: %+v", prepared.Options)
	}
	updates := roomStore.Updates()
	if len(updates) != 1 || updates[0].sdkSessionID != newSessionID {
		t.Fatalf("Room SQL fork identity updates=%+v", updates)
	}

	// finished 事件在最后一笔历史写入前送达，测试目录须等运行时收尾后释放。
	t.Cleanup(func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := service.Runtime.CloseSession(closeCtx, sessionKey); err != nil {
			t.Error(err)
		}
	})
	sender := newDMTestSender("sender-eager-connector-tool-surface-fork")
	permission.BindSession(sessionKey, sender)
	if err = service.HandleChat(ctx, Request{
		SessionKey: sessionKey,
		Content:    "使用已经预备的飞书工具面",
		RoundID:    "round-eager-connector-tool-surface-fork",
	}); err != nil {
		t.Fatalf("HandleChat() error=%v", err)
	}
	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})
	factory.mu.Lock()
	createdClientCount := len(factory.options)
	factory.mu.Unlock()
	if createdClientCount != 1 {
		t.Fatalf("next user message created %d runtime clients, want reuse of prepared client", createdClientCount)
	}
	lastOptions := factory.LastOptions()
	if lastOptions.Session.ResumeID != oldSessionID || !lastOptions.Session.Fork {
		t.Fatalf("prepared client did not originate from source fork: %+v", lastOptions.Session)
	}
	if lastOptions.Runtime.Kind != agentclient.RuntimeNXS {
		t.Fatalf("prepared runtime kind=%q", lastOptions.Runtime.Kind)
	}
}

func TestCreateTransientWorkGraphEditorSessionUsesHiddenWebSocketDMWithoutFork(t *testing.T) {
	cfg := newDMTestConfig(t)
	agents := newDMAgentService(t, cfg)
	if err := agents.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	service := NewService(
		cfg,
		agents,
		runtimectx.NewManagerWithFactory(&fakeDMFactory{}),
		permissionctx.NewContext(),
	)
	service.SetInboxStore(newDMInboxStore(t, cfg))
	sessionKey := protocol.BuildAgentSessionKey(
		cfg.DefaultAgentID,
		protocol.SessionChannelWebSocketSegment,
		protocol.RoomTypeDM,
		"editor-a",
		"",
	)
	created, err := service.CreateTransientSession(context.Background(), TransientSessionRequest{
		AgentID: cfg.DefaultAgentID, TargetSessionKey: sessionKey,
		Purpose: protocol.SessionPurposeWorkGraphEditor, Title: "调整草图",
		DisplayAfterUnixMilli: 12345,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !protocol.SessionIsHiddenFromDirectory(*created) ||
		protocol.SessionPurpose(*created) != protocol.SessionPurposeWorkGraphEditor ||
		fmt.Sprint(created.Options[protocol.OptionSessionDisplayAfterUnixMilli]) != "12345" ||
		created.SessionID != nil || created.Options[protocol.OptionRuntimeForkSourceSessionID] != nil {
		t.Fatalf("hidden WorkGraph editor Session = %#v", created)
	}
}
