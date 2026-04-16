// =====================================================
// @File   ：service_test.go
// @Date   ：2026/04/10 23:22:00
// @Author ：leemysw
// 2026/04/10 23:22:00   Create
// =====================================================

package launcher

import (
	"context"
	"database/sql"
	agent2 "github.com/nexus-research-lab/nexus-core/internal/agent"
	"github.com/nexus-research-lab/nexus-core/internal/config"
	roomsvc "github.com/nexus-research-lab/nexus-core/internal/room"
	"path/filepath"
	"runtime"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
)

func TestLauncherQueryAndSuggestions(t *testing.T) {
	cfg := newLauncherTestConfig(t)
	migrateLauncherSQLite(t, cfg.DatabaseURL)

	agentService, err := agent2.NewService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService, err := roomsvc.NewService(cfg)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}
	service := NewService(cfg, agentService, roomService)

	ctx := context.Background()
	agentA := createLauncherAgent(t, agentService, ctx, "产品助手")
	agentB := createLauncherAgent(t, agentService, ctx, "设计助手")
	roomContext, err := roomService.CreateRoom(ctx, roomsvc.CreateRoomRequest{
		AgentIDs: []string{agentA.AgentID, agentB.AgentID},
		Name:     "设计评审",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}
	if _, err = roomService.EnsureDirectRoom(ctx, agentA.AgentID); err != nil {
		t.Fatalf("创建直聊失败: %v", err)
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
}

func createLauncherAgent(
	t *testing.T,
	service *agent2.Service,
	ctx context.Context,
	name string,
) *agent2.Agent {
	t.Helper()

	item, err := service.CreateAgent(ctx, agent2.CreateRequest{Name: name})
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
	return filepath.Join(filepath.Dir(file), "..", "..", "db", "migrations", "sqlite")
}
