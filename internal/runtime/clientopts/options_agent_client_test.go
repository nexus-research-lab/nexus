package clientopts

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
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

type fakeRuntimeConfigForRuntimeResolver struct {
	config      *RuntimeConfig
	runtimeKind string
	calls       int
	legacyCalls int
}

func (r *fakeRuntimeConfigForRuntimeResolver) ResolveRuntimeConfig(
	context.Context,
	string,
	string,
) (*RuntimeConfig, error) {
	r.legacyCalls++
	return r.config, nil
}

func (r *fakeRuntimeConfigForRuntimeResolver) ResolveRuntimeConfigForRuntime(
	_ context.Context,
	_ string,
	_ string,
	runtimeKind string,
) (*RuntimeConfig, error) {
	r.calls++
	r.runtimeKind = runtimeKind
	return r.config, nil
}

func TestBuildAgentClientOptionsUsesProviderRuntimeEnv(t *testing.T) {
	thinkingTokens := 2048
	maxTurns := 8
	resolveCalls := 0
	options, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{
		config: &RuntimeConfig{
			AuthToken: "token-1",
			BaseURL:   "https://provider.example.com",
			Model:     "kimi-k2",
		},
		calls: &resolveCalls,
	}, AgentClientOptionsInput{
		WorkspacePath:      "/tmp/workspace",
		Provider:           "kimi",
		AllowedTools:       []string{"Read"},
		DisallowedTools:    []string{"Edit"},
		SettingSources:     []string{"project"},
		AppendSystemPrompt: "你是测试 Agent",
		ResumeSessionID:    "sdk-session-1",
		MaxThinkingTokens:  &thinkingTokens,
		MaxTurns:           &maxTurns,
	})
	if err != nil {
		t.Fatalf("BuildAgentClientOptions 失败: %v", err)
	}
	if options.Runtime.Kind != agentclient.RuntimeNXS {
		t.Fatalf("空 runtime kind 应默认启用 nxs: %+v", options.Runtime)
	}
	if options.Runtime.PermissionMode != sdkpermission.ModeDefault {
		t.Fatalf("默认权限模式不正确: %+v", options)
	}
	if !options.Runtime.AllowDangerouslySkipPermissions {
		t.Fatalf("运行时应允许后续切换到 bypassPermissions，避免复用 session 时发送失败")
	}
	if options.Env[anthropicModelEnvName] != "kimi-k2" {
		t.Fatalf("运行时模型未写入 env: %+v", options.Env)
	}
	if options.Env[anthropicAuthTokenEnvName] != "token-1" {
		t.Fatalf("Anthropic-compatible bearer token 未写入 env: %+v", options.Env)
	}
	if _, ok := options.Env[anthropicAPIKeyEnvName]; ok {
		t.Fatalf("Anthropic-compatible 非官方 endpoint 不应写入 API key env: %+v", options.Env)
	}
	if options.Env[nexusAPIProviderEnvName] != "anthropic-compatible" {
		t.Fatalf("Anthropic-compatible provider 标记未写入 env: %+v", options.Env)
	}
	if options.Model != "kimi-k2" {
		t.Fatalf("运行时模型未写入 SDK options: %+v", options)
	}
	if options.Env["ENABLE_TOOL_SEARCH"] != "false" {
		t.Fatalf("kimi 模型应关闭 tool search: %+v", options.Env)
	}
	if options.Env[claudeAutoCompactPctOverrideEnvName] != defaultClaudeAutoCompactPctOverride {
		t.Fatalf("默认自动压缩阈值未注入: %+v", options.Env)
	}
	if options.Session.ResumeID != "sdk-session-1" {
		t.Fatalf("resume session_id 不正确: %+v", options)
	}
	if options.Runtime.MaxThinkingTokens != 2048 || options.Runtime.MaxTurns != 8 {
		t.Fatalf("思考/轮次限制未透传: %+v", options)
	}
	for _, tool := range []string{"Edit", "ScheduleWakeup", "CronCreate", "CronList", "CronDelete"} {
		if !containsTool(options.Tools.Deny, tool) {
			t.Fatalf("运行时 deny 工具缺少 %s: %+v", tool, options.Tools.Deny)
		}
	}
	if resolveCalls != 1 {
		t.Fatalf("provider runtime config 解析次数不正确: got=%d want=1", resolveCalls)
	}
}

func TestBuildAgentClientOptionsUsesOfficialAnthropicAPIKeyEnv(t *testing.T) {
	options, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{
		config: &RuntimeConfig{
			Provider:  "anthropic",
			AuthToken: "official-key",
			BaseURL:   "https://api.anthropic.com",
			Model:     "claude-sonnet-4-5",
		},
	}, AgentClientOptionsInput{
		WorkspacePath: "/tmp/workspace",
		Provider:      "anthropic",
	})
	if err != nil {
		t.Fatalf("BuildAgentClientOptions 失败: %v", err)
	}
	if options.Env[anthropicAPIKeyEnvName] != "official-key" {
		t.Fatalf("官方 Anthropic API key 未写入 env: %+v", options.Env)
	}
	if _, ok := options.Env[anthropicAuthTokenEnvName]; ok {
		t.Fatalf("官方 Anthropic runtime 不应写入 OAuth token env: %+v", options.Env)
	}
}

func TestAnthropicRuntimeEnvRoutesCredentialsByBaseURL(t *testing.T) {
	tests := []struct {
		name          string
		baseURL       string
		authToken     string
		wantAPIKey    string
		wantAuthToken string
	}{
		{name: "compatible gateway", baseURL: "https://provider.example.com/anthropic", authToken: "token-1", wantAuthToken: "token-1"},
		{name: "first party anthropic", baseURL: "https://api.anthropic.com", authToken: "token-1", wantAPIKey: "token-1"},
		{name: "empty base url defaults first party", baseURL: "", authToken: "token-1", wantAPIKey: "token-1"},
		{name: "empty token", baseURL: "https://provider.example.com/anthropic", authToken: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := anthropicRuntimeEnvFromConfig(&RuntimeConfig{
				AuthToken: tt.authToken,
				BaseURL:   tt.baseURL,
				Model:     "model-1",
			})
			if got := env[anthropicAPIKeyEnvName]; got != tt.wantAPIKey {
				t.Fatalf("%s = %q, want %q; env=%+v", anthropicAPIKeyEnvName, got, tt.wantAPIKey, env)
			}
			if got := env[anthropicAuthTokenEnvName]; got != tt.wantAuthToken {
				t.Fatalf("%s = %q, want %q; env=%+v", anthropicAuthTokenEnvName, got, tt.wantAuthToken, env)
			}
		})
	}
}

func TestBuildAgentClientOptionsAllowsExtraEnvOverride(t *testing.T) {
	options, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		WorkspacePath: "/tmp/workspace",
		ExtraEnv: map[string]string{
			claudeAutoCompactPctOverrideEnvName: "80",
		},
	})
	if err != nil {
		t.Fatalf("BuildAgentClientOptions 失败: %v", err)
	}
	if options.Env[claudeAutoCompactPctOverrideEnvName] != "80" {
		t.Fatalf("ExtraEnv 应覆盖默认自动压缩阈值: %+v", options.Env)
	}
}

func TestBuildAgentClientOptionsAllowsExtraEnvOverrideNXSCacheDefaults(t *testing.T) {
	options, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		RuntimeKind: runtimeKindNXS,
		ExtraEnv: map[string]string{
			nxsCachedMicrocompactEnvName:     "0",
			nxsAPIClearToolResultsEnvName:    "",
			nxsPromptCache1hEligibleEnvName:  "0",
			nxsPromptCache1hAllowlistEnvName: "agent:*",
			nxsAgentSDKDiagnosticsEnvName:    "",
		},
	})
	if err != nil {
		t.Fatalf("BuildAgentClientOptions 失败: %v", err)
	}
	if options.Env[nxsCachedMicrocompactEnvName] != "0" ||
		options.Env[nxsAPIClearToolResultsEnvName] != "" ||
		options.Env[nxsPromptCache1hEligibleEnvName] != "0" ||
		options.Env[nxsPromptCache1hAllowlistEnvName] != "agent:*" ||
		options.Env[nxsAgentSDKDiagnosticsEnvName] != "" {
		t.Fatalf("ExtraEnv 应覆盖 nxs cache 默认值: %+v", options.Env)
	}
	if options.Env[nxsAPIClearToolUsesEnvName] != "1" {
		t.Fatalf("nxs tool use 清理默认值丢失: %+v", options.Env)
	}
}

func TestBuildAgentClientOptionsInjectsReasoningCapabilities(t *testing.T) {
	options, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{
		config: &RuntimeConfig{
			AuthToken: "token-1",
			BaseURL:   "https://provider.example.com",
			Model:     "glm-5.1",
			Reasoning: true,
		},
	}, AgentClientOptionsInput{
		WorkspacePath: "/tmp/workspace",
	})
	if err != nil {
		t.Fatalf("BuildAgentClientOptions 失败: %v", err)
	}
	for _, key := range []string{
		"ANTHROPIC_DEFAULT_OPUS_MODEL_SUPPORTED_CAPABILITIES",
		"ANTHROPIC_DEFAULT_SONNET_MODEL_SUPPORTED_CAPABILITIES",
		"ANTHROPIC_DEFAULT_HAIKU_MODEL_SUPPORTED_CAPABILITIES",
	} {
		if options.Env[key] != "thinking" {
			t.Fatalf("%s = %q, want thinking; env=%+v", key, options.Env[key], options.Env)
		}
	}
}

func TestBuildAgentClientOptionsUsesBridgeRuntimeKind(t *testing.T) {
	t.Setenv(nexusAppRootEnvName, "")
	t.Setenv(nexusNXSCommandPathEnvName, "")
	t.Setenv(nxsAgentSDKDiagnosticsEnvName, "stderr")
	t.Setenv(nxsAgentSDKDebugEnvName, "1")
	t.Setenv(nxsAgentSDKProviderDebugBodyEnvName, "full")

	options, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		RuntimeKind: runtimeKindNXS,
	})
	if err != nil {
		t.Fatalf("BuildAgentClientOptions 失败: %v", err)
	}
	if options.Runtime.Kind != agentclient.RuntimeNXS {
		t.Fatalf("未把 nxs runtime kind 交给 bridge: %+v", options.Runtime)
	}
	if strings.TrimSpace(options.CLIPath) != "" {
		t.Fatalf("nxs 默认路径不应由 Nexus 解析: CLIPath=%q", options.CLIPath)
	}
	for _, key := range []string{
		nxsCachedMicrocompactEnvName,
		nxsAPIClearToolResultsEnvName,
		nxsAPIClearToolUsesEnvName,
		nxsPromptCache1hEligibleEnvName,
	} {
		if options.Env[key] != "1" {
			t.Fatalf("%s = %q, want 1; env=%+v", key, options.Env[key], options.Env)
		}
	}
	if options.Env[nxsPromptCache1hAllowlistEnvName] != "sdk" {
		t.Fatalf("%s = %q, want sdk; env=%+v", nxsPromptCache1hAllowlistEnvName, options.Env[nxsPromptCache1hAllowlistEnvName], options.Env)
	}
	for _, key := range []string{
		nxsAgentSDKDiagnosticsEnvName,
		nxsAgentSDKDebugEnvName,
		nxsAgentSDKProviderDebugBodyEnvName,
	} {
		if options.Env[key] != "" {
			t.Fatalf("%s = %q, want empty; env=%+v", key, options.Env[key], options.Env)
		}
	}
}

func TestBuildAgentClientOptionsEnablesNXSAgentSDKDiagnostics(t *testing.T) {
	options, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		RuntimeKind:                runtimeKindNXS,
		AgentSDKDiagnosticsEnabled: true,
	})
	if err != nil {
		t.Fatalf("BuildAgentClientOptions 失败: %v", err)
	}
	if options.Env[nxsAgentSDKDiagnosticsEnvName] != "stderr" {
		t.Fatalf("%s = %q, want stderr; env=%+v", nxsAgentSDKDiagnosticsEnvName, options.Env[nxsAgentSDKDiagnosticsEnvName], options.Env)
	}
	if _, ok := options.Env[nxsAgentSDKProviderDebugBodyEnvName]; ok {
		t.Fatalf("开启 diagnostics 不应强制请求体 dump 范围: %+v", options.Env)
	}
}

func TestBuildAgentClientOptionsDefaultsToNXSChatCompletionsProviderEnv(t *testing.T) {
	resolver := &fakeRuntimeConfigForRuntimeResolver{
		config: &RuntimeConfig{
			Provider:  "openai",
			AuthToken: "openai-token",
			BaseURL:   "https://api.openai.com/v1",
			Model:     "gpt-4o",
			APIFormat: apiFormatChatCompletions,
		},
	}
	options, err := BuildAgentClientOptions(context.Background(), resolver, AgentClientOptionsInput{})
	if err != nil {
		t.Fatalf("BuildAgentClientOptions 失败: %v", err)
	}
	if resolver.calls != 1 || resolver.legacyCalls != 0 || resolver.runtimeKind != runtimeKindNXS {
		t.Fatalf("未按 runtime kind 解析 provider: calls=%d legacy=%d kind=%q", resolver.calls, resolver.legacyCalls, resolver.runtimeKind)
	}
	if options.Runtime.Kind != agentclient.RuntimeNXS {
		t.Fatalf("未启用 nxs runtime: %+v", options.Runtime)
	}
	wantEnv := map[string]string{
		"OPENAI_API_KEY":             "openai-token",
		"OPENAI_BASE_URL":            "https://api.openai.com/v1",
		"OPENAI_MODEL":               "gpt-4o",
		"CLAUDE_CODE_SUBAGENT_MODEL": "gpt-4o",
		NexusRuntimeProviderEnvName:  "openai",
		nexusAPIProviderEnvName:      "openai",
	}
	for key, want := range wantEnv {
		if options.Env[key] != want {
			t.Fatalf("%s=%q, want %q; env=%+v", key, options.Env[key], want, options.Env)
		}
	}
	if _, ok := options.Env[anthropicAuthTokenEnvName]; ok {
		t.Fatalf("nxs chat_completions 不应注入 Anthropic token: %+v", options.Env)
	}
	if _, ok := options.Env[anthropicAPIKeyEnvName]; ok {
		t.Fatalf("nxs chat_completions 不应注入 Anthropic API key: %+v", options.Env)
	}
	if options.Model != "gpt-4o" {
		t.Fatalf("运行时模型未写入 SDK options: %+v", options)
	}
}

func TestBuildAgentClientOptionsRejectsClaudeNonAnthropicAPIFormat(t *testing.T) {
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
	if err == nil || !strings.Contains(err.Error(), "暂不可用于 Agent runtime") {
		t.Fatalf("Claude runtime 下非 anthropic_messages provider 应被拒绝: %v", err)
	}
}

func TestBuildAgentClientOptionsRejectsNXSResponsesAPIFormat(t *testing.T) {
	_, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{
		config: &RuntimeConfig{
			AuthToken: "token-1",
			BaseURL:   "https://provider.example.com/v1",
			Model:     "gpt-4.1",
			APIFormat: "responses",
		},
	}, AgentClientOptionsInput{
		RuntimeKind: runtimeKindNXS,
	})
	if err == nil || !strings.Contains(err.Error(), "暂不可用于 Agent runtime") {
		t.Fatalf("nxs responses provider 应被拒绝: %v", err)
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

func TestBuildAgentClientOptionsInjectsWorkspaceBinEnv(t *testing.T) {
	configDir := filepath.Join(t.TempDir(), ".nexus")
	t.Setenv("NEXUS_CONFIG_DIR", configDir)
	workspacePath := filepath.Join(os.TempDir(), "nexus-owner", "agent-1")
	options, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		WorkspacePath: workspacePath,
	})
	if err != nil {
		t.Fatalf("BuildAgentClientOptions 失败: %v", err)
	}
	pathItems := strings.Split(options.Env["PATH"], string(os.PathListSeparator))
	expectedBinDir := filepath.Join(configDir, ".agents", "bin")
	if len(pathItems) == 0 || pathItems[0] != expectedBinDir {
		t.Fatalf("运行时 PATH 未优先注入共享 runtime bin: %q", options.Env["PATH"])
	}
	if strings.TrimSpace(options.Env["NEXUS_PROJECT_ROOT"]) == "" {
		t.Fatalf("运行时未注入 NEXUS_PROJECT_ROOT: %+v", options.Env)
	}
	if options.Env[nexusctlWorkspacePathEnvName] != workspacePath {
		t.Fatalf("运行时未注入 nexusctl workspace 路径: %+v", options.Env)
	}
}

func TestBuildAgentClientOptionsInjectsMCPServerConfigs(t *testing.T) {
	options, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		WorkspacePath: "/tmp/workspace",
		MCPServers: map[string]sdkmcp.ServerConfig{
			"amap_maps": sdkmcp.HTTPServerConfig{URL: "https://mcp.amap.com/mcp?key=test-key"},
		},
	})
	if err != nil {
		t.Fatalf("BuildAgentClientOptions 失败: %v", err)
	}
	if len(options.MCP.Servers) != 1 {
		t.Fatalf("MCP server config 未注入: %+v", options.MCP)
	}
	if _, ok := options.MCP.Servers["amap_maps"].(sdkmcp.HTTPServerConfig); !ok {
		t.Fatalf("MCP server 类型不正确: %+v", options.MCP.Servers["amap_maps"])
	}
}

func TestBuildAgentClientOptionsInjectsScopedUserEnv(t *testing.T) {
	ctx := authctx.WithState(context.Background(), authctx.State{
		AuthRequired: true,
		UserCount:    2,
	})
	ctx = authctx.WithPrincipal(ctx, &authctx.Principal{
		UserID:     "user-123",
		Username:   "alice",
		AuthMethod: "test",
	})

	options, err := BuildAgentClientOptions(ctx, fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		WorkspacePath: "/tmp/workspace",
	})
	if err != nil {
		t.Fatalf("BuildAgentClientOptions 失败: %v", err)
	}
	if options.Env[nexusctlUserIDEnvName] != "user-123" {
		t.Fatalf("未把当前 user_id 注入运行时环境: %+v", options.Env)
	}
	if options.Env[nexusRuntimeUserIDEnvName] != "user-123" {
		t.Fatalf("未把通用运行时 user_id 注入环境: %+v", options.Env)
	}
	if options.Env[nexusRuntimeScopeModeEnvName] != "user_scoped" {
		t.Fatalf("未把多用户作用域模式注入环境: %+v", options.Env)
	}
}

func TestBuildAgentClientOptionsInjectsSingleUserScopeEnv(t *testing.T) {
	ctx := authctx.WithState(context.Background(), authctx.State{
		AuthRequired: false,
		UserCount:    0,
	})

	options, err := BuildAgentClientOptions(ctx, fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		WorkspacePath: "/tmp/workspace",
	})
	if err != nil {
		t.Fatalf("BuildAgentClientOptions 失败: %v", err)
	}
	if options.Env[nexusRuntimeScopeModeEnvName] != "single_user" {
		t.Fatalf("未把单用户作用域模式注入环境: %+v", options.Env)
	}
	if options.Env[nexusRuntimeUserIDEnvName] != authctx.SystemUserID {
		t.Fatalf("未把单用户保底主体注入环境: %+v", options.Env)
	}
}

func TestBuildAgentClientOptionsBypassKeepsQuestionChannel(t *testing.T) {
	var handledTools []string
	handler := func(_ context.Context, request sdkpermission.Request) (sdkpermission.Decision, error) {
		handledTools = append(handledTools, request.ToolName)
		updatedInput := map[string]any{
			"answers": []any{
				map[string]any{"question_index": float64(0), "text": "继续"},
			},
		}
		return sdkpermission.Allow(updatedInput, nil), nil
	}

	options, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		WorkspacePath:     "/tmp/workspace",
		PermissionMode:    sdkpermission.ModeBypassPermissions,
		PermissionHandler: handler,
	})
	if err != nil {
		t.Fatalf("BuildAgentClientOptions 失败: %v", err)
	}
	if options.Callbacks.PermissionHandler == nil {
		t.Fatalf("bypass 模式应保留 AskUserQuestion 交互通道")
	}
	if !options.Runtime.AllowDangerouslySkipPermissions {
		t.Fatalf("bypass 模式应在 session 启动时显式启用 allowDangerouslySkipPermissions")
	}

	questionDecision, err := options.Callbacks.PermissionHandler(context.Background(), sdkpermission.Request{
		ToolName: " AskUserQuestion ",
		Input: map[string]any{
			"questions": []any{"测试问题"},
		},
	})
	if err != nil {
		t.Fatalf("AskUserQuestion handler 返回错误: %v", err)
	}
	if len(handledTools) != 1 || handledTools[0] != " AskUserQuestion " {
		t.Fatalf("AskUserQuestion 未走真实交互处理器: tools=%+v", handledTools)
	}
	if questionDecision.UpdatedInput["answers"] == nil {
		t.Fatalf("AskUserQuestion 未保留用户答案: %+v", questionDecision)
	}

	bypassDecision, err := options.Callbacks.PermissionHandler(context.Background(), sdkpermission.Request{
		ToolName: "Bash",
		Input: map[string]any{
			"command": "pwd",
		},
	})
	if err != nil {
		t.Fatalf("bypass 工具自动放行失败: %v", err)
	}
	if len(handledTools) != 1 {
		t.Fatalf("非提问工具不应进入交互处理器: tools=%+v", handledTools)
	}
	if bypassDecision.Behavior != sdkpermission.BehaviorAllow {
		t.Fatalf("bypass 工具应自动放行: %+v", bypassDecision)
	}
	if bypassDecision.UpdatedInput["command"] != "pwd" {
		t.Fatalf("bypass 工具输入未原样保留: %+v", bypassDecision.UpdatedInput)
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
