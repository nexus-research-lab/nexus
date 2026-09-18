// INPUT: 不进入普通 settings JSON 的宿主沙箱要求与进程级 Provider 所有权。
// OUTPUT: 能力、资源范围、scratch 与凭据隔离声明改变时替换进程，普通凭据更新仍可复用。
// POS: runtime 重用前的资源版本栅栏回归。
package runtime

import (
	"testing"

	bridge "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

// TestProcessPolicyIncludesHostSandboxRequirements 不因 json:"-" 丢失初始化权威。
func TestProcessPolicyIncludesHostSandboxRequirements(t *testing.T) {
	base := bridge.Options{Sandbox: &bridge.SandboxSettings{RequireSandbox: true}}
	previous := managedRuntimeProcessPolicyFingerprint(base)
	for _, change := range []func(){
		func() { base.Sandbox.RequireFileTools = true },
		func() { base.Sandbox.RequireSearchTools = true },
		func() { base.Sandbox.RequireMediaFiles = true },
		func() { base.Sandbox.RequireSkillFiles = true },
		func() { base.Sandbox.RequireContextFiles = true },
		func() { base.Sandbox.RequireProjectFiles = true },
		func() { base.Sandbox.RequireManagedPolicy = true },
		func() { base.Sandbox.RequireSettingsFiles = true },
		func() { base.Sandbox.RequireSettingsWrites = true },
		func() {
			base.Sandbox.Resources = &bridge.SandboxResourcePolicy{Version: 1, WriteScope: bridge.SandboxWriteScopeReadOnly, ScratchRoot: t.TempDir()}
		},
		func() { base.Sandbox.Resources.WriteScope = bridge.SandboxWriteScopeWorkspaceWrite },
		func() { base.Sandbox.Resources.ScratchRoot = t.TempDir() },
	} {
		change()
		next := managedRuntimeProcessPolicyFingerprint(base)
		if next == previous {
			t.Fatal("host sandbox requirement did not change process identity")
		}
		previous = next
	}
}

func TestProviderOwnershipChangeReplacesRuntime(t *testing.T) {
	for _, key := range []string{"NEXUS_PROVIDER_MANAGED_BY_HOST", "NEXUS_SUBPROCESS_ENV_SCRUB", "CLAUDE_CODE_SUBPROCESS_ENV_SCRUB", "NEXUS_AUTO_DREAM_WAKE_MODE", "NEXUS_MEMORY_DIR", "NEXUS_ENABLE_REMOTE_MEMORY", "NEXUS_REMOTE_MEMORY_DIR"} {
		t.Run(key, func(t *testing.T) {
			stale, fresh := &fakeRuntimeClient{}, &fakeRuntimeClient{}
			manager := NewManagerWithFactory(&fakeRuntimeFactory{clients: []*fakeRuntimeClient{stale, fresh}})
			base := bridge.Options{CWD: t.TempDir(), Env: map[string]string{key: "0"}}
			first, err := manager.GetOrCreate(t.Context(), "agent:nexus:ws:dm:provider-policy", base)
			if err != nil {
				t.Fatal(err)
			}
			next := base
			next.Env = map[string]string{key: "1"}
			if key == "NEXUS_AUTO_DREAM_WAKE_MODE" {
				next.Env[key] = "host"
			}
			second, err := manager.GetOrCreate(t.Context(), "agent:nexus:ws:dm:provider-policy", next)
			if err != nil {
				t.Fatal(err)
			}
			if first != stale || second != fresh || stale.reconfigureCalls != 0 || stale.disconnectCalls != 1 {
				t.Fatal("process-owned environment change reused the old runtime")
			}
		})
	}
}
