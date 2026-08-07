// INPUT: 已打开的 migration 数据库、当前 Goose 版本与数据库驱动。
// OUTPUT: 修复旧版 00061 私有 Skill 迁移与 Execution 迁移的账本碰撞。
// POS: schema migration 前置兼容层；只识别私有 Skill schema 已落地但 Execution schema 缺失的精确旧库。
package migration

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/nexus-research-lab/nexus/internal/storage"
)

const (
	legacyPrivateSkillMigrationVersion  = int64(61)
	currentPrivateSkillMigrationVersion = int64(71)
)

var privateSkillSchemaColumns = []struct {
	table  string
	column string
}{
	{table: "skill_sources", column: "managed_by"},
	{table: "skill_sources", column: "auth_type"},
	{table: "skill_sources", column: "credentials_encrypted"},
	{table: "imported_skills", column: "source_skill_id"},
	{table: "imported_skills", column: "artifact_sha256"},
}

// RepairLegacyPrivateSkillMigrationCollision 把旧版私有 Skill 00061 账本迁到
// 00071，让 Goose 通过 allow-missing 按原顺序补跑真正的 Execution 00061-00070。
func RepairLegacyPrivateSkillMigrationCollision(
	ctx context.Context,
	driver string,
	db *sql.DB,
	currentVersion int64,
	logger *slog.Logger,
) (bool, error) {
	applied, err := appliedMigrationVersions(ctx, db)
	if err != nil {
		return false, err
	}
	if _, hasReplayMarker := applied[currentPrivateSkillMigrationVersion]; hasReplayMarker {
		missing := missingExecutionMigrationVersions(applied)
		if len(missing) > 0 {
			logger.Info(
				"继续补跑私有 Skill migration 冲突缺失的 Execution schema",
				"missing_versions", missing,
			)
			return true, nil
		}
		if currentVersion < currentPrivateSkillMigrationVersion {
			if err = movePrivateSkillMigrationLedger(ctx, db, false); err != nil {
				return false, err
			}
			logger.Info(
				"已收口私有 Skill migration 冲突账本",
				"current_version", currentPrivateSkillMigrationVersion,
			)
		}
		return false, nil
	}
	if currentVersion != legacyPrivateSkillMigrationVersion {
		return false, nil
	}

	hasExecutionSchema, err := migrationTableExists(ctx, driver, db, "executions")
	if err != nil {
		return false, err
	}
	if hasExecutionSchema {
		return false, nil
	}
	for _, field := range privateSkillSchemaColumns {
		exists, columnErr := migrationColumnExists(ctx, driver, db, field.table, field.column)
		if columnErr != nil {
			return false, columnErr
		}
		if !exists {
			return false, nil
		}
	}

	if err = movePrivateSkillMigrationLedger(ctx, db, true); err != nil {
		return false, err
	}

	logger.Info(
		"已修复私有 Skill 与 Execution migration 编号冲突",
		"previous_version", legacyPrivateSkillMigrationVersion,
		"current_version", currentPrivateSkillMigrationVersion,
	)
	return true, nil
}

func appliedMigrationVersions(ctx context.Context, db *sql.DB) (map[int64]struct{}, error) {
	rows, err := db.QueryContext(ctx, `
SELECT version_id
FROM goose_db_version
WHERE is_applied = TRUE
`)
	if err != nil {
		return nil, fmt.Errorf("read applied migration versions: %w", err)
	}
	defer rows.Close()

	versions := make(map[int64]struct{})
	for rows.Next() {
		var version int64
		if err = rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("scan applied migration version: %w", err)
		}
		versions[version] = struct{}{}
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate applied migration versions: %w", err)
	}
	return versions, nil
}

func missingExecutionMigrationVersions(applied map[int64]struct{}) []int64 {
	missing := make([]int64, 0)
	for version := legacyPrivateSkillMigrationVersion; version < currentPrivateSkillMigrationVersion; version++ {
		if _, ok := applied[version]; !ok {
			missing = append(missing, version)
		}
	}
	return missing
}

func movePrivateSkillMigrationLedger(
	ctx context.Context,
	db *sql.DB,
	removeLegacyVersion bool,
) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin private Skill migration ledger repair: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if removeLegacyVersion {
		if _, err = tx.ExecContext(ctx, "DELETE FROM goose_db_version WHERE version_id = 61"); err != nil {
			return fmt.Errorf("remove legacy private Skill migration version: %w", err)
		}
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM goose_db_version WHERE version_id = 71"); err != nil {
		return fmt.Errorf("remove stale private Skill migration marker: %w", err)
	}
	if _, err = tx.ExecContext(ctx, `
INSERT INTO goose_db_version (version_id, is_applied) VALUES (71, TRUE)
`); err != nil {
		return fmt.Errorf("record current private Skill migration version: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit private Skill migration ledger repair: %w", err)
	}
	return nil
}

func migrationTableExists(
	ctx context.Context,
	driver string,
	db *sql.DB,
	table string,
) (bool, error) {
	if storage.IsSQLiteSQLDriver(storage.NormalizeSQLDriver(driver)) {
		var count int
		if err := db.QueryRowContext(
			ctx,
			"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?",
			table,
		).Scan(&count); err != nil {
			return false, fmt.Errorf("inspect SQLite table %s: %w", table, err)
		}
		return count > 0, nil
	}

	var exists bool
	if err := db.QueryRowContext(ctx, `
SELECT EXISTS (
    SELECT 1
      FROM information_schema.tables
     WHERE table_schema = current_schema()
       AND table_name = $1
)
`, table).Scan(&exists); err != nil {
		return false, fmt.Errorf("inspect PostgreSQL table %s: %w", table, err)
	}
	return exists, nil
}

func migrationColumnExists(
	ctx context.Context,
	driver string,
	db *sql.DB,
	table string,
	column string,
) (bool, error) {
	if storage.IsSQLiteSQLDriver(storage.NormalizeSQLDriver(driver)) {
		return sqliteColumnExists(ctx, db, table, column)
	}

	var exists bool
	if err := db.QueryRowContext(ctx, `
SELECT EXISTS (
    SELECT 1
      FROM information_schema.columns
     WHERE table_schema = current_schema()
       AND table_name = $1
       AND column_name = $2
)
`, table, column).Scan(&exists); err != nil {
		return false, fmt.Errorf("inspect PostgreSQL column %s.%s: %w", table, column, err)
	}
	return exists, nil
}
