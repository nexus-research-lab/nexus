package host

import (
	"os"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
)

func TestHostManagedCLIScopeHidesSelectionFlags(t *testing.T) {
	for _, mode := range []string{runtimeScopeModeUserScoped, runtimeScopeModeSingleUser} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv(nexusctlUserIDEnvName, "user-owner")
			t.Setenv(nexusRuntimeScopeModeEnvName, mode)

			command, err := New(config.Config{})
			if err != nil {
				t.Fatalf("创建 CLI 命令失败: %v", err)
			}
			for _, name := range []string{"scope-user-id", "global-scope"} {
				flag := command.PersistentFlags().Lookup(name)
				if flag == nil {
					t.Fatalf("缺少作用域 flag: %s", name)
				}
				if !flag.Hidden {
					t.Fatalf("宿主注入作用域下不应在帮助中暴露 flag: %s", name)
				}
			}
		})
	}
}

func TestHostManagedCLIScopeRejectsSelectionOverrides(t *testing.T) {
	t.Setenv(nexusctlUserIDEnvName, "user-owner")
	t.Setenv(nexusRuntimeScopeModeEnvName, runtimeScopeModeUserScoped)

	testCases := [][]string{
		{"--scope-user-id", "user-owner", "agent", "list"},
		{"--global-scope", "--scope-user-id", "", "user", "list"},
	}
	for _, args := range testCases {
		command, err := New(config.Config{})
		if err != nil {
			t.Fatalf("创建 CLI 命令失败: %v", err)
		}
		command.SetArgs(args)
		err = command.Execute()
		if err == nil || !strings.Contains(err.Error(), hostManagedScopeOverrideError) {
			t.Fatalf("宿主注入作用域应拒绝显式覆盖: args=%q err=%v", args, err)
		}
	}
}

func TestManualCLIScopeFlagsRemainAvailableOutsideManagedRuntime(t *testing.T) {
	t.Setenv(nexusctlUserIDEnvName, "user-owner")
	t.Setenv(nexusRuntimeScopeModeEnvName, "")

	command, err := New(config.Config{})
	if err != nil {
		t.Fatalf("创建 CLI 命令失败: %v", err)
	}
	for _, name := range []string{"scope-user-id", "global-scope"} {
		flag := command.PersistentFlags().Lookup(name)
		if flag == nil {
			t.Fatalf("缺少作用域 flag: %s", name)
		}
		if flag.Hidden {
			t.Fatalf("人工 CLI 模式不应隐藏 flag: %s", name)
		}
	}
}

func TestCLIExecuteReleasesDatabasesAfterSuccessAndFailure(t *testing.T) {
	testCases := []struct {
		name      string
		args      []string
		wantError bool
	}{
		{name: "success", args: []string{"agent", "list"}},
		{name: "command error", args: []string{"agent", "get", "missing-agent"}, wantError: true},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			cfg := newCLITestConfig(t)
			migrateCLISQLite(t, cfg.DatabaseURL)
			app, err := New(cfg)
			if err != nil {
				t.Fatalf("创建 CLI 应用失败: %v", err)
			}
			app.SetArgs(testCase.args)
			_, _, executeErr := captureCLIStreams(t, app)
			if (executeErr != nil) != testCase.wantError {
				t.Fatalf("CLI 执行结果错误: err=%v wantError=%t", executeErr, testCase.wantError)
			}
			if err := os.Remove(cfg.DatabaseURL); err != nil {
				t.Fatalf("CLI 执行后数据库仍被占用: %v", err)
			}
		})
	}
}
