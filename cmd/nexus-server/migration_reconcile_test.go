package main

import (
	"database/sql"
	"io"
	"log/slog"
	"math"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

func TestRunMigrationsRepairsLegacySQLiteVersionZero(t *testing.T) {
	t.Chdir(projectRootForMigrationTest(t))

	dbPath := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("设置 goose dialect 失败: %v", err)
	}
	if err = goose.UpTo(db, filepath.Join("db", "migrations", "sqlite"), 2); err != nil {
		t.Fatalf("准备 legacy schema 失败: %v", err)
	}
	if _, err = db.Exec("DELETE FROM goose_db_version"); err != nil {
		t.Fatalf("清理 goose version 失败: %v", err)
	}
	if _, err = db.Exec("INSERT INTO goose_db_version (version_id, is_applied) VALUES (0, 1)"); err != nil {
		t.Fatalf("写入 legacy goose version 失败: %v", err)
	}
	if err = db.Close(); err != nil {
		t.Fatalf("关闭准备数据库失败: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.Config{DatabaseDriver: "sqlite", DatabaseURL: dbPath}
	if err = runMigrations(cfg, logger); err != nil {
		t.Fatalf("legacy schema 迁移失败: %v", err)
	}

	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("重新打开测试数据库失败: %v", err)
	}
	defer db.Close()
	version, err := goose.GetDBVersion(db)
	if err != nil {
		t.Fatalf("读取 goose version 失败: %v", err)
	}
	wantVersion := latestSQLiteMigrationVersion(t)
	if version != wantVersion {
		t.Fatalf("goose version = %d, want %d", version, wantVersion)
	}
	if !migrationTestColumnExists(t, db, "rooms", "skill_names") {
		t.Fatal("后续迁移未继续执行: rooms.skill_names 不存在")
	}
}

func projectRootForMigrationTest(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("无法定位测试文件")
	}
	return filepath.Join(filepath.Dir(file), "..", "..")
}

func latestSQLiteMigrationVersion(t *testing.T) int64 {
	t.Helper()

	migrations, err := goose.CollectMigrations(filepath.Join("db", "migrations", "sqlite"), 0, math.MaxInt64)
	if err != nil {
		t.Fatalf("读取 SQLite migration 文件失败: %v", err)
	}
	if len(migrations) == 0 {
		t.Fatal("未读取到 SQLite migration 文件")
	}
	return migrations[len(migrations)-1].Version
}

func migrationTestColumnExists(t *testing.T, db *sql.DB, table string, column string) bool {
	t.Helper()

	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		t.Fatalf("读取表结构失败: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var pk int
		if err = rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			t.Fatalf("扫描表结构失败: %v", err)
		}
		if name == column {
			return true
		}
	}
	if err = rows.Err(); err != nil {
		t.Fatalf("遍历表结构失败: %v", err)
	}
	return false
}
