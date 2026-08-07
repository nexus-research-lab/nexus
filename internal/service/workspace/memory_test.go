package workspace

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"
)

func TestGetMemorySnapshotProjectsSDKFileLayout(t *testing.T) {
	cfg := newWorkspaceTestConfig(t)
	migrateWorkspaceSQLite(t, cfg.DatabaseURL)
	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()
	agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	workspaceService := NewService(cfg, agentService)
	agentValue, err := agentService.CreateAgent(context.Background(), protocol.CreateRequest{Name: "记忆投影测试"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}

	writeMemoryTestFile(t, agentValue.WorkspacePath, "MEMORY.md", "# MEMORY.md\n\n## Index\n\n- [协作偏好](memory/feedback_collaboration.md) — 保留真实证据\n")
	writeMemoryTestFile(t, agentValue.WorkspacePath, "memory/feedback_collaboration.md", `---
name: 协作偏好
description: 用户要求先读真实证据再动手
type: feedback
---

先读日志与代码。`)
	writeMemoryTestFile(t, agentValue.WorkspacePath, "memory/project_release.md", `---
name: 发布计划
description: 当前版本发布背景
type: project
---

发布计划正文。`)
	writeMemoryTestFile(t, agentValue.WorkspacePath, "memory/logs/2026/07/2026-07-10.md", "- 10:30 用户确认文件式记忆方案\n")

	snapshot, err := workspaceService.GetMemorySnapshot(context.Background(), agentValue.AgentID)
	if err != nil {
		t.Fatalf("读取记忆投影失败: %v", err)
	}
	if snapshot.Layout != "mixed" || snapshot.Index == nil || snapshot.Index.Path != "MEMORY.md" {
		t.Fatalf("记忆布局不正确: %+v", snapshot)
	}
	if len(snapshot.Documents) != 3 {
		t.Fatalf("记忆文件数量 = %d, want 3: %+v", len(snapshot.Documents), snapshot.Documents)
	}
	feedback := findMemoryDocument(t, snapshot.Documents, "memory/feedback_collaboration.md")
	if feedback.Type != "feedback" || feedback.Description != "用户要求先读真实证据再动手" || !feedback.Indexed {
		t.Fatalf("feedback 元数据不正确: %+v", feedback)
	}
	project := findMemoryDocument(t, snapshot.Documents, "memory/project_release.md")
	if project.Type != "project" || project.Indexed {
		t.Fatalf("project 索引状态不正确: %+v", project)
	}
	log := findMemoryDocument(t, snapshot.Documents, "memory/logs/2026/07/2026-07-10.md")
	if log.Kind != "daily_log" || log.Title != "2026-07-10" {
		t.Fatalf("daily log 元数据不正确: %+v", log)
	}
}

func TestGetMemorySnapshotReturnsEmptyLayoutBeforeRuntimeInitialization(t *testing.T) {
	cfg := newWorkspaceTestConfig(t)
	migrateWorkspaceSQLite(t, cfg.DatabaseURL)
	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()
	agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	workspaceService := NewService(cfg, agentService)
	agentValue, err := agentService.CreateAgent(context.Background(), protocol.CreateRequest{Name: "空记忆测试"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}

	snapshot, err := workspaceService.GetMemorySnapshot(context.Background(), agentValue.AgentID)
	if err != nil {
		t.Fatalf("读取空记忆投影失败: %v", err)
	}
	if snapshot.Layout != "empty" || snapshot.Index != nil || len(snapshot.Documents) != 0 {
		t.Fatalf("空记忆投影不正确: %+v", snapshot)
	}
}

func TestDeleteMemoryDocumentRemovesFileAndIndexEntry(t *testing.T) {
	cfg := newWorkspaceTestConfig(t)
	migrateWorkspaceSQLite(t, cfg.DatabaseURL)
	db, err := sql.Open("sqlite", cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()
	agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	workspaceService := NewService(cfg, agentService)
	agentValue, err := agentService.CreateAgent(context.Background(), protocol.CreateRequest{Name: "记忆删除测试"})
	if err != nil {
		t.Fatalf("创建 agent 失败: %v", err)
	}

	indexContent := "# MEMORY.md\n\n保留用户自定义说明。\n\n## Index\n\n" +
		"- [删除项](memory/remove_me.md) — 即将删除\n" +
		"- [保留项](memory/keep_me.md) — 必须保留\n"
	writeMemoryTestFile(t, agentValue.WorkspacePath, "MEMORY.md", indexContent)
	writeMemoryTestFile(t, agentValue.WorkspacePath, "memory/remove_me.md", "待删除正文。\n")
	writeMemoryTestFile(t, agentValue.WorkspacePath, "memory/keep_me.md", "保留正文。\n")

	for _, invalidPath := range []string{"MEMORY.md", "notes.md", "memory"} {
		if _, deleteErr := workspaceService.DeleteMemoryDocument(
			context.Background(),
			agentValue.AgentID,
			invalidPath,
		); deleteErr == nil {
			t.Fatalf("DeleteMemoryDocument(%q) 应拒绝非正文记忆路径", invalidPath)
		}
	}

	result, err := workspaceService.DeleteMemoryDocument(
		context.Background(),
		agentValue.AgentID,
		"memory/remove_me.md",
	)
	if err != nil {
		t.Fatalf("删除记忆失败: %v", err)
	}
	if result.Path != "memory/remove_me.md" {
		t.Fatalf("删除路径 = %q, want memory/remove_me.md", result.Path)
	}
	if _, statErr := os.Stat(filepath.Join(agentValue.WorkspacePath, "memory", "remove_me.md")); !os.IsNotExist(statErr) {
		t.Fatalf("删除后的文件状态错误: %v", statErr)
	}
	updatedIndex, err := os.ReadFile(filepath.Join(agentValue.WorkspacePath, "MEMORY.md"))
	if err != nil {
		t.Fatalf("读取更新后的索引失败: %v", err)
	}
	if strings.Contains(string(updatedIndex), "memory/remove_me.md") {
		t.Fatalf("索引仍包含已删除记忆: %s", updatedIndex)
	}
	for _, expected := range []string{"保留用户自定义说明", "memory/keep_me.md"} {
		if !strings.Contains(string(updatedIndex), expected) {
			t.Fatalf("索引丢失 %q: %s", expected, updatedIndex)
		}
	}

	snapshot, err := workspaceService.GetMemorySnapshot(context.Background(), agentValue.AgentID)
	if err != nil {
		t.Fatalf("读取删除后的记忆投影失败: %v", err)
	}
	if len(snapshot.Documents) != 1 || snapshot.Documents[0].Path != "memory/keep_me.md" || !snapshot.Documents[0].Indexed {
		t.Fatalf("删除后的记忆投影不正确: %+v", snapshot.Documents)
	}
}

func writeMemoryTestFile(t *testing.T, root string, relativePath string, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("创建记忆目录失败: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("写入记忆文件失败: %v", err)
	}
	modifiedAt := time.Date(2026, time.July, 10, 10, 30, 0, 0, time.UTC)
	if err := os.Chtimes(path, modifiedAt, modifiedAt); err != nil {
		t.Fatalf("设置记忆时间失败: %v", err)
	}
}

func findMemoryDocument(t *testing.T, documents []MemoryDocument, path string) MemoryDocument {
	t.Helper()
	for _, document := range documents {
		if document.Path == path {
			return document
		}
	}
	t.Fatalf("未找到记忆文件 %s: %+v", path, documents)
	return MemoryDocument{}
}
