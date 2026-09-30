// INPUT: 实际 SQLite pending key、持锁回调与可失败的原生恢复器。
// OUTPUT: 失败项不重放且不饿死后续项，批次分页与取消保留未处理记录。
// POS: 恢复编排测试；原生集合回收由 Bridge 的独立验收证明。
package runtime

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/supervision"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestProcessRecoveryBatchAdvancesPastFailure(t *testing.T) {
	h, store, base := newProcessHostFixture(t)
	var keys []protocol.SandboxProcessKey
	for n := 1; n <= 3; n++ {
		host, err := newSandboxProcessHost(store, h.root, sandboxProcessBinding{Owner: "owner", Session: fmt.Sprintf("session-%d", n), RuntimeKind: "nxs", Generation: 1})
		if err != nil {
			t.Fatal(err)
		}
		intent := base
		intent.ID = fmt.Sprintf("%032x", n)
		intent.JobLabel = "cn.nexus.runtime." + intent.ID
		if _, err := host.Reserve(t.Context(), intent); err != nil {
			t.Fatal(err)
		}
		keys = append(keys, host.intent.Key)
	}
	m := NewManager()
	m.SetSandboxPolicyReceiptStore(store)
	if err := m.SetSandboxProcessSupervisor(SandboxProcessSupervisor{Root: h.root, HelperPath: "/trusted/bootstrap", HelperSHA256: base.HelperSHA256}); err != nil {
		t.Fatal(err)
	}
	holding := false
	ownership := recoveryOwnershipFunc(func(fn func(string) error) error {
		holding = true
		defer func() { holding = false }()
		return fn(h.root.Name())
	})
	failure := errors.New("native retirement unavailable")
	var calls []string
	native := func(ctx context.Context, record supervision.Recovery, host supervision.RecoveryHost) error {
		if !holding {
			t.Fatal("recovery escaped host ownership")
		}
		calls = append(calls, record.Intent.ID)
		if record.Intent.ID == keys[0].LaunchID {
			return failure
		}
		return host.Finish(ctx, record.Intent, nil)
	}
	batch, err := m.recoverPendingSandboxProcesses(t.Context(), ownership, "", 2, native)
	if !errors.Is(err, failure) || len(batch.Items) != 2 || !batch.HasMore || batch.NextCursor != keys[1].LaunchID || batch.Items[0].Err == nil || batch.Items[1].Snapshot.Phase != protocol.SandboxProcessAborted {
		t.Fatalf("batch=%+v err=%v", batch, err)
	}
	next, err := m.recoverPendingSandboxProcesses(t.Context(), ownership, batch.NextCursor, 2, native)
	if err != nil || len(next.Items) != 1 || next.HasMore || next.Items[0].Key != keys[2] || len(calls) != 3 {
		t.Fatalf("next=%+v calls=%v err=%v", next, calls, err)
	}
	snapshot, _, err := store.Process(t.Context(), keys[0])
	if err != nil || snapshot.Phase != protocol.SandboxProcessPrepared {
		t.Fatalf("failed item lost fence: %+v %v", snapshot, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := m.recoverPendingSandboxProcesses(ctx, ownership, "", 2, native); !errors.Is(err, context.Canceled) || len(calls) != 3 {
		t.Fatalf("canceled batch ran: %v calls=%v", err, calls)
	}
	if _, err := m.recoverPendingSandboxProcesses(t.Context(), nil, "", 2, native); err == nil || len(calls) != 3 {
		t.Fatal("unowned scan reached native")
	}
	for n := 4; n <= 5; n++ {
		host, err := newSandboxProcessHost(store, h.root, sandboxProcessBinding{Owner: "owner", Session: fmt.Sprintf("session-%d", n), RuntimeKind: "nxs", Generation: 1})
		if err != nil {
			t.Fatal(err)
		}
		intent := base
		intent.ID = fmt.Sprintf("%032x", n)
		intent.JobLabel = "cn.nexus.runtime." + intent.ID
		if _, err := host.Reserve(t.Context(), intent); err != nil {
			t.Fatal(err)
		}
	}
	partialContext, stop := context.WithCancel(t.Context())
	defer stop()
	partial, err := m.recoverPendingSandboxProcesses(partialContext, ownership, "", 3, func(ctx context.Context, record supervision.Recovery, host supervision.RecoveryHost) error {
		err := host.Finish(ctx, record.Intent, nil)
		stop()
		return err
	})
	if !errors.Is(err, context.Canceled) || len(partial.Items) != 1 || !partial.HasMore || partial.NextCursor != keys[0].LaunchID {
		t.Fatalf("partial=%+v err=%v", partial, err)
	}
	resumed, err := m.recoverPendingSandboxProcesses(t.Context(), ownership, partial.NextCursor, 3, native)
	if err != nil || len(resumed.Items) != 2 || resumed.HasMore || len(calls) != 5 {
		t.Fatalf("resumed=%+v calls=%v err=%v", resumed, calls, err)
	}
}
