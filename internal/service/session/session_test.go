// session_test.go 覆盖会话列表、历史、恢复、子代理任务控制与测试夹具。
package session_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/app"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	automationsvc "github.com/nexus-research-lab/nexus/internal/service/automation"
	titlegensvc "github.com/nexus-research-lab/nexus/internal/service/conversation/titlegen"
	sessionsvc "github.com/nexus-research-lab/nexus/internal/service/session"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
	_ "modernc.org/sqlite"
)

func seedWorkspaceSessionArtifacts(
	t *testing.T,
	cfg config.Config,
	workspacePath string,
	sessionKey string,
	sessionID string,
) {
	t.Helper()

	history := workspacestore.NewAgentHistoryStore(cfg.WorkspacePath)
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

	roomHistory := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath)
	if err := roomHistory.AppendInlineMessage("", conversationID, map[string]any{
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
	item *protocol.Session,
) string {
	t.Helper()

	sessionID := "550e8400-e29b-41d4-a716-446655440000"
	item.SessionID = &sessionID
	store := workspacestore.NewSessionFileStore(cfg.WorkspacePath)
	if _, err := store.UpsertSession(workspacePath, *item); err != nil {
		t.Fatalf("回写 session_id 失败: %v", err)
	}
	return sessionID
}

func findSessionByKey(items []protocol.Session, sessionKey string) *protocol.Session {
	for index := range items {
		if items[index].SessionKey == sessionKey {
			return &items[index]
		}
	}
	return nil
}

type staticTitleProvider struct {
	baseURL string
}

func (p staticTitleProvider) ResolveLLMConfig(context.Context, string, string) (*clientopts.RuntimeConfig, error) {
	return &clientopts.RuntimeConfig{
		Provider:  "test-title-provider",
		AuthToken: "token",
		BaseURL:   p.baseURL,
		Model:     "title-model",
	}, nil
}

func newSessionTestConfig(t *testing.T) config.Config {
	t.Helper()

	root := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", filepath.Join(root, ".nexus"))
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(root, ".nexus"))
	return config.Config{
		Host:           "127.0.0.1",
		Port:           18012,
		ProjectName:    "nexus-session-test",
		APIPrefix:      "/nexus/v1",
		WebSocketPath:  "/nexus/v1/chat/ws",
		DefaultAgentID: "nexus",
		WorkspacePath:  filepath.Join(root, "workspace"),
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(root, "nexus.db"),
	}
}

func newSessionTestAgentService(t *testing.T, cfg config.Config) (*agentsvc.Service, *sql.DB) {
	t.Helper()

	agentService, db, err := app.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("关闭测试数据库失败: %v", err)
		}
	})
	return agentService, db
}

func newSessionTestCoreServices(
	t *testing.T,
	cfg config.Config,
) (*app.CoreServices, *sql.DB) {
	t.Helper()
	db, err := app.OpenDB(cfg)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("关闭测试数据库失败: %v", err)
		}
	})
	return app.NewCoreServicesWithDB(cfg, db), db
}

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
	defer func() { _ = file.Close() }()

	encoder := json.NewEncoder(file)
	for _, row := range rows {
		if err := encoder.Encode(row); err != nil {
			t.Fatalf("写入 transcript fixture 失败: %v", err)
		}
	}
}

func sessionTranscriptProjectDir(workspacePath string) string {
	return filepath.Join(
		workspacestore.TranscriptProjectsDirForWorkspace(workspacePath),
		workspacestore.TranscriptProjectDirectoryName(workspacePath),
	)
}

func sessionTranscriptFilePath(workspacePath string, sessionID string) string {
	return filepath.Join(sessionTranscriptProjectDir(workspacePath), strings.TrimSpace(sessionID)+".jsonl")
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
	handlertest.MigrateSQLiteFromDir(t, databaseURL, sessionMigrationDir(t))
}

func sessionMigrationDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations", "sqlite")
}

func TestSessionServiceLifecycle(t *testing.T) {
	cfg := newSessionTestConfig(t)
	migrateSessionSQLite(t, cfg.DatabaseURL)

	agentService, db := newSessionTestAgentService(t, cfg)
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	sessionService := app.NewSessionServiceWithDB(cfg, db, agentService)
	runtimeManager := runtimectx.NewManager()
	sessionService.SetRuntimeManager(runtimeManager)

	ctx := context.Background()
	agentA, err := agentService.CreateAgent(ctx, protocol.CreateRequest{Name: "测试会话助手"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}

	dmKey := protocol.BuildAgentSessionKey(agentA.AgentID, "ws", "dm", "launcher-app-"+agentA.AgentID, "")
	runtimeManager.MarkSubagentHistory(dmKey)
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
	seedWorkspaceSessionArtifacts(t, cfg, agentA.WorkspacePath, dmKey, dmSessionID)
	previousDMSessionID := "650e8400-e29b-41d4-a716-446655440000"
	created.TranscriptSessionIDs = []string{previousDMSessionID}
	if _, err = workspacestore.NewSessionFileStore(cfg.WorkspacePath).UpsertSession(
		agentA.WorkspacePath,
		*created,
	); err != nil {
		t.Fatalf("回写 transcript lineage 失败: %v", err)
	}
	writeSessionTranscriptFixture(t, agentA.WorkspacePath, previousDMSessionID, []map[string]any{{
		"type":      "assistant",
		"uuid":      "previous-transcript",
		"sessionId": previousDMSessionID,
	}})

	dmContext, err := roomService.EnsureDirectRoom(ctx, agentA.AgentID)
	if err != nil {
		t.Fatalf("创建直聊 room 失败: %v", err)
	}
	seedRoomConversationMessages(t, cfg, dmContext.Conversation.ID)
	dmSessionKey := protocol.BuildRoomAgentSessionKey(
		dmContext.Conversation.ID,
		agentA.AgentID,
		protocol.RoomTypeDM,
	)
	dmSessionStore := workspacestore.NewSessionFileStore(cfg.WorkspacePath)
	roomBacked, _, err := dmSessionStore.FindSession(
		[]string{agentA.WorkspacePath},
		dmSessionKey,
	)
	if err != nil {
		t.Fatalf("读取 Room-backed workspace session 失败: %v", err)
	}
	if roomBacked == nil {
		roomBacked = &protocol.Session{
			SessionKey:     dmSessionKey,
			AgentID:        agentA.AgentID,
			ConversationID: &dmContext.Conversation.ID,
			CreatedAt:      time.Now().UTC(),
			Options:        map[string]any{},
		}
	}
	roomBacked.MessageCount = 11
	roomBacked.LastActivity = time.Now().UTC().Add(time.Minute)
	if _, err = dmSessionStore.UpsertSession(agentA.WorkspacePath, *roomBacked); err != nil {
		t.Fatalf("写入 Room-backed workspace 进度失败: %v", err)
	}

	sessions, err := sessionService.ListSessions(ctx)
	if err != nil {
		t.Fatalf("列出 sessions 失败: %v", err)
	}
	if len(sessions) < 2 {
		t.Fatalf("session 列表未合并 room 视图: got=%d", len(sessions))
	}
	listedRoomSession := findSessionByKey(sessions, dmSessionKey)
	if listedRoomSession == nil || listedRoomSession.MessageCount != 11 {
		t.Fatalf("全部 session 列表未保留 workspace 消息进度: %+v", listedRoomSession)
	}

	agentSessions, err := sessionService.ListAgentSessions(ctx, agentA.AgentID)
	if err != nil {
		t.Fatalf("读取 agent sessions 失败: %v", err)
	}
	if len(agentSessions) < 2 {
		t.Fatalf("agent sessions 数量不正确: got=%d", len(agentSessions))
	}
	listedRoomSession = findSessionByKey(agentSessions, dmSessionKey)
	if listedRoomSession == nil || listedRoomSession.MessageCount != 11 {
		t.Fatalf("Agent session 列表未保留 workspace 消息进度: %+v", listedRoomSession)
	}
	loadedRoomSession, err := sessionService.GetSession(ctx, dmSessionKey)
	if err != nil || loadedRoomSession.MessageCount != 11 {
		t.Fatalf("单 session 读取未保留 workspace 消息进度: item=%+v err=%v", loadedRoomSession, err)
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
	if messagePage.Items[0]["message_id"] != "msg_user_round_1" {
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

	canceledPageCtx, cancelPage := context.WithCancel(ctx)
	cancelPage()
	if _, err = sessionService.GetSessionMessagesPage(
		canceledPageCtx,
		dmKey,
		sessionsvc.MessagePageRequest{Limit: 1},
	); !errors.Is(err, context.Canceled) {
		t.Fatalf("已取消的 DM 分页请求应在进入存储前退出: %v", err)
	}
	if _, err = sessionService.GetSessionMessagesPage(
		canceledPageCtx,
		protocol.BuildRoomSharedSessionKey(dmContext.Conversation.ID),
		sessionsvc.MessagePageRequest{Limit: 1},
	); !errors.Is(err, context.Canceled) {
		t.Fatalf("已取消的 Room 分页请求应在进入存储前退出: %v", err)
	}

	dmStorageCtx := newCancelAfterEntryCheckContext(ctx)
	if _, err = sessionService.GetSessionMessagesPage(
		dmStorageCtx,
		dmKey,
		sessionsvc.MessagePageRequest{Limit: 1},
	); !errors.Is(err, context.Canceled) {
		t.Fatalf("DM 存储分页应观察原请求取消信号: %v", err)
	}
	roomStorageCtx := newCancelAfterEntryCheckContext(ctx)
	if _, err = sessionService.GetSessionMessagesPage(
		roomStorageCtx,
		protocol.BuildRoomSharedSessionKey(dmContext.Conversation.ID),
		sessionsvc.MessagePageRequest{Limit: 1},
	); !errors.Is(err, context.Canceled) {
		t.Fatalf("Room 存储分页应观察原请求取消信号: %v", err)
	}

	updatedTitle := "Launcher 重命名"
	updated, err := sessionService.UpdateSession(ctx, dmKey, sessionsvc.UpdateRequest{Title: &updatedTitle})
	if err != nil {
		t.Fatalf("更新 session 失败: %v", err)
	}
	if updated.Title != updatedTitle {
		t.Fatalf("更新标题失败: got=%s want=%s", updated.Title, updatedTitle)
	}
	if _, err = db.Exec(`
INSERT INTO automation_delivery_routes (
    route_id, agent_id, session_key, mode, enabled
) VALUES ('session-route', ?, ?, 'last', TRUE)`, agentA.AgentID, dmKey); err != nil {
		t.Fatalf("准备 Session route 失败: %v", err)
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
	if _, err = os.Stat(sessionTranscriptFilePath(agentA.WorkspacePath, previousDMSessionID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("删除 session 后历史 transcript 仍残留: %v", err)
	}
	if runtimeManager.HasSubagentHistory(dmKey) {
		t.Fatal("删除 session 后 runtime state 仍残留")
	}
	var routeCount int
	if err = db.QueryRow(
		"SELECT COUNT(*) FROM automation_delivery_routes WHERE session_key = ?",
		dmKey,
	).Scan(&routeCount); err != nil {
		t.Fatalf("读取 Session route 失败: %v", err)
	}
	if routeCount != 0 {
		t.Fatalf("删除 Session 后仍残留 delivery route: %d", routeCount)
	}
}

type cancelAfterEntryCheckContext struct {
	context.Context
	cancel context.CancelFunc
	checks atomic.Int32
}

func newCancelAfterEntryCheckContext(parent context.Context) *cancelAfterEntryCheckContext {
	ctx, cancel := context.WithCancel(parent)
	return &cancelAfterEntryCheckContext{Context: ctx, cancel: cancel}
}

// Err 让服务入口的第一次检查通过并随即取消，之后进入存储的任何读取
// （无论是 Err 还是 QueryContext 监听的 Done）都必须观察到取消；
// 不依赖存储层恰好再调用一次 Err（读模型已打开时不会再调用）。
func (c *cancelAfterEntryCheckContext) Err() error {
	if c.checks.Add(1) == 1 {
		c.cancel()
		return nil
	}
	return c.Context.Err()
}

func TestSessionLocalDirectoriesRequireDesktopAndPersist(t *testing.T) {
	cfg := newSessionTestConfig(t)
	cfg.Debug = true
	migrateSessionSQLite(t, cfg.DatabaseURL)

	agentService, db := newSessionTestAgentService(t, cfg)
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	sessionService := app.NewSessionServiceWithDB(cfg, db, agentService)
	ctx := context.Background()
	agentValue, err := agentService.CreateAgent(ctx, protocol.CreateRequest{
		Name: "本机目录助手",
	})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}
	sessionKey := protocol.BuildAgentSessionKey(
		agentValue.AgentID,
		"ws",
		"dm",
		"local-directories",
		"",
	)
	if _, err = sessionService.CreateSession(ctx, sessionsvc.CreateRequest{
		SessionKey: sessionKey,
	}); err != nil {
		t.Fatalf("创建 Session 失败: %v", err)
	}
	if _, err = sessionService.GetLocalDirectories(ctx, sessionKey); !errors.Is(
		err,
		sessionsvc.ErrLocalDirectoriesUnavailable,
	) {
		t.Fatalf("普通 Web 读取错误 = %v", err)
	}

	desktopCtx := authctx.WithInteractiveHumanEvidence(
		ctx,
		"desktop_session_token",
	)
	directory := t.TempDir()
	secondDirectory := t.TempDir()
	updated, err := sessionService.UpdateLocalDirectories(
		desktopCtx,
		sessionKey,
		protocol.SessionLocalDirectories{
			Directories: []string{directory, directory},
		},
	)
	if err != nil {
		t.Fatalf("更新本机目录失败: %v", err)
	}
	if len(updated.Directories) != 1 || updated.Directories[0] != directory {
		t.Fatalf("本机目录未去重: %+v", updated)
	}
	loaded, err := sessionService.GetLocalDirectories(desktopCtx, sessionKey)
	if err != nil || len(loaded.Directories) != 1 || loaded.Directories[0] != directory {
		t.Fatalf("读取持久化本机目录 = %+v, err=%v", loaded, err)
	}
	if _, err = sessionService.UpdateLocalDirectories(
		desktopCtx,
		sessionKey,
		protocol.SessionLocalDirectories{Directories: []string{"relative"}},
	); !errors.Is(err, sessionsvc.ErrInvalidLocalDirectories) {
		t.Fatalf("相对目录错误 = %v", err)
	}

	directRoom, err := roomService.EnsureDirectRoom(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("创建 Room-backed DM 失败: %v", err)
	}
	roomSessionKey := protocol.BuildRoomAgentSessionKey(
		directRoom.Conversation.ID,
		agentValue.AgentID,
		protocol.RoomTypeDM,
	)
	if _, err = sessionService.UpdateLocalDirectories(
		desktopCtx,
		roomSessionKey,
		protocol.SessionLocalDirectories{
			Directories: []string{directory, secondDirectory},
		},
	); err != nil {
		t.Fatalf("更新 Room-backed DM 本机目录失败: %v", err)
	}
	loaded, err = sessionService.GetLocalDirectories(desktopCtx, roomSessionKey)
	if err != nil || len(loaded.Directories) != 2 ||
		loaded.Directories[0] != directory ||
		loaded.Directories[1] != secondDirectory {
		t.Fatalf("读取 Room-backed DM 本机目录 = %+v, err=%v", loaded, err)
	}

	peerAgent, err := agentService.CreateAgent(ctx, protocol.CreateRequest{
		Name: "本机目录协作者",
	})
	if err != nil {
		t.Fatalf("创建协作 Agent 失败: %v", err)
	}
	groupContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentValue.AgentID, peerAgent.AgentID},
		Name:     "本机目录群聊",
	})
	if err != nil {
		t.Fatalf("创建群聊失败: %v", err)
	}
	groupSessionKey := protocol.BuildRoomAgentSessionKey(
		groupContext.Conversation.ID,
		agentValue.AgentID,
		protocol.RoomTypeGroup,
	)
	peerGroupSessionKey := protocol.BuildRoomAgentSessionKey(
		groupContext.Conversation.ID,
		peerAgent.AgentID,
		protocol.RoomTypeGroup,
	)
	if _, err = sessionService.UpdateLocalDirectories(
		desktopCtx,
		groupSessionKey,
		protocol.SessionLocalDirectories{Directories: []string{directory}},
	); err != nil {
		t.Fatalf("更新群聊本机目录失败: %v", err)
	}
	loaded, err = sessionService.GetLocalDirectories(
		desktopCtx,
		peerGroupSessionKey,
	)
	if err != nil || len(loaded.Directories) != 1 || loaded.Directories[0] != directory {
		t.Fatalf("群聊本机目录未同步到协作 Agent: %+v, err=%v", loaded, err)
	}
}

func TestTitleGenerationUpdatesExternalIMWorkspaceSession(t *testing.T) {
	cfg := newSessionTestConfig(t)
	migrateSessionSQLite(t, cfg.DatabaseURL)

	agentService, db := newSessionTestAgentService(t, cfg)
	sessionService := app.NewSessionServiceWithDB(cfg, db, agentService)

	agentValue, err := agentService.CreateAgent(context.Background(), protocol.CreateRequest{Name: "微信助手"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}
	sessionKey := protocol.BuildAgentSessionKey(
		agentValue.AgentID,
		protocol.SessionChannelWeixinPersonalSegment,
		protocol.RoomTypeDM,
		"wx-user-1",
		"",
	)
	now := time.Now().UTC()
	store := workspacestore.NewSessionFileStore(cfg.WorkspacePath)
	if _, err = store.UpsertSession(agentValue.WorkspacePath, protocol.Session{
		SessionKey:   sessionKey,
		AgentID:      agentValue.AgentID,
		ChannelType:  protocol.SessionChannelWeixinPersonal,
		ChatType:     protocol.RoomTypeDM,
		Status:       "closed",
		CreatedAt:    now.Add(-time.Hour),
		LastActivity: now.Add(-time.Minute),
		Title:        "New Chat",
		MessageCount: 74,
		Options:      map[string]any{},
	}); err != nil {
		t.Fatalf("写入外部 IM session 失败: %v", err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": "午餐建议"},
			},
		})
	}))
	defer server.Close()

	titleService := titlegensvc.NewService(
		staticTitleProvider{baseURL: server.URL},
		sessionService,
		nil,
		nil,
	)
	titleService.Schedule(context.Background(), titlegensvc.Request{
		OwnerUserID:              "__system__",
		SessionKey:               sessionKey,
		Content:                  "中午吃点啥好你觉得",
		SessionTitle:             "New Chat",
		SessionMessageCount:      74,
		ConversationMessageCount: -1,
	})

	deadline := time.Now().Add(2 * time.Second)
	for {
		updated, err := sessionService.GetSession(context.Background(), sessionKey)
		if err != nil {
			t.Fatalf("读取更新后的 IM session 失败: %v", err)
		}
		if updated.Title == "午餐建议" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("外部 IM session 标题未写回: %+v", updated)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestSessionServiceReadsRoomTopicHistoryFromWorkspaceMetaSessionID(t *testing.T) {
	cfg := newSessionTestConfig(t)
	migrateSessionSQLite(t, cfg.DatabaseURL)

	agentService, db := newSessionTestAgentService(t, cfg)
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	sessionService := app.NewSessionServiceWithDB(cfg, db, agentService)

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

func TestSessionDeletionRecoveryInvalidatesBoundAutomationTask(t *testing.T) {
	cfg := newSessionTestConfig(t)
	migrateSessionSQLite(t, cfg.DatabaseURL)
	core, db := newSessionTestCoreServices(t, cfg)
	core.Session.SetRuntimeManager(runtimectx.NewManager())
	automationService := automationsvc.NewService(cfg, db, core.Agent, nil, nil, nil, nil, nil)
	core.Deletion.SetTaskCleaner(automationService)

	ctx := context.Background()
	agentValue, err := core.Agent.CreateAgent(ctx, protocol.CreateRequest{Name: "删除恢复任务助手"})
	if err != nil {
		t.Fatal(err)
	}
	sessionKey := protocol.BuildAgentSessionKey(
		agentValue.AgentID,
		protocol.SessionChannelWebSocket,
		"dm",
		"crash-task-recovery",
		"",
	)
	created, err := core.Session.CreateSession(ctx, sessionsvc.CreateRequest{
		SessionKey: sessionKey,
		Title:      "crash task recovery",
	})
	if err != nil {
		t.Fatal(err)
	}
	interval := 300
	task, err := automationService.CreateTask(ctx, automationdomain.CreateJobInput{
		Name:        "崩溃恢复绑定任务",
		AgentID:     agentValue.AgentID,
		Schedule:    automationdomain.Schedule{Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: &interval, Timezone: "UTC"},
		Instruction: "run",
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetBound,
			BoundSessionKey: sessionKey,
		},
		Delivery: automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Source:   automationdomain.Source{Kind: automationdomain.SourceKindUserPage},
		Enabled:  true,
	})
	if err != nil {
		t.Fatal(err)
	}
	files := workspacestore.NewSessionFileStore(cfg.WorkspacePath).ForOwner(authctx.SystemUserID)
	if _, err = files.BeginSessionDeletion(
		agentValue.WorkspacePath,
		sessionKey,
		created.ConfigurationVersion,
		"",
	); err != nil {
		t.Fatal(err)
	}

	reconciled, err := core.Session.ReconcilePendingDeletions(ctx)
	if err != nil || reconciled != 1 {
		t.Fatalf("ReconcilePendingDeletions() reconciled=%d err=%v", reconciled, err)
	}
	kept, err := automationService.GetTask(ctx, task.JobID)
	if err != nil || kept == nil || kept.Enabled ||
		kept.SessionBindingState != automationdomain.TaskSessionBindingStateRebindRequired {
		t.Fatalf("恢复删除后任务未停用待重绑: task=%+v err=%v", kept, err)
	}
}

type subagentControlCall struct {
	taskID  string
	message string
}

type subagentControlClient struct {
	mu       sync.Mutex
	stopped  []string
	messages []subagentControlCall
}

func (c *subagentControlClient) Connect(context.Context) error { return nil }

func (c *subagentControlClient) Query(context.Context, string) error { return nil }

func (c *subagentControlClient) ReceiveMessages(context.Context) <-chan sdkprotocol.ReceivedMessage {
	messages := make(chan sdkprotocol.ReceivedMessage)
	close(messages)
	return messages
}

func (c *subagentControlClient) Interrupt(context.Context) error { return nil }

func (c *subagentControlClient) StopTask(_ context.Context, taskID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stopped = append(c.stopped, taskID)
	return nil
}

func (c *subagentControlClient) SendTaskMessage(_ context.Context, taskID string, message string, _ string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, subagentControlCall{taskID: taskID, message: message})
	return nil
}

func (c *subagentControlClient) RemoveMessages(context.Context, []string) error { return nil }

func (c *subagentControlClient) SetPermissionMode(context.Context, sdkpermission.Mode) error {
	return nil
}

func (c *subagentControlClient) Retire() {}

func (c *subagentControlClient) Disconnect(context.Context) error { return nil }

func (c *subagentControlClient) Reconfigure(context.Context, agentclient.Options) error { return nil }

func (c *subagentControlClient) SessionID() string { return "" }

type subagentControlFactory struct {
	client runtimectx.Client
}

func (f subagentControlFactory) New(agentclient.Options) runtimectx.Client { return f.client }

func TestSessionServiceRoutesCompletedNXSTaskControlsToRoomHostRuntime(t *testing.T) {
	cfg := newSessionTestConfig(t)
	migrateSessionSQLite(t, cfg.DatabaseURL)
	agentService, db := newSessionTestAgentService(t, cfg)
	sessionService := app.NewSessionServiceWithDB(cfg, db, agentService)
	runtimeManager := runtimectx.NewManager()
	sessionService.SetRuntimeManager(runtimeManager)

	conversationID := "conversation-nxs-task-control"
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(conversationID)
	runtimeSessionKey := protocol.BuildRoomAgentSessionKey(conversationID, "host-agent", protocol.RoomTypeGroup)
	client := &subagentControlClient{}
	_, err := runtimeManager.GetOrCreateWithFactory(
		context.Background(),
		runtimeSessionKey,
		agentclient.Options{Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeNXS}},
		subagentControlFactory{client: client},
	)
	if err != nil {
		t.Fatalf("创建 nxs slot runtime 失败: %v", err)
	}

	history := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath)
	if err = history.AppendInlineMessage("", conversationID, protocol.Message{
		"message_id":  "task-notification-1",
		"session_key": sharedSessionKey,
		"agent_id":    "host-agent",
		"round_id":    "round-1",
		"role":        "system",
		"timestamp":   int64(1000),
		"metadata": map[string]any{
			"subtype":          "task_notification",
			"task_id":          "task-1",
			"agent_id":         "sdk-subagent-1",
			"child_session_id": "child-session-1",
			"runtime_kind":     "nxs",
			"status":           "completed",
		},
	}); err != nil {
		t.Fatalf("写入 Room task history 失败: %v", err)
	}

	list, err := sessionService.ListSubagentTasks(context.Background(), sharedSessionKey)
	if err != nil {
		t.Fatalf("ListSubagentTasks() error = %v", err)
	}
	if list.RuntimeKind != "nxs" || !list.Capabilities.SendMessage || !list.Capabilities.Resume {
		t.Fatalf("nxs list capabilities = %+v runtime=%q", list.Capabilities, list.RuntimeKind)
	}
	if len(list.Items) != 1 || list.Items[0].AgentID != "sdk-subagent-1" || list.Items[0].HostAgentID != "host-agent" {
		t.Fatalf("task identity = %+v", list.Items)
	}

	result, err := sessionService.SendSubagentTaskMessage(context.Background(), sharedSessionKey, "task-1", "继续检查")
	if err != nil {
		t.Fatalf("completed nxs task 应允许续聊: %v", err)
	}
	if result.Status != "queued" {
		t.Fatalf("SendSubagentTaskMessage() = %+v", result)
	}
	if _, err = sessionService.StopSubagentTask(context.Background(), sharedSessionKey, "task-1"); err != nil {
		t.Fatalf("StopSubagentTask() error = %v", err)
	}
	client.mu.Lock()
	defer client.mu.Unlock()
	if len(client.messages) != 1 || client.messages[0].taskID != "task-1" || client.messages[0].message != "继续检查" {
		t.Fatalf("task messages = %+v", client.messages)
	}
	if len(client.stopped) != 1 || client.stopped[0] != "task-1" {
		t.Fatalf("stopped tasks = %+v", client.stopped)
	}
}
