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

// nxsSandboxRequirements 是桌面 nxs 合同的唯一清单：设置开关、协商能力与缺失时的诊断名。
var nxsSandboxRequirements = []struct {
	name       string
	required   func(*bridge.SandboxSettings) bool
	capability bridge.Capability
}{
	{"sandbox", func(s *bridge.SandboxSettings) bool { return s.RequireSandbox }, bridge.CapabilityRequiredSandbox},
	{"file_tools", func(s *bridge.SandboxSettings) bool { return s.RequireFileTools }, bridge.CapabilitySandboxFileTools},
	{"search_tools", func(s *bridge.SandboxSettings) bool { return s.RequireSearchTools }, bridge.CapabilitySandboxSearchTools},
	{"media_files", func(s *bridge.SandboxSettings) bool { return s.RequireMediaFiles }, bridge.CapabilitySandboxMediaFiles},
	{"mcp_network", func(s *bridge.SandboxSettings) bool { return s.RequireMCPNetwork }, bridge.CapabilitySandboxMCPNetwork},
	{"mcp_helpers", func(s *bridge.SandboxSettings) bool { return s.RequireMCPHelpers }, bridge.CapabilitySandboxMCPHelpers},
	{"mcp_stdio", func(s *bridge.SandboxSettings) bool { return s.RequireMCPStdio }, bridge.CapabilitySandboxMCPStdio},
	{"media_network", func(s *bridge.SandboxSettings) bool { return s.RequireMediaNetwork }, bridge.CapabilitySandboxMediaNetwork},
	{"notebook_files", func(s *bridge.SandboxSettings) bool { return s.RequireNotebookFiles }, bridge.CapabilitySandboxNotebookFiles},
	{"skill_files", func(s *bridge.SandboxSettings) bool { return s.RequireSkillFiles }, bridge.CapabilitySandboxSkillFiles},
	{"context_files", func(s *bridge.SandboxSettings) bool { return s.RequireContextFiles }, bridge.CapabilitySandboxContextFiles},
	{"project_files", func(s *bridge.SandboxSettings) bool { return s.RequireProjectFiles }, bridge.CapabilitySandboxProjectFiles},
	{"managed_policy", func(s *bridge.SandboxSettings) bool { return s.RequireManagedPolicy }, bridge.CapabilitySandboxManagedPolicy},
	{"settings_files", func(s *bridge.SandboxSettings) bool { return s.RequireSettingsFiles }, bridge.CapabilitySandboxSettingsFiles},
	{"settings_writes", func(s *bridge.SandboxSettings) bool { return s.RequireSettingsWrites }, bridge.CapabilitySandboxSettingsWrites},
}

func requiredSandboxCapabilities(options bridge.Options) []bridge.Capability {
	if options.Sandbox == nil {
		return nil
	}
	switch sandboxRuntimeKind(options.Runtime.Kind) {
	case bridge.RuntimeNXS:
		result := make([]bridge.Capability, 0, len(nxsSandboxRequirements))
		for _, requirement := range nxsSandboxRequirements {
			if requirement.required(options.Sandbox) {
				result = append(result, requirement.capability)
			}
		}
		return result
	case bridge.RuntimeClaude:
		if options.Sandbox.RequireClaudeNativeSandbox {
			return []bridge.Capability{bridge.CapabilityClaudeNativeSandbox}
		}
		if options.Sandbox.RequireClaudeRestricted {
			return []bridge.Capability{bridge.CapabilityClaudeRestricted}
		}
	}
	return nil
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
	missing := make([]string, 0, len(nxsSandboxRequirements))
	for _, requirement := range nxsSandboxRequirements {
		if !requirement.required(settings) {
			missing = append(missing, requirement.name)
		}
	}
	if !options.MCP.StrictConfig {
		missing = append(missing, "mcp_strict_config")
	}
	if runtime.GOOS == "windows" && settings.Resources == nil {
		missing = append(missing, "resources")
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
