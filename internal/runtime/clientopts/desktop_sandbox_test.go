// desktop_sandbox_test.go 覆盖桌面沙箱策略、宿主路径、网络准入、Windows 资源与实机协商测试。
package clientopts

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestBuildAgentClientOptionsInstallsClaudeNativeContract(t *testing.T) {
	options, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		WorkspacePath: t.TempDir(), RuntimeKind: "claude", AppMode: "desktop", DesktopSandboxEnabled: true, PermissionMode: sdkpermission.ModeDefault,
	})
	if runtime.GOOS == "windows" {
		if err == nil {
			t.Fatalf("Windows must reject the restricted Claude sandbox contract: options=%#v err=%v", options.Sandbox, err)
		}
		return
	}
	if runtime.GOOS != "darwin" {
		if err == nil {
			t.Fatal("unsupported Claude native sandbox was accepted")
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if options.Sandbox == nil || !options.Sandbox.RequireClaudeNativeSandbox {
		t.Fatalf("Claude native sandbox contract missing: %#v", options.Sandbox)
	}
	if options.Sandbox.RequireClaudeRestricted || options.Sandbox.RequireSandbox || options.Sandbox.RequireFileTools || options.Sandbox.RequireSearchTools || options.Sandbox.Enabled == nil || !*options.Sandbox.Enabled || options.Sandbox.FailIfUnavailable == nil || !*options.Sandbox.FailIfUnavailable || options.Sandbox.AllowUnsandboxedCommands == nil || *options.Sandbox.AllowUnsandboxedCommands {
		t.Fatalf("Claude claimed nxs sandbox capabilities: %#v", options.Sandbox)
	}
}

func TestDesktopSandboxPolicySeparatesGrantsAndFullAccess(t *testing.T) {
	input := AgentClientOptionsInput{AppMode: "desktop", DesktopSandboxEnabled: true,
		SkillDirectories: []string{"/skills"}, AdditionalDirectories: []string{"/mounted"}}
	for _, platform := range []string{"darwin"} {
		for _, mode := range []sdkpermission.Mode{sdkpermission.ModeDefault, sdkpermission.ModeAuto, sdkpermission.ModeBypassPermissions} {
			before := agentclient.Options{Env: map[string]string{"existing": "value"}, Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeNXS, PermissionMode: mode}}
			got, err := applyDesktopSandboxForPlatform(before, input, platform)
			if err != nil {
				t.Fatal(err)
			}
			if got.Env[protocol.NexusDesktopSandboxPolicyEnvName] != "1" || before.Env[protocol.NexusDesktopSandboxPolicyEnvName] != "" {
				t.Fatal("host policy marker missing or caller map mutated")
			}
			if got.Runtime.PermissionMode != mode {
				t.Fatal("sandbox changed approval mode")
			}
			if mode == sdkpermission.ModeBypassPermissions {
				if got.Sandbox != nil || !got.MCP.StrictConfig {
					t.Fatal("Full Access must retain strict MCP configuration without a sandbox file executor")
				}
				continue
			}
			if got.Sandbox == nil || !got.Sandbox.RequireSandbox || !got.Sandbox.RequireFileTools || !got.Sandbox.RequireSearchTools || !got.Sandbox.RequireMediaFiles || !got.Sandbox.RequireMediaNetwork || !got.Sandbox.RequireNotebookFiles || !got.Sandbox.RequireSkillFiles || !got.Sandbox.RequireContextFiles || !got.Sandbox.RequireProjectFiles || !got.Sandbox.RequireManagedPolicy || !got.Sandbox.RequireSettingsFiles || !got.Sandbox.RequireSettingsWrites || !*got.Sandbox.FailIfUnavailable {
				t.Fatal("mandatory execution requirement lost")
			}
			if got.Sandbox.Resources != nil || got.Sandbox.AllowUnsandboxedCommands == nil || !*got.Sandbox.AllowUnsandboxedCommands {
				t.Fatal("restricted nxs must keep the approval-gated escape and carry no host scratch contract")
			}
			if !reflect.DeepEqual(got.Sandbox.Filesystem.AllowRead, input.SkillDirectories) || !reflect.DeepEqual(got.Sandbox.Filesystem.AllowWrite, input.AdditionalDirectories) {
				t.Fatal("resource/write grants were conflated")
			}
			got.Sandbox.Filesystem.AllowWrite[0] = "/changed"
			if input.AdditionalDirectories[0] != "/mounted" {
				t.Fatal("policy aliases caller grants")
			}
		}
	}
}

func TestDesktopSandboxClaudeRestrictedPreservesOrdinarySettingsButRejectsNXSContract(t *testing.T) {
	enabled := true
	ordinary := &agentclient.SandboxSettings{Enabled: &enabled, Extra: map[string]any{"uiHint": "native"}}
	got, err := applyDesktopSandboxForPlatform(agentclient.Options{
		Sandbox: ordinary,
		Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeClaude, PermissionMode: sdkpermission.ModeDefault},
	}, AgentClientOptionsInput{AppMode: "desktop", DesktopSandboxEnabled: true}, "darwin")
	if err != nil {
		t.Fatal(err)
	}
	if got.Sandbox == ordinary || got.Sandbox == nil || !got.Sandbox.RequireClaudeNativeSandbox || got.Sandbox.RequireClaudeRestricted || got.Sandbox.Enabled == nil || !*got.Sandbox.Enabled || got.Sandbox.FailIfUnavailable == nil || !*got.Sandbox.FailIfUnavailable || got.Sandbox.AllowUnsandboxedCommands == nil || *got.Sandbox.AllowUnsandboxedCommands || got.Sandbox.Extra["uiHint"] != "native" {
		t.Fatalf("ordinary Claude settings were not copied: %#v", got.Sandbox)
	}
	ordinary.RequireClaudeNativeSandbox = false
	if got.Sandbox.RequireClaudeNativeSandbox != true {
		t.Fatal("Claude native sandbox contract aliases caller settings")
	}
	for _, existing := range []*agentclient.SandboxSettings{{RequireSandbox: true}, {RequireFileTools: true}, {Resources: &agentclient.SandboxResourcePolicy{Version: 1, WriteScope: agentclient.SandboxWriteScopeReadOnly, ScratchRoot: "/tmp/scratch"}}} {
		if _, err := applyDesktopSandboxForPlatform(agentclient.Options{Sandbox: existing, Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeClaude}}, AgentClientOptionsInput{AppMode: "desktop", DesktopSandboxEnabled: true}, "darwin"); err == nil {
			t.Fatalf("mixed nxs Claude sandbox contract accepted: %#v", existing)
		}
	}
	gotWindows, err := applyDesktopSandboxForPlatform(agentclient.Options{Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeClaude, PermissionMode: sdkpermission.ModeBypassPermissions}}, AgentClientOptionsInput{
		AppMode:               "desktop",
		DesktopSandboxEnabled: true,
	}, "windows")
	if err != nil {
		t.Fatal(err)
	}
	if gotWindows.Sandbox != nil {
		t.Fatalf("Windows must defer the desktop sandbox contract: %#v", gotWindows.Sandbox)
	}
}

func TestDesktopSandboxDoesNotAlterServerIsolationOrDisabledFeature(t *testing.T) {
	for _, input := range []AgentClientOptionsInput{{AppMode: "server", DesktopSandboxEnabled: true}, {AppMode: "desktop"}} {
		original := &agentclient.SandboxSettings{}
		before := agentclient.Options{Sandbox: original, Env: map[string]string{protocol.NexusDesktopSandboxPolicyEnvName: "1"}}
		got, err := applyDesktopSandboxForPlatform(before, input, "linux")
		if err != nil || got.Sandbox != original || got.Env[protocol.NexusDesktopSandboxPolicyEnvName] != "" {
			t.Fatalf("legacy/server policy changed: %#v, %v", got, err)
		}
	}
	input := AgentClientOptionsInput{AppMode: "desktop", DesktopSandboxEnabled: true}
	if _, err := applyDesktopSandboxForPlatform(agentclient.Options{}, input, "linux"); err == nil {
		t.Fatal("unsupported desktop platform silently accepted")
	}
	if _, err := applyDesktopSandboxForPlatform(agentclient.Options{Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeKind("other")}}, input, "darwin"); err == nil {
		t.Fatal("unsupported runtime silently accepted")
	}
}

func TestDesktopHostPathsDenyUserSecretReads(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []agentclient.RuntimeKind{agentclient.RuntimeNXS, agentclient.RuntimeClaude} {
		before := agentclient.Options{Runtime: agentclient.RuntimeOptions{Kind: kind}, Sandbox: &agentclient.SandboxSettings{Filesystem: &agentclient.SandboxFilesystemConfig{}}}
		got, err := applyDesktopHostPaths(before, AgentClientOptionsInput{AppMode: "desktop", DesktopSandboxEnabled: true}, "darwin", filepath.Join(root, "state", "app"), home)
		if err != nil {
			t.Fatal(err)
		}
		for _, secret := range []string{filepath.Join(home, ".ssh"), filepath.Join(home, ".aws"), filepath.Join(home, ".npmrc"), filepath.Join(home, ".codex"), filepath.Join(home, ".claude"), filepath.Join(home, "Library", "Keychains")} {
			if !slices.Contains(got.Sandbox.Filesystem.DenyRead, secret) {
				t.Fatalf("%s: secret path readable: %s", kind, secret)
			}
			if slices.Contains(got.Sandbox.Filesystem.DenyWrite, secret) {
				t.Fatalf("%s: secret path needlessly added to write denials: %s", kind, secret)
			}
			if kind == agentclient.RuntimeClaude && !slices.Contains(got.Tools.Deny, "Read(/"+filepath.ToSlash(secret)+"/**)") {
				t.Fatalf("Claude secret read rule missing: %s", secret)
			}
		}
	}
}

func TestDesktopHostPathsIgnoreTaskStateRoot(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("完整桌面沙箱装配仅在 macOS 启用；宿主路径规则由显式平台测试覆盖")
	}
	root := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, root)
	got, err := BuildAgentClientOptions(context.Background(), nil, AgentClientOptionsInput{AppMode: "desktop", DesktopSandboxEnabled: true, RuntimeKind: "nxs", WorkspacePath: t.TempDir(), OwnerUserID: "owner", ExtraEnv: map[string]string{appfs.NexusStateRootEnvName: "/task-controlled"}})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(got.Sandbox.Filesystem.DenyRead, filepath.Join(root, "app", "data")) || slices.Contains(got.Sandbox.Filesystem.DenyRead, "/task-controlled/app/data") {
		t.Fatal("task environment replaced host authority")
	}
}

func TestDesktopHostPathsProtectWindowsNXSState(t *testing.T) {
	root := filepath.Clean(t.TempDir())
	appRoot := filepath.Join(root, "app")
	if err := os.MkdirAll(filepath.Join(appRoot, "data"), 0700); err != nil {
		t.Fatal(err)
	}
	options, err := applyDesktopHostPaths(
		agentclient.Options{
			Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeNXS},
			Sandbox: &agentclient.SandboxSettings{Filesystem: &agentclient.SandboxFilesystemConfig{}},
		},
		AgentClientOptionsInput{AppMode: "desktop", DesktopSandboxEnabled: true},
		"windows",
		appRoot, "",
	)
	if err != nil {
		t.Fatal(err)
	}
	filesystem := options.Sandbox.Filesystem
	if !slices.Contains(filesystem.DenyRead, filepath.Join(appRoot, "data")) {
		t.Fatalf("Windows state data directory is readable: %#v", filesystem.DenyRead)
	}
	if !slices.Contains(filesystem.DenyWrite, appRoot) {
		t.Fatalf("Windows app root is writable: %#v", filesystem.DenyWrite)
	}
}

func TestDesktopHostPathsApplyToWindowsRestrictedNXS(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows MXC host protection")
	}
	appRoot := filepath.Join(t.TempDir(), "state", "app")
	if err := os.MkdirAll(appRoot, 0700); err != nil {
		t.Fatal(err)
	}
	options := agentclient.Options{
		Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeNXS},
		Sandbox: &agentclient.SandboxSettings{Filesystem: &agentclient.SandboxFilesystemConfig{}},
	}
	got, err := applyDesktopHostPaths(options, AgentClientOptionsInput{AppMode: "desktop", DesktopSandboxEnabled: true}, "windows", appRoot, "")
	if err != nil {
		t.Fatal(err)
	}
	record := filepath.Join(filepath.Dir(appRoot), "NexusSidecar.pid.json")
	if !slices.Contains(got.Sandbox.Filesystem.DenyRead, appRoot) || !slices.Contains(got.Sandbox.Filesystem.DenyWrite, appRoot) {
		t.Fatalf("Windows host app paths are not protected: %#v", got.Sandbox.Filesystem)
	}
	if !slices.Contains(got.Sandbox.Filesystem.DenyRead, record) || !slices.Contains(got.Sandbox.Filesystem.DenyWrite, record) {
		t.Fatalf("Windows sidecar record is not protected: %#v", got.Sandbox.Filesystem)
	}
}

func TestDesktopSandboxNetworkAdmissionDefaultsToDenyAll(t *testing.T) {
	var admission *DesktopSandboxNetworkAdmission
	config, err := admission.DesktopSandboxNetworkConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config == nil || config.AllowedDomains == nil || len(config.AllowedDomains) != 0 {
		t.Fatalf("nil admission must serialize an explicit empty allowlist: %#v", config)
	}
	if admission.Allows("https://api.example.com/v1") {
		t.Fatal("nil admission allowed an external destination")
	}
}

func TestDesktopSandboxNetworkAdmissionNormalizesAndScopesExactHTTPSDomains(t *testing.T) {
	admission := &DesktopSandboxNetworkAdmission{AllowedDomains: []string{" API.Example.com. ", "api.example.com", "auth.example.com"}}
	domains, err := admission.CanonicalAllowedDomains()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(domains, ","), "api.example.com,auth.example.com"; got != want {
		t.Fatalf("canonical domains=%q want %q", got, want)
	}
	tests := []struct {
		url string
		ok  bool
	}{
		{"https://api.example.com/mcp", true},
		{"https://api.example.com:443/mcp", true},
		{"https://sub.api.example.com/mcp", false},
		{"http://api.example.com/mcp", false},
		{"https://api.example.com:8443/mcp", false},
		{"https://api.example.com@evil.example/mcp", false},
		{"https://127.0.0.1/mcp", false},
	}
	for _, test := range tests {
		if got := admission.Allows(test.url); got != test.ok {
			t.Errorf("Allows(%q)=%v want %v", test.url, got, test.ok)
		}
	}
}

func TestDesktopSandboxNetworkAdmissionRejectsUnsafeDomains(t *testing.T) {
	for _, domain := range []string{"*.example.com", "https://example.com", "127.0.0.1", "example.com:443", "example..com"} {
		if _, err := (&DesktopSandboxNetworkAdmission{AllowedDomains: []string{domain}}).CanonicalAllowedDomains(); err == nil {
			t.Errorf("unsafe domain %q was accepted", domain)
		}
	}
}

func TestDesktopSandboxRemoteMCPRequiresHostApprovedDomain(t *testing.T) {
	configured := map[string]any{
		"remote": map[string]any{
			"type": "http",
			"url":  "https://mcp.example.com/mcp",
		},
	}
	if err := RejectDesktopSandboxRemoteMCPWithNetworkAdmission(configured, runtimeKindNXS, "desktop", true, sdkpermission.ModeDefault, nil); (err == nil) != desktopMCPContractPlatform(runtime.GOOS) {
		t.Fatalf("endpoint contract: %v", err)
	}
	if err := RejectDesktopSandboxRemoteMCPWithNetworkAdmission(configured, runtimeKindNXS, "desktop", true, sdkpermission.ModeDefault, &DesktopSandboxNetworkAdmission{AllowedDomains: []string{"other.example.com"}}); (err == nil) != desktopMCPContractPlatform(runtime.GOOS) {
		t.Fatalf("unapproved remote MCP error=%v", err)
	}
	if err := RejectDesktopSandboxRemoteMCPWithNetworkAdmission(configured, runtimeKindNXS, "desktop", true, sdkpermission.ModeDefault, &DesktopSandboxNetworkAdmission{AllowedDomains: []string{"mcp.example.com"}}); err != nil {
		t.Fatalf("approved remote MCP rejected: %v", err)
	}
}

func TestDesktopSandboxChecksHostOwnedTypedRemoteMCP(t *testing.T) {
	servers := map[string]sdkmcp.ServerConfig{
		"github": sdkmcp.HTTPServerConfig{URL: "https://mcp.example.com/mcp", Headers: map[string]string{"Authorization": "Bearer connector-secret"}},
	}
	if err := RejectDesktopSandboxTypedMCPServersWithNetworkAdmission(servers, runtimeKindNXS, "desktop", true, sdkpermission.ModeDefault, nil); (err == nil) != desktopMCPContractPlatform(runtime.GOOS) {
		t.Fatalf("host-owned remote MCP bypassed deny-all admission: %v", err)
	}
	if err := RejectDesktopSandboxTypedMCPServersWithNetworkAdmission(servers, runtimeKindNXS, "desktop", true, sdkpermission.ModeDefault, &DesktopSandboxNetworkAdmission{AllowedDomains: []string{"other.example.com"}}); (err == nil) != desktopMCPContractPlatform(runtime.GOOS) {
		t.Fatalf("unapproved host-owned remote MCP was admitted: %v", err)
	}
	if err := RejectDesktopSandboxTypedMCPServersWithNetworkAdmission(servers, runtimeKindNXS, "desktop", true, sdkpermission.ModeDefault, &DesktopSandboxNetworkAdmission{AllowedDomains: []string{"mcp.example.com"}}); err != nil {
		t.Fatalf("approved host-owned remote MCP was rejected: %v", err)
	}
}

func TestDesktopSandboxTypedMCPHelpersRequirePlatformContract(t *testing.T) {
	servers := map[string]sdkmcp.ServerConfig{
		"remote": sdkmcp.HTTPServerConfig{URL: "https://mcp.example.com/mcp", HeadersHelper: "/tmp/untrusted-helper"},
	}
	if err := RejectDesktopSandboxTypedMCPServersWithNetworkAdmission(servers, runtimeKindNXS, "desktop", true, sdkpermission.ModeBypassPermissions, nil); (err == nil) != desktopMCPContractPlatform(runtime.GOOS) {
		t.Fatalf("typed helper platform contract: %v", err)
	}
}

func TestDesktopSandboxNetworkAdmissionIsCopiedIntoSettings(t *testing.T) {
	input := AgentClientOptionsInput{
		AppMode:                        "desktop",
		DesktopSandboxEnabled:          true,
		DesktopSandboxNetworkAdmission: &DesktopSandboxNetworkAdmission{AllowedDomains: []string{"api.example.com"}},
	}
	options, err := applyDesktopSandboxForPlatform(agentclient.Options{Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeNXS}}, input, "darwin")
	if err != nil {
		t.Fatal(err)
	}
	options, err = applyDesktopSandboxNetworkAdmission(options, input)
	if err != nil {
		t.Fatal(err)
	}
	if options.Sandbox == nil || options.Sandbox.Network == nil || len(options.Sandbox.Network.AllowedDomains) != 1 || options.Sandbox.Network.AllowedDomains[0] != "api.example.com" {
		t.Fatalf("network settings=%#v", options.Sandbox)
	}
	input.DesktopSandboxNetworkAdmission.AllowedDomains[0] = "changed.example.com"
	if options.Sandbox.Network.AllowedDomains[0] != "api.example.com" {
		t.Fatal("network settings aliased host admission")
	}
}

func TestDesktopSandboxRejectsWebSearchPrivateNetwork(t *testing.T) {
	old := desktopSandboxProcessElevated
	desktopSandboxProcessElevated = func() bool { return false }
	t.Cleanup(func() { desktopSandboxProcessElevated = old })
	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		t.Skip("完整桌面沙箱装配仅在 macOS 与 Windows 启用")
	}
	_, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		AppMode: "desktop", DesktopSandboxEnabled: true,
		RuntimeKind:   runtimeKindNXS,
		WorkspacePath: t.TempDir(),
		WebSearch:     WebSearchConfig{AllowPrivateNetwork: true},
	})
	if err == nil || !strings.Contains(err.Error(), "private-network") {
		t.Fatalf("private WebSearch access was not rejected: %v", err)
	}
}

func TestDesktopProviderCredentialIsProjectedOnlyFromResolvedConfig(t *testing.T) {
	old := desktopSandboxProcessElevated
	desktopSandboxProcessElevated = func() bool { return false }
	t.Cleanup(func() { desktopSandboxProcessElevated = old })
	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		t.Skip("完整桌面沙箱装配仅在 macOS 与 Windows 启用")
	}
	t.Setenv("OPENAI_API_KEY", "inherited-host-secret")
	options, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{
		config: &RuntimeConfig{
			Provider:  "desktop-provider",
			AuthToken: "resolved-session-secret",
			BaseURL:   "https://api.example.com/v1",
			Model:     "gpt-test",
			APIFormat: apiFormatResponses,
		},
	}, AgentClientOptionsInput{
		AppMode: "desktop", DesktopSandboxEnabled: true,
		RuntimeKind:   runtimeKindNXS,
		WorkspacePath: t.TempDir(),
		ExtraEnv: map[string]string{
			"OPENAI_API_KEY": "task-override-must-not-win",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := options.Env["OPENAI_API_KEY"]; got != "resolved-session-secret" {
		t.Fatalf("provider credential=%q want resolved session credential", got)
	}
}

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
