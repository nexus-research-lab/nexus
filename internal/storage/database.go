package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/mattn/go-sqlite3"
)

// OpenDB 打开当前配置对应的数据库连接。
func OpenDB(cfg config.Config) (*sql.DB, error) {
	driver := NormalizeSQLDriver(cfg.DatabaseDriver)
	dsn := NormalizeDatabaseURL(cfg.DatabaseURL)

	// SQLite 场景需要提前创建父目录，否则第一次启动会直接报错。
	if driver == "sqlite3" {
		if err := ensureParentDir(dsn); err != nil {
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
	if err := configureConnectionPool(db, driver); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func configureConnectionPool(db *sql.DB, driver string) error {
	if driver == "sqlite3" {
		// SQLite 只有单写者，收敛连接数能避免多连接写入互相抢锁。
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
		if _, err := db.Exec("PRAGMA busy_timeout = 5000"); err != nil {
			return fmt.Errorf("set sqlite busy_timeout: %w", err)
		}
		return nil
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(30 * time.Minute)
	return nil
}

func ensureParentDir(path string) error {
	normalized := strings.TrimSpace(path)
	if normalized == "" || normalized == ":memory:" {
		return nil
	}
	parent := filepath.Dir(normalized)
	if parent == "." || parent == "/" {
		return nil
	}
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("create sqlite parent dir: %w", err)
	}
	return nil
}
