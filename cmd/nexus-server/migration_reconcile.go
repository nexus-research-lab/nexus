package main

import (
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/storage"
)

const sqliteLegacyVersionTable = "goose_db_version"

var sqliteMigrationVersions = []int64{
	1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17,
}

var sqliteInitialMigrationTables = []string{
	"agents",
	"automation_system_events",
	"auth_sessions",
	"rooms",
	"connector_connections",
	"automation_cron_jobs",
	"automation_delivery_routes",
	"automation_heartbeat_states",
	"contacts",
	"conversations",
	"members",
	"profiles",
	"runtimes",
	"provider",
	"automation_cron_runs",
	"sessions",
	"messages",
	"rounds",
}

type sqliteSchemaMarker struct {
	version int64
	tables  []string
	columns map[string][]string
	indexes []string
}

var sqliteLegacySchemaMarkers = []sqliteSchemaMarker{
	{version: 2, tables: []string{"users", "auth_password_credentials"}, columns: map[string][]string{"auth_sessions": {"session_id", "user_id", "auth_method"}}},
	{version: 3, columns: map[string][]string{"agents": {"owner_user_id", "is_main"}, "rooms": {"owner_user_id"}}},
	{version: 5, tables: []string{"connector_oauth_states"}},
	{version: 6, columns: map[string][]string{"connector_connections": {"credentials_encrypted"}}},
	{version: 7, tables: []string{"connector_oauth_clients"}},
	{version: 8, tables: []string{"token_usage_records"}},
	{version: 9, columns: map[string][]string{"users": {"avatar"}}},
	{version: 10, indexes: []string{"idx_rooms_owner_updated", "idx_agents_owner_status_main_created"}},
	{version: 11, columns: map[string][]string{"automation_cron_jobs": {"owner_user_id", "overlap_policy"}, "automation_cron_runs": {"owner_user_id", "trigger_kind", "result_summary"}}},
	{version: 12, tables: []string{"im_channel_configs", "im_pairings"}},
	{version: 13, columns: map[string][]string{"rooms": {"skill_names"}}},
	{version: 14, columns: map[string][]string{"provider": {"provider_kind"}}},
	{version: 15, columns: map[string][]string{"connector_oauth_states": {"redirect_kind"}}},
	{version: 16, columns: map[string][]string{"rooms": {"host_agent_id", "host_auto_reply_enabled"}}},
	{
		version: 17,
		tables:  []string{"provider_models", "im_ingress_messages", "automation_task_events"},
		columns: map[string][]string{
			"provider":             {"preset_key", "api_format", "models_path", "last_test_status"},
			"runtimes":             {"model"},
			"automation_cron_jobs": {"next_run_at", "running_run_id", "last_delivery_status", "execution_kind"},
			"automation_cron_runs": {
				"assistant_text",
				"result_text",
				"artifact_path",
				"delivery_status",
				"delivery_error",
				"delivered_at",
				"delivery_attempts",
				"delivery_next_attempt_at",
				"delivery_dead_letter_at",
			},
		},
	},
}

func reconcileLegacySQLiteMigrationVersion(db *sql.DB, driver string, currentVersion int64, logger *slog.Logger) (int64, error) {
	if currentVersion != 0 || !storage.IsSQLiteSQLDriver(storage.NormalizeSQLDriver(driver)) {
		return currentVersion, nil
	}

	initialPresent, missingInitial, err := sqliteRequiredTablesStatus(db, sqliteInitialMigrationTables)
	if err != nil {
		return currentVersion, fmt.Errorf("inspect sqlite legacy schema: %w", err)
	}
	if initialPresent == 0 {
		return currentVersion, nil
	}
	if len(missingInitial) > 0 {
		return currentVersion, fmt.Errorf("sqlite legacy schema is partial while goose version is 0; missing initial tables: %s", strings.Join(missingInitial, ", "))
	}

	repairedVersion, err := detectSQLiteLegacySchemaVersion(db)
	if err != nil {
		return currentVersion, err
	}
	if repairedVersion == 0 {
		return currentVersion, nil
	}

	if err = markSQLiteMigrationVersionsApplied(db, repairedVersion); err != nil {
		return currentVersion, err
	}
	logger.Info("已修复 SQLite legacy migration 版本", "detected_version", repairedVersion)
	return repairedVersion, nil
}

func detectSQLiteLegacySchemaVersion(db *sql.DB) (int64, error) {
	version := int64(1)
	for _, marker := range sqliteLegacySchemaMarkers {
		matched, err := sqliteSchemaMarkerMatches(db, marker)
		if err != nil {
			return 0, fmt.Errorf("inspect sqlite migration marker %d: %w", marker.version, err)
		}
		if matched && marker.version > version {
			version = marker.version
		}
	}
	return version, nil
}

func sqliteSchemaMarkerMatches(db *sql.DB, marker sqliteSchemaMarker) (bool, error) {
	for _, table := range marker.tables {
		exists, err := sqliteTableExists(db, table)
		if err != nil || !exists {
			return false, err
		}
	}
	for table, columns := range marker.columns {
		for _, column := range columns {
			exists, err := sqliteColumnExists(db, table, column)
			if err != nil || !exists {
				return false, err
			}
		}
	}
	for _, index := range marker.indexes {
		exists, err := sqliteIndexExists(db, index)
		if err != nil || !exists {
			return false, err
		}
	}
	return true, nil
}

func sqliteRequiredTablesStatus(db *sql.DB, tables []string) (int, []string, error) {
	present := 0
	var missing []string
	for _, table := range tables {
		exists, err := sqliteTableExists(db, table)
		if err != nil {
			return 0, nil, err
		}
		if exists {
			present++
		} else {
			missing = append(missing, table)
		}
	}
	return present, missing, nil
}

func sqliteTableExists(db *sql.DB, table string) (bool, error) {
	return sqliteObjectExists(db, "table", table)
}

func sqliteIndexExists(db *sql.DB, index string) (bool, error) {
	return sqliteObjectExists(db, "index", index)
}

func sqliteObjectExists(db *sql.DB, objectType string, name string) (bool, error) {
	var count int
	err := db.QueryRow(
		"SELECT COUNT(1) FROM sqlite_master WHERE type = ? AND name = ?",
		objectType,
		name,
	).Scan(&count)
	return count > 0, err
}

func sqliteColumnExists(db *sql.DB, table string, column string) (bool, error) {
	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err = rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}

func markSQLiteMigrationVersionsApplied(db *sql.DB, version int64) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, migrationVersion := range sqliteMigrationVersions {
		if migrationVersion > version {
			break
		}
		if _, err = tx.Exec(
			"INSERT INTO "+sqliteLegacyVersionTable+" (version_id, is_applied) VALUES (?, ?)",
			migrationVersion,
			true,
		); err != nil {
			return fmt.Errorf("mark sqlite migration %d applied: %w", migrationVersion, err)
		}
	}
	return tx.Commit()
}
