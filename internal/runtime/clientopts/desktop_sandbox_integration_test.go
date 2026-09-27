package clientopts

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

// This opt-in test checks host option assembly through the pinned Bridge and a
// real nxs handshake. It does not send a model request or prove tool confinement.
func TestDesktopSandboxRealRuntimeNegotiation(t *testing.T) {
	binary := os.Getenv("NEXUS_SANDBOX_TEST_BINARY")
	if binary == "" {
		t.Skip("set NEXUS_SANDBOX_TEST_BINARY to a built nxs binary")
	}
	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		t.Skip("desktop host platform required")
	}
	root := t.TempDir()
	options, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		WorkspacePath: root, RuntimeKind: "nxs", AppMode: "desktop", DesktopSandboxEnabled: true, PermissionMode: sdkpermission.ModeDefault,
		SkillDirectories: []string{t.TempDir()}, AdditionalDirectories: []string{t.TempDir()},
	})
	if err != nil {
		t.Fatal(err)
	}
	options.CLIPath = binary
	options.Env["NEXUS_CONFIG_DIR"] = filepath.Join(root, "config")
	options.Runtime.InitializeTimeout = 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	session, err := agentclient.NewSession(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer closeCancel()
		if err := session.Close(closeCtx); err != nil {
			t.Errorf("runtime cleanup failed: %v", err)
		}
	}()
	if !session.Supports(agentclient.CapabilityRequiredSandbox) || !session.Supports(agentclient.CapabilitySandboxFileTools) || !session.Supports(agentclient.CapabilitySandboxSearchTools) || !session.Supports(agentclient.CapabilitySandboxMediaFiles) || !session.Supports(agentclient.CapabilitySandboxSkillFiles) || !session.Supports(agentclient.CapabilitySandboxContextFiles) || !session.Supports(agentclient.CapabilitySandboxProjectFiles) || !session.Supports(agentclient.CapabilitySandboxManagedPolicy) || !session.Supports(agentclient.CapabilitySandboxSettingsFiles) || !session.Supports(agentclient.CapabilitySandboxSettingsWrites) {
		t.Fatal("host-required command, file, search and local-media contracts were not all acknowledged")
	}
	if !session.Supports(agentclient.CapabilitySandboxMediaNetwork) {
		t.Fatal("host-required remote-media network contract was not acknowledged")
	}
}

// TestDesktopSandboxRealRuntimeWithHostResources exercises the combination
// used by DM/Room: a host-owned scratch lease plus the required nxs sandbox
// contract. It intentionally sends no model request; the value of this test
// is proving that the Bridge accepts the complete initialize contract.
func TestDesktopSandboxRealRuntimeWithHostResources(t *testing.T) {
	binary := os.Getenv("NEXUS_SANDBOX_TEST_BINARY")
	if binary == "" {
		t.Skip("set NEXUS_SANDBOX_TEST_BINARY to a built nxs binary")
	}
	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		t.Skip("desktop host platform required")
	}
	workspace := t.TempDir()
	scratch := t.TempDir()
	options, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		WorkspacePath:         workspace,
		RuntimeKind:           "nxs",
		AppMode:               "desktop",
		DesktopSandboxEnabled: true,
		PermissionMode:        sdkpermission.ModeDefault,
		SandboxResources: &agentclient.SandboxResourcePolicy{
			Version: 1, WriteScope: agentclient.SandboxWriteScopeWorkspaceWrite, ScratchRoot: scratch,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if options.Sandbox == nil || options.Sandbox.Resources == nil || options.Sandbox.AllowUnsandboxedCommands == nil || *options.Sandbox.AllowUnsandboxedCommands {
		t.Fatalf("resource-backed options are not strict: %#v", options.Sandbox)
	}
	options.CLIPath = binary
	options.Env["NEXUS_CONFIG_DIR"] = filepath.Join(workspace, "config")
	options.Runtime.InitializeTimeout = 10 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	session, err := agentclient.NewSession(ctx, options)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		closeCtx, closeCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer closeCancel()
		if err := session.Close(closeCtx); err != nil {
			t.Errorf("runtime cleanup failed: %v", err)
		}
	}()
	if !session.Supports(agentclient.CapabilitySandboxResources) {
		t.Fatal("host resource capability was not acknowledged")
	}
}
