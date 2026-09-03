// INPUT: 已打开的 migration 数据库、数据库驱动与 Goose 账本。
// OUTPUT: 把旧分支占用的 00128/00129 Control schema 映射到当前 00131/00132，并请求补跑被遮蔽的正式迁移。
// POS: schema migration 前置兼容层；只接受完整的 Control schema，混合或残缺状态保持 fail-closed。
package migration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
)

const (
	legacyControlBindingMigrationVersion  = int64(128)
	legacyOwnerProfileMigrationVersion    = int64(129)
	currentControlBindingMigrationVersion = int64(131)
	currentOwnerProfileMigrationVersion   = int64(132)
)

var controlBindingSchemaColumns = []migrationSchemaColumn{
	{table: "local_owner_bindings", column: "deployment_id"},
	{table: "local_owner_bindings", column: "control_user_id"},
	{table: "local_owner_bindings", column: "local_owner_key"},
	{table: "local_owner_bindings", column: "created_at"},
	{table: "local_owner_bindings", column: "updated_at"},
}

var ownerProfileSchemaColumns = []migrationSchemaColumn{
	{table: "owner_profiles", column: "owner_user_id"},
	{table: "owner_profiles", column: "username"},
	{table: "owner_profiles", column: "display_name"},
	{table: "owner_profiles", column: "role"},
	{table: "owner_profiles", column: "status"},
	{table: "owner_profiles", column: "avatar"},
	{table: "owner_profiles", column: "created_at"},
	{table: "owner_profiles", column: "updated_at"},
}

// RepairLegacyControlMigrationCollision 修复未发布 Control 分支与正式
// 00128-00130 迁移的编号碰撞。返回 true 表示 Goose 仍需 allow-missing 补跑。
func RepairLegacyControlMigrationCollision(
	ctx context.Context,
	driver string,
	db *sql.DB,
	logger *slog.Logger,
) (bool, error) {
	bindings, err := inspectCompleteControlSchema(ctx, driver, db, "Control owner binding", controlBindingSchemaColumns)
	if err != nil {
		return false, err
	}
	profiles, err := inspectCompleteControlSchema(ctx, driver, db, "owner profile", ownerProfileSchemaColumns)
	if err != nil {
		return false, err
	}
	applied, err := appliedMigrationVersions(ctx, db)
	if err != nil {
		return false, err
	}

	_, currentBindings := applied[currentControlBindingMigrationVersion]
	_, currentProfiles := applied[currentOwnerProfileMigrationVersion]
	if currentProfiles && !currentBindings {
		return false, errors.New("owner profile migration is applied without Control owner bindings")
	}
	if currentBindings {
		if !bindings || profiles != currentProfiles {
			return false, fmt.Errorf(
				"current Control migration ledger does not match schema: bindings=%t profiles=%t marker_profiles=%t",
				bindings,
				profiles,
				currentProfiles,
			)
		}
		complete, inspectErr := controlCollisionReplayComplete(ctx, driver, db, applied)
		return !complete, inspectErr
	}

	if !bindings && !profiles {
		return false, nil
	}
	if profiles && !bindings {
		return false, errors.New("owner profile schema exists without Control owner bindings")
	}
	canonicalPresent, err := controlCanonicalSchemaPresent(ctx, driver, db)
	if err != nil {
		return false, err
	}
	if canonicalPresent {
		return false, errors.New("Control and canonical Connector migration schemas are mixed without current ledger markers")
	}

	_, legacyBindings := applied[legacyControlBindingMigrationVersion]
	_, legacyProfiles := applied[legacyOwnerProfileMigrationVersion]
	if !legacyBindings || legacyProfiles != profiles {
		return false, fmt.Errorf(
			"legacy Control migration ledger does not match schema: marker_bindings=%t marker_profiles=%t profiles=%t",
			legacyBindings,
			legacyProfiles,
			profiles,
		)
	}
	if err = moveControlMigrationLedger(ctx, db, profiles); err != nil {
		return false, err
	}
	logger.Info(
		"已修复 Control migration 编号冲突",
		"shifted_versions", "128-129 -> 131-132",
		"owner_profiles", profiles,
	)
	return true, nil
}

func inspectCompleteControlSchema(
	ctx context.Context,
	driver string,
	db *sql.DB,
	name string,
	columns []migrationSchemaColumn,
) (bool, error) {
	present, missing, err := inspectMigrationSchema(ctx, driver, db, columns, nil)
	if err != nil {
		return false, err
	}
	if present > 0 && len(missing) > 0 {
		return false, fmt.Errorf("incomplete %s schema; missing: %s", name, strings.Join(missing, ", "))
	}
	return present > 0, nil
}

func controlCanonicalSchemaPresent(ctx context.Context, driver string, db *sql.DB) (bool, error) {
	for _, column := range []migrationSchemaColumn{
		{table: "connector_connections", column: "enabled"},
		{table: "connector_connections", column: "credentials_key_id"},
		{table: "im_channel_accounts", column: "sync_cursor"},
	} {
		exists, err := migrationColumnExists(ctx, driver, db, column.table, column.column)
		if err != nil {
			return false, err
		}
		if exists {
			return true, nil
		}
	}
	return false, nil
}

func controlCollisionReplayComplete(
	ctx context.Context,
	driver string,
	db *sql.DB,
	applied map[int64]struct{},
) (bool, error) {
	for version, column := range map[int64]migrationSchemaColumn{
		128: {table: "connector_connections", column: "enabled"},
		129: {table: "connector_connections", column: "credentials_key_id"},
		130: {table: "im_channel_accounts", column: "sync_cursor"},
	} {
		if _, ok := applied[version]; !ok {
			return false, nil
		}
		exists, err := migrationColumnExists(ctx, driver, db, column.table, column.column)
		if err != nil {
			return false, err
		}
		if !exists {
			return false, nil
		}
	}
	return true, nil
}

func moveControlMigrationLedger(ctx context.Context, db *sql.DB, profiles bool) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin Control migration ledger repair: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err = tx.ExecContext(ctx, "DELETE FROM goose_db_version WHERE version_id IN (128, 129)"); err != nil {
		return fmt.Errorf("remove legacy Control migration markers: %w", err)
	}
	if _, err = tx.ExecContext(ctx, "INSERT INTO goose_db_version (version_id, is_applied) VALUES (131, TRUE)"); err != nil {
		return fmt.Errorf("record current Control owner binding migration: %w", err)
	}
	if profiles {
		if _, err = tx.ExecContext(ctx, "INSERT INTO goose_db_version (version_id, is_applied) VALUES (132, TRUE)"); err != nil {
			return fmt.Errorf("record current owner profile migration: %w", err)
		}
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit Control migration ledger repair: %w", err)
	}
	return nil
}
