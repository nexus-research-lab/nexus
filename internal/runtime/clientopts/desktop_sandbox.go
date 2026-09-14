// INPUT: Desktop host rollout flag, approval mode and explicitly mounted directories.
// OUTPUT: Mandatory shell execution policy, or unchanged legacy/server options.
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
	if options.Runtime.PermissionMode == sdkpermission.ModeBypassPermissions {
		return options, nil
	}
	if options.Runtime.Kind != agentclient.RuntimeNXS {
		return agentclient.Options{}, fmt.Errorf("desktop sandbox requires a runtime with required_sandbox_v1 support")
	}
	yes := true
	options.Sandbox = &agentclient.SandboxSettings{
		RequireSandbox: true, Enabled: &yes, FailIfUnavailable: &yes,
		// Explicit escape still passes the independent SDK approval boundary.
		AllowUnsandboxedCommands: &yes,
		Filesystem: &agentclient.SandboxFilesystemConfig{
			AllowRead:  slices.Clone(input.SkillDirectories),
			AllowWrite: slices.Clone(input.AdditionalDirectories),
		},
		Network: &agentclient.SandboxNetworkConfig{AllowedDomains: []string{}},
	}
	return options, nil
}
