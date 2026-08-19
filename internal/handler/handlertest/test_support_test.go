package handlertest

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestCreateAppRootWithSkillsCopiesOnlySelectedSkill(t *testing.T) {
	root, err := createAppRootWithSkills([]string{"goal-manager"})
	if err != nil {
		t.Fatalf("创建带选定 Skill 的测试应用根失败: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })

	if _, err = os.Stat(filepath.Join(root, "skills", "goal-manager", "SKILL.md")); err != nil {
		t.Fatalf("选定 Skill 未复制: %v", err)
	}
	if _, err = os.Stat(filepath.Join(root, "skills", "wechat-article-search")); !os.IsNotExist(err) {
		t.Fatalf("未选定 Skill 不应复制: %v", err)
	}
}

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

func TestMigrateSQLiteFromDirPreservesExistingDatabase(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "existing.db")
	database := openSnapshotTestDB(t, databasePath)
	if _, err := database.Exec(`CREATE TABLE existing_probe (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("创建已有测试表失败: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("关闭已有测试数据库失败: %v", err)
	}

	MigrateSQLiteFromDir(t, databasePath, migrationDir(t))

	database = openSnapshotTestDB(t, databasePath)
	defer func() { _ = database.Close() }()
	var tableCount int
	if err := database.QueryRow(
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'existing_probe'`,
	).Scan(&tableCount); err != nil {
		t.Fatalf("检查已有测试表失败: %v", err)
	}
	if tableCount != 1 {
		t.Fatalf("已有数据库不应被快照覆盖: count=%d", tableCount)
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
