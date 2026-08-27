package clientopts

import (
	"context"
	"maps"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	runtimeprovider "github.com/nexus-research-lab/nexus/internal/runtime/provider"
)

// RuntimeConfig 表示运行时使用的 Provider 解析结果。
type RuntimeConfig struct {
	Provider    string
	DisplayName string
	AuthToken   string
	BaseURL     string
	Model       string
	APIFormat   string
	// UseMaxCompletionTokens 让 Chat Completions 使用现代 token 上限字段。
	UseMaxCompletionTokens bool
	Reasoning              bool
	Vision                 bool
	ContextWindow          int
	MaxOutputTokens        int
}

const nexusctlUserIDEnvName = "NEXUSCTL_USER_ID"
const nexusctlWorkspacePathEnvName = "NEXUSCTL_WORKSPACE_PATH"
const nexusctlCommandPathEnvName = "NEXUSCTL_COMMAND_PATH"
const nexuscfgCommandPathEnvName = "NEXUSCFG_COMMAND_PATH"
const legacyNexusCommandPathEnvName = "NEXUS_COMMAND_PATH"
const nexusConfigDirEnvName = "NEXUS_CONFIG_DIR"
const claudeConfigDirEnvName = "CLAUDE_CONFIG_DIR"
const nexusAppRootEnvName = "NEXUS_APP_ROOT"
const workspacePathEnvName = "WORKSPACE_PATH"
const cacheFileDirEnvName = "CACHE_FILE_DIR"
const logPathEnvName = "LOG_PATH"
const databaseDriverEnvName = "DATABASE_DRIVER"
const databaseURLEnvName = "DATABASE_URL"
const connectorCredentialsKeyEnvName = "CONNECTOR_CREDENTIALS_KEY"
const connectorCredentialsKeyFileEnvName = "CONNECTOR_CREDENTIALS_KEY_FILE"
const nexusDesktopSessionTokenEnvName = "NEXUS_DESKTOP_SESSION_TOKEN"
const nexusNXSRuntimeCacheDirEnvName = "NEXUS_NXS_RUNTIME_CACHE_DIR"
const nexusClaudeCommandPathEnvName = "NEXUS_CLAUDE_COMMAND_PATH"
const nexusMemoryDirEnvName = "NEXUS_MEMORY_DIR"
const nexusEnableRemoteMemoryEnvName = "NEXUS_ENABLE_REMOTE_MEMORY"
const nexusRemoteMemoryDirEnvName = "NEXUS_REMOTE_MEMORY_DIR"
const apiFormatAnthropicMessages = runtimeprovider.APIFormatAnthropicMessages
const apiFormatChatCompletions = runtimeprovider.APIFormatChatCompletions
const apiFormatResponses = runtimeprovider.APIFormatResponses
const nexusNXSCommandPathEnvName = "NEXUS_NXS_COMMAND_PATH"
const nexusAgentRuntimeKindEnvName = "NEXUS_AGENT_RUNTIME_KIND"
const nexusAgentRuntimeEnvName = "NEXUS_AGENT_RUNTIME"
const runtimeKindClaude = runtimeprovider.RuntimeKindClaude
const runtimeKindNXS = runtimeprovider.RuntimeKindNXS
const nexusAutoCompactPctOverrideEnvName = "NEXUS_AUTOCOMPACT_PCT_OVERRIDE"
const claudeAutoCompactPctOverrideEnvName = "CLAUDE_AUTOCOMPACT_PCT_OVERRIDE"
const claudeAutoCompactWindowEnvName = "CLAUDE_CODE_AUTO_COMPACT_WINDOW"
const defaultAutoCompactPctOverride = "70"
const thinkingCapabilityName = "thinking"
const nexusAPIProviderEnvName = "NEXUS_API_PROVIDER"
const nexusOpenAIProtocolEnvName = "NEXUS_OPENAI_PROTOCOL"
const nexusOpenAIPromptCacheEnvName = "NEXUS_OPENAI_PROMPT_CACHE"
const nexusOpenAIPromptCacheModeEnvName = "NEXUS_OPENAI_PROMPT_CACHE_MODE"
const nexusOpenAIPromptCacheTTLEnvName = "NEXUS_OPENAI_PROMPT_CACHE_TTL"
const nexusOpenAIPromptCacheRetentionEnvName = "NEXUS_OPENAI_PROMPT_CACHE_RETENTION"
const anthropicBaseURLEnvName = "ANTHROPIC_BASE_URL"
const anthropicAPIKeyEnvName = "ANTHROPIC_API_KEY"
const anthropicAuthTokenEnvName = "ANTHROPIC_AUTH_TOKEN"
const anthropicModelEnvName = "ANTHROPIC_MODEL"
const anthropicSmallFastModelEnvName = "ANTHROPIC_SMALL_FAST_MODEL"
const claudeEmitToolUseSummariesEnvName = "CLAUDE_CODE_EMIT_TOOL_USE_SUMMARIES"
const nexusBackgroundModelEnvName = "NEXUS_BACKGROUND_MODEL"
const enableToolSearchEnvName = "ENABLE_TOOL_SEARCH"
const nexusEnableToolSearchEnvName = "NEXUS_ENABLE_TOOL_SEARCH"
const firstPartyAnthropicAPIHost = "api.anthropic.com"
const nexusDisableProjectInstructionsEnvName = "NEXUS_DISABLE_PROJECT_INSTRUCTIONS"
const nexusCachedMicrocompactEnvName = "NEXUS_CACHED_MICROCOMPACT"
const nexusMaxContextTokensEnvName = "NEXUS_MAX_CONTEXT_TOKENS"
const nexusMaxOutputTokensEnvName = "NEXUS_MAX_OUTPUT_TOKENS"
const nexusModelSupportsVisionEnvName = "NEXUS_MODEL_SUPPORTS_VISION"
const nexusMultimodalUserContentEnvName = "NEXUS_MULTIMODAL_USER_CONTENT"
const nexusMultimodalToolResultEnvName = "NEXUS_MULTIMODAL_TOOL_RESULT"
const nexusRemoteImageURLEnvName = "NEXUS_REMOTE_IMAGE_URL"
const nexusUsePowerShellToolEnvName = "NEXUS_USE_POWERSHELL_TOOL"

// NexusRuntimeProviderEnvName 表示当前 SDK runtime 实际解析出的 provider key。
const NexusRuntimeProviderEnvName = "NEXUS_RUNTIME_PROVIDER"
const nexusRuntimeScopeModeEnvName = "NEXUS_RUNTIME_SCOPE_MODE"
const nexusRuntimeUserIDEnvName = "NEXUS_RUNTIME_USER_ID"

const (
	nexusAutoDreamWakeModeEnvName     = "NEXUS_AUTO_DREAM_WAKE_MODE"
	nexusProviderManagedByHostEnvName = "NEXUS_PROVIDER_MANAGED_BY_HOST"
	claudeDisableCronEnvName          = "CLAUDE_CODE_DISABLE_CRON"
)

type runtimeProfile struct {
	kind string
}

func resolveRuntimeProfile(runtimeKind string, getenv func(string) string) runtimeProfile {
	return runtimeProfileForKind(resolveRuntimeKind(runtimeKind, getenv))
}

func runtimeProfileForKind(runtimeKind string) runtimeProfile {
	if runtimeKind == runtimeKindNXS {
		return runtimeProfile{kind: runtimeKindNXS}
	}
	return runtimeProfile{kind: runtimeKindClaude}
}

func (p runtimeProfile) isNXS() bool {
	return p.kind == runtimeKindNXS
}

func (p runtimeProfile) supportsAPIFormat(apiFormat string) bool {
	return runtimeprovider.SupportsAPIFormat(p.kind, apiFormat)
}

func resolveRuntimeKind(runtimeKind string, getenv func(string) string) string {
	// 调用方显式选择代表本次会话；进程环境只作为未指定时的默认值。
	if getenv == nil {
		getenv = func(string) string { return "" }
	}
	for _, value := range []string{
		runtimeKind,
		getenv(nexusAgentRuntimeKindEnvName),
		getenv(nexusAgentRuntimeEnvName),
	} {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case runtimeKindNXS, "go", "go-native", "gonative":
			return runtimeKindNXS
		case runtimeKindClaude, "claude-code", "claudecode":
			return runtimeKindClaude
		case "":
			continue
		}
	}
	return runtimeKindNXS
}

func runtimeEnvFromConfig(runtimeConfig *RuntimeConfig, runtimeKind string) map[string]string {
	if runtimeConfig == nil {
		return nil
	}
	profile := resolveRuntimeProfile(runtimeKind, os.Getenv)
	var env map[string]string
	switch strings.TrimSpace(runtimeConfig.APIFormat) {
	case "", apiFormatAnthropicMessages:
		env = anthropicRuntimeEnvFromConfig(runtimeConfig)
	case apiFormatChatCompletions, apiFormatResponses:
		if profile.isNXS() {
			env = openAIRuntimeEnvFromConfig(runtimeConfig)
			env[nexusOpenAIProtocolEnvName] = strings.TrimSpace(runtimeConfig.APIFormat)
		}
	}
	if profile.isNXS() {
		applyNXSModelMetadataEnv(env, runtimeConfig)
	} else {
		applyClaudeModelMetadataEnv(env, runtimeConfig)
	}
	return env
}

// toolSearchRuntimeEnv 把 Nexus 设置投影到 nxs 的两个兼容环境变量。
// 同时写入旧别名，避免宿主进程遗留的 ENABLE_TOOL_SEARCH 覆盖显式设置。
func toolSearchRuntimeEnv(runtimeKind string, enabled bool) map[string]string {
	if !runtimeProfileForKind(runtimeKind).isNXS() {
		return nil
	}
	value := "0"
	if enabled {
		value = "1"
	}
	return map[string]string{
		enableToolSearchEnvName:      value,
		nexusEnableToolSearchEnvName: value,
	}
}

// applyNXSModelMetadataEnv 把产品模型卡和 API format 能力交给 nxs，不让运行时按模型名猜测。
func applyNXSModelMetadataEnv(env map[string]string, runtimeConfig *RuntimeConfig) {
	if len(env) == 0 || runtimeConfig == nil {
		return
	}
	if runtimeConfig.ContextWindow > 0 {
		env[nexusMaxContextTokensEnvName] = strconv.Itoa(runtimeConfig.ContextWindow)
	}
	if runtimeConfig.MaxOutputTokens > 0 {
		env[nexusMaxOutputTokensEnvName] = strconv.Itoa(runtimeConfig.MaxOutputTokens)
	}
	env[nexusModelSupportsVisionEnvName] = strconv.FormatBool(runtimeConfig.Vision)
	apiFormat := strings.TrimSpace(runtimeConfig.APIFormat)
	if apiFormat == "" || apiFormat == apiFormatAnthropicMessages || apiFormat == apiFormatChatCompletions || apiFormat == apiFormatResponses {
		env[nexusMultimodalUserContentEnvName] = "1"
		env[nexusMultimodalToolResultEnvName] = "1"
	}
}

// applyClaudeModelMetadataEnv 使用 Claude Code 对外支持的窗口上限控制本地压缩决策。
// CLAUDE_CODE_MAX_CONTEXT_TOKENS 仅供 Claude 内部用户使用，外部运行时不能依赖它。
func applyClaudeModelMetadataEnv(env map[string]string, runtimeConfig *RuntimeConfig) {
	if len(env) == 0 || runtimeConfig == nil || runtimeConfig.ContextWindow <= 0 {
		return
	}
	env[claudeAutoCompactWindowEnvName] = strconv.Itoa(runtimeConfig.ContextWindow)
}

// visionRuntimeEnvFromConfig 为辅助视觉模型生成独立命名空间，避免覆盖主模型路由。
func visionRuntimeEnvFromConfig(runtimeConfig *RuntimeConfig) map[string]string {
	if runtimeConfig == nil {
		return nil
	}
	providerType := "anthropic-compatible"
	switch strings.TrimSpace(runtimeConfig.APIFormat) {
	case apiFormatChatCompletions:
		providerType = "openai"
	case apiFormatResponses:
		providerType = "responses"
	}
	env := map[string]string{
		"NEXUS_VISION_PROVIDER_REF":            runtimeConfig.Provider,
		"NEXUS_VISION_API_PROVIDER":            providerType,
		"NEXUS_VISION_BASE_URL":                runtimeConfig.BaseURL,
		"NEXUS_VISION_API_KEY":                 runtimeConfig.AuthToken,
		"NEXUS_VISION_MODEL":                   runtimeConfig.Model,
		"NEXUS_VISION_MULTIMODAL_USER_CONTENT": "1",
	}
	return env
}

func anthropicRuntimeEnvFromConfig(runtimeConfig *RuntimeConfig) map[string]string {
	env := map[string]string{
		anthropicBaseURLEnvName:          runtimeConfig.BaseURL,
		anthropicModelEnvName:            runtimeConfig.Model,
		"ANTHROPIC_DEFAULT_OPUS_MODEL":   runtimeConfig.Model,
		"ANTHROPIC_DEFAULT_SONNET_MODEL": runtimeConfig.Model,
		"ANTHROPIC_DEFAULT_HAIKU_MODEL":  runtimeConfig.Model,
		"NEXUS_SUBAGENT_MODEL":           runtimeConfig.Model,
		NexusRuntimeProviderEnvName:      runtimeConfig.Provider,
		nexusAPIProviderEnvName:          "anthropic-compatible",
	}
	applyAnthropicCredentialsEnv(env, runtimeConfig)
	if runtimeConfig.Reasoning {
		applyDefaultModelCapabilitiesEnv(env, thinkingCapabilityName)
	}
	return env
}

func applyAnthropicCredentialsEnv(env map[string]string, runtimeConfig *RuntimeConfig) {
	token := strings.TrimSpace(runtimeConfig.AuthToken)
	if token == "" {
		return
	}
	if isFirstPartyAnthropicBaseURL(runtimeConfig.BaseURL) {
		env[anthropicAPIKeyEnvName] = token
		env[anthropicAuthTokenEnvName] = ""
		return
	}
	env[anthropicAuthTokenEnvName] = token
	env[anthropicAPIKeyEnvName] = ""
}

func isFirstPartyAnthropicBaseURL(baseURL string) bool {
	trimmed := strings.TrimSpace(baseURL)
	if trimmed == "" {
		return true
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return false
	}
	host := parsed.Hostname()
	if host == "" {
		return false
	}
	return strings.EqualFold(host, firstPartyAnthropicAPIHost)
}

func openAIRuntimeEnvFromConfig(runtimeConfig *RuntimeConfig) map[string]string {
	return map[string]string{
		"OPENAI_API_KEY":            runtimeConfig.AuthToken,
		"OPENAI_BASE_URL":           runtimeConfig.BaseURL,
		"OPENAI_MODEL":              runtimeConfig.Model,
		"NEXUS_SUBAGENT_MODEL":      runtimeConfig.Model,
		NexusRuntimeProviderEnvName: runtimeConfig.Provider,
		nexusAPIProviderEnvName:     "openai",
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

func defaultRuntimeEnv(runtimeKind string) map[string]string {
	if runtimeProfileForKind(runtimeKind).isNXS() {
		return map[string]string{
			nexusAutoCompactPctOverrideEnvName:     defaultAutoCompactPctOverride,
			nexusDisableProjectInstructionsEnvName: "1",
		}
	}
	return map[string]string{
		claudeAutoCompactPctOverrideEnvName: defaultAutoCompactPctOverride,
	}
}

// nxsHostManagedRuntimeEnv 声明 Nexus 是 provider 路由和 AutoDream 唤醒的唯一宿主。
func nxsHostManagedRuntimeEnv(runtimeKind string) map[string]string {
	if !runtimeProfileForKind(runtimeKind).isNXS() {
		return nil
	}
	return map[string]string{
		nexusAutoDreamWakeModeEnvName:     "host",
		nexusProviderManagedByHostEnvName: "1",
	}
}

// hostManagedScheduleRuntimeEnv 关闭仍内置调度器的第三方内核。
func hostManagedScheduleRuntimeEnv(runtimeKind string) map[string]string {
	if runtimeProfileForKind(runtimeKind).isNXS() {
		return nil
	}
	return map[string]string{claudeDisableCronEnvName: "1"}
}

func nxsDiagnosticsRuntimeEnv(runtimeKind string, enabled bool) map[string]string {
	if !enabled || !runtimeProfileForKind(runtimeKind).isNXS() {
		return nil
	}
	env := map[string]string{
		runtimectx.AgentSDKDiagnosticsJSONLEnvName:          "1",
		runtimectx.AgentSDKDiagnosticsStreamProgressEnvName: "0",
	}
	if value := strings.TrimSpace(os.Getenv(runtimectx.AgentSDKProviderDebugBodyEnvName)); value != "" {
		env[runtimectx.AgentSDKProviderDebugBodyEnvName] = value
	}
	return env
}

func explicitNXSProcessRuntimeEnv(runtimeKind string) map[string]string {
	if !runtimeProfileForKind(runtimeKind).isNXS() {
		return nil
	}
	env := map[string]string{}
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
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			env[key] = value
		}
	}
	if len(env) == 0 {
		return nil
	}
	return env
}

func buildScopedRuntimeEnv(
	ctx context.Context,
	ownerUserID string,
	isMainAgent bool,
) map[string]string {
	state, hasState := authctx.StateFromContext(ctx)
	ownerUserID = strings.TrimSpace(ownerUserID)
	env := map[string]string{
		// bridge 会继承宿主环境；未授予 CLI capability 时必须显式清空。
		nexusctlUserIDEnvName: "",
	}
	if ownerUserID != "" {
		if isMainAgent {
			env[nexusctlUserIDEnvName] = ownerUserID
		}
		env[nexusRuntimeUserIDEnvName] = ownerUserID
		if ownerUserID != authctx.SystemUserID || state.AuthRequired || authctx.PrincipalFromContext(ctx) != nil {
			env[nexusRuntimeScopeModeEnvName] = "user_scoped"
		} else {
			env[nexusRuntimeScopeModeEnvName] = "single_user"
		}
		return env
	}
	if hasState && !state.AuthRequired {
		env[nexusRuntimeScopeModeEnvName] = "single_user"
		env[nexusRuntimeUserIDEnvName] = authctx.SystemUserID
		return env
	}
	return env
}

// scrubInheritedRuntimeEnv 清空不应进入 runtime 子进程的宿主环境。
//
// bridge 的进程 transport 会把 os.Environ 与 options.Env 合并，单纯不
// 注入某个变量并不能形成隔离。这里先显式写空值，随后再覆盖当前用户
// 允许使用的 provider、诊断和协议变量。
func scrubInheritedRuntimeEnv() map[string]string {
	env := map[string]string{}
	for _, key := range []string{
		appfs.NexusStateRootEnvName,
		nexusAppRootEnvName,
		workspacePathEnvName,
		cacheFileDirEnvName,
		logPathEnvName,
		databaseDriverEnvName,
		databaseURLEnvName,
		connectorCredentialsKeyEnvName,
		connectorCredentialsKeyFileEnvName,
		nexusDesktopSessionTokenEnvName,
		"ACCESS_TOKEN",
		"AUTH_INIT_OWNER_USERNAME",
		"AUTH_INIT_OWNER_DISPLAY_NAME",
		"AUTH_INIT_OWNER_PASSWORD",
		"AUTH_SESSION_SECRET",
		"AUTH_SESSION_COOKIE_NAME",
		"DISCORD_BOT_TOKEN",
		"TELEGRAM_BOT_TOKEN",
		// provider 的环境凭据属于宿主全局秘密；当前用户的凭据只能
		// 通过 runtime config 显式投影，不能依赖继承环境。
		anthropicBaseURLEnvName,
		anthropicAPIKeyEnvName,
		anthropicAuthTokenEnvName,
		anthropicModelEnvName,
		"OPENAI_API_KEY",
		"OPENAI_BASE_URL",
		"OPENAI_MODEL",
		"AZURE_OPENAI_API_KEY",
		"AZURE_OPENAI_ENDPOINT",
		"AZURE_OPENAI_API_VERSION",
		"AWS_ACCESS_KEY_ID",
		"AWS_SECRET_ACCESS_KEY",
		"AWS_SESSION_TOKEN",
		"AWS_PROFILE",
		"GOOGLE_API_KEY",
		"GEMINI_API_KEY",
		"COHERE_API_KEY",
		"MISTRAL_API_KEY",
		"DEEPSEEK_API_KEY",
		"XAI_API_KEY",
	} {
		// 只为真实继承值生成清空覆盖，避免让 options 看起来像显式
		// 配置了另一套 provider 环境，同时仍能切断宿主中的实际秘密。
		if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
			env[key] = ""
		}
	}
	return env
}

// managedUserRuntimeEnv 注入宿主强制的用户 runtime 边界。
//
// 空值覆盖用于切断旧的 host root、数据库、密钥和远端记忆；路径则全部
// 落在当前 owner 边界，且 ExtraEnv 无法改写 workspace 或长期记忆根。
func managedUserRuntimeEnv(
	ownerUserID string,
	workspacePath string,
	runtimeKind string,
) map[string]string {
	runtimeRoot := appfs.UserRuntimeRoot(ownerUserID)
	homeRoot := filepath.Join(runtimeRoot, "home")
	workspacePath = strings.TrimSpace(workspacePath)
	env := map[string]string{
		appfs.NexusStateRootEnvName:                "",
		nexusAppRootEnvName:                        "",
		nexusConfigDirEnvName:                      runtimeRoot,
		claudeConfigDirEnvName:                     runtimeRoot,
		"HOME":                                     homeRoot,
		"USERPROFILE":                              homeRoot,
		"APPDATA":                                  filepath.Join(homeRoot, "AppData", "Roaming"),
		"LOCALAPPDATA":                             filepath.Join(homeRoot, "AppData", "Local"),
		"XDG_CONFIG_HOME":                          filepath.Join(homeRoot, ".config"),
		"XDG_CACHE_HOME":                           filepath.Join(runtimeRoot, "cache"),
		"XDG_DATA_HOME":                            filepath.Join(homeRoot, ".local", "share"),
		"XDG_STATE_HOME":                           filepath.Join(homeRoot, ".local", "state"),
		"TMPDIR":                                   filepath.Join(runtimeRoot, "tmp"),
		"TEMP":                                     filepath.Join(runtimeRoot, "tmp"),
		"TMP":                                      filepath.Join(runtimeRoot, "tmp"),
		cacheFileDirEnvName:                        filepath.Join(runtimeRoot, "cache"),
		logPathEnvName:                             filepath.Join(runtimeRoot, "logs", "runtime.log"),
		databaseDriverEnvName:                      "",
		databaseURLEnvName:                         "",
		connectorCredentialsKeyEnvName:             "",
		connectorCredentialsKeyFileEnvName:         "",
		nexusDesktopSessionTokenEnvName:            "",
		nexusNXSRuntimeCacheDirEnvName:             filepath.Join(runtimeRoot, "cache"),
		nexusAgentRuntimeKindEnvName:               strings.TrimSpace(runtimeKind),
		nexusAgentRuntimeEnvName:                   strings.TrimSpace(runtimeKind),
		nexusNXSCommandPathEnvName:                 "",
		nexusClaudeCommandPathEnvName:              "",
		nexusMemoryDirEnvName:                      workspacePath,
		nexusEnableRemoteMemoryEnvName:             "",
		nexusRemoteMemoryDirEnvName:                "",
		nexusctlCommandPathEnvName:                 "",
		nexuscfgCommandPathEnvName:                 "",
		protocol.NexusConfigBrokerURLEnvName:       "",
		protocol.NexusConfigCapabilityTokenEnvName: "",
		legacyNexusCommandPathEnvName:              "",
		nexusctlWorkspacePathEnvName:               "",
	}
	env[workspacePathEnvName] = workspacePath
	return env
}

func workspaceRuntimeEnv(workspacePath string, includeNexusctl bool) map[string]string {
	trimmedWorkspacePath := strings.TrimSpace(workspacePath)
	if trimmedWorkspacePath == "" {
		return nil
	}
	binDir := appfs.AgentRuntimeBinDir()
	configurationCommandPath := strings.TrimSpace(os.Getenv(nexuscfgCommandPathEnvName))
	if configurationCommandPath == "" {
		configurationCommandPath = runtimeCLIShimPath(binDir, "nexuscfg")
	}
	env := map[string]string{
		nexuscfgCommandPathEnvName: configurationCommandPath,
	}
	if includeNexusctl {
		commandPath := strings.TrimSpace(os.Getenv(nexusctlCommandPathEnvName))
		if commandPath == "" {
			commandPath = runtimeCLIShimPath(binDir, "nexusctl")
		}
		env[nexusctlCommandPathEnvName] = commandPath
		env[nexusctlWorkspacePathEnvName] = trimmedWorkspacePath
	}
	currentPath := strings.TrimSpace(os.Getenv("PATH"))
	if currentPath == "" {
		env["PATH"] = binDir
	} else {
		env["PATH"] = binDir + string(os.PathListSeparator) + currentPath
	}
	return env
}

func runtimeCLIShimPath(binDir string, name string) string {
	fileName := name
	if runtime.GOOS == "windows" {
		fileName = name + ".cmd"
	}
	return filepath.Join(binDir, fileName)
}

func mergeRuntimeEnv(
	base map[string]string,
	extra map[string]string,
) map[string]string {
	if len(base) == 0 && len(extra) == 0 {
		return nil
	}
	result := make(map[string]string, len(base)+len(extra))
	maps.Copy(result, base)
	maps.Copy(result, extra)
	return result
}
