package launcher

import (
	"context"
	"database/sql"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"

	"github.com/nexus-research-lab/nexus/internal/config"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	sessionsvc "github.com/nexus-research-lab/nexus/internal/service/session"
	sqliterepo "github.com/nexus-research-lab/nexus/internal/storage/sqlite"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
)

func TestLauncherQueryAndSuggestions(t *testing.T) {
	cfg := newLauncherTestConfig(t)
	migrateLauncherSQLite(t, cfg.DatabaseURL)

	db, err := sql.Open("sqlite3", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer db.Close()
	agentService := agentsvc.NewService(cfg, sqliterepo.NewAgentRepository(db))
	roomService := roomsvc.NewService(cfg, agentService, sqliterepo.NewRoomRepository(db))
	sessionService := sessionsvc.NewService(cfg, agentService, sqliterepo.NewSessionRepository(db))
	service := NewService(cfg, agentService, roomService, sessionService)

	ctx := context.Background()
	agentA := createLauncherAgent(t, agentService, ctx, "产品助手")
	agentB := createLauncherAgent(t, agentService, ctx, "设计助手")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentA.AgentID, agentB.AgentID},
		Name:     "设计评审",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}
	queryResult, err := service.Query(ctx, "@产品助手 请梳理需求")
	if err != nil {
		t.Fatalf("解析 @agent 查询失败: %v", err)
	}
	if queryResult.ActionType != actionOpenAgentDM || queryResult.TargetID != agentA.AgentID {
		t.Fatalf("@agent 查询动作不正确: %+v", queryResult)
	}
	if queryResult.InitialMessage != "请梳理需求" {
		t.Fatalf("@agent 初始消息不正确: %s", queryResult.InitialMessage)
	}

	queryResult, err = service.Query(ctx, "#设计评审 进入房间")
	if err != nil {
		t.Fatalf("解析 #room 查询失败: %v", err)
	}
	if queryResult.ActionType != actionOpenRoom || queryResult.TargetID != roomContext.Room.ID {
		t.Fatalf("#room 查询动作不正确: %+v", queryResult)
	}

	queryResult, err = service.Query(ctx, "随便聊聊")
	if err != nil {
		t.Fatalf("解析普通查询失败: %v", err)
	}
	if queryResult.ActionType != actionOpenApp || queryResult.TargetID != "app" {
		t.Fatalf("open_app 动作不正确: %+v", queryResult)
	}

	suggestions, err := service.Suggestions(ctx)
	if err != nil {
		t.Fatalf("读取 Launcher 推荐失败: %v", err)
	}
	if len(suggestions.Agents) != 2 {
		t.Fatalf("推荐 agent 数量不正确: got=%d want=2", len(suggestions.Agents))
	}
	if len(suggestions.Rooms) != 1 {
		t.Fatalf("推荐 room 数量不正确: got=%d want=1", len(suggestions.Rooms))
	}
	if suggestions.Rooms[0].ID != roomContext.Room.ID {
		t.Fatalf("推荐 room 不正确: %+v", suggestions.Rooms[0])
	}
	if suggestions.Rooms[0].Type != "room" {
		t.Fatalf("推荐 room 类型不正确: %+v", suggestions.Rooms[0])
	}

	if _, err = roomService.UpdateConversation(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.UpdateConversationRequest{
		Title: "需求讨论",
	}); err != nil {
		t.Fatalf("更新 room 对话标题失败: %v", err)
	}

	dmContext, err := roomService.EnsureDirectRoom(ctx, agentA.AgentID)
	if err != nil {
		t.Fatalf("创建直聊失败: %v", err)
	}
	if _, err = roomService.UpdateConversation(ctx, dmContext.Room.ID, dmContext.Conversation.ID, protocol.UpdateConversationRequest{
		Title: "产品私聊",
	}); err != nil {
		t.Fatalf("更新直聊标题失败: %v", err)
	}

	bootstrap, err := service.Bootstrap(ctx)
	if err != nil {
		t.Fatalf("读取 launcher bootstrap 失败: %v", err)
	}
	if len(bootstrap.Conversations) == 0 {
		t.Fatalf("bootstrap conversations 不应为空")
	}
	assertContainsBootstrapRoomType(t, bootstrap.Rooms, roomContext.Room.ID, "room")
	assertContainsConversationTitle(t, bootstrap.Conversations, "需求讨论")
	assertContainsConversationTitle(t, bootstrap.Conversations, "产品私聊")
}

func assertContainsBootstrapRoomType(
	t *testing.T,
	items []BootstrapRoom,
	roomID string,
	roomType string,
) {
	t.Helper()

	for _, item := range items {
		if item.ID == roomID && item.RoomType == roomType {
			return
		}
	}
	t.Fatalf("bootstrap room 类型缺失: room_id=%s room_type=%s items=%+v", roomID, roomType, items)
}

func assertContainsConversationTitle(
	t *testing.T,
	items []BootstrapConversation,
	title string,
) {
	t.Helper()

	for _, item := range items {
		if item.Title == title {
			return
		}
	}
	t.Fatalf("bootstrap conversations 缺少标题 %q: %+v", title, items)
}

func createLauncherAgent(
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

func newLauncherTestConfig(t *testing.T) config.Config {
	t.Helper()

	root := t.TempDir()
	return config.Config{
		Host:           "127.0.0.1",
		Port:           18012,
		ProjectName:    "nexus-launcher-test",
		APIPrefix:      "/agent/v1",
		WebSocketPath:  "/agent/v1/chat/ws",
		DefaultAgentID: "nexus",
		WorkspacePath:  filepath.Join(root, "workspace"),
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(root, "nexus.db"),
	}
}

func migrateLauncherSQLite(t *testing.T, databaseURL string) {
	t.Helper()

	db, err := sql.Open("sqlite3", databaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer db.Close()

	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("设置 goose 方言失败: %v", err)
	}
	if err = goose.Up(db, launcherMigrationDir(t)); err != nil {
		t.Fatalf("执行 migration 失败: %v", err)
	}
}

func launcherMigrationDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations", "sqlite")
}
