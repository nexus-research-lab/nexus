package dm

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

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
	workspacePath := filepath.Join(cfg.WorkspacePath, cfg.DefaultAgentID)
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
		ReqID:      "round-stale-resume-retry",
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

func TestServiceHandleChatKeepsSDKSessionResumeWhenRuntimeFingerprintMissingAndTranscriptExists(t *testing.T) {
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

	resumeID := "33333333-3333-4333-8333-333333333333"
	permission := permissionctx.NewContext()
	client := newFakeDMClient()
	client.sessionID = resumeID
	client.onQuery = func(_ context.Context, _ string) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-legacy-resume",
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
	sender := newDMTestSender("sender-legacy-resume")
	sessionKey := "agent:nexus:ws:dm:legacy-resume-chat"
	permission.BindSession(sessionKey, sender)

	workspacePath := filepath.Join(cfg.WorkspacePath, cfg.DefaultAgentID)
	writeTranscriptFixture(t, workspacePath, resumeID, []map[string]any{
		{
			"type":      "user",
			"uuid":      "33000000-0000-4000-8000-000000000001",
			"sessionId": resumeID,
			"timestamp": "2026-06-09T00:00:00Z",
			"cwd":       workspacePath,
			"message": map[string]any{
				"role":    "user",
				"content": "旧版无指纹会话",
			},
		},
	})
	now := time.Now().UTC()
	if _, err := service.files.UpsertSession(workspacePath, protocol.Session{
		SessionKey:   sessionKey,
		AgentID:      cfg.DefaultAgentID,
		SessionID:    &resumeID,
		ChannelType:  "websocket",
		ChatType:     "dm",
		Status:       "active",
		CreatedAt:    now,
		LastActivity: now,
		Title:        "Legacy Resume Chat",
		Options:      map[string]any{},
		IsActive:     true,
	}); err != nil {
		t.Fatalf("预写入 legacy 会话 meta 失败: %v", err)
	}

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "测试 legacy resume",
		RoundID:    "round-legacy-resume",
		ReqID:      "round-legacy-resume",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	options := factory.LastOptions()
	if options.Session.ResumeID != resumeID {
		t.Fatalf("legacy session 缺少 runtime 指纹时仍应 resume: %+v", options)
	}
	if options.Model != "glm-5.1" {
		t.Fatalf("runtime 应使用当前 provider model: %+v", options)
	}
	sessionValue, _ := mustFindDMSession(t, service, cfg, sessionKey)
	if stringPointer(t, sessionValue.SessionID) != resumeID {
		t.Fatalf("legacy resume 不应被清空或替换: %+v", sessionValue)
	}
	if sessionValue.Options[protocol.OptionRuntimeProvider] != "glm" {
		t.Fatalf("legacy resume 后应补写 runtime provider 指纹: %+v", sessionValue.Options)
	}
	if sessionValue.Options[protocol.OptionRuntimeModel] != "glm-5.1" {
		t.Fatalf("legacy resume 后应补写 runtime model 指纹: %+v", sessionValue.Options)
	}
}

func TestServiceHandleChatReusesSDKSessionWhenRuntimeModelFingerprintDiffersWithTranscript(t *testing.T) {
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
	resumeID := "77777777-7777-4777-8777-777777777777"
	client := newFakeDMClient()
	client.sessionID = resumeID
	client.onQuery = func(_ context.Context, _ string) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-resume-model-change",
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
	permission.BindSession(sessionKey, sender)

	workspacePath := filepath.Join(cfg.WorkspacePath, cfg.DefaultAgentID)
	writeTranscriptFixture(t, workspacePath, resumeID, []map[string]any{
		{
			"type":      "user",
			"uuid":      "77000000-0000-4000-8000-000000000001",
			"sessionId": resumeID,
			"timestamp": "2026-06-09T00:00:00Z",
			"cwd":       workspacePath,
			"message": map[string]any{
				"role":    "user",
				"content": "旧模型会话消息",
			},
		},
	})
	now := time.Now().UTC()
	if _, err := service.files.UpsertSession(workspacePath, protocol.Session{
		SessionKey:   sessionKey,
		AgentID:      cfg.DefaultAgentID,
		SessionID:    &resumeID,
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
		Content:    "测试模型变更仍 resume",
		RoundID:    "round-stale-model",
		ReqID:      "round-stale-model",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	options := factory.LastOptions()
	if options.Session.ResumeID != resumeID {
		t.Fatalf("runtime 模型变更不应阻止 transcript resume: %+v", options)
	}
	if options.Model != "glm-5.1" {
		t.Fatalf("runtime 应使用当前 provider model: %+v", options)
	}
	sessionValue, _ := mustFindDMSession(t, service, cfg, sessionKey)
	if stringPointer(t, sessionValue.SessionID) != resumeID {
		t.Fatalf("模型变更 resume 不应替换 sdk session_id: %+v", sessionValue)
	}
	if sessionValue.Options[protocol.OptionRuntimeModel] != "glm-5.1" {
		t.Fatalf("runtime model 指纹未回写: %+v", sessionValue.Options)
	}
}

func TestServiceHandleChatSkipsStaleSDKSessionWhenRuntimeKindFingerprintDiffersWithoutTranscript(t *testing.T) {
	isolateDMRuntimeKindEnv(t)

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
	client.sessionID = "sdk-new-claude-kind"
	client.onQuery = func(_ context.Context, _ string) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-new-claude-kind",
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
	service.SetPreferences(fakeDMPreferencesService{prefs: preferencessvc.Preferences{
		AgentRuntimeKind: "claude",
	}})
	sender := newDMTestSender("sender-stale-runtime-kind")
	sessionKey := "agent:nexus:ws:dm:stale-runtime-kind"
	permission.BindSession(sessionKey, sender)

	staleResumeID := "44444444-4444-4444-8444-444444444444"
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
		Title:        "Stale Runtime Kind",
		Options: map[string]any{
			protocol.OptionRuntimeKind:     "nxs",
			protocol.OptionRuntimeProvider: "glm",
			protocol.OptionRuntimeModel:    "glm-5.1",
		},
		IsActive: true,
	}); err != nil {
		t.Fatalf("预写入 runtime kind stale 会话 meta 失败: %v", err)
	}

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "测试 runtime kind 变更不 resume",
		RoundID:    "round-stale-runtime-kind",
		ReqID:      "round-stale-runtime-kind",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	options := factory.LastOptions()
	if options.Session.ResumeID != "" {
		t.Fatalf("runtime kind 变更后不应 resume 过期 sdk session: %+v", options)
	}
	if options.Runtime.Kind != agentclient.RuntimeClaude {
		t.Fatalf("runtime 应切到 Claude: %+v", options)
	}
	sessionValue, _ := mustFindDMSession(t, service, cfg, sessionKey)
	if stringPointer(t, sessionValue.SessionID) != "sdk-new-claude-kind" {
		t.Fatalf("新 sdk session_id 未回写: %+v", sessionValue)
	}
	if sessionValue.Options[protocol.OptionRuntimeKind] != "claude" {
		t.Fatalf("runtime kind 指纹未回写: %+v", sessionValue.Options)
	}
}

func TestServiceHandleChatReusesSDKSessionWhenRuntimeKindSwitchHasSharedTranscript(t *testing.T) {
	isolateDMRuntimeKindEnv(t)

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
	sharedResumeID := "55555555-5555-4555-8555-555555555555"
	client := newFakeDMClient()
	client.sessionID = sharedResumeID
	client.onQuery = func(_ context.Context, _ string) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: sharedResumeID,
				UUID:      "result-shared-runtime-resume",
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
	service.SetPreferences(fakeDMPreferencesService{prefs: preferencessvc.Preferences{
		AgentRuntimeKind: "claude",
	}})
	sender := newDMTestSender("sender-shared-runtime-resume")
	sessionKey := "agent:nexus:ws:dm:shared-runtime-resume"
	permission.BindSession(sessionKey, sender)

	workspacePath := filepath.Join(cfg.WorkspacePath, cfg.DefaultAgentID)
	writeTranscriptFixture(t, workspacePath, sharedResumeID, []map[string]any{
		{
			"type":       "user",
			"uuid":       "55000000-0000-4000-8000-000000000001",
			"sessionId":  sharedResumeID,
			"parentUuid": nil,
			"timestamp":  "2026-06-09T00:00:00Z",
			"cwd":        workspacePath,
			"message": map[string]any{
				"role":    "user",
				"content": "继续之前的任务",
			},
		},
		{
			"type":       "assistant",
			"uuid":       "55000000-0000-4000-8000-000000000002",
			"sessionId":  sharedResumeID,
			"parentUuid": "55000000-0000-4000-8000-000000000001",
			"timestamp":  "2026-06-09T00:00:01Z",
			"message": map[string]any{
				"role": "assistant",
				"content": []any{map[string]any{
					"type": "text",
					"text": "旧 runtime 已经写入 transcript",
				}},
			},
		},
	})
	if exists, err := service.history.TranscriptSessionExists(workspacePath, sharedResumeID); err != nil || !exists {
		t.Fatalf("shared transcript fixture 未被 history store 识别: exists=%v err=%v", exists, err)
	}

	now := time.Now().UTC()
	if _, err := service.files.UpsertSession(workspacePath, protocol.Session{
		SessionKey:   sessionKey,
		AgentID:      cfg.DefaultAgentID,
		SessionID:    &sharedResumeID,
		ChannelType:  "websocket",
		ChatType:     "dm",
		Status:       "active",
		CreatedAt:    now,
		LastActivity: now,
		Title:        "Shared Runtime Resume",
		Options: map[string]any{
			protocol.OptionRuntimeKind:     "nxs",
			protocol.OptionRuntimeProvider: "glm",
			protocol.OptionRuntimeModel:    "glm-5.1",
		},
		IsActive: true,
	}); err != nil {
		t.Fatalf("预写入 shared runtime resume 会话 meta 失败: %v", err)
	}
	if sessionValue, _ := mustFindDMSession(t, service, cfg, sessionKey); stringPointer(t, sessionValue.SessionID) != sharedResumeID {
		t.Fatalf("预写入 session meta 未被 DM service 识别: %+v", sessionValue)
	}

	if err := service.HandleChat(context.Background(), Request{
		SessionKey: sessionKey,
		Content:    "测试 runtime 切换共享 resume",
		RoundID:    "round-shared-runtime-resume",
		ReqID:      "round-shared-runtime-resume",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	collectEventsUntil(t, sender.events, func(event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	options := factory.LastOptions()
	if options.Session.ResumeID != sharedResumeID {
		t.Fatalf("runtime kind 切换时应复用共享 transcript resume: %+v", options)
	}
	if options.Runtime.Kind != agentclient.RuntimeClaude {
		t.Fatalf("runtime 应切到 Claude: %+v", options)
	}
	sessionValue, _ := mustFindDMSession(t, service, cfg, sessionKey)
	if stringPointer(t, sessionValue.SessionID) != sharedResumeID {
		t.Fatalf("共享 resume 不应替换 sdk session_id: %+v", sessionValue)
	}
	if sessionValue.Options[protocol.OptionRuntimeKind] != "claude" {
		t.Fatalf("runtime kind 指纹未更新为新 runtime: %+v", sessionValue.Options)
	}
}

func TestServiceResolveReusableSDKSessionIDReusesSharedTranscriptRuntimeSwitch(t *testing.T) {
	cfg := newDMTestConfig(t)
	agentService := newDMAgentService(t, cfg)
	service := NewService(cfg, agentService, runtimectx.NewManagerWithFactory(&fakeDMFactory{}), permissionctx.NewContext())

	workspacePath := filepath.Join(cfg.WorkspacePath, cfg.DefaultAgentID)
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

	got := service.resolveReusableSDKSessionID(context.Background(), workspacePath, protocol.Session{
		SessionKey: "agent:nexus:ws:dm:resolve-shared-runtime-resume",
		AgentID:    cfg.DefaultAgentID,
		SessionID:  &resumeID,
		Options: map[string]any{
			protocol.OptionRuntimeKind:     "nxs",
			protocol.OptionRuntimeProvider: "glm",
			protocol.OptionRuntimeModel:    "glm-5.1",
		},
	}, "glm", agentclient.Options{
		Model: "glm-5.1",
		Session: agentclient.SessionOptions{
			ResumeID: resumeID,
		},
		Runtime: agentclient.RuntimeOptions{
			Kind: agentclient.RuntimeClaude,
		},
	})
	if got != resumeID {
		t.Fatalf("runtime 切换存在共享 transcript 时应复用 resume: got=%q want=%q", got, resumeID)
	}
}
