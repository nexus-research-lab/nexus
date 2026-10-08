// INPUT: 真实 SQLite 的资源清理登记、阶段变更、并发启动与历史迁移。
// OUTPUT: exact 资源身份不可改绑、pending 栅栏与提交重试。
// POS: 数据库恢复合同；原生目录删除由 runtime 独立验证。
package sandbox

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/pressly/goose/v3"
)

func scratchRecoveryProcess(t *testing.T, r *Repository) protocol.SandboxProcessIntent {
	t.Helper()
	i := processIntent()
	i.LeaseID = strings.Repeat("c", 32)
	i.Scratch = protocol.SandboxProcessScratch{BasePath: filepath.Join(t.TempDir(), "sandbox"), LeafName: ".scratch-original", BaseIdentity: "darwin-v1:1:2:0:3:4", LeafIdentity: "darwin-v1:1:5:0:3:4"}
	if err := r.PrepareProcess(t.Context(), i); err != nil {
		t.Fatal(err)
	}
	return i
}
func TestScratchRecoveryLedgerFencesStartupAndRejectsLossyRollback(t *testing.T) {
	r := newSandboxReceiptRepository(t)
	ctx := t.Context()
	i := scratchRecoveryProcess(t, r)
	if _, err := r.PrepareScratchRecovery(ctx, i.Key); err == nil {
		t.Fatal("accepted active process")
	}
	if err := r.AbortPreparedProcess(ctx, i.Key); err != nil {
		t.Fatal(err)
	}
	record, err := r.PrepareScratchRecovery(ctx, i.Key)
	if err != nil {
		t.Fatal(err)
	}
	next := i
	next.Key.Generation++
	next.Key.LaunchID = strings.Repeat("d", 32)
	next.JobLabel = "cn.nexus.runtime." + next.Key.LaunchID
	if err := r.PrepareProcess(ctx, next); err == nil {
		t.Fatal("pending recovery admitted new launch")
	}
	if err := r.AdvanceScratchRecovery(ctx, record, protocol.SandboxScratchRecoveryComplete); err == nil {
		t.Fatal("skipped deletion phases")
	}
	forged := record
	forged.Scratch.LeafName = ".scratch-another"
	if err := r.AdvanceScratchRecovery(ctx, forged, protocol.SandboxScratchRecoveryQuarantined); err == nil {
		t.Fatal("rebound recovery directory")
	}
	if err := goose.DownTo(r.db, "../../../db/migrations/sqlite", 147); err == nil {
		t.Fatal("discarded pending recovery")
	}
	for _, phase := range []protocol.SandboxScratchRecoveryPhase{protocol.SandboxScratchRecoveryQuarantined, protocol.SandboxScratchRecoveryDeleting, protocol.SandboxScratchRecoveryComplete} {
		if err := r.AdvanceScratchRecovery(ctx, record, phase); err != nil {
			t.Fatal(err)
		}
		if err := r.AdvanceScratchRecovery(ctx, record, phase); err != nil {
			t.Fatal("lost response retry", err)
		}
		record.Phase = phase
	}
	if err := r.PrepareProcess(ctx, next); err != nil {
		t.Fatal("completed recovery retained fence", err)
	}
	// Even an existing complete row cannot authorize filesystem work while a
	// later process for this session is still active.
	if _, err := r.PrepareScratchRecovery(ctx, i.Key); err == nil {
		t.Fatal("ignored later active process")
	}
	got, found, err := r.ScratchRecovery(ctx, i.Key.OwnerUserID, i.Key.SessionKey, i.LeaseID)
	if err != nil || !found || got != record {
		t.Fatalf("record changed: %+v %v", got, err)
	}
	// A damaged ledger cannot turn an old no-proof intent into completed cleanup.
	i.Scratch = protocol.SandboxProcessScratch{}
	encoded, err := json.Marshal(i)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.db.Exec("UPDATE sandbox_process_launches SET intent_json=? WHERE launch_id=?", string(encoded), i.Key.LaunchID); err != nil {
		t.Fatal(err)
	}
	if _, err := r.db.Exec("UPDATE sandbox_scratch_recoveries SET scratch_json='{}' WHERE lease_id=?", i.LeaseID); err != nil {
		t.Fatal(err)
	}
	if _, _, err := r.ScratchRecovery(ctx, i.Key.OwnerUserID, i.Key.SessionKey, i.LeaseID); err == nil {
		t.Fatal("accepted complete recovery without original scratch proof")
	}
}
