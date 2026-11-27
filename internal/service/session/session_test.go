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
	sessionrepo "github.com/nexus-research-lab/nexus/internal/storage/sessionrepo"
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

type runtimeSettingsPreparationRecorder struct {
	mu       sync.Mutex
	sessions []protocol.Session
}

func (r *runtimeSettingsPreparationRecorder) ScheduleRuntimeSettingsPreparation(
	_ context.Context,
	session protocol.Session,
) {
	r.mu.Lock()
	r.sessions = append(r.sessions, session)
	r.mu.Unlock()
}

func (r *runtimeSettingsPreparationRecorder) snapshot() []protocol.Session {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]protocol.Session(nil), r.sessions...)
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

	dmStorageCtx := newCancelOnSecondErrContext(ctx)
	if _, err = sessionService.GetSessionMessagesPage(
		dmStorageCtx,
		dmKey,
		sessionsvc.MessagePageRequest{Limit: 1},
	); !errors.Is(err, context.Canceled) {
		t.Fatalf("DM 存储分页应观察原请求取消信号: %v", err)
	}
	roomStorageCtx := newCancelOnSecondErrContext(ctx)
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

type cancelOnSecondErrContext struct {
	context.Context
	cancel context.CancelFunc
	checks atomic.Int32
}

func newCancelOnSecondErrContext(parent context.Context) *cancelOnSecondErrContext {
	ctx, cancel := context.WithCancel(parent)
	return &cancelOnSecondErrContext{Context: ctx, cancel: cancel}
}

func (c *cancelOnSecondErrContext) Err() error {
	if c.checks.Add(1) == 2 {
		c.cancel()
	}
	return c.Context.Err()
}

func TestSessionRuntimeSettingsPersistWithoutChangingAgentDefaults(t *testing.T) {
	cfg := newSessionTestConfig(t)
	migrateSessionSQLite(t, cfg.DatabaseURL)

	agentService, db := newSessionTestAgentService(t, cfg)
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	sessionService := app.NewSessionServiceWithDB(cfg, db, agentService)
	ctx := context.Background()
	agentValue, err := agentService.CreateAgent(ctx, protocol.CreateRequest{
		Name: "Session 设置助手",
		Options: &protocol.Options{
			Provider:       "agent-provider",
			Model:          "agent-model",
			PermissionMode: "default",
		},
	})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}

	dmKey := protocol.BuildAgentSessionKey(
		agentValue.AgentID,
		"ws",
		"dm",
		"session-settings",
		"",
	)
	if _, err = sessionService.CreateSession(ctx, sessionsvc.CreateRequest{
		SessionKey: dmKey,
	}); err != nil {
		t.Fatalf("创建 DM Session 失败: %v", err)
	}
	want := protocol.SessionRuntimeSettings{
		Provider:       "session-provider",
		Model:          "session-model",
		PermissionMode: "auto",
	}
	if _, err = sessionService.UpdateRuntimeSettings(ctx, dmKey, want); err != nil {
		t.Fatalf("更新 DM Session 设置失败: %v", err)
	}
	if got, getErr := sessionService.GetRuntimeSettings(ctx, dmKey); getErr != nil {
		t.Fatalf("读取 DM Session 设置失败: %v", getErr)
	} else if got != want {
		t.Fatalf("DM Session 设置 = %+v, want %+v", got, want)
	}

	roomContext, err := roomService.EnsureDirectRoom(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("创建 Room Session 失败: %v", err)
	}
	roomKey := protocol.BuildRoomAgentSessionKey(
		roomContext.Conversation.ID,
		agentValue.AgentID,
		protocol.RoomTypeDM,
	)
	if _, err = sessionService.UpdateRuntimeSettings(ctx, roomKey, want); err != nil {
		t.Fatalf("更新 Room Session 设置失败: %v", err)
	}
	if got, getErr := sessionService.GetRuntimeSettings(ctx, roomKey); getErr != nil {
		t.Fatalf("读取 Room Session 设置失败: %v", getErr)
	} else if got != want {
		t.Fatalf("Room Session 设置 = %+v, want %+v", got, want)
	}

	agentPeer, err := agentService.CreateAgent(ctx, protocol.CreateRequest{
		Name: "Session 设置协作者",
		Options: &protocol.Options{
			Provider:       "peer-provider",
			Model:          "peer-default-model",
			PermissionMode: "plan",
		},
	})
	if err != nil {
		t.Fatalf("创建协作 Agent 失败: %v", err)
	}
	groupContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentValue.AgentID, agentPeer.AgentID},
		Name:     "Session 设置群聊",
	})
	if err != nil {
		t.Fatalf("创建群聊 Session 失败: %v", err)
	}
	primaryRoomKey := protocol.BuildRoomAgentSessionKey(
		groupContext.Conversation.ID,
		agentValue.AgentID,
		protocol.RoomTypeGroup,
	)
	peerRoomKey := protocol.BuildRoomAgentSessionKey(
		groupContext.Conversation.ID,
		agentPeer.AgentID,
		protocol.RoomTypeGroup,
	)
	primaryRoomSession, err := sessionService.GetSession(ctx, primaryRoomKey)
	if err != nil || primaryRoomSession.RoomSessionID == nil {
		t.Fatalf("读取群聊成员 Session 失败: session=%+v err=%v", primaryRoomSession, err)
	}
	if err = roomService.UpdateSessionRuntimeIdentity(
		ctx,
		*primaryRoomSession.RoomSessionID,
		"group-runtime-session",
		"surface-before-settings",
	); err != nil {
		t.Fatalf("预写入 Room runtime identity 失败: %v", err)
	}
	staleOptions := protocol.WithSessionRuntimeSettings(primaryRoomSession.Options, want)
	repository := sessionrepo.NewSQLRepository(cfg.DatabaseDriver, db)
	if _, err = repository.UpdateRoomConversationRuntimeSettings(
		ctx,
		*primaryRoomSession.RoomSessionID,
		staleOptions,
		want.PermissionMode,
	); err != nil {
		t.Fatalf("用旧设置快照更新 Room Session 失败: %v", err)
	}
	primaryRoomSession, err = sessionService.GetSession(ctx, primaryRoomKey)
	if err != nil {
		t.Fatalf("重读群聊成员 Session 失败: %v", err)
	}
	if primaryRoomSession.Options[protocol.OptionRuntimeToolSurfaceFingerprint] != "surface-before-settings" {
		t.Fatalf("旧设置快照不应覆盖 runtime 工具面基线: %+v", primaryRoomSession.Options)
	}
	peerSettings := protocol.SessionRuntimeSettings{
		Provider:       "peer-session-provider",
		Model:          "peer-session-model",
		PermissionMode: "plan",
	}
	if _, err = sessionService.UpdateRuntimeSettings(
		ctx,
		peerRoomKey,
		peerSettings,
	); err != nil {
		t.Fatalf("更新协作 Agent Session 设置失败: %v", err)
	}
	if _, err = sessionService.UpdateRuntimeSettings(
		ctx,
		primaryRoomKey,
		want,
	); err != nil {
		t.Fatalf("更新群聊统一权限失败: %v", err)
	}
	gotPeer, err := sessionService.GetRuntimeSettings(ctx, peerRoomKey)
	if err != nil {
		t.Fatalf("读取协作 Agent Session 设置失败: %v", err)
	}
	if gotPeer.Provider != peerSettings.Provider ||
		gotPeer.Model != peerSettings.Model ||
		gotPeer.PermissionMode != want.PermissionMode {
		t.Fatalf(
			"群聊权限应统一且模型应保持独立: got=%+v peer=%+v room=%+v",
			gotPeer,
			peerSettings,
			want,
		)
	}
	resetRoomPermission := want
	resetRoomPermission.PermissionMode = ""
	if _, err = sessionService.UpdateRuntimeSettings(
		ctx,
		primaryRoomKey,
		resetRoomPermission,
	); err != nil {
		t.Fatalf("重置群聊统一权限失败: %v", err)
	}
	gotPeer, err = sessionService.GetRuntimeSettings(ctx, peerRoomKey)
	if err != nil {
		t.Fatalf("重置后读取协作 Agent Session 设置失败: %v", err)
	}
	if gotPeer.Provider != peerSettings.Provider ||
		gotPeer.Model != peerSettings.Model ||
		gotPeer.PermissionMode != "" {
		t.Fatalf("群聊权限重置应统一清空且保持模型: %+v", gotPeer)
	}

	unchangedAgent, err := agentService.GetAgent(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("读取 Agent 失败: %v", err)
	}
	if unchangedAgent.Options.Provider != "agent-provider" ||
		unchangedAgent.Options.Model != "agent-model" ||
		unchangedAgent.Options.PermissionMode != "default" {
		t.Fatalf("Session 设置不应修改 Agent 默认值: %+v", unchangedAgent.Options)
	}
}

func TestSessionRuntimeSettingsSchedulesOnlyEffectiveConnectorChanges(t *testing.T) {
	cfg := newSessionTestConfig(t)
	migrateSessionSQLite(t, cfg.DatabaseURL)
	agentService, db := newSessionTestAgentService(t, cfg)
	sessionService := app.NewSessionServiceWithDB(cfg, db, agentService)
	recorder := &runtimeSettingsPreparationRecorder{}
	sessionService.SetRuntimeSettingsPreparationScheduler(recorder)
	ctx := context.Background()
	agentValue, err := agentService.CreateAgent(ctx, protocol.CreateRequest{
		Name: "Connector 预备助手",
		Options: &protocol.Options{
			ConnectorIDs: []string{"github"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	sessionKey := protocol.BuildAgentSessionKey(
		agentValue.AgentID,
		"ws",
		"dm",
		"connector-preparation",
		"",
	)
	if _, err = sessionService.CreateSession(ctx, sessionsvc.CreateRequest{SessionKey: sessionKey}); err != nil {
		t.Fatal(err)
	}
	if _, err = sessionService.UpdateRuntimeSettings(ctx, sessionKey, protocol.SessionRuntimeSettings{
		Provider: "provider-only-settings-change",
		Model:    "model-only-settings-change",
	}); err != nil {
		t.Fatal(err)
	}
	sameAsDefault := []string{"github"}
	if _, err = sessionService.UpdateRuntimeSettings(ctx, sessionKey, protocol.SessionRuntimeSettings{
		ConnectorIDs: &sameAsDefault,
	}); err != nil {
		t.Fatal(err)
	}
	if got := recorder.snapshot(); len(got) != 0 {
		t.Fatalf("equivalent Connector settings scheduled preparation: %+v", got)
	}

	feishu := []string{"feishu-docx"}
	if _, err = sessionService.UpdateRuntimeSettings(ctx, sessionKey, protocol.SessionRuntimeSettings{
		ConnectorIDs: &feishu,
	}); err != nil {
		t.Fatal(err)
	}
	got := recorder.snapshot()
	if len(got) != 1 {
		t.Fatalf("effective Connector change scheduled %d preparations, want 1", len(got))
	}
	if got[0].ConfigurationVersion < 2 {
		t.Fatalf("scheduled Session configuration_version=%d", got[0].ConfigurationVersion)
	}
	selected := protocol.EffectiveSessionConnectorIDs(nil, got[0].Options)
	if len(selected) != 1 || selected[0] != "feishu-docx" {
		t.Fatalf("scheduled Connector selection=%v", selected)
	}
}

func TestRoomSessionSDKIdentityCASUsesCurrentConnectorSelection(t *testing.T) {
	cfg := newSessionTestConfig(t)
	migrateSessionSQLite(t, cfg.DatabaseURL)
	agentService, db := newSessionTestAgentService(t, cfg)
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	sessionService := app.NewSessionServiceWithDB(cfg, db, agentService)
	ctx := context.Background()
	agentValue, err := agentService.CreateAgent(ctx, protocol.CreateRequest{Name: "Room Connector CAS"})
	if err != nil {
		t.Fatal(err)
	}
	roomContext, err := roomService.EnsureDirectRoom(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	sessionKey := protocol.BuildRoomAgentSessionKey(
		roomContext.Conversation.ID,
		agentValue.AgentID,
		protocol.RoomTypeDM,
	)
	feishu := []string{"feishu-docx"}
	if _, err = sessionService.UpdateRuntimeSettings(ctx, sessionKey, protocol.SessionRuntimeSettings{
		ConnectorIDs: &feishu,
	}); err != nil {
		t.Fatal(err)
	}
	roomSession, err := sessionService.GetSession(ctx, sessionKey)
	if err != nil || roomSession.RoomSessionID == nil {
		t.Fatalf("Room Session projection=%+v err=%v", roomSession, err)
	}
	staleSelection := protocol.SessionConnectorSelectionFromOptions(roomSession.Options)
	github := []string{"github"}
	if _, err = sessionService.UpdateRuntimeSettings(ctx, sessionKey, protocol.SessionRuntimeSettings{
		ConnectorIDs: &github,
	}); err != nil {
		t.Fatal(err)
	}
	repository := sessionrepo.NewSQLRepository(cfg.DatabaseDriver, db)
	committed, err := repository.UpdateRoomSessionRuntimeIdentityAtConnectorSelection(
		ctx,
		*roomSession.RoomSessionID,
		"stale-fork-session",
		"stale-surface",
		staleSelection,
	)
	if err != nil || committed {
		t.Fatalf("stale Connector selection committed=%t err=%v", committed, err)
	}
	current, err := sessionService.GetSession(ctx, sessionKey)
	if err != nil {
		t.Fatal(err)
	}
	current.Options[protocol.OptionRuntimeForkSourceSessionID] = "source-fork-session"
	current.Options[protocol.OptionRuntimeForkMessageID] = "source-fork-message"
	optionsJSON, err := json.Marshal(current.Options)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.ExecContext(ctx, "UPDATE sessions SET options_json = ? WHERE id = ?", optionsJSON, *roomSession.RoomSessionID); err != nil {
		t.Fatal(err)
	}
	currentSelection := protocol.SessionConnectorSelectionFromOptions(current.Options)
	committed, err = repository.UpdateRoomSessionRuntimeIdentityAtConnectorSelection(
		ctx,
		*roomSession.RoomSessionID,
		"current-fork-session",
		"current-surface",
		currentSelection,
	)
	if err != nil || !committed {
		t.Fatalf("current Connector selection committed=%t err=%v", committed, err)
	}
	reloaded, err := repository.GetRoomSessionByKey(
		ctx,
		authctx.OwnerUserID(ctx),
		protocol.ParseSessionKey(sessionKey),
	)
	if err != nil || reloaded == nil || reloaded.SessionID == nil ||
		*reloaded.SessionID != "current-fork-session" {
		t.Fatalf("reloaded Room Session=%+v err=%v", reloaded, err)
	}
	if _, exists := reloaded.Options[protocol.OptionRuntimeForkSourceSessionID]; exists {
		t.Fatalf("fork source 未清理: %+v", reloaded.Options)
	}
	if _, exists := reloaded.Options[protocol.OptionRuntimeForkMessageID]; exists {
		t.Fatalf("fork message 未清理: %+v", reloaded.Options)
	}
	if retained := protocol.RetainedTranscriptSessionIDsFromOptions(reloaded.Options); len(retained) != 1 || retained[0] != "source-fork-session" {
		t.Fatalf("fork source 清理所有权=%v", retained)
	}
	if reloaded.Options[protocol.OptionRuntimeToolSurfaceFingerprint] != "current-surface" {
		t.Fatalf("fork identity 未与工具面基线共同提交: %+v", reloaded.Options)
	}
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

func TestSessionServiceReconcilesStaleActiveWorkspaceMeta(t *testing.T) {
	cfg := newSessionTestConfig(t)
	migrateSessionSQLite(t, cfg.DatabaseURL)

	agentService, db := newSessionTestAgentService(t, cfg)
	sessionService := app.NewSessionServiceWithDB(cfg, db, agentService)
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

	_ = runtimeManager.StartRound(context.Background(), dmKey, "round_running", nil)
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

	agentService, db := newSessionTestAgentService(t, cfg)
	sessionService := app.NewSessionServiceWithDB(cfg, db, agentService)

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

func TestSessionDeletionRecoveryCommitsCrashInterruptedDelete(t *testing.T) {
	cfg := newSessionTestConfig(t)
	migrateSessionSQLite(t, cfg.DatabaseURL)
	agentService, db, err := app.NewAgentService(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	agentValue, err := agentService.CreateAgent(
		context.Background(),
		protocol.CreateRequest{Name: "删除恢复助手"},
	)
	if err != nil {
		t.Fatal(err)
	}

	files := workspacestore.NewSessionFileStore(cfg.WorkspacePath).
		ForOwner(authctx.SystemUserID)
	sessionKey := protocol.BuildAgentSessionKey(
		agentValue.AgentID,
		"ws",
		"dm",
		"crash-recovery",
		"",
	)
	transcriptID := "550e8400-e29b-41d4-a716-446655440001"
	now := time.Now().UTC()
	created, err := files.UpsertSession(agentValue.WorkspacePath, protocol.Session{
		SessionKey: sessionKey, AgentID: agentValue.AgentID,
		ChannelType: "websocket", ChatType: "dm", Status: "closed",
		CreatedAt: now, LastActivity: now, Title: "crash recovery",
		SessionID: &transcriptID,
	})
	if err != nil {
		t.Fatal(err)
	}
	seedWorkspaceSessionArtifacts(
		t,
		cfg,
		agentValue.WorkspacePath,
		sessionKey,
		transcriptID,
	)
	if _, err = files.BeginSessionDeletion(
		agentValue.WorkspacePath,
		sessionKey,
		created.ConfigurationVersion,
		transcriptID,
	); err != nil {
		t.Fatal(err)
	}

	recoveryService := app.NewSessionServiceWithDB(cfg, db, agentService)
	recoveryService.SetRuntimeManager(runtimectx.NewManager())
	reconciled, err := recoveryService.ReconcilePendingDeletions(context.Background())
	if err != nil || reconciled != 1 {
		t.Fatalf("ReconcilePendingDeletions() reconciled=%d err=%v", reconciled, err)
	}
	if current, _, findErr := files.FindSession(
		[]string{agentValue.WorkspacePath},
		sessionKey,
	); findErr != nil || current != nil {
		t.Fatalf("recovered session still exists: item=%+v err=%v", current, findErr)
	}
	if _, statErr := os.Stat(
		sessionTranscriptFilePath(agentValue.WorkspacePath, transcriptID),
	); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("recovered transcript still exists: %v", statErr)
	}
	pending, err := workspacestore.NewSessionFileStore(
		cfg.WorkspacePath,
	).ListPendingSessionDeletions()
	if err != nil || len(pending) != 0 {
		t.Fatalf("pending deletion records=%d err=%v", len(pending), err)
	}
	restartedRuntime := runtimectx.NewManager()
	restartedService := app.NewSessionServiceWithDB(cfg, db, agentService)
	restartedService.SetRuntimeManager(restartedRuntime)
	if reconciled, err = restartedService.ReconcilePendingDeletions(
		context.Background(),
	); err != nil || reconciled != 0 {
		t.Fatalf(
			"completed tombstone restart reconcile=%d err=%v",
			reconciled,
			err,
		)
	}
	if _, err = restartedRuntime.BeginSessionDeletion(sessionKey); !errors.Is(
		err,
		runtimectx.ErrRuntimeSessionDeleted,
	) {
		t.Fatalf("completed tombstone did not restore runtime fence after restart: %v", err)
	}
	if _, err = files.UpsertSession(agentValue.WorkspacePath, *created); !errors.Is(
		err,
		workspacestore.ErrSessionDeleted,
	) {
		t.Fatalf("recovered tombstone did not block late writer: %v", err)
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

func TestDeleteSessionArtifactsFencesMissingMeta(t *testing.T) {
	cfg := newSessionTestConfig(t)
	migrateSessionSQLite(t, cfg.DatabaseURL)
	agentService, db, err := app.NewAgentService(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	agentValue, err := agentService.CreateAgent(
		context.Background(),
		protocol.CreateRequest{Name: "Artifact 清理助手"},
	)
	if err != nil {
		t.Fatal(err)
	}
	sessionService := app.NewSessionServiceWithDB(cfg, db, agentService)
	sessionService.SetRuntimeManager(runtimectx.NewManager())
	sessionKey := protocol.BuildAgentSessionKey(
		agentValue.AgentID,
		"automation",
		"dm",
		"scheduled-task:missing-meta",
		"",
	)
	if err = sessionService.DeleteSessionArtifacts(
		context.Background(),
		authctx.SystemUserID,
		agentValue.WorkspacePath,
		sessionKey,
		"",
	); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	files := workspacestore.NewSessionFileStore(cfg.WorkspacePath).
		ForOwner(authctx.SystemUserID)
	if _, err = files.UpsertSession(agentValue.WorkspacePath, protocol.Session{
		SessionKey: sessionKey, AgentID: agentValue.AgentID,
		ChannelType: "automation", ChatType: "dm", Status: "closed",
		CreatedAt: now, LastActivity: now, Title: "late",
	}); !errors.Is(err, workspacestore.ErrSessionDeleted) {
		t.Fatalf("artifact tombstone did not block missing-meta resurrection: %v", err)
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

func TestSessionServiceRejectsCCSendBeforeRuntimeWire(t *testing.T) {
	cfg := newSessionTestConfig(t)
	migrateSessionSQLite(t, cfg.DatabaseURL)
	agentService, db := newSessionTestAgentService(t, cfg)
	sessionService := app.NewSessionServiceWithDB(cfg, db, agentService)
	runtimeManager := runtimectx.NewManager()
	sessionService.SetRuntimeManager(runtimeManager)

	conversationID := "conversation-cc-task-control"
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(conversationID)
	runtimeSessionKey := protocol.BuildRoomAgentSessionKey(conversationID, "host-agent", protocol.RoomTypeGroup)
	client := &subagentControlClient{}
	_, err := runtimeManager.GetOrCreateWithFactory(
		context.Background(),
		runtimeSessionKey,
		agentclient.Options{Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeClaude}},
		subagentControlFactory{client: client},
	)
	if err != nil {
		t.Fatalf("创建 CC slot runtime 失败: %v", err)
	}
	history := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath)
	if err = history.AppendInlineMessage("", conversationID, protocol.Message{
		"message_id":  "task-started-cc",
		"session_key": sharedSessionKey,
		"agent_id":    "host-agent",
		"round_id":    "round-1",
		"role":        "system",
		"timestamp":   int64(1000),
		"metadata": map[string]any{
			"subtype":      "task_started",
			"task_id":      "task-cc",
			"agent_id":     "sdk-subagent-cc",
			"runtime_kind": "claude",
		},
	}); err != nil {
		t.Fatalf("写入 CC task history 失败: %v", err)
	}

	_, err = sessionService.SendSubagentTaskMessage(context.Background(), sharedSessionKey, "task-cc", "继续")
	if !errors.Is(err, sessionsvc.ErrSubagentOperationUnsupported) {
		t.Fatalf("CC send error = %v, want unsupported", err)
	}
	client.mu.Lock()
	if len(client.messages) != 0 {
		t.Fatalf("CC unsupported 不应进入 wire: %+v", client.messages)
	}
	client.mu.Unlock()
	if _, err = sessionService.StopSubagentTask(context.Background(), sharedSessionKey, "task-cc"); err != nil {
		t.Fatalf("CC stop 应受支持: %v", err)
	}

	if err = runtimeManager.CloseSession(context.Background(), runtimeSessionKey); err != nil {
		t.Fatalf("关闭 CC runtime 失败: %v", err)
	}
	list, err := sessionService.ListSubagentTasks(context.Background(), sharedSessionKey)
	if err != nil {
		t.Fatalf("离线后读取 task 失败: %v", err)
	}
	if len(list.Items) != 1 || list.Items[0].RuntimeKind != "claude" || list.Items[0].Capabilities.SendMessage {
		t.Fatalf("离线 task 应从 metadata 恢复 CC 能力: %+v", list.Items)
	}
}
