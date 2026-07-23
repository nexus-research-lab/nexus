// INPUT: 同时包含旧版宿主数据、runtime 数据和 workspace 的临时状态根。
// OUTPUT: 验证分类迁移、冲突保护、完成标记与重复执行语义。
// POS: .nexus/app 与 users/<owner> 布局迁移的文件安全回归测试。
package migration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
)

func TestRunStateLayoutMigratesLegacyEntries(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	writeMigrationTestFile(t, filepath.Join(stateRoot, "data", "nexus.db"), "database\n")
	writeMigrationTestFile(t, filepath.Join(stateRoot, "config", "runtime-settings.json"), "{}\n")
	writeMigrationTestFile(t, filepath.Join(stateRoot, "logs", "logger.log"), "host-log\n")
	writeMigrationTestFile(t, filepath.Join(stateRoot, "logs", "debug", "runtime.log"), "runtime-log\n")
	writeMigrationTestFile(t, filepath.Join(stateRoot, "projects", "session.jsonl"), "{}\n")
	writeMigrationTestFile(t, filepath.Join(stateRoot, "settings.json"), "{}\n")
	writeMigrationTestFile(t, filepath.Join(stateRoot, "backups", "claude.json"), "backup\n")
	writeMigrationTestFile(t, filepath.Join(stateRoot, ".config.json"), "{\"legacy\":true}\n")
	writeMigrationTestFile(t, filepath.Join(stateRoot, "skills", "demo", "SKILL.md"), "skill\n")
	writeMigrationTestFile(t, filepath.Join(stateRoot, "config", ".claude.json"), "legacy-claude\n")
	writeMigrationTestFile(t, filepath.Join(stateRoot, ".claude", "profile.json"), "current-profile\n")
	writeMigrationTestFile(t, filepath.Join(stateRoot, "config", ".claude", "profile.json"), "legacy-profile\n")
	writeMigrationTestFile(t, filepath.Join(stateRoot, "config", "settings.json"), "{\"nested\":true}\n")
	writeMigrationTestFile(t, filepath.Join(stateRoot, "config", "projects", "legacy", "session.jsonl"), "nested-project\n")
	writeMigrationTestFile(t, filepath.Join(stateRoot, "config", "desktop-state.json"), "{}\n")
	writeMigrationTestFile(t, filepath.Join(stateRoot, "config", "future-claude-state.bin"), "runtime\n")
	writeMigrationTestFile(t, filepath.Join(stateRoot, "workspace", "nexus", "AGENTS.md"), "keep\n")
	writeMigrationTestFile(t, filepath.Join(stateRoot, "custom-host-data", "state.json"), "{}\n")
	writeMigrationTestFile(t, filepath.Join(stateRoot, "NexusDesktop.lock"), "active\n")

	if err := RunStateLayout(stateRoot, discardMigrationLogger()); err != nil {
		t.Fatalf("执行状态根布局迁移失败: %v", err)
	}

	assertMigrationFileContent(t, filepath.Join(stateRoot, "app", "data", "nexus.db"), "database\n")
	assertMigrationFileContent(t, filepath.Join(stateRoot, "app", "logs", "logger.log"), "host-log\n")
	assertMigrationFileContent(
		t,
		filepath.Join(
			stateRoot,
			"users",
			authctx.SystemUserID,
			"runtime",
			"logs",
			"debug",
			"runtime.log",
		),
		"runtime-log\n",
	)
	assertMigrationFileContent(
		t,
		filepath.Join(stateRoot, "app", "config", "runtime-settings.json"),
		"{}\n",
	)
	assertMigrationFileContent(
		t,
		filepath.Join(stateRoot, "users", authctx.SystemUserID, "runtime", "projects", "session.jsonl"),
		"{}\n",
	)
	assertMigrationFileContent(
		t,
		filepath.Join(stateRoot, "users", authctx.SystemUserID, "runtime", "settings.json"),
		"{}\n",
	)
	assertMigrationFileContent(
		t,
		filepath.Join(
			stateRoot,
			"users",
			authctx.SystemUserID,
			"runtime",
			"backups",
			"claude.json",
		),
		"backup\n",
	)
	assertMigrationFileContent(
		t,
		filepath.Join(stateRoot, "users", authctx.SystemUserID, "runtime", ".config.json"),
		"{\"legacy\":true}\n",
	)
	assertMigrationFileContent(
		t,
		filepath.Join(
			stateRoot,
			"users",
			authctx.SystemUserID,
			"runtime",
			"skills",
			"demo",
			"SKILL.md",
		),
		"skill\n",
	)
	assertMigrationFileContent(
		t,
		filepath.Join(stateRoot, "users", authctx.SystemUserID, "runtime", ".claude.json"),
		"legacy-claude\n",
	)
	assertMigrationFileContent(
		t,
		filepath.Join(
			stateRoot,
			"users",
			authctx.SystemUserID,
			"runtime",
			".claude",
			"profile.json",
		),
		"current-profile\n",
	)
	assertMigrationFileContent(
		t,
		filepath.Join(
			stateRoot,
			"users",
			authctx.SystemUserID,
			"runtime",
			".claude",
			"profile.json.legacy-config",
		),
		"legacy-profile\n",
	)
	assertMigrationFileContent(
		t,
		filepath.Join(stateRoot, "users", authctx.SystemUserID, "runtime", "settings.json"),
		"{}\n",
	)
	assertMigrationFileContent(
		t,
		filepath.Join(
			stateRoot,
			"users",
			authctx.SystemUserID,
			"runtime",
			"settings.json.legacy-config",
		),
		"{\"nested\":true}\n",
	)
	assertMigrationFileContent(
		t,
		filepath.Join(
			stateRoot,
			"users",
			authctx.SystemUserID,
			"runtime",
			"projects",
			"legacy",
			"session.jsonl",
		),
		"nested-project\n",
	)
	assertMigrationFileContent(
		t,
		filepath.Join(stateRoot, "app", "config", "desktop-state.json"),
		"{}\n",
	)
	assertMigrationFileContent(
		t,
		filepath.Join(
			stateRoot,
			"users",
			authctx.SystemUserID,
			"runtime",
			"future-claude-state.bin",
		),
		"runtime\n",
	)
	assertMigrationFileContent(
		t,
		filepath.Join(
			stateRoot,
			"users",
			authctx.SystemUserID,
			"runtime",
			"custom-host-data",
			"state.json",
		),
		"{}\n",
	)
	assertMigrationFileContent(t, filepath.Join(stateRoot, "workspace", "nexus", "AGENTS.md"), "keep\n")
	assertMigrationFileContent(t, filepath.Join(stateRoot, "NexusDesktop.lock"), "active\n")
	assertMigrationPathMissing(t, filepath.Join(stateRoot, "data"))
	assertMigrationPathMissing(t, filepath.Join(stateRoot, "projects"))

	markerPath := filepath.Join(stateRoot, ".layout-migrations", stateLayoutMigrationName)
	assertLayoutMigrationMarker(t, markerPath)
	if err := RunStateLayout(stateRoot, discardMigrationLogger()); err != nil {
		t.Fatalf("重复执行状态根布局迁移失败: %v", err)
	}
}

func TestRunStateLayoutMergesIdenticalDestinations(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	sourcePath := filepath.Join(stateRoot, "data", "nexus.db")
	targetPath := filepath.Join(stateRoot, "app", "data", "nexus.db")
	writeMigrationTestFile(t, sourcePath, "same\n")
	writeMigrationTestFile(t, targetPath, "same\n")
	writeMigrationTestFile(t, filepath.Join(stateRoot, "data", "new.db"), "new\n")

	if err := RunStateLayout(stateRoot, discardMigrationLogger()); err != nil {
		t.Fatalf("合并相同目标失败: %v", err)
	}

	assertMigrationPathMissing(t, filepath.Join(stateRoot, "data"))
	assertMigrationFileContent(t, targetPath, "same\n")
	assertMigrationFileContent(t, filepath.Join(stateRoot, "app", "data", "new.db"), "new\n")
}

func TestMoveLayoutEntrySamePathIsNoop(t *testing.T) {
	path := filepath.Join(t.TempDir(), "same.txt")
	writeMigrationTestFile(t, path, "keep\n")

	moved, err := moveLayoutEntry(path, path)
	if err != nil {
		t.Fatalf("同路径迁移不应失败: %v", err)
	}
	if moved {
		t.Fatal("同路径迁移不应报告移动")
	}
	assertMigrationFileContent(t, path, "keep\n")
}

func TestRunStateLayoutRejectsConflictingDestination(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	sourcePath := filepath.Join(stateRoot, "data", "nexus.db")
	targetPath := filepath.Join(stateRoot, "app", "data", "nexus.db")
	writeMigrationTestFile(t, sourcePath, "legacy\n")
	writeMigrationTestFile(t, targetPath, "current\n")

	if err := RunStateLayout(stateRoot, discardMigrationLogger()); err == nil {
		t.Fatal("目标内容不一致时应返回冲突错误")
	}

	assertMigrationFileContent(t, sourcePath, "legacy\n")
	assertMigrationFileContent(t, targetPath, "current\n")
	if _, err := os.Stat(filepath.Join(stateRoot, ".layout-migrations", stateLayoutMigrationName)); !os.IsNotExist(err) {
		t.Fatalf("失败迁移不应写完成标记: %v", err)
	}
}

func TestRunStateLayoutPreservesPrecreatedAppConfig(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	writeMigrationTestFile(
		t,
		filepath.Join(stateRoot, "config", "desktop-state.json"),
		"legacy\n",
	)
	writeMigrationTestFile(
		t,
		filepath.Join(stateRoot, "app", "config", "desktop-state.json"),
		"precreated\n",
	)

	if err := RunStateLayout(stateRoot, discardMigrationLogger()); err != nil {
		t.Fatalf("预创建 app/config 不应阻断迁移: %v", err)
	}

	assertMigrationFileContent(
		t,
		filepath.Join(stateRoot, "app", "config", "desktop-state.json"),
		"precreated\n",
	)
	assertMigrationFileContent(
		t,
		filepath.Join(stateRoot, "app", "config", "desktop-state.json.legacy-config"),
		"legacy\n",
	)
}

func TestRunStateLayoutPreservesLegacyClaudeConfigConflict(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	writeMigrationTestFile(t, filepath.Join(stateRoot, ".claude.json"), "current\n")
	writeMigrationTestFile(t, filepath.Join(stateRoot, "config", ".claude.json"), "legacy\n")

	if err := RunStateLayout(stateRoot, discardMigrationLogger()); err != nil {
		t.Fatalf("Claude 配置冲突不应阻断迁移: %v", err)
	}

	assertMigrationFileContent(
		t,
		filepath.Join(stateRoot, "users", authctx.SystemUserID, "runtime", ".claude.json"),
		"current\n",
	)
	assertMigrationFileContent(
		t,
		filepath.Join(
			stateRoot,
			"users",
			authctx.SystemUserID,
			"runtime",
			".claude.json.legacy-config",
		),
		"legacy\n",
	)
}

func TestRunStateLayoutPreservesPrecreatedRuntimeConfig(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	writeMigrationTestFile(t, filepath.Join(stateRoot, ".claude.json"), "legacy\n")
	writeMigrationTestFile(
		t,
		filepath.Join(stateRoot, "users", authctx.SystemUserID, "runtime", ".claude.json"),
		"precreated\n",
	)

	if err := RunStateLayout(stateRoot, discardMigrationLogger()); err != nil {
		t.Fatalf("预创建 runtime 配置不应阻断迁移: %v", err)
	}
	assertMigrationFileContent(
		t,
		filepath.Join(stateRoot, "users", authctx.SystemUserID, "runtime", ".claude.json"),
		"precreated\n",
	)
	assertMigrationFileContent(
		t,
		filepath.Join(
			stateRoot,
			"users",
			authctx.SystemUserID,
			"runtime",
			".claude.json.legacy-config",
		),
		"legacy\n",
	)
}

func TestRunStateLayoutHardensSharedWorkspacePermissions(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	sharedFile := filepath.Join(stateRoot, "shared-workspaces", "project", "README.md")
	writeMigrationTestFile(t, sharedFile, "shared\n")
	if err := os.Chmod(filepath.Dir(sharedFile), 0o777); err != nil {
		t.Fatalf("设置旧共享目录权限失败: %v", err)
	}
	if err := os.Chmod(sharedFile, 0o666); err != nil {
		t.Fatalf("设置旧共享文件权限失败: %v", err)
	}

	if err := RunStateLayout(stateRoot, discardMigrationLogger()); err != nil {
		t.Fatalf("共享 workspace 权限收紧失败: %v", err)
	}

	directoryInfo, err := os.Stat(filepath.Join(stateRoot, "shared-workspaces", "project"))
	if err != nil {
		t.Fatalf("读取共享目录失败: %v", err)
	}
	if directoryInfo.Mode().Perm() != 0o700 {
		t.Fatalf("共享目录权限错误: %o", directoryInfo.Mode().Perm())
	}
	fileInfo, err := os.Stat(filepath.Join(stateRoot, "shared-workspaces", "project", "README.md"))
	if err != nil {
		t.Fatalf("读取共享文件失败: %v", err)
	}
	if fileInfo.Mode().Perm() != 0o600 {
		t.Fatalf("共享文件权限错误: %o", fileInfo.Mode().Perm())
	}
}

func assertLayoutMigrationMarker(t *testing.T, markerPath string) {
	t.Helper()
	content, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("读取状态布局迁移标记失败: %v", err)
	}
	if string(content) != "completed\n" {
		t.Fatalf("状态布局迁移标记内容错误: %q", content)
	}
	info, err := os.Stat(markerPath)
	if err != nil {
		t.Fatalf("读取状态布局迁移标记权限失败: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("状态布局迁移标记权限错误: %o", info.Mode().Perm())
	}
}
