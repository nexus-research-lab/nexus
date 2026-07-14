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
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	runtimeprovider "github.com/nexus-research-lab/nexus/internal/runtime/provider"
)

const nexusctlUserIDEnvName = "NEXUSCTL_USER_ID"
const nexusctlWorkspacePathEnvName = "NEXUSCTL_WORKSPACE_PATH"
const nexusctlCommandPathEnvName = "NEXUSCTL_COMMAND_PATH"
const apiFormatAnthropicMessages = runtimeprovider.APIFormatAnthropicMessages
const apiFormatChatCompletions = runtimeprovider.APIFormatChatCompletions
const nexusAutoCompactPctOverrideEnvName = "NEXUS_AUTOCOMPACT_PCT_OVERRIDE"
const defaultClaudeAutoCompactPctOverride = "70"
const thinkingCapabilityName = "thinking"
const nexusAPIProviderEnvName = "NEXUS_API_PROVIDER"
const anthropicBaseURLEnvName = "ANTHROPIC_BASE_URL"
const anthropicAPIKeyEnvName = "ANTHROPIC_API_KEY"
const anthropicAuthTokenEnvName = "ANTHROPIC_AUTH_TOKEN"
const anthropicModelEnvName = "ANTHROPIC_MODEL"
const enableToolSearchEnvName = "ENABLE_TOOL_SEARCH"
const firstPartyAnthropicAPIHost = "api.anthropic.com"
const nexusDisableProjectInstructionsEnvName = "NEXUS_DISABLE_PROJECT_INSTRUCTIONS"
const nexusCachedMicrocompactEnvName = "NEXUS_CACHED_MICROCOMPACT"
const nexusMaxContextTokensEnvName = "NEXUS_MAX_CONTEXT_TOKENS"

// NexusRuntimeProviderEnvName 表示当前 SDK runtime 实际解析出的 provider key。
const NexusRuntimeProviderEnvName = "NEXUS_RUNTIME_PROVIDER"
const nexusRuntimeScopeModeEnvName = "NEXUS_RUNTIME_SCOPE_MODE"
const nexusRuntimeUserIDEnvName = "NEXUS_RUNTIME_USER_ID"

const (
	nexusAutoDreamWakeModeEnvName     = "NEXUS_AUTO_DREAM_WAKE_MODE"
	nexusProviderManagedByHostEnvName = "NEXUS_PROVIDER_MANAGED_BY_HOST"
	claudeDisableCronEnvName          = "CLAUDE_CODE_DISABLE_CRON"
)

func runtimeEnvFromConfig(runtimeConfig *RuntimeConfig, runtimeKind string) map[string]string {
	if runtimeConfig == nil {
		return nil
	}
	profile := resolveRuntimeProfile(runtimeKind, os.Getenv)
	var env map[string]string
	switch strings.TrimSpace(runtimeConfig.APIFormat) {
	case "", apiFormatAnthropicMessages:
		env = anthropicRuntimeEnvFromConfig(runtimeConfig)
	case apiFormatChatCompletions:
		if profile.isNXS() {
			env = openAIRuntimeEnvFromConfig(runtimeConfig)
		}
	}
	if profile.isNXS() {
		applyNXSModelLimitsEnv(env, runtimeConfig)
	}
	return env
}

// applyNXSModelLimitsEnv 把产品模型卡中的已知上限交给 nxs，不让运行时按模型名猜测。
func applyNXSModelLimitsEnv(env map[string]string, runtimeConfig *RuntimeConfig) {
	if len(env) == 0 || runtimeConfig == nil || runtimeConfig.ContextWindow <= 0 {
		return
	}
	env[nexusMaxContextTokensEnvName] = strconv.Itoa(runtimeConfig.ContextWindow)
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

func defaultRuntimeEnv() map[string]string {
	return map[string]string{
		nexusAutoCompactPctOverrideEnvName:     defaultClaudeAutoCompactPctOverride,
		nexusDisableProjectInstructionsEnvName: "1",
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
	commandPath := strings.TrimSpace(os.Getenv(nexusctlCommandPathEnvName))
	if commandPath == "" {
		commandPath = nexusctlShimPath(binDir)
	}
	env := map[string]string{
		nexusctlCommandPathEnvName:   commandPath,
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

func nexusctlShimPath(binDir string) string {
	fileName := "nexusctl"
	if runtime.GOOS == "windows" {
		fileName = "nexusctl.cmd"
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
