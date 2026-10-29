package runtime

import (
	goruntime "runtime"
	"strings"
	"testing"

	bridge "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

func TestDesktopSandboxOptionsRequireMCPContractOnEveryPlatform(t *testing.T) {
	yes := true
	settings := &bridge.SandboxSettings{
		RequireSandbox: true, RequireFileTools: true, RequireSearchTools: true, RequireMediaFiles: true, RequireMediaNetwork: true, RequireMCPNetwork: true, RequireMCPHelpers: true, RequireMCPStdio: true, RequireNotebookFiles: true, RequireSkillFiles: true, RequireContextFiles: true, RequireProjectFiles: true, RequireManagedPolicy: true, RequireSettingsFiles: true, RequireSettingsWrites: true, Enabled: &yes, FailIfUnavailable: &yes,
	}
	if goruntime.GOOS == "windows" {
		settings.Resources = &bridge.SandboxResourcePolicy{Version: 1}
	}
	options := bridge.Options{Runtime: bridge.RuntimeOptions{Kind: bridge.RuntimeNXS}, Sandbox: settings}
	options.MCP.StrictConfig = true
	if err := validateDesktopSandboxOptions(options); err != nil {
		t.Fatalf("complete contract rejected: %v", err)
	}
	if got := len(requiredSandboxCapabilities(options)); got != len(nxsSandboxRequirements) {
		t.Fatalf("required %d capabilities, want %d", got, len(nxsSandboxRequirements))
	}
	options.MCP.StrictConfig = false
	if err := validateDesktopSandboxOptions(options); err == nil || !strings.Contains(err.Error(), "mcp_strict_config") {
		t.Fatalf("missing strict MCP config accepted: %v", err)
	}
	options.MCP.StrictConfig = true
	settings.RequireMCPStdio = false
	if err := validateDesktopSandboxOptions(options); err == nil || !strings.Contains(err.Error(), "mcp_stdio") {
		t.Fatalf("missing MCP stdio requirement accepted: %v", err)
	}
}
