// INPUT: review_required 删除快照、私有 token 与 configuration_version fence。
// OUTPUT: stale state/version 时整笔事务不生效，exact fence 时一次性收口。
// POS: 删除 finalization 的仓储级原子性回归。
package automation

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

func TestFinalizeTaskDeletionReviewFenceIsAtomic(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "automation-delete.db")
	db, err := sql.Open("sqlite", databasePath)
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
) VALUES ('delete-agent', 'delete-agent', 'Delete Agent', '', '', 'active', '/tmp/delete-agent')`); err != nil {
		t.Fatal(err)
	}

	repository := NewRepository(config.Config{DatabaseDriver: "sqlite"}, db)
	intervalSeconds := 3600
	task, err := repository.UpsertScheduledTask(context.Background(), automationdomain.ScheduledTask{
		JobID:       "delete-job",
		OwnerUserID: "delete-owner",
		Name:        "Deletion fence",
		AgentID:     "delete-agent",
		Schedule: automationdomain.Schedule{
			Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: &intervalSeconds, Timezone: "UTC",
		},
		Instruction:    "verify deletion fence",
		ExecutionKind:  automationdomain.ExecutionKindAgent,
		PermissionMode: automationdomain.PermissionModeDefault,
		SessionTarget: automationdomain.SessionTarget{
			Kind: automationdomain.SessionTargetIsolated, WakeMode: automationdomain.WakeModeNextHeartbeat,
		},
		Delivery:            automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Source:              automationdomain.Source{Kind: automationdomain.SourceKindSystem},
		OverlapPolicy:       automationdomain.OverlapPolicySkip,
		SessionBindingState: automationdomain.TaskSessionBindingStateReady,
		PermissionState:     automationdomain.TaskPermissionStateReady,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = repository.InsertRunPending(context.Background(), RunPendingInput{
		RunID: "delete-run", JobID: task.JobID, OwnerUserID: task.OwnerUserID,
		Status: automationdomain.RunStatusRunning, DeliveryStatus: automationdomain.DeliveryStatusPending,
	}); err != nil {
		t.Fatal(err)
	}
	claim, err := repository.ClaimScheduledTaskDeletion(context.Background(), TaskDeletionClaimInput{
		OwnerUserID: task.OwnerUserID, JobID: task.JobID,
		DeletionToken: "private-delete-token", ClaimedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = repository.MarkTaskDeletionReviewRequired(
		context.Background(), task.OwnerUserID, task.JobID, claim.Token,
	); err != nil {
		t.Fatal(err)
	}
	cleanupRuns, err := repository.ListReviewRequiredTaskDeletionCleanupRuns(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(cleanupRuns) != 1 || cleanupRuns[0].RunID != "delete-run" ||
		cleanupRuns[0].OwnerUserID != task.OwnerUserID || cleanupRuns[0].JobID != task.JobID {
		t.Fatalf("batched review cleanup runs = %+v", cleanupRuns)
	}

	staleVersion := claim.Task.ConfigurationVersion - 1
	input := TaskDeleteFinalizationInput{
		OwnerUserID: task.OwnerUserID, JobID: task.JobID, DeletionToken: claim.Token,
		ExpectedDeletionState:        automationdomain.TaskDeletionStateReviewRequired,
		ExpectedConfigurationVersion: &staleVersion,
		FinishedAt:                   time.Now().UTC(),
		ActiveRunMessage:             "confirmed stopped",
		DeliveryDeadLetter:           time.Now().UTC(),
		DeliveryError:                "deleted",
		UnconfirmedDeliveryError:     "unconfirmed",
		PendingDeliveryError:         "not attempted",
		DeleteEvent: TaskEventInput{
			EventID: "delete-event", JobID: task.JobID, OwnerUserID: task.OwnerUserID,
			AgentID: task.AgentID, Action: automationdomain.TaskEventActionDelete,
		},
	}
	if _, err = repository.FinalizeScheduledTaskDeletion(context.Background(), input); !errors.Is(err, automationdomain.ErrConfigurationVersionConflict) {
		t.Fatalf("stale finalize error=%v", err)
	}
	assertDeletionFenceUnchanged(t, db, task.JobID, "delete-run")

	currentVersion := claim.Task.ConfigurationVersion
	input.ExpectedConfigurationVersion = &currentVersion
	input.ExpectedDeletionState = automationdomain.TaskDeletionStateDeleting
	if _, err = repository.FinalizeScheduledTaskDeletion(context.Background(), input); !errors.Is(err, automationdomain.ErrTaskDeletionReviewConflict) {
		t.Fatalf("wrong-state finalize error=%v", err)
	}
	assertDeletionFenceUnchanged(t, db, task.JobID, "delete-run")

	input.ExpectedDeletionState = automationdomain.TaskDeletionStateReviewRequired
	if _, err = repository.FinalizeScheduledTaskDeletion(context.Background(), input); err != nil {
		t.Fatalf("exact finalize error=%v", err)
	}
	var taskCount int
	if err = db.QueryRow(`SELECT COUNT(1) FROM automation_scheduled_tasks WHERE job_id = ?`, task.JobID).Scan(&taskCount); err != nil || taskCount != 0 {
		t.Fatalf("task count=%d err=%v", taskCount, err)
	}
	var status, deliveryStatus string
	var deadLetterAt sql.NullTime
	if err = db.QueryRow(`SELECT status, delivery_status, delivery_dead_letter_at FROM automation_task_runs WHERE run_id = 'delete-run'`).Scan(
		&status, &deliveryStatus, &deadLetterAt,
	); err != nil || status != automationdomain.RunStatusCancelled ||
		deliveryStatus != automationdomain.DeliveryStatusNotAttempted || !deadLetterAt.Valid {
		t.Fatalf("closed run status=%q delivery=%q deadletter=%v err=%v", status, deliveryStatus, deadLetterAt, err)
	}
}

func assertDeletionFenceUnchanged(t *testing.T, db *sql.DB, jobID string, runID string) {
	t.Helper()
	var taskCount, eventCount int
	if err := db.QueryRow(`SELECT COUNT(1) FROM automation_scheduled_tasks WHERE job_id = ?`, jobID).Scan(&taskCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(1) FROM automation_task_events WHERE job_id = ?`, jobID).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	var status, deliveryStatus string
	var finishedAt, deadLetterAt sql.NullTime
	if err := db.QueryRow(`SELECT status, delivery_status, finished_at, delivery_dead_letter_at
FROM automation_task_runs WHERE run_id = ?`, runID).Scan(&status, &deliveryStatus, &finishedAt, &deadLetterAt); err != nil {
		t.Fatal(err)
	}
	if taskCount != 1 || eventCount != 0 || status != automationdomain.RunStatusRunning ||
		deliveryStatus != automationdomain.DeliveryStatusPending || finishedAt.Valid || deadLetterAt.Valid {
		t.Fatalf("failed fence partially mutated state: task=%d events=%d status=%q delivery=%q finished=%v deadletter=%v",
			taskCount, eventCount, status, deliveryStatus, finishedAt, deadLetterAt)
	}
}
