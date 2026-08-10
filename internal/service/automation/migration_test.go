package automation

import (
	"database/sql"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

func TestSQLiteMigrationsKeepScheduledTaskRunsAfterTaskDelete(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "nexus.db"))
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("设置 goose 方言失败: %v", err)
	}
	if err = goose.Up(db, automationMigrationDir(t)); err != nil {
		t.Fatalf("执行 migration 失败: %v", err)
	}
	if _, err = db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		t.Fatalf("启用 foreign_keys 失败: %v", err)
	}

	rows, err := db.Query(`PRAGMA foreign_key_list(automation_task_runs)`)
	if err != nil {
		t.Fatalf("读取 automation_task_runs 外键失败: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var seq int
		var tableName string
		var fromColumn string
		var toColumn string
		var onUpdate string
		var onDelete string
		var match string
		if err = rows.Scan(&id, &seq, &tableName, &fromColumn, &toColumn, &onUpdate, &onDelete, &match); err != nil {
			t.Fatalf("扫描 automation_task_runs 外键失败: %v", err)
		}
		if tableName == "automation_scheduled_tasks" && fromColumn == "job_id" {
			t.Fatalf("automation_task_runs.job_id 不应级联依赖 automation_scheduled_tasks: on_delete=%s", onDelete)
		}
	}
	if err = rows.Err(); err != nil {
		t.Fatalf("遍历 automation_task_runs 外键失败: %v", err)
	}

	_, err = db.Exec(`
INSERT INTO automation_task_runs (
    run_id, job_id, owner_user_id, status, trigger_kind, attempts
) VALUES (
    'run-orphan', 'deleted-job', '__system__', 'succeeded', 'manual', 1
)`)
	if err != nil {
		t.Fatalf("删除任务后的 run ledger 应可独立保留: %v", err)
	}
}

func TestSQLiteScheduledTaskNamingMigrationPreservesRuns(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "nexus.db"))
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("设置 goose 方言失败: %v", err)
	}
	dir := automationMigrationDir(t)
	if err = goose.UpTo(db, dir, 48); err != nil {
		t.Fatalf("迁移到旧调度 schema 失败: %v", err)
	}
	if _, err = db.Exec(`
INSERT INTO automation_cron_runs (
    run_id, job_id, owner_user_id, status, trigger_kind, attempts
) VALUES (
    'run-before-rename', 'task-before-rename', '__system__', 'succeeded', 'cron', 1
)`); err != nil {
		t.Fatalf("写入旧调度运行记录失败: %v", err)
	}

	if err = goose.Up(db, dir); err != nil {
		t.Fatalf("执行 scheduled task 命名迁移失败: %v", err)
	}
	var triggerKind string
	if err = db.QueryRow(`SELECT trigger_kind FROM automation_task_runs WHERE run_id = 'run-before-rename'`).Scan(&triggerKind); err != nil {
		t.Fatalf("读取迁移后的运行记录失败: %v", err)
	}
	if triggerKind != "scheduled" {
		t.Fatalf("trigger_kind = %q, want scheduled", triggerKind)
	}
	if _, err = db.Exec(`SELECT 1 FROM automation_cron_runs LIMIT 1`); err == nil {
		t.Fatal("命名迁移后不应继续保留 automation_cron_runs")
	}

	if err = goose.DownTo(db, dir, 48); err != nil {
		t.Fatalf("回滚 scheduled task 命名迁移失败: %v", err)
	}
	if err = db.QueryRow(`SELECT trigger_kind FROM automation_cron_runs WHERE run_id = 'run-before-rename'`).Scan(&triggerKind); err != nil {
		t.Fatalf("读取回滚后的运行记录失败: %v", err)
	}
	if triggerKind != "cron" {
		t.Fatalf("回滚后 trigger_kind = %q, want cron", triggerKind)
	}
}

func TestSQLiteAutomationConfigurationVersionMigration(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "nexus.db"))
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("设置 goose 方言失败: %v", err)
	}
	if err = goose.Up(db, automationMigrationDir(t)); err != nil {
		t.Fatalf("执行 migration 失败: %v", err)
	}
	for _, target := range []struct {
		table  string
		column string
	}{
		{table: "automation_scheduled_tasks", column: "configuration_version"},
		{table: "automation_heartbeat_states", column: "configuration_version"},
	} {
		rows, queryErr := db.Query("PRAGMA table_info(" + target.table + ")")
		if queryErr != nil {
			t.Fatalf("读取 %s schema: %v", target.table, queryErr)
		}
		found := false
		for rows.Next() {
			var cid int
			var name string
			var columnType string
			var notNull int
			var defaultValue sql.NullString
			var primaryKey int
			if scanErr := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); scanErr != nil {
				_ = rows.Close()
				t.Fatalf("扫描 %s schema: %v", target.table, scanErr)
			}
			if name == target.column {
				found = notNull == 1 && defaultValue.Valid && defaultValue.String == "1"
			}
		}
		_ = rows.Close()
		if !found {
			t.Fatalf("%s.%s missing required default/version constraint", target.table, target.column)
		}
	}
	if _, err = db.Exec(`
INSERT INTO automation_task_create_requests (
    owner_user_id, request_id, job_id, agent_id, intent_digest
) VALUES ('owner', 'request', 'job', 'agent', 'digest')`); err != nil {
		t.Fatalf("写入 automation_task_create_requests: %v", err)
	}
	if _, err = db.Exec(`
INSERT INTO automation_task_create_requests (
    owner_user_id, request_id, job_id, agent_id, intent_digest
) VALUES ('owner', 'request', 'other-job', 'agent', 'other-digest')`); err == nil {
		t.Fatal("owner/request_id unique boundary was not enforced")
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
