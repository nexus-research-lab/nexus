package channels

import (
	"context"
	"database/sql"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	sqliterepo "github.com/nexus-research-lab/nexus/internal/storage/sqlite"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

type fakeIngressDMHandler struct {
	requests []dmsvc.Request
}

func (f *fakeIngressDMHandler) HandleChat(_ context.Context, request dmsvc.Request) error {
	f.requests = append(f.requests, request)
	return nil
}

func TestIngressServiceAcceptInternalBuildsSessionAndRemembersRoute(t *testing.T) {
	cfg := newIngressTestConfig(t)
	db := migrateIngressSQLite(t, cfg.DatabaseURL)
	defer db.Close()

	agentService := agentsvc.NewService(cfg, sqliterepo.NewAgentRepository(db))
	handler := &fakeIngressDMHandler{}
	router := NewRouter(cfg, db, agentService, permissionctx.NewContext())
	service := NewIngressService(cfg, agentService, handler, router)

	result, err := service.Accept(context.Background(), IngressRequest{
		Channel: "internal",
		Ref:     "chat",
		Content: "来自内部系统的消息",
	})
	if err != nil {
		t.Fatalf("Accept 失败: %v", err)
	}

	if result.SessionKey != "agent:nexus:internal:dm:chat" {
		t.Fatalf("session_key 不正确: %s", result.SessionKey)
	}
	if len(handler.requests) != 1 {
		t.Fatalf("聊天请求数量不正确: %d", len(handler.requests))
	}
	if handler.requests[0].SessionKey != result.SessionKey {
		t.Fatalf("聊天请求 session_key 不正确: %+v", handler.requests[0])
	}
	if handler.requests[0].PermissionHandler == nil {
		t.Fatal("internal ingress 应注入权限处理器")
	}

	route, err := router.GetLastRoute(context.Background(), cfg.DefaultAgentID)
	if err != nil {
		t.Fatalf("读取 last route 失败: %v", err)
	}
	if route == nil || route.Channel != ChannelTypeInternal || route.SessionKey != result.SessionKey {
		t.Fatalf("internal route 记忆不正确: %+v", route)
	}

	decision, err := handler.requests[0].PermissionHandler(context.Background(), sdkprotocol.PermissionRequest{
		ToolName: "Read",
		Input:    map[string]any{"file_path": "README.md"},
	})
	if err != nil {
		t.Fatalf("执行权限处理器失败: %v", err)
	}
	if decision.Behavior != sdkprotocol.PermissionBehaviorAllow {
		t.Fatalf("internal ingress 的 Read 应自动允许: %+v", decision)
	}
}

func TestIngressServiceAcceptTelegramUsesReadOnlyPermissionPolicy(t *testing.T) {
	cfg := newIngressTestConfig(t)
	db := migrateIngressSQLite(t, cfg.DatabaseURL)
	defer db.Close()

	agentService := agentsvc.NewService(cfg, sqliterepo.NewAgentRepository(db))
	handler := &fakeIngressDMHandler{}
	router := NewRouter(cfg, db, agentService, permissionctx.NewContext())
	service := NewIngressService(cfg, agentService, handler, router)

	result, err := service.Accept(context.Background(), IngressRequest{
		Channel:  "telegram",
		ChatType: "group",
		Ref:      "-100123456",
		ThreadID: "12",
		Content:  "群组消息",
	})
	if err != nil {
		t.Fatalf("Accept 失败: %v", err)
	}

	if result.SessionKey != "agent:nexus:tg:group:-100123456:topic:12" {
		t.Fatalf("telegram session_key 不正确: %s", result.SessionKey)
	}
	route, err := router.GetLastRoute(context.Background(), cfg.DefaultAgentID)
	if err != nil {
		t.Fatalf("读取 last route 失败: %v", err)
	}
	if route == nil || route.Channel != ChannelTypeTelegram || route.To != "-100123456" || route.ThreadID != "12" {
		t.Fatalf("telegram route 记忆不正确: %+v", route)
	}

	readDecision, err := handler.requests[0].PermissionHandler(context.Background(), sdkprotocol.PermissionRequest{
		ToolName: "Read",
		Input:    map[string]any{"file_path": "README.md"},
	})
	if err != nil {
		t.Fatalf("Read 权限处理失败: %v", err)
	}
	if readDecision.Behavior != sdkprotocol.PermissionBehaviorAllow {
		t.Fatalf("telegram ingress 的 Read 应自动允许: %+v", readDecision)
	}

	writeDecision, err := handler.requests[0].PermissionHandler(context.Background(), sdkprotocol.PermissionRequest{
		ToolName: "Write",
		Input:    map[string]any{"file_path": "README.md"},
	})
	if err != nil {
		t.Fatalf("Write 权限处理失败: %v", err)
	}
	if writeDecision.Behavior != sdkprotocol.PermissionBehaviorDeny {
		t.Fatalf("telegram ingress 的 Write 应默认拒绝: %+v", writeDecision)
	}
}

func newIngressTestConfig(t *testing.T) config.Config {
	t.Helper()
	root := t.TempDir()
	return config.Config{
		Host:           "127.0.0.1",
		Port:           18040,
		ProjectName:    "nexus-channel-test",
		APIPrefix:      "/agent/v1",
		WebSocketPath:  "/agent/v1/chat/ws",
		DefaultAgentID: "nexus",
		WorkspacePath:  filepath.Join(root, "workspace"),
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(root, "nexus.db"),
	}
}

func migrateIngressSQLite(t *testing.T, databaseURL string) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite3", databaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("设置 goose 方言失败: %v", err)
	}
	if err = goose.Up(db, ingressMigrationDir(t)); err != nil {
		t.Fatalf("执行 migration 失败: %v", err)
	}
	return db
}

func ingressMigrationDir(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("无法定位当前测试文件")
	}
	return filepath.Join(filepath.Dir(filename), "..", "..", "..", "db", "migrations", "sqlite")
}
