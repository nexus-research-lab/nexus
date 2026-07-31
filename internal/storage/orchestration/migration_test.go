package orchestration

import (
	"database/sql"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

func TestExecutionOrchestrationMigrationEnforcesOneExecutionChain(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "orchestration.db")
	migrationDB, err := sql.Open("sqlite", databasePath+"?_pragma=foreign_keys(0)")
	if err != nil {
		t.Fatal(err)
	}
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	migrationDir := orchestrationMigrationDir(t, "sqlite")
	if err = goose.Up(migrationDB, migrationDir); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	if err = migrationDB.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", databasePath+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	var foreignKeys int
	if err = db.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, want 1", foreignKeys)
	}

	insertExecutionChain(t, db, "1")
	insertExecutionChain(t, db, "2")

	if _, err = db.Exec(`
INSERT INTO execution_plan_dependencies (
    plan_id, execution_id, work_item_id, depends_on_work_item_id
) VALUES ('plan-1', 'execution-1', 'work-1', 'work-2')
`); err == nil {
		t.Fatal("cross-Execution dependency should violate the Plan membership foreign key")
	}
	if _, err = db.Exec(`
INSERT INTO execution_attempts (
    attempt_id, execution_id, plan_id, work_item_id, spec_id, assignment_id,
    executor_kind, executor_agent_id, status
) VALUES (
    'attempt-cross', 'execution-2', 'plan-2', 'work-2', 'spec-2', 'assignment-1',
    'agent', 'agent-1', 'pending'
)
`); err == nil {
		t.Fatal("cross-Execution Assignment/Attempt chain should violate the composite foreign key")
	}

	rows, err := db.Query("PRAGMA foreign_key_check")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		var table string
		var rowID int64
		var parent string
		var fkID int
		if err = rows.Scan(&table, &rowID, &parent, &fkID); err != nil {
			t.Fatal(err)
		}
		t.Fatalf("foreign key violation: table=%s row=%d parent=%s fk=%d", table, rowID, parent, fkID)
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestExecutionOrchestrationMigrationCanRollbackAndReapply(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "orchestration-roundtrip.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	migrationDir := orchestrationMigrationDir(t, "sqlite")
	if err = goose.Up(db, migrationDir); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	if err = goose.DownTo(db, migrationDir, 59); err != nil {
		t.Fatalf("roll back orchestration migration: %v", err)
	}
	if err = goose.UpTo(db, migrationDir, 60); err != nil {
		t.Fatalf("reapply orchestration migration: %v", err)
	}
}

func insertExecutionChain(t *testing.T, db *sql.DB, suffix string) {
	t.Helper()
	statements := []string{
		`INSERT INTO executions (
    execution_id, owner_user_id, session_key, scope_kind, origin, objective, status
) VALUES (
    'execution-` + suffix + `', 'owner-1', 'agent:nexus:workspace:dm:thread-` + suffix + `',
    'dm', 'user_request', 'deliver result ` + suffix + `', 'active'
)`,
		`INSERT INTO execution_plan_revisions (
    plan_id, execution_id, revision, status
) VALUES ('plan-` + suffix + `', 'execution-` + suffix + `', 1, 'active')`,
		`INSERT INTO execution_work_items (
    work_item_id, execution_id, logical_key, kind
) VALUES ('work-` + suffix + `', 'execution-` + suffix + `', 'deliver', 'produce')`,
		`INSERT INTO execution_work_item_specs (
    spec_id, work_item_id, execution_id, spec_version, subject, objective,
    deliverable, acceptance_criteria_json, spec_hash
) VALUES (
    'spec-` + suffix + `', 'work-` + suffix + `', 'execution-` + suffix + `', 1,
    'Deliver', 'Produce result', 'A verified result', '["verified"]', 'hash-` + suffix + `'
)`,
		`INSERT INTO execution_plan_items (
    plan_id, execution_id, work_item_id, spec_id, is_required, is_terminal
) VALUES (
    'plan-` + suffix + `', 'execution-` + suffix + `', 'work-` + suffix + `',
    'spec-` + suffix + `', TRUE, TRUE
)`,
		`INSERT INTO execution_work_item_states (
    work_item_id, execution_id, current_spec_id, status
) VALUES ('work-` + suffix + `', 'execution-` + suffix + `', 'spec-` + suffix + `', 'open')`,
		`INSERT INTO execution_work_assignments (
    assignment_id, execution_id, plan_id, work_item_id, spec_id,
    owner_agent_id, return_to_agent_id, strategy, status
) VALUES (
    'assignment-` + suffix + `', 'execution-` + suffix + `', 'plan-` + suffix + `',
    'work-` + suffix + `', 'spec-` + suffix + `', 'agent-` + suffix + `',
    'reviewer-` + suffix + `', 'self', 'active'
)`,
		`INSERT INTO execution_attempts (
    attempt_id, execution_id, plan_id, work_item_id, spec_id, assignment_id,
    executor_kind, executor_agent_id, status
) VALUES (
    'attempt-` + suffix + `', 'execution-` + suffix + `', 'plan-` + suffix + `',
    'work-` + suffix + `', 'spec-` + suffix + `', 'assignment-` + suffix + `',
    'agent', 'agent-` + suffix + `', 'succeeded'
)`,
		`INSERT INTO execution_submissions (
    submission_id, execution_id, plan_id, work_item_id, spec_id, assignment_id,
    attempt_id, submission_sequence, submitter_agent_id, result_summary
) VALUES (
    'submission-` + suffix + `', 'execution-` + suffix + `', 'plan-` + suffix + `',
    'work-` + suffix + `', 'spec-` + suffix + `', 'assignment-` + suffix + `',
    'attempt-` + suffix + `', 1, 'agent-` + suffix + `', 'done'
)`,
		`INSERT INTO execution_acceptances (
    acceptance_id, execution_id, plan_id, work_item_id, spec_id, assignment_id,
    submission_id, decision, reviewer_kind, reviewer_id
) VALUES (
    'acceptance-` + suffix + `', 'execution-` + suffix + `', 'plan-` + suffix + `',
    'work-` + suffix + `', 'spec-` + suffix + `', 'assignment-` + suffix + `',
    'submission-` + suffix + `', 'accepted', 'agent', 'reviewer-` + suffix + `'
)`,
		`INSERT INTO execution_events (
    event_id, execution_id, sequence, command_id, event_type, entity_type,
    entity_id, entity_version, actor_kind
) VALUES (
    'event-` + suffix + `', 'execution-` + suffix + `', 1, 'command-` + suffix + `',
    'execution_created', 'execution', 'execution-` + suffix + `', 1, 'system'
)`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("insert orchestration chain %s: %v\n%s", suffix, err, statement)
		}
	}
}

func orchestrationMigrationDir(t *testing.T, dialect string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate orchestration migration test")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations", dialect)
}
