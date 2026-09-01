package automation

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
)

func TestDeletionClaimFencesNewMutationsAndRunLedger(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil, nil,
		permissionctx.NewContext(), &fakeWorkspaceReader{}, nil)
	task := createDeletionClaimTestTask(t, service)
	expectedVersion := task.ConfigurationVersion
	claim, err := service.repository.ClaimScheduledTaskDeletion(context.Background(), automationstore.TaskDeletionClaimInput{
		OwnerUserID: task.OwnerUserID, JobID: task.JobID, ExpectedVersion: &expectedVersion,
		DeletionToken: "delete-token-1", ClaimedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("ClaimScheduledTaskDeletion() error = %v", err)
	}
	if !claim.Claimed || claim.Token != "delete-token-1" || claim.Task.Enabled ||
		claim.Task.DeletionState != automationdomain.TaskDeletionStateDeleting ||
		claim.Task.ConfigurationVersion != expectedVersion+1 {
		t.Fatalf("unexpected deletion claim: %+v", claim)
	}

	replayed, err := service.repository.ClaimScheduledTaskDeletion(context.Background(), automationstore.TaskDeletionClaimInput{
		OwnerUserID: task.OwnerUserID, JobID: task.JobID, ExpectedVersion: &expectedVersion,
		DeletionToken: "different-token", ClaimedAt: time.Now().UTC(),
	})
	if err != nil || replayed.Claimed || replayed.Token != claim.Token {
		t.Fatalf("deletion replay must reuse exact token: result=%+v err=%v", replayed, err)
	}

	claimed, err := service.repository.ClaimScheduledTaskRuntime(context.Background(), automationstore.JobRuntimeClaimInput{
		JobID: task.JobID, RunID: "run-after-delete", StartedAt: time.Now().UTC(), AllowDisabled: true,
	})
	if err != nil || claimed {
		t.Fatalf("runtime claim crossed deletion fence: claimed=%v err=%v", claimed, err)
	}
	err = service.repository.InsertRunPending(context.Background(), automationstore.RunPendingInput{
		RunID: "run-after-delete", JobID: task.JobID, OwnerUserID: task.OwnerUserID,
	})
	if !errors.Is(err, automationdomain.ErrTaskDeleting) {
		t.Fatalf("InsertRunPending() error = %v, want ErrTaskDeleting", err)
	}
	name := "must-not-update"
	if _, err = service.UpdateTask(context.Background(), task.JobID, automationdomain.UpdateJobInput{Name: &name}); !errors.Is(err, automationdomain.ErrTaskDeleting) {
		t.Fatalf("UpdateTask() error = %v, want ErrTaskDeleting", err)
	}
}

func TestFinalizeDeletionCancelsAllRunsAndPreservesUnconfirmedDelivery(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil, nil,
		permissionctx.NewContext(), &fakeWorkspaceReader{}, nil)
	task := createDeletionClaimTestTask(t, service)
	ctx := context.Background()
	for _, runID := range []string{"run-active-1", "run-active-2"} {
		if err := service.repository.InsertRunPending(ctx, automationstore.RunPendingInput{
			RunID: runID, JobID: task.JobID, OwnerUserID: task.OwnerUserID,
			Status: automationdomain.RunStatusRunning, DeliveryStatus: automationdomain.DeliveryStatusPending,
		}); err != nil {
			t.Fatalf("InsertRunPending(%s): %v", runID, err)
		}
	}
	for _, item := range []struct{ runID, status string }{
		{"run-delivery-failed", automationdomain.DeliveryStatusFailed},
		{"run-delivery-unknown", automationdomain.DeliveryStatusRetrying},
		{"run-delivery-pending", automationdomain.DeliveryStatusPending},
	} {
		if err := service.repository.InsertRunPending(ctx, automationstore.RunPendingInput{
			RunID: item.runID, JobID: task.JobID, OwnerUserID: task.OwnerUserID,
			Status: automationdomain.RunStatusSucceeded,
		}); err != nil {
			t.Fatalf("InsertRunPending(%s): %v", item.runID, err)
		}
		if _, err := db.Exec(`UPDATE automation_task_runs SET delivery_status = ?, delivery_attempt_id = CASE WHEN ? = 'retrying' THEN 'attempt-1' ELSE NULL END WHERE run_id = ?`, item.status, item.status, item.runID); err != nil {
			t.Fatalf("seed delivery %s: %v", item.runID, err)
		}
	}
	terminalFinishedAt := time.Now().UTC().Add(-time.Hour)
	if err := service.repository.InsertRunPending(ctx, automationstore.RunPendingInput{
		RunID: "run-terminal-stale-block", JobID: task.JobID, OwnerUserID: task.OwnerUserID,
		Status: automationdomain.RunStatusSucceeded,
	}); err != nil {
		t.Fatalf("InsertRunPending terminal blocked run: %v", err)
	}
	if _, err := db.Exec(`UPDATE automation_task_runs
SET finished_at = ?, error_message = 'historical result', delivery_status = 'succeeded',
    block_state = 'awaiting_approval', blocked_request_id = 'stale-request'
WHERE run_id = 'run-terminal-stale-block'`, terminalFinishedAt); err != nil {
		t.Fatalf("seed terminal blocked run: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO automation_permission_requests (
request_id, owner_user_id, job_id, run_id, policy_revision, kind, status,
tool_name, effect, input_fingerprint, capability_json, input_summary_json
) VALUES (?, ?, ?, ?, 1, 'tool', 'pending', 'WebSearch', 'read', 'fingerprint', '{}', '{}')`,
		"permission-delete", task.OwnerUserID, task.JobID, "run-active-1"); err != nil {
		t.Fatalf("seed permission request: %v", err)
	}
	claim, err := service.repository.ClaimScheduledTaskDeletion(ctx, automationstore.TaskDeletionClaimInput{
		OwnerUserID: task.OwnerUserID, JobID: task.JobID, DeletionToken: "delete-token-2", ClaimedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("claim deletion: %v", err)
	}
	now := time.Now().UTC()
	finalized, err := service.repository.FinalizeScheduledTaskDeletion(ctx, automationstore.TaskDeleteFinalizationInput{
		OwnerUserID: task.OwnerUserID, JobID: task.JobID, DeletionToken: claim.Token,
		FinishedAt: now, ActiveRunMessage: "deleted", DeliveryDeadLetter: now,
		DeliveryError: "deleted before retry", UnconfirmedDeliveryError: "outcome unconfirmed; no automatic retry",
		DeleteEvent: automationstore.TaskEventInput{
			EventID: "delete-event-1", JobID: task.JobID, OwnerUserID: task.OwnerUserID,
			AgentID: task.AgentID, Action: automationdomain.TaskEventActionDelete,
		},
	})
	if err != nil {
		t.Fatalf("FinalizeScheduledTaskDeletion() error = %v", err)
	}
	if len(finalized.CancelledRuns) != 2 || len(finalized.SupersededPermissionRequests) != 1 ||
		len(finalized.DeadLetteredDeliveryRunIDs) != 1 || len(finalized.UnconfirmedDeliveryRunIDs) != 1 ||
		len(finalized.NotAttemptedDeliveryRunIDs) != 1 {
		t.Fatalf("unexpected finalization result: %+v", finalized)
	}
	for _, runID := range []string{"run-active-1", "run-active-2"} {
		var status, activeDeliveryStatus string
		var activeDeliveryDeadLetter sql.NullTime
		if err = db.QueryRow(`SELECT status, delivery_status, delivery_dead_letter_at FROM automation_task_runs WHERE run_id = ?`, runID).Scan(&status, &activeDeliveryStatus, &activeDeliveryDeadLetter); err != nil || status != automationdomain.RunStatusCancelled {
			t.Fatalf("run %s status=%s err=%v", runID, status, err)
		}
		if activeDeliveryStatus != automationdomain.DeliveryStatusNotAttempted || !activeDeliveryDeadLetter.Valid {
			t.Fatalf("active run %s delivery was not durably closed: status=%s deadletter=%v", runID, activeDeliveryStatus, activeDeliveryDeadLetter)
		}
	}
	var terminalStatus, terminalError, terminalDeliveryStatus, terminalBlockState string
	var persistedTerminalFinishedAt sql.NullTime
	if err = db.QueryRow(`SELECT status, finished_at, error_message, delivery_status, block_state
FROM automation_task_runs WHERE run_id = 'run-terminal-stale-block'`).Scan(
		&terminalStatus, &persistedTerminalFinishedAt, &terminalError, &terminalDeliveryStatus, &terminalBlockState,
	); err != nil {
		t.Fatalf("read terminal blocked run: %v", err)
	}
	if terminalStatus != automationdomain.RunStatusSucceeded || !persistedTerminalFinishedAt.Valid ||
		!persistedTerminalFinishedAt.Time.Equal(terminalFinishedAt) || terminalError != "historical result" ||
		terminalDeliveryStatus != automationdomain.DeliveryStatusSucceeded || terminalBlockState != "" {
		t.Fatalf("terminal history was overwritten while clearing stale block: status=%s finished=%v error=%q delivery=%s block=%q",
			terminalStatus, persistedTerminalFinishedAt, terminalError, terminalDeliveryStatus, terminalBlockState)
	}
	var permissionStatus string
	if err = db.QueryRow(`SELECT status FROM automation_permission_requests WHERE request_id = 'permission-delete'`).Scan(&permissionStatus); err != nil || permissionStatus != automationdomain.PermissionRequestStatusSuperseded {
		t.Fatalf("permission status=%s err=%v", permissionStatus, err)
	}
	var deliveryStatus string
	var deadLetterAt sql.NullTime
	var deliveryError, attemptID sql.NullString
	if err = db.QueryRow(`SELECT delivery_status, delivery_dead_letter_at, delivery_error, delivery_attempt_id FROM automation_task_runs WHERE run_id = 'run-delivery-unknown'`).Scan(&deliveryStatus, &deadLetterAt, &deliveryError, &attemptID); err != nil {
		t.Fatalf("read unknown delivery: %v", err)
	}
	if deliveryStatus != automationdomain.DeliveryStatusRetrying || !deadLetterAt.Valid || !deliveryError.Valid || attemptID.Valid {
		t.Fatalf("unconfirmed delivery lost fail-closed state: status=%s deadletter=%v error=%v attempt=%v", deliveryStatus, deadLetterAt, deliveryError, attemptID)
	}
	if err = db.QueryRow(`SELECT delivery_status, delivery_dead_letter_at FROM automation_task_runs WHERE run_id = 'run-delivery-pending'`).Scan(&deliveryStatus, &deadLetterAt); err != nil {
		t.Fatalf("read pending delivery: %v", err)
	}
	if deliveryStatus != automationdomain.DeliveryStatusNotAttempted || !deadLetterAt.Valid {
		t.Fatalf("pending delivery was not closed before delete: status=%s deadletter=%v", deliveryStatus, deadLetterAt)
	}
	var taskCount int
	if err = db.QueryRow(`SELECT COUNT(1) FROM automation_scheduled_tasks WHERE job_id = ?`, task.JobID).Scan(&taskCount); err != nil || taskCount != 0 {
		t.Fatalf("task count=%d err=%v", taskCount, err)
	}
	var eventDetail string
	if err = db.QueryRow(`SELECT detail_json FROM automation_task_events WHERE event_id = 'delete-event-1'`).Scan(&eventDetail); err != nil {
		t.Fatalf("read delete event: %v", err)
	}
	if !containsAll(eventDetail, "unconfirmed_delivery_run_ids", "run-delivery-unknown", "delivery_outcome_unconfirmed", "not_attempted_delivery_run_ids", "run-delivery-pending") {
		t.Fatalf("delete event omitted unknown delivery fact: %s", eventDetail)
	}
}

func TestSchedulerRecoversClaimedTaskDeletion(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil, nil,
		permissionctx.NewContext(), &fakeWorkspaceReader{}, nil)
	task := createDeletionClaimTestTask(t, service)
	claim, err := service.repository.ClaimScheduledTaskDeletion(context.Background(), automationstore.TaskDeletionClaimInput{
		OwnerUserID: task.OwnerUserID, JobID: task.JobID, DeletionToken: "recovery-token", ClaimedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("claim deletion: %v", err)
	}
	service.startTaskDeletionRecovery(claim.Task)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		current, loadErr := service.repository.GetScheduledTask(context.Background(), task.OwnerUserID, task.JobID)
		if loadErr != nil {
			t.Fatalf("GetScheduledTask(): %v", loadErr)
		}
		if current == nil {
			events, eventErr := service.repository.ListTaskEventsByJob(context.Background(), task.OwnerUserID, task.JobID, 10)
			foundDelete := false
			for _, event := range events {
				if event.Action == automationdomain.TaskEventActionDelete {
					foundDelete = true
					break
				}
			}
			if eventErr != nil || !foundDelete {
				t.Fatalf("recovered deletion omitted durable event: events=%+v err=%v", events, eventErr)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("scheduler did not resume durable task deletion")
}

func TestDeleteKeepsClaimWhenActiveScriptBelongsToAnotherInstance(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil, nil,
		permissionctx.NewContext(), &fakeWorkspaceReader{}, nil)
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name: "remote script", AgentID: "agent-1", Instruction: "sleep 30",
		ExecutionKind: automationdomain.ExecutionKindScript,
		Schedule:      automationdomain.Schedule{Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: intRef(3600), Timezone: "UTC"},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated},
		Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone}, Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateTask(): %v", err)
	}
	if err = service.repository.InsertRunPending(context.Background(), automationstore.RunPendingInput{
		RunID: "remote-script-run", JobID: task.JobID, OwnerUserID: task.OwnerUserID,
		Status: automationdomain.RunStatusRunning,
	}); err != nil {
		t.Fatalf("InsertRunPending(): %v", err)
	}
	if _, err = db.Exec(`UPDATE automation_task_runs SET effect_started = 1 WHERE run_id = 'remote-script-run'`); err != nil {
		t.Fatalf("mark remote script effect started: %v", err)
	}
	if _, err = service.DeleteTaskAtVersion(context.Background(), task.JobID, task.ConfigurationVersion); !TaskDeletionPrepared(err) ||
		!errors.Is(err, ErrExecutionAttemptOwnershipUnconfirmed) {
		t.Fatalf("delete must remain prepared when script ownership is unknown: %v", err)
	}
	persisted, err := service.repository.GetScheduledTask(context.Background(), task.OwnerUserID, task.JobID)
	if err != nil || persisted == nil || persisted.DeletionState != automationdomain.TaskDeletionStateReviewRequired {
		t.Fatalf("unknown script deletion claim was finalized early: task=%+v err=%v", persisted, err)
	}
	run, err := service.repository.GetRun(context.Background(), task.OwnerUserID, task.JobID, "remote-script-run")
	if err != nil || run == nil || run.Status != automationdomain.RunStatusRunning {
		t.Fatalf("unknown remote script was falsely cancelled: run=%+v err=%v", run, err)
	}
	if err = service.bootstrapRuntime(context.Background()); err != nil {
		t.Fatalf("bootstrapRuntime(): %v", err)
	}
	service.mu.Lock()
	_, recoveryLooped := service.deletionRecoveries[task.JobID]
	service.mu.Unlock()
	if recoveryLooped {
		t.Fatal("review-required deletion without a local exact attempt entered automatic recovery")
	}
	terminalAt := time.Now().UTC()
	if _, err = db.Exec(`UPDATE automation_task_runs
SET status = 'cancelled', finished_at = ?, error_message = 'deletion suppressed terminal',
    delivery_status = 'not_attempted', delivery_error = 'task deletion already claimed',
    delivery_dead_letter_at = ?
WHERE run_id = 'remote-script-run' AND status = 'running'`, terminalAt, terminalAt); err != nil {
		t.Fatalf("persist suppressed terminal proof: %v", err)
	}
	if err = service.bootstrapRuntime(context.Background()); err != nil {
		t.Fatalf("bootstrapRuntime() after terminal proof: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		persisted, loadErr := service.repository.GetScheduledTask(context.Background(), task.OwnerUserID, task.JobID)
		if loadErr != nil {
			t.Fatalf("GetScheduledTask(): %v", loadErr)
		}
		if persisted == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("review-required deletion did not finalize after durable terminal proof")
}

func TestOwnerConfirmedStoppedFinalizesReviewDeletionAtExactVersion(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil, nil,
		permissionctx.NewContext(), &fakeWorkspaceReader{}, nil)
	task := createDeletionClaimTestTask(t, service)
	ownerCtx := contextForOwner(context.Background(), task.OwnerUserID)
	runID := "review-confirm-run"
	requestID := "review-confirm-permission"
	if err := service.repository.InsertRunPending(ownerCtx, automationstore.RunPendingInput{
		RunID: runID, JobID: task.JobID, OwnerUserID: task.OwnerUserID,
		Status:         automationdomain.RunStatusRunning,
		SessionKey:     protocol.BuildAgentSessionKey(task.AgentID, "automation", "dm", "review-confirm", ""),
		RoundID:        "remote-round",
		DeliveryStatus: automationdomain.DeliveryStatusPending,
	}); err != nil {
		t.Fatalf("InsertRunPending(): %v", err)
	}
	if _, err := db.Exec(`UPDATE automation_task_runs
SET block_state = 'awaiting_approval', blocked_request_id = ?
WHERE run_id = ?`, requestID, runID); err != nil {
		t.Fatalf("block active run: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO automation_permission_requests (
request_id, owner_user_id, job_id, run_id, policy_revision, kind, status,
tool_name, effect, input_fingerprint, capability_json, input_summary_json
) VALUES (?, ?, ?, ?, 1, 'tool', 'pending', 'WebSearch', 'read', 'review-confirm', '{}', '{}')`,
		requestID, task.OwnerUserID, task.JobID, runID); err != nil {
		t.Fatalf("seed permission request: %v", err)
	}

	if _, err := service.DeleteTaskAtVersion(ownerCtx, task.JobID, task.ConfigurationVersion); !TaskDeletionPrepared(err) || !errors.Is(err, ErrExecutionAttemptOwnershipUnconfirmed) {
		t.Fatalf("DeleteTaskAtVersion() must stop at review_required: %v", err)
	}
	claimed, err := service.repository.GetScheduledTask(ownerCtx, task.OwnerUserID, task.JobID)
	if err != nil || claimed == nil || claimed.DeletionState != automationdomain.TaskDeletionStateReviewRequired {
		t.Fatalf("review claim missing: task=%+v err=%v", claimed, err)
	}
	if _, err = service.DeleteTaskAtVersion(ownerCtx, task.JobID, claimed.ConfigurationVersion); !TaskDeletionPrepared(err) || !errors.Is(err, ErrExecutionAttemptOwnershipUnconfirmed) {
		t.Fatalf("ordinary repeated DELETE must remain fail-closed: %v", err)
	}
	if _, err = service.ConfirmTaskDeletionStoppedAtVersion(ownerCtx, task.JobID, claimed.ConfigurationVersion-1); !errors.Is(err, automationdomain.ErrConfigurationVersionConflict) {
		t.Fatalf("stale confirmation error=%v, want version conflict", err)
	}
	otherOwnerCtx := contextForOwner(context.Background(), "different-owner")
	if _, err = service.ConfirmTaskDeletionStoppedAtVersion(otherOwnerCtx, task.JobID, claimed.ConfigurationVersion); !errors.Is(err, automationdomain.ErrJobNotFound) {
		t.Fatalf("cross-owner confirmation error=%v, want not found", err)
	}

	result, err := service.ConfirmTaskDeletionStoppedAtVersion(ownerCtx, task.JobID, claimed.ConfigurationVersion)
	if err != nil {
		t.Fatalf("ConfirmTaskDeletionStoppedAtVersion(): %v", err)
	}
	if result == nil || !result.Deleted || !result.CancelledActiveRun || result.CancelledRunID != runID {
		t.Fatalf("unexpected confirmation result: %+v", result)
	}
	current, err := service.repository.GetScheduledTask(ownerCtx, task.OwnerUserID, task.JobID)
	if err != nil || current != nil {
		t.Fatalf("confirmed review task still exists: task=%+v err=%v", current, err)
	}
	run, err := service.repository.GetRun(ownerCtx, task.OwnerUserID, task.JobID, runID)
	if err != nil || run == nil || run.Status != automationdomain.RunStatusCancelled ||
		run.DeliveryStatus != automationdomain.DeliveryStatusNotAttempted || run.DeliveryDeadLetterAt == nil ||
		run.BlockState != "" || run.BlockedRequestID != "" {
		t.Fatalf("active run was not atomically closed: run=%+v err=%v", run, err)
	}
	request, err := service.repository.GetPermissionRequest(ownerCtx, task.OwnerUserID, requestID)
	if err != nil || request.Status != automationdomain.PermissionRequestStatusSuperseded {
		t.Fatalf("pending permission was not superseded: request=%+v err=%v", request, err)
	}
	events, err := service.repository.ListTaskEventsByJob(ownerCtx, task.OwnerUserID, task.JobID, 10)
	if err != nil || len(events) == 0 {
		t.Fatalf("confirmed deletion event missing: events=%+v err=%v", events, err)
	}
	var detail map[string]any
	for _, event := range events {
		if event.Action == automationdomain.TaskEventActionDelete {
			detail = event.Detail
			break
		}
	}
	if detail == nil {
		t.Fatalf("confirmed deletion event missing from event history: %+v", events)
	}
	if detail["execution_stop_confirmed_by_owner"] != true ||
		detail["review_required_finalized"] != true || detail["external_actions_replayed"] != false {
		t.Fatalf("confirmed deletion audit omitted safety facts: %+v", detail)
	}
}

func TestConfirmStoppedRejectsTaskOutsideReviewRequired(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil, nil,
		permissionctx.NewContext(), &fakeWorkspaceReader{}, nil)
	task := createDeletionClaimTestTask(t, service)
	ownerCtx := contextForOwner(context.Background(), task.OwnerUserID)
	if _, err := service.ConfirmTaskDeletionStoppedAtVersion(ownerCtx, task.JobID, task.ConfigurationVersion); !errors.Is(err, automationdomain.ErrTaskDeletionReviewConflict) {
		t.Fatalf("confirmation outside review_required error=%v", err)
	}
	current, err := service.repository.GetScheduledTask(ownerCtx, task.OwnerUserID, task.JobID)
	if err != nil || current == nil || current.DeletionState != "" {
		t.Fatalf("ordinary task changed during rejected confirmation: task=%+v err=%v", current, err)
	}
}

func TestDeleteFinalizesPendingScriptThatNeverStartedEffects(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil, nil,
		permissionctx.NewContext(), &fakeWorkspaceReader{}, nil)
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name: "blocked script", AgentID: "agent-1", Instruction: "echo never-started",
		ExecutionKind: automationdomain.ExecutionKindScript,
		Schedule:      automationdomain.Schedule{Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: intRef(3600), Timezone: "UTC"},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated},
		Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone}, Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateTask(): %v", err)
	}
	if err = service.repository.InsertRunPending(context.Background(), automationstore.RunPendingInput{
		RunID: "blocked-script-run", JobID: task.JobID, OwnerUserID: task.OwnerUserID,
		Status: automationdomain.RunStatusPending,
	}); err != nil {
		t.Fatalf("InsertRunPending(): %v", err)
	}
	if _, err = db.Exec(`UPDATE automation_task_runs
SET block_state = 'awaiting_approval', blocked_request_id = 'permission-never-started'
WHERE run_id = 'blocked-script-run'`); err != nil {
		t.Fatalf("block pending script: %v", err)
	}
	if _, err = service.DeleteTaskAtVersion(context.Background(), task.JobID, task.ConfigurationVersion); err != nil {
		t.Fatalf("never-started script deletion should finalize: %v", err)
	}
	persisted, err := service.repository.GetScheduledTask(context.Background(), task.OwnerUserID, task.JobID)
	if err != nil || persisted != nil {
		t.Fatalf("never-started script task was retained: task=%+v err=%v", persisted, err)
	}
	run, err := service.repository.GetRun(context.Background(), task.OwnerUserID, task.JobID, "blocked-script-run")
	if err != nil || run == nil || run.Status != automationdomain.RunStatusCancelled || run.EffectStarted {
		t.Fatalf("never-started script run did not close safely: run=%+v err=%v", run, err)
	}
}

func TestDeleteKeepsReviewRequiredForRemoteConversationRuns(t *testing.T) {
	tests := []struct {
		name       string
		sessionKey string
	}{
		{name: "dm", sessionKey: protocol.BuildAgentSessionKey("agent-1", "automation", "dm", "remote-delete", "")},
		{name: "room", sessionKey: protocol.BuildRoomSharedSessionKey("remote-delete-room")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := newAutomationTestDB(t)
			permission := permissionctx.NewContext()
			service := NewService(config.Config{DatabaseDriver: "sqlite"}, db, nil,
				&fakeDMRunner{permission: permission}, &fakeRoomRunner{permission: permission},
				permission, &fakeWorkspaceReader{}, nil)
			task := createDeletionClaimTestTask(t, service)
			runID := "remote-" + test.name + "-run"
			if err := service.repository.InsertRunPending(context.Background(), automationstore.RunPendingInput{
				RunID: runID, JobID: task.JobID, OwnerUserID: task.OwnerUserID,
				Status: automationdomain.RunStatusRunning, SessionKey: test.sessionKey, RoundID: "round-" + test.name,
			}); err != nil {
				t.Fatalf("InsertRunPending(): %v", err)
			}
			if _, err := service.DeleteTaskAtVersion(context.Background(), task.JobID, task.ConfigurationVersion); !TaskDeletionPrepared(err) ||
				!errors.Is(err, ErrExecutionAttemptOwnershipUnconfirmed) {
				t.Fatalf("remote %s run was not held for review: %v", test.name, err)
			}
			persisted, err := service.repository.GetScheduledTask(context.Background(), task.OwnerUserID, task.JobID)
			if err != nil || persisted == nil || persisted.DeletionState != automationdomain.TaskDeletionStateReviewRequired {
				t.Fatalf("remote %s task finalized early: task=%+v err=%v", test.name, persisted, err)
			}
			if err = service.bootstrapRuntime(context.Background()); err != nil {
				t.Fatalf("bootstrapRuntime(): %v", err)
			}
			service.mu.Lock()
			_, recoveryLooped := service.deletionRecoveries[task.JobID]
			service.mu.Unlock()
			if recoveryLooped {
				t.Fatalf("remote %s review entered automatic recovery without exact ownership", test.name)
			}
		})
	}
}

func TestReviewRequiredDeletionFinalizesAfterRemoteScriptDurablySettles(t *testing.T) {
	db := newAutomationTestDB(t)
	workspacePath := newAutomationOwnerWorkspace(t, authctx.SystemUserID, "agent-1")
	newService := func() *Service {
		return NewService(
			config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspacePath, AppMode: "desktop"},
			db, nil, nil, nil, permissionctx.NewContext(), &fakeWorkspaceReader{}, nil,
		)
	}
	executionOwner := newService()
	deleteReceiver := newService()
	startedName := "remote-review-script-started"
	finishedName := "remote-review-script-finished"
	script := "touch " + startedName + "; sleep 0.5; touch " + finishedName
	if runtime.GOOS == "windows" {
		script = "echo started>" + startedName + " & timeout /T 1 /NOBREAK >NUL & echo finished>" + finishedName
	}
	task, err := executionOwner.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name: "remote settling script", AgentID: "agent-1", Instruction: script,
		ExecutionKind: automationdomain.ExecutionKindScript,
		Schedule:      automationdomain.Schedule{Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: intRef(3600), Timezone: "UTC"},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated},
		Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone}, Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateTask(): %v", err)
	}
	result, err := executionOwner.RunTaskNow(context.Background(), task.JobID)
	if err != nil || result == nil || result.RunID == nil {
		t.Fatalf("RunTaskNow(): result=%+v err=%v", result, err)
	}
	waitFor(t, 2*time.Second, func() bool {
		_, statErr := os.Stat(filepath.Join(workspacePath, startedName))
		return statErr == nil
	})
	if _, err = deleteReceiver.DeleteTaskAtVersion(context.Background(), task.JobID, task.ConfigurationVersion); !TaskDeletionPrepared(err) ||
		!errors.Is(err, ErrExecutionAttemptOwnershipUnconfirmed) {
		t.Fatalf("non-owner delete must wait for durable terminal proof: %v", err)
	}
	waitFor(t, 3*time.Second, func() bool {
		run, loadErr := executionOwner.repository.GetRun(context.Background(), task.OwnerUserID, task.JobID, *result.RunID)
		return loadErr == nil && run != nil && run.Status == automationdomain.RunStatusSucceeded &&
			run.DeliveryStatus == automationdomain.DeliveryStatusNotAttempted && run.DeliveryDeadLetterAt != nil
	})
	if _, statErr := os.Stat(filepath.Join(workspacePath, finishedName)); statErr != nil {
		t.Fatalf("pre-existing remote script did not naturally settle: %v", statErr)
	}
	if err = deleteReceiver.bootstrapRuntime(context.Background()); err != nil {
		t.Fatalf("bootstrapRuntime(): %v", err)
	}
	waitFor(t, 2*time.Second, func() bool {
		persisted, loadErr := deleteReceiver.repository.GetScheduledTask(context.Background(), task.OwnerUserID, task.JobID)
		return loadErr == nil && persisted == nil
	})
}

func createDeletionClaimTestTask(t *testing.T, service *Service) *automationdomain.ScheduledTask {
	t.Helper()
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name: "durable delete", AgentID: "agent-1", Instruction: "delete safely",
		Schedule:      automationdomain.Schedule{Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: intRef(3600), Timezone: "UTC"},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetMain},
		Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone}, Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateTask(): %v", err)
	}
	return task
}

func containsAll(value string, needles ...string) bool {
	for _, needle := range needles {
		if !strings.Contains(value, needle) {
			return false
		}
	}
	return true
}
