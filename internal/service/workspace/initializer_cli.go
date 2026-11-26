package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
)

const (
	nexusctlCommandPathEnvName = "NEXUSCTL_COMMAND_PATH"
	nexuscfgCommandPathEnvName = "NEXUSCFG_COMMAND_PATH"
)

type runtimeCLIShimDefinition struct {
	Name               string
	CommandPathEnvName string
}

var runtimeCLIShimDefinitions = []runtimeCLIShimDefinition{
	{Name: "nexusctl", CommandPathEnvName: nexusctlCommandPathEnvName},
	{Name: "nexuscfg", CommandPathEnvName: nexuscfgCommandPathEnvName},
}

var runtimeCLIShimMu sync.Mutex

type runtimeCLIShimTarget struct {
	Kind        string
	CommandPath string
	ProjectRoot string
	Arguments   []string
}

// EnsureRuntimeCLICommands refreshes host control shims during startup。
func EnsureRuntimeCLICommands() error {
	return ensureRuntimeCLIShims(appfs.AgentRuntimeBinDir(), map[string]string{
		"project_root": appfs.Root(),
	})
}

func ensureRuntimeCLIShims(binDir string, context map[string]string) error {
	runtimeCLIShimMu.Lock()
	defer runtimeCLIShimMu.Unlock()
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
	for _, candidate := range packagedRuntimeCLICandidates(root, definition.Name) {
		if err := validateRuntimeCLIExecutable(candidate, definition.Name); err == nil {
			return runtimeCLIShimTarget{Kind: "executable", CommandPath: filepath.Clean(candidate)}, nil
		} else if !os.IsNotExist(err) {
			return runtimeCLIShimTarget{}, err
		}
	}
	sourceEntry := filepath.Join(root, "cmd", definition.Name, "main.go")
	if _, err := os.Stat(sourceEntry); err == nil {
		return runtimeCLIShimTarget{Kind: "source", ProjectRoot: root}, nil
	} else if !os.IsNotExist(err) {
		return runtimeCLIShimTarget{}, err
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
		arguments := renderRuntimeCLIArguments(target.Arguments, shellSingleQuote)
		return `#!/bin/sh
set -eu

CALLER_CWD="$(pwd)"
export NEXUSCTL_WORKSPACE_PATH="${NEXUSCTL_WORKSPACE_PATH:-$CALLER_CWD}"

exec ` + shellSingleQuote(target.CommandPath) + arguments + ` "$@"
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
		arguments := renderRuntimeCLIArguments(target.Arguments, func(value string) string {
			return `"` + windowsBatchValue(value) + `"`
		})
		return `@echo off
setlocal

set "CALLER_CWD=%CD%"
if "%NEXUSCTL_WORKSPACE_PATH%"=="" set "NEXUSCTL_WORKSPACE_PATH=%CALLER_CWD%"

"` + windowsBatchValue(target.CommandPath) + `"` + arguments + ` %*
exit /b %ERRORLEVEL%
`, nil
	default:
		return "", fmt.Errorf("未知 %s shim 类型: %s", name, target.Kind)
	}
}

func renderRuntimeCLIArguments(arguments []string, quote func(string) string) string {
	if len(arguments) == 0 {
		return ""
	}
	encoded := make([]string, 0, len(arguments))
	for _, argument := range arguments {
		encoded = append(encoded, quote(argument))
	}
	return " " + strings.Join(encoded, " ")
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
