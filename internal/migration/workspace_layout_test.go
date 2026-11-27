// workspace_layout_test.go 覆盖工作区布局、文件、Room 文件与公共 Skill 投影迁移测试。
package migration

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
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
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
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

type fakeConversationOwnerLookup map[string]string

func (f fakeConversationOwnerLookup) LookupConversationOwnerUserID(
	_ context.Context,
	conversationID string,
) (string, error) {
	return f[conversationID], nil
}

func TestMigrateLegacyRoomFilesSplitsStateByOwner(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv(appfs.NexusStateRootEnvName, stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	legacyRoot := filepath.Join(stateRoot, "app", "rooms")
	if err := os.MkdirAll(filepath.Join(legacyRoot, "room-conversation-a"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(legacyRoot, "room-conversation-a", "overlay.jsonl"),
		[]byte("{\"conversation_id\":\"conversation-a\",\"message_id\":\"message-a\"}\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	legacyAttachment := filepath.Join(
		legacyRoot,
		"room-conversation-a",
		"attachments",
		"batch-a",
		"brief.txt",
	)
	if err := os.MkdirAll(filepath.Dir(legacyAttachment), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyAttachment, []byte("room attachment"), 0o600); err != nil {
		t.Fatal(err)
	}
	legacyDirectory := filepath.Join(legacyRoot, "Y29udmVyc2F0aW9uLWxlZ2FjeQ")
	if err := os.MkdirAll(legacyDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		filepath.Join(legacyDirectory, "messages.jsonl"),
		[]byte("{\"conversation_id\":\"conversation-legacy\",\"message_id\":\"legacy-message\",\"owner_user_id\":\"stale-owner\"}\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	targetOverlay := filepath.Join(
		appfs.UserRoomRootAt(stateRoot, "user-a"),
		"room-conversation-a",
		"overlay.jsonl",
	)
	if err := os.MkdirAll(filepath.Dir(targetOverlay), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(
		targetOverlay,
		[]byte(
			"{\"conversation_id\":\"conversation-a\",\"message_id\":\"existing\"}\n"+
				"{\"conversation_id\":\"conversation-a\",\"message_id\":\"message-a\"}\n",
		),
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	wakes := strings.Join([]string{
		`{"action":"schedule","wake":{"wake_id":"wake-a","owner_user_id":"stale-owner","message":{"conversation_id":"conversation-a"}}}`,
		`{"action":"complete","wake_id":"wake-a"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(legacyRoot, "directed_message_wakes.jsonl"), []byte(wakes), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyRoot, "orphan.bin"), []byte("orphan"), 0o600); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(legacyRoot)
	if err != nil {
		t.Fatal(err)
	}
	result, err := migrateLegacyRoomFiles(
		context.Background(),
		stateRoot,
		legacyRoot,
		entries,
		fakeConversationOwnerLookup{
			"conversation-a":      "user-a",
			"conversation-legacy": "__system__",
		},
		discardMigrationLogger(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.conversations != 2 || result.wakeEvents != 2 || result.quarantined != 1 {
		t.Fatalf("迁移统计不正确: %+v", result)
	}
	if _, err = os.Stat(legacyRoot); !os.IsNotExist(err) {
		t.Fatalf("旧 Room 根未移除: %v", err)
	}
	overlay, err := os.ReadFile(targetOverlay)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(overlay), `"message_id":"message-a"`) != 1 {
		t.Fatalf("重试合并产生重复消息: %s", overlay)
	}
	if !strings.Contains(string(overlay), `"message_id":"existing"`) {
		t.Fatalf("已有目标消息丢失: %s", overlay)
	}
	assetTarget := filepath.Join(
		appfs.UserRoomAssetsRootAt(stateRoot, "user-a"),
		"room-conversation-a",
		"attachments",
		"batch-a",
		"brief.txt",
	)
	if content, readErr := os.ReadFile(assetTarget); readErr != nil || string(content) != "room attachment" {
		t.Fatalf("Room 附件未迁入 owner workspace: content=%q err=%v", content, readErr)
	}
	if _, statErr := os.Stat(filepath.Join(
		appfs.UserRoomRootAt(stateRoot, "user-a"),
		"room-conversation-a",
		"attachments",
	)); !os.IsNotExist(statErr) {
		t.Fatalf("Room 附件不应留在宿主 ledger 根: %v", statErr)
	}
	legacyOverlay := filepath.Join(
		appfs.UserRoomRootAt(stateRoot, "__system__"),
		"room-conversation-legacy",
		"overlay.jsonl",
	)
	if content, readErr := os.ReadFile(legacyOverlay); readErr != nil ||
		!strings.Contains(string(content), `"message_id":"legacy-message"`) {
		t.Fatalf("旧 messages.jsonl 未归并为 overlay: content=%q err=%v", content, readErr)
	} else if strings.Contains(string(content), "stale-owner") ||
		!strings.Contains(string(content), `"owner_user_id":"__system__"`) {
		t.Fatalf("Room ledger owner 未按数据库归属重写: %s", content)
	}
	wakeTarget := filepath.Join(appfs.UserRoomRootAt(stateRoot, "user-a"), "directed_message_wakes.jsonl")
	wakeContent, err := os.ReadFile(wakeTarget)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(strings.TrimSpace(string(wakeContent)), "\n") != 1 {
		t.Fatalf("唤醒事件未按 owner 完整迁移: %s", wakeContent)
	}
	if strings.Contains(string(wakeContent), "stale-owner") ||
		!strings.Contains(string(wakeContent), `"owner_user_id":"user-a"`) {
		t.Fatalf("唤醒事件 owner 未按数据库归属重写: %s", wakeContent)
	}
	if _, statErr := os.Stat(filepath.Join(
		appfs.UserRoomRootAt(stateRoot, "stale-owner"),
		"directed_message_wakes.jsonl",
	)); !os.IsNotExist(statErr) {
		t.Fatalf("旧 wake 中的 owner 不应覆盖数据库归属: %v", statErr)
	}
	quarantined := filepath.Join(stateRoot, "app", ".migration-quarantine", "room-state-v1", "orphan.bin")
	if content, readErr := os.ReadFile(quarantined); readErr != nil || string(content) != "orphan" {
		t.Fatalf("未知宿主文件未隔离保存: content=%q err=%v", content, readErr)
	}
}

func TestResolveLegacyRoomDirectoryOwnerSupportsBase64Name(t *testing.T) {
	root := t.TempDir()
	directoryName := "Y29udmVyc2F0aW9uLWxlZ2FjeQ"
	sourcePath := filepath.Join(root, directoryName)
	if err := os.MkdirAll(sourcePath, 0o700); err != nil {
		t.Fatal(err)
	}
	conversationID, ownerUserID, err := resolveLegacyRoomDirectoryOwner(
		context.Background(),
		sourcePath,
		directoryName,
		fakeConversationOwnerLookup{"conversation-legacy": "__system__"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if conversationID != "conversation-legacy" || ownerUserID != "__system__" {
		t.Fatalf("旧目录名解析失败: conversation=%q owner=%q", conversationID, ownerUserID)
	}
}

func TestMigrateLegacyRoomFilesQuarantinesMixedConversationDirectory(t *testing.T) {
	stateRoot := t.TempDir()
	legacyRoot := filepath.Join(stateRoot, "app", "rooms")
	sourceRoot := filepath.Join(legacyRoot, "room-conversation-a")
	if err := os.MkdirAll(sourceRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	content := strings.Join([]string{
		`{"conversation_id":"conversation-a","message_id":"message-a"}`,
		`{"conversation_id":"conversation-b","message_id":"message-b"}`,
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(sourceRoot, "overlay.jsonl"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(legacyRoot)
	if err != nil {
		t.Fatal(err)
	}
	result, err := migrateLegacyRoomFiles(
		context.Background(),
		stateRoot,
		legacyRoot,
		entries,
		fakeConversationOwnerLookup{
			"conversation-a": "user-a",
			"conversation-b": "user-b",
		},
		discardMigrationLogger(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.conversations != 0 || result.quarantined != 1 {
		t.Fatalf("混合归属目录应完整隔离: %+v", result)
	}
	quarantined := filepath.Join(
		stateRoot,
		"app",
		".migration-quarantine",
		"room-state-v1",
		"room-conversation-a",
		"overlay.jsonl",
	)
	if quarantinedContent, readErr := os.ReadFile(quarantined); readErr != nil ||
		string(quarantinedContent) != content {
		t.Fatalf("隔离目录内容不完整: content=%q err=%v", quarantinedContent, readErr)
	}
	if _, statErr := os.Stat(filepath.Join(
		appfs.UserRoomRootAt(stateRoot, "user-a"),
		"room-conversation-a",
	)); !os.IsNotExist(statErr) {
		t.Fatalf("混合归属目录不得产生部分目标状态: %v", statErr)
	}
}

func TestMigrateLegacyRoomFilesQuarantinesHardLinkedState(t *testing.T) {
	stateRoot := t.TempDir()
	legacyRoot := filepath.Join(stateRoot, "app", "rooms")
	sourceRoot := filepath.Join(legacyRoot, "room-conversation-a")
	if err := os.MkdirAll(sourceRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	overlayPath := filepath.Join(sourceRoot, "overlay.jsonl")
	if err := os.WriteFile(
		overlayPath,
		[]byte("{\"conversation_id\":\"conversation-a\",\"message_id\":\"message-a\"}\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(overlayPath, filepath.Join(sourceRoot, "overlay-copy.jsonl")); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(legacyRoot)
	if err != nil {
		t.Fatal(err)
	}
	result, err := migrateLegacyRoomFiles(
		context.Background(),
		stateRoot,
		legacyRoot,
		entries,
		fakeConversationOwnerLookup{"conversation-a": "user-a"},
		discardMigrationLogger(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.conversations != 0 || result.quarantined != 1 {
		t.Fatalf("硬链接状态应完整隔离: %+v", result)
	}
	quarantined := filepath.Join(
		stateRoot,
		"app",
		".migration-quarantine",
		"room-state-v1",
		"room-conversation-a",
	)
	firstInfo, err := os.Stat(filepath.Join(quarantined, "overlay.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	secondInfo, err := os.Stat(filepath.Join(quarantined, "overlay-copy.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(firstInfo, secondInfo) {
		t.Fatal("隔离过程不应跟随或拆解硬链接")
	}
}

func TestPublicSkillProjectionMigrationRetriesWithoutDataLoss(t *testing.T) {
	state := t.TempDir()
	app := filepath.Join(state, "app")
	ctx := workspaceFileMigrationContext{configRoot: app}
	for _, name := range []string{"platform-skills", "host-skills"} {
		writeMigrationTestFile(t, filepath.Join(app, name, "SKILL.md"), name)
	}
	if n, err := migratePublicSkillProjections(ctx); err != nil || n != 2 {
		t.Fatalf("moved=%d err=%v", n, err)
	}
	if n, err := migratePublicSkillProjections(ctx); err != nil || n != 0 {
		t.Fatalf("retry moved=%d err=%v", n, err)
	}
	for _, name := range []string{"platform-skills", "host-skills"} {
		assertMigrationFileContent(t, filepath.Join(state, "skills-public", name, "SKILL.md"), name)
		assertMigrationPathMissing(t, filepath.Join(app, name))
	}
	writeMigrationTestFile(t, filepath.Join(app, "host-skills", "SKILL.md"), "keep both")
	if _, err := migratePublicSkillProjections(ctx); err == nil {
		t.Fatal("conflicting projection overwritten")
	}
	assertMigrationFileContent(t, filepath.Join(app, "host-skills", "SKILL.md"), "keep both")
}

func TestPublicSkillProjectionMigrationRejectsLinkedDestination(t *testing.T) {
	state, outside := t.TempDir(), t.TempDir()
	app := filepath.Join(state, "app")
	writeMigrationTestFile(t, filepath.Join(app, "platform-skills", "SKILL.md"), "keep")
	if err := os.Symlink(outside, filepath.Join(state, "skills-public")); err != nil {
		t.Skip(err)
	}
	if _, err := migratePublicSkillProjections(workspaceFileMigrationContext{configRoot: app}); err == nil {
		t.Fatal("followed linked destination")
	}
	assertMigrationFileContent(t, filepath.Join(app, "platform-skills", "SKILL.md"), "keep")
}
