// INPUT: 数据库驱动、连接 URL 与 SQLite 运行/迁移连接语义。
// OUTPUT: 带连接池约束、busy timeout 与运行期外键保护的 sql.DB。
// POS: storage 包的数据库连接入口；migration 必须显式使用无外键连接。
package storage

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

// OpenDB 打开当前配置对应的数据库连接。
func OpenDB(cfg config.Config) (*sql.DB, error) {
	return openDB(cfg, true)
}

// OpenMigrationDB 打开允许 migration 自行切换 SQLite 外键状态的数据库连接。
func OpenMigrationDB(cfg config.Config) (*sql.DB, error) {
	return openDB(cfg, false)
}

func openDB(cfg config.Config, enforceForeignKeys bool) (*sql.DB, error) {
	driver := NormalizeSQLDriver(cfg.DatabaseDriver)
	dsn := NormalizeDatabaseURL(cfg.DatabaseURL)

	// SQLite 场景需要提前创建父目录，否则第一次启动会直接报错。
	if IsSQLiteSQLDriver(driver) {
		if err := ensureParentDir(dsn); err != nil {
			return nil, err
		}
		var err error
		dsn, err = sqliteConnectionDSN(dsn, enforceForeignKeys)
		if err != nil {
			return nil, err
		}
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := configureConnectionPool(db, driver, enforceForeignKeys); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func configureConnectionPool(db *sql.DB, driver string, enforceForeignKeys bool) error {
	if IsSQLiteSQLDriver(driver) {
		// SQLite 只有单写者，收敛连接数能避免多连接写入互相抢锁。
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
		var foreignKeys int
		if err := db.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
			return fmt.Errorf("read sqlite foreign_keys: %w", err)
		}
		expected := 0
		if enforceForeignKeys {
			expected = 1
		}
		if foreignKeys != expected {
			return fmt.Errorf("sqlite foreign_keys = %d, want %d", foreignKeys, expected)
		}
		return nil
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(30 * time.Minute)
	return nil
}

func sqliteConnectionDSN(dsn string, enforceForeignKeys bool) (string, error) {
	base, rawQuery, _ := strings.Cut(dsn, "?")
	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return "", fmt.Errorf("parse sqlite DSN query: %w", err)
	}
	pragmas := values["_pragma"][:0]
	for _, pragma := range values["_pragma"] {
		switch sqlitePragmaName(pragma) {
		case "busy_timeout", "foreign_keys":
			continue
		default:
			pragmas = append(pragmas, pragma)
		}
	}
	foreignKeys := "foreign_keys(0)"
	if enforceForeignKeys {
		foreignKeys = "foreign_keys(1)"
	}
	values["_pragma"] = append(pragmas, "busy_timeout(5000)", foreignKeys)
	return base + "?" + values.Encode(), nil
}

func sqlitePragmaName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if index := strings.IndexAny(value, "(= \t"); index >= 0 {
		return value[:index]
	}
	return value
}

func ensureParentDir(path string) error {
	path, _, _ = strings.Cut(path, "?")
	normalized := strings.TrimSpace(path)
	if normalized == "" || normalized == ":memory:" {
		return nil
	}
	parent := filepath.Dir(normalized)
	if parent == "." || parent == "/" {
		return nil
	}
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return fmt.Errorf("create sqlite parent dir: %w", err)
	}
	return nil
}
