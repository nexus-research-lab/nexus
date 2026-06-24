package runtime

import (
	"strings"
)

const (
	// AgentSDKDiagnosticsEnvName 控制 Agent SDK 诊断事件输出。
	AgentSDKDiagnosticsEnvName = "NEXUS_AGENT_SDK_DIAGNOSTICS"
	// AgentSDKDiagnosticsStreamProgressEnvName 控制瞬时 stream 诊断事件输出。
	AgentSDKDiagnosticsStreamProgressEnvName = "NEXUS_AGENT_SDK_DIAGNOSTICS_STREAM_PROGRESS"
	// AgentSDKDebugEnvName 兼容 SDK debug 开关，开启后也输出诊断事件。
	AgentSDKDebugEnvName = "NEXUS_AGENT_SDK_DEBUG"
	// AgentSDKProviderDebugBodyEnvName 控制 provider 请求体诊断输出范围。
	AgentSDKProviderDebugBodyEnvName = "NEXUS_AGENT_SDK_PROVIDER_DEBUG_BODY"
)

// AgentSDKDiagnosticsEnabled 判断当前 runtime 环境是否开启 Agent SDK 诊断。
func AgentSDKDiagnosticsEnabled(env map[string]string) bool {
	if value, exists := lookupRuntimeEnv(env, AgentSDKDiagnosticsEnvName); exists {
		return runtimeEnvTruthy(value)
	}
	if value, exists := lookupRuntimeEnv(env, AgentSDKDebugEnvName); exists {
		return runtimeEnvTruthy(value)
	}
	return false
}

// AgentSDKDiagnosticsValue 返回当前生效的诊断开关值。
func AgentSDKDiagnosticsValue(env map[string]string) string {
	if value, exists := lookupRuntimeEnv(env, AgentSDKDiagnosticsEnvName); exists {
		return strings.TrimSpace(value)
	}
	if value, exists := lookupRuntimeEnv(env, AgentSDKDebugEnvName); exists {
		return strings.TrimSpace(value)
	}
	return ""
}

// AgentSDKProviderDebugBodyValue 返回请求体诊断开关值，仅用于日志摘要。
func AgentSDKProviderDebugBodyValue(env map[string]string) string {
	value, _ := lookupRuntimeEnv(env, AgentSDKProviderDebugBodyEnvName)
	return strings.TrimSpace(value)
}

func lookupRuntimeEnv(env map[string]string, key string) (string, bool) {
	if env != nil {
		if value, exists := env[key]; exists {
			return value, true
		}
	}
	return "", false
}

func runtimeEnvTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on", "stderr", "terminal", "debug":
		return true
	default:
		return false
	}
}
