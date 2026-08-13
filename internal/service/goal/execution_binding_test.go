package goal

import (
	"context"
	"errors"
	"slices"
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
			protocol.GoalMetadataExecutionID: protocol.ExplicitGoalReservedExecutionID(
				"explicit-command-1",
			),
			protocol.GoalMetadataExecutionBindingState: string(
				protocol.GoalExecutionBindingStateReserved,
			),
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
		ExecutionID:               protocol.ExplicitGoalReservedExecutionID("explicit-command-1"),
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
	); got != input.ExecutionID {
		t.Fatalf("execution binding = %q", got)
	}
	if got := protocol.GoalExecutionBindingStateFromGoal(*bound); got !=
		protocol.GoalExecutionBindingStatePending {
		t.Fatalf("execution binding state = %q, want pending", got)
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
	pendingEvents := 0
	for _, event := range repo.events {
		if event.EventType == "execution_binding_pending" {
			pendingEvents++
		}
	}
	if pendingEvents != 1 {
		t.Fatalf("execution_binding_pending events = %d, want 1", pendingEvents)
	}

	input.ExecutionID = "execution-other"
	_, err = service.BindExplicitExecution(context.Background(), input)
	if !errors.Is(err, ErrGoalExecutionBindingConflict) {
		t.Fatalf("conflicting rebind error = %v", err)
	}
}

func TestConfirmExplicitExecutionBindingWithoutObjectiveTransition(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	created, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:initial-binding-confirmation",
		Objective:  "Confirm the existing Execution binding",
		CreatedBy:  "model",
		AgentID:    "agent-1",
		Metadata: map[string]any{
			protocol.GoalMetadataExecutionID: "execution-current",
			protocol.GoalMetadataExecutionBindingState: string(
				protocol.GoalExecutionBindingStatePending,
			),
			protocol.GoalMetadataActivationOrigin: string(protocol.GoalActivationOriginUserExplicit),
			protocol.GoalMetadataActivationReason: string(protocol.GoalActivationReasonPersistenceRequested),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	confirmed, err := service.ConfirmObjectiveExecutionBinding(
		context.Background(),
		created.ID,
		created.ObjectiveRevision(),
		"execution-current",
		[]string{" accepted ", "tests pass"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if got := protocol.GoalExecutionBindingStateFromGoal(*confirmed); got !=
		protocol.GoalExecutionBindingStateConfirmed {
		t.Fatalf("binding state = %q, want confirmed", got)
	}
	if got := goalMetadataStrings(confirmed.Metadata, protocol.GoalMetadataCompletionCriteria); !slices.Equal(got, []string{"accepted", "tests pass"}) {
		t.Fatalf("completion criteria = %#v", got)
	}
	if _, transitioning := ObjectiveTransitionFromGoal(*confirmed); transitioning {
		t.Fatalf("initial binding unexpectedly created objective transition: %#v", confirmed.Metadata)
	}
	events := 0
	for _, event := range repo.events {
		if event.EventType == "execution_bound" {
			events++
		}
	}
	if events != 1 {
		t.Fatalf("execution_bound events = %d, want 1", events)
	}

	replayed, err := service.ConfirmObjectiveExecutionBinding(
		context.Background(),
		confirmed.ID,
		confirmed.ObjectiveRevision(),
		"execution-current",
		[]string{"accepted", "tests pass"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Version != confirmed.Version {
		t.Fatalf("confirmation replay advanced version from %d to %d", confirmed.Version, replayed.Version)
	}
}

func TestConfirmExplicitExecutionBindingRejectsFutureReservation(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.idFactory = sequentialID()
	created, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:future-reservation",
		Objective:  "Wait for authoritative materialization",
		CreatedBy:  "model",
		Metadata: map[string]any{
			protocol.GoalMetadataExecutionID: "execution-future",
			protocol.GoalMetadataExecutionBindingState: string(
				protocol.GoalExecutionBindingStateReserved,
			),
			protocol.GoalMetadataActivationOrigin: string(protocol.GoalActivationOriginUserExplicit),
			protocol.GoalMetadataActivationReason: string(protocol.GoalActivationReasonPersistenceRequested),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.ConfirmObjectiveExecutionBinding(
		context.Background(),
		created.ID,
		created.ObjectiveRevision(),
		"execution-future",
		nil,
	)
	if !errors.Is(err, ErrGoalExecutionBindingConflict) {
		t.Fatalf("reserved confirmation error = %v, want binding conflict", err)
	}
	stored, loadErr := repo.GetGoal(context.Background(), created.ID)
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if got := protocol.GoalExecutionBindingStateFromGoal(*stored); got !=
		protocol.GoalExecutionBindingStateReserved {
		t.Fatalf("reserved binding state changed to %q", got)
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

func TestReservedExplicitGoalCompletionDoesNotRequireExecutionAudit(t *testing.T) {
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
	completed, err := service.CompleteByModel(context.Background(), created.ID, protocol.CompleteGoalRequest{
		AgentID:                   "agent-1",
		ExpectedObjectiveRevision: created.ObjectiveRevision(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != protocol.GoalStatusComplete {
		t.Fatalf("Goal status = %q, want complete", completed.Status)
	}
}

func TestConfirmedGoalCompletionFailsClosedWithoutExecutionAudit(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.idFactory = sequentialID()
	created, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:confirmed-missing-execution-audit",
		Objective:  "Complete only after WorkGraph acceptance",
		CreatedBy:  "model",
		AgentID:    "agent-1",
		Metadata: map[string]any{
			protocol.GoalMetadataExecutionID: "execution-confirmed",
			protocol.GoalMetadataExecutionBindingState: string(
				protocol.GoalExecutionBindingStateConfirmed,
			),
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
}
