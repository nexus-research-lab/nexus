package clientopts

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

type fakeRuntimeConfigResolver struct {
	config *RuntimeConfig
	err    error
	calls  *int
}

func (r fakeRuntimeConfigResolver) ResolveRuntimeConfig(
	context.Context,
	string,
) (*RuntimeConfig, error) {
	if r.calls != nil {
		*r.calls = *r.calls + 1
	}
	return r.config, r.err
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
	if options.PermissionMode != sdkprotocol.PermissionModeDefault {
		t.Fatalf("默认权限模式不正确: %+v", options)
	}
	if options.Env["ANTHROPIC_MODEL"] != "kimi-k2" {
		t.Fatalf("运行时模型未写入 env: %+v", options.Env)
	}
	if options.Model != "kimi-k2" {
		t.Fatalf("运行时模型未写入 SDK options: %+v", options)
	}
	if options.Env["ENABLE_TOOL_SEARCH"] != "false" {
		t.Fatalf("kimi 模型应关闭 tool search: %+v", options.Env)
	}
	if options.Resume != "sdk-session-1" {
		t.Fatalf("resume session_id 不正确: %+v", options)
	}
	if options.MaxThinkingTokens != 2048 || options.MaxTurns != 8 {
		t.Fatalf("思考/轮次限制未透传: %+v", options)
	}
	if resolveCalls != 1 {
		t.Fatalf("provider runtime config 解析次数不正确: got=%d want=1", resolveCalls)
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
