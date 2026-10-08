package dm

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

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
	migrateDMSQLite(t, cfg.DatabaseURL)

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
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-stale-resume-retry",
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
	if _, err := service.files.UpsertSession(workspacePath, protocol.Session{
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
	migrateDMSQLite(t, cfg.DatabaseURL)

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
	if _, err = service.files.UpsertSession(workspacePath, sessionItem); err != nil {
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
	migrateDMSQLite(t, cfg.DatabaseURL)

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
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: newSessionID,
				UUID:      "result-connector-tool-surface-fork",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "success",
					DurationMS: 1,
					NumTurns:   1,
					Result:     "ok",
				},
			}
		}()
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
	if _, err := service.files.UpsertSession(workspacePath, protocol.Session{
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
	for _, want := range []string{
		`"connector_id":"feishu-docx","configuration_state":"configured","selected_in_current_session":true`,
		`"connector_id":"github","configuration_state":"configured","selected_in_current_session":false`,
	} {
		if !strings.Contains(options.System.AppendDynamic, want) {
			t.Fatalf("fork runtime 动态上下文缺少 Connector 状态 %q: %s", want, options.System.AppendDynamic)
		}
	}
	for _, want := range []string{
		`<current_turn_connector_tools>`,
		`"connector_id":"feishu-docx","server_alias":"nexus_feishu_docx"`,
		`mcp__nexus_feishu_docx__read`,
	} {
		if !strings.Contains(options.System.AppendDynamic, want) {
			t.Fatalf("fork runtime system prompt 缺少 Connector 工具事实 %q: %s", want, options.System.AppendDynamic)
		}
	}
	client.mu.Lock()
	currentTurnPrompt := strings.Join(client.queryPrompts, "\n")
	client.mu.Unlock()
	if strings.Contains(currentTurnPrompt, `<current_turn_connector_tools>`) {
		t.Fatalf("当前用户轮次重复注入 Connector 工具事实: %s", currentTurnPrompt)
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
	rows, err := service.history.ForOwner("__system__").ReadMessages(workspacePath, sessionValue, nil)
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
	stored, err := service.files.UpsertSession(workspacePath, current)
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
