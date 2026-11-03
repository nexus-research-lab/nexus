package operation

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/mcp/command"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

func TestAssignmentBatchInvokesWithoutContractLookupAndBindsOnlyDurableReceipt(t *testing.T) {
	for _, outcome := range []orchestration.MutationOutcome{orchestration.MutationApplied, orchestration.MutationRejected} {
		t.Run(string(outcome), func(t *testing.T) {
			snapshot := executionSnapshot(9)
			binding := &protocol.ExecutionWorkBinding{
				ExecutionID: snapshot.Execution.ID, PlanID: "plan-1", WorkItemID: "work-self",
				SpecID: "spec-self", AssignmentID: "assignment-self", AttemptID: "attempt-self",
			}
			sctx := executionContext()
			sctx.ResponsibilityAuthority = runtimectx.NewResponsibilityAuthorityState(nil, snapshot.Execution.ID, nil, nil)
			calls := 0
			svc := &fakeExecutionService{
				current: func() *protocol.ExecutionSnapshot { return snapshot },
				assignBatch: func(input orchestration.AssignWorkBatchInput) orchestration.MutationResult {
					calls++
					if input.ExecutionID != snapshot.Execution.ID || input.SnapshotRevision != 9 || input.CommandID != "batch-request-001" || len(input.Assignments) != 2 || input.Assignments[0].TargetAgentID != "agent-worker" {
						t.Fatalf("batch envelope = %+v", input)
					}
					return orchestration.MutationResult{Outcome: outcome, ExecutionID: snapshot.Execution.ID, Snapshot: snapshot, WorkBinding: &orchestration.WorkBindingReceipt{Binding: binding}}
				},
			}
			result, err := command.HandleSemantic(context.Background(), command.Actor{}, "execution", "get_execution", BuildAll(svc, sctx), command.Request{
				Domain: "execution", Action: "invoke", Operation: "assign_work_batch", RequestID: "batch-request-001",
				Input: map[string]any{"assignments": []any{
					map[string]any{"logical_key": "first", "target_agent_id": "agent-worker"},
					map[string]any{"logical_key": "self", "target_agent_id": "agent-1"},
				}},
			})
			if err != nil || calls != 1 {
				t.Fatalf("invoke = %#v err=%v calls=%d", result, err, calls)
			}
			actor := sctx.Actor()
			if outcome == orchestration.MutationApplied && (actor.WorkBinding == nil || *actor.WorkBinding != *binding) {
				t.Fatalf("applied batch did not bind exact self work: %+v", actor)
			}
			if outcome == orchestration.MutationRejected && actor.WorkBinding != nil {
				t.Fatal("rejected batch changed responsibility")
			}
		})
	}
}

func TestAssignmentBatchRejectsNestedIdentityAndBoundsBeforeStateReads(t *testing.T) {
	inputs := []map[string]any{
		{"assignments": []any{}},
		{"assignments": make([]any, 33)},
		{"assignments": []any{map[string]any{"target_agent_id": "worker", "execution_id": "another"}}},
		{"assignments": []any{map[string]any{"logical_key": "one"}}},
		{"assignments": []any{map[string]any{"target_agent_id": "worker", "snapshot_revision": 7}}},
	}
	for _, input := range inputs {
		svc := &fakeExecutionService{}
		_, err := assignWorkBatch(svc, executionContext()).Invoke(context.Background(), input, &command.CallContext{RequestID: "batch-invalid"})
		if err == nil || svc.snapshotReads != 0 || svc.currentReads != 0 {
			t.Fatalf("invalid input reached state: %#v err=%v reads=%d/%d", input, err, svc.snapshotReads, svc.currentReads)
		}
	}
}
