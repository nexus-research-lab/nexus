package operation

import (
	"context"
	"testing"

	runtimecommand "github.com/nexus-research-lab/nexus/internal/cli/runtimecommand"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

func TestMutationResultProjectsDurableGoalConfirmationPendingAsSuccess(t *testing.T) {
	result := mutationResult(orchestration.MutationResult{
		Outcome:          orchestration.MutationApplied,
		ExecutionID:      "execution-1",
		SnapshotRevision: 2,
		GoalConfirmation: orchestration.GoalConfirmationPending,
		Message:          "Execution and Goal identity are durable; confirmation is pending.",
		NextActions: []orchestration.NextAction{{
			Domain: "execution", Operation: "promote_execution_to_goal",
			Reason: "retry the same promotion intent",
		}},
	})
	if result.IsError {
		t.Fatalf("durable pending mutation became transport error: %#v", result)
	}
	if result.StructuredContent["outcome"] != "applied" ||
		result.StructuredContent["goal_confirmation_status"] != "pending" ||
		result.StructuredContent["next_actions"] == nil {
		t.Fatalf("pending mutation projection = %#v", result.StructuredContent)
	}
}

func TestPromoteOperationKeepsDurablePendingConfirmationOutOfTransportErrors(t *testing.T) {
	current := executionSnapshot(1)
	current.Execution.CompletionCriteria = []string{"verified"}
	promoted := *current
	promoted.Execution = current.Execution
	promoted.Execution.Version = 2
	promoted.Execution.GoalID = "goal-pending"
	promoted.Execution.GoalObjectiveRevision = 1
	promoted.Execution.GoalActivationOrigin = protocol.GoalActivationOriginAdaptivePromoted
	promoted.Execution.GoalActivationReason = protocol.GoalActivationReasonObservedBoundary
	svc := &fakeExecutionService{
		current: func() *protocol.ExecutionSnapshot { return current },
		promote: func(orchestration.PromoteExecutionToGoalInput) orchestration.MutationResult {
			result := orchestration.AppliedResult(
				&promoted,
				[]string{"goal:goal-pending"},
				[]orchestration.NextAction{{
					Domain: "execution", Operation: "promote_execution_to_goal",
					Reason: "retry the same promotion intent",
				}},
			)
			result.GoalConfirmation = orchestration.GoalConfirmationPending
			result.Message = "Execution binding is durable; Goal confirmation is pending."
			return result
		},
	}
	sctx := executionContext()
	sctx.GoalAuthority = runtimectx.NewGoalAuthorityState("", 0, "")
	result, err := promoteExecutionToGoal(svc, sctx).ContextHandler(
		context.Background(),
		map[string]any{"activation_reason": "observed_boundary"},
		&runtimecommand.CallContext{RequestID: "tool-pending-confirmation"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError || result.StructuredContent["outcome"] != "applied" ||
		result.StructuredContent["goal_confirmation_status"] != "pending" ||
		result.StructuredContent["next_actions"] == nil {
		t.Fatalf("promotion pending result = %#v", result)
	}
	if authority, ok := sctx.GoalAuthority.Load(); ok {
		t.Fatalf("pending confirmation minted Goal authority: %+v", authority)
	}
}
