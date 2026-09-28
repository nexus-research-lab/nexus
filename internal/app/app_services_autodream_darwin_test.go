//go:build darwin

// INPUT: 固定真实 nxs/helper、独立 SQLite/owner/scratch 与显式监督配置。
// OUTPUT: AutoDream control 往返、跨次 generation/LeaseID 和进程/策略双终态。
// POS: 原生生命周期集成；AutoDream gate 关闭，不发模型请求或证明记忆整理。
package app

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	bridge "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	permission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/infra/desktopinstance"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtime "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/runtime/clientopts"
	sandboxstore "github.com/nexus-research-lab/nexus/internal/storage/sandbox"
	"github.com/pressly/goose/v3"
)

func TestAppManagedAutoDreamSupervisedNative(t *testing.T) {
	binary, helper := os.Getenv("NEXUS_SANDBOX_TEST_BINARY"), os.Getenv("NEXUS_SUPERVISION_TEST_HELPER")
	if binary == "" || helper == "" {
		t.Skip("explicit fixed nxs and supervision helper required")
	}
	data, err := os.ReadFile(helper)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	guard, err := desktopinstance.Acquire(root)
	if err != nil {
		t.Fatal(err)
	}
	defer guard.Close()
	t.Setenv("NEXUS_STATE_ROOT", root)
	for _, directory := range []string{"workspace", "home", "config", "runtime"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0700); err != nil {
			t.Fatal(err)
		}
	}
	jobs := filepath.Join(root, "app", strings.Repeat("long-private-state-", 8))
	if err := os.MkdirAll(jobs, 0700); err != nil {
		t.Fatal(err)
	}
	if len(jobs) <= 103 {
		t.Fatal("fixture must exceed Unix socket address length")
	}
	jobRoot, err := confinedfs.Open(jobs)
	if err != nil {
		t.Fatal(err)
	}
	defer jobRoot.Close()
	db, err := sql.Open("sqlite", filepath.Join(root, "app.db")+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err := goose.Up(db, "../../db/migrations/sqlite"); err != nil {
		t.Fatal(err)
	}
	store := sandboxstore.NewRepository(config.Config{DatabaseDriver: "sqlite"}, db)
	manager := runtime.NewManager()
	manager.SetSandboxPolicyReceiptStore(store)
	if err := manager.SetSandboxProcessSupervisor(runtime.SandboxProcessSupervisor{Root: jobRoot, HelperPath: helper, HelperSHA256: fmt.Sprintf("%x", sha256.Sum256(data))}); err != nil {
		t.Fatal(err)
	}
	defer manager.Close(context.Background())
	ctx, cancel := context.WithTimeout(t.Context(), 60*time.Second)
	defer cancel()
	const key = "memory-maintenance:agent"
	for generation := uint64(1); generation <= 2; generation++ {
		lease, err := runtime.AcquireSandboxResource(ctx, runtime.SandboxResourceInput{OwnerUserID: "owner", SessionKey: key, Root: filepath.Join(root, "runtime")})
		if err != nil {
			t.Fatal(err)
		}
		defer lease.Release()
		leaseID, scratchPath := lease.Marker().LeaseID, lease.Path()
		options, err := clientopts.BuildAgentClientOptions(ctx, nil, clientopts.AgentClientOptionsInput{
			OwnerUserID: "owner", WorkspacePath: filepath.Join(root, "workspace"), RuntimeKind: "nxs", AppMode: "desktop",
			PermissionMode: permission.ModeDefault, AutoMemoryDisabled: true, AutoDreamDisabled: true,
			SettingSources: []string{"user"}, SandboxResources: lease.Resources(),
		})
		if err != nil {
			t.Fatal(err)
		}
		options.CLIPath = binary
		options.Env["HOME"] = filepath.Join(root, "home")
		options.Env["NEXUS_CONFIG_DIR"] = filepath.Join(root, "config")
		options.Env["CLAUDE_CONFIG_DIR"] = filepath.Join(root, "config")
		options.Env["TMPDIR"] = scratchPath
		options.Skills = bridge.SkillOptions{Mode: bridge.SkillModeNone}
		options.MCP.StrictConfig = true
		options.Runtime.InitializeTimeout = 15 * time.Second
		result, consumed, err := manager.TryAutoDream(ctx, key, options, lease)
		if err != nil || !consumed || result.Status != bridge.AutoDreamStatusSkipped {
			t.Fatalf("result=%+v consumed=%v err=%v", result, consumed, err)
		}
		if err := manager.WaitBackgroundTasks(ctx, key); err != nil {
			t.Fatal(err)
		}
		process, found, err := store.LatestProcess(ctx, "owner", key)
		if err != nil || !found || process.Intent.Key.Generation != generation || process.Intent.LeaseID != leaseID || process.Phase != protocol.SandboxProcessReaped || process.Intent.Scratch.BaseIdentity == "" || process.Intent.Scratch.LeafIdentity == "" || filepath.Join(process.Intent.Scratch.BasePath, process.Intent.Scratch.LeafName) != scratchPath {
			t.Fatalf("process=%+v found=%v err=%v", process, found, err)
		}
		policy, found, err := store.Latest(ctx, "owner", key)
		if err != nil || !found || policy.Generation != generation || policy.Phase != protocol.SandboxPolicyReceiptRetired || policy.ProcessKey == nil || *policy.ProcessKey != process.Intent.Key {
			t.Fatalf("policy=%+v found=%v err=%v", policy, found, err)
		}
		if _, err := os.Stat(scratchPath); !os.IsNotExist(err) {
			t.Fatalf("scratch remains: %v", err)
		}
		recovery, found, err := store.ScratchRecovery(ctx, "owner", key, leaseID)
		if err != nil || !found || recovery.Phase != protocol.SandboxScratchRecoveryComplete {
			t.Fatalf("normal cleanup lacks durable completion: %+v %v", recovery, err)
		}
		batch, err := manager.RecoverPendingSandboxLifecycles(ctx, guard, "", 16)
		if err != nil || batch.HasMore {
			t.Fatalf("clean lifecycle scan=%+v %v", batch, err)
		}
		empty, err := manager.RecoverPendingSandboxLifecycles(ctx, guard, "", 16)
		if err != nil || len(empty.Items) != 0 {
			t.Fatalf("clean lifecycle remained pending: %+v %v", empty, err)
		}
	}
}
