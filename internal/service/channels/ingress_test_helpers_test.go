package channels

import (
	"context"
	"database/sql"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"

	_ "modernc.org/sqlite"
)

type fakeIngressDMHandler struct {
	requests     []dmsvc.Request
	ownerUserIDs []string
	err          error
}

type externalSessionNotifyCall struct {
	agentID    string
	sessionKey string
}

type fakeExternalSessionNotifier struct {
	calls []externalSessionNotifyCall
}

func (f *fakeIngressDMHandler) HandleChat(ctx context.Context, request dmsvc.Request) error {
	f.requests = append(f.requests, request)
	f.ownerUserIDs = append(f.ownerUserIDs, authctx.OwnerUserID(ctx))
	if f.err != nil {
		return f.err
	}
	return nil
}

func (f *fakeExternalSessionNotifier) NotifyExternalSessionUpdated(_ context.Context, agentID string, sessionKey string) {
	f.calls = append(f.calls, externalSessionNotifyCall{
		agentID:    agentID,
		sessionKey: sessionKey,
	})
}

func newIngressTestConfig(t *testing.T) config.Config {
	t.Helper()
	root := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", filepath.Join(root, ".nexus"))
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(root, ".nexus"))
	return config.Config{
		Host:           "127.0.0.1",
		Port:           18040,
		ProjectName:    "nexus-channel-test",
		APIPrefix:      "/nexus/v1",
		WebSocketPath:  "/nexus/v1/chat/ws",
		DefaultAgentID: "nexus",
		WorkspacePath:  filepath.Join(root, "workspace"),
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(root, "nexus.db"),
	}
}

func ingressTestOwnerContext(ownerUserID string) context.Context {
	return authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID:     ownerUserID,
		Username:   ownerUserID,
		Role:       authctx.RoleOwner,
		AuthMethod: authctx.AuthMethodLocal,
	})
}

func migrateIngressSQLite(t *testing.T, databaseURL string) *sql.DB {
	t.Helper()

	handlertest.MigrateSQLiteFromDir(t, databaseURL, ingressMigrationDir(t))
	db, err := sql.Open("sqlite", databaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
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
