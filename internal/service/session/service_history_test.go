package session_test

import (
	"context"
	"strings"
	"testing"
	"time"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	sessionsvc "github.com/nexus-research-lab/nexus/internal/service/session"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	_ "modernc.org/sqlite"
)

func TestSessionServiceGetSessionMessagesSkipsActiveRoundMaterialization(t *testing.T) {
	cfg := newSessionTestConfig(t)
	migrateSessionSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	sessionService := serverapp.NewSessionServiceWithDB(cfg, db, agentService)
	runtimeManager := runtimectx.NewManager()
	sessionService.SetRuntimeManager(runtimeManager)

	ctx := context.Background()
	agentA, err := agentService.CreateAgent(ctx, protocol.CreateRequest{Name: "活跃轮次助手"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}
	dmKey := protocol.BuildAgentSessionKey(agentA.AgentID, "ws", "dm", "active-"+agentA.AgentID, "")
	sessionValue, err := sessionService.CreateSession(ctx, sessionsvc.CreateRequest{SessionKey: dmKey})
	if err != nil {
		t.Fatalf("创建 session 失败: %v", err)
	}
	dmSessionID := bindTranscriptSessionID(t, cfg, agentA.WorkspacePath, sessionValue)
	seedWorkspaceSessionArtifacts(t, cfg, agentA.WorkspacePath, dmKey, dmSessionID)
	runtimeManager.StartRound(dmKey, "round_1", nil)
	defer runtimeManager.MarkRoundFinished(dmKey, "round_1")

	messages, err := sessionService.GetSessionMessages(ctx, dmKey)
	if err != nil {
		t.Fatalf("读取 session 消息失败: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("活跃 round 不应物化 interrupted result: got=%d want=2", len(messages))
	}
	if _, exists := messages[1]["stream_status"]; exists {
		t.Fatalf("活跃 round 不应把 assistant 快照强制终止: %+v", messages[1])
	}
}

func TestSessionServiceReconcilesStaleActiveWorkspaceMeta(t *testing.T) {
	cfg := newSessionTestConfig(t)
	migrateSessionSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	sessionService := serverapp.NewSessionServiceWithDB(cfg, db, agentService)
	runtimeManager := runtimectx.NewManager()
	sessionService.SetRuntimeManager(runtimeManager)

	ctx := context.Background()
	agentA, err := agentService.CreateAgent(ctx, protocol.CreateRequest{Name: "残留活跃状态助手"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}
	dmKey := protocol.BuildAgentSessionKey(agentA.AgentID, "ws", "dm", "stale-"+agentA.AgentID, "")
	created, err := sessionService.CreateSession(ctx, sessionsvc.CreateRequest{SessionKey: dmKey})
	if err != nil {
		t.Fatalf("创建 session 失败: %v", err)
	}
	if created.Status != "closed" || created.IsActive {
		t.Fatalf("新建空闲 session 应为 closed: %+v", created)
	}

	store := workspacestore.NewSessionFileStore(cfg.WorkspacePath)
	created.Status = "active"
	created.IsActive = true
	if _, err := store.UpsertSession(agentA.WorkspacePath, *created); err != nil {
		t.Fatalf("写入残留 active meta 失败: %v", err)
	}

	reconciled, err := sessionService.GetSession(ctx, dmKey)
	if err != nil {
		t.Fatalf("读取 session 失败: %v", err)
	}
	if reconciled.Status != "closed" || reconciled.IsActive {
		t.Fatalf("无运行 round 时应纠正为 closed: %+v", reconciled)
	}
	persisted, _, err := store.FindSession([]string{agentA.WorkspacePath}, dmKey)
	if err != nil {
		t.Fatalf("读取持久化 meta 失败: %v", err)
	}
	if persisted == nil || persisted.Status != "closed" || persisted.IsActive {
		t.Fatalf("残留 active meta 未持久化纠正: %+v", persisted)
	}

	runtimeManager.StartRound(dmKey, "round_running", nil)
	active, err := sessionService.GetSession(ctx, dmKey)
	if err != nil {
		t.Fatalf("读取运行中 session 失败: %v", err)
	}
	if active.Status != "active" || !active.IsActive {
		t.Fatalf("有运行 round 时应回到 active: %+v", active)
	}
	runtimeManager.MarkRoundFinished(dmKey, "round_running")

	agentSessions, err := sessionService.ListAgentSessions(ctx, agentA.AgentID)
	if err != nil {
		t.Fatalf("读取 agent sessions 失败: %v", err)
	}
	if len(agentSessions) != 1 || agentSessions[0].Status != "closed" || agentSessions[0].IsActive {
		t.Fatalf("运行 round 结束后列表应纠正为 closed: %+v", agentSessions)
	}
}

func TestSessionServiceReadsTranscriptHistoryWithRoundMarkers(t *testing.T) {
	cfg := newSessionTestConfig(t)
	migrateSessionSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	sessionService := serverapp.NewSessionServiceWithDB(cfg, db, agentService)

	ctx := context.Background()
	agentA, err := agentService.CreateAgent(ctx, protocol.CreateRequest{Name: "Transcript 助手"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}

	dmKey := protocol.BuildAgentSessionKey(agentA.AgentID, "ws", "dm", "transcript-"+agentA.AgentID, "")
	created, err := sessionService.CreateSession(ctx, sessionsvc.CreateRequest{SessionKey: dmKey})
	if err != nil {
		t.Fatalf("创建 transcript session 失败: %v", err)
	}

	sessionID := "550e8400-e29b-41d4-a716-446655440000"
	created.SessionID = &sessionID
	store := workspacestore.NewSessionFileStore(cfg.WorkspacePath)
	if _, err := store.UpsertSession(agentA.WorkspacePath, *created); err != nil {
		t.Fatalf("回写 session_id 失败: %v", err)
	}

	history := workspacestore.NewAgentHistoryStore(cfg.WorkspacePath)
	if err := history.AppendRoundMarker(agentA.WorkspacePath, dmKey, "round_transcript_1", "请总结这个仓库", time.Now().Add(-2*time.Second).UnixMilli()); err != nil {
		t.Fatalf("写入 round marker 失败: %v", err)
	}
	writeSessionTranscriptFixture(t, agentA.WorkspacePath, sessionID, []map[string]any{
		{
			"type":      "user",
			"uuid":      "transcript-user-1",
			"sessionId": sessionID,
			"timestamp": time.Now().Add(-2 * time.Second).UTC().Format(time.RFC3339Nano),
			"message": map[string]any{
				"role":    "user",
				"content": "请总结这个仓库",
			},
		},
		{
			"type":       "assistant",
			"uuid":       "transcript-assistant-1",
			"sessionId":  sessionID,
			"parentUuid": "transcript-user-1",
			"message": map[string]any{
				"role":        "assistant",
				"stop_reason": "end_turn",
				"content": []map[string]any{
					{"type": "text", "text": "这是一个 Go + React 的 Nexus 项目。"},
				},
			},
		},
		{
			"type":            "result",
			"uuid":            "transcript-result-1",
			"session_id":      sessionID,
			"parentUuid":      "transcript-assistant-1",
			"subtype":         "success",
			"duration_ms":     12,
			"duration_api_ms": 8,
			"num_turns":       1,
			"result":          "done",
			"is_error":        false,
		},
	})

	messages, err := sessionService.GetSessionMessages(ctx, dmKey)
	if err != nil {
		t.Fatalf("读取 transcript 历史失败: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("transcript 历史数量不正确: got=%d want=2", len(messages))
	}
	if got := strings.TrimSpace(stringValue(messages[0]["round_id"])); got != "round_transcript_1" {
		t.Fatalf("round marker 未覆盖 transcript round_id: got=%s want=round_transcript_1", got)
	}
	if got := strings.TrimSpace(stringValue(messages[1]["round_id"])); got != "round_transcript_1" {
		t.Fatalf("assistant round_id 未继承 round marker: got=%s want=round_transcript_1", got)
	}
	if _, exists := messages[1]["result_summary"]; exists {
		t.Fatalf("transcript 内置 result 不应直接进入历史摘要: %+v", messages[1])
	}
}

func TestSessionServiceReadsRoomTopicHistoryFromWorkspaceMetaSessionID(t *testing.T) {
	cfg := newSessionTestConfig(t)
	migrateSessionSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	sessionService := serverapp.NewSessionServiceWithDB(cfg, db, agentService)

	ctx := context.Background()
	agentA, err := agentService.CreateAgent(ctx, protocol.CreateRequest{Name: "Room Topic Transcript 助手"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}

	dmContext, err := roomService.EnsureDirectRoom(ctx, agentA.AgentID)
	if err != nil {
		t.Fatalf("创建直聊 room 失败: %v", err)
	}
	topicContext, err := roomService.CreateConversation(ctx, dmContext.Room.ID, protocol.CreateConversationRequest{
		Title: "Topic Transcript",
	})
	if err != nil {
		t.Fatalf("创建话题失败: %v", err)
	}
	if len(topicContext.Sessions) == 0 {
		t.Fatal("话题上下文缺少成员 session")
	}

	sessionKey := protocol.BuildRoomAgentSessionKey(
		topicContext.Conversation.ID,
		agentA.AgentID,
		topicContext.Room.RoomType,
	)
	sessionID := "2944aa53-db7c-4b9f-a3e6-74401402abc5"
	now := time.Now().UTC()
	store := workspacestore.NewSessionFileStore(cfg.WorkspacePath)
	if _, err := store.UpsertSession(agentA.WorkspacePath, protocol.Session{
		SessionKey:     sessionKey,
		AgentID:        agentA.AgentID,
		SessionID:      &sessionID,
		RoomSessionID:  stringPointer(topicContext.Sessions[0].ID),
		RoomID:         stringPointer(topicContext.Room.ID),
		ConversationID: stringPointer(topicContext.Conversation.ID),
		ChannelType:    "ws",
		ChatType:       "dm",
		Status:         "active",
		CreatedAt:      now,
		LastActivity:   now,
		Title:          topicContext.Conversation.Title,
		Options:        map[string]any{},
		IsActive:       true,
	}); err != nil {
		t.Fatalf("回写 room topic session meta 失败: %v", err)
	}

	history := workspacestore.NewAgentHistoryStore(cfg.WorkspacePath)
	if err := history.AppendRoundMarker(agentA.WorkspacePath, sessionKey, "round_room_topic_1", "啥意思", now.Add(-2*time.Second).UnixMilli()); err != nil {
		t.Fatalf("写入 room topic round marker 失败: %v", err)
	}
	writeSessionTranscriptFixture(t, agentA.WorkspacePath, sessionID, []map[string]any{
		{
			"type":      "user",
			"uuid":      "room-topic-user-1",
			"sessionId": sessionID,
			"timestamp": now.Add(-2 * time.Second).UTC().Format(time.RFC3339Nano),
			"message": map[string]any{
				"role":    "user",
				"content": "啥意思",
			},
		},
		{
			"type":       "assistant",
			"uuid":       "room-topic-assistant-1",
			"sessionId":  sessionID,
			"parentUuid": "room-topic-user-1",
			"timestamp":  now.Add(-1500 * time.Millisecond).UTC().Format(time.RFC3339Nano),
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "thinking", "thinking": "先确认用户具体想问什么。"},
				},
			},
		},
		{
			"type":       "assistant",
			"uuid":       "room-topic-assistant-1",
			"sessionId":  sessionID,
			"parentUuid": "room-topic-user-1",
			"timestamp":  now.Add(-time.Second).UTC().Format(time.RFC3339Nano),
			"message": map[string]any{
				"role":        "assistant",
				"stop_reason": "end_turn",
				"content": []map[string]any{
					{"type": "text", "text": "你好！你能具体说说你想问什么吗？"},
				},
			},
		},
	})

	messages, err := sessionService.GetSessionMessages(ctx, sessionKey)
	if err != nil {
		t.Fatalf("读取 room topic transcript 历史失败: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("room topic transcript 历史数量不正确: got=%d want=2 messages=%+v", len(messages), messages)
	}
	if messages[0]["role"] != "user" || messages[1]["role"] != "assistant" {
		t.Fatalf("room topic transcript 历史角色不正确: %+v", messages)
	}
	if _, exists := messages[1]["stream_status"]; exists {
		t.Fatalf("room topic assistant 不应补写 stream_status: %+v", messages[1])
	}
	updatedSession, err := sessionService.GetSession(ctx, sessionKey)
	if err != nil {
		t.Fatalf("读取更新后的 room topic session 失败: %v", err)
	}
	if updatedSession.SessionID == nil || strings.TrimSpace(*updatedSession.SessionID) != sessionID {
		t.Fatalf("room topic sdk_session_id 未从 workspace meta 回写数据库: %+v", updatedSession)
	}
	updatedContext, err := roomService.GetConversationContext(ctx, topicContext.Conversation.ID)
	if err != nil {
		t.Fatalf("读取更新后的 room topic context 失败: %v", err)
	}
	if len(updatedContext.Sessions) == 0 || updatedContext.Sessions[0].SDKSessionID != sessionID {
		t.Fatalf("room topic context 未同步 sdk_session_id: %+v", updatedContext.Sessions)
	}
}
