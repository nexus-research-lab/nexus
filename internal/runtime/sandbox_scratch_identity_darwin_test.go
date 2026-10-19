//go:build darwin

// INPUT: 真实宿主 lease、任务可写标记、目录替换及数据库进程意图。
// OUTPUT: 标记不产生目录身份，替换/释放不能继续生成监督启动。
// POS: macOS 可信资源登记测试；不宣称自动回收已经接入。
package runtime

import (
	"os"
	"path/filepath"
	"strings"
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

func TestScratchIdentityPersistsGoldenFormat(t *testing.T) {
	info, err := os.Stat(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	identity, err := sandboxDirectoryIdentity(info)
	if err != nil {
		t.Fatal(err)
	}
	// 持久格式必须保持 darwin-v1:dev:ino:gen:birthSec:birthNSec：
	// 存储校验与旧记录兼容都依赖该形状，稳定字段解析必须能读回。
	if !strings.HasPrefix(identity, "darwin-v1:") || strings.Count(identity, ":") != 5 {
		t.Fatalf("identity format drifted: %s", identity)
	}
	if _, ok := scratchIdentityStableFields(identity); !ok {
		t.Fatalf("generated identity does not parse: %s", identity)
	}
}

// 正常 Release 的 cleanup 绑定核验同样只容忍 device 漂移：数据库保存原身份，
// 绑定身份的 device 被重编后清理照常隔离删除原目录；inode 级差异仍拒绝。
func TestSupervisedLeaseCleanupToleratesDeviceRenumber(t *testing.T) {
	for _, mode := range []string{"device", "inode"} {
		t.Run(mode, func(t *testing.T) {
			h, store, intent := newProcessHostFixture(t)
			lease, err := Acquire(t.Context(), Input{OwnerUserID: "owner", SessionKey: "session", Root: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			proof, err := supervisedScratchBinding(lease)
			if err != nil {
				t.Fatal(err)
			}
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
			drifted := proof
			if mode == "device" {
				drifted.BaseIdentity = driftScratchIdentityDevice(drifted.BaseIdentity, 2)
				drifted.LeafIdentity = driftScratchIdentityDevice(drifted.LeafIdentity, 2)
			} else {
				drifted.LeafIdentity = bumpScratchIdentityInode(drifted.LeafIdentity)
			}
			binding := sandboxProcessBinding{Owner: h.binding.Owner, Session: h.binding.Session, RuntimeKind: "nxs", Generation: h.binding.Generation, LeaseID: h.binding.LeaseID, Scratch: drifted}
			if err := configureSupervisedLeaseCleanup(lease, &SandboxProcessSupervisor{Root: h.root, HelperPath: "/trusted/bootstrap", HelperSHA256: intent.HelperSHA256}, store, binding); err != nil {
				t.Fatal(err)
			}
			leaf := lease.Path()
			releaseErr := lease.Release()
			if mode == "device" {
				if releaseErr != nil {
					t.Fatal(releaseErr)
				}
				if _, err := os.Stat(leaf); !os.IsNotExist(err) {
					t.Fatal("source scratch remains after device-renumber cleanup", err)
				}
				pending, err := store.PendingScratchRecovery(t.Context(), "owner", "session")
				if err != nil || pending {
					t.Fatalf("pending fence after device-renumber cleanup: %v %v", pending, err)
				}
				return
			}
			if releaseErr == nil {
				t.Fatal("stable-field drift accepted by cleanup")
			}
			if _, err := os.Stat(leaf); err != nil {
				t.Fatal("stable-field drift removed source", err)
			}
		})
	}
}
