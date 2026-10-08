//go:build darwin

// INPUT: 真实宿主 lease、任务可写标记、目录替换及数据库进程意图。
// OUTPUT: 标记不产生目录身份，替换/释放不能继续生成监督启动。
// POS: macOS 可信资源登记测试；不宣称自动回收已经接入。
package runtime

import (
	"os"
	"path/filepath"
	"testing"

	bridge "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/supervision"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestSupervisedScratchIdentityUsesOriginalDirectory(t *testing.T) {
	for _, kind := range []string{"leaf", "parent"} {
		t.Run(kind, func(t *testing.T) {
			lease, err := Acquire(t.Context(), Input{OwnerUserID: "owner", SessionKey: "session", Root: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := lease.Release(); err != nil {
					t.Error(err)
				}
			}()
			original, err := supervisedScratchBinding(lease)
			if err != nil {
				t.Fatal(err)
			}
			if original.BaseIdentity == "" || original.LeafIdentity == "" {
				t.Fatal("missing filesystem proof")
			}
			// The writable marker cannot select which filesystem object the host trusts.
			if err := os.WriteFile(filepath.Join(lease.Path(), leaseMarkerName), []byte(`{"lease_id":"forged"}`), 0600); err != nil {
				t.Fatal(err)
			}
			after, err := supervisedScratchBinding(lease)
			if err != nil || after != original {
				t.Fatalf("marker changed host identity: %+v %v", after, err)
			}
			path := lease.Path()
			if kind == "parent" {
				path = filepath.Dir(path)
			}
			moved := path + "-original"
			if err := os.Rename(path, moved); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := os.RemoveAll(path); err != nil {
					t.Error(err)
				}
				if err := os.Rename(moved, path); err != nil {
					t.Error(err)
				}
			}()
			if err := os.Mkdir(path, 0700); err != nil {
				t.Fatal(err)
			}
			if kind == "parent" {
				if err := os.Mkdir(filepath.Join(path, original.LeafName), 0700); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := supervisedScratchBinding(lease); err == nil {
				t.Fatal("adopted replacement directory")
			}
		})
	}
}

func TestSupervisedScratchIntentPersistsAndRechecksIdentity(t *testing.T) {
	h, store, intent := newProcessHostFixture(t)
	manager := NewManager()
	manager.SetSandboxPolicyReceiptStore(store)
	if err := manager.SetSandboxProcessSupervisor(SandboxProcessSupervisor{Root: h.root, HelperPath: "/trusted/bootstrap", HelperSHA256: intent.HelperSHA256}); err != nil {
		t.Fatal(err)
	}
	lease, err := Acquire(t.Context(), Input{OwnerUserID: "owner", SessionKey: "session", Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release()
	expected, err := supervisedScratchBinding(lease)
	if err != nil {
		t.Fatal(err)
	}
	opts := bridge.Options{Sandbox: &bridge.SandboxSettings{Resources: lease.Resources()}}
	options, err := manager.supervisedProcessOptions(opts, "owner", "session", 0, lease)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := options.ProcessSupervision(t.Context(), supervision.VersionProbe)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cfg.Host.Reserve(t.Context(), intent); err != nil {
		t.Fatal(err)
	}
	got, found, err := store.LatestProcess(t.Context(), "owner", "session")
	if err != nil || !found || got.Intent.Scratch != expected || got.Intent.LeaseID != lease.Marker().LeaseID {
		t.Fatalf("intent=%+v found=%v err=%v", got, found, err)
	}
	if err := cfg.Host.Finish(t.Context(), intent, nil); err != nil {
		t.Fatal(err)
	}
	// The factory must recheck the original identity before every later purpose.
	leaf := lease.Path()
	moved := leaf + "-original"
	if err := os.Rename(leaf, moved); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.RemoveAll(leaf); err != nil {
			t.Error(err)
		}
		if err := os.Rename(moved, leaf); err != nil {
			t.Error(err)
		}
	}()
	if err := os.Mkdir(leaf, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := options.ProcessSupervision(t.Context(), supervision.Runtime); err == nil {
		t.Fatal("launched runtime after probe scratch replacement")
	}
	got, found, err = store.LatestProcess(t.Context(), "owner", "session")
	if err != nil || !found || got.Phase != protocol.SandboxProcessAborted || got.Intent.Purpose != protocol.SandboxProcessVersionProbe {
		t.Fatalf("replacement produced launch: %+v %v", got, err)
	}
}

func TestSupervisedScratchIdentityRejectsReleasedLease(t *testing.T) {
	lease, err := Acquire(t.Context(), Input{OwnerUserID: "owner", SessionKey: "session", Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if err := lease.Release(); err != nil {
		t.Fatal(err)
	}
	if _, err := supervisedScratchBinding(lease); err == nil {
		t.Fatal("released lease produced proof")
	}
}
