package migration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

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

func TestRunDesktopStateRootRebasePreservesExternalPaths(t *testing.T) {
	root := t.TempDir()
	previousRoot := filepath.Join(root, "old", ".nexus")
	currentRoot := filepath.Join(root, "new", "NexusData")
	externalWorkspace := filepath.Join(root, "external", "agent-a")
	configureStateRootMigrationTest(t, previousRoot, currentRoot)
	cfg := config.Config{
		AppMode:        "desktop",
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(currentRoot, "app", "data", "nexus.db"),
	}
	seedStateRootMigrationDatabase(t, cfg, "agent-a", "owner-a", externalWorkspace, externalWorkspace)

	if err := RunDesktopStateRootRebase(context.Background(), cfg, nil); err != nil {
		t.Fatal(err)
	}
	db, err := storage.OpenDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var workspacePath string
	if err = db.QueryRow(`SELECT workspace_path FROM agents WHERE id = 'agent-a'`).Scan(&workspacePath); err != nil {
		t.Fatal(err)
	}
	if workspacePath != externalWorkspace {
		t.Fatalf("外部 workspace 被错误重写: %q", workspacePath)
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
