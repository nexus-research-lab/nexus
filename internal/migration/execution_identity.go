// INPUT: 已打开的 migration 数据库、当前 Goose 版本与数据库驱动。
// OUTPUT: 为已应用旧版 00061 但缺少 identity-claim table 的数据库补齐当前 schema。
// POS: migration 前置兼容层；只修复整张 goal_execution_identity_claims 表缺失的历史数据库。
package migration

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/nexus-research-lab/nexus/internal/storage"
)

const executionOrchestrationMigrationVersion = int64(61)

const createGoalExecutionIdentityClaimsTableSQL = `
CREATE TABLE IF NOT EXISTS goal_execution_identity_claims (
    execution_id VARCHAR(64) NOT NULL PRIMARY KEY,
    goal_id VARCHAR(64) NOT NULL,
    goal_objective_revision INTEGER NOT NULL,
    owner_user_id VARCHAR(128),
    claim_state VARCHAR(16) NOT NULL,
    command_id VARCHAR(128) NOT NULL,
    successor_execution_id VARCHAR(64),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT ck_goal_execution_identity_claims_revision
        CHECK (goal_objective_revision > 0),
    CONSTRAINT ck_goal_execution_identity_claims_state
        CHECK (claim_state IN ('materialized', 'fenced')),
    CONSTRAINT ck_goal_execution_identity_claims_successor
        CHECK (
            (claim_state = 'materialized' AND successor_execution_id IS NULL)
            OR
            (claim_state = 'fenced' AND successor_execution_id IS NOT NULL)
        ),
    CONSTRAINT uq_goal_execution_identity_claims_revision
        UNIQUE (goal_id, goal_objective_revision)
)
`

// RepairLegacyExecutionIdentityClaimSchema repairs databases that recorded
// 00061 before goal_execution_identity_claims became part of that migration.
func RepairLegacyExecutionIdentityClaimSchema(
	ctx context.Context,
	driver string,
	db *sql.DB,
	currentVersion int64,
	logger *slog.Logger,
) error {
	if currentVersion < executionOrchestrationMigrationVersion {
		return nil
	}

	exists, err := executionIdentityClaimTableExists(ctx, driver, db)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin execution identity claim schema repair: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err = tx.ExecContext(ctx, createGoalExecutionIdentityClaimsTableSQL); err != nil {
		return fmt.Errorf("create goal_execution_identity_claims: %w", err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit execution identity claim schema repair: %w", err)
	}

	logger.Info(
		"已修复旧版 Execution identity claim schema",
		"migration_version", currentVersion,
		"schema", "goal_execution_identity_claims",
	)
	return nil
}

func executionIdentityClaimTableExists(
	ctx context.Context,
	driver string,
	db *sql.DB,
) (bool, error) {
	if storage.IsSQLiteSQLDriver(storage.NormalizeSQLDriver(driver)) {
		var count int
		if err := db.QueryRowContext(
			ctx,
			"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?",
			"goal_execution_identity_claims",
		).Scan(&count); err != nil {
			return false, fmt.Errorf("inspect SQLite execution identity claim table: %w", err)
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
`, "goal_execution_identity_claims").Scan(&exists); err != nil {
		return false, fmt.Errorf("inspect PostgreSQL execution identity claim table: %w", err)
	}
	return exists, nil
}
