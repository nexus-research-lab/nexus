// INPUT: Agent runtime、主/后台模型、权限/工具/Skill、round capability、内建 MCP 与持久化 MCP 配置。
// OUTPUT: 经统一校验、最小输入目录授权、后台进度模型环境投影与 MCP 名称隔离后的 SDK client options。
// POS: Agent 数据库配置进入 DM/Room runtime 前的统一启动选项装配边界。
package clientopts

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimepermission "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	"github.com/nexus-research-lab/nexus/internal/runtime/workspaceisolation"
)

var agentSessionDeniedTools = []string{
	"EnterPlanMode",
	"ScheduleWakeup",
	"CronCreate",
	"CronList",
	"CronDelete",
}

// claudeSessionAvailableTools 与 Nexus 当前 nxs 会话的默认模型可见工具保持一致。
var claudeSessionAvailableTools = []string{
	"Agent",
	"AskUserQuestion",
	"Bash",
	"Edit",
	"ExitPlanMode",
	"Read",
	"Skill",
	"TaskCreate",
	"TaskGet",
	"TaskList",
	"TaskOutput",
	"TaskStop",
	"TaskUpdate",
	"WebFetch",
	"WebSearch",
	"Write",
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
	WorkspacePath string
	OwnerUserID   string
	// IsMainAgent 表示当前 runtime 是否属于 Nexus 主智能体。
	// 只有该宿主事实可启用 owner-scoped nexusctl；nexuscfg 由独立 round capability 授权。
	IsMainAgent        bool
	RuntimeKind        string
	Provider           string
	Model              string
	BackgroundProvider string
	BackgroundModel    string
	VisionProvider     string
	VisionModel        string
	PermissionMode     sdkpermission.Mode
	PermissionHandler  sdkpermission.Handler
	AllowedTools       []string
	DisallowedTools    []string
	// SkillIDs 是宿主保存的 Skill 引用，进入 SDK 前投影为当前 runtime 的 Skill 名称白名单。
	SkillIDs []string
	// DisabledSkillIDs 是当前 Agent 明确停用或未绑定的 Skill 名称。
	//
	// 项目级 Skill 允许动态发现，不能仅靠启动时白名单表达显式停用状态。
	DisabledSkillIDs []string
	// SkillDirectories 是宿主授予 runtime 的平台与用户级资源根，不随 Agent workspace 变化。
	SkillDirectories []string
	// AdditionalDirectories 是用户为当前 Session 显式挂载的本机工作目录。
	AdditionalDirectories     []string
	SettingSources            []string
	AppendSystemPrompt        string
	AppendSystemPromptStatic  string
	AppendSystemPromptDynamic string
	ResumeSessionID           string
	MaxThinkingTokens         *int
	MaxTurns                  *int
	MCPServers                map[string]sdkmcp.ServerConfig
	AgentMCPServers           map[string]any
	ExtraEnv                  map[string]string
	// ConfigurationEnv 只接受宿主按当前 runtime round 签发的 nexuscfg broker capability。
	ConfigurationEnv map[string]string
	// RuntimeCommandEnv 只接受宿主按当前 runtime round 签发的 Agent-facing nexus CLI capability。
	RuntimeCommandEnv          map[string]string
	AgentSDKDiagnosticsEnabled bool
	ToolSearchEnabled          bool
	WebSearch                  WebSearchConfig
	RuntimeIsolationMode       string
	RuntimeLauncherPath        string
}

// BuildAgentClientOptions 构建统一的 SDK client options。
func BuildAgentClientOptions(
	ctx context.Context,
	resolver RuntimeConfigResolver,
	input AgentClientOptionsInput,
) (agentclient.Options, error) {
	options, _, err := BuildAgentClientOptionsWithConfig(ctx, resolver, input)
	return options, err
}

// BuildAgentClientOptionsWithConfig 构建 SDK options，并返回同一次解析得到的模型配置。
// 调用方需要模型窗口等宿主侧元数据时应使用此入口，避免重复解析 Provider。
func BuildAgentClientOptionsWithConfig(
	ctx context.Context,
	resolver RuntimeConfigResolver,
	input AgentClientOptionsInput,
) (agentclient.Options, *RuntimeConfig, error) {
	ownerUserID := strings.TrimSpace(input.OwnerUserID)
	if contextOwner, ok := authctx.CurrentUserID(ctx); ok &&
		ownerUserID != "" && ownerUserID != strings.TrimSpace(contextOwner) {
		return agentclient.Options{}, nil, errors.New("runtime owner 与认证上下文不一致")
	}
	if ownerUserID == "" {
		// 老的后台调用方可能只把 owner 放在认证上下文里；统一解析后，
		// 配置目录、环境变量和 workspace policy 必须使用同一个 owner。
		ownerUserID = authctx.OwnerUserID(ctx)
	}
	mcpServers, err := MergeAgentMCPServers(input.MCPServers, input.AgentMCPServers)
	if err != nil {
		return agentclient.Options{}, nil, err
	}
	effectiveRuntimeKind := resolveRuntimeKind(input.RuntimeKind, os.Getenv)
	runtimeConfig, err := resolveRuntimeConfig(ctx, resolver, input.Provider, input.Model, effectiveRuntimeKind)
	if err != nil {
		return agentclient.Options{}, nil, err
	}
	runtimeEnv := defaultRuntimeEnv(effectiveRuntimeKind)
	// bridge 会继承宿主进程环境；先清掉全局路径和密钥，再由后续
	// provider/runtime 投影显式恢复当前会话允许使用的变量。
	runtimeEnv = mergeRuntimeEnv(runtimeEnv, scrubInheritedRuntimeEnv())
	runtimeEnv = mergeRuntimeEnv(runtimeEnv, nxsHostManagedRuntimeEnv(effectiveRuntimeKind))
	runtimeEnv = mergeRuntimeEnv(runtimeEnv, nxsDiagnosticsRuntimeEnv(effectiveRuntimeKind, input.AgentSDKDiagnosticsEnabled))
	runtimeEnv = mergeRuntimeEnv(runtimeEnv, explicitNXSProcessRuntimeEnv(effectiveRuntimeKind))
	runtimeEnv = mergeRuntimeEnv(runtimeEnv, runtimeEnvFromConfig(runtimeConfig, effectiveRuntimeKind))
	runtimeEnv = mergeRuntimeEnv(runtimeEnv, toolUseSummaryRuntimeEnv(
		ctx,
		resolver,
		input,
		runtimeConfig,
		effectiveRuntimeKind,
	))
	runtimeEnv = mergeRuntimeEnv(runtimeEnv, toolSearchRuntimeEnv(effectiveRuntimeKind, input.ToolSearchEnabled))
	visionConfig, err := resolveVisionRuntimeConfig(ctx, resolver, input, effectiveRuntimeKind)
	if err != nil {
		return agentclient.Options{}, nil, err
	}
	runtimeEnv = mergeRuntimeEnv(runtimeEnv, visionRuntimeEnvFromConfig(visionConfig))
	runtimeEnv = mergeRuntimeEnv(runtimeEnv, BuildWebSearchRuntimeEnv(effectiveRuntimeKind, input.WebSearch))
	runtimeEnv = mergeRuntimeEnv(runtimeEnv, input.ExtraEnv)
	// 身份与作用域是宿主授权事实，不能交给调用方的 ExtraEnv 覆盖。
	// 必须在所有可配置环境合并后再次写入，确保 session metadata 和下游
	// hook 始终绑定同一个 owner；nexusctl 仍只对主智能体开放，nexuscfg 则
	// 只有拿到宿主 round capability 的 runtime 才恢复命令入口。
	runtimeEnv = mergeRuntimeEnv(runtimeEnv, buildScopedRuntimeEnv(ctx, ownerUserID, input.IsMainAgent))
	runtimeEnv = mergeRuntimeEnv(
		runtimeEnv,
		managedUserRuntimeEnv(ownerUserID, input.WorkspacePath, effectiveRuntimeKind),
	)
	if input.IsMainAgent || len(input.ConfigurationEnv) > 0 || len(input.RuntimeCommandEnv) > 0 {
		runtimeEnv = mergeRuntimeEnv(
			runtimeEnv,
			workspaceRuntimeEnv(input.WorkspacePath, input.IsMainAgent),
		)
	}
	runtimeEnv = mergeRuntimeEnv(runtimeEnv, input.ConfigurationEnv)
	// 防止上层误把不完整 capability 当成可信环境。
	if len(input.ConfigurationEnv) > 0 &&
		(strings.TrimSpace(runtimeEnv[protocol.NexusConfigBrokerURLEnvName]) == "" ||
			strings.TrimSpace(runtimeEnv[protocol.NexusConfigCapabilityTokenEnvName]) == "") {
		return agentclient.Options{}, nil, errors.New("nexuscfg runtime capability 不完整")
	}
	runtimeEnv = mergeRuntimeEnv(runtimeEnv, input.RuntimeCommandEnv)
	var runtimeCommandInputDirectory string
	if len(input.RuntimeCommandEnv) > 0 {
		if strings.TrimSpace(runtimeEnv[protocol.NexusCommandBrokerURLEnvName]) == "" ||
			strings.TrimSpace(runtimeEnv[protocol.NexusCommandCapabilityTokenEnvName]) == "" {
			return agentclient.Options{}, nil, errors.New("Nexus runtime command capability 不完整")
		}
		runtimeCommandInputDirectory, err = validateRuntimeCommandInputDirectory(
			runtimeEnv[protocol.NexusCommandInputPathEnvName],
		)
		if err != nil {
			return agentclient.Options{}, nil, err
		}
	}
	// Claude 仍内置 Cron，调用方不得通过 ExtraEnv 重新开启第二套调度器。
	runtimeEnv = mergeRuntimeEnv(runtimeEnv, hostManagedScheduleRuntimeEnv(effectiveRuntimeKind))

	permissionMode := runtimepermission.NormalizeMode(input.PermissionMode)
	additionalDirectories := appendDistinctStrings(
		input.SkillDirectories,
		input.AdditionalDirectories...,
	)
	additionalDirectories = appendDistinctStrings(additionalDirectories, runtimeCommandInputDirectory)
	writeDirectories := appendDistinctStrings(input.AdditionalDirectories, runtimeCommandInputDirectory)
	options := agentclient.Options{
		CWD:                    strings.TrimSpace(input.WorkspacePath),
		SettingSources:         slices.Clone(input.SettingSources),
		AdditionalDirectories:  additionalDirectories,
		IncludePartialMessages: true,
		Env:                    runtimeEnv,
		System: agentclient.SystemOptions{
			Append:        input.AppendSystemPrompt,
			AppendStatic:  input.AppendSystemPromptStatic,
			AppendDynamic: input.AppendSystemPromptDynamic,
		},
		Tools: agentclient.ToolOptions{
			Available: runtimeAvailableTools(effectiveRuntimeKind),
			Allow:     slices.Clone(input.AllowedTools),
			Deny:      appendDistinctStrings(input.DisallowedTools, agentSessionDeniedTools...),
		},
		Runtime: agentclient.RuntimeOptions{
			Kind:                            agentRuntimeKind(effectiveRuntimeKind),
			PermissionMode:                  permissionMode,
			AllowDangerouslySkipPermissions: true,
		},
		Callbacks: agentclient.CallbackOptions{
			PermissionHandler: input.PermissionHandler,
		},
	}
	if effectiveRuntimeKind == runtimeKindClaude {
		// Claude 的项目级 Skill 会在会话期间动态发现；用 deny 规则隔离
		// 未绑定的全局 Skill，不能把启动时的白名单当成完整发现快照。
		options = options.WithAllSkills()
	} else if input.SkillIDs != nil {
		options = options.WithSkills(input.SkillIDs...)
	}
	if input.DisabledSkillIDs != nil {
		options = options.WithDisabledSkills(input.DisabledSkillIDs...)
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
	if len(mcpServers) > 0 {
		options.MCP.Servers = cloneMCPServers(mcpServers)
	}
	options, err = workspaceisolation.Apply(
		ctx,
		options,
		workspaceisolation.Config{
			Mode:         workspaceisolation.Mode(input.RuntimeIsolationMode),
			LauncherPath: input.RuntimeLauncherPath,
		},
		workspaceisolation.Input{
			OwnerUserID: ownerUserID,
			IsMainAgent: input.IsMainAgent,
			RuntimeKind: effectiveRuntimeKind,
			CWD:         input.WorkspacePath,
			ReadRoots:   additionalDirectories,
			WriteRoots:  writeDirectories,
		},
	)
	if err != nil {
		return agentclient.Options{}, nil, fmt.Errorf("装配 runtime workspace isolation: %w", err)
	}
	return options, runtimeConfig, nil
}

// toolUseSummaryRuntimeEnv 把 owner 后台模型投影给当前 bridge。工具进度是纯展示能力：
// 后台模型缺失、解析失败或属于另一 Provider 时回退主模型，不能阻断 Agent 启动。
func toolUseSummaryRuntimeEnv(
	ctx context.Context,
	resolver RuntimeConfigResolver,
	input AgentClientOptionsInput,
	mainConfig *RuntimeConfig,
	runtimeKind string,
) map[string]string {
	mainProvider := strings.TrimSpace(input.Provider)
	mainModel := strings.TrimSpace(input.Model)
	mainAPIFormat := ""
	if mainConfig != nil {
		mainProvider = firstNonEmptyRuntimeValue(mainConfig.Provider, mainProvider)
		mainModel = firstNonEmptyRuntimeValue(mainConfig.Model, mainModel)
		mainAPIFormat = normalizedRuntimeAPIFormat(mainConfig.APIFormat)
	}
	selectedModel := mainModel
	backgroundProvider := strings.TrimSpace(input.BackgroundProvider)
	backgroundModel := strings.TrimSpace(input.BackgroundModel)
	if backgroundProvider != "" && backgroundModel != "" &&
		strings.EqualFold(backgroundProvider, mainProvider) {
		backgroundConfig, err := resolveRuntimeConfig(
			ctx,
			resolver,
			backgroundProvider,
			backgroundModel,
			runtimeKind,
		)
		switch {
		case err != nil:
			// Cosmetic progress falls back to the already validated main model.
		case backgroundConfig == nil:
			if resolver == nil {
				selectedModel = backgroundModel
			}
		case normalizedRuntimeAPIFormat(backgroundConfig.APIFormat) == mainAPIFormat:
			selectedModel = firstNonEmptyRuntimeValue(backgroundConfig.Model, backgroundModel)
		}
	}
	if selectedModel == "" {
		return nil
	}
	env := map[string]string{claudeEmitToolUseSummariesEnvName: "1"}
	if runtimeProfileForKind(runtimeKind).isNXS() {
		env[nexusBackgroundModelEnvName] = selectedModel
	} else {
		env[anthropicSmallFastModelEnvName] = selectedModel
	}
	return env
}

func normalizedRuntimeAPIFormat(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return apiFormatAnthropicMessages
	}
	return value
}

func firstNonEmptyRuntimeValue(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

// validateRuntimeCommandInputDirectory narrows a host-signed input file to its
// exact per-round parent. Broad roots and relative paths fail closed before the
// capability reaches either SDK add-dir projection or workspace isolation.
func validateRuntimeCommandInputDirectory(inputPath string) (string, error) {
	inputPath = filepath.Clean(strings.TrimSpace(inputPath))
	if inputPath == "." || !filepath.IsAbs(inputPath) {
		return "", errors.New("Nexus runtime command input path 必须是绝对文件路径")
	}
	directory := filepath.Dir(inputPath)
	if directory == inputPath || filepath.Dir(directory) == directory {
		return "", errors.New("Nexus runtime command input path 不能授权文件系统根目录")
	}
	return directory, nil
}

func runtimeAvailableTools(runtimeKind string) []string {
	if runtimeKind != runtimeKindClaude {
		return nil
	}
	return slices.Clone(claudeSessionAvailableTools)
}

func cloneMCPServers(
	current map[string]sdkmcp.ServerConfig,
) map[string]sdkmcp.ServerConfig {
	if len(current) == 0 {
		return nil
	}
	return maps.Clone(current)
}

// resolveVisionRuntimeConfig 只为 nxs 解析用户明确选择的辅助视觉模型。
func resolveVisionRuntimeConfig(
	ctx context.Context,
	resolver RuntimeConfigResolver,
	input AgentClientOptionsInput,
	runtimeKind string,
) (*RuntimeConfig, error) {
	if !runtimeProfileForKind(runtimeKind).isNXS() {
		return nil, nil
	}
	providerName := strings.TrimSpace(input.VisionProvider)
	model := strings.TrimSpace(input.VisionModel)
	if providerName == "" && model == "" {
		return nil, nil
	}
	if providerName == "" || model == "" {
		return nil, errors.New("视觉模型必须同时配置 provider 和 model")
	}
	config, err := resolveRuntimeConfig(ctx, resolver, providerName, model, runtimeKind)
	if err != nil {
		return nil, fmt.Errorf("解析视觉模型: %w", err)
	}
	if config == nil || !config.Vision {
		return nil, fmt.Errorf("视觉模型 %s/%s 未声明 vision 能力", providerName, model)
	}
	return config, nil
}

func agentRuntimeKind(runtimeKind string) agentclient.RuntimeKind {
	if runtimeProfileForKind(runtimeKind).isNXS() {
		return agentclient.RuntimeNXS
	}
	return agentclient.RuntimeClaude
}

func appendDistinctStrings(base []string, extra ...string) []string {
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
		return nil, fmt.Errorf("api_format=%s 暂不可用于 %s Agent runtime", apiFormat, runtimeKind)
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
