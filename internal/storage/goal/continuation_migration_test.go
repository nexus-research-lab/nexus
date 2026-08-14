package goal

import (
	"database/sql"
	"os"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestGoalContinuationMigrationRefundsOnlyOutstandingOpaqueLegacyReservations(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	applyGoalMigrationFiles(t, db,
		"../../../db/migrations/sqlite/00025_session_goals.sql",
		"../../../db/migrations/sqlite/00026_goal_codex_statuses.sql",
		"../../../db/migrations/sqlite/00027_goal_budget_token_total.sql",
		"../../../db/migrations/sqlite/00028_goal_remove_cleared_status.sql",
		"../../../db/migrations/sqlite/00037_session_goals_compat.sql",
		"../../../db/migrations/sqlite/00051_goal_token_totals.sql",
		"../../../db/migrations/sqlite/00052_goal_usage_source_checkpoints.sql",
		"../../../db/migrations/sqlite/00053_goal_usage_source_round_pending.sql",
		"../../../db/migrations/sqlite/00054_goal_usage_finalization.sql",
		"../../../db/migrations/sqlite/00055_goal_usage_source_baseline.sql",
	)
	fixtures := []struct {
		goalID   string
		count    int
		metadata string
		want     int
	}{
		{
			goalID: "goal-legacy-mixed", count: 5, want: 3,
			metadata: `{"continuation_reservation_round_ids":["round-a"," round-b ","round-a","",null,7,{"id":"round-c"}],"keep":"yes"}`,
		},
		{
			goalID: "goal-legacy-clamped", count: 1, want: 0,
			metadata: `{"continuation_reservation_round_ids":["round-a","round-b"],"keep":"yes"}`,
		},
		{
			goalID: "goal-legacy-malformed-elements", count: 4, want: 4,
			metadata: `{"continuation_reservation_round_ids":[null,false,8,{},[],"   "],"keep":"yes"}`,
		},
	}
	for _, fixture := range fixtures {
		if _, err = db.Exec(`INSERT INTO session_goals (
        goal_id, session_key, objective, status, continuation_count,
        version, created_at, updated_at, metadata_json
    ) VALUES (?, ?, ?, 'active', 3, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, ?)`,
			fixture.goalID, "agent:nexus:ws:dm:"+fixture.goalID, "recover legacy",
			fixture.metadata,
		); err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(`UPDATE session_goals SET continuation_count = ? WHERE goal_id = ?`, fixture.count, fixture.goalID); err != nil {
			t.Fatal(err)
		}
	}
	applyGoalMigrationFiles(t, db, "../../../db/migrations/sqlite/00104_goal_continuation_plans.sql")
	for _, fixture := range fixtures {
		var count, version int
		var metadata string
		if err = db.QueryRow(`SELECT continuation_count, version, metadata_json FROM session_goals WHERE goal_id = ?`, fixture.goalID).Scan(&count, &version, &metadata); err != nil {
			t.Fatal(err)
		}
		if count != fixture.want || version != 2 || strings.Contains(metadata, "continuation_reservation_round_ids") || !strings.Contains(metadata, `"keep":"yes"`) {
			t.Fatalf("legacy repair %s count=%d want=%d version=%d metadata=%s", fixture.goalID, count, fixture.want, version, metadata)
		}
	}
}

func TestGoalContinuationMigrationHasDialectSafeLegacyRepair(t *testing.T) {
	postgresBody, err := os.ReadFile("../../../db/migrations/postgres/00104_goal_continuation_plans.sql")
	if err != nil {
		t.Fatal(err)
	}
	sqlText := string(postgresBody)
	for _, required := range []string{
		"CREATE TABLE goal_continuation_plans",
		"metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb",
		"CREATE UNIQUE INDEX uq_goal_continuation_plans_open_goal_revision",
		"WHERE status IN ('scheduled', 'claimed', 'started')",
		"CHECK (status IN ('scheduled', 'claimed', 'started', 'settled', 'released', 'cancelled'))",
		"jsonb_typeof(goal.metadata_json -> 'continuation_reservation_round_ids') = 'array'",
		"GREATEST(",
		"COUNT(DISTINCT btrim(reservation.value #>> '{}'))::integer",
		"jsonb_typeof(reservation.value) = 'string'",
		"metadata_json = goal.metadata_json - 'continuation_reservation_round_ids'",
	} {
		if !strings.Contains(sqlText, required) {
			t.Fatalf("Postgres migration missing %q", required)
		}
	}
	if strings.Contains(sqlText, "metadata_json = (goal.metadata_json::jsonb - 'continuation_reservation_round_ids')::text") {
		t.Fatal("Postgres migration assigns text into JSONB metadata_json")
	}
}
