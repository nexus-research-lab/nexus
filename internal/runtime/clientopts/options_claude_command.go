package clientopts

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
)

const nexusClaudeCommandPathEnvName = "NEXUS_CLAUDE_COMMAND_PATH"

func processBackendOptions() agentclient.ProcessOptions {
	return agentclient.ProcessOptions{
		CommandPath: resolveClaudeCommandPath(),
	}
}

func resolveClaudeCommandPath() string {
	return resolveClaudeCommandPathWith(
		runtime.GOOS,
		os.Getenv,
		exec.LookPath,
		func(path string) bool {
			info, err := os.Stat(path)
			return err == nil && !info.IsDir()
		},
	)
}

func resolveClaudeCommandPathWith(
	goos string,
	getenv func(string) string,
	lookPath func(string) (string, error),
	fileExists func(string) bool,
) string {
	if override := strings.TrimSpace(getenv(nexusClaudeCommandPathEnvName)); override != "" {
		return override
	}
	if goos != "windows" {
		return ""
	}

	// Windows 的 npm 全局安装通常只提供 claude.cmd/claude.ps1，SDK v0.1.0 默认查 claude.exe 会漏掉它。
	for _, name := range []string{"claude.exe", "claude.cmd", "claude.ps1", "claude"} {
		if path, err := lookPath(name); err == nil && strings.TrimSpace(path) != "" {
			return path
		}
	}
	for _, candidate := range knownWindowsClaudeCommandPaths(getenv) {
		if fileExists(candidate) {
			return candidate
		}
	}
	return ""
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

func appendWindowsClaudeNames(candidates []string, directory string) []string {
	return append(candidates,
		filepath.Join(directory, "claude.exe"),
		filepath.Join(directory, "claude.cmd"),
		filepath.Join(directory, "claude.ps1"),
		filepath.Join(directory, "claude"),
	)
}
