package clientopts

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-go/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-go/permission"
)

const nexusctlUserIDEnvName = "NEXUSCTL_USER_ID"
const nexusRuntimeScopeModeEnvName = "NEXUS_RUNTIME_SCOPE_MODE"
const nexusRuntimeUserIDEnvName = "NEXUS_RUNTIME_USER_ID"
const askUserQuestionToolName = "AskUserQuestion"

// RuntimeConfigResolver 负责解析 Agent 运行时环境。
type RuntimeConfigResolver interface {
	ResolveRuntimeConfig(context.Context, string) (*RuntimeConfig, error)
}

// AgentClientOptionsInput 表示构造 SDK options 所需的统一输入。
type AgentClientOptionsInput struct {
	WorkspacePath      string
	Provider           string
	PermissionMode     sdkpermission.Mode
	PermissionHandler  agentclient.PermissionHandler
	AllowedTools       []string
	DisallowedTools    []string
	SettingSources     []string
	AppendSystemPrompt string
	ResumeSessionID    string
	MaxThinkingTokens  *int
	MaxTurns           *int
	MCPServers         map[string]sdkmcp.SDKMCPServer
}

// BuildAgentClientOptions 构建统一的 SDK client options。
func BuildAgentClientOptions(
	ctx context.Context,
	resolver RuntimeConfigResolver,
	input AgentClientOptionsInput,
) (agentclient.Options, error) {
	runtimeConfig, err := resolveRuntimeConfig(ctx, resolver, input.Provider)
	if err != nil {
		return agentclient.Options{}, err
	}
	runtimeEnv := runtimeEnvFromConfig(runtimeConfig)
	runtimeEnv = mergeRuntimeEnv(runtimeEnv, buildScopedRuntimeEnv(ctx))

	permissionMode := input.PermissionMode
	if permissionMode == "" {
		permissionMode = sdkpermission.ModeDefault
	}
	permissionHandler := permissionHandlerForMode(permissionMode, input.PermissionHandler)

	options := agentclient.Options{
		Backend:                agentclient.ProcessBackend(agentclient.ProcessBackendOptions{}),
		CWD:                    strings.TrimSpace(input.WorkspacePath),
		SettingSources:         append([]string(nil), input.SettingSources...),
		IncludePartialMessages: true,
		Env:                    runtimeEnv,
		System: agentclient.SystemOptions{
			Append: input.AppendSystemPrompt,
		},
		Tools: agentclient.ToolOptions{
			Allow: append([]string(nil), input.AllowedTools...),
			Deny:  append([]string(nil), input.DisallowedTools...),
		},
		Runtime: agentclient.RuntimeOptions{
			PermissionMode: permissionMode,
		},
		Adapters: agentclient.AdapterOptions{
			PermissionHandler: permissionHandler,
		},
	}
	if runtimeConfig != nil {
		options.Model = strings.TrimSpace(runtimeConfig.Model)
	}
	if strings.TrimSpace(input.ResumeSessionID) != "" {
		options.Session.ResumeID = strings.TrimSpace(input.ResumeSessionID)
	}
	if input.MaxThinkingTokens != nil && *input.MaxThinkingTokens > 0 {
		options.Runtime.MaxThinkingTokens = *input.MaxThinkingTokens
	}
	if input.MaxTurns != nil && *input.MaxTurns > 0 {
		options.Runtime.MaxTurns = *input.MaxTurns
	}
	if len(input.MCPServers) > 0 {
		options.MCP.SDKServers = cloneMCPServers(input.MCPServers)
	}
	return options, nil
}

func permissionHandlerForMode(
	permissionMode sdkpermission.Mode,
	handler agentclient.PermissionHandler,
) agentclient.PermissionHandler {
	if permissionMode != sdkpermission.ModeBypassPermissions || handler == nil {
		return handler
	}
	return func(ctx context.Context, request sdkpermission.Request) (sdkpermission.Decision, error) {
		if strings.TrimSpace(request.ToolName) == askUserQuestionToolName {
			return handler(ctx, request)
		}
		return sdkpermission.Allow(clonePermissionInput(request.Input), nil), nil
	}
}

func clonePermissionInput(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	result := make(map[string]any, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

// BuildRuntimeEnv 统一把 provider 配置收口成 Claude SDK 所需环境变量。
func BuildRuntimeEnv(
	ctx context.Context,
	resolver RuntimeConfigResolver,
	provider string,
) (map[string]string, error) {
	runtimeConfig, err := resolveRuntimeConfig(ctx, resolver, provider)
	if err != nil {
		return nil, err
	}
	if runtimeConfig == nil {
		return nil, nil
	}
	return runtimeEnvFromConfig(runtimeConfig), nil
}

func runtimeEnvFromConfig(runtimeConfig *RuntimeConfig) map[string]string {
	if runtimeConfig == nil {
		return nil
	}
	env := map[string]string{
		"ANTHROPIC_AUTH_TOKEN":           runtimeConfig.AuthToken,
		"ANTHROPIC_BASE_URL":             runtimeConfig.BaseURL,
		"ANTHROPIC_MODEL":                runtimeConfig.Model,
		"ANTHROPIC_DEFAULT_OPUS_MODEL":   runtimeConfig.Model,
		"ANTHROPIC_DEFAULT_SONNET_MODEL": runtimeConfig.Model,
		"ANTHROPIC_DEFAULT_HAIKU_MODEL":  runtimeConfig.Model,
		"CLAUDE_CODE_SUBAGENT_MODEL":     runtimeConfig.Model,
	}
	if strings.Contains(strings.ToLower(runtimeConfig.Model), "kimi") {
		env["ENABLE_TOOL_SEARCH"] = "false"
	}
	return env
}

func resolveRuntimeConfig(
	ctx context.Context,
	resolver RuntimeConfigResolver,
	provider string,
) (*RuntimeConfig, error) {
	if resolver == nil {
		return nil, nil
	}
	return resolver.ResolveRuntimeConfig(ctx, strings.TrimSpace(provider))
}

func cloneMCPServers(
	current map[string]sdkmcp.SDKMCPServer,
) map[string]sdkmcp.SDKMCPServer {
	if len(current) == 0 {
		return nil
	}
	result := make(map[string]sdkmcp.SDKMCPServer, len(current))
	for key, value := range current {
		result[key] = value
	}
	return result
}

func buildScopedRuntimeEnv(ctx context.Context) map[string]string {
	state, hasState := authctx.StateFromContext(ctx)
	userID, ok := authctx.CurrentUserID(ctx)
	env := map[string]string{}
	if ok {
		trimmedUserID := strings.TrimSpace(userID)
		if trimmedUserID != "" {
			env[nexusctlUserIDEnvName] = trimmedUserID
			env[nexusRuntimeUserIDEnvName] = trimmedUserID
			env[nexusRuntimeScopeModeEnvName] = "user_scoped"
		}
	}
	if len(env) > 0 {
		return env
	}
	if hasState && !state.AuthRequired {
		return map[string]string{
			nexusRuntimeScopeModeEnvName: "single_user",
			nexusRuntimeUserIDEnvName:    authctx.SystemUserID,
		}
	}
	return nil
}

func mergeRuntimeEnv(
	base map[string]string,
	extra map[string]string,
) map[string]string {
	if len(base) == 0 && len(extra) == 0 {
		return nil
	}
	result := make(map[string]string, len(base)+len(extra))
	for key, value := range base {
		result[key] = value
	}
	for key, value := range extra {
		result[key] = value
	}
	return result
}
