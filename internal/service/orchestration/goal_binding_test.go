package orchestration

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestResolveGoalExecutionBindingSeparatesReservationFromMaterialization(t *testing.T) {
	goal := goalBindingTestGoal(protocol.GoalExecutionBindingStateReserved)
	service := NewService(&fakeRepository{})

	resolution, err := service.ResolveGoalExecutionBinding(context.Background(), goal)
	if err != nil {
		t.Fatal(err)
	}
	if resolution.State != protocol.GoalExecutionBindingStateReserved ||
		resolution.ExecutionID != "" || resolution.ReservedExecutionID != "execution-goal-1" {
		t.Fatalf("resolution = %+v", resolution)
	}
	blocker, err := service.GoalExecutionCompletionBlocker(context.Background(), goal)
	if err != nil {
		t.Fatal(err)
	}
	if blocker != "" {
		t.Fatalf("reserved Goal blocker = %q, want empty", blocker)
	}
}

func TestResolveGoalExecutionBindingRequiresExactBilateralIdentity(t *testing.T) {
	tests := []struct {
		name        string
		state       protocol.GoalExecutionBindingState
		execution   protocol.Execution
		want        protocol.GoalExecutionBindingState
		wantBlocker string
	}{
		{
			name:  "pending exact execution",
			state: protocol.GoalExecutionBindingStatePending,
			execution: protocol.Execution{
				ID: "execution-goal-1", OwnerUserID: "owner-1", SessionKey: "agent:agent-1",
				ScopeKind: protocol.ExecutionScopeDM, GoalID: "goal-1", GoalObjectiveRevision: 1,
			},
			want:        protocol.GoalExecutionBindingStatePending,
			wantBlocker: "execution_binding_pending:execution-goal-1",
		},
		{
			name:  "confirmed exact execution",
			state: protocol.GoalExecutionBindingStateConfirmed,
			execution: protocol.Execution{
				ID: "execution-goal-1", OwnerUserID: "owner-1", SessionKey: "agent:agent-1",
				ScopeKind: protocol.ExecutionScopeDM, GoalID: "goal-1", GoalObjectiveRevision: 1,
			},
			want: protocol.GoalExecutionBindingStateConfirmed,
		},
		{
			name:  "legacy exact execution",
			state: "",
			execution: protocol.Execution{
				ID: "execution-goal-1", OwnerUserID: "owner-1", SessionKey: "agent:agent-1",
				ScopeKind: protocol.ExecutionScopeDM, GoalID: "goal-1", GoalObjectiveRevision: 1,
			},
			want: protocol.GoalExecutionBindingStateConfirmed,
		},
		{
			name:  "session mismatch",
			state: protocol.GoalExecutionBindingStateConfirmed,
			execution: protocol.Execution{
				ID: "execution-goal-1", OwnerUserID: "owner-1", SessionKey: "agent:other",
				ScopeKind: protocol.ExecutionScopeDM, GoalID: "goal-1", GoalObjectiveRevision: 1,
			},
			want:        protocol.GoalExecutionBindingStateConflict,
			wantBlocker: "execution_binding_conflict:execution-goal-1",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			goal := goalBindingTestGoal(test.state)
			service := NewService(&fakeRepository{snapshot: &protocol.ExecutionSnapshot{
				Execution: test.execution,
			}})
			resolution, err := service.ResolveGoalExecutionBinding(context.Background(), goal)
			if err != nil {
				t.Fatal(err)
			}
			if resolution.State != test.want {
				t.Fatalf("state = %q, want %q; resolution=%+v", resolution.State, test.want, resolution)
			}
			if test.wantBlocker != "" {
				blocker, blockerErr := service.GoalExecutionCompletionBlocker(context.Background(), goal)
				if blockerErr != nil {
					t.Fatal(blockerErr)
				}
				if blocker != test.wantBlocker {
					t.Fatalf("blocker = %q, want %q", blocker, test.wantBlocker)
				}
			}
		})
	}
}

func TestResolveGoalExecutionBindingFailsClosedWhenConfirmationLosesExecution(t *testing.T) {
	goal := goalBindingTestGoal(protocol.GoalExecutionBindingStateConfirmed)
	resolution, err := NewService(&fakeRepository{}).ResolveGoalExecutionBinding(
		context.Background(),
		goal,
	)
	if err != nil {
		t.Fatal(err)
	}
	if resolution.State != protocol.GoalExecutionBindingStateConflict {
		t.Fatalf("state = %q, want conflict", resolution.State)
	}
}

func goalBindingTestGoal(state protocol.GoalExecutionBindingState) protocol.Goal {
	metadata := map[string]any{
		protocol.GoalMetadataOwnerUserID: "owner-1",
		protocol.GoalMetadataExecutionID: "execution-goal-1",
	}
	if state != "" {
		metadata[protocol.GoalMetadataExecutionBindingState] = string(state)
	}
	return protocol.Goal{
		ID: "goal-1", SessionKey: "agent:agent-1", Objective: "ship",
		Status: protocol.GoalStatusActive, Version: 1, Metadata: metadata,
	}
}
