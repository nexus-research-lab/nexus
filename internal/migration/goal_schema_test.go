package migration

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"
)

func TestRepairLegacyGoalMigrationCollisionReplaysMainMigrations(t *testing.T) {
	db := openAgentDisabledSkillMigrationTestDB(t, "goal-migration-collision.db")
	migrationDir := providerRecoveryMigrationDir(t)
	if err := goose.UpTo(db, migrationDir, 86); err != nil {
		t.Fatal(err)
	}
	applyLegacyGoalMigrations(t, db, migrationDir)

	pending, err := RepairLegacyGoalMigrationCollision(
		t.Context(), "sqlite", db, discardMigrationLogger(),
	)
	if err != nil || !pending {
		t.Fatalf("repair legacy Goal collision: pending=%t err=%v", pending, err)
	}
	for _, version := range []int64{87, 88, 89} {
		assertMigrationApplied(t, db, version, false)
	}
	for _, version := range []int64{97, 98} {
		assertMigrationApplied(t, db, version, true)
	}

	if err = goose.Up(db, migrationDir, goose.WithAllowMissing()); err != nil {
		t.Fatalf("replay main and current Goal migrations: %v", err)
	}
	pending, err = RepairLegacyGoalMigrationCollision(
		t.Context(), "sqlite", db, discardMigrationLogger(),
	)
	if err != nil || pending {
		t.Fatalf("finalize legacy Goal collision: pending=%t err=%v", pending, err)
	}
	assertCurrentMigrationVersion(t, db, 99)
	for _, version := range []int64{87, 93, 94, 95, 96, 97, 98, 99} {
		assertMigrationApplied(t, db, version, true)
	}
	if present, inspectErr := inspectAgentContactSchema(t.Context(), "sqlite", db); inspectErr != nil || present != len(agentContactSchemaColumns)+1 {
		t.Fatalf("Agent contact schema present=%d err=%v", present, inspectErr)
	}
	assertLegacyGoalSchema(t, db)
}

func TestRepairLegacyGoalMigrationCollisionLeavesCurrentMainUpgrade(t *testing.T) {
	db := openAgentDisabledSkillMigrationTestDB(t, "goal-migration-current.db")
	migrationDir := providerRecoveryMigrationDir(t)
	if err := goose.UpTo(db, migrationDir, 96); err != nil {
		t.Fatal(err)
	}

	pending, err := RepairLegacyGoalMigrationCollision(
		t.Context(), "sqlite", db, discardMigrationLogger(),
	)
	if err != nil || pending {
		t.Fatalf("current main upgrade was mistaken for collision: pending=%t err=%v", pending, err)
	}
	if err = goose.Up(db, migrationDir); err != nil {
		t.Fatal(err)
	}
	assertCurrentMigrationVersion(t, db, 99)
	assertLegacyGoalSchema(t, db)
}

func TestRepairLegacyGoalMigrationCollisionRejectsPartialSchema(t *testing.T) {
	db := openAgentDisabledSkillMigrationTestDB(t, "goal-migration-partial.db")
	migrationDir := providerRecoveryMigrationDir(t)
	if err := goose.UpTo(db, migrationDir, 86); err != nil {
		t.Fatal(err)
	}
	applyMigrationUpSQL(
		t,
		db,
		filepath.Join(migrationDir, "00097_execution_goal_activation_reason.sql"),
	)
	insertMigrationMarkers(t, db, 87)

	pending, err := RepairLegacyGoalMigrationCollision(
		t.Context(), "sqlite", db, discardMigrationLogger(),
	)
	if err == nil || pending || !strings.Contains(err.Error(), "incomplete legacy Goal migration schema") {
		t.Fatalf("partial Goal schema was not rejected: pending=%t err=%v", pending, err)
	}
	assertMigrationApplied(t, db, 97, false)
	assertMigrationApplied(t, db, 98, false)
}

func applyLegacyGoalMigrations(t *testing.T, db *sql.DB, migrationDir string) {
	t.Helper()
	for _, name := range []string{
		"00097_execution_goal_activation_reason.sql",
		"00098_execution_goal_confirmations.sql",
		"00099_goal_actual_token_zero_repair.sql",
	} {
		applyMigrationUpSQL(t, db, filepath.Join(migrationDir, name))
	}
	insertMigrationMarkers(t, db, 87, 88, 89)
}

func applyMigrationUpSQL(t *testing.T, db *sql.DB, path string) {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	upSQL, _, found := strings.Cut(string(contents), "-- +goose Down")
	if !found {
		t.Fatalf("migration %s has no Goose Down boundary", path)
	}
	if _, err = db.Exec(upSQL); err != nil {
		t.Fatalf("apply migration %s: %v", path, err)
	}
}

func insertMigrationMarkers(t *testing.T, db *sql.DB, versions ...int64) {
	t.Helper()
	for _, version := range versions {
		if _, err := db.Exec(
			"INSERT INTO goose_db_version (version_id, is_applied) VALUES (?, TRUE)",
			version,
		); err != nil {
			t.Fatalf("record migration %d: %v", version, err)
		}
	}
}

func assertLegacyGoalSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	expanded, err := goalActivationReasonExpanded(t.Context(), "sqlite", db)
	if err != nil || !expanded {
		t.Fatalf("Goal activation reason expanded=%t err=%v", expanded, err)
	}
	confirmationTable, err := migrationTableExists(
		t.Context(), "sqlite", db, "execution_goal_confirmations",
	)
	if err != nil || !confirmationTable {
		t.Fatalf("Goal confirmation table exists=%t err=%v", confirmationTable, err)
	}
}
