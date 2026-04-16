// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：legacy_bootstrap.go
// @Date   ：2026/04/16 13:41:14
// @Author ：leemysw
// 2026/04/16 13:41:14   Create
// =====================================================

package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	goosedb "github.com/pressly/goose/v3/database"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type sqlExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

// bootstrapLegacyMigrationVersion 会在 Goose 真正执行前把 Python 时代的最终库结构
// 修补到当前 Go 运行时需要的最小基线，然后回写单版本 Goose 历史。
func bootstrapLegacyMigrationVersion(
	ctx context.Context,
	db *sql.DB,
	cfg config.Config,
	migrationDir string,
) error {
	driver := protocol.NormalizeSQLDriver(cfg.DatabaseDriver)
	hasAgents, err := tableExists(ctx, db, driver, "agents")
	if err != nil {
		return err
	}
	if !hasAgents {
		return nil
	}

	baselineVersion, latestVersion, err := migrationVersionRange(migrationDir)
	if err != nil {
		return err
	}
	appliedVersions, err := migrationVersionsUpTo(migrationDir, latestVersion)
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	currentVersion, hasVersionTable, err := currentGooseVersionTx(ctx, tx, driver)
	if err != nil {
		return err
	}
	targetVersion, targetErr := detectSchemaVersion(ctx, tx, driver, baselineVersion, latestVersion)
	if targetErr != nil {
		return targetErr
	}
	expectedVersions := filterAppliedVersions(appliedVersions, targetVersion)

	needReset := !hasVersionTable || currentVersion <= 0 || currentVersion > latestVersion || currentVersion != targetVersion
	if !needReset && hasVersionTable {
		matched, matchErr := versionTableMatches(ctx, tx, cfg.DatabaseDriver, expectedVersions)
		if matchErr != nil {
			return matchErr
		}
		needReset = !matched
	}
	if needReset {
		if err = resetVersionTable(
			ctx,
			tx,
			cfg.DatabaseDriver,
			expectedVersions,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func detectSchemaVersion(
	ctx context.Context,
	executor sqlExecutor,
	driver string,
	baselineVersion int64,
	latestVersion int64,
) (int64, error) {
	currentGo, err := isCurrentGoSchema(ctx, executor, driver)
	if err != nil {
		return 0, err
	}
	if currentGo {
		return latestVersion, nil
	}

	pythonFinal, err := isPythonFinalSchema(ctx, executor, driver)
	if err != nil {
		return 0, err
	}
	if pythonFinal {
		return baselineVersion, nil
	}
	return 0, fmt.Errorf("无法识别当前数据库结构，无法自动推断 Goose 版本")
}

func isCurrentGoSchema(ctx context.Context, executor sqlExecutor, driver string) (bool, error) {
	hasUsers, err := tableExists(ctx, executor, driver, "users")
	if err != nil || !hasUsers {
		return false, err
	}
	hasPasswordCredentials, err := tableExists(ctx, executor, driver, "auth_password_credentials")
	if err != nil || !hasPasswordCredentials {
		return false, err
	}
	hasProvider, err := tableExists(ctx, executor, driver, "provider")
	if err != nil || !hasProvider {
		return false, err
	}
	return authSessionsHasNewShape(ctx, executor, driver)
}

func isPythonFinalSchema(ctx context.Context, executor sqlExecutor, driver string) (bool, error) {
	hasLegacyProvider, err := tableExists(ctx, executor, driver, "provider")
	if err != nil || !hasLegacyProvider {
		return false, err
	}
	hasUsers, err := tableExists(ctx, executor, driver, "users")
	if err != nil {
		return false, err
	}
	if hasUsers {
		return false, nil
	}
	hasNewAuthShape, err := authSessionsHasNewShape(ctx, executor, driver)
	if err != nil {
		return false, err
	}
	if hasNewAuthShape {
		return false, nil
	}
	hasProviderColumn, err := columnExists(ctx, executor, driver, "runtimes", "provider")
	if err != nil {
		return false, err
	}
	hasModelColumn, err := columnExists(ctx, executor, driver, "runtimes", "model")
	if err != nil {
		return false, err
	}
	return hasProviderColumn && !hasModelColumn, nil
}

func authSessionsHasNewShape(ctx context.Context, executor sqlExecutor, driver string) (bool, error) {
	requiredColumns := []string{
		"session_id",
		"user_id",
		"session_token_hash",
		"auth_method",
		"last_seen_at",
		"client_ip",
		"user_agent",
		"revoked_at",
	}
	for _, columnName := range requiredColumns {
		hasColumn, err := columnExists(ctx, executor, driver, "auth_sessions", columnName)
		if err != nil {
			return false, err
		}
		if !hasColumn {
			return false, nil
		}
	}
	return true, nil
}

func migrationVersionsUpTo(migrationDir string, targetVersion int64) ([]int64, error) {
	pattern := filepath.Join(migrationDir, "*.sql")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("migration 目录为空: %s", migrationDir)
	}

	seen := make(map[int64]struct{}, len(files))
	versions := make([]int64, 0, len(files))
	for _, filePath := range files {
		baseName := filepath.Base(filePath)
		underscoreIndex := strings.IndexByte(baseName, '_')
		if underscoreIndex <= 0 {
			continue
		}
		version, parseErr := strconv.ParseInt(baseName[:underscoreIndex], 10, 64)
		if parseErr != nil {
			return nil, fmt.Errorf("解析 migration 版本失败: %s", baseName)
		}
		if version > targetVersion {
			continue
		}
		if _, ok := seen[version]; ok {
			continue
		}
		seen[version] = struct{}{}
		versions = append(versions, version)
	}
	sort.Slice(versions, func(left, right int) bool {
		return versions[left] < versions[right]
	})
	return versions, nil
}

func filterAppliedVersions(versions []int64, targetVersion int64) []int64 {
	result := make([]int64, 0, len(versions))
	for _, version := range versions {
		if version <= targetVersion {
			result = append(result, version)
		}
	}
	return result
}

func migrationVersionRange(migrationDir string) (int64, int64, error) {
	pattern := filepath.Join(migrationDir, "*.sql")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return 0, 0, err
	}
	if len(files) == 0 {
		return 0, 0, fmt.Errorf("migration 目录为空: %s", migrationDir)
	}

	versions := make([]int64, 0, len(files))
	for _, filePath := range files {
		baseName := filepath.Base(filePath)
		underscoreIndex := strings.IndexByte(baseName, '_')
		if underscoreIndex <= 0 {
			continue
		}
		version, parseErr := strconv.ParseInt(baseName[:underscoreIndex], 10, 64)
		if parseErr != nil {
			return 0, 0, fmt.Errorf("解析 migration 版本失败: %s", baseName)
		}
		versions = append(versions, version)
	}
	if len(versions) == 0 {
		return 0, 0, fmt.Errorf("未在 migration 目录中找到有效版本: %s", migrationDir)
	}

	sort.Slice(versions, func(left, right int) bool {
		return versions[left] < versions[right]
	})
	return versions[0], versions[len(versions)-1], nil
}

func currentGooseVersion(ctx context.Context, db *sql.DB, databaseDriver string) (int64, error) {
	version, _, err := currentGooseVersionTx(ctx, db, protocol.NormalizeSQLDriver(databaseDriver))
	return version, err
}

func currentGooseVersionTx(
	ctx context.Context,
	executor sqlExecutor,
	driver string,
) (int64, bool, error) {
	hasVersionTable, err := tableExists(ctx, executor, driver, "goose_db_version")
	if err != nil {
		return 0, false, err
	}
	if !hasVersionTable {
		return 0, false, nil
	}

	row := executor.QueryRowContext(ctx, `SELECT version_id FROM goose_db_version ORDER BY id DESC LIMIT 1`)
	var version sql.NullInt64
	if err = row.Scan(&version); err != nil {
		if err == sql.ErrNoRows {
			return 0, true, nil
		}
		return 0, true, err
	}
	if !version.Valid {
		return 0, true, nil
	}
	return version.Int64, true, nil
}

func versionTableMatches(
	ctx context.Context,
	tx *sql.Tx,
	databaseDriver string,
	expectedVersions []int64,
) (bool, error) {
	store, err := goosedb.NewStore(gooseDialect(databaseDriver), "goose_db_version")
	if err != nil {
		return false, err
	}
	items, err := store.ListMigrations(ctx, tx)
	if err != nil {
		return false, err
	}
	actualVersions := make([]int64, 0, len(items))
	for _, item := range items {
		if item == nil || !item.IsApplied || item.Version <= 0 {
			continue
		}
		actualVersions = append(actualVersions, item.Version)
	}
	sort.Slice(actualVersions, func(left, right int) bool {
		return actualVersions[left] < actualVersions[right]
	})
	if len(actualVersions) != len(expectedVersions) {
		return false, nil
	}
	for index := range actualVersions {
		if actualVersions[index] != expectedVersions[index] {
			return false, nil
		}
	}
	return true, nil
}

func resetVersionTable(
	ctx context.Context,
	tx *sql.Tx,
	databaseDriver string,
	versions []int64,
) error {
	if len(versions) == 0 {
		return fmt.Errorf("version 不能为空")
	}
	store, err := goosedb.NewStore(gooseDialect(databaseDriver), "goose_db_version")
	if err != nil {
		return err
	}
	driver := protocol.NormalizeSQLDriver(databaseDriver)
	hasVersionTable, err := tableExists(ctx, tx, driver, "goose_db_version")
	if err != nil {
		return err
	}
	if !hasVersionTable {
		if err = store.CreateVersionTable(ctx, tx); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM goose_db_version`); err != nil {
		return err
	}
	for _, version := range versions {
		if err = store.Insert(ctx, tx, goosedb.InsertRequest{Version: version}); err != nil {
			return err
		}
	}
	return nil
}

func tableExists(ctx context.Context, executor sqlExecutor, driver string, tableName string) (bool, error) {
	query := `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`
	args := []any{tableName}
	if driver == "pgx" {
		query = `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1`
	}
	var count int
	if err := executor.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func columnExists(ctx context.Context, executor sqlExecutor, driver string, tableName string, columnName string) (bool, error) {
	query := fmt.Sprintf(
		"SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name = ?",
		strings.ReplaceAll(tableName, "'", "''"),
	)
	args := []any{columnName}
	if driver == "pgx" {
		query = `SELECT COUNT(*) FROM information_schema.columns WHERE table_schema = 'public' AND table_name = $1 AND column_name = $2`
		args = []any{tableName, columnName}
	}
	var count int
	if err := executor.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func gooseDialect(databaseDriver string) goosedb.Dialect {
	switch protocol.NormalizeSQLDriver(databaseDriver) {
	case "pgx":
		return goosedb.DialectPostgres
	default:
		return goosedb.DialectSQLite3
	}
}

func migrationDirFromDriver(databaseDriver string) string {
	switch protocol.NormalizeSQLDriver(databaseDriver) {
	case "pgx":
		return filepath.Join("db", "migrations", "postgres")
	default:
		return filepath.Join("db", "migrations", "sqlite")
	}
}

func resolveMigrationDir(databaseDriver string) string {
	relativeDir := migrationDirFromDriver(databaseDriver)
	candidates := []string{
		relativeDir,
		filepath.Join("..", relativeDir),
		filepath.Join("..", "..", relativeDir),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	return relativeDir
}
