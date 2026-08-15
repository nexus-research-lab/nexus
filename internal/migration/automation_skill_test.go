package migration

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/pressly/goose/v3"
)

func TestDefaultAutomationSkillMigrationUpdatesExistingAgents(t *testing.T) {
	db := openAgentDisabledSkillMigrationTestDB(t, "default-automation-skill.db")
	migrationDir := providerRecoveryMigrationDir(t)
	if err := goose.UpTo(db, migrationDir, 109); err != nil {
		t.Fatal(err)
	}
	insertRecoveryAgent(t, db, "worker", "owner", "")
	insertRecoveryAgent(t, db, "existing", "owner", "")
	if _, err := db.Exec(
		`UPDATE runtimes SET skill_ids_json = CASE agent_id WHEN 'worker' THEN '["imagegen","visualize"]' ELSE '["automation"]' END WHERE agent_id IN ('worker', 'existing')`,
	); err != nil {
		t.Fatal(err)
	}

	if err := goose.UpTo(db, migrationDir, 110); err != nil {
		t.Fatal(err)
	}
	var payload string
	if err := db.QueryRow(
		`SELECT skill_ids_json FROM runtimes WHERE agent_id = ?`,
		"worker",
	).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	var skillNames []string
	if err := json.Unmarshal([]byte(payload), &skillNames); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(skillNames, []string{"imagegen", "visualize", "automation"}) {
		t.Fatalf("migrated skill ids = %#v", skillNames)
	}
	var existingCount int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM json_each((SELECT skill_ids_json FROM runtimes WHERE agent_id = 'existing')) WHERE value = 'automation'`,
	).Scan(&existingCount); err != nil {
		t.Fatal(err)
	}
	if existingCount != 1 {
		t.Fatalf("existing automation count = %d, want 1", existingCount)
	}
}
