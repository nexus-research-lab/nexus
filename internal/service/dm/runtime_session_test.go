package dm

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"

	_ "modernc.org/sqlite"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestDMRuntimeDiagnosticsLogsStderrWhenEnabled(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		var output bytes.Buffer
		service := &Service{Host: runtimehost.Host{Logger: logx.New(logx.Options{Output: &output, Format: "json"})}}
		value := "0"
		if enabled {
			value = "1"
		}
		options := service.withRuntimeDiagnosticsLogger(agentclient.Options{
			Env: map[string]string{runtimectx.AgentSDKDiagnosticsEnvName: value},
		}, "session", "agent")
		options.Callbacks.Stderr("startup error")
		if got := strings.Contains(output.String(), "startup error"); got != enabled {
			t.Fatalf("enabled=%t, stderr logged=%t", enabled, got)
		}
	}
}

func TestServiceEnsureClientInjectsRuntimePrompt(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)

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
	migrateDMSQLite(t, cfg.DatabaseURL)

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
	migrateDMSQLite(t, cfg.DatabaseURL)

	agentService := newDMAgentService(t, cfg)
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.sessionID = "11111111-2222-4222-8222-111111111111"
	client.onQuery = func(_ context.Context, _ string) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-without-transcript",
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
	sender := newDMTestSender("sender-without-transcript")
	sessionKey := "agent:nexus:ws:dm:without-transcript"
	permission.BindSession(sessionKey, sender)

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
	migrateDMSQLite(t, cfg.DatabaseURL)

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
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-owner-runtime",
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
	sender := newDMTestSender("sender-owner-runtime")
	sessionKey := protocol.BuildAgentSessionKey(
		agentValue.AgentID,
		protocol.SessionChannelWebSocketSegment,
		protocol.RoomTypeDM,
		"owner-runtime",
		"",
	)
	permission.BindSession(sessionKey, sender)

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

	sessionValue, _, err := service.files.ForOwner(ownerUserID).FindSession(
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
