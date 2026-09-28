//go:build darwin

// INPUT: 真实实例锁/SQLite/目录与提交前后故障注入。
// OUTPUT: 原目录隔离删除、清理中断可续、替换保留与 pending 准入栅栏。
// POS: 资源恢复集成；测试中的进程回收事实是注入值，不冒充原生回收。
package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	bridge "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/supervision"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/infra/desktopinstance"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	sandboxstore "github.com/nexus-research-lab/nexus/internal/storage/sandbox"
)

type scratchFaultStore struct {
	*sandboxstore.Repository
	phase protocol.SandboxScratchRecoveryPhase
	after bool
	fired bool
}

func (s *scratchFaultStore) AdvanceScratchRecovery(ctx context.Context, r protocol.SandboxScratchRecovery, next protocol.SandboxScratchRecoveryPhase) error {
	if !s.fired && next == s.phase {
		s.fired = true
		if s.after {
			if err := s.Repository.AdvanceScratchRecovery(ctx, r, next); err != nil {
				return err
			}
		}
		return errors.New("injected cleanup commit response failure")
	}
	return s.Repository.AdvanceScratchRecovery(ctx, r, next)
}

type scratchRecoveryFixture struct {
	manager *Manager
	store   *sandboxstore.Repository
	guard   *desktopinstance.Guard
	key     protocol.SandboxProcessKey
	lease   *SandboxResourceLease
	root    *confinedfs.Root
	path    string
}

func newScratchRecoveryFixture(t *testing.T) scratchRecoveryFixture {
	t.Helper()
	h, store, intent := newProcessHostFixture(t)
	statePath, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	guard, err := desktopinstance.Acquire(statePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { guard.Close() })
	jobs := filepath.Join(statePath, "app", "processes")
	if err := os.Mkdir(jobs, 0700); err != nil {
		t.Fatal(err)
	}
	root, err := confinedfs.Open(jobs)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	runtimePath := filepath.Join(statePath, "users", "owner", "runtime")
	if err := os.MkdirAll(runtimePath, 0700); err != nil {
		t.Fatal(err)
	}
	lease, err := Acquire(t.Context(), Input{OwnerUserID: "owner", SessionKey: "session", Root: runtimePath})
	if err != nil {
		t.Fatal(err)
	}
	// Simulate losing the old host registry without running its cleanup defer.
	// Actual process-exit/native recovery remains separately tested.
	t.Cleanup(func() { forgetScratchLeaseForRecoveryTest(lease) })
	proof, err := supervisedScratchBinding(lease)
	if err != nil {
		t.Fatal(err)
	}
	h.root = root
	h.binding.Scratch = proof
	h.binding.LeaseID = lease.Marker().LeaseID
	if _, err := h.Reserve(t.Context(), intent); err != nil {
		t.Fatal(err)
	}
	registration := supervision.Registration{Version: 1, BootID: intent.BootID, OwnerUID: intent.OwnerUID, CoalitionID: 19}
	if err := h.Register(t.Context(), intent, registration); err != nil {
		t.Fatal(err)
	}
	if err := h.ClaimRelease(t.Context(), intent); err != nil {
		t.Fatal(err)
	}
	if err := h.Finish(t.Context(), intent, &supervision.Evidence{Registration: registration, Reason: "coalition_reaped", ObservedBootID: intent.BootID}); err != nil {
		t.Fatal(err)
	}
	manager := NewManager()
	manager.SetSandboxPolicyReceiptStore(store)
	if err := manager.SetSandboxProcessSupervisor(SandboxProcessSupervisor{Root: root, HelperPath: "/trusted/bootstrap", HelperSHA256: intent.HelperSHA256}); err != nil {
		t.Fatal(err)
	}
	return scratchRecoveryFixture{manager: manager, store: store, guard: guard, key: h.intent.Key, lease: lease, root: root, path: lease.Path()}
}

func forgetScratchLeaseForRecoveryTest(lease *SandboxResourceLease) {
	lease.mu.Lock()
	defer lease.mu.Unlock()
	resource := lease.resource
	if resource == nil {
		return
	}
	resource.mu.Lock()
	defer resource.mu.Unlock()
	registryMu.Lock()
	delete(registry, resource.path)
	delete(byScope, resource.scopeKey)
	registryMu.Unlock()
	if resource.base != nil {
		resource.base.Close()
		resource.base = nil
	}
	lease.resource = nil
	lease.released = true
}

func TestScratchRecoveryResumesEveryCommitBoundary(t *testing.T) {
	for _, phase := range []protocol.SandboxScratchRecoveryPhase{protocol.SandboxScratchRecoveryQuarantined, protocol.SandboxScratchRecoveryDeleting, protocol.SandboxScratchRecoveryComplete} {
		for _, after := range []bool{false, true} {
			name := string(phase) + "/before"
			if after {
				name = string(phase) + "/after"
			}
			t.Run(name, func(t *testing.T) {
				f := newScratchRecoveryFixture(t)
				if err := f.lease.MarkCleanupUncertain(errors.New("old host cleanup unknown")); err != nil {
					t.Fatal(err)
				}
				forgetScratchLeaseForRecoveryTest(f.lease)
				fault := &scratchFaultStore{Repository: f.store, phase: phase, after: after}
				f.manager.SetSandboxPolicyReceiptStore(fault)
				if _, err := f.manager.RecoverSandboxScratch(t.Context(), f.key, f.guard); err == nil || !fault.fired {
					t.Fatalf("fault not reached: %v", err)
				}
				pending, err := f.store.PendingScratchRecovery(t.Context(), "owner", "session")
				if err != nil {
					t.Fatal(err)
				}
				if !(after && phase == protocol.SandboxScratchRecoveryComplete) && !pending {
					t.Fatal("failed cleanup cleared startup fence")
				}
				result, err := f.manager.RecoverSandboxScratch(t.Context(), f.key, f.guard)
				if err != nil || result.Phase != protocol.SandboxScratchRecoveryComplete {
					t.Fatalf("resume=%+v %v", result, err)
				}
				if _, err := os.Stat(f.path); !os.IsNotExist(err) {
					t.Fatal("source scratch remains", err)
				}
				if _, err := f.root.Lstat("scratch-recovery/s-" + result.LeaseID); !os.IsNotExist(err) {
					t.Fatal("quarantined scratch remains", err)
				}
				pending, err = f.store.PendingScratchRecovery(t.Context(), "owner", "session")
				if err != nil || pending {
					t.Fatalf("completion fence: %v %v", pending, err)
				}
				// Idempotent completion never deletes a subsequently-created source path.
				if err := os.Mkdir(f.path, 0700); err != nil {
					t.Fatal(err)
				}
				if _, err := f.manager.RecoverSandboxScratch(t.Context(), f.key, f.guard); err != nil {
					t.Fatal(err)
				}
				if _, err := os.Stat(f.path); err != nil {
					t.Fatal("repeat deleted replacement", err)
				}
			})
		}
	}
}

func TestScratchRecoveryRejectsLiveLeaseAndMissingOrReplacedSource(t *testing.T) {
	for _, mode := range []string{"live", "missing", "replaced", "wrong_lock"} {
		t.Run(mode, func(t *testing.T) {
			f := newScratchRecoveryFixture(t)
			if mode != "live" {
				forgetScratchLeaseForRecoveryTest(f.lease)
			}
			if mode == "missing" || mode == "replaced" {
				if err := os.Rename(f.path, f.path+"-original"); err != nil {
					t.Fatal(err)
				}
				if mode == "replaced" {
					if err := os.Mkdir(f.path, 0700); err != nil {
						t.Fatal(err)
					}
				}
			}
			ownership := SandboxProcessRecoveryOwnership(f.guard)
			if mode == "wrong_lock" {
				g, err := desktopinstance.Acquire(t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
				defer g.Close()
				ownership = g
			}
			if _, err := f.manager.RecoverSandboxScratch(t.Context(), f.key, ownership); err == nil {
				t.Fatal("unsafe cleanup succeeded")
			}
			if mode == "live" || mode == "wrong_lock" {
				pending, err := f.store.PendingScratchRecovery(t.Context(), "owner", "session")
				if err != nil || pending {
					t.Fatalf("unauthorized attempt mutated ledger: %v %v", pending, err)
				}
			}
			if mode == "replaced" {
				if _, err := os.Stat(f.path); err != nil {
					t.Fatal("replacement removed")
				}
			}
			if mode == "missing" || mode == "replaced" {
				calls := 0
				f.manager.factory = runtimeFactoryFunc(func(bridge.Options) Client { calls++; return &fakeRuntimeClient{} })
				if _, err := f.manager.GetOrCreate(t.Context(), "session", bridge.Options{Env: map[string]string{"NEXUS_RUNTIME_USER_ID": "owner"}}); !errors.Is(err, ErrSandboxCleanupPending) || calls != 0 {
					t.Fatalf("pending cleanup admitted runtime: %v calls=%d", err, calls)
				}
			}
		})
	}
}

func TestScratchRecoveryDoesNotGuessMissingOrReplacedQuarantine(t *testing.T) {
	for _, mode := range []string{"missing", "replaced"} {
		t.Run(mode, func(t *testing.T) {
			f := newScratchRecoveryFixture(t)
			forgetScratchLeaseForRecoveryTest(f.lease)
			fault := &scratchFaultStore{Repository: f.store, phase: protocol.SandboxScratchRecoveryDeleting}
			f.manager.SetSandboxPolicyReceiptStore(fault)
			result, err := f.manager.RecoverSandboxScratch(t.Context(), f.key, f.guard)
			if err == nil {
				t.Fatal("expected injected interruption")
			}
			target := "scratch-recovery/s-" + result.LeaseID
			if err := f.root.Rename(target, target+"-original"); err != nil {
				t.Fatal(err)
			}
			if mode == "replaced" {
				if err := f.root.Mkdir(target, 0700); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := f.manager.RecoverSandboxScratch(t.Context(), f.key, f.guard); err == nil {
				t.Fatal("guessed quarantined directory was deleted")
			}
			pending, err := f.store.PendingScratchRecovery(t.Context(), "owner", "session")
			if err != nil || !pending {
				t.Fatal("cleared unknown quarantine fence", err)
			}
			if mode == "replaced" {
				if _, err := f.root.Stat(target); err != nil {
					t.Fatal("deleted replacement", err)
				}
			}
		})
	}
}

func TestScratchAndPolicyRecoveryReconcilesOnlyOriginalBoundReceipts(t *testing.T) {
	f := newScratchRecoveryFixture(t)
	process, found, err := f.store.Process(t.Context(), f.key)
	if err != nil || !found {
		t.Fatal(err)
	}
	for generation := uint64(1); generation <= 4; generation++ {
		receipt := testSandboxReceiptSnapshotForManager()
		receipt.OwnerUserID = "owner"
		receipt.SessionKey = "session"
		receipt.Generation = generation
		receipt.RuntimeKind = "nxs"
		receipt.LeaseID = process.Intent.LeaseID
		receipt.Phase = protocol.SandboxPolicyReceiptUnknown
		receipt.UnknownReason = "old cleanup failed"
		if generation < 4 {
			receipt.ProcessKey = &f.key
		}
		if err := f.store.Save(t.Context(), receipt); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.manager.ReconcileSandboxPolicy(t.Context(), f.key, f.guard); err == nil {
		t.Fatal("reconciled policy before scratch cleanup")
	}
	forgetScratchLeaseForRecoveryTest(f.lease)
	if _, err := f.manager.RecoverSandboxScratch(t.Context(), f.key, f.guard); err != nil {
		t.Fatal(err)
	}
	count, err := f.manager.ReconcileSandboxPolicy(t.Context(), f.key, f.guard)
	if err != nil || count != 3 {
		t.Fatalf("reconcile=%d %v", count, err)
	}
	for generation := uint64(1); generation <= 4; generation++ {
		receipt, found, err := f.store.Get(t.Context(), "owner", "session", generation)
		if err != nil || !found {
			t.Fatal(err)
		}
		phase := protocol.SandboxPolicyReceiptReconciled
		if generation == 4 {
			phase = protocol.SandboxPolicyReceiptUnknown
		}
		if receipt.Phase != phase || receipt.UnknownReason != "old cleanup failed" {
			t.Fatalf("lost scope or audit: %+v", receipt)
		}
	}
	if count, err := f.manager.ReconcileSandboxPolicy(t.Context(), f.key, f.guard); err != nil || count != 0 {
		t.Fatalf("repeated reconciliation: %d %v", count, err)
	}
	if _, err := f.manager.sandboxStartupGeneration(t.Context(), "owner", "session"); !errors.Is(err, ErrSandboxCleanupPending) {
		t.Fatalf("unbound historical unknown was ignored: %v", err)
	}
}
