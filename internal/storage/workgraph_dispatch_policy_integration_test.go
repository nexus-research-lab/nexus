// INPUT: Migrated SQLite/PostgreSQL, explicit routing and real domain review commands.
// OUTPUT: Dependency gating, exactly-once assignment, pause/restart and immutable intent proofs.
// POS: Declarative scheduler database integration; queue acceptance is a controlled port here.
package storage_test

import (
	"context"
	"sync"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	svc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	repo "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

type policyAdmission struct {
	mu       sync.Mutex
	paused   bool
	receipts map[string]bool
}

func (p *policyAdmission) AutomaticDispatchAllowed(context.Context, protocol.Execution, string) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return !p.paused, nil
}
func (p *policyAdmission) QueueScheduledWork(_ context.Context, _ protocol.Execution, _ protocol.ExecutionWorkBinding, _, key string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.receipts[key] = true
	return nil
}

type policyTargetAuthorizer struct{}

func (policyTargetAuthorizer) AuthorizeAssignmentTarget(context.Context, svc.AssignmentTargetRequest) error {
	return nil
}

func TestWorkGraphDispatchPolicyIntegration(t *testing.T) {
	for _, driver := range []string{"sqlite", "postgres"} {
		t.Run(driver, func(t *testing.T) {
			cfg := integrationDatabase(t, driver)
			migrateIntegrationDatabase(t, cfg, 0)
			db := openIntegrationDatabase(t, cfg)
			repository := repo.NewSQLRepository(driver, db)
			for _, scope := range []protocol.ExecutionScopeKind{protocol.ExecutionScopeDM, protocol.ExecutionScopeRoom} {
				t.Run(string(scope), func(t *testing.T) {
					batch := prepareIntegrationBatchScope(t, repository, "policy-"+string(scope), scope, true)
					snapshot, _ := repository.GetSnapshot(t.Context(), batch.ExecutionID)
					actor := svc.ActorContext{OwnerUserID: snapshot.Execution.OwnerUserID, SessionKey: snapshot.Execution.SessionKey, AgentID: "lead", ScopeKind: scope, RoomID: snapshot.Execution.RoomID, ConversationID: snapshot.Execution.ConversationID, ExecutionID: snapshot.Execution.ID, Role: svc.ExecutionActorCoordinator, RootRoundID: "policy-round", RuntimeRoundID: "policy-round", AgentRoundID: "policy-agent-round"}
					admission := &policyAdmission{receipts: map[string]bool{}}
					service := svc.NewService(repository)
					service.SetScheduledWorkConsumer(admission)
					service.SetAssignmentTargetAuthorizer(policyTargetAuthorizer{})
					if err := service.ActivateRuntimeCoordination(t.Context(), actor, snapshot); err != nil {
						t.Fatal(err)
					}
					input := svc.ConfigureDispatchInput{ExecutionID: batch.ExecutionID, SnapshotRevision: snapshot.Execution.Version, CommandID: "configure-" + string(scope)}
					for _, work := range snapshot.WorkItems {
						input.Policies = append(input.Policies, svc.DispatchPolicyInput{WorkItemID: work.ID, TargetAgentID: "lead", ReturnToAgentID: "lead"})
					}
					configured, err := service.ConfigureDispatch(t.Context(), actor, input)
					if err != nil || configured.Outcome != svc.MutationApplied {
						t.Fatalf("configure=%+v %v", configured, err)
					}
					replay, err := service.ConfigureDispatch(t.Context(), actor, input)
					if err != nil || replay.Outcome != svc.MutationNoOp {
						t.Fatalf("stale exact replay=%+v %v", replay, err)
					}
					input.Policies[0].Instruction = "different intent"
					changed, err := service.ConfigureDispatch(t.Context(), actor, input)
					if err != nil || changed.Outcome != svc.MutationRejected {
						t.Fatalf("changed intent=%+v %v", changed, err)
					}
					// A reconstructed scheduler uses only durable policy authority, not the old round map.
					service = svc.NewService(repository)
					service.SetScheduledWorkConsumer(admission)
					service.SetAssignmentTargetAuthorizer(policyTargetAuthorizer{})
					var group sync.WaitGroup
					for range 2 {
						group.Go(func() { _, _ = service.ReconcileDispatchPolicies(t.Context(), 32) })
					}
					group.Wait()
					snapshot, _ = repository.GetSnapshot(t.Context(), batch.ExecutionID)
					if len(snapshot.Assignments) != 1 || len(snapshot.Attempts) != 1 {
						t.Fatalf("dependency/concurrency admitted %d assignments", len(snapshot.Assignments))
					}
					a, attempt := snapshot.Assignments[0], snapshot.Attempts[0]
					bound := actor
					bound.Role = svc.ExecutionActorMember
					bound.WorkBinding = &protocol.ExecutionWorkBinding{ExecutionID: a.ExecutionID, PlanID: a.PlanID, WorkItemID: a.WorkItemID, SpecID: a.SpecID, AssignmentID: a.ID, AttemptID: attempt.ID, DispatchID: attempt.DispatchID}
					bound.AgentRoundID = "worker-round"
					bound.RuntimeRoundID = "worker-runtime"
					if scope == protocol.ExecutionScopeDM {
						if _, err = service.PrepareScheduledDMRound(t.Context(), bound); err != nil {
							t.Fatal(err)
						}
					}
					snapshot, _ = repository.GetSnapshot(t.Context(), batch.ExecutionID)
					submitted, err := service.SubmitWork(t.Context(), bound, svc.SubmitWorkInput{ExecutionID: batch.ExecutionID, SnapshotRevision: snapshot.Execution.Version, CommandID: "deliver-" + string(scope), ResultSummary: "verified", Evidence: []string{"checked"}})
					if err != nil || submitted.Outcome != svc.MutationApplied {
						t.Fatalf("submit=%+v %v", submitted, err)
					}
					if len(submitted.Snapshot.ReviewDispatches) != 1 {
						t.Fatal("automatic self work has no review outbox")
					}
					if _, err = service.ReconcileDispatchPolicies(t.Context(), 32); err != nil {
						t.Fatal(err)
					}
					still, _ := repository.GetSnapshot(t.Context(), batch.ExecutionID)
					if len(still.Assignments) != 1 {
						t.Fatal("submission unlocked dependency before acceptance")
					}
					reviewer, _ := runtimeReviewFixture(submitted.Snapshot)
					reviewed, err := service.ReviewWork(t.Context(), reviewer, svc.ReviewWorkInput{ExecutionID: batch.ExecutionID, SnapshotRevision: submitted.Snapshot.Execution.Version, CommandID: "review-" + string(scope), Decision: protocol.WorkAcceptanceAccepted, CriteriaResults: []protocol.WorkAcceptanceCriterionResult{{Criterion: "verified", Passed: true, Evidence: []string{"checked"}}}})
					if err != nil || reviewed.Outcome != svc.MutationApplied {
						t.Fatalf("review=%+v %v", reviewed, err)
					}
					admission.paused = true
					if _, err = service.ReconcileDispatchPolicies(t.Context(), 32); err != nil {
						t.Fatal(err)
					}
					paused, _ := repository.GetSnapshot(t.Context(), batch.ExecutionID)
					if len(paused.Assignments) != 1 {
						t.Fatal("paused queue was automatically resumed")
					}
					admission.paused = false
					// Move only this fixture's deferred deadline, without sleeping or mutating domain state.
					stmt := `UPDATE execution_dispatch_policies SET next_attempt_at=created_at WHERE execution_id=?`
					if driver == "postgres" {
						stmt = `UPDATE execution_dispatch_policies SET next_attempt_at=created_at WHERE execution_id=$1`
					}
					if _, err = db.ExecContext(t.Context(), stmt, batch.ExecutionID); err != nil {
						t.Fatal(err)
					}
					if _, err = service.ReconcileDispatchPolicies(t.Context(), 32); err != nil {
						t.Fatal(err)
					}
					final, _ := repository.GetSnapshot(t.Context(), batch.ExecutionID)
					if len(final.Assignments) != 2 || len(final.Attempts) != 2 {
						t.Fatalf("accepted dependency did not dispatch: %+v", final.DispatchPolicies)
					}
					if scope == protocol.ExecutionScopeRoom && len(final.Dispatches) != 2 {
						t.Fatal("Room self policy did not use durable outbox")
					}
					if scope == protocol.ExecutionScopeDM && len(admission.receipts) != 2 {
						t.Fatal("DM policy did not enter durable queue port")
					}

				})
			}
		})
	}
}
