// workgraph_runtime_test.go 覆盖 WorkGraph 管线运行、结果与重试集成测试。
package storage_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	bridge "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/mcp/command"
	"github.com/nexus-research-lab/nexus/internal/mcp/command/execution/contract"
	"github.com/nexus-research-lab/nexus/internal/mcp/command/execution/operation"
	"github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtime "github.com/nexus-research-lab/nexus/internal/runtime"
	svc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	"github.com/nexus-research-lab/nexus/internal/service/orchestration/runtimehook"
	repo "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

func TestWorkGraphPipelineRuntimeAcceptance(t *testing.T) {
	binary, model := os.Getenv("NEXUS_TEST_RESULT_RUNTIME_BINARY"), os.Getenv("NEXUS_TEST_RESULT_MODEL")
	if os.Getenv("NEXUS_TEST_PIPELINE_RUNTIME") != "1" || binary == "" || model == "" {
		t.Skip("explicit pipeline provider acceptance opt-in required")
	}
	cfg := integrationDatabase(t, "sqlite")
	migrateIntegrationDatabase(t, cfg, 0)
	repository := repo.NewSQLRepository("sqlite", openIntegrationDatabase(t, cfg))
	for sample := range 3 {
		modes := []string{"tool", "hook"}
		if sample%2 == 1 {
			modes = []string{"hook", "tool"}
		}
		for _, mode := range modes {
			t.Run(fmt.Sprintf("pair%d_%s", sample+1, mode), func(t *testing.T) {
				service := svc.NewService(repository)
				service.SetScheduledWorkConsumer(&policyAdmission{receipts: map[string]bool{}})
				actor := svc.ActorContext{OwnerUserID: "pipeline-owner", SessionKey: fmt.Sprintf("pipeline-%d-%s", sample, mode), AgentID: "lead", Role: svc.ExecutionActorCoordinator, ScopeKind: protocol.ExecutionScopeDM, RootRoundID: "pipeline-root", RuntimeRoundID: "pipeline-root", AgentRoundID: "pipeline-agent"}
				draft := svc.PlanDraft{}
				for i, deliverable := range []string{"fixture:sum contains 2+2=4", "fixture:increment uses accepted sum and contains 4+1=5", "fixture:double uses accepted increment and contains 5*2=10"} {
					work := svc.PlanWorkItemDraft{LogicalKey: fmt.Sprintf("step%d", i), Kind: protocol.WorkItemKindProduce, Subject: fmt.Sprintf("Arithmetic step %d", i), Objective: deliverable, Deliverable: deliverable, AcceptanceCriteria: []string{"verified"}, Required: true, Terminal: i == 2, OutputScopes: []protocol.WorkOutputScope{{Scope: fmt.Sprintf("semantic:step%d", i), Mode: protocol.WorkOutputScopeExclusive}}}
					if i > 0 {
						work.DependsOn = []svc.PlanDependencyDraft{{LogicalKey: fmt.Sprintf("step%d", i-1), Kind: protocol.WorkDependencyHard}}
					}
					draft.Items = append(draft.Items, work)
				}
				planned, err := service.PlanExecution(t.Context(), actor, svc.PlanExecutionInput{CommandID: "pipeline-plan", Objective: "verify the three-step arithmetic result", CompletionCriteria: []string{"verified"}, Draft: draft})
				if err != nil || planned.Outcome != svc.MutationApplied {
					t.Fatalf("plan %+v %v", planned, err)
				}
				actor.ExecutionID = planned.Snapshot.Execution.ID
				input := svc.ConfigureDispatchInput{ExecutionID: actor.ExecutionID, SnapshotRevision: planned.Snapshot.Execution.Version, CommandID: "pipeline-dispatch"}
				for _, work := range planned.Snapshot.WorkItems {
					input.Policies = append(input.Policies, svc.DispatchPolicyInput{WorkItemID: work.ID, TargetAgentID: "lead", ReturnToAgentID: "lead"})
				}
				configured, err := service.ConfigureDispatch(t.Context(), actor, input)
				if err != nil || configured.Outcome != svc.MutationApplied {
					t.Fatalf("dispatch %+v %v", configured, err)
				}
				for step := range 3 {
					t.Run(fmt.Sprintf("step%d", step), func(t *testing.T) {
						if _, err = service.ReconcileDispatchPolicies(t.Context(), 32); err != nil {
							t.Fatal(err)
						}
						snapshot, _ := repository.GetSnapshot(t.Context(), actor.ExecutionID)
						var assignment protocol.WorkAssignment
						var attempt protocol.WorkAttempt
						for _, a := range snapshot.Attempts {
							if a.Status == protocol.WorkAttemptStatusPending {
								attempt = a
							}
						}
						for _, a := range snapshot.Assignments {
							if a.ID == attempt.AssignmentID {
								assignment = a
							}
						}
						if attempt.ID == "" || len(snapshot.Assignments) != step+1 {
							t.Fatal("dependency dispatch missing or premature")
						}
						worker := actor
						worker.Role = svc.ExecutionActorMember
						worker.RuntimeRoundID = fmt.Sprintf("pipeline-worker-%d", step)
						worker.AgentRoundID = fmt.Sprintf("pipeline-worker-agent-%d", step)
						worker.WorkBinding = &protocol.ExecutionWorkBinding{ExecutionID: actor.ExecutionID, PlanID: assignment.PlanID, WorkItemID: assignment.WorkItemID, SpecID: assignment.SpecID, AssignmentID: assignment.ID, AttemptID: attempt.ID}
						if _, err = service.PrepareScheduledDMRound(t.Context(), worker); err != nil {
							t.Fatal(err)
						}
						runResultProviderRound(t, service, worker, binary, model, mode, "delivery", "Deliver only the current bound step's arithmetic artifact. Use accepted upstream evidence for dependent steps. The full input is 2+2, followed by adding 1, followed by multiplying by 2; results must be 4, 5, 10 respectively. Use the exact artifact reference from this Work Item's deliverable.")
						snapshot, _ = repository.GetSnapshot(t.Context(), actor.ExecutionID)
						if len(snapshot.Submissions) != step+1 || len(snapshot.Acceptances) != step {
							t.Fatal("delivery count or premature acceptance")
						}
						var current protocol.ExecutionReviewDispatch
						for _, d := range snapshot.ReviewDispatches {
							if d.AssignmentID == assignment.ID {
								current = d
							}
						}
						reviewer := actor
						reviewer.AgentRoundID = fmt.Sprintf("pipeline-review-agent-%d", step)
						reviewer.RuntimeRoundID = fmt.Sprintf("pipeline-review-%d", step)
						reviewer.ReviewBinding = &protocol.ExecutionReviewBinding{ExecutionID: actor.ExecutionID, PlanID: assignment.PlanID, WorkItemID: assignment.WorkItemID, SpecID: assignment.SpecID, AssignmentID: assignment.ID, SubmissionID: current.SubmissionID, ReviewDispatchID: current.ID, TargetAgentID: "lead"}
						runResultProviderRound(t, service, reviewer, binary, model, mode, "review", "Review this exact arithmetic step using its submission and accepted predecessors. Expected step results are 4, 5, 10, and the immutable criterion is verified. Decide explicitly with observed evidence; do not coordinate other work.")
						snapshot, _ = repository.GetSnapshot(t.Context(), actor.ExecutionID)
						if len(snapshot.Acceptances) != step+1 {
							t.Fatal("acceptance missing")
						}
						for _, a := range snapshot.Acceptances {
							if a.Decision != protocol.WorkAcceptanceAccepted {
								t.Fatal("arithmetic rejected")
							}
						}
					})
					if t.Failed() {
						return
					}
				}
				snapshot, _ := repository.GetSnapshot(t.Context(), actor.ExecutionID)
				completed, err := service.CompleteIfReady(t.Context(), actor, svc.CompleteExecutionInput{ExecutionID: actor.ExecutionID, SnapshotRevision: snapshot.Execution.Version, CommandID: "pipeline-complete"})
				if err != nil || completed.Snapshot == nil || completed.Snapshot.Execution.Status != protocol.ExecutionStatusCompleted {
					t.Fatalf("pipeline completion %+v %v", completed, err)
				}
			})
		}
	}
}

type resultFaultRepository struct {
	*repo.Repository
	failSubmit, failSettle, failAfterSubmit bool
}

func (r *resultFaultRepository) Submit(ctx context.Context, command repo.SubmitCommand) (*protocol.ExecutionSnapshot, error) {
	if r.failSubmit {
		return nil, errors.New("injected crash before recording Submission")
	}
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

// toolAtSubmitRepository parks the explicit tool where it records its Submission.
type toolAtSubmitRepository struct {
	*repo.Repository
	atSubmit, hookDone chan struct{}
}

func (r *toolAtSubmitRepository) Submit(ctx context.Context, command repo.SubmitCommand) (*protocol.ExecutionSnapshot, error) {
	close(r.atSubmit)
	<-r.hookDone
	return r.Repository.Submit(ctx, command)
}

// hookAfterReceiptRepository runs a step once the hook's receipt is durable.
type hookAfterReceiptRepository struct {
	*repo.Repository
	afterReceipt func()
}

func (r *hookAfterReceiptRepository) PutRuntimeResultReceipt(ctx context.Context, value repo.RuntimeResultReceipt) (*repo.RuntimeResultReceipt, error) {
	stored, err := r.Repository.PutRuntimeResultReceipt(ctx, value)
	if err == nil {
		r.afterReceipt()
	}
	return stored, err
}

func TestWorkGraphRuntimeResultIntegration(t *testing.T) {
	for _, driver := range []string{"sqlite", "postgres"} {
		t.Run(driver, func(t *testing.T) {
			cfg := integrationDatabase(t, driver)
			migrateIntegrationDatabase(t, cfg, 0)
			database := openIntegrationDatabase(t, cfg)
			repository := repo.NewSQLRepository(driver, database)
			for _, mode := range []string{"normal", "before_submit", "after_submit", "before_receipt"} {
				t.Run(mode, func(t *testing.T) {
					actor, declaration := runtimeResultFixture(t, repository, mode)
					faults := &resultFaultRepository{Repository: repository, failSubmit: mode == "before_submit", failAfterSubmit: mode == "after_submit", failSettle: mode == "before_receipt"}
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
			// The hook's receipt is durable before the tool starts and is applied
			// while the tool is mid-delivery: the tool's Attempt completion must
			// never be visible without its Submission.
			t.Run("hook_applies_while_tool_delivers", func(t *testing.T) {
				actor, declaration := runtimeResultFixture(t, repository, "window")
				snapshot, _ := repository.GetSnapshot(t.Context(), actor.ExecutionID)
				tool := &toolAtSubmitRepository{Repository: repository, atSubmit: make(chan struct{}), hookDone: make(chan struct{})}
				toolResult, toolReturned := make(chan error, 1), make(chan struct{})
				hook := svc.NewService(&hookAfterReceiptRepository{Repository: repository, afterReceipt: func() {
					go func() {
						defer close(toolReturned)
						_, err := svc.NewService(tool).SubmitWork(t.Context(), actor, svc.SubmitWorkInput{ExecutionID: actor.ExecutionID, SnapshotRevision: snapshot.Execution.Version, CommandID: "explicit-window-result", ResultSummary: declaration.Delivery.ResultSummary, Evidence: declaration.Delivery.Evidence})
						toolResult <- err
					}()
					select {
					case <-tool.atSubmit:
					case <-toolReturned:
						t.Error("explicit tool returned before recording its Submission")
					}
				}})
				outcome, err := hook.AcceptRuntimeResult(t.Context(), actor, declaration)
				close(tool.hookDone)
				if toolErr := <-toolResult; toolErr != nil {
					t.Fatal(toolErr)
				}
				if err != nil || outcome.State != "applied" {
					t.Fatalf("race result=%+v %v", outcome, err)
				}
				snapshot, _ = repository.GetSnapshot(t.Context(), actor.ExecutionID)
				if len(snapshot.Submissions) != 1 || len(snapshot.ReviewDispatches) != 1 {
					t.Fatal("race duplicated delivery")
				}
			})
			t.Run("hook_confirms_tool_delivery", func(t *testing.T) {
				actor, declaration := runtimeResultFixture(t, repository, "tool-first")
				service := svc.NewService(repository)
				snapshot, _ := repository.GetSnapshot(t.Context(), actor.ExecutionID)
				delivered, err := service.SubmitWork(t.Context(), actor, svc.SubmitWorkInput{ExecutionID: actor.ExecutionID, SnapshotRevision: snapshot.Execution.Version, CommandID: "explicit-first-result", ResultSummary: declaration.Delivery.ResultSummary, Evidence: declaration.Delivery.Evidence})
				if err != nil || delivered.Outcome != svc.MutationApplied {
					t.Fatalf("tool delivery=%+v %v", delivered, err)
				}
				outcome, err := service.AcceptRuntimeResult(t.Context(), actor, declaration)
				if err != nil || outcome.State != "applied" {
					t.Fatalf("hook result=%+v %v", outcome, err)
				}
				snapshot, _ = repository.GetSnapshot(t.Context(), actor.ExecutionID)
				if len(snapshot.Submissions) != 1 || len(snapshot.ReviewDispatches) != 1 {
					t.Fatal("hook duplicated the tool's delivery")
				}
			})
			t.Run("terminal_after_intake", func(t *testing.T) {
				actor, declaration := runtimeResultFixture(t, repository, "terminal")
				service := svc.NewService(&resultFaultRepository{Repository: repository, failSubmit: true})
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
				service := svc.NewService(&resultFaultRepository{Repository: repository, failSubmit: true})
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
			t.Run("ended_attempt_is_not_resubmitted", func(t *testing.T) {
				actor, declaration := runtimeResultFixture(t, repository, "ended-attempt")
				service := svc.NewService(&resultFaultRepository{Repository: repository, failSubmit: true})
				intake, err := service.AcceptRuntimeResult(t.Context(), actor, declaration)
				if err != nil || intake.State != "pending" {
					t.Fatalf("intake=%+v %v", intake, err)
				}
				snapshot, _ := repository.GetSnapshot(t.Context(), actor.ExecutionID)
				attempt := snapshot.Attempts[0]
				attempt.Status = protocol.WorkAttemptStatusFailed
				if _, err = repository.FinishAttempt(t.Context(), repo.FinishAttemptCommand{ExpectedExecutionVersion: snapshot.Execution.Version, ExpectedAttemptVersion: attempt.Version, Attempt: attempt, Meta: integrationCommandMeta("fail-ended-result")}); err != nil {
					t.Fatal(err)
				}
				attemptsBefore := len(snapshot.Attempts)
				service = svc.NewService(repository)
				if _, err := service.ReconcileRuntimeResults(t.Context(), 64); err != nil {
					t.Fatal(err)
				}
				receipt, err := repository.GetRuntimeResultReceipt(t.Context(), intake.ReceiptID)
				if err != nil || receipt.State != "rejected" {
					t.Fatalf("ended Attempt intake=%+v %v", receipt, err)
				}
				snapshot, _ = repository.GetSnapshot(t.Context(), actor.ExecutionID)
				if len(snapshot.Submissions) != 0 || len(snapshot.Attempts) != attemptsBefore {
					t.Fatalf("ended Attempt opened a successor or delivered: attempts=%d submissions=%d", len(snapshot.Attempts), len(snapshot.Submissions))
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
	actor := svc.ActorContext{OwnerUserID: snapshot.Execution.OwnerUserID, SessionKey: snapshot.Execution.SessionKey, ExecutionID: snapshot.Execution.ID, ScopeKind: snapshot.Execution.ScopeKind, RoomID: snapshot.Execution.RoomID, ConversationID: snapshot.Execution.ConversationID, AgentID: assignment.OwnerAgentID, Role: svc.ExecutionActorMember, RuntimeRoundID: "root-" + suffix, AgentRoundID: "agent-round-" + suffix, WorkBinding: &protocol.ExecutionWorkBinding{ExecutionID: assignment.ExecutionID, PlanID: assignment.PlanID, WorkItemID: assignment.WorkItemID, SpecID: assignment.SpecID, AssignmentID: assignment.ID, AttemptID: attempt.ID, DispatchID: attempt.DispatchID}}
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
	actor := svc.ActorContext{OwnerUserID: snapshot.Execution.OwnerUserID, SessionKey: snapshot.Execution.SessionKey, ExecutionID: snapshot.Execution.ID, ScopeKind: snapshot.Execution.ScopeKind, RoomID: snapshot.Execution.RoomID, ConversationID: snapshot.Execution.ConversationID, AgentID: dispatch.TargetAgentID, Role: svc.ExecutionActorCoordinator, RuntimeRoundID: "review-root", AgentRoundID: "review-agent-round", ReviewBinding: &protocol.ExecutionReviewBinding{ExecutionID: dispatch.ExecutionID, PlanID: dispatch.PlanID, WorkItemID: dispatch.WorkItemID, SpecID: dispatch.SpecID, AssignmentID: dispatch.AssignmentID, SubmissionID: dispatch.SubmissionID, ReviewDispatchID: dispatch.ID, TargetAgentID: dispatch.TargetAgentID}}
	declaration := protocol.ExecutionResultDeclaration{Version: 1, RoundID: actor.AgentRoundID, Review: &protocol.ExecutionReviewResult{Decision: protocol.WorkAcceptanceAccepted, CriteriaResults: []protocol.WorkAcceptanceCriterionResult{{Criterion: "verified", Passed: true, Evidence: []string{"checked artifact"}}}}}
	return actor, declaration
}

func TestWorkGraphResultRuntimeAcceptance(t *testing.T) {
	binary, model := os.Getenv("NEXUS_TEST_RESULT_RUNTIME_BINARY"), os.Getenv("NEXUS_TEST_RESULT_MODEL")
	if binary == "" || model == "" {
		t.Skip("set NEXUS_TEST_RESULT_RUNTIME_BINARY and NEXUS_TEST_RESULT_MODEL with a test provider environment")
	}
	samples := 1
	if value := os.Getenv("NEXUS_TEST_RESULT_SAMPLES"); value != "" {
		var err error
		samples, err = strconv.Atoi(value)
		if err != nil || samples < 1 || samples > 5 {
			t.Fatal("NEXUS_TEST_RESULT_SAMPLES must be 1-5")
		}
	}
	cfg := integrationDatabase(t, "sqlite")
	migrateIntegrationDatabase(t, cfg, 0)
	repository := repo.NewSQLRepository("sqlite", openIntegrationDatabase(t, cfg))
	service := svc.NewService(repository)
	for sample := range samples {
		modes := []string{"tool", "hook"}
		if sample%2 == 1 {
			modes = []string{"hook", "tool"}
		}
		for _, mode := range modes {
			t.Run(fmt.Sprintf("%d_%s", sample+1, mode), func(t *testing.T) {
				actor, _ := runtimeResultFixture(t, repository, fmt.Sprintf("provider-%d-%s", sample, mode))
				runResultProviderRound(t, service, actor, binary, model, mode, "delivery")
				snapshot, err := repository.GetSnapshot(t.Context(), actor.ExecutionID)
				if err != nil || len(snapshot.Submissions) != 1 || len(snapshot.ReviewDispatches) != 1 || len(snapshot.Acceptances) != 0 {
					t.Fatalf("delivery unproven: submissions/reviews/acceptances or read error=%v", err)
				}
				reviewer, _ := runtimeReviewFixture(snapshot)
				runResultProviderRound(t, service, reviewer, binary, model, mode, "review")
				snapshot, err = repository.GetSnapshot(t.Context(), actor.ExecutionID)
				if err != nil || len(snapshot.Acceptances) != 1 || snapshot.Acceptances[0].Decision != protocol.WorkAcceptanceAccepted {
					t.Fatalf("review unproven: %v", err)
				}
			})
		}
	}
}

func runResultProviderRound(t *testing.T, service *svc.Service, actor svc.ActorContext, binary, model, mode, kind string, task ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	manager := runtime.NewManager()
	defer func() {
		cleanup, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		if err := manager.Close(cleanup); err != nil {
			t.Error("runtime cleanup failed")
		}
	}()
	key := actor.ExecutionID + ":" + kind
	scope := contract.Context{OwnerUserID: actor.OwnerUserID, AgentID: actor.AgentID, Role: actor.Role, ScopeKind: actor.ScopeKind, ScopeSessionKey: actor.SessionKey, RuntimeSessionKey: key, ExecutionID: actor.ExecutionID, WorkBinding: actor.WorkBinding, ReviewBinding: actor.ReviewBinding, RuntimeRoundID: actor.RuntimeRoundID, AgentRoundID: actor.AgentRoundID, RoomID: actor.RoomID, ConversationID: actor.ConversationID}
	operations := operation.BuildAll(service, scope)
	var commandCalls atomic.Int64
	tool := command.NewTool(func(ctx context.Context, request command.Request) (any, error) {
		commandCalls.Add(1)
		return command.HandleSemantic(ctx, command.Actor{}, command.DomainExecution, "get_execution", operations, request)
	})
	server := sdktool.NewSimpleSDKMCPServer("nexus", "1", []sdktool.Tool{tool})
	environment := map[string]string{
		"NEXUS_CONFIG_DIR": t.TempDir(), "NEXUS_DISABLE_AUTO_MEMORY": "1", "NEXUS_API_PROVIDER": "anthropic-compatible",
		"ANTHROPIC_BASE_URL": os.Getenv("ANTHROPIC_BASE_URL"), "ANTHROPIC_AUTH_TOKEN": os.Getenv("ANTHROPIC_AUTH_TOKEN"), "ANTHROPIC_API_KEY": os.Getenv("ANTHROPIC_API_KEY"),
	}
	options := manager.WithWorkResultHook(bridge.Options{
		CLIPath: binary, CWD: t.TempDir(), Env: environment, Model: model,
		System:  bridge.SystemOptions{Text: "You are validating a synthetic WorkGraph responsibility. Use only supplied evidence and the bound WorkGraph. Be concise. No other work or external actions are needed."},
		Runtime: bridge.RuntimeOptions{Kind: bridge.RuntimeNXS, MaxTurns: 8, Thinking: &bridge.ThinkingConfig{Type: bridge.ThinkingModeDisabled}},
		Tools:   bridge.ToolOptions{Available: []string{}, Allow: []string{"mcp__nexus__command"}},
		Skills:  bridge.SkillOptions{Mode: bridge.SkillModeNone}, SettingSources: []string{},
		MCP: bridge.MCPOptions{SDKServers: map[string]sdkmcp.SDKMCPServer{"nexus": server}, StrictConfig: true},
	}, key)
	client, err := manager.GetOrCreate(ctx, key, options)
	if err != nil {
		t.Fatal("runtime initialization failed: " + err.Error())
	}
	if err := client.Connect(ctx); err != nil {
		t.Fatal("runtime connection failed: " + err.Error())
	}
	if !runtime.SupportsWorkResultHooks(client) {
		t.Fatal("runtime lacks negotiated Stop/ACK prerequisites")
	}
	contextText, err := service.RuntimeContext(ctx, actor)
	if err != nil {
		t.Fatal(err)
	}
	prompt := contextText + "\nThe exact acceptance criterion is verified. The supplied artifact is fixture:arithmetic, containing 2 + 2 = 4. Check it directly from this content; no file or network access is needed."
	operationName := "submit_work"
	if kind == "delivery" {
		prompt += "\nDeliver this bound responsibility with a concise result_summary, result_refs containing fixture:arithmetic, and evidence that 2+2 equals 4. Do not review it yourself."
	} else {
		operationName = "review_work"
		prompt += "\nReview the bound submission against the supplied artifact and the exact criterion verified. Make your explicit decision with evidence. Do not continue coordinating other work."
	}
	if len(task) > 0 {
		prompt = contextText + "\n" + task[0]
	}
	if mode == "hook" {
		manager.SetWorkResultHook(key, actor.AgentRoundID, runtimehook.ResultCallback(service, actor, nil))
		defer manager.ClearWorkResultHook(key, actor.AgentRoundID)
		prompt += "\n" + runtimehook.ResultContract(actor)
	} else {
		op, ok := command.FindOperation(operations, operationName)
		if !ok {
			t.Fatal("missing canonical operation")
		}
		schema, _ := json.Marshal(op.InputSchema)
		prompt += "\nReport the result using nexus.command with domain=execution, action=invoke, operation=" + operationName + ", a new valid request_id and the following exact input schema (bound locators may be omitted): " + string(schema) + ". Finish with a short reply after the receipt."
	}
	messages := client.ReceiveMessages(ctx)
	start := time.Now()
	if err := client.Query(ctx, prompt); err != nil {
		t.Fatal("runtime query failed")
	}
	for {
		select {
		case <-ctx.Done():
			evidence, _ := json.Marshal(map[string]any{"mode": mode, "kind": kind, "model": model, "elapsed_ms": time.Since(start).Milliseconds(), "turn_budget": 8, "command_calls": commandCalls.Load(), "has_usage": false, "outcome": "timeout"})
			t.Log(string(evidence))
			t.Fatal("provider round timed out")
		case message, ok := <-messages:
			if !ok {
				t.Fatal("runtime ended without terminal receipt")
			}
			if message.Result == nil {
				continue
			}
			result := message.Result
			usage, hasUsage := sdkprotocol.TokenUsageFromResult(*result)
			usage.Raw = nil
			evidence, _ := json.Marshal(map[string]any{"mode": mode, "kind": kind, "model": model, "elapsed_ms": time.Since(start).Milliseconds(), "turn_budget": 8, "turns": result.NumTurns, "command_calls": commandCalls.Load(), "usage": usage, "has_usage": hasUsage, "runtime_error": result.IsError})
			t.Log(string(evidence))
			if result.IsError {
				detail := strings.Join(result.Errors, "; ")
				for _, credential := range []string{environment["ANTHROPIC_AUTH_TOKEN"], environment["ANTHROPIC_API_KEY"]} {
					if credential != "" {
						detail = strings.ReplaceAll(detail, credential, "[redacted]")
					}
				}
				t.Fatalf("provider/runtime rejected round: %s %s", result.Subtype, detail)
			}
			if mode == "hook" && commandCalls.Load() != 0 {
				t.Log("hook-enabled round fell back to explicit commands; retain its full usage and timing in the comparison")
			}
			return
		}
	}
}

func TestWorkGraphRetryIntegration(t *testing.T) {
	for _, driver := range []string{"sqlite", "postgres"} {
		t.Run(driver, func(t *testing.T) {
			cfg := integrationDatabase(t, driver)
			migrateIntegrationDatabase(t, cfg, 0)
			database := openIntegrationDatabase(t, cfg)
			repository := repo.NewSQLRepository(driver, database)
			for _, scenario := range []string{"submitted", "accepted_running", "completed", "parallel", "paused_failed"} {
				t.Run(scenario, func(t *testing.T) {
					suffix := "retry-" + scenario
					batch := prepareIntegrationBatchScope(t, repository, suffix, protocol.ExecutionScopeRoom, scenario != "parallel")
					root, child := batch.Assignments[0], batch.Assignments[1]
					snapshot, err := repository.Assign(t.Context(), root)
					if err != nil {
						t.Fatal(err)
					}
					snapshot = startRetryFixture(t, repository, snapshot, root.RootAttempt.ID)
					if scenario == "paused_failed" {
						attempt := retryAttempt(t, snapshot, root.RootAttempt.ID)
						attempt.Status = protocol.WorkAttemptStatusFailed
						snapshot, err = repository.FinishAttempt(t.Context(), repo.FinishAttemptCommand{ExpectedExecutionVersion: snapshot.Execution.Version, ExpectedAttemptVersion: attempt.Version, Attempt: attempt, Meta: integrationCommandMeta("fail-" + suffix)})
						if err != nil {
							t.Fatal(err)
						}
						// Simulate the persisted global pause left by the host, independent of model work.
						_, err = database.Exec(`UPDATE executions SET status='paused' WHERE execution_id='` + snapshot.Execution.ID + `'`)
						if err != nil {
							t.Fatal(err)
						}
						snapshot = retrySnapshot(t, repository, snapshot.Execution.ID)
					} else {
						snapshot = submitRetryFixture(t, repository, snapshot, root.RootAttempt.ID, scenario != "submitted")
					}
					if scenario == "accepted_running" || scenario == "completed" || scenario == "parallel" {
						child.ExpectedExecutionVersion = snapshot.Execution.Version
						snapshot, err = repository.Assign(t.Context(), child)
						if err != nil {
							t.Fatal(err)
						}
						snapshot = startRetryFixture(t, repository, snapshot, child.RootAttempt.ID)
						if scenario != "accepted_running" {
							snapshot = submitRetryFixture(t, repository, snapshot, child.RootAttempt.ID, true)
						}
					}
					if scenario == "completed" {
						// Fixture dispatches have been consumed by the controlled runtime above.
						_, err = database.Exec(`UPDATE execution_dispatches SET status='delivered' WHERE execution_id='` + snapshot.Execution.ID + `'`)
						if err != nil {
							t.Fatal(err)
						}
						snapshot, err = repository.Complete(t.Context(), repo.CompleteCommand{ExecutionID: snapshot.Execution.ID, ExpectedExecutionVersion: snapshot.Execution.Version, Meta: integrationCommandMeta("complete-" + suffix)})
						if err != nil {
							t.Fatal(err)
						}
					}
					service := svc.NewService(repository)
					delivery := &continuationDelivery{policyAdmission: policyAdmission{receipts: map[string]bool{}}}
					service.SetScheduledWorkConsumer(delivery)
					service.SetAssignmentTargetAuthorizer(policyTargetAuthorizer{})
					view, viewErr := service.GetLatestView(t.Context(), snapshot.Execution.OwnerUserID, snapshot.Execution.SessionKey)
					if viewErr != nil {
						t.Fatal(viewErr)
					}
					retryVisible := false
					for _, node := range view.Graph.Nodes {
						if node.AttemptID == root.RootAttempt.ID && node.CanRetry {
							retryVisible = true
						}
					}
					if !retryVisible {
						t.Fatal("normal graph read omitted task retry action")
					}
					input := protocol.RequestExecutionRecovery{SessionKey: snapshot.Execution.SessionKey, WorkItemID: root.Assignment.WorkItemID, AttemptID: root.RootAttempt.ID, RequestID: "retry-request"}
					preview, err := service.PreviewWorkRetry(t.Context(), snapshot.Execution.OwnerUserID, snapshot.Execution.ID, input)
					if err != nil {
						t.Fatal(err)
					}
					wantAffected := 2
					if scenario == "parallel" || scenario == "paused_failed" {
						wantAffected = 1
					}
					if len(preview.Affected) != wantAffected {
						t.Fatalf("wrong closure: %+v", preview)
					}
					request := protocol.RequestWorkRetry{RequestExecutionRecovery: input, SnapshotRevision: preview.SnapshotRevision}
					stale := request
					stale.SnapshotRevision--
					if _, err = service.RequestWorkRetry(t.Context(), snapshot.Execution.OwnerUserID, snapshot.Execution.ID, stale); err == nil {
						t.Fatal("stale preview accepted")
					}
					for range 2 {
						receipt, requestErr := service.RequestWorkRetry(t.Context(), snapshot.Execution.OwnerUserID, snapshot.Execution.ID, request)
						if requestErr != nil || !receipt.Queued {
							t.Fatalf("retry %+v: %v", receipt, requestErr)
						}
					}
					after := retrySnapshot(t, repository, snapshot.Execution.ID)
					if after.Execution.Status != protocol.ExecutionStatusActive || after.Execution.Version != snapshot.Execution.Version+1 {
						t.Fatalf("non-atomic retry: %+v", after.Execution)
					}
					if !slices.Contains(after.ReadyWorkItemIDs, root.Assignment.WorkItemID) {
						t.Fatalf("root not ready: %+v", after.CompletionBlockers)
					}
					if scenario != "parallel" && slices.Contains(after.ReadyWorkItemIDs, child.Assignment.WorkItemID) {
						t.Fatal("downstream ready before new acceptance")
					}
					if scenario == "parallel" {
						if len(after.Acceptances) != 1 || after.Acceptances[0].WorkItemID != child.Assignment.WorkItemID {
							t.Fatal("parallel branch invalidated")
						}
					} else if len(after.Acceptances) != 0 || len(after.Submissions) != 0 {
						t.Fatal("old results still current")
					}
					history, err := repository.ListWorkGraphHistory(t.Context(), after.Plan.ID)
					if err != nil {
						t.Fatal(err)
					}
					for _, submission := range history.Submissions {
						if submission.WorkItemID == root.Assignment.WorkItemID && !submission.Superseded {
							t.Fatal("retry lost history supersession")
						}
					}
					if len(snapshot.Submissions) > 0 {
						old := snapshot.Submissions[0]
						if old.WorkItemID != root.Assignment.WorkItemID {
							old = snapshot.Submissions[1]
						}
						oldAssignment := root.Assignment
						for _, a := range snapshot.Assignments {
							if a.ID == old.AssignmentID {
								oldAssignment = a
							}
						}
						_, err = repository.Review(t.Context(), repo.ReviewCommand{ExpectedExecutionVersion: after.Execution.Version, ExpectedAssignmentVersion: oldAssignment.Version, Acceptance: protocol.WorkAcceptance{ID: "late-" + suffix, ExecutionID: old.ExecutionID, PlanID: old.PlanID, WorkItemID: old.WorkItemID, SpecID: old.SpecID, AssignmentID: old.AssignmentID, SubmissionID: old.ID, Decision: protocol.WorkAcceptanceAccepted, ReviewerKind: protocol.WorkReviewerAgent, ReviewerID: oldAssignment.ReturnToAgentID, CriteriaResults: []protocol.WorkAcceptanceCriterionResult{{Criterion: "verified", Passed: true}}}, Meta: integrationCommandMeta("late-" + suffix)})
						if err == nil {
							t.Fatal("late old review accepted")
						}
					}
					// A restarted worker observes only durable policies. Physical cancellation gates replacement work.
					if _, err = service.ReconcileDispatchPolicies(t.Context(), 32); err != nil {
						t.Fatal(err)
					}
					after = retrySnapshot(t, repository, snapshot.Execution.ID)
					if scenario == "accepted_running" {
						if len(after.CancellationDispatches) == 0 {
							t.Fatal("running dependent was not cancelled")
						}
						if len(after.Attempts) != len(snapshot.Attempts) {
							t.Fatal("replacement raced physical cancellation")
						}
						for _, cancel := range after.CancellationDispatches {
							claimed, claimErr := repository.ClaimCancellationDispatch(t.Context(), cancel.ID, cancel.Version, "fixture", time.Minute)
							if claimErr != nil {
								t.Fatal(claimErr)
							}
							if _, err = repository.ResolveCancellationDispatch(t.Context(), claimed.ID, claimed.Version, "fixture", protocol.ExecutionCancellationDispatchDelivered, protocol.ExecutionCancellationOutcomeProviderInterrupted, "", "fixture physical stop"); err != nil {
								t.Fatal(err)
							}
						}
						if _, err = repository.WakeDispatchPolicies(t.Context(), snapshot.Execution.ID, time.Now()); err != nil {
							t.Fatal(err)
						}
						if _, err = service.ReconcileDispatchPolicies(t.Context(), 32); err != nil {
							t.Fatal(err)
						}
						after = retrySnapshot(t, repository, snapshot.Execution.ID)
					}
					var replacement protocol.WorkAttempt
					for _, attempt := range after.Attempts {
						if attempt.WorkItemID == root.Assignment.WorkItemID && attempt.ID != root.RootAttempt.ID {
							replacement = attempt
						}
					}
					if replacement.ID == "" || replacement.ExecutorAgentID != root.Assignment.OwnerAgentID {
						t.Fatalf("original owner not restarted: %+v", after.DispatchPolicies)
					}
					after = startRetryFixture(t, repository, after, replacement.ID)
					after = submitRetryFixture(t, repository, after, replacement.ID, true)
					if scenario != "parallel" && !slices.Contains(after.ReadyWorkItemIDs, child.Assignment.WorkItemID) {
						t.Fatal("new acceptance did not unlock downstream")
					}
					if scenario == "completed" || scenario == "accepted_running" {
						if _, err = repository.WakeDispatchPolicies(t.Context(), after.Execution.ID, time.Now()); err != nil {
							t.Fatal(err)
						}
						if _, err = service.ReconcileDispatchPolicies(t.Context(), 32); err != nil {
							t.Fatal(err)
						}
						after = retrySnapshot(t, repository, after.Execution.ID)
						found := false
						var childAttemptID string
						for _, attempt := range after.Attempts {
							if attempt.WorkItemID == child.Assignment.WorkItemID && attempt.ID != child.RootAttempt.ID {
								found = true
								childAttemptID = attempt.ID
							}
						}
						if !found {
							t.Fatal("dependent not scheduled after acceptance")
						}
						after = startRetryFixture(t, repository, after, childAttemptID)
						after = submitRetryFixture(t, repository, after, childAttemptID, true)
						_, err = database.Exec(`UPDATE execution_dispatches SET status='delivered' WHERE execution_id='` + after.Execution.ID + `' AND status IN ('pending','claimed')`)
						if err != nil {
							t.Fatal(err)
						}
						after, err = repository.Complete(t.Context(), repo.CompleteCommand{ExecutionID: after.Execution.ID, ExpectedExecutionVersion: after.Execution.Version, Meta: integrationCommandMeta("recomplete-" + suffix)})
						if err != nil || len(after.Acceptances) != 2 || after.Execution.Status != protocol.ExecutionStatusCompleted {
							t.Fatalf("retry pipeline did not complete: %v", err)
						}

					}
				})
			}
		})
	}
}
func retrySnapshot(t *testing.T, r *repo.Repository, id string) *protocol.ExecutionSnapshot {
	t.Helper()
	s, err := r.GetSnapshot(t.Context(), id)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func retryAttempt(t *testing.T, s *protocol.ExecutionSnapshot, id string) protocol.WorkAttempt {
	t.Helper()
	for _, a := range s.Attempts {
		if a.ID == id {
			return a
		}
	}
	t.Fatalf("missing attempt %s", id)
	return protocol.WorkAttempt{}
}
func startRetryFixture(t *testing.T, r *repo.Repository, s *protocol.ExecutionSnapshot, id string) *protocol.ExecutionSnapshot {
	t.Helper()
	a := retryAttempt(t, s, id)
	var assignment protocol.WorkAssignment
	for _, candidate := range s.Assignments {
		if candidate.ID == a.AssignmentID {
			assignment = candidate
		}
	}
	a.AgentRoundID = "round-" + id
	a.RuntimeRoundID = a.AgentRoundID
	a.RuntimeSessionKey = s.Execution.SessionKey
	updated, err := r.StartAttempt(t.Context(), repo.StartAttemptCommand{ExpectedExecutionVersion: s.Execution.Version, ExpectedAssignmentVersion: assignment.Version, ExpectedAttemptVersion: a.Version, Attempt: a, Meta: integrationCommandMeta("start-" + id)})
	if err != nil {
		t.Fatal(err)
	}
	return updated
}
func submitRetryFixture(t *testing.T, r *repo.Repository, s *protocol.ExecutionSnapshot, id string, accept bool) *protocol.ExecutionSnapshot {
	t.Helper()
	a := retryAttempt(t, s, id)
	a.Status = protocol.WorkAttemptStatusSucceeded
	s, err := r.FinishAttempt(t.Context(), repo.FinishAttemptCommand{ExpectedExecutionVersion: s.Execution.Version, ExpectedAttemptVersion: a.Version, Attempt: a, Meta: integrationCommandMeta("finish-" + id)})
	if err != nil {
		t.Fatal(err)
	}
	var assignment protocol.WorkAssignment
	for _, candidate := range s.Assignments {
		if candidate.ID == a.AssignmentID {
			assignment = candidate
		}
	}
	submission := protocol.WorkSubmission{ID: "submission-" + id, ExecutionID: a.ExecutionID, PlanID: a.PlanID, WorkItemID: a.WorkItemID, SpecID: a.SpecID, AssignmentID: a.AssignmentID, AttemptID: id, SubmitterAgentID: a.ExecutorAgentID, ResultSummary: fmt.Sprintf("result %s", id), Evidence: []string{"verified"}}
	s, err = r.Submit(t.Context(), repo.SubmitCommand{ExpectedExecutionVersion: s.Execution.Version, ExpectedAssignmentVersion: assignment.Version, Submission: submission, ReviewDispatch: &protocol.ExecutionReviewDispatch{ID: "review-" + id, TargetAgentID: assignment.ReturnToAgentID, DedupeKey: "review-" + id, Instruction: "review evidence"}, Meta: integrationCommandMeta("submit-" + id)})
	if err != nil {
		t.Fatal(err)
	}
	if !accept {
		return s
	}
	for _, candidate := range s.Assignments {
		if candidate.ID == a.AssignmentID {
			assignment = candidate
		}
	}
	s, err = r.Review(t.Context(), repo.ReviewCommand{ExpectedExecutionVersion: s.Execution.Version, ExpectedAssignmentVersion: assignment.Version, Acceptance: protocol.WorkAcceptance{ID: "accept-" + id, ExecutionID: a.ExecutionID, PlanID: a.PlanID, WorkItemID: a.WorkItemID, SpecID: a.SpecID, AssignmentID: a.AssignmentID, SubmissionID: submission.ID, Decision: protocol.WorkAcceptanceAccepted, ReviewerKind: protocol.WorkReviewerAgent, ReviewerID: assignment.ReturnToAgentID, CriteriaResults: []protocol.WorkAcceptanceCriterionResult{{Criterion: "verified", Passed: true, Evidence: []string{"checked"}}}}, Meta: integrationCommandMeta("accept-" + id)})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestWorkGraphQueuedRetryConcurrentReceipt(t *testing.T) {
	cfg := integrationDatabase(t, "sqlite")
	migrateIntegrationDatabase(t, cfg, 0)
	database := openIntegrationDatabase(t, cfg)
	repository := repo.NewSQLRepository("sqlite", database)
	batch := prepareIntegrationBatchScope(t, repository, "retry-queued", protocol.ExecutionScopeRoom, true)
	root := batch.Assignments[0].Assignment
	child := batch.Assignments[1].Assignment
	_, err := repository.ConfigureDispatch(t.Context(), repo.ConfigureDispatchCommand{ExecutionID: root.ExecutionID, PlanID: root.PlanID, ExpectedExecutionVersion: 2, RequestDigest: "queued", Meta: integrationCommandMeta("configure-queued"), Policies: []protocol.ExecutionDispatchPolicy{{ID: "queued-policy", ExecutionID: root.ExecutionID, PlanID: root.PlanID, WorkItemID: root.WorkItemID, SpecID: root.SpecID, TargetAgentID: root.OwnerAgentID, ReturnToAgentID: root.ReturnToAgentID}, {ID: "paused-child-policy", ExecutionID: child.ExecutionID, PlanID: child.PlanID, WorkItemID: child.WorkItemID, SpecID: child.SpecID, TargetAgentID: child.OwnerAgentID, ReturnToAgentID: child.ReturnToAgentID}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`UPDATE executions SET status='paused' WHERE execution_id='` + root.ExecutionID + `'`); err != nil {
		t.Fatal(err)
	}
	snapshot := retrySnapshot(t, repository, root.ExecutionID)
	service := svc.NewService(repository)
	service.SetScheduledWorkConsumer(&continuationDelivery{policyAdmission: policyAdmission{receipts: map[string]bool{}}})
	service.SetAssignmentTargetAuthorizer(policyTargetAuthorizer{})
	input := protocol.RequestExecutionRecovery{SessionKey: snapshot.Execution.SessionKey, WorkItemID: root.WorkItemID, RequestID: "retry-never-started"}
	preview, err := service.PreviewWorkRetry(t.Context(), snapshot.Execution.OwnerUserID, root.ExecutionID, input)
	if err != nil || len(preview.Affected) != 1 {
		t.Fatalf("queued preview %+v %v", preview, err)
	}
	var group sync.WaitGroup
	for range 4 {
		group.Go(func() {
			_, _ = service.RequestWorkRetry(t.Context(), snapshot.Execution.OwnerUserID, root.ExecutionID, protocol.RequestWorkRetry{RequestExecutionRecovery: input, SnapshotRevision: preview.SnapshotRevision})
		})
	}
	group.Wait()
	receipt, err := service.GetWorkRecoveryReceipt(t.Context(), snapshot.Execution.OwnerUserID, root.ExecutionID, input.SessionKey, input.RequestID)
	if err != nil || !receipt.Queued || receipt.AttemptID != "" {
		t.Fatalf("queued receipt %+v %v", receipt, err)
	}
	after := retrySnapshot(t, repository, root.ExecutionID)
	if after.Execution.Version != snapshot.Execution.Version+1 || len(after.DispatchPolicies) != 2 {
		t.Fatalf("duplicate retry policies %+v", after.DispatchPolicies)
	}
	if _, err = service.ReconcileDispatchPolicies(t.Context(), 32); err != nil {
		t.Fatal(err)
	}
	after = retrySnapshot(t, repository, root.ExecutionID)
	if len(after.Attempts) != 1 || after.Assignments[0].OwnerAgentID != root.OwnerAgentID {
		t.Fatalf("queued task did not start once: %+v", after.Attempts)
	}
	for range 2 {
		if _, err = repository.WakeDispatchPolicies(t.Context(), root.ExecutionID, time.Now()); err != nil {
			t.Fatal(err)
		}
		if _, err = service.ReconcileDispatchPolicies(t.Context(), 32); err != nil {
			t.Fatal(err)
		}
	}
	after = retrySnapshot(t, repository, root.ExecutionID)
	for _, policy := range after.DispatchPolicies {
		if policy.WorkItemID == child.WorkItemID && (policy.State != "pending" || policy.Reason != "user_paused") {
			t.Fatalf("unrelated pause was lost: %+v", policy)
		}
	}

}
