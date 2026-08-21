package dm

import (
	"context"
	"errors"
	"strconv"
	"strings"
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

type forkRoomSessionStore struct {
	sessions  map[string]protocol.Session
	updateErr error
}

func (s forkRoomSessionStore) GetRoomSessionByKey(
	_ context.Context,
	_ string,
	key protocol.SessionKey,
) (*protocol.Session, error) {
	item, ok := s.sessions[key.Raw]
	if !ok {
		return nil, nil
	}
	copy := item
	return &copy, nil
}

func (s forkRoomSessionStore) UpdateRoomSessionSDKSessionID(context.Context, string, string) error {
	return s.updateErr
}

func TestConversationForkPreparesAndMaterializesIndependentHistory(t *testing.T) {
	cfg := newDMTestConfig(t)
	migrateDMSQLite(t, cfg.DatabaseURL)
	agentService := newDMAgentService(t, cfg)
	if err := agentService.EnsureReady(context.Background()); err != nil {
		t.Fatalf("初始化 Agent 失败: %v", err)
	}
	service := NewService(
		cfg,
		agentService,
		runtimectx.NewManagerWithFactory(&fakeDMFactory{}),
		permissionctx.NewContext(),
	)
	workspacePath := dmMainWorkspacePath(cfg)
	sourceSessionID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	sourceSessionKey := "agent:nexus:ws:dm:fork-source"
	targetSessionKey := "agent:nexus:ws:dm:fork-target"
	racingTargetSessionKey := "agent:nexus:ws:dm:fork-target-racing-delete"
	now := time.Now().UTC()
	service.SetRoomSessionStore(forkRoomSessionStore{sessions: map[string]protocol.Session{
		sourceSessionKey: {
			SessionKey:   sourceSessionKey,
			AgentID:      cfg.DefaultAgentID,
			SessionID:    &sourceSessionID,
			ChannelType:  "websocket",
			ChatType:     "dm",
			Status:       "active",
			CreatedAt:    now,
			LastActivity: now,
			Options: map[string]any{
				protocol.OptionRuntimeKind:     "claude",
				protocol.OptionRuntimeProvider: "glm",
				protocol.OptionRuntimeModel:    "glm-5.1",
			},
			IsActive: true,
		},
		targetSessionKey: {
			SessionKey:   targetSessionKey,
			AgentID:      cfg.DefaultAgentID,
			ChannelType:  "websocket",
			ChatType:     "dm",
			Status:       "active",
			CreatedAt:    now,
			LastActivity: now,
			Options:      map[string]any{},
			IsActive:     true,
		},
		racingTargetSessionKey: {
			SessionKey:   racingTargetSessionKey,
			AgentID:      cfg.DefaultAgentID,
			ChannelType:  "websocket",
			ChatType:     "dm",
			Status:       "active",
			CreatedAt:    now,
			LastActivity: now,
			Options:      map[string]any{},
			IsActive:     true,
		},
	}})

	history := workspacestore.NewAgentHistoryStore(cfg.WorkspacePath)
	transcriptRows := make([]map[string]any, 0, 6)
	for index := 1; index <= 3; index++ {
		roundID := "round-" + strconv.Itoa(index)
		timestamp := int64(index * 1000)
		if err := history.AppendRoundMarker(
			workspacePath,
			sourceSessionKey,
			roundID,
			"问题 "+roundID,
			timestamp,
		); err != nil {
			t.Fatalf("写入 %s marker 失败: %v", roundID, err)
		}
		userID := "user-" + roundID
		assistantID := "assistant-" + roundID
		parentID := ""
		if index > 1 {
			parentID = "assistant-round-" + strconv.Itoa(index-1)
		}
		transcriptRows = append(transcriptRows,
			map[string]any{
				"type": "user", "uuid": userID, "parentUuid": parentID,
				"sessionId": sourceSessionID, "timestamp": timestamp,
				"message": map[string]any{"role": "user", "content": "问题 " + roundID},
			},
			map[string]any{
				"type": "assistant", "uuid": assistantID, "parentUuid": userID,
				"sessionId": sourceSessionID, "timestamp": timestamp + 100,
				"message": map[string]any{
					"role":    "assistant",
					"content": []map[string]any{{"type": "text", "text": "回答 " + roundID}},
				},
			},
		)
		if err := history.AppendOverlayMessage(workspacePath, sourceSessionKey, protocol.Message{
			"message_id": "result-" + roundID,
			"round_id":   roundID,
			"role":       "result",
			"subtype":    "success",
			"timestamp":  timestamp + 200,
		}); err != nil {
			t.Fatalf("写入 %s result 失败: %v", roundID, err)
		}
	}
	writeTranscriptFixture(t, workspacePath, sourceSessionID, transcriptRows)

	preparedSessionID, preparedMessageID, err := service.PrepareConversationFork(
		context.Background(),
		sourceSessionKey,
		"round-2",
	)
	if err != nil {
		t.Fatalf("预检 fork 失败: %v", err)
	}
	if preparedSessionID != sourceSessionID || preparedMessageID != "assistant-round-2" {
		t.Fatalf("fork 边界 = (%q, %q)", preparedSessionID, preparedMessageID)
	}
	if err = service.ForkConversationSession(
		context.Background(),
		sourceSessionKey,
		targetSessionKey,
		"round-2",
		preparedSessionID,
		preparedMessageID,
	); err != nil {
		t.Fatalf("物化 fork 失败: %v", err)
	}

	target, _ := mustFindDMSession(t, service, cfg, targetSessionKey)
	if sourceID, messageID := pendingConversationFork(target.Options); sourceID != sourceSessionID || messageID != preparedMessageID {
		t.Fatalf("pending fork 依赖不正确: source=%q message=%q", sourceID, messageID)
	}
	targetRows := readDMSessionHistory(t, cfg, service, targetSessionKey)
	if forkHistoryHasRound(targetRows, "round-3") ||
		!forkHistoryHasRound(targetRows, "round-1") ||
		!forkHistoryHasRound(targetRows, "round-2") {
		t.Fatalf("target fork 历史边界不正确: %+v", targetRows)
	}
	sourceRows := readDMSessionHistory(t, cfg, service, sourceSessionKey)
	if !forkHistoryHasRound(sourceRows, "round-3") {
		t.Fatal("fork 不应改写 source 历史")
	}

	preparedSessionID, preparedMessageID, err = service.PrepareConversationFork(
		context.Background(),
		sourceSessionKey,
		"round-2",
	)
	if err != nil {
		t.Fatalf("并发删除前预检 fork 失败: %v", err)
	}
	agentValue, err := agentService.GetAgent(context.Background(), cfg.DefaultAgentID)
	if err != nil {
		t.Fatalf("读取 fork Agent 失败: %v", err)
	}
	if _, err = service.files.ForOwner(agentValue.OwnerUserID).DeleteSession(
		workspacePath,
		sourceSessionKey,
	); err != nil {
		t.Fatalf("模拟删除 source workspace Session 失败: %v", err)
	}
	targetRows = readDMSessionHistory(t, cfg, service, targetSessionKey)
	if !forkHistoryHasRound(targetRows, "round-1") || !forkHistoryHasRound(targetRows, "round-2") {
		t.Fatalf("删除 source workspace Session 后 pending fork 应保留完整轮次: %+v", targetRows)
	}
	if err = service.ForkConversationSession(
		context.Background(),
		sourceSessionKey,
		racingTargetSessionKey,
		"round-2",
		preparedSessionID,
		preparedMessageID,
	); err == nil {
		t.Fatal("source marker 并发消失时不应生成轮次身份不完整的 fork")
	}
}

func forkHistoryHasRound(rows []protocol.Message, roundID string) bool {
	for _, row := range rows {
		if strings.TrimSpace(protocol.MessageRoundID(row)) == roundID {
			return true
		}
	}
	return false
}

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
	if options.Session.ID != "" {
		t.Fatalf("fork target session id 应由 runtime 分配: %+v", options.Session)
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

func TestSDKSessionSyncKeepsWorkspacePendingWhenRoomPersistenceFails(t *testing.T) {
	cfg := newDMTestConfig(t)
	agentService := newDMAgentService(t, cfg)
	service := NewService(
		cfg,
		agentService,
		runtimectx.NewManagerWithFactory(&fakeDMFactory{}),
		permissionctx.NewContext(),
	)
	persistErr := errors.New("persist room fork identity")
	service.SetRoomSessionStore(forkRoomSessionStore{updateErr: persistErr})

	workspacePath := dmMainWorkspacePath(cfg)
	newSessionID := "55555555-5555-4555-8555-555555555555"
	writeTranscriptFixture(t, workspacePath, newSessionID, []map[string]any{{
		"type":      "user",
		"uuid":      "55000000-0000-4000-8000-000000000001",
		"sessionId": newSessionID,
		"timestamp": "2026-08-17T00:00:00Z",
		"cwd":       workspacePath,
		"message": map[string]any{
			"role":    "user",
			"content": "继续 fork",
		},
	}})
	roomSessionID := "room-session-fork-pending"
	now := time.Now().UTC()
	stored, err := service.files.UpsertSession(workspacePath, protocol.Session{
		SessionKey:    "agent:nexus:ws:dm:fork-room-persist-failure",
		AgentID:       cfg.DefaultAgentID,
		RoomSessionID: &roomSessionID,
		ChannelType:   "websocket",
		ChatType:      "dm",
		Status:        "active",
		CreatedAt:     now,
		LastActivity:  now,
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

	if _, err = service.syncSDKSessionIDForOwner(
		context.Background(),
		"__system__",
		workspacePath,
		*stored,
		newSessionID,
		"claude",
		"glm",
		"glm-5.1",
		"surface",
	); !errors.Is(err, persistErr) {
		t.Fatalf("Room 持久化失败应原样返回: %v", err)
	}
	reloaded, _, err := service.files.ForOwner("__system__").FindSession(
		[]string{workspacePath},
		stored.SessionKey,
	)
	if err != nil || reloaded == nil {
		t.Fatalf("重读 pending fork Session 失败: session=%+v err=%v", reloaded, err)
	}
	if reloaded.SessionID != nil && strings.TrimSpace(*reloaded.SessionID) != "" {
		t.Fatalf("SQL 未提交时 workspace 不应先切换 target SDK: %q", *reloaded.SessionID)
	}
	if sourceID, messageID := pendingConversationFork(reloaded.Options); sourceID != "source-session" || messageID != "source-boundary" {
		t.Fatalf("SQL 未提交时 workspace 应保留 pending fork: source=%q message=%q", sourceID, messageID)
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

func TestCompletedAssistantRoundUsesPagedMessageSemantics(t *testing.T) {
	tests := []struct {
		name   string
		rows   []protocol.Message
		active []string
		want   bool
	}{
		{
			name: "successful assistant",
			rows: []protocol.Message{{
				"round_id": "round-target",
				"role":     "assistant",
				"result_summary": map[string]any{
					"subtype": "success",
				},
			}},
			want: true,
		},
		{
			name: "active assistant",
			rows: []protocol.Message{{
				"round_id": "round-target",
				"role":     "assistant",
			}},
			active: []string{"round-target"},
		},
		{
			name: "interrupted assistant",
			rows: []protocol.Message{{
				"round_id":    "round-target",
				"role":        "assistant",
				"stop_reason": "cancelled",
			}},
		},
		{
			name: "result without assistant",
			rows: []protocol.Message{{
				"round_id": "round-target",
				"role":     "result",
				"subtype":  "success",
			}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := completedAssistantRound(test.rows, "round-target", test.active); got != test.want {
				t.Fatalf("completedAssistantRound() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestLatestCompletedAssistantRoundSkipsActiveAndFailedTail(t *testing.T) {
	rows := []protocol.Message{
		{
			"round_id": "round-success",
			"role":     "assistant",
			"result_summary": map[string]any{
				"subtype": "success",
			},
		},
		{
			"round_id":    "round-failed",
			"role":        "assistant",
			"stop_reason": "error",
		},
		{
			"round_id": "round-active",
			"role":     "assistant",
		},
	}
	if got := latestCompletedAssistantRound(rows, []string{"round-active"}); got != "round-success" {
		t.Fatalf("latestCompletedAssistantRound() = %q, want round-success", got)
	}
}

func TestTransientForkAtTranscriptTailOmitsProviderSpecificMessageBoundary(t *testing.T) {
	legacyMessageID := "msg_20260821090320f5fbc914cb404581-2"
	if got := conversationForkResumeAt(map[string]any{
		protocol.OptionRuntimeForkAtTranscriptTail: true,
	}, legacyMessageID); got != "" {
		t.Fatalf("conversationForkResumeAt() = %q, want full source transcript", got)
	}
	if got := conversationForkResumeAt(nil, legacyMessageID); got != legacyMessageID {
		t.Fatalf("bounded conversationForkResumeAt() = %q, want %q", got, legacyMessageID)
	}
	if !transcriptRoundIsSessionTail(workspacestore.TranscriptRoundTail{
		RoundIDs: []string{"round-target"},
	}, "round-target") {
		t.Fatal("single terminal round was not recognized as the transcript tail")
	}
	if transcriptRoundIsSessionTail(workspacestore.TranscriptRoundTail{
		RoundIDs: []string{"round-target", "round-later"},
	}, "round-target") {
		t.Fatal("historical round was incorrectly recognized as the transcript tail")
	}
}
