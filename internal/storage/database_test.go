package storage

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
)

func TestNullableTimeSupportsDriverAndSQLiteAggregateValues(t *testing.T) {
	want := time.Date(2026, 8, 17, 12, 34, 56, 123456789, time.UTC)
	values := []any{
		want,
		want.Format(time.RFC3339Nano),
		[]byte(want.Format("2006-01-02 15:04:05.999999999-07:00")),
	}
	for _, value := range values {
		got, err := NullableTime(value)
		if err != nil {
			t.Fatalf("NullableTime(%T): %v", value, err)
		}
		if got == nil || !got.Equal(want) {
			t.Fatalf("NullableTime(%T) = %v, want %v", value, got, want)
		}
	}
	got, err := NullableTime(nil)
	if err != nil || got != nil {
		t.Fatalf("NullableTime(nil) = %v, %v", got, err)
	}
}

func TestNormalizeDatabaseURLExpandsHomeAfterSQLiteScheme(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("读取用户目录失败: %v", err)
	}

	got := NormalizeDatabaseURL("sqlite:///~/.nexus/data/nexus.db")
	want := filepath.Join(home, ".nexus", "data", "nexus.db")
	if got != want {
		t.Fatalf("sqlite URL home 展开不正确: got=%q want=%q", got, want)
	}

	got = NormalizeDatabaseURL(`sqlite:///~\.nexus\data\nexus.db`)
	want = filepath.Join(home, ".nexus", "data", "nexus.db")
	if got != want {
		t.Fatalf("sqlite URL Windows home 展开不正确: got=%q want=%q", got, want)
	}
}

func TestSQLDialect(t *testing.T) {
	tests := []struct {
		name       string
		driver     string
		firstBind  string
		threeBinds string
		timestamp  string
		jsonText   string
		jsonValue  string
		insert     string
		suffix     string
	}{
		{
			name:       "postgres",
			driver:     "postgres",
			firstBind:  "$1",
			threeBinds: "$1,$2,$3",
			timestamp:  "now()",
			jsonText:   "payload::text",
			jsonValue:  "$2::json",
			insert:     "INSERT INTO sessions",
			suffix:     "\nON CONFLICT DO NOTHING",
		},
		{
			name:       "sqlite",
			driver:     "sqlite",
			firstBind:  "?",
			threeBinds: "?,?,?",
			timestamp:  "CURRENT_TIMESTAMP",
			jsonText:   "payload",
			jsonValue:  "json(?)",
			insert:     "INSERT OR IGNORE INTO sessions",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dialect := NewSQLDialect(test.driver)
			if got := dialect.Bind(1); got != test.firstBind {
				t.Fatalf("Bind(1) = %q, want %q", got, test.firstBind)
			}
			if got := dialect.BindList(3); got != test.threeBinds {
				t.Fatalf("BindList(3) = %q, want %q", got, test.threeBinds)
			}
			if got := dialect.CurrentTimestamp(); got != test.timestamp {
				t.Fatalf("CurrentTimestamp() = %q, want %q", got, test.timestamp)
			}
			if got := dialect.JSONText("payload"); got != test.jsonText {
				t.Fatalf("JSONText() = %q, want %q", got, test.jsonText)
			}
			if got := dialect.JSONValue(2); got != test.jsonValue {
				t.Fatalf("JSONValue() = %q, want %q", got, test.jsonValue)
			}
			if got := dialect.InsertIgnoreInto("sessions"); got != test.insert {
				t.Fatalf("InsertIgnoreInto() = %q, want %q", got, test.insert)
			}
			if got := dialect.InsertIgnoreSuffix(); got != test.suffix {
				t.Fatalf("InsertIgnoreSuffix() = %q, want %q", got, test.suffix)
			}
		})
	}
}

func TestOpenDBCreatesSQLiteParentDir(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "missing", "data", "nexus.db")
	db, err := OpenDB(config.Config{
		DatabaseDriver: "sqlite",
		DatabaseURL:    databasePath,
	})
	if err != nil {
		t.Fatalf("打开 SQLite 数据库失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err = os.Stat(filepath.Dir(databasePath)); err != nil {
		t.Fatalf("SQLite 父目录未创建: %v", err)
	}
}

func TestOpenDBEnablesSQLiteForeignKeysAndCascades(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "foreign-keys.db")
	db, err := OpenDB(config.Config{
		DatabaseDriver: "sqlite",
		DatabaseURL:    databasePath,
	})
	if err != nil {
		t.Fatalf("打开 SQLite 数据库失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	var enabled int
	if err = db.QueryRow("PRAGMA foreign_keys").Scan(&enabled); err != nil {
		t.Fatalf("读取 foreign_keys 失败: %v", err)
	}
	if enabled != 1 {
		t.Fatalf("foreign_keys = %d, want 1", enabled)
	}
	for _, statement := range []string{
		`CREATE TABLE parents (id TEXT PRIMARY KEY)`,
		`CREATE TABLE children (
			id TEXT PRIMARY KEY,
			parent_id TEXT NOT NULL REFERENCES parents(id) ON DELETE CASCADE
		)`,
		`INSERT INTO parents (id) VALUES ('parent-1')`,
		`INSERT INTO children (id, parent_id) VALUES ('child-1', 'parent-1')`,
		`DELETE FROM parents WHERE id = 'parent-1'`,
	} {
		if _, err = db.Exec(statement); err != nil {
			t.Fatalf("执行外键夹具失败: %v", err)
		}
	}
	var childCount int
	if err = db.QueryRow("SELECT COUNT(*) FROM children").Scan(&childCount); err != nil {
		t.Fatalf("读取 child 数量失败: %v", err)
	}
	if childCount != 0 {
		t.Fatalf("级联删除后 children = %d, want 0", childCount)
	}
}

func TestOpenMigrationDBLeavesSQLiteForeignKeysDisabled(t *testing.T) {
	db, err := OpenMigrationDB(config.Config{
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(t.TempDir(), "migration.db"),
	})
	if err != nil {
		t.Fatalf("打开 migration SQLite 数据库失败: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	var enabled int
	if err = db.QueryRow("PRAGMA foreign_keys").Scan(&enabled); err != nil {
		t.Fatalf("读取 foreign_keys 失败: %v", err)
	}
	if enabled != 0 {
		t.Fatalf("migration foreign_keys = %d, want 0", enabled)
	}
}

func TestOpenDBStartsSQLiteTransactionsImmediate(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "immediate.db")
	db, err := OpenDB(config.Config{
		DatabaseDriver: "sqlite",
		DatabaseURL:    databasePath,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err = db.Exec(`CREATE TABLE writes (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}

	competitor, err := sql.Open(
		"sqlite",
		databasePath+"?_pragma=busy_timeout(25)&_pragma=foreign_keys(1)",
	)
	if err != nil {
		t.Fatal(err)
	}
	competitor.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = competitor.Close() })

	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, writeErr := competitor.Exec(`INSERT INTO writes (id) VALUES (1)`); writeErr == nil || !strings.Contains(strings.ToLower(writeErr.Error()), "locked") {
		_ = tx.Rollback()
		t.Fatalf("competing write error = %v, want immediate transaction lock", writeErr)
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if _, err = competitor.Exec(`INSERT INTO writes (id) VALUES (1)`); err != nil {
		t.Fatalf("write after immediate transaction release: %v", err)
	}
}
