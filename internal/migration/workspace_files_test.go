// INPUT: 带历史记忆与退役 Skill 的临时 workspace。
// OUTPUT: 验证四项迁移、全局标记和一次性执行语义。
// POS: 工作区文件迁移账本的回归测试。
package migration

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestRunWorkspaceFilesPreservesHistoricalMemoryMigrations(t *testing.T) {
	root := t.TempDir()
	configRoot := filepath.Join(root, ".nexus")
	workspaceRoot := filepath.Join(configRoot, "workspace")

	agentWorkspace := filepath.Join(workspaceRoot, "user_demo", "Amy")
	agentMemory := filepath.Join(agentWorkspace, "memory")
	systemAgentMemory := filepath.Join(workspaceRoot, "nexus", "memory")
	roomMemory := filepath.Join(configRoot, "rooms", "room-demo", "memory")
	legacyAgentSessions := filepath.Join(agentMemory, "sessions", "dm.md")
	legacyAgentCheckpoints := filepath.Join(agentMemory, "checkpoints.json")
	legacyAgentDiary := filepath.Join(agentMemory, "2026-07-10.md")
	legacyRoomSessions := filepath.Join(roomMemory, "sessions", "room.md")
	newTopic := filepath.Join(agentMemory, "preferences.md")
	newDailyLog := filepath.Join(agentMemory, "logs", "2026", "07", "2026-07-12.md")
	newSessionTopic := filepath.Join(agentMemory, "sessions", "project-notes.md")
	newCheckpoint := filepath.Join(workspaceRoot, "user_demo", "Bob", "memory", "checkpoints.json")
	keptProjectMemory := filepath.Join(agentWorkspace, "project", "memory", "README.md")
	keptIndex := filepath.Join(agentWorkspace, "MEMORY.md")

	writeMigrationTestFile(t, legacyAgentSessions, legacySessionSummary())
	writeMigrationTestFile(t, legacyAgentCheckpoints, "{\"scopes\":{}}\n")
	writeMigrationTestFile(t, legacyAgentDiary, legacyMemoryDiary())
	writeMigrationTestFile(t, filepath.Join(systemAgentMemory, "checkpoints.json"), "{\"scopes\":{}}\n")
	writeMigrationTestFile(t, legacyRoomSessions, legacySessionSummary())
	writeMigrationTestFile(t, newTopic, "---\ndescription: 用户偏好\ntype: preference\n---\n\n保持简洁。\n")
	writeMigrationTestFile(t, newDailyLog, "- 09:30 用户偏好简洁回复。\n")
	writeMigrationTestFile(t, newSessionTopic, "用户自定义的 sessions 主题，不是旧摘要。\n")
	writeMigrationTestFile(t, newCheckpoint, "{\"project\":\"user-owned\"}\n")
	writeMigrationTestFile(t, keptProjectMemory, "keep\n")
	writeMigrationTestFile(t, keptIndex, "- [偏好](memory/preferences.md) — 用户偏好\n")

	if err := RunWorkspaceFiles(configRoot, workspaceRoot, discardMigrationLogger()); err != nil {
		t.Fatalf("执行工作区文件迁移失败: %v", err)
	}
	assertMigrationPathMissing(t, legacyAgentSessions)
	assertMigrationPathMissing(t, legacyAgentCheckpoints)
	assertMigrationPathMissing(t, legacyAgentDiary)
	assertMigrationPathMissing(t, systemAgentMemory)
	assertMigrationPathMissing(t, roomMemory)
	assertMigrationPathExists(t, newTopic)
	assertMigrationPathExists(t, newDailyLog)
	assertMigrationPathExists(t, newSessionTopic)
	assertMigrationPathExists(t, newCheckpoint)
	assertMigrationPathExists(t, keptProjectMemory)
	assertMigrationPathExists(t, keptIndex)
	assertCompletedMigrationMarker(t, configRoot, legacyMemorySessionsMigrationName)
	assertCompletedMigrationMarker(t, configRoot, legacyMemoryDirectoryMigrationName)

	lateLegacyPath := filepath.Join(agentMemory, "late.md")
	writeMigrationTestFile(t, lateLegacyPath, "late\n")
	if err := RunWorkspaceFiles(configRoot, workspaceRoot, discardMigrationLogger()); err != nil {
		t.Fatalf("重复执行工作区文件迁移失败: %v", err)
	}
	assertMigrationPathExists(t, lateLegacyPath)
}

func TestRunWorkspaceFilesRemovesRetiredManagedSkillsOnce(t *testing.T) {
	root := t.TempDir()
	configRoot := filepath.Join(root, ".nexus")
	workspaceRoot := filepath.Join(configRoot, "workspace")
	agentWorkspace := filepath.Join(workspaceRoot, "user_demo", "Amy")
	roomWorkspace := filepath.Join(configRoot, "rooms", "room-demo")

	for _, workspacePath := range []string{agentWorkspace, roomWorkspace} {
		for _, skillName := range retiredManagedSkillNames {
			for _, entryPath := range deployedSkillEntries(workspacePath, skillName) {
				writeMigrationTestFile(t, filepath.Join(entryPath, "SKILL.md"), "legacy\n")
			}
		}
		writeMigrationTestFile(t, filepath.Join(workspacePath, ".agents", "skills", "goal-manager", "SKILL.md"), "keep\n")
	}

	if err := RunWorkspaceFiles(configRoot, workspaceRoot, discardMigrationLogger()); err != nil {
		t.Fatalf("执行已退役 Skill 迁移失败: %v", err)
	}
	for _, workspacePath := range []string{agentWorkspace, roomWorkspace} {
		for _, skillName := range retiredManagedSkillNames {
			for _, entryPath := range deployedSkillEntries(workspacePath, skillName) {
				assertMigrationPathMissing(t, entryPath)
			}
		}
		assertMigrationPathExists(t, filepath.Join(workspacePath, ".agents", "skills", "goal-manager", "SKILL.md"))
	}
	assertCompletedMigrationMarker(t, configRoot, retiredManagedSkillsMigrationName)

	customSkill := filepath.Join(agentWorkspace, ".agents", "skills", "scheduled-task-manager", "SKILL.md")
	writeMigrationTestFile(t, customSkill, "custom\n")
	if err := RunWorkspaceFiles(configRoot, workspaceRoot, discardMigrationLogger()); err != nil {
		t.Fatalf("重复执行已退役 Skill 迁移失败: %v", err)
	}
	if content, err := os.ReadFile(customSkill); err != nil || string(content) != "custom\n" {
		t.Fatalf("已完成迁移后不应重复删除同名自定义 Skill: content=%q err=%v", content, err)
	}
}

func TestRunWorkspaceFilesRemovesOnlyLegacyMemoryManagerSkill(t *testing.T) {
	root := t.TempDir()
	configRoot := filepath.Join(root, ".nexus")
	workspaceRoot := filepath.Join(configRoot, "workspace")
	managedWorkspace := filepath.Join(workspaceRoot, "user_demo", "Amy")
	managedRoom := filepath.Join(configRoot, "rooms", "room-demo")
	customWorkspace := filepath.Join(workspaceRoot, "user_demo", "Bob")

	legacyContent := "---\nname: memory-manager\n---\n\n# memory-manager\n\n真正的记忆能力已经沉到 `nexusctl memory`。\n"
	for _, workspacePath := range []string{managedWorkspace, managedRoom} {
		for _, entryPath := range deployedSkillEntries(workspacePath, legacyMemoryManagerSkillName) {
			writeMigrationTestFile(t, filepath.Join(entryPath, "SKILL.md"), legacyContent)
		}
	}
	customSkill := filepath.Join(customWorkspace, ".agents", "skills", legacyMemoryManagerSkillName, "SKILL.md")
	writeMigrationTestFile(t, customSkill, "---\nname: memory-manager\n---\n\n# 用户自建记忆工具\n")

	if err := RunWorkspaceFiles(configRoot, workspaceRoot, discardMigrationLogger()); err != nil {
		t.Fatalf("执行旧版记忆 Skill 迁移失败: %v", err)
	}
	for _, workspacePath := range []string{managedWorkspace, managedRoom} {
		for _, entryPath := range deployedSkillEntries(workspacePath, legacyMemoryManagerSkillName) {
			assertMigrationPathMissing(t, entryPath)
		}
	}
	if content, err := os.ReadFile(customSkill); err != nil || string(content) != "---\nname: memory-manager\n---\n\n# 用户自建记忆工具\n" {
		t.Fatalf("不应删除用户自建的同名 Skill: content=%q err=%v", content, err)
	}
	assertCompletedMigrationMarker(t, configRoot, legacyMemoryManagerMigrationName)

	recreatedSkill := filepath.Join(managedWorkspace, ".agents", "skills", legacyMemoryManagerSkillName, "SKILL.md")
	writeMigrationTestFile(t, recreatedSkill, "custom-after-migration\n")
	if err := RunWorkspaceFiles(configRoot, workspaceRoot, discardMigrationLogger()); err != nil {
		t.Fatalf("重复执行旧版记忆 Skill 迁移失败: %v", err)
	}
	if content, err := os.ReadFile(recreatedSkill); err != nil || string(content) != "custom-after-migration\n" {
		t.Fatalf("已完成迁移后不应再次删除同名 Skill: content=%q err=%v", content, err)
	}
}

func writeMigrationTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("创建测试目录失败: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("写入测试文件失败: %v", err)
	}
}

func legacySessionSummary() string {
	return "## 2026-07-10 14:00\n\n- Entry: LRN-20260710-140000-1\n- Scope: agent:demo\n"
}

func legacyMemoryDiary() string {
	return "### 2026-07-10 14:00 - [LRN] preference: concise replies\n*   **ID**: LRN-20260710-140000-1\n*   **状态**: auto\n"
}

func assertCompletedMigrationMarker(t *testing.T, configRoot string, migrationName string) {
	t.Helper()
	markerPath := workspaceFileMigrationMarker(configRoot, migrationName)
	content, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("读取迁移标记失败 %q: %v", markerPath, err)
	}
	if string(content) != "completed\n" {
		t.Fatalf("迁移标记内容错误 %q: %q", markerPath, content)
	}
	info, err := os.Stat(markerPath)
	if err != nil {
		t.Fatalf("读取迁移标记权限失败 %q: %v", markerPath, err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("迁移标记权限错误 %q: %o", markerPath, info.Mode().Perm())
	}
}

func assertMigrationPathExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("路径应存在 %q: %v", path, err)
	}
}

func assertMigrationPathMissing(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		t.Fatalf("路径应已删除 %q: %v", path, err)
	}
}

func discardMigrationLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
