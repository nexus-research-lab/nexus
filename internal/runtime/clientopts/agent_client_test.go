package clientopts

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"

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
		WorkspacePath:     "/tmp/workspace",
		Provider:          "kimi",
		ResumeSessionID:   "sdk-session-1",
		MaxThinkingTokens: &thinkingTokens,
		MaxTurns:          &maxTurns,
	})
	if err != nil {
		t.Fatalf("BuildAgentClientOptions 失败: %v", err)
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
	if options.Env[anthropicAPIKeyEnvName] != "" {
		t.Fatalf("Anthropic-compatible 非官方 endpoint 应清空 API key env，避免继承脏 key: %+v", options.Env)
	}
	if options.Env[nexusAPIProviderEnvName] != "anthropic-compatible" {
		t.Fatalf("Anthropic-compatible provider 标记未写入 env: %+v", options.Env)
	}
	if options.Env[claudeEmitToolUseSummariesEnvName] != "0" {
		t.Fatalf("ToolUseSummary 默认开关不正确: %+v", options.Env)
	}
	if options.Model != "kimi-k2" {
		t.Fatalf("运行时模型未写入 SDK options: %+v", options)
	}
	if options.Session.ResumeID != "sdk-session-1" {
		t.Fatalf("resume session_id 不正确: %+v", options)
	}
	if options.Runtime.MaxThinkingTokens != 2048 || options.Runtime.MaxTurns != 8 {
		t.Fatalf("思考/轮次限制未透传: %+v", options)
	}
	if resolveCalls != 1 {
		t.Fatalf("provider runtime config 解析次数不正确: got=%d want=1", resolveCalls)
	}
}

func TestAnthropicRuntimeEnvRoutesCredentialsByBaseURL(t *testing.T) {
	tests := []struct {
		name          string
		baseURL       string
		authToken     string
		wantAPIKey    string
		wantAuthToken string
		wantPresent   bool
	}{
		{
			name:          "compatible gateway",
			baseURL:       "https://provider.example.com/anthropic",
			authToken:     "token-1",
			wantAuthToken: "token-1",
			wantPresent:   true,
		},
		{
			name:        "first party anthropic",
			baseURL:     "https://api.anthropic.com",
			authToken:   "token-1",
			wantAPIKey:  "token-1",
			wantPresent: true,
		},
		{
			name:        "empty base url defaults first party",
			authToken:   "token-1",
			wantAPIKey:  "token-1",
			wantPresent: true,
		},
		{name: "empty token", baseURL: "https://provider.example.com/anthropic", authToken: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := anthropicRuntimeEnvFromConfig(&RuntimeConfig{
				AuthToken: tt.authToken,
				BaseURL:   tt.baseURL,
				Model:     "model-1",
			})
			apiKey, apiKeyExists := env[anthropicAPIKeyEnvName]
			authToken, authTokenExists := env[anthropicAuthTokenEnvName]
			if apiKey != tt.wantAPIKey || authToken != tt.wantAuthToken ||
				apiKeyExists != tt.wantPresent || authTokenExists != tt.wantPresent {
				t.Fatalf("credential route = api_key:%q/%t auth_token:%q/%t; env=%+v", apiKey, apiKeyExists, authToken, authTokenExists, env)
			}
		})
	}
}

func TestBuildAgentClientOptionsProjectsToolSearchByRuntime(t *testing.T) {
	nxsOptions, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		RuntimeKind:       runtimeKindNXS,
		ToolSearchEnabled: true,
	})
	if err != nil {
		t.Fatalf("构建 nxs options 失败: %v", err)
	}
	if nxsOptions.Env[enableToolSearchEnvName] != "1" || nxsOptions.Env[nexusEnableToolSearchEnvName] != "1" {
		t.Fatalf("nxs ToolSearch 开关未投影: %+v", nxsOptions.Env)
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

func TestBuildAgentClientOptionsAlignsClaudeToolsWithNXSDefaults(t *testing.T) {
	options, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		RuntimeKind: runtimeKindClaude,
	})
	if err != nil {
		t.Fatalf("构建 Claude options 失败: %v", err)
	}
	want := "Agent,AskUserQuestion,Bash,Edit,ExitPlanMode,Read,Skill,TaskCreate,TaskGet,TaskList,TaskOutput,TaskStop,TaskUpdate,WebFetch,WebSearch,Write"
	if got := strings.Join(options.Tools.Available, ","); got != want {
		t.Fatalf("Claude 可见工具 = %q, want %q", got, want)
	}

	nxsOptions, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		RuntimeKind: runtimeKindNXS,
	})
	if err != nil {
		t.Fatalf("构建 nxs options 失败: %v", err)
	}
	if nxsOptions.Tools.Available != nil {
		t.Fatalf("nxs 应继续使用原生 catalog gate: %#v", nxsOptions.Tools.Available)
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

func TestBuildAgentClientOptionsUsesNXSResponsesProviderEnv(t *testing.T) {
	clearInheritedAnthropicProviderEnv(t)
	options, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{
		config: &RuntimeConfig{
			Provider:      "openai-responses",
			AuthToken:     "token-1",
			BaseURL:       "https://provider.example.com/v1",
			Model:         "gpt-4.1",
			APIFormat:     apiFormatResponses,
			Vision:        true,
			ContextWindow: 1_047_576,
		},
	}, AgentClientOptionsInput{
		RuntimeKind: runtimeKindNXS,
	})
	if err != nil {
		t.Fatalf("nxs responses provider 应可用: %v", err)
	}
	wantEnv := map[string]string{
		"OPENAI_API_KEY":                  "token-1",
		"OPENAI_BASE_URL":                 "https://provider.example.com/v1",
		"OPENAI_MODEL":                    "gpt-4.1",
		"NEXUS_SUBAGENT_MODEL":            "gpt-4.1",
		NexusRuntimeProviderEnvName:       "openai-responses",
		nexusAPIProviderEnvName:           "openai",
		nexusOpenAIProtocolEnvName:        apiFormatResponses,
		nexusMaxContextTokensEnvName:      "1047576",
		nexusModelSupportsVisionEnvName:   "true",
		nexusMultimodalUserContentEnvName: "1",
		nexusMultimodalToolResultEnvName:  "1",
	}
	for key, want := range wantEnv {
		if options.Env[key] != want {
			t.Fatalf("%s=%q, want %q; env=%+v", key, options.Env[key], want, options.Env)
		}
	}
	for _, key := range []string{anthropicAuthTokenEnvName, anthropicAPIKeyEnvName, anthropicBaseURLEnvName, anthropicModelEnvName} {
		if _, exists := options.Env[key]; exists {
			t.Fatalf("nxs responses 不应注入 %s: %+v", key, options.Env)
		}
	}
	if options.Model != "gpt-4.1" {
		t.Fatalf("运行时模型未写入 SDK options: %+v", options)
	}
}

func clearInheritedAnthropicProviderEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		anthropicAuthTokenEnvName,
		anthropicAPIKeyEnvName,
		anthropicBaseURLEnvName,
		anthropicModelEnvName,
	} {
		t.Setenv(key, "")
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

func TestBuildAgentClientOptionsNeverInjectsRawNexusCLI(t *testing.T) {
	configDir := filepath.Join(t.TempDir(), ".nexus")
	t.Setenv("NEXUS_CONFIG_DIR", configDir)
	t.Setenv("NEXUS_STATE_ROOT", "")
	t.Setenv(nexusctlCommandPathEnvName, "/opt/nexus/bin/nexusctl")
	t.Setenv(nexuscfgCommandPathEnvName, "/opt/nexus/bin/nexuscfg")
	t.Setenv(legacyNexusCommandPathEnvName, "/opt/nexus/bin/nexus")
	t.Setenv(nexusctlUserIDEnvName, "ambient-owner")
	t.Setenv(nexusctlWorkspacePathEnvName, "/ambient/workspace")
	workspacePath := filepath.Join(os.TempDir(), "nexus-owner", "agent-1")
	options, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		WorkspacePath: workspacePath,
	})
	if err != nil {
		t.Fatalf("BuildAgentClientOptions 失败: %v", err)
	}
	for _, key := range []string{
		nexusctlCommandPathEnvName,
		nexuscfgCommandPathEnvName,
		nexusctlUserIDEnvName,
		nexusctlWorkspacePathEnvName,
		protocol.NexusConfigBrokerURLEnvName,
		protocol.NexusConfigCapabilityTokenEnvName,
		legacyNexusCommandPathEnvName,
	} {
		if value := strings.TrimSpace(options.Env[key]); value != "" {
			t.Fatalf("Agent runtime 泄漏原始 CLI 环境 %s=%q: %+v", key, value, options.Env)
		}
	}
}

func TestBuildAgentClientOptionsExposesOnlyNexuscfgWithRuntimeCapability(t *testing.T) {
	t.Setenv(nexusctlCommandPathEnvName, "/opt/nexus/bin/nexusctl")
	t.Setenv(nexuscfgCommandPathEnvName, "/opt/nexus/bin/nexuscfg")
	options, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		OwnerUserID:   "owner-a",
		WorkspacePath: "/tmp/ordinary-agent",
		ConfigurationEnv: map[string]string{
			protocol.NexusConfigBrokerURLEnvName:       "http://127.0.0.1:8010/nexus/v1/internal/runtime/configuration",
			protocol.NexusConfigCapabilityTokenEnvName: "runtime-token",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if options.Env[nexuscfgCommandPathEnvName] != "/opt/nexus/bin/nexuscfg" ||
		options.Env[protocol.NexusConfigCapabilityTokenEnvName] != "runtime-token" {
		t.Fatalf("普通 Agent 未获得 nexuscfg capability: %+v", options.Env)
	}
	if options.Env[nexusctlCommandPathEnvName] != "" ||
		options.Env[nexusctlWorkspacePathEnvName] != "" ||
		options.Env[nexusctlUserIDEnvName] != "" {
		t.Fatalf("普通 Agent 不应获得 nexusctl owner capability: %+v", options.Env)
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

func TestBuildAgentClientOptionsDoesNotExposeNexusCLIWithoutCapability(t *testing.T) {
	ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID: "ordinary-owner",
	})
	options, err := BuildAgentClientOptions(ctx, fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		WorkspacePath: "/tmp/ordinary-agent",
	})
	if err != nil {
		t.Fatalf("BuildAgentClientOptions 失败: %v", err)
	}
	for _, key := range []string{
		nexusctlUserIDEnvName,
		nexusctlCommandPathEnvName,
		nexuscfgCommandPathEnvName,
		nexusctlWorkspacePathEnvName,
	} {
		if value := strings.TrimSpace(options.Env[key]); value != "" {
			t.Fatalf("未授权 runtime 泄漏 %s=%q: %+v", key, value, options.Env)
		}
	}
	if options.Env[nexusRuntimeUserIDEnvName] != "ordinary-owner" {
		t.Fatalf("通用 runtime user scope 不应随 CLI capability 一起删除: %+v", options.Env)
	}
}

func TestBuildAgentClientOptionsInjectsControlCLIsForMainAgent(t *testing.T) {
	t.Setenv(nexusctlCommandPathEnvName, "/opt/nexus/bin/nexusctl")
	t.Setenv(nexuscfgCommandPathEnvName, "/opt/nexus/bin/nexuscfg")
	ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{UserID: "owner-a"})
	options, err := BuildAgentClientOptions(ctx, fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		OwnerUserID:   "owner-a",
		WorkspacePath: "/tmp/main-agent",
		IsMainAgent:   true,
	})
	if err != nil {
		t.Fatalf("BuildAgentClientOptions 失败: %v", err)
	}
	for key, want := range map[string]string{
		nexusctlCommandPathEnvName: "/opt/nexus/bin/nexusctl",
		nexuscfgCommandPathEnvName: "/opt/nexus/bin/nexuscfg",
	} {
		if got := options.Env[key]; got != want {
			t.Fatalf("主智能体 %s=%q, want %q", key, got, want)
		}
	}
}

func TestBuildAgentClientOptionsProtectsManagedUserDirectories(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	t.Setenv("NEXUS_APP_ROOT", "/tmp/host-app")
	t.Setenv("DATABASE_URL", "/tmp/host.db")
	t.Setenv("CONNECTOR_CREDENTIALS_KEY", "host-secret")
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "host-token")
	t.Setenv(nexusMemoryDirEnvName, "/tmp/host-memory")
	t.Setenv(nexusEnableRemoteMemoryEnvName, "1")
	t.Setenv(nexusRemoteMemoryDirEnvName, "/tmp/host-remote-memory")
	ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{UserID: "user-123"})

	options, err := BuildAgentClientOptions(ctx, fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		WorkspacePath: "/tmp/workspace",
		ExtraEnv: map[string]string{
			nexusConfigDirEnvName:          "/tmp/escaped-nexus",
			claudeConfigDirEnvName:         "/tmp/escaped-claude",
			"HOME":                         "/tmp/escaped-home",
			"TMPDIR":                       "/tmp/escaped-tmp",
			appfs.NexusStateRootEnvName:    "/tmp/escaped-state",
			"WORKSPACE_PATH":               "/tmp/escaped-workspace",
			"DATABASE_URL":                 "/tmp/escaped-db",
			"CONNECTOR_CREDENTIALS_KEY":    "request-secret",
			nexusMemoryDirEnvName:          "/tmp/escaped-memory",
			nexusEnableRemoteMemoryEnvName: "1",
			nexusRemoteMemoryDirEnvName:    "/tmp/escaped-remote-memory",
		},
	})
	if err != nil {
		t.Fatalf("BuildAgentClientOptions 失败: %v", err)
	}
	expectedRuntimeRoot := filepath.Join(stateRoot, "users", "user-123", "runtime")
	if options.Env[nexusConfigDirEnvName] != expectedRuntimeRoot ||
		options.Env[claudeConfigDirEnvName] != expectedRuntimeRoot ||
		options.Env["HOME"] != filepath.Join(expectedRuntimeRoot, "home") ||
		options.Env["USERPROFILE"] != filepath.Join(expectedRuntimeRoot, "home") ||
		options.Env["TMPDIR"] != filepath.Join(expectedRuntimeRoot, "tmp") ||
		options.Env["TEMP"] != filepath.Join(expectedRuntimeRoot, "tmp") ||
		options.Env["TMP"] != filepath.Join(expectedRuntimeRoot, "tmp") ||
		options.Env[appfs.NexusStateRootEnvName] != "" ||
		options.Env[nexusAppRootEnvName] != "" ||
		options.Env["DATABASE_URL"] != "" ||
		options.Env[connectorCredentialsKeyEnvName] != "" ||
		options.Env[anthropicAuthTokenEnvName] != "" ||
		options.Env[nexusMemoryDirEnvName] != "/tmp/workspace" ||
		options.Env[nexusEnableRemoteMemoryEnvName] != "" ||
		options.Env[nexusRemoteMemoryDirEnvName] != "" ||
		options.Env[workspacePathEnvName] != "/tmp/workspace" ||
		options.Env[nexusctlWorkspacePathEnvName] != "" {
		t.Fatalf("ExtraEnv 覆盖了宿主管理的用户目录: %+v", options.Env)
	}
}

func TestBuildAgentClientOptionsProtectsScopedIdentityFromExtraEnv(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	ctx := authctx.WithState(context.Background(), authctx.State{
		AuthRequired: true,
		UserCount:    2,
	})
	ctx = authctx.WithPrincipal(ctx, &authctx.Principal{UserID: "user-a"})

	options, err := BuildAgentClientOptions(ctx, fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		WorkspacePath: "/tmp/workspace",
		ExtraEnv: map[string]string{
			nexusctlUserIDEnvName:                      "user-b",
			nexusRuntimeUserIDEnvName:                  "user-b",
			nexusRuntimeScopeModeEnvName:               "single_user",
			protocol.NexusConfigBrokerURLEnvName:       "http://127.0.0.1:9/forged",
			protocol.NexusConfigCapabilityTokenEnvName: "forged",
			"NEXUS_RUNTIME_ISOLATION_MODE":             "off",
		},
		ConfigurationEnv: map[string]string{
			protocol.NexusConfigBrokerURLEnvName:       "http://127.0.0.1:8010/nexus/v1/internal/runtime/configuration",
			protocol.NexusConfigCapabilityTokenEnvName: "trusted",
		},
	})
	if err != nil {
		t.Fatalf("BuildAgentClientOptions 失败: %v", err)
	}
	if options.Env[nexusctlUserIDEnvName] != "" ||
		options.Env[nexusRuntimeUserIDEnvName] != "user-a" ||
		options.Env[nexusRuntimeScopeModeEnvName] != "user_scoped" ||
		options.Env[protocol.NexusConfigCapabilityTokenEnvName] != "trusted" {
		t.Fatalf("ExtraEnv 覆盖了 runtime 身份作用域: %+v", options.Env)
	}
}

func TestBuildAgentClientOptionsBypassKeepsPermissionHandler(t *testing.T) {
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

func clearAmbientNXSProcessRuntimeEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		runtimectx.AgentSDKDiagnosticsJSONLEnvName,
		runtimectx.AgentSDKDiagnosticsStreamProgressEnvName,
		runtimectx.AgentSDKProviderDebugBodyEnvName,
		nexusCachedMicrocompactEnvName,
		nexusOpenAIPromptCacheEnvName,
		nexusOpenAIPromptCacheModeEnvName,
		nexusOpenAIPromptCacheTTLEnvName,
		nexusOpenAIPromptCacheRetentionEnvName,
		nexusUsePowerShellToolEnvName,
	} {
		t.Setenv(key, "")
	}
}
