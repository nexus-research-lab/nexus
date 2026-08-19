package dm

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	providercfg "github.com/nexus-research-lab/nexus/internal/service/provider"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestPreparedRoomForkRejectsSupersededConnectorSelection(t *testing.T) {
	cfg := newDMTestConfig(t)
	agentService := newDMAgentService(t, cfg)
	service := NewService(
		cfg,
		agentService,
		runtimectx.NewManagerWithFactory(&fakeDMFactory{}),
		permissionctx.NewContext(),
	)
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
	current, err := service.files.UpsertSession(workspacePath, protocol.Session{
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
		contextWithExactOwner(context.Background(), "__system__"),
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

	oldSessionID := "19191919-1919-4919-8919-191919191919"
	newSessionID := "29292929-2929-4929-8929-292929292929"
	roomSessionID := "room-session-eager-connector-fork"
	sessionKey := "agent:nexus:ws:dm:eager-connector-tool-surface-fork"
	client := newFakeDMClient()
	client.sessionID = newSessionID
	client.onQuery = func(_ context.Context, _ string) {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: newSessionID,
				UUID:      "result-eager-connector-tool-surface-fork",
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
	permission := permissionctx.NewContext()
	service := NewService(
		cfg,
		agentService,
		runtimectx.NewManagerWithFactory(factory),
		permission,
	)
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
	stored, err := service.files.UpsertSession(workspacePath, workspaceSnapshot)
	if err != nil || stored == nil {
		t.Fatalf("seed Session: item=%+v err=%v", stored, err)
	}
	ctx := contextWithExactOwner(context.Background(), "__system__")
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
