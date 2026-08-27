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

func TestGoalExecutionCLIMigrationBindsExistingAgentsToExecutionSkill(t *testing.T) {
	db := openAgentDisabledSkillMigrationTestDB(t, "goal-execution-cli-skill.db")
	migrationDir := providerRecoveryMigrationDir(t)
	if err := goose.UpTo(db, migrationDir, 114); err != nil {
		t.Fatal(err)
	}
	insertRecoveryAgent(t, db, "worker", "owner", "")
	insertRecoveryAgent(t, db, "existing", "owner", "")
	if _, err := db.Exec(
		`UPDATE runtimes SET skill_ids_json = CASE agent_id WHEN 'worker' THEN '["goal-manager","automation"]' ELSE '["execution-orchestrator"]' END WHERE agent_id IN ('worker', 'existing')`,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(
		`UPDATE runtimes SET disabled_skill_ids_json = '["goal-manager","execution-orchestrator","private-skill"]' WHERE agent_id = 'worker'`,
	); err != nil {
		t.Fatal(err)
	}

	if err := goose.UpTo(db, migrationDir, 115); err != nil {
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
	if !slices.Equal(skillNames, []string{"goal-manager", "automation", "execution-orchestrator"}) {
		t.Fatalf("migrated skill ids = %#v", skillNames)
	}
	var existingCount int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM json_each((SELECT skill_ids_json FROM runtimes WHERE agent_id = 'existing')) WHERE value = 'execution-orchestrator'`,
	).Scan(&existingCount); err != nil {
		t.Fatal(err)
	}
	if existingCount != 1 {
		t.Fatalf("existing execution-orchestrator count = %d, want 1", existingCount)
	}
	var disabledPayload string
	if err := db.QueryRow(
		`SELECT disabled_skill_ids_json FROM runtimes WHERE agent_id = ?`,
		"worker",
	).Scan(&disabledPayload); err != nil {
		t.Fatal(err)
	}
	var disabledSkillNames []string
	if err := json.Unmarshal([]byte(disabledPayload), &disabledSkillNames); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(disabledSkillNames, []string{"private-skill"}) {
		t.Fatalf("migrated disabled skill ids = %#v", disabledSkillNames)
	}
}

func TestDefaultNexusProductGuideSkillMigrationUpdatesExistingAgents(t *testing.T) {
	db := openAgentDisabledSkillMigrationTestDB(t, "default-nexus-product-guide-skill.db")
	migrationDir := providerRecoveryMigrationDir(t)
	if err := goose.UpTo(db, migrationDir, 119); err != nil {
		t.Fatal(err)
	}
	insertRecoveryAgent(t, db, "worker", "owner", "")
	insertRecoveryAgent(t, db, "existing", "owner", "")
	if _, err := db.Exec(
		`UPDATE runtimes SET skill_ids_json = CASE agent_id WHEN 'worker' THEN '["imagegen","visualize"]' ELSE '["nexus-product-guide"]' END WHERE agent_id IN ('worker', 'existing')`,
	); err != nil {
		t.Fatal(err)
	}

	if err := goose.UpTo(db, migrationDir, 120); err != nil {
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
	if !slices.Equal(skillNames, []string{"imagegen", "visualize", "nexus-product-guide"}) {
		t.Fatalf("migrated skill ids = %#v", skillNames)
	}
	var existingCount int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM json_each((SELECT skill_ids_json FROM runtimes WHERE agent_id = 'existing')) WHERE value = 'nexus-product-guide'`,
	).Scan(&existingCount); err != nil {
		t.Fatal(err)
	}
	if existingCount != 1 {
		t.Fatalf("existing nexus-product-guide count = %d, want 1", existingCount)
	}
}

func TestDefaultVisualizeSkillMigrationUpdatesExistingAgents(t *testing.T) {
	db := openAgentDisabledSkillMigrationTestDB(t, "default-visualize-skill.db")
	migrationDir := providerRecoveryMigrationDir(t)
	if err := goose.UpTo(db, migrationDir, 106); err != nil {
		t.Fatal(err)
	}
	insertRecoveryAgent(t, db, "worker", "owner", "")
	insertRecoveryAgent(t, db, "existing", "owner", "")
	if _, err := db.Exec(
		`UPDATE runtimes SET skill_ids_json = CASE agent_id WHEN 'worker' THEN '["imagegen","goal-manager"]' ELSE '["visualize"]' END WHERE agent_id IN ('worker', 'existing')`,
	); err != nil {
		t.Fatal(err)
	}

	if err := goose.UpTo(db, migrationDir, 107); err != nil {
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
	if !slices.Equal(skillNames, []string{"imagegen", "goal-manager", "visualize"}) {
		t.Fatalf("migrated skill ids = %#v", skillNames)
	}
	var existingCount int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM json_each((SELECT skill_ids_json FROM runtimes WHERE agent_id = 'existing')) WHERE value = 'visualize'`,
	).Scan(&existingCount); err != nil {
		t.Fatal(err)
	}
	if existingCount != 1 {
		t.Fatalf("existing visualize count = %d, want 1", existingCount)
	}
}
