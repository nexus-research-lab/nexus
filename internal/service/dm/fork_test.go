package dm

import (
	"context"
	"testing"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func TestEnsureClientForConversationForkUsesSourceBoundary(t *testing.T) {
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
	client.sessionID = "fork-session-new"
	factory := &fakeDMFactory{client: client}
	runtimeManager := runtimectx.NewManagerWithFactory(factory)
	service := NewService(cfg, agentService, runtimeManager, permissionctx.NewContext())
	service.SetProviderResolver(providerService)
	service.SetPreferences(fakeDMPreferencesService{prefs: preferencessvc.Preferences{
		AgentRuntimeKind: "nxs",
	}})

	ctx := context.Background()
	agentValue, err := agentService.GetAgent(ctx, cfg.DefaultAgentID)
	if err != nil {
		t.Fatalf("读取默认 Agent 失败: %v", err)
	}
	workspacePath := dmMainWorkspacePath(cfg)
	sourceSessionID := "11111111-1111-4111-8111-111111111111"
	boundaryUUID := "11000000-0000-4000-8000-000000000001"
	writeTranscriptFixture(t, workspacePath, sourceSessionID, []map[string]any{{
		"type":      "assistant",
		"uuid":      boundaryUUID,
		"sessionId": sourceSessionID,
		"timestamp": "2026-08-17T00:00:00Z",
		"cwd":       workspacePath,
		"message": map[string]any{
			"role":    "assistant",
			"content": []map[string]any{{"type": "text", "text": "完成"}},
		},
	}})
	targetSessionKey := "agent:nexus:ws:dm:fork-target-options"
	now := time.Now().UTC()
	targetSession := protocol.Session{
		SessionKey:   targetSessionKey,
		AgentID:      cfg.DefaultAgentID,
		ChannelType:  "websocket",
		ChatType:     "dm",
		Status:       "active",
		CreatedAt:    now,
		LastActivity: now,
		Title:        "Fork Target",
		Options: protocol.WithSessionRuntimeSettings(map[string]any{
			protocol.OptionRuntimeKind:                "claude",
			protocol.OptionRuntimeProvider:            "glm",
			protocol.OptionRuntimeModel:               "glm-5.1",
			protocol.OptionRuntimeForkSourceSessionID: sourceSessionID,
			protocol.OptionRuntimeForkMessageID:       boundaryUUID,
		}, protocol.SessionRuntimeSettings{
			Provider: "glm",
			Model:    "glm-5.1",
		}),
		IsActive: true,
	}
	if _, err = service.files.UpsertSession(workspacePath, targetSession); err != nil {
		t.Fatalf("写入 target session 失败: %v", err)
	}

	if _, err = service.ensureClient(ctx, targetSessionKey, agentValue, targetSession, Request{
		RoundID:      "round-fork-setup",
		AgentRoundID: "agent-round-fork-setup",
	}); err != nil {
		t.Fatalf("准备 fork runtime 失败: %v", err)
	}
	t.Cleanup(func() {
		_ = runtimeManager.CloseSession(context.Background(), targetSessionKey)
	})

	options := factory.LastOptions()
	if options.Session.ResumeID != sourceSessionID ||
		options.Session.ResumeAt != boundaryUUID ||
		!options.Session.Fork {
		t.Fatalf("fork runtime options 不正确: %+v", options.Session)
	}
	if options.Runtime.Kind != agentclient.RuntimeClaude {
		t.Fatalf("fork 必须沿用 source runtime，got=%q", options.Runtime.Kind)
	}
	if options.Session.ID == "" {
		t.Fatal("Claude fork 必须在启动前固定 target session id")
	}
}

func TestSDKSessionSyncClearsPendingConversationFork(t *testing.T) {
	cfg := newDMTestConfig(t)
	agentService := newDMAgentService(t, cfg)
	service := NewService(
		cfg,
		agentService,
		runtimectx.NewManagerWithFactory(&fakeDMFactory{}),
		permissionctx.NewContext(),
	)

	workspacePath := dmMainWorkspacePath(cfg)
	newSessionID := "44444444-4444-4444-8444-444444444444"
	writeTranscriptFixture(t, workspacePath, newSessionID, []map[string]any{{
		"type":      "user",
		"uuid":      "44000000-0000-4000-8000-000000000001",
		"sessionId": newSessionID,
		"timestamp": "2026-08-17T00:00:00Z",
		"cwd":       workspacePath,
		"message": map[string]any{
			"role":    "user",
			"content": "继续 fork",
		},
	}})
	now := time.Now().UTC()
	stored, err := service.files.UpsertSession(workspacePath, protocol.Session{
		SessionKey:   "agent:nexus:ws:dm:fork-materialize",
		AgentID:      cfg.DefaultAgentID,
		ChannelType:  "websocket",
		ChatType:     "dm",
		Status:       "active",
		CreatedAt:    now,
		LastActivity: now,
		Options: map[string]any{
			protocol.OptionRuntimeKind:                "claude",
			protocol.OptionRuntimeProvider:            "glm",
			protocol.OptionRuntimeModel:               "glm-5.1",
			protocol.OptionRuntimeForkSourceSessionID: "source-session",
			protocol.OptionRuntimeForkMessageID:       "source-boundary",
		},
		IsActive: true,
	})
	if err != nil || stored == nil {
		t.Fatalf("预写入 pending fork Session 失败: stored=%+v err=%v", stored, err)
	}

	updated, err := service.syncSDKSessionIDForOwner(
		context.Background(),
		"__system__",
		workspacePath,
		*stored,
		newSessionID,
		"claude",
		"glm",
		"glm-5.1",
		"surface",
	)
	if err != nil {
		t.Fatalf("物化 fork session 失败: %v", err)
	}
	if got := stringPointer(t, updated.SessionID); got != newSessionID {
		t.Fatalf("fork session_id = %q, want %q", got, newSessionID)
	}
	if sourceID, messageID := pendingConversationFork(updated.Options); sourceID != "" || messageID != "" {
		t.Fatalf("pending fork 未清理: source=%q message=%q", sourceID, messageID)
	}
}

func TestResolveConversationForkBoundaryFindsHistoricalTranscriptSegment(t *testing.T) {
	cfg := newDMTestConfig(t)
	workspacePath := dmMainWorkspacePath(cfg)
	sessionKey := "agent:nexus:ws:dm:segmented-fork-source"
	oldSessionID := "22222222-2222-4222-8222-222222222222"
	currentSessionID := "33333333-3333-4333-8333-333333333333"
	history := workspacestore.NewAgentHistoryStore(cfg.WorkspacePath)
	if err := history.AppendRoundMarker(workspacePath, sessionKey, "round-old", "旧问题", 1000); err != nil {
		t.Fatalf("写入旧 round marker 失败: %v", err)
	}
	if err := history.AppendRoundMarker(workspacePath, sessionKey, "round-new", "新问题", 2000); err != nil {
		t.Fatalf("写入新 round marker 失败: %v", err)
	}
	writeTranscriptFixture(t, workspacePath, oldSessionID, []map[string]any{
		{
			"type": "user", "uuid": "old-user", "sessionId": oldSessionID,
			"timestamp": 1000,
			"message":   map[string]any{"role": "user", "content": "旧问题"},
		},
		{
			"type": "assistant", "uuid": "old-assistant", "parentUuid": "old-user",
			"sessionId": oldSessionID, "timestamp": 1100,
			"message": map[string]any{"role": "assistant", "content": "旧回答"},
		},
	})
	writeTranscriptFixture(t, workspacePath, currentSessionID, []map[string]any{
		{
			"type": "user", "uuid": "new-user", "sessionId": currentSessionID,
			"timestamp": 2000,
			"message":   map[string]any{"role": "user", "content": "新问题"},
		},
		{
			"type": "assistant", "uuid": "new-assistant", "parentUuid": "new-user",
			"sessionId": currentSessionID, "timestamp": 2100,
			"message": map[string]any{"role": "assistant", "content": "新回答"},
		},
	})

	sessionID, messageID, err := resolveConversationForkBoundary(
		history,
		workspacePath,
		sessionKey,
		protocol.Session{
			SessionID:            &currentSessionID,
			TranscriptSessionIDs: []string{oldSessionID, currentSessionID},
			Options: map[string]any{
				protocol.OptionRuntimeSegmentedTranscript: true,
			},
		},
		"round-old",
	)
	if err != nil {
		t.Fatalf("解析历史 transcript segment 失败: %v", err)
	}
	if sessionID != oldSessionID || messageID != "old-assistant" {
		t.Fatalf("fork boundary = (%q, %q), want (%q, old-assistant)", sessionID, messageID, oldSessionID)
	}
}
