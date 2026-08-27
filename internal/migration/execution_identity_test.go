package migration

import (
	"testing"

	"github.com/pressly/goose/v3"
)

func TestRepairExecutionIdentityRestoresMissingTable(t *testing.T) {
	db := openAgentDisabledSkillMigrationTestDB(t, "missing-execution-identity-claims.db")
	if err := goose.Up(db, providerRecoveryMigrationDir(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("DROP TABLE goal_execution_identity_claims"); err != nil {
		t.Fatal(err)
	}

	if err := RepairLegacyExecutionIdentityClaimSchema(
		t.Context(),
		"sqlite",
		db,
		71,
		discardMigrationLogger(),
	); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
INSERT INTO goal_execution_identity_claims (
    execution_id,
    goal_id,
    goal_objective_revision,
    owner_user_id,
    claim_state,
    command_id
) VALUES (?, ?, ?, ?, ?, ?)
`, "execution-1", "goal-1", 1, "owner-1", "materialized", "command-1"); err != nil {
		t.Fatalf("insert repaired execution identity claim: %v", err)
	}

	if err := RepairLegacyExecutionIdentityClaimSchema(
		t.Context(),
		"sqlite",
		db,
		71,
		discardMigrationLogger(),
	); err != nil {
		t.Fatalf("repeat repair: %v", err)
	}
	var count int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM goal_execution_identity_claims WHERE execution_id = ?",
		"execution-1",
	).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("preserved identity claim count = %d, want 1", count)
	}
}

func TestRepairExecutionIdentitySkipsPreExecutionDatabase(t *testing.T) {
	db := openAgentDisabledSkillMigrationTestDB(t, "pre-execution-identity-claims.db")

	if err := RepairLegacyExecutionIdentityClaimSchema(
		t.Context(),
		"sqlite",
		db,
		60,
		discardMigrationLogger(),
	); err != nil {
		t.Fatal(err)
	}
	exists, err := executionIdentityClaimTableExists(t.Context(), "sqlite", db)
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("pre-00061 database unexpectedly received execution identity claim table")
	}
}
