package handlertest

import (
	"database/sql"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pressly/goose/v3"
)

// NewConfig 返回HTTP 服务测试配置。
func NewConfig(t testing.TB) config.Config {
	t.Helper()

	root := t.TempDir()
	t.Setenv("HOME", root)
	return config.Config{
		Host:           "127.0.0.1",
		Port:           18031,
		ProjectName:    "nexus-handler-test",
		APIPrefix:      "/agent/v1",
		WebSocketPath:  "/agent/v1/chat/ws",
		DefaultAgentID: "nexus",
		WorkspacePath:  filepath.Join(root, "workspace"),
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(root, "nexus.db"),
	}
}

// OpenSQLite 打开测试数据库。
func OpenSQLite(t testing.TB, databaseURL string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", databaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	return db
}

// MigrateSQLite 执行 SQLite migration。
func MigrateSQLite(t testing.TB, databaseURL string) {
	t.Helper()

	db := OpenSQLite(t, databaseURL)
	defer db.Close()

	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("设置 goose 方言失败: %v", err)
	}
	if err := goose.Up(db, migrationDir(t)); err != nil {
		t.Fatalf("执行 migration 失败: %v", err)
	}
}

func migrationDir(t testing.TB) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations", "sqlite")
}
