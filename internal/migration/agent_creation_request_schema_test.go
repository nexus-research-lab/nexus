package migration

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"
)

func TestRepairLegacyAgentCreationMigrationCollisionResumesPartialLegacyPrefixes(t *testing.T) {
	for _, appliedCount := range []int{1, 4} {
		t.Run(fmt.Sprintf("through-legacy-%d", 120+appliedCount), func(t *testing.T) {
			db := openAgentDisabledSkillMigrationTestDB(t, "agent-creation-prefix.db")
			migrationDir := providerRecoveryMigrationDir(t)
			if err := goose.UpTo(db, migrationDir, 120); err != nil {
				t.Fatal(err)
			}
			applyLegacyShiftedRecoveryMigrationPrefix(t, db, migrationDir, appliedCount)

			pending, err := RepairLegacyAgentCreationMigrationCollision(
				t.Context(), "sqlite", db, discardMigrationLogger(),
			)
			if err != nil || !pending {
				t.Fatalf("repair partial legacy prefix: pending=%t err=%v", pending, err)
			}
			assertMigrationApplied(t, db, 121, false)
			for version := int64(122); version <= int64(121+appliedCount); version++ {
				assertMigrationApplied(t, db, version, true)
			}

			// A restart after the atomic ledger rewrite must request the same safe
			// replay rather than guessing that the missing business-tag schema ran.
			pending, err = RepairLegacyAgentCreationMigrationCollision(
				t.Context(), "sqlite", db, discardMigrationLogger(),
			)
			if err != nil || !pending {
				t.Fatalf("resume partial legacy prefix: pending=%t err=%v", pending, err)
			}
			if err = goose.Up(db, migrationDir, goose.WithAllowMissing()); err != nil {
				t.Fatalf("complete canonical migrations: %v", err)
			}
			pending, err = RepairLegacyAgentCreationMigrationCollision(
				t.Context(), "sqlite", db, discardMigrationLogger(),
			)
			if err != nil || pending {
				t.Fatalf("finalize partial legacy prefix: pending=%t err=%v", pending, err)
			}
			assertCurrentMigrationVersion(t, db, latestTestMigrationVersion(t))
		})
	}
}

func TestRepairLegacyAgentCreationMigrationCollisionRejectsPartialSchema(t *testing.T) {
	db := openAgentDisabledSkillMigrationTestDB(t, "agent-creation-partial.db")
	migrationDir := providerRecoveryMigrationDir(t)
	if err := goose.UpTo(db, migrationDir, 124); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
CREATE TABLE agent_creation_requests (
    owner_user_id VARCHAR(64) NOT NULL,
    creation_request_id VARCHAR(128) NOT NULL
)
`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(
		"INSERT INTO goose_db_version (version_id, is_applied) VALUES (125, TRUE)",
	); err != nil {
		t.Fatal(err)
	}

	pending, err := RepairLegacyAgentCreationMigrationCollision(
		t.Context(), "sqlite", db, discardMigrationLogger(),
	)
	if err == nil || pending || !strings.Contains(err.Error(), "incomplete Agent creation receipt schema") {
		t.Fatalf("partial Agent creation schema was not rejected: pending=%t err=%v", pending, err)
	}
	assertMigrationApplied(t, db, 126, false)
}

func applyLegacyShiftedRecoveryMigrationPrefix(
	t *testing.T,
	db *sql.DB,
	migrationDir string,
	appliedCount int,
) {
	t.Helper()
	files := []string{
		"00122_automation_delivery_attempt_claim.sql",
		"00123_automation_task_deletion_claim.sql",
		"00124_automation_run_request_identity.sql",
		"00125_automation_heartbeat_wake_outbox.sql",
		"00126_agent_creation_requests.sql",
	}
	if appliedCount < 0 || appliedCount > len(files) {
		t.Fatalf("invalid shifted migration prefix length %d", appliedCount)
	}
	for index, name := range files[:appliedCount] {
		contents, err := os.ReadFile(filepath.Join(migrationDir, name))
		if err != nil {
			t.Fatal(err)
		}
		upSQL, _, found := strings.Cut(string(contents), "-- +goose Down")
		if !found {
			t.Fatalf("migration %s has no Goose Down boundary", name)
		}
		if _, err = db.Exec(upSQL); err != nil {
			t.Fatalf("apply legacy shifted schema %s: %v", name, err)
		}
		if _, err = db.Exec(
			"INSERT INTO goose_db_version (version_id, is_applied) VALUES (?, TRUE)",
			121+index,
		); err != nil {
			t.Fatal(err)
		}
	}
}
