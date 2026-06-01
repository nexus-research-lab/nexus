package clientopts

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveClaudeCommandPathUsesOverride(t *testing.T) {
	expected := `D:\tools\claude.exe`
	got := resolveClaudeCommandPathWith(
		"windows",
		fakeEnv(map[string]string{nexusClaudeCommandPathEnvName: expected}),
		func(string) (string, error) { return "", os.ErrNotExist },
		func(string) bool { return false },
	)
	if got != expected {
		t.Fatalf("NEXUS_CLAUDE_COMMAND_PATH override 未生效: got=%q want=%q", got, expected)
	}
}

func TestResolveClaudeCommandPathUsesWindowsNpmShim(t *testing.T) {
	appData := `C:\Users\lee\AppData\Roaming`
	expected := filepath.Join(appData, "npm", "claude.cmd")
	got := resolveClaudeCommandPathWith(
		"windows",
		fakeEnv(map[string]string{"APPDATA": appData}),
		func(string) (string, error) { return "", os.ErrNotExist },
		func(path string) bool { return path == expected },
	)
	if got != expected {
		t.Fatalf("Windows npm claude.cmd 未被识别: got=%q want=%q", got, expected)
	}
}

func TestResolveClaudeCommandConfigBypassesWindowsNpmShim(t *testing.T) {
	appData := `C:\Users\lee\AppData\Roaming`
	shimPath := filepath.Join(appData, "npm", "claude.cmd")
	scriptPath := filepath.Clean(filepath.Join(appData, "npm", "node_modules", "@anthropic-ai", "claude-code", "cli.js"))
	nodePath := `C:\Program Files\nodejs\node.exe`
	got := resolveClaudeCommandConfigWith(
		"windows",
		fakeEnv(map[string]string{"APPDATA": appData}),
		func(name string) (string, error) {
			switch name {
			case "claude.cmd":
				return shimPath, nil
			case "node.exe":
				return nodePath, nil
			default:
				return "", os.ErrNotExist
			}
		},
		func(path string) bool {
			return path == shimPath || path == scriptPath
		},
	)
	if got.CLIPath != "" {
		t.Fatalf("Windows npm shim 应切换到 node 启动，不应继续走 CLIPath: %+v", got)
	}
	if got.Executable != nodePath {
		t.Fatalf("node executable 不正确: got=%q want=%q", got.Executable, nodePath)
	}
	if got.PathToExecutable != scriptPath {
		t.Fatalf("Claude Code script path 不正确: got=%q want=%q", got.PathToExecutable, scriptPath)
	}
}

func TestResolveClaudeCommandConfigFallsBackWhenShimScriptMissing(t *testing.T) {
	appData := `C:\Users\lee\AppData\Roaming`
	shimPath := filepath.Join(appData, "npm", "claude.cmd")
	got := resolveClaudeCommandConfigWith(
		"windows",
		fakeEnv(map[string]string{"APPDATA": appData}),
		func(name string) (string, error) {
			if name == "claude.cmd" {
				return shimPath, nil
			}
			return "", os.ErrNotExist
		},
		func(path string) bool {
			return path == shimPath
		},
	)
	if got.CLIPath != shimPath || got.Executable != "" || got.PathToExecutable != "" {
		t.Fatalf("找不到 npm script 时应回退到原 CLIPath: %+v", got)
	}
}

func TestResolveClaudeCommandPathPrefersLookPath(t *testing.T) {
	expected := `C:\Users\lee\AppData\Roaming\npm\claude.cmd`
	got := resolveClaudeCommandPathWith(
		"windows",
		fakeEnv(nil),
		func(name string) (string, error) {
			if name == "claude.cmd" {
				return expected, nil
			}
			return "", os.ErrNotExist
		},
		func(string) bool { return false },
	)
	if got != expected {
		t.Fatalf("PATH 中的 claude.cmd 未被优先识别: got=%q want=%q", got, expected)
	}
}

func TestResolveClaudeCommandPathNonWindowsDefersToSDK(t *testing.T) {
	got := resolveClaudeCommandPathWith(
		"linux",
		fakeEnv(nil),
		func(string) (string, error) { return "/usr/local/bin/claude", nil },
		func(string) bool { return true },
	)
	if got != "" {
		t.Fatalf("非 Windows 应继续交给 SDK 默认解析: got=%q", got)
	}
}

func fakeEnv(values map[string]string) func(string) string {
	return func(key string) string {
		return values[key]
	}
}
