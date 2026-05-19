package clientopts

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"

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
	if options.Runtime.PermissionMode != sdkpermission.ModeDefault {
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

func TestBuildAgentClientOptionsInjectsWorkspaceBinEnv(t *testing.T) {
	workspacePath := "/tmp/workspace"
	options, err := BuildAgentClientOptions(context.Background(), fakeRuntimeConfigResolver{}, AgentClientOptionsInput{
		WorkspacePath: workspacePath,
	})
	if err != nil {
		t.Fatalf("BuildAgentClientOptions 失败: %v", err)
	}
	pathItems := strings.Split(options.Env["PATH"], string(os.PathListSeparator))
	if len(pathItems) == 0 || pathItems[0] != filepath.Join(workspacePath, ".agents", "bin") {
		t.Fatalf("运行时 PATH 未优先注入 workspace bin: %q", options.Env["PATH"])
	}
	if strings.TrimSpace(options.Env["NEXUS_PROJECT_ROOT"]) == "" {
		t.Fatalf("运行时未注入 NEXUS_PROJECT_ROOT: %+v", options.Env)
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
