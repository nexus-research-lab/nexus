// INPUT: 真实 SQLite 启动仓储、受限文件句柄及提交后响应丢失故障。
// OUTPUT: 启动器回调与 exact scope 的绑定、不可重放放行和失败保留栅栏。
// POS: Host 数据库适配验证；临时根仅为夹具，不是生产可信路径或原生回收证据。
package runtime

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/supervision"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	sandboxstore "github.com/nexus-research-lab/nexus/internal/storage/sandbox"
	"github.com/pressly/goose/v3"
)

func newProcessHostFixture(t *testing.T) (*sandboxProcessHost, *sandboxstore.Repository, supervision.Intent) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "process.db")+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	db.SetMaxOpenConns(1)
	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err := goose.Up(db, "../../db/migrations/sqlite"); err != nil {
		t.Fatal(err)
	}
	store := sandboxstore.NewRepository(config.Config{DatabaseDriver: "sqlite"}, db)
	// 短路径只用于测试 Unix socket 长度规则；本测试不执行任务。
	path, err := os.MkdirTemp("/tmp", "nsp-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(path) })
	root, err := confinedfs.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	h, err := newSandboxProcessHost(store, root, sandboxProcessBinding{Owner: "owner", Session: "session", Generation: 1, RuntimeKind: "nxs", LeaseID: "lease"})
	if err != nil {
		t.Fatal(err)
	}
	id := strings.Repeat("a", 32)
	return h, store, supervision.Intent{Version: 1, ID: id, BootID: "12345678-1234-1234-1234-123456789abc", OwnerUID: 501, JobLabel: "cn.nexus.runtime." + id, HelperSHA256: strings.Repeat("b", 64)}
}

func TestSandboxProcessHostExactLifecycle(t *testing.T) {
	h, store, i := newProcessHostFixture(t)
	ctx := t.Context()
	paths, err := h.Reserve(ctx, i)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.Publish(ctx, i, []byte("fixture job")); err != nil {
		t.Fatal(err)
	}
	r := supervision.Registration{Version: 1, BootID: i.BootID, OwnerUID: i.OwnerUID, CoalitionID: 9}
	if err := h.Register(ctx, i, r); err != nil {
		t.Fatal(err)
	}
	if err := h.ClaimRelease(ctx, i); err != nil {
		t.Fatal(err)
	}
	if err := h.ClaimRelease(ctx, i); err == nil {
		t.Fatal("release replay succeeded")
	}
	if err := h.Finish(ctx, i, nil); err == nil {
		t.Fatal("retired without evidence")
	}
	wrong := i
	wrong.HelperSHA256 = strings.Repeat("c", 64)
	if err := h.ClaimRelease(ctx, wrong); err == nil {
		t.Fatal("changed helper accepted")
	}
	proof := supervision.Evidence{Registration: r, Reason: "coalition_reaped", ObservedBootID: i.BootID}
	wrongProof := proof
	wrongProof.Registration.CoalitionID++
	if err := h.Finish(ctx, i, &wrongProof); err == nil {
		t.Fatal("wrong collection accepted")
	}
	if _, err := os.Stat(paths.JobFile); err != nil {
		t.Fatal("rejected evidence removed job", err)
	}
	if err := h.Finish(ctx, i, &proof); err != nil {
		t.Fatal(err)
	}
	if err := h.Finish(ctx, i, &proof); err != nil {
		t.Fatal("terminal reconciliation", err)
	}
	if _, err := os.Stat(filepath.Dir(paths.JobFile)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("launch directory retained", err)
	}
	s, found, err := store.LatestProcess(ctx, "owner", "session")
	if err != nil || !found || s.Phase != protocol.SandboxProcessReaped || s.Intent.LeaseID != "lease" {
		t.Fatalf("snapshot=%+v found=%v err=%v", s, found, err)
	}
	if _, err := h.Reserve(ctx, i); err == nil {
		t.Fatal("terminal reservation reused")
	}
}

type responseLostProcessStore struct {
	SandboxProcessStore
	stage string
}

func (s responseLostProcessStore) PrepareProcess(ctx context.Context, i protocol.SandboxProcessIntent) error {
	err := s.SandboxProcessStore.PrepareProcess(ctx, i)
	if err == nil && s.stage == "prepare" {
		return errors.New("response lost")
	}
	return err
}
func (s responseLostProcessStore) RegisterProcess(ctx context.Context, k protocol.SandboxProcessKey, r protocol.SandboxProcessRegistration) error {
	err := s.SandboxProcessStore.RegisterProcess(ctx, k, r)
	if err == nil && s.stage == "register" {
		return errors.New("response lost")
	}
	return err
}
func (s responseLostProcessStore) ClaimProcessRelease(ctx context.Context, k protocol.SandboxProcessKey) error {
	err := s.SandboxProcessStore.ClaimProcessRelease(ctx, k)
	if err == nil && s.stage == "release" {
		return errors.New("response lost")
	}
	return err
}

func TestSandboxProcessHostReconcilesLostResponses(t *testing.T) {
	for _, stage := range []string{"prepare", "register", "release"} {
		t.Run(stage, func(t *testing.T) {
			h, store, i := newProcessHostFixture(t)
			ctx := t.Context()
			h.store = responseLostProcessStore{store, stage}
			_, err := h.Reserve(ctx, i)
			if stage == "prepare" {
				if err == nil {
					t.Fatal("expected lost response")
				}
				if err := h.Finish(ctx, i, nil); err != nil {
					t.Fatal(err)
				}
				s, _, _ := store.LatestProcess(ctx, "owner", "session")
				if s.Phase != protocol.SandboxProcessAborted {
					t.Fatal(s.Phase)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			r := supervision.Registration{Version: 1, BootID: i.BootID, OwnerUID: i.OwnerUID, CoalitionID: 9}
			err = h.Register(ctx, i, r)
			if stage == "register" {
				if err == nil {
					t.Fatal("expected lost response")
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if err := h.ClaimRelease(ctx, i); err == nil {
					t.Fatal("expected lost response")
				}
			}
			if err := h.Finish(ctx, i, nil); err == nil {
				t.Fatal("cleared uncertain launch without evidence")
			}
			if err := h.Finish(ctx, i, &supervision.Evidence{Registration: r, Reason: "coalition_reaped", ObservedBootID: i.BootID}); err != nil {
				t.Fatal(err)
			}
			s, _, _ := store.LatestProcess(ctx, "owner", "session")
			if s.Phase != protocol.SandboxProcessReaped {
				t.Fatal(s.Phase)
			}
		})
	}
}

func TestSandboxProcessHostRejectsUnsafePaths(t *testing.T) {
	h, store, i := newProcessHostFixture(t)
	ctx := t.Context()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "sentinel"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(h.root.Name(), "p")); err != nil {
		t.Fatal(err)
	}
	if _, err := h.Reserve(ctx, i); err == nil {
		t.Fatal("symlink accepted")
	}
	if err := h.Finish(ctx, i, nil); err == nil {
		t.Fatal("unsafe cleanup cleared startup fence")
	}
	s, _, _ := store.LatestProcess(ctx, "owner", "session")
	if s.Phase != protocol.SandboxProcessPrepared {
		t.Fatal(s.Phase)
	}
	if _, err := os.Stat(filepath.Join(outside, "sentinel")); err != nil {
		t.Fatal(err)
	}
}

func TestSandboxProcessHostRejectsLongSocketBeforeReservation(t *testing.T) {
	h, store, i := newProcessHostFixture(t)
	child, err := h.root.OpenOrCreateRootNoSymlink(strings.Repeat("x", 100), 0700)
	if err != nil {
		t.Fatal(err)
	}
	defer child.Close()
	h.root = child
	if _, err := h.Reserve(t.Context(), i); err == nil {
		t.Fatal("long socket accepted")
	}
	_, found, err := store.LatestProcess(t.Context(), "owner", "session")
	if err != nil || found {
		t.Fatalf("failed local validation left a reservation: %v %v", found, err)
	}
}
