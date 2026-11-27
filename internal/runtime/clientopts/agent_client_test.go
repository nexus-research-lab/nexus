// agent_client_test.go 覆盖 Agent runtime 选项构造、MCP 配置合并、Provider/视觉/指令环境与准入测试。
package clientopts

import (
	"context"
	"errors"
	"runtime"
	"slices"
	"strings"
	"testing"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
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

func TestBuildAgentClientOptionsIgnoresInvalidOptionalVisionModel(t *testing.T) {
	options, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{
		config: &RuntimeConfig{Model: "text-model", Vision: false},
	}, AgentClientOptionsInput{
		Provider:       "text-provider",
		Model:          "text-model",
		VisionProvider: "vision-provider",
		VisionModel:    "vision-model",
	})
	if err != nil {
		t.Fatalf("纯文本会话不应因视觉模型无效而失败: %v", err)
	}
	if options.Env["NEXUS_VISION_MODEL"] != "" {
		t.Fatalf("无效视觉模型应覆盖旧路由: %+v", options.Env)
	}
	if !strings.Contains(options.Env["NEXUS_VISION_CONFIG_ERROR"], "未声明 vision 能力") {
		t.Fatalf("视觉配置错误应保留供图片能力说明使用: %q", options.Env["NEXUS_VISION_CONFIG_ERROR"])
	}
}

func TestAnthropicRuntimeEnvRoutesCredentialsByBaseURL(t *testing.T) {
	tests := []struct {
		name          string
		runtimeKind   string
		baseURL       string
		authToken     string
		wantAPIKey    string
		wantAuthToken string
		wantPresent   bool
	}{
		{
			name:          "compatible gateway",
			runtimeKind:   runtimeKindClaude,
			baseURL:       "https://provider.example.com/anthropic",
			authToken:     "token-1",
			wantAuthToken: "token-1",
			wantPresent:   true,
		},
		{
			name:        "nxs compatible gateway uses api key compatibility path",
			runtimeKind: runtimeKindNXS,
			baseURL:     "https://provider.example.com/anthropic",
			authToken:   "token-1",
			wantAPIKey:  "token-1",
			wantPresent: true,
		},
		{
			name:        "first party anthropic",
			runtimeKind: runtimeKindClaude,
			baseURL:     "https://api.anthropic.com",
			authToken:   "token-1",
			wantAPIKey:  "token-1",
			wantPresent: true,
		},
		{
			name:        "empty base url defaults first party",
			runtimeKind: runtimeKindClaude,
			authToken:   "token-1",
			wantAPIKey:  "token-1",
			wantPresent: true,
		},
		{
			name:        "empty token",
			runtimeKind: runtimeKindClaude,
			baseURL:     "https://provider.example.com/anthropic",
			authToken:   "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := anthropicRuntimeEnvFromConfig(&RuntimeConfig{
				AuthToken: tt.authToken,
				BaseURL:   tt.baseURL,
				Model:     "model-1",
			}, tt.runtimeKind)
			apiKey, apiKeyExists := env[anthropicAPIKeyEnvName]
			authToken, authTokenExists := env[anthropicAuthTokenEnvName]
			if apiKey != tt.wantAPIKey || authToken != tt.wantAuthToken ||
				apiKeyExists != tt.wantPresent || authTokenExists != tt.wantPresent {
				t.Fatalf("credential route = api_key:%q/%t auth_token:%q/%t; env=%+v", apiKey, apiKeyExists, authToken, authTokenExists, env)
			}
		})
	}
}

func TestBuildAgentClientOptionsProjectsNXSPreferencesByRuntime(t *testing.T) {
	defaultOptions, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		RuntimeKind: runtimeKindNXS,
	})
	if err != nil {
		t.Fatalf("构建默认 nxs options 失败: %v", err)
	}
	if defaultOptions.Env[nexusDisableAutoMemoryExtractionEnvName] != "0" ||
		defaultOptions.Env[nexusEnableAutoMemoryExtractionEnvName] != "1" {
		t.Fatalf("nxs 自动记忆默认应开启: %+v", defaultOptions.Env)
	}
	if defaultOptions.Env[nexusDisableAutoDreamEnvName] != "0" ||
		defaultOptions.Env[nexusEnableAutoDreamEnvName] != "0" {
		t.Fatalf("nxs AutoDream 默认应交给 Agent 设置: %+v", defaultOptions.Env)
	}

	nxsOptions, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		RuntimeKind:        runtimeKindNXS,
		AutoMemoryDisabled: true,
		AutoDreamDisabled:  true,
		ToolSearchEnabled:  true,
	})
	if err != nil {
		t.Fatalf("构建 nxs options 失败: %v", err)
	}
	if nxsOptions.Env[enableToolSearchEnvName] != "1" || nxsOptions.Env[nexusEnableToolSearchEnvName] != "1" {
		t.Fatalf("nxs ToolSearch 开关未投影: %+v", nxsOptions.Env)
	}
	if nxsOptions.Env[nexusDisableAutoMemoryExtractionEnvName] != "1" ||
		nxsOptions.Env[nexusEnableAutoMemoryExtractionEnvName] != "0" {
		t.Fatalf("nxs 自动记忆开关未投影: %+v", nxsOptions.Env)
	}
	if nxsOptions.Env[nexusDisableAutoDreamEnvName] != "1" ||
		nxsOptions.Env[nexusEnableAutoDreamEnvName] != "0" {
		t.Fatalf("nxs AutoDream 总开关未投影: %+v", nxsOptions.Env)
	}

	claudeOptions, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		RuntimeKind:       runtimeKindClaude,
		ToolSearchEnabled: true,
	})
	if err != nil {
		t.Fatalf("构建 Claude options 失败: %v", err)
	}
	if _, ok := claudeOptions.Env[enableToolSearchEnvName]; ok {
		t.Fatalf("Claude runtime 不应接收 nxs ToolSearch 设置: %+v", claudeOptions.Env)
	}
	if _, ok := claudeOptions.Env[nexusDisableAutoMemoryExtractionEnvName]; ok {
		t.Fatalf("Claude runtime 不应接收 nxs 自动记忆设置: %+v", claudeOptions.Env)
	}
	if _, ok := claudeOptions.Env[nexusEnableAutoMemoryExtractionEnvName]; ok {
		t.Fatalf("Claude runtime 不应接收 nxs 自动记忆设置: %+v", claudeOptions.Env)
	}
	if _, ok := claudeOptions.Env[nexusDisableAutoDreamEnvName]; ok {
		t.Fatalf("Claude runtime 不应接收 nxs AutoDream 设置: %+v", claudeOptions.Env)
	}
	if _, ok := claudeOptions.Env[nexusEnableAutoDreamEnvName]; ok {
		t.Fatalf("Claude runtime 不应接收 nxs AutoDream 设置: %+v", claudeOptions.Env)
	}
}

func TestBackgroundModelRuntimeEnvFallsBackWithoutBlockingRuntime(t *testing.T) {
	mainConfig := &RuntimeConfig{
		Provider:  "main-provider",
		Model:     "main-model",
		APIFormat: apiFormatAnthropicMessages,
	}
	tests := []struct {
		name     string
		resolver RuntimeConfigResolver
		input    AgentClientOptionsInput
	}{
		{
			name:     "background provider differs",
			resolver: fakeRuntimeConfigResolver{err: errors.New("must not resolve")},
			input: AgentClientOptionsInput{
				BackgroundProvider: "other-provider",
				BackgroundModel:    "other-model",
			},
		},
		{
			name:     "background resolution fails",
			resolver: fakeRuntimeConfigResolver{err: errors.New("temporarily unavailable")},
			input: AgentClientOptionsInput{
				BackgroundProvider: "main-provider",
				BackgroundModel:    "small-model",
			},
		},
		{
			name: "background api format differs",
			resolver: fakeRuntimeConfigResolver{config: &RuntimeConfig{
				Provider:  "main-provider",
				Model:     "small-model",
				APIFormat: apiFormatResponses,
			}},
			input: AgentClientOptionsInput{
				BackgroundProvider: "main-provider",
				BackgroundModel:    "small-model",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.input.Provider = "main-provider"
			test.input.Model = "main-model"
			env := backgroundModelRuntimeEnv(
				context.Background(),
				test.resolver,
				test.input,
				mainConfig,
				runtimeKindNXS,
			)
			if env[nexusBackgroundModelEnvName] != "main-model" ||
				env[claudeEmitToolUseSummariesEnvName] != "0" {
				t.Fatalf("后台模型应回退主模型: %+v", env)
			}
		})
	}
}

func TestBuildAgentClientOptionsRejectsClaudeNonAnthropicAPIFormat(t *testing.T) {
	t.Setenv(nexusAgentRuntimeKindEnvName, "")
	t.Setenv(nexusAgentRuntimeEnvName, "")

	_, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{
		config: &RuntimeConfig{
			AuthToken: "token-1",
			BaseURL:   "https://provider.example.com",
			Model:     "gpt-4o",
			APIFormat: "chat_completions",
		},
	}, AgentClientOptionsInput{
		RuntimeKind: runtimeKindClaude,
	})
	if err == nil || !strings.Contains(err.Error(), "claude Agent runtime") {
		t.Fatalf("Claude runtime 下非 anthropic_messages provider 应被拒绝: %v", err)
	}
}

func TestBuildAgentClientOptionsDeniesClaudeSessionUnavailableTools(t *testing.T) {
	options, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		WorkspacePath:   "/tmp/workspace",
		RuntimeKind:     runtimeKindClaude,
		DisallowedTools: []string{" ScheduleWakeup ", "Write", "EnterPlanMode"},
	})
	if err != nil {
		t.Fatalf("BuildAgentClientOptions 失败: %v", err)
	}
	for _, tool := range []string{"EnterPlanMode", "ScheduleWakeup", "CronCreate", "CronList", "CronDelete", "Write"} {
		if !containsTool(options.Tools.Deny, tool) {
			t.Fatalf("运行时 deny 工具缺少 %s: %+v", tool, options.Tools.Deny)
		}
	}
	if countTool(options.Tools.Deny, "EnterPlanMode") != 1 {
		t.Fatalf("EnterPlanMode deny 规则应去重: %+v", options.Tools.Deny)
	}
	if countTool(options.Tools.Deny, "ScheduleWakeup") != 1 {
		t.Fatalf("ScheduleWakeup deny 规则应去重: %+v", options.Tools.Deny)
	}
}

func TestBuildAgentClientOptionsRejectsOwnerContextMismatch(t *testing.T) {
	ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID: "user-a",
	})

	_, err := BuildAgentClientOptions(ctx, fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		WorkspacePath: "/tmp/workspace",
		OwnerUserID:   "user-b",
	})
	if err == nil || !strings.Contains(err.Error(), "runtime owner 与认证上下文不一致") {
		t.Fatalf("owner/context 不一致应拒绝 runtime 启动，err=%v", err)
	}
}

func containsTool(tools []string, expected string) bool {
	return countTool(tools, expected) > 0
}

func countTool(tools []string, expected string) int {
	count := 0
	for _, tool := range tools {
		if tool == expected {
			count++
		}
	}
	return count
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

type countingMCPRuntimeResolver struct {
	calls int
}

func (r *countingMCPRuntimeResolver) ResolveRuntimeConfig(
	context.Context,
	string,
	string,
) (*RuntimeConfig, error) {
	r.calls++
	return nil, nil
}

func TestMergeAgentMCPServersParsesSupportedTransports(t *testing.T) {
	builtIn := map[string]sdkmcp.ServerConfig{
		"nexus": sdkmcp.HTTPServerConfig{URL: "https://nexus.invalid/mcp"},
	}
	merged, err := MergeAgentMCPServers(builtIn, map[string]any{
		"local_tools": map[string]any{
			"type":    "stdio",
			"command": "npx",
			"args":    []any{"-y", "local-mcp"},
			"env":     map[string]any{"TOKEN": "secret"},
		},
		"remote_http": map[string]any{
			"type":          "http",
			"url":           "https://mcp.example.com/rpc",
			"headers":       map[string]any{"Authorization": "Bearer token"},
			"headersHelper": "/opt/nexus/header-helper",
			"oauth": map[string]any{
				"clientId":              "client-1",
				"callbackPort":          43123,
				"authServerMetadataUrl": "https://auth.example.com/.well-known/oauth-authorization-server",
				"xaa":                   true,
			},
		},
		"remote_sse": map[string]any{
			"type":    "sse",
			"url":     "http://127.0.0.1:9000/events",
			"headers": map[string]any{"X-Test": "value"},
		},
	})
	if err != nil {
		t.Fatalf("MergeAgentMCPServers() error = %v", err)
	}
	if len(merged) != 4 {
		t.Fatalf("merged servers = %#v, want four entries", merged)
	}
	if len(builtIn) != 1 {
		t.Fatalf("built-in server map was mutated: %#v", builtIn)
	}

	stdio, ok := merged["local_tools"].(sdkmcp.StdioServerConfig)
	if !ok || stdio.Command != "npx" || len(stdio.Args) != 2 || stdio.Env["TOKEN"] != "secret" {
		t.Fatalf("stdio config = %#v", merged["local_tools"])
	}
	httpServer, ok := merged["remote_http"].(sdkmcp.HTTPServerConfig)
	if !ok || httpServer.URL != "https://mcp.example.com/rpc" {
		t.Fatalf("http config = %#v", merged["remote_http"])
	}
	if httpServer.Headers["Authorization"] != "Bearer token" || httpServer.HeadersHelper != "/opt/nexus/header-helper" {
		t.Fatalf("http headers config = %#v", httpServer)
	}
	if httpServer.OAuth == nil || httpServer.OAuth.ClientID != "client-1" || httpServer.OAuth.CallbackPort != 43123 {
		t.Fatalf("http oauth config = %#v", httpServer.OAuth)
	}
	if httpServer.OAuth.XAA == nil || !*httpServer.OAuth.XAA {
		t.Fatalf("http oauth xaa config = %#v", httpServer.OAuth)
	}
	sse, ok := merged["remote_sse"].(sdkmcp.SSEServerConfig)
	if !ok || sse.URL != "http://127.0.0.1:9000/events" || sse.Headers["X-Test"] != "value" {
		t.Fatalf("sse config = %#v", merged["remote_sse"])
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

func TestMergeAgentMCPServersRejectsManagedNames(t *testing.T) {
	tests := []struct {
		name       string
		builtIn    map[string]sdkmcp.ServerConfig
		configured map[string]any
		serverName string
	}{
		{
			name: "active built-in name",
			builtIn: map[string]sdkmcp.ServerConfig{
				"amap_maps": sdkmcp.HTTPServerConfig{URL: "https://mcp.amap.com/mcp"},
			},
			configured: map[string]any{
				"amap_maps": map[string]any{"command": "custom"},
			},
			serverName: "amap_maps",
		},
		{
			name: "reserved nexus namespace",
			configured: map[string]any{
				"nexus_shadow": map[string]any{"command": "custom"},
			},
			serverName: "nexus_shadow",
		},
		{
			name: "reserved nexus root",
			configured: map[string]any{
				"nexus": map[string]any{"command": "custom"},
			},
			serverName: "nexus",
		},
		{
			name:       "reserved GitHub name case insensitive",
			configured: map[string]any{"GitHub": map[string]any{"command": "custom"}},
			serverName: "GitHub",
		},
		{
			name:       "reserved RichMail name",
			configured: map[string]any{"richmail": map[string]any{"command": "custom"}},
			serverName: "richmail",
		},
		{
			name: "reserved connector name",
			configured: map[string]any{
				"amap_maps": map[string]any{"command": "custom"},
			},
			serverName: "amap_maps",
		},
		{
			name: "case insensitive active built-in name",
			builtIn: map[string]sdkmcp.ServerConfig{
				"ManagedTools": sdkmcp.HTTPServerConfig{URL: "https://managed.example.com/mcp"},
			},
			configured: map[string]any{
				"managedtools": map[string]any{"command": "custom"},
			},
			serverName: "managedtools",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := MergeAgentMCPServers(test.builtIn, test.configured)
			if err == nil || !strings.Contains(err.Error(), test.serverName) {
				t.Fatalf("MergeAgentMCPServers() error = %v, want named collision", err)
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

func TestBuildAgentClientOptionsMergesAgentMCPServersBeforeRuntimeResolution(t *testing.T) {
	resolver := &countingMCPRuntimeResolver{}
	options, err := BuildAgentClientOptions(context.Background(), resolver, AgentClientOptionsInput{
		MCPServers: map[string]sdkmcp.ServerConfig{
			"managed": sdkmcp.HTTPServerConfig{URL: "https://managed.example.com/mcp"},
		},
		AgentMCPServers: map[string]any{
			"custom": map[string]any{"command": "custom-mcp"},
		},
	})
	if err != nil {
		t.Fatalf("BuildAgentClientOptions() error = %v", err)
	}
	if resolver.calls != 1 {
		t.Fatalf("runtime resolver calls = %d, want 1", resolver.calls)
	}
	if len(options.MCP.Servers) != 2 {
		t.Fatalf("MCP servers = %#v, want merged servers", options.MCP.Servers)
	}
	if _, ok := options.MCP.Servers["custom"].(sdkmcp.StdioServerConfig); !ok {
		t.Fatalf("custom MCP server = %#v", options.MCP.Servers["custom"])
	}

	resolver.calls = 0
	_, err = BuildAgentClientOptions(context.Background(), resolver, AgentClientOptionsInput{
		AgentMCPServers: map[string]any{
			"broken": map[string]any{"type": "sdk"},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "broken") {
		t.Fatalf("invalid Agent MCP error = %v", err)
	}
	if resolver.calls != 0 {
		t.Fatalf("invalid MCP config reached runtime resolver: calls=%d", resolver.calls)
	}
}

func TestRejectDesktopSandboxRemoteMCP(t *testing.T) {
	configured := map[string]any{
		"remote": map[string]any{
			"type": "http",
			"url":  "https://mcp.example.com/mcp",
		},
	}
	if err := RejectDesktopSandboxRemoteMCP(configured, runtimeKindNXS, "desktop", true, sdkpermission.ModeDefault); (err == nil) != desktopMCPContractPlatform(runtime.GOOS) {
		t.Fatalf("remote MCP platform contract: %v", err)
	}
	if err := RejectDesktopSandboxRemoteMCP(configured, runtimeKindNXS, "desktop", true, sdkpermission.ModeBypassPermissions); err != nil {
		t.Fatalf("explicit bypass should retain existing MCP behavior: %v", err)
	}
	if err := RejectDesktopSandboxRemoteMCP(configured, runtimeKindClaude, "desktop", true, sdkpermission.ModeDefault); err != nil {
		t.Fatalf("Claude must not receive nxs-only rejection: %v", err)
	}
	if err := RejectDesktopSandboxRemoteMCP(map[string]any{
		"local": map[string]any{"type": "stdio", "command": "local-mcp"},
	}, runtimeKindNXS, "desktop", true, sdkpermission.ModeDefault); err != nil {
		t.Fatalf("stdio MCP should remain a separate lifecycle boundary: %v", err)
	}
	for _, mode := range []sdkpermission.Mode{sdkpermission.ModeDefault, sdkpermission.ModeBypassPermissions} {
		err := RejectDesktopSandboxRemoteMCP(map[string]any{
			"helper": map[string]any{
				"type":          "http",
				"url":           "https://mcp.example.com/mcp",
				"headersHelper": "/tmp/unattested-helper",
			},
		}, runtimeKindNXS, "desktop", true, mode)
		if (err == nil) != desktopMCPContractPlatform(runtime.GOOS) {
			t.Fatalf("mode %v helper platform contract: %v", mode, err)
		}
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

func TestWorkspaceInstructionsGoToNXSLoaderOrFallBackToPrompt(t *testing.T) {
	files := []string{"AGENTS.md", "USER.md"}
	render := func() (string, error) { return "# USER.md\n\nnotes", nil }

	env := workspaceInstructionRuntimeEnv(runtimeKindNXS, files)
	if env[nexusWorkspaceInstructionFilesEnvName] != "AGENTS.md,USER.md" {
		t.Fatalf("nxs env = %#v, want named workspace files", env)
	}
	prompt, err := withWorkspaceInstructionPrompt(runtimeKindNXS, "base", render)
	if err != nil || prompt != "base" {
		t.Fatalf("nxs prompt = %q, %v; workspace files must stay out of the system prompt", prompt, err)
	}

	if env := workspaceInstructionRuntimeEnv(runtimeKindClaude, files); env != nil {
		t.Fatalf("claude env = %#v, want none", env)
	}
	prompt, err = withWorkspaceInstructionPrompt(runtimeKindClaude, "base", render)
	if err != nil || prompt != "base\n\n---\n\n# USER.md\n\nnotes" {
		t.Fatalf("claude prompt = %q, %v; want rendered workspace files appended", prompt, err)
	}
}
