//go:build darwin

// INPUT: 真实实例锁、SQLite 原启动记录、受保护目录与原生恢复替身。
// OUTPUT: 锁缺失/失效/跨根/目录替换均先于回收拒绝，成功只收口原记录。
// POS: 宿主所有权与恢复入口的集成；实际后代回收另由 Bridge 原生测试验证。
package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/supervision"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/infra/desktopinstance"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestSandboxProcessRecoveryRequiresLiveMatchingOwnership(t *testing.T) {
	h, store, intent := newProcessHostFixture(t)
	stateRoot := t.TempDir()
	guard, err := desktopinstance.Acquire(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer guard.Close()
	rootPath := filepath.Join(stateRoot, "app", "processes")
	if err := os.Mkdir(rootPath, 0700); err != nil {
		t.Fatal(err)
	}
	root, err := confinedfs.Open(rootPath)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	h.root = root
	m := NewManager()
	m.SetSandboxPolicyReceiptStore(store)
	if err := m.SetSandboxProcessSupervisor(SandboxProcessSupervisor{Root: root, HelperPath: "/trusted/bootstrap", HelperSHA256: intent.HelperSHA256}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Reserve(t.Context(), intent); err != nil {
		t.Fatal(err)
	}
	calls := 0
	native := func(ctx context.Context, record supervision.Recovery, host supervision.RecoveryHost) error {
		calls++
		return host.Finish(ctx, record.Intent, nil)
	}
	key := h.intent.Key
	assertRejected := func(ownership SandboxProcessRecoveryOwnership) {
		t.Helper()
		if _, err := m.recoverSandboxProcess(t.Context(), key, ownership, native); err == nil || calls != 0 {
			t.Fatalf("invalid ownership reached native: %v calls=%d", err, calls)
		}
		snapshot, _, err := store.Process(t.Context(), key)
		if err != nil || snapshot.Phase != protocol.SandboxProcessPrepared {
			t.Fatalf("rejection changed durable fence: %+v %v", snapshot, err)
		}
	}
	assertRejected(nil)
	other, err := desktopinstance.Acquire(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	assertRejected(other)
	other.Close()
	assertRejected(other)
	// A replacement directory at the same pathname must not inherit the old handle.
	if err := os.Rename(rootPath, rootPath+"-original"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(rootPath, 0700); err != nil {
		t.Fatal(err)
	}
	assertRejected(guard)
	if err := os.Remove(rootPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(rootPath+"-original", rootPath); err != nil {
		t.Fatal(err)
	}
	// Replacing the lock inode invalidates ownership even though the old fd is locked.
	lockPath := filepath.Join(stateRoot, "app", "sidecar.lock")
	if err := os.Rename(lockPath, lockPath+"-original"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lockPath, nil, 0600); err != nil {
		t.Fatal(err)
	}
	assertRejected(guard)
	if err := os.Remove(lockPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(lockPath+"-original", lockPath); err != nil {
		t.Fatal(err)
	}
	result, err := m.recoverSandboxProcess(t.Context(), key, guard, native)
	if err != nil || calls != 1 || result.Phase != protocol.SandboxProcessAborted {
		t.Fatalf("recovery=%+v calls=%d err=%v", result, calls, err)
	}
}
