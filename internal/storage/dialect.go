package storage

import (
	"os"
	"path/filepath"
	"strings"
)

// MigrationDirName 返回数据库驱动对应的 migration 目录名。
func MigrationDirName(driver string) string {
	switch strings.ToLower(driver) {
	case "postgres", "postgresql", "pg":
		return "postgres"
	default:
		return "sqlite"
	}
}

// GooseDialect 返回 goose 识别的方言名。
func GooseDialect(driver string) string {
	switch strings.ToLower(driver) {
	case "postgres", "postgresql", "pg":
		return "postgres"
	default:
		return "sqlite3"
	}
}

// NormalizeSQLDriver 把配置里的数据库驱动名规范化为 database/sql 名称。
func NormalizeSQLDriver(driver string) string {
	switch strings.ToLower(driver) {
	case "postgres", "postgresql", "pg":
		return "pgx"
	default:
		return "sqlite"
	}
}

// IsSQLiteSQLDriver 判断 database/sql 驱动名是否为 SQLite。
func IsSQLiteSQLDriver(driver string) bool {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "sqlite", "sqlite3":
		return true
	default:
		return false
	}
}

// NormalizeDatabaseURL 把配置格式转为 Go SQL 驱动可识别的 DSN。
func NormalizeDatabaseURL(raw string) string {
	normalized := strings.TrimSpace(raw)
	normalized = trimSQLiteScheme(normalized)
	return expandHomePath(normalized)
}

func trimSQLiteScheme(value string) string {
	lower := strings.ToLower(value)
	switch {
	case strings.HasPrefix(lower, "sqlite:///"):
		return value[len("sqlite:///"):]
	case strings.HasPrefix(lower, "sqlite://"):
		return value[len("sqlite://"):]
	default:
		return value
	}
}

func expandHomePath(value string) string {
	switch {
	case value == "~":
		home, err := os.UserHomeDir()
		if err == nil {
			return home
		}
	case strings.HasPrefix(value, "~/"), strings.HasPrefix(value, `~\`):
		home, err := os.UserHomeDir()
		if err == nil {
			relative := strings.TrimLeft(value[2:], `/\`)
			relative = strings.ReplaceAll(relative, `\`, "/")
			return filepath.Join(home, filepath.FromSlash(relative))
		}
	}
	return value
}
