// INPUT: 已收口但 scratch 恢复卡在 prepared 的原进程、可能存活的原进程与绑定策略。
// OUTPUT: owner 放弃只收口终态进程的栅栏，保留审计理由，不放行仍可能存活的进程。
// POS: 启动恢复失败后的显式出口存储测试，不执行目录删除。
package sandbox

import (
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestDiscardProcessLifecycleClosesStuckScratchAndPolicyFences(t *testing.T) {
	r := newSandboxReceiptRepository(t)
	ctx := t.Context()
	i := scratchRecoveryProcess(t, r)
	reg := processRegistration(i)
	if err := r.RegisterProcess(ctx, i.Key, reg); err != nil {
		t.Fatal(err)
	}
	if _, err := r.DiscardProcessLifecycle(ctx, i.Key, "owner removed scratch"); err == nil {
		t.Fatal("discarded a process that may still be alive")
	}
	if err := r.ClaimProcessRelease(ctx, i.Key); err != nil {
		t.Fatal(err)
	}
	receipt := testSandboxReceipt()
	receipt.OwnerUserID = i.Key.OwnerUserID
	receipt.SessionKey = i.Key.SessionKey
	receipt.LeaseID = i.LeaseID
	receipt.ProcessKey = &i.Key
	if err := r.Save(ctx, receipt); err != nil {
		t.Fatal(err)
	}
	if err := r.ReapProcess(ctx, i.Key, protocol.SandboxProcessEvidence{Registration: reg, ObservedBootID: i.BootID, Reason: "coalition_reaped"}); err != nil {
		t.Fatal(err)
	}
	if err := r.UpdatePhase(ctx, receipt.OwnerUserID, receipt.SessionKey, receipt.Generation, protocol.SandboxPolicyReceiptUnknown, "close failed"); err != nil {
		t.Fatal(err)
	}
	// Automatic recovery stopped in prepared: the original directory is gone.
	if _, err := r.PrepareScratchRecovery(ctx, i.Key); err != nil {
		t.Fatal(err)
	}
	if pending, err := r.PendingScratchRecovery(ctx, i.Key.OwnerUserID, i.Key.SessionKey); err != nil || !pending {
		t.Fatalf("fixture not fenced: %v %v", pending, err)
	}
	if _, err := r.DiscardProcessLifecycle(ctx, i.Key, "   "); err == nil {
		t.Fatal("discard accepted an empty reason")
	}
	count, err := r.DiscardProcessLifecycle(ctx, i.Key, "owner removed scratch\nby hand")
	if err != nil || count != 1 {
		t.Fatalf("discard=%d %v", count, err)
	}
	if pending, err := r.PendingScratchRecovery(ctx, i.Key.OwnerUserID, i.Key.SessionKey); err != nil || pending {
		t.Fatalf("scratch fence kept: %v %v", pending, err)
	}
	if keys, _, err := r.PendingLifecycleKeys(ctx, "", 16); err != nil || len(keys) != 0 {
		t.Fatalf("lifecycle still pending: %+v %v", keys, err)
	}
	got, found, err := r.Get(ctx, receipt.OwnerUserID, receipt.SessionKey, receipt.Generation)
	if err != nil || !found || got.Phase != protocol.SandboxPolicyReceiptReconciled ||
		!strings.Contains(got.UnknownReason, "close failed") || !strings.Contains(got.UnknownReason, "discarded by owner: owner removed scratch by hand") {
		t.Fatalf("receipt=%+v %v", got, err)
	}
	// A later launch in the same session is admitted again.
	next := i
	next.Key.Generation++
	next.Key.LaunchID = strings.Repeat("d", 32)
	next.JobLabel = "cn.nexus.runtime." + next.Key.LaunchID
	if err := r.PrepareProcess(ctx, next); err != nil {
		t.Fatalf("session stayed fenced: %v", err)
	}
}

func TestDiscardProcessLifecycleCreatesMissingScratchRecord(t *testing.T) {
	r := newSandboxReceiptRepository(t)
	ctx := t.Context()
	i := scratchRecoveryProcess(t, r)
	if err := r.AbortPreparedProcess(ctx, i.Key); err != nil {
		t.Fatal(err)
	}
	if _, err := r.DiscardProcessLifecycle(ctx, i.Key, "never registered"); err != nil {
		t.Fatal(err)
	}
	record, found, err := r.ScratchRecovery(ctx, i.Key.OwnerUserID, i.Key.SessionKey, i.LeaseID)
	if err != nil || !found || record.Phase != protocol.SandboxScratchRecoveryComplete || record.Scratch != i.Scratch {
		t.Fatalf("record=%+v %v %v", record, found, err)
	}
	if err := r.CompleteProcessResources(ctx, i.Key); err != nil {
		t.Fatalf("completed resources disagree with discard: %v", err)
	}
}
