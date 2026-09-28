// INPUT: 原生已终止记录、完成资源标记与仍待收口的策略回执。
// OUTPUT: 后续阶段可跨重启分页发现，完成的记录不会反复扫描。
// POS: 生命周期恢复索引/游标测试，不执行任务或目录清理。
package sandbox

import (
	"fmt"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestLifecycleRecoveryScanFindsTerminalFollowupAndStablePages(t *testing.T) {
	r := newSandboxReceiptRepository(t)
	ctx := t.Context()
	var keys []protocol.SandboxProcessKey
	for n := 1; n <= 4; n++ {
		i := processIntent()
		i.LeaseID = ""
		i.Key.SessionKey = fmt.Sprintf("session-%d", n)
		i.Key.LaunchID = fmt.Sprintf("%032x", n)
		i.JobLabel = "cn.nexus.runtime." + i.Key.LaunchID
		if err := r.PrepareProcess(ctx, i); err != nil {
			t.Fatal(err)
		}
		if n == 4 {
			continue
		} // Native pending is deliberately handled by the first pass.
		reg := processRegistration(i)
		for _, err := range []error{r.RegisterProcess(ctx, i.Key, reg), r.ClaimProcessRelease(ctx, i.Key), r.ReapProcess(ctx, i.Key, protocol.SandboxProcessEvidence{Registration: reg, ObservedBootID: i.BootID, Reason: "coalition_reaped"})} {
			if err != nil {
				t.Fatal(err)
			}
		}
		keys = append(keys, i.Key)
	}
	first, more, err := r.PendingLifecycleKeys(ctx, "", 1)
	if err != nil || !more || len(first) != 1 || first[0] != keys[0] {
		t.Fatalf("first=%+v %v %v", first, more, err)
	}
	if err := r.CompleteProcessResources(ctx, keys[0]); err != nil {
		t.Fatal(err)
	}
	next, more, err := r.PendingLifecycleKeys(ctx, first[0].LaunchID, 1)
	if err != nil || !more || len(next) != 1 || next[0] != keys[1] {
		t.Fatalf("next=%+v %v %v", next, more, err)
	}
	for _, key := range keys {
		if err := r.CompleteProcessResources(ctx, key); err != nil {
			t.Fatal(err)
		}
	}
	// Resource completion must not hide a policy receipt written independently.
	receipt := testSandboxReceipt()
	receipt.OwnerUserID = keys[1].OwnerUserID
	receipt.SessionKey = keys[1].SessionKey
	receipt.LeaseID = ""
	receipt.ProcessKey = &keys[1]
	if err := r.Save(ctx, receipt); err != nil {
		t.Fatal(err)
	}
	pending, more, err := r.PendingLifecycleKeys(ctx, "", 16)
	if err != nil || more || len(pending) != 1 || pending[0] != keys[1] {
		t.Fatalf("policy omitted: %+v %v", pending, err)
	}
	if _, err := r.ReconcileProcessPolicies(ctx, keys[1]); err != nil {
		t.Fatal(err)
	}
	pending, more, err = r.PendingLifecycleKeys(ctx, "", 16)
	if err != nil || more || len(pending) != 0 {
		t.Fatalf("completed rows rescanned: %+v %v", pending, err)
	}
	native, _, err := r.PendingProcessKeys(ctx, "", 16)
	if err != nil || len(native) != 1 {
		t.Fatalf("native pending lost: %+v %v", native, err)
	}
	if err := r.CompleteProcessResources(ctx, native[0]); err == nil {
		t.Fatal("marked active native process resources complete")
	}
}
