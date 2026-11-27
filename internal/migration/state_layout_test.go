// state_layout_test.go 覆盖状态根与布局迁移及跨版本恢复测试。
package migration

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
	"github.com/pressly/goose/v3"
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
	// Desktop has a separate conflict-preserving recovery path; this tests server behavior.
	t.Setenv(nexusAppModeEnvironment, "web")
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
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("状态布局迁移标记权限错误: %o", info.Mode().Perm())
	}
}

func TestRunDesktopStateRootRebaseCommitsCopiedState(t *testing.T) {
	root := t.TempDir()
	previousRoot := filepath.Join(root, "旧目录", ".nexus")
	currentRoot := filepath.Join(root, "新目录", "NexusData")
	databasePath := filepath.Join(currentRoot, "app", "data", "nexus.db")
	ownerUserID := "owner-a"
	agentID := "agent-a"
	oldWorkspace := filepath.Join(previousRoot, "users", ownerUserID, "workspace", agentID)
	newWorkspace := filepath.Join(currentRoot, "users", ownerUserID, "workspace", agentID)
	configureStateRootMigrationTest(t, previousRoot, currentRoot)

	cfg := config.Config{
		AppMode:        "desktop",
		DatabaseDriver: "sqlite",
		DatabaseURL:    databasePath,
		WorkspacePath:  filepath.Join(currentRoot, "users"),
	}
	seedStateRootMigrationDatabase(t, cfg, agentID, ownerUserID, oldWorkspace, previousRoot)
	seedStateRootMigrationFiles(
		t,
		previousRoot,
		currentRoot,
		ownerUserID,
		oldWorkspace,
		newWorkspace,
	)
	if err := RunDesktopStateRootRebase(
		context.Background(),
		cfg,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	); err != nil {
		t.Fatal(err)
	}

	assertRebasedDatabasePaths(t, cfg, newWorkspace, currentRoot)
	assertRebasedTranscriptProject(t, previousRoot, currentRoot, ownerUserID, oldWorkspace, newWorkspace)
	assertRebasedRoomMetadata(t, previousRoot, currentRoot, ownerUserID, oldWorkspace, newWorkspace)
	assertRebasedSessionLifecycle(t, cfg, currentRoot, ownerUserID, oldWorkspace, newWorkspace)
	// 桌面宿主若在健康回执前中断，会以同一 previous root 重试；提交必须幂等。
	if err := RunDesktopStateRootRebase(context.Background(), cfg, nil); err != nil {
		t.Fatalf("重复提交状态根迁移失败: %v", err)
	}
}

func TestValidateStateRootTransitionRejectsNestedRoots(t *testing.T) {
	root := t.TempDir()
	if err := validateStateRootTransition(root, filepath.Join(root, "nested")); err == nil {
		t.Fatal("嵌套状态根应被拒绝")
	}
	if err := validateStateRootTransition(filepath.Join(root, "nested"), root); err == nil {
		t.Fatal("包含旧根的目标状态根应被拒绝")
	}
}

func configureStateRootMigrationTest(t *testing.T, previousRoot string, currentRoot string) {
	t.Helper()
	t.Setenv(appfs.NexusStateRootEnvName, currentRoot)
	t.Setenv(appfs.NexusPreviousStateRootEnvName, previousRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	for _, directory := range []string{
		filepath.Join(currentRoot, "app", "config"),
		filepath.Join(currentRoot, "app", "data"),
		filepath.Join(currentRoot, "users"),
	} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
}

func seedStateRootMigrationDatabase(
	t *testing.T,
	cfg config.Config,
	agentID string,
	ownerUserID string,
	workspacePath string,
	artifactPath string,
) {
	t.Helper()
	db, err := storage.OpenDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	statements := []string{
		`CREATE TABLE agents (
            id TEXT PRIMARY KEY,
            owner_user_id TEXT NOT NULL,
            workspace_path TEXT NOT NULL
        )`,
		`CREATE TABLE task_artifacts (
            id TEXT PRIMARY KEY,
            artifact_path TEXT,
            description TEXT
        )`,
	}
	for _, statement := range statements {
		if _, err = db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.Exec(
		`INSERT INTO agents (id, owner_user_id, workspace_path) VALUES (?, ?, ?)`,
		agentID,
		ownerUserID,
		workspacePath,
	); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(
		`INSERT INTO task_artifacts (id, artifact_path, description) VALUES ('artifact-a', ?, ?)`,
		filepath.Join(artifactPath, "users", ownerUserID, "workspace", agentID, "result.txt"),
		"正文保留 "+artifactPath,
	); err != nil {
		t.Fatal(err)
	}
}

func seedStateRootMigrationFiles(
	t *testing.T,
	previousRoot string,
	currentRoot string,
	ownerUserID string,
	oldWorkspace string,
	newWorkspace string,
) {
	t.Helper()
	projectsRoot := filepath.Join(currentRoot, "users", ownerUserID, "runtime", "projects")
	oldProject := filepath.Join(projectsRoot, workspacestore.TranscriptProjectDirectoryName(oldWorkspace))
	if err := os.MkdirAll(oldProject, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldProject, "session.jsonl"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	roomsRoot := filepath.Join(currentRoot, "users", ownerUserID, "state", "rooms")
	if err := os.MkdirAll(roomsRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{
		"workspace_path": oldWorkspace,
		"nested": map[string]any{
			"cwd":  filepath.Join(oldWorkspace, "repo"),
			"text": "正文保留 " + previousRoot,
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(roomsRoot, "room.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	sessionsRoot := filepath.Join(
		currentRoot,
		"users",
		ownerUserID,
		"workspace",
		"agent-a",
		".agents",
		"sessions",
		"dm",
	)
	if err = os.MkdirAll(sessionsRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	sessionLine, err := json.Marshal(map[string]any{
		"output_file": filepath.Join(oldWorkspace, "result.txt"),
		"text":        "正文保留 " + previousRoot,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(
		filepath.Join(sessionsRoot, "overlay.jsonl"),
		append(sessionLine, '\n'),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	seedStateRootMigrationLifecycle(
		t,
		currentRoot,
		ownerUserID,
		oldWorkspace,
		newWorkspace,
	)
}

func seedStateRootMigrationLifecycle(
	t *testing.T,
	currentRoot string,
	ownerUserID string,
	oldWorkspace string,
	newWorkspace string,
) {
	t.Helper()
	lifecycleRoot := filepath.Join(
		currentRoot,
		"users",
		ownerUserID,
		"state",
		"session-lifecycle",
	)
	if err := os.MkdirAll(lifecycleRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	writeRecord := func(sessionKey string, workspacePath string) {
		payload, err := json.MarshalIndent(map[string]any{
			"session_key":           sessionKey,
			"owner_user_id":         ownerUserID,
			"workspace_path":        workspacePath,
			"state":                 "deleted",
			"generation":            2,
			"configuration_version": 1,
			"updated_at":            "2026-09-01T00:00:00Z",
		}, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(
			filepath.Join(lifecycleRoot, stateRootMigrationLifecycleFileName(workspacePath, sessionKey)),
			payload,
			0o600,
		); err != nil {
			t.Fatal(err)
		}
	}
	writeRecord("agent:agent-a:ws:dm:migration", oldWorkspace)
	// 模拟上次迁移已写入新文件、但尚未来得及删除旧文件时进程退出。
	writeRecord("agent:agent-a:ws:dm:interrupted", oldWorkspace)
	writeRecord("agent:agent-a:ws:dm:interrupted", newWorkspace)
}

func stateRootMigrationLifecycleFileName(workspacePath string, sessionKey string) string {
	physicalIdentity := filepath.Clean(workspacePath) + "\x00" +
		protocol.LegacySessionDirectoryIdentity(sessionKey)
	sum := sha256.Sum256([]byte(physicalIdentity))
	return hex.EncodeToString(sum[:]) + ".json"
}

func assertRebasedDatabasePaths(t *testing.T, cfg config.Config, workspacePath string, currentRoot string) {
	t.Helper()
	db, err := storage.OpenDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var gotWorkspace, artifactPath, description string
	if err = db.QueryRow(`SELECT workspace_path FROM agents WHERE id = 'agent-a'`).Scan(&gotWorkspace); err != nil {
		t.Fatal(err)
	}
	if gotWorkspace != workspacePath {
		t.Fatalf("workspace_path = %q, want %q", gotWorkspace, workspacePath)
	}
	if err = db.QueryRow(
		`SELECT artifact_path, description FROM task_artifacts WHERE id = 'artifact-a'`,
	).Scan(&artifactPath, &description); err != nil {
		t.Fatal(err)
	}
	wantArtifact := filepath.Join(workspacePath, "result.txt")
	if artifactPath != wantArtifact {
		t.Fatalf("artifact_path = %q, want %q", artifactPath, wantArtifact)
	}
	if description == "正文保留 "+currentRoot {
		t.Fatal("非路径字段不应被批量替换")
	}
}

func assertRebasedTranscriptProject(
	t *testing.T,
	previousRoot string,
	currentRoot string,
	ownerUserID string,
	oldWorkspace string,
	newWorkspace string,
) {
	t.Helper()
	projectsRoot := filepath.Join(currentRoot, "users", ownerUserID, "runtime", "projects")
	oldProject := filepath.Join(projectsRoot, workspacestore.TranscriptProjectDirectoryName(oldWorkspace))
	newProject := filepath.Join(projectsRoot, workspacestore.TranscriptProjectDirectoryName(newWorkspace))
	if _, err := os.Stat(oldProject); !os.IsNotExist(err) {
		t.Fatalf("旧 transcript 项目仍存在: %v", err)
	}
	if _, err := os.Stat(filepath.Join(newProject, "session.jsonl")); err != nil {
		t.Fatalf("新 transcript 项目缺失: %v (old root %s)", err, previousRoot)
	}
}

func assertRebasedRoomMetadata(
	t *testing.T,
	previousRoot string,
	currentRoot string,
	ownerUserID string,
	oldWorkspace string,
	newWorkspace string,
) {
	t.Helper()
	path := filepath.Join(currentRoot, "users", ownerUserID, "state", "rooms", "room.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err = json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["workspace_path"] != newWorkspace {
		t.Fatalf("Room workspace_path = %#v, want %q", payload["workspace_path"], newWorkspace)
	}
	nested := payload["nested"].(map[string]any)
	if nested["cwd"] != filepath.Join(newWorkspace, "repo") {
		t.Fatalf("Room cwd = %#v", nested["cwd"])
	}
	if nested["text"] != "正文保留 "+previousRoot {
		t.Fatalf("正文被路径重写: %#v", nested["text"])
	}
	sessionPath := filepath.Join(
		currentRoot,
		"users",
		ownerUserID,
		"workspace",
		"agent-a",
		".agents",
		"sessions",
		"dm",
		"overlay.jsonl",
	)
	sessionData, err := os.ReadFile(sessionPath)
	if err != nil {
		t.Fatal(err)
	}
	var sessionRow map[string]any
	if err = json.Unmarshal(sessionData, &sessionRow); err != nil {
		t.Fatal(err)
	}
	if sessionRow["output_file"] != filepath.Join(newWorkspace, "result.txt") {
		t.Fatalf("Agent session output_file = %#v", sessionRow["output_file"])
	}
	if sessionRow["text"] != "正文保留 "+previousRoot {
		t.Fatalf("Agent session 正文被路径重写: %#v", sessionRow["text"])
	}
}

func assertRebasedSessionLifecycle(
	t *testing.T,
	cfg config.Config,
	currentRoot string,
	ownerUserID string,
	oldWorkspace string,
	newWorkspace string,
) {
	t.Helper()
	lifecycleRoot := filepath.Join(
		currentRoot,
		"users",
		ownerUserID,
		"state",
		"session-lifecycle",
	)
	for _, sessionKey := range []string{
		"agent:agent-a:ws:dm:migration",
		"agent:agent-a:ws:dm:interrupted",
	} {
		if _, err := os.Stat(filepath.Join(
			lifecycleRoot,
			stateRootMigrationLifecycleFileName(oldWorkspace, sessionKey),
		)); !os.IsNotExist(err) {
			t.Fatalf("旧 Session 删除恢复记录仍存在: %v", err)
		}
		payload, err := os.ReadFile(filepath.Join(
			lifecycleRoot,
			stateRootMigrationLifecycleFileName(newWorkspace, sessionKey),
		))
		if err != nil {
			t.Fatal(err)
		}
		var record map[string]any
		if err = json.Unmarshal(payload, &record); err != nil {
			t.Fatal(err)
		}
		if record["workspace_path"] != newWorkspace {
			t.Fatalf("Session 删除恢复路径 = %#v, want %q", record["workspace_path"], newWorkspace)
		}
	}
	records, err := workspacestore.NewSessionFileStore(
		cfg.WorkspacePath,
	).ListSessionDeletionRecords()
	if err != nil {
		t.Fatalf("扫描迁移后的 Session 删除恢复记录失败: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("Session 删除恢复记录数量 = %d, want 2", len(records))
	}
}

func TestRunStateLayoutPreflightsRecoveryTargetsBeforeStagingUsers(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	t.Setenv("NEXUS_APP_MODE", "desktop")
	writeLayoutTestFile(
		t,
		filepath.Join(stateRoot, "data", skippedStateLayoutDatabaseName),
		"legacy database",
	)
	writeLayoutTestFile(
		t,
		filepath.Join(stateRoot, "app", "data", skippedStateLayoutDatabaseName),
		"new canonical database",
	)
	canonicalUserFile := filepath.Join(stateRoot, "users", "__system__", "workspace", "agent-new", "AGENTS.md")
	writeLayoutTestFile(t, canonicalUserFile, "new agent\n")
	writeLayoutTestFile(
		t,
		filepath.Join(skippedStateLayoutRecoveryDataRoot(stateRoot), "occupied"),
		"do not overwrite\n",
	)

	if err := RunStateLayout(stateRoot, discardMigrationLogger()); err == nil {
		t.Fatal("隔离目标冲突时状态迁移应失败")
	}
	assertMigrationFileContent(t, canonicalUserFile, "new agent\n")
	if _, err := os.Stat(skippedStateLayoutRecoveryUsersRoot(stateRoot)); !os.IsNotExist(err) {
		t.Fatalf("预检失败前不应移动 canonical users: %v", err)
	}
}

// TestV027AndV028ToV030LayoutRecoveryPreservesAgentAndRoomFiles 分别用两个发布版
// 的真实 schema 复现直接升级 v0.1.30 后，新旧数据同时存在的迁移缺口。
func TestV027AndV028ToV030LayoutRecoveryPreservesAgentAndRoomFiles(t *testing.T) {
	for _, release := range []struct {
		name          string
		schemaVersion int64
	}{
		{name: "v0.1.27", schemaVersion: 49},
		{name: "v0.1.28", schemaVersion: 50},
	} {
		t.Run(release.name, func(t *testing.T) {
			testLegacyReleaseToV030LayoutRecovery(t, release.schemaVersion)
		})
	}
}

func testLegacyReleaseToV030LayoutRecovery(t *testing.T, legacySchemaVersion int64) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	t.Setenv(appfs.NexusStateRootEnvName, stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	t.Setenv("NEXUS_APP_MODE", "desktop")

	legacyWorkspace := filepath.Join(stateRoot, "workspace", "legacy-agent")
	legacyWorkspaceTarget := filepath.Join(stateRoot, "users", "__system__", "workspace", "legacy-agent")
	canonicalWorkspace := filepath.Join(stateRoot, "users", "__system__", "workspace", "new-agent")
	legacyDatabase := filepath.Join(stateRoot, "data", skippedStateLayoutDatabaseName)
	canonicalDatabase := filepath.Join(stateRoot, "app", "data", skippedStateLayoutDatabaseName)
	createLayoutReleaseDatabase(t, legacyDatabase, legacySchemaVersion, layoutReleaseRecord{
		agentID:        "legacy-agent",
		agentName:      "legacy",
		workspacePath:  legacyWorkspace,
		roomID:         "legacy-room",
		conversationID: "legacy-conversation",
	})
	createLayoutReleaseDatabase(t, canonicalDatabase, 0, layoutReleaseRecord{
		agentID:        "new-agent",
		agentName:      "new",
		workspacePath:  canonicalWorkspace,
		roomID:         "new-room",
		conversationID: "new-conversation",
	})
	writeLayoutTestFile(t, filepath.Join(legacyWorkspace, "AGENTS.md"), "legacy agent\n")
	writeLayoutTestFile(t, filepath.Join(legacyWorkspace, "memory", "history.md"), "legacy memory\n")
	legacyProjectName := workspacestore.TranscriptProjectDirectoryName(legacyWorkspace)
	writeLayoutTestFile(
		t,
		filepath.Join(stateRoot, "projects", legacyProjectName, "legacy-session.jsonl"),
		"legacy transcript\n",
	)
	writeLayoutTestFile(
		t,
		filepath.Join(stateRoot, "rooms", "room-legacy-conversation", "overlay.jsonl"),
		"{\"conversation_id\":\"legacy-conversation\",\"message_id\":\"legacy-message\"}\n",
	)
	writeLayoutTestFile(
		t,
		filepath.Join(
			stateRoot,
			"rooms",
			"room-legacy-conversation",
			"attachments",
			"brief.txt",
		),
		"legacy attachment\n",
	)
	writeLayoutTestFile(t, filepath.Join(canonicalWorkspace, "AGENTS.md"), "new agent\n")
	writeLayoutTestFile(t, filepath.Join(canonicalWorkspace, "memory", "new.md"), "new memory\n")
	writeLayoutTestFile(
		t,
		filepath.Join(
			stateRoot,
			"users",
			"__system__",
			"state",
			"rooms",
			"room-new-conversation",
			"overlay.jsonl",
		),
		"{\"conversation_id\":\"new-conversation\",\"message_id\":\"new-message\"}\n",
	)
	// v0.1.30 会在找不到 app/rooms 时提前写下空完成标记。
	if err := writeWorkspaceFileMigrationMarker(workspaceFileMigrationMarker(
		filepath.Join(stateRoot, "app"),
		roomStateMigrationName,
	)); err != nil {
		t.Fatal(err)
	}

	if err := RunStateLayout(stateRoot, discardMigrationLogger()); err != nil {
		t.Fatal(err)
	}
	upgradeLayoutReleaseDatabase(t, canonicalDatabase)
	cfg := config.Config{DatabaseDriver: "sqlite", DatabaseURL: canonicalDatabase}
	if err := MergeSkippedStateLayoutDatabase(context.Background(), cfg, discardMigrationLogger()); err != nil {
		t.Fatal(err)
	}
	if err := RunWorkspaceLayout(context.Background(), cfg, stateRoot, discardMigrationLogger()); err != nil {
		t.Fatal(err)
	}
	if err := MergeSkippedStateLayoutUsers(stateRoot, discardMigrationLogger()); err != nil {
		t.Fatal(err)
	}
	if err := RunRoomFiles(context.Background(), cfg, discardMigrationLogger()); err != nil {
		t.Fatal(err)
	}

	assertMigrationFileContent(
		t,
		filepath.Join(legacyWorkspaceTarget, "AGENTS.md"),
		"legacy agent\n",
	)
	assertMigrationFileContent(
		t,
		filepath.Join(legacyWorkspaceTarget, "memory", "history.md"),
		"legacy memory\n",
	)
	assertMigrationFileContent(t, filepath.Join(canonicalWorkspace, "AGENTS.md"), "new agent\n")
	assertMigrationFileContent(t, filepath.Join(canonicalWorkspace, "memory", "new.md"), "new memory\n")
	targetProjectName := workspacestore.TranscriptProjectDirectoryName(legacyWorkspaceTarget)
	assertMigrationFileContent(
		t,
		filepath.Join(
			stateRoot,
			"users",
			"__system__",
			"runtime",
			"projects",
			targetProjectName,
			"legacy-session.jsonl",
		),
		"legacy transcript\n",
	)
	assertMigrationFileContent(
		t,
		filepath.Join(
			stateRoot,
			"users",
			"__system__",
			"state",
			"rooms",
			"room-legacy-conversation",
			"overlay.jsonl",
		),
		"{\"conversation_id\":\"legacy-conversation\",\"message_id\":\"legacy-message\"}\n",
	)
	assertMigrationFileContent(
		t,
		filepath.Join(
			stateRoot,
			"users",
			"__system__",
			"state",
			"rooms",
			"room-new-conversation",
			"overlay.jsonl",
		),
		"{\"conversation_id\":\"new-conversation\",\"message_id\":\"new-message\"}\n",
	)
	assertMigrationFileContent(
		t,
		filepath.Join(
			stateRoot,
			"users",
			"__system__",
			"workspace",
			".rooms",
			"room-legacy-conversation",
			"attachments",
			"brief.txt",
		),
		"legacy attachment\n",
	)
	assertLayoutReleaseRows(t, cfg)
}

func TestMergeSkippedStateLayoutDatabasePreservesBothBranches(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	t.Setenv(appfs.NexusStateRootEnvName, stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	canonicalDatabase := filepath.Join(stateRoot, "app", "data", skippedStateLayoutDatabaseName)
	recoveryDatabase := filepath.Join(
		skippedStateLayoutRecoveryDataRoot(stateRoot),
		skippedStateLayoutDatabaseName,
	)
	createSkippedLayoutMergeDatabase(t, canonicalDatabase, []string{
		`INSERT INTO agents (id, label) VALUES ('shared-agent', 'legacy')`,
		`INSERT INTO rooms (id, agent_id) VALUES ('legacy-room', 'shared-agent')`,
		`INSERT INTO conversations (id, room_id) VALUES ('legacy-conversation', 'legacy-room')`,
	})
	createSkippedLayoutMergeDatabase(t, recoveryDatabase, []string{
		`INSERT INTO agents (id, label) VALUES ('shared-agent', 'new-conflict')`,
		`INSERT INTO agents (id, label) VALUES ('new-agent', 'new')`,
		`INSERT INTO rooms (id, agent_id) VALUES ('new-room', 'new-agent')`,
		`INSERT INTO conversations (id, room_id) VALUES ('new-conversation', 'new-room')`,
	})
	cfg := config.Config{DatabaseDriver: "sqlite", DatabaseURL: canonicalDatabase}

	if err := MergeSkippedStateLayoutDatabase(context.Background(), cfg, discardMigrationLogger()); err != nil {
		t.Fatal(err)
	}
	assertSkippedLayoutMergeRows(t, cfg)
	if err := MergeSkippedStateLayoutDatabase(context.Background(), cfg, discardMigrationLogger()); err != nil {
		t.Fatalf("重复合并迁移缺口数据库失败: %v", err)
	}
	assertSkippedLayoutMergeRows(t, cfg)
}

func TestMergeSkippedStateLayoutUsersPreservesConflicts(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	canonicalUserRoot := filepath.Join(stateRoot, "users", "user-a")
	recoveryUserRoot := filepath.Join(skippedStateLayoutRecoveryUsersRoot(stateRoot), "user-a")
	writeLayoutTestFile(
		t,
		filepath.Join(canonicalUserRoot, "workspace", "agent-a", "conflict.txt"),
		"legacy wins\n",
	)
	writeLayoutTestFile(
		t,
		filepath.Join(recoveryUserRoot, "workspace", "agent-a", "conflict.txt"),
		"new conflict\n",
	)
	writeLayoutTestFile(
		t,
		filepath.Join(canonicalUserRoot, "workspace", "agent-a", "identical.txt"),
		"same\n",
	)
	writeLayoutTestFile(
		t,
		filepath.Join(recoveryUserRoot, "workspace", "agent-a", "identical.txt"),
		"same\n",
	)
	writeLayoutTestFile(
		t,
		filepath.Join(recoveryUserRoot, "workspace", "agent-new", "AGENTS.md"),
		"new agent\n",
	)
	legacyOverlayLine := "{\"conversation_id\":\"conversation-a\",\"message_id\":\"legacy-message\"}\n"
	newOverlayLine := "{\"conversation_id\":\"conversation-a\",\"message_id\":\"new-message\"}\n"
	writeLayoutTestFile(
		t,
		filepath.Join(canonicalUserRoot, "state", "rooms", "room-conversation-a", "overlay.jsonl"),
		legacyOverlayLine,
	)
	writeLayoutTestFile(
		t,
		filepath.Join(recoveryUserRoot, "state", "rooms", "room-conversation-a", "overlay.jsonl"),
		legacyOverlayLine+newOverlayLine,
	)

	if err := MergeSkippedStateLayoutUsers(stateRoot, discardMigrationLogger()); err != nil {
		t.Fatal(err)
	}
	assertMigrationFileContent(
		t,
		filepath.Join(canonicalUserRoot, "workspace", "agent-a", "conflict.txt"),
		"legacy wins\n",
	)
	assertMigrationFileContent(
		t,
		filepath.Join(recoveryUserRoot, "workspace", "agent-a", "conflict.txt"),
		"new conflict\n",
	)
	if _, err := os.Stat(filepath.Join(recoveryUserRoot, "workspace", "agent-a", "identical.txt")); !os.IsNotExist(err) {
		t.Fatalf("相同文件未从隔离分支去重: %v", err)
	}
	assertMigrationFileContent(
		t,
		filepath.Join(canonicalUserRoot, "workspace", "agent-new", "AGENTS.md"),
		"new agent\n",
	)
	assertMigrationFileContent(
		t,
		filepath.Join(canonicalUserRoot, "state", "rooms", "room-conversation-a", "overlay.jsonl"),
		legacyOverlayLine+newOverlayLine,
	)
	if err := MergeSkippedStateLayoutUsers(stateRoot, discardMigrationLogger()); err != nil {
		t.Fatalf("重复合并用户文件失败: %v", err)
	}
}

func createSkippedLayoutMergeDatabase(t *testing.T, path string, inserts []string) {
	t.Helper()
	cfg := config.Config{DatabaseDriver: "sqlite", DatabaseURL: path}
	db, err := storage.OpenMigrationDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	statements := []string{
		`CREATE TABLE agents (id TEXT PRIMARY KEY, label TEXT NOT NULL)`,
		`CREATE TABLE rooms (id TEXT PRIMARY KEY, agent_id TEXT NOT NULL REFERENCES agents(id))`,
		`CREATE TABLE conversations (id TEXT PRIMARY KEY, room_id TEXT NOT NULL REFERENCES rooms(id))`,
		`CREATE TABLE goose_db_version (id INTEGER PRIMARY KEY, version_id INTEGER NOT NULL, is_applied INTEGER NOT NULL)`,
		`INSERT INTO goose_db_version (version_id, is_applied) VALUES (60, 1)`,
	}
	statements = append(statements, inserts...)
	for _, statement := range statements {
		if _, err = db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
}

type layoutReleaseRecord struct {
	agentID        string
	agentName      string
	workspacePath  string
	roomID         string
	conversationID string
}

func createLayoutReleaseDatabase(
	t *testing.T,
	path string,
	schemaVersion int64,
	record layoutReleaseRecord,
) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	migrationDirectory := providerRecoveryMigrationDir(t)
	if schemaVersion == 0 {
		err = goose.Up(db, migrationDirectory)
	} else {
		err = goose.UpTo(db, migrationDirectory, schemaVersion)
	}
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`
INSERT INTO agents (
    id, slug, name, description, definition, status, workspace_path, owner_user_id, is_main
) VALUES (?, ?, ?, '', '', 'active', ?, '__system__', 0)`,
		record.agentID,
		record.agentID,
		record.agentName,
		record.workspacePath,
	); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`
INSERT INTO rooms (id, room_type, name, description, owner_user_id)
VALUES (?, 'room', ?, '', '__system__')`, record.roomID, record.agentName); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`
INSERT INTO conversations (id, room_id, conversation_type, title)
VALUES (?, ?, 'room_main', ?)`, record.conversationID, record.roomID, record.agentName); err != nil {
		t.Fatal(err)
	}
}

func upgradeLayoutReleaseDatabase(t *testing.T, path string) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err = goose.Up(db, providerRecoveryMigrationDir(t)); err != nil {
		t.Fatal(err)
	}
}

func assertLayoutReleaseRows(t *testing.T, cfg config.Config) {
	t.Helper()
	db, err := storage.OpenDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, query := range []string{
		`SELECT COUNT(*) FROM agents WHERE id IN ('legacy-agent', 'new-agent')`,
		`SELECT COUNT(*) FROM rooms WHERE id IN ('legacy-room', 'new-room')`,
		`SELECT COUNT(*) FROM conversations WHERE id IN ('legacy-conversation', 'new-conversation')`,
	} {
		var count int
		if err = db.QueryRow(query).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 2 {
			t.Fatalf("v0.1.28/v0.1.30 分支记录未完整恢复: query=%s count=%d", query, count)
		}
	}
}

func assertSkippedLayoutMergeRows(t *testing.T, cfg config.Config) {
	t.Helper()
	db, err := storage.OpenDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var label string
	if err = db.QueryRow(`SELECT label FROM agents WHERE id = 'shared-agent'`).Scan(&label); err != nil {
		t.Fatal(err)
	}
	if label != "legacy" {
		t.Fatalf("历史冲突记录被覆盖: %q", label)
	}
	for _, query := range []string{
		`SELECT COUNT(*) FROM agents WHERE id = 'new-agent'`,
		`SELECT COUNT(*) FROM rooms WHERE id = 'new-room'`,
		`SELECT COUNT(*) FROM conversations WHERE id = 'new-conversation'`,
	} {
		var count int
		if err = db.QueryRow(query).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("新分支记录未合并: query=%s count=%d", query, count)
		}
	}
}

func writeLayoutTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
