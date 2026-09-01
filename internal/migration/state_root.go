// INPUT: 桌面宿主注入的新旧 NEXUS_STATE_ROOT、已完成 schema migration 的 SQLite 数据库。
// OUTPUT: 把结构化绝对路径和 transcript 项目索引一次性重映射到新状态根。
// POS: 整体状态根复制后的启动期提交阶段；失败会让桌面宿主回退到旧根。
package migration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode/utf8"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/confinedfs"
	"github.com/nexus-research-lab/nexus/internal/storage"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

type stateRootAgentPath struct {
	agentID       string
	ownerUserID   string
	workspacePath string
}

type sqlitePathColumn struct {
	table string
	name  string
}

// RunDesktopStateRootRebase 提交桌面宿主已经复制完成的整体状态根迁移。
func RunDesktopStateRootRebase(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	previousRoot := filepath.Clean(strings.TrimSpace(os.Getenv(appfs.NexusPreviousStateRootEnvName)))
	if previousRoot == "." || strings.TrimSpace(os.Getenv(appfs.NexusPreviousStateRootEnvName)) == "" {
		return nil
	}
	if !strings.EqualFold(strings.TrimSpace(cfg.AppMode), "desktop") {
		return errors.New("previous state root is only valid in desktop mode")
	}
	if !storage.IsSQLiteSQLDriver(cfg.DatabaseDriver) {
		return errors.New("desktop state root migration requires SQLite")
	}
	currentRoot := filepath.Clean(appfs.StateRoot())
	if err := validateStateRootTransition(previousRoot, currentRoot); err != nil {
		return err
	}
	if logger == nil {
		logger = slog.Default()
	}

	db, err := storage.OpenDB(cfg)
	if err != nil {
		return fmt.Errorf("打开新状态根数据库: %w", err)
	}
	defer db.Close()
	agents, err := readStateRootAgentPaths(ctx, db)
	if err != nil {
		return err
	}
	if err = rebaseTranscriptProjectDirectories(ctx, previousRoot, currentRoot, agents); err != nil {
		return err
	}
	if err = rewriteManagedStateRootPaths(ctx, currentRoot, previousRoot, currentRoot); err != nil {
		return fmt.Errorf("重映射文件状态路径: %w", err)
	}
	if err = workspacestore.NewSessionFileStore(cfg.WorkspacePath).
		RebaseSessionLifecycleRecords(ctx, previousRoot, currentRoot); err != nil {
		return fmt.Errorf("重映射 Session 删除恢复记录: %w", err)
	}
	if err = rebaseSQLitePathColumns(ctx, db, previousRoot, currentRoot); err != nil {
		return fmt.Errorf("重映射数据库路径: %w", err)
	}
	logger.Info(
		"桌面状态根迁移已提交",
		"previous_root", previousRoot,
		"state_root", currentRoot,
		"agent_count", len(agents),
	)
	return nil
}

func validateStateRootTransition(previousRoot string, currentRoot string) error {
	if !filepath.IsAbs(previousRoot) || !filepath.IsAbs(currentRoot) {
		return errors.New("state roots must be absolute paths")
	}
	if sameStateRootPath(previousRoot, currentRoot) {
		return errors.New("previous and current state roots are identical")
	}
	if stateRootPathContains(previousRoot, currentRoot) || stateRootPathContains(currentRoot, previousRoot) {
		return errors.New("state roots must not contain one another")
	}
	return nil
}

func readStateRootAgentPaths(ctx context.Context, db *sql.DB) ([]stateRootAgentPath, error) {
	rows, err := db.QueryContext(ctx, `
SELECT id,
       COALESCE(NULLIF(TRIM(owner_user_id), ''), '__system__'),
       workspace_path
FROM agents
ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("读取 Agent workspace 路径: %w", err)
	}
	defer rows.Close()
	result := make([]stateRootAgentPath, 0)
	for rows.Next() {
		var item stateRootAgentPath
		if err = rows.Scan(&item.agentID, &item.ownerUserID, &item.workspacePath); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func rebaseTranscriptProjectDirectories(
	ctx context.Context,
	previousRoot string,
	currentRoot string,
	agents []stateRootAgentPath,
) error {
	for _, agent := range agents {
		if err := ctx.Err(); err != nil {
			return err
		}
		workspacePath, changed := appfs.RebaseStateRootPath(
			agent.workspacePath,
			previousRoot,
			currentRoot,
		)
		if !changed {
			continue
		}
		projectsRoot := filepath.Join(
			appfs.UserRuntimeRootAt(currentRoot, agent.ownerUserID),
			"projects",
		)
		root, err := confinedfs.Open(projectsRoot)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		targetName := workspacestore.TranscriptProjectDirectoryName(workspacePath)
		for _, sourceName := range workspacestore.TranscriptProjectDirectoryNames(agent.workspacePath) {
			if sourceName == targetName {
				continue
			}
			if err = renameTranscriptProject(root, sourceName, targetName); err != nil {
				root.Close()
				return fmt.Errorf("重映射 Agent %s transcript 项目: %w", agent.agentID, err)
			}
		}
		if err = root.Close(); err != nil {
			return err
		}
	}
	return nil
}

func renameTranscriptProject(root *confinedfs.Root, sourceName string, targetName string) error {
	sourceInfo, err := root.Lstat(sourceName)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !sourceInfo.IsDir() || sourceInfo.Mode()&os.ModeSymlink != 0 {
		return errors.New("transcript 项目不是安全目录")
	}
	if targetInfo, targetErr := root.Lstat(targetName); targetErr == nil {
		if targetInfo.IsDir() && targetInfo.Mode()&os.ModeSymlink == 0 {
			return errors.New("目标 transcript 项目已存在")
		}
		return errors.New("目标 transcript 项目不是安全目录")
	} else if !errors.Is(targetErr, os.ErrNotExist) {
		return targetErr
	}
	return root.Rename(sourceName, targetName)
}

func rebaseSQLitePathColumns(ctx context.Context, db *sql.DB, previousRoot string, currentRoot string) error {
	columns, err := discoverSQLitePathColumns(ctx, db)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, column := range columns {
		if err = rebaseSQLitePathColumn(ctx, tx, column, previousRoot, currentRoot); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func discoverSQLitePathColumns(ctx context.Context, db *sql.DB) ([]sqlitePathColumn, error) {
	rows, err := db.QueryContext(ctx, `
SELECT name
FROM sqlite_master
WHERE type = 'table'
  AND name NOT LIKE 'sqlite_%'
ORDER BY name`)
	if err != nil {
		return nil, err
	}
	var tables []string
	for rows.Next() {
		var table string
		if err = rows.Scan(&table); err != nil {
			rows.Close()
			return nil, err
		}
		tables = append(tables, table)
	}
	if err = rows.Close(); err != nil {
		return nil, err
	}

	result := make([]sqlitePathColumn, 0)
	for _, table := range tables {
		columns, queryErr := db.QueryContext(ctx, "PRAGMA table_info("+quoteSQLiteIdentifier(table)+")")
		if queryErr != nil {
			return nil, queryErr
		}
		for columns.Next() {
			var cid int
			var name, dataType string
			var notNull, primaryKey int
			var defaultValue any
			if err = columns.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
				columns.Close()
				return nil, err
			}
			if isPersistedPathColumn(name) {
				result = append(result, sqlitePathColumn{table: table, name: name})
			}
		}
		if err = columns.Close(); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func isPersistedPathColumn(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	return name == "path" || strings.HasSuffix(name, "_path") ||
		strings.HasSuffix(name, "_dir") || strings.HasSuffix(name, "_directory") ||
		strings.HasSuffix(name, "_file")
}

func rebaseSQLitePathColumn(
	ctx context.Context,
	tx *sql.Tx,
	column sqlitePathColumn,
	previousRoot string,
	currentRoot string,
) error {
	tableName := quoteSQLiteIdentifier(column.table)
	columnName := quoteSQLiteIdentifier(column.name)
	equalExpression := columnName + " = ?"
	prefixExpression := "substr(" + columnName + ", 1, ?) = ?"
	if runtime.GOOS == "windows" {
		equalExpression = "lower(" + columnName + ") = lower(?)"
		prefixExpression = "lower(substr(" + columnName + ", 1, ?)) = lower(?)"
	}
	query := `UPDATE ` + tableName + `
SET ` + columnName + ` = CASE
    WHEN ` + equalExpression + ` THEN ?
    ELSE ? || substr(` + columnName + `, ?)
END
WHERE typeof(` + columnName + `) = 'text'
  AND (
    ` + equalExpression + `
    OR (` + prefixExpression + ` AND substr(` + columnName + `, ?, 1) IN ('/', '\'))
  )`
	// SQLite substr 按 Unicode code point 计数，不能使用 Go 字节长度。
	rootLength := utf8.RuneCountInString(previousRoot)
	_, err := tx.ExecContext(
		ctx,
		query,
		previousRoot,
		currentRoot,
		currentRoot,
		rootLength+1,
		previousRoot,
		rootLength,
		previousRoot,
		rootLength+1,
	)
	if err != nil {
		return fmt.Errorf("更新 %s.%s: %w", column.table, column.name, err)
	}
	return nil
}

func quoteSQLiteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func sameStateRootPath(left string, right string) bool {
	left = filepath.Clean(strings.TrimSpace(left))
	right = filepath.Clean(strings.TrimSpace(right))
	if runtime.GOOS == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func stateRootPathContains(root string, path string) bool {
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil || relative == "." || relative == ".." {
		return false
	}
	return !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}
