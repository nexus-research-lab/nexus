package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
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

// SQLDialect 封装数据库方言中实际分叉的 SQL 片段。
type SQLDialect struct {
	postgres bool
}

func NewSQLDialect(driver string) SQLDialect {
	return SQLDialect{postgres: NormalizeSQLDriver(driver) == "pgx"}
}

func (d SQLDialect) Bind(index int) string {
	if d.postgres {
		return fmt.Sprintf("$%d", index)
	}
	return "?"
}

func (d SQLDialect) BindList(count int) string {
	items := make([]string, 0, count)
	for index := 1; index <= count; index++ {
		items = append(items, d.Bind(index))
	}
	return strings.Join(items, ",")
}

func (d SQLDialect) TrueValue() string {
	if d.postgres {
		return "true"
	}
	return "1"
}

func (d SQLDialect) FalseValue() string {
	if d.postgres {
		return "false"
	}
	return "0"
}

func (d SQLDialect) CurrentTimestamp() string {
	if d.postgres {
		return "now()"
	}
	return "CURRENT_TIMESTAMP"
}

// TimestampValue 返回适合当前 SQL 方言比较和写入的时间值。
func (d SQLDialect) TimestampValue(value time.Time) any {
	normalized := value.UTC()
	if d.postgres {
		return normalized
	}
	return normalized.Format("2006-01-02 15:04:05.999999999")
}

func (d SQLDialect) JSONText(expression string) string {
	if d.postgres {
		return expression + "::text"
	}
	return expression
}

func (d SQLDialect) JSONValue(index int) string {
	bind := d.Bind(index)
	if d.postgres {
		return bind + "::json"
	}
	return "json(" + bind + ")"
}

func (d SQLDialect) InsertIgnoreInto(table string) string {
	if d.postgres {
		return "INSERT INTO " + table
	}
	return "INSERT OR IGNORE INTO " + table
}

func (d SQLDialect) InsertIgnoreSuffix() string {
	if d.postgres {
		return "\nON CONFLICT DO NOTHING"
	}
	return ""
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
