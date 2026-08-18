package orchestration

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

func TestServicePlanExecutionCreatesExecutionAndFirstPlanAtomically(t *testing.T) {
	var captured orchestrationstore.CreateWithPlanCommand
	repository := &fakeRepository{
		createWithPlan: func(
			_ context.Context,
			command orchestrationstore.CreateWithPlanCommand,
		) (*protocol.ExecutionSnapshot, error) {
			captured = command
			return snapshotFromInitialPlan(command.Execution, command.Plan), nil
		},
	}
	service := testService(repository)
	result, err := service.PlanExecution(context.Background(), coordinatorActor(), PlanExecutionInput{
		CommandID:          "initial-plan",
		Objective:          "Deliver a verified orchestration report",
		CompletionCriteria: []string{"The final report is accepted"},
		Draft:              validPlanDraft(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationApplied ||
		captured.Execution.ID == "" ||
		captured.Plan.Plan.ExecutionID != captured.Execution.ID ||
		captured.Plan.Plan.Revision != 1 ||
		result.Snapshot == nil ||
		result.Snapshot.Plan == nil {
		t.Fatalf("initial atomic result=%#v command=%#v", result, captured)
	}
}

func TestServicePlanExecutionRequiresExplicitReplacementForBoundaryChange(t *testing.T) {
	snapshot := executionSnapshot()
	snapshot.Execution.CompletionCriteria = []string{"old criterion"}
	service := testService(&fakeRepository{snapshot: snapshot})
	result, err := service.PlanExecution(context.Background(), coordinatorActor(), PlanExecutionInput{
		ExecutionID:      snapshot.Execution.ID,
		SnapshotRevision: snapshot.Execution.Version,
		CommandID:        "wrong-boundary-replan",
		Objective:        "A different objective",
		CompletionCriteria: []string{
			"a new criterion",
		},
		Draft: validPlanDraft(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationRejected ||
		result.ReasonCode != ErrorCodeObjectiveChangeReplace ||
		len(result.NextActions) != 1 ||
		result.NextActions[0].Operation != "prepare_plan_execution" {
		t.Fatalf("boundary mismatch result = %#v", result)
	}
}

func TestServicePlanExecutionReplacesTransientWithCompleteSuccessor(t *testing.T) {
	old := executionSnapshot()
	old.Execution.CompletionCriteria = []string{"old criterion"}
	var captured orchestrationstore.ReplaceWithPlanCommand
	repository := &fakeRepository{
		snapshot: old,
		replaceWithPlan: func(
			_ context.Context,
			command orchestrationstore.ReplaceWithPlanCommand,
		) (*protocol.ExecutionSnapshot, error) {
			captured = command
			return snapshotFromInitialPlan(command.Successor, command.Plan), nil
		},
	}
	service := testService(repository)
	result, err := service.PlanExecution(context.Background(), coordinatorActor(), PlanExecutionInput{
		ExecutionID:             old.Execution.ID,
		SnapshotRevision:        old.Execution.Version,
		CommandID:               "replace-objective",
		Objective:               "Deliver the replacement objective",
		CompletionCriteria:      []string{"replacement accepted"},
		ReplaceCurrentExecution: true,
		ReplacementReason:       "user explicitly changed the objective",
		Draft:                   validPlanDraft(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != MutationApplied ||
		captured.ExecutionID != old.Execution.ID ||
		captured.Successor.ReplacesExecutionID != old.Execution.ID ||
		captured.Successor.Objective == old.Execution.Objective ||
		captured.Plan.Plan.ExecutionID != captured.Successor.ID ||
		result.ExecutionID != captured.Successor.ID {
		t.Fatalf("replacement result=%#v command=%#v", result, captured)
	}
}

func TestServicePlanExecutionRejectsGoalBoundReplacement(t *testing.T) {
	snapshot := executionSnapshot()
	snapshot.Execution.GoalID = "goal-1"
	snapshot.Execution.GoalObjectiveRevision = 1
	snapshot.Execution.GoalActivationOrigin = protocol.GoalActivationOriginUserExplicit
	snapshot.Execution.GoalActivationReason = protocol.GoalActivationReasonPersistenceRequested
	service := testService(&fakeRepository{snapshot: snapshot})
	result, err := service.PlanExecution(context.Background(), coordinatorActor(), PlanExecutionInput{
		ExecutionID:             snapshot.Execution.ID,
		SnapshotRevision:        snapshot.Execution.Version,
		CommandID:               "replace-goal",
		Objective:               "different",
		CompletionCriteria:      []string{"accepted"},
		ReplaceCurrentExecution: true,
		ReplacementReason:       "user changed objective",
		Draft:                   validPlanDraft(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ReasonCode != ErrorCodeGoalRetargetRequired {
		t.Fatalf("Goal-bound replacement result = %#v", result)
	}
}

func TestServiceAbandonExecutionWritesOnlyOutsidePlanMode(t *testing.T) {
	for _, test := range []struct {
		name     string
		planMode bool
		outcome  MutationOutcome
		calls    int
	}{
		{name: "validate", planMode: true, outcome: MutationNoOp, calls: 0},
		{name: "apply", outcome: MutationApplied, calls: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			snapshot := executionSnapshot()
			calls := 0
			repository := &fakeRepository{
				snapshot: snapshot,
				abandon: func(
					_ context.Context,
					command orchestrationstore.AbandonCommand,
				) (*protocol.ExecutionSnapshot, error) {
					calls++
					updated := cloneExecutionSnapshot(snapshot)
					updated.Execution.Version++
					updated.Execution.Status = protocol.ExecutionStatusCancelled
					return updated, nil
				},
			}
			service := testService(repository)
			actor := coordinatorActor()
			actor.PlanMode = test.planMode
			result, err := service.AbandonExecution(context.Background(), actor, AbandonExecutionInput{
				ExecutionID:      snapshot.Execution.ID,
				SnapshotRevision: snapshot.Execution.Version,
				CommandID:        "abandon",
				Reason:           "user stopped this objective",
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.Outcome != test.outcome || calls != test.calls {
				t.Fatalf("result=%#v calls=%d", result, calls)
			}
		})
	}
}

func TestServiceTerminalExecutionRejectsLateMutation(t *testing.T) {
	snapshot := assignedExecutionSnapshot()
	snapshot.Execution.Status = protocol.ExecutionStatusSuperseded
	service := testService(&fakeRepository{snapshot: snapshot})
	result, err := service.AssignWork(context.Background(), coordinatorActor(), AssignWorkInput{
		ExecutionID:      snapshot.Execution.ID,
		SnapshotRevision: snapshot.Execution.Version,
		CommandID:        "late-assign",
		LogicalKey:       snapshot.WorkItems[0].LogicalKey,
		TargetAgentID:    "agent-worker",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ReasonCode != ErrorCodeExecutionTerminal {
		t.Fatalf("terminal mutation result = %#v", result)
	}
}

func snapshotFromInitialPlan(
	execution protocol.Execution,
	command orchestrationstore.WritePlanCommand,
) *protocol.ExecutionSnapshot {
	execution.Version = 1
	plan := command.Plan
	plan.Version = 1
	result := &protocol.ExecutionSnapshot{
		Execution:    execution,
		Plan:         &plan,
		Dependencies: append([]protocol.ExecutionPlanDependency(nil), command.Dependencies...),
	}
	for _, item := range command.WorkItems {
		result.WorkItems = append(result.WorkItems, item.WorkItem)
		result.WorkItemSpecs = append(result.WorkItemSpecs, item.Spec)
		result.WorkItemStates = append(result.WorkItemStates, item.State)
		result.PlanItems = append(result.PlanItems, item.Item)
		for _, claim := range item.OutputClaims {
			claim.ExecutionID = execution.ID
			claim.PlanID = plan.ID
			claim.WorkItemID = item.WorkItem.ID
			claim.SpecID = item.Spec.ID
			result.OutputClaims = append(result.OutputClaims, claim)
		}
	}
	return result
}
