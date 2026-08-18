package orchestration

import (
	"context"
	"errors"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

func TestEnsureCreatesExecutionOnActiveExplicitGoalChain(t *testing.T) {
	var created orchestrationstore.CreateCommand
	repository := &fakeRepository{
		create: func(
			_ context.Context,
			command orchestrationstore.CreateCommand,
		) (*protocol.ExecutionSnapshot, error) {
			created = command
			item := command.Execution
			item.Version = 1
			return &protocol.ExecutionSnapshot{Execution: item}, nil
		},
	}
	service := testService(repository)
	service.SetExplicitGoalBindingGateway(explicitGoalGatewayFunc(func(
		_ context.Context,
		request ExplicitGoalBindingRequest,
	) (*ExplicitGoalBinding, error) {
		if request.ExistingExecution ||
			request.CandidateExecutionID != "execution-1" ||
			request.Objective != "Ship orchestration" {
			t.Fatalf("binding request = %#v", request)
		}
		return &ExplicitGoalBinding{
			ExecutionID:           "execution-reserved",
			GoalID:                "goal-explicit",
			GoalObjectiveRevision: 3,
			ActivationOrigin:      protocol.GoalActivationOriginUserExplicit,
			ActivationReason:      protocol.GoalActivationReasonPersistenceRequested,
		}, nil
	}))
	result, err := service.Ensure(context.Background(), coordinatorActor(), EnsureInput{
		CommandID:          "plan-command",
		Objective:          "Ship orchestration",
		CompletionCriteria: []string{"accepted"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationApplied ||
		created.Execution.ID != "execution-reserved" ||
		created.Execution.GoalID != "goal-explicit" ||
		created.Execution.GoalObjectiveRevision != 3 ||
		created.Execution.GoalActivationOrigin != protocol.GoalActivationOriginUserExplicit ||
		created.Execution.GoalActivationReason != protocol.GoalActivationReasonPersistenceRequested {
		t.Fatalf("result=%#v created=%#v", result, created.Execution)
	}
}

func TestEnsureBindsCurrentTransientExecutionToExplicitGoal(t *testing.T) {
	snapshot := executionSnapshot()
	var bound orchestrationstore.BindGoalCommand
	repository := &fakeRepository{
		snapshot: snapshot,
		bindGoal: func(
			_ context.Context,
			command orchestrationstore.BindGoalCommand,
		) (*protocol.ExecutionSnapshot, error) {
			bound = command
			updated := cloneExecutionSnapshot(snapshot)
			updated.Execution.Version++
			updated.Execution.GoalID = command.Execution.GoalID
			updated.Execution.GoalObjectiveRevision = command.Execution.GoalObjectiveRevision
			updated.Execution.GoalActivationOrigin = command.Execution.GoalActivationOrigin
			updated.Execution.GoalActivationReason = command.Execution.GoalActivationReason
			return updated, nil
		},
	}
	service := testService(repository)
	gateway := &confirmingGoalBindingGateway{
		binding: ExplicitGoalBinding{
			ExecutionID:           snapshot.Execution.ID,
			GoalID:                "goal-explicit",
			GoalObjectiveRevision: 1,
			ActivationOrigin:      protocol.GoalActivationOriginUserExplicit,
			ActivationReason:      protocol.GoalActivationReasonPersistenceRequested,
		},
	}
	service.SetExplicitGoalBindingGateway(gateway)
	result, err := service.Ensure(context.Background(), coordinatorActor(), EnsureInput{
		CommandID: "create-goal-command",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationApplied ||
		bound.ExpectedExecutionVersion != snapshot.Execution.Version ||
		bound.Meta.CommandID != "create-goal-command:bind-explicit-goal" ||
		bound.Execution.GoalID != "goal-explicit" {
		t.Fatalf("result=%#v command=%#v", result, bound)
	}
	if gateway.prepareCalls != 1 || gateway.confirmCalls != 1 ||
		gateway.lastConfirmation.ExecutionID != snapshot.Execution.ID {
		t.Fatalf("binding gateway = %#v", gateway)
	}
}

func TestEnsureRejectsIncompatibleExplicitGoalWithoutCreatingExecution(t *testing.T) {
	created := false
	service := testService(&fakeRepository{
		create: func(
			context.Context,
			orchestrationstore.CreateCommand,
		) (*protocol.ExecutionSnapshot, error) {
			created = true
			return nil, nil
		},
	})
	service.SetExplicitGoalBindingGateway(explicitGoalGatewayFunc(func(
		context.Context,
		ExplicitGoalBindingRequest,
	) (*ExplicitGoalBinding, error) {
		return nil, fmtExplicitGoalObjectiveConflict()
	}))
	result, err := service.Ensure(context.Background(), coordinatorActor(), EnsureInput{
		CommandID:          "plan-command",
		Objective:          "Different objective",
		CompletionCriteria: []string{"accepted"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationRejected ||
		result.ReasonCode != ErrorCodeGoalObjectiveConflict ||
		created {
		t.Fatalf("result=%#v created=%t", result, created)
	}
}

func TestGoalRevisionSuccessorPlanUsesReservedIdentityAndRepairsConfirmationOnRetry(t *testing.T) {
	confirmationFailure := errors.New("confirmation temporarily unavailable")
	gateway := &confirmingGoalBindingGateway{
		binding: ExplicitGoalBinding{
			ExecutionID:           "execution-reserved-successor",
			GoalID:                "goal-adaptive",
			GoalObjectiveRevision: 2,
			ActivationOrigin:      protocol.GoalActivationOriginAdaptivePromoted,
			ActivationReason:      protocol.GoalActivationReasonObservedBoundary,
			ReplacesExecutionID:   "execution-old-revision",
		},
		confirmErrOnce: confirmationFailure,
	}
	createCalls := 0
	var repository *fakeRepository
	repository = &fakeRepository{
		createWithPlan: func(
			_ context.Context,
			command orchestrationstore.CreateWithPlanCommand,
		) (*protocol.ExecutionSnapshot, error) {
			createCalls++
			repository.snapshot = snapshotFromInitialPlan(command.Execution, command.Plan)
			return repository.snapshot, nil
		},
	}
	service := testService(repository)
	service.SetExplicitGoalBindingGateway(gateway)
	input := PlanExecutionInput{
		CommandID:          "plan-goal-successor",
		Objective:          "Deliver the revised Goal result",
		CompletionCriteria: []string{"revised result accepted"},
		Draft:              validPlanDraft(),
	}

	if _, err := service.PlanExecution(
		context.Background(),
		coordinatorActor(),
		input,
	); !errors.Is(err, confirmationFailure) {
		t.Fatalf("first plan error = %v, want confirmation failure", err)
	}
	if createCalls != 1 || repository.snapshot == nil ||
		repository.snapshot.Execution.ID != gateway.binding.ExecutionID ||
		repository.snapshot.Execution.GoalID != gateway.binding.GoalID ||
		repository.snapshot.Execution.GoalObjectiveRevision != gateway.binding.GoalObjectiveRevision ||
		repository.snapshot.Execution.GoalActivationOrigin != gateway.binding.ActivationOrigin ||
		repository.snapshot.Execution.GoalActivationReason != gateway.binding.ActivationReason ||
		repository.snapshot.Execution.ReplacesExecutionID != gateway.binding.ReplacesExecutionID ||
		repository.snapshot.Plan == nil {
		t.Fatalf(
			"successor snapshot=%#v createCalls=%d",
			repository.snapshot,
			createCalls,
		)
	}

	replayed, err := service.PlanExecution(
		context.Background(),
		coordinatorActor(),
		input,
	)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Outcome != MutationNoOp ||
		createCalls != 1 ||
		gateway.prepareCalls != 1 ||
		gateway.confirmCalls != 2 ||
		gateway.lastConfirmation.ExecutionID != gateway.binding.ExecutionID ||
		gateway.lastConfirmation.GoalObjectiveRevision != gateway.binding.GoalObjectiveRevision {
		t.Fatalf(
			"replay=%#v createCalls=%d gateway=%#v",
			replayed,
			createCalls,
			gateway,
		)
	}
}

func TestGoalRevisionSuccessorPlanModeDoesNotReserveOrCreateState(t *testing.T) {
	service := testService(&fakeRepository{
		createWithPlan: func(
			context.Context,
			orchestrationstore.CreateWithPlanCommand,
		) (*protocol.ExecutionSnapshot, error) {
			t.Fatal("Plan Mode must not create a successor Execution")
			return nil, nil
		},
	})
	service.SetExplicitGoalBindingGateway(explicitGoalGatewayFunc(func(
		context.Context,
		ExplicitGoalBindingRequest,
	) (*ExplicitGoalBinding, error) {
		t.Fatal("Plan Mode must not reserve Goal successor metadata")
		return nil, nil
	}))
	actor := coordinatorActor()
	actor.PlanMode = true
	result, err := service.PlanExecution(context.Background(), actor, PlanExecutionInput{
		CommandID:          "plan-mode-goal-successor",
		Objective:          "Deliver the revised Goal result",
		CompletionCriteria: []string{"revised result accepted"},
		Draft:              validPlanDraft(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationNoOp || result.Snapshot != nil ||
		len(result.NextActions) != 1 ||
		result.NextActions[0].Operation != "prepare_plan_execution" {
		t.Fatalf("Plan Mode result = %#v", result)
	}
}

func TestGoalExecutionCompletionBlockerSeparatesReservationFromConfirmedBinding(t *testing.T) {
	service := testService(&fakeRepository{})
	blocker, err := service.GoalExecutionCompletionBlocker(context.Background(), protocol.Goal{
		ID:         "goal-explicit",
		SessionKey: "session-1",
		Metadata: map[string]any{
			protocol.GoalMetadataActivationOrigin: string(protocol.GoalActivationOriginUserExplicit),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if blocker != "" {
		t.Fatalf("unbound explicit Goal blocker = %q, want empty", blocker)
	}
	legacyGoal := protocol.Goal{
		ID:         "goal-explicit-legacy",
		SessionKey: "session-legacy",
		Metadata: map[string]any{
			protocol.GoalMetadataExplicitCommand:  "explicit_goal_legacy",
			protocol.GoalMetadataActivationOrigin: string(protocol.GoalActivationOriginUserExplicit),
			protocol.GoalMetadataActivationReason: string(protocol.GoalActivationReasonPersistenceRequested),
		},
	}
	blocker, err = service.GoalExecutionCompletionBlocker(context.Background(), legacyGoal)
	if err != nil {
		t.Fatal(err)
	}
	if blocker != "" {
		t.Fatalf("legacy reserved Goal blocker = %q, want empty", blocker)
	}

	snapshot := executionSnapshot()
	snapshot.Execution.GoalID = "goal-explicit"
	snapshot.Execution.GoalObjectiveRevision = 1
	snapshot.CompletionBlockers = []string{"work_item:W1:required_not_accepted"}
	service = testService(&fakeRepository{snapshot: snapshot})
	goal := protocol.Goal{
		ID:         "goal-explicit",
		SessionKey: "session-1",
		Metadata: map[string]any{
			protocol.GoalMetadataExecutionID: snapshot.Execution.ID,
			protocol.GoalMetadataExecutionBindingState: string(
				protocol.GoalExecutionBindingStateConfirmed,
			),
			protocol.GoalMetadataActivationOrigin: string(protocol.GoalActivationOriginUserExplicit),
		},
	}
	blocker, err = service.GoalExecutionCompletionBlocker(context.Background(), goal)
	if err != nil {
		t.Fatal(err)
	}
	if blocker != "execution_work_graph:execution-1:work_item:W1:required_not_accepted" {
		t.Fatalf("bound blocker = %q", blocker)
	}
	snapshot.Execution.Status = protocol.ExecutionStatusCompleted
	snapshot.CompletionBlockers = nil
	blocker, err = service.GoalExecutionCompletionBlocker(context.Background(), goal)
	if err != nil || blocker != "" {
		t.Fatalf("completed blocker = %q, err=%v", blocker, err)
	}
}

type explicitGoalGatewayFunc func(
	context.Context,
	ExplicitGoalBindingRequest,
) (*ExplicitGoalBinding, error)

func (f explicitGoalGatewayFunc) PrepareExplicitGoalBinding(
	ctx context.Context,
	request ExplicitGoalBindingRequest,
) (*ExplicitGoalBinding, error) {
	return f(ctx, request)
}

type confirmingGoalBindingGateway struct {
	binding          ExplicitGoalBinding
	prepareCalls     int
	confirmCalls     int
	confirmErrOnce   error
	lastConfirmation GoalExecutionBindingConfirmation
}

func (g *confirmingGoalBindingGateway) PrepareExplicitGoalBinding(
	_ context.Context,
	_ ExplicitGoalBindingRequest,
) (*ExplicitGoalBinding, error) {
	g.prepareCalls++
	result := g.binding
	return &result, nil
}

func (g *confirmingGoalBindingGateway) ConfirmGoalExecutionBinding(
	_ context.Context,
	confirmation GoalExecutionBindingConfirmation,
) error {
	g.confirmCalls++
	g.lastConfirmation = confirmation
	if g.confirmErrOnce != nil {
		err := g.confirmErrOnce
		g.confirmErrOnce = nil
		return err
	}
	return nil
}

func fmtExplicitGoalObjectiveConflict() error {
	return errors.Join(
		ErrExplicitGoalObjectiveConflict,
		errors.New("active Goal and Execution objectives differ"),
	)
}
