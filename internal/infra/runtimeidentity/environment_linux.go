//go:build linux

package runtimeidentity

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode"
)

var blockedRuntimeEnvironment = map[string]struct{}{
	"ACCESS_TOKEN":                   {},
	"AUTH_INIT_OWNER_DISPLAY_NAME":   {},
	"AUTH_INIT_OWNER_PASSWORD":       {},
	"AUTH_INIT_OWNER_USERNAME":       {},
	"AUTH_SESSION_SECRET":            {},
	"CONNECTOR_CREDENTIALS_KEY":      {},
	"CONNECTOR_CREDENTIALS_KEY_FILE": {},
	"DATABASE_DRIVER":                {},
	"DATABASE_URL":                   {},
	"DISCORD_BOT_TOKEN":              {},
	"LD_AUDIT":                       {},
	"LD_LIBRARY_PATH":                {},
	"LD_PRELOAD":                     {},
	"NEXUS_APP_ROOT":                 {},
	"NEXUS_CLAUDE_COMMAND_PATH":      {},
	"NEXUS_DESKTOP_SESSION_TOKEN":    {},
	"NEXUS_RUNTIME_ISOLATION_CONFIG": {},
	"NEXUS_RUNTIME_LAUNCHER_PATH":    {},
	"NEXUS_NXS_COMMAND_PATH":         {},
	"NEXUS_SIMPLE":                   {},
	"NEXUS_STATE_ROOT":               {},
	"NEXUSCTL_COMMAND_PATH":          {},
	"NEXUSCTL_USER_ID":               {},
	"NEXUSCTL_WORKSPACE_PATH":        {},
	"NEXUS_RUNTIME_USER_ID":          {},
	"TELEGRAM_BOT_TOKEN":             {},
	"CLAUDE_CODE_DISABLE_HOOKS":      {},
	"CLAUDE_CODE_SIMPLE":             {},
}

var allowedRuntimeEnvironmentNames = map[string]struct{}{
	"ALL_PROXY": {}, "BUN_CONFIG_REGISTRY": {}, "CACHE_FILE_DIR": {},
	"CLAUDE_AGENT_SDK_VERSION": {}, "CLAUDE_CODE_ENTRYPOINT": {},
	"COLORTERM": {}, "ENABLE_TOOL_SEARCH": {}, "FORCE_COLOR": {},
	"GOCACHE": {}, "GOMODCACHE": {}, "GONOSUMDB": {}, "GOPRIVATE": {},
	"GOPROXY": {}, "GOROOT": {}, "GOSUMDB": {}, "GOTOOLCHAIN": {},
	"HTTPS_PROXY": {}, "HTTP_PROXY": {}, "LANG": {}, "LANGUAGE": {},
	"LOG_PATH": {}, "NO_COLOR": {}, "NO_PROXY": {}, "NPM_CONFIG_REGISTRY": {},
	"PATH": {}, "PIP_BREAK_SYSTEM_PACKAGES": {}, "PIP_INDEX_URL": {},
	"PNPM_HOME": {}, "PNPM_REGISTRY": {}, "SSL_CERT_DIR": {},
	"SSL_CERT_FILE": {}, "TERM": {}, "TZ": {}, "UV_BREAK_SYSTEM_PACKAGES": {},
	"UV_DEFAULT_INDEX": {}, "UV_INDEX_URL": {},
	"all_proxy": {}, "http_proxy": {}, "https_proxy": {}, "no_proxy": {},
}

var allowedRuntimeEnvironmentPrefixes = []string{
	"ANTHROPIC_",
	"AWS_",
	"AZURE_",
	"BRAVE_",
	"BUN_",
	"CARGO_",
	"CLAUDE_",
	"COHERE_",
	"DEEPSEEK_",
	"EXA_",
	"GEMINI_",
	"GIT_",
	"GOOGLE_",
	"JAVA_",
	"MISTRAL_",
	"NEXUS_",
	"NODE_",
	"OPENAI_",
	"PIP_",
	"PNPM_",
	"RUSTUP_",
	"TAVILY_",
	"UV_",
	"XAI_",
}

func sanitizedRuntimeEnvironment(
	environ []string,
	config launcherConfig,
	policy preparedPolicy,
) []string {
	values := map[string]string{}
	explicitNames := make(map[string]struct{}, len(policy.EnvironmentNames))
	for _, name := range policy.EnvironmentNames {
		explicitNames[name] = struct{}{}
	}
	for _, entry := range environ {
		name, value, ok := strings.Cut(entry, "=")
		name = strings.TrimSpace(name)
		if !ok || name == "" ||
			(!safeInheritedRuntimeEnvironment(name) && !explicitRuntimeEnvironment(name, explicitNames)) {
			continue
		}
		values[name] = value
	}
	runtimeRoot := runtimeRootForPolicy(config, policy.OwnerUserID)
	homeRoot := policy.Identity.HomeDir
	tempRoot := policy.Identity.TempDir
	runtimeBinRoot := filepath.Join(config.StateRoot, "app", ".agents", "bin")
	pathValue := strings.Join([]string{
		runtimeBinRoot,
		filepath.Join(homeRoot, ".local", "bin"),
		"/usr/local/sbin",
		"/usr/local/bin",
		"/usr/sbin",
		"/usr/bin",
		"/sbin",
		"/bin",
	}, ":")
	for name, value := range map[string]string{
		"APPDATA":                     filepath.Join(homeRoot, "AppData", "Roaming"),
		"CACHE_FILE_DIR":              filepath.Join(runtimeRoot, "cache"),
		"CLAUDE_CONFIG_DIR":           runtimeRoot,
		"HOME":                        homeRoot,
		"LOCALAPPDATA":                filepath.Join(homeRoot, "AppData", "Local"),
		"LOG_PATH":                    filepath.Join(runtimeRoot, "logs", "runtime.log"),
		"LOGNAME":                     policy.Identity.Username,
		"NEXUS_CONFIG_DIR":            runtimeRoot,
		"NEXUS_NXS_RUNTIME_CACHE_DIR": filepath.Join(runtimeRoot, "cache"),
		"NEXUS_RUNTIME_ISOLATION":     "enforced",
		"NEXUS_RUNTIME_SCOPE_MODE":    "user_scoped",
		"NEXUS_RUNTIME_USER_ID":       policy.OwnerUserID,
		"NEXUSCTL_USER_ID":            policy.OwnerUserID,
		"NEXUSCTL_COMMAND_PATH":       filepath.Join(runtimeBinRoot, "nexusctl"),
		"NEXUSCTL_WORKSPACE_PATH":     policy.CWD,
		"PATH":                        pathValue,
		"PWD":                         policy.CWD,
		"SHELL":                       "/bin/bash",
		"TEMP":                        tempRoot,
		"TMP":                         tempRoot,
		"TMPDIR":                      tempRoot,
		"USER":                        policy.Identity.Username,
		"USERPROFILE":                 homeRoot,
		"WORKSPACE_PATH":              policy.CWD,
		"XDG_CACHE_HOME":              filepath.Join(runtimeRoot, "cache"),
		"XDG_CONFIG_HOME":             filepath.Join(homeRoot, ".config"),
		"XDG_DATA_HOME":               filepath.Join(homeRoot, ".local", "share"),
		"XDG_STATE_HOME":              filepath.Join(homeRoot, ".local", "state"),
	} {
		values[name] = value
	}
	appendGitSafeDirectories(values, policy)
	delete(values, "NEXUS_RUNTIME_ISOLATION_TICKET")
	delete(values, "NEXUS_RUNTIME_ISOLATION_MODE")

	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	slices.Sort(names)
	result := make([]string, 0, len(names))
	for _, name := range names {
		result = append(result, name+"="+values[name])
	}
	return result
}

// appendGitSafeDirectories 只放宽当前 policy 已授权的根，解决 shared project
// 由 root:project 持有时 Git 的 dubious ownership 检查；Landlock 与 ACL 仍是
// 实际访问边界。
func appendGitSafeDirectories(values map[string]string, policy preparedPolicy) {
	count, err := strconv.Atoi(strings.TrimSpace(values["GIT_CONFIG_COUNT"]))
	if err != nil || count < 0 || count > 64 {
		count = 0
	}
	directories := []string{policy.CWD}
	for _, root := range append(slices.Clone(policy.ReadRoots), policy.WriteRoots...) {
		if pathWithin(policy.CWD, root) {
			directories = append(directories, root)
		}
	}
	for _, directory := range compactPaths(directories) {
		index := strconv.Itoa(count)
		values["GIT_CONFIG_KEY_"+index] = "safe.directory"
		values["GIT_CONFIG_VALUE_"+index] = directory
		count++
	}
	values["GIT_CONFIG_COUNT"] = strconv.Itoa(count)
}

func explicitRuntimeEnvironment(name string, explicitNames map[string]struct{}) bool {
	if _, ok := explicitNames[name]; !ok {
		return false
	}
	if _, blocked := blockedRuntimeEnvironment[name]; blocked {
		return false
	}
	if strings.HasPrefix(name, "LD_") {
		return false
	}
	if _, allowed := allowedRuntimeEnvironmentNames[name]; allowed {
		return true
	}
	if strings.HasPrefix(name, "LC_") {
		return true
	}
	for _, prefix := range allowedRuntimeEnvironmentPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func safeInheritedRuntimeEnvironment(name string) bool {
	switch name {
	case "CLAUDE_AGENT_SDK_VERSION", "CLAUDE_CODE_ENTRYPOINT", "COLORTERM",
		"FORCE_COLOR", "LANG", "LANGUAGE", "NEXUS_ENTRYPOINT", "NO_COLOR",
		"PATH", "SSL_CERT_DIR", "SSL_CERT_FILE", "TERM", "TZ":
		return true
	}
	return strings.HasPrefix(name, "LC_")
}

func normalizeEnvironmentNames(names []string) ([]string, error) {
	normalized := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if !validEnvironmentName(name) {
			return nil, fmt.Errorf("runtime environment name 无效: %q", name)
		}
		normalized = append(normalized, name)
	}
	slices.Sort(normalized)
	return slices.Compact(normalized), nil
}

func validEnvironmentName(name string) bool {
	if name == "" {
		return false
	}
	for index, character := range name {
		if character == '_' || unicode.IsLetter(character) ||
			(index > 0 && unicode.IsDigit(character)) {
			continue
		}
		return false
	}
	return true
}
