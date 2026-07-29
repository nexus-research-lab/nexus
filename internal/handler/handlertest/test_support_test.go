package handlertest

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestMigrateSQLiteFromDirClonesIndependentDatabases(t *testing.T) {
	root := t.TempDir()
	firstPath := filepath.Join(root, "first.db")
	secondPath := filepath.Join(root, "second.db")
	migrationDirectory := migrationDir(t)

	MigrateSQLiteFromDir(t, firstPath, migrationDirectory)
	MigrateSQLiteFromDir(t, secondPath, migrationDirectory)

	first := openSnapshotTestDB(t, firstPath)
	defer func() { _ = first.Close() }()
	second := openSnapshotTestDB(t, secondPath)
	defer func() { _ = second.Close() }()

	if _, err := first.Exec(`CREATE TABLE snapshot_probe (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("修改第一个快照数据库失败: %v", err)
	}
	var tableCount int
	if err := second.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'snapshot_probe'`,
	).Scan(&tableCount); err != nil {
		t.Fatalf("检查第二个快照数据库失败: %v", err)
	}
	if tableCount != 0 {
		t.Fatalf("快照数据库不应共享后续 schema 修改: count=%d", tableCount)
	}
}

func openSnapshotTestDB(t *testing.T, databaseURL string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", databaseURL)
	if err != nil {
		t.Fatalf("打开快照测试数据库失败: %v", err)
	}
	return db
}
