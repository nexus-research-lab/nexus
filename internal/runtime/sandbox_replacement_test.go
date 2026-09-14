// INPUT: 同一 Session 的沙箱策略开关变化。
// OUTPUT: 旧 runtime 退出、新 runtime 接管，禁止通过热更新伪装边界变化。
// POS: 桌面沙箱接线所依赖的既有 Manager 进程替换合同回归。
package runtime

import (
	"context"
	"errors"
	"testing"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestDesktopSandboxModeChangeRetiresInsteadOfHotUpdate(t *testing.T) {
	for _, modes := range [][2]sdkpermission.Mode{{sdkpermission.ModeDefault, sdkpermission.ModeBypassPermissions}, {sdkpermission.ModeBypassPermissions, sdkpermission.ModeAuto}} {
		client := &agentClient{options: agentclient.Options{Env: map[string]string{protocol.NexusDesktopSandboxPolicyEnvName: "1"}, Runtime: agentclient.RuntimeOptions{PermissionMode: modes[0]}}}
		if err := client.SetPermissionMode(context.Background(), modes[1]); !errors.Is(err, ErrDesktopSandboxPolicyChanged) {
			t.Fatalf("transition did not require replacement: %v", err)
		}
		if !client.retired || client.options.Runtime.PermissionMode != modes[0] {
			t.Fatal("old process was hot-updated or remained usable")
		}
		if err := client.Connect(context.Background()); !errors.Is(err, agentclient.ErrAborted) {
			t.Fatalf("retired client reconnected: %v", err)
		}
	}
	client := &agentClient{options: agentclient.Options{Env: map[string]string{protocol.NexusDesktopSandboxPolicyEnvName: "1"}, Runtime: agentclient.RuntimeOptions{PermissionMode: sdkpermission.ModeDefault}}}
	if err := client.SetPermissionMode(context.Background(), sdkpermission.ModeAuto); err != nil || client.retired {
		t.Fatalf("same sandbox boundary could not hot update: %v", err)
	}
}

func TestManagerHandlesSandboxModeReplacementAsExpectedTransition(t *testing.T) {
	stale := &fakeRuntimeClient{permissionModeErr: ErrDesktopSandboxPolicyChanged}
	manager := NewManagerWithFactory(&fakeRuntimeFactory{clients: []*fakeRuntimeClient{stale}})
	key := "agent:agent-a:ws:dm:policy-switch"
	if _, err := manager.GetOrCreate(context.Background(), key, agentclient.Options{}); err != nil {
		t.Fatal(err)
	}
	if err := manager.SetPermissionModeForAgent(context.Background(), "agent-a", sdkpermission.ModeBypassPermissions); err != nil {
		t.Fatal(err)
	}
	if stale.disconnectCalls != 1 || manager.HasSession(key) {
		t.Fatal("old runtime remained available")
	}
}

func TestManagerReplacesRuntimeForSandboxTransitions(t *testing.T) {
	for _, startRestricted := range []bool{false, true} {
		name := "enable"
		if startRestricted {
			name = "disable"
		}
		t.Run(name, func(t *testing.T) {
			stale, fresh := &fakeRuntimeClient{}, &fakeRuntimeClient{}
			manager := NewManagerWithFactory(&fakeRuntimeFactory{clients: []*fakeRuntimeClient{stale, fresh}})
			enabled := true
			restricted := &agentclient.SandboxSettings{Enabled: &enabled, FailIfUnavailable: &enabled}
			before := agentclient.Options{CWD: t.TempDir()}
			if startRestricted {
				before.Sandbox = restricted
			}
			key := "agent:nexus:ws:dm:sandbox-transition"
			if _, err := manager.GetOrCreate(context.Background(), key, before); err != nil {
				t.Fatal(err)
			}
			after := before
			if startRestricted {
				after.Sandbox = nil
			} else {
				after.Sandbox = restricted
			}
			got, err := manager.GetOrCreate(context.Background(), key, after)
			if err != nil {
				t.Fatal(err)
			}
			if got != fresh || stale.disconnectCalls != 1 || stale.reconfigureCalls != 0 {
				t.Fatalf("sandbox transition reused stale runtime: got=%p fresh=%p disconnect=%d reconfigure=%d", got, fresh, stale.disconnectCalls, stale.reconfigureCalls)
			}
		})
	}
}
