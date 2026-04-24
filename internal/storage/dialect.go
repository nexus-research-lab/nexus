package storage

import (
	"os"
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
		return "sqlite3"
	}
}

// NormalizeDatabaseURL 把配置格式转为 Go SQL 驱动可识别的 DSN。
func NormalizeDatabaseURL(raw string) string {
	normalized := strings.TrimSpace(raw)
	switch {
	case strings.HasPrefix(normalized, "sqlite:///"):
		return strings.TrimPrefix(normalized, "sqlite:///")
	case strings.HasPrefix(normalized, "~/"):
		home, err := os.UserHomeDir()
		if err == nil {
			return strings.Replace(normalized, "~/", home+"/", 1)
		}
	}
	return normalized
}
