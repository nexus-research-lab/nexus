// INPUT: 共享应用资源、真实 SQLite 与提供沙箱回执的受控 runtime。
// OUTPUT: 正常退出在数据库关闭前完成 runtime 回收并持久化终态。
// POS: AppServices 生命周期集成回归；不替代平台进程隔离验收。
package app

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	bridge "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtime "github.com/nexus-research-lab/nexus/internal/runtime"
	sandboxstore "github.com/nexus-research-lab/nexus/internal/storage/sandbox"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

type shutdownFactory struct{ client *shutdownClient }

func (f shutdownFactory) New(bridge.Options) runtime.Client { return f.client }

type shutdownClient struct {
	runtime.Client
	disconnected bool
}

func (*shutdownClient) Connect(context.Context) error { return nil }
func (*shutdownClient) Retire()                       {}
func (c *shutdownClient) Disconnect(context.Context) error {
	c.disconnected = true
	return nil
}
func (*shutdownClient) EffectiveSandboxPolicyReceipt() *runtime.SandboxEffectivePolicyReceipt {
	return &runtime.SandboxEffectivePolicyReceipt{
		Version: 1, SessionID: "app-shutdown-runtime", RuntimeKind: bridge.RuntimeNXS,
		PolicyDigest: "sha256:app-shutdown-policy", Phase: protocol.SandboxPolicyReceiptConfirmed,
		CapabilityEvidence: "bridge_negotiated", IsolationEvidence: "not_attested", ConfirmedAt: time.Now().UTC(),
	}
}

func TestAppServicesClosePersistsSandboxRetirementBeforeDatabaseClose(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.db")
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err := goose.Up(db, "../../db/migrations/sqlite"); err != nil {
		t.Fatal(err)
	}
	store := sandboxstore.NewRepository(config.Config{DatabaseDriver: "sqlite"}, db)
	client := &shutdownClient{}
	manager := runtime.NewManagerWithFactory(shutdownFactory{client})
	manager.SetSandboxPolicyReceiptStore(store)
	startup, err := manager.BeginClientStartup(t.Context(), "session", "owner")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := startup.GetOrCreateWithFactory(t.Context(), bridge.Options{Env: map[string]string{"NEXUS_RUNTIME_USER_ID": "owner"}}, nil); err != nil {
		t.Fatal(err)
	}
	if err := startup.Connect(t.Context()); err != nil {
		t.Fatal(err)
	}
	startup.Close()
	services := &AppServices{DB: db, Runtime: manager, ownsDB: true}
	if err := services.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := db.Ping(); err == nil {
		t.Fatal("application-owned database remained open")
	}
	reopened, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	snapshot, found, err := sandboxstore.NewRepository(config.Config{DatabaseDriver: "sqlite"}, reopened).Latest(t.Context(), "owner", "session")
	if err != nil || !found || snapshot.Phase != protocol.SandboxPolicyReceiptRetired || !client.disconnected {
		t.Fatalf("normal App exit left runtime unresolved: phase=%s found=%t disconnected=%t err=%v", snapshot.Phase, found, client.disconnected, err)
	}
}

func TestAppServicesCloseTimeoutKeepsDatabaseForPendingRuntimeWrites(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "app.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	manager := runtime.NewManager()
	canceled, release := make(chan struct{}), make(chan struct{})
	if !manager.StartBackgroundTask("session", func(ctx context.Context) {
		<-ctx.Done()
		close(canceled)
		<-release
	}) {
		t.Fatal("background task was not started")
	}
	services := &AppServices{DB: db, Runtime: manager, ownsDB: true}
	short, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	if err := services.Close(short); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Close=%v, want deadline", err)
	}
	<-canceled
	if err := db.Ping(); err != nil {
		t.Fatalf("database closed while runtime still had writes: %v", err)
	}
	close(release)
	if err := services.Close(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := db.Ping(); err == nil {
		t.Fatal("database remained open after runtime finished")
	}
}
