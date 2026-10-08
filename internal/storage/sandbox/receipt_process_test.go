// INPUT: 真实 SQLite 中的监督启动与跨 warm 代次策略。
// OUTPUT: 原 launch 关联持久化、错误 scope/lease/probe 拒绝及旧记录不补猜。
// POS: 数据库身份边界测试，不宣称原生进程回收已经完成。
package sandbox

import (
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/pressly/goose/v3"
)

func TestReceiptProcessBindingWarmGeneration(t *testing.T) {
	r := newSandboxReceiptRepository(t)
	ctx := t.Context()
	i := processIntent()
	i.Purpose = protocol.SandboxProcessRuntime
	for _, err := range []error{r.PrepareProcess(ctx, i), r.RegisterProcess(ctx, i.Key, processRegistration(i)), r.ClaimProcessRelease(ctx, i.Key)} {
		if err != nil {
			t.Fatal(err)
		}
	}
	receipt := testSandboxReceipt()
	receipt.OwnerUserID, receipt.SessionKey, receipt.LeaseID = i.Key.OwnerUserID, i.Key.SessionKey, i.LeaseID
	receipt.Generation = 9 // Warm policy generation differs from process generation 4.
	receipt.ProcessKey = &i.Key
	if err := r.Save(ctx, receipt); err != nil {
		t.Fatal(err)
	}
	got, found, err := r.Get(ctx, receipt.OwnerUserID, receipt.SessionKey, receipt.Generation)
	if err != nil || !found || !sameReceiptProcess(got.ProcessKey, &i.Key) {
		t.Fatalf("binding=%+v found=%v err=%v", got, found, err)
	}
	if err := r.Save(ctx, receipt); err != nil {
		t.Fatal(err)
	}
	without := receipt
	without.ProcessKey = nil
	if err := r.Save(ctx, without); err == nil {
		t.Fatal("removed original process binding")
	}
	// A later process with the same lease and runtime cannot replace the original.
	reg := processRegistration(i)
	if err := r.ReapProcess(ctx, i.Key, protocol.SandboxProcessEvidence{Registration: reg, ObservedBootID: i.BootID, Reason: "coalition_reaped"}); err != nil {
		t.Fatal(err)
	}
	next := i
	next.Key.Generation = 8
	next.Key.LaunchID = strings.Repeat("c", 32)
	next.JobLabel = "cn.nexus.runtime." + next.Key.LaunchID
	for _, err := range []error{r.PrepareProcess(ctx, next), r.RegisterProcess(ctx, next.Key, processRegistration(next)), r.ClaimProcessRelease(ctx, next.Key)} {
		if err != nil {
			t.Fatal(err)
		}
	}
	changed := receipt
	changed.ProcessKey = &next.Key
	if err := r.Save(ctx, changed); err == nil {
		t.Fatal("rebound policy to newer process")
	}
	exact, found, err := r.RuntimeProcess(ctx, i.Key.OwnerUserID, i.Key.SessionKey, i.Key.Generation)
	if err != nil || !found || exact.Intent.Key != i.Key {
		t.Fatalf("exact lookup followed latest: %+v %v", exact, err)
	}
	// Old unsupervised rows must never acquire invented supervision proof.
	historical := receipt
	historical.Generation = 10
	historical.ProcessKey = nil
	if err := r.Save(ctx, historical); err != nil {
		t.Fatal(err)
	}
	historical.ProcessKey = &next.Key
	if err := r.Save(ctx, historical); err == nil {
		t.Fatal("backfilled historical binding")
	}
	for name, mutate := range map[string]func(*protocol.SandboxPolicyReceiptSnapshot){
		"owner":          func(s *protocol.SandboxPolicyReceiptSnapshot) { s.OwnerUserID = "other" },
		"session":        func(s *protocol.SandboxPolicyReceiptSnapshot) { s.SessionKey = "other" },
		"lease":          func(s *protocol.SandboxPolicyReceiptSnapshot) { s.LeaseID = "other" },
		"runtime":        func(s *protocol.SandboxPolicyReceiptSnapshot) { s.RuntimeKind = "claude" },
		"future process": func(s *protocol.SandboxPolicyReceiptSnapshot) { s.Generation = 1 },
	} {
		t.Run(name, func(t *testing.T) {
			bad := receipt
			bad.Generation = 11
			mutate(&bad)
			if err := r.Save(ctx, bad); err == nil {
				t.Fatal("accepted invalid binding")
			}
		})
	}
}

func TestReceiptProcessBindingRejectsUnreleasedAndProbe(t *testing.T) {
	for _, purpose := range []protocol.SandboxProcessPurpose{protocol.SandboxProcessRuntime, protocol.SandboxProcessVersionProbe} {
		t.Run(string(purpose), func(t *testing.T) {
			r := newSandboxReceiptRepository(t)
			ctx := t.Context()
			i := processIntent()
			i.Purpose = purpose
			if err := r.PrepareProcess(ctx, i); err != nil {
				t.Fatal(err)
			}
			receipt := testSandboxReceipt()
			receipt.OwnerUserID = i.Key.OwnerUserID
			receipt.SessionKey = i.Key.SessionKey
			receipt.LeaseID = i.LeaseID
			receipt.ProcessKey = &i.Key
			if err := r.Save(ctx, receipt); err == nil {
				t.Fatal("accepted prepared runtime")
			}
			if err := r.RegisterProcess(ctx, i.Key, processRegistration(i)); err != nil {
				t.Fatal(err)
			}
			if err := r.Save(ctx, receipt); err == nil {
				t.Fatal("accepted unreleased runtime")
			}
			if err := r.ClaimProcessRelease(ctx, i.Key); err != nil {
				t.Fatal(err)
			}
			err := r.Save(ctx, receipt)
			if purpose == protocol.SandboxProcessRuntime && err != nil {
				t.Fatal(err)
			}
			if purpose != protocol.SandboxProcessRuntime && err == nil {
				t.Fatal("accepted probe as runtime")
			}
		})
	}
}

func TestReceiptProcessMigrationPreservesHistoricalAndRejectsLossyRollback(t *testing.T) {
	r := newSandboxReceiptRepository(t)
	ctx := t.Context()
	historical := testSandboxReceipt()
	historical.Phase = protocol.SandboxPolicyReceiptUnknown
	historical.UnknownReason = "historical cleanup unknown"
	if err := r.Save(ctx, historical); err != nil {
		t.Fatal(err)
	}
	if err := goose.DownTo(r.db, "../../../db/migrations/sqlite", 146); err != nil {
		t.Fatal(err)
	}
	if err := goose.Up(r.db, "../../../db/migrations/sqlite"); err != nil {
		t.Fatal(err)
	}
	got, found, err := r.Get(ctx, historical.OwnerUserID, historical.SessionKey, historical.Generation)
	if err != nil || !found || got.ProcessKey != nil || got.Phase != historical.Phase || got.UnknownReason != historical.UnknownReason || got.LeaseID != historical.LeaseID {
		t.Fatalf("migration altered historical receipt: %+v %v", got, err)
	}
	i := processIntent()
	for _, err := range []error{r.PrepareProcess(ctx, i), r.RegisterProcess(ctx, i.Key, processRegistration(i)), r.ClaimProcessRelease(ctx, i.Key)} {
		if err != nil {
			t.Fatal(err)
		}
	}
	bound := testSandboxReceipt()
	bound.OwnerUserID = i.Key.OwnerUserID
	bound.SessionKey = i.Key.SessionKey
	bound.LeaseID = i.LeaseID
	bound.ProcessKey = &i.Key
	if err := r.Save(ctx, bound); err != nil {
		t.Fatal(err)
	}
	if err := goose.DownTo(r.db, "../../../db/migrations/sqlite", 146); err == nil {
		t.Fatal("discarded process binding during rollback")
	}
	got, found, err = r.Get(ctx, bound.OwnerUserID, bound.SessionKey, bound.Generation)
	if err != nil || !found || !sameReceiptProcess(got.ProcessKey, &i.Key) {
		t.Fatalf("failed rollback lost evidence: %+v %v", got, err)
	}
}
