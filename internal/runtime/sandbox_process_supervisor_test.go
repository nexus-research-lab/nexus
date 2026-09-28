// INPUT: 真实启动仓储、Manager factory、四种启动目的与 client 热更新。
// OUTPUT: factory 前冻结 exact 代次、未收口 probe 阻断新 factory、热更新保留监督。
// POS: Manager 监督装配验证，不替代 App 初始化与 helper 打包。
package runtime

import (
	"context"
	"errors"
	"fmt"
	"testing"

	bridge "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	"github.com/nexus-research-lab/nexus-agent-sdk-bridge/supervision"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestSandboxProcessSupervisorBindsBeforeFactory(t *testing.T) {
	h, store, intent := newProcessHostFixture(t)
	config := SandboxProcessSupervisor{Root: h.root, HelperPath: "/trusted/bootstrap", HelperSHA256: intent.HelperSHA256}
	ctx := t.Context()
	var savedOptions bridge.Options
	manager := NewManagerWithFactory(runtimeFactoryFunc(func(o bridge.Options) Client { savedOptions = o; return &fakeRuntimeClient{} }))
	manager.SetSandboxPolicyReceiptStore(store)
	if err := manager.SetSandboxProcessSupervisor(config); err != nil {
		t.Fatal(err)
	}
	options := bridge.Options{Runtime: bridge.RuntimeOptions{Kind: bridge.RuntimeClaude}, Env: map[string]string{"NEXUS_RUNTIME_USER_ID": "owner"}}
	if _, err := manager.GetOrCreate(ctx, "session", options); err != nil {
		t.Fatal(err)
	}
	if savedOptions.ProcessSupervision == nil {
		t.Fatal("factory did not receive supervision")
	}
	for n, purpose := range []supervision.Purpose{supervision.ClaudeSandboxProbe, supervision.ClaudeRestrictedProbe, supervision.VersionProbe, supervision.Runtime} {
		cfg, err := savedOptions.ProcessSupervision(ctx, purpose)
		if err != nil {
			t.Fatal(err)
		}
		i := intent
		i.ID = fmt.Sprintf("%032x", n+1)
		i.JobLabel = "cn.nexus.runtime." + i.ID
		if _, err := cfg.Host.Reserve(ctx, i); err != nil {
			t.Fatal(err)
		}
		got, found, err := store.LatestProcess(ctx, "owner", "session")
		if err != nil || !found || got.Intent.Key.Generation != manager.sessions["session"].StartupGeneration || got.Intent.Purpose != protocol.SandboxProcessPurpose(purpose) || got.Intent.RuntimeKind != "claude" {
			t.Fatalf("binding=%+v %v %v", got, found, err)
		}
		if err := cfg.Host.Finish(ctx, i, nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := manager.SetSandboxProcessSupervisor(config); err == nil {
		t.Fatal("changed live supervisor")
	}
	// 重建 Manager 使用已完成 generation 下界，不能把 probe 身份当新会话。
	restarted := NewManagerWithFactory(runtimeFactoryFunc(func(o bridge.Options) Client { savedOptions = o; return &fakeRuntimeClient{} }))
	restarted.SetSandboxPolicyReceiptStore(store)
	if err := restarted.SetSandboxProcessSupervisor(config); err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.GetOrCreate(ctx, "session", options); err != nil {
		t.Fatal(err)
	}
	cfg, err := savedOptions.ProcessSupervision(ctx, supervision.ClaudeSandboxProbe)
	if err != nil {
		t.Fatal(err)
	}
	intent.ID = fmt.Sprintf("%032x", 10)
	intent.JobLabel = "cn.nexus.runtime." + intent.ID
	if _, err := cfg.Host.Reserve(ctx, intent); err != nil {
		t.Fatal(err)
	}
	got, _, err := store.LatestProcess(ctx, "owner", "session")
	if err != nil || got.Intent.Key.Generation != 2 {
		t.Fatalf("restart=%+v %v", got, err)
	}
	calls := 0
	blocked := NewManagerWithFactory(runtimeFactoryFunc(func(bridge.Options) Client { calls++; return &fakeRuntimeClient{} }))
	blocked.SetSandboxPolicyReceiptStore(store)
	if err := blocked.SetSandboxProcessSupervisor(config); err != nil {
		t.Fatal(err)
	}
	if _, err := blocked.GetOrCreate(ctx, "session", options); !errors.Is(err, ErrSandboxCleanupPending) || calls != 0 {
		t.Fatalf("probe bypass: %v calls=%d", err, calls)
	}
	if err := cfg.Host.Finish(ctx, intent, nil); err != nil {
		t.Fatal(err)
	}
}

func TestSandboxProcessSupervisorPersistsAcrossReconfigure(t *testing.T) {
	calls := 0
	original := supervision.Factory(func(context.Context, supervision.Purpose) (supervision.Config, error) {
		calls++
		return supervision.Config{}, nil
	})
	client := NewAgentClient(bridge.Options{ProcessSupervision: original}).(*agentClient)
	for _, replacement := range []supervision.Factory{nil, func(context.Context, supervision.Purpose) (supervision.Config, error) {
		t.Fatal("replacement supervisor invoked")
		return supervision.Config{}, nil
	}} {
		if err := client.Reconfigure(t.Context(), bridge.Options{ProcessSupervision: replacement}); err != nil {
			t.Fatal(err)
		}
		client.mu.Lock()
		factory := client.options.ProcessSupervision
		client.mu.Unlock()
		if factory == nil {
			t.Fatal("hot update removed supervisor")
		}
		if _, err := factory(t.Context(), supervision.Runtime); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 2 {
		t.Fatalf("original calls=%d", calls)
	}
}

func TestSandboxProcessSupervisorRejectsRequestedFactory(t *testing.T) {
	h, store, i := newProcessHostFixture(t)
	m := NewManager()
	m.SetSandboxPolicyReceiptStore(store)
	if err := m.SetSandboxProcessSupervisor(SandboxProcessSupervisor{Root: h.root, HelperPath: "/trusted/bootstrap", HelperSHA256: i.HelperSHA256}); err != nil {
		t.Fatal(err)
	}
	injected := bridge.Options{ProcessSupervision: func(context.Context, supervision.Purpose) (supervision.Config, error) {
		t.Fatal("untrusted factory called")
		return supervision.Config{}, nil
	}}
	if _, err := m.supervisedProcessOptions(injected, "owner", "session", 0); err == nil {
		t.Fatal("request replaced supervisor")
	}
	options, err := m.supervisedProcessOptions(bridge.Options{}, "owner", "session", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := options.ProcessSupervision(t.Context(), supervision.Purpose("invented")); err == nil {
		t.Fatal("unknown purpose accepted")
	}
}

func TestSandboxProcessSupervisorReplacementUsesNextLiveGeneration(t *testing.T) {
	h, store, intent := newProcessHostFixture(t)
	var created []bridge.Options
	m := NewManagerWithFactory(runtimeFactoryFunc(func(o bridge.Options) Client { created = append(created, o); return &fakeRuntimeClient{} }))
	m.SetSandboxPolicyReceiptStore(store)
	if err := m.SetSandboxProcessSupervisor(SandboxProcessSupervisor{Root: h.root, HelperPath: "/trusted/bootstrap", HelperSHA256: intent.HelperSHA256}); err != nil {
		t.Fatal(err)
	}
	o := bridge.Options{Runtime: bridge.RuntimeOptions{Kind: bridge.RuntimeNXS}, Env: map[string]string{"NEXUS_RUNTIME_USER_ID": "owner"}}
	for range 2 {
		if _, err := m.GetOrCreate(t.Context(), "session", o); err != nil {
			t.Fatal(err)
		}
	}
	o.Runtime.Kind = bridge.RuntimeClaude
	if _, err := m.GetOrCreate(t.Context(), "session", o); err != nil {
		t.Fatal(err)
	}
	if len(created) != 2 {
		t.Fatalf("factories=%d", len(created))
	}
	cfg, err := created[1].ProcessSupervision(t.Context(), supervision.Runtime)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cfg.Host.Reserve(t.Context(), intent); err != nil {
		t.Fatal(err)
	}
	got, _, err := store.LatestProcess(t.Context(), "owner", "session")
	if err != nil || got.Intent.Key.Generation != 3 || m.sessions["session"].StartupGeneration != 3 {
		t.Fatalf("replacement=%+v err=%v", got, err)
	}
	if err := cfg.Host.Finish(t.Context(), intent, nil); err != nil {
		t.Fatal(err)
	}
}
