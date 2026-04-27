package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServiceLifecycle(t *testing.T) {
	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "MEMORY.md"), []byte("# MEMORY.md\n\n"), 0o644); err != nil {
		t.Fatalf("写入 MEMORY.md 失败: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(workspace, "memory"), 0o755); err != nil {
		t.Fatalf("创建 memory 目录失败: %v", err)
	}

	service := NewService(workspace)
	logged, err := service.Log("LRN", "用户要求注释使用中文", "correction", []Field{
		{Key: "详情", Value: "所有非平凡注释都要使用中文"},
		{Key: "行动", Value: "后续统一使用中文注释"},
	}, "")
	if err != nil {
		t.Fatalf("记录记忆失败: %v", err)
	}
	if logged.EntryID == "" || logged.Path == "" {
		t.Fatalf("记录结果不完整: %+v", logged)
	}

	matches, err := service.Search("中文 注释", 10)
	if err != nil {
		t.Fatalf("检索记忆失败: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("预期命中 memory 日志")
	}

	reviewItems, err := service.ReviewRecentEntries(3, 6)
	if err != nil {
		t.Fatalf("回顾近期日志失败: %v", err)
	}
	if len(reviewItems) == 0 {
		t.Fatal("预期至少一条 review 结果")
	}

	promoted, err := service.Promote("agents", "注释使用中文", "仓库规范", logged.EntryID)
	if err != nil {
		t.Fatalf("提升长期规则失败: %v", err)
	}
	if promoted.Path != "AGENTS.md" {
		t.Fatalf("提升目标文件不正确: %+v", promoted)
	}

	resolved, err := service.ResolveEntry(logged.EntryID, "已同步到 Go 代码规范")
	if err != nil {
		t.Fatalf("标记解决失败: %v", err)
	}
	if resolved.Status != "resolved" {
		t.Fatalf("解决状态不正确: %+v", resolved)
	}

	statusUpdated, err := service.SetEntryStatus(logged.EntryID, "case_by_case", "仅当前仓库启用")
	if err != nil {
		t.Fatalf("更新状态失败: %v", err)
	}
	if statusUpdated.Status != "case_by_case" {
		t.Fatalf("状态更新结果不正确: %+v", statusUpdated)
	}

	content, err := os.ReadFile(filepath.Join(workspace, "AGENTS.md"))
	if err != nil {
		t.Fatalf("读取 AGENTS.md 失败: %v", err)
	}
	if !strings.Contains(string(content), "仓库规范") {
		t.Fatalf("长期规则未写入 AGENTS.md: %s", string(content))
	}
}
