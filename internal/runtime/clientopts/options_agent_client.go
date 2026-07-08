package clientopts

import (
	"context"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

const askUserQuestionToolName = "AskUserQuestion"

var agentSessionDeniedTools = []string{
	"EnterPlanMode",
	"ScheduleWakeup",
	"CronCreate",
	"CronList",
	"CronDelete",
}

// RuntimeConfigResolver 负责解析 Agent 运行时环境。
type RuntimeConfigResolver interface {
	ResolveRuntimeConfig(context.Context, string, string) (*RuntimeConfig, error)
}

// RuntimeConfigForRuntimeResolver 可按 Agent runtime 类型解析 Provider 配置。
type RuntimeConfigForRuntimeResolver interface {
	ResolveRuntimeConfigForRuntime(context.Context, string, string, string) (*RuntimeConfig, error)
}

// AgentClientOptionsInput 表示构造 SDK options 所需的统一输入。
type AgentClientOptionsInput struct {
	WorkspacePath              string
	RuntimeKind                string
	Provider                   string
	Model                      string
	PermissionMode             sdkpermission.Mode
	PermissionHandler          sdkpermission.Handler
	AllowedTools               []string
	DisallowedTools            []string
	SettingSources             []string
	AppendSystemPrompt         string
	ResumeSessionID            string
	MaxThinkingTokens          *int
	MaxTurns                   *int
	MCPServers                 map[string]sdkmcp.ServerConfig
	ExtraEnv                   map[string]string
	AgentSDKDiagnosticsEnabled bool
}

// BuildAgentClientOptions 构建统一的 SDK client options。
func BuildAgentClientOptions(
	ctx context.Context,
	resolver RuntimeConfigResolver,
	input AgentClientOptionsInput,
) (agentclient.Options, error) {
	effectiveRuntimeKind := resolveRuntimeKind(input.RuntimeKind, os.Getenv)
	runtimeConfig, err := resolveRuntimeConfig(ctx, resolver, input.Provider, input.Model, effectiveRuntimeKind)
	if err != nil {
		return agentclient.Options{}, err
	}
	runtimeEnv := defaultRuntimeEnv()
	runtimeEnv = mergeRuntimeEnv(runtimeEnv, nxsDiagnosticsRuntimeEnv(effectiveRuntimeKind, input.AgentSDKDiagnosticsEnabled))
	runtimeEnv = mergeRuntimeEnv(runtimeEnv, explicitNXSProcessRuntimeEnv(effectiveRuntimeKind))
	runtimeEnv = mergeRuntimeEnv(runtimeEnv, runtimeEnvFromConfig(runtimeConfig, effectiveRuntimeKind))
	runtimeEnv = mergeRuntimeEnv(runtimeEnv, workspaceRuntimeEnv(input.WorkspacePath))
	runtimeEnv = mergeRuntimeEnv(runtimeEnv, buildScopedRuntimeEnv(ctx))
	runtimeEnv = mergeRuntimeEnv(runtimeEnv, input.ExtraEnv)

	permissionMode := input.PermissionMode
	if permissionMode == "" {
		permissionMode = sdkpermission.ModeDefault
	}
	permissionHandler := permissionHandlerForMode(permissionMode, input.PermissionHandler)
	options := agentclient.Options{
		CWD:                    strings.TrimSpace(input.WorkspacePath),
		SettingSources:         slices.Clone(input.SettingSources),
		IncludePartialMessages: true,
		Env:                    runtimeEnv,
		System: agentclient.SystemOptions{
			Append: input.AppendSystemPrompt,
		},
		Tools: agentclient.ToolOptions{
			Allow: slices.Clone(input.AllowedTools),
			Deny:  appendDistinctTools(input.DisallowedTools, agentSessionDeniedTools...),
		},
		Runtime: agentclient.RuntimeOptions{
			Kind:                            agentRuntimeKind(effectiveRuntimeKind),
			PermissionMode:                  permissionMode,
			AllowDangerouslySkipPermissions: true,
		},
		Callbacks: agentclient.CallbackOptions{
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
		options.MCP.Servers = cloneMCPServers(input.MCPServers)
	}
	return options, nil
}

func agentRuntimeKind(runtimeKind string) agentclient.RuntimeKind {
	if runtimeProfileForKind(runtimeKind).isNXS() {
		return agentclient.RuntimeNXS
	}
	return agentclient.RuntimeClaude
}

func appendDistinctTools(base []string, extra ...string) []string {
	result := make([]string, 0, len(base)+len(extra))
	seen := make(map[string]struct{}, len(base)+len(extra))
	for _, tool := range slices.Concat(base, extra) {
		normalized := strings.TrimSpace(tool)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func permissionHandlerForMode(
	permissionMode sdkpermission.Mode,
	handler sdkpermission.Handler,
) sdkpermission.Handler {
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
	return maps.Clone(input)
}

func resolveRuntimeConfig(
	ctx context.Context,
	resolver RuntimeConfigResolver,
	provider string,
	model string,
	runtimeKind string,
) (*RuntimeConfig, error) {
	if resolver == nil {
		return nil, nil
	}
	runtimeConfig, err := resolveProviderRuntimeConfig(ctx, resolver, provider, model, runtimeKind)
	if err != nil {
		return nil, err
	}
	if runtimeConfig == nil {
		return nil, nil
	}
	apiFormat := strings.TrimSpace(runtimeConfig.APIFormat)
	if !runtimeSupportsAPIFormat(runtimeKind, apiFormat) {
		return nil, fmt.Errorf("api_format=%s 暂不可用于 Agent runtime", apiFormat)
	}
	return runtimeConfig, nil
}

func resolveProviderRuntimeConfig(
	ctx context.Context,
	resolver RuntimeConfigResolver,
	provider string,
	model string,
	runtimeKind string,
) (*RuntimeConfig, error) {
	provider = strings.TrimSpace(provider)
	model = strings.TrimSpace(model)
	if runtimeResolver, ok := resolver.(RuntimeConfigForRuntimeResolver); ok {
		return runtimeResolver.ResolveRuntimeConfigForRuntime(ctx, provider, model, runtimeKind)
	}
	return resolver.ResolveRuntimeConfig(ctx, provider, model)
}

func runtimeSupportsAPIFormat(runtimeKind string, apiFormat string) bool {
	profile := resolveRuntimeProfile(runtimeKind, os.Getenv)
	return profile.supportsAPIFormat(apiFormat)
}

func cloneMCPServers(
	current map[string]sdkmcp.ServerConfig,
) map[string]sdkmcp.ServerConfig {
	if len(current) == 0 {
		return nil
	}
	return maps.Clone(current)
}
