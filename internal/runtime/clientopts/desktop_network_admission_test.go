package clientopts

import (
	"context"
	"strings"
	"testing"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

type desktopNetworkTestSDKServer struct{}

func (desktopNetworkTestSDKServer) HandleMessage(context.Context, map[string]any) (map[string]any, error) {
	return map[string]any{"ok": true}, nil
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
	if err := RejectDesktopSandboxRemoteMCPWithNetworkAdmission(configured, runtimeKindNXS, "desktop", true, sdkpermission.ModeDefault, nil); err == nil {
		t.Fatal("nil host admission allowed remote MCP")
	}
	if err := RejectDesktopSandboxRemoteMCPWithNetworkAdmission(configured, runtimeKindNXS, "desktop", true, sdkpermission.ModeDefault, &DesktopSandboxNetworkAdmission{AllowedDomains: []string{"other.example.com"}}); err == nil || !strings.Contains(err.Error(), "域名准入") {
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
	if err := RejectDesktopSandboxTypedMCPServersWithNetworkAdmission(servers, runtimeKindNXS, "desktop", true, sdkpermission.ModeDefault, nil); err == nil || !strings.Contains(err.Error(), "域名准入") {
		t.Fatalf("host-owned remote MCP bypassed deny-all admission: %v", err)
	}
	if err := RejectDesktopSandboxTypedMCPServersWithNetworkAdmission(servers, runtimeKindNXS, "desktop", true, sdkpermission.ModeDefault, &DesktopSandboxNetworkAdmission{AllowedDomains: []string{"other.example.com"}}); err == nil || !strings.Contains(err.Error(), "域名准入") {
		t.Fatalf("unapproved host-owned remote MCP was admitted: %v", err)
	}
	if err := RejectDesktopSandboxTypedMCPServersWithNetworkAdmission(servers, runtimeKindNXS, "desktop", true, sdkpermission.ModeDefault, &DesktopSandboxNetworkAdmission{AllowedDomains: []string{"mcp.example.com"}}); err != nil {
		t.Fatalf("approved host-owned remote MCP was rejected: %v", err)
	}
}

func TestDesktopSandboxTypedMCPRejectsHeadersHelperEvenInFullAccess(t *testing.T) {
	servers := map[string]sdkmcp.ServerConfig{
		"remote": sdkmcp.HTTPServerConfig{URL: "https://mcp.example.com/mcp", HeadersHelper: "/tmp/untrusted-helper"},
	}
	if err := RejectDesktopSandboxTypedMCPServersWithNetworkAdmission(servers, runtimeKindNXS, "desktop", true, sdkpermission.ModeBypassPermissions, nil); err == nil || !strings.Contains(err.Error(), "headers helper") {
		t.Fatalf("typed headers helper was admitted in Full Access: %v", err)
	}
}

func TestDesktopSandboxTypedMCPLeavesStdioAndSDKServersToTheirOwnContracts(t *testing.T) {
	servers := map[string]sdkmcp.ServerConfig{
		"stdio": sdkmcp.StdioServerConfig{Command: "connector-helper"},
		"sdk":   sdkmcp.SDKServerConfig{Name: "managed", Instance: desktopNetworkTestSDKServer{}},
	}
	if err := RejectDesktopSandboxTypedMCPServersWithNetworkAdmission(servers, runtimeKindNXS, "desktop", true, sdkpermission.ModeDefault, nil); err != nil {
		t.Fatalf("non-HTTP/SSE host-owned MCP should remain a separate contract: %v", err)
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

func TestDesktopSandboxNetworkAdmissionDoesNotOverrideClaudeNativeSandbox(t *testing.T) {
	original := &agentclient.SandboxNetworkConfig{AllowedDomains: []string{"api.example.com"}}
	options := agentclient.Options{
		Runtime: agentclient.RuntimeOptions{Kind: agentclient.RuntimeClaude},
		Sandbox: &agentclient.SandboxSettings{Network: original},
	}
	updated, err := applyDesktopSandboxNetworkAdmission(options, AgentClientOptionsInput{
		AppMode:                        "desktop",
		DesktopSandboxEnabled:          true,
		DesktopSandboxNetworkAdmission: nil,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Sandbox == nil || updated.Sandbox.Network != original || len(updated.Sandbox.Network.AllowedDomains) != 1 || updated.Sandbox.Network.AllowedDomains[0] != "api.example.com" {
		t.Fatalf("Claude native network settings were overridden: %#v", updated.Sandbox)
	}
	granted, err := applyDesktopSandboxNetworkAdmission(options, AgentClientOptionsInput{
		AppMode:                        "desktop",
		DesktopSandboxEnabled:          true,
		DesktopSandboxNetworkAdmission: &DesktopSandboxNetworkAdmission{AllowedDomains: []string{"mcp.example.com"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if granted.Sandbox == nil || granted.Sandbox.Network == original || len(granted.Sandbox.Network.AllowedDomains) != 1 || granted.Sandbox.Network.AllowedDomains[0] != "mcp.example.com" {
		t.Fatalf("explicit Claude host grant was not applied: %#v", granted.Sandbox)
	}
}

func TestDesktopSandboxRejectsWebSearchPrivateNetwork(t *testing.T) {
	_, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		AppMode:       "desktop",
		RuntimeKind:   runtimeKindNXS,
		WorkspacePath: t.TempDir(),
		WebSearch:     WebSearchConfig{AllowPrivateNetwork: true},
	})
	if err == nil || !strings.Contains(err.Error(), "private-network") {
		t.Fatalf("private WebSearch access was not rejected: %v", err)
	}
}

func TestDesktopProviderCredentialIsProjectedOnlyFromResolvedConfig(t *testing.T) {
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
		AppMode:       "desktop",
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
