// INPUT: blocked permission runs with stale pending delivery and retry metadata.
// OUTPUT: deny or policy revision closes delivery as not_attempted and projects only the exact terminal summary.
// POS: storage regression for permission terminal/delivery transaction boundaries.
package automation

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

func TestPermissionTerminalTransitionsClosePendingDeliveryAtomically(t *testing.T) {
	db, repository := newPermissionTerminalRepository(t)

	t.Run("deny", func(t *testing.T) {
		task := insertPermissionTerminalTask(t, repository, "permission-deny-job")
		runID := "run-permission-deny"
		requestID := "request-permission-deny"
		insertBlockedPermissionRun(t, db, repository, task, runID, requestID)

		resolvedAt := time.Date(2026, 8, 27, 9, 0, 0, 0, time.UTC)
		if _, err := repository.ResolvePermissionRequest(context.Background(), PermissionRequestDecisionStoreInput{
			OwnerUserID: task.OwnerUserID, RequestID: requestID,
			Decision: automationdomain.PermissionDecisionDeny, ResolvedByUserID: task.OwnerUserID,
			ResolvedAt: resolvedAt, ExpectedRevision: task.PermissionPolicy.Revision,
			TaskState: automationdomain.TaskPermissionStateDenied, FinishRunAsDenied: true,
			DeniedMessage: "permission denied",
		}); err != nil {
			t.Fatal(err)
		}

		assertPermissionTerminalDeliveryClosed(t, db, task.JobID, runID, automationdomain.RunStatusFailed)
	})

	t.Run("policy revision", func(t *testing.T) {
		task := insertPermissionTerminalTask(t, repository, "permission-revision-job")
		runID := "run-permission-revision"
		requestID := "request-permission-revision"
		insertBlockedPermissionRun(t, db, repository, task, runID, requestID)

		next := task
		next.Instruction = "revised instruction"
		next.ConfigurationVersion++
		next.PermissionPolicy.Revision++
		next.PermissionState = automationdomain.TaskPermissionStateReady
		next.PendingPermissionRequestID = ""
		updated, invalidated, err := repository.UpdateTaskAndInvalidatePermissionBoundary(
			context.Background(),
			TaskPermissionBoundaryUpdateInput{
				Job: next, ExpectedVersion: &task.ConfigurationVersion,
				CancellationMessage: "permission policy changed",
			},
		)
		if err != nil {
			t.Fatal(err)
		}
		if len(invalidated) != 1 || invalidated[0].RequestID != requestID {
			t.Fatalf("invalidated requests = %+v", invalidated)
		}
		if updated.LastRunStatus != automationdomain.RunStatusCancelled ||
			updated.LastDeliveryStatus != automationdomain.DeliveryStatusNotAttempted {
			t.Fatalf("updated task summary = %+v", updated)
		}
		assertPermissionTerminalDeliveryClosed(t, db, task.JobID, runID, automationdomain.RunStatusCancelled)
	})

	t.Run("late denial does not replace newer completion", func(t *testing.T) {
		task := insertPermissionTerminalTask(t, repository, "permission-late-deny-job")
		blockedRunID := "run-permission-old-blocked"
		requestID := "request-permission-old-blocked"
		insertBlockedPermissionRun(t, db, repository, task, blockedRunID, requestID)
		if err := repository.InsertRunPending(context.Background(), RunPendingInput{
			RunID: "run-permission-newer", JobID: task.JobID, OwnerUserID: task.OwnerUserID,
			Status: automationdomain.RunStatusSucceeded, DeliveryStatus: automationdomain.DeliveryStatusSucceeded,
		}); err != nil {
			t.Fatal(err)
		}
		newerFinishedAt := time.Date(2026, 8, 27, 11, 0, 0, 0, time.UTC)
		if _, err := db.Exec(`UPDATE automation_task_runs
SET finished_at = ?, delivery_status = 'succeeded'
WHERE run_id = 'run-permission-newer'`, newerFinishedAt); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`UPDATE automation_scheduled_tasks
SET last_run_at = ?, last_run_status = 'succeeded', last_delivery_status = 'succeeded',
    last_completed_run_id = 'run-permission-newer'
WHERE job_id = ?`, newerFinishedAt, task.JobID); err != nil {
			t.Fatal(err)
		}
		if _, err := repository.ResolvePermissionRequest(context.Background(), PermissionRequestDecisionStoreInput{
			OwnerUserID: task.OwnerUserID, RequestID: requestID,
			Decision: automationdomain.PermissionDecisionDeny, ResolvedByUserID: task.OwnerUserID,
			ResolvedAt:       time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC),
			ExpectedRevision: task.PermissionPolicy.Revision,
			TaskState:        automationdomain.TaskPermissionStateDenied, FinishRunAsDenied: true,
			DeniedMessage: "permission denied",
		}); err != nil {
			t.Fatal(err)
		}
		var status, delivery, completed string
		if err := db.QueryRow(`SELECT last_run_status, last_delivery_status, last_completed_run_id
FROM automation_scheduled_tasks WHERE job_id = ?`, task.JobID).Scan(&status, &delivery, &completed); err != nil {
			t.Fatal(err)
		}
		if status != automationdomain.RunStatusSucceeded ||
			delivery != automationdomain.DeliveryStatusSucceeded || completed != "run-permission-newer" {
			t.Fatalf("late denial replaced newer task summary: status=%s delivery=%s completed=%s", status, delivery, completed)
		}
		var oldDelivery string
		if err := db.QueryRow(`SELECT delivery_status FROM automation_task_runs WHERE run_id = ?`, blockedRunID).Scan(&oldDelivery); err != nil {
			t.Fatal(err)
		}
		if oldDelivery != automationdomain.DeliveryStatusNotAttempted {
			t.Fatalf("old denied run delivery = %s", oldDelivery)
		}
	})
}

func newPermissionTerminalRepository(t *testing.T) (*sql.DB, *Repository) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "permission-terminal.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatal(err)
	}
	if err = goose.Up(db, "../../../db/migrations/sqlite"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO agents (
id, slug, name, description, definition, status, workspace_path
) VALUES ('permission-agent', 'permission-agent', 'Permission Agent', '', '', 'active', '/tmp/permission-agent')`); err != nil {
		t.Fatal(err)
	}
	return db, NewRepository(config.Config{DatabaseDriver: "sqlite"}, db)
}

func insertPermissionTerminalTask(
	t *testing.T,
	repository *Repository,
	jobID string,
) automationdomain.ScheduledTask {
	t.Helper()
	intervalSeconds := 3600
	task, err := repository.UpsertScheduledTask(context.Background(), automationdomain.ScheduledTask{
		JobID: jobID, OwnerUserID: "permission-owner", Name: jobID, AgentID: "permission-agent",
		Schedule: automationdomain.Schedule{
			Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: &intervalSeconds, Timezone: "UTC",
		},
		Instruction: "permission terminal", ExecutionKind: automationdomain.ExecutionKindAgent,
		PermissionMode: automationdomain.PermissionModeDefault,
		SessionTarget: automationdomain.SessionTarget{
			Kind: automationdomain.SessionTargetIsolated, WakeMode: automationdomain.WakeModeNextHeartbeat,
		},
		Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeExplicit, Channel: "test", To: "recipient"},
		Source:        automationdomain.Source{Kind: automationdomain.SourceKindSystem},
		OverlapPolicy: automationdomain.OverlapPolicySkip, SessionBindingState: automationdomain.TaskSessionBindingStateReady,
		PermissionPolicy: automationdomain.TaskPermissionPolicy{Version: 1, Revision: 1},
		PermissionState:  automationdomain.TaskPermissionStateReady, Enabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return *task
}

func insertBlockedPermissionRun(
	t *testing.T,
	db *sql.DB,
	repository *Repository,
	task automationdomain.ScheduledTask,
	runID string,
	requestID string,
) {
	t.Helper()
	if err := repository.InsertRunPending(context.Background(), RunPendingInput{
		RunID: runID, JobID: task.JobID, OwnerUserID: task.OwnerUserID,
		Status: automationdomain.RunStatusPending, DeliveryStatus: automationdomain.DeliveryStatusPending,
		PermissionPolicyRevision: task.PermissionPolicy.Revision,
	}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := repository.CreatePermissionRequestAndBlockRun(
		context.Background(),
		PermissionRequestCreateInput{
			Request: automationdomain.AutomationPermissionRequest{
				RequestID: requestID, OwnerUserID: task.OwnerUserID, JobID: task.JobID, RunID: runID,
				PolicyRevision: task.PermissionPolicy.Revision, Kind: automationdomain.PermissionRequestKindTool,
				Capability: automationdomain.PermissionCapability{
					ToolName: "Write", Effect: automationdomain.PermissionEffectWrite,
					InputFingerprint: "sha256:" + runID,
				},
			},
			TaskState:  automationdomain.TaskPermissionStateAwaitingApproval,
			BlockState: automationdomain.RunBlockStateAwaitingApproval,
		},
	); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE automation_task_runs
SET delivery_status = 'pending', delivery_error = 'stale', delivered_at = CURRENT_TIMESTAMP,
    delivery_attempts = 2, delivery_attempt_id = 'stale-attempt',
    delivery_attempt_started_at = CURRENT_TIMESTAMP,
    delivery_next_attempt_at = CURRENT_TIMESTAMP,
    delivery_dead_letter_at = CURRENT_TIMESTAMP
WHERE run_id = ?`, runID); err != nil {
		t.Fatal(err)
	}
}

func assertPermissionTerminalDeliveryClosed(
	t *testing.T,
	db *sql.DB,
	jobID string,
	runID string,
	wantRunStatus string,
) {
	t.Helper()
	var (
		runStatus, deliveryStatus                         string
		deliveryError, attemptID                          sql.NullString
		deliveredAt, attemptStartedAt, nextAt, deadLetter sql.NullTime
		attempts                                          int
	)
	if err := db.QueryRow(`SELECT status, delivery_status, delivery_error, delivered_at,
delivery_attempts, delivery_attempt_id, delivery_attempt_started_at,
delivery_next_attempt_at, delivery_dead_letter_at
FROM automation_task_runs WHERE run_id = ?`, runID).Scan(
		&runStatus, &deliveryStatus, &deliveryError, &deliveredAt,
		&attempts, &attemptID, &attemptStartedAt, &nextAt, &deadLetter,
	); err != nil {
		t.Fatal(err)
	}
	if runStatus != wantRunStatus || deliveryStatus != automationdomain.DeliveryStatusNotAttempted ||
		deliveryError.Valid || deliveredAt.Valid || attempts != 0 || attemptID.Valid ||
		attemptStartedAt.Valid || nextAt.Valid || deadLetter.Valid {
		t.Fatalf("run delivery was not closed: status=%s delivery=%s error=%+v delivered=%+v attempts=%d attempt=%+v started=%+v next=%+v deadletter=%+v",
			runStatus, deliveryStatus, deliveryError, deliveredAt, attempts, attemptID, attemptStartedAt, nextAt, deadLetter)
	}
	var lastStatus, lastDelivery, lastCompleted string
	if err := db.QueryRow(`SELECT last_run_status, last_delivery_status, last_completed_run_id
FROM automation_scheduled_tasks WHERE job_id = ?`, jobID).Scan(&lastStatus, &lastDelivery, &lastCompleted); err != nil {
		t.Fatal(err)
	}
	if lastStatus != wantRunStatus || lastDelivery != automationdomain.DeliveryStatusNotAttempted || lastCompleted != runID {
		t.Fatalf("task summary = status=%s delivery=%s completed=%s", lastStatus, lastDelivery, lastCompleted)
	}
}
