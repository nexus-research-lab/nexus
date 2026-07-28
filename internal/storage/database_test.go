package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
)

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
