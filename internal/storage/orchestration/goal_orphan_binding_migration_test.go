// INPUT: legacy Goal rows whose pending reverse binding may or may not have an exact Execution counterpart.
// OUTPUT: migration proof that only unbound half-commits become Goal-only and exact bilateral truth is preserved.
// POS: cross-domain create_goal compatibility regression; current code no longer creates this state.
package orchestration

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pressly/goose/v3"
)

func TestGoalOnlyOrphanBindingMigrationReleasesOnlyUnboundHalfCommit(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "goal-orphan-binding.db")
	db, err := sql.Open("sqlite", databasePath+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err = goose.UpTo(db, orchestrationMigrationDir(t, "sqlite"), 104); err != nil {
		t.Fatal(err)
	}

	type fixture struct {
		goalID          string
		executionID     string
		executionGoalID any
		wantMode        string
		wantExecutionID any
		wantState       any
	}
	fixtures := []fixture{
		{
			goalID:          "goal-orphan-unbound",
			executionID:     "execution-orphan-unbound",
			executionGoalID: nil,
			wantMode:        "goal_only",
			wantExecutionID: nil,
			wantState:       nil,
		},
		{
			goalID:          "goal-exact-bound",
			executionID:     "execution-exact-bound",
			executionGoalID: "goal-exact-bound",
			wantMode:        "managed",
			wantExecutionID: "execution-exact-bound",
			wantState:       "pending",
		},
	}
	for _, item := range fixtures {
		sessionKey := "agent:nexus:workspace:dm:" + item.goalID
		metadata := `{"objective_revision":2,"execution_mode":"managed",` +
			`"execution_id":"` + item.executionID + `","execution_binding_state":"pending",` +
			`"completion_criteria":["verified"],"promotion_command":"legacy-command",` +
			`"activation_origin":"user_explicit","activation_reason":"persistence_requested"}`
		if _, err = db.Exec(`
INSERT INTO session_goals (goal_id, session_key, objective, status, metadata_json)
VALUES (?, ?, 'legacy objective', 'active', ?)`, item.goalID, sessionKey, metadata); err != nil {
			t.Fatal(err)
		}
		if item.executionGoalID == nil {
			_, err = db.Exec(`
INSERT INTO executions (
    execution_id, owner_user_id, session_key, scope_kind, coordinator_agent_id,
    origin, objective, completion_criteria_json, status, version, metadata_json
) VALUES (?, 'owner-1', ?, 'dm', 'agent-lead', 'user_request',
          'legacy objective', '["verified"]', 'active', 1, '{}')`, item.executionID, sessionKey)
		} else {
			_, err = db.Exec(`
INSERT INTO executions (
    execution_id, owner_user_id, session_key, scope_kind, coordinator_agent_id,
    origin, objective, completion_criteria_json, goal_id,
    goal_objective_revision, goal_activation_origin, goal_activation_reason,
    status, version, metadata_json
) VALUES (?, 'owner-1', ?, 'dm', 'agent-lead', 'user_request',
          'legacy objective', '["verified"]', ?, 2, 'user_explicit',
          'persistence_requested', 'active', 1, '{}')`,
				item.executionID, sessionKey, item.executionGoalID)
		}
		if err != nil {
			t.Fatal(err)
		}
	}

	if _, err = db.Exec(`
INSERT INTO session_goals (goal_id, session_key, objective, status, metadata_json)
VALUES (
    'goal-orphan-missing', 'agent:nexus:workspace:dm:goal-orphan-missing',
    'legacy objective', 'active',
    '{"objective_revision":1,"execution_mode":"managed","execution_id":"execution-missing","execution_binding_state":"pending","completion_criteria":["verified"]}'
)`); err != nil {
		t.Fatal(err)
	}

	if err = goose.Up(db, orchestrationMigrationDir(t, "sqlite")); err != nil {
		t.Fatal(err)
	}

	for _, item := range fixtures {
		var mode string
		var executionID, state sql.NullString
		if err = db.QueryRow(`
SELECT
    json_extract(metadata_json, '$.execution_mode'),
    json_extract(metadata_json, '$.execution_id'),
    json_extract(metadata_json, '$.execution_binding_state')
FROM session_goals
WHERE goal_id = ?`, item.goalID).Scan(&mode, &executionID, &state); err != nil {
			t.Fatal(err)
		}
		if mode != item.wantMode || nullableString(executionID) != item.wantExecutionID ||
			nullableString(state) != item.wantState {
			t.Fatalf(
				"Goal %s binding after migration = mode:%q execution:%#v state:%#v",
				item.goalID,
				mode,
				nullableString(executionID),
				nullableString(state),
			)
		}
	}
	var missingMode string
	var missingExecutionID sql.NullString
	if err = db.QueryRow(`
SELECT json_extract(metadata_json, '$.execution_mode'),
       json_extract(metadata_json, '$.execution_id')
FROM session_goals
WHERE goal_id = 'goal-orphan-missing'`).Scan(&missingMode, &missingExecutionID); err != nil {
		t.Fatal(err)
	}
	if missingMode != "goal_only" || missingExecutionID.Valid {
		t.Fatalf("missing Execution recovery = mode:%q execution:%#v", missingMode, missingExecutionID)
	}
	var unboundGoalID sql.NullString
	if err = db.QueryRow(`
SELECT goal_id FROM executions WHERE execution_id = 'execution-orphan-unbound'`).Scan(&unboundGoalID); err != nil {
		t.Fatal(err)
	}
	if unboundGoalID.Valid {
		t.Fatalf("unbound WorkGraph was mutated: goal_id=%q", unboundGoalID.String)
	}
}

func TestGoalOnlyOrphanBindingMigrationDefinesBothDialects(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		payload, err := os.ReadFile(filepath.Join(
			orchestrationMigrationDir(t, dialect),
			"00105_goal_only_orphan_binding_recovery.sql",
		))
		if err != nil {
			t.Fatal(err)
		}
		text := string(payload)
		for _, required := range []string{
			"execution_binding_state",
			"goal_only",
			"NOT EXISTS",
			"execution.goal_id = goal.goal_id",
			"execution.goal_objective_revision",
		} {
			if !strings.Contains(text, required) {
				t.Fatalf("%s migration missing %q", dialect, required)
			}
		}
	}
}

func nullableString(value sql.NullString) any {
	if !value.Valid {
		return nil
	}
	return value.String
}
