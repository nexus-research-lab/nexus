// INPUT: 迁移到 00168 并带 heartbeat.wake 与主会话事件的 SQLite 数据库。
// OUTPUT: 验证 00169 删除 Heartbeat 表、wake 行和 00125 列/索引，保留主会话事件，并可回滚重放。
// POS: Heartbeat 退役迁移的回归测试。
package migration

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"
)

func TestRetireHeartbeatMigrationDropsSchemaAndRollsBack(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "host.db"))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	dir := providerRecoveryMigrationDir(t)
	if err = goose.UpTo(db, dir, 168); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`
INSERT INTO automation_system_events (event_id, event_type, source_type, source_id, payload, status, request_id)
VALUES ('wake-1', 'heartbeat.wake', 'agent', 'agent-a', '{}', 'new', 'req-1'),
       ('task-1', 'scheduled_task.trigger', 'agent', 'agent-a', '{}', 'new', NULL)`); err != nil {
		t.Fatal(err)
	}

	if err = goose.UpTo(db, dir, 169); err != nil {
		t.Fatal(err)
	}
	var events []string
	rows, err := db.Query(`SELECT event_id FROM automation_system_events ORDER BY event_id`)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		events = append(events, id)
	}
	_ = rows.Close()
	if len(events) != 1 || events[0] != "task-1" {
		t.Fatalf("events after 00169 = %v, want [task-1]", events)
	}
	assertRetiredHeartbeatSchema(t, db, false)

	if err = goose.DownTo(db, dir, 168); err != nil {
		t.Fatal(err)
	}
	assertRetiredHeartbeatSchema(t, db, true)
	if err = goose.Up(db, dir); err != nil {
		t.Fatal(err)
	}
	assertRetiredHeartbeatSchema(t, db, false)
}

func assertRetiredHeartbeatSchema(t *testing.T, db *sql.DB, want bool) {
	t.Helper()
	var objects int
	if err := db.QueryRow(`
SELECT COUNT(*) FROM sqlite_master
WHERE name IN ('automation_heartbeat_states', 'uq_automation_heartbeat_wake_request', 'idx_automation_heartbeat_wake_due')`).Scan(&objects); err != nil {
		t.Fatal(err)
	}
	var columns int
	if err := db.QueryRow(`
SELECT COUNT(*) FROM pragma_table_info('automation_system_events')
WHERE name IN ('owner_user_id', 'request_id', 'intent_digest', 'accepted_configuration_version', 'claim_token', 'claim_expires_at')`).Scan(&columns); err != nil {
		t.Fatal(err)
	}
	wantObjects, wantColumns := 0, 0
	if want {
		wantObjects, wantColumns = 3, 6
	}
	if objects != wantObjects || columns != wantColumns {
		t.Fatalf("heartbeat schema objects=%d columns=%d, want %d/%d", objects, columns, wantObjects, wantColumns)
	}
}
