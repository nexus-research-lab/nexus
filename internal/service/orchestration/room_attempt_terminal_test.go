package orchestration

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

func TestFinishRoomAttemptAtomicallyTerminatesBoundRootWithoutSubmission(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		status     protocol.WorkAttemptStatus
		reason     string
		wantReason string
	}{
		{
			name:   "finished",
			status: protocol.WorkAttemptStatusSucceeded,
			reason: "must be cleared",
		},
		{
			name:       "error",
			status:     protocol.WorkAttemptStatusFailed,
			reason:     "runtime failed",
			wantReason: "runtime failed",
		},
		{
			name:       "interrupted",
			status:     protocol.WorkAttemptStatusInterrupted,
			reason:     "user stopped",
			wantReason: "user stopped",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			snapshot, binding := structuredRoomWorkBindingSnapshot()
			snapshot.Attempts[0].Status = protocol.WorkAttemptStatusRunning
			var captured orchestrationstore.FinishAttemptCommand
			repository := &fakeRepository{
				snapshot: snapshot,
				finishAttempt: func(
					_ context.Context,
					command orchestrationstore.FinishAttemptCommand,
				) (*protocol.ExecutionSnapshot, error) {
					captured = command
					updated := cloneExecutionSnapshot(snapshot)
					updated.Execution.Version++
					updated.Attempts[0] = command.Attempt
					updated.Attempts[0].Version++
					return updated, nil
				},
			}
			service := NewService(repository)
			actor := structuredRoomMemberActor(binding)
			err := service.FinishRoomAttempt(
				context.Background(),
				actor,
				RoomAttemptTerminalInput{
					Binding:           binding,
					Status:            testCase.status,
					FailureReason:     testCase.reason,
					RuntimeSessionKey: "runtime-session-1",
					RoomSessionID:     "room-session-1",
					SDKSessionID:      "sdk-session-1",
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			if captured.Attempt.ID != binding.AttemptID ||
				captured.Attempt.Status != testCase.status ||
				captured.Attempt.FailureReason != testCase.wantReason ||
				captured.Attempt.RuntimeSessionKey != "runtime-session-1" ||
				captured.Attempt.RoomSessionID != "room-session-1" ||
				captured.Attempt.SDKSessionID != "sdk-session-1" {
				t.Fatalf("terminal command = %#v", captured)
			}
			if len(repository.snapshot.Submissions) != 0 ||
				len(repository.snapshot.Acceptances) != 0 {
				t.Fatal("root Attempt settlement created semantic review state")
			}
		})
	}
}

func TestFinishRoomAttemptIsNoOpAfterRootAlreadyTerminal(t *testing.T) {
	snapshot, binding := structuredRoomWorkBindingSnapshot()
	snapshot.Assignments[0].Status = protocol.WorkAssignmentStatusCompleted
	snapshot.Attempts[0].Status = protocol.WorkAttemptStatusSucceeded
	service := NewService(&fakeRepository{snapshot: snapshot})

	if err := service.FinishRoomAttempt(
		context.Background(),
		structuredRoomMemberActor(binding),
		RoomAttemptTerminalInput{
			Binding: binding,
			Status:  protocol.WorkAttemptStatusFailed,
		},
	); err != nil {
		t.Fatal(err)
	}
}

func TestFinishRoomAttemptIsNoOpAfterRetargetClosedPredecessor(t *testing.T) {
	snapshot, binding := structuredRoomWorkBindingSnapshot()
	snapshot.Execution.Status = protocol.ExecutionStatusSuperseded
	snapshot.Plan = nil
	snapshot.PlanItems = nil
	snapshot.WorkItems = nil
	snapshot.WorkItemSpecs = nil
	snapshot.Assignments = nil
	snapshot.Dispatches = nil
	snapshot.Attempts = nil
	service := NewService(&fakeRepository{snapshot: snapshot})

	if err := service.FinishRoomAttempt(
		context.Background(),
		structuredRoomMemberActor(binding),
		RoomAttemptTerminalInput{
			Binding: binding,
			Status:  protocol.WorkAttemptStatusInterrupted,
		},
	); err != nil {
		t.Fatalf("late predecessor settlement error = %v, want idempotent no-op", err)
	}
}

func TestFinishRoomSelfAttemptRequiresExactDispatchlessBinding(t *testing.T) {
	snapshot, binding := structuredRoomWorkBindingSnapshot()
	snapshot.Assignments[0].OwnerAgentID = "agent-lead"
	snapshot.Assignments[0].Strategy = protocol.AssignmentStrategySelf
	snapshot.Attempts[0].ExecutorAgentID = "agent-lead"
	snapshot.Attempts[0].DispatchID = ""
	snapshot.Attempts[0].Status = protocol.WorkAttemptStatusPending
	binding.DispatchID = ""
	var captured orchestrationstore.FinishAttemptCommand
	repository := &fakeRepository{
		snapshot: snapshot,
		finishAttempt: func(
			_ context.Context,
			command orchestrationstore.FinishAttemptCommand,
		) (*protocol.ExecutionSnapshot, error) {
			captured = command
			updated := cloneExecutionSnapshot(snapshot)
			updated.Execution.Version++
			updated.Attempts[0] = command.Attempt
			return updated, nil
		},
	}
	service := NewService(repository)
	actor := structuredRoomMemberActor(binding)
	actor.AgentID = "agent-lead"
	actor.Role = ExecutionActorCoordinator

	if err := service.FinishRoomAttempt(
		context.Background(),
		actor,
		RoomAttemptTerminalInput{
			Binding:       binding,
			Status:        protocol.WorkAttemptStatusFailed,
			FailureReason: "runtime failed before submit_work",
		},
	); err != nil {
		t.Fatal(err)
	}
	if captured.Attempt.ID != binding.AttemptID ||
		captured.Attempt.Status != protocol.WorkAttemptStatusFailed {
		t.Fatalf("self terminal command = %#v", captured)
	}

	stale := binding
	stale.DispatchID = "dispatch-model-forged"
	actor.WorkBinding = &stale
	if err := service.FinishRoomAttempt(
		context.Background(),
		actor,
		RoomAttemptTerminalInput{
			Binding: stale,
			Status:  protocol.WorkAttemptStatusInterrupted,
		},
	); err == nil {
		t.Fatal("self Room Attempt accepted a forged Dispatch binding")
	}
}
