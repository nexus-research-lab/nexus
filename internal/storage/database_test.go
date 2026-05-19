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
