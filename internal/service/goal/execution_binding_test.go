package goal

import (
	"context"
	"errors"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestBindExplicitExecutionIsIdempotentAndFenced(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()

	created, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:explicit-binding",
		Objective:  "Ship the verified report",
		CreatedBy:  "model",
		AgentID:    "agent-1",
		Metadata: map[string]any{
			protocol.GoalMetadataExplicitCommand:  "explicit-command-1",
			protocol.GoalMetadataActivationOrigin: string(protocol.GoalActivationOriginUserExplicit),
			protocol.GoalMetadataActivationReason: string(protocol.GoalActivationReasonPersistenceRequested),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	input := ExplicitExecutionBinding{
		GoalID:                    created.ID,
		ExpectedObjectiveRevision: created.ObjectiveRevision(),
		ExecutionID:               "execution-1",
		CompletionCriteria:        []string{" report accepted ", "tests pass"},
		RoundID:                   "round-plan",
	}
	bound, err := service.BindExplicitExecution(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if got := protocol.GoalMetadataString(
		bound.Metadata,
		protocol.GoalMetadataExecutionID,
	); got != "execution-1" {
		t.Fatalf("execution binding = %q", got)
	}
	if bound.Version != created.Version+1 {
		t.Fatalf("bound version = %d, want %d", bound.Version, created.Version+1)
	}
	criteria := goalMetadataStrings(
		bound.Metadata,
		protocol.GoalMetadataCompletionCriteria,
	)
	if len(criteria) != 2 || criteria[0] != "report accepted" || criteria[1] != "tests pass" {
		t.Fatalf("completion criteria = %#v", criteria)
	}

	replayed, err := service.BindExplicitExecution(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Version != bound.Version {
		t.Fatalf("idempotent replay advanced version from %d to %d", bound.Version, replayed.Version)
	}
	executionBoundEvents := 0
	for _, event := range repo.events {
		if event.EventType == "execution_bound" {
			executionBoundEvents++
		}
	}
	if executionBoundEvents != 1 {
		t.Fatalf("execution_bound events = %d, want 1", executionBoundEvents)
	}

	input.ExecutionID = "execution-other"
	_, err = service.BindExplicitExecution(context.Background(), input)
	if !errors.Is(err, ErrGoalExecutionBindingConflict) {
		t.Fatalf("conflicting rebind error = %v", err)
	}
}

func TestBindExplicitExecutionRejectsRetargetedGoalRevision(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.idFactory = sequentialID()
	created, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:retarget-binding",
		Objective:  "Original objective",
		CreatedBy:  "model",
		AgentID:    "agent-1",
		Metadata: map[string]any{
			protocol.GoalMetadataExplicitCommand:  "explicit-command-2",
			protocol.GoalMetadataActivationOrigin: string(protocol.GoalActivationOriginUserExplicit),
			protocol.GoalMetadataActivationReason: string(protocol.GoalActivationReasonPersistenceRequested),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	repo.goals[created.ID] = protocol.Goal{
		ID:         created.ID,
		SessionKey: created.SessionKey,
		Objective:  "Retargeted objective",
		Status:     protocol.GoalStatusActive,
		Version:    created.Version + 1,
		Metadata: map[string]any{
			protocol.GoalMetadataObjectiveRevision: int64(2),
			protocol.GoalMetadataExplicitCommand:   "explicit-command-2",
			protocol.GoalMetadataActivationOrigin:  string(protocol.GoalActivationOriginUserExplicit),
			protocol.GoalMetadataActivationReason:  string(protocol.GoalActivationReasonPersistenceRequested),
		},
	}
	_, err = service.BindExplicitExecution(context.Background(), ExplicitExecutionBinding{
		GoalID:                    created.ID,
		ExpectedObjectiveRevision: 1,
		ExecutionID:               "execution-1",
	})
	if !errors.Is(err, ErrGoalRevisionStale) {
		t.Fatalf("retargeted binding error = %v, want revision stale", err)
	}
}

func TestExplicitGoalCompletionFailsClosedWithoutExecutionAudit(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.idFactory = sequentialID()
	created, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:missing-execution-audit",
		Objective:  "Complete only after WorkGraph acceptance",
		CreatedBy:  "model",
		AgentID:    "agent-1",
		Metadata: map[string]any{
			protocol.GoalMetadataExplicitCommand:  "explicit-command-3",
			protocol.GoalMetadataActivationOrigin: string(protocol.GoalActivationOriginUserExplicit),
			protocol.GoalMetadataActivationReason: string(protocol.GoalActivationReasonPersistenceRequested),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.CompleteByModel(context.Background(), created.ID, protocol.CompleteGoalRequest{
		AgentID:                   "agent-1",
		ExpectedObjectiveRevision: created.ObjectiveRevision(),
	})
	if !errors.Is(err, ErrGoalInvalidState) {
		t.Fatalf("CompleteByModel() error = %v, want fail-closed audit rejection", err)
	}
	current, loadErr := service.Current(context.Background(), created.SessionKey)
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if current.Status != protocol.GoalStatusActive {
		t.Fatalf("Goal status = %q, want active", current.Status)
	}
}
