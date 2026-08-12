package migration

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"
)

func TestRepairLegacyAutomationPermissionMigrationCollisionReplaysPrivateSkill(t *testing.T) {
	db := openAgentDisabledSkillMigrationTestDB(t, "automation-permission-collision.db")
	migrationDir := providerRecoveryMigrationDir(t)
	if err := goose.UpTo(db, migrationDir, 70); err != nil {
		t.Fatal(err)
	}
	applyLegacyAutomationPermissionMigration(t, db, migrationDir)
	insertLegacyAutomationPermissionRequest(t, db)
	if err := goose.UpTo(db, migrationDir, 85); err != nil {
		t.Fatalf("apply migrations after legacy permission schema: %v", err)
	}

	pending, err := RepairLegacyAutomationPermissionMigrationCollision(
		t.Context(), "sqlite", db, 85, discardMigrationLogger(),
	)
	if err != nil || !pending {
		t.Fatalf("repair legacy permission collision: pending=%t err=%v", pending, err)
	}
	assertMigrationApplied(t, db, 71, false)
	assertMigrationApplied(t, db, 86, true)
	assertCurrentMigrationVersion(t, db, 86)

	pending, err = RepairLegacyAutomationPermissionMigrationCollision(
		t.Context(), "sqlite", db, 86, discardMigrationLogger(),
	)
	if err != nil || !pending {
		t.Fatalf("idempotent repair before replay: pending=%t err=%v", pending, err)
	}
	if err = goose.Up(db, migrationDir, goose.WithAllowMissing()); err != nil {
		t.Fatalf("replay private Skill migration: %v", err)
	}
	currentVersion, err := goose.GetDBVersion(db)
	if err != nil {
		t.Fatal(err)
	}
	pending, err = RepairLegacyAutomationPermissionMigrationCollision(
		t.Context(), "sqlite", db, currentVersion, discardMigrationLogger(),
	)
	if err != nil || pending {
		t.Fatalf("finalize permission collision repair: pending=%t err=%v", pending, err)
	}
	assertMigrationApplied(t, db, 71, true)
	assertMigrationApplied(t, db, 86, true)
	assertCurrentMigrationVersion(t, db, 87)
	assertLegacyAutomationPermissionRequest(t, db)
	for _, field := range privateSkillSchemaColumns {
		exists, columnErr := sqliteColumnExists(t.Context(), db, field.table, field.column)
		if columnErr != nil {
			t.Fatal(columnErr)
		}
		if !exists {
			t.Fatalf("private Skill column %s.%s missing", field.table, field.column)
		}
	}
}

func TestRepairLegacyAutomationPermissionMigrationCollisionLeavesOfficialUpgrade(t *testing.T) {
	db := openAgentDisabledSkillMigrationTestDB(t, "automation-permission-official.db")
	migrationDir := providerRecoveryMigrationDir(t)
	if err := goose.UpTo(db, migrationDir, 85); err != nil {
		t.Fatal(err)
	}

	pending, err := RepairLegacyAutomationPermissionMigrationCollision(
		t.Context(), "sqlite", db, 85, discardMigrationLogger(),
	)
	if err != nil || pending {
		t.Fatalf("official migration was mistaken for collision: pending=%t err=%v", pending, err)
	}
	if err = goose.Up(db, migrationDir); err != nil {
		t.Fatalf("apply official permission migration: %v", err)
	}
	assertCurrentMigrationVersion(t, db, 87)
}

func TestRepairLegacyAutomationPermissionMigrationCollisionRejectsPartialSchema(t *testing.T) {
	db := openAgentDisabledSkillMigrationTestDB(t, "automation-permission-partial.db")
	migrationDir := providerRecoveryMigrationDir(t)
	if err := goose.UpTo(db, migrationDir, 70); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
ALTER TABLE automation_scheduled_tasks
ADD COLUMN permission_policy_json TEXT NOT NULL DEFAULT '{}'
`); err != nil {
		t.Fatal(err)
	}

	pending, err := RepairLegacyAutomationPermissionMigrationCollision(
		t.Context(), "sqlite", db, 70, discardMigrationLogger(),
	)
	if err == nil || pending || !strings.Contains(err.Error(), "incomplete automation permission schema") {
		t.Fatalf("partial permission schema was not rejected: pending=%t err=%v", pending, err)
	}
	assertMigrationApplied(t, db, 71, false)
	assertMigrationApplied(t, db, 86, false)
}

func applyLegacyAutomationPermissionMigration(t *testing.T, db *sql.DB, migrationDir string) {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join(migrationDir, "00086_automation_permission_pipeline.sql"))
	if err != nil {
		t.Fatal(err)
	}
	upSQL, _, found := strings.Cut(string(contents), "-- +goose Down")
	if !found {
		t.Fatal("automation permission migration has no Goose Down boundary")
	}
	if _, err = db.Exec(upSQL); err != nil {
		t.Fatalf("apply legacy automation permission schema: %v", err)
	}
	if _, err = db.Exec(
		"INSERT INTO goose_db_version (version_id, is_applied) VALUES (71, TRUE)",
	); err != nil {
		t.Fatal(err)
	}
}

func insertLegacyAutomationPermissionRequest(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`
INSERT INTO automation_permission_requests (
    request_id, owner_user_id, job_id, policy_revision, kind, status,
    tool_name, effect, input_fingerprint, capability_json
) VALUES (
    'permission-legacy', 'owner-legacy', 'task-legacy', 1, 'tool', 'pending',
    'mcp__nexus_connectors__feishu_docx_read', 'read', 'fingerprint-legacy', '{}'
)
`); err != nil {
		t.Fatal(err)
	}
}

func assertLegacyAutomationPermissionRequest(t *testing.T, db *sql.DB) {
	t.Helper()
	var status, toolName string
	if err := db.QueryRow(`
SELECT status, tool_name
FROM automation_permission_requests
WHERE request_id = 'permission-legacy'
`).Scan(&status, &toolName); err != nil {
		t.Fatal(err)
	}
	if status != "pending" || toolName != "mcp__nexus_connectors__feishu_docx_read" {
		t.Fatalf("legacy permission request changed: status=%q tool=%q", status, toolName)
	}
}
