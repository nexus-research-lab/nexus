package cli

import (
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
