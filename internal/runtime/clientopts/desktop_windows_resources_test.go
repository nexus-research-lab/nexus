package clientopts

import (
	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWindowsSandboxStatelessResources(t *testing.T) {
	old := desktopSandboxProcessElevated
	desktopSandboxProcessElevated = func() bool { return false }
	defer func() { desktopSandboxProcessElevated = old }()
	workspace := t.TempDir()
	input := AgentClientOptionsInput{AppMode: "desktop", DesktopSandboxEnabled: true, WorkspacePath: workspace}
	options, err := applyDesktopSandboxForPlatform(agentclient.Options{Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeNXS}}, input, "windows")
	if err != nil {
		t.Fatal(err)
	}
	if options.Sandbox == nil || options.Sandbox.Resources == nil || !options.MCP.StrictConfig || !options.Sandbox.RequireMCPStdio {
		t.Fatal("missing Windows contract")
	}
	if options.Runtime.InitializeTimeout != 3*time.Minute {
		t.Fatal("missing initialization budget")
	}
	scratch := options.Sandbox.Resources.ScratchRoot
	if scratch != filepath.Join(workspace, ".nexus", "sandbox-tmp") {
		t.Fatal(scratch)
	}
	if info, err := os.Stat(scratch); err != nil || !info.IsDir() {
		t.Fatalf("scratch unavailable: %v", err)
	}
	full, err := applyDesktopSandboxForPlatform(agentclient.Options{Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeNXS, PermissionMode: sdkpermission.ModeBypassPermissions}}, input, "windows")
	if err != nil || full.Sandbox != nil {
		t.Fatalf("Full Access installed resources: %v", err)
	}
}
func TestWindowsSandboxScratchRejectsLink(t *testing.T) {
	workspace, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(workspace, ".nexus")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := prepareWindowsSandboxResources(workspace); err == nil {
		t.Fatal("accepted linked scratch parent")
	}
	if _, err := os.Stat(filepath.Join(outside, "sandbox-tmp")); !os.IsNotExist(err) {
		t.Fatal("wrote outside workspace")
	}
}
