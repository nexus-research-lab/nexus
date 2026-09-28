//go:build darwin && cgo

// INPUT: 独立宿主子进程、真实 helper/launchd、实例锁与共享 SQLite。
// OUTPUT: 宿主突然退出后从原登记分页回收，不启动第二个任务。
// POS: 原生进程扫描恢复验收，不清理策略/资源 unknown 或推断业务结果。
package runtime

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	bridge "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/supervision"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/infra/desktopinstance"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	sandboxstore "github.com/nexus-research-lab/nexus/internal/storage/sandbox"
	"github.com/pressly/goose/v3"
)

func recoveryScanNativeFixture(t *testing.T, root, helper string) (*Manager, *sandboxstore.Repository, *desktopinstance.Guard) {
	t.Helper()
	guard, err := desktopinstance.Acquire(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { guard.Close() })
	appRoot, err := confinedfs.Open(filepath.Join(root, "app"))
	if err != nil {
		t.Fatal(err)
	}
	defer appRoot.Close()
	jobs, err := appRoot.OpenOrCreateRootNoSymlink("processes", 0700)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { jobs.Close() })
	db, err := sql.Open("sqlite", filepath.Join(root, "app", "processes.db")+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err := goose.Up(db, "../../db/migrations/sqlite"); err != nil {
		t.Fatal(err)
	}
	store := sandboxstore.NewRepository(config.Config{DatabaseDriver: "sqlite"}, db)
	data, err := os.ReadFile(helper)
	if err != nil {
		t.Fatal(err)
	}
	manager := NewManager()
	manager.SetSandboxPolicyReceiptStore(store)
	if err := manager.SetSandboxProcessSupervisor(SandboxProcessSupervisor{Root: jobs, HelperPath: helper, HelperSHA256: fmt.Sprintf("%x", sha256.Sum256(data))}); err != nil {
		t.Fatal(err)
	}
	return manager, store, guard
}

func TestRecoveryScanNativeHostExit(t *testing.T) {
	helper := os.Getenv("NEXUS_SUPERVISION_TEST_HELPER")
	if helper == "" {
		t.Skip("explicit native helper required")
	}
	root := t.TempDir()
	ctx, cancel := context.WithTimeout(t.Context(), 45*time.Second)
	defer cancel()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	child := exec.CommandContext(ctx, executable, "-test.run=^TestRecoveryScanNativeHostFixture$")
	child.Env = append(os.Environ(), "NEXUS_SCAN_RECOVERY_FIXTURE="+root)
	if output, err := child.CombinedOutput(); err != nil {
		t.Fatalf("original host: %v\n%s", err, output)
	}
	manager, store, guard := recoveryScanNativeFixture(t, root, helper)
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_, _ = manager.RecoverPendingSandboxProcesses(cleanup, guard, "", 16)
		_ = manager.Close(cleanup)
	})
	before, found, err := store.LatestProcess(ctx, "owner", "crashed-session")
	if err != nil || !found || before.Phase != protocol.SandboxProcessReleased || before.Registration == nil {
		t.Fatalf("original record=%+v found=%v err=%v", before, found, err)
	}
	inspection, err := exec.CommandContext(ctx, "/bin/launchctl", "print", fmt.Sprintf("gui/%d/%s", before.Intent.OwnerUID, before.Intent.JobLabel)).CombinedOutput()
	if err != nil || !strings.Contains(string(inspection), "state = running") {
		t.Fatalf("original task is not running after host exit: %v", err)
	}
	batch, err := manager.RecoverPendingSandboxProcesses(ctx, guard, "", 16)
	if err != nil || len(batch.Items) != 1 || batch.HasMore {
		t.Fatalf("batch=%+v err=%v", batch, err)
	}
	after := batch.Items[0].Snapshot
	if after.Intent != before.Intent || after.Phase != protocol.SandboxProcessReaped || after.Evidence == nil || after.Evidence.Reason != "coalition_reaped" {
		t.Fatalf("retirement=%+v", after)
	}
	second, err := manager.RecoverPendingSandboxProcesses(ctx, guard, "", 16)
	if err != nil || len(second.Items) != 0 || second.HasMore {
		t.Fatalf("repeated scan=%+v err=%v", second, err)
	}
}

func TestRecoveryScanNativeHostFixture(t *testing.T) {
	root := os.Getenv("NEXUS_SCAN_RECOVERY_FIXTURE")
	if root == "" {
		return
	}
	manager, _, _ := recoveryScanNativeFixture(t, root, os.Getenv("NEXUS_SUPERVISION_TEST_HELPER"))
	var captured bridge.Options
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	_, err := manager.GetOrCreateWithFactory(ctx, "crashed-session", bridge.Options{Runtime: bridge.RuntimeOptions{Kind: bridge.RuntimeNXS}, Env: map[string]string{"NEXUS_RUNTIME_USER_ID": "owner"}}, runtimeFactoryFunc(func(options bridge.Options) Client { captured = options; return &fakeRuntimeClient{} }))
	if err != nil {
		t.Fatal(err)
	}
	launch, err := captured.ProcessSupervision(ctx, supervision.Runtime)
	if err != nil {
		t.Fatal(err)
	}
	process, err := supervision.Start(ctx, launch, supervision.Command{Version: 1, Command: "/bin/sh", Directory: root, Args: []string{"-c", "printf ready; exec /bin/sleep 120"}, Env: []string{"PATH=/usr/bin:/bin"}})
	if process != nil {
		defer process.Close(context.Background())
	}
	if err != nil {
		t.Fatal(err)
	}
	stdout := process.Streams().Stdout
	stdout.SetReadDeadline(time.Now().Add(5 * time.Second))
	marker := make([]byte, 5)
	if _, err := io.ReadFull(stdout, marker); err != nil || string(marker) != "ready" {
		t.Fatalf("readiness=%q err=%v", marker, err)
	}
	// 不运行 defer、Manager.Close 或任何回收回调；任务留给下一个持锁宿主恢复。
	os.Exit(0)
}
