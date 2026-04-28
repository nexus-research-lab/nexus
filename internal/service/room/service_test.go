package room_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
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

func TestRoomServiceLifecycle(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)

	ctx := context.Background()
	agentA := createTestAgent(t, agentService, ctx, "测试助手A")
	agentB := createTestAgent(t, agentService, ctx, "测试助手B")
	agentC := createTestAgent(t, agentService, ctx, "测试助手C")

	mainContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentA.AgentID, agentB.AgentID},
		Name:     "产品讨论",
		Title:    "主对话",
		Avatar:   "7",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}
	if mainContext.Room.RoomType != protocol.RoomTypeGroup {
		t.Fatalf("room_type 不正确: %s", mainContext.Room.RoomType)
	}
	if mainContext.Conversation.ConversationType != protocol.ConversationTypeMain {
		t.Fatalf("主对话类型不正确: %s", mainContext.Conversation.ConversationType)
	}
	if len(mainContext.Members) != 3 {
		t.Fatalf("成员数量不正确: got=%d want=3", len(mainContext.Members))
	}
	if len(mainContext.Sessions) != 2 {
		t.Fatalf("主对话 session 数量不正确: got=%d want=2", len(mainContext.Sessions))
	}
	if mainContext.Room.Avatar != "7" {
		t.Fatalf("room avatar 不正确: got=%q want=%q", mainContext.Room.Avatar, "7")
	}

	rooms, err := roomService.ListRooms(ctx, 20)
	if err != nil {
		t.Fatalf("列出 room 失败: %v", err)
	}
	if len(rooms) != 1 {
		t.Fatalf("room 数量不正确: got=%d want=1", len(rooms))
	}
	if rooms[0].Room.Avatar != "7" {
		t.Fatalf("list room avatar 不正确: got=%q want=%q", rooms[0].Room.Avatar, "7")
	}

	updatedAvatar := "12"
	mainContext, err = roomService.UpdateRoom(ctx, mainContext.Room.ID, protocol.UpdateRoomRequest{
		Avatar: &updatedAvatar,
	})
	if err != nil {
		t.Fatalf("更新 room avatar 失败: %v", err)
	}
	if mainContext.Room.Avatar != updatedAvatar {
		t.Fatalf("更新后 room avatar 不正确: got=%q want=%q", mainContext.Room.Avatar, updatedAvatar)
	}

	topicContext, err := roomService.CreateConversation(ctx, mainContext.Room.ID, protocol.CreateConversationRequest{})
	if err != nil {
		t.Fatalf("创建 topic 失败: %v", err)
	}
	if topicContext.Conversation.ConversationType != protocol.ConversationTypeTopic {
		t.Fatalf("topic 类型不正确: %s", topicContext.Conversation.ConversationType)
	}
	if len(topicContext.Sessions) != 2 {
		t.Fatalf("topic session 数量不正确: got=%d want=2", len(topicContext.Sessions))
	}

	updatedContext, err := roomService.AddRoomMember(ctx, mainContext.Room.ID, protocol.AddRoomMemberRequest{
		AgentID: agentC.AgentID,
	})
	if err != nil {
		t.Fatalf("追加成员失败: %v", err)
	}
	if len(updatedContext.Sessions) != 3 {
		t.Fatalf("追加成员后主对话 session 数量不正确: got=%d want=3", len(updatedContext.Sessions))
	}

	updatedContext, err = roomService.RemoveRoomMember(ctx, mainContext.Room.ID, agentC.AgentID)
	if err != nil {
		t.Fatalf("移除成员失败: %v", err)
	}
	if len(updatedContext.Sessions) != 2 {
		t.Fatalf("移除成员后主对话 session 数量不正确: got=%d want=2", len(updatedContext.Sessions))
	}

	fallbackContext, err := roomService.DeleteConversation(ctx, mainContext.Room.ID, topicContext.Conversation.ID)
	if err != nil {
		t.Fatalf("删除 topic 失败: %v", err)
	}
	if fallbackContext.Conversation.ConversationType != protocol.ConversationTypeMain {
		t.Fatalf("删除 topic 后未回退到主对话: %s", fallbackContext.Conversation.ConversationType)
	}

	dmContext, err := roomService.EnsureDirectRoom(ctx, agentA.AgentID)
	if err != nil {
		t.Fatalf("创建直聊失败: %v", err)
	}
	if dmContext.Room.RoomType != protocol.RoomTypeDM {
		t.Fatalf("直聊类型不正确: %s", dmContext.Room.RoomType)
	}
	if len(dmContext.Sessions) != 1 {
		t.Fatalf("直聊 session 数量不正确: got=%d want=1", len(dmContext.Sessions))
	}
}

func TestRoomServiceAllowsMainAgentDirectRoom(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}

	ctx := context.Background()
	if err = agentService.EnsureReady(ctx); err != nil {
		t.Fatalf("初始化主智能体失败: %v", err)
	}

	dmContext, err := roomService.EnsureDirectRoom(ctx, cfg.DefaultAgentID)
	if err != nil {
		t.Fatalf("主智能体直聊创建失败: %v", err)
	}
	if dmContext.Room.RoomType != protocol.RoomTypeDM {
		t.Fatalf("主智能体直聊类型不正确: got=%s", dmContext.Room.RoomType)
	}
	if len(dmContext.Sessions) != 1 {
		t.Fatalf("主智能体直聊 session 数量不正确: got=%d want=1", len(dmContext.Sessions))
	}
	if dmContext.Sessions[0].AgentID != cfg.DefaultAgentID {
		t.Fatalf("主智能体直聊 session agent_id 不正确: got=%s want=%s", dmContext.Sessions[0].AgentID, cfg.DefaultAgentID)
	}

	reusedContext, err := roomService.EnsureDirectRoom(ctx, cfg.DefaultAgentID)
	if err != nil {
		t.Fatalf("复用主智能体直聊失败: %v", err)
	}
	if reusedContext.Room.ID != dmContext.Room.ID {
		t.Fatalf("主智能体直聊未复用既有 room: got=%s want=%s", reusedContext.Room.ID, dmContext.Room.ID)
	}
	if reusedContext.Conversation.ID != dmContext.Conversation.ID {
		t.Fatalf("主智能体直聊未复用既有对话: got=%s want=%s", reusedContext.Conversation.ID, dmContext.Conversation.ID)
	}
}

func TestRoomServiceRejectsMainAgentAsGroupMember(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}

	ctx := context.Background()
	agentA := createTestAgent(t, agentService, ctx, "分组测试助手A")

	if _, err = roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{cfg.DefaultAgentID, agentA.AgentID},
	}); err == nil {
		t.Fatal("group room 不应允许主智能体作为成员")
	}
	if _, err = roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{cfg.DefaultAgentID},
	}); err == nil {
		t.Fatal("仅主智能体不应创建 group room")
	}
}

func TestRoomServiceCleansRoomArtifacts(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := serverapp.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := serverapp.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}

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

	mainAgentASession := seedRoomPrivateSession(t, files, agentA.WorkspacePath, mainContextAfterAdd.Room.RoomType, mainContextAfterAdd.Conversation.ID, agentA.AgentID)
	mainAgentBSession := seedRoomPrivateSession(t, files, agentB.WorkspacePath, mainContextAfterAdd.Room.RoomType, mainContextAfterAdd.Conversation.ID, agentB.AgentID)
	topicAgentASession := seedRoomPrivateSession(t, files, agentA.WorkspacePath, topicContextAfterAdd.Room.RoomType, topicContextAfterAdd.Conversation.ID, agentA.AgentID)
	topicAgentBSession := seedRoomPrivateSession(t, files, agentB.WorkspacePath, topicContextAfterAdd.Room.RoomType, topicContextAfterAdd.Conversation.ID, agentB.AgentID)
	mainAgentCSession := seedRoomPrivateSession(t, files, agentC.WorkspacePath, mainContextAfterAdd.Room.RoomType, mainContextAfterAdd.Conversation.ID, agentC.AgentID)
	topicAgentCSession := seedRoomPrivateSession(t, files, agentC.WorkspacePath, topicContextAfterAdd.Room.RoomType, topicContextAfterAdd.Conversation.ID, agentC.AgentID)
	seedRoomConversationLog(t, cfg.WorkspacePath, mainContextAfterAdd.Conversation.ID, mainContextAfterAdd.Room.ID)
	seedRoomConversationLog(t, cfg.WorkspacePath, topicContextAfterAdd.Conversation.ID, topicContextAfterAdd.Room.ID)

	if _, err = roomService.RemoveRoomMember(ctx, mainContext.Room.ID, agentC.AgentID); err != nil {
		t.Fatalf("移除成员失败: %v", err)
	}
	assertPathRemoved(t, paths.SessionDir(agentC.WorkspacePath, mainAgentCSession))
	assertPathRemoved(t, paths.SessionDir(agentC.WorkspacePath, topicAgentCSession))
	assertPathExists(t, paths.RoomConversationDir(topicContextAfterAdd.Conversation.ID))

	fallbackContext, err := roomService.DeleteConversation(ctx, mainContext.Room.ID, topicContextAfterAdd.Conversation.ID)
	if err != nil {
		t.Fatalf("删除 topic 失败: %v", err)
	}
	if fallbackContext.Conversation.ID != mainContextAfterAdd.Conversation.ID {
		t.Fatalf("删除 topic 后未回退到主对话: %+v", fallbackContext.Conversation)
	}
	assertPathRemoved(t, paths.RoomConversationDir(topicContextAfterAdd.Conversation.ID))
	assertPathRemoved(t, paths.SessionDir(agentA.WorkspacePath, topicAgentASession))
	assertPathRemoved(t, paths.SessionDir(agentB.WorkspacePath, topicAgentBSession))
	assertPathExists(t, paths.SessionDir(agentA.WorkspacePath, mainAgentASession))
	assertPathExists(t, paths.SessionDir(agentB.WorkspacePath, mainAgentBSession))

	if err = roomService.DeleteRoom(ctx, mainContext.Room.ID); err != nil {
		t.Fatalf("删除 room 失败: %v", err)
	}
	assertPathRemoved(t, paths.RoomConversationDir(mainContextAfterAdd.Conversation.ID))
	assertPathRemoved(t, paths.SessionDir(agentA.WorkspacePath, mainAgentASession))
	assertPathRemoved(t, paths.SessionDir(agentB.WorkspacePath, mainAgentBSession))
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
	t.Setenv("HOME", root)
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(root, ".nexus"))
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
	if err := roomHistory.AppendInlineMessage(conversationID, protocol.Message{
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

	db, err := sql.Open("sqlite3", databaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer db.Close()

	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("设置 goose 方言失败: %v", err)
	}
	if err = goose.Up(db, roomMigrationDir(t)); err != nil {
		t.Fatalf("执行 migration 失败: %v", err)
	}
}

func roomMigrationDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations", "sqlite")
}
