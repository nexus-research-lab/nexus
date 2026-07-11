package cli

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestImagegenCommandUsesProviderBackedCLI(t *testing.T) {
	cfg := newCLITestConfig(t)
	migrateCLISQLite(t, cfg.DatabaseURL)

	errText := runCLICommandError(
		t,
		cfg,
		nil,
		"imagegen",
		"generate",
		"--prompt",
		"test image",
	)
	if !strings.Contains(errText, "未配置可用的图片生成 Provider") {
		t.Fatalf("imagegen CLI 应读取 Provider 配置并给出明确错误: %s", errText)
	}
}

func TestImagegenCommandRejectsPromptAndPromptFileTogether(t *testing.T) {
	cfg := newCLITestConfig(t)
	migrateCLISQLite(t, cfg.DatabaseURL)

	errText := runCLICommandError(
		t,
		cfg,
		nil,
		"imagegen",
		"generate",
		"--prompt",
		"test image",
		"--prompt-file",
		"prompt.txt",
	)
	if !strings.Contains(errText, "--prompt 与 --prompt-file 不能同时使用") {
		t.Fatalf("imagegen CLI 未校验 prompt 来源互斥: %s", errText)
	}
}

func TestResolveImagegenWorkspacePathUsesRuntimeWorkspaceEnv(t *testing.T) {
	workspacePath := t.TempDir()
	t.Setenv(nexusctlWorkspacePathEnvName, workspacePath)

	got, err := resolveImagegenWorkspacePath("")
	if err != nil {
		t.Fatalf("解析 imagegen workspace 失败: %v", err)
	}
	if got != workspacePath {
		t.Fatalf("imagegen workspace 应优先使用运行时环境变量: got=%s want=%s", got, workspacePath)
	}
}

func TestResolveImagegenWorkspacePathPrefersExplicitFlag(t *testing.T) {
	envWorkspacePath := t.TempDir()
	flagWorkspacePath := filepath.Join(t.TempDir(), "explicit")
	t.Setenv(nexusctlWorkspacePathEnvName, envWorkspacePath)

	got, err := resolveImagegenWorkspacePath(flagWorkspacePath)
	if err != nil {
		t.Fatalf("解析 imagegen workspace 失败: %v", err)
	}
	if got != flagWorkspacePath {
		t.Fatalf("显式 --workspace-path 应优先于环境变量: got=%s want=%s", got, flagWorkspacePath)
	}
}
