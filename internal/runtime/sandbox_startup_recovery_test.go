// INPUT: 重建的 Manager、持久 sandbox 回执及待创建的 runtime。
// OUTPUT: 新代次不重用旧回执，未收口状态在进程创建前阻断。
// POS: 宿主重启准入回归；不把回执当作后代退出证明。
package runtime

import (
	"context"
	"errors"
	"math"
	"testing"

	bridge "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestManagerSandboxRestartContinuesDurableGeneration(t *testing.T) {
	for _, phase := range []protocol.SandboxPolicyReceiptPhase{protocol.SandboxPolicyReceiptRetired, protocol.SandboxPolicyReceiptReconciled} {
		t.Run(string(phase), func(t *testing.T) {
			snapshot := testSandboxReceiptSnapshotForManager()
			snapshot.Generation, snapshot.Phase, snapshot.UnknownReason = 41, phase, ""
			store := &recordingSandboxReceiptStore{latest: snapshot, found: true}
			manager := NewManagerWithFactory(&fakeRuntimeFactory{client: &fakeRuntimeClient{}})
			manager.SetSandboxPolicyReceiptStore(store)
			_, err := manager.GetOrCreate(t.Context(), snapshot.SessionKey, bridge.Options{Env: map[string]string{"NEXUS_RUNTIME_USER_ID": snapshot.OwnerUserID}})
			if err != nil {
				t.Fatal(err)
			}
			if got := manager.sessions[snapshot.SessionKey].StartupGeneration; got != 42 {
				t.Fatalf("new generation = %d, want 42 after durable generation 41", got)
			}
		})
	}
}

func TestManagerSandboxRestartRejectsUnresolvedReceiptBeforeFactory(t *testing.T) {
	for _, phase := range []protocol.SandboxPolicyReceiptPhase{protocol.SandboxPolicyReceiptConfirmed, protocol.SandboxPolicyReceiptRetiring, protocol.SandboxPolicyReceiptUnknown} {
		t.Run(string(phase), func(t *testing.T) {
			snapshot := testSandboxReceiptSnapshotForManager()
			snapshot.Phase = phase
			store := &recordingSandboxReceiptStore{latest: snapshot, found: true}
			for _, kind := range []bridge.RuntimeKind{bridge.RuntimeNXS, bridge.RuntimeClaude} {
				calls := 0
				manager := NewManagerWithFactory(runtimeFactoryFunc(func(bridge.Options) Client { calls++; return &fakeRuntimeClient{} }))
				manager.SetSandboxPolicyReceiptStore(store)
				_, err := manager.GetOrCreate(t.Context(), snapshot.SessionKey, bridge.Options{
					Runtime: bridge.RuntimeOptions{Kind: kind}, Env: map[string]string{"NEXUS_RUNTIME_USER_ID": snapshot.OwnerUserID},
				})
				if err == nil || calls != 0 {
					t.Fatalf("unresolved %s started %s: error=%v factory_calls=%d", phase, kind, err, calls)
				}
			}
		})
	}
}

// failingRecoveryReader 验证恢复查询不可用时不能猜测不存在旧回执。
type failingRecoveryReader struct{ recordingSandboxReceiptStore }

func (*failingRecoveryReader) Latest(context.Context, string, string) (protocol.SandboxPolicyReceiptSnapshot, bool, error) {
	return protocol.SandboxPolicyReceiptSnapshot{}, false, errors.New("receipt database unavailable")
}

func TestManagerSandboxRestartRejectsUnreadableAndExhaustedReceipt(t *testing.T) {
	snapshot := testSandboxReceiptSnapshotForManager()
	snapshot.Phase, snapshot.UnknownReason, snapshot.Generation = protocol.SandboxPolicyReceiptRetired, "", math.MaxInt64
	for name, store := range map[string]SandboxPolicyReceiptStore{
		"unreadable": &failingRecoveryReader{},
		"exhausted":  &recordingSandboxReceiptStore{latest: snapshot, found: true},
	} {
		t.Run(name, func(t *testing.T) {
			calls := 0
			manager := NewManagerWithFactory(runtimeFactoryFunc(func(bridge.Options) Client { calls++; return &fakeRuntimeClient{} }))
			manager.SetSandboxPolicyReceiptStore(store)
			_, err := manager.GetOrCreate(t.Context(), snapshot.SessionKey, bridge.Options{Env: map[string]string{"NEXUS_RUNTIME_USER_ID": snapshot.OwnerUserID}})
			if err == nil || calls != 0 {
				t.Fatalf("unsafe restart: error=%v factory_calls=%d", err, calls)
			}
		})
	}
}
