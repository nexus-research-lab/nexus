// runtime_session_test.go 覆盖 DM 运行时会话、恢复、配置准备集成与临时会话测试。
package dm

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
	usagesvc "github.com/nexus-research-lab/nexus/internal/service/usage"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
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

func TestDMRuntimeDiagnosticsLogsStderrWhenEnabled(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		var output bytes.Buffer
		service := &Service{Host: runtimehost.Host{Logger: logx.New(logx.Options{Output: &output, Format: "json"})}}
		value := "0"
		if enabled {
			value = "1"
		}
		options := runtimehost.WithRuntimeDiagnosticsLogger(agentclient.Options{
			Env: map[string]string{runtimectx.AgentSDKDiagnosticsEnvName: value},
		}, service.LoggerFor(context.Background()).With("session_key", "session", "agent_id", "agent"))
		options.Callbacks.Stderr("startup error")
		if got := strings.Contains(output.String(), "startup error"); got != enabled {
			t.Fatalf("enabled=%t, stderr logged=%t", enabled, got)
		}
	}
}

func TestServiceEnsureClientInjectsRuntimePrompt(t *testing.T) {
	cfg := newDMTestConfig(t)

	agentService := newDMAgentService(t, cfg)
	created, err := agentService.CreateAgent(context.Background(), protocol.CreateRequest{
		Name:        "提示词助手",
		Description: "负责执行工作区规则",
		VibeTags:    []string{"规则优先", "稳健"},
	})
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

	agentValue, err := agentService.GetAgent(context.Background(), created.AgentID)
	if err != nil {
		t.Fatalf("读取测试 agent 失败: %v", err)
	}

	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	service.SetInboxStore(newDMInboxStore(t, cfg))

	sessionKey := protocol.BuildAgentSessionKey(created.AgentID, protocol.SessionChannelWebSocketSegment, "dm", "prompt-ref", "")
	parsed := protocol.ParseSessionKey(sessionKey)
	sessionItem, err := service.ensureSession(context.Background(), agentValue, parsed, sessionKey)
	if err != nil {
		t.Fatalf("初始化 session 失败: %v", err)
	}
	if _, err = service.ensureClient(context.Background(), sessionKey, agentValue, sessionItem, Request{
		SessionKey:     sessionKey,
		PermissionMode: sdkpermission.ModeDefault,
	}); err != nil {
		t.Fatalf("构建 runtime client 失败: %v", err)
	}

	promptOptions := factory.LastOptions().System
	appendSystemPrompt := promptOptions.Append
	if !strings.Contains(promptOptions.AppendStatic, "## Execution Orchestration") {
		t.Fatalf("DM static prompt 未注入 execution contract: %s", promptOptions.AppendStatic)
	}
	if strings.Count(promptOptions.AppendStatic, "## Execution Orchestration") != 1 ||
		strings.Count(appendSystemPrompt, "## Execution Orchestration") != 1 {
		t.Fatalf("DM execution contract 应只注入一次: static=%q combined=%q", promptOptions.AppendStatic, appendSystemPrompt)
	}
	if strings.Contains(promptOptions.AppendDynamic, "## Execution Orchestration") {
		t.Fatalf("DM dynamic prompt 不应重复 execution contract: %s", promptOptions.AppendDynamic)
	}
	for _, expected := range []string{
		"Before substantial execution, assess separability",
		"Use subagents only when benefit exceeds coordination cost",
		"the parent integrates, verifies, and delivers",
	} {
		if !strings.Contains(promptOptions.AppendStatic, expected) {
			t.Fatalf("DM 固定 execution contract 缺少全 Agent 自适应分解规则 %q: %s", expected, promptOptions.AppendStatic)
		}
	}
	if strings.Contains(appendSystemPrompt, "执行规则：必须先加载工作区规则") {
		t.Fatalf("Nexus runtime prompt 不应重复注入由 SDK 加载的 AGENTS.md: %s", appendSystemPrompt)
	}
	if !strings.Contains(appendSystemPrompt, "Description: 负责执行工作区规则") {
		t.Fatalf("runtime prompt 未注入 Agent description: %s", appendSystemPrompt)
	}
	if !strings.Contains(appendSystemPrompt, "Vibe Tags: 规则优先, 稳健") {
		t.Fatalf("runtime prompt 未注入 Agent vibe_tags: %s", appendSystemPrompt)
	}
	for _, expected := range []string{
		"Description: 负责执行工作区规则",
		"Vibe Tags: 规则优先, 稳健",
	} {
		if !strings.Contains(promptOptions.AppendDynamic, expected) {
			t.Fatalf("DM Agent surface prompt 应保留在 dynamic 段，缺少 %q: %s", expected, promptOptions.AppendDynamic)
		}
		if strings.Contains(promptOptions.AppendStatic, expected) {
			t.Fatalf("DM static execution contract 不应混入 Agent surface 内容 %q: %s", expected, promptOptions.AppendStatic)
		}
	}
}

func TestServiceEnsureClientPropagatesMainAgentWorkspaceIdentity(t *testing.T) {
	cfg := newDMTestConfig(t)
	cfg.RuntimeIsolationMode = "audit"

	agentService := newDMAgentService(t, cfg)
	agentValue, err := agentService.GetAgent(context.Background(), cfg.DefaultAgentID)
	if err != nil {
		t.Fatalf("读取主智能体失败: %v", err)
	}
	if !agentValue.IsMain {
		t.Fatalf("默认 Agent 应为主智能体: %#v", agentValue)
	}

	factory := &fakeDMFactory{client: newFakeDMClient()}
	service := NewService(
		cfg,
		agentService,
		runtimectx.NewManagerWithFactory(factory),
		permissionctx.NewContext(),
	)
	service.SetInboxStore(newDMInboxStore(t, cfg))
	sessionKey := protocol.BuildAgentSessionKey(
		agentValue.AgentID,
		protocol.SessionChannelWebSocketSegment,
		"dm",
		"main-workspace-policy",
		"",
	)
	sessionItem, err := service.ensureSession(
		context.Background(),
		agentValue,
		protocol.ParseSessionKey(sessionKey),
		sessionKey,
	)
	if err != nil {
		t.Fatalf("初始化主智能体 session 失败: %v", err)
	}
	if _, err = service.ensureClient(
		context.Background(),
		sessionKey,
		agentValue,
		sessionItem,
		Request{
			SessionKey:     sessionKey,
			PermissionMode: sdkpermission.ModeDefault,
		},
	); err != nil {
		t.Fatalf("构建主智能体 runtime client 失败: %v", err)
	}

	matchers := factory.LastOptions().Hooks.Matchers[sdkhook.EventPreToolUse]
	var workspaceHook, agentHook sdkhook.Callback
	for _, matcher := range matchers {
		if len(matcher.Hooks) != 1 {
			continue
		}
		switch matcher.Matcher {
		case "":
			workspaceHook = matcher.Hooks[0]
		case "Agent":
			agentHook = matcher.Hooks[0]
		}
	}
	if workspaceHook == nil || agentHook == nil {
		t.Fatalf("主智能体应同时保留 workspace 与 Agent admission hooks: %#v", matchers)
	}
	output, err := workspaceHook(context.Background(), sdkhook.Input{
		CWD:      agentValue.WorkspacePath,
		ToolName: "Bash",
		ToolInput: map[string]any{
			"command": `"$NEXUSCTL_COMMAND_PATH" --json agent list`,
		},
	}, "main-tool")
	if err != nil {
		t.Fatal(err)
	}
	if output.SpecificOutput != nil {
		t.Fatalf("DM runtime 丢失主智能体身份: %#v", output)
	}
	output, err = agentHook(context.Background(), sdkhook.Input{
		ToolName: "Agent",
	}, "agent-tool")
	if err != nil {
		t.Fatal(err)
	}
	if output.SpecificOutput == nil ||
		!strings.Contains(output.SpecificOutput.PermissionDecisionReason, "subagent_admission_unavailable") {
		t.Fatalf("未注入 provider 时 Agent tool 必须 fail closed: %#v", output)
	}
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

func TestServiceHandleChatPersistsSDKSessionIDInAuthenticatedOwnerRuntime(t *testing.T) {
	cfg := newDMTestConfig(t)

	ownerUserID := "user_runtime_owner"
	ownerContext := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID:     ownerUserID,
		Username:   "runtime-owner",
		Role:       authctx.RoleOwner,
		AuthMethod: authctx.AuthMethodPassword,
	})
	agentService := newDMAgentService(t, cfg)
	agentValue, err := agentService.GetDefaultAgent(ownerContext)
	if err != nil {
		t.Fatalf("初始化 owner 主 Agent 失败: %v", err)
	}

	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.sessionID = "33333333-4444-4555-8666-777777777777"
	client.onQuery = func(_ context.Context, _ string) {
		client.sendResultAsync(client.sessionID, "result-owner-runtime", "success", "ok")
	}

	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permission)
	service.SetInboxStore(newDMInboxStore(t, cfg))
	sender := newDMTestSender("sender-owner-runtime")
	sessionKey := protocol.BuildAgentSessionKey(
		agentValue.AgentID,
		protocol.SessionChannelWebSocketSegment,
		protocol.RoomTypeDM,
		"owner-runtime",
		"",
	)
	permission.BindSession(sessionKey, sender)
	cleanupDMRuntimeSession(t, runtimeManager, sessionKey)

	writeOwnerTranscriptFixture(t, ownerUserID, agentValue.WorkspacePath, client.sessionID, []map[string]any{
		{
			"type":      "user",
			"uuid":      "33000000-0000-4000-8000-000000000001",
			"sessionId": client.sessionID,
			"timestamp": "2026-07-27T00:00:00Z",
			"cwd":       agentValue.WorkspacePath,
			"message": map[string]any{
				"role":    "user",
				"content": "owner transcript",
			},
		},
	})

	if err = service.HandleChat(ownerContext, Request{
		SessionKey: sessionKey,
		Content:    "测试 owner runtime transcript",
		RoundID:    "round-owner-runtime",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}
	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	sessionValue, _, err := service.Files.ForOwner(ownerUserID).FindSession(
		[]string{agentValue.WorkspacePath},
		sessionKey,
	)
	if err != nil {
		t.Fatalf("读取 owner session 元数据失败: %v", err)
	}
	if sessionValue == nil || sessionValue.SessionID == nil ||
		strings.TrimSpace(*sessionValue.SessionID) != client.sessionID {
		t.Fatalf("SDK session_id 未写入 owner 会话: %+v", sessionValue)
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

func TestServiceEnsureClientRetriesFreshWhenAutomaticToolSurfaceForkCannotResume(t *testing.T) {
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

	client := newFakeDMClient()
	client.sessionID = "sdk-fresh-after-failed-tool-surface-fork"
	client.connectErrors = []error{agentclient.ErrNotConnected}
	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(
		cfg,
		agentService,
		runtimeManager,
		permissionctx.NewContext(),
	)
	service.SetInboxStore(newDMInboxStore(t, cfg))
	service.SetProviderResolver(providerService)
	service.SetPreferences(fakeDMPreferencesService{prefs: preferencessvc.Preferences{
		AgentRuntimeKind: "claude",
		DefaultAgentOptions: protocol.Options{
			Provider: "glm",
			Model:    "glm-5.1",
		},
	}})

	ctx := context.Background()
	agentValue, err := agentService.GetAgent(ctx, cfg.DefaultAgentID)
	if err != nil {
		t.Fatalf("读取默认 Agent 失败: %v", err)
	}
	workspacePath := dmMainWorkspacePath(cfg)
	staleResumeID := "12121212-1212-4212-8212-121212121212"
	writeTranscriptFixture(t, workspacePath, staleResumeID, []map[string]any{{
		"type":      "user",
		"uuid":      "12000000-0000-4000-8000-000000000001",
		"sessionId": staleResumeID,
		"timestamp": "2026-08-27T00:00:00Z",
		"cwd":       workspacePath,
		"message": map[string]any{
			"role":    "user",
			"content": "旧工具面消息",
		},
	}})
	sessionKey := "agent:nexus:ws:dm:failed-tool-surface-fork-retry"
	now := time.Now().UTC()
	sessionItem := protocol.Session{
		SessionKey:   sessionKey,
		AgentID:      cfg.DefaultAgentID,
		SessionID:    &staleResumeID,
		ChannelType:  "websocket",
		ChatType:     "dm",
		Status:       "active",
		CreatedAt:    now,
		LastActivity: now,
		Title:        "Failed Tool Surface Fork Retry",
		Options: map[string]any{
			protocol.OptionRuntimeKind:                   "claude",
			protocol.OptionRuntimeProvider:               "glm",
			protocol.OptionRuntimeModel:                  "glm-5.1",
			protocol.OptionRuntimeToolSurfaceFingerprint: "stale-tool-surface",
		},
		IsActive: true,
	}
	if _, err = service.Files.UpsertSession(workspacePath, sessionItem); err != nil {
		t.Fatalf("预写入自动工具面 fork 会话失败: %v", err)
	}

	preparation, err := service.ensureClient(ctx, sessionKey, agentValue, sessionItem, Request{
		SessionKey:   sessionKey,
		RoundID:      "round-failed-tool-surface-fork-retry",
		AgentRoundID: "agent-round-failed-tool-surface-fork-retry",
	})
	if err != nil {
		t.Fatalf("自动工具面 fork 失效后应创建新 runtime: %v", err)
	}
	t.Cleanup(func() {
		_ = runtimeManager.CloseSession(context.Background(), sessionKey)
	})
	if preparation.forkSourceSessionID != "" {
		t.Fatalf("新 runtime 不应保留失效 fork source: %q", preparation.forkSourceSessionID)
	}

	firstOptions := factory.OptionAt(0)
	if firstOptions.Session.ResumeID != staleResumeID || !firstOptions.Session.Fork {
		t.Fatalf("首次启动应尝试自动工具面 fork: %+v", firstOptions.Session)
	}
	retryOptions := factory.OptionAt(1)
	if retryOptions.Session.ResumeID != "" || retryOptions.Session.ResumeAt != "" || retryOptions.Session.Fork {
		t.Fatalf("重试应创建全新 SDK session: %+v", retryOptions.Session)
	}

	updated, _ := mustFindDMSession(t, service, cfg, sessionKey)
	if updated.SessionID != nil {
		t.Fatalf("失效 resume id 未清除: %+v", updated)
	}
	if got := protocol.SessionTranscriptIDs(updated); len(got) != 1 || got[0] != staleResumeID {
		t.Fatalf("旧 transcript lineage 未保留: %+v", got)
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

func TestServiceResolveReusableSDKSessionIDReusesSharedTranscriptRuntimeSwitch(t *testing.T) {
	cfg := newDMTestConfig(t)
	agentService := newDMAgentService(t, cfg)
	service := NewService(cfg, agentService, runtimectx.NewManagerWithFactory(&fakeDMFactory{}), permissionctx.NewContext())
	service.SetInboxStore(newDMInboxStore(t, cfg))

	workspacePath := dmMainWorkspacePath(cfg)
	resumeID := "66666666-6666-4666-8666-666666666666"
	writeTranscriptFixture(t, workspacePath, resumeID, []map[string]any{
		{
			"type":      "user",
			"uuid":      "66000000-0000-4000-8000-000000000001",
			"sessionId": resumeID,
			"timestamp": "2026-06-09T00:00:00Z",
			"cwd":       workspacePath,
			"message": map[string]any{
				"role":    "user",
				"content": "继续",
			},
		},
	})

	got, reset := service.resolveReusableSDKSessionID(context.Background(), workspacePath, protocol.Session{
		SessionKey: "agent:nexus:ws:dm:resolve-shared-runtime-resume",
		AgentID:    cfg.DefaultAgentID,
		SessionID:  &resumeID,
		Options: map[string]any{
			protocol.OptionRuntimeKind:                   "nxs",
			protocol.OptionRuntimeProvider:               "glm",
			protocol.OptionRuntimeModel:                  "glm-5.1",
			protocol.OptionRuntimeToolSurfaceFingerprint: "surface-before",
		},
	}, "glm", agentclient.Options{
		Model: "glm-5.1",
		Session: agentclient.SessionOptions{
			ResumeID: resumeID,
		},
		Runtime: agentclient.RuntimeOptions{
			Kind: agentclient.RuntimeClaude,
		},
	}, "surface-current", false)
	if got != resumeID {
		t.Fatalf("runtime 切换存在共享 transcript 时应复用 resume: got=%q want=%q", got, resumeID)
	}
	if reset {
		t.Fatal("NXS/Claude 共享 transcript 时不应因 runtime 工具面差异强制 fork")
	}
}

func TestSyncSDKSessionDoesNotCommitToolSurfaceBeforeForkTranscriptIsPersistable(t *testing.T) {
	cfg := newDMTestConfig(t)
	agentService := newDMAgentService(t, cfg)
	service := NewService(cfg, agentService, runtimectx.NewManagerWithFactory(&fakeDMFactory{}), permissionctx.NewContext())
	service.SetInboxStore(newDMInboxStore(t, cfg))

	workspacePath := dmMainWorkspacePath(cfg)
	oldSessionID := "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	newSessionID := "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	writeTranscriptFixture(t, workspacePath, oldSessionID, []map[string]any{
		{
			"type":      "user",
			"uuid":      "bb000000-0000-4000-8000-000000000001",
			"sessionId": oldSessionID,
			"timestamp": "2026-08-17T00:00:00Z",
			"cwd":       workspacePath,
			"message": map[string]any{
				"role":    "user",
				"content": "旧工具面",
			},
		},
	})
	now := time.Now().UTC()
	current := protocol.Session{
		SessionKey:   "agent:nexus:ws:dm:tool-surface-fork-not-yet-persistable",
		AgentID:      cfg.DefaultAgentID,
		SessionID:    &oldSessionID,
		ChannelType:  "websocket",
		ChatType:     "dm",
		Status:       "active",
		CreatedAt:    now,
		LastActivity: now,
		Options: map[string]any{
			protocol.OptionRuntimeKind:                   "nxs",
			protocol.OptionRuntimeProvider:               "glm",
			protocol.OptionRuntimeModel:                  "glm-5.1",
			protocol.OptionRuntimeToolSurfaceFingerprint: "surface-before",
		},
		IsActive: true,
	}
	stored, err := service.Files.UpsertSession(workspacePath, current)
	if err != nil || stored == nil {
		t.Fatalf("预写入 Session 失败: stored=%+v err=%v", stored, err)
	}

	updated, err := service.syncSDKSessionIDForOwner(
		context.Background(),
		"__system__",
		workspacePath,
		*stored,
		newSessionID,
		"nxs",
		"glm",
		"glm-5.1",
		"surface-after",
	)
	if err != nil {
		t.Fatalf("syncSDKSessionIDForOwner() error = %v", err)
	}
	if got := stringPointer(t, updated.SessionID); got != oldSessionID {
		t.Fatalf("不可恢复的新 transcript 不应替换 current id: got=%q want=%q", got, oldSessionID)
	}
	if got := updated.Options[protocol.OptionRuntimeToolSurfaceFingerprint]; got != "surface-before" {
		t.Fatalf("新工具面不得提前提交到旧物理 session: got=%v", got)
	}
}

func TestPreparedRoomForkRejectsSupersededConnectorSelection(t *testing.T) {
	cfg := newDMTestConfig(t)
	agentService := newDMAgentService(t, cfg)
	service := NewService(
		cfg,
		agentService,
		runtimectx.NewManagerWithFactory(&fakeDMFactory{}),
		permissionctx.NewContext(),
	)
	service.SetInboxStore(newDMInboxStore(t, cfg))
	workspacePath := dmMainWorkspacePath(cfg)
	oldSessionID := "39393939-3939-4939-8939-393939393939"
	newSessionID := "49494949-4949-4949-8949-494949494949"
	roomSessionID := "room-session-superseded-connector-fork"
	sessionKey := "agent:nexus:ws:dm:superseded-connector-fork"
	for _, sessionID := range []string{oldSessionID, newSessionID} {
		writeTranscriptFixture(t, workspacePath, sessionID, []map[string]any{{
			"type":      "user",
			"uuid":      sessionID + "-message",
			"sessionId": sessionID,
			"timestamp": "2026-08-18T00:00:00Z",
			"cwd":       workspacePath,
			"message":   map[string]any{"role": "user", "content": "history"},
		}})
	}
	feishu := []string{"feishu-docx"}
	current, err := service.Files.UpsertSession(workspacePath, protocol.Session{
		SessionKey:    sessionKey,
		AgentID:       cfg.DefaultAgentID,
		SessionID:     &oldSessionID,
		RoomSessionID: &roomSessionID,
		ChatType:      protocol.RoomTypeDM,
		Options: protocol.WithSessionRuntimeSettings(map[string]any{
			protocol.OptionRuntimeToolSurfaceFingerprint: "surface-before",
		}, protocol.SessionRuntimeSettings{ConnectorIDs: &feishu}),
	})
	if err != nil || current == nil {
		t.Fatalf("seed Session: item=%+v err=%v", current, err)
	}
	github := []string{"github"}
	roomStore := &fakeDMRoomSessionStore{sessions: map[string]protocol.Session{
		sessionKey: {
			SessionKey:    sessionKey,
			AgentID:       cfg.DefaultAgentID,
			SessionID:     &oldSessionID,
			RoomSessionID: &roomSessionID,
			ChatType:      protocol.RoomTypeDM,
			Options: protocol.WithSessionRuntimeSettings(nil, protocol.SessionRuntimeSettings{
				ConnectorIDs: &github,
			}),
		},
	}}
	service.SetRoomSessionStore(roomStore)
	expected := protocol.SessionConnectorSelectionFromOptions(current.Options)
	_, err = service.syncSDKSessionIDForOwner(
		runtimehost.ContextWithExactOwner(context.Background(), "__system__"),
		"__system__",
		workspacePath,
		*current,
		newSessionID,
		"nxs",
		"glm",
		"glm-5.1",
		"surface-after",
		sdkSessionSyncConstraint{
			configurationVersion: current.ConfigurationVersion,
			connectorSelection:   &expected,
		},
	)
	if !errors.Is(err, workspacestore.ErrSessionConfigurationVersionConflict) {
		t.Fatalf("superseded Room fork error=%v", err)
	}
	reloaded, _ := mustFindDMSession(t, service, cfg, sessionKey)
	if got := stringPointer(t, reloaded.SessionID); got != oldSessionID {
		t.Fatalf("superseded fork committed SDK session_id=%q", got)
	}
	if got := reloaded.Options[protocol.OptionRuntimeToolSurfaceFingerprint]; got != "surface-before" {
		t.Fatalf("superseded fork committed tool surface=%v", got)
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

func TestCreateTransientSessionIsHiddenAndDoesNotForkTranscript(t *testing.T) {
	cfg := newDMTestConfig(t)
	agents := newDMAgentService(t, cfg)
	if err := agents.EnsureReady(context.Background()); err != nil {
		t.Fatalf("初始化 Agent 失败: %v", err)
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
		protocol.SessionChannelInternalSegment,
		protocol.RoomTypeDM,
		"preview-a",
		"",
	)
	created, err := service.CreateTransientSession(context.Background(), TransientSessionRequest{
		AgentID:          cfg.DefaultAgentID,
		TargetSessionKey: sessionKey,
		Purpose:          protocol.SessionPurposeWorkGraphDistillation,
		Title:            "保存工作图草图",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.SessionKey != sessionKey || !protocol.SessionIsHiddenFromDirectory(*created) ||
		protocol.SessionPurpose(*created) != protocol.SessionPurposeWorkGraphDistillation {
		t.Fatalf("transient Session = %#v", created)
	}
	if created.SessionID != nil ||
		created.Options[protocol.OptionRuntimeForkSourceSessionID] != nil ||
		created.Options[protocol.OptionRuntimeForkMessageID] != nil {
		t.Fatalf("isolated Session inherited a transcript: %#v", created)
	}

	_, err = service.CreateTransientSession(context.Background(), TransientSessionRequest{
		AgentID: cfg.DefaultAgentID,
		TargetSessionKey: protocol.BuildAgentSessionKey(
			cfg.DefaultAgentID,
			protocol.SessionChannelWebSocketSegment,
			protocol.RoomTypeDM,
			"preview-a",
			"",
		),
		Purpose: protocol.SessionPurposeWorkGraphDistillation,
	})
	if err == nil {
		t.Fatal("transient internal Session accepted a user WebSocket key")
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
