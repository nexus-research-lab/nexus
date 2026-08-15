package migration

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/pressly/goose/v3"
)

func TestDefaultNexusConfigurationSkillMigrationReplacesRoleSkills(t *testing.T) {
	db := openAgentDisabledSkillMigrationTestDB(t, "default-nexus-configuration-skill.db")
	migrationDir := providerRecoveryMigrationDir(t)
	if err := goose.UpTo(db, migrationDir, 107); err != nil {
		t.Fatal(err)
	}
	insertRecoveryAgent(t, db, "worker", "owner", "")
	if _, err := db.Exec(
		`UPDATE runtimes SET skill_ids_json = '["imagegen","nexus-agent-self-configuration","nexus-configuration"]' WHERE agent_id = 'worker'`,
	); err != nil {
		t.Fatal(err)
	}
	if err := goose.UpTo(db, migrationDir, 109); err != nil {
		t.Fatal(err)
	}
	var payload string
	if err := db.QueryRow(
		`SELECT skill_ids_json FROM runtimes WHERE agent_id = 'worker'`,
	).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	var names []string
	if err := json.Unmarshal([]byte(payload), &names); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(names, []string{"imagegen", "nexus-configuration"}) {
		t.Fatalf("migrated skill ids = %#v", names)
	}
}
