package clientopts

import (
	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWindowsSandboxStatelessResources(t *testing.T) {
	t.Setenv(appfs.NexusStateRootEnvName, t.TempDir())
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
	if !strings.HasPrefix(scratch, filepath.Join(appfs.UserRuntimeRoot(""), "tmp", "sandbox")+string(filepath.Separator)) {
		t.Fatal(scratch)
	}
	for _, relative := range []string{"home/AppData/Local", "home/AppData/Roaming"} {
		if info, err := os.Stat(filepath.Join(appfs.UserRuntimeRoot(""), filepath.FromSlash(relative))); err != nil || !info.IsDir() {
			t.Fatalf("profile unavailable: %s: %v", relative, err)
		}
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
	t.Setenv(appfs.NexusStateRootEnvName, t.TempDir())
	if err := appfs.EnsureUserRuntimeLayout(""); err != nil {
		t.Fatal(err)
	}
	workspace, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(appfs.UserRuntimeRoot(""), "tmp", "sandbox")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := prepareWindowsSandboxResources(workspace, ""); err == nil {
		t.Fatal("accepted linked scratch parent")
	}
	if entries, err := os.ReadDir(outside); err != nil || len(entries) != 0 {
		t.Fatal("wrote outside runtime root")
	}
}

func TestWindowsSandboxRejectsElevatedProcess(t *testing.T) {
	old := desktopSandboxProcessElevated
	desktopSandboxProcessElevated = func() bool { return true }
	defer func() { desktopSandboxProcessElevated = old }()
	_, err := applyDesktopSandboxForPlatform(agentclient.Options{Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeNXS}}, AgentClientOptionsInput{AppMode: "desktop", DesktopSandboxEnabled: true, WorkspacePath: t.TempDir()}, "windows")
	if err == nil {
		t.Fatal("elevated process accepted")
	}
}

func TestWindowsProtectionMissingParentRemainsDenied(t *testing.T) {
	home := t.TempDir()
	missing := filepath.Join(home, "missing", "nested", "secret")
	roots, err := windowsProtectionRoots([]string{missing})
	if err != nil || len(roots) != 1 || roots[0] != filepath.Join(home, "missing") {
		t.Fatalf("roots=%v err=%v", roots, err)
	}
	if _, err := os.Stat(filepath.Join(home, "missing")); !os.IsNotExist(err) {
		t.Fatal("protection created a user directory")
	}
}
