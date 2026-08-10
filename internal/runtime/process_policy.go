// INPUT: 即将复用 runtime session 的进程路径、工作目录、sandbox 与身份隔离选项。
// OUTPUT: 不包含明文凭据的稳定 process-policy 指纹。
// POS: Reconfigure 之前的进程级安全边界；指纹变化必须替换旧 runtime。
package runtime

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

type runtimeHookShape struct {
	Event     string `json:"event"`
	Matcher   string `json:"matcher"`
	Timeout   int64  `json:"timeout"`
	Callbacks int    `json:"callbacks"`
}

type runtimeDirectConnectShape struct {
	URL                  string `json:"url"`
	SessionKey           string `json:"session_key"`
	DeleteSessionOnClose bool   `json:"delete_session_on_close"`
	DialTimeout          int64  `json:"dial_timeout"`
	HTTPClientType       string `json:"http_client_type"`
}

type runtimeProcessPolicy struct {
	CLIPath           string                       `json:"cli_path"`
	CWD               string                       `json:"cwd"`
	User              string                       `json:"user"`
	Executable        string                       `json:"executable"`
	ExecutableArgs    []string                     `json:"executable_args"`
	PathToExecutable  string                       `json:"path_to_executable"`
	TransportType     string                       `json:"transport_type"`
	DirectConnect     *runtimeDirectConnectShape   `json:"direct_connect,omitempty"`
	Settings          string                       `json:"settings"`
	SettingsObject    map[string]any               `json:"settings_object,omitempty"`
	Sandbox           *agentclient.SandboxSettings `json:"sandbox,omitempty"`
	ExtraArgs         map[string]string            `json:"extra_args,omitempty"`
	ExtraBoolArgs     []string                     `json:"extra_bool_args,omitempty"`
	AvailableTools    []string                     `json:"available_tools,omitempty"`
	ToolPreset        string                       `json:"tool_preset,omitempty"`
	IsolationEnv      map[string]string            `json:"isolation_env,omitempty"`
	Hooks             []runtimeHookShape           `json:"hooks,omitempty"`
	HookEventsEnabled bool                         `json:"hook_events_enabled"`
}

func managedRuntimeProcessPolicyFingerprint(options agentclient.Options) string {
	policy := runtimeProcessPolicy{
		CLIPath:           strings.TrimSpace(options.CLIPath),
		CWD:               strings.TrimSpace(options.CWD),
		User:              strings.TrimSpace(options.User),
		Executable:        strings.TrimSpace(options.Executable),
		ExecutableArgs:    append([]string(nil), options.ExecutableArgs...),
		PathToExecutable:  strings.TrimSpace(options.PathToExecutable),
		TransportType:     fmt.Sprintf("%T", options.Transport),
		Settings:          options.Settings,
		SettingsObject:    options.SettingsObject,
		Sandbox:           options.Sandbox,
		ExtraArgs:         options.ExtraArgs,
		ExtraBoolArgs:     append([]string(nil), options.ExtraBoolArgs...),
		AvailableTools:    append([]string(nil), options.Tools.Available...),
		IsolationEnv:      runtimeIsolationEnvironment(options.Env),
		Hooks:             runtimeHookShapes(options),
		HookEventsEnabled: options.Hooks.IncludeEvents,
	}
	if options.Tools.Preset != nil {
		policy.ToolPreset = options.Tools.Preset.Preset
	}
	if options.DirectConnect != nil {
		policy.DirectConnect = &runtimeDirectConnectShape{
			URL:                  strings.TrimSpace(options.DirectConnect.URL),
			SessionKey:           strings.TrimSpace(options.DirectConnect.SessionKey),
			DeleteSessionOnClose: options.DirectConnect.DeleteSessionOnClose,
			DialTimeout:          int64(options.DirectConnect.DialTimeout),
			HTTPClientType:       fmt.Sprintf("%T", options.DirectConnect.HTTPClient),
		}
	}
	payload, err := json.Marshal(policy)
	if err != nil {
		payload = []byte(fmt.Sprintf("%#v", policy))
	}
	return fmt.Sprintf("sha256:%x", sha256.Sum256(payload))
}

func runtimeIsolationEnvironment(environment map[string]string) map[string]string {
	result := make(map[string]string)
	for key, value := range environment {
		normalized := strings.ToUpper(strings.TrimSpace(key))
		if runtimeProcessEnvironmentKey(normalized) {
			result[normalized] = value
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func runtimeProcessEnvironmentKey(key string) bool {
	switch key {
	case "HOME", "PATH", "TMPDIR", "NEXUS_STATE_ROOT", "NEXUS_CONFIG_DIR",
		"CLAUDE_CONFIG_DIR", "NEXUS_RUNTIME_USER_ID", "NEXUS_RUNTIME_LAUNCHER":
		return true
	}
	return strings.HasPrefix(key, "NEXUS_RUNTIME_IDENTITY_") ||
		strings.HasPrefix(key, "NEXUS_RUNTIME_ISOLATION_") ||
		strings.HasPrefix(key, "NEXUS_RUNTIME_POLICY_")
}

func runtimeHookShapes(options agentclient.Options) []runtimeHookShape {
	result := make([]runtimeHookShape, 0)
	for event, matchers := range options.Hooks.Matchers {
		for _, matcher := range matchers {
			result = append(result, runtimeHookShape{
				Event:     string(event),
				Matcher:   matcher.Matcher,
				Timeout:   int64(matcher.Timeout),
				Callbacks: len(matcher.Hooks),
			})
		}
	}
	slices.SortFunc(result, func(left, right runtimeHookShape) int {
		if order := strings.Compare(left.Event, right.Event); order != 0 {
			return order
		}
		if order := strings.Compare(left.Matcher, right.Matcher); order != 0 {
			return order
		}
		if left.Timeout < right.Timeout {
			return -1
		}
		if left.Timeout > right.Timeout {
			return 1
		}
		return left.Callbacks - right.Callbacks
	})
	return result
}
