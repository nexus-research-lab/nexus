// INPUT: 不进入普通 settings JSON 的宿主沙箱要求。
// OUTPUT: 文件/搜索/媒体/Skill 能力、资源范围和 scratch 改变都会改变进程策略身份。
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
