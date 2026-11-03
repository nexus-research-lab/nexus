package orchestration

import (
	"context"
	"errors"
	store "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type continuationRepositoryFake struct {
	*fakeRepository
	receipt *protocol.ExecutionEvent
	writes  int
}

func (r *continuationRepositoryFake) FindAssignmentBatchReceipt(_ context.Context, _, key string) (*protocol.ExecutionEvent, error) {
	if r.receipt != nil && r.receipt.CommandID == key {
		return r.receipt, nil
	}
	return nil, nil
}
func (r *continuationRepositoryFake) Takeover(_ context.Context, c store.TakeoverCommand) (*protocol.ExecutionSnapshot, error) {
	r.writes++
	if c.Replacement.OwnerAgentID != r.snapshot.Assignments[0].OwnerAgentID || c.Replacement.ReturnToAgentID != r.snapshot.Assignments[0].ReturnToAgentID || c.ContinuationInstruction == "" {
		return nil, errors.New("continuation changed owner/reviewer or omitted handoff")
	}
	if r.snapshot.Execution.ScopeKind == protocol.ExecutionScopeRoom && c.Dispatch == nil {
		return nil, errors.New("Room continuation has no durable dispatch")
	}
	updated := cloneExecutionSnapshot(r.snapshot)
	updated.Assignments[0].Status = protocol.WorkAssignmentStatusReleased
	updated.Assignments = append(updated.Assignments, c.Replacement)
	updated.Attempts = append(updated.Attempts, *c.RootAttempt)
	updated.Execution.Version++
	r.snapshot = updated
	r.receipt = &protocol.ExecutionEvent{Type: protocol.ExecutionEventWorkTakenOver, CommandID: c.Meta.CommandID, ActorKind: c.Meta.ActorKind, ActorID: c.Meta.ActorID, WorkItemID: c.Replacement.WorkItemID, Payload: c.Meta.Payload}
	return updated, nil
}
func TestUserRecoveryExactTargetReplayAndReconciliation(t *testing.T) {
	for _, scope := range []protocol.ExecutionScopeKind{protocol.ExecutionScopeDM, protocol.ExecutionScopeRoom} {
		t.Run(string(scope), func(t *testing.T) {
			snapshot := assignedExecutionSnapshot()
			snapshot.Execution.ScopeKind = scope
			snapshot.Assignments[0].ReturnToAgentID = snapshot.Execution.CoordinatorAgentID
			if scope == protocol.ExecutionScopeDM {
				snapshot.Assignments[0].OwnerAgentID = snapshot.Execution.CoordinatorAgentID
			}
			snapshot.Attempts[0].Status = protocol.WorkAttemptStatusInterrupted
			repository := &continuationRepositoryFake{fakeRepository: &fakeRepository{snapshot: snapshot}}
			service := testService(repository)
			service.SetScheduledWorkConsumer(continuationAdmission{})
			service.SetAssignmentTargetAuthorizer(assignmentTargetAuthorizerFunc(func(context.Context, AssignmentTargetRequest) error { return nil }))
			input := protocol.RequestExecutionRecovery{SessionKey: "session-1", RequestID: "user-request", WorkItemID: "work-1", AttemptID: "attempt-1"}
			for range 2 {
				receipt, err := service.RequestWorkRecovery(context.Background(), "owner-1", "execution-1", input)
				if err != nil || !receipt.Queued {
					t.Fatalf("request: %+v %v", receipt, err)
				}
			}
			if repository.writes != 1 || len(repository.snapshot.Attempts) != 2 || repository.snapshot.Attempts[0].Status != protocol.WorkAttemptStatusInterrupted {
				t.Fatal("continuation duplicated or rewrote old attempt")
			}
			// Read-only reconciliation survives service restart and later graph progress.
			repository.snapshot.Execution.Status = protocol.ExecutionStatusCompleted
			restarted := testService(repository)
			receipt, err := restarted.GetWorkRecoveryReceipt(context.Background(), "owner-1", "execution-1", "session-1", input.RequestID)
			if err != nil || !receipt.Queued || receipt.AttemptID != "attempt-1" {
				t.Fatalf("lost ACK: %+v %v", receipt, err)
			}
			input.WorkItemID = "other"
			if _, err = restarted.RequestWorkRecovery(context.Background(), "owner-1", "execution-1", input); err == nil {
				t.Fatal("same request changed intent")
			}
			if _, err = restarted.GetWorkRecoveryReceipt(context.Background(), "other-owner", "execution-1", "session-1", input.RequestID); err == nil {
				t.Fatal("cross-owner receipt")
			}
		})
	}
}

func TestUserRecoveryRejectsStaleOrNonRecoverableTargets(t *testing.T) {
	for name, mutate := range map[string]func(*protocol.ExecutionSnapshot){
		"running":    func(s *protocol.ExecutionSnapshot) { s.Attempts[0].Status = protocol.WorkAttemptStatusRunning },
		"child":      func(s *protocol.ExecutionSnapshot) { s.Attempts[0].ParentAttemptID = "parent" },
		"paused":     func(s *protocol.ExecutionSnapshot) { s.Execution.Status = protocol.ExecutionStatusPaused },
		"historical": func(s *protocol.ExecutionSnapshot) { s.Execution.ID = "new-execution" },
		"revoked":    func(s *protocol.ExecutionSnapshot) { s.Assignments[0].Status = protocol.WorkAssignmentStatusRevoked },
		"replan":     func(s *protocol.ExecutionSnapshot) { s.Plan.ID = "new-plan" },
		"submitted": func(s *protocol.ExecutionSnapshot) {
			s.Submissions = []protocol.WorkSubmission{{AttemptID: "attempt-1"}}
		},
	} {
		t.Run(name, func(t *testing.T) {
			snapshot := assignedExecutionSnapshot()
			snapshot.Attempts[0].Status = protocol.WorkAttemptStatusFailed
			mutate(snapshot)
			repository := &continuationRepositoryFake{fakeRepository: &fakeRepository{snapshot: snapshot}}
			service := testService(repository)
			service.SetScheduledWorkConsumer(continuationAdmission{})
			_, err := service.RequestWorkRecovery(context.Background(), "owner-1", "execution-1", protocol.RequestExecutionRecovery{SessionKey: "session-1", RequestID: "request", WorkItemID: "work-1", AttemptID: "attempt-1"})
			if err == nil || repository.writes != 0 {
				t.Fatalf("accepted %s: %v", name, err)
			}
		})
	}
}

type continuationAdmission struct{}

func (continuationAdmission) AutomaticDispatchAllowed(context.Context, protocol.Execution, string) (bool, error) {
	return false, nil
}
func (continuationAdmission) QueueScheduledWork(context.Context, protocol.Execution, protocol.ExecutionWorkBinding, string, string) error {
	return errors.New("unexpected automatic dispatch")
}
func (continuationAdmission) ContinuationPauseRevision(context.Context, protocol.Execution, string) (int64, error) {
	return 1, nil
}
