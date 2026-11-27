// repository_test.go 覆盖 Goal 仓储基础操作与续跑计划测试。
package goal

import (
	"database/sql"
	"os"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	_ "modernc.org/sqlite"
)

func newTestRepository(t *testing.T) *Repository {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	applyGoalMigration(t, db)
	return NewRepository(config.Config{DatabaseDriver: "sqlite"}, db)
}

func applyGoalMigration(t *testing.T, db *sql.DB) {
	t.Helper()
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
		"../../../db/migrations/sqlite/00104_goal_continuation_plans.sql",
	)
}

func applyGoalMigrationFiles(t *testing.T, db *sql.DB, paths ...string) {
	t.Helper()
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		upSQL := strings.Split(string(body), "-- +goose Down")[0]
		upSQL = strings.ReplaceAll(upSQL, "-- +goose Up", "")
		for _, statement := range strings.Split(upSQL, ";") {
			statement = strings.TrimSpace(statement)
			if statement == "" {
				continue
			}
			if _, err := db.Exec(statement); err != nil {
				t.Fatalf("exec migration %s statement %q: %v", path, statement, err)
			}
		}
	}
}
