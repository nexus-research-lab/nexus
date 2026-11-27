// domain_migrations_test.go 覆盖凭据、会话草稿修复、Provider 作用域恢复与废弃资源迁移测试。
package migration

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/connectors/credentials"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

const (
	migrationActiveKey  = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
	migrationLegacyKey  = "YWJjZGVmMDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODk="
	migrationUnknownKey = "enl4d3Z1dHNycXBvbm1sa2ppaGdmZWRjYmEwMTIzNDU="
)

func TestRunConnectorCredentialKeyMigrationResumesMixedKeyRows(t *testing.T) {
	databaseURL := filepath.Join(t.TempDir(), "connector-keyring.db")
	db, err := sql.Open("sqlite", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err = goose.Up(db, sqliteMigrationDir(t)); err != nil {
		t.Fatal(err)
	}

	activeKey := decodeMigrationKey(t, migrationActiveKey)
	legacyKey := decodeMigrationKey(t, migrationLegacyKey)
	unknownKey := decodeMigrationKey(t, migrationUnknownKey)
	activeKeyID := credentials.KeyID(activeKey)
	legacyKeyID := credentials.KeyID(legacyKey)
	unknownEncrypted := insertMigrationCredential(t, db, "unknown", unknownKey, "", "unknown")
	insertMigrationCredential(t, db, "active-current", activeKey, activeKeyID, "active-current")
	insertMigrationCredential(t, db, "active-unidentified", activeKey, "", "active-unidentified")
	insertMigrationCredential(t, db, "legacy-unidentified", legacyKey, "", "legacy-unidentified")
	insertMigrationCredential(t, db, "legacy-identified", legacyKey, legacyKeyID, "legacy-identified")
	if _, err = db.Exec(`
INSERT INTO connector_connections (
    owner_user_id, connector_id, state, enabled, credentials,
    credentials_encrypted, credentials_key_id, auth_type
) VALUES ('owner-1', 'custom-mcp:corrupt', 'connected', TRUE, '__encrypted__', 'v1:not-valid', ?, 'custom_mcp')`, legacyKeyID); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{
		DatabaseDriver:                 "sqlite",
		DatabaseURL:                    databaseURL,
		ConnectorCredentialsKey:        migrationActiveKey,
		ConnectorCredentialsLegacyKeys: []string{migrationLegacyKey},
	}
	report, err := RunConnectorCredentialKeyMigration(t.Context(), cfg, discardMigrationLogger())
	if err != nil {
		t.Fatal(err)
	}
	if report.Scanned != 6 || report.AlreadyCurrent != 1 || report.Identified != 1 ||
		report.Reencrypted != 2 || report.RecoveryRequired != 1 || report.Corrupt != 1 ||
		report.Conflicted != 0 || !report.KeyringAvailable {
		t.Fatalf("first migration report mismatch: %+v", report)
	}

	db, err = sql.Open("sqlite", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	for _, connectorID := range []string{
		"active-current",
		"active-unidentified",
		"legacy-unidentified",
		"legacy-identified",
	} {
		var encrypted string
		var keyID sql.NullString
		if err = db.QueryRow(`
SELECT credentials_encrypted, credentials_key_id
  FROM connector_connections
 WHERE connector_id = ?`, "custom-mcp:"+connectorID).Scan(&encrypted, &keyID); err != nil {
			t.Fatal(err)
		}
		plain, decryptErr := credentials.DecryptPayload(activeKey, encrypted)
		if decryptErr != nil || string(plain) != connectorID || !keyID.Valid || keyID.String != activeKeyID {
			t.Fatalf("%s was not migrated to active key: plain=%q key_id=%q err=%v", connectorID, plain, keyID.String, decryptErr)
		}
	}
	var unknownAfter string
	var unknownKeyID sql.NullString
	if err = db.QueryRow(`
SELECT credentials_encrypted, credentials_key_id
  FROM connector_connections
 WHERE connector_id = 'custom-mcp:unknown'`).Scan(&unknownAfter, &unknownKeyID); err != nil {
		t.Fatal(err)
	}
	if unknownAfter != unknownEncrypted || unknownKeyID.Valid {
		t.Fatalf("unknown-key row must remain untouched: key_id=%q", unknownKeyID.String)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}

	repeated, err := RunConnectorCredentialKeyMigration(t.Context(), cfg, discardMigrationLogger())
	if err != nil {
		t.Fatal(err)
	}
	if repeated.AlreadyCurrent != 4 || repeated.Identified != 0 || repeated.Reencrypted != 0 ||
		repeated.RecoveryRequired != 1 || repeated.Corrupt != 1 {
		t.Fatalf("repeated migration must resume idempotently: %+v", repeated)
	}
}

func decodeMigrationKey(t *testing.T, raw string) []byte {
	t.Helper()
	key, err := credentials.DecodeKey(raw)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func insertMigrationCredential(
	t *testing.T,
	db *sql.DB,
	suffix string,
	key []byte,
	keyID string,
	plain string,
) string {
	t.Helper()
	encrypted, err := credentials.EncryptPayload(key, []byte(plain))
	if err != nil {
		t.Fatal(err)
	}
	var storedKeyID any
	if keyID != "" {
		storedKeyID = keyID
	}
	if _, err = db.Exec(`
INSERT INTO connector_connections (
    owner_user_id, connector_id, state, enabled, credentials,
    credentials_encrypted, credentials_key_id, auth_type
) VALUES ('owner-1', ?, 'connected', TRUE, '__encrypted__', ?, ?, 'custom_mcp')`,
		"custom-mcp:"+suffix,
		encrypted,
		storedKeyID,
	); err != nil {
		t.Fatal(err)
	}
	return encrypted
}

func TestRunWorkspaceFilesRemovesRetiredSandboxResources(t *testing.T) {
	stateRoot := t.TempDir()
	appRoot := filepath.Join(stateRoot, "app")
	workspaceRoot := filepath.Join(stateRoot, "workspace")
	scratchBase := filepath.Join(stateRoot, "users", "owner-a", "runtime", "sandbox")
	leasedLeaf := filepath.Join(scratchBase, "lease-1")
	readOnlyCache := filepath.Join(leasedLeaf, "gomod", "pkg@v1")
	unmarkedLeaf := filepath.Join(scratchBase, "not-ours")
	otherOwnerBase := filepath.Join(stateRoot, "users", "owner-b", "runtime", "sandbox")
	keptRuntimeFile := filepath.Join(stateRoot, "users", "owner-a", "runtime", "settings.json")

	writeMigrationTestFile(t, filepath.Join(appRoot, "processes", "p", "job.json"), "{}\n")
	writeMigrationTestFile(t, filepath.Join(appRoot, "processes", "scratch-recovery", "q", "file"), "x\n")
	writeMigrationTestFile(t, filepath.Join(leasedLeaf, retiredSandboxLeaseMarkerName), "{}\n")
	writeMigrationTestFile(t, filepath.Join(readOnlyCache, "go.mod"), "module x\n")
	writeMigrationTestFile(t, filepath.Join(unmarkedLeaf, "keep.txt"), "keep\n")
	writeMigrationTestFile(t, filepath.Join(otherOwnerBase, "lease-2", retiredSandboxLeaseMarkerName), "{}\n")
	writeMigrationTestFile(t, keptRuntimeFile, "{}\n")
	if err := os.Chmod(readOnlyCache, 0o500); err != nil {
		t.Fatal(err)
	}

	if err := RunWorkspaceFiles(appRoot, workspaceRoot, discardMigrationLogger()); err != nil {
		t.Fatalf("执行工作区文件迁移失败: %v", err)
	}
	assertMigrationPathMissing(t, filepath.Join(appRoot, "processes"))
	assertMigrationPathMissing(t, leasedLeaf)
	assertMigrationPathMissing(t, otherOwnerBase)
	assertMigrationPathExists(t, filepath.Join(unmarkedLeaf, "keep.txt"))
	assertMigrationPathExists(t, keptRuntimeFile)
	assertMigrationPathExists(t, workspaceFileMigrationMarker(appRoot, retiredSandboxResourcesName))
}

func TestRunWorkspaceFilesRetriesRetiredSandboxCleanupWithoutBlockingStartup(t *testing.T) {
	stateRoot := t.TempDir()
	appRoot := filepath.Join(stateRoot, "app")
	scratchBase := filepath.Join(stateRoot, "users", "owner-a", "runtime", "sandbox")
	leasedLeaf := filepath.Join(scratchBase, "lease-1")
	lockedDir := filepath.Join(leasedLeaf, "locked")
	writeMigrationTestFile(t, filepath.Join(leasedLeaf, retiredSandboxLeaseMarkerName), "{}\n")
	writeMigrationTestFile(t, filepath.Join(lockedDir, "file"), "x\n")
	// 改名成功、删除中途失败：租约标记已被删掉，墓碑名仍须让下次启动识别。
	failRemove := true
	previousRemove := removeRetiredTree
	removeRetiredTree = func(root string) error {
		if failRemove {
			_ = os.Remove(filepath.Join(root, retiredSandboxLeaseMarkerName))
			return os.ErrPermission
		}
		return previousRemove(root)
	}
	t.Cleanup(func() { removeRetiredTree = previousRemove })

	if err := RunWorkspaceFiles(appRoot, filepath.Join(stateRoot, "workspace"), discardMigrationLogger()); err != nil {
		t.Fatalf("尽力型清理失败不应阻断启动: %v", err)
	}
	assertMigrationPathExists(t, leasedLeaf+retiredSandboxTombstoneSuffix)
	assertMigrationPathMissing(t, workspaceFileMigrationMarker(appRoot, retiredSandboxResourcesName))

	failRemove = false
	if err := RunWorkspaceFiles(appRoot, filepath.Join(stateRoot, "workspace"), discardMigrationLogger()); err != nil {
		t.Fatal(err)
	}
	assertMigrationPathMissing(t, scratchBase)
	assertMigrationPathExists(t, workspaceFileMigrationMarker(appRoot, retiredSandboxResourcesName))
}

func TestRunWorkspaceFilesRemovesRetiredStateEntries(t *testing.T) {
	stateRoot := t.TempDir()
	appRoot := filepath.Join(stateRoot, "app")
	projects := filepath.Join(stateRoot, "users", "owner-a", "runtime", "projects")
	orphan := filepath.Join(projects, "-s-users-owner-a-runtime-projects--s-workspace-amy-0b2d-session-memory")
	keptProject := filepath.Join(projects, "-s-users-owner-a-workspace-amy")
	keptMemoryDir := filepath.Join(projects, "-s-workspace-notes-session-memory")
	writeMigrationTestFile(t, filepath.Join(orphan, "session-memory-0b2d.jsonl"), "{}\n")
	writeMigrationTestFile(t, filepath.Join(keptProject, "s.jsonl"), "{}\n")
	writeMigrationTestFile(t, filepath.Join(keptMemoryDir, "s.jsonl"), "{}\n")
	writeMigrationTestFile(t, filepath.Join(appRoot, "MEMORY.md"), "# MEMORY.md\n")
	writeMigrationTestFile(t, filepath.Join(appRoot, "debug", ".DS_Store"), "x")
	writeMigrationTestFile(t, filepath.Join(appRoot, "data", "nexus.sqlite"), "")
	writeMigrationTestFile(t, filepath.Join(appRoot, "data", "nexus.db"), "db")
	writeMigrationTestFile(t, filepath.Join(stateRoot, "runtime", "arg-files", "mcp-config.json"), "{}\n")
	if err := os.MkdirAll(filepath.Join(appRoot, "memory"), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := RunWorkspaceFiles(appRoot, filepath.Join(stateRoot, "workspace"), discardMigrationLogger()); err != nil {
		t.Fatalf("执行工作区文件迁移失败: %v", err)
	}
	for _, path := range []string{
		orphan,
		filepath.Join(appRoot, "memory"),
		filepath.Join(appRoot, "MEMORY.md"),
		filepath.Join(appRoot, "debug"),
		filepath.Join(appRoot, "data", "nexus.sqlite"),
		filepath.Join(stateRoot, "runtime"),
	} {
		assertMigrationPathMissing(t, path)
	}
	assertMigrationPathExists(t, keptProject)
	assertMigrationPathExists(t, keptMemoryDir)
	assertMigrationPathExists(t, filepath.Join(appRoot, "data", "nexus.db"))
	assertMigrationPathExists(t, workspaceFileMigrationMarker(appRoot, retiredStateEntriesName))
}

func TestRunWorkspaceFilesRelocatesAppToolRoots(t *testing.T) {
	stateRoot := t.TempDir()
	appRoot := filepath.Join(stateRoot, "app")
	binary := filepath.Join("versions", "0.2.0-arm64", "nexuscua")
	writeMigrationTestFile(t, filepath.Join(appRoot, "data", "computer-use", binary), "cua")
	writeMigrationTestFile(t, filepath.Join(appRoot, "data", "computer-use", "state.json"), "{}\n")
	writeMigrationTestFile(t, filepath.Join(appRoot, ".agents", "bin", "nexusctl"), "#!/bin/sh\n")

	if err := RunWorkspaceFiles(appRoot, filepath.Join(stateRoot, "workspace"), discardMigrationLogger()); err != nil {
		t.Fatal(err)
	}
	assertMigrationPathExists(t, filepath.Join(appRoot, "computer-use", binary))
	assertMigrationPathExists(t, filepath.Join(appRoot, "computer-use", "state.json"))
	assertMigrationPathMissing(t, filepath.Join(appRoot, "data", "computer-use"))
	assertMigrationPathMissing(t, filepath.Join(appRoot, ".agents"))
	assertMigrationPathExists(t, workspaceFileMigrationMarker(appRoot, appToolRootsMigrationName))
}

func TestRunWorkspaceFilesKeepsNonEmptyRetiredStateCandidates(t *testing.T) {
	stateRoot := t.TempDir()
	appRoot := filepath.Join(stateRoot, "app")
	writeMigrationTestFile(t, filepath.Join(appRoot, "memory", "note.md"), "keep\n")
	writeMigrationTestFile(t, filepath.Join(appRoot, "MEMORY.md"), "- [note](memory/note.md)\n")
	writeMigrationTestFile(t, filepath.Join(appRoot, "data", "nexus.sqlite"), "data")

	if err := RunWorkspaceFiles(appRoot, filepath.Join(stateRoot, "workspace"), discardMigrationLogger()); err != nil {
		t.Fatal(err)
	}
	assertMigrationPathExists(t, filepath.Join(appRoot, "memory", "note.md"))
	assertMigrationPathExists(t, filepath.Join(appRoot, "MEMORY.md"))
	assertMigrationPathExists(t, filepath.Join(appRoot, "data", "nexus.sqlite"))
}

type historyBackfillFake struct {
	completed map[string]bool
}

func (f *historyBackfillFake) ListRuntimeHistoryBackfills(_ context.Context, after protocol.Execution, _ int) ([]protocol.Execution, error) {
	var result []protocol.Execution
	for _, id := range []string{"a", "b"} {
		if !f.completed[id] && id > after.ID {
			result = append(result, protocol.Execution{ID: id, OwnerUserID: "owner", SessionKey: "session"})
		}
	}
	return result, nil
}
func (f *historyBackfillFake) RuntimeGraphForMigration(context.Context, protocol.Execution) (protocol.ExecutionRuntimeGraph, error) {
	return protocol.ExecutionRuntimeGraph{Nodes: []protocol.ExecutionRuntimeNodeRun{{ID: "launch", Kind: protocol.ExecutionRuntimeNodeTool, Name: "Agent", SubjectID: "launch-use"}}}, nil
}
func (f *historyBackfillFake) CompleteRuntimeHistoryBackfill(_ context.Context, e protocol.Execution, _ protocol.ExecutionRuntimeGraph) error {
	f.completed[e.ID] = true
	return nil
}

func TestWorkGraphHistoryRetriesFailuresAndReadsEachSessionOnce(t *testing.T) {
	repository := &historyBackfillFake{completed: map[string]bool{}}
	calls := 0
	unavailable := errors.New("transcript unavailable")
	source := func(ctx context.Context, owner, session string) ([]orchestrationsvc.RuntimeGraphSubagentTaskHistory, []orchestrationsvc.RuntimeGraphSubagentToolHistory, error) {
		calls++
		if authctx.OwnerUserID(ctx) != owner || session != "session" {
			t.Fatal("migration lost owner scope")
		}
		return nil, nil, unavailable
	}
	if err := backfillWorkGraphHistory(context.Background(), repository, source, nil); !errors.Is(err, unavailable) {
		t.Fatalf("lost retryable failure: %v", err)
	}
	if calls != 1 || len(repository.completed) != 0 {
		t.Fatal("failed history was marked done or session was repeatedly read")
	}
	unavailable = nil
	if err := backfillWorkGraphHistory(context.Background(), repository, source, nil); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(repository.completed) != 2 {
		t.Fatal("retry did not finish the frozen backlog")
	}
	if err := backfillWorkGraphHistory(context.Background(), repository, source, nil); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatal("completed history was read again")
	}
}

func TestRunWorkspaceFilesRemovesRetiredMigrationMarkers(t *testing.T) {
	appRoot := t.TempDir()
	retired := workspaceFileMigrationMarker(appRoot, retiredMigrationMarkers[0])
	current := workspaceFileMigrationMarker(appRoot, retiredSandboxResourcesName)
	writeMigrationTestFile(t, retired, "completed\n")
	writeMigrationTestFile(t, current, "completed\n")

	if err := RunWorkspaceFiles(appRoot, filepath.Join(appRoot, "workspace"), discardMigrationLogger()); err != nil {
		t.Fatalf("RunWorkspaceFiles: %v", err)
	}
	assertMigrationPathMissing(t, retired)
	assertMigrationPathExists(t, current)
}
