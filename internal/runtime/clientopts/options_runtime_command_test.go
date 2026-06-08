package clientopts

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveRuntimeCommandPathUsesClaudeOverride(t *testing.T) {
	expected := `D:\tools\claude.exe`
	got := resolveRuntimeCommandPathWith(
		runtimeKindClaude,
		"windows",
		fakeEnv(map[string]string{nexusClaudeCommandPathEnvName: expected}),
		func(string) (string, error) { return "", os.ErrNotExist },
		func(string) bool { return false },
		fakeGlob(nil),
	)
	if got != expected {
		t.Fatalf("NEXUS_CLAUDE_COMMAND_PATH override 未生效: got=%q want=%q", got, expected)
	}
}

func TestResolveRuntimeCommandPathUsesWindowsNpmShim(t *testing.T) {
	appData := `C:\Users\lee\AppData\Roaming`
	expected := filepath.Join(appData, "npm", "claude.cmd")
	got := resolveRuntimeCommandPathWith(
		runtimeKindClaude,
		"windows",
		fakeEnv(map[string]string{"APPDATA": appData}),
		func(string) (string, error) { return "", os.ErrNotExist },
		func(path string) bool { return path == expected },
		fakeGlob(nil),
	)
	if got != expected {
		t.Fatalf("Windows npm claude.cmd 未被识别: got=%q want=%q", got, expected)
	}
}

func TestResolveRuntimeCommandConfigKeepsWindowsNpmShim(t *testing.T) {
	appData := `C:\Users\lee\AppData\Roaming`
	shimPath := filepath.Join(appData, "npm", "claude.cmd")
	got := resolveRuntimeCommandConfigWith(
		runtimeKindClaude,
		"windows",
		fakeEnv(map[string]string{"APPDATA": appData}),
		func(name string) (string, error) {
			switch name {
			case "claude.cmd":
				return shimPath, nil
			default:
				return "", os.ErrNotExist
			}
		},
		func(path string) bool {
			return path == shimPath
		},
		fakeGlob(nil),
	)
	if got.CLIPath != shimPath || got.Executable != "" || got.PathToExecutable != "" {
		t.Fatalf("Windows npm shim 应按官方入口原样启动: %+v", got)
	}
}

func TestResolveRuntimeCommandConfigKeepsWindowsNpmShimWhenScriptMissing(t *testing.T) {
	appData := `C:\Users\lee\AppData\Roaming`
	shimPath := filepath.Join(appData, "npm", "claude.cmd")
	got := resolveRuntimeCommandConfigWith(
		runtimeKindClaude,
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
		fakeGlob(nil),
	)
	if got.CLIPath != shimPath || got.Executable != "" || got.PathToExecutable != "" {
		t.Fatalf("找不到 npm script 时应回退到原 CLIPath: %+v", got)
	}
}

func TestResolveRuntimeCommandPathDefersNXSRuntimeKindToBridge(t *testing.T) {
	got := resolveRuntimeCommandPathWith(
		runtimeKindNXS,
		"darwin",
		fakeEnv(nil),
		func(name string) (string, error) {
			if name == "nxs" {
				return "/Users/lee/bin/nxs", nil
			}
			return "", os.ErrNotExist
		},
		func(string) bool { return false },
		fakeGlob(nil),
	)
	if got != "" {
		t.Fatalf("没有预置 nxs 时应交给 bridge 解析: got=%q", got)
	}
}

func TestResolveRuntimeCommandPathDefaultsToNXSRuntimeKind(t *testing.T) {
	got := resolveRuntimeCommandPathWith(
		"",
		"darwin",
		fakeEnv(nil),
		func(name string) (string, error) {
			if name == "claude" {
				return "/opt/homebrew/bin/claude", nil
			}
			return "", os.ErrNotExist
		},
		func(string) bool { return false },
		fakeGlob(nil),
	)
	if got != "" {
		t.Fatalf("空 runtime kind 默认应走 nxs 并交给 bridge 解析: got=%q", got)
	}
}

func TestResolveRuntimeCommandPathUsesAppRootNXSRuntime(t *testing.T) {
	expected := "/opt/app/bin/nxs"
	got := resolveRuntimeCommandPathWith(
		runtimeKindNXS,
		"linux",
		fakeEnv(map[string]string{nexusAppRootEnvName: "/opt/app"}),
		func(string) (string, error) { return "", os.ErrNotExist },
		func(path string) bool { return path == expected },
		fakeGlob(nil),
	)
	if got != expected {
		t.Fatalf("NEXUS_APP_ROOT 预置 nxs 未生效: got=%q want=%q", got, expected)
	}
}

func TestResolveRuntimeCommandPathUsesWindowsAppRootNXSRuntime(t *testing.T) {
	expected := filepath.Join(`C:\Nexus\Resources`, "bin", "nxs.exe")
	got := resolveRuntimeCommandPathWith(
		runtimeKindNXS,
		"windows",
		fakeEnv(map[string]string{nexusAppRootEnvName: `C:\Nexus\Resources`}),
		func(string) (string, error) { return "", os.ErrNotExist },
		func(path string) bool { return path == expected },
		fakeGlob(nil),
	)
	if got != expected {
		t.Fatalf("Windows NEXUS_APP_ROOT 预置 nxs 未生效: got=%q want=%q", got, expected)
	}
}

func TestResolveRuntimeCommandPathUsesNXSOverride(t *testing.T) {
	expected := "/tmp/custom-nxs"
	got := resolveRuntimeCommandPathWith(
		runtimeKindNXS,
		"darwin",
		fakeEnv(map[string]string{nexusNXSCommandPathEnvName: expected}),
		func(string) (string, error) { return "", os.ErrNotExist },
		func(string) bool { return false },
		fakeGlob(nil),
	)
	if got != expected {
		t.Fatalf("NEXUS_NXS_COMMAND_PATH override 未生效: got=%q want=%q", got, expected)
	}
}

func TestResolveRuntimeCommandPathPrefersAppRootNXSRuntimeOverOverride(t *testing.T) {
	expected := filepath.Join(`C:\Nexus\Resources`, "bin", "nxs.exe")
	got := resolveRuntimeCommandPathWith(
		runtimeKindNXS,
		"windows",
		fakeEnv(map[string]string{
			nexusAppRootEnvName:        `C:\Nexus\Resources`,
			nexusNXSCommandPathEnvName: `C:\Stale\nxs.exe`,
		}),
		func(string) (string, error) { return "", os.ErrNotExist },
		func(path string) bool { return path == expected },
		fakeGlob(nil),
	)
	if got != expected {
		t.Fatalf("包内 nxs 应优先于 NEXUS_NXS_COMMAND_PATH override: got=%q want=%q", got, expected)
	}
}

func TestResolveRuntimeCommandPathIgnoresClaudeOverrideForNXS(t *testing.T) {
	got := resolveRuntimeCommandPathWith(
		runtimeKindNXS,
		"darwin",
		fakeEnv(map[string]string{
			nexusClaudeCommandPathEnvName: "/tmp/manual-claude",
		}),
		func(string) (string, error) { return "", os.ErrNotExist },
		func(string) bool { return false },
		fakeGlob(nil),
	)
	if got != "" {
		t.Fatalf("NEXUS_CLAUDE_COMMAND_PATH 不应覆盖 nxs runtime: got=%q", got)
	}
}

func TestResolveRuntimeCommandPathUsesEnvRuntimeKind(t *testing.T) {
	got := resolveRuntimeCommandPathWith(
		"",
		"darwin",
		fakeEnv(map[string]string{
			nexusAgentRuntimeKindEnvName: "nxs",
		}),
		func(string) (string, error) { return "", os.ErrNotExist },
		func(string) bool { return false },
		fakeGlob(nil),
	)
	if got != "" {
		t.Fatalf("NEXUS_AGENT_RUNTIME_KIND=nxs 无预置路径时应交给 bridge 解析: got=%q", got)
	}
}

func TestResolveRuntimeCommandPathAllowsEnvRuntimeKindOverrideToClaude(t *testing.T) {
	expected := "/opt/homebrew/bin/claude"
	got := resolveRuntimeCommandPathWith(
		runtimeKindNXS,
		"darwin",
		fakeEnv(map[string]string{nexusAgentRuntimeKindEnvName: "claude"}),
		func(string) (string, error) { return "", os.ErrNotExist },
		func(path string) bool { return path == expected },
		fakeGlob(nil),
	)
	if got != expected {
		t.Fatalf("NEXUS_AGENT_RUNTIME_KIND=claude 应覆盖 nxs 输入: got=%q want=%q", got, expected)
	}
}

func TestResolveRuntimeCommandPathDefersNXSFallbackToBridge(t *testing.T) {
	got := resolveRuntimeCommandPathWith(
		runtimeKindNXS,
		"darwin",
		fakeEnv(nil),
		func(string) (string, error) { return "", os.ErrNotExist },
		func(string) bool { return false },
		fakeGlob(nil),
	)
	if got != "" {
		t.Fatalf("nxs fallback 无预置路径时应交给 bridge 处理: got=%q", got)
	}
}

func TestResolveRuntimeCommandPathPrefersClaudeLookPath(t *testing.T) {
	expected := `C:\Users\lee\AppData\Roaming\npm\claude.cmd`
	got := resolveRuntimeCommandPathWith(
		runtimeKindClaude,
		"windows",
		fakeEnv(nil),
		func(name string) (string, error) {
			if name == "claude.cmd" {
				return expected, nil
			}
			return "", os.ErrNotExist
		},
		func(string) bool { return false },
		fakeGlob(nil),
	)
	if got != expected {
		t.Fatalf("PATH 中的 claude.cmd 未被优先识别: got=%q want=%q", got, expected)
	}
}

func TestResolveRuntimeCommandPathNonWindowsUsesClaudeLookPath(t *testing.T) {
	expected := "/Users/lee/.local/bin/claude"
	got := resolveRuntimeCommandPathWith(
		runtimeKindClaude,
		"linux",
		fakeEnv(nil),
		func(name string) (string, error) {
			if name == "claude" {
				return expected, nil
			}
			return "", os.ErrNotExist
		},
		func(string) bool { return false },
		fakeGlob(nil),
	)
	if got != expected {
		t.Fatalf("非 Windows PATH 中的 claude 未被识别: got=%q want=%q", got, expected)
	}
}

func TestResolveRuntimeCommandPathUsesMacOSHomebrewClaudePath(t *testing.T) {
	expected := "/opt/homebrew/bin/claude"
	got := resolveRuntimeCommandPathWith(
		runtimeKindClaude,
		"darwin",
		fakeEnv(nil),
		func(string) (string, error) { return "", os.ErrNotExist },
		func(path string) bool { return path == expected },
		fakeGlob(nil),
	)
	if got != expected {
		t.Fatalf("macOS Homebrew claude 未被识别: got=%q want=%q", got, expected)
	}
}

func TestResolveRuntimeCommandPathUsesNativeClaudeInstallerPath(t *testing.T) {
	home := "/Users/lee"
	expected := filepath.Join(home, ".local", "bin", "claude")
	got := resolveRuntimeCommandPathWith(
		runtimeKindClaude,
		"darwin",
		fakeEnv(map[string]string{"HOME": home}),
		func(string) (string, error) { return "", os.ErrNotExist },
		func(path string) bool { return path == expected },
		fakeGlob(nil),
	)
	if got != expected {
		t.Fatalf("native installer claude 未被识别: got=%q want=%q", got, expected)
	}
}

func TestResolveRuntimeCommandPathUsesNVMClaudeGlobalInstall(t *testing.T) {
	home := "/Users/lee"
	expected := filepath.Join(home, ".nvm", "versions", "node", "v22.11.0", "bin", "claude")
	pattern := filepath.Join(home, ".nvm", "versions", "node", "*", "bin", "claude")
	got := resolveRuntimeCommandPathWith(
		runtimeKindClaude,
		"darwin",
		fakeEnv(map[string]string{"HOME": home}),
		func(string) (string, error) { return "", os.ErrNotExist },
		func(path string) bool { return path == expected },
		fakeGlob(map[string][]string{pattern: []string{expected}}),
	)
	if got != expected {
		t.Fatalf("nvm npm global claude 未被识别: got=%q want=%q", got, expected)
	}
}

func TestResolveRuntimeCommandPathUsesVoltaClaudeShim(t *testing.T) {
	home := "/Users/lee"
	expected := filepath.Join(home, ".volta", "bin", "claude")
	got := resolveRuntimeCommandPathWith(
		runtimeKindClaude,
		"darwin",
		fakeEnv(map[string]string{"HOME": home}),
		func(string) (string, error) { return "", os.ErrNotExist },
		func(path string) bool { return path == expected },
		fakeGlob(nil),
	)
	if got != expected {
		t.Fatalf("Volta claude shim 未被识别: got=%q want=%q", got, expected)
	}
}

func TestResolveRuntimeCommandPathUsesASDFClaudeShim(t *testing.T) {
	home := "/Users/lee"
	expected := filepath.Join(home, ".asdf", "shims", "claude")
	got := resolveRuntimeCommandPathWith(
		runtimeKindClaude,
		"linux",
		fakeEnv(map[string]string{"HOME": home}),
		func(string) (string, error) { return "", os.ErrNotExist },
		func(path string) bool { return path == expected },
		fakeGlob(nil),
	)
	if got != expected {
		t.Fatalf("asdf claude shim 未被识别: got=%q want=%q", got, expected)
	}
}

func TestResolveRuntimeCommandPathUsesLinuxPackageClaudePath(t *testing.T) {
	expected := "/usr/bin/claude"
	got := resolveRuntimeCommandPathWith(
		runtimeKindClaude,
		"linux",
		fakeEnv(nil),
		func(string) (string, error) { return "", os.ErrNotExist },
		func(path string) bool { return path == expected },
		fakeGlob(nil),
	)
	if got != expected {
		t.Fatalf("Linux package manager claude 未被识别: got=%q want=%q", got, expected)
	}
}

func TestResolveRuntimeCommandPathUsesLinuxHomebrewClaudePath(t *testing.T) {
	expected := "/home/linuxbrew/.linuxbrew/bin/claude"
	got := resolveRuntimeCommandPathWith(
		runtimeKindClaude,
		"linux",
		fakeEnv(nil),
		func(string) (string, error) { return "", os.ErrNotExist },
		func(path string) bool { return path == expected },
		fakeGlob(nil),
	)
	if got != expected {
		t.Fatalf("Linux Homebrew claude 未被识别: got=%q want=%q", got, expected)
	}
}

func TestResolveRuntimeCommandPathFallsBackToSDKDefault(t *testing.T) {
	got := resolveRuntimeCommandPathWith(
		runtimeKindClaude,
		"linux",
		fakeEnv(nil),
		func(string) (string, error) { return "", os.ErrNotExist },
		func(string) bool { return false },
		fakeGlob(nil),
	)
	if got != "" {
		t.Fatalf("找不到 claude 时应继续交给 SDK 默认解析: got=%q", got)
	}
}

func fakeEnv(values map[string]string) func(string) string {
	return func(key string) string {
		return values[key]
	}
}

func fakeGlob(values map[string][]string) func(string) ([]string, error) {
	return func(pattern string) ([]string, error) {
		return values[pattern], nil
	}
}
