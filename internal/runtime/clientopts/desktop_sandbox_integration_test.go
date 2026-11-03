// INPUT: 隔离宿主目录、固定 Bridge 与真实 nxs。
// OUTPUT: 按正式启动前提验证受限/完全访问选项与真实 runtime 握手。
// POS: 无模型调用的宿主集成回归，不代替工具隔离或发布验收。
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
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
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
	prepareDesktopSandboxRuntimeLayout(t)
	for _, mode := range []sdkpermission.Mode{sdkpermission.ModeDefault, sdkpermission.ModeBypassPermissions} {
		t.Run(string(mode), func(t *testing.T) {
			verifyDesktopRuntimeNegotiation(t, binary, mode)
		})
	}
}

// verifyDesktopRuntimeNegotiation 验证两种模式均可启动且完全访问不会重新安装受限策略。
func verifyDesktopRuntimeNegotiation(t *testing.T, binary string, mode sdkpermission.Mode) {
	t.Helper()
	root := t.TempDir()
	options, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		WorkspacePath: root, RuntimeKind: "nxs", AppMode: "desktop", DesktopSandboxEnabled: true, PermissionMode: mode,
		SkillDirectories: []string{t.TempDir()}, AdditionalDirectories: []string{t.TempDir()},
	})
	if err != nil {
		t.Fatal(err)
	}
	if mode == sdkpermission.ModeBypassPermissions && options.Sandbox != nil {
		t.Fatal("full access installed sandbox settings")
	}
	options.CLIPath = binary
	options.Env["NEXUS_CONFIG_DIR"] = filepath.Join(root, "config")
	// 保留宿主的真实启动预算；Windows 不应被测试专有的 10 秒上限替换。
	if options.Runtime.InitializeTimeout == 0 {
		options.Runtime.InitializeTimeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), options.Runtime.InitializeTimeout+20*time.Second)
	defer cancel()
	started := time.Now()
	session, err := agentclient.NewSession(ctx, options)
	t.Logf("native %s initialization: %s (budget %s)", mode, time.Since(started), options.Runtime.InitializeTimeout)
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
	if mode == sdkpermission.ModeBypassPermissions {
		return
	}
	if !session.Supports(agentclient.CapabilityRequiredSandbox) || !session.Supports(agentclient.CapabilitySandboxFileTools) || !session.Supports(agentclient.CapabilitySandboxSearchTools) || !session.Supports(agentclient.CapabilitySandboxMediaFiles) || !session.Supports(agentclient.CapabilitySandboxSkillFiles) || !session.Supports(agentclient.CapabilitySandboxContextFiles) || !session.Supports(agentclient.CapabilitySandboxProjectFiles) || !session.Supports(agentclient.CapabilitySandboxManagedPolicy) || !session.Supports(agentclient.CapabilitySandboxSettingsFiles) || !session.Supports(agentclient.CapabilitySandboxSettingsWrites) {
		t.Fatal("host-required command, file, search and local-media contracts were not all acknowledged")
	}
	if !session.Supports(agentclient.CapabilitySandboxMediaNetwork) {
		t.Fatal("host-required remote-media network contract was not acknowledged")
	}
}

// prepareDesktopSandboxRuntimeLayout 复用正式 Agent ready 阶段的目录准备，夹具不借用用户状态。
func prepareDesktopSandboxRuntimeLayout(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, root)
	if err := appfs.EnsureUserRuntimeLayoutAt(root, "__system__"); err != nil {
		t.Fatal(err)
	}
}
