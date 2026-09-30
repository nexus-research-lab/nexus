//go:build darwin

// INPUT: 固定真实 nxs、SQLite 与独立 owner/workspace/scratch。
// OUTPUT: AppServices 正常退出/数据库重开后的沙箱回执与资源回收证据。
// POS: 显式 macOS 集成门禁；不发送模型请求，不替代 App UI 或签名安装。
package app

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	bridge "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	permission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtime "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	sandboxstore "github.com/nexus-research-lab/nexus/internal/storage/sandbox"
	"github.com/pressly/goose/v3"
)

func TestAppServicesCloseRealSandboxRuntimeAfterRestart(t *testing.T) {
	binary := os.Getenv("NEXUS_SANDBOX_TEST_BINARY")
	if binary == "" {
		t.Skip("set NEXUS_SANDBOX_TEST_BINARY to a fixed nxs binary")
	}
	root := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", root)
	for _, directory := range []string{"workspace", "home", "config", "runtime"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for generation := uint64(1); generation <= 2; generation++ {
		runAppSandboxShutdownCycle(t, binary, root, generation)
	}
}

func runAppSandboxShutdownCycle(t *testing.T, binary, root string, generation uint64) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(root, "app.db")+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if generation == 1 {
		if err := goose.SetDialect("sqlite3"); err != nil {
			t.Fatal(err)
		}
		if err := goose.Up(db, "../../db/migrations/sqlite"); err != nil {
			t.Fatal(err)
		}
	}
	store := sandboxstore.NewRepository(config.Config{DatabaseDriver: "sqlite"}, db)
	manager := runtime.NewManager()
	manager.SetSandboxPolicyReceiptStore(store)
	defer manager.Close(context.Background())
	lease, err := runtime.AcquireSandboxResource(t.Context(), runtime.SandboxResourceInput{
		OwnerUserID: "owner", SessionKey: "session", Root: filepath.Join(root, "runtime"),
	})
	if err != nil {
		t.Fatal(err)
	}
	options, err := clientopts.BuildAgentClientOptions(t.Context(), nil, clientopts.AgentClientOptionsInput{
		OwnerUserID: "owner", WorkspacePath: filepath.Join(root, "workspace"), RuntimeKind: "nxs", AppMode: "desktop",
		PermissionMode: permission.ModeDefault, AutoMemoryDisabled: true, AutoDreamDisabled: true,
		SettingSources: []string{"user"}, SandboxResources: lease.Resources(),
	})
	if err != nil {
		_ = lease.Release()
		t.Fatal(err)
	}
	options.CLIPath = binary
	options.Env["HOME"] = filepath.Join(root, "home")
	options.Env["NEXUS_CONFIG_DIR"] = filepath.Join(root, "config")
	options.Env["CLAUDE_CONFIG_DIR"] = filepath.Join(root, "config")
	options.Env["TMPDIR"] = lease.Path()
	options.Skills = bridge.SkillOptions{Mode: bridge.SkillModeNone}
	options.MCP.StrictConfig = true
	options.Runtime.InitializeTimeout = 15 * time.Second
	startup, err := manager.BeginClientStartup(t.Context(), "session", "owner")
	if err != nil {
		_ = lease.Release()
		t.Fatal(err)
	}
	defer startup.Close()
	if _, err := startup.GetOrCreateWithFactory(t.Context(), options, nil); err != nil {
		_ = lease.Release()
		t.Fatal(err)
	}
	if consumed, err := startup.BindSandboxLease(lease); err != nil || !consumed {
		_ = lease.Release()
		t.Fatalf("bind lease: consumed=%t err=%v", consumed, err)
	}
	if err := startup.Connect(t.Context()); err != nil {
		t.Fatal(err)
	}
	startup.Close()
	before, found, err := store.Latest(t.Context(), "owner", "session")
	if err != nil || !found || before.Generation != generation || before.Phase != protocol.SandboxPolicyReceiptConfirmed {
		t.Fatalf("connected receipt=%#v found=%t err=%v", before, found, err)
	}
	services := &AppServices{DB: db, Runtime: manager} // Keep shared DB open for verification.
	if err := services.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	after, found, err := store.Latest(t.Context(), "owner", "session")
	if err != nil || !found || after.Generation != generation || after.Phase != protocol.SandboxPolicyReceiptRetired {
		t.Fatalf("closed receipt=%#v found=%t err=%v", after, found, err)
	}
	if _, err := os.Stat(lease.Path()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("scratch survived normal App shutdown: %v", err)
	}
}
