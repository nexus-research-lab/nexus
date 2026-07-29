package handlertest

import (
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

type sqliteMigrationTemplate struct {
	once sync.Once
	db   *sql.DB
	err  error
}

var (
	sqliteMigrationTemplatesMu sync.Mutex
	sqliteMigrationTemplates   = map[string]*sqliteMigrationTemplate{}
)

// NewConfig 返回HTTP 服务测试配置。
func NewConfig(t testing.TB) config.Config {
	t.Helper()

	root := t.TempDir()
	stateRoot := filepath.Join(root, ".nexus")
	t.Setenv("HOME", root)
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", stateRoot)
	return config.Config{
		Host:           "127.0.0.1",
		Port:           18031,
		ProjectName:    "nexus-handler-test",
		APIPrefix:      "/nexus/v1",
		WebSocketPath:  "/nexus/v1/chat/ws",
		DefaultAgentID: "nexus",
		WorkspacePath:  filepath.Join(root, "workspace"),
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(root, "nexus.db"),
	}
}

// OpenSQLite 打开测试数据库。
func OpenSQLite(t testing.TB, databaseURL string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", databaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	return db
}

// MigrateSQLite 执行 SQLite migration。
func MigrateSQLite(t testing.TB, databaseURL string) {
	t.Helper()
	MigrateSQLiteFromDir(t, databaseURL, migrationDir(t))
}

// MigrateSQLiteFromDir 从预迁移快照复制 SQLite 测试数据库。
//
// 每个测试仍拿到独立数据库文件，但同一个测试进程只执行一次完整
// migration，避免几十个测试重复解析和执行同一批 schema 变更。
func MigrateSQLiteFromDir(t testing.TB, databaseURL string, migrationDirectory string) {
	t.Helper()

	if shouldUseSQLiteSnapshot(databaseURL) {
		template := sqliteMigrationTemplateFor(migrationDirectory)
		template.once.Do(func() {
			template.db, template.err = openMigratedSQLiteTemplate(migrationDirectory)
		})
		if template.err == nil {
			if err := cloneSQLiteTemplate(template.db, databaseURL); err == nil {
				return
			}
		}
	}

	migrateSQLiteDirect(t, databaseURL, migrationDirectory)
}

func sqliteMigrationTemplateFor(migrationDirectory string) *sqliteMigrationTemplate {
	key := normalizedMigrationDirectory(migrationDirectory)
	sqliteMigrationTemplatesMu.Lock()
	defer sqliteMigrationTemplatesMu.Unlock()
	template := sqliteMigrationTemplates[key]
	if template == nil {
		template = &sqliteMigrationTemplate{}
		sqliteMigrationTemplates[key] = template
	}
	return template
}

func openMigratedSQLiteTemplate(migrationDirectory string) (*sql.DB, error) {
	migrationDirectory = normalizedMigrationDirectory(migrationDirectory)
	digest := sha256.Sum256([]byte(migrationDirectory))
	dsn := fmt.Sprintf("file:nexus-test-migration-%x?mode=memory&cache=shared", digest[:8])
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err = db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err = goose.SetDialect("sqlite3"); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err = goose.Up(db, migrationDirectory); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func normalizedMigrationDirectory(migrationDirectory string) string {
	directory := filepath.Clean(strings.TrimSpace(migrationDirectory))
	absoluteDirectory, err := filepath.Abs(directory)
	if err != nil {
		return directory
	}
	return absoluteDirectory
}

func cloneSQLiteTemplate(template *sql.DB, databaseURL string) error {
	if template == nil {
		return errors.New("SQLite migration template 为空")
	}
	if err := os.MkdirAll(filepath.Dir(databaseURL), 0o755); err != nil {
		return err
	}
	if _, err := template.Exec(`VACUUM INTO ?`, databaseURL); err != nil {
		return err
	}
	return nil
}

func shouldUseSQLiteSnapshot(databaseURL string) bool {
	value := strings.TrimSpace(databaseURL)
	return value != "" &&
		value != ":memory:" &&
		!strings.Contains(value, "?") &&
		!strings.HasPrefix(strings.ToLower(value), "file:")
}

func migrateSQLiteDirect(t testing.TB, databaseURL string, migrationDirectory string) {
	t.Helper()

	db := OpenSQLite(t, databaseURL)
	defer func() { _ = db.Close() }()

	if err := goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("设置 goose 方言失败: %v", err)
	}
	if err := goose.Up(db, migrationDirectory); err != nil {
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
