package server

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	handlershared "github.com/nexus-research-lab/nexus/internal/handler/shared"
	"github.com/nexus-research-lab/nexus/internal/infra/logx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

func TestCompletionAuditLifecycleRecoversAcceptedReviewAfterDatabaseRestart(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)
	db, err := OpenDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	store := orchestrationstore.NewRepository(cfg, db)
	snapshot := prepareCompletionAuditLifecycleReview(t, ctx, store)
	if snapshot.Execution.Status != protocol.ExecutionStatusActive ||
		len(snapshot.CompletionBlockers) != 0 {
		t.Fatalf("accepted review did not expose completion boundary: %#v", snapshot)
	}
	receipt, err := store.GetCompletionAuditReceipt(ctx, snapshot.Execution.ID)
	if err != nil {
		t.Fatal(err)
	}
	if receipt == nil || receipt.State != orchestrationstore.CompletionAuditPending {
		t.Fatalf("pre-crash completion receipt = %#v", receipt)
	}

	// Simulate the process disappearing after Review committed but before the
	// foreground ReviewWork path could call Complete.
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = OpenDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	services := NewAppServicesWithDB(cfg, db, logx.NewDiscardLogger())
	server := &Server{
		api:      handlershared.NewAPI(logx.NewDiscardLogger()),
		services: services,
	}
	stop, err := server.startCompletionAuditRecovery(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stop != nil {
		stop()
	}

	restartedStore := orchestrationstore.NewRepository(cfg, db)
	snapshot, err = restartedStore.GetSnapshot(ctx, snapshot.Execution.ID)
	if err != nil {
		t.Fatal(err)
	}
	receipt, err = restartedStore.GetCompletionAuditReceipt(ctx, snapshot.Execution.ID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Execution.Status != protocol.ExecutionStatusCompleted ||
		receipt == nil || receipt.State != orchestrationstore.CompletionAuditCompleted ||
		receipt.SettledAt == nil {
		t.Fatalf("restart recovery snapshot=%#v receipt=%#v", snapshot.Execution, receipt)
	}
	var completedEvents int
	if err = db.QueryRow(`
SELECT COUNT(1)
FROM execution_events
WHERE execution_id = ? AND event_type = 'execution_completed'`,
		snapshot.Execution.ID,
	).Scan(&completedEvents); err != nil {
		t.Fatal(err)
	}
	if completedEvents != 1 {
		t.Fatalf("completed event count = %d, want 1", completedEvents)
	}

	// A second startup pass must observe the settled receipt and remain a no-op.
	stop, err = server.startCompletionAuditRecovery(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stop != nil {
		stop()
	}
	if err = db.QueryRow(`
SELECT COUNT(1)
FROM execution_events
WHERE execution_id = ? AND event_type = 'execution_completed'`,
		snapshot.Execution.ID,
	).Scan(&completedEvents); err != nil {
		t.Fatal(err)
	}
	if completedEvents != 1 {
		t.Fatalf("idempotent startup completed event count = %d, want 1", completedEvents)
	}
}

func TestCompletionAuditLifecycleDefersBlockedGraphWithoutStartupFailure(t *testing.T) {
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)
	db, err := OpenDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	ctx := context.Background()
	store := orchestrationstore.NewRepository(cfg, db)
	snapshot := prepareCompletionAuditLifecycleReview(t, ctx, store)

	// Reopen the just-completed Assignment chain under a second required item by
	// adding a fresh Plan revision would be needlessly broad for this lifecycle
	// check. A paused Execution is itself a fail-closed completion blocker and
	// exercises the same durable defer path without terminalizing the graph.
	if _, err = db.Exec(`
UPDATE executions SET status = 'paused' WHERE execution_id = ?`,
		snapshot.Execution.ID,
	); err != nil {
		t.Fatal(err)
	}
	services := NewAppServicesWithDB(cfg, db, logx.NewDiscardLogger())
	server := &Server{
		api:      handlershared.NewAPI(logx.NewDiscardLogger()),
		services: services,
	}
	stop, err := server.startCompletionAuditRecovery(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if stop != nil {
		stop()
	}
	receipt, err := store.GetCompletionAuditReceipt(ctx, snapshot.Execution.ID)
	if err != nil {
		t.Fatal(err)
	}
	current, err := store.GetSnapshot(ctx, snapshot.Execution.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.Execution.Status != protocol.ExecutionStatusPaused ||
		receipt == nil || receipt.State != orchestrationstore.CompletionAuditPending ||
		receipt.AttemptCount != 1 || receipt.NextAttemptAt == nil ||
		receipt.LastError != "Execution is paused" {
		t.Fatalf("paused recovery snapshot=%#v receipt=%#v", current.Execution, receipt)
	}
}

func prepareCompletionAuditLifecycleReview(
	t *testing.T,
	ctx context.Context,
	store *orchestrationstore.Repository,
) *protocol.ExecutionSnapshot {
	t.Helper()
	const (
		executionID  = "execution-completion-lifecycle"
		planID       = "plan-completion-lifecycle"
		workID       = "work-completion-lifecycle"
		specID       = "spec-completion-lifecycle"
		assignmentID = "assignment-completion-lifecycle"
		attemptID    = "attempt-completion-lifecycle"
		submissionID = "submission-completion-lifecycle"
	)
	meta := func(id string) orchestrationstore.CommandMeta {
		return orchestrationstore.CommandMeta{
			CommandID: "command-" + id,
			EventID:   "event-" + id,
			ActorKind: protocol.ExecutionActorSystem,
			ActorID:   "test-server",
		}
	}
	snapshot, err := store.Create(ctx, orchestrationstore.CreateCommand{
		Execution: protocol.Execution{
			ID:                 executionID,
			OwnerUserID:        "owner-1",
			SessionKey:         "agent:nexus:workspace:dm:completion-lifecycle",
			ScopeKind:          protocol.ExecutionScopeDM,
			CoordinatorAgentID: "agent-lead",
			Origin:             protocol.ExecutionOriginUserRequest,
			Objective:          "recover completion after accepted review",
			CompletionCriteria: []string{"verified"},
			Status:             protocol.ExecutionStatusActive,
		},
		Meta: meta("create-completion-lifecycle"),
	})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err = store.WritePlan(ctx, orchestrationstore.WritePlanCommand{
		ExecutionID:              executionID,
		ExpectedExecutionVersion: snapshot.Execution.Version,
		Plan: protocol.ExecutionPlanRevision{
			ID:               planID,
			ExecutionID:      executionID,
			Revision:         1,
			Status:           protocol.PlanRevisionStatusActive,
			CreatedByAgentID: "agent-lead",
			RevisionReason:   "test",
		},
		WorkItems: []orchestrationstore.PlanWorkItem{{
			WorkItem: protocol.WorkItem{
				ID:          workID,
				ExecutionID: executionID,
				LogicalKey:  "completion",
				Kind:        protocol.WorkItemKindProduce,
			},
			Spec: protocol.WorkItemSpec{
				ID:                 specID,
				WorkItemID:         workID,
				ExecutionID:        executionID,
				Version:            1,
				Subject:            "Completion",
				Objective:          "Finish work",
				Deliverable:        "Verified output",
				AcceptanceCriteria: []string{"verified"},
				SpecHash:           "hash-completion-lifecycle",
				CreatedByAgentID:   "agent-lead",
			},
			State: protocol.WorkItemState{
				WorkItemID:    workID,
				ExecutionID:   executionID,
				CurrentSpecID: specID,
				Status:        protocol.WorkItemStatusOpen,
				Version:       1,
			},
			Item: protocol.ExecutionPlanItem{
				PlanID:      planID,
				ExecutionID: executionID,
				WorkItemID:  workID,
				SpecID:      specID,
				Required:    true,
				Terminal:    true,
			},
		}},
		Meta: meta("plan-completion-lifecycle"),
	})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err = store.Assign(ctx, orchestrationstore.AssignCommand{
		ExpectedExecutionVersion: snapshot.Execution.Version,
		Assignment: protocol.WorkAssignment{
			ID:                assignmentID,
			ExecutionID:       executionID,
			PlanID:            planID,
			WorkItemID:        workID,
			SpecID:            specID,
			OwnerAgentID:      "agent-worker",
			AssignedByAgentID: "agent-lead",
			ReturnToAgentID:   "agent-lead",
			Strategy:          protocol.AssignmentStrategySelf,
			Status:            protocol.WorkAssignmentStatusAssigned,
		},
		RootAttempt: &protocol.WorkAttempt{
			ID:              attemptID,
			ExecutionID:     executionID,
			PlanID:          planID,
			WorkItemID:      workID,
			SpecID:          specID,
			AssignmentID:    assignmentID,
			ExecutorKind:    protocol.AttemptExecutorAgent,
			ExecutorAgentID: "agent-worker",
			Status:          protocol.WorkAttemptStatusPending,
		},
		Meta: meta("assign-completion-lifecycle"),
	})
	if err != nil {
		t.Fatal(err)
	}
	assignment := snapshot.Assignments[0]
	attempt := snapshot.Attempts[0]
	snapshot, err = store.StartAttempt(ctx, orchestrationstore.StartAttemptCommand{
		ExpectedExecutionVersion:  snapshot.Execution.Version,
		ExpectedAssignmentVersion: assignment.Version,
		ExpectedAttemptVersion:    attempt.Version,
		Attempt:                   attempt,
		Meta:                      meta("start-completion-lifecycle"),
	})
	if err != nil {
		t.Fatal(err)
	}
	attempt = snapshot.Attempts[0]
	attempt.Status = protocol.WorkAttemptStatusSucceeded
	snapshot, err = store.FinishAttempt(ctx, orchestrationstore.FinishAttemptCommand{
		ExpectedExecutionVersion: snapshot.Execution.Version,
		ExpectedAttemptVersion:   attempt.Version,
		Attempt:                  attempt,
		Meta:                     meta("finish-completion-lifecycle"),
	})
	if err != nil {
		t.Fatal(err)
	}
	assignment = snapshot.Assignments[0]
	snapshot, err = store.Submit(ctx, orchestrationstore.SubmitCommand{
		ExpectedExecutionVersion:  snapshot.Execution.Version,
		ExpectedAssignmentVersion: assignment.Version,
		Submission: protocol.WorkSubmission{
			ID:               submissionID,
			ExecutionID:      executionID,
			PlanID:           planID,
			WorkItemID:       workID,
			SpecID:           specID,
			AssignmentID:     assignmentID,
			AttemptID:        attemptID,
			SubmitterAgentID: "agent-worker",
			ResultSummary:    "done",
			Evidence:         []string{"verified"},
		},
		Meta: meta("submit-completion-lifecycle"),
	})
	if err != nil {
		t.Fatal(err)
	}
	assignment = snapshot.Assignments[0]
	snapshot, err = store.Review(ctx, orchestrationstore.ReviewCommand{
		ExpectedExecutionVersion:  snapshot.Execution.Version,
		ExpectedAssignmentVersion: assignment.Version,
		Acceptance: protocol.WorkAcceptance{
			ID:           "acceptance-completion-lifecycle",
			ExecutionID:  executionID,
			PlanID:       planID,
			WorkItemID:   workID,
			SpecID:       specID,
			AssignmentID: assignmentID,
			SubmissionID: submissionID,
			Decision:     protocol.WorkAcceptanceAccepted,
			ReviewerKind: protocol.WorkReviewerAgent,
			ReviewerID:   "agent-lead",
			CriteriaResults: []protocol.WorkAcceptanceCriterionResult{{
				Criterion: "verified",
				Passed:    true,
			}},
		},
		Meta: meta("review-completion-lifecycle"),
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}
