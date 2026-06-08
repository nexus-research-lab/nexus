package clientopts

import (
	"fmt"
	"strings"

	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

const nexusAPIProviderEnvName = "NEXUS_API_PROVIDER"

// ResolvedRuntimeProvider 返回 SDK runtime 实际使用的 provider key。
func ResolvedRuntimeProvider(provider string, options agentclient.Options) string {
	if options.Env != nil {
		if resolved := strings.TrimSpace(options.Env[NexusRuntimeProviderEnvName]); resolved != "" {
			return resolved
		}
	}
	return strings.TrimSpace(provider)
}

// RuntimeStartupLogFields 构造安全的 runtime 启动日志字段。
func RuntimeStartupLogFields(options agentclient.Options) []any {
	return []any{
		"runtime_kind", string(options.Runtime.Kind),
		"runtime_model", strings.TrimSpace(options.Model),
		"permission_mode", string(options.Runtime.PermissionMode),
		"allow_skip_permissions", options.Runtime.AllowDangerouslySkipPermissions,
		"cli_path", strings.TrimSpace(options.CLIPath),
		"executable", strings.TrimSpace(options.Executable),
		"path_to_executable", strings.TrimSpace(options.PathToExecutable),
		"cwd", strings.TrimSpace(options.CWD),
		"resume_id_present", strings.TrimSpace(options.Session.ResumeID) != "",
		"max_thinking_tokens", options.Runtime.MaxThinkingTokens,
		"max_turns", options.Runtime.MaxTurns,
		"setting_sources", append([]string(nil), options.SettingSources...),
		"allowed_tools_count", len(options.Tools.Allow),
		"denied_tools_count", len(options.Tools.Deny),
		"mcp_servers_count", len(options.MCP.Servers),
		"api_provider_env", strings.TrimSpace(options.Env[nexusAPIProviderEnvName]),
		"nexus_runtime_provider_env", strings.TrimSpace(options.Env[NexusRuntimeProviderEnvName]),
		"anthropic_base_url_env", RuntimeEnvConfigured(options.Env, "ANTHROPIC_BASE_URL"),
		"anthropic_model_env", RuntimeEnvConfigured(options.Env, "ANTHROPIC_MODEL"),
		"anthropic_auth_token_env", RuntimeEnvConfigured(options.Env, "ANTHROPIC_AUTH_TOKEN"),
		"anthropic_api_key_env", RuntimeEnvConfigured(options.Env, "ANTHROPIC_API_KEY"),
		"openai_base_url_env", RuntimeEnvConfigured(options.Env, "OPENAI_BASE_URL"),
		"openai_model_env", RuntimeEnvConfigured(options.Env, "OPENAI_MODEL"),
		"openai_api_key_env", RuntimeEnvConfigured(options.Env, "OPENAI_API_KEY"),
		"nxs_command_path_env", RuntimeEnvConfigured(options.Env, nexusNXSCommandPathEnvName),
		"diagnostics_enabled", runtimectx.AgentSDKDiagnosticsEnabled(options.Env),
		"diagnostics_env", runtimectx.AgentSDKDiagnosticsValue(options.Env),
	}
}

// RuntimeEnvConfigured 判断 runtime 环境变量是否已显式配置。
func RuntimeEnvConfigured(env map[string]string, key string) bool {
	if env == nil {
		return false
	}
	return strings.TrimSpace(env[key]) != ""
}

// ShouldLogRuntimeStartupDiagnostic 判断默认模式下是否记录启动诊断事件。
func ShouldLogRuntimeStartupDiagnostic(event agentclient.DiagnosticEvent) bool {
	return strings.TrimSpace(event.Component) == "bridge.process" &&
		strings.TrimSpace(event.Event) == "process_start"
}

// ShouldWarnRuntimeStartupDiagnostic 判断默认模式下是否记录启动告警事件。
func ShouldWarnRuntimeStartupDiagnostic(event agentclient.DiagnosticEvent) bool {
	if strings.TrimSpace(event.Component) != "bridge.process" {
		return false
	}
	switch strings.TrimSpace(event.Event) {
	case "process_exit":
		return diagnosticAttrConfigured(event.Attributes, "error")
	case "stdout_decode_error",
		"stderr_read_error",
		"cli_version_unsupported",
		"process_terminate_error",
		"process_terminate_timeout_kill":
		return true
	default:
		return false
	}
}

func diagnosticAttrConfigured(attrs map[string]any, key string) bool {
	if attrs == nil {
		return false
	}
	value, ok := attrs[key]
	if !ok {
		return false
	}
	return strings.TrimSpace(fmt.Sprint(value)) != ""
}

// SanitizeRuntimeDiagnosticAttributes 返回适合落日志的 SDK diagnostics 属性。
func SanitizeRuntimeDiagnosticAttributes(event string, attrs map[string]any) map[string]any {
	if len(attrs) == 0 {
		return nil
	}
	result := make(map[string]any, len(attrs))
	for key, value := range attrs {
		normalizedKey := strings.TrimSpace(key)
		if normalizedKey == "" {
			continue
		}
		switch normalizedKey {
		case "args":
			if strings.TrimSpace(event) == "process_start" {
				result["args_summary"] = summarizeRuntimeArgs(value)
			}
		case "stdout_prefix", "stdout_suffix":
			continue
		default:
			result[normalizedKey] = value
		}
	}
	return result
}

func summarizeRuntimeArgs(value any) map[string]any {
	values, ok := value.([]string)
	if !ok {
		rawValues, rawOK := value.([]any)
		if !rawOK {
			return map[string]any{"count": 0}
		}
		values = make([]string, 0, len(rawValues))
		for _, item := range rawValues {
			values = append(values, fmt.Sprint(item))
		}
	}
	flags := make([]string, 0, len(values))
	for index := 0; index < len(values); index++ {
		arg := strings.TrimSpace(values[index])
		if !strings.HasPrefix(arg, "--") {
			continue
		}
		flagName := runtimeArgName(arg)
		flags = append(flags, flagName)
		if runtimeArgHasValue(flagName) && !strings.Contains(arg, "=") && index+1 < len(values) {
			index++
		}
	}
	return map[string]any{
		"count": len(values),
		"flags": flags,
	}
}

func runtimeArgName(arg string) string {
	if equalIndex := strings.Index(arg, "="); equalIndex >= 0 {
		return strings.TrimSpace(arg[:equalIndex])
	}
	return strings.TrimSpace(arg)
}

func runtimeArgHasValue(arg string) bool {
	switch strings.TrimSpace(arg) {
	case "--output-format",
		"--input-format",
		"--system-prompt",
		"--system-prompt-file",
		"--append-system-prompt",
		"--tools",
		"--allowedTools",
		"--disallowedTools",
		"--model",
		"--permission-mode",
		"--resume",
		"--session-id",
		"--resume-session-at",
		"--settings",
		"--mcp-config",
		"--debug-file",
		"--max-turns":
		return true
	default:
		return false
	}
}
