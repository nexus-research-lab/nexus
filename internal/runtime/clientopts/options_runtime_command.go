package clientopts

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	runtimeprovider "github.com/nexus-research-lab/nexus/internal/runtime/provider"
)

const nexusClaudeCommandPathEnvName = "NEXUS_CLAUDE_COMMAND_PATH"
const nexusNXSCommandPathEnvName = "NEXUS_NXS_COMMAND_PATH"
const nexusAgentRuntimeKindEnvName = "NEXUS_AGENT_RUNTIME_KIND"
const nexusAgentRuntimeEnvName = "NEXUS_AGENT_RUNTIME"
const nexusAppRootEnvName = "NEXUS_APP_ROOT"
const runtimeKindClaude = runtimeprovider.RuntimeKindClaude
const runtimeKindNXS = runtimeprovider.RuntimeKindNXS

type runtimeCommandConfig struct {
	CLIPath          string
	Executable       string
	PathToExecutable string
}

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

func (p runtimeProfile) isClaude() bool {
	return p.kind == runtimeKindClaude
}

func (p runtimeProfile) supportsAPIFormat(apiFormat string) bool {
	return runtimeprovider.SupportsAPIFormat(p.kind, apiFormat)
}

func processRuntimeCommandConfig(runtimeKind string) runtimeCommandConfig {
	return resolveRuntimeCommandConfigWith(
		runtimeKind,
		runtime.GOOS,
		os.Getenv,
		exec.LookPath,
		func(path string) bool {
			info, err := os.Stat(path)
			return err == nil && !info.IsDir()
		},
		filepath.Glob,
	)
}

func resolveRuntimeCommandConfigWith(
	runtimeKind string,
	goos string,
	getenv func(string) string,
	lookPath func(string) (string, error),
	fileExists func(string) bool,
	globPaths func(string) ([]string, error),
) runtimeCommandConfig {
	profile := resolveRuntimeProfile(runtimeKind, getenv)
	commandPath := profile.resolveCommandPath(goos, getenv, lookPath, fileExists, globPaths)
	return runtimeCommandConfig{CLIPath: commandPath}
}

func resolveRuntimeCommandPathWith(
	runtimeKind string,
	goos string,
	getenv func(string) string,
	lookPath func(string) (string, error),
	fileExists func(string) bool,
	globPaths func(string) ([]string, error),
) string {
	profile := resolveRuntimeProfile(runtimeKind, getenv)
	return profile.resolveCommandPath(goos, getenv, lookPath, fileExists, globPaths)
}

func (p runtimeProfile) resolveCommandPath(
	goos string,
	getenv func(string) string,
	lookPath func(string) (string, error),
	fileExists func(string) bool,
	globPaths func(string) ([]string, error),
) string {
	if p.isNXS() {
		return resolveNXSCommandPathWith(goos, getenv, fileExists)
	}
	if override := strings.TrimSpace(getenv(nexusClaudeCommandPathEnvName)); override != "" {
		return override
	}

	for _, name := range claudeCommandNames(goos) {
		if path, err := lookPath(name); err == nil && strings.TrimSpace(path) != "" {
			return path
		}
	}
	for _, candidate := range knownClaudeCommandPaths(goos, getenv) {
		if fileExists(candidate) {
			return candidate
		}
	}
	for _, candidate := range knownClaudeCommandPathGlobs(goos, getenv, globPaths) {
		if fileExists(candidate) {
			return candidate
		}
	}
	return ""
}

func resolveRuntimeKind(runtimeKind string, getenv func(string) string) string {
	for _, value := range []string{
		getenv(nexusAgentRuntimeKindEnvName),
		getenv(nexusAgentRuntimeEnvName),
		runtimeKind,
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

func resolveNXSCommandPathWith(
	goos string,
	getenv func(string) string,
	fileExists func(string) bool,
) string {
	if appRoot := strings.TrimSpace(getenv(nexusAppRootEnvName)); appRoot != "" {
		candidate := filepath.Join(appRoot, "bin", nxsExecutableName(goos))
		if fileExists(candidate) {
			return candidate
		}
	}
	if override := strings.TrimSpace(getenv(nexusNXSCommandPathEnvName)); override != "" {
		return override
	}
	return ""
}

func nxsExecutableName(goos string) string {
	if goos == "windows" {
		return "nxs.exe"
	}
	return "nxs"
}

func claudeCommandNames(goos string) []string {
	if goos == "windows" {
		// Windows 的 npm 全局安装通常只提供 claude.cmd/claude.ps1，默认查 claude.exe 会漏掉它。
		return []string{"claude.exe", "claude.cmd", "claude.ps1", "claude"}
	}
	return []string{"claude"}
}

func knownClaudeCommandPaths(goos string, getenv func(string) string) []string {
	switch goos {
	case "windows":
		return knownWindowsClaudeCommandPaths(getenv)
	case "darwin":
		return knownDarwinClaudeCommandPaths(getenv)
	default:
		candidates := []string{
			"/usr/local/bin/claude",
			"/usr/bin/claude",
			"/home/linuxbrew/.linuxbrew/bin/claude",
		}
		if homebrewPrefix := strings.TrimSpace(getenv("HOMEBREW_PREFIX")); homebrewPrefix != "" {
			candidates = append([]string{filepath.Join(homebrewPrefix, "bin", "claude")}, candidates...)
		}
		return knownUnixClaudeCommandPaths(getenv, candidates)
	}
}

func knownWindowsClaudeCommandPaths(getenv func(string) string) []string {
	candidates := []string{}
	if appData := strings.TrimSpace(getenv("APPDATA")); appData != "" {
		candidates = appendWindowsClaudeNames(candidates, filepath.Join(appData, "npm"))
	}
	if userProfile := strings.TrimSpace(getenv("USERPROFILE")); userProfile != "" {
		candidates = appendWindowsClaudeNames(candidates, filepath.Join(userProfile, ".local", "bin"))
		candidates = appendWindowsClaudeNames(candidates, filepath.Join(userProfile, ".claude", "local"))
		candidates = appendWindowsClaudeNames(candidates, filepath.Join(userProfile, "node_modules", ".bin"))
	}
	return candidates
}

func knownDarwinClaudeCommandPaths(getenv func(string) string) []string {
	candidates := []string{
		"/opt/homebrew/bin/claude",
		"/usr/local/bin/claude",
	}
	if homebrewPrefix := strings.TrimSpace(getenv("HOMEBREW_PREFIX")); homebrewPrefix != "" {
		candidates = append([]string{filepath.Join(homebrewPrefix, "bin", "claude")}, candidates...)
	}
	candidates = append(candidates, knownUserClaudeCommandPaths(getenv)...)
	if home := strings.TrimSpace(getenv("HOME")); home != "" {
		candidates = append(candidates, filepath.Join(home, "Library", "pnpm", "claude"))
	}
	candidates = append(candidates, knownPackageManagerClaudeCommandPaths(getenv)...)
	return compactClaudeCommandCandidates(candidates)
}

func knownUnixClaudeCommandPaths(getenv func(string) string, systemCandidates []string) []string {
	candidates := append([]string(nil), systemCandidates...)
	candidates = append(candidates, knownUserClaudeCommandPaths(getenv)...)
	candidates = append(candidates, knownPackageManagerClaudeCommandPaths(getenv)...)
	return compactClaudeCommandCandidates(candidates)
}

func knownUserClaudeCommandPaths(getenv func(string) string) []string {
	home := strings.TrimSpace(getenv("HOME"))
	if home == "" {
		return nil
	}
	return []string{
		filepath.Join(home, ".local", "bin", "claude"),
		filepath.Join(home, ".claude", "local", "claude"),
		filepath.Join(home, ".npm-global", "bin", "claude"),
		filepath.Join(home, ".volta", "bin", "claude"),
		filepath.Join(home, ".asdf", "shims", "claude"),
		filepath.Join(home, ".local", "share", "mise", "shims", "claude"),
		filepath.Join(home, ".local", "share", "pnpm", "claude"),
	}
}

func knownPackageManagerClaudeCommandPaths(getenv func(string) string) []string {
	candidates := []string{}
	if nvmBin := strings.TrimSpace(getenv("NVM_BIN")); nvmBin != "" {
		candidates = append(candidates, filepath.Join(nvmBin, "claude"))
	}
	if fnmMultishellPath := strings.TrimSpace(getenv("FNM_MULTISHELL_PATH")); fnmMultishellPath != "" {
		candidates = append(candidates, filepath.Join(fnmMultishellPath, "bin", "claude"))
	}
	if npmPrefix := strings.TrimSpace(getenv("NPM_CONFIG_PREFIX")); npmPrefix != "" {
		candidates = append(candidates, filepath.Join(npmPrefix, "bin", "claude"))
	}
	if pnpmHome := strings.TrimSpace(getenv("PNPM_HOME")); pnpmHome != "" {
		candidates = append(candidates, filepath.Join(pnpmHome, "claude"))
	}
	if voltaHome := strings.TrimSpace(getenv("VOLTA_HOME")); voltaHome != "" {
		candidates = append(candidates, filepath.Join(voltaHome, "bin", "claude"))
	}
	if asdfDataDir := strings.TrimSpace(getenv("ASDF_DATA_DIR")); asdfDataDir != "" {
		candidates = append(candidates, filepath.Join(asdfDataDir, "shims", "claude"))
	}
	if miseDataDir := strings.TrimSpace(getenv("MISE_DATA_DIR")); miseDataDir != "" {
		candidates = append(candidates, filepath.Join(miseDataDir, "shims", "claude"))
	}
	return candidates
}

func knownClaudeCommandPathGlobs(
	goos string,
	getenv func(string) string,
	globPaths func(string) ([]string, error),
) []string {
	if goos == "windows" || globPaths == nil {
		return nil
	}
	patterns := []string{}
	if home := strings.TrimSpace(getenv("HOME")); home != "" {
		patterns = append(patterns,
			filepath.Join(home, ".nvm", "versions", "node", "*", "bin", "claude"),
			filepath.Join(home, ".fnm", "node-versions", "*", "installation", "bin", "claude"),
		)
	}
	if nvmDir := strings.TrimSpace(getenv("NVM_DIR")); nvmDir != "" {
		patterns = append(patterns, filepath.Join(nvmDir, "versions", "node", "*", "bin", "claude"))
	}
	if fnmDir := strings.TrimSpace(getenv("FNM_DIR")); fnmDir != "" {
		patterns = append(patterns, filepath.Join(fnmDir, "node-versions", "*", "installation", "bin", "claude"))
	}
	candidates := []string{}
	for _, pattern := range compactClaudeCommandCandidates(patterns) {
		matches, err := globPaths(pattern)
		if err != nil {
			continue
		}
		candidates = append(candidates, matches...)
	}
	return compactClaudeCommandCandidates(candidates)
}

func appendWindowsClaudeNames(candidates []string, directory string) []string {
	return append(candidates,
		filepath.Join(directory, "claude.exe"),
		filepath.Join(directory, "claude.cmd"),
		filepath.Join(directory, "claude.ps1"),
		filepath.Join(directory, "claude"),
	)
}

func compactClaudeCommandCandidates(candidates []string) []string {
	result := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		normalized := strings.TrimSpace(candidate)
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
