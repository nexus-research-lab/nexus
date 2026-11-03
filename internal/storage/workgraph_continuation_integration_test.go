// INPUT: Migrated SQL, stopped responsibility and concurrent explicit user continuation.
// OUTPUT: Same-owner atomic continuation, durable dispatch after restart and preserved prior attempts.
// POS: Service/storage integration; runtime delivery is observed through a controlled consumer.
package storage_test

import (
	"context"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	svc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	repo "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
	workspace "github.com/nexus-research-lab/nexus/internal/storage/workspace"
	"sync"
	"testing"
)

type continuationDelivery struct {
	policyAdmission
	binding         *protocol.ExecutionWorkBinding
	target          string
	continuationErr error
}

func (*continuationDelivery) ContinuationPauseRevision(context.Context, protocol.Execution, string) (int64, error) {
	return 3, nil
}
func (p *continuationDelivery) QueueContinuedWork(_ context.Context, _ protocol.Execution, b protocol.ExecutionWorkBinding, _, key string, revision int64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.continuationErr != nil {
		return p.continuationErr
	}
	p.binding = &b
	p.receipts[key] = true
	return nil
}
func (p *continuationDelivery) DeliverExecutionDispatch(_ context.Context, d svc.ExecutionDispatchDelivery) (svc.ExecutionDispatchReceipt, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.continuationErr != nil {
		return svc.ExecutionDispatchReceipt{}, svc.PermanentExecutionDispatchDelivery(p.continuationErr)
	}
	p.binding = &d.Binding
	p.target = d.TargetAgentID
	p.receipts[d.DispatchDedupeKey] = true
	if !d.UserContinuation || d.ContinuationPauseRevision != 3 {
		panic("missing explicit continuation admission")
	}
	return svc.ExecutionDispatchReceipt{QueueItemID: "continued"}, nil
}

func TestWorkGraphSameOwnerContinuationIntegration(t *testing.T) {
	for _, driver := range []string{"sqlite", "postgres"} {
		t.Run(driver, func(t *testing.T) {
			cfg := integrationDatabase(t, driver)
			migrateIntegrationDatabase(t, cfg, 0)
			db := openIntegrationDatabase(t, cfg)
			repository := repo.NewSQLRepository(driver, db)
			for _, scope := range []protocol.ExecutionScopeKind{protocol.ExecutionScopeDM, protocol.ExecutionScopeRoom} {
				t.Run(string(scope), func(t *testing.T) {
					batch := prepareIntegrationBatchScope(t, repository, "continue-"+string(scope), scope)
					assignment := batch.Assignments[0]
					if scope == protocol.ExecutionScopeDM {
						assignment.Assignment.OwnerAgentID = "lead"
						assignment.Assignment.Strategy = protocol.AssignmentStrategySelf
						assignment.RootAttempt.ExecutorAgentID = "lead"
						assignment.Dispatch = nil
					}
					snapshot, err := repository.Assign(t.Context(), assignment)
					if err != nil {
						t.Fatal(err)
					}
					old := snapshot.Attempts[0]
					old.Status = protocol.WorkAttemptStatusInterrupted
					old.FailureReason = protocol.InterruptHostShutdown
					snapshot, err = repository.FinishAttempt(t.Context(), repo.FinishAttemptCommand{ExpectedExecutionVersion: snapshot.Execution.Version, ExpectedAttemptVersion: old.Version, Attempt: old, Meta: integrationCommandMeta("stop-" + string(scope))})
					if err != nil {
						t.Fatal(err)
					}
					delivery := &continuationDelivery{policyAdmission: policyAdmission{paused: true, receipts: map[string]bool{}}}
					service := svc.NewService(repository)
					service.SetAssignmentTargetAuthorizer(policyTargetAuthorizer{})
					service.SetScheduledWorkConsumer(delivery)
					input := protocol.RequestExecutionRecovery{SessionKey: snapshot.Execution.SessionKey, RequestID: "continue", WorkItemID: old.WorkItemID, AttemptID: old.ID}
					var wg sync.WaitGroup
					for range 4 {
						wg.Go(func() {
							_, _ = service.RequestWorkRecovery(t.Context(), snapshot.Execution.OwnerUserID, snapshot.Execution.ID, input)
						})
					}
					wg.Wait()
					receipt, err := service.GetWorkRecoveryReceipt(t.Context(), snapshot.Execution.OwnerUserID, snapshot.Execution.ID, input.SessionKey, input.RequestID)
					if err != nil || !receipt.Queued {
						t.Fatalf("receipt %+v %v", receipt, err)
					}
					current, err := repository.GetSnapshot(t.Context(), snapshot.Execution.ID)
					if err != nil {
						t.Fatal(err)
					}
					if len(current.Assignments) != 1 || len(current.Attempts) != 2 || len(current.Submissions) != 0 || len(current.Acceptances) != 0 {
						t.Fatalf("unexpected projection: assignments=%d attempts=%d submissions=%d acceptances=%d", len(current.Assignments), len(current.Attempts), len(current.Submissions), len(current.Acceptances))
					}
					var replacement protocol.WorkAssignment
					for _, a := range current.Assignments {
						if a.ID != assignment.Assignment.ID {
							replacement = a
						}
					}
					if replacement.OwnerAgentID != assignment.Assignment.OwnerAgentID || replacement.ReturnToAgentID != assignment.Assignment.ReturnToAgentID {
						t.Fatal("continuation changed owner/reviewer")
					}
					// A fresh service drains the durable outbox, even though automatic work remains paused.
					restarted := svc.NewService(repository)
					restarted.SetAssignmentTargetAuthorizer(policyTargetAuthorizer{})
					restarted.SetScheduledWorkConsumer(delivery)
					restarted.SetExecutionDispatchConsumer(delivery)
					if scope == protocol.ExecutionScopeDM {
						_, err = restarted.ReconcileDispatchPolicies(t.Context(), 32)
					} else {
						_, err = restarted.DispatchPending(t.Context(), "restart-worker", 32)
					}
					if err != nil || delivery.binding == nil || delivery.binding.AttemptID == old.ID || delivery.binding.WorkItemID != old.WorkItemID {
						t.Fatalf("restart delivery %+v %v", delivery.binding, err)
					}
					if scope == protocol.ExecutionScopeRoom && delivery.target != assignment.Assignment.OwnerAgentID {
						t.Fatal("delivered to coordinator instead of original worker")
					}
					input.RequestID = "different-click"
					if _, err = restarted.RequestWorkRecovery(t.Context(), snapshot.Execution.OwnerUserID, snapshot.Execution.ID, input); err == nil {
						t.Fatal("old attempt admitted twice")
					}
					{
						current, _ = repository.GetSnapshot(t.Context(), snapshot.Execution.ID)
						var stopped protocol.WorkAttempt
						for _, candidate := range current.Attempts {
							if candidate.ID == delivery.binding.AttemptID {
								stopped = candidate
							}
						}
						stopped.Status = protocol.WorkAttemptStatusInterrupted
						_, err = repository.FinishAttempt(t.Context(), repo.FinishAttemptCommand{ExpectedExecutionVersion: current.Execution.Version, ExpectedAttemptVersion: stopped.Version, Attempt: stopped, Meta: integrationCommandMeta("second-stop-" + string(scope))})
						if err != nil {
							t.Fatal(err)
						}
						input.AttemptID, input.RequestID = stopped.ID, "continue-before-later-stop"
						if _, err = restarted.RequestWorkRecovery(t.Context(), snapshot.Execution.OwnerUserID, snapshot.Execution.ID, input); err != nil {
							t.Fatal(err)
						}
						delivery.continuationErr = workspace.ErrContinuationStopped
						if scope == protocol.ExecutionScopeDM {
							_, err = restarted.ReconcileDispatchPolicies(t.Context(), 32)
						} else {
							_, err = restarted.DispatchPending(t.Context(), "late-stop-worker", 32)
						}
						if err != nil {
							t.Fatal(err)
						}
						current, _ = repository.GetSnapshot(t.Context(), snapshot.Execution.ID)
						for _, candidate := range current.Attempts {
							if candidate.ID != old.ID && candidate.ID != stopped.ID {
								if candidate.Status != protocol.WorkAttemptStatusInterrupted {
									t.Fatalf("late stop stranded attempt: %+v", candidate)
								}
								input.AttemptID = candidate.ID
							}
						}
						input.RequestID = "fresh-explicit-continuation"
						if _, err = restarted.RequestWorkRecovery(t.Context(), snapshot.Execution.OwnerUserID, snapshot.Execution.ID, input); err != nil {
							t.Fatal(err)
						}
					}
				})
			}
		})
	}
}
