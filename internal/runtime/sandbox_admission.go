// INPUT: 已初始化的 Bridge session 与宿主沙箱选项。
// OUTPUT: 桌面沙箱 runtime 是否完整确认了宿主要求的沙箱能力。
// POS: Connect 后的 fail-closed 准入；不持久化，也不把能力协商当成 OS 隔离证明。
package runtime

import (
	"errors"
	"fmt"
	"runtime"
	"strings"

	bridge "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

// validateDesktopSandboxConnection 在新 session 投入使用前核对能力确认；
// 非桌面沙箱 runtime 直接放行。
func validateDesktopSandboxConnection(session *bridge.Session, options bridge.Options) error {
	if session == nil {
		return errors.New("desktop sandbox admission requires a connected session")
	}
	if options.Env[protocol.NexusDesktopSandboxPolicyEnvName] != "1" {
		return nil
	}
	if err := validateDesktopSandboxOptions(options); err != nil {
		return err
	}
	for _, capability := range requiredSandboxCapabilities(options) {
		if !session.Supports(capability) {
			return fmt.Errorf("desktop sandbox capability was not acknowledged: %s", capability)
		}
	}
	return nil
}

func sandboxRuntimeKind(kind bridge.RuntimeKind) bridge.RuntimeKind {
	switch strings.ToLower(strings.TrimSpace(string(kind))) {
	case "", "nxs", "go", "go-native", "gonative":
		return bridge.RuntimeNXS
	case "claude", "claude-code", "claudecode", "cc":
		return bridge.RuntimeClaude
	default:
		return bridge.RuntimeKind(strings.ToLower(strings.TrimSpace(string(kind))))
	}
}

func requiredSandboxCapabilities(options bridge.Options) []bridge.Capability {
	if options.Sandbox == nil {
		return nil
	}
	result := make([]bridge.Capability, 0, 12)
	if sandboxRuntimeKind(options.Runtime.Kind) == bridge.RuntimeNXS {
		if options.Sandbox.RequireSandbox {
			result = append(result, bridge.CapabilityRequiredSandbox)
		}
		if options.Sandbox.RequireFileTools {
			result = append(result, bridge.CapabilitySandboxFileTools)
		}
		if options.Sandbox.RequireSearchTools {
			result = append(result, bridge.CapabilitySandboxSearchTools)
		}
		if options.Sandbox.RequireMediaFiles {
			result = append(result, bridge.CapabilitySandboxMediaFiles)
		}
		if options.Sandbox.RequireMCPNetwork {
			result = append(result, bridge.CapabilitySandboxMCPNetwork)
		}
		if options.Sandbox.RequireMCPHelpers {
			result = append(result, bridge.CapabilitySandboxMCPHelpers)
		}
		if options.Sandbox.RequireMCPStdio {
			result = append(result, bridge.CapabilitySandboxMCPStdio)
		}
		if options.Sandbox.RequireMediaNetwork {
			result = append(result, bridge.CapabilitySandboxMediaNetwork)
		}
		if options.Sandbox.RequireNotebookFiles {
			result = append(result, bridge.CapabilitySandboxNotebookFiles)
		}
		if options.Sandbox.RequireSkillFiles {
			result = append(result, bridge.CapabilitySandboxSkillFiles)
		}
		if options.Sandbox.RequireContextFiles {
			result = append(result, bridge.CapabilitySandboxContextFiles)
		}
		if options.Sandbox.RequireProjectFiles {
			result = append(result, bridge.CapabilitySandboxProjectFiles)
		}
		if options.Sandbox.RequireManagedPolicy {
			result = append(result, bridge.CapabilitySandboxManagedPolicy)
		}
		if options.Sandbox.RequireSettingsFiles {
			result = append(result, bridge.CapabilitySandboxSettingsFiles)
		}
		if options.Sandbox.RequireSettingsWrites {
			result = append(result, bridge.CapabilitySandboxSettingsWrites)
		}
	} else if sandboxRuntimeKind(options.Runtime.Kind) == bridge.RuntimeClaude && options.Sandbox.RequireClaudeNativeSandbox {
		result = append(result, bridge.CapabilityClaudeNativeSandbox)
	} else if sandboxRuntimeKind(options.Runtime.Kind) == bridge.RuntimeClaude && options.Sandbox.RequireClaudeRestricted {
		result = append(result, bridge.CapabilityClaudeRestricted)
	}
	return result
}

// validateDesktopSandboxOptions rejects a hand-built desktop nxs configuration
// that omits part of the host's required sandbox capability contract.
// Production option assembly sets every field below; this check is the
// fail-closed boundary for alternate callers and tests.
func validateDesktopSandboxOptions(options bridge.Options) error {
	runtimeKind := sandboxRuntimeKind(options.Runtime.Kind)
	if runtimeKind != bridge.RuntimeNXS && runtimeKind != bridge.RuntimeClaude {
		return fmt.Errorf("desktop sandbox runtime kind is unsupported: %q", strings.TrimSpace(string(options.Runtime.Kind)))
	}
	if runtimeKind != bridge.RuntimeNXS {
		return nil
	}
	settings := options.Sandbox
	if settings == nil {
		return errors.New("desktop nxs sandbox settings are missing")
	}
	missing := make([]string, 0, 12)
	if !settings.RequireSandbox {
		missing = append(missing, "sandbox")
	}
	if !settings.RequireFileTools {
		missing = append(missing, "file_tools")
	}
	if !settings.RequireSearchTools {
		missing = append(missing, "search_tools")
	}
	if !settings.RequireMediaFiles {
		missing = append(missing, "media_files")
	}
	if runtime.GOOS == "darwin" && (!settings.RequireMCPNetwork || !options.MCP.StrictConfig) {
		missing = append(missing, "mcp_network")
	}
	if runtime.GOOS == "darwin" && (!settings.RequireMCPHelpers || !options.MCP.StrictConfig) {
		missing = append(missing, "mcp_helpers")
	}
	if runtime.GOOS == "darwin" && (!settings.RequireMCPStdio || !options.MCP.StrictConfig) {
		missing = append(missing, "mcp_stdio")
	}
	if !settings.RequireMediaNetwork {
		missing = append(missing, "media_network")
	}
	if !settings.RequireNotebookFiles {
		missing = append(missing, "notebook_files")
	}
	if !settings.RequireSkillFiles {
		missing = append(missing, "skill_files")
	}
	if !settings.RequireContextFiles {
		missing = append(missing, "context_files")
	}
	if !settings.RequireProjectFiles {
		missing = append(missing, "project_files")
	}
	if !settings.RequireManagedPolicy {
		missing = append(missing, "managed_policy")
	}
	if !settings.RequireSettingsFiles {
		missing = append(missing, "settings_files")
	}
	if !settings.RequireSettingsWrites {
		missing = append(missing, "settings_writes")
	}
	if settings.Enabled == nil || !*settings.Enabled {
		missing = append(missing, "enabled")
	}
	if settings.FailIfUnavailable == nil || !*settings.FailIfUnavailable {
		missing = append(missing, "fail_if_unavailable")
	}
	if len(missing) != 0 {
		return fmt.Errorf("desktop nxs sandbox contract is incomplete: missing %s", strings.Join(missing, ", "))
	}
	return nil
}
