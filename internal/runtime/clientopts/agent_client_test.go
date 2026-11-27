// agent_client_test.go 覆盖 Agent runtime 选项构造、MCP 配置合并、Provider/视觉/指令环境与准入测试。
package clientopts

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/runtimeadmission"
	"github.com/nexus-research-lab/nexus/internal/protocol"
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
