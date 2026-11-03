// INPUT: Real migrated databases and injected failures around result domain commits.
// OUTPUT: Durable delivery/review, immutable conflict detection and restart proof without replaying effects.
// POS: Cross-database integration of result intake and existing WorkGraph transactions.
package storage_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	svc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	repo "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

type resultFaultRepository struct {
	*repo.Repository
	failFinish, failSettle, failAfterSubmit bool
}

func (r *resultFaultRepository) FinishAttempt(ctx context.Context, command repo.FinishAttemptCommand) (*protocol.ExecutionSnapshot, error) {
	if r.failFinish {
		return nil, errors.New("injected crash before finishing Attempt")
	}
	return r.Repository.FinishAttempt(ctx, command)
}
func (r *resultFaultRepository) Submit(ctx context.Context, command repo.SubmitCommand) (*protocol.ExecutionSnapshot, error) {
	result, err := r.Repository.Submit(ctx, command)
	if err == nil && r.failAfterSubmit {
		return nil, errors.New("injected lost committed submission reply")
	}
	return result, err
}
func (r *resultFaultRepository) SetRuntimeResultState(ctx context.Context, id, state, reason string, next time.Time) error {
	if r.failSettle {
		return errors.New("injected receipt settlement failure")
	}
	return r.Repository.SetRuntimeResultState(ctx, id, state, reason, next)
}

func TestWorkGraphRuntimeResultIntegration(t *testing.T) {
	for _, driver := range []string{"sqlite", "postgres"} {
		t.Run(driver, func(t *testing.T) {
			cfg := integrationDatabase(t, driver)
			migrateIntegrationDatabase(t, cfg, 0)
			database := openIntegrationDatabase(t, cfg)
			repository := repo.NewSQLRepository(driver, database)
			for _, mode := range []string{"normal", "before_finish", "after_submit", "before_receipt"} {
				t.Run(mode, func(t *testing.T) {
					actor, declaration := runtimeResultFixture(t, repository, mode)
					faults := &resultFaultRepository{Repository: repository, failFinish: mode == "before_finish", failAfterSubmit: mode == "after_submit", failSettle: mode == "before_receipt"}
					service := svc.NewService(faults)
					outcome, err := service.AcceptRuntimeResult(t.Context(), actor, declaration)
					if mode == "normal" && (err != nil || outcome.State != "applied") {
						t.Fatalf("delivery = %+v %v", outcome, err)
					}
					if mode != "normal" && (err != nil || outcome.State != "pending" || outcome.Reason != "reconcile_required") {
						t.Fatalf("durable intake was confused with domain success: %+v %v", outcome, err)
					}
					// Recreate the service, as on restart. Intake must survive and prevent
					// orphan settlement from destroying a declared delivery in progress.
					restarted := svc.NewService(repository)
					if err := restarted.RecoverRootAttempts(t.Context()); err != nil {
						t.Fatal(err)
					}
					if _, err := restarted.ReconcileRuntimeResults(t.Context(), 64); err != nil {
						t.Fatal(err)
					}
					again, err := restarted.AcceptRuntimeResult(t.Context(), actor, declaration)
					if err != nil || again.State != "applied" {
						t.Fatalf("reconciled=%+v %v", again, err)
					}
					snapshot, err := repository.GetSnapshot(t.Context(), actor.ExecutionID)
					if err != nil || len(snapshot.Submissions) != 1 || len(snapshot.ReviewDispatches) != 1 || len(snapshot.Acceptances) != 0 {
						t.Fatalf("unproven/duplicate delivery: %+v %v", snapshot, err)
					}
					changed := declaration
					delivery := *declaration.Delivery
					changed.Delivery = &delivery
					changed.Delivery.ResultSummary = "different result"
					if _, err := restarted.AcceptRuntimeResult(t.Context(), actor, changed); !errors.Is(err, repo.ErrRuntimeResultConflict) {
						t.Fatalf("changed result accepted: %v", err)
					}
					checkStructuredReview(t, repository, restarted, snapshot)
				})
			}
			t.Run("concurrent_tool_and_hook", func(t *testing.T) {
				actor, declaration := runtimeResultFixture(t, repository, "race")
				service := svc.NewService(repository)
				snapshot, _ := repository.GetSnapshot(t.Context(), actor.ExecutionID)
				var group sync.WaitGroup
				group.Go(func() { _, _ = service.AcceptRuntimeResult(t.Context(), actor, declaration) })
				group.Go(func() {
					_, _ = service.SubmitWork(t.Context(), actor, svc.SubmitWorkInput{ExecutionID: actor.ExecutionID, SnapshotRevision: snapshot.Execution.Version, CommandID: "explicit-result", ResultSummary: declaration.Delivery.ResultSummary, Evidence: declaration.Delivery.Evidence})
				})
				group.Wait()
				if _, err := service.ReconcileRuntimeResults(t.Context(), 64); err != nil {
					t.Fatal(err)
				}
				outcome, err := service.AcceptRuntimeResult(t.Context(), actor, declaration)
				if err != nil || outcome.State != "applied" {
					t.Fatalf("race result=%+v %v", outcome, err)
				}
				snapshot, _ = repository.GetSnapshot(t.Context(), actor.ExecutionID)
				if len(snapshot.Submissions) != 1 || len(snapshot.ReviewDispatches) != 1 {
					t.Fatal("race duplicated delivery")
				}
			})
			t.Run("terminal_after_intake", func(t *testing.T) {
				actor, declaration := runtimeResultFixture(t, repository, "terminal")
				service := svc.NewService(&resultFaultRepository{Repository: repository, failFinish: true})
				intake, err := service.AcceptRuntimeResult(t.Context(), actor, declaration)
				if err != nil || intake.State != "pending" {
					t.Fatalf("intake: %+v %v", intake, err)
				}
				snapshot, _ := repository.GetSnapshot(t.Context(), actor.ExecutionID)
				if _, err := repository.Abandon(t.Context(), repo.AbandonCommand{ExecutionID: actor.ExecutionID, ExpectedExecutionVersion: snapshot.Execution.Version, Reason: "user cancelled", Meta: integrationCommandMeta("cancel-result")}); err != nil {
					t.Fatal(err)
				}
				service = svc.NewService(repository)
				if _, err := service.ReconcileRuntimeResults(t.Context(), 64); err != nil {
					t.Fatal(err)
				}
				receipt, err := repository.GetRuntimeResultReceipt(t.Context(), intake.ReceiptID)
				if err != nil || receipt.State != "rejected" {
					t.Fatalf("terminal receipt did not settle: %+v %v", receipt, err)
				}
				snapshot, _ = repository.GetSnapshot(t.Context(), actor.ExecutionID)
				if len(snapshot.Submissions) != 0 {
					t.Fatal("cancelled work was delivered")
				}
			})
			t.Run("wrong_scope_and_round", func(t *testing.T) {
				actor, declaration := runtimeResultFixture(t, repository, "wrong-scope")
				service := svc.NewService(repository)
				for _, change := range []func(*svc.ActorContext){
					func(a *svc.ActorContext) { a.ScopeKind = protocol.ExecutionScopeDM },
					func(a *svc.ActorContext) { a.RoomID = "" },
					func(a *svc.ActorContext) { a.RuntimeRoundID = "old-round" },
					func(a *svc.ActorContext) { a.WorkBinding.AttemptID = "unknown-attempt" },
				} {
					candidate := actor
					candidate.WorkBinding = actor.WorkBinding.Clone()
					change(&candidate)
					if _, err := service.AcceptRuntimeResult(t.Context(), candidate, declaration); err == nil {
						t.Fatal("invalid host identity accepted")
					}
				}
				pending, err := repository.ListPendingRuntimeResults(t.Context(), time.Now().Add(time.Minute), 64)
				if err != nil || len(pending) != 0 {
					t.Fatalf("invalid result left intake: %+v %v", pending, err)
				}
			})
			t.Run("intake_cannot_move_to_new_attempt", func(t *testing.T) {
				actor, declaration := runtimeResultFixture(t, repository, "new-attempt")
				service := svc.NewService(&resultFaultRepository{Repository: repository, failFinish: true})
				intake, err := service.AcceptRuntimeResult(t.Context(), actor, declaration)
				if err != nil || intake.State != "pending" {
					t.Fatalf("intake=%+v %v", intake, err)
				}
				snapshot, _ := repository.GetSnapshot(t.Context(), actor.ExecutionID)
				attempt := snapshot.Attempts[0]
				attempt.Status = protocol.WorkAttemptStatusInterrupted
				snapshot, err = repository.FinishAttempt(t.Context(), repo.FinishAttemptCommand{ExpectedExecutionVersion: snapshot.Execution.Version, ExpectedAttemptVersion: attempt.Version, Attempt: attempt, Meta: integrationCommandMeta("interrupt-old-result")})
				if err != nil {
					t.Fatal(err)
				}
				attempt.ID, attempt.AgentRoundID, attempt.RuntimeRoundID = "fresh-root", "fresh-agent-round", "fresh-runtime-round"
				attempt.Version = 0
				attempt.CreatedAt = time.Time{}
				attempt.StartedAt, attempt.FinishedAt = nil, nil
				_, err = repository.StartAttempt(t.Context(), repo.StartAttemptCommand{ExpectedExecutionVersion: snapshot.Execution.Version, ExpectedAssignmentVersion: snapshot.Assignments[0].Version, Attempt: attempt, Meta: integrationCommandMeta("start-new-result")})
				if err != nil {
					t.Fatal(err)
				}
				service = svc.NewService(repository)
				if _, err := service.ReconcileRuntimeResults(t.Context(), 64); err != nil {
					t.Fatal(err)
				}
				receipt, err := repository.GetRuntimeResultReceipt(t.Context(), intake.ReceiptID)
				if err != nil || receipt.State != "rejected" {
					t.Fatalf("stale intake=%+v %v", receipt, err)
				}
				snapshot, _ = repository.GetSnapshot(t.Context(), actor.ExecutionID)
				if len(snapshot.Submissions) != 0 {
					t.Fatal("old declaration moved to new Attempt")
				}
				for _, current := range snapshot.Attempts {
					if current.ID == "fresh-root" && current.Status != protocol.WorkAttemptStatusRunning {
						t.Fatal("new Attempt was closed by old receipt")
					}
				}
			})
			for _, conflict := range []bool{false, true} {
				name := "review_tool_and_hook"
				if conflict {
					name += "_conflict"
				}
				t.Run(name, func(t *testing.T) {
					worker, delivery := runtimeResultFixture(t, repository, name)
					service := svc.NewService(repository)
					if outcome, err := service.AcceptRuntimeResult(t.Context(), worker, delivery); err != nil || outcome.State != "applied" {
						t.Fatalf("delivery=%+v %v", outcome, err)
					}
					snapshot, _ := repository.GetSnapshot(t.Context(), worker.ExecutionID)
					actor, declaration := runtimeReviewFixture(snapshot)
					decision := protocol.WorkAcceptanceAccepted
					if conflict {
						decision = protocol.WorkAcceptanceChangesRequested
					}
					var group sync.WaitGroup
					group.Go(func() { _, _ = service.AcceptRuntimeResult(t.Context(), actor, declaration) })
					group.Go(func() {
						_, _ = service.ReviewWork(t.Context(), actor, svc.ReviewWorkInput{ExecutionID: actor.ExecutionID, SnapshotRevision: snapshot.Execution.Version, CommandID: "explicit-review", Decision: decision, CriteriaResults: declaration.Review.CriteriaResults})
					})
					group.Wait()
					if _, err := service.ReconcileRuntimeResults(t.Context(), 64); err != nil {
						t.Fatal(err)
					}
					outcome, resultErr := service.AcceptRuntimeResult(t.Context(), actor, declaration)
					after, err := repository.GetSnapshot(t.Context(), actor.ExecutionID)
					if err != nil || len(after.Acceptances) != 1 {
						t.Fatalf("review duplicated: %+v %v", after, err)
					}
					if after.Acceptances[0].Decision == protocol.WorkAcceptanceAccepted {
						if resultErr != nil || outcome.State != "applied" {
							t.Fatalf("matching winner unproven: %+v %v", outcome, resultErr)
						}
					} else if outcome.State != "rejected" && !errors.Is(resultErr, repo.ErrRuntimeResultConflict) {
						t.Fatalf("conflicting decision overwritten: %+v %v", outcome, resultErr)
					}
				})
			}
		})
	}
}

func runtimeResultFixture(t *testing.T, repository *repo.Repository, suffix string) (svc.ActorContext, protocol.ExecutionResultDeclaration) {
	t.Helper()
	batch := prepareIntegrationBatchScope(t, repository, "result-"+suffix, protocol.ExecutionScopeRoom)
	snapshot, err := repository.AssignBatch(t.Context(), batch)
	if err != nil {
		t.Fatal(err)
	}
	assignment, attempt := snapshot.Assignments[0], snapshot.Attempts[0]
	actor := svc.ActorContext{OwnerUserID: snapshot.Execution.OwnerUserID, SessionKey: snapshot.Execution.SessionKey, ExecutionID: snapshot.Execution.ID, ScopeKind: protocol.ExecutionScopeRoom, RoomID: snapshot.Execution.RoomID, ConversationID: snapshot.Execution.ConversationID, AgentID: assignment.OwnerAgentID, Role: svc.ExecutionActorMember, RuntimeRoundID: "root-" + suffix, AgentRoundID: "agent-round-" + suffix, WorkBinding: &protocol.ExecutionWorkBinding{ExecutionID: assignment.ExecutionID, PlanID: assignment.PlanID, WorkItemID: assignment.WorkItemID, SpecID: assignment.SpecID, AssignmentID: assignment.ID, AttemptID: attempt.ID, DispatchID: attempt.DispatchID}}
	attempt.RuntimeRoundID = actor.RuntimeRoundID
	attempt.AgentRoundID = actor.AgentRoundID
	if _, err := repository.StartAttempt(t.Context(), repo.StartAttemptCommand{ExpectedExecutionVersion: snapshot.Execution.Version, ExpectedAssignmentVersion: assignment.Version, ExpectedAttemptVersion: attempt.Version, Attempt: attempt, Meta: integrationCommandMeta("start-result-" + suffix)}); err != nil {
		t.Fatal(err)
	}
	return actor, protocol.ExecutionResultDeclaration{Version: 1, RoundID: actor.AgentRoundID, Delivery: &protocol.ExecutionDeliveryResult{ResultSummary: "delivered evidence", Evidence: []string{"verified"}}}
}

func checkStructuredReview(t *testing.T, repository *repo.Repository, service *svc.Service, snapshot *protocol.ExecutionSnapshot) {
	t.Helper()
	actor, declaration := runtimeReviewFixture(snapshot)
	outcome, err := service.AcceptRuntimeResult(t.Context(), actor, declaration)
	if err != nil || outcome.State != "applied" {
		t.Fatalf("review=%+v %v", outcome, err)
	}
	if _, err := service.AcceptRuntimeResult(t.Context(), actor, declaration); err != nil {
		t.Fatal(err)
	}
	declaration.Review.Decision = protocol.WorkAcceptanceChangesRequested
	if _, err := service.AcceptRuntimeResult(t.Context(), actor, declaration); !errors.Is(err, repo.ErrRuntimeResultConflict) {
		t.Fatalf("review decision overwritten: %v", err)
	}
	after, err := repository.GetSnapshot(t.Context(), actor.ExecutionID)
	if err != nil || len(after.Acceptances) != 1 || after.Acceptances[0].Decision != protocol.WorkAcceptanceAccepted {
		t.Fatalf("acceptances=%+v %v", after, err)
	}
}

func runtimeReviewFixture(snapshot *protocol.ExecutionSnapshot) (svc.ActorContext, protocol.ExecutionResultDeclaration) {
	dispatch := snapshot.ReviewDispatches[0]
	actor := svc.ActorContext{OwnerUserID: snapshot.Execution.OwnerUserID, SessionKey: snapshot.Execution.SessionKey, ExecutionID: snapshot.Execution.ID, ScopeKind: protocol.ExecutionScopeRoom, RoomID: snapshot.Execution.RoomID, ConversationID: snapshot.Execution.ConversationID, AgentID: dispatch.TargetAgentID, Role: svc.ExecutionActorCoordinator, RuntimeRoundID: "review-root", AgentRoundID: "review-agent-round", ReviewBinding: &protocol.ExecutionReviewBinding{ExecutionID: dispatch.ExecutionID, PlanID: dispatch.PlanID, WorkItemID: dispatch.WorkItemID, SpecID: dispatch.SpecID, AssignmentID: dispatch.AssignmentID, SubmissionID: dispatch.SubmissionID, ReviewDispatchID: dispatch.ID, TargetAgentID: dispatch.TargetAgentID}}
	declaration := protocol.ExecutionResultDeclaration{Version: 1, RoundID: actor.AgentRoundID, Review: &protocol.ExecutionReviewResult{Decision: protocol.WorkAcceptanceAccepted, CriteriaResults: []protocol.WorkAcceptanceCriterionResult{{Criterion: "verified", Passed: true, Evidence: []string{"checked artifact"}}}}}
	return actor, declaration
}
