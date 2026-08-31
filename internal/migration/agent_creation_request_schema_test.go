package migration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"
)

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
		"00125_agent_creation_requests.sql",
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
