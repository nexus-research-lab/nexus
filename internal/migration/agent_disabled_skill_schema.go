// INPUT: 已打开的 migration 数据库、当前 Goose 版本与数据库驱动。
// OUTPUT: 修复旧版 00056 编号冲突留下的 SQLite runtime 禁用 Skill 列与迁移账本。
// POS: schema migration 前置兼容层；只识别已应用旧会话草稿迁移的精确旧库特征。
package migration

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/nexus-research-lab/nexus/internal/storage"
)

const (
	legacyConversationDraftMigrationVersion  = int64(56)
	currentConversationDraftMigrationVersion = int64(57)
)

// RepairLegacyAgentDisabledSkillSchema 修复旧版 00056 会话草稿迁移与当前
// 00056 Agent disabled Skill 迁移共用版本号造成的 SQLite schema 缺口。
func RepairLegacyAgentDisabledSkillSchema(
	ctx context.Context,
	driver string,
	db *sql.DB,
	currentVersion int64,
	logger *slog.Logger,
) error {
	if !storage.IsSQLiteSQLDriver(driver) || currentVersion < legacyConversationDraftMigrationVersion {
		return nil
	}

	hasConversationDrafts, err := sqliteColumnExists(ctx, db, "conversations", "is_draft")
	if err != nil {
		return err
	}
	hasDisabledSkills, err := sqliteColumnExists(ctx, db, "runtimes", "disabled_skill_ids_json")
	if err != nil {
		return err
	}
	if !hasConversationDrafts || hasDisabledSkills {
		return nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin legacy agent disabled skill schema repair: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err = tx.ExecContext(ctx, `
ALTER TABLE runtimes
ADD COLUMN disabled_skill_ids_json TEXT NOT NULL DEFAULT '[]'
`); err != nil {
		return fmt.Errorf("add runtimes.disabled_skill_ids_json: %w", err)
	}
	if currentVersion == legacyConversationDraftMigrationVersion {
		if _, err = tx.ExecContext(ctx, `
INSERT INTO goose_db_version (version_id, is_applied)
VALUES (?, 1)
`, currentConversationDraftMigrationVersion); err != nil {
			return fmt.Errorf("advance legacy conversation draft migration version: %w", err)
		}
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit legacy agent disabled skill schema repair: %w", err)
	}

	logger.Info(
		"已修复旧版 migration 编号冲突",
		"previous_version", currentVersion,
		"schema", "runtimes.disabled_skill_ids_json",
	)
	return nil
}

func sqliteColumnExists(ctx context.Context, db *sql.DB, table string, column string) (bool, error) {
	var count int
	if err := db.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?",
		table,
		column,
	).Scan(&count); err != nil {
		return false, fmt.Errorf("inspect sqlite column %s.%s: %w", table, column, err)
	}
	return count > 0, nil
}
