// INPUT: 已打开的 migration 数据库、当前 Goose 版本与数据库驱动。
// OUTPUT: 识别旧版 00071 权限 schema，修复其与当前私有 Skill 00071/权限 00086 的账本碰撞。
// POS: schema migration 前置兼容层；只接受完整旧权限结构，残缺结构保持 fail-closed。
package migration

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/storage"
)

const (
	legacyAutomationPermissionMigrationVersion  = int64(71)
	currentAutomationPermissionMigrationVersion = int64(86)
)

var automationPermissionSchemaColumns = []struct {
	table  string
	column string
}{
	{table: "automation_scheduled_tasks", column: "permission_policy_json"},
	{table: "automation_scheduled_tasks", column: "permission_policy_revision"},
	{table: "automation_scheduled_tasks", column: "permission_state"},
	{table: "automation_scheduled_tasks", column: "pending_permission_request_id"},
	{table: "automation_task_runs", column: "permission_policy_revision"},
	{table: "automation_task_runs", column: "block_state"},
	{table: "automation_task_runs", column: "blocked_request_id"},
	{table: "automation_task_runs", column: "effect_started"},
	{table: "automation_permission_requests", column: "request_id"},
	{table: "automation_permission_requests", column: "owner_user_id"},
	{table: "automation_permission_requests", column: "job_id"},
	{table: "automation_permission_requests", column: "run_id"},
	{table: "automation_permission_requests", column: "policy_revision"},
	{table: "automation_permission_requests", column: "kind"},
	{table: "automation_permission_requests", column: "status"},
	{table: "automation_permission_requests", column: "decision"},
	{table: "automation_permission_requests", column: "tool_name"},
	{table: "automation_permission_requests", column: "connector_id"},
	{table: "automation_permission_requests", column: "effect"},
	{table: "automation_permission_requests", column: "resource_scope"},
	{table: "automation_permission_requests", column: "input_fingerprint"},
	{table: "automation_permission_requests", column: "capability_json"},
	{table: "automation_permission_requests", column: "input_summary_json"},
	{table: "automation_permission_requests", column: "title"},
	{table: "automation_permission_requests", column: "description"},
	{table: "automation_permission_requests", column: "reason"},
	{table: "automation_permission_requests", column: "session_key"},
	{table: "automation_permission_requests", column: "round_id"},
	{table: "automation_permission_requests", column: "tool_use_id"},
	{table: "automation_permission_requests", column: "resume_safe"},
	{table: "automation_permission_requests", column: "resolved_by_user_id"},
	{table: "automation_permission_requests", column: "resolved_at"},
	{table: "automation_permission_requests", column: "created_at"},
	{table: "automation_permission_requests", column: "updated_at"},
}

var automationPermissionSchemaIndexes = []string{
	"idx_automation_permission_requests_owner_status_created",
	"idx_automation_permission_requests_job_created",
	"idx_automation_permission_requests_run_created",
	"uq_automation_permission_requests_pending_capability",
	"idx_automation_scheduled_tasks_permission_state",
	"idx_automation_task_runs_block_state",
}

// RepairLegacyAutomationPermissionMigrationCollision 把旧分支已用 00071 落地的
// 完整定时任务权限 schema 记为当前 00086，并让 Goose 通过 allow-missing 补跑
// 正式的私有 Skill 00071。返回 true 表示补跑尚未完成。
func RepairLegacyAutomationPermissionMigrationCollision(
	ctx context.Context,
	driver string,
	db *sql.DB,
	currentVersion int64,
	logger *slog.Logger,
) (bool, error) {
	present, missing, err := inspectAutomationPermissionSchema(ctx, driver, db)
	if err != nil {
		return false, err
	}
	if present == 0 {
		return false, nil
	}
	if len(missing) > 0 {
		return false, fmt.Errorf(
			"incomplete automation permission schema; refusing ledger repair; missing: %s",
			strings.Join(missing, ", "),
		)
	}

	privateSkillComplete, err := migrationColumnsExist(ctx, driver, db, privateSkillSchemaColumns)
	if err != nil {
		return false, err
	}
	applied, err := appliedMigrationVersions(ctx, db)
	if err != nil {
		return false, err
	}
	_, hasLegacyMarker := applied[legacyAutomationPermissionMigrationVersion]
	_, hasCurrentMarker := applied[currentAutomationPermissionMigrationVersion]
	highestApplied := highestAppliedMigrationVersion(applied)
	expectedCurrent := highestApplied
	if expectedCurrent < currentAutomationPermissionMigrationVersion {
		expectedCurrent = currentAutomationPermissionMigrationVersion
	}
	needsRepair := !hasCurrentMarker ||
		privateSkillComplete != hasLegacyMarker ||
		currentVersion != expectedCurrent
	if needsRepair {
		if err = normalizeAutomationPermissionMigrationLedger(
			ctx,
			db,
			privateSkillComplete,
			highestApplied,
		); err != nil {
			return false, err
		}
		logger.Info(
			"已修复定时任务权限 migration 编号冲突",
			"legacy_version", legacyAutomationPermissionMigrationVersion,
			"current_version", currentAutomationPermissionMigrationVersion,
			"private_skill_replay_pending", !privateSkillComplete,
		)
	}
	if !privateSkillComplete {
		if !needsRepair {
			logger.Info("继续补跑定时任务权限 migration 冲突缺失的私有 Skill schema")
		}
		return true, nil
	}
	return false, nil
}

func inspectAutomationPermissionSchema(
	ctx context.Context,
	driver string,
	db *sql.DB,
) (int, []string, error) {
	present := 0
	missing := make([]string, 0)
	for _, field := range automationPermissionSchemaColumns {
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
	for _, index := range automationPermissionSchemaIndexes {
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

func migrationColumnsExist(
	ctx context.Context,
	driver string,
	db *sql.DB,
	fields []struct {
		table  string
		column string
	},
) (bool, error) {
	for _, field := range fields {
		exists, err := migrationColumnExists(ctx, driver, db, field.table, field.column)
		if err != nil {
			return false, err
		}
		if !exists {
			return false, nil
		}
	}
	return true, nil
}

func migrationIndexExists(
	ctx context.Context,
	driver string,
	db *sql.DB,
	index string,
) (bool, error) {
	if storage.IsSQLiteSQLDriver(storage.NormalizeSQLDriver(driver)) {
		var count int
		if err := db.QueryRowContext(
			ctx,
			"SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?",
			index,
		).Scan(&count); err != nil {
			return false, fmt.Errorf("inspect SQLite index %s: %w", index, err)
		}
		return count > 0, nil
	}

	var exists bool
	if err := db.QueryRowContext(ctx, `
SELECT EXISTS (
    SELECT 1
      FROM pg_indexes
     WHERE schemaname = current_schema()
       AND indexname = $1
)
`, index).Scan(&exists); err != nil {
		return false, fmt.Errorf("inspect PostgreSQL index %s: %w", index, err)
	}
	return exists, nil
}

func highestAppliedMigrationVersion(applied map[int64]struct{}) int64 {
	var highest int64
	for version := range applied {
		if version > highest {
			highest = version
		}
	}
	return highest
}

func normalizeAutomationPermissionMigrationLedger(
	ctx context.Context,
	db *sql.DB,
	privateSkillComplete bool,
	highestApplied int64,
) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin automation permission migration ledger repair: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, version := range []int64{
		legacyAutomationPermissionMigrationVersion,
		currentAutomationPermissionMigrationVersion,
	} {
		if _, err = tx.ExecContext(
			ctx,
			fmt.Sprintf("DELETE FROM goose_db_version WHERE version_id = %d", version),
		); err != nil {
			return fmt.Errorf("remove automation permission migration marker %d: %w", version, err)
		}
	}
	if privateSkillComplete {
		if err = insertAppliedMigrationVersion(ctx, tx, legacyAutomationPermissionMigrationVersion); err != nil {
			return err
		}
	}
	if err = insertAppliedMigrationVersion(ctx, tx, currentAutomationPermissionMigrationVersion); err != nil {
		return err
	}
	if highestApplied > currentAutomationPermissionMigrationVersion {
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
		return fmt.Errorf("commit automation permission migration ledger repair: %w", err)
	}
	return nil
}

func insertAppliedMigrationVersion(ctx context.Context, tx *sql.Tx, version int64) error {
	if _, err := tx.ExecContext(
		ctx,
		fmt.Sprintf(
			"INSERT INTO goose_db_version (version_id, is_applied) VALUES (%d, TRUE)",
			version,
		),
	); err != nil {
		return fmt.Errorf("record migration version %d: %w", version, err)
	}
	return nil
}
