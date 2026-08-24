package clientopts

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"

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

func TestBuildAgentClientOptionsWithConfigReturnsSingleResolvedModelCard(t *testing.T) {
	resolveCalls := 0
	configured := &RuntimeConfig{
		Provider:      "glm",
		Model:         "glm-5.2",
		ContextWindow: 128_000,
	}
	options, resolved, err := BuildAgentClientOptionsWithConfig(
		context.Background(),
		fakeRuntimeConfigResolver{config: configured, calls: &resolveCalls},
		AgentClientOptionsInput{},
	)
	if err != nil {
		t.Fatalf("BuildAgentClientOptionsWithConfig 失败: %v", err)
	}
	if resolveCalls != 1 {
		t.Fatalf("模型配置应只解析一次: got=%d want=1", resolveCalls)
	}
	if resolved != configured || resolved.ContextWindow != 128_000 {
		t.Fatalf("未返回同一次解析的模型卡: %+v", resolved)
	}
	if options.Model != "glm-5.2" {
		t.Fatalf("SDK options 未使用已解析模型: %+v", options)
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

func TestToolUseSummaryRuntimeEnvUsesSameProviderBackgroundModel(t *testing.T) {
	mainConfig := &RuntimeConfig{
		Provider:  "glm",
		Model:     "glm-main",
		APIFormat: apiFormatAnthropicMessages,
	}
	env := toolUseSummaryRuntimeEnv(
		context.Background(),
		fakeRuntimeConfigResolver{config: &RuntimeConfig{
			Provider:  "glm",
			Model:     "glm-air",
			APIFormat: apiFormatAnthropicMessages,
		}},
		AgentClientOptionsInput{
			Provider:           "glm",
			Model:              "glm-main",
			BackgroundProvider: "glm",
			BackgroundModel:    "glm-air",
		},
		mainConfig,
		runtimeKindNXS,
	)
	if env[nexusBackgroundModelEnvName] != "glm-air" ||
		env[claudeEmitToolUseSummariesEnvName] != "1" {
		t.Fatalf("nxs ToolUseSummary 后台模型环境不正确: %+v", env)
	}
}

func TestToolUseSummaryRuntimeEnvFallsBackWithoutBlockingRuntime(t *testing.T) {
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
			env := toolUseSummaryRuntimeEnv(
				context.Background(),
				test.resolver,
				test.input,
				mainConfig,
				runtimeKindNXS,
			)
			if env[nexusBackgroundModelEnvName] != "main-model" ||
				env[claudeEmitToolUseSummariesEnvName] != "1" {
				t.Fatalf("ToolUseSummary 应回退主模型: %+v", env)
			}
		})
	}
}

func TestToolUseSummaryRuntimeEnvProjectsClaudeSmallFastModel(t *testing.T) {
	mainConfig := &RuntimeConfig{
		Provider:  "glm",
		Model:     "glm-main",
		APIFormat: apiFormatAnthropicMessages,
	}
	env := toolUseSummaryRuntimeEnv(
		context.Background(),
		fakeRuntimeConfigResolver{config: &RuntimeConfig{
			Provider:  "glm",
			Model:     "glm-air",
			APIFormat: apiFormatAnthropicMessages,
		}},
		AgentClientOptionsInput{
			Provider:           "glm",
			Model:              "glm-main",
			BackgroundProvider: "glm",
			BackgroundModel:    "glm-air",
		},
		mainConfig,
		runtimeKindClaude,
	)
	if env[anthropicSmallFastModelEnvName] != "glm-air" ||
		env[claudeEmitToolUseSummariesEnvName] != "1" {
		t.Fatalf("Claude ToolUseSummary 后台模型环境不正确: %+v", env)
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

func TestBuildAgentClientOptionsKeepsClaudeSkillDiscoveryDynamic(t *testing.T) {
	options, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		RuntimeKind:      runtimeKindClaude,
		SkillIDs:         []string{"ima-skill"},
		DisabledSkillIDs: []string{"unused-global", "workspace-review"},
		SkillDirectories: []string{"/tmp/platform-skills"},
		AdditionalDirectories: []string{
			"/tmp/current-project",
			"/tmp/platform-skills",
		},
	})
	if err != nil {
		t.Fatalf("构建带平台 Skill 的 options 失败: %v", err)
	}
	if options.Skills.Mode != agentclient.SkillModeAll {
		t.Fatalf("Claude Skill 应保持动态发现模式: %#v", options.Skills)
	}
	if len(options.Skills.DisabledNames) != 2 ||
		options.Skills.DisabledNames[0] != "unused-global" ||
		options.Skills.DisabledNames[1] != "workspace-review" {
		t.Fatalf("Claude Skill 停用集合未投影: %#v", options.Skills)
	}
	wantDirectories := []string{"/tmp/platform-skills", "/tmp/current-project"}
	if !slices.Equal(options.AdditionalDirectories, wantDirectories) {
		t.Fatalf("Skill 与 Session 目录未合并: %#v", options.AdditionalDirectories)
	}
}

func TestBuildAgentClientOptionsProjectsCompactionConfigByRuntime(t *testing.T) {
	config := &RuntimeConfig{
		Provider:        "glm",
		BaseURL:         "https://provider.example.com",
		Model:           "glm-5.2",
		ContextWindow:   300_000,
		MaxOutputTokens: 96_000,
	}

	nxsOptions, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{config: config}, AgentClientOptionsInput{
		RuntimeKind: runtimeKindNXS,
	})
	if err != nil {
		t.Fatalf("构建 nxs options 失败: %v", err)
	}
	if nxsOptions.Env[nexusAutoCompactPctOverrideEnvName] != defaultAutoCompactPctOverride ||
		nxsOptions.Env[nexusMaxContextTokensEnvName] != "300000" ||
		nxsOptions.Env[nexusMaxOutputTokensEnvName] != "96000" {
		t.Fatalf("nxs 压缩配置未按原生环境变量投影: %+v", nxsOptions.Env)
	}
	for _, key := range []string{claudeAutoCompactPctOverrideEnvName, claudeAutoCompactWindowEnvName} {
		if _, ok := nxsOptions.Env[key]; ok {
			t.Fatalf("nxs 不应接收 Claude Code 环境变量 %s: %+v", key, nxsOptions.Env)
		}
	}

	claudeOptions, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{config: config}, AgentClientOptionsInput{
		RuntimeKind: runtimeKindClaude,
	})
	if err != nil {
		t.Fatalf("构建 Claude options 失败: %v", err)
	}
	if claudeOptions.Env[claudeAutoCompactPctOverrideEnvName] != defaultAutoCompactPctOverride ||
		claudeOptions.Env[claudeAutoCompactWindowEnvName] != "300000" {
		t.Fatalf("Claude Code 压缩配置未按原生环境变量投影: %+v", claudeOptions.Env)
	}
	for _, key := range []string{nexusAutoCompactPctOverrideEnvName, nexusMaxContextTokensEnvName, nexusMaxOutputTokensEnvName} {
		if _, ok := claudeOptions.Env[key]; ok {
			t.Fatalf("Claude Code 不应接收 nxs 环境变量 %s: %+v", key, claudeOptions.Env)
		}
	}
}

func TestBuildAgentClientOptionsAllowsExtraEnvOverride(t *testing.T) {
	options, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		WorkspacePath: "/tmp/workspace",
		ExtraEnv: map[string]string{
			nexusAutoCompactPctOverrideEnvName:     "80",
			nexusDisableProjectInstructionsEnvName: "0",
			enableToolSearchEnvName:                "true",
			"NEXUS_API_CLEAR_TOOL_RESULTS":         "",
		},
	})
	if err != nil {
		t.Fatalf("BuildAgentClientOptions 失败: %v", err)
	}
	if options.Env[nexusAutoCompactPctOverrideEnvName] != "80" {
		t.Fatalf("ExtraEnv 应覆盖默认自动压缩阈值: %+v", options.Env)
	}
	if options.Env[nexusDisableProjectInstructionsEnvName] != "0" {
		t.Fatalf("ExtraEnv 应允许覆盖项目指令加载开关: %+v", options.Env)
	}
	if options.Env[enableToolSearchEnvName] != "true" {
		t.Fatalf("ExtraEnv 应允许显式覆盖 tool search 开关: %+v", options.Env)
	}
	if value, exists := options.Env["NEXUS_API_CLEAR_TOOL_RESULTS"]; !exists || value != "" {
		t.Fatalf("ExtraEnv 应保留显式空值: %+v", options.Env)
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
	clearAmbientNXSProcessRuntimeEnv(t)
	t.Setenv(nexusNXSCommandPathEnvName, "")
	t.Setenv(runtimectx.AgentSDKDiagnosticsEnvName, "stderr")
	t.Setenv(runtimectx.AgentSDKDiagnosticsJSONLEnvName, "1")
	t.Setenv(runtimectx.AgentSDKDiagnosticsStreamProgressEnvName, "0")
	t.Setenv(runtimectx.AgentSDKDebugEnvName, "1")
	t.Setenv(runtimectx.AgentSDKProviderDebugBodyEnvName, "full")
	t.Setenv(nexusCachedMicrocompactEnvName, "1")
	t.Setenv(nexusUsePowerShellToolEnvName, "1")

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
	if options.Env[runtimectx.AgentSDKDiagnosticsJSONLEnvName] != "1" {
		t.Fatalf("显式 JSONL diagnostics env 应透传给 nxs: %+v", options.Env)
	}
	if options.Env[runtimectx.AgentSDKProviderDebugBodyEnvName] != "full" {
		t.Fatalf("显式 provider body env 应透传给 nxs: %+v", options.Env)
	}
	if options.Env[runtimectx.AgentSDKDiagnosticsStreamProgressEnvName] != "0" ||
		options.Env[nexusCachedMicrocompactEnvName] != "1" ||
		options.Env[nexusUsePowerShellToolEnvName] != "1" {
		t.Fatalf("显式 nxs runtime env 未透传: %+v", options.Env)
	}
	for _, key := range []string{
		"NEXUS_API_CLEAR_TOOL_RESULTS",
		"NEXUS_API_CLEAR_TOOL_USES",
		"NEXUS_PROMPT_CACHE_1H_ELIGIBLE",
		"NEXUS_PROMPT_CACHE_1H_ALLOWLIST",
		runtimectx.AgentSDKDiagnosticsEnvName,
		runtimectx.AgentSDKDebugEnvName,
	} {
		if _, ok := options.Env[key]; ok {
			t.Fatalf("%s 应由 bridge 处理或显式输入，不应由 Nexus 默认注入: %+v", key, options.Env)
		}
	}
}

// TestBuildAgentClientOptionsForwardsOpenAIResponsesPromptCacheControls 验证宿主显式缓存策略进入 nxs。
func TestBuildAgentClientOptionsForwardsOpenAIResponsesPromptCacheControls(t *testing.T) {
	clearAmbientNXSProcessRuntimeEnv(t)
	t.Setenv(nexusOpenAIPromptCacheEnvName, "1")
	t.Setenv(nexusOpenAIPromptCacheModeEnvName, "explicit")
	t.Setenv(nexusOpenAIPromptCacheTTLEnvName, "30m")

	options, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		RuntimeKind: runtimeKindNXS,
	})
	if err != nil {
		t.Fatalf("BuildAgentClientOptions 失败: %v", err)
	}
	for key, want := range map[string]string{
		nexusOpenAIPromptCacheEnvName:     "1",
		nexusOpenAIPromptCacheModeEnvName: "explicit",
		nexusOpenAIPromptCacheTTLEnvName:  "30m",
	} {
		if options.Env[key] != want {
			t.Fatalf("%s = %q, want %q; env=%+v", key, options.Env[key], want, options.Env)
		}
	}
}

func TestBuildAgentClientOptionsEnablesNXSAgentSDKDiagnostics(t *testing.T) {
	clearAmbientNXSProcessRuntimeEnv(t)
	options, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		RuntimeKind:                runtimeKindNXS,
		AgentSDKDiagnosticsEnabled: true,
	})
	if err != nil {
		t.Fatalf("BuildAgentClientOptions 失败: %v", err)
	}
	if options.Env[runtimectx.AgentSDKDiagnosticsJSONLEnvName] != "1" {
		t.Fatalf("%s = %q, want 1; env=%+v",
			runtimectx.AgentSDKDiagnosticsJSONLEnvName,
			options.Env[runtimectx.AgentSDKDiagnosticsJSONLEnvName],
			options.Env)
	}
	if _, ok := options.Env[runtimectx.AgentSDKDiagnosticsEnvName]; ok {
		t.Fatalf("宿主默认不应继续注入旧 stderr diagnostics env: %+v", options.Env)
	}
	if options.Env[runtimectx.AgentSDKDiagnosticsStreamProgressEnvName] != "0" {
		t.Fatalf("%s = %q, want 0; env=%+v",
			runtimectx.AgentSDKDiagnosticsStreamProgressEnvName,
			options.Env[runtimectx.AgentSDKDiagnosticsStreamProgressEnvName],
			options.Env)
	}
	if _, ok := options.Env[runtimectx.AgentSDKProviderDebugBodyEnvName]; ok {
		t.Fatalf("开启 diagnostics 不应强制请求体 dump 范围: %+v", options.Env)
	}
	if !runtimectx.AgentSDKDiagnosticsEnabled(options.Env) {
		t.Fatalf("JSONL diagnostics env 应被运行时摘要识别为已开启: %+v", options.Env)
	}
}

func TestBuildAgentClientOptionsDefaultsToNXSChatCompletionsProviderEnv(t *testing.T) {
	clearInheritedAnthropicProviderEnv(t)
	resolver := &fakeRuntimeConfigForRuntimeResolver{
		config: &RuntimeConfig{
			Provider:      "openai",
			AuthToken:     "openai-token",
			BaseURL:       "https://api.openai.com/v1",
			Model:         "gpt-4o",
			APIFormat:     apiFormatChatCompletions,
			ContextWindow: 128000,
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
		"NEXUS_SUBAGENT_MODEL":       "gpt-4o",
		NexusRuntimeProviderEnvName:  "openai",
		nexusAPIProviderEnvName:      "openai",
		nexusOpenAIProtocolEnvName:   apiFormatChatCompletions,
		nexusMaxContextTokensEnvName: "128000",
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

func TestBuildAgentClientOptionsProjectsHostManagedEnvironment(t *testing.T) {
	tests := []struct {
		name                    string
		runtimeKind             string
		wantAutoDreamWakeMode   string
		wantProviderManagedFlag string
	}{
		{
			name:                    "nxs",
			runtimeKind:             runtimeKindNXS,
			wantAutoDreamWakeMode:   "host",
			wantProviderManagedFlag: "1",
		},
		{
			name:        "claude",
			runtimeKind: runtimeKindClaude,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options, err := BuildAgentClientOptions(
				context.Background(),
				fakeRuntimeConfigResolver{},
				AgentClientOptionsInput{
					RuntimeKind:   test.runtimeKind,
					WorkspacePath: t.TempDir(),
				},
			)
			if err != nil {
				t.Fatalf("BuildAgentClientOptions() error = %v", err)
			}
			if got := options.Env[nexusAutoDreamWakeModeEnvName]; got != test.wantAutoDreamWakeMode {
				t.Fatalf("%s = %q, want %q", nexusAutoDreamWakeModeEnvName, got, test.wantAutoDreamWakeMode)
			}
			if got := options.Env[nexusProviderManagedByHostEnvName]; got != test.wantProviderManagedFlag {
				t.Fatalf("%s = %q, want %q", nexusProviderManagedByHostEnvName, got, test.wantProviderManagedFlag)
			}
		})
	}
}

func TestBuildAgentClientOptionsInjectsWebSearchConfigForNXS(t *testing.T) {
	webSearch := WebSearchConfig{
		Enabled:         true,
		Provider:        "brave",
		BaseURL:         "https://search.example.com",
		DefaultCount:    7,
		TimeoutSeconds:  30,
		CacheTTLSeconds: 60,
		Country:         "CN",
	}.WithAPIKey("search-key")
	options, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		WebSearch: webSearch,
	})
	if err != nil {
		t.Fatalf("BuildAgentClientOptions 失败: %v", err)
	}
	if options.Env["NEXUS_WEBSEARCH_API_KEY"] != "search-key" {
		t.Fatalf("API key 未投影: %+v", options.Env)
	}
	var config map[string]any
	if err := json.Unmarshal([]byte(options.Env["NEXUS_WEBSEARCH_CONFIG"]), &config); err != nil {
		t.Fatalf("解析 WebSearch 配置失败: %v", err)
	}
	if config["enabled"] != true || config["provider"] != "brave" || config["base_url"] != "https://search.example.com" || config["default_count"] != float64(7) || config["timeout_seconds"] != float64(30) || config["cache_ttl_seconds"] != float64(60) || config["country"] != "CN" {
		t.Fatalf("Nexus 未完整投影 WebSearch 配置: %#v", config)
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

func TestBuildAgentClientOptionsDisablesClaudeKernelScheduler(t *testing.T) {
	options, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		RuntimeKind: runtimeKindClaude,
		ExtraEnv: map[string]string{
			claudeDisableCronEnvName: "0",
		},
	})
	if err != nil {
		t.Fatalf("BuildAgentClientOptions 失败: %v", err)
	}
	if options.Env[claudeDisableCronEnvName] != "1" {
		t.Fatalf("%s = %q, want 1; env=%+v", claudeDisableCronEnvName, options.Env[claudeDisableCronEnvName], options.Env)
	}
}

func TestBuildAgentClientOptionsNeverInjectsRawNexusCLI(t *testing.T) {
	configDir := filepath.Join(t.TempDir(), ".nexus")
	t.Setenv("NEXUS_CONFIG_DIR", configDir)
	t.Setenv("NEXUS_STATE_ROOT", "")
	t.Setenv(nexusctlCommandPathEnvName, "/opt/nexus/bin/nexusctl")
	t.Setenv(nexuscfgCommandPathEnvName, "/opt/nexus/bin/nexuscfg")
	t.Setenv(protocol.NexusCommandPathEnvName, "/opt/nexus/bin/nexus")
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
		protocol.NexusCommandPathEnvName,
		protocol.NexusCommandBrokerURLEnvName,
		protocol.NexusCommandCapabilityTokenEnvName,
		protocol.NexusCommandInputPathEnvName,
	} {
		if value := strings.TrimSpace(options.Env[key]); value != "" {
			t.Fatalf("Agent runtime 泄漏原始 CLI 环境 %s=%q: %+v", key, value, options.Env)
		}
	}
}

func TestBuildAgentClientOptionsExposesRoundScopedNexusRuntimeCLI(t *testing.T) {
	t.Setenv(protocol.NexusCommandPathEnvName, "/opt/nexus/bin/nexus")
	options, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		OwnerUserID:   "owner-a",
		WorkspacePath: "/tmp/ordinary-agent",
		RuntimeCommandEnv: map[string]string{
			protocol.NexusCommandBrokerURLEnvName:       "http://127.0.0.1:8010/nexus/v1/internal/runtime/automation",
			protocol.NexusCommandCapabilityTokenEnvName: "automation-token",
			protocol.NexusCommandInputPathEnvName:       "/private/round/input.json",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if options.Env[protocol.NexusCommandPathEnvName] != "/opt/nexus/bin/nexus" ||
		options.Env[protocol.NexusCommandCapabilityTokenEnvName] != "automation-token" ||
		options.Env[protocol.NexusCommandInputPathEnvName] != "/private/round/input.json" {
		t.Fatalf("ordinary Agent did not receive round-scoped nexus CLI: %+v", options.Env)
	}
	if options.Env[nexusctlCommandPathEnvName] != "" || options.Env[nexusctlUserIDEnvName] != "" {
		t.Fatalf("runtime nexus CLI leaked nexusctl owner authority: %+v", options.Env)
	}
	if !slices.Contains(options.AdditionalDirectories, "/private/round") {
		t.Fatalf("runtime command input directory was not granted to this round: %+v", options.AdditionalDirectories)
	}
}

func TestBuildAgentClientOptionsRejectsInvalidRuntimeCommandInputBoundary(t *testing.T) {
	for _, inputPath := range []string{"", "relative/input.json", "/input.json"} {
		_, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
			RuntimeCommandEnv: map[string]string{
				protocol.NexusCommandBrokerURLEnvName:       "http://127.0.0.1:8010/nexus/v1/internal/runtime/command",
				protocol.NexusCommandCapabilityTokenEnvName: "round-token",
				protocol.NexusCommandInputPathEnvName:       inputPath,
			},
		})
		if err == nil {
			t.Fatalf("runtime command input path %q should fail closed", inputPath)
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
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")
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
		OwnerUserID:   "user-123",
	})
	if err != nil {
		t.Fatalf("BuildAgentClientOptions 失败: %v", err)
	}
	if options.Env[nexusctlUserIDEnvName] != "" {
		t.Fatalf("多用户 runtime 不应获得 nexusctl user scope: %+v", options.Env)
	}
	if options.Env[nexusRuntimeUserIDEnvName] != "user-123" {
		t.Fatalf("未把通用运行时 user_id 注入环境: %+v", options.Env)
	}
	if options.Env[nexusRuntimeScopeModeEnvName] != "user_scoped" {
		t.Fatalf("未把多用户作用域模式注入环境: %+v", options.Env)
	}
	expectedRuntimeRoot := filepath.Join(stateRoot, "users", "user-123", "runtime")
	if options.Env[nexusConfigDirEnvName] != expectedRuntimeRoot ||
		options.Env[claudeConfigDirEnvName] != expectedRuntimeRoot {
		t.Fatalf("nxs 与 Claude 未使用统一用户 runtime 根: %+v", options.Env)
	}
	if options.Env["HOME"] != filepath.Join(expectedRuntimeRoot, "home") ||
		options.Env["USERPROFILE"] != filepath.Join(expectedRuntimeRoot, "home") ||
		options.Env["XDG_CACHE_HOME"] != filepath.Join(expectedRuntimeRoot, "cache") ||
		options.Env["TMPDIR"] != filepath.Join(expectedRuntimeRoot, "tmp") ||
		options.Env["TEMP"] != filepath.Join(expectedRuntimeRoot, "tmp") ||
		options.Env["TMP"] != filepath.Join(expectedRuntimeRoot, "tmp") {
		t.Fatalf("用户 runtime 环境目录不完整: %+v", options.Env)
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

func TestBuildAgentClientOptionsInjectsSingleUserScopeEnv(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")
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
	expectedRuntimeRoot := filepath.Join(stateRoot, "users", authctx.SystemUserID, "runtime")
	if options.Env[nexusConfigDirEnvName] != expectedRuntimeRoot ||
		options.Env[claudeConfigDirEnvName] != expectedRuntimeRoot {
		t.Fatalf("App 单用户未使用 system 用户 runtime 根: %+v", options.Env)
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
