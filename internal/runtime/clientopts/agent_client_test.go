// agent_client_test.go 覆盖 Agent runtime 选项构造、MCP 配置合并、Provider/视觉/指令环境与准入测试。
package clientopts

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/runtimeadmission"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

type fakeRuntimeConfigResolver struct {
	config *RuntimeConfig
	err    error
	calls  *int
}

func (r fakeRuntimeConfigResolver) ResolveRuntimeConfig(
	context.Context,
	string,
	string,
) (*RuntimeConfig, error) {
	if r.calls != nil {
		*r.calls = *r.calls + 1
	}
	return r.config, r.err
}

func TestRuntimePreauthorizationPreservesExplicitRules(t *testing.T) {
	for _, configured := range [][]string{nil, {"WebFetch(domain:example.com)", "Bash(git status)"}} {
		options, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{
			config: &RuntimeConfig{Model: "test"},
		}, AgentClientOptionsInput{
			WorkspacePath: t.TempDir(), AllowedTools: configured,
			DisallowedTools: []string{"WebSearch", "Write"},
		})
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Contains(options.Tools.Allow, "WebSearch") ||
			!slices.Contains(options.Tools.Deny, "WebSearch") || !slices.Contains(options.Tools.Deny, "Write") {
			t.Fatalf("默认检索授权不得移除显式禁止规则: %+v", options.Tools)
		}
		if len(configured) > 0 && (slices.Contains(options.Tools.Allow, "WebFetch") || !slices.Contains(options.Tools.Allow, configured[0])) {
			t.Fatalf("不得扩大已有域名授权: %+v", options.Tools.Allow)
		}
		for _, tool := range []string{"Bash", "Agent", "Write", "Edit", "Read"} {
			if slices.Contains(options.Tools.Allow, tool) {
				t.Fatalf("不得无条件预授权 %s", tool)
			}
		}
	}
}

func TestMergeAgentMCPServersRejectsUnsafeStaticHeaders(t *testing.T) {
	for name, headers := range map[string]map[string]any{
		"control-character-name":  {"X-Bad\nName": "value"},
		"control-character-value": {"X-Bad": "value\r\nmore"},
		"non-token-name":          {"X Bad": "value"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := MergeAgentMCPServers(nil, map[string]any{
				"remote": map[string]any{
					"type":    "http",
					"url":     "https://mcp.example.com/rpc",
					"headers": headers,
				},
			})
			if err == nil || !strings.Contains(err.Error(), "非法") {
				t.Fatalf("unsafe headers accepted: %v", err)
			}
		})
	}
}

func TestMergeAgentMCPServersRejectsUnstableNames(t *testing.T) {
	for _, name := range []string{" white-space", "with space", "line\nbreak", strings.Repeat("a", 65)} {
		_, err := MergeAgentMCPServers(nil, map[string]any{
			name: map[string]any{"command": "custom"},
		})
		if err == nil {
			t.Fatalf("server name %q should be rejected", name)
		}
	}
}

func TestMergeAgentMCPServersRejectsInvalidConfigWithServerName(t *testing.T) {
	tests := []struct {
		name       string
		configured any
	}{
		{name: "not an object", configured: []any{"npx"}},
		{name: "unknown field", configured: map[string]any{"command": "npx", "scope": "user"}},
		{name: "wrong args", configured: map[string]any{"command": "npx", "args": []any{1}}},
		{name: "unsupported sdk", configured: map[string]any{"type": "sdk", "name": "internal"}},
		{name: "invalid URL", configured: map[string]any{"type": "http", "url": "not-a-url"}},
		{name: "invalid OAuth metadata URL", configured: map[string]any{
			"type": "sse",
			"url":  "https://mcp.example.com/events",
			"oauth": map[string]any{
				"authServerMetadataUrl": "http://auth.example.com/metadata",
			},
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := MergeAgentMCPServers(nil, map[string]any{"broken_server": test.configured})
			if err == nil {
				t.Fatal("MergeAgentMCPServers() error = nil")
			}
			if !strings.Contains(err.Error(), "broken_server") {
				t.Fatalf("error does not identify server: %v", err)
			}
		})
	}
}

func TestBuildAgentClientOptionsPreservesHostProviderOwnership(t *testing.T) {
	for _, source := range []string{"extra", "configuration"} {
		t.Run(source, func(t *testing.T) {
			input := AgentClientOptionsInput{RuntimeKind: runtimeKindNXS, WorkspacePath: t.TempDir()}
			override := map[string]string{nexusProviderManagedByHostEnvName: "0", nexusAutoDreamWakeModeEnvName: "task"}
			if source == "extra" {
				input.ExtraEnv = override
			} else {
				input.ConfigurationEnv = map[string]string{
					protocol.NexusConfigBrokerURLEnvName:       "http://127.0.0.1:8010/configuration",
					protocol.NexusConfigCapabilityTokenEnvName: "test-capability",
				}
			}
			options, err := BuildAgentClientOptions(t.Context(), fakeRuntimeConfigResolver{}, input)
			if err != nil {
				t.Fatal(err)
			}
			if options.Env[nexusProviderManagedByHostEnvName] != "1" || options.Env[nexusAutoDreamWakeModeEnvName] != "host" {
				t.Fatal("runtime environment revoked host provider or wake ownership")
			}
		})
	}
}

func TestBuildClaudeOptionsDoNotClaimNXSProviderOwnership(t *testing.T) {
	options, err := BuildAgentClientOptions(t.Context(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{RuntimeKind: runtimeKindClaude, WorkspacePath: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if options.Env[nexusProviderManagedByHostEnvName] == "1" || options.Env[nexusAutoDreamWakeModeEnvName] == "host" {
		t.Fatal("Claude received an nxs-specific ownership contract")
	}
}

func TestBuildAgentClientOptionsRejectsUnknownConfigurationEnvironment(t *testing.T) {
	_, err := BuildAgentClientOptions(t.Context(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		RuntimeKind:   runtimeKindNXS,
		WorkspacePath: t.TempDir(),
		ConfigurationEnv: map[string]string{
			protocol.NexusConfigBrokerURLEnvName:       "http://127.0.0.1:8010/configuration",
			protocol.NexusConfigCapabilityTokenEnvName: "test-capability",
			"HOME": "/tmp/escaped-home",
		},
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported environment key") {
		t.Fatalf("unknown nexuscfg environment key was accepted: %v", err)
	}
}

type fakeRuntimeAdmissionResolver struct {
	err error
}

func (f fakeRuntimeAdmissionResolver) BeginAgentRuntimeAdmission(
	ctx context.Context,
) (*runtimeadmission.Lease, error) {
	return runtimeadmission.NewDetachedLease(ctx), f.err
}

func TestBeginAgentRuntimeAdmissionFailsClosed(t *testing.T) {
	lease, err := BeginAgentRuntimeAdmission(
		context.Background(),
		fakeRuntimeAdmissionResolver{err: errors.New("database unavailable")},
	)
	if lease != nil || err == nil || !strings.Contains(err.Error(), "database unavailable") {
		t.Fatalf("认证状态读取失败必须 fail closed，lease=%v err=%v", lease, err)
	}
}

func TestBuildAgentClientOptionsProjectsDiagnosticsForBothRuntimes(t *testing.T) {
	t.Setenv(runtimectx.AgentSDKDiagnosticsJSONLEnvName, "")
	for _, test := range []struct {
		name    string
		kind    string
		enabled bool
	}{
		{"Claude enabled", runtimeKindClaude, true},
		{"Claude disabled", runtimeKindClaude, false},
		{"nxs enabled", runtimeKindNXS, true},
		{"nxs disabled", runtimeKindNXS, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			options, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
				RuntimeKind:                test.kind,
				AgentSDKDiagnosticsEnabled: test.enabled,
			})
			if err != nil {
				t.Fatal(err)
			}
			if got := runtimectx.AgentSDKDiagnosticsEnabled(options.Env); got != test.enabled {
				t.Fatalf("diagnostics enabled = %t, want %t", got, test.enabled)
			}
			_, hasBridgeFlag := options.Env[runtimectx.AgentSDKDiagnosticsEnvName]
			if wantBridgeFlag := test.enabled && test.kind == runtimeKindClaude; hasBridgeFlag != wantBridgeFlag {
				t.Fatalf("Claude diagnostics flag present = %t, want %t", hasBridgeFlag, wantBridgeFlag)
			}
			_, hasJSONL := options.Env[runtimectx.AgentSDKDiagnosticsJSONLEnvName]
			if wantJSONL := test.enabled && test.kind == runtimeKindNXS; hasJSONL != wantJSONL {
				t.Fatalf("nxs JSONL diagnostics present = %t, want %t", hasJSONL, wantJSONL)
			}
		})
	}
}

type visionTestResolver struct {
	main, auxiliary       *RuntimeConfig
	mainErr, auxiliaryErr error
}

func (r visionTestResolver) ResolveRuntimeConfig(_ context.Context, provider, _ string) (*RuntimeConfig, error) {
	if provider == "main" {
		return r.main, r.mainErr
	}
	return r.auxiliary, r.auxiliaryErr
}

func TestOptionalVisionDoesNotBlockMainChat(t *testing.T) {
	for _, mainVision := range []bool{false, true} {
		for _, tc := range []struct {
			name, provider, model string
			config                *RuntimeConfig
			err                   error
		}{
			{name: "unknown", provider: "custom", model: "kimi-k2.6", config: &RuntimeConfig{Model: "kimi-k2.6", APIFormat: apiFormatChatCompletions}},
			{name: "removed", provider: "custom", model: "kimi-k2.6", err: errors.New("provider no longer exists")},
			{name: "disabled", provider: "custom", model: "kimi-k2.6", err: errors.New("provider disabled")},
			{name: "partial", provider: "custom"},
			{name: "unset"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Setenv("NEXUS_VISION_MODEL", "inherited-model")
				t.Setenv("NEXUS_VISION_API_KEY", "inherited-key")
				resolver := visionTestResolver{main: &RuntimeConfig{Model: "main-model", Vision: mainVision, APIFormat: apiFormatChatCompletions, AuthToken: "main-key", BaseURL: "https://main.test"}, auxiliary: tc.config, auxiliaryErr: tc.err}
				options, err := BuildAgentClientOptions(context.Background(), resolver, AgentClientOptionsInput{RuntimeKind: "nxs", Provider: "main", Model: "main-model", VisionProvider: tc.provider, VisionModel: tc.model, ExtraEnv: map[string]string{"NEXUS_VISION_MODEL": "stale-model", "NEXUS_VISION_API_KEY": "stale-key"}})
				if err != nil {
					t.Fatal(err)
				}
				if options.Model != "main-model" {
					t.Fatalf("main model changed: %s", options.Model)
				}
				for _, key := range []string{"NEXUS_VISION_MODEL", "NEXUS_VISION_API_KEY", "NEXUS_VISION_BASE_URL"} {
					if options.Env[key] != "" {
						t.Fatalf("invalid auxiliary retained %s", key)
					}
				}
				if (options.Env[nexusModelSupportsVisionEnvName] == "true") != mainVision {
					t.Fatal("main vision capability changed")
				}
			})
		}
	}
}

func TestVisionAdmissionRetainsValidRouteAndMainFailures(t *testing.T) {
	resolver := visionTestResolver{main: &RuntimeConfig{Model: "main-model", APIFormat: apiFormatChatCompletions}, auxiliary: &RuntimeConfig{Provider: "vision", Model: "visual-model", Vision: true, APIFormat: apiFormatResponses, AuthToken: "visual-key", BaseURL: "https://vision.test"}}
	input := AgentClientOptionsInput{RuntimeKind: "nxs", Provider: "main", Model: "main-model", VisionProvider: "vision", VisionModel: "visual-model"}
	options, err := BuildAgentClientOptions(context.Background(), resolver, input)
	if err != nil || options.Env["NEXUS_VISION_MODEL"] != "visual-model" || options.Env["NEXUS_VISION_API_PROVIDER"] != "responses" {
		t.Fatalf("valid route lost: %v", err)
	}
	resolver.mainErr = errors.New("main model unavailable")
	if _, err = BuildAgentClientOptions(context.Background(), resolver, input); !errors.Is(err, resolver.mainErr) {
		t.Fatalf("main failure swallowed: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	resolver.mainErr = nil
	resolver.auxiliaryErr = ctx.Err()
	if _, err = BuildAgentClientOptions(ctx, resolver, input); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation swallowed: %v", err)
	}
}
