// INPUT: 真实 SQLite 启动记录和显式原生结果注入。
// OUTPUT: 只收口 exact 记录、拒绝活动会话、失败保留栅栏且不清除 policy unknown。
// POS: 产品恢复适配验证；原生断宿主实测由 Bridge 独立覆盖。
package runtime

import (
	"context"
	"errors"
	"testing"

	bridge "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/supervision"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestSandboxProcessRecoveryExactScope(t *testing.T) {
	for _, mode := range []string{"prepared", "released", "native_failure", "active_client"} {
		t.Run(mode, func(t *testing.T) {
			h, store, i := newProcessHostFixture(t)
			ctx := t.Context()
			m := NewManagerWithFactory(runtimeFactoryFunc(func(bridge.Options) Client { return &fakeRuntimeClient{} }))
			m.SetSandboxPolicyReceiptStore(store)
			if err := m.SetSandboxProcessSupervisor(SandboxProcessSupervisor{Root: h.root, HelperPath: "/trusted/bootstrap", HelperSHA256: i.HelperSHA256}); err != nil {
				t.Fatal(err)
			}
			if mode == "active_client" {
				if _, err := m.GetOrCreate(ctx, "session", bridge.Options{Env: map[string]string{"NEXUS_RUNTIME_USER_ID": "owner"}}); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := h.Reserve(ctx, i); err != nil {
				t.Fatal(err)
			}
			r := supervision.Registration{Version: 1, BootID: i.BootID, OwnerUID: i.OwnerUID, CoalitionID: 19}
			if mode != "prepared" {
				if err := h.Register(ctx, i, r); err != nil {
					t.Fatal(err)
				}
				if err := h.ClaimRelease(ctx, i); err != nil {
					t.Fatal(err)
				}
			}
			policy := testSandboxReceiptSnapshotForManager()
			policy.OwnerUserID = "owner"
			policy.SessionKey = "session"
			policy.Generation = 1
			policy.Phase = protocol.SandboxPolicyReceiptUnknown
			if err := store.Save(ctx, policy); err != nil {
				t.Fatal(err)
			}
			calls := 0
			failure := errors.New("native recovery failed")
			native := func(ctx context.Context, record supervision.Recovery, host supervision.RecoveryHost) error {
				calls++
				if record.Intent != i {
					t.Fatal("original intent changed")
				}
				if mode == "native_failure" {
					return failure
				}
				var evidence *supervision.Evidence
				if record.Registration != nil {
					if *record.Registration != r {
						t.Fatal("original registration changed")
					}
					evidence = &supervision.Evidence{Registration: r, Reason: "coalition_reaped", ObservedBootID: i.BootID}
				}
				return host.Finish(ctx, i, evidence)
			}
			key := h.intent.Key
			result, err := m.recoverSandboxProcess(ctx, key, recoveryOwnershipFunc(func(fn func(string) error) error { return fn(h.root.Name()) }), native)
			if mode == "active_client" {
				if err == nil || calls != 0 {
					t.Fatalf("active recovery=%v calls=%d", err, calls)
				}
				return
			}
			if mode == "native_failure" {
				if !errors.Is(err, failure) {
					t.Fatal(err)
				}
				s, _, _ := store.Process(ctx, key)
				if s.Phase != protocol.SandboxProcessReleased {
					t.Fatal("failed recovery cleared fence")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			want := protocol.SandboxProcessReaped
			if mode == "prepared" {
				want = protocol.SandboxProcessAborted
			}
			if result.Phase != want {
				t.Fatal(result.Phase)
			}
			if _, err := m.recoverSandboxProcess(ctx, key, recoveryOwnershipFunc(func(fn func(string) error) error { return fn(h.root.Name()) }), native); err != nil || calls != 1 {
				t.Fatalf("terminal recovery replayed: %v calls=%d", err, calls)
			}
			got, found, err := store.Latest(ctx, "owner", "session")
			if err != nil || !found || got.Phase != protocol.SandboxPolicyReceiptUnknown {
				t.Fatal("process recovery changed policy receipt", got, err)
			}
			other := key
			other.OwnerUserID = "other"
			if _, err := m.recoverSandboxProcess(ctx, other, recoveryOwnershipFunc(func(fn func(string) error) error { return fn(h.root.Name()) }), native); err == nil || calls != 1 {
				t.Fatal("cross owner recovery reached native")
			}
		})
	}
}

// 仅用于仓储/控制流夹具；真实内核锁由 Darwin 集成测试覆盖。
type recoveryOwnershipFunc func(func(string) error) error

func (f recoveryOwnershipFunc) WithOwnership(operation func(string) error) error { return f(operation) }
