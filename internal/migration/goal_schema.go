// INPUT: 已打开的 migration 数据库、数据库驱动与旧 Goal 分支的 00087-00089 账本/schema。
// OUTPUT: 把完整旧 Goal schema 映射到当前 00097-00099，并请求 Goose 补跑被旧编号遮蔽的 main 迁移。
// POS: schema migration 前置兼容层；只接受完整旧 Goal 结构，残缺或含糊状态保持 fail-closed。
package migration

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/storage"
)

const (
	legacyGoalActivationMigrationVersion    = int64(87)
	legacyGoalConfirmationMigrationVersion  = int64(88)
	legacyGoalTokenRepairMigrationVersion   = int64(89)
	currentGoalActivationMigrationVersion   = int64(97)
	currentGoalConfirmationMigrationVersion = int64(98)
	currentGoalTokenRepairMigrationVersion  = int64(99)
)

var agentContactSchemaColumns = []struct {
	table  string
	column string
}{
	{table: "rooms", column: "is_contact_channel"},
	{table: "contacts", column: "direct_room_id"},
}

// RepairLegacyGoalMigrationCollision maps the unpublished Goal branch's
// 00087-00089 ledger onto 00097-00099. It returns true when Goose must use
// allow-missing to apply the main migrations that the old 00087 marker hid.
func RepairLegacyGoalMigrationCollision(
	ctx context.Context,
	driver string,
	db *sql.DB,
	logger *slog.Logger,
) (bool, error) {
	applied, err := appliedMigrationVersions(ctx, db)
	if err != nil {
		return false, err
	}
	_, currentActivationApplied := applied[currentGoalActivationMigrationVersion]
	_, currentConfirmationApplied := applied[currentGoalConfirmationMigrationVersion]
	if currentActivationApplied != currentConfirmationApplied {
		return false, fmt.Errorf(
			"incomplete current Goal migration ledger: versions %d/%d applied=%t/%t",
			currentGoalActivationMigrationVersion,
			currentGoalConfirmationMigrationVersion,
			currentActivationApplied,
			currentConfirmationApplied,
		)
	}

	activationExpanded, err := goalActivationReasonExpanded(ctx, driver, db)
	if err != nil {
		return false, err
	}
	confirmationTable, err := migrationTableExists(ctx, driver, db, "execution_goal_confirmations")
	if err != nil {
		return false, err
	}
	contactSchema, err := inspectAgentContactSchema(ctx, driver, db)
	if err != nil {
		return false, err
	}
	if contactSchema > 0 && contactSchema < len(agentContactSchemaColumns)+1 {
		return false, fmt.Errorf(
			"incomplete Agent contact migration schema; refusing Goal ledger repair: present=%d expected=%d",
			contactSchema,
			len(agentContactSchemaColumns)+1,
		)
	}
	if currentActivationApplied {
		if !activationExpanded || !confirmationTable {
			return false, fmt.Errorf(
				"incomplete current Goal migration schema; activation_expanded=%t confirmation_table=%t",
				activationExpanded,
				confirmationTable,
			)
		}
		return goalMigrationReplayPending(applied, contactSchema)
	}

	_, hasLegacyActivation := applied[legacyGoalActivationMigrationVersion]
	_, hasLegacyConfirmation := applied[legacyGoalConfirmationMigrationVersion]
	_, hasLegacyTokenRepair := applied[legacyGoalTokenRepairMigrationVersion]
	legacyEvidence := activationExpanded || confirmationTable || hasLegacyConfirmation || hasLegacyTokenRepair
	if !legacyEvidence {
		return false, nil
	}
	if !activationExpanded || !confirmationTable || !hasLegacyActivation ||
		!hasLegacyConfirmation || !hasLegacyTokenRepair {
		return false, fmt.Errorf(
			"incomplete legacy Goal migration schema; refusing ledger repair: activation_expanded=%t confirmation_table=%t versions_87_88_89=%t/%t/%t",
			activationExpanded,
			confirmationTable,
			hasLegacyActivation,
			hasLegacyConfirmation,
			hasLegacyTokenRepair,
		)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("begin legacy Goal migration ledger repair: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	versionsToRemove := []int64{
		legacyGoalConfirmationMigrationVersion,
		legacyGoalTokenRepairMigrationVersion,
		currentGoalActivationMigrationVersion,
		currentGoalConfirmationMigrationVersion,
	}
	if contactSchema == 0 {
		versionsToRemove = append(versionsToRemove, legacyGoalActivationMigrationVersion)
	}
	for _, version := range versionsToRemove {
		if _, err = tx.ExecContext(
			ctx,
			fmt.Sprintf("DELETE FROM goose_db_version WHERE version_id = %d", version),
		); err != nil {
			return false, fmt.Errorf("remove legacy Goal migration marker %d: %w", version, err)
		}
	}
	if err = insertAppliedMigrationVersion(ctx, tx, currentGoalActivationMigrationVersion); err != nil {
		return false, err
	}
	if err = insertAppliedMigrationVersion(ctx, tx, currentGoalConfirmationMigrationVersion); err != nil {
		return false, err
	}
	if err = tx.Commit(); err != nil {
		return false, fmt.Errorf("commit legacy Goal migration ledger repair: %w", err)
	}

	logger.Info(
		"已修复 Goal migration 编号冲突",
		"legacy_versions", "87-89",
		"current_versions", "97-99",
		"agent_contact_replay_pending", contactSchema == 0,
		"goal_token_repair_replay_pending", true,
	)
	return true, nil
}

func goalMigrationReplayPending(applied map[int64]struct{}, contactSchema int) (bool, error) {
	_, contactMarker := applied[legacyGoalActivationMigrationVersion]
	contactComplete := contactSchema == len(agentContactSchemaColumns)+1
	if contactComplete != contactMarker {
		if contactSchema == 0 && !contactMarker {
			return true, nil
		}
		return false, fmt.Errorf(
			"Agent contact migration schema/ledger mismatch: schema_complete=%t version_87_applied=%t",
			contactComplete,
			contactMarker,
		)
	}
	for _, version := range []int64{93, 94, 95, 96, currentGoalTokenRepairMigrationVersion} {
		if _, ok := applied[version]; !ok {
			return true, nil
		}
	}
	return false, nil
}

func goalActivationReasonExpanded(ctx context.Context, driver string, db *sql.DB) (bool, error) {
	if storage.IsSQLiteSQLDriver(storage.NormalizeSQLDriver(driver)) {
		var definition sql.NullString
		if err := db.QueryRowContext(ctx, `
SELECT sql
FROM sqlite_master
WHERE type = 'table' AND name = 'executions'
`).Scan(&definition); err != nil {
			if err == sql.ErrNoRows {
				return false, nil
			}
			return false, fmt.Errorf("inspect SQLite executions constraint: %w", err)
		}
		return strings.Contains(definition.String, "'substantial_complexity'"), nil
	}

	var expanded bool
	if err := db.QueryRowContext(ctx, `
SELECT EXISTS (
    SELECT 1
      FROM pg_constraint AS constraint_value
      JOIN pg_class AS table_value ON table_value.oid = constraint_value.conrelid
     WHERE table_value.relname = 'executions'
       AND constraint_value.conname = 'ck_executions_goal_activation_reason'
       AND pg_get_constraintdef(constraint_value.oid) LIKE '%substantial_complexity%'
)
`).Scan(&expanded); err != nil {
		return false, fmt.Errorf("inspect PostgreSQL executions constraint: %w", err)
	}
	return expanded, nil
}

func inspectAgentContactSchema(ctx context.Context, driver string, db *sql.DB) (int, error) {
	present := 0
	for _, field := range agentContactSchemaColumns {
		exists, err := migrationColumnExists(ctx, driver, db, field.table, field.column)
		if err != nil {
			return 0, err
		}
		if exists {
			present++
		}
	}
	indexExists, err := migrationIndexExists(ctx, driver, db, "idx_contacts_direct_room")
	if err != nil {
		return 0, err
	}
	if indexExists {
		present++
	}
	return present, nil
}
