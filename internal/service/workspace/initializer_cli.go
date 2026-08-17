package workspace

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

const (
	nexusctlCommandPathEnvName = "NEXUSCTL_COMMAND_PATH"
	nexuscfgCommandPathEnvName = "NEXUSCFG_COMMAND_PATH"
	nexusCommandPathEnvName    = "NEXUS_COMMAND_PATH"
)

type runtimeCLIShimDefinition struct {
	Name               string
	CommandPathEnvName string
}

var runtimeCLIShimDefinitions = []runtimeCLIShimDefinition{
	{Name: "nexusctl", CommandPathEnvName: nexusctlCommandPathEnvName},
	{Name: "nexuscfg", CommandPathEnvName: nexuscfgCommandPathEnvName},
	{Name: "nexus", CommandPathEnvName: nexusCommandPathEnvName},
}

type runtimeCLIShimTarget struct {
	Kind        string
	CommandPath string
	ProjectRoot string
}

func ensureRuntimeCLIShims(binDir string, context map[string]string) error {
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return err
	}
	root, err := confinedfs.Open(binDir)
	if err != nil {
		return err
	}
	defer root.Close()
	for _, definition := range runtimeCLIShimDefinitions {
		target, resolveErr := resolveRuntimeCLIShimTarget(
			binDir,
			context["project_root"],
			definition,
		)
		if resolveErr != nil {
			return resolveErr
		}
		content, renderErr := renderRuntimeCLIShellShim(target, definition.Name)
		if renderErr != nil {
			return renderErr
		}
		if err = root.WriteFileAtomic(definition.Name, []byte(content), 0o755); err != nil {
			return err
		}
		cmdContent, renderErr := renderRuntimeCLIWindowsShim(target, definition.Name)
		if renderErr != nil {
			return renderErr
		}
		if err = root.WriteFileAtomic(definition.Name+".cmd", []byte(cmdContent), 0o755); err != nil {
			return err
		}
	}
	return nil
}

func resolveRuntimeCLIShimTarget(
	binDir string,
	projectRoot string,
	definition runtimeCLIShimDefinition,
) (runtimeCLIShimTarget, error) {
	root := filepath.Clean(strings.TrimSpace(projectRoot))
	if commandPath := strings.TrimSpace(os.Getenv(definition.CommandPathEnvName)); commandPath != "" &&
		!samePath(commandPath, filepath.Join(binDir, definition.Name)) &&
		!samePath(commandPath, filepath.Join(binDir, definition.Name+".cmd")) {
		if err := validateRuntimeCLIExecutable(commandPath, definition.Name); err != nil {
			return runtimeCLIShimTarget{}, err
		}
		return runtimeCLIShimTarget{Kind: "executable", CommandPath: filepath.Clean(commandPath)}, nil
	}
	sourceEntry := filepath.Join(root, "cmd", definition.Name, "main.go")
	if _, err := os.Stat(sourceEntry); err == nil {
		return runtimeCLIShimTarget{Kind: "source", ProjectRoot: root}, nil
	} else if err != nil && !os.IsNotExist(err) {
		return runtimeCLIShimTarget{}, err
	}
	for _, candidate := range packagedRuntimeCLICandidates(root, definition.Name) {
		if err := validateRuntimeCLIExecutable(candidate, definition.Name); err == nil {
			return runtimeCLIShimTarget{Kind: "executable", CommandPath: filepath.Clean(candidate)}, nil
		} else if err != nil && !os.IsNotExist(err) {
			return runtimeCLIShimTarget{}, err
		}
	}
	return runtimeCLIShimTarget{}, fmt.Errorf(
		"%s command path is required: set %s or provide cmd/%s/main.go under %s",
		definition.Name,
		definition.CommandPathEnvName,
		definition.Name,
		root,
	)
}

func packagedRuntimeCLICandidates(root string, name string) []string {
	if runtime.GOOS == "windows" {
		return []string{filepath.Join(root, "bin", name+".exe")}
	}
	return []string{filepath.Join(root, "bin", name)}
}

func validateRuntimeCLIExecutable(commandPath string, name string) error {
	cleanPath := filepath.Clean(strings.TrimSpace(commandPath))
	info, err := os.Stat(cleanPath)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return fmt.Errorf("%s 指向目录，不是 %s 可执行文件", cleanPath, name)
	}
	if runtime.GOOS != "windows" && info.Mode()&0o111 == 0 {
		return fmt.Errorf("%s 不可执行", cleanPath)
	}
	return nil
}

func renderRuntimeCLIShellShim(target runtimeCLIShimTarget, name string) (string, error) {
	switch target.Kind {
	case "source":
		return `#!/bin/sh
set -eu

CALLER_CWD="$(pwd)"
export NEXUSCTL_WORKSPACE_PATH="${NEXUSCTL_WORKSPACE_PATH:-$CALLER_CWD}"

cd ` + shellSingleQuote(target.ProjectRoot) + `
exec go run ./cmd/` + name + ` "$@"
`, nil
	case "executable":
		return `#!/bin/sh
set -eu

CALLER_CWD="$(pwd)"
export NEXUSCTL_WORKSPACE_PATH="${NEXUSCTL_WORKSPACE_PATH:-$CALLER_CWD}"

exec ` + shellSingleQuote(target.CommandPath) + ` "$@"
`, nil
	default:
		return "", fmt.Errorf("未知 %s shim 类型: %s", name, target.Kind)
	}
}

func renderRuntimeCLIWindowsShim(target runtimeCLIShimTarget, name string) (string, error) {
	switch target.Kind {
	case "source":
		return `@echo off
setlocal

set "CALLER_CWD=%CD%"
if "%NEXUSCTL_WORKSPACE_PATH%"=="" set "NEXUSCTL_WORKSPACE_PATH=%CALLER_CWD%"

cd /d "` + windowsBatchValue(target.ProjectRoot) + `"
go run ./cmd/` + name + ` %*
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
		return "", fmt.Errorf("未知 %s shim 类型: %s", name, target.Kind)
	}
}

func removeWorkspaceBinShim(root *confinedfs.Root) error {
	binRoot, err := root.OpenRootNoSymlink(".agents/bin")
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	closed := false
	defer func() {
		if !closed {
			_ = binRoot.Close()
		}
	}()
	for _, fileName := range []string{"nexusctl", "nexusctl.cmd", "nexuscfg", "nexuscfg.cmd", "nexus", "nexus.cmd"} {
		info, statErr := binRoot.Lstat(fileName)
		if os.IsNotExist(statErr) {
			continue
		}
		if statErr != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			continue
		}
		file, openErr := binRoot.OpenFileNoSymlink(fileName, os.O_RDONLY, 0)
		if os.IsNotExist(openErr) {
			continue
		}
		if openErr != nil {
			if errors.Is(openErr, confinedfs.ErrSymlink) || errors.Is(openErr, confinedfs.ErrHardlink) {
				continue
			}
			return openErr
		}
		content, readErr := io.ReadAll(file)
		closeErr := file.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		if !looksLikeGeneratedRuntimeCLIShim(string(content)) {
			continue
		}
		if err = binRoot.Remove(fileName); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	entries, err := fs.ReadDir(binRoot.FS(), ".")
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return nil
	}
	if err = binRoot.Close(); err != nil {
		return err
	}
	closed = true
	return root.Remove(".agents/bin")
}

func looksLikeGeneratedRuntimeCLIShim(content string) bool {
	return strings.Contains(content, "NEXUSCTL_WORKSPACE_PATH") &&
		(strings.Contains(content, "go run ./cmd/nexusctl") ||
			strings.Contains(content, "go run ./cmd/nexuscfg") ||
			strings.Contains(content, "go run ./cmd/nexus") ||
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
