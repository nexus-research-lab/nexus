// INPUT: 已迁移到 app/data 的数据库与旧版 workspace 目录。
// OUTPUT: 验证按 owner 搬迁文件、更新 Agent 路径和冲突失败恢复。
// POS: workspace 布局迁移的文件系统与数据库一致性回归测试。
package migration

import (
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	workspacepkg "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func TestRunWorkspaceLayoutMigratesOwnersAndAgentPaths(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	databaseURL := filepath.Join(stateRoot, "app", "data", "nexus.db")
	legacyRoot := filepath.Join(stateRoot, "workspace")
	systemWorkspace := filepath.Join(legacyRoot, "nexus")
	userWorkspace := filepath.Join(legacyRoot, "user_demo", "agent-a")
	orphanWorkspace := filepath.Join(legacyRoot, "orphan", "agent-b")
	writeMigrationTestFile(t, filepath.Join(systemWorkspace, "system.txt"), "system\n")
	writeMigrationTestFile(t, filepath.Join(userWorkspace, "user.txt"), "user\n")
	writeMigrationTestFile(t, filepath.Join(orphanWorkspace, "orphan.txt"), "orphan\n")
	legacyProjectName := workspacepkg.TranscriptProjectDirectoryName(userWorkspace)
	userTarget := filepath.Join(stateRoot, "users", "user_demo", "workspace", "agent-a")
	writeMigrationTestFile(
		t,
		filepath.Join(
			stateRoot,
			"users",
			authctx.SystemUserID,
			"runtime",
			"projects",
			legacyProjectName,
			"session.jsonl",
		),
		"transcript\n",
	)

	db := createWorkspaceLayoutDB(t, databaseURL)
	insertWorkspaceLayoutUser(t, db, "user_demo")
	insertWorkspaceLayoutAgent(t, db, "nexus", authctx.SystemUserID, systemWorkspace)
	insertWorkspaceLayoutAgent(t, db, "agent-a", "user_demo", userWorkspace)
	if err := db.Close(); err != nil {
		t.Fatalf("关闭迁移准备数据库失败: %v", err)
	}

	cfg := config.Config{DatabaseDriver: "sqlite", DatabaseURL: databaseURL}
	if err := RunWorkspaceLayout(t.Context(), cfg, stateRoot, discardMigrationLogger()); err != nil {
		t.Fatalf("执行 workspace 布局迁移失败: %v", err)
	}

	systemTarget := filepath.Join(stateRoot, "users", authctx.SystemUserID, "workspace", "nexus")
	orphanTarget := filepath.Join(
		stateRoot,
		"users",
		authctx.SystemUserID,
		"workspace",
		"orphan",
		"agent-b",
	)
	targetProjectName := workspacepkg.TranscriptProjectDirectoryName(userTarget)
	assertMigrationFileContent(t, filepath.Join(systemTarget, "system.txt"), "system\n")
	assertMigrationFileContent(t, filepath.Join(userTarget, "user.txt"), "user\n")
	assertMigrationFileContent(
		t,
		filepath.Join(
			stateRoot,
			"users",
			"user_demo",
			"runtime",
			"projects",
			targetProjectName,
			"session.jsonl",
		),
		"transcript\n",
	)
	assertMigrationFileContent(t, filepath.Join(orphanTarget, "orphan.txt"), "orphan\n")
	assertMigrationPathMissing(t, legacyRoot)
	assertCompletedMigrationMarker(t, filepath.Join(stateRoot, "app"), workspaceLayoutMigrationName)

	db = openWorkspaceLayoutDB(t, databaseURL)
	defer db.Close()
	assertWorkspaceLayoutAgentPath(t, db, "nexus", systemTarget)
	assertWorkspaceLayoutAgentPath(t, db, "agent-a", userTarget)

	if err := RunWorkspaceLayout(t.Context(), cfg, stateRoot, discardMigrationLogger()); err != nil {
		t.Fatalf("重复执行 workspace 布局迁移失败: %v", err)
	}
}

func TestRunWorkspaceLayoutLeavesPermissionsToIsolationLauncher(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows 不提供 Unix 权限位语义")
	}

	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	databaseURL := filepath.Join(stateRoot, "app", "data", "nexus.db")
	usersRoot := filepath.Join(stateRoot, "users")
	ownerRoot := filepath.Join(usersRoot, "user_demo")
	legacyWorkspace := filepath.Join(stateRoot, "workspace", "user_demo", "agent-a")
	legacyFile := filepath.Join(legacyWorkspace, "state.json")
	writeMigrationTestFile(t, legacyFile, "legacy\n")
	for _, directory := range []string{usersRoot, ownerRoot, legacyWorkspace} {
		if err := os.MkdirAll(directory, 0o770); err != nil {
			t.Fatalf("创建强隔离 workspace 目录失败 %q: %v", directory, err)
		}
		if err := os.Chmod(directory, 0o770); err != nil {
			t.Fatalf("设置强隔离 workspace 目录权限失败 %q: %v", directory, err)
		}
	}
	if err := os.Chmod(legacyFile, 0o660); err != nil {
		t.Fatalf("设置强隔离 workspace 文件权限失败: %v", err)
	}

	db := createWorkspaceLayoutDB(t, databaseURL)
	insertWorkspaceLayoutUser(t, db, "user_demo")
	insertWorkspaceLayoutAgent(t, db, "agent-a", "user_demo", legacyWorkspace)
	if err := db.Close(); err != nil {
		t.Fatalf("关闭迁移准备数据库失败: %v", err)
	}

	cfg := config.Config{DatabaseDriver: "sqlite", DatabaseURL: databaseURL}
	if err := runWorkspaceLayout(
		t.Context(),
		cfg,
		stateRoot,
		discardMigrationLogger(),
		true,
	); err != nil {
		t.Fatalf("launcher 管理权限时执行 workspace 迁移失败: %v", err)
	}

	targetWorkspace := filepath.Join(ownerRoot, "workspace", "agent-a")
	targetFile := filepath.Join(targetWorkspace, "state.json")
	assertMigrationFileContent(t, targetFile, "legacy\n")
	for _, directory := range []string{usersRoot, ownerRoot, targetWorkspace} {
		info, err := os.Stat(directory)
		if err != nil {
			t.Fatalf("读取强隔离 workspace 目录权限失败 %q: %v", directory, err)
		}
		if info.Mode().Perm() != 0o770 {
			t.Fatalf("workspace 迁移不应覆盖 launcher 目录权限 %q: %o", directory, info.Mode().Perm())
		}
	}
	fileInfo, err := os.Stat(targetFile)
	if err != nil {
		t.Fatalf("读取强隔离 workspace 文件权限失败: %v", err)
	}
	if fileInfo.Mode().Perm() != 0o660 {
		t.Fatalf("workspace 迁移不应覆盖 launcher 文件权限: %o", fileInfo.Mode().Perm())
	}
	assertCompletedMigrationMarker(t, filepath.Join(stateRoot, "app"), workspaceLayoutMigrationName)
}

func TestRunWorkspaceLayoutRejectsConflictingWorkspace(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	databaseURL := filepath.Join(stateRoot, "app", "data", "nexus.db")
	legacyWorkspace := filepath.Join(stateRoot, "workspace", "user_demo", "agent-a")
	targetWorkspace := filepath.Join(stateRoot, "users", "user_demo", "workspace", "agent-a")
	sourcePath := filepath.Join(legacyWorkspace, "state.json")
	targetPath := filepath.Join(targetWorkspace, "state.json")
	writeMigrationTestFile(t, sourcePath, "legacy\n")
	writeMigrationTestFile(t, targetPath, "current\n")

	db := createWorkspaceLayoutDB(t, databaseURL)
	insertWorkspaceLayoutUser(t, db, "user_demo")
	insertWorkspaceLayoutAgent(t, db, "agent-a", "user_demo", legacyWorkspace)
	if err := db.Close(); err != nil {
		t.Fatalf("关闭迁移准备数据库失败: %v", err)
	}

	cfg := config.Config{DatabaseDriver: "sqlite", DatabaseURL: databaseURL}
	if err := RunWorkspaceLayout(t.Context(), cfg, stateRoot, discardMigrationLogger()); err == nil {
		t.Fatal("workspace 内容冲突时应返回错误")
	}

	assertMigrationFileContent(t, sourcePath, "legacy\n")
	assertMigrationFileContent(t, targetPath, "current\n")
	if _, err := os.Stat(workspaceFileMigrationMarker(filepath.Join(stateRoot, "app"), workspaceLayoutMigrationName)); !os.IsNotExist(err) {
		t.Fatalf("失败迁移不应写完成标记: %v", err)
	}
	db = openWorkspaceLayoutDB(t, databaseURL)
	defer db.Close()
	assertWorkspaceLayoutAgentPath(t, db, "agent-a", legacyWorkspace)
}

func TestRunWorkspaceLayoutNormalizesLegacyOwnerPathSegments(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	databaseURL := filepath.Join(stateRoot, "app", "data", "nexus.db")
	ownerUserID := "owner/a"
	legacyWorkspace := filepath.Join(stateRoot, "workspace", "owner_a", "agent-a")
	targetWorkspace := filepath.Join(
		stateRoot,
		"users",
		"owner_a-1844893a",
		"workspace",
		"agent-a",
	)
	writeMigrationTestFile(t, filepath.Join(legacyWorkspace, "state.json"), "legacy\n")

	db := createWorkspaceLayoutDB(t, databaseURL)
	insertWorkspaceLayoutUser(t, db, ownerUserID)
	insertWorkspaceLayoutAgent(t, db, "agent-a", ownerUserID, legacyWorkspace)
	if err := db.Close(); err != nil {
		t.Fatalf("关闭迁移准备数据库失败: %v", err)
	}

	cfg := config.Config{DatabaseDriver: "sqlite", DatabaseURL: databaseURL}
	if err := RunWorkspaceLayout(t.Context(), cfg, stateRoot, discardMigrationLogger()); err != nil {
		t.Fatalf("执行包含路径分隔符 owner 的迁移失败: %v", err)
	}

	assertMigrationFileContent(t, filepath.Join(targetWorkspace, "state.json"), "legacy\n")
	assertMigrationPathMissing(t, legacyWorkspace)
	db = openWorkspaceLayoutDB(t, databaseURL)
	defer db.Close()
	assertWorkspaceLayoutAgentPath(t, db, "agent-a", targetWorkspace)
}

func TestRunWorkspaceLayoutNormalizesJSONQuotedOwnerIDs(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	databaseURL := filepath.Join(stateRoot, "app", "data", "nexus.db")
	ownerUserID := "user_83b9da59d344b590"
	quotedOwnerUserID := strconv.Quote(ownerUserID)
	legacyWorkspace := filepath.Join(stateRoot, "workspace", ownerUserID, "agent-a")
	targetWorkspace := filepath.Join(
		stateRoot,
		"users",
		ownerUserID,
		"workspace",
		"agent-a",
	)
	writeMigrationTestFile(t, filepath.Join(legacyWorkspace, "state.json"), "legacy\n")

	db := createWorkspaceLayoutDB(t, databaseURL)
	insertWorkspaceLayoutUser(t, db, ownerUserID)
	insertWorkspaceLayoutUser(t, db, quotedOwnerUserID)
	insertWorkspaceLayoutAgent(t, db, "agent-a", quotedOwnerUserID, legacyWorkspace)
	if err := db.Close(); err != nil {
		t.Fatalf("关闭迁移准备数据库失败: %v", err)
	}

	cfg := config.Config{DatabaseDriver: "sqlite", DatabaseURL: databaseURL}
	if err := RunWorkspaceLayout(t.Context(), cfg, stateRoot, discardMigrationLogger()); err != nil {
		t.Fatalf("执行 JSON 引号 owner 迁移失败: %v", err)
	}

	assertMigrationFileContent(t, filepath.Join(targetWorkspace, "state.json"), "legacy\n")
	assertMigrationPathMissing(t, legacyWorkspace)
	db = openWorkspaceLayoutDB(t, databaseURL)
	defer db.Close()
	assertWorkspaceLayoutAgentPath(t, db, "agent-a", targetWorkspace)
	assertWorkspaceLayoutAgentOwner(t, db, "agent-a", ownerUserID)
}

func TestWorkspaceOwnerSourceNamesRejectTraversal(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	names := workspaceOwnerSourceNames(stateRoot, "../outside")
	if len(names) != 2 {
		t.Fatalf("包含路径穿越的 owner 不应生成 raw 候选: %#v", names)
	}
	for _, name := range names {
		if filepath.Clean(filepath.Join(stateRoot, "workspace", name)) !=
			filepath.Join(stateRoot, "workspace", name) {
			t.Fatalf("owner 候选不是稳定的相对路径: %#v", names)
		}
	}
}

func TestValidateWorkspaceOwnerSegmentsRejectsCollision(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	err := validateWorkspaceOwnerSegments(stateRoot, map[string]struct{}{
		"owner/a":          {},
		"owner_a-1844893a": {},
	})
	if err == nil {
		t.Fatal("owner 路径碰撞时应拒绝迁移")
	}
}

func TestValidateLegacyWorkspaceOwnerSourcesRejectsOverlap(t *testing.T) {
	err := validateLegacyWorkspaceOwnerSources(map[string]struct{}{
		"owner":   {},
		"owner/a": {},
	})
	if err == nil {
		t.Fatal("旧 owner 路径互为父子时应拒绝迁移")
	}
}

func TestValidateWorkspaceLayoutAgentsRejectsCrossOwnerPath(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	agents := []workspaceLayoutAgent{{
		id:            "agent-a",
		ownerUserID:   "owner-a",
		workspacePath: filepath.Join(stateRoot, "users", "owner-b", "workspace", "agent-a"),
	}}
	if err := validateWorkspaceLayoutAgents(stateRoot, agents); err == nil {
		t.Fatal("workspace 指向其他 owner 目录时应拒绝迁移")
	}
}

func createWorkspaceLayoutDB(t *testing.T, databaseURL string) *sql.DB {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(databaseURL), 0o700); err != nil {
		t.Fatalf("创建测试数据库目录失败: %v", err)
	}
	db := openWorkspaceLayoutDB(t, databaseURL)
	for _, statement := range []string{
		`CREATE TABLE owner_profiles (owner_user_id TEXT PRIMARY KEY)`,
		`CREATE TABLE agents (
			id TEXT PRIMARY KEY,
			owner_user_id TEXT NOT NULL,
			workspace_path TEXT NOT NULL
		)`,
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("创建 workspace 布局测试表失败: %v", err)
		}
	}
	return db
}

func openWorkspaceLayoutDB(t *testing.T, databaseURL string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", databaseURL)
	if err != nil {
		t.Fatalf("打开 workspace 布局测试数据库失败: %v", err)
	}
	if err = db.Ping(); err != nil {
		_ = db.Close()
		t.Fatalf("连接 workspace 布局测试数据库失败: %v", err)
	}
	return db
}

func insertWorkspaceLayoutUser(t *testing.T, db *sql.DB, ownerUserID string) {
	t.Helper()
	if _, err := db.Exec(`INSERT INTO owner_profiles (owner_user_id) VALUES (?)`, ownerUserID); err != nil {
		t.Fatalf("插入 workspace 布局测试用户失败: %v", err)
	}
}

func insertWorkspaceLayoutAgent(
	t *testing.T,
	db *sql.DB,
	agentID string,
	ownerUserID string,
	workspacePath string,
) {
	t.Helper()
	if _, err := db.Exec(
		`INSERT INTO agents (id, owner_user_id, workspace_path) VALUES (?, ?, ?)`,
		agentID,
		ownerUserID,
		workspacePath,
	); err != nil {
		t.Fatalf("插入 workspace 布局测试 Agent 失败: %v", err)
	}
}

func assertWorkspaceLayoutAgentPath(
	t *testing.T,
	db *sql.DB,
	agentID string,
	expectedPath string,
) {
	t.Helper()
	var actualPath string
	if err := db.QueryRow(
		`SELECT workspace_path FROM agents WHERE id = ?`,
		agentID,
	).Scan(&actualPath); err != nil {
		t.Fatalf("读取 workspace 布局测试 Agent 失败: %v", err)
	}
	if actualPath != expectedPath {
		t.Fatalf("Agent %s workspace_path = %q, want %q", agentID, actualPath, expectedPath)
	}
}

func assertWorkspaceLayoutAgentOwner(
	t *testing.T,
	db *sql.DB,
	agentID string,
	expectedOwnerUserID string,
) {
	t.Helper()
	var actualOwnerUserID string
	if err := db.QueryRow(
		`SELECT owner_user_id FROM agents WHERE id = ?`,
		agentID,
	).Scan(&actualOwnerUserID); err != nil {
		t.Fatalf("读取 workspace 布局测试 Agent owner 失败: %v", err)
	}
	if actualOwnerUserID != expectedOwnerUserID {
		t.Fatalf(
			"Agent %s owner_user_id = %q, want %q",
			agentID,
			actualOwnerUserID,
			expectedOwnerUserID,
		)
	}
}
