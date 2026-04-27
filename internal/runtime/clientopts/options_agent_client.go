package clientopts

import (
	"context"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-go/client"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-go/protocol"
)

const nexusctlUserIDEnvName = "NEXUSCTL_USER_ID"
const nexusRuntimeScopeModeEnvName = "NEXUS_RUNTIME_SCOPE_MODE"
const nexusRuntimeUserIDEnvName = "NEXUS_RUNTIME_USER_ID"

// RuntimeConfigResolver 负责解析 Agent 运行时环境。
type RuntimeConfigResolver interface {
	ResolveRuntimeConfig(context.Context, string) (*RuntimeConfig, error)
}

// AgentClientOptionsInput 表示构造 SDK options 所需的统一输入。
type AgentClientOptionsInput struct {
	WorkspacePath      string
	Provider           string
	PermissionMode     sdkprotocol.PermissionMode
	PermissionHandler  agentclient.PermissionHandler
	AllowedTools       []string
	DisallowedTools    []string
	SettingSources     []string
	AppendSystemPrompt string
	ResumeSessionID    string
	MaxThinkingTokens  *int
	MaxTurns           *int
	MCPServers         map[string]agentclient.SDKMCPServer
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
		permissionMode = sdkprotocol.PermissionModeDefault
	}
	permissionHandler := input.PermissionHandler
	if permissionMode == sdkprotocol.PermissionModeBypassPermissions {
		permissionHandler = nil
	}

	options := agentclient.Options{
		CWD:                    strings.TrimSpace(input.WorkspacePath),
		PermissionMode:         permissionMode,
		AllowedTools:           append([]string(nil), input.AllowedTools...),
		DisallowedTools:        append([]string(nil), input.DisallowedTools...),
		SettingSources:         append([]string(nil), input.SettingSources...),
		IncludePartialMessages: true,
		Env:                    runtimeEnv,
		PermissionHandler:      permissionHandler,
		AppendSystemPrompt:     input.AppendSystemPrompt,
	}
	if runtimeConfig != nil {
		options.Model = strings.TrimSpace(runtimeConfig.Model)
	}
	if strings.TrimSpace(input.ResumeSessionID) != "" {
		options.Resume = strings.TrimSpace(input.ResumeSessionID)
	}
	if input.MaxThinkingTokens != nil && *input.MaxThinkingTokens > 0 {
		options.MaxThinkingTokens = *input.MaxThinkingTokens
	}
	if input.MaxTurns != nil && *input.MaxTurns > 0 {
		options.MaxTurns = *input.MaxTurns
	}
	if len(input.MCPServers) > 0 {
		options.SDKMCPServers = cloneMCPServers(input.MCPServers)
	}
	return options, nil
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
	current map[string]agentclient.SDKMCPServer,
) map[string]agentclient.SDKMCPServer {
	if len(current) == 0 {
		return nil
	}
	result := make(map[string]agentclient.SDKMCPServer, len(current))
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
