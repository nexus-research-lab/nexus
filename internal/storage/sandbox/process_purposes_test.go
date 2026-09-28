// INPUT: 同 owner/session/generation 的独立启动用途及已有 144 版 SQLite 数据。
// OUTPUT: 探测顺序与唯一活跃边界、旧登记保留及无损回退拒绝。
// POS: 多进程启动代次的持久语义，不证明原生集合状态。
package sandbox

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/pressly/goose/v3"
)

func TestProcessPurposesShareGenerationWithoutReplay(t *testing.T) {
	r := newSandboxReceiptRepository(t)
	ctx := t.Context()
	purposes := []protocol.SandboxProcessPurpose{protocol.SandboxProcessClaudeSandboxProbe, protocol.SandboxProcessClaudeRestrictedProbe, protocol.SandboxProcessVersionProbe, protocol.SandboxProcessRuntime}
	var previous protocol.SandboxProcessIntent
	for n, purpose := range purposes {
		i := processIntent()
		i.Purpose = purpose
		i.Key.LaunchID = fmt.Sprintf("%032x", n+1)
		i.JobLabel = "cn.nexus.runtime." + i.Key.LaunchID
		if n > 0 {
			if err := r.PrepareProcess(ctx, i); !errors.Is(err, ErrProcessConflict) {
				t.Fatalf("started with previous active: %v", err)
			}
			proof := protocol.SandboxProcessEvidence{Registration: processRegistration(previous), Reason: "coalition_reaped", ObservedBootID: previous.BootID}
			if err := r.ReapProcess(ctx, previous.Key, proof); err != nil {
				t.Fatal(err)
			}
		}
		if err := r.PrepareProcess(ctx, i); err != nil {
			t.Fatal(err)
		}
		if err := r.RegisterProcess(ctx, i.Key, processRegistration(i)); err != nil {
			t.Fatal(err)
		}
		if err := r.ClaimProcessRelease(ctx, i.Key); err != nil {
			t.Fatal(err)
		}
		got, found, err := r.LatestProcess(ctx, i.Key.OwnerUserID, i.Key.SessionKey)
		if err != nil || !found || got.Intent != i || got.Phase != protocol.SandboxProcessReleased {
			t.Fatalf("latest=%+v found=%v err=%v", got, found, err)
		}
		previous = i
	}
	proof := protocol.SandboxProcessEvidence{Registration: processRegistration(previous), Reason: "coalition_reaped", ObservedBootID: previous.BootID}
	if err := r.ReapProcess(ctx, previous.Key, proof); err != nil {
		t.Fatal(err)
	}
	// 同代次新的 ID 不能重放已经越过的阶段，也不能再启动第二个 runtime。
	for n, purpose := range purposes {
		i := previous
		i.Purpose = purpose
		i.Key.LaunchID = fmt.Sprintf("%032x", n+101)
		i.JobLabel = "cn.nexus.runtime." + i.Key.LaunchID
		if err := r.PrepareProcess(ctx, i); !errors.Is(err, ErrProcessConflict) {
			t.Fatalf("replayed %s: %v", purpose, err)
		}
	}
	// 迁移回退不得选择性丢弃同代次的探测证据。
	if err := goose.DownTo(r.db, "../../../db/migrations/sqlite", 144); err == nil {
		t.Fatal("lossy rollback succeeded")
	}
	version, err := goose.GetDBVersion(r.db)
	if err != nil || version != 145 {
		t.Fatalf("failed rollback changed schema: %d %v", version, err)
	}
	latest, _, err := r.LatestProcess(ctx, previous.Key.OwnerUserID, previous.Key.SessionKey)
	if err != nil || latest.Intent != previous {
		t.Fatalf("rollback lost data: %+v %v", latest, err)
	}
	var count int
	if err := r.db.QueryRow("SELECT COUNT(*) FROM sandbox_process_launches").Scan(&count); err != nil || count != 4 {
		t.Fatalf("records=%d %v", count, err)
	}
}

func TestProcessPurposeMigrationPreservesLegacyIntent(t *testing.T) {
	r := newSandboxReceiptRepository(t)
	ctx := t.Context()
	if err := goose.DownTo(r.db, "../../../db/migrations/sqlite", 144); err != nil {
		t.Fatal(err)
	}
	i := processIntent()
	data, err := json.Marshal(i)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if _, err := r.db.Exec(`INSERT INTO sandbox_process_launches(owner_user_id,session_key,generation,launch_id,phase,intent_json,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)`, i.Key.OwnerUserID, i.Key.SessionKey, i.Key.Generation, i.Key.LaunchID, "prepared", string(data), now, now); err != nil {
		t.Fatal(err)
	}
	if err := goose.Up(r.db, "../../../db/migrations/sqlite"); err != nil {
		t.Fatal(err)
	}
	got, found, err := r.Process(ctx, i.Key)
	if err != nil || !found || got.Intent != i || got.Phase != protocol.SandboxProcessPrepared {
		t.Fatalf("legacy=%+v %v %v", got, found, err)
	}
	var saved string
	if err := r.db.QueryRow("SELECT intent_json FROM sandbox_process_launches WHERE launch_id=?", i.Key.LaunchID).Scan(&saved); err != nil || saved != string(data) {
		t.Fatal("legacy intent rewritten", err)
	}
	if err := r.AbortPreparedProcess(ctx, i.Key); err != nil {
		t.Fatal(err)
	}
	// 旧单进程代次按 runtime 最后一个阶段解释，禁止补写同代次的前置探测。
	probe := i
	probe.Purpose = protocol.SandboxProcessVersionProbe
	probe.Key.LaunchID = fmt.Sprintf("%032x", 71)
	probe.JobLabel = "cn.nexus.runtime." + probe.Key.LaunchID
	if err := r.PrepareProcess(ctx, probe); !errors.Is(err, ErrProcessConflict) {
		t.Fatalf("legacy generation reopened: %v", err)
	}
	probe.Key.Generation++
	if err := r.PrepareProcess(ctx, probe); err != nil {
		t.Fatal(err)
	}
	if _, err := r.db.Exec("UPDATE sandbox_process_launches SET launch_order=2 WHERE launch_id=?", probe.Key.LaunchID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.Process(ctx, probe.Key); !errors.Is(err, ErrInvalidProcess) {
		t.Fatalf("purpose mismatch accepted: %v", err)
	}
}
