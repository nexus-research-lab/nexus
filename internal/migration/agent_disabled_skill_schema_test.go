package migration

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

func TestRepairLegacyAgentDisabledSkillSchemaAdvancesVersionCollision(t *testing.T) {
	db := openAgentDisabledSkillMigrationTestDB(t, "legacy.db")
	migrationDir := providerRecoveryMigrationDir(t)
	if err := goose.UpTo(db, migrationDir, 55); err != nil {
		t.Fatal(err)
	}
	applyLegacyConversationDraftMigration(t, db, 56)

	if err := RepairLegacyAgentDisabledSkillSchema(
		t.Context(),
		"sqlite",
		db,
		56,
		discardMigrationLogger(),
	); err != nil {
		t.Fatal(err)
	}
	if err := goose.Up(db, migrationDir); err != nil {
		t.Fatalf("apply current migrations after compatibility repair: %v", err)
	}

	assertAgentDisabledSkillSchema(t, db, 87)
}

func TestRepairLegacyAgentDisabledSkillSchemaKeepsLaterLegacyVersion(t *testing.T) {
	db := openAgentDisabledSkillMigrationTestDB(t, "legacy-later.db")
	migrationDir := providerRecoveryMigrationDir(t)
	if err := goose.UpTo(db, migrationDir, 55); err != nil {
		t.Fatal(err)
	}
	applyLegacyConversationDraftMigration(t, db, 56)
	if _, err := db.Exec(
		"INSERT INTO goose_db_version (version_id, is_applied) VALUES (?, 1)",
		57,
	); err != nil {
		t.Fatal(err)
	}

	if err := RepairLegacyAgentDisabledSkillSchema(
		t.Context(),
		"sqlite",
		db,
		57,
		discardMigrationLogger(),
	); err != nil {
		t.Fatal(err)
	}
	if err := goose.Up(db, migrationDir); err != nil {
		t.Fatalf("apply current migrations after later legacy version: %v", err)
	}

	assertAgentDisabledSkillSchema(t, db, 87)
}

func TestRepairLegacyAgentDisabledSkillSchemaLeavesCurrentSchemaUntouched(t *testing.T) {
	db := openAgentDisabledSkillMigrationTestDB(t, "current.db")
	migrationDir := providerRecoveryMigrationDir(t)
	if err := goose.UpTo(db, migrationDir, 56); err != nil {
		t.Fatal(err)
	}

	if err := RepairLegacyAgentDisabledSkillSchema(
		t.Context(),
		"sqlite",
		db,
		56,
		discardMigrationLogger(),
	); err != nil {
		t.Fatal(err)
	}
	if err := goose.Up(db, migrationDir); err != nil {
		t.Fatalf("apply current migrations from current version 56: %v", err)
	}

	assertAgentDisabledSkillSchema(t, db, 87)
}

func openAgentDisabledSkillMigrationTestDB(t *testing.T, name string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), name))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	return db
}

func applyLegacyConversationDraftMigration(t *testing.T, db *sql.DB, version int64) {
	t.Helper()
	if _, err := db.Exec(`
ALTER TABLE conversations
ADD COLUMN is_draft BOOLEAN NOT NULL DEFAULT FALSE;

CREATE UNIQUE INDEX uq_conversations_room_draft
ON conversations (room_id)
WHERE is_draft = TRUE;
`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(
		"INSERT INTO goose_db_version (version_id, is_applied) VALUES (?, 1)",
		version,
	); err != nil {
		t.Fatal(err)
	}
}

func assertAgentDisabledSkillSchema(t *testing.T, db *sql.DB, expectedVersion int64) {
	t.Helper()
	hasColumn, err := sqliteColumnExists(
		t.Context(),
		db,
		"runtimes",
		"disabled_skill_ids_json",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !hasColumn {
		t.Fatal("runtimes.disabled_skill_ids_json missing")
	}

	var version int64
	if err = db.QueryRow(
		"SELECT version_id FROM goose_db_version ORDER BY id DESC LIMIT 1",
	).Scan(&version); err != nil {
		t.Fatal(err)
	}
	if version != expectedVersion {
		t.Fatalf("goose version = %d, want %d", version, expectedVersion)
	}
}
