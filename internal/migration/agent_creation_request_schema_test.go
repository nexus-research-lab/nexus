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

func TestRepairLegacyAgentCreationMigrationCollisionReplaysBusinessTags(t *testing.T) {
	db := openAgentDisabledSkillMigrationTestDB(t, "agent-creation-collision.db")
	migrationDir := providerRecoveryMigrationDir(t)
	if err := goose.UpTo(db, migrationDir, 120); err != nil {
		t.Fatal(err)
	}
	applyLegacyShiftedRecoveryMigrations(t, db, migrationDir)
	if _, err := db.Exec(`
INSERT INTO agent_creation_requests (
    owner_user_id, creation_request_id, intent_digest, agent_id, workspace_path, status
) VALUES ('owner-legacy', 'web-create:legacy', 'digest', 'agent-legacy', '/tmp/agent-legacy', 'deleted')
`); err != nil {
		t.Fatal(err)
	}

	pending, err := RepairLegacyAgentCreationMigrationCollision(
		t.Context(), "sqlite", db, discardMigrationLogger(),
	)
	if err != nil || !pending {
		t.Fatalf("repair Agent creation collision: pending=%t err=%v", pending, err)
	}
	assertMigrationApplied(t, db, 121, false)
	for version := int64(122); version <= 126; version++ {
		assertMigrationApplied(t, db, version, true)
	}
	assertCurrentMigrationVersion(t, db, 126)

	pending, err = RepairLegacyAgentCreationMigrationCollision(
		t.Context(), "sqlite", db, discardMigrationLogger(),
	)
	if err != nil || !pending {
		t.Fatalf("idempotent repair before replay: pending=%t err=%v", pending, err)
	}
	if err = goose.Up(db, migrationDir, goose.WithAllowMissing()); err != nil {
		t.Fatalf("replay Agent business tags migration: %v", err)
	}
	pending, err = RepairLegacyAgentCreationMigrationCollision(
		t.Context(), "sqlite", db, discardMigrationLogger(),
	)
	if err != nil || pending {
		t.Fatalf("finalize Agent creation collision: pending=%t err=%v", pending, err)
	}
	for version := int64(121); version <= 126; version++ {
		assertMigrationApplied(t, db, version, true)
	}
	assertCurrentMigrationVersion(t, db, latestTestMigrationVersion(t))
	if exists, columnErr := sqliteColumnExists(t.Context(), db, "agents", "business_tags"); columnErr != nil {
		t.Fatal(columnErr)
	} else if !exists {
		t.Fatal("Agent business_tags column missing")
	}
	for _, field := range automationHeartbeatWakeSchemaColumns {
		exists, columnErr := sqliteColumnExists(t.Context(), db, field.table, field.column)
		if columnErr != nil {
			t.Fatal(columnErr)
		}
		if !exists {
			t.Fatalf("Heartbeat column %s.%s missing", field.table, field.column)
		}
	}
	for _, index := range automationHeartbeatWakeSchemaIndexes {
		exists, indexErr := migrationIndexExists(t.Context(), "sqlite", db, index)
		if indexErr != nil {
			t.Fatal(indexErr)
		}
		if !exists {
			t.Fatalf("Heartbeat index %s missing", index)
		}
	}
	var status string
	if err = db.QueryRow(`
SELECT status
FROM agent_creation_requests
WHERE owner_user_id = 'owner-legacy' AND creation_request_id = 'web-create:legacy'
`).Scan(&status); err != nil || status != "deleted" {
		t.Fatalf("legacy Agent creation receipt changed: status=%q err=%v", status, err)
	}
}

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

func TestShiftedRecoveryMigrationSignaturesMatchBothDialects(t *testing.T) {
	migrationRoot := filepath.Dir(providerRecoveryMigrationDir(t))
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			businessTagFiles, err := filepath.Glob(filepath.Join(
				migrationRoot,
				dialect,
				"00121_*.sql",
			))
			if err != nil || len(businessTagFiles) != 1 {
				t.Fatalf("find canonical business-tag migration: files=%v err=%v", businessTagFiles, err)
			}
			assertMigrationSQLContains(t, businessTagFiles[0], "agents", "business_tags")

			for _, schema := range shiftedRecoveryMigrationSchemas {
				files, globErr := filepath.Glob(filepath.Join(
					migrationRoot,
					dialect,
					fmt.Sprintf("%05d_*.sql", schema.version),
				))
				if globErr != nil || len(files) != 1 {
					t.Fatalf("find migration %d: files=%v err=%v", schema.version, files, globErr)
				}
				for _, field := range schema.columns {
					assertMigrationSQLContains(t, files[0], field.table, field.column)
				}
				for _, index := range schema.indexes {
					assertMigrationSQLContains(t, files[0], index)
				}
			}
		})
	}
}

func assertMigrationSQLContains(t *testing.T, path string, fragments ...string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	upSQL, _, found := strings.Cut(string(body), "-- +goose Down")
	if !found {
		t.Fatalf("migration %s has no Goose Down boundary", path)
	}
	for _, fragment := range fragments {
		if !strings.Contains(upSQL, fragment) {
			t.Fatalf("migration %s Up schema missing %q", path, fragment)
		}
	}
}

func TestRepairLegacyAgentCreationMigrationCollisionLeavesOfficialUpgrade(t *testing.T) {
	db := openAgentDisabledSkillMigrationTestDB(t, "agent-creation-official.db")
	migrationDir := providerRecoveryMigrationDir(t)
	if err := goose.UpTo(db, migrationDir, 125); err != nil {
		t.Fatal(err)
	}

	pending, err := RepairLegacyAgentCreationMigrationCollision(
		t.Context(), "sqlite", db, discardMigrationLogger(),
	)
	if err != nil || pending {
		t.Fatalf("official migration was mistaken for collision: pending=%t err=%v", pending, err)
	}
	if err = goose.Up(db, migrationDir); err != nil {
		t.Fatalf("apply official Agent creation migration: %v", err)
	}
	assertCurrentMigrationVersion(t, db, latestTestMigrationVersion(t))
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

func applyLegacyShiftedRecoveryMigrations(t *testing.T, db *sql.DB, migrationDir string) {
	t.Helper()
	applyLegacyShiftedRecoveryMigrationPrefix(t, db, migrationDir, 5)
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

func TestAgentCreationRequestMigrationKeepsDeletedReceipts(t *testing.T) {
	db := openAgentDisabledSkillMigrationTestDB(t, "agent-creation-request.db")
	if err := goose.Up(db, providerRecoveryMigrationDir(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`
INSERT INTO agent_creation_requests (
    owner_user_id, creation_request_id, intent_digest, agent_id, workspace_path, status
) VALUES ('owner-a', 'web-create:test', 'digest', 'agent-a', '/tmp/agent-a', 'deleted')`); err != nil {
		t.Fatalf("insert deleted receipt: %v", err)
	}
	var status string
	if err := db.QueryRow(`
SELECT status FROM agent_creation_requests
WHERE owner_user_id = 'owner-a' AND creation_request_id = 'web-create:test'`).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "deleted" {
		t.Fatalf("status = %q, want deleted", status)
	}
	if _, err := db.Exec(`
INSERT INTO agent_creation_requests (
    owner_user_id, creation_request_id, intent_digest, agent_id, workspace_path, status
) VALUES ('owner-a', 'web-create:test', 'other', 'agent-b', '/tmp/agent-b', 'pending')`); err == nil {
		t.Fatal("owner/request uniqueness did not reject a late replay")
	}
}

func TestPostgresAgentCreationRequestMigrationHasOwnerScopedKeys(t *testing.T) {
	path := filepath.Join(
		filepath.Dir(providerRecoveryMigrationDir(t)),
		"postgres",
		"00126_agent_creation_requests.sql",
	)
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	for _, required := range []string{
		"PRIMARY KEY (owner_user_id, creation_request_id)",
		"UNIQUE (owner_user_id, agent_id)",
		"'pending', 'committed', 'deleted', 'failed'",
		"'reserved', 'workspace_prepared'",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("postgres migration missing %q", required)
		}
	}
}
