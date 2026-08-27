// INPUT: latest completed run identity and exact delivery attempt completions.
// OUTPUT: historical completion updates its run only; latest completion also updates the task summary.
// POS: storage regression for migration 00121 and exact last_delivery_status projection.
package automation

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

func TestCompleteRunDeliveryAttemptOnlyProjectsLatestCompletedRun(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "automation-delivery.db"))
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
) VALUES ('delivery-agent', 'delivery-agent', 'Delivery Agent', '', '', 'active', '/tmp/delivery-agent')`); err != nil {
		t.Fatal(err)
	}
	repository := NewRepository(config.Config{DatabaseDriver: "sqlite"}, db)
	intervalSeconds := 3600
	task, err := repository.UpsertScheduledTask(context.Background(), automationdomain.ScheduledTask{
		JobID: "delivery-job", OwnerUserID: "delivery-owner", Name: "Delivery projection", AgentID: "delivery-agent",
		Schedule:    automationdomain.Schedule{Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: &intervalSeconds, Timezone: "UTC"},
		Instruction: "deliver", ExecutionKind: automationdomain.ExecutionKindAgent, PermissionMode: automationdomain.PermissionModeDefault,
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated, WakeMode: automationdomain.WakeModeNextHeartbeat},
		Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone}, Source: automationdomain.Source{Kind: automationdomain.SourceKindSystem},
		OverlapPolicy: automationdomain.OverlapPolicySkip, SessionBindingState: automationdomain.TaskSessionBindingStateReady,
		PermissionState: automationdomain.TaskPermissionStateReady,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, runID := range []string{"run-history", "run-latest"} {
		if err = repository.InsertRunPending(context.Background(), RunPendingInput{
			RunID: runID, JobID: task.JobID, OwnerUserID: task.OwnerUserID,
			Status: automationdomain.RunStatusSucceeded, DeliveryStatus: automationdomain.DeliveryStatusRetrying,
		}); err != nil {
			t.Fatal(err)
		}
		if _, err = db.Exec(`UPDATE automation_task_runs
SET delivery_attempts = 1, delivery_attempt_id = ? WHERE run_id = ?`, "attempt-"+runID, runID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.Exec(`UPDATE automation_scheduled_tasks
SET last_completed_run_id = 'run-latest', last_delivery_status = 'pending' WHERE job_id = ?`, task.JobID); err != nil {
		t.Fatal(err)
	}
	complete := func(runID string) {
		t.Helper()
		if completeErr := repository.CompleteRunDeliveryAttempt(context.Background(), RunDeliveryAttemptCompletionInput{
			OwnerUserID: task.OwnerUserID, JobID: task.JobID, RunID: runID,
			AttemptID: "attempt-" + runID, DeliveryStatus: automationdomain.DeliveryStatusSucceeded,
		}); completeErr != nil {
			t.Fatalf("complete %s: %v", runID, completeErr)
		}
	}
	complete("run-history")
	var taskDelivery string
	if err = db.QueryRow(`SELECT last_delivery_status FROM automation_scheduled_tasks WHERE job_id = ?`, task.JobID).Scan(&taskDelivery); err != nil {
		t.Fatal(err)
	}
	if taskDelivery != automationdomain.DeliveryStatusPending {
		t.Fatalf("historical run overwrote task delivery summary: %s", taskDelivery)
	}
	complete("run-latest")
	if err = db.QueryRow(`SELECT last_delivery_status FROM automation_scheduled_tasks WHERE job_id = ?`, task.JobID).Scan(&taskDelivery); err != nil {
		t.Fatal(err)
	}
	if taskDelivery != automationdomain.DeliveryStatusSucceeded {
		t.Fatalf("latest run did not update task delivery summary: %s", taskDelivery)
	}
}
