package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMemoryCommand(t *testing.T) {
	cfg := newCLITestConfig(t)
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, "memory"), 0o755); err != nil {
		t.Fatalf("创建 memory 目录失败: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, "MEMORY.md"), []byte("# MEMORY.md\n\n"), 0o644); err != nil {
		t.Fatalf("写入 MEMORY.md 失败: %v", err)
	}

	logPayload := runCLICommand(
		t,
		cfg,
		"memory",
		"--workspace",
		workspace,
		"log",
		"--kind",
		"LRN",
		"--title",
		"用户要求默认用中文回复",
		"--field",
		"详情=当前仓库要求统一中文输出",
	)
	item := asMap(t, logPayload["item"])
	if item["entry_id"] == "" {
		t.Fatalf("memory log 结果不正确: %+v", item)
	}

	searchPayload := runCLICommand(
		t,
		cfg,
		"memory",
		"--workspace",
		workspace,
		"search",
		"--query",
		"中文 回复",
	)
	items, ok := searchPayload["items"].([]any)
	if !ok || len(items) == 0 {
		t.Fatalf("memory search 结果不正确: %+v", searchPayload)
	}
}
