// INPUT: v0.1.27/v0.1.28 旧状态根，以及 v0.1.30 起错误启动后生成的 canonical 数据。
// OUTPUT: 先恢复旧布局真相，再把错误窗口内新增且不冲突的数据库与用户文件安全并回。
// POS: v0.1.30 根目录迁移缺口补偿层；所有冲突先隔离备份，绝不覆盖任一版本。
package migration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

const (
	skippedStateLayoutRecoveryDirectory = "skipped-state-layout-v1"
	skippedStateLayoutMergeMarker       = "20260804_merge_skipped_state_layout_database"
	skippedStateLayoutUsersMergeMarker  = "20260804_merge_skipped_state_layout_users"
	skippedStateLayoutDatabaseName      = "nexus.db"
	skippedStateLayoutDatabaseAlias     = "skipped_layout"
)

type sqliteMergeColumn struct {
	name         string
	notNull      bool
	defaultValue sql.NullString
	primaryKey   bool
}

// prepareSkippedStateLayoutRecovery 处理旧 data 与新 app/data 同时存在的发布迁移缺口。
//
// 旧根没有完成标记，说明旧数据库从未被迁移；canonical 数据库则是缺失迁移时
// 误创建的新分支。桌面启动期没有并发 writer，因此先整体隔离 v0.1.30 起的
// 新分支，再让旧迁移按原顺序接管 canonical 路径。隔离副本稍后用于安全合并。
func prepareSkippedStateLayoutRecovery(stateRoot string, logger *slog.Logger) error {
	legacyDatabase := filepath.Join(stateRoot, "data", skippedStateLayoutDatabaseName)
	canonicalDataRoot := filepath.Join(stateRoot, "app", "data")
	canonicalDatabase := filepath.Join(canonicalDataRoot, skippedStateLayoutDatabaseName)
	legacyExists, err := isRegularLayoutFile(legacyDatabase)
	if err != nil || !legacyExists {
		return err
	}
	canonicalExists, err := isRegularLayoutFile(canonicalDatabase)
	if err != nil || !canonicalExists {
		return err
	}
	identical, err := sameLayoutFile(legacyDatabase, canonicalDatabase)
	if err != nil || identical {
		return err
	}
	if !strings.EqualFold(strings.TrimSpace(os.Getenv("NEXUS_APP_MODE")), "desktop") {
		return fmt.Errorf(
			"检测到未迁移旧数据库 %q 与 canonical 数据库 %q 同时存在；非桌面部署拒绝自动选择数据真相",
			legacyDatabase,
			canonicalDatabase,
		)
	}
	canonicalDataInfo, err := os.Lstat(canonicalDataRoot)
	if err != nil {
		return fmt.Errorf("检查 canonical 数据根 %q: %w", canonicalDataRoot, err)
	}
	if canonicalDataInfo.Mode()&os.ModeSymlink != 0 || !canonicalDataInfo.IsDir() {
		return fmt.Errorf("canonical 数据根不是安全目录: %q", canonicalDataRoot)
	}

	recoveryUsersRoot := skippedStateLayoutRecoveryUsersRoot(stateRoot)
	canonicalUsersRoot := filepath.Join(stateRoot, "users")
	canonicalUsersInfo, usersErr := os.Lstat(canonicalUsersRoot)
	canonicalUsersExist := false
	switch {
	case usersErr == nil:
		if canonicalUsersInfo.Mode()&os.ModeSymlink != 0 || !canonicalUsersInfo.IsDir() {
			return fmt.Errorf("canonical 用户根不是安全目录: %q", canonicalUsersRoot)
		}
		canonicalUsersExist = true
		if _, statErr := os.Lstat(recoveryUsersRoot); statErr == nil {
			return fmt.Errorf("迁移缺口用户隔离目录已存在，拒绝覆盖: %q", recoveryUsersRoot)
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return fmt.Errorf("检查迁移缺口用户隔离目录 %q: %w", recoveryUsersRoot, statErr)
		}
	case !errors.Is(usersErr, os.ErrNotExist):
		return fmt.Errorf("检查 canonical 用户根 %q: %w", canonicalUsersRoot, usersErr)
	}

	recoveryDataRoot := skippedStateLayoutRecoveryDataRoot(stateRoot)
	if _, statErr := os.Lstat(recoveryDataRoot); statErr == nil {
		return fmt.Errorf("迁移缺口隔离目录已存在，拒绝覆盖: %q", recoveryDataRoot)
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return fmt.Errorf("检查迁移缺口隔离目录 %q: %w", recoveryDataRoot, statErr)
	}
	if err = os.MkdirAll(filepath.Dir(recoveryDataRoot), 0o700); err != nil {
		return fmt.Errorf("创建迁移缺口隔离根: %w", err)
	}
	usersStaged := false
	if canonicalUsersExist {
		if err = os.Rename(canonicalUsersRoot, recoveryUsersRoot); err != nil {
			return fmt.Errorf("隔离错误启动生成的 canonical 用户数据 %q: %w", canonicalUsersRoot, err)
		}
		usersStaged = true
	}
	if err = os.Rename(canonicalDataRoot, recoveryDataRoot); err != nil {
		if usersStaged {
			if rollbackErr := os.Rename(recoveryUsersRoot, canonicalUsersRoot); rollbackErr != nil {
				return fmt.Errorf(
					"隔离错误启动生成的 canonical 数据 %q: %w；回滚用户目录失败: %v",
					canonicalDataRoot,
					err,
					rollbackErr,
				)
			}
		}
		return fmt.Errorf("隔离错误启动生成的 canonical 数据 %q: %w", canonicalDataRoot, err)
	}
	logger.Warn(
		"检测到 v0.1.30 根目录迁移缺口，已隔离新数据并准备恢复旧布局",
		"legacy_database", legacyDatabase,
		"recovery_database", filepath.Join(recoveryDataRoot, skippedStateLayoutDatabaseName),
		"recovery_users", recoveryUsersRoot,
	)
	return nil
}

func isRegularLayoutFile(path string) (bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("检查布局文件 %q: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return false, fmt.Errorf("布局数据库不是安全普通文件: %q", path)
	}
	return true, nil
}

func skippedStateLayoutRecoveryDataRoot(stateRoot string) string {
	return filepath.Join(
		filepath.Clean(stateRoot),
		"app",
		".migration-quarantine",
		skippedStateLayoutRecoveryDirectory,
		"canonical-data",
	)
}

func skippedStateLayoutRecoveryUsersRoot(stateRoot string) string {
	return filepath.Join(
		filepath.Clean(stateRoot),
		"app",
		".migration-quarantine",
		skippedStateLayoutRecoveryDirectory,
		"canonical-users",
	)
}

type recoveredLayoutMergeResult struct {
	moved     int
	identical int
	merged    int
	conflicts int
}

// MergeSkippedStateLayoutUsers 把 v0.1.30 起错误窗口内生成的 owner 文件并回旧布局。
//
// 不冲突路径直接进入 canonical users；相同文件去重；可证明可合并的 Room overlay
// 追加去重。其余冲突继续留在隔离目录，旧文件始终保持运行时优先级。
func MergeSkippedStateLayoutUsers(stateRoot string, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.Default()
	}
	stateRoot = filepath.Clean(stateRoot)
	markerPath := workspaceFileMigrationMarker(filepath.Join(stateRoot, "app"), skippedStateLayoutUsersMergeMarker)
	applied, err := workspaceFileMigrationApplied(markerPath)
	if err != nil || applied {
		return err
	}
	recoveryUsersRoot := skippedStateLayoutRecoveryUsersRoot(stateRoot)
	info, err := os.Lstat(recoveryUsersRoot)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("检查迁移缺口用户隔离根 %q: %w", recoveryUsersRoot, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("迁移缺口用户隔离根不是安全目录: %q", recoveryUsersRoot)
	}
	canonicalUsersRoot := filepath.Join(stateRoot, "users")
	if err = os.MkdirAll(canonicalUsersRoot, 0o700); err != nil {
		return fmt.Errorf("创建 canonical 用户根: %w", err)
	}
	result := recoveredLayoutMergeResult{}
	if err = mergeRecoveredLayoutDirectory(recoveryUsersRoot, canonicalUsersRoot, &result); err != nil {
		return err
	}
	if shouldHardenMigratedPermissions(appfs.RuntimeIsolationEnforced()) {
		if err = hardenLayoutTree(canonicalUsersRoot); err != nil {
			return fmt.Errorf("收紧恢复后的用户状态权限: %w", err)
		}
	}
	if err = writeWorkspaceFileMigrationMarker(markerPath); err != nil {
		return err
	}
	logger.Info(
		"状态布局迁移缺口的用户文件恢复完成",
		"migration", skippedStateLayoutUsersMergeMarker,
		"moved", result.moved,
		"identical", result.identical,
		"merged", result.merged,
		"conflicts_preserved", result.conflicts,
		"recovery_users", recoveryUsersRoot,
	)
	return nil
}

func mergeRecoveredLayoutDirectory(
	sourceRoot string,
	targetRoot string,
	result *recoveredLayoutMergeResult,
) error {
	entries, err := os.ReadDir(sourceRoot)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("读取迁移缺口隔离目录 %q: %w", sourceRoot, err)
	}
	for _, entry := range entries {
		if err = mergeRecoveredLayoutEntry(
			filepath.Join(sourceRoot, entry.Name()),
			filepath.Join(targetRoot, entry.Name()),
			result,
		); err != nil {
			return err
		}
	}
	if remaining, readErr := os.ReadDir(sourceRoot); readErr == nil && len(remaining) == 0 {
		if removeErr := os.Remove(sourceRoot); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return removeErr
		}
	}
	return nil
}

func mergeRecoveredLayoutEntry(
	sourcePath string,
	targetPath string,
	result *recoveredLayoutMergeResult,
) error {
	sourceInfo, err := os.Lstat(sourcePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	targetInfo, targetErr := os.Lstat(targetPath)
	if errors.Is(targetErr, os.ErrNotExist) {
		if sourceInfo.Mode()&os.ModeType != 0 && sourceInfo.Mode()&os.ModeSymlink == 0 && !sourceInfo.IsDir() {
			result.conflicts++
			return nil
		}
		if err = os.MkdirAll(filepath.Dir(targetPath), 0o700); err != nil {
			return err
		}
		if err = os.Rename(sourcePath, targetPath); err != nil {
			return err
		}
		result.moved++
		return nil
	}
	if targetErr != nil {
		return targetErr
	}
	if sourceInfo.IsDir() && sourceInfo.Mode()&os.ModeSymlink == 0 &&
		targetInfo.IsDir() && targetInfo.Mode()&os.ModeSymlink == 0 {
		return mergeRecoveredLayoutDirectory(sourcePath, targetPath, result)
	}
	if isRoomOverlayPath(sourcePath) && sourceInfo.Mode().IsRegular() && targetInfo.Mode().IsRegular() {
		handled, _, mergeErr := mergeRoomOverlayFiles(sourcePath, targetPath)
		if mergeErr != nil {
			return mergeErr
		}
		if handled {
			result.merged++
			return nil
		}
	}
	if sourceInfo.Mode().IsRegular() && targetInfo.Mode().IsRegular() {
		identical, compareErr := sameLayoutFile(sourcePath, targetPath)
		if compareErr != nil {
			return compareErr
		}
		if identical {
			if err = os.Remove(sourcePath); err != nil {
				return err
			}
			result.identical++
			return nil
		}
	}
	if sourceInfo.Mode()&os.ModeSymlink != 0 && targetInfo.Mode()&os.ModeSymlink != 0 {
		sourceLink, sourceErr := os.Readlink(sourcePath)
		targetLink, targetErr := os.Readlink(targetPath)
		if sourceErr == nil && targetErr == nil && sourceLink == targetLink {
			if err = os.Remove(sourcePath); err != nil {
				return err
			}
			result.identical++
			return nil
		}
	}
	result.conflicts++
	return nil
}

// MergeSkippedStateLayoutDatabase 合并 v0.1.30 起错误窗口内写入的新 canonical 数据库。
//
// 已恢复的旧库保持冲突优先级；新库只补入不冲突记录。整个合并在单事务中完成，
// 并在提交前执行 foreign_key_check。任何不确定冲突都会回滚，隔离副本保持原样。
func MergeSkippedStateLayoutDatabase(
	ctx context.Context,
	cfg config.Config,
	logger *slog.Logger,
) error {
	if logger == nil {
		logger = slog.Default()
	}
	stateRoot := appfs.StateRoot()
	markerPath := workspaceFileMigrationMarker(filepath.Join(stateRoot, "app"), skippedStateLayoutMergeMarker)
	applied, err := workspaceFileMigrationApplied(markerPath)
	if err != nil || applied {
		return err
	}
	recoveryDatabase := filepath.Join(
		skippedStateLayoutRecoveryDataRoot(stateRoot),
		skippedStateLayoutDatabaseName,
	)
	exists, err := isRegularLayoutFile(recoveryDatabase)
	if err != nil || !exists {
		return err
	}
	if !storage.IsSQLiteSQLDriver(cfg.DatabaseDriver) {
		return errors.New("状态布局迁移缺口的数据库恢复仅支持 SQLite")
	}

	db, err := storage.OpenMigrationDB(cfg)
	if err != nil {
		return fmt.Errorf("打开迁移缺口恢复目标数据库: %w", err)
	}
	defer db.Close()
	mergedRows, err := mergeSQLiteDatabase(ctx, db, recoveryDatabase)
	if err != nil {
		return fmt.Errorf("合并迁移缺口隔离数据库: %w", err)
	}
	if err = writeWorkspaceFileMigrationMarker(markerPath); err != nil {
		return err
	}
	logger.Info(
		"状态布局迁移缺口的数据库恢复完成",
		"migration", skippedStateLayoutMergeMarker,
		"merged_rows", mergedRows,
		"recovery_database", recoveryDatabase,
	)
	return nil
}

func mergeSQLiteDatabase(ctx context.Context, db *sql.DB, sourcePath string) (int64, error) {
	connection, err := db.Conn(ctx)
	if err != nil {
		return 0, err
	}
	defer connection.Close()
	if _, err = connection.ExecContext(
		ctx,
		"ATTACH DATABASE ? AS "+quoteSQLiteIdentifier(skippedStateLayoutDatabaseAlias),
		sourcePath,
	); err != nil {
		return 0, err
	}
	attached := true
	defer func() {
		if attached {
			_, _ = connection.ExecContext(
				context.Background(),
				"DETACH DATABASE "+quoteSQLiteIdentifier(skippedStateLayoutDatabaseAlias),
			)
		}
	}()

	if err = validateRecoverySQLiteDatabase(ctx, connection); err != nil {
		return 0, err
	}
	tables, err := recoverySQLiteTables(ctx, connection)
	if err != nil {
		return 0, err
	}
	tx, err := connection.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var mergedRows int64
	for _, table := range tables {
		rows, mergeErr := mergeSQLiteTable(ctx, tx, table)
		if mergeErr != nil {
			return 0, mergeErr
		}
		mergedRows += rows
	}
	if err = ensureSQLiteForeignKeysValid(ctx, tx); err != nil {
		return 0, err
	}
	if err = tx.Commit(); err != nil {
		return 0, err
	}
	if _, err = connection.ExecContext(
		ctx,
		"DETACH DATABASE "+quoteSQLiteIdentifier(skippedStateLayoutDatabaseAlias),
	); err != nil {
		return 0, err
	}
	attached = false
	return mergedRows, nil
}

func validateRecoverySQLiteDatabase(ctx context.Context, connection *sql.Conn) error {
	var result string
	if err := connection.QueryRowContext(
		ctx,
		"PRAGMA "+quoteSQLiteIdentifier(skippedStateLayoutDatabaseAlias)+".quick_check",
	).Scan(&result); err != nil {
		return err
	}
	if result != "ok" {
		return fmt.Errorf("隔离数据库 quick_check 失败: %s", result)
	}
	for _, table := range []string{"agents", "rooms", "conversations"} {
		var count int
		if err := connection.QueryRowContext(
			ctx,
			"SELECT COUNT(*) FROM "+quoteSQLiteIdentifier(skippedStateLayoutDatabaseAlias)+".sqlite_master WHERE type = 'table' AND name = ?",
			table,
		).Scan(&count); err != nil {
			return err
		}
		if count != 1 {
			return fmt.Errorf("隔离数据库缺少 Nexus 表 %s", table)
		}
	}
	return nil
}

func recoverySQLiteTables(ctx context.Context, connection *sql.Conn) ([]string, error) {
	rows, err := connection.QueryContext(ctx, `
SELECT source.name
FROM `+quoteSQLiteIdentifier(skippedStateLayoutDatabaseAlias)+`.sqlite_master source
JOIN main.sqlite_master target ON target.type = 'table' AND target.name = source.name
WHERE source.type = 'table'
  AND source.name NOT LIKE 'sqlite_%'
  AND source.name NOT IN (
      'goose_db_version',
      'alembic_version',
      'automation_scheduler_leases'
  )
ORDER BY source.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tables []string
	for rows.Next() {
		var table string
		if err = rows.Scan(&table); err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	sort.Strings(tables)
	return tables, nil
}

func mergeSQLiteTable(ctx context.Context, tx *sql.Tx, table string) (int64, error) {
	targetColumns, err := sqliteTableColumns(ctx, tx, "main", table)
	if err != nil {
		return 0, err
	}
	sourceColumns, err := sqliteTableColumns(ctx, tx, skippedStateLayoutDatabaseAlias, table)
	if err != nil {
		return 0, err
	}
	sourceNames := make(map[string]struct{}, len(sourceColumns))
	for _, column := range sourceColumns {
		sourceNames[column.name] = struct{}{}
	}
	columns := make([]string, 0, len(targetColumns))
	for _, column := range targetColumns {
		if _, ok := sourceNames[column.name]; ok {
			columns = append(columns, column.name)
			continue
		}
		if column.notNull && !column.defaultValue.Valid && !column.primaryKey {
			return 0, fmt.Errorf("表 %s 的必需列 %s 不存在于隔离数据库", table, column.name)
		}
	}
	if len(columns) == 0 {
		return 0, nil
	}
	quoted := make([]string, 0, len(columns))
	for _, column := range columns {
		quoted = append(quoted, quoteSQLiteIdentifier(column))
	}
	columnList := strings.Join(quoted, ", ")
	query := "INSERT OR IGNORE INTO main." + quoteSQLiteIdentifier(table) +
		" (" + columnList + ") SELECT " + columnList + " FROM " +
		quoteSQLiteIdentifier(skippedStateLayoutDatabaseAlias) + "." + quoteSQLiteIdentifier(table)
	result, err := tx.ExecContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("合并表 %s: %w", table, err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return rows, nil
}

func sqliteTableColumns(ctx context.Context, tx *sql.Tx, schema string, table string) ([]sqliteMergeColumn, error) {
	query := "PRAGMA " + quoteSQLiteIdentifier(schema) + ".table_info(" + quoteSQLiteIdentifier(table) + ")"
	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var columns []sqliteMergeColumn
	for rows.Next() {
		var (
			columnID   int
			columnType string
			notNull    int
			primaryKey int
			column     sqliteMergeColumn
		)
		if err = rows.Scan(
			&columnID,
			&column.name,
			&columnType,
			&notNull,
			&column.defaultValue,
			&primaryKey,
		); err != nil {
			return nil, err
		}
		column.notNull = notNull != 0
		column.primaryKey = primaryKey != 0
		columns = append(columns, column)
	}
	return columns, rows.Err()
}

func ensureSQLiteForeignKeysValid(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return err
	}
	defer rows.Close()
	if !rows.Next() {
		return rows.Err()
	}
	var table, parent string
	var rowID any
	var foreignKeyID int
	if err = rows.Scan(&table, &rowID, &parent, &foreignKeyID); err != nil {
		return err
	}
	return fmt.Errorf(
		"合并后外键不完整: table=%s rowid=%v parent=%s foreign_key=%d",
		table,
		rowID,
		parent,
		foreignKeyID,
	)
}
