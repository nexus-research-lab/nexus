package orchestration

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type userRecoveryQueue struct {
	mu       sync.Mutex
	receipts map[string]protocol.ExecutionRecoveryTarget
	writes   int
}

func (q *userRecoveryQueue) QueueExecutionRecovery(context.Context, protocol.Execution, string) error {
	return errors.New("unexpected automatic recovery")
}
func (q *userRecoveryQueue) QueueWorkRecovery(_ context.Context, _ protocol.Execution, target protocol.ExecutionRecoveryTarget, key string) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if old, ok := q.receipts[key]; ok {
		if old != target {
			return errors.New("intent conflict")
		}
		return nil
	}
	q.receipts[key] = target
	q.writes++
	return nil
}
func (q *userRecoveryQueue) FindWorkRecovery(_ context.Context, _ protocol.Execution, key string) (*protocol.ExecutionRecoveryTarget, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if value, ok := q.receipts[key]; ok {
		return &value, nil
	}
	return nil, nil
}

func TestUserRecoveryExactTargetReplayAndReconciliation(t *testing.T) {
	for _, scope := range []protocol.ExecutionScopeKind{protocol.ExecutionScopeDM, protocol.ExecutionScopeRoom} {
		t.Run(string(scope), func(t *testing.T) {
			snapshot := assignedExecutionSnapshot()
			snapshot.Execution.ScopeKind = scope
			snapshot.Attempts[0].Status = protocol.WorkAttemptStatusInterrupted
			snapshot.Attempts[0].FailureReason = "__nexus_interrupt_without_message__"
			queue := &userRecoveryQueue{receipts: map[string]protocol.ExecutionRecoveryTarget{}}
			service := testService(&fakeRepository{snapshot: snapshot})
			service.SetExecutionRecoveryConsumer(queue)
			input := protocol.RequestExecutionRecovery{SessionKey: "session-1", RequestID: "user-request", WorkItemID: "work-1", AttemptID: "attempt-1"}
			var group sync.WaitGroup
			for range 8 {
				group.Go(func() {
					receipt, err := service.RequestWorkRecovery(context.Background(), "owner-1", "execution-1", input)
					if err != nil || !receipt.Queued {
						t.Errorf("request = %+v, %v", receipt, err)
					}
				})
			}
			group.Wait()
			if queue.writes != 1 || snapshot.Attempts[0].Status != protocol.WorkAttemptStatusInterrupted || snapshot.Execution.Version != 10 {
				t.Fatal("duplicate enqueue or premature domain mutation")
			}
			target := protocol.ExecutionRecoveryTarget{ExecutionID: "execution-1", WorkItemID: "work-1", AttemptID: "attempt-1"}
			if current, err := service.RecoveryTargetCurrent(context.Background(), "owner-1", "session-1", target); err != nil || !current {
				t.Fatalf("current=%v err=%v", current, err)
			}
			snapshot.Attempts = append(snapshot.Attempts, protocol.WorkAttempt{ID: "new-attempt", AssignmentID: "assignment-1", Status: protocol.WorkAttemptStatusRunning, CreatedAt: time.Now()})
			if current, err := service.RecoveryTargetCurrent(context.Background(), "owner-1", "session-1", target); err != nil || current {
				t.Fatalf("old target dispatched: %v %v", current, err)
			}
			// A lost ACK is still readable/replayable after the graph has advanced.
			receipt, err := service.RequestWorkRecovery(context.Background(), "owner-1", "execution-1", input)
			if err != nil || !receipt.Queued || queue.writes != 1 {
				t.Fatalf("lost ACK: %+v %v", receipt, err)
			}
			input.WorkItemID = "another-work"
			if _, err := service.RequestWorkRecovery(context.Background(), "owner-1", "execution-1", input); err == nil {
				t.Fatal("same request changed intent")
			}
			if _, err := service.GetWorkRecoveryReceipt(context.Background(), "other-owner", "execution-1", "session-1", input.RequestID); err == nil {
				t.Fatal("receipt leaked to another owner")
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
			queue := &userRecoveryQueue{receipts: map[string]protocol.ExecutionRecoveryTarget{}}
			service := testService(&fakeRepository{snapshot: snapshot})
			service.SetExecutionRecoveryConsumer(queue)
			_, err := service.RequestWorkRecovery(context.Background(), "owner-1", "execution-1", protocol.RequestExecutionRecovery{SessionKey: "session-1", RequestID: "request", WorkItemID: "work-1", AttemptID: "attempt-1"})
			if err == nil || queue.writes != 0 {
				t.Fatalf("accepted %s: %v", name, err)
			}
		})
	}
}
