package orchestration

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRepositoryCreatesExecutionOnReservedExplicitGoalChain(t *testing.T) {
	repository := newRepositoryTestStore(t)
	ctx := context.Background()
	command := createTestCommand("explicit-create")
	if _, err := repository.db.Exec(`
INSERT INTO session_goals (goal_id, session_key, objective, status, metadata_json)
VALUES (?, ?, ?, 'active', ?)`,
		"goal-explicit",
		command.Execution.SessionKey,
		command.Execution.Objective,
		`{"activation_origin":"user_explicit","activation_reason":"persistence_requested","execution_id":"execution-explicit-create"}`,
	); err != nil {
		t.Fatal(err)
	}
	command.Execution.GoalID = "goal-explicit"
	command.Execution.GoalObjectiveRevision = 1
	command.Execution.GoalActivationOrigin = protocol.GoalActivationOriginUserExplicit
	command.Execution.GoalActivationReason = protocol.GoalActivationReasonPersistenceRequested

	snapshot, err := repository.Create(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Execution.GoalID != "goal-explicit" ||
		snapshot.Execution.GoalObjectiveRevision != 1 ||
		snapshot.Execution.GoalActivationOrigin != protocol.GoalActivationOriginUserExplicit ||
		snapshot.Execution.GoalActivationReason != protocol.GoalActivationReasonPersistenceRequested {
		t.Fatalf("explicit Goal binding = %#v", snapshot.Execution)
	}
	replayed, err := repository.Create(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Execution.ID != snapshot.Execution.ID ||
		replayed.Execution.Version != snapshot.Execution.Version {
		t.Fatalf("replayed snapshot = %#v, want %#v", replayed.Execution, snapshot.Execution)
	}
}
