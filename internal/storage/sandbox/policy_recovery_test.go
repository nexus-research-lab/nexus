// INPUT: exact native 收口事实与有/无监督关联的策略记录。
// OUTPUT: 无 scratch 的原进程可收口关联策略，其他历史/未回收进程不变。
// POS: 策略恢复存储测试，不执行真实进程。
package sandbox

import (
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"testing"
)

func TestPolicyRecoveryRequiresOriginalTerminalProcess(t *testing.T) {
	r := newSandboxReceiptRepository(t)
	ctx := t.Context()
	i := processIntent()
	i.LeaseID = ""
	if err := r.PrepareProcess(ctx, i); err != nil {
		t.Fatal(err)
	}
	reg := processRegistration(i)
	if err := r.RegisterProcess(ctx, i.Key, reg); err != nil {
		t.Fatal(err)
	}
	if err := r.ClaimProcessRelease(ctx, i.Key); err != nil {
		t.Fatal(err)
	}
	receipt := testSandboxReceipt()
	receipt.OwnerUserID = i.Key.OwnerUserID
	receipt.SessionKey = i.Key.SessionKey
	receipt.LeaseID = ""
	receipt.ProcessKey = &i.Key
	if err := r.Save(ctx, receipt); err != nil {
		t.Fatal(err)
	}
	if _, err := r.ReconcileProcessPolicies(ctx, i.Key); err == nil {
		t.Fatal("reconciled still released process")
	}
	if err := r.ReapProcess(ctx, i.Key, protocol.SandboxProcessEvidence{Registration: reg, ObservedBootID: i.BootID, Reason: "coalition_reaped"}); err != nil {
		t.Fatal(err)
	}
	if count, err := r.ReconcileProcessPolicies(ctx, i.Key); err != nil || count != 1 {
		t.Fatalf("reconcile=%d %v", count, err)
	}
	if err := r.UpdatePhase(ctx, receipt.OwnerUserID, receipt.SessionKey, receipt.Generation, protocol.SandboxPolicyReceiptUnknown, "late callback"); err != nil {
		t.Fatal(err)
	}
	got, found, err := r.Get(ctx, receipt.OwnerUserID, receipt.SessionKey, receipt.Generation)
	if err != nil || !found || got.Phase != protocol.SandboxPolicyReceiptReconciled {
		t.Fatalf("late callback reopened recovery: %+v %v", got, err)
	}
}
