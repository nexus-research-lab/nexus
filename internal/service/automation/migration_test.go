package automation

import (
	"database/sql"
	"math"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

func TestSQLiteAutomationDeliveryRouteMigrationUpgradesNewerParallelLedger(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "nexus.db"))
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("设置 goose 方言失败: %v", err)
	}
	dir := automationMigrationDir(t)
	if err = goose.UpTo(db, dir, 86); err != nil {
		t.Fatalf("迁移到 Automation 权限 schema 失败: %v", err)
	}
	for version := int64(87); version <= 92; version++ {
		if _, err = db.Exec(
			"INSERT INTO goose_db_version (version_id, is_applied) VALUES (?, TRUE)",
			version,
		); err != nil {
			t.Fatalf("模拟并行分支 migration %d 失败: %v", version, err)
		}
	}
	if err = goose.Up(db, dir); err != nil {
		t.Fatalf("从较新并行账本执行 Automation 投递 migration 失败: %v", err)
	}
	version, err := goose.GetDBVersion(db)
	if err != nil {
		t.Fatalf("读取 migration 版本失败: %v", err)
	}
	migrations, err := goose.CollectMigrations(dir, 0, math.MaxInt64)
	if err != nil || len(migrations) == 0 {
		t.Fatalf("collect current migrations: %v", err)
	}
	wantVersion := migrations[len(migrations)-1].Version
	if version != wantVersion {
		t.Fatalf("migration version = %d, want %d", version, wantVersion)
	}
	for _, target := range []struct {
		table  string
		column string
	}{
		{table: "automation_scheduled_tasks", column: "delivery_session_key"},
		{table: "automation_scheduled_tasks", column: "permission_mode"},
		{table: "automation_scheduled_tasks", column: "session_binding_state"},
		{table: "automation_scheduled_tasks", column: "invalidated_session_keys_json"},
		{table: "automation_scheduled_tasks", column: "delivery_grant_json"},
		{table: "automation_task_runs", column: "delivery_target_json"},
		{table: "automation_delivery_routes", column: "context_token"},
		{table: "automation_permission_requests", column: "delivery_session_key"},
	} {
		var found string
		if err = db.QueryRow(
			"SELECT name FROM pragma_table_info(?) WHERE name = ?",
			target.table,
			target.column,
		).Scan(&found); err != nil {
			t.Fatalf("缺少升级字段 %s.%s: %v", target.table, target.column, err)
		}
	}
}

func automationMigrationDir(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("无法定位当前测试文件")
	}
	return filepath.Join(filepath.Dir(filename), "..", "..", "..", "db", "migrations", "sqlite")
}
