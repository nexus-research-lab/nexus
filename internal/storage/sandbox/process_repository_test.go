// INPUT: 真实 SQLite 连接、exact 启动身份与并发放行领取。
// OUTPUT: 跨重启身份保留、单次领取、活跃范围唯一和迟到/跨 scope 写入拒绝。
// POS: 持久启动事实测试，不伪造原生执行或回收验收。
package sandbox

import (
	"database/sql"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func processIntent() protocol.SandboxProcessIntent {
	id := strings.Repeat("a", 32)
	return protocol.SandboxProcessIntent{Version: 1, Key: protocol.SandboxProcessKey{OwnerUserID: "owner", SessionKey: "session", Generation: 4, LaunchID: id}, RuntimeKind: "nxs", LeaseID: "lease", BootID: "12345678-1234-1234-1234-123456789abc", OwnerUID: 501, JobLabel: "cn.nexus.runtime." + id, HelperSHA256: strings.Repeat("b", 64)}
}
func processRegistration(i protocol.SandboxProcessIntent) protocol.SandboxProcessRegistration {
	return protocol.SandboxProcessRegistration{Version: 1, BootID: i.BootID, CoalitionID: 91, OwnerUID: i.OwnerUID}
}

func TestProcessRegistryOnceOnlyReleaseSurvivesReopen(t *testing.T) {
	r := newSandboxReceiptRepository(t)
	i := processIntent()
	ctx := t.Context()
	if err := r.PrepareProcess(ctx, i); err != nil {
		t.Fatal(err)
	}
	if err := r.PrepareProcess(ctx, i); err != nil {
		t.Fatalf("prepared retry: %v", err)
	}
	changed := i
	changed.HelperSHA256 = strings.Repeat("c", 64)
	if err := r.PrepareProcess(ctx, changed); !errors.Is(err, ErrProcessConflict) {
		t.Fatalf("intent rewrite: %v", err)
	}
	if err := r.ClaimProcessRelease(ctx, i.Key); !errors.Is(err, ErrProcessConflict) {
		t.Fatalf("release before registration: %v", err)
	}
	registration := processRegistration(i)
	if err := r.RegisterProcess(ctx, i.Key, registration); err != nil {
		t.Fatal(err)
	}
	if err := r.RegisterProcess(ctx, i.Key, registration); err != nil {
		t.Fatalf("registration retry: %v", err)
	}
	r.db.SetMaxOpenConns(8)
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for j := 0; j < 8; j++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- r.ClaimProcessRelease(ctx, i.Key) }()
	}
	wg.Wait()
	close(results)
	granted := 0
	for err := range results {
		if err == nil {
			granted++
		} else if !errors.Is(err, ErrProcessConflict) {
			t.Fatal(err)
		}
	}
	if granted != 1 {
		t.Fatalf("release grants=%d", granted)
	}
	var seq int
	var name, path string
	if err := r.db.QueryRow("PRAGMA database_list").Scan(&seq, &name, &path); err != nil {
		t.Fatal(err)
	}
	if err := r.db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_txlock=immediate")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	r = NewRepository(config.Config{DatabaseDriver: "sqlite"}, db)
	snapshot, found, err := r.LatestProcess(ctx, i.Key.OwnerUserID, i.Key.SessionKey)
	if err != nil || !found || snapshot.Phase != protocol.SandboxProcessReleased || snapshot.Intent != i || snapshot.Registration == nil || *snapshot.Registration != registration {
		t.Fatalf("reopened=%#v %v %v", snapshot, found, err)
	}
	if err := r.ClaimProcessRelease(ctx, i.Key); !errors.Is(err, ErrProcessConflict) {
		t.Fatalf("replayed release: %v", err)
	}
	if err := r.AbortPreparedProcess(ctx, i.Key); !errors.Is(err, ErrProcessConflict) {
		t.Fatalf("aborted released process: %v", err)
	}
	proof := protocol.SandboxProcessEvidence{Registration: registration, Reason: "coalition_reaped", ObservedBootID: i.BootID}
	if err := r.ReapProcess(ctx, i.Key, proof); err != nil {
		t.Fatal(err)
	}
	if err := r.ReapProcess(ctx, i.Key, proof); err != nil {
		t.Fatalf("proof retry: %v", err)
	}
	if err := r.RegisterProcess(ctx, i.Key, registration); !errors.Is(err, ErrProcessConflict) {
		t.Fatalf("late registration: %v", err)
	}
	if err := r.ClaimProcessRelease(ctx, i.Key); !errors.Is(err, ErrProcessConflict) {
		t.Fatalf("late release: %v", err)
	}
	if err := r.PrepareProcess(ctx, i); !errors.Is(err, ErrProcessConflict) {
		t.Fatalf("resurrected intent: %v", err)
	}
}

func TestProcessRegistryActiveScopeAndGenerationAreExclusive(t *testing.T) {
	r := newSandboxReceiptRepository(t)
	i := processIntent()
	ctx := t.Context()
	if err := r.PrepareProcess(ctx, i); err != nil {
		t.Fatal(err)
	}
	next := i
	next.Key.Generation++
	next.Key.LaunchID = strings.Repeat("c", 32)
	next.JobLabel = "cn.nexus.runtime." + next.Key.LaunchID
	if err := r.PrepareProcess(ctx, next); !errors.Is(err, ErrProcessConflict) {
		t.Fatalf("parallel generation: %v", err)
	}
	other := next
	other.Key.OwnerUserID = "other"
	if err := r.PrepareProcess(ctx, other); err != nil {
		t.Fatalf("independent owner: %v", err)
	}
	if err := r.AbortPreparedProcess(ctx, i.Key); err != nil {
		t.Fatal(err)
	}
	if err := r.RegisterProcess(ctx, i.Key, processRegistration(i)); !errors.Is(err, ErrProcessConflict) {
		t.Fatalf("late registration after abort: %v", err)
	}
	next.Key.LaunchID = strings.Repeat("d", 32)
	next.JobLabel = "cn.nexus.runtime." + next.Key.LaunchID
	if err := r.PrepareProcess(ctx, next); err != nil {
		t.Fatal(err)
	}
	if err := r.AbortPreparedProcess(ctx, next.Key); err != nil {
		t.Fatal(err)
	}
	old := next
	old.Key.Generation = 3
	old.Key.LaunchID = strings.Repeat("e", 32)
	old.JobLabel = "cn.nexus.runtime." + old.Key.LaunchID
	if err := r.PrepareProcess(ctx, old); !errors.Is(err, ErrProcessConflict) {
		t.Fatalf("generation regressed: %v", err)
	}
}

func TestProcessRegistryRejectsWrongScopeAndFalseProof(t *testing.T) {
	r := newSandboxReceiptRepository(t)
	i := processIntent()
	ctx := t.Context()
	if err := r.PrepareProcess(ctx, i); err != nil {
		t.Fatal(err)
	}
	registration := processRegistration(i)
	wrong := registration
	wrong.OwnerUID++
	if err := r.RegisterProcess(ctx, i.Key, wrong); !errors.Is(err, ErrInvalidProcess) {
		t.Fatalf("owner drift: %v", err)
	}
	if err := r.RegisterProcess(ctx, i.Key, registration); err != nil {
		t.Fatal(err)
	}
	key := i.Key
	key.OwnerUserID = "other"
	if err := r.ClaimProcessRelease(ctx, key); !errors.Is(err, ErrProcessConflict) {
		t.Fatalf("cross owner release: %v", err)
	}
	proof := protocol.SandboxProcessEvidence{Registration: registration, ObservedBootID: i.BootID, Reason: "coalition_reaped"}
	for _, name := range []string{"root_exit", "boot_same", "wrong_coalition", "wrong_boot"} {
		t.Run(name, func(t *testing.T) {
			p := proof
			switch name {
			case "root_exit":
				p.Reason = "root_exit"
			case "boot_same":
				p.Reason = "boot_changed"
			case "wrong_coalition":
				p.Registration.CoalitionID++
			case "wrong_boot":
				p.ObservedBootID = "22345678-1234-1234-1234-123456789abc"
			}
			if err := r.ReapProcess(ctx, i.Key, p); !errors.Is(err, ErrInvalidProcess) {
				t.Fatalf("false proof: %v", err)
			}
		})
	}
	snapshot, found, err := r.Process(ctx, i.Key)
	if err != nil || !found || snapshot.Phase != protocol.SandboxProcessRegistered {
		t.Fatalf("invalid evidence changed phase: %#v %v", snapshot, err)
	}
}

func TestProcessRegistryCorruptBindingFailsClosed(t *testing.T) {
	r := newSandboxReceiptRepository(t)
	i := processIntent()
	ctx := t.Context()
	if err := r.PrepareProcess(ctx, i); err != nil {
		t.Fatal(err)
	}
	if _, err := r.db.Exec(`UPDATE sandbox_process_launches SET generation=generation+1`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.LatestProcess(ctx, i.Key.OwnerUserID, i.Key.SessionKey); !errors.Is(err, ErrInvalidProcess) {
		t.Fatalf("corrupt key accepted: %v", err)
	}
}
