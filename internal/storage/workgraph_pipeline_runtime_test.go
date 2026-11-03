// INPUT: Explicit real-provider opt-in and three dependency-ordered synthetic work contracts.
// OUTPUT: Paired tool/hook multistep usage logs and unique accepted SQL outcomes.
// POS: Bounded model benchmark; runtime startup is excluded from per-round timing.
package storage_test

import (
	"fmt"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	svc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	repo "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
	"os"
	"testing"
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
