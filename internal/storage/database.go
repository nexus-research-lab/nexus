// =====================================================
// @File   ：database.go
// @Date   ：2026/04/10 21:22:41
// @Author ：leemysw
// 2026/04/10 21:22:41   Create
// =====================================================

package storage

import (
	"database/sql"
	"fmt"
	"github.com/nexus-research-lab/nexus-core/internal/config"
	"github.com/nexus-research-lab/nexus-core/internal/protocol"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/mattn/go-sqlite3"
)

// OpenDB 打开当前配置对应的数据库连接。
func OpenDB(cfg config.Config) (*sql.DB, error) {
	driver := protocol.NormalizeSQLDriver(cfg.DatabaseDriver)
	dsn := protocol.NormalizeDatabaseURL(cfg.DatabaseURL)

	// 中文注释：SQLite 场景需要提前创建父目录，否则第一次启动会直接报错。
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
	return db, nil
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
