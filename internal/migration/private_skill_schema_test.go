package migration

import (
	"database/sql"
	"testing"

	"github.com/pressly/goose/v3"
)

func TestRepairLegacyPrivateSkillMigrationCollision(t *testing.T) {
	db := openAgentDisabledSkillMigrationTestDB(t, "private-skill-collision.db")
	migrationDir := providerRecoveryMigrationDir(t)
	if err := goose.UpTo(db, migrationDir, 60); err != nil {
		t.Fatal(err)
	}
	applyLegacyPrivateSkillMigration(t, db)

	repaired, err := RepairLegacyPrivateSkillMigrationCollision(
		t.Context(),
		"sqlite",
		db,
		61,
		discardMigrationLogger(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !repaired {
		t.Fatal("private Skill migration collision was not repaired")
	}
	assertMigrationApplied(t, db, 61, false)
	assertMigrationApplied(t, db, 71, true)

	if err = goose.Up(db, migrationDir, goose.WithAllowMissing()); err != nil {
		t.Fatalf("apply Execution migrations after ledger repair: %v", err)
	}
	finalizePrivateSkillMigrationCollision(t, db)
	finalizePrivateSkillMigrationCollision(t, db)
	assertMigrationApplied(t, db, 61, true)
	assertMigrationApplied(t, db, 71, true)
	assertCurrentMigrationVersion(t, db, 87)
	assertMigrationTable(t, db, "executions")
	assertPrivateSkillMigrationData(t, db)
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

func TestRepairLegacyPrivateSkillMigrationCollisionResumesBackfill(t *testing.T) {
	db := openAgentDisabledSkillMigrationTestDB(t, "private-skill-collision-resume.db")
	migrationDir := providerRecoveryMigrationDir(t)
	if err := goose.UpTo(db, migrationDir, 60); err != nil {
		t.Fatal(err)
	}
	applyLegacyPrivateSkillMigration(t, db)
	if repaired, err := RepairLegacyPrivateSkillMigrationCollision(
		t.Context(), "sqlite", db, 61, discardMigrationLogger(),
	); err != nil || !repaired {
		t.Fatalf("prepare migration replay: repaired=%t err=%v", repaired, err)
	}
	if err := goose.UpTo(db, migrationDir, 65, goose.WithAllowMissing()); err != nil {
		t.Fatalf("partially apply Execution migrations: %v", err)
	}
	currentVersion, err := goose.GetDBVersion(db)
	if err != nil {
		t.Fatal(err)
	}
	resume, err := RepairLegacyPrivateSkillMigrationCollision(
		t.Context(), "sqlite", db, currentVersion, discardMigrationLogger(),
	)
	if err != nil || !resume {
		t.Fatalf("resume migration replay: resume=%t err=%v", resume, err)
	}
	if err = goose.Up(db, migrationDir, goose.WithAllowMissing()); err != nil {
		t.Fatalf("finish Execution migration replay: %v", err)
	}
	finalizePrivateSkillMigrationCollision(t, db)
	assertCurrentMigrationVersion(t, db, 87)
}

func TestRepairLegacyPrivateSkillMigrationCollisionKeepsExecutionMigration(t *testing.T) {
	db := openAgentDisabledSkillMigrationTestDB(t, "execution-61.db")
	migrationDir := providerRecoveryMigrationDir(t)
	if err := goose.UpTo(db, migrationDir, 61); err != nil {
		t.Fatal(err)
	}

	repaired, err := RepairLegacyPrivateSkillMigrationCollision(
		t.Context(),
		"sqlite",
		db,
		61,
		discardMigrationLogger(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if repaired {
		t.Fatal("current Execution migration was mistaken for the private Skill collision")
	}
	if err = goose.Up(db, migrationDir); err != nil {
		t.Fatalf("apply current migrations: %v", err)
	}
	assertMigrationApplied(t, db, 71, true)
	if _, err = db.Exec(
		"INSERT INTO goose_db_version (version_id, is_applied) VALUES (72, TRUE)",
	); err != nil {
		t.Fatal(err)
	}
	repaired, err = RepairLegacyPrivateSkillMigrationCollision(
		t.Context(), "sqlite", db, 72, discardMigrationLogger(),
	)
	if err != nil || repaired {
		t.Fatalf("future migration version changed: repaired=%t err=%v", repaired, err)
	}
	assertCurrentMigrationVersion(t, db, 72)
}

func applyLegacyPrivateSkillMigration(t *testing.T, db *sql.DB) {
	t.Helper()
	if _, err := db.Exec(`
ALTER TABLE skill_sources ADD COLUMN managed_by VARCHAR(32) NOT NULL DEFAULT 'system';
ALTER TABLE skill_sources ADD COLUMN auth_type VARCHAR(32) NOT NULL DEFAULT 'none';
ALTER TABLE skill_sources ADD COLUMN credentials_encrypted TEXT NOT NULL DEFAULT '';
ALTER TABLE imported_skills ADD COLUMN source_skill_id VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE imported_skills ADD COLUMN artifact_sha256 VARCHAR(64) NOT NULL DEFAULT '';
INSERT INTO skill_sources (
    owner_user_id, source_id, name, kind, url,
    managed_by, auth_type, credentials_encrypted
) VALUES (
    'owner-1', 'private-1', 'Private source', 'git', 'https://example.com/private.git',
    'user', 'bearer', 'encrypted-token'
);
INSERT INTO imported_skills (
    owner_user_id, skill_name, source_skill_id, artifact_sha256
) VALUES (
    'owner-1', 'private-skill', 'upstream-skill', 'artifact-digest'
);
INSERT INTO goose_db_version (version_id, is_applied) VALUES (61, TRUE);
`); err != nil {
		t.Fatal(err)
	}
}

func assertPrivateSkillMigrationData(t *testing.T, db *sql.DB) {
	t.Helper()
	var managedBy, authType, credentials string
	if err := db.QueryRow(`
SELECT managed_by, auth_type, credentials_encrypted
FROM skill_sources
WHERE owner_user_id = 'owner-1' AND source_id = 'private-1'
`).Scan(&managedBy, &authType, &credentials); err != nil {
		t.Fatal(err)
	}
	if managedBy != "user" || authType != "bearer" || credentials != "encrypted-token" {
		t.Fatalf(
			"private source metadata = (%q, %q, %q)",
			managedBy,
			authType,
			credentials,
		)
	}

	var sourceSkillID, artifactSHA256 string
	if err := db.QueryRow(`
SELECT source_skill_id, artifact_sha256
FROM imported_skills
WHERE owner_user_id = 'owner-1' AND skill_name = 'private-skill'
`).Scan(&sourceSkillID, &artifactSHA256); err != nil {
		t.Fatal(err)
	}
	if sourceSkillID != "upstream-skill" || artifactSHA256 != "artifact-digest" {
		t.Fatalf(
			"imported Skill metadata = (%q, %q)",
			sourceSkillID,
			artifactSHA256,
		)
	}
}

func assertMigrationApplied(t *testing.T, db *sql.DB, version int64, expected bool) {
	t.Helper()
	var count int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM goose_db_version WHERE version_id = ? AND is_applied = TRUE",
		version,
	).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if (count > 0) != expected {
		t.Fatalf("migration %d applied = %t, want %t", version, count > 0, expected)
	}
}

func finalizePrivateSkillMigrationCollision(t *testing.T, db *sql.DB) {
	t.Helper()
	currentVersion, err := goose.GetDBVersion(db)
	if err != nil {
		t.Fatal(err)
	}
	pending, err := RepairLegacyPrivateSkillMigrationCollision(
		t.Context(), "sqlite", db, currentVersion, discardMigrationLogger(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if pending {
		t.Fatal("Execution migration replay still pending after Goose completed")
	}
}

func assertCurrentMigrationVersion(t *testing.T, db *sql.DB, expected int64) {
	t.Helper()
	version, err := goose.GetDBVersion(db)
	if err != nil {
		t.Fatal(err)
	}
	if version != expected {
		t.Fatalf("current migration version = %d, want %d", version, expected)
	}
}

func assertMigrationTable(t *testing.T, db *sql.DB, table string) {
	t.Helper()
	var count int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?",
		table,
	).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("migration table %s count = %d, want 1", table, count)
	}
}
