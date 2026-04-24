package session_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	agent2 "github.com/nexus-research-lab/nexus/internal/agent"
	"github.com/nexus-research-lab/nexus/internal/bootstrap"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	roomsvc "github.com/nexus-research-lab/nexus/internal/room"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	sessionsvc "github.com/nexus-research-lab/nexus/internal/session"
	workspace2 "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
)

func TestSessionServiceLifecycle(t *testing.T) {
	cfg := newSessionTestConfig(t)
	migrateSessionSQLite(t, cfg.DatabaseURL)

	agentService, db, err := bootstrap.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := bootstrap.NewRoomServiceWithDB(cfg, db, agentService)
	sessionService := bootstrap.NewSessionServiceWithDB(cfg, db, agentService)
	sessionService.SetRuntimeManager(runtimectx.NewManager())

	ctx := context.Background()
	agentA, err := agentService.CreateAgent(ctx, agent2.CreateRequest{Name: "测试会话助手"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}

	dmKey := protocol.BuildAgentSessionKey(agentA.AgentID, "ws", "dm", "launcher-app-"+agentA.AgentID, "")
	created, err := sessionService.CreateSession(ctx, sessionsvc.CreateRequest{
		SessionKey: dmKey,
		Title:      "Launcher App",
	})
	if err != nil {
		t.Fatalf("创建普通 session 失败: %v", err)
	}
	if created.Title != "Launcher App" {
		t.Fatalf("session 标题不正确: got=%s", created.Title)
	}

	dmSessionID := bindTranscriptSessionID(t, cfg, agentA.WorkspacePath, created)
	seedWorkspaceSessionArtifacts(t, cfg, agentA.AgentID, agentA.WorkspacePath, dmKey, dmSessionID)

	dmContext, err := roomService.EnsureDirectRoom(ctx, agentA.AgentID)
	if err != nil {
		t.Fatalf("创建直聊 room 失败: %v", err)
	}
	seedRoomConversationMessages(t, cfg, dmContext.Conversation.ID)

	sessions, err := sessionService.ListSessions(ctx)
	if err != nil {
		t.Fatalf("列出 sessions 失败: %v", err)
	}
	if len(sessions) < 2 {
		t.Fatalf("session 列表未合并 room 视图: got=%d", len(sessions))
	}

	agentSessions, err := sessionService.ListAgentSessions(ctx, agentA.AgentID)
	if err != nil {
		t.Fatalf("读取 agent sessions 失败: %v", err)
	}
	if len(agentSessions) < 2 {
		t.Fatalf("agent sessions 数量不正确: got=%d", len(agentSessions))
	}

	messages, err := sessionService.GetSessionMessages(ctx, dmKey)
	if err != nil {
		t.Fatalf("读取普通 session 消息失败: %v", err)
	}
	if len(messages) != 3 {
		t.Fatalf("消息归一化结果不正确: got=%d want=3 messages=%+v", len(messages), messages)
	}
	contentBlocks, ok := messages[1]["content"].([]map[string]any)
	if !ok && messages[1]["content"] != nil {
		rawBlocks, okAny := messages[1]["content"].([]any)
		if okAny {
			contentBlocks = make([]map[string]any, 0, len(rawBlocks))
			for _, item := range rawBlocks {
				if payload, okMap := item.(map[string]any); okMap {
					contentBlocks = append(contentBlocks, payload)
				}
			}
			ok = true
		}
	}
	if !ok || len(contentBlocks) != 1 || contentBlocks[0]["type"] != "text" || contentBlocks[0]["text"] != "最终回复" {
		t.Fatalf("消息压缩未保留最新快照: %+v", messages[1])
	}
	if _, exists := messages[1]["stream_status"]; exists {
		t.Fatalf("未终止 round 的 assistant 不应补写 stream_status: %+v", messages[1])
	}
	if messages[2]["role"] != "assistant" {
		t.Fatalf("未终止 round 应追加 synthetic assistant: %+v", messages)
	}
	if strings.TrimSpace(stringValue(messages[2]["stop_reason"])) != "cancelled" {
		t.Fatalf("synthetic assistant stop_reason 不正确: %+v", messages[2])
	}
	summary, ok := messages[2]["result_summary"].(map[string]any)
	if !ok || strings.TrimSpace(stringValue(summary["subtype"])) != "interrupted" {
		t.Fatalf("未终止 round 应把 interrupted 摘要挂到 synthetic assistant 上: %+v", messages[2])
	}

	messagePage, err := sessionService.GetSessionMessagesPage(ctx, dmKey, sessionsvc.MessagePageRequest{
		Limit: 1,
	})
	if err != nil {
		t.Fatalf("分页读取普通 session 消息失败: %v", err)
	}
	if len(messagePage.Items) != 3 || messagePage.HasMore {
		t.Fatalf("普通 session 最新页结果不正确: %+v", messagePage)
	}
	if messagePage.Items[0]["message_id"] != "round_1" {
		t.Fatalf("普通 session 最新页起点不正确: %+v", messagePage.Items)
	}
	if messagePage.Items[1]["message_id"] != "msg_assistant_1" {
		t.Fatalf("普通 session 最新页终点不正确: %+v", messagePage.Items)
	}
	if messagePage.Items[2]["message_id"] != "assistant_interrupt_round_1" {
		t.Fatalf("普通 session synthetic assistant 不正确: %+v", messagePage.Items)
	}

	roomMessages, err := sessionService.GetSessionMessages(ctx, protocol.BuildRoomSharedSessionKey(dmContext.Conversation.ID))
	if err != nil {
		t.Fatalf("读取 Room 共享流失败: %v", err)
	}
	if len(roomMessages) != 2 {
		t.Fatalf("Room 共享消息数量不正确: got=%d want=2 messages=%+v", len(roomMessages), roomMessages)
	}
	if _, exists := roomMessages[0]["stream_status"]; exists {
		t.Fatalf("Room assistant 历史回放不应补写 stream_status: %+v", roomMessages[0])
	}
	roomSummary, ok := roomMessages[1]["result_summary"].(map[string]any)
	if !ok || strings.TrimSpace(stringValue(roomSummary["subtype"])) != "interrupted" {
		t.Fatalf("Room 未终止 round 应把 interrupted 摘要挂到 synthetic assistant 上: %+v", roomMessages)
	}

	roomMessagePage, err := sessionService.GetSessionMessagesPage(
		ctx,
		protocol.BuildRoomSharedSessionKey(dmContext.Conversation.ID),
		sessionsvc.MessagePageRequest{Limit: 1},
	)
	if err != nil {
		t.Fatalf("分页读取 Room 共享流失败: %v", err)
	}
	if len(roomMessagePage.Items) != 2 || roomMessagePage.HasMore {
		t.Fatalf("Room 最新页结果不正确: %+v", roomMessagePage)
	}
	if roomMessagePage.Items[0]["role"] != "assistant" {
		t.Fatalf("Room 最新页应返回 assistant 聚合结果: %+v", roomMessagePage.Items)
	}
	if roomMessagePage.Items[1]["role"] != "assistant" {
		t.Fatalf("Room synthetic assistant 应保留在同一轮分页结果里: %+v", roomMessagePage.Items)
	}

	updatedTitle := "Launcher 重命名"
	updated, err := sessionService.UpdateSession(ctx, dmKey, sessionsvc.UpdateRequest{Title: &updatedTitle})
	if err != nil {
		t.Fatalf("更新 session 失败: %v", err)
	}
	if updated.Title != updatedTitle {
		t.Fatalf("更新标题失败: got=%s want=%s", updated.Title, updatedTitle)
	}

	if err = sessionService.DeleteSession(ctx, dmKey); err != nil {
		t.Fatalf("删除 session 失败: %v", err)
	}
	if _, err = sessionService.GetSession(ctx, dmKey); err == nil {
		t.Fatal("删除后不应还能读取到 session")
	}
	if _, err = os.Stat(sessionTranscriptFilePath(agentA.WorkspacePath, dmSessionID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("删除 session 后 transcript 仍残留: %v", err)
	}
}

func TestSessionServiceGetSessionMessagesSkipsActiveRoundMaterialization(t *testing.T) {
	cfg := newSessionTestConfig(t)
	migrateSessionSQLite(t, cfg.DatabaseURL)

	agentService, db, err := bootstrap.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	sessionService := bootstrap.NewSessionServiceWithDB(cfg, db, agentService)
	runtimeManager := runtimectx.NewManager()
	sessionService.SetRuntimeManager(runtimeManager)

	ctx := context.Background()
	agentA, err := agentService.CreateAgent(ctx, agent2.CreateRequest{Name: "活跃轮次助手"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}
	dmKey := protocol.BuildAgentSessionKey(agentA.AgentID, "ws", "dm", "active-"+agentA.AgentID, "")
	sessionValue, err := sessionService.CreateSession(ctx, sessionsvc.CreateRequest{SessionKey: dmKey})
	if err != nil {
		t.Fatalf("创建 session 失败: %v", err)
	}
	dmSessionID := bindTranscriptSessionID(t, cfg, agentA.WorkspacePath, sessionValue)
	seedWorkspaceSessionArtifacts(t, cfg, agentA.AgentID, agentA.WorkspacePath, dmKey, dmSessionID)
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

func TestSessionServiceReadsTranscriptHistoryWithRoundMarkers(t *testing.T) {
	cfg := newSessionTestConfig(t)
	migrateSessionSQLite(t, cfg.DatabaseURL)

	agentService, db, err := bootstrap.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	sessionService := bootstrap.NewSessionServiceWithDB(cfg, db, agentService)

	ctx := context.Background()
	agentA, err := agentService.CreateAgent(ctx, agent2.CreateRequest{Name: "Transcript 助手"})
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
	store := workspace2.NewSessionFileStore(cfg.WorkspacePath)
	if _, err := store.UpsertSession(agentA.WorkspacePath, *created); err != nil {
		t.Fatalf("回写 session_id 失败: %v", err)
	}

	history := workspace2.NewAgentHistoryStore(cfg.WorkspacePath)
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

	agentService, db, err := bootstrap.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := bootstrap.NewRoomServiceWithDB(cfg, db, agentService)
	sessionService := bootstrap.NewSessionServiceWithDB(cfg, db, agentService)

	ctx := context.Background()
	agentA, err := agentService.CreateAgent(ctx, agent2.CreateRequest{Name: "Room Topic Transcript 助手"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}

	dmContext, err := roomService.EnsureDirectRoom(ctx, agentA.AgentID)
	if err != nil {
		t.Fatalf("创建直聊 room 失败: %v", err)
	}
	topicContext, err := roomService.CreateConversation(ctx, dmContext.Room.ID, roomsvc.CreateConversationRequest{
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
	store := workspace2.NewSessionFileStore(cfg.WorkspacePath)
	if _, err := store.UpsertSession(agentA.WorkspacePath, sessionsvc.Session{
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

	history := workspace2.NewAgentHistoryStore(cfg.WorkspacePath)
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

func seedWorkspaceSessionArtifacts(
	t *testing.T,
	cfg config.Config,
	agentID string,
	workspacePath string,
	sessionKey string,
	sessionID string,
) {
	t.Helper()

	history := workspace2.NewAgentHistoryStore(cfg.WorkspacePath)
	if err := history.AppendRoundMarker(workspacePath, sessionKey, "round_1", "你好", 1000); err != nil {
		t.Fatalf("写入 round marker 失败: %v", err)
	}
	writeSessionTranscriptFixture(t, workspacePath, sessionID, []map[string]any{
		{
			"type":      "user",
			"uuid":      "transcript-user-1",
			"sessionId": sessionID,
			"timestamp": "2026-04-19T10:00:00Z",
			"message": map[string]any{
				"role":    "user",
				"content": "你好",
			},
		},
		{
			"type":       "assistant",
			"uuid":       "msg_assistant_1",
			"sessionId":  sessionID,
			"parentUuid": "transcript-user-1",
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "text", "text": "最终回复"},
				},
			},
		},
	})
}

func seedRoomConversationMessages(t *testing.T, cfg config.Config, conversationID string) {
	t.Helper()

	roomHistory := workspace2.NewRoomHistoryStore(cfg.WorkspacePath)
	if err := roomHistory.AppendInlineMessage(conversationID, map[string]any{
		"message_id":      "room_msg_1",
		"session_key":     protocol.BuildRoomSharedSessionKey(conversationID),
		"conversation_id": conversationID,
		"agent_id":        "agent_room",
		"round_id":        "room_round_1",
		"role":            "assistant",
		"content":         "Room 共享消息",
		"timestamp":       100,
	}); err != nil {
		t.Fatalf("写入 room 共享历史失败: %v", err)
	}
}

func bindTranscriptSessionID(
	t *testing.T,
	cfg config.Config,
	workspacePath string,
	item *sessionsvc.Session,
) string {
	t.Helper()

	sessionID := "550e8400-e29b-41d4-a716-446655440000"
	item.SessionID = &sessionID
	store := workspace2.NewSessionFileStore(cfg.WorkspacePath)
	if _, err := store.UpsertSession(workspacePath, *item); err != nil {
		t.Fatalf("回写 session_id 失败: %v", err)
	}
	return sessionID
}

func newSessionTestConfig(t *testing.T) config.Config {
	t.Helper()

	root := t.TempDir()
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(root, ".nexus"))
	return config.Config{
		Host:           "127.0.0.1",
		Port:           18012,
		ProjectName:    "nexus-session-test",
		APIPrefix:      "/agent/v1",
		WebSocketPath:  "/agent/v1/chat/ws",
		DefaultAgentID: "nexus",
		WorkspacePath:  filepath.Join(root, "workspace"),
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(root, "nexus.db"),
	}
}

var sessionTranscriptSanitizePattern = regexp.MustCompile(`[^a-zA-Z0-9]`)

func writeSessionTranscriptFixture(t *testing.T, workspacePath string, sessionID string, rows []map[string]any) {
	t.Helper()
	projectDir := sessionTranscriptProjectDir(workspacePath)
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

func sessionTranscriptProjectDir(workspacePath string) string {
	return filepath.Join(
		os.Getenv("NEXUS_CONFIG_DIR"),
		"projects",
		sanitizeSessionTranscriptPath(canonicalizeSessionTranscriptPath(workspacePath)),
	)
}

func sessionTranscriptFilePath(workspacePath string, sessionID string) string {
	return filepath.Join(sessionTranscriptProjectDir(workspacePath), strings.TrimSpace(sessionID)+".jsonl")
}

func canonicalizeSessionTranscriptPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	if absolutePath, err := filepath.Abs(path); err == nil {
		path = absolutePath
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	return path
}

func sanitizeSessionTranscriptPath(path string) string {
	const maxLength = 200
	sanitized := sessionTranscriptSanitizePattern.ReplaceAllString(path, "-")
	if len(sanitized) <= maxLength {
		return sanitized
	}
	return sanitized[:maxLength] + "-" + sessionTranscriptHash(path)
}

func sessionTranscriptHash(value string) string {
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

func stringValue(value any) string {
	typed, _ := value.(string)
	return typed
}

func stringPointer(value string) *string {
	copyValue := value
	return &copyValue
}

func migrateSessionSQLite(t *testing.T, databaseURL string) {
	t.Helper()

	db, err := sql.Open("sqlite3", databaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer db.Close()

	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("设置 goose 方言失败: %v", err)
	}
	if err = goose.Up(db, sessionMigrationDir(t)); err != nil {
		t.Fatalf("执行 migration 失败: %v", err)
	}
}

func sessionMigrationDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "db", "migrations", "sqlite")
}
