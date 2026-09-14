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
	if !session.Supports(agentclient.CapabilityRequiredSandbox) {
		t.Fatal("host-required execution was not acknowledged")
	}
}
