package clientopts

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
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
