package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
)

const nexusctlCommandPathEnvName = "NEXUSCTL_COMMAND_PATH"

type nexusctlShimTarget struct {
	Kind        string
	CommandPath string
	ProjectRoot string
}

func ensureNexusctlShim(binDir string, context map[string]string) error {
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return err
	}
	target, err := resolveNexusctlShimTarget(binDir, context["project_root"])
	if err != nil {
		return err
	}
	content, err := renderNexusctlShellShim(target)
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(binDir, "nexusctl"), []byte(content), 0o755); err != nil {
		return err
	}
	cmdContent, err := renderNexusctlWindowsShim(target)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(binDir, "nexusctl.cmd"), []byte(cmdContent), 0o755)
}

func resolveNexusctlShimTarget(binDir string, projectRoot string) (nexusctlShimTarget, error) {
	root := filepath.Clean(strings.TrimSpace(projectRoot))
	if commandPath := strings.TrimSpace(os.Getenv(nexusctlCommandPathEnvName)); commandPath != "" &&
		!samePath(commandPath, filepath.Join(binDir, "nexusctl")) &&
		!samePath(commandPath, filepath.Join(binDir, "nexusctl.cmd")) {
		if err := validateNexusctlExecutable(commandPath); err != nil {
			return nexusctlShimTarget{}, err
		}
		return nexusctlShimTarget{Kind: "executable", CommandPath: filepath.Clean(commandPath)}, nil
	}
	sourceEntry := filepath.Join(root, "cmd", "nexusctl", "main.go")
	if _, err := os.Stat(sourceEntry); err == nil {
		return nexusctlShimTarget{Kind: "source", ProjectRoot: root}, nil
	} else if err != nil && !os.IsNotExist(err) {
		return nexusctlShimTarget{}, err
	}
	for _, candidate := range packagedNexusctlCandidates(root) {
		if err := validateNexusctlExecutable(candidate); err == nil {
			return nexusctlShimTarget{Kind: "executable", CommandPath: filepath.Clean(candidate)}, nil
		} else if err != nil && !os.IsNotExist(err) {
			return nexusctlShimTarget{}, err
		}
	}
	return nexusctlShimTarget{}, fmt.Errorf(
		"nexusctl command path is required: set %s or provide cmd/nexusctl/main.go under %s",
		nexusctlCommandPathEnvName,
		root,
	)
}

func packagedNexusctlCandidates(root string) []string {
	if runtime.GOOS == "windows" {
		return []string{filepath.Join(root, "bin", "nexusctl.exe")}
	}
	return []string{filepath.Join(root, "bin", "nexusctl")}
}

func validateNexusctlExecutable(commandPath string) error {
	cleanPath := filepath.Clean(strings.TrimSpace(commandPath))
	info, err := os.Stat(cleanPath)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%s 指向目录，不是 nexusctl 可执行文件", cleanPath)
	}
	if runtime.GOOS != "windows" && info.Mode()&0o111 == 0 {
		return fmt.Errorf("%s 不可执行", cleanPath)
	}
	return nil
}

func renderNexusctlShellShim(target nexusctlShimTarget) (string, error) {
	switch target.Kind {
	case "source":
		return `#!/bin/sh
set -eu

CALLER_CWD="$(pwd)"
export NEXUSCTL_WORKSPACE_PATH="${NEXUSCTL_WORKSPACE_PATH:-$CALLER_CWD}"

cd ` + shellSingleQuote(target.ProjectRoot) + `
exec go run ./cmd/nexusctl "$@"
`, nil
	case "executable":
		return `#!/bin/sh
set -eu

CALLER_CWD="$(pwd)"
export NEXUSCTL_WORKSPACE_PATH="${NEXUSCTL_WORKSPACE_PATH:-$CALLER_CWD}"

exec ` + shellSingleQuote(target.CommandPath) + ` "$@"
`, nil
	default:
		return "", fmt.Errorf("未知 nexusctl shim 类型: %s", target.Kind)
	}
}

func renderNexusctlWindowsShim(target nexusctlShimTarget) (string, error) {
	switch target.Kind {
	case "source":
		return `@echo off
setlocal

set "CALLER_CWD=%CD%"
if "%NEXUSCTL_WORKSPACE_PATH%"=="" set "NEXUSCTL_WORKSPACE_PATH=%CALLER_CWD%"

cd /d "` + windowsBatchValue(target.ProjectRoot) + `"
go run ./cmd/nexusctl %*
exit /b %ERRORLEVEL%
`, nil
	case "executable":
		return `@echo off
setlocal

set "CALLER_CWD=%CD%"
if "%NEXUSCTL_WORKSPACE_PATH%"=="" set "NEXUSCTL_WORKSPACE_PATH=%CALLER_CWD%"

"` + windowsBatchValue(target.CommandPath) + `" %*
exit /b %ERRORLEVEL%
`, nil
	default:
		return "", fmt.Errorf("未知 nexusctl shim 类型: %s", target.Kind)
	}
}

func removeWorkspaceBinShim(workspacePath string) error {
	// TODO: 迁移期清理旧 per-agent / per-owner nexusctl shim；确认旧版本用户已覆盖后删除。
	root := filepath.Clean(strings.TrimSpace(workspacePath))
	for _, binDir := range []string{
		filepath.Join(root, ".agents", "bin"),
		filepath.Join(filepath.Dir(root), ".agents", "bin"),
	} {
		if err := removeGeneratedNexusctlBinDir(binDir); err != nil {
			return err
		}
	}
	return nil
}

func removeGeneratedNexusctlBinDir(binDir string) error {
	if filepath.Clean(binDir) == filepath.Clean(appfs.AgentRuntimeBinDir()) {
		return nil
	}
	for _, fileName := range []string{"nexusctl", "nexusctl.cmd"} {
		targetPath := filepath.Join(binDir, fileName)
		content, err := os.ReadFile(targetPath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if !looksLikeGeneratedNexusctlShim(string(content)) {
			continue
		}
		if err = os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return removeDirIfEmpty(binDir)
}

func looksLikeGeneratedNexusctlShim(content string) bool {
	return strings.Contains(content, "NEXUSCTL_WORKSPACE_PATH") &&
		(strings.Contains(content, "go run ./cmd/nexusctl") ||
			strings.Contains(content, "nexusctl is unavailable: set NEXUS_PROJECT_ROOT or install nexusctl") ||
			strings.Contains(content, "exit /b %ERRORLEVEL%"))
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func windowsBatchValue(value string) string {
	return strings.ReplaceAll(value, "%", "%%")
}

func samePath(left string, right string) bool {
	return filepath.Clean(strings.TrimSpace(left)) == filepath.Clean(strings.TrimSpace(right))
}
