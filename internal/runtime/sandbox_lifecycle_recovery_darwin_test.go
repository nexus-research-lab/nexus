//go:build darwin

// INPUT: SQLite 终态记录、持锁回调与资源阶段提交失败。
// OUTPUT: 失败项保留，后续项继续，分页结束不掩盖失败。
// POS: 生命周期扫描编排测试；不宣称真实 App 启动验收。
package runtime

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	sandboxstore "github.com/nexus-research-lab/nexus/internal/storage/sandbox"
)

type lifecycleFaultStore struct {
	*sandboxstore.Repository
	fail    protocol.SandboxProcessKey
	failure error
	holding *bool
}

func (s *lifecycleFaultStore) CompleteProcessResources(ctx context.Context, key protocol.SandboxProcessKey) error {
	if !*s.holding {
		return errors.New("resource completion escaped ownership")
	}
	if key == s.fail {
		return s.failure
	}
	return s.Repository.CompleteProcessResources(ctx, key)
}

func TestLifecycleRecoveryBatchPreservesFailure(t *testing.T) {
	h, repo, base := newProcessHostFixture(t)
	var keys []protocol.SandboxProcessKey
	for n := 1; n <= 3; n++ {
		host, err := newSandboxProcessHost(repo, h.root, sandboxProcessBinding{Owner: "owner", Session: fmt.Sprintf("session-%d", n), RuntimeKind: "nxs", Generation: 1})
		if err != nil {
			t.Fatal(err)
		}
		intent := base
		intent.ID = fmt.Sprintf("%032x", n)
		intent.JobLabel = "cn.nexus.runtime." + intent.ID
		if _, err := host.Reserve(t.Context(), intent); err != nil {
			t.Fatal(err)
		}
		if err := host.Finish(t.Context(), intent, nil); err != nil {
			t.Fatal(err)
		}
		keys = append(keys, host.intent.Key)
	}
	holding := false
	ownership := recoveryOwnershipFunc(func(fn func(string) error) error {
		holding = true
		defer func() { holding = false }()
		return fn(h.root.Name())
	})
	failure := errors.New("resource commit unavailable")
	store := &lifecycleFaultStore{Repository: repo, fail: keys[0], failure: failure, holding: &holding}
	m := NewManager()
	m.SetSandboxPolicyReceiptStore(store)
	if err := m.SetSandboxProcessSupervisor(SandboxProcessSupervisor{Root: h.root, HelperPath: "/trusted/bootstrap", HelperSHA256: base.HelperSHA256}); err != nil {
		t.Fatal(err)
	}
	batch, err := m.RecoverPendingSandboxLifecycles(t.Context(), ownership, "", 2)
	if !errors.Is(err, failure) || len(batch.Items) != 2 || !batch.HasMore || batch.NextCursor != keys[1].LaunchID || batch.Items[0].ResourcesComplete || !batch.Items[1].ResourcesComplete {
		t.Fatalf("first batch=%+v err=%v", batch, err)
	}
	next, err := m.RecoverPendingSandboxLifecycles(t.Context(), ownership, batch.NextCursor, 2)
	if err != nil || len(next.Items) != 1 || next.HasMore || !next.Items[0].ResourcesComplete {
		t.Fatalf("next=%+v err=%v", next, err)
	}
	retry, err := m.RecoverPendingSandboxLifecycles(t.Context(), ownership, "", 2)
	if !errors.Is(err, failure) || len(retry.Items) != 1 || retry.HasMore || retry.Items[0].Key != keys[0] {
		t.Fatalf("retry=%+v err=%v", retry, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := m.RecoverPendingSandboxLifecycles(ctx, ownership, "", 2); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := m.RecoverPendingSandboxLifecycles(t.Context(), nil, "", 2); err == nil {
		t.Fatal("unowned recovery accepted")
	}
	store.fail = protocol.SandboxProcessKey{}
	complete, err := m.RecoverPendingSandboxLifecycles(t.Context(), ownership, "", 2)
	if err != nil || len(complete.Items) != 1 || !complete.Items[0].ResourcesComplete {
		t.Fatalf("complete=%+v err=%v", complete, err)
	}
}
