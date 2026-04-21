// # !/usr/bin/env go
// -*- coding: utf-8 -*-
// =====================================================
// @File   ：legacy_bootstrap_test.go
// @Date   ：2026/04/16 13:01
// @Author ：leemysw
// 2026/04/16 13:01   Create
// =====================================================

package main

import (
	"context"
	"database/sql"
	"path/filepath"
	"sort"
	"testing"

	"github.com/pressly/goose/v3"
	goosedb "github.com/pressly/goose/v3/database"

	"github.com/nexus-research-lab/nexus/internal/config"

	_ "github.com/mattn/go-sqlite3"
)

func TestBootstrapLegacyMigrationVersionStampsBaselineForCurrentSchema(t *testing.T) {
	t.Helper()

	db := openSQLiteTestDB(t, "baseline-current.db")
	defer db.Close()

	if err := seedCurrentSQLiteBaselineSchema(t, db); err != nil {
		t.Fatalf("准备最新 sqlite schema 失败: %v", err)
	}

	cfg := config.Config{DatabaseDriver: "sqlite"}
	migrationDir := filepath.Join("..", "..", migrationDirFromDriver("sqlite"))
	if err := bootstrapLegacyMigrationVersion(context.Background(), db, cfg, migrationDir); err != nil {
		t.Fatalf("bootstrap baseline version 失败: %v", err)
	}

	assertCurrentVersion(t, db, "sqlite", 3)
}

func TestBootstrapLegacyMigrationVersionRepairsLatestPythonSchema(t *testing.T) {
	t.Helper()

	db := openSQLiteTestDB(t, "python-final.db")
	defer db.Close()

	if err := seedLatestPythonSQLiteSchema(t, db); err != nil {
		t.Fatalf("准备 Python 最终 sqlite schema 失败: %v", err)
	}

	cfg := config.Config{DatabaseDriver: "sqlite"}
	migrationDir := filepath.Join("..", "..", migrationDirFromDriver("sqlite"))
	if err := bootstrapLegacyMigrationVersion(context.Background(), db, cfg, migrationDir); err != nil {
		t.Fatalf("bootstrap Python 最终 schema 失败: %v", err)
	}

	assertCurrentVersion(t, db, "sqlite", 1)
	assertSQLiteColumnNotExists(t, db, "runtimes", "model")
	assertSQLiteTableExists(t, db, "provider")
	assertSQLiteTableNotExists(t, db, "users")
	assertSQLiteTableNotExists(t, db, "auth_password_credentials")
	assertSQLiteColumnExists(t, db, "auth_sessions", "id")
	assertSQLiteColumnNotExists(t, db, "auth_sessions", "session_id")

	if err := goose.Up(db, migrationDir); err != nil {
		t.Fatalf("执行 Go 适配迁移失败: %v", err)
	}
	assertCurrentVersion(t, db, "sqlite", 3)
	assertSQLiteTableExists(t, db, "provider")
	assertSQLiteTableExists(t, db, "users")
	assertSQLiteTableExists(t, db, "auth_password_credentials")
	assertSQLiteColumnExists(t, db, "auth_sessions", "session_id")
	assertSQLiteColumnExists(t, db, "auth_sessions", "user_id")
	assertSQLiteColumnExists(t, db, "agents", "owner_user_id")
	assertSQLiteColumnExists(t, db, "agents", "is_main")
	assertSQLiteColumnExists(t, db, "rooms", "owner_user_id")
}

func TestBootstrapLegacyMigrationVersionRepairsZeroVersionTable(t *testing.T) {
	t.Helper()

	db := openSQLiteTestDB(t, "baseline-zero.db")
	defer db.Close()

	if err := seedCurrentSQLiteBaselineSchema(t, db); err != nil {
		t.Fatalf("准备最新 sqlite schema 失败: %v", err)
	}
	if err := seedGooseVersions(context.Background(), db, "sqlite", 0); err != nil {
		t.Fatalf("准备 goose 零版本失败: %v", err)
	}

	cfg := config.Config{DatabaseDriver: "sqlite"}
	migrationDir := filepath.Join("..", "..", migrationDirFromDriver("sqlite"))
	if err := bootstrapLegacyMigrationVersion(context.Background(), db, cfg, migrationDir); err != nil {
		t.Fatalf("bootstrap baseline version after zero row 失败: %v", err)
	}

	assertCurrentVersion(t, db, "sqlite", 3)
}

func TestBootstrapLegacyMigrationVersionNormalizesCollapsedVersions(t *testing.T) {
	t.Helper()

	db := openSQLiteTestDB(t, "baseline-collapsed.db")
	defer db.Close()

	if err := seedCurrentSQLiteBaselineSchema(t, db); err != nil {
		t.Fatalf("准备最新 sqlite schema 失败: %v", err)
	}
	if err := seedGooseVersions(context.Background(), db, "sqlite", 0, 1, 2, 3, 4); err != nil {
		t.Fatalf("准备 goose 历史版本失败: %v", err)
	}

	cfg := config.Config{DatabaseDriver: "sqlite"}
	migrationDir := filepath.Join("..", "..", migrationDirFromDriver("sqlite"))
	if err := bootstrapLegacyMigrationVersion(context.Background(), db, cfg, migrationDir); err != nil {
		t.Fatalf("bootstrap baseline version after collapsed versions 失败: %v", err)
	}

	assertCurrentVersion(t, db, "sqlite", 3)
}

func TestBootstrapLegacyMigrationVersionRepairsSingletonLatestVersion(t *testing.T) {
	t.Helper()

	db := openSQLiteTestDB(t, "baseline-singleton-latest.db")
	defer db.Close()

	if err := seedCurrentSQLiteBaselineSchema(t, db); err != nil {
		t.Fatalf("准备最新 sqlite schema 失败: %v", err)
	}
	if err := seedGooseVersions(context.Background(), db, "sqlite", 3); err != nil {
		t.Fatalf("准备 goose 单版本历史失败: %v", err)
	}

	cfg := config.Config{DatabaseDriver: "sqlite"}
	migrationDir := filepath.Join("..", "..", migrationDirFromDriver("sqlite"))
	if err := bootstrapLegacyMigrationVersion(context.Background(), db, cfg, migrationDir); err != nil {
		t.Fatalf("bootstrap singleton latest version 失败: %v", err)
	}

	assertCurrentVersion(t, db, "sqlite", 3)
	assertAppliedVersions(t, db, "sqlite", []int64{1, 2, 3})
}

func openSQLiteTestDB(t *testing.T, fileName string) *sql.DB {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), fileName)
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("打开 sqlite db 失败: %v", err)
	}
	return db
}

func seedCurrentSQLiteBaselineSchema(t *testing.T, db *sql.DB) error {
	t.Helper()

	schema := `
CREATE TABLE agents (
    id VARCHAR(64) NOT NULL PRIMARY KEY,
    owner_user_id VARCHAR(64) NOT NULL,
    is_main BOOLEAN NOT NULL DEFAULT 0
);
CREATE TABLE users (
    user_id VARCHAR(64) NOT NULL PRIMARY KEY,
    username VARCHAR(128) NOT NULL,
    display_name VARCHAR(128) NOT NULL,
    role VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL,
    last_login_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL
);
CREATE TABLE auth_password_credentials (
    credential_id VARCHAR(64) NOT NULL PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    password_hash TEXT NOT NULL,
    password_algo VARCHAR(32) NOT NULL,
    password_updated_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL
);
CREATE TABLE auth_sessions (
    session_id VARCHAR(64) NOT NULL PRIMARY KEY,
    user_id VARCHAR(64) NOT NULL,
    session_token_hash VARCHAR(64) NOT NULL,
    auth_method VARCHAR(32) NOT NULL,
    expires_at DATETIME NOT NULL,
    last_seen_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    client_ip VARCHAR(255),
    user_agent TEXT,
    revoked_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL
);
CREATE TABLE rooms (
    id VARCHAR(64) NOT NULL PRIMARY KEY,
    owner_user_id VARCHAR(64) NOT NULL,
    avatar VARCHAR(255)
);
CREATE TABLE runtimes (
    id VARCHAR(64) NOT NULL PRIMARY KEY,
    provider VARCHAR(128)
);
CREATE TABLE provider (
    id VARCHAR(64) NOT NULL PRIMARY KEY,
    provider VARCHAR(128) NOT NULL,
    display_name VARCHAR(128) NOT NULL,
    auth_token TEXT NOT NULL,
    base_url TEXT NOT NULL,
    model VARCHAR(255) NOT NULL,
    enabled BOOLEAN NOT NULL,
    is_default BOOLEAN NOT NULL
);
CREATE TABLE automation_cron_jobs (
    job_id VARCHAR(64) NOT NULL PRIMARY KEY,
    source_kind VARCHAR(32) NOT NULL DEFAULT 'system',
    source_context_id VARCHAR(255)
);`
	_, err := db.Exec(schema)
	return err
}

func seedLatestPythonSQLiteSchema(t *testing.T, db *sql.DB) error {
	t.Helper()

	schema := `
CREATE TABLE agents (
    id VARCHAR(64) NOT NULL PRIMARY KEY
);
CREATE TABLE auth_sessions (
    id VARCHAR(64) NOT NULL PRIMARY KEY,
    session_token_hash VARCHAR(64) NOT NULL,
    username VARCHAR(128) NOT NULL,
    expires_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL
);
CREATE UNIQUE INDEX uq_auth_sessions_token_hash ON auth_sessions (session_token_hash);
CREATE INDEX idx_auth_sessions_expires_at ON auth_sessions (expires_at);
CREATE INDEX idx_auth_sessions_username ON auth_sessions (username);
CREATE TABLE rooms (
    id VARCHAR(64) NOT NULL PRIMARY KEY,
    avatar VARCHAR(64)
);
CREATE TABLE runtimes (
    id VARCHAR(64) NOT NULL PRIMARY KEY,
    provider VARCHAR(32)
);
CREATE TABLE provider (
    id VARCHAR(64) NOT NULL PRIMARY KEY,
    provider VARCHAR(64) NOT NULL,
    display_name VARCHAR(128) NOT NULL,
    auth_token TEXT NOT NULL,
    base_url TEXT NOT NULL,
    model VARCHAR(255) NOT NULL,
    enabled BOOLEAN NOT NULL,
    is_default BOOLEAN NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL
);
CREATE UNIQUE INDEX ix_provider_provider ON provider (provider);
CREATE TABLE automation_cron_jobs (
    job_id VARCHAR(64) NOT NULL PRIMARY KEY,
    source_kind VARCHAR(32) NOT NULL DEFAULT 'system',
    source_creator_agent_id VARCHAR(64),
    source_context_type VARCHAR(32),
    source_context_id VARCHAR(255),
    source_context_label VARCHAR(255),
    source_session_key VARCHAR(255),
    source_session_label VARCHAR(255)
);
INSERT INTO provider (
    id, provider, display_name, auth_token, base_url, model, enabled, is_default, created_at, updated_at
) VALUES (
    'prov_1', 'glm', 'GLM', 'token', 'https://example.com', 'glm-5.1', 1, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
);`
	_, err := db.Exec(schema)
	return err
}

func seedGooseVersions(ctx context.Context, db *sql.DB, databaseDriver string, versions ...int64) error {
	store, err := goosedb.NewStore(gooseDialect(databaseDriver), "goose_db_version")
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if err = store.CreateVersionTable(ctx, tx); err != nil {
		return err
	}
	for _, version := range versions {
		if err = store.Insert(ctx, tx, goosedb.InsertRequest{Version: version}); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func assertCurrentVersion(t *testing.T, db *sql.DB, databaseDriver string, expected int64) {
	t.Helper()

	version, err := currentGooseVersion(context.Background(), db, databaseDriver)
	if err != nil {
		t.Fatalf("读取当前 goose 版本失败: %v", err)
	}
	if version != expected {
		t.Fatalf("期望 goose 当前版本为 %d，实际得到 %d", expected, version)
	}
}

func assertAppliedVersions(t *testing.T, db *sql.DB, databaseDriver string, expected []int64) {
	t.Helper()

	store, err := goosedb.NewStore(gooseDialect(databaseDriver), "goose_db_version")
	if err != nil {
		t.Fatalf("创建 goose store 失败: %v", err)
	}
	items, err := store.ListMigrations(context.Background(), db)
	if err != nil {
		t.Fatalf("读取 goose 版本列表失败: %v", err)
	}
	actual := make([]int64, 0, len(items))
	for _, item := range items {
		if item == nil || !item.IsApplied || item.Version <= 0 {
			continue
		}
		actual = append(actual, item.Version)
	}
	sort.Slice(actual, func(left, right int) bool {
		return actual[left] < actual[right]
	})
	if len(actual) != len(expected) {
		t.Fatalf("期望 goose 已应用版本为 %v，实际得到 %v", expected, actual)
	}
	for index := range actual {
		if actual[index] != expected[index] {
			t.Fatalf("期望 goose 已应用版本为 %v，实际得到 %v", expected, actual)
		}
	}
}

func assertSQLiteColumnExists(t *testing.T, db *sql.DB, tableName string, columnName string) {
	t.Helper()

	exists, err := columnExists(context.Background(), db, "sqlite", tableName, columnName)
	if err != nil {
		t.Fatalf("检查列存在失败: table=%s column=%s err=%v", tableName, columnName, err)
	}
	if !exists {
		t.Fatalf("期望列存在: table=%s column=%s", tableName, columnName)
	}
}

func assertSQLiteColumnNotExists(t *testing.T, db *sql.DB, tableName string, columnName string) {
	t.Helper()

	exists, err := columnExists(context.Background(), db, "sqlite", tableName, columnName)
	if err != nil {
		t.Fatalf("检查列不存在失败: table=%s column=%s err=%v", tableName, columnName, err)
	}
	if exists {
		t.Fatalf("期望列不存在: table=%s column=%s", tableName, columnName)
	}
}

func assertSQLiteTableExists(t *testing.T, db *sql.DB, tableName string) {
	t.Helper()

	exists, err := tableExists(context.Background(), db, "sqlite", tableName)
	if err != nil {
		t.Fatalf("检查表存在失败: table=%s err=%v", tableName, err)
	}
	if !exists {
		t.Fatalf("期望表存在: %s", tableName)
	}
}

func assertSQLiteTableNotExists(t *testing.T, db *sql.DB, tableName string) {
	t.Helper()

	exists, err := tableExists(context.Background(), db, "sqlite", tableName)
	if err != nil {
		t.Fatalf("检查表不存在失败: table=%s err=%v", tableName, err)
	}
	if exists {
		t.Fatalf("期望表不存在: %s", tableName)
	}
}
