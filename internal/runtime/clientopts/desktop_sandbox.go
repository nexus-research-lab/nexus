// INPUT: Desktop host rollout flag, approval mode and explicitly mounted directories.
// OUTPUT: 必需命令、原生文件、搜索、媒体、Skill、指令上下文与受控设置写入合同；其余路径保留既有选项。
// POS: Common DM/Room/background sandbox policy assembly; backend negotiation is Bridge-owned.
package clientopts

import (
	"fmt"
	"maps"
	"runtime"
	"slices"
	"strings"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func applyDesktopSandbox(options agentclient.Options, input AgentClientOptionsInput) (agentclient.Options, error) {
	return applyDesktopSandboxForPlatform(options, input, runtime.GOOS)
}

func applyDesktopSandboxForPlatform(options agentclient.Options, input AgentClientOptionsInput, platform string) (agentclient.Options, error) {
	options.Env = maps.Clone(options.Env)
	delete(options.Env, protocol.NexusDesktopSandboxPolicyEnvName)
	if !input.DesktopSandboxEnabled || !strings.EqualFold(strings.TrimSpace(input.AppMode), "desktop") {
		return options, nil
	}
	if platform != "darwin" && platform != "windows" {
		return agentclient.Options{}, fmt.Errorf("desktop sandbox is unsupported on %s", platform)
	}
	if options.Env == nil {
		options.Env = make(map[string]string)
	}
	options.Env[protocol.NexusDesktopSandboxPolicyEnvName] = "1"
	if options.Runtime.Kind != agentclient.RuntimeNXS {
		// Claude's native sandbox adapter has a separate contract. Until that
		// contract is negotiated here, a desktop request cannot claim the nxs
		// required-sandbox capabilities. Preserve legacy Full Access handling
		// for Claude while keeping the failure closed for restricted mode.
		if options.Runtime.PermissionMode == sdkpermission.ModeBypassPermissions {
			return options, nil
		}
		return agentclient.Options{}, fmt.Errorf("desktop sandbox requires a runtime with required_sandbox_v1 support")
	}
	yes := true
	options.Sandbox = &agentclient.SandboxSettings{
		RequireSandbox: true, RequireFileTools: true, RequireSearchTools: true, RequireMediaFiles: true, RequireSkillFiles: true, RequireContextFiles: true, RequireProjectFiles: true, RequireManagedPolicy: true, RequireSettingsFiles: true, RequireSettingsWrites: true, Enabled: &yes, FailIfUnavailable: &yes,
		// Explicit escape still passes the independent SDK approval boundary.
		AllowUnsandboxedCommands: &yes,
		Filesystem: &agentclient.SandboxFilesystemConfig{
			AllowRead:  slices.Clone(input.SkillDirectories),
			AllowWrite: slices.Clone(input.AdditionalDirectories),
		},
		Network: &agentclient.SandboxNetworkConfig{AllowedDomains: []string{}},
	}
	// Full Access changes the command/file resource policy, but the nxs
	// runtime, capability handshake and lifecycle remain installed. Carry the
	// explicit escape in the SDK setting instead of dropping the environment.
	if options.Runtime.PermissionMode == sdkpermission.ModeBypassPermissions {
		if input.SandboxResources != nil {
			return agentclient.Options{}, fmt.Errorf("Full Access cannot carry a restricted sandbox resource policy")
		}
		options.Sandbox.Filesystem = &agentclient.SandboxFilesystemConfig{}
	} else if input.SandboxResources != nil {
		resources := *input.SandboxResources
		if err := resources.Validate(); err != nil {
			return agentclient.Options{}, fmt.Errorf("invalid desktop sandbox resources: %w", err)
		}
		options.Sandbox.Resources = &resources
	}
	return options, nil
}
