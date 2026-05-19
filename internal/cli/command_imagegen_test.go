package cli

import (
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
