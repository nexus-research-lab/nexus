// INPUT: 已打开的 migration 数据库、数据库驱动与旧版 00121-00125 Automation/Agent schema 及账本。
// OUTPUT: 把旧 00121-00125 映射到当前 00122-00126，并请求 Goose 补跑被遮蔽的 Agent 标签 00121。
// POS: schema migration 前置兼容层；只接受连续完整旧结构，残缺、跳号或无账本结构保持 fail-closed。
package migration

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sort"
	"strings"
)

const (
	agentBusinessTagsMigrationVersion    = int64(121)
	firstShiftedRecoveryMigrationVersion = int64(122)
	currentAgentCreationMigrationVersion = int64(126)
)

type migrationSchemaColumn struct {
	table  string
	column string
}

type shiftedMigrationSchema struct {
	version int64
	name    string
	columns []migrationSchemaColumn
	indexes []string
}

var agentCreationSchemaColumns = []migrationSchemaColumn{
	{table: "agent_creation_requests", column: "owner_user_id"},
	{table: "agent_creation_requests", column: "creation_request_id"},
	{table: "agent_creation_requests", column: "intent_digest"},
	{table: "agent_creation_requests", column: "agent_id"},
	{table: "agent_creation_requests", column: "workspace_path"},
	{table: "agent_creation_requests", column: "status"},
	{table: "agent_creation_requests", column: "stage"},
	{table: "agent_creation_requests", column: "claim_token"},
	{table: "agent_creation_requests", column: "lease_expires_at_ms"},
	{table: "agent_creation_requests", column: "failure_code"},
	{table: "agent_creation_requests", column: "created_at"},
	{table: "agent_creation_requests", column: "updated_at"},
}

var automationHeartbeatWakeSchemaColumns = []migrationSchemaColumn{
	{table: "automation_system_events", column: "owner_user_id"},
	{table: "automation_system_events", column: "request_id"},
	{table: "automation_system_events", column: "intent_digest"},
	{table: "automation_system_events", column: "accepted_configuration_version"},
	{table: "automation_system_events", column: "claim_token"},
	{table: "automation_system_events", column: "claim_expires_at"},
}

var automationHeartbeatWakeSchemaIndexes = []string{
	"uq_automation_heartbeat_wake_request",
	"idx_automation_heartbeat_wake_due",
}

var shiftedRecoveryMigrationSchemas = []shiftedMigrationSchema{
	{
		version: 122,
		name:    "Automation delivery attempt claim",
		columns: []migrationSchemaColumn{
			{table: "automation_task_runs", column: "delivery_attempt_id"},
			{table: "automation_task_runs", column: "delivery_attempt_started_at"},
			{table: "automation_scheduled_tasks", column: "last_completed_run_id"},
		},
		indexes: []string{"idx_automation_task_runs_delivery_due"},
	},
	{
		version: 123,
		name:    "Automation task deletion claim",
		columns: []migrationSchemaColumn{
			{table: "automation_scheduled_tasks", column: "deletion_state"},
			{table: "automation_scheduled_tasks", column: "deletion_token"},
			{table: "automation_scheduled_tasks", column: "deletion_claimed_at"},
		},
	},
	{
		version: 124,
		name:    "Automation run request identity",
		columns: []migrationSchemaColumn{
			{table: "automation_task_runs", column: "client_request_id"},
			{table: "automation_task_runs", column: "client_intent_digest"},
		},
		indexes: []string{"uq_automation_task_runs_owner_request"},
	},
	{
		version: 125,
		name:    "Automation Heartbeat wake",
		columns: automationHeartbeatWakeSchemaColumns,
		indexes: automationHeartbeatWakeSchemaIndexes,
	},
	{
		version: 126,
		name:    "Agent creation receipt",
		columns: agentCreationSchemaColumns,
		indexes: []string{"idx_agent_creation_requests_agent"},
	},
}

// RepairLegacyAgentCreationMigrationCollision maps the unpublished recovery
// migration range from 00121-00125 to 00122-00126. It returns true while Goose
// still needs allow-missing to apply the canonical Agent business tags 00121.
func RepairLegacyAgentCreationMigrationCollision(
	ctx context.Context,
	driver string,
	db *sql.DB,
	logger *slog.Logger,
) (bool, error) {
	businessTags, err := migrationColumnExists(ctx, driver, db, "agents", "business_tags")
	if err != nil {
		return false, err
	}

	highestSchemaVersion := int64(0)
	foundSchemaGap := false
	for _, schema := range shiftedRecoveryMigrationSchemas {
		present, missing, inspectErr := inspectMigrationSchema(
			ctx,
			driver,
			db,
			schema.columns,
			schema.indexes,
		)
		if inspectErr != nil {
			return false, inspectErr
		}
		if present > 0 && len(missing) > 0 {
			return false, fmt.Errorf(
				"incomplete %s schema; refusing ledger repair; missing: %s",
				schema.name,
				strings.Join(missing, ", "),
			)
		}
		if present == 0 {
			foundSchemaGap = true
			continue
		}
		if foundSchemaGap {
			return false, fmt.Errorf(
				"non-contiguous recovery migration schema at version %d (%s)",
				schema.version,
				schema.name,
			)
		}
		highestSchemaVersion = schema.version
	}

	applied, err := appliedMigrationVersions(ctx, db)
	if err != nil {
		return false, err
	}
	_, businessTagsMarker := applied[agentBusinessTagsMigrationVersion]
	if highestSchemaVersion == 0 {
		if !businessTags && businessTagsMarker {
			return false, fmt.Errorf(
				"migration %d is marked applied but Agent business tags and shifted recovery schemas are absent",
				agentBusinessTagsMigrationVersion,
			)
		}
		return false, nil
	}
	for version := highestSchemaVersion + 1; version <= currentAgentCreationMigrationVersion; version++ {
		if _, exists := applied[version]; exists {
			return false, fmt.Errorf(
				"migration %d is marked applied but its recovery schema is absent",
				version,
			)
		}
	}

	legacyMarkers := migrationRangeApplied(
		applied,
		agentBusinessTagsMigrationVersion,
		highestSchemaVersion-1,
	)
	currentMarkers := migrationRangeApplied(
		applied,
		firstShiftedRecoveryMigrationVersion,
		highestSchemaVersion,
	)
	if !legacyMarkers && !currentMarkers {
		return false, fmt.Errorf(
			"complete shifted recovery schema through version %d has neither legacy nor current migration ledger",
			highestSchemaVersion,
		)
	}

	highestApplied := highestAppliedMigrationVersion(applied)
	currentVersion, err := currentMigrationLedgerVersion(ctx, db)
	if err != nil {
		return false, err
	}
	needsRepair := !businessTags ||
		!businessTagsMarker ||
		!currentMarkers ||
		currentVersion != highestApplied
	if needsRepair {
		if err = normalizeShiftedRecoveryMigrationLedger(
			ctx,
			db,
			businessTags,
			highestSchemaVersion,
			highestApplied,
		); err != nil {
			return false, err
		}
		logger.Info(
			"已修复 Agent 标签与恢复 migration 编号冲突",
			"shifted_versions", "121-125 -> 122-126",
			"highest_schema_version", highestSchemaVersion,
			"business_tags_replay_pending", !businessTags,
		)
	}
	if !businessTags {
		return true, nil
	}
	return false, nil
}

func migrationRangeApplied(applied map[int64]struct{}, first int64, last int64) bool {
	if last < first {
		return true
	}
	for version := first; version <= last; version++ {
		if _, exists := applied[version]; !exists {
			return false
		}
	}
	return true
}

func currentMigrationLedgerVersion(ctx context.Context, db *sql.DB) (int64, error) {
	var version int64
	if err := db.QueryRowContext(ctx, `
SELECT version_id
FROM goose_db_version
WHERE is_applied = TRUE
ORDER BY id DESC
LIMIT 1
`).Scan(&version); err != nil {
		return 0, fmt.Errorf("read current migration ledger version: %w", err)
	}
	return version, nil
}

func inspectMigrationSchema(
	ctx context.Context,
	driver string,
	db *sql.DB,
	columns []migrationSchemaColumn,
	indexes []string,
) (int, []string, error) {
	present := 0
	missing := make([]string, 0)
	for _, field := range columns {
		exists, err := migrationColumnExists(ctx, driver, db, field.table, field.column)
		if err != nil {
			return 0, nil, err
		}
		name := field.table + "." + field.column
		if exists {
			present++
		} else {
			missing = append(missing, name)
		}
	}
	for _, index := range indexes {
		exists, err := migrationIndexExists(ctx, driver, db, index)
		if err != nil {
			return 0, nil, err
		}
		if exists {
			present++
		} else {
			missing = append(missing, index)
		}
	}
	sort.Strings(missing)
	return present, missing, nil
}

func normalizeShiftedRecoveryMigrationLedger(
	ctx context.Context,
	db *sql.DB,
	businessTagsComplete bool,
	highestSchemaVersion int64,
	highestApplied int64,
) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin shifted recovery migration ledger repair: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for version := agentBusinessTagsMigrationVersion; version <= currentAgentCreationMigrationVersion; version++ {
		if _, err = tx.ExecContext(
			ctx,
			fmt.Sprintf("DELETE FROM goose_db_version WHERE version_id = %d", version),
		); err != nil {
			return fmt.Errorf("remove shifted recovery migration marker %d: %w", version, err)
		}
	}
	if businessTagsComplete {
		if err = insertAppliedMigrationVersion(ctx, tx, agentBusinessTagsMigrationVersion); err != nil {
			return err
		}
	}
	for version := firstShiftedRecoveryMigrationVersion; version <= highestSchemaVersion; version++ {
		if err = insertAppliedMigrationVersion(ctx, tx, version); err != nil {
			return err
		}
	}
	if highestApplied > currentAgentCreationMigrationVersion {
		if _, err = tx.ExecContext(
			ctx,
			fmt.Sprintf("DELETE FROM goose_db_version WHERE version_id = %d", highestApplied),
		); err != nil {
			return fmt.Errorf("preserve latest migration marker %d: %w", highestApplied, err)
		}
		if err = insertAppliedMigrationVersion(ctx, tx, highestApplied); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit shifted recovery migration ledger repair: %w", err)
	}
	return nil
}
