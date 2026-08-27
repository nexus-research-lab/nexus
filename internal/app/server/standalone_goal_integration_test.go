// INPUT: 真实 Goal/Execution SQL repository、应用层 explicit Goal coordinator 与 reserved Goal 生命周期。
// OUTPUT: 未物化 WorkGraph 时 create/retarget/complete 保持 Goal-only，且不会误入 Execution completion audit。
// POS: Goal 与 WorkGraph 生产装配边界的组合回归测试。
package server

import (
	"context"
	"database/sql"
	"testing"

	servergoal "github.com/nexus-research-lab/nexus/internal/app/server/goal"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	goalstore "github.com/nexus-research-lab/nexus/internal/storage/goal"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

func TestStandaloneExplicitGoalRetargetAndCompleteWithoutWorkGraph(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	cfg.GoalEnabled = true
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)
	db, err := OpenDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()

	goals := goalsvc.NewService(cfg, goalstore.NewRepository(cfg, db))
	executions := orchestrationsvc.NewService(orchestrationstore.NewRepository(cfg, db))
	coordinator := servergoal.NewExplicitExecutionCoordinator(goals, executions)
	goals.SetObjectiveRetargetCoordinator(coordinator)
	goals.SetExecutionGoalCompletionReadiness(servergoal.NewExecutionCompletionReadiness(executions))

	const (
		sessionKey = "agent:agent-1:ws:dm:standalone-goal-integration"
		agentID    = "agent-1"
	)
	created, err := coordinator.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey:  sessionKey,
		Objective:   "Draft the standalone report",
		CreatedBy:   "model",
		RoundID:     "round-create",
		OwnerUserID: "owner-1",
		AgentID:     agentID,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertStandaloneGoalWithoutExecution(t, db, *created)

	retargeted, err := coordinator.RetargetByModel(
		context.Background(),
		sessionKey,
		protocol.RetargetGoalRequest{
			Objective:                 "Draft and verify the standalone report",
			RoundID:                   "round-retarget",
			AgentID:                   agentID,
			ExpectedGoalID:            created.ID,
			ExpectedObjectiveRevision: created.ObjectiveRevision(),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if retargeted.ID != created.ID || retargeted.ObjectiveRevision() != 2 {
		t.Fatalf("retargeted Goal = %#v", retargeted)
	}
	assertStandaloneGoalWithoutExecution(t, db, *retargeted)

	completed, err := coordinator.CompleteByModel(
		context.Background(),
		retargeted.ID,
		protocol.CompleteGoalRequest{
			Summary:                   "Standalone report verified",
			RoundID:                   "round-complete",
			AgentID:                   agentID,
			ExpectedObjectiveRevision: retargeted.ObjectiveRevision(),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != protocol.GoalStatusComplete {
		t.Fatalf("Goal status = %q, want complete", completed.Status)
	}
	assertNoExecutionForGoal(t, db, completed.ID)
}

func assertStandaloneGoalWithoutExecution(
	t *testing.T,
	db *sql.DB,
	goal protocol.Goal,
) {
	t.Helper()
	if state := protocol.GoalExecutionBindingStateFromGoal(goal); state !=
		protocol.GoalExecutionBindingStateStandalone {
		t.Fatalf("Goal binding state = %q, want standalone", state)
	}
	assertNoExecutionForGoal(t, db, goal.ID)
}

func assertNoExecutionForGoal(
	t *testing.T,
	db *sql.DB,
	goalID string,
) {
	t.Helper()
	var count int
	if err := db.QueryRowContext(
		context.Background(),
		"SELECT COUNT(*) FROM executions WHERE goal_id = ?",
		goalID,
	).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("Goal %q materialized %d Execution rows, want 0", goalID, count)
	}
}
