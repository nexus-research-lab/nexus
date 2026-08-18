package orchestration

import (
	"context"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/storage"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

func TestMeasureExecutionCoordinatorLatency(t *testing.T) {
	if os.Getenv("NEXUS_MEASURE_BACKGROUND_LATENCY") != "1" {
		t.Skip("set NEXUS_MEASURE_BACKGROUND_LATENCY=1 to run wall-clock latency measurements")
	}
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)
	db, err := storage.OpenDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := orchestrationstore.NewRepository(cfg, db)
	service := NewService(repository)
	roomDelivered := make(chan time.Time, 1)
	reviewDelivered := make(chan time.Time, 1)
	cancellationDelivered := make(chan time.Time, 1)
	service.SetExecutionDispatchConsumer(executionDispatchConsumerFunc(func(
		_ context.Context,
		delivery ExecutionDispatchDelivery,
	) (ExecutionDispatchReceipt, error) {
		roomDelivered <- time.Now()
		return ExecutionDispatchReceipt{
			HandoffID:   "handoff-" + delivery.Binding.DispatchID,
			QueueItemID: "queue-" + delivery.Binding.DispatchID,
		}, nil
	}))
	service.SetExecutionReviewDispatchConsumer(executionReviewDispatchConsumerFunc(func(
		_ context.Context,
		delivery ExecutionReviewDispatchDelivery,
	) (ExecutionReviewDispatchReceipt, error) {
		reviewDelivered <- time.Now()
		return ExecutionReviewDispatchReceipt{
			HandoffID:   "handoff-" + delivery.Binding.ReviewDispatchID,
			QueueItemID: "queue-" + delivery.Binding.ReviewDispatchID,
		}, nil
	}))
	service.SetExecutionCancellationConsumer(cancellationConsumerFunc(func(
		context.Context,
		ExecutionCancellationDelivery,
	) (ExecutionCancellationReceipt, error) {
		cancellationDelivered <- time.Now()
		return ExecutionCancellationReceipt{
			Outcome: protocol.ExecutionCancellationOutcomeLocalRoundCancelled,
			Detail:  "latency measurement cancellation",
		}, nil
	}))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- service.RunExecutionDispatchCoordinator(
			ctx,
			"latency-worker",
			32,
			DispatchCoordinatorObserver{},
		)
	}()

	const sampleCount = 30
	roomSamples := make([]time.Duration, 0, sampleCount)
	reviewSamples := make([]time.Duration, 0, sampleCount)
	cancellationSamples := make([]time.Duration, 0, sampleCount)
	for index := 0; index < sampleCount; index++ {
		suffix := fmt.Sprintf("latency-room-%02d", index)
		snapshot := latencyCreatePlannedExecution(t, repository, suffix)
		assignmentID := "assignment-" + suffix
		attemptID := "attempt-" + suffix
		dispatchID := "dispatch-" + suffix
		assignment := latencyAssignment(snapshot, assignmentID, "agent-worker", protocol.AssignmentStrategyRoomMember)
		attempt := latencyRootAttempt(snapshot, assignment, attemptID)
		attempt.DispatchID = dispatchID
		_, err = repository.Assign(context.Background(), orchestrationstore.AssignCommand{
			ExpectedExecutionVersion: snapshot.Execution.Version,
			Assignment:               assignment,
			RootAttempt:              &attempt,
			Dispatch: &protocol.ExecutionDispatch{
				ID: dispatchID, DedupeKey: "dedupe-" + suffix,
				TargetAgentID: "agent-worker", Kind: protocol.ExecutionDispatchRoomDirected,
				Status: protocol.ExecutionDispatchStatusPending, Instruction: "measure dispatch",
			},
			Meta: latencyCommandMeta("assign-" + suffix),
		})
		if err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		service.WakeExecutionDispatch()
		roomSamples = append(roomSamples, waitOrchestrationMeasurement(t, roomDelivered).Sub(start))
	}
	t.Log(formatOrchestrationLatency("execution_room_post_commit_to_consumer", roomSamples))

	for index := 0; index < sampleCount; index++ {
		suffix := fmt.Sprintf("latency-review-%02d", index)
		snapshot := latencyCreatePlannedExecution(t, repository, suffix)
		assignmentID := "assignment-" + suffix
		attemptID := "attempt-" + suffix
		dispatchID := "dispatch-" + suffix
		assignment := latencyAssignment(snapshot, assignmentID, "agent-worker", protocol.AssignmentStrategyRoomMember)
		attempt := latencyRootAttempt(snapshot, assignment, attemptID)
		attempt.DispatchID = dispatchID
		snapshot, err = repository.Assign(context.Background(), orchestrationstore.AssignCommand{
			ExpectedExecutionVersion: snapshot.Execution.Version,
			Assignment:               assignment,
			RootAttempt:              &attempt,
			Dispatch: &protocol.ExecutionDispatch{
				ID: dispatchID, DedupeKey: "dedupe-" + suffix,
				TargetAgentID: "agent-worker", Kind: protocol.ExecutionDispatchRoomDirected,
				Status: protocol.ExecutionDispatchStatusPending, Instruction: "prepare review",
			},
			Meta: latencyCommandMeta("assign-" + suffix),
		})
		if err != nil {
			t.Fatal(err)
		}
		service.WakeExecutionDispatch()
		_ = waitOrchestrationMeasurement(t, roomDelivered)
		assignment = latencyFindAssignment(t, snapshot, assignmentID)
		attempt = latencyFindAttempt(t, snapshot, attemptID)
		snapshot, err = repository.StartAttempt(context.Background(), orchestrationstore.StartAttemptCommand{
			ExpectedExecutionVersion:  snapshot.Execution.Version,
			ExpectedAssignmentVersion: assignment.Version,
			ExpectedAttemptVersion:    attempt.Version,
			Attempt:                   attempt,
			Meta:                      latencyCommandMeta("start-" + suffix),
		})
		if err != nil {
			t.Fatal(err)
		}
		attempt = latencyFindAttempt(t, snapshot, attemptID)
		attempt.Status = protocol.WorkAttemptStatusSucceeded
		snapshot, err = repository.FinishAttempt(context.Background(), orchestrationstore.FinishAttemptCommand{
			ExpectedExecutionVersion: snapshot.Execution.Version,
			ExpectedAttemptVersion:   attempt.Version,
			Attempt:                  attempt,
			Meta:                     latencyCommandMeta("finish-" + suffix),
		})
		if err != nil {
			t.Fatal(err)
		}
		assignment = latencyFindAssignment(t, snapshot, assignmentID)
		reviewDispatchID := "review-dispatch-" + suffix
		_, err = repository.Submit(context.Background(), orchestrationstore.SubmitCommand{
			ExpectedExecutionVersion:  snapshot.Execution.Version,
			ExpectedAssignmentVersion: assignment.Version,
			Submission: protocol.WorkSubmission{
				ID: "submission-" + suffix, ExecutionID: snapshot.Execution.ID,
				PlanID: snapshot.Plan.ID, WorkItemID: assignment.WorkItemID,
				SpecID: assignment.SpecID, AssignmentID: assignment.ID,
				AttemptID: attemptID, SubmitterAgentID: "agent-worker",
				ResultSummary: "latency result", Evidence: []string{"measured"},
			},
			ReviewDispatch: &protocol.ExecutionReviewDispatch{
				ID: reviewDispatchID, DedupeKey: "review-" + suffix,
				TargetAgentID: "agent-lead", Status: protocol.ExecutionReviewDispatchStatusPending,
				Instruction: "measure review dispatch",
			},
			Meta: latencyCommandMeta("submit-" + suffix),
		})
		if err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		service.WakeExecutionDispatch()
		reviewSamples = append(reviewSamples, waitOrchestrationMeasurement(t, reviewDelivered).Sub(start))
	}
	t.Log(formatOrchestrationLatency("execution_review_post_commit_to_consumer", reviewSamples))

	for index := 0; index < sampleCount; index++ {
		suffix := fmt.Sprintf("latency-cancel-%02d", index)
		snapshot := latencyCreatePlannedExecution(t, repository, suffix)
		assignmentID := "assignment-" + suffix
		attemptID := "attempt-" + suffix
		dispatchID := "dispatch-" + suffix
		assignment := latencyAssignment(snapshot, assignmentID, "agent-worker", protocol.AssignmentStrategyRoomMember)
		attempt := latencyRootAttempt(snapshot, assignment, attemptID)
		attempt.DispatchID = dispatchID
		snapshot, err = repository.Assign(context.Background(), orchestrationstore.AssignCommand{
			ExpectedExecutionVersion: snapshot.Execution.Version,
			Assignment:               assignment,
			RootAttempt:              &attempt,
			Dispatch: &protocol.ExecutionDispatch{
				ID: dispatchID, DedupeKey: "dedupe-" + suffix,
				TargetAgentID: "agent-worker", Kind: protocol.ExecutionDispatchRoomDirected,
				Status: protocol.ExecutionDispatchStatusPending, Instruction: "prepare cancellation",
			},
			Meta: latencyCommandMeta("assign-" + suffix),
		})
		if err != nil {
			t.Fatal(err)
		}
		service.WakeExecutionDispatch()
		_ = waitOrchestrationMeasurement(t, roomDelivered)
		assignment = latencyFindAssignment(t, snapshot, assignmentID)
		attempt = latencyFindAttempt(t, snapshot, attemptID)
		attempt.RuntimeSessionKey = "runtime-session-" + suffix
		attempt.RuntimeRoundID = "runtime-round-" + suffix
		attempt.AgentRoundID = "agent-round-" + suffix
		snapshot, err = repository.StartAttempt(context.Background(), orchestrationstore.StartAttemptCommand{
			ExpectedExecutionVersion:  snapshot.Execution.Version,
			ExpectedAssignmentVersion: assignment.Version,
			ExpectedAttemptVersion:    attempt.Version,
			Attempt:                   attempt,
			Meta:                      latencyCommandMeta("start-" + suffix),
		})
		if err != nil {
			t.Fatal(err)
		}
		_, err = repository.Abandon(context.Background(), orchestrationstore.AbandonCommand{
			ExecutionID: snapshot.Execution.ID, ExpectedExecutionVersion: snapshot.Execution.Version,
			Reason: "latency measurement", Meta: latencyCommandMeta("abandon-" + suffix),
		})
		if err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		service.WakeExecutionDispatch()
		cancellationSamples = append(cancellationSamples, waitOrchestrationMeasurement(t, cancellationDelivered).Sub(start))
	}
	t.Log(formatOrchestrationLatency("execution_cancellation_post_commit_to_consumer", cancellationSamples))

	cancel()
	if err = <-done; err != nil {
		t.Fatal(err)
	}
}

func TestMeasureExecutionLostWakeAuditLatency(t *testing.T) {
	if os.Getenv("NEXUS_MEASURE_BACKGROUND_LATENCY") != "1" {
		t.Skip("set NEXUS_MEASURE_BACKGROUND_LATENCY=1 to run wall-clock latency measurements")
	}
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)
	db, err := storage.OpenDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := orchestrationstore.NewRepository(cfg, db)
	service := NewService(repository)
	delivered := make(chan time.Time, 1)
	service.SetExecutionDispatchConsumer(executionDispatchConsumerFunc(func(
		_ context.Context,
		delivery ExecutionDispatchDelivery,
	) (ExecutionDispatchReceipt, error) {
		delivered <- time.Now()
		return ExecutionDispatchReceipt{
			HandoffID:   "handoff-" + delivery.Binding.DispatchID,
			QueueItemID: "queue-" + delivery.Binding.DispatchID,
		}, nil
	}))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- service.RunExecutionDispatchCoordinator(
			ctx,
			"lost-wake-worker",
			8,
			DispatchCoordinatorObserver{},
		)
	}()
	// Ensure the startup scan has completed, then intentionally omit WakeExecutionDispatch.
	time.Sleep(100 * time.Millisecond)
	snapshot := latencyCreatePlannedExecution(t, repository, "latency-lost-wake")
	assignment := latencyAssignment(
		snapshot,
		"assignment-latency-lost-wake",
		"agent-worker",
		protocol.AssignmentStrategyRoomMember,
	)
	attempt := latencyRootAttempt(snapshot, assignment, "attempt-latency-lost-wake")
	attempt.DispatchID = "dispatch-latency-lost-wake"
	_, err = repository.Assign(context.Background(), orchestrationstore.AssignCommand{
		ExpectedExecutionVersion: snapshot.Execution.Version,
		Assignment:               assignment,
		RootAttempt:              &attempt,
		Dispatch: &protocol.ExecutionDispatch{
			ID: "dispatch-latency-lost-wake", DedupeKey: "dedupe-latency-lost-wake",
			TargetAgentID: "agent-worker", Kind: protocol.ExecutionDispatchRoomDirected,
			Status: protocol.ExecutionDispatchStatusPending, Instruction: "measure lost wake audit",
		},
		Meta: latencyCommandMeta("assign-latency-lost-wake"),
	})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	select {
	case actual := <-delivered:
		t.Log(formatOrchestrationLatency(
			"execution_lost_wake_to_audit_recovery",
			[]time.Duration{actual.Sub(start)},
		))
	case <-time.After(35 * time.Second):
		t.Fatal("lost-wake audit did not recover Execution dispatch within 35 seconds")
	}
	cancel()
	if err = <-done; err != nil {
		t.Fatal(err)
	}
}

func TestMeasureSubagentCoordinatorLatency(t *testing.T) {
	if os.Getenv("NEXUS_MEASURE_BACKGROUND_LATENCY") != "1" {
		t.Skip("set NEXUS_MEASURE_BACKGROUND_LATENCY=1 to run wall-clock latency measurements")
	}
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)
	db, err := storage.OpenDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := orchestrationstore.NewRepository(cfg, db)
	service := NewService(repository)
	reconciled := make(chan time.Time, 1)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- service.RunSubagentReconciliationCoordinator(
			ctx,
			time.Now().UTC(),
			32,
			func(kind string, result SubagentReconciliationResult) {
				if kind == "deadline" && result.Reconciled > 0 {
					reconciled <- time.Now()
				}
			},
			nil,
		)
	}()

	const sampleCount = 30
	samples := make([]time.Duration, 0, sampleCount)
	for index := 0; index < sampleCount; index++ {
		suffix := fmt.Sprintf("latency-subagent-%02d", index)
		snapshot := latencyCreatePlannedExecution(t, repository, suffix)
		assignmentID := "assignment-" + suffix
		rootAttemptID := "attempt-root-" + suffix
		assignment := latencyAssignment(snapshot, assignmentID, "agent-worker", protocol.AssignmentStrategySelf)
		rootAttempt := latencyRootAttempt(snapshot, assignment, rootAttemptID)
		snapshot, err = repository.Assign(context.Background(), orchestrationstore.AssignCommand{
			ExpectedExecutionVersion: snapshot.Execution.Version,
			Assignment:               assignment,
			RootAttempt:              &rootAttempt,
			Meta:                     latencyCommandMeta("assign-" + suffix),
		})
		if err != nil {
			t.Fatal(err)
		}
		assignment = latencyFindAssignment(t, snapshot, assignmentID)
		rootAttempt = latencyFindAttempt(t, snapshot, rootAttemptID)
		rootAttempt.RuntimeSessionKey = "runtime-session-" + suffix
		rootAttempt.RuntimeRoundID = "runtime-round-" + suffix
		rootAttempt.AgentRoundID = "agent-round-" + suffix
		snapshot, err = repository.StartAttempt(context.Background(), orchestrationstore.StartAttemptCommand{
			ExpectedExecutionVersion:  snapshot.Execution.Version,
			ExpectedAssignmentVersion: assignment.Version,
			ExpectedAttemptVersion:    rootAttempt.Version,
			Attempt:                   rootAttempt,
			Meta:                      latencyCommandMeta("start-root-" + suffix),
		})
		if err != nil {
			t.Fatal(err)
		}
		assignment = latencyFindAssignment(t, snapshot, assignmentID)
		childAttempt := protocol.WorkAttempt{
			ID: "attempt-child-" + suffix, ExecutionID: snapshot.Execution.ID,
			PlanID: snapshot.Plan.ID, WorkItemID: assignment.WorkItemID,
			SpecID: assignment.SpecID, AssignmentID: assignment.ID,
			ParentAttemptID: rootAttemptID, ExecutorKind: protocol.AttemptExecutorSubagent,
			ParentAgentID: "agent-worker", RuntimeSessionKey: "child-session-" + suffix,
			SDKSessionID: "sdk-session-" + suffix, ToolUseID: "tool-" + suffix,
			Status: protocol.WorkAttemptStatusRunning,
		}
		snapshot, err = repository.StartAttempt(context.Background(), orchestrationstore.StartAttemptCommand{
			ExpectedExecutionVersion:  snapshot.Execution.Version,
			ExpectedAssignmentVersion: assignment.Version,
			ExpectedAttemptVersion:    0,
			Attempt:                   childAttempt,
			Meta:                      latencyCommandMeta("start-child-" + suffix),
		})
		if err != nil {
			t.Fatal(err)
		}
		childAttempt = latencyFindAttempt(t, snapshot, childAttempt.ID)
		deadline := time.Now().UTC().Add(10 * time.Millisecond)
		exitedAt := deadline.Add(-protocol.SubagentReconciliationGrace)
		_, err = repository.ScheduleSubagentReconciliation(
			context.Background(),
			orchestrationstore.ScheduleSubagentReconciliationCommand{
				ExecutionID: snapshot.Execution.ID, ExpectedExecutionVersion: snapshot.Execution.Version,
				ExpectedAttemptVersion: childAttempt.Version, AttemptID: childAttempt.ID,
				ParentRoundExitedAt: exitedAt, ReconcileAfter: deadline,
				Meta: latencyCommandMeta("schedule-child-" + suffix),
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		service.WakeSubagentReconciliation()
		actual := waitOrchestrationMeasurement(t, reconciled)
		lateness := actual.Sub(deadline)
		if lateness < 0 {
			lateness = 0
		}
		samples = append(samples, lateness)
	}
	t.Log(formatOrchestrationLatency("subagent_deadline_to_durable_reconciliation", samples))
	cancel()
	if err = <-done; err != nil {
		t.Fatal(err)
	}
}

func TestMeasureRecoveryCoordinatorLatency(t *testing.T) {
	if os.Getenv("NEXUS_MEASURE_BACKGROUND_LATENCY") != "1" {
		t.Skip("set NEXUS_MEASURE_BACKGROUND_LATENCY=1 to run wall-clock latency measurements")
	}
	cfg := handlertest.NewConfig(t)
	handlertest.MigrateSQLite(t, cfg.DatabaseURL)
	db, err := storage.OpenDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	repository := orchestrationstore.NewRepository(cfg, db)
	service := NewService(repository)
	service.SetExplicitGoalBindingGateway(&confirmingGoalBindingGateway{})
	completionRecovered := make(chan time.Time, 1)
	goalConfirmed := make(chan time.Time, 1)
	proposalMaterialized := make(chan time.Time, 1)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- service.RunRecoveryCoordinator(ctx, 32, RecoveryCoordinatorObserver{
			OnCompletionAudit: func(result CompletionAuditRecoveryResult) {
				if result.Completed > 0 {
					completionRecovered <- time.Now()
				}
			},
			OnGoalConfirmation: func(result GoalConfirmationRecoveryResult) {
				if result.Confirmed > 0 {
					goalConfirmed <- time.Now()
				}
			},
			OnPlanProposal: func(result PlanProposalRecoveryResult) {
				if result.Materialized > 0 {
					proposalMaterialized <- time.Now()
				}
			},
		})
	}()
	// Let the startup recovery pass reach its idle wait before timed mutations.
	time.Sleep(20 * time.Millisecond)

	const sampleCount = 30
	completionSamples := make([]time.Duration, 0, sampleCount)
	goalSamples := make([]time.Duration, 0, sampleCount)
	proposalSamples := make([]time.Duration, 0, sampleCount)
	for index := 0; index < sampleCount; index++ {
		suffix := fmt.Sprintf("latency-completion-%02d", index)
		latencyPrepareCompletionAudit(t, repository, suffix)
		start := time.Now()
		service.WakeOrchestrationRecovery()
		completionSamples = append(
			completionSamples,
			waitOrchestrationMeasurement(t, completionRecovered).Sub(start),
		)
	}
	t.Log(formatOrchestrationLatency("completion_audit_post_commit_to_completed", completionSamples))

	for index := 0; index < sampleCount; index++ {
		suffix := fmt.Sprintf("latency-goal-confirmation-%02d", index)
		goalID := "goal-" + suffix
		sessionKey := protocol.BuildAgentSessionKey("agent-lead", "ws", protocol.RoomTypeDM, suffix, "")
		if _, insertErr := db.ExecContext(context.Background(), `
INSERT INTO session_goals (
    goal_id, session_key, objective, status, version, metadata_json
) VALUES (?, ?, ?, 'active', 1, '{}')`, goalID, sessionKey, "measure Goal confirmation"); insertErr != nil {
			t.Fatal(insertErr)
		}
		_, err = repository.Create(context.Background(), orchestrationstore.CreateCommand{
			Execution: protocol.Execution{
				ID: "execution-" + suffix, OwnerUserID: "owner-latency",
				SessionKey: sessionKey,
				ScopeKind:  protocol.ExecutionScopeDM, CoordinatorAgentID: "agent-lead",
				Origin: protocol.ExecutionOriginUserRequest, Objective: "measure Goal confirmation",
				CompletionCriteria: []string{"confirmed"}, GoalID: goalID,
				GoalObjectiveRevision: 1, GoalActivationOrigin: protocol.GoalActivationOriginUserExplicit,
				GoalActivationReason: protocol.GoalActivationReasonPersistenceRequested,
				Status:               protocol.ExecutionStatusActive,
			},
			Meta: latencyCommandMeta("create-" + suffix),
		})
		if err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		service.WakeOrchestrationRecovery()
		goalSamples = append(goalSamples, waitOrchestrationMeasurement(t, goalConfirmed).Sub(start))
	}
	t.Log(formatOrchestrationLatency("goal_confirmation_post_commit_to_confirmed", goalSamples))

	for index := 0; index < sampleCount; index++ {
		suffix := fmt.Sprintf("latency-proposal-%02d", index)
		actor := coordinatorActor()
		actor.OwnerUserID = "owner-latency"
		actor.SessionKey = protocol.BuildAgentSessionKey("agent-lead", "ws", protocol.RoomTypeDM, suffix, "")
		actor.RootRoundID = "root-round-" + suffix
		actor.RuntimeRoundID = "runtime-round-" + suffix
		actor.AgentRoundID = "agent-round-" + suffix
		proposal, prepareErr := service.PreparePlanExecution(
			context.Background(),
			actor,
			PreparePlanExecutionInput{
				CommandID:    "prepare-" + suffix,
				PlanDocument: createPlanProposalDocument,
				GoalBinding:  PlanGoalBindingNone,
			},
		)
		if prepareErr != nil {
			t.Fatal(prepareErr)
		}
		deadline := time.Now().UTC().Add(10 * time.Millisecond)
		_, err = repository.MarkPlanProposalMaterializing(
			context.Background(),
			orchestrationstore.MarkPlanProposalMaterializingCommand{
				Access: proposalAccess(actor, proposal.ID), ExpectedVersion: proposal.Version,
				ReservedExecutionID:      deterministicProposalExecutionID(proposal.ID, proposal.ContentDigest),
				MaterializationCommandID: deterministicProposalCommandID(proposal.ID, proposal.ContentDigest),
				NextAttemptAt:            &deadline,
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		service.WakeOrchestrationRecovery()
		actual := waitOrchestrationMeasurement(t, proposalMaterialized)
		lateness := actual.Sub(deadline)
		if lateness < 0 {
			lateness = 0
		}
		proposalSamples = append(proposalSamples, lateness)
	}
	t.Log(formatOrchestrationLatency("plan_proposal_deadline_to_materialized", proposalSamples))

	cancel()
	if err = <-done; err != nil {
		t.Fatal(err)
	}
}

func latencyPrepareCompletionAudit(
	t *testing.T,
	repository *orchestrationstore.Repository,
	suffix string,
) {
	t.Helper()
	snapshot := latencyCreatePlannedExecution(t, repository, suffix)
	assignmentID := "assignment-" + suffix
	attemptID := "attempt-" + suffix
	submissionID := "submission-" + suffix
	assignment := latencyAssignment(snapshot, assignmentID, "agent-worker", protocol.AssignmentStrategySelf)
	attempt := latencyRootAttempt(snapshot, assignment, attemptID)
	var err error
	snapshot, err = repository.Assign(context.Background(), orchestrationstore.AssignCommand{
		ExpectedExecutionVersion: snapshot.Execution.Version,
		Assignment:               assignment,
		RootAttempt:              &attempt,
		Meta:                     latencyCommandMeta("assign-" + suffix),
	})
	if err != nil {
		t.Fatal(err)
	}
	assignment = latencyFindAssignment(t, snapshot, assignmentID)
	attempt = latencyFindAttempt(t, snapshot, attemptID)
	snapshot, err = repository.StartAttempt(context.Background(), orchestrationstore.StartAttemptCommand{
		ExpectedExecutionVersion:  snapshot.Execution.Version,
		ExpectedAssignmentVersion: assignment.Version,
		ExpectedAttemptVersion:    attempt.Version,
		Attempt:                   attempt,
		Meta:                      latencyCommandMeta("start-" + suffix),
	})
	if err != nil {
		t.Fatal(err)
	}
	attempt = latencyFindAttempt(t, snapshot, attemptID)
	attempt.Status = protocol.WorkAttemptStatusSucceeded
	snapshot, err = repository.FinishAttempt(context.Background(), orchestrationstore.FinishAttemptCommand{
		ExpectedExecutionVersion: snapshot.Execution.Version,
		ExpectedAttemptVersion:   attempt.Version,
		Attempt:                  attempt,
		Meta:                     latencyCommandMeta("finish-" + suffix),
	})
	if err != nil {
		t.Fatal(err)
	}
	assignment = latencyFindAssignment(t, snapshot, assignmentID)
	snapshot, err = repository.Submit(context.Background(), orchestrationstore.SubmitCommand{
		ExpectedExecutionVersion:  snapshot.Execution.Version,
		ExpectedAssignmentVersion: assignment.Version,
		Submission: protocol.WorkSubmission{
			ID: submissionID, ExecutionID: snapshot.Execution.ID, PlanID: snapshot.Plan.ID,
			WorkItemID: assignment.WorkItemID, SpecID: assignment.SpecID,
			AssignmentID: assignment.ID, AttemptID: attempt.ID,
			SubmitterAgentID: "agent-worker", ResultSummary: "measured",
			Evidence: []string{"verified"},
		},
		Meta: latencyCommandMeta("submit-" + suffix),
	})
	if err != nil {
		t.Fatal(err)
	}
	assignment = latencyFindAssignment(t, snapshot, assignmentID)
	_, err = repository.Review(context.Background(), orchestrationstore.ReviewCommand{
		ExpectedExecutionVersion:  snapshot.Execution.Version,
		ExpectedAssignmentVersion: assignment.Version,
		Acceptance: protocol.WorkAcceptance{
			ID: "acceptance-" + suffix, ExecutionID: snapshot.Execution.ID,
			PlanID: snapshot.Plan.ID, WorkItemID: assignment.WorkItemID,
			SpecID: assignment.SpecID, AssignmentID: assignment.ID,
			SubmissionID: submissionID, Decision: protocol.WorkAcceptanceAccepted,
			ReviewerKind: protocol.WorkReviewerAgent, ReviewerID: "agent-lead",
			CriteriaResults: []protocol.WorkAcceptanceCriterionResult{{Criterion: "measured", Passed: true}},
		},
		Meta: latencyCommandMeta("review-" + suffix),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func latencyCreatePlannedExecution(
	t *testing.T,
	repository *orchestrationstore.Repository,
	suffix string,
) *protocol.ExecutionSnapshot {
	t.Helper()
	executionID := "execution-" + suffix
	planID := "plan-" + suffix
	workID := "work-" + suffix
	specID := "spec-" + suffix
	snapshot, err := repository.Create(context.Background(), orchestrationstore.CreateCommand{
		Execution: protocol.Execution{
			ID: executionID, OwnerUserID: "owner-latency",
			SessionKey: "room:room-latency:group:" + suffix,
			ScopeKind:  protocol.ExecutionScopeRoom, RoomID: "room-latency",
			ConversationID: suffix, CoordinatorAgentID: "agent-lead",
			Origin: protocol.ExecutionOriginUserRequest, Objective: "measure " + suffix,
			CompletionCriteria: []string{"measured"}, Status: protocol.ExecutionStatusActive,
		},
		Meta: latencyCommandMeta("create-" + suffix),
	})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err = repository.WritePlan(context.Background(), orchestrationstore.WritePlanCommand{
		ExecutionID: executionID, ExpectedExecutionVersion: snapshot.Execution.Version,
		Plan: protocol.ExecutionPlanRevision{
			ID: planID, ExecutionID: executionID, Revision: 1,
			Status: protocol.PlanRevisionStatusActive, CreatedByAgentID: "agent-lead",
			RevisionReason: "latency measurement",
		},
		WorkItems: []orchestrationstore.PlanWorkItem{{
			WorkItem: protocol.WorkItem{ID: workID, ExecutionID: executionID, LogicalKey: workID, Kind: protocol.WorkItemKindProduce},
			Spec: protocol.WorkItemSpec{
				ID: specID, WorkItemID: workID, ExecutionID: executionID, Version: 1,
				Subject: "measure", Objective: "measure dispatch", Deliverable: "measurement",
				AcceptanceCriteria: []string{"measured"}, SpecHash: "hash-" + suffix,
				CreatedByAgentID: "agent-lead",
			},
			State: protocol.WorkItemState{WorkItemID: workID, ExecutionID: executionID, CurrentSpecID: specID, Status: protocol.WorkItemStatusOpen, Version: 1},
			Item:  protocol.ExecutionPlanItem{PlanID: planID, ExecutionID: executionID, WorkItemID: workID, SpecID: specID, Required: true, Terminal: true},
			OutputClaims: []protocol.ExecutionPlanOutputClaim{{
				PlanID: planID, ExecutionID: executionID, WorkItemID: workID,
				SpecID: specID, Scope: "dir:latency/" + suffix, Mode: protocol.WorkOutputScopeExclusive,
			}},
		}},
		Meta: latencyCommandMeta("plan-" + suffix),
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func latencyAssignment(
	snapshot *protocol.ExecutionSnapshot,
	assignmentID string,
	ownerAgentID string,
	strategy protocol.AssignmentStrategy,
) protocol.WorkAssignment {
	return protocol.WorkAssignment{
		ID: assignmentID, ExecutionID: snapshot.Execution.ID, PlanID: snapshot.Plan.ID,
		WorkItemID: snapshot.WorkItems[0].ID, SpecID: snapshot.WorkItemSpecs[0].ID,
		OwnerAgentID: ownerAgentID, AssignedByAgentID: "agent-lead",
		ReturnToAgentID: "agent-lead", Strategy: strategy,
		Status: protocol.WorkAssignmentStatusAssigned,
	}
}

func latencyRootAttempt(
	snapshot *protocol.ExecutionSnapshot,
	assignment protocol.WorkAssignment,
	attemptID string,
) protocol.WorkAttempt {
	return protocol.WorkAttempt{
		ID: attemptID, ExecutionID: snapshot.Execution.ID, PlanID: snapshot.Plan.ID,
		WorkItemID: assignment.WorkItemID, SpecID: assignment.SpecID,
		AssignmentID: assignment.ID, ExecutorKind: protocol.AttemptExecutorAgent,
		ExecutorAgentID: assignment.OwnerAgentID, Status: protocol.WorkAttemptStatusPending,
	}
}

func latencyCommandMeta(id string) orchestrationstore.CommandMeta {
	return orchestrationstore.CommandMeta{
		CommandID: "command-" + id, EventID: "event-" + id,
		ActorKind: protocol.ExecutionActorSystem, ActorID: "latency-test",
	}
}

func latencyFindAssignment(
	t *testing.T,
	snapshot *protocol.ExecutionSnapshot,
	id string,
) protocol.WorkAssignment {
	t.Helper()
	for _, item := range snapshot.Assignments {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("assignment %s not found", id)
	return protocol.WorkAssignment{}
}

func latencyFindAttempt(
	t *testing.T,
	snapshot *protocol.ExecutionSnapshot,
	id string,
) protocol.WorkAttempt {
	t.Helper()
	for _, item := range snapshot.Attempts {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("attempt %s not found", id)
	return protocol.WorkAttempt{}
}

func waitOrchestrationMeasurement(t *testing.T, values <-chan time.Time) time.Time {
	t.Helper()
	select {
	case value := <-values:
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for orchestration latency measurement")
		return time.Time{}
	}
}

func formatOrchestrationLatency(name string, samples []time.Duration) string {
	ordered := append([]time.Duration(nil), samples...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	var total time.Duration
	for _, sample := range ordered {
		total += sample
	}
	return fmt.Sprintf(
		"LATENCY %s n=%d min=%s p50=%s p95=%s max=%s mean=%s",
		name, len(ordered), ordered[0],
		ordered[int(float64(len(ordered)-1)*0.50)],
		ordered[int(float64(len(ordered)-1)*0.95)],
		ordered[len(ordered)-1], total/time.Duration(len(ordered)),
	)
}
