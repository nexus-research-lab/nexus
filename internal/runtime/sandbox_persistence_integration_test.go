// INPUT: 真实 SQLite migrations、重建 Manager 与可控 runtime client。
// OUTPUT: 正常关闭/重启后的独立代次和不可变历史；异常回执不启动新进程。
// POS: 宿主持久化集成测试，不替代实际 OS 后代清理。
package runtime

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	bridge "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	sandboxstore "github.com/nexus-research-lab/nexus/internal/storage/sandbox"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

// receiptRuntimeClient 提供真实持久化路径所需的 runtime 确认事实。
type receiptRuntimeClient struct {
	fakeRuntimeClient
	receipt SandboxEffectivePolicyReceipt
}

func (c *receiptRuntimeClient) EffectiveSandboxPolicyReceipt() *SandboxEffectivePolicyReceipt {
	copy := c.receipt
	return &copy
}

func (c *receiptRuntimeClient) setSandboxReceiptGeneration(generation uint64) {
	c.receipt.Generation = generation
}

func TestManagerSandboxReceiptSurvivesCleanRestartAndIdleRecreation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "receipts.db")
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(0)&_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err := goose.Up(db, "../../db/migrations/sqlite"); err != nil {
		t.Fatal(err)
	}
	store := sandboxstore.NewRepository(config.Config{DatabaseDriver: "sqlite"}, db)
	var starts int
	factory := runtimeFactoryFunc(func(bridge.Options) Client {
		starts++
		return &receiptRuntimeClient{receipt: SandboxEffectivePolicyReceipt{
			Version: 1, SessionID: fmt.Sprintf("runtime-%d", starts), RuntimeKind: bridge.RuntimeNXS,
			PolicyDigest: fmt.Sprintf("sha256:policy-%d", starts), Phase: protocol.SandboxPolicyReceiptConfirmed,
			CapabilityEvidence: "bridge_negotiated", IsolationEvidence: "not_attested", ConfirmedAt: time.Now().UTC(),
		}}
	})
	manager := NewManagerWithFactory(factory)
	manager.SetSandboxPolicyReceiptStore(store)
	options := bridge.Options{Env: map[string]string{"NEXUS_RUNTIME_USER_ID": "owner"}}
	for generation := uint64(1); generation <= 3; generation++ {
		if generation == 2 {
			// Close and reopen the database as well as the Manager. Generation 3
			// exercises normal recreation after an idle-session removal.
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			db, err = sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)")
			if err != nil {
				t.Fatal(err)
			}
			db.SetMaxOpenConns(1)
			store = sandboxstore.NewRepository(config.Config{DatabaseDriver: "sqlite"}, db)
			manager = NewManagerWithFactory(factory)
			manager.SetSandboxPolicyReceiptStore(store)
		}
		connectSandboxReceiptFixture(t, manager, options)
		snapshot, found, err := store.Latest(t.Context(), "owner", "session")
		if err != nil || !found || snapshot.Generation != generation || snapshot.SessionID != fmt.Sprintf("runtime-%d", generation) || snapshot.Phase != protocol.SandboxPolicyReceiptConfirmed {
			t.Fatalf("generation %d receipt=%#v found=%t error=%v", generation, snapshot, found, err)
		}
		if err := manager.CloseSession(t.Context(), "session"); err != nil {
			t.Fatal(err)
		}
	}
	for generation := uint64(1); generation <= 3; generation++ {
		snapshot, found, err := store.Get(t.Context(), "owner", "session", generation)
		if err != nil || !found || snapshot.Phase != protocol.SandboxPolicyReceiptRetired || snapshot.PolicyDigest != fmt.Sprintf("sha256:policy-%d", generation) {
			t.Fatalf("historical generation %d receipt=%#v found=%t error=%v", generation, snapshot, found, err)
		}
	}
	if err := store.UpdatePhase(t.Context(), "owner", "session", 3, protocol.SandboxPolicyReceiptUnknown, "late cleanup failure"); err != nil {
		t.Fatal(err)
	}
	manager = NewManagerWithFactory(factory)
	manager.SetSandboxPolicyReceiptStore(store)
	if _, err := manager.GetOrCreate(t.Context(), "session", options); err == nil || starts != 3 {
		t.Fatalf("restart bypassed actual durable unknown receipt: starts=%d error=%v", starts, err)
	}
}

// connectSandboxReceiptFixture 使用与 DM/Room 相同的完整启动事务。
func connectSandboxReceiptFixture(t *testing.T, manager *Manager, options bridge.Options) {
	t.Helper()
	startup, err := manager.BeginClientStartup(t.Context(), "session", "owner")
	if err != nil {
		t.Fatal(err)
	}
	defer startup.Close()
	if _, err := startup.GetOrCreateWithFactory(t.Context(), options, nil); err != nil {
		t.Fatal(err)
	}
	if err := startup.Connect(t.Context()); err != nil {
		t.Fatal(err)
	}
}
