package migration

import (
	"testing"

	"github.com/pressly/goose/v3"
)

func TestCustomMCPEnabledMigrationPreservesLegacyAvailability(t *testing.T) {
	db := openAgentDisabledSkillMigrationTestDB(t, "custom-mcp-enabled.db")
	migrationDir := providerRecoveryMigrationDir(t)
	if err := goose.UpTo(db, migrationDir, 127); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		connectorID string
		state       string
	}{
		{connectorID: "custom-mcp:active", state: "connected"},
		{connectorID: "custom-mcp:disabled-preview", state: "disconnected"},
	} {
		if _, err := db.Exec(`
INSERT INTO connector_connections (
    owner_user_id, connector_id, state, credentials, auth_type
) VALUES ('owner-1', ?, ?, '{}', 'custom_mcp')`, item.connectorID, item.state); err != nil {
			t.Fatal(err)
		}
	}

	if err := goose.UpTo(db, migrationDir, 128); err != nil {
		t.Fatal(err)
	}

	for connectorID, wantEnabled := range map[string]bool{
		"custom-mcp:active":           true,
		"custom-mcp:disabled-preview": false,
	} {
		var enabled bool
		if err := db.QueryRow(
			"SELECT enabled FROM connector_connections WHERE connector_id = ?",
			connectorID,
		).Scan(&enabled); err != nil {
			t.Fatal(err)
		}
		if enabled != wantEnabled {
			t.Fatalf("%s enabled = %v, want %v", connectorID, enabled, wantEnabled)
		}
	}
}
