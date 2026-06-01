package clientopts

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const nexusClaudeCommandPathEnvName = "NEXUS_CLAUDE_COMMAND_PATH"

type claudeCommandConfig struct {
	CLIPath          string
	Executable       string
	PathToExecutable string
}

func processCLICommandConfig() claudeCommandConfig {
	return resolveClaudeCommandConfigWith(
		runtime.GOOS,
		os.Getenv,
		exec.LookPath,
		func(path string) bool {
			info, err := os.Stat(path)
			return err == nil && !info.IsDir()
		},
	)
}

func resolveClaudeCommandConfigWith(
	goos string,
	getenv func(string) string,
	lookPath func(string) (string, error),
	fileExists func(string) bool,
) claudeCommandConfig {
	commandPath := resolveClaudeCommandPathWith(goos, getenv, lookPath, fileExists)
	if goos != "windows" || strings.TrimSpace(commandPath) == "" {
		return claudeCommandConfig{CLIPath: commandPath}
	}
	if config, ok := windowsNodeClaudeCommandConfig(commandPath, lookPath, fileExists); ok {
		return config
	}
	return claudeCommandConfig{CLIPath: commandPath}
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

	// Windows 的 npm 全局安装通常只提供 claude.cmd/claude.ps1，默认查 claude.exe 会漏掉它。
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

func windowsNodeClaudeCommandConfig(
	commandPath string,
	lookPath func(string) (string, error),
	fileExists func(string) bool,
) (claudeCommandConfig, bool) {
	extension := strings.ToLower(filepath.Ext(commandPath))
	if extension != ".cmd" && extension != ".bat" && extension != ".ps1" {
		return claudeCommandConfig{}, false
	}
	scriptPath := windowsClaudeScriptPath(commandPath, fileExists)
	if scriptPath == "" {
		return claudeCommandConfig{}, false
	}
	return claudeCommandConfig{
		Executable:       windowsNodeExecutable(commandPath, lookPath, fileExists),
		PathToExecutable: scriptPath,
	}, true
}

func windowsClaudeScriptPath(commandPath string, fileExists func(string) bool) string {
	directory := filepath.Dir(commandPath)
	candidates := []string{
		filepath.Join(directory, "node_modules", "@anthropic-ai", "claude-code", "cli.js"),
		filepath.Join(directory, "node_modules", "@anthropic-ai", "claude-code", "cli.mjs"),
		filepath.Join(directory, "..", "@anthropic-ai", "claude-code", "cli.js"),
		filepath.Join(directory, "..", "@anthropic-ai", "claude-code", "cli.mjs"),
	}
	for _, candidate := range candidates {
		cleanCandidate := filepath.Clean(candidate)
		if fileExists(cleanCandidate) {
			return cleanCandidate
		}
	}
	return ""
}

func windowsNodeExecutable(commandPath string, lookPath func(string) (string, error), fileExists func(string) bool) string {
	if localNode := filepath.Join(filepath.Dir(commandPath), "node.exe"); fileExists(localNode) {
		return localNode
	}
	for _, name := range []string{"node.exe", "node"} {
		if path, err := lookPath(name); err == nil && strings.TrimSpace(path) != "" {
			return path
		}
	}
	return "node"
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
