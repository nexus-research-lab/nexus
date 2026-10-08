// INPUT: 真实进程/策略仓储、可控 client 与重复 warm startup。
// OUTPUT: 每轮策略保留原 launch，缺失监督记录阻断 Connect。
// POS: Manager 到持久身份的集成测试；不替代原生隔离验收。
package runtime

import (
	"testing"
	"time"

	bridge "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/supervision"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestSupervisedReceiptKeepsOriginalProcessAcrossWarmStartup(t *testing.T) {
	h, store, intent := newProcessHostFixture(t)
	var original protocol.SandboxProcessKey
	starts := 0
	factory := runtimeFactoryFunc(func(options bridge.Options) Client {
		starts++
		cfg, err := options.ProcessSupervision(t.Context(), supervision.Runtime)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := cfg.Host.Reserve(t.Context(), intent); err != nil {
			t.Fatal(err)
		}
		process, found, err := store.RuntimeProcess(t.Context(), "owner", "session", 1)
		if err != nil || !found {
			t.Fatalf("process missing %v", err)
		}
		original = process.Intent.Key
		reg := protocol.SandboxProcessRegistration{Version: 1, BootID: process.Intent.BootID, OwnerUID: process.Intent.OwnerUID, CoalitionID: 91}
		if err := store.RegisterProcess(t.Context(), original, reg); err != nil {
			t.Fatal(err)
		}
		if err := store.ClaimProcessRelease(t.Context(), original); err != nil {
			t.Fatal(err)
		}
		return &receiptRuntimeClient{receipt: SandboxEffectivePolicyReceipt{Version: 1, RuntimeKind: bridge.RuntimeNXS, PolicyDigest: "test-policy", ConfirmedAt: time.Now().UTC()}}
	})
	m := NewManagerWithFactory(factory)
	m.SetSandboxPolicyReceiptStore(store)
	if err := m.SetSandboxProcessSupervisor(SandboxProcessSupervisor{Root: h.root, HelperPath: "/trusted/bootstrap", HelperSHA256: intent.HelperSHA256}); err != nil {
		t.Fatal(err)
	}
	options := bridge.Options{Env: map[string]string{"NEXUS_RUNTIME_USER_ID": "owner"}}
	for generation := uint64(1); generation <= 3; generation++ {
		connectSandboxReceiptFixture(t, m, options)
		receipt, found, err := store.Get(t.Context(), "owner", "session", generation)
		if err != nil || !found || receipt.ProcessKey == nil || *receipt.ProcessKey != original {
			t.Fatalf("generation=%d receipt=%+v found=%v err=%v", generation, receipt, found, err)
		}
		if m.sessions["session"].ProcessGeneration != 1 {
			t.Fatal("warm startup changed process generation")
		}
	}
	if starts != 1 {
		t.Fatalf("warm client restarted %d times", starts)
	}
}

func TestSupervisedReceiptRejectsMissingOriginalProcess(t *testing.T) {
	h, store, intent := newProcessHostFixture(t)
	client := &receiptRuntimeClient{receipt: SandboxEffectivePolicyReceipt{Version: 1, RuntimeKind: bridge.RuntimeNXS, PolicyDigest: "test-policy", ConfirmedAt: time.Now().UTC()}}
	m := NewManagerWithFactory(runtimeFactoryFunc(func(bridge.Options) Client { return client }))
	m.SetSandboxPolicyReceiptStore(store)
	if err := m.SetSandboxProcessSupervisor(SandboxProcessSupervisor{Root: h.root, HelperPath: "/trusted/bootstrap", HelperSHA256: intent.HelperSHA256}); err != nil {
		t.Fatal(err)
	}
	startup, err := m.BeginClientStartup(t.Context(), "session", "owner")
	if err != nil {
		t.Fatal(err)
	}
	defer startup.Close()
	if _, err := startup.GetOrCreateWithFactory(t.Context(), bridge.Options{Env: map[string]string{"NEXUS_RUNTIME_USER_ID": "owner"}}, nil); err != nil {
		t.Fatal(err)
	}
	if err := startup.Connect(t.Context()); err == nil {
		t.Fatal("confirmed policy without original process")
	}
	if _, found, err := store.Latest(t.Context(), "owner", "session"); err != nil || found {
		t.Fatalf("persisted unsupported policy: found=%v err=%v", found, err)
	}
}
