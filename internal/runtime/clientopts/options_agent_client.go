package clientopts

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	runtimeprovider "github.com/nexus-research-lab/nexus/internal/runtime/provider"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

const nexusctlUserIDEnvName = "NEXUSCTL_USER_ID"
const nexusctlWorkspacePathEnvName = "NEXUSCTL_WORKSPACE_PATH"
const apiFormatAnthropicMessages = runtimeprovider.APIFormatAnthropicMessages
const apiFormatChatCompletions = runtimeprovider.APIFormatChatCompletions
const claudeAutoCompactPctOverrideEnvName = "CLAUDE_AUTOCOMPACT_PCT_OVERRIDE"
const defaultClaudeAutoCompactPctOverride = "70"
const thinkingCapabilityName = "thinking"

// NexusRuntimeProviderEnvName 表示当前 SDK runtime 实际解析出的 provider key。
const NexusRuntimeProviderEnvName = "NEXUS_RUNTIME_PROVIDER"
const nexusRuntimeScopeModeEnvName = "NEXUS_RUNTIME_SCOPE_MODE"
const nexusRuntimeUserIDEnvName = "NEXUS_RUNTIME_USER_ID"
const askUserQuestionToolName = "AskUserQuestion"

var claudeSessionScheduleTools = []string{
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
	WorkspacePath      string
	RuntimeKind        string
	Provider           string
	Model              string
	PermissionMode     sdkpermission.Mode
	PermissionHandler  sdkpermission.Handler
	AllowedTools       []string
	DisallowedTools    []string
	SettingSources     []string
	AppendSystemPrompt string
	ResumeSessionID    string
	MaxThinkingTokens  *int
	MaxTurns           *int
	MCPServers         map[string]sdkmcp.ServerConfig
	ExtraEnv           map[string]string
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
	runtimeEnv = mergeRuntimeEnv(runtimeEnv, runtimeEnvFromConfig(runtimeConfig, effectiveRuntimeKind))
	runtimeEnv = mergeRuntimeEnv(runtimeEnv, workspaceRuntimeEnv(input.WorkspacePath))
	runtimeEnv = mergeRuntimeEnv(runtimeEnv, buildScopedRuntimeEnv(ctx))
	runtimeEnv = mergeRuntimeEnv(runtimeEnv, input.ExtraEnv)

	permissionMode := input.PermissionMode
	if permissionMode == "" {
		permissionMode = sdkpermission.ModeDefault
	}
	permissionHandler := permissionHandlerForMode(permissionMode, input.PermissionHandler)
	commandConfig := processRuntimeCommandConfig(effectiveRuntimeKind)

	options := agentclient.Options{
		CLIPath:                commandConfig.CLIPath,
		CWD:                    strings.TrimSpace(input.WorkspacePath),
		SettingSources:         append([]string(nil), input.SettingSources...),
		IncludePartialMessages: true,
		Env:                    runtimeEnv,
		Executable:             commandConfig.Executable,
		PathToExecutable:       commandConfig.PathToExecutable,
		System: agentclient.SystemOptions{
			Append: input.AppendSystemPrompt,
		},
		Tools: agentclient.ToolOptions{
			Allow: append([]string(nil), input.AllowedTools...),
			Deny:  appendDistinctTools(input.DisallowedTools, claudeSessionScheduleTools...),
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
	if err := materializeProcessArgFiles(&options); err != nil {
		return agentclient.Options{}, err
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
	for _, tool := range append(append([]string(nil), base...), extra...) {
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
	model string,
) (map[string]string, error) {
	runtimeConfig, err := resolveRuntimeConfig(ctx, resolver, provider, model, runtimeKindClaude)
	if err != nil {
		return nil, err
	}
	if runtimeConfig == nil {
		return nil, nil
	}
	return runtimeEnvFromConfig(runtimeConfig, runtimeKindClaude), nil
}

func runtimeEnvFromConfig(runtimeConfig *RuntimeConfig, runtimeKind string) map[string]string {
	if runtimeConfig == nil {
		return nil
	}
	profile := resolveRuntimeProfile(runtimeKind, os.Getenv)
	switch strings.TrimSpace(runtimeConfig.APIFormat) {
	case "", apiFormatAnthropicMessages:
		return anthropicRuntimeEnvFromConfig(runtimeConfig)
	case apiFormatChatCompletions:
		if profile.isNXS() {
			return openAIRuntimeEnvFromConfig(runtimeConfig)
		}
	}
	return nil
}

func anthropicRuntimeEnvFromConfig(runtimeConfig *RuntimeConfig) map[string]string {
	env := map[string]string{
		"ANTHROPIC_AUTH_TOKEN":           runtimeConfig.AuthToken,
		"ANTHROPIC_BASE_URL":             runtimeConfig.BaseURL,
		"ANTHROPIC_MODEL":                runtimeConfig.Model,
		"ANTHROPIC_DEFAULT_OPUS_MODEL":   runtimeConfig.Model,
		"ANTHROPIC_DEFAULT_SONNET_MODEL": runtimeConfig.Model,
		"ANTHROPIC_DEFAULT_HAIKU_MODEL":  runtimeConfig.Model,
		"CLAUDE_CODE_SUBAGENT_MODEL":     runtimeConfig.Model,
		NexusRuntimeProviderEnvName:      runtimeConfig.Provider,
		"NEXUS_API_PROVIDER":             "anthropic-compatible",
	}
	if runtimeConfig.Reasoning {
		applyDefaultModelCapabilitiesEnv(env, thinkingCapabilityName)
	}
	if strings.Contains(strings.ToLower(runtimeConfig.Model), "kimi") {
		env["ENABLE_TOOL_SEARCH"] = "false"
	}
	return env
}

func openAIRuntimeEnvFromConfig(runtimeConfig *RuntimeConfig) map[string]string {
	return map[string]string{
		"OPENAI_API_KEY":             runtimeConfig.AuthToken,
		"OPENAI_BASE_URL":            runtimeConfig.BaseURL,
		"OPENAI_MODEL":               runtimeConfig.Model,
		"CLAUDE_CODE_SUBAGENT_MODEL": runtimeConfig.Model,
		NexusRuntimeProviderEnvName:  runtimeConfig.Provider,
		"NEXUS_API_PROVIDER":         "openai",
	}
}

func applyDefaultModelCapabilitiesEnv(env map[string]string, capabilities ...string) {
	capabilityValue := strings.Join(capabilities, ",")
	for _, key := range []string{
		"ANTHROPIC_DEFAULT_OPUS_MODEL_SUPPORTED_CAPABILITIES",
		"ANTHROPIC_DEFAULT_SONNET_MODEL_SUPPORTED_CAPABILITIES",
		"ANTHROPIC_DEFAULT_HAIKU_MODEL_SUPPORTED_CAPABILITIES",
	} {
		env[key] = capabilityValue
	}
}

func defaultRuntimeEnv() map[string]string {
	return map[string]string{
		claudeAutoCompactPctOverrideEnvName: defaultClaudeAutoCompactPctOverride,
	}
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
	result := make(map[string]sdkmcp.ServerConfig, len(current))
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

func workspaceRuntimeEnv(workspacePath string) map[string]string {
	trimmedWorkspacePath := strings.TrimSpace(workspacePath)
	if trimmedWorkspacePath == "" {
		return nil
	}
	binDir := appfs.AgentRuntimeBinDir()
	env := map[string]string{
		"NEXUS_PROJECT_ROOT":         strings.TrimSpace(appfs.Root()),
		nexusctlWorkspacePathEnvName: trimmedWorkspacePath,
	}
	currentPath := strings.TrimSpace(os.Getenv("PATH"))
	if currentPath == "" {
		env["PATH"] = binDir
	} else {
		env["PATH"] = binDir + string(os.PathListSeparator) + currentPath
	}
	return env
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
