// INPUT: Migrated database, accepted/submitted/paused tasks and explicit retry requests.
// OUTPUT: Atomic cascade, immutable history, stale reviewer fences and dependency-gated redelivery.
// POS: WorkGraph retry service/storage integration through the user entry points.
package storage_test

import (
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	svc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
	repo "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

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
	snapshot, err := repository.ConfigureDispatch(t.Context(), repo.ConfigureDispatchCommand{ExecutionID: root.ExecutionID, PlanID: root.PlanID, ExpectedExecutionVersion: 2, RequestDigest: "queued", Meta: integrationCommandMeta("configure-queued"), Policies: []protocol.ExecutionDispatchPolicy{{ID: "queued-policy", ExecutionID: root.ExecutionID, PlanID: root.PlanID, WorkItemID: root.WorkItemID, SpecID: root.SpecID, TargetAgentID: root.OwnerAgentID, ReturnToAgentID: root.ReturnToAgentID}, {ID: "paused-child-policy", ExecutionID: child.ExecutionID, PlanID: child.PlanID, WorkItemID: child.WorkItemID, SpecID: child.SpecID, TargetAgentID: child.OwnerAgentID, ReturnToAgentID: child.ReturnToAgentID}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = database.Exec(`UPDATE executions SET status='paused' WHERE execution_id='` + root.ExecutionID + `'`); err != nil {
		t.Fatal(err)
	}
	snapshot = retrySnapshot(t, repository, root.ExecutionID)
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
