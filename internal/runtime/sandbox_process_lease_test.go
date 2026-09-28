// INPUT: Manager、真实 scratch 句柄、SQLite 进程登记与跨 scope/释放反例。
// OUTPUT: factory 前精确 lease 绑定，持久进程身份及失败时保留调用方所有权。
// POS: 监督启动资源合同，不代替 App 默认装配或崩溃后的资源回收。
package runtime

import (
	"errors"
	"testing"

	bridge "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/supervision"
)

func TestSandboxProcessSupervisorScratchBinding(t *testing.T) {
	h, store, intent := newProcessHostFixture(t)
	lease, err := Acquire(t.Context(), Input{OwnerUserID: "owner", SessionKey: "session", Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release()
	expectedID := lease.Marker().LeaseID
	options := bridge.Options{Runtime: bridge.RuntimeOptions{Kind: bridge.RuntimeNXS}, Env: map[string]string{"NEXUS_RUNTIME_USER_ID": "owner"}, Sandbox: &bridge.SandboxSettings{Resources: lease.Resources()}}
	var captured bridge.Options
	calls := 0
	m := NewManagerWithFactory(runtimeFactoryFunc(func(o bridge.Options) Client {
		calls++
		captured = o
		// Factory may launch immediately, before BindSandboxLease transfers ownership.
		cfg, err := o.ProcessSupervision(t.Context(), supervision.Runtime)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := cfg.Host.Reserve(t.Context(), intent); err != nil {
			t.Fatal(err)
		}
		got, found, err := store.LatestProcess(t.Context(), "owner", "session")
		if err != nil || !found || got.Intent.LeaseID != expectedID {
			t.Fatalf("process=%+v found=%v err=%v", got, found, err)
		}
		if err := cfg.Host.Finish(t.Context(), intent, nil); err != nil {
			t.Fatal(err)
		}
		return NewAgentClient(o)
	}))
	m.SetSandboxPolicyReceiptStore(store)
	if err := m.SetSandboxProcessSupervisor(SandboxProcessSupervisor{Root: h.root, HelperPath: "/trusted/bootstrap", HelperSHA256: intent.HelperSHA256}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.GetOrCreate(t.Context(), "session", options); err == nil || calls != 0 {
		t.Fatalf("missing lease admitted: %v calls=%d", err, calls)
	}
	startup, err := m.BeginClientStartup(t.Context(), "session", "owner")
	if err != nil {
		t.Fatal(err)
	}
	defer startup.Close()
	if _, err := startup.GetOrCreateWithLease(t.Context(), options, nil, lease); err != nil {
		t.Fatal(err)
	}
	if consumed, err := startup.BindSandboxLease(nil); err == nil || consumed {
		t.Fatal("different handle accepted")
	}
	if !lease.active() {
		t.Fatal("failed transfer released caller lease")
	}
	if consumed, err := startup.BindSandboxLease(lease); err != nil || !consumed {
		t.Fatalf("exact transfer=%v %v", consumed, err)
	}
	if _, err := startup.GetOrCreateWithLease(t.Context(), options, nil, nil); err == nil || calls != 1 {
		t.Fatalf("warm client bypassed lease admission: %v calls=%d", err, calls)
	}
	if err := lease.Release(); err != nil {
		t.Fatal(err)
	}
	if _, err := captured.ProcessSupervision(t.Context(), supervision.Runtime); err == nil {
		t.Fatal("released lease permitted launch")
	}
}

func TestSupervisedLeaseRejectsForeignOrUncertainResource(t *testing.T) {
	lease, err := Acquire(t.Context(), Input{OwnerUserID: "owner", SessionKey: "session", Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release()
	options := bridge.Options{Runtime: bridge.RuntimeOptions{Kind: bridge.RuntimeNXS}, Sandbox: &bridge.SandboxSettings{Resources: lease.Resources()}}
	for _, scope := range [][2]string{{"other", "session"}, {"owner", "other"}} {
		if _, err := supervisedLeaseIdentity(options, scope[0], scope[1], lease); err == nil {
			t.Fatal("foreign lease accepted", scope)
		}
	}
	changed := *options.Sandbox.Resources
	changed.ScratchRoot += "-other"
	options.Sandbox.Resources = &changed
	if _, err := supervisedLeaseIdentity(options, "owner", "session", lease); err == nil {
		t.Fatal("different policy accepted")
	}
	options.Sandbox.Resources = lease.Resources()
	if err := lease.MarkCleanupUncertain(errors.New("test uncertain cleanup")); err != nil {
		t.Fatal(err)
	}
	if _, err := supervisedLeaseIdentity(options, "owner", "session", lease); err == nil {
		t.Fatal("unknown cleanup accepted")
	}
}
