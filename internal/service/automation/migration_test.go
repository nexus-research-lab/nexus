package automation

import (
	"database/sql"
	"math"
	"path/filepath"
	"runtime"
	"strings"
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

func TestSQLiteDeliveryAttemptMigrationBackfillsLatestAuthoritativeTerminalRun(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "nexus.db"))
	if err != nil {
		t.Fatalf("open migration database: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("set goose dialect: %v", err)
	}
	dir := automationMigrationDir(t)
	if err = goose.UpTo(db, dir, 120); err != nil {
		t.Fatalf("migrate to pre-attempt schema: %v", err)
	}
	insertTask := func(jobID string) {
		t.Helper()
		if _, insertErr := db.Exec(`INSERT INTO automation_scheduled_tasks (
    job_id, owner_user_id, name, agent_id, schedule_kind, interval_seconds,
    timezone, instruction, execution_kind, permission_mode,
    session_target_kind, wake_mode, delivery_mode, source_kind,
    overlap_policy, enabled, permission_policy_json,
    permission_policy_revision, permission_state
) VALUES (?, 'owner-legacy', ?, 'agent-legacy', 'every', 3600,
          'UTC', 'legacy task', 'agent', 'default',
          'isolated', 'next-heartbeat', 'none', 'system',
          'skip', TRUE, '{"version":1,"revision":1,"grants":[]}', 1, 'ready')`,
			jobID, jobID,
		); insertErr != nil {
			t.Fatalf("insert legacy task %s: %v", jobID, insertErr)
		}
	}
	insertTask("legacy-with-terminal")
	insertTask("legacy-without-authority")
	for _, run := range []struct {
		runID      string
		jobID      string
		owner      string
		status     string
		finishedAt any
	}{
		{runID: "run-terminal-old", jobID: "legacy-with-terminal", owner: "owner-legacy", status: "succeeded", finishedAt: "2026-08-27T08:00:00Z"},
		{runID: "run-terminal-a", jobID: "legacy-with-terminal", owner: "owner-legacy", status: "failed", finishedAt: "2026-08-27T09:00:00Z"},
		{runID: "run-terminal-z", jobID: "legacy-with-terminal", owner: "owner-legacy", status: "cancelled", finishedAt: "2026-08-27T09:00:00Z"},
		{runID: "run-skipped-newer", jobID: "legacy-with-terminal", owner: "owner-legacy", status: "skipped", finishedAt: "2026-08-27T10:00:00Z"},
		{runID: "run-active", jobID: "legacy-with-terminal", owner: "owner-legacy", status: "running", finishedAt: nil},
		{runID: "run-wrong-owner", jobID: "legacy-with-terminal", owner: "other-owner", status: "succeeded", finishedAt: "2026-08-27T11:00:00Z"},
		{runID: "run-only-skipped", jobID: "legacy-without-authority", owner: "owner-legacy", status: "skipped", finishedAt: "2026-08-27T12:00:00Z"},
	} {
		if _, err = db.Exec(`INSERT INTO automation_task_runs (
    run_id, job_id, owner_user_id, status, trigger_kind, attempts, finished_at
) VALUES (?, ?, ?, ?, 'manual', 1, ?)`, run.runID, run.jobID, run.owner, run.status, run.finishedAt); err != nil {
			t.Fatalf("insert legacy run %s: %v", run.runID, err)
		}
	}
	if err = goose.UpTo(db, dir, 122); err != nil {
		t.Fatalf("apply delivery attempt migration: %v", err)
	}
	var latest sql.NullString
	if err = db.QueryRow(`SELECT last_completed_run_id FROM automation_scheduled_tasks
WHERE owner_user_id = 'owner-legacy' AND job_id = 'legacy-with-terminal'`).Scan(&latest); err != nil {
		t.Fatalf("read backfilled authority: %v", err)
	}
	if !latest.Valid || latest.String != "run-terminal-z" {
		t.Fatalf("last completed authority = %+v, want exact latest terminal tie-break", latest)
	}
	if err = db.QueryRow(`SELECT last_completed_run_id FROM automation_scheduled_tasks
WHERE owner_user_id = 'owner-legacy' AND job_id = 'legacy-without-authority'`).Scan(&latest); err != nil {
		t.Fatalf("read task without terminal authority: %v", err)
	}
	if latest.Valid {
		t.Fatalf("skipped-only legacy task must remain without delivery authority: %+v", latest)
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
	for _, column := range []string{"deletion_state", "deletion_token", "deletion_claimed_at"} {
		rows, queryErr := db.Query("PRAGMA table_info(automation_scheduled_tasks)")
		if queryErr != nil {
			t.Fatalf("读取 automation_scheduled_tasks schema: %v", queryErr)
		}
		found := false
		for rows.Next() {
			var cid, notNull, primaryKey int
			var name, columnType string
			var defaultValue sql.NullString
			if scanErr := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); scanErr != nil {
				_ = rows.Close()
				t.Fatalf("扫描 automation_scheduled_tasks schema: %v", scanErr)
			}
			found = found || name == column
		}
		_ = rows.Close()
		if !found {
			t.Fatalf("automation_scheduled_tasks.%s missing", column)
		}
	}
	for _, target := range []struct {
		table  string
		column string
	}{
		{table: "automation_task_runs", column: "delivery_attempt_id"},
		{table: "automation_task_runs", column: "delivery_attempt_started_at"},
		{table: "automation_scheduled_tasks", column: "last_completed_run_id"},
	} {
		rows, queryErr := db.Query("PRAGMA table_info(" + target.table + ")")
		if queryErr != nil {
			t.Fatalf("读取 %s schema: %v", target.table, queryErr)
		}
		found := false
		for rows.Next() {
			var cid, notNull, primaryKey int
			var name, columnType string
			var defaultValue sql.NullString
			if scanErr := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); scanErr != nil {
				_ = rows.Close()
				t.Fatalf("扫描 %s schema: %v", target.table, scanErr)
			}
			found = found || name == target.column
		}
		_ = rows.Close()
		if !found {
			t.Fatalf("%s.%s missing", target.table, target.column)
		}
	}
	var dueIndexSQL string
	if err = db.QueryRow(`SELECT sql FROM sqlite_master
WHERE type = 'index' AND name = 'idx_automation_task_runs_delivery_due'`).Scan(&dueIndexSQL); err != nil {
		t.Fatalf("读取 delivery due partial index: %v", err)
	}
	normalizedIndexSQL := strings.ToLower(strings.Join(strings.Fields(dueIndexSQL), " "))
	for _, fragment := range []string{
		"delivery_status, delivery_next_attempt_at, updated_at, run_id",
		"where delivery_dead_letter_at is null",
		"delivery_status in ('pending', 'failed')",
	} {
		if !strings.Contains(normalizedIndexSQL, fragment) {
			t.Fatalf("delivery due index 缺少 %q: %s", fragment, dueIndexSQL)
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

func TestSQLiteAutomationDeliveryRouteMigrationPreservesPersonalWeixinContext(t *testing.T) {
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
		t.Fatalf("迁移到旧投递路由 schema 失败: %v", err)
	}
	if _, err = db.Exec(`
INSERT INTO automation_delivery_routes (
    route_id, agent_id, session_key, mode, channel, "to", thread_id, enabled
) VALUES (
    'route-weixin', 'agent-weixin', 'agent:agent-weixin:weixin-personal:dm:user-1',
    'explicit', 'weixin-personal', 'user-1', 'legacy-context-token', TRUE
)`); err != nil {
		t.Fatalf("写入旧个人微信路由失败: %v", err)
	}

	if err = goose.UpTo(db, dir, 93); err != nil {
		t.Fatalf("执行投递路由 migration 失败: %v", err)
	}
	var runDeliverySnapshotColumn string
	if err = db.QueryRow(`
SELECT name
FROM pragma_table_info('automation_task_runs')
WHERE name = 'delivery_target_json'`).Scan(&runDeliverySnapshotColumn); err != nil {
		t.Fatalf("run 投递快照字段未迁移: %v", err)
	}
	var threadID sql.NullString
	var contextToken sql.NullString
	if err = db.QueryRow(`
SELECT thread_id, context_token
FROM automation_delivery_routes
WHERE route_id = 'route-weixin'`).Scan(&threadID, &contextToken); err != nil {
		t.Fatalf("读取迁移后的个人微信路由失败: %v", err)
	}
	if threadID.Valid || !contextToken.Valid || contextToken.String != "legacy-context-token" {
		t.Fatalf("旧 context token 未迁移到独立字段: thread=%+v context=%+v", threadID, contextToken)
	}

	if err = goose.DownTo(db, dir, 86); err != nil {
		t.Fatalf("回滚投递路由 migration 失败: %v", err)
	}
	if err = db.QueryRow(`
SELECT thread_id
FROM automation_delivery_routes
WHERE route_id = 'route-weixin'`).Scan(&threadID); err != nil {
		t.Fatalf("读取回滚后的个人微信路由失败: %v", err)
	}
	if !threadID.Valid || threadID.String != "legacy-context-token" {
		t.Fatalf("回滚后旧 context token 未恢复: %+v", threadID)
	}
}

func TestSQLiteAutomationTaskModelCompatibilityMigrationPreservesLegacyTasks(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "nexus.db"))
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("设置 goose 方言失败: %v", err)
	}
	dir := automationMigrationDir(t)
	if err = goose.UpTo(db, dir, 93); err != nil {
		t.Fatalf("迁移到旧任务模型失败: %v", err)
	}

	insertTask := func(
		jobID string,
		deliveryMode string,
		deliveryChannel any,
		deliveryTo any,
		deliverySessionKey any,
		sourceSessionKey any,
	) {
		t.Helper()
		_, insertErr := db.Exec(`
INSERT INTO automation_scheduled_tasks (
    job_id, owner_user_id, name, agent_id,
    schedule_kind, interval_seconds, timezone, instruction, execution_kind,
    session_target_kind, wake_mode,
    delivery_mode, delivery_channel, delivery_to, delivery_session_key,
    source_kind, source_session_key, overlap_policy, enabled,
    permission_policy_json, permission_policy_revision, permission_state
) VALUES (?, 'owner-1', ?, 'agent-1',
          'every', 3600, 'Asia/Shanghai', ?, 'agent',
          'isolated', 'next-heartbeat',
          ?, ?, ?, ?,
          'user_page', ?, 'skip', TRUE,
          '{"version":1,"revision":7,"grants":[]}', 7, 'ready')`,
			jobID,
			"历史任务 "+jobID,
			"保留历史指令 "+jobID,
			deliveryMode,
			deliveryChannel,
			deliveryTo,
			deliverySessionKey,
			sourceSessionKey,
		)
		if insertErr != nil {
			t.Fatalf("写入历史任务 %s 失败: %v", jobID, insertErr)
		}
	}

	legacyExternal := "agent:agent-1:weixin-personal:dm:acct:account-old:contact-old"
	legacyWeb := "agent:agent-1:ws:dm:web-chat"
	legacySource := "agent:agent-1:fs:dm:acct:account-fs:chat-fs"
	currentExternal := "agent:agent-1:tg:dm:acct:account-tg:chat-tg"
	insertTask("legacy-external", "explicit", "websocket", legacyExternal, nil, nil)
	insertTask("legacy-web", "explicit", "websocket", legacyWeb, nil, nil)
	insertTask("legacy-last", "last", nil, nil, nil, legacySource)
	insertTask("current", "last", nil, nil, currentExternal, nil)

	if err = goose.UpTo(db, dir, 94); err != nil {
		t.Fatalf("执行旧任务兼容 migration 失败: %v", err)
	}

	assertTask := func(
		jobID string,
		wantMode string,
		wantChannel any,
		wantTo any,
		wantSessionKey string,
	) {
		t.Helper()
		var (
			name             string
			instruction      string
			mode             string
			channel          sql.NullString
			to               sql.NullString
			sessionKey       sql.NullString
			intervalSeconds  int
			enabled          bool
			permissionPolicy string
			permissionMode   string
		)
		if queryErr := db.QueryRow(`
SELECT name, instruction, delivery_mode, delivery_channel, delivery_to,
       delivery_session_key, interval_seconds, enabled,
       permission_policy_json, permission_mode
FROM automation_scheduled_tasks
WHERE job_id = ?`, jobID).Scan(
			&name,
			&instruction,
			&mode,
			&channel,
			&to,
			&sessionKey,
			&intervalSeconds,
			&enabled,
			&permissionPolicy,
			&permissionMode,
		); queryErr != nil {
			t.Fatalf("读取迁移任务 %s 失败: %v", jobID, queryErr)
		}
		if name != "历史任务 "+jobID || instruction != "保留历史指令 "+jobID ||
			intervalSeconds != 3600 || !enabled || permissionMode != "default" ||
			permissionPolicy != `{"version":1,"revision":7,"grants":[]}` {
			t.Fatalf("任务 %s 的用户配置在迁移中被修改", jobID)
		}
		if mode != wantMode || sessionKey.String != wantSessionKey {
			t.Fatalf("任务 %s 路由 = mode:%s session:%+v", jobID, mode, sessionKey)
		}
		if wantChannel == nil {
			if channel.Valid {
				t.Fatalf("任务 %s channel 应为空: %+v", jobID, channel)
			}
		} else if channel.String != wantChannel.(string) {
			t.Fatalf("任务 %s channel = %q, want %q", jobID, channel.String, wantChannel)
		}
		if wantTo == nil {
			if to.Valid {
				t.Fatalf("任务 %s to 应为空: %+v", jobID, to)
			}
		} else if to.String != wantTo.(string) {
			t.Fatalf("任务 %s to = %q, want %q", jobID, to.String, wantTo)
		}
	}

	assertTask("legacy-external", "last", nil, nil, legacyExternal)
	assertTask("legacy-web", "explicit", "websocket", legacyWeb, legacyWeb)
	assertTask("legacy-last", "last", nil, nil, legacySource)
	assertTask("current", "last", nil, nil, currentExternal)
}

func TestSQLiteAutomationDeliveryGrantMigrationCopiesLegacyProvenance(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "nexus.db"))
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("设置 goose 方言失败: %v", err)
	}
	dir := automationMigrationDir(t)
	if err = goose.UpTo(db, dir, 95); err != nil {
		t.Fatalf("迁移到旧 delivery grant schema 失败: %v", err)
	}
	_, err = db.Exec(`
INSERT INTO automation_scheduled_tasks (
    job_id, owner_user_id, name, agent_id,
    schedule_kind, interval_seconds, timezone, instruction,
    session_target_kind, wake_mode, delivery_mode,
    source_kind, source_creator_agent_id, source_context_type,
    source_context_id, source_context_label, source_session_key,
    source_session_label, enabled
) VALUES (
    'legacy-agent-task', 'owner-1', '旧 Agent 任务', 'agent-1',
    'every', 3600, 'Asia/Shanghai', '保持配置',
    'isolated', 'next-heartbeat', 'none',
    'agent', 'agent-1', 'agent',
    'agent-1', 'Agent One', 'agent:agent-1:internal:dm:operator',
    '创建会话', FALSE
)`)
	if err != nil {
		t.Fatalf("写入 migration 前任务失败: %v", err)
	}
	if err = goose.UpTo(db, dir, 96); err != nil {
		t.Fatalf("执行 delivery grant migration 失败: %v", err)
	}
	var grantJSON string
	if err = db.QueryRow(`
SELECT delivery_grant_json
FROM automation_scheduled_tasks
WHERE job_id = 'legacy-agent-task'`).Scan(&grantJSON); err != nil {
		t.Fatalf("读取 delivery grant 失败: %v", err)
	}
	for _, fragment := range []string{
		`"kind":"agent"`,
		`"creator_agent_id":"agent-1"`,
		`"context_label":"Agent One"`,
		`"session_label":"创建会话"`,
	} {
		if !strings.Contains(grantJSON, fragment) {
			t.Fatalf("delivery grant 未复制 legacy provenance %s: %s", fragment, grantJSON)
		}
	}
}

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
