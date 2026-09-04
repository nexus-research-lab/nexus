// INPUT: 同时包含旧版宿主数据、runtime 数据和 workspace 的临时状态根。
// OUTPUT: 验证分类迁移、冲突保护、完成标记与重复执行语义。
// POS: .nexus/app 与 users/<owner> 布局迁移的文件安全回归测试。
package migration

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
)

func assertMigrationFileContent(t *testing.T, path string, expected string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取迁移文件失败 %q: %v", path, err)
	}
	if string(content) != expected {
		t.Fatalf("迁移文件内容 %q = %q, want %q", path, content, expected)
	}
}

func TestRunStateLayoutMigratesLegacyEntries(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	writeMigrationTestFile(t, filepath.Join(stateRoot, "data", "nexus.db"), "database\n")
	writeMigrationTestFile(t, filepath.Join(stateRoot, "config", "runtime-settings.json"), "{}\n")
	writeMigrationTestFile(
		t,
		filepath.Join(stateRoot, ".migrations", "20260723_migrate_v0_1_27_skill_storage"),
		"completed\n",
	)
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
	writeMigrationTestFile(t, filepath.Join(stateRoot, "control", "data", "control.db"), "control\n")
	writeMigrationTestFile(t, filepath.Join(stateRoot, "control-public", "control-signing.pub"), "public\n")
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
		filepath.Join(
			stateRoot,
			"app",
			".migrations",
			"20260723_migrate_v0_1_27_skill_storage",
		),
		"completed\n",
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
	assertMigrationFileContent(t, filepath.Join(stateRoot, "control", "data", "control.db"), "control\n")
	assertMigrationFileContent(t, filepath.Join(stateRoot, "control-public", "control-signing.pub"), "public\n")
	assertMigrationFileContent(t, filepath.Join(stateRoot, "NexusDesktop.lock"), "active\n")
	assertMigrationPathMissing(t, filepath.Join(stateRoot, "data"))
	assertMigrationPathMissing(t, filepath.Join(stateRoot, "projects"))

	markerPath := filepath.Join(stateRoot, "app", ".migrations", stateLayoutMigrationName)
	assertLayoutMigrationMarker(t, markerPath)
	assertMigrationPathMissing(t, filepath.Join(stateRoot, ".layout-migrations"))
	if err := RunStateLayout(stateRoot, discardMigrationLogger()); err != nil {
		t.Fatalf("重复执行状态根布局迁移失败: %v", err)
	}
}

func TestRunStateLayoutPreservesCompletedTreePermissions(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	runtimeFile := filepath.Join(
		stateRoot,
		"users",
		authctx.SystemUserID,
		"runtime",
		"settings.json",
	)
	writeMigrationTestFile(t, filepath.Join(stateRoot, "settings.json"), "{}\n")
	if err := RunStateLayout(stateRoot, discardMigrationLogger()); err != nil {
		t.Fatalf("执行状态根布局迁移失败: %v", err)
	}
	if err := os.Chmod(runtimeFile, 0o660); err != nil {
		t.Fatalf("模拟 runtime 私有组权限失败: %v", err)
	}

	if err := RunStateLayout(stateRoot, discardMigrationLogger()); err != nil {
		t.Fatalf("完成迁移后的重复启动失败: %v", err)
	}
	info, err := os.Stat(runtimeFile)
	if err != nil {
		t.Fatalf("读取 runtime 文件权限失败: %v", err)
	}
	if info.Mode().Perm() != 0o660 {
		t.Fatalf("完成标记后不应重新收紧 runtime ACL mask: %o", info.Mode().Perm())
	}
}

func TestRunStateLayoutLeavesPermissionsToIsolationLauncher(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 不提供 Unix 权限位语义")
	}

	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	appRoot := filepath.Join(stateRoot, "app")
	usersRoot := filepath.Join(stateRoot, "users")
	sharedRoot := filepath.Join(stateRoot, "shared-workspaces")
	sharedFile := filepath.Join(sharedRoot, "project", "README.md")
	writeMigrationTestFile(t, filepath.Join(stateRoot, "data", "nexus.db"), "database\n")
	writeMigrationTestFile(t, sharedFile, "shared\n")
	for _, directory := range []string{stateRoot, appRoot, usersRoot, sharedRoot} {
		if err := os.MkdirAll(directory, 0o770); err != nil {
			t.Fatalf("创建强隔离目录失败 %q: %v", directory, err)
		}
		if err := os.Chmod(directory, 0o770); err != nil {
			t.Fatalf("设置强隔离目录权限失败 %q: %v", directory, err)
		}
	}
	if err := os.Chmod(sharedFile, 0o660); err != nil {
		t.Fatalf("设置强隔离文件权限失败: %v", err)
	}

	if err := runStateLayout(
		stateRoot,
		discardMigrationLogger(),
		true,
	); err != nil {
		t.Fatalf("launcher 管理权限时执行状态迁移失败: %v", err)
	}

	for _, directory := range []string{stateRoot, appRoot, usersRoot, sharedRoot} {
		info, err := os.Stat(directory)
		if err != nil {
			t.Fatalf("读取强隔离目录权限失败 %q: %v", directory, err)
		}
		if info.Mode().Perm() != 0o770 {
			t.Fatalf("迁移不应覆盖 launcher 目录权限 %q: %o", directory, info.Mode().Perm())
		}
	}
	sharedInfo, err := os.Stat(sharedFile)
	if err != nil {
		t.Fatalf("读取强隔离文件权限失败: %v", err)
	}
	if sharedInfo.Mode().Perm() != 0o660 {
		t.Fatalf("迁移不应覆盖 launcher 文件权限: %o", sharedInfo.Mode().Perm())
	}
	assertMigrationFileContent(
		t,
		filepath.Join(appRoot, "data", "nexus.db"),
		"database\n",
	)
	assertLayoutMigrationMarker(
		t,
		filepath.Join(appRoot, ".migrations", stateLayoutMigrationName),
	)
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

func TestRunStateLayoutRemovesLegacyCacheWhenCanonicalMissing(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	sourcePath := filepath.Join(stateRoot, "cache", "WebView2", "last-runtime-version.txt")
	targetPath := filepath.Join(stateRoot, "app", "cache", "WebView2", "last-runtime-version.txt")
	writeMigrationTestFile(t, sourcePath, "0.1.27\n")

	if err := RunStateLayout(stateRoot, discardMigrationLogger()); err != nil {
		t.Fatalf("迁移唯一旧缓存失败: %v", err)
	}

	assertMigrationPathMissing(t, filepath.Join(stateRoot, "cache"))
	assertMigrationPathMissing(t, targetPath)
}

func TestRunStateLayoutPrefersCanonicalCacheOnConflict(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	sourceRoot := filepath.Join(stateRoot, "cache")
	targetRoot := filepath.Join(stateRoot, "app", "cache")
	writeMigrationTestFile(
		t,
		filepath.Join(sourceRoot, "WebView2", "last-runtime-version.txt"),
		"0.1.27\n",
	)
	writeMigrationTestFile(t, filepath.Join(sourceRoot, "WebView2", "legacy-only.bin"), "legacy cache\n")
	writeMigrationTestFile(
		t,
		filepath.Join(targetRoot, "WebView2", "last-runtime-version.txt"),
		"0.1.33\n",
	)
	writeMigrationTestFile(t, filepath.Join(targetRoot, "WebView2", "current-only.bin"), "current cache\n")

	if err := RunStateLayout(stateRoot, discardMigrationLogger()); err != nil {
		t.Fatalf("新旧缓存冲突不应阻断状态迁移: %v", err)
	}

	assertMigrationPathMissing(t, sourceRoot)
	assertMigrationFileContent(
		t,
		filepath.Join(targetRoot, "WebView2", "last-runtime-version.txt"),
		"0.1.33\n",
	)
	assertMigrationFileContent(
		t,
		filepath.Join(targetRoot, "WebView2", "current-only.bin"),
		"current cache\n",
	)
	assertMigrationPathMissing(t, filepath.Join(targetRoot, "WebView2", "legacy-only.bin"))
}

func TestRunStateLayoutIgnoresConflictingFinderMetadata(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	writeMigrationTestFile(t, filepath.Join(stateRoot, "rooms", ".DS_Store"), "legacy-finder-cache\n")
	writeMigrationTestFile(t, filepath.Join(stateRoot, "app", "rooms", ".DS_Store"), "current-finder-cache\n")

	if err := RunStateLayout(stateRoot, discardMigrationLogger()); err != nil {
		t.Fatalf("Finder 元数据冲突不应阻断迁移: %v", err)
	}

	assertMigrationFileContent(
		t,
		filepath.Join(stateRoot, "app", "rooms", ".DS_Store"),
		"current-finder-cache\n",
	)
	assertMigrationPathMissing(t, filepath.Join(stateRoot, "rooms"))
}

func TestRunStateLayoutMergesRoomOverlaySuperset(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	sourcePath := filepath.Join(stateRoot, "rooms", "room-1", "overlay.jsonl")
	targetPath := filepath.Join(stateRoot, "app", "rooms", "room-1", "overlay.jsonl")
	writeMigrationTestFile(t, sourcePath, "source-1\nsource-2\n")
	writeMigrationTestFile(t, targetPath, "source-1\n")

	if err := RunStateLayout(stateRoot, discardMigrationLogger()); err != nil {
		t.Fatalf("Room overlay 子集合并不应阻断迁移: %v", err)
	}

	assertMigrationFileContent(t, targetPath, "source-1\nsource-2\n")
	assertMigrationPathMissing(t, sourcePath)
}

func TestRunStateLayoutAcceptsRoomOverlayTimestampRefresh(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	sourcePath := filepath.Join(stateRoot, "rooms", "room-1", "overlay.jsonl")
	targetPath := filepath.Join(stateRoot, "app", "rooms", "room-1", "overlay.jsonl")
	writeMigrationTestFile(
		t,
		sourcePath,
		`{"message_id":"message-1","role":"user","content":"hello","timestamp":100}`+"\n",
	)
	writeMigrationTestFile(
		t,
		targetPath,
		`{"message_id":"message-1","role":"user","content":"hello","timestamp":200}`+"\n",
	)

	if err := RunStateLayout(stateRoot, discardMigrationLogger()); err != nil {
		t.Fatalf("Room overlay 时间戳刷新不应阻断迁移: %v", err)
	}

	assertMigrationFileContent(
		t,
		targetPath,
		`{"message_id":"message-1","role":"user","content":"hello","timestamp":200}`+"\n",
	)
	assertMigrationPathMissing(t, sourcePath)
}

func TestRunStateLayoutPreservesRoomOverlayConflict(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	sourcePath := filepath.Join(stateRoot, "rooms", "room-1", "overlay.jsonl")
	targetPath := filepath.Join(stateRoot, "app", "rooms", "room-1", "overlay.jsonl")
	writeMigrationTestFile(t, sourcePath, "source\n")
	writeMigrationTestFile(t, targetPath, "target\n")

	if err := RunStateLayout(stateRoot, discardMigrationLogger()); err == nil {
		t.Fatal("无法证明包含关系的 Room overlay 冲突应保留并阻断迁移")
	}

	assertMigrationFileContent(t, sourcePath, "source\n")
	assertMigrationFileContent(t, targetPath, "target\n")
}

func TestRunStateLayoutPreservesRoomOverlayNumericConflict(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	sourcePath := filepath.Join(stateRoot, "rooms", "room-1", "overlay.jsonl")
	targetPath := filepath.Join(stateRoot, "app", "rooms", "room-1", "overlay.jsonl")
	writeMigrationTestFile(
		t,
		sourcePath,
		`{"message_id":"message-1","sequence":9007199254740992,"timestamp":100}`+"\n",
	)
	writeMigrationTestFile(
		t,
		targetPath,
		`{"message_id":"message-1","sequence":9007199254740993,"timestamp":200}`+"\n",
	)

	if err := RunStateLayout(stateRoot, discardMigrationLogger()); err == nil {
		t.Fatal("Room overlay 非时间戳数值不同必须保留并阻断迁移")
	}

	assertMigrationFileContent(
		t,
		sourcePath,
		`{"message_id":"message-1","sequence":9007199254740992,"timestamp":100}`+"\n",
	)
	assertMigrationFileContent(
		t,
		targetPath,
		`{"message_id":"message-1","sequence":9007199254740993,"timestamp":200}`+"\n",
	)
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
	if _, err := os.Stat(filepath.Join(stateRoot, "app", ".migrations", stateLayoutMigrationName)); !os.IsNotExist(err) {
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

func TestRetryMissingLayoutSourceTreatsRemovedSourceAsCompleted(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "source")
	writeMigrationTestFile(t, sourcePath, "legacy\n")
	calls := 0

	moved, err := retryMissingLayoutSource(sourcePath, func() (bool, error) {
		calls++
		if removeErr := os.Remove(sourcePath); removeErr != nil {
			return false, removeErr
		}
		return false, fmt.Errorf("打开源文件: %w", os.ErrNotExist)
	})
	if err != nil {
		t.Fatalf("源路径已被并发移除时不应失败: %v", err)
	}
	if moved {
		t.Fatal("源路径已被并发移除时不应报告本次移动")
	}
	if calls != 1 {
		t.Fatalf("源路径消失后应立即结束，实际调用次数: %d", calls)
	}
}

func TestRetryMissingLayoutSourceRetriesWhenSourceRemains(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "source")
	writeMigrationTestFile(t, sourcePath, "legacy\n")
	calls := 0

	moved, err := retryMissingLayoutSource(sourcePath, func() (bool, error) {
		calls++
		if calls == 1 {
			return true, fmt.Errorf("rename: %w", os.ErrNotExist)
		}
		return false, nil
	})
	if err != nil {
		t.Fatalf("源路径仍存在时应重试并完成: %v", err)
	}
	if !moved {
		t.Fatal("重试前已经产生移动时应保留 moved 结果")
	}
	if calls != 2 {
		t.Fatalf("瞬态 ENOENT 应重试一次，实际调用次数: %d", calls)
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
