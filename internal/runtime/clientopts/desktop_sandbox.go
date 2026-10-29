// INPUT: Desktop host rollout flag, approval mode and explicitly mounted directories.
// OUTPUT: 必需命令、原生文件、搜索、媒体、Skill、指令上下文与受控设置写入合同；其余路径保留既有选项。
// POS: Common DM/Room/background sandbox policy assembly for macOS and Windows (SDK MXC); backend negotiation is Bridge-owned.
package clientopts

import (
	"fmt"
	"maps"
	"runtime"
	"slices"
	"strings"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func applyDesktopSandbox(options agentclient.Options, input AgentClientOptionsInput) (agentclient.Options, error) {
	return applyDesktopSandboxForPlatform(options, input, runtime.GOOS)
}

// desktopSandboxProcessElevated 是可替换的提权检测入口，测试据此覆盖拒绝分支。
var desktopSandboxProcessElevated = windowsDesktopSandboxIsElevated

func applyDesktopSandboxForPlatform(options agentclient.Options, input AgentClientOptionsInput, platform string) (agentclient.Options, error) {
	options.Env = maps.Clone(options.Env)
	delete(options.Env, protocol.NexusDesktopSandboxPolicyEnvName)
	if !input.DesktopSandboxEnabled || !strings.EqualFold(strings.TrimSpace(input.AppMode), "desktop") {
		return options, nil
	}
	if platform != "darwin" && platform != "windows" {
		return agentclient.Options{}, fmt.Errorf("desktop sandbox is unsupported on %s", platform)
	}
	if platform == "windows" && options.Runtime.Kind == agentclient.RuntimeNXS {
		// Windows nxs runs on the SDK MXC ProcessContainer backend. Full Access
		// carries no restricted resource contract and projects no sandbox policy;
		// restricted sessions prepare a confined workspace scratch without recovery records.
		if options.Runtime.PermissionMode == sdkpermission.ModeBypassPermissions {
			return options, nil
		}
		// PSEC 初始化包含项目发现；仅为未显式配置的受限 Windows 会话扩展预算。
		if options.Runtime.InitializeTimeout == 0 {
			options.Runtime.InitializeTimeout = 3 * time.Minute
		}
		if desktopSandboxProcessElevated() {
			return agentclient.Options{}, fmt.Errorf("Windows 桌面沙箱不能在管理员权限下运行；请以普通用户权限启动 Nexus。也可以前往「设置 → 沙箱」关闭桌面沙箱，再重新发起会话继续使用 Nexus；关闭后，命令与文件工具将不再受桌面沙箱隔离。")
		}
	}
	if options.Env == nil {
		options.Env = make(map[string]string)
	}
	options.Env[protocol.NexusDesktopSandboxPolicyEnvName] = "1"
	if options.Runtime.Kind == agentclient.RuntimeClaude {
		// Claude's native command sandbox is a separate Bridge contract. Full
		// Access is an explicit user exception; restricted Claude keeps Bash and
		// build commands but fails closed when Claude cannot install its OS
		// sandbox. Native Claude sandbox is not supported by the Windows CLI.
		if options.Runtime.PermissionMode == sdkpermission.ModeBypassPermissions {
			return options, nil
		}
		if platform != "darwin" {
			return agentclient.Options{}, fmt.Errorf("Claude 原生命令沙箱在 %s 上不可用；请切换 nxs 或配置受支持的 Claude 沙箱环境", platform)
		}
		settings, err := cloneClaudeSandboxSettings(options.Sandbox)
		if err != nil {
			return agentclient.Options{}, err
		}
		yes := true
		no := false
		settings.RequireClaudeNativeSandbox = true
		settings.Enabled = &yes
		settings.FailIfUnavailable = &yes
		settings.AutoAllowBashIfSandboxed = &yes
		settings.AllowUnsandboxedCommands = &no
		filesystem := settings.Filesystem
		if filesystem == nil {
			filesystem = &agentclient.SandboxFilesystemConfig{}
		}
		filesystem.AllowRead = appendDistinctStrings(filesystem.AllowRead, input.SkillDirectories...)
		filesystem.AllowWrite = appendDistinctStrings(filesystem.AllowWrite, input.AdditionalDirectories...)
		settings.Filesystem = filesystem
		options.Sandbox = settings
		return options, nil
	}
	if options.Runtime.Kind != agentclient.RuntimeNXS {
		return agentclient.Options{}, fmt.Errorf("desktop sandbox requires a supported runtime contract")
	}
	yes := true
	options.MCP.StrictConfig = true
	options.Sandbox = &agentclient.SandboxSettings{
		RequireSandbox: true, RequireFileTools: true, RequireSearchTools: true, RequireMediaFiles: true, RequireMediaNetwork: true, RequireMCPNetwork: true, RequireMCPHelpers: true, RequireMCPStdio: true, RequireNotebookFiles: true, RequireSkillFiles: true, RequireContextFiles: true, RequireProjectFiles: true, RequireManagedPolicy: true, RequireSettingsFiles: true, RequireSettingsWrites: true, Enabled: &yes, FailIfUnavailable: &yes,
		// Explicit escape still passes the independent SDK approval boundary.
		AllowUnsandboxedCommands: &yes,
		Filesystem: &agentclient.SandboxFilesystemConfig{
			AllowRead:  slices.Clone(input.SkillDirectories),
			AllowWrite: slices.Clone(input.AdditionalDirectories),
		},
		Network: &agentclient.SandboxNetworkConfig{AllowedDomains: []string{}},
	}
	if platform == "windows" {
		resources, err := prepareWindowsSandboxResources(input.WorkspacePath)
		if err != nil {
			return agentclient.Options{}, err
		}
		options.Sandbox.Resources = resources
		no := false
		options.Sandbox.AllowUnsandboxedCommands = &no
	}
	// Full Access changes the command/file resource policy, but the nxs
	// runtime, capability handshake and lifecycle remain installed. Carry the
	// explicit escape in the SDK setting instead of dropping the environment.
	if options.Runtime.PermissionMode == sdkpermission.ModeBypassPermissions {
		options.Sandbox.Filesystem = &agentclient.SandboxFilesystemConfig{}
	}
	return options, nil
}

// cloneClaudeSandboxSettings preserves ordinary Claude sandbox presentation
// settings while refusing to mix nxs-only capability requirements or host
// resource contracts into the native --restricted launch path.
func cloneClaudeSandboxSettings(input *agentclient.SandboxSettings) (*agentclient.SandboxSettings, error) {
	if input == nil {
		return &agentclient.SandboxSettings{}, nil
	}
	if input.Resources != nil || input.RequireSandbox || input.RequireFileTools || input.RequireSearchTools || input.RequireMediaFiles || input.RequireMediaNetwork || input.RequireMCPNetwork || input.RequireMCPHelpers || input.RequireMCPStdio || input.RequireNotebookFiles || input.RequireSkillFiles || input.RequireContextFiles || input.RequireProjectFiles || input.RequireManagedPolicy || input.RequireSettingsFiles || input.RequireSettingsWrites {
		return nil, fmt.Errorf("Claude native restricted contract cannot carry nxs sandbox requirements")
	}
	if input.RequireClaudeRestricted {
		return nil, fmt.Errorf("Claude native sandbox cannot carry --restricted tool mode")
	}
	settings := *input
	settings.EnabledPlatforms = slices.Clone(input.EnabledPlatforms)
	settings.ExcludedCommands = slices.Clone(input.ExcludedCommands)
	if input.Filesystem != nil {
		filesystem := *input.Filesystem
		filesystem.AllowWrite = slices.Clone(input.Filesystem.AllowWrite)
		filesystem.DenyWrite = slices.Clone(input.Filesystem.DenyWrite)
		filesystem.DenyRead = slices.Clone(input.Filesystem.DenyRead)
		filesystem.AllowRead = slices.Clone(input.Filesystem.AllowRead)
		settings.Filesystem = &filesystem
	}
	if input.Network != nil {
		network := *input.Network
		network.AllowedDomains = slices.Clone(input.Network.AllowedDomains)
		network.DeniedDomains = slices.Clone(input.Network.DeniedDomains)
		network.AllowUnixSockets = slices.Clone(input.Network.AllowUnixSockets)
		network.AllowMachLookup = slices.Clone(input.Network.AllowMachLookup)
		if input.Network.MITMProxy != nil {
			mitm := *input.Network.MITMProxy
			mitm.Domains = slices.Clone(input.Network.MITMProxy.Domains)
			network.MITMProxy = &mitm
		}
		settings.Network = &network
	}
	if input.IgnoreViolations != nil {
		settings.IgnoreViolations = make(map[string][]string, len(input.IgnoreViolations))
		for key, values := range input.IgnoreViolations {
			settings.IgnoreViolations[key] = slices.Clone(values)
		}
	}
	if input.Extra != nil {
		settings.Extra = maps.Clone(input.Extra)
	}
	return &settings, nil
}
