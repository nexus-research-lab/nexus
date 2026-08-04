package migration

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/storage"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	"github.com/pressly/goose/v3"
)

func TestRunStateLayoutRecoversSkippedDesktopUpgrade(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	t.Setenv("NEXUS_APP_MODE", "desktop")
	legacyDatabase := filepath.Join(stateRoot, "data", skippedStateLayoutDatabaseName)
	canonicalDatabase := filepath.Join(stateRoot, "app", "data", skippedStateLayoutDatabaseName)
	writeLayoutTestFile(t, legacyDatabase, "legacy database")
	writeLayoutTestFile(t, canonicalDatabase, "new canonical database")
	writeLayoutTestFile(t, filepath.Join(stateRoot, "rooms", "room-a", "overlay.jsonl"), "old room\n")

	if err := RunStateLayout(stateRoot, discardMigrationLogger()); err != nil {
		t.Fatal(err)
	}
	assertMigrationFileContent(t, canonicalDatabase, "legacy database")
	assertMigrationFileContent(
		t,
		filepath.Join(skippedStateLayoutRecoveryDataRoot(stateRoot), skippedStateLayoutDatabaseName),
		"new canonical database",
	)
	assertMigrationFileContent(
		t,
		filepath.Join(stateRoot, "app", "rooms", "room-a", "overlay.jsonl"),
		"old room\n",
	)
	if err := RunStateLayout(stateRoot, discardMigrationLogger()); err != nil {
		t.Fatalf("重复执行状态迁移缺口恢复失败: %v", err)
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

func TestMergeSkippedStateLayoutDatabaseSupportsCurrentSchema(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	t.Setenv(appfs.NexusStateRootEnvName, stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	canonicalDatabase := filepath.Join(stateRoot, "app", "data", skippedStateLayoutDatabaseName)
	recoveryDatabase := filepath.Join(
		skippedStateLayoutRecoveryDataRoot(stateRoot),
		skippedStateLayoutDatabaseName,
	)
	createCurrentSchemaMergeDatabase(t, canonicalDatabase, "legacy", false)
	createCurrentSchemaMergeDatabase(t, recoveryDatabase, "new", true)
	cfg := config.Config{DatabaseDriver: "sqlite", DatabaseURL: canonicalDatabase}

	if err := MergeSkippedStateLayoutDatabase(context.Background(), cfg, discardMigrationLogger()); err != nil {
		t.Fatal(err)
	}
	db, err := storage.OpenDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var sharedName string
	if err = db.QueryRow(`SELECT name FROM agents WHERE id = 'shared-agent'`).Scan(&sharedName); err != nil {
		t.Fatal(err)
	}
	if sharedName != "legacy" {
		t.Fatalf("当前 schema 合并覆盖了历史 Agent: %q", sharedName)
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
			t.Fatalf("当前 schema 新分支记录未合并: query=%s count=%d", query, count)
		}
	}
}

func TestRunRoomFilesReplaysWhenSourceAppearsAfterMarker(t *testing.T) {
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	t.Setenv(appfs.NexusStateRootEnvName, stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	databasePath := filepath.Join(stateRoot, "app", "data", skippedStateLayoutDatabaseName)
	cfg := config.Config{DatabaseDriver: "sqlite", DatabaseURL: databasePath}
	db, err := storage.OpenMigrationDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE rooms (id TEXT PRIMARY KEY, owner_user_id TEXT NOT NULL)`,
		`CREATE TABLE conversations (id TEXT PRIMARY KEY, room_id TEXT NOT NULL)`,
		`INSERT INTO rooms (id, owner_user_id) VALUES ('room-a', 'user-a')`,
		`INSERT INTO conversations (id, room_id) VALUES ('conversation-a', 'room-a')`,
	} {
		if _, err = db.Exec(statement); err != nil {
			db.Close()
			t.Fatal(err)
		}
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	markerPath := workspaceFileMigrationMarker(filepath.Join(stateRoot, "app"), roomStateMigrationName)
	if err = writeWorkspaceFileMigrationMarker(markerPath); err != nil {
		t.Fatal(err)
	}
	legacyOverlay := filepath.Join(stateRoot, "app", "rooms", "room-conversation-a", "overlay.jsonl")
	writeLayoutTestFile(
		t,
		legacyOverlay,
		"{\"conversation_id\":\"conversation-a\",\"message_id\":\"message-a\"}\n",
	)

	if err = RunRoomFiles(context.Background(), cfg, discardMigrationLogger()); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(stateRoot, "app", "rooms")); !os.IsNotExist(err) {
		t.Fatalf("旧 Room 根未清理: %v", err)
	}
	assertMigrationFileContent(
		t,
		filepath.Join(appfs.UserRoomRootAt(stateRoot, "user-a"), "room-conversation-a", "overlay.jsonl"),
		"{\"conversation_id\":\"conversation-a\",\"message_id\":\"message-a\"}\n",
	)
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

func createCurrentSchemaMergeDatabase(t *testing.T, path string, name string, includeNew bool) {
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
	if err = goose.Up(db, providerRecoveryMigrationDir(t)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`
INSERT INTO agents (
    id, slug, name, description, definition, status, workspace_path, owner_user_id, is_main
) VALUES ('shared-agent', 'shared-agent', ?, '', '', 'active', '/tmp/shared-agent', '__system__', 0)`, name); err != nil {
		t.Fatal(err)
	}
	roomID := name + "-branch-room"
	conversationID := name + "-branch-conversation"
	if _, err = db.Exec(`
INSERT INTO rooms (id, room_type, name, description, owner_user_id)
VALUES (?, 'room', ?, '', '__system__')`, roomID, name); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`
INSERT INTO conversations (id, room_id, conversation_type, title)
VALUES (?, ?, 'room_main', ?)`, conversationID, roomID, name); err != nil {
		t.Fatal(err)
	}
	if !includeNew {
		return
	}
	if _, err = db.Exec(`
INSERT INTO agents (
    id, slug, name, description, definition, status, workspace_path, owner_user_id, is_main
) VALUES ('new-agent', 'new-agent', 'new', '', '', 'active', '/tmp/new-agent', '__system__', 0)`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`
INSERT INTO rooms (id, room_type, name, description, owner_user_id)
VALUES ('new-room', 'room', 'new', '', '__system__')`); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`
INSERT INTO conversations (id, room_id, conversation_type, title)
VALUES ('new-conversation', 'new-room', 'room_main', 'new')`); err != nil {
		t.Fatal(err)
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
