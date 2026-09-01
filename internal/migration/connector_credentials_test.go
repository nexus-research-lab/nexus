package migration

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/connectors/credentials"

	"github.com/pressly/goose/v3"
)

const (
	migrationActiveKey  = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="
	migrationLegacyKey  = "YWJjZGVmMDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODk="
	migrationUnknownKey = "enl4d3Z1dHNycXBvbm1sa2ppaGdmZWRjYmEwMTIzNDU="
)

func TestRunConnectorCredentialKeyMigrationResumesMixedKeyRows(t *testing.T) {
	databaseURL := filepath.Join(t.TempDir(), "connector-keyring.db")
	db, err := sql.Open("sqlite", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err = goose.Up(db, providerRecoveryMigrationDir(t)); err != nil {
		t.Fatal(err)
	}

	activeKey := decodeMigrationKey(t, migrationActiveKey)
	legacyKey := decodeMigrationKey(t, migrationLegacyKey)
	unknownKey := decodeMigrationKey(t, migrationUnknownKey)
	activeKeyID := credentials.KeyID(activeKey)
	legacyKeyID := credentials.KeyID(legacyKey)
	unknownEncrypted := insertMigrationCredential(t, db, "unknown", unknownKey, "", "unknown")
	insertMigrationCredential(t, db, "active-current", activeKey, activeKeyID, "active-current")
	insertMigrationCredential(t, db, "active-unidentified", activeKey, "", "active-unidentified")
	insertMigrationCredential(t, db, "legacy-unidentified", legacyKey, "", "legacy-unidentified")
	insertMigrationCredential(t, db, "legacy-identified", legacyKey, legacyKeyID, "legacy-identified")
	if _, err = db.Exec(`
INSERT INTO connector_connections (
    owner_user_id, connector_id, state, enabled, credentials,
    credentials_encrypted, credentials_key_id, auth_type
) VALUES ('owner-1', 'custom-mcp:corrupt', 'connected', TRUE, '__encrypted__', 'v1:not-valid', ?, 'custom_mcp')`, legacyKeyID); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}

	cfg := config.Config{
		DatabaseDriver:                 "sqlite",
		DatabaseURL:                    databaseURL,
		ConnectorCredentialsKey:        migrationActiveKey,
		ConnectorCredentialsLegacyKeys: []string{migrationLegacyKey},
	}
	report, err := RunConnectorCredentialKeyMigration(t.Context(), cfg, discardMigrationLogger())
	if err != nil {
		t.Fatal(err)
	}
	if report.Scanned != 6 || report.AlreadyCurrent != 1 || report.Identified != 1 ||
		report.Reencrypted != 2 || report.RecoveryRequired != 1 || report.Corrupt != 1 ||
		report.Conflicted != 0 || !report.KeyringAvailable {
		t.Fatalf("first migration report mismatch: %+v", report)
	}

	db, err = sql.Open("sqlite", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	for _, connectorID := range []string{
		"active-current",
		"active-unidentified",
		"legacy-unidentified",
		"legacy-identified",
	} {
		var encrypted string
		var keyID sql.NullString
		if err = db.QueryRow(`
SELECT credentials_encrypted, credentials_key_id
  FROM connector_connections
 WHERE connector_id = ?`, "custom-mcp:"+connectorID).Scan(&encrypted, &keyID); err != nil {
			t.Fatal(err)
		}
		plain, decryptErr := credentials.DecryptPayload(activeKey, encrypted)
		if decryptErr != nil || string(plain) != connectorID || !keyID.Valid || keyID.String != activeKeyID {
			t.Fatalf("%s was not migrated to active key: plain=%q key_id=%q err=%v", connectorID, plain, keyID.String, decryptErr)
		}
	}
	var unknownAfter string
	var unknownKeyID sql.NullString
	if err = db.QueryRow(`
SELECT credentials_encrypted, credentials_key_id
  FROM connector_connections
 WHERE connector_id = 'custom-mcp:unknown'`).Scan(&unknownAfter, &unknownKeyID); err != nil {
		t.Fatal(err)
	}
	if unknownAfter != unknownEncrypted || unknownKeyID.Valid {
		t.Fatalf("unknown-key row must remain untouched: key_id=%q", unknownKeyID.String)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}

	repeated, err := RunConnectorCredentialKeyMigration(t.Context(), cfg, discardMigrationLogger())
	if err != nil {
		t.Fatal(err)
	}
	if repeated.AlreadyCurrent != 4 || repeated.Identified != 0 || repeated.Reencrypted != 0 ||
		repeated.RecoveryRequired != 1 || repeated.Corrupt != 1 {
		t.Fatalf("repeated migration must resume idempotently: %+v", repeated)
	}
}

func decodeMigrationKey(t *testing.T, raw string) []byte {
	t.Helper()
	key, err := credentials.DecodeKey(raw)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func insertMigrationCredential(
	t *testing.T,
	db *sql.DB,
	suffix string,
	key []byte,
	keyID string,
	plain string,
) string {
	t.Helper()
	encrypted, err := credentials.EncryptPayload(key, []byte(plain))
	if err != nil {
		t.Fatal(err)
	}
	var storedKeyID any
	if keyID != "" {
		storedKeyID = keyID
	}
	if _, err = db.Exec(`
INSERT INTO connector_connections (
    owner_user_id, connector_id, state, enabled, credentials,
    credentials_encrypted, credentials_key_id, auth_type
) VALUES ('owner-1', ?, 'connected', TRUE, '__encrypted__', ?, ?, 'custom_mcp')`,
		"custom-mcp:"+suffix,
		encrypted,
		storedKeyID,
	); err != nil {
		t.Fatal(err)
	}
	return encrypted
}
