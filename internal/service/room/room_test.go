// room_test.go 覆盖 Room 持久化、分支、清理、删除、成员参与、私域与 Relay 执行测试。
package room_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/app"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	"github.com/nexus-research-lab/nexus/internal/storage/roomrepo"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
	_ "modernc.org/sqlite"
)

func findConversationContext(
	contexts []protocol.ConversationContextAggregate,
	conversationID string,
) (protocol.ConversationContextAggregate, bool) {
	for _, item := range contexts {
		if item.Conversation.ID == conversationID {
			return item, true
		}
	}
	return protocol.ConversationContextAggregate{}, false
}

type fakeRoomGoalCleaner struct {
	conversationCalls [][]string
	memberCalls       []fakeRoomGoalMemberCleanup
	conversationErr   error
	memberErr         error
}

type fakeRoomGoalMemberCleanup struct {
	agentID         string
	conversationIDs []string
}

type roomSessionArtifactDeletionCall struct {
	ownerUserID      string
	workspacePath    string
	sessionKey       string
	cleanupSessionID string
}

type fakeRoomSessionArtifactDeletionCoordinator struct {
	mu       sync.Mutex
	calls    []roomSessionArtifactDeletionCall
	err      error
	deleteFn func(context.Context, roomSessionArtifactDeletionCall) error
}

func (f *fakeRoomSessionArtifactDeletionCoordinator) DeleteSessionArtifacts(
	ctx context.Context,
	ownerUserID string,
	workspacePath string,
	sessionKey string,
	cleanupSessionID string,
) error {
	call := roomSessionArtifactDeletionCall{
		ownerUserID:      ownerUserID,
		workspacePath:    workspacePath,
		sessionKey:       sessionKey,
		cleanupSessionID: cleanupSessionID,
	}
	f.mu.Lock()
	f.calls = append(f.calls, call)
	deleteFn := f.deleteFn
	err := f.err
	f.mu.Unlock()
	if deleteFn != nil {
		if deleteErr := deleteFn(ctx, call); deleteErr != nil {
			return deleteErr
		}
	}
	return err
}

func (f *fakeRoomSessionArtifactDeletionCoordinator) Calls() []roomSessionArtifactDeletionCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	result := make([]roomSessionArtifactDeletionCall, len(f.calls))
	copy(result, f.calls)
	return result
}

func (f *fakeRoomGoalCleaner) DeleteGoalsForRoomConversations(_ context.Context, conversationIDs []string) (int, error) {
	f.conversationCalls = append(f.conversationCalls, append([]string(nil), conversationIDs...))
	return len(conversationIDs), f.conversationErr
}

func (f *fakeRoomGoalCleaner) DeleteGoalsForRoomMember(_ context.Context, agentID string, conversationIDs []string) (int, error) {
	f.memberCalls = append(f.memberCalls, fakeRoomGoalMemberCleanup{
		agentID:         agentID,
		conversationIDs: append([]string(nil), conversationIDs...),
	})
	return len(conversationIDs), f.memberErr
}

func createTestAgent(
	t *testing.T,
	service *agentsvc.Service,
	ctx context.Context,
	name string,
) *protocol.Agent {
	t.Helper()

	item, err := service.CreateAgent(ctx, protocol.CreateRequest{Name: name})
	if err != nil {
		t.Fatalf("创建测试 agent 失败: %v", err)
	}
	return item
}

func newRoomTestConfig(t *testing.T) config.Config {
	t.Helper()

	root := t.TempDir()
	stateRoot := filepath.Join(root, ".nexus")
	t.Setenv("HOME", root)
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", stateRoot)
	return config.Config{
		Host:           "127.0.0.1",
		Port:           18011,
		ProjectName:    "nexus-room-test",
		APIPrefix:      "/nexus/v1",
		WebSocketPath:  "/nexus/v1/chat/ws",
		DefaultAgentID: "nexus",
		WorkspacePath:  filepath.Join(root, "workspace"),
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(root, "nexus.db"),
	}
}

func seedRoomPrivateSession(
	t *testing.T,
	files *workspacestore.SessionFileStore,
	workspacePath string,
	roomType string,
	conversationID string,
	agentID string,
) string {
	t.Helper()

	sessionKey := protocol.BuildRoomAgentSessionKey(conversationID, agentID, roomType)
	now := time.Now().UTC()
	if _, err := files.UpsertSession(workspacePath, protocol.Session{
		SessionKey:     sessionKey,
		AgentID:        agentID,
		ChannelType:    "websocket",
		ChatType:       "group",
		Status:         "active",
		CreatedAt:      now,
		LastActivity:   now,
		Title:          "Room Chat",
		MessageCount:   0,
		Options:        map[string]any{},
		IsActive:       true,
		ConversationID: stringPointer(conversationID),
	}); err != nil {
		t.Fatalf("创建 room 私有会话失败: %v", err)
	}
	return sessionKey
}

func seedRoomConversationLog(
	t *testing.T,
	root string,
	conversationID string,
	roomID string,
) {
	t.Helper()

	roomHistory := workspacestore.NewRoomHistoryStore(root)
	if err := roomHistory.AppendInlineMessage("", conversationID, protocol.Message{
		"message_id":      "seed_" + conversationID,
		"session_key":     protocol.BuildRoomSharedSessionKey(conversationID),
		"room_id":         roomID,
		"conversation_id": conversationID,
		"round_id":        "seed-round",
		"role":            "user",
		"content":         "seed",
		"timestamp":       time.Now().UnixMilli(),
	}); err != nil {
		t.Fatalf("写入 room 共享日志失败: %v", err)
	}
}

func seedRoomDatabaseMessageRound(
	t *testing.T,
	db *sql.DB,
	conversationID string,
	sessionID string,
	suffix string,
) (string, string) {
	t.Helper()

	messageID := "msg-" + suffix
	roundID := "round-" + suffix
	if _, err := db.Exec(`
INSERT INTO messages (
    id, conversation_id, session_id, sender_type, kind, status,
    content_preview, jsonl_path, round_id
) VALUES (?, ?, ?, 'agent', 'text', 'completed', 'seed', 'seed.jsonl', ?)`,
		messageID,
		conversationID,
		sessionID,
		roundID,
	); err != nil {
		t.Fatalf("写入测试 message 失败: %v", err)
	}
	if _, err := db.Exec(`
INSERT INTO rounds (
    id, session_id, round_id, trigger_message_id, status
) VALUES (?, ?, ?, ?, 'success')`,
		"round-row-"+suffix,
		sessionID,
		roundID,
		messageID,
	); err != nil {
		t.Fatalf("写入测试 round 失败: %v", err)
	}
	return messageID, roundID
}

func findRoomSessionID(
	t *testing.T,
	contextValue protocol.ConversationContextAggregate,
	agentID string,
) string {
	t.Helper()

	for _, sessionValue := range contextValue.Sessions {
		if sessionValue.AgentID == agentID {
			return sessionValue.ID
		}
	}
	t.Fatalf("未找到 agent session: conversation=%s agent=%s", contextValue.Conversation.ID, agentID)
	return ""
}

func assertSQLCount(t *testing.T, db *sql.DB, query string, want int, args ...any) {
	t.Helper()

	var got int
	if err := db.QueryRow(query, args...).Scan(&got); err != nil {
		t.Fatalf("查询数量失败: %v query=%s", err, query)
	}
	if got != want {
		t.Fatalf("数量不符合预期: got=%d want=%d query=%s args=%v", got, want, query, args)
	}
}

func assertRoomGoalConversationCleanup(t *testing.T, cleaner *fakeRoomGoalCleaner, index int, wantConversationIDs []string) {
	t.Helper()
	if len(cleaner.conversationCalls) <= index {
		t.Fatalf("goal conversation cleanup calls = %#v, want index %d", cleaner.conversationCalls, index)
	}
	if !sameStringSet(cleaner.conversationCalls[index], wantConversationIDs) {
		t.Fatalf("goal conversation cleanup[%d] = %#v, want %#v", index, cleaner.conversationCalls[index], wantConversationIDs)
	}
}

func assertRoomGoalMemberCleanup(t *testing.T, cleaner *fakeRoomGoalCleaner, index int, wantAgentID string, wantConversationIDs []string) {
	t.Helper()
	if len(cleaner.memberCalls) <= index {
		t.Fatalf("goal member cleanup calls = %#v, want index %d", cleaner.memberCalls, index)
	}
	call := cleaner.memberCalls[index]
	if call.agentID != wantAgentID || !sameStringSet(call.conversationIDs, wantConversationIDs) {
		t.Fatalf("goal member cleanup[%d] = %#v, want agent=%s conversations=%#v", index, call, wantAgentID, wantConversationIDs)
	}
}

func sameStringSet(got []string, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	counts := make(map[string]int, len(want))
	for _, item := range got {
		counts[item]++
	}
	for _, item := range want {
		if counts[item] == 0 {
			return false
		}
		counts[item]--
	}
	for _, count := range counts {
		if count != 0 {
			return false
		}
	}
	return true
}

func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	copyValue := value
	return &copyValue
}

func assertPathExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("期望路径存在: %s err=%v", path, err)
	}
}

func assertPathRemoved(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("期望路径已删除: %s err=%v", path, err)
	}
}

func migrateRoomSQLite(t *testing.T, databaseURL string) {
	t.Helper()
	handlertest.MigrateSQLiteFromDir(t, databaseURL, roomMigrationDir(t))
}

func newRoomTestAgentService(
	t *testing.T,
	cfg config.Config,
) (*agentsvc.Service, *sql.DB, error) {
	t.Helper()
	agentService, db, err := app.NewAgentService(cfg)
	if err != nil {
		return nil, nil, err
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("关闭 Room 测试数据库失败: %v", err)
		}
	})
	return agentService, db, nil
}

func roomMigrationDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations", "sqlite")
}

func TestRoomServiceCleansRoomArtifacts(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	goalCleaner := &fakeRoomGoalCleaner{}
	roomService.SetGoalCleaner(goalCleaner)

	ctx := context.Background()
	agentA := createTestAgent(t, agentService, ctx, "清理助手A")
	agentB := createTestAgent(t, agentService, ctx, "清理助手B")
	agentC := createTestAgent(t, agentService, ctx, "清理助手C")

	mainContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentA.AgentID, agentB.AgentID},
		Name:     "清理测试 room",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}
	if err = roomService.MarkConversationStarted(ctx, mainContext.Conversation.ID, time.Now().UTC()); err != nil {
		t.Fatalf("标记主对话已开始失败: %v", err)
	}
	topicContext, err := roomService.CreateConversation(ctx, mainContext.Room.ID, protocol.CreateConversationRequest{
		Title: "待删除话题",
	})
	if err != nil {
		t.Fatalf("创建话题失败: %v", err)
	}
	if _, err = roomService.AddRoomMember(ctx, mainContext.Room.ID, protocol.AddRoomMemberRequest{AgentID: agentC.AgentID}); err != nil {
		t.Fatalf("追加成员失败: %v", err)
	}

	contextsAfterAdd, err := roomService.GetRoomContexts(ctx, mainContext.Room.ID)
	if err != nil {
		t.Fatalf("读取房间上下文失败: %v", err)
	}
	mainContextAfterAdd, ok := findConversationContext(contextsAfterAdd, mainContext.Conversation.ID)
	if !ok {
		t.Fatalf("未找到主对话上下文")
	}
	topicContextAfterAdd, ok := findConversationContext(contextsAfterAdd, topicContext.Conversation.ID)
	if !ok {
		t.Fatalf("未找到 topic 上下文")
	}

	files := workspacestore.NewSessionFileStore(cfg.WorkspacePath)
	paths := workspacestore.New(cfg.WorkspacePath)
	coordinator := &fakeRoomSessionArtifactDeletionCoordinator{
		deleteFn: func(_ context.Context, call roomSessionArtifactDeletionCall) error {
			_, deleteErr := files.ForOwner(call.ownerUserID).DeleteSession(
				call.workspacePath,
				call.sessionKey,
			)
			return deleteErr
		},
	}
	roomService.SetSessionArtifactDeletionCoordinator(coordinator)

	mainAgentASession := seedRoomPrivateSession(t, files, agentA.WorkspacePath, mainContextAfterAdd.Room.RoomType, mainContextAfterAdd.Conversation.ID, agentA.AgentID)
	mainAgentBSession := seedRoomPrivateSession(t, files, agentB.WorkspacePath, mainContextAfterAdd.Room.RoomType, mainContextAfterAdd.Conversation.ID, agentB.AgentID)
	topicAgentASession := seedRoomPrivateSession(t, files, agentA.WorkspacePath, topicContextAfterAdd.Room.RoomType, topicContextAfterAdd.Conversation.ID, agentA.AgentID)
	topicAgentBSession := seedRoomPrivateSession(t, files, agentB.WorkspacePath, topicContextAfterAdd.Room.RoomType, topicContextAfterAdd.Conversation.ID, agentB.AgentID)
	mainAgentCSession := seedRoomPrivateSession(t, files, agentC.WorkspacePath, mainContextAfterAdd.Room.RoomType, mainContextAfterAdd.Conversation.ID, agentC.AgentID)
	topicAgentCSession := seedRoomPrivateSession(t, files, agentC.WorkspacePath, topicContextAfterAdd.Room.RoomType, topicContextAfterAdd.Conversation.ID, agentC.AgentID)
	seedRoomConversationLog(t, cfg.WorkspacePath, mainContextAfterAdd.Conversation.ID, mainContextAfterAdd.Room.ID)
	seedRoomConversationLog(t, cfg.WorkspacePath, topicContextAfterAdd.Conversation.ID, topicContextAfterAdd.Room.ID)
	mainAgentCDBSessionID := findRoomSessionID(t, mainContextAfterAdd, agentC.AgentID)
	_, mainAgentCRoundID := seedRoomDatabaseMessageRound(
		t,
		db,
		mainContextAfterAdd.Conversation.ID,
		mainAgentCDBSessionID,
		"remove-member",
	)
	mainAgentADBSessionID := findRoomSessionID(t, mainContextAfterAdd, agentA.AgentID)
	_, mainRoundID := seedRoomDatabaseMessageRound(
		t,
		db,
		mainContextAfterAdd.Conversation.ID,
		mainAgentADBSessionID,
		"delete-room",
	)
	topicAgentADBSessionID := findRoomSessionID(t, topicContextAfterAdd, agentA.AgentID)
	if err = roomService.UpdateSessionRuntimeIdentity(
		ctx,
		topicAgentADBSessionID,
		"sdk-topic-agent-a",
		"",
	); err != nil {
		t.Fatalf("写入待清理 SDK session_id 失败: %v", err)
	}
	_, topicRoundID := seedRoomDatabaseMessageRound(
		t,
		db,
		topicContextAfterAdd.Conversation.ID,
		topicAgentADBSessionID,
		"delete-topic",
	)

	if _, err = roomService.RemoveRoomMember(ctx, mainContext.Room.ID, agentC.AgentID); err != nil {
		t.Fatalf("移除成员失败: %v", err)
	}
	assertPathRemoved(t, paths.SessionDir(agentC.WorkspacePath, mainAgentCSession))
	assertPathRemoved(t, paths.SessionDir(agentC.WorkspacePath, topicAgentCSession))
	assertPathExists(t, paths.RoomConversationDir(
		topicContextAfterAdd.Room.OwnerUserID,
		topicContextAfterAdd.Conversation.ID,
	))
	assertSQLCount(t, db, `
SELECT COUNT(*) FROM sessions
WHERE conversation_id = ? AND agent_id = ?`, 0, mainContextAfterAdd.Conversation.ID, agentC.AgentID)
	assertSQLCount(t, db, `SELECT COUNT(*) FROM rounds WHERE round_id = ?`, 0, mainAgentCRoundID)
	assertRoomGoalMemberCleanup(t, goalCleaner, 0, agentC.AgentID, []string{
		mainContextAfterAdd.Conversation.ID,
		topicContextAfterAdd.Conversation.ID,
	})

	fallbackContext, err := roomService.DeleteConversation(ctx, mainContext.Room.ID, topicContextAfterAdd.Conversation.ID)
	if err != nil {
		t.Fatalf("删除 topic 失败: %v", err)
	}
	if fallbackContext.Conversation.ID != mainContextAfterAdd.Conversation.ID {
		t.Fatalf("删除 topic 后未回退到主对话: %+v", fallbackContext.Conversation)
	}
	assertPathRemoved(t, paths.RoomConversationDir(
		topicContextAfterAdd.Room.OwnerUserID,
		topicContextAfterAdd.Conversation.ID,
	))
	assertPathRemoved(t, paths.SessionDir(agentA.WorkspacePath, topicAgentASession))
	assertPathRemoved(t, paths.SessionDir(agentB.WorkspacePath, topicAgentBSession))
	assertPathExists(t, paths.SessionDir(agentA.WorkspacePath, mainAgentASession))
	assertPathExists(t, paths.SessionDir(agentB.WorkspacePath, mainAgentBSession))
	assertSQLCount(t, db, `SELECT COUNT(*) FROM conversations WHERE id = ?`, 0, topicContextAfterAdd.Conversation.ID)
	assertSQLCount(t, db, `SELECT COUNT(*) FROM sessions WHERE conversation_id = ?`, 0, topicContextAfterAdd.Conversation.ID)
	assertSQLCount(t, db, `SELECT COUNT(*) FROM messages WHERE conversation_id = ?`, 0, topicContextAfterAdd.Conversation.ID)
	assertSQLCount(t, db, `SELECT COUNT(*) FROM rounds WHERE round_id = ?`, 0, topicRoundID)
	assertRoomGoalConversationCleanup(t, goalCleaner, 0, []string{topicContextAfterAdd.Conversation.ID})

	currentRoom, err := roomService.GetRoom(ctx, mainContext.Room.ID)
	if err != nil {
		t.Fatalf("读取待删除 Room 当前版本失败: %v", err)
	}
	if err = roomService.DeleteRoomAtVersion(
		ctx,
		mainContext.Room.ID,
		currentRoom.Room.ConfigurationVersion,
	); err != nil {
		t.Fatalf("删除 room 失败: %v", err)
	}
	assertPathRemoved(t, paths.RoomConversationDir(
		mainContextAfterAdd.Room.OwnerUserID,
		mainContextAfterAdd.Conversation.ID,
	))
	assertPathRemoved(t, paths.SessionDir(agentA.WorkspacePath, mainAgentASession))
	assertPathRemoved(t, paths.SessionDir(agentB.WorkspacePath, mainAgentBSession))
	assertSQLCount(t, db, `SELECT COUNT(*) FROM rooms WHERE id = ?`, 0, mainContextAfterAdd.Room.ID)
	assertSQLCount(t, db, `SELECT COUNT(*) FROM members WHERE room_id = ?`, 0, mainContextAfterAdd.Room.ID)
	assertSQLCount(t, db, `SELECT COUNT(*) FROM conversations WHERE room_id = ?`, 0, mainContextAfterAdd.Room.ID)
	assertSQLCount(t, db, `SELECT COUNT(*) FROM sessions WHERE conversation_id = ?`, 0, mainContextAfterAdd.Conversation.ID)
	assertSQLCount(t, db, `SELECT COUNT(*) FROM messages WHERE conversation_id = ?`, 0, mainContextAfterAdd.Conversation.ID)
	assertSQLCount(t, db, `SELECT COUNT(*) FROM rounds WHERE round_id = ?`, 0, mainRoundID)
	assertRoomGoalConversationCleanup(t, goalCleaner, 1, []string{mainContextAfterAdd.Conversation.ID})

	calls := coordinator.Calls()
	if len(calls) != 6 {
		t.Fatalf("应通过统一协调器清理 6 个 Room 成员 Session，实际 %+v", calls)
	}
	var foundSDKCleanup bool
	for _, call := range calls {
		if call.ownerUserID != mainContextAfterAdd.Room.OwnerUserID ||
			call.workspacePath == "" ||
			call.sessionKey == "" {
			t.Fatalf("Room Session artifact 删除参数不完整: %+v", call)
		}
		if call.sessionKey == topicAgentASession {
			foundSDKCleanup = call.cleanupSessionID == "sdk-topic-agent-a"
		}
	}
	if !foundSDKCleanup {
		t.Fatalf("Room 未把 SQL SDK session_id 交给统一协调器: %+v", calls)
	}
}

type fakeConversationSessionForker struct {
	sourceSessionKey string
	targetSessionKey string
	targetRoundID    string
	sourceSessionID  string
	sourceMessageID  string
	forkErr          error
	onFork           func()
}

func (f *fakeConversationSessionForker) PrepareConversationFork(
	_ context.Context,
	sourceSessionKey string,
	targetRoundID string,
) (string, string, error) {
	f.sourceSessionKey = sourceSessionKey
	f.targetRoundID = targetRoundID
	return "source-sdk-session", "source-boundary-message", nil
}

func (f *fakeConversationSessionForker) ForkConversationSession(
	_ context.Context,
	sourceSessionKey string,
	targetSessionKey string,
	targetRoundID string,
	sourceSessionID string,
	sourceMessageID string,
) error {
	f.sourceSessionKey = sourceSessionKey
	f.targetSessionKey = targetSessionKey
	f.targetRoundID = targetRoundID
	f.sourceSessionID = sourceSessionID
	f.sourceMessageID = sourceMessageID
	if f.onFork != nil {
		f.onFork()
	}
	return f.forkErr
}

func TestRoomServiceForkFailureCompensatesAfterRequestCancellation(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)
	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)

	baseCtx := context.Background()
	if err = agentService.EnsureReady(baseCtx); err != nil {
		t.Fatalf("初始化主智能体失败: %v", err)
	}
	// 主智能体 DM 不支持分支，这里用普通 Agent 的 DM。
	forkAgent := createTestAgent(t, agentService, baseCtx, "fork-agent")
	source, err := roomService.EnsureDirectRoom(baseCtx, forkAgent.AgentID)
	if err != nil {
		t.Fatalf("创建 source DM 失败: %v", err)
	}
	requestCtx, cancel := context.WithCancel(baseCtx)
	forker := &fakeConversationSessionForker{
		forkErr: context.Canceled,
		onFork:  cancel,
	}
	roomService.SetConversationSessionForker(forker)
	artifactErr := errors.New("cleanup fork artifacts")
	roomService.SetSessionArtifactDeletionCoordinator(&fakeRoomSessionArtifactDeletionCoordinator{
		err: artifactErr,
	})

	if _, err = roomService.ForkConversation(
		requestCtx,
		source.Room.ID,
		source.Conversation.ID,
		protocol.ForkConversationRequest{RoundID: "round-finished"},
	); !errors.Is(err, context.Canceled) || !errors.Is(err, artifactErr) {
		t.Fatalf("fork 应保留原始错误和提交后清理错误: %v", err)
	}
	assertSQLCount(t, db, "SELECT COUNT(*) FROM conversations WHERE room_id = ?", 1, source.Room.ID)
}

func TestPruneEmptyConversationsReadsCanonicalDMHistory(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := context.Background()

	agentValue := createTestAgent(t, agentService, ctx, "DM 历史助手")
	mainContext, err := roomService.EnsureDirectRoom(ctx, agentValue.AgentID)
	if err != nil {
		t.Fatalf("创建 DM room 失败: %v", err)
	}
	clearRoomDraftForLegacyFixture(t, db, mainContext.Room.ID)
	emptyContext, err := roomService.CreateConversation(
		ctx,
		mainContext.Room.ID,
		protocol.CreateConversationRequest{Title: "DM 空白页"},
	)
	if err != nil {
		t.Fatalf("创建 DM 空白会话失败: %v", err)
	}
	setConversationCreatedAt(t, db, mainContext.Conversation.ID, time.Now().UTC().Add(-2*time.Hour))
	setConversationCreatedAt(t, db, emptyContext.Conversation.ID, time.Now().UTC().Add(-time.Hour))

	sessionKey := protocol.BuildRoomAgentSessionKey(
		mainContext.Conversation.ID,
		agentValue.AgentID,
		protocol.RoomTypeDM,
	)
	history := workspacestore.NewAgentHistoryStore(cfg.WorkspacePath)
	if err = history.AppendRoundMarker(
		agentValue.WorkspacePath,
		sessionKey,
		"round-user-input",
		"真实用户输入",
		time.Now().UnixMilli(),
	); err != nil {
		t.Fatalf("写入 DM canonical history 失败: %v", err)
	}

	report, err := roomService.PruneEmptyConversations(ctx, roomsvc.PruneEmptyConversationsOptions{
		RoomID: mainContext.Room.ID,
		Apply:  true,
	})
	if err != nil {
		t.Fatalf("清理 DM room 失败: %v", err)
	}
	mainItem := requirePruneItem(t, report, mainContext.Conversation.ID)
	if mainItem.State != "occupied" || !slices.Contains(mainItem.Reasons, "canonical_user_input_present") {
		t.Fatalf("DM canonical history 必须保护会话: %+v", mainItem)
	}
	contexts, err := roomService.GetRoomContexts(ctx, mainContext.Room.ID)
	if err != nil || len(contexts) != 2 {
		t.Fatalf("有用户输入 DM + 一个空白页都应保留: contexts=%+v err=%v", contexts, err)
	}
}

func TestPruneEmptyConversationsSkipsUnknownPersistentReferenceProbe(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := context.Background()

	agentValue := createTestAgent(t, agentService, ctx, "引用探测失败助手")
	mainContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentValue.AgentID},
		Name:     "引用探测失败 Room",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}
	clearRoomDraftForLegacyFixture(t, db, mainContext.Room.ID)
	if _, err = roomService.CreateConversation(
		ctx,
		mainContext.Room.ID,
		protocol.CreateConversationRequest{Title: "无法确认引用的空白页"},
	); err != nil {
		t.Fatalf("创建空白页失败: %v", err)
	}
	if _, err = db.Exec(`DROP TABLE goal_usage_source_checkpoints`); err != nil {
		t.Fatalf("制造持久引用查询错误失败: %v", err)
	}

	report, err := roomService.PruneEmptyConversations(ctx, roomsvc.PruneEmptyConversationsOptions{
		RoomID: mainContext.Room.ID,
		Apply:  true,
	})
	if err != nil {
		t.Fatalf("引用探测错误应降级为 unknown/skip: %v", err)
	}
	if report.Deleted != 0 || report.Unknown != 2 {
		t.Fatalf("持久引用无法确认时不得删除: %+v", report)
	}
	for _, item := range report.Items {
		if item.Action != "skip_unknown" ||
			!containsReasonPrefix(item.Reasons, "persistent_reference_probe_failed:") {
			t.Fatalf("unknown item 缺少持久引用错误: %+v", item)
		}
	}
}

func clearRoomDraftForLegacyFixture(t *testing.T, db *sql.DB, roomID string) {
	t.Helper()
	if _, err := db.Exec(
		`UPDATE conversations SET is_draft = false WHERE room_id = ? AND is_draft = true`,
		roomID,
	); err != nil {
		t.Fatalf("模拟旧版本已遗留的非 draft 空白页失败: %v", err)
	}
}

func setConversationCreatedAt(t *testing.T, db *sql.DB, conversationID string, createdAt time.Time) {
	t.Helper()
	if _, err := db.Exec(
		`UPDATE conversations SET created_at = ?, updated_at = ?, last_activity_at = ? WHERE id = ?`,
		createdAt,
		createdAt,
		createdAt,
		conversationID,
	); err != nil {
		t.Fatalf("设置 conversation 时间失败: %v", err)
	}
}

func requirePruneItem(
	t *testing.T,
	report roomsvc.EmptyConversationPruneReport,
	conversationID string,
) roomsvc.EmptyConversationPruneItem {
	t.Helper()
	for _, item := range report.Items {
		if item.ConversationID == conversationID {
			return item
		}
	}
	t.Fatalf("未找到 prune item: conversation=%s report=%+v", conversationID, report)
	return roomsvc.EmptyConversationPruneItem{}
}

func containsReasonPrefix(reasons []string, prefix string) bool {
	for _, reason := range reasons {
		if strings.HasPrefix(reason, prefix) {
			return true
		}
	}
	return false
}

func TestPrivateDomainIncludesAgentContactChannelsOutsideDirectoryLimit(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)
	agents, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	service := app.NewRoomServiceWithDB(cfg, db, agents)
	ctx := context.Background()
	a := createTestAgent(t, agents, ctx, "QA")
	b := createTestAgent(t, agents, ctx, "Polish")
	c := createTestAgent(t, agents, ctx, "Other")
	createContact := func(ids []string) *protocol.ConversationContextAggregate {
		t.Helper()
		value, err := service.CreateRoom(ctx, protocol.CreateRoomRequest{AgentIDs: ids, IsContactChannel: true, PrivateMessagesEnabled: true})
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	contact := createContact([]string{a.AgentID, b.AgentID})
	unrelated := createContact([]string{a.AgentID, c.AgentID})
	foreign := createContact([]string{b.AgentID, c.AgentID})
	if _, err = db.ExecContext(ctx, "UPDATE rooms SET owner_user_id = ? WHERE id = ?", "foreign-owner", foreign.Room.ID); err != nil {
		t.Fatal(err)
	}
	store := workspacestore.NewRoomDirectedMessageStore(cfg.WorkspacePath)
	for _, fact := range []struct {
		source, target, id string
		room               *protocol.ConversationContextAggregate
	}{
		{a.AgentID, b.AgentID, "incoming", contact}, {b.AgentID, a.AgentID, "outgoing", contact}, {a.AgentID, c.AgentID, "other", unrelated},
	} {
		if err = store.AppendMessage(fact.room.Room.OwnerUserID, protocol.RoomDirectedMessageRecord{MessageID: fact.id, RoomID: fact.room.Room.ID, ConversationID: fact.room.Conversation.ID, SourceAgentID: fact.source, Recipients: []string{fact.target}, Content: fact.id, Timestamp: 1}); err != nil {
			t.Fatal(err)
		}
	}
	// A newer ordinary room consumes the entire directory budget, but must not hide contact history.
	if _, err = service.CreateRoom(ctx, protocol.CreateRoomRequest{AgentIDs: []string{a.AgentID, c.AgentID}, Name: "Recent ordinary room"}); err != nil {
		t.Fatal(err)
	}
	page, err := service.ListAgentPrivateThreads(ctx, b.AgentID, roomsvc.AgentPrivateDomainQuery{RoomLimit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].MessageCount != 2 {
		t.Fatalf("global threads = %#v", page.Items)
	}
	events, err := service.ListAgentPrivateEvents(ctx, b.AgentID, page.Items[0].ThreadID, roomsvc.AgentPrivateDomainQuery{RoomLimit: 1})
	if err != nil || len(events.Items) != 2 {
		t.Fatalf("events = %#v, %v", events, err)
	}
	if events.Items[0].Direction != "incoming" || events.Items[1].Direction != "outgoing" {
		t.Fatalf("directions = %#v", events.Items)
	}
	scoped, err := service.ListAgentPrivateThreads(ctx, b.AgentID, roomsvc.AgentPrivateDomainQuery{RoomID: unrelated.Room.ID, ConversationID: unrelated.Conversation.ID})
	if err != nil || len(scoped.Items) != 0 {
		t.Fatalf("unrelated scope = %#v, %v", scoped, err)
	}
	rooms, err := service.ListRooms(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range rooms {
		if item.Room.IsContactChannel {
			t.Fatal("contact channel leaked into ordinary directory")
		}
	}
	repository := roomrepo.NewSQLRepository(cfg.DatabaseDriver, db)
	ids, err := repository.ListAgentContactRoomIDs(ctx, contact.Room.OwnerUserID, b.AgentID)
	if err != nil || len(ids) != 1 || ids[0] != contact.Room.ID {
		t.Fatalf("owner/member scope = %v, %v", ids, err)
	}
}

func TestRelayExecutionRoomKeepsOneLocalAgentAndStableBinding(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)
	agents, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	service := app.NewRoomServiceWithDB(cfg, db, agents)
	ctx := t.Context()
	agent := createTestAgent(t, agents, ctx, "在线执行助手")
	first, err := service.EnsureRelayExecutionRoom(ctx, "scope:online-room", agent.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	again, err := service.EnsureRelayExecutionRoom(ctx, "scope:online-room", agent.AgentID)
	if err != nil || first.Room.ID != again.Room.ID || first.Conversation.ID != again.Conversation.ID || first.Room.PrivateMessagesEnabled || first.Room.HostAutoReplyEnabled {
		t.Fatalf("unstable or unsafe room: %+v %v", again, err)
	}
	other, err := service.EnsureRelayExecutionRoom(ctx, "other-scope:online-room", agent.AgentID)
	if err != nil || other.Room.ID == first.Room.ID {
		t.Fatalf("account scopes mixed: %+v %v", other, err)
	}
	second := createTestAgent(t, agents, ctx, "其他助手")
	if _, err = service.AddRoomMember(ctx, first.Room.ID, protocol.AddRoomMemberRequest{AgentID: second.AgentID}); err != nil {
		t.Fatal(err)
	}
	if _, err = service.EnsureRelayExecutionRoom(ctx, "scope:online-room", agent.AgentID); err == nil {
		t.Fatal("must not silently reuse modified execution members")
	}
}
