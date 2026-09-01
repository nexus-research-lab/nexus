// INPUT: exact owner/job/run/config/permission claim 与可选 client request identity。
// OUTPUT: claim+run 原子提交、冲突回滚和并发幂等重放证明。
// POS: Automation 首条 run crash-window 的 storage 回归。
package automation

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

func TestClaimScheduledTaskRunRollsBackRuntimeWhenInsertFails(t *testing.T) {
	db, repository, task := newAtomicRuntimeRepository(t, automationdomain.OverlapPolicyAllow)
	if err := repository.InsertRunPending(context.Background(), RunPendingInput{
		RunID: "run-conflict", JobID: task.JobID, OwnerUserID: task.OwnerUserID,
		Status: automationdomain.RunStatusSucceeded,
	}); err != nil {
		t.Fatal(err)
	}
	input := atomicRuntimeClaimInput(task, "run-conflict", "", "")
	if _, err := repository.ClaimScheduledTaskRun(context.Background(), input); err == nil {
		t.Fatal("duplicate run insert unexpectedly succeeded")
	}
	assertAtomicRuntimeState(t, db, task.JobID, "", 1)
}

func TestClaimScheduledTaskRunConcurrentRequestReplaysSingleRun(t *testing.T) {
	db, repository, task := newAtomicRuntimeRepository(t, automationdomain.OverlapPolicyAllow)
	inputs := []InitialRunClaimInput{
		atomicRuntimeClaimInput(task, "run-request-a", "manual-request-shared", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		atomicRuntimeClaimInput(task, "run-request-b", "manual-request-shared", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
	}
	results := make([]InitialRunClaimResult, len(inputs))
	errs := make([]error, len(inputs))
	var wait sync.WaitGroup
	for index := range inputs {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			results[index], errs[index] = repository.ClaimScheduledTaskRun(context.Background(), inputs[index])
		}(index)
	}
	wait.Wait()
	for index, err := range errs {
		if err != nil {
			t.Fatalf("claim %d: %v", index, err)
		}
	}
	if results[0].RunID == "" || results[0].RunID != results[1].RunID {
		t.Fatalf("concurrent request results = %+v", results)
	}
	claimed := 0
	replayed := 0
	for _, result := range results {
		if result.Claimed {
			claimed++
		}
		if result.Replayed {
			replayed++
		}
	}
	if claimed != 1 || replayed != 1 {
		t.Fatalf("claimed=%d replayed=%d results=%+v", claimed, replayed, results)
	}
	assertAtomicRuntimeState(t, db, task.JobID, results[0].RunID, 1)
}

func TestClaimScheduledTaskRunConflictingIntentRollsBackSecondTaskUpdate(t *testing.T) {
	db, repository, task := newAtomicRuntimeRepository(t, automationdomain.OverlapPolicyAllow)
	first := atomicRuntimeClaimInput(task, "run-intent-a", "manual-request-conflict", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	result, err := repository.ClaimScheduledTaskRun(context.Background(), first)
	if err != nil || !result.Claimed {
		t.Fatalf("first claim = %+v err=%v", result, err)
	}
	second := atomicRuntimeClaimInput(task, "run-intent-b", "manual-request-conflict", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	if _, err = repository.ClaimScheduledTaskRun(context.Background(), second); !errors.Is(err, automationdomain.ErrRuntimeCommandConflict) {
		t.Fatalf("conflicting claim error = %v", err)
	}
	assertAtomicRuntimeState(t, db, task.JobID, first.Run.RunID, 1)
}

func TestClaimScheduledTaskRunConcurrentOverlapTerminalReplayAndConflict(t *testing.T) {
	db, repository, task := newAtomicRuntimeRepository(t, automationdomain.OverlapPolicySkip)
	if _, err := db.Exec(`UPDATE automation_scheduled_tasks
SET running_run_id = 'run-active-other', running_started_at = CURRENT_TIMESTAMP
WHERE job_id = ?`, task.JobID); err != nil {
		t.Fatal(err)
	}
	build := func(runID string, digest string) InitialRunClaimInput {
		claim := atomicRuntimeClaimInput(task, runID, "manual-skipped-shared", digest)
		claim.Runtime.AllowDisabled = true
		claim.Run.TriggerKind = automationdomain.TriggerKindManual
		finishedAt := time.Date(2026, 8, 27, 9, 1, 0, 0, time.UTC)
		message := "previous run is still running; overlap_policy=skip"
		terminal := claim.Run
		terminal.Status = automationdomain.RunStatusSkipped
		terminal.StartedAt = nil
		terminal.FinishedAt = &finishedAt
		terminal.Attempts = 0
		terminal.ErrorMessage = &message
		terminal.DeliveryStatus = automationdomain.DeliveryStatusNotAttempted
		claim.OverlapTerminalRun = &terminal
		return claim
	}
	inputs := []InitialRunClaimInput{
		build("run-skipped-a", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		build("run-skipped-b", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
	}
	results := make([]InitialRunClaimResult, len(inputs))
	errs := make([]error, len(inputs))
	var wait sync.WaitGroup
	for index := range inputs {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			results[index], errs[index] = repository.ClaimScheduledTaskRun(context.Background(), inputs[index])
		}(index)
	}
	wait.Wait()
	for index, err := range errs {
		if err != nil {
			t.Fatalf("terminal request %d: %v", index, err)
		}
	}
	if results[0].RunID == "" || results[0].RunID != results[1].RunID {
		t.Fatalf("terminal request results=%+v", results)
	}
	terminal := 0
	replayed := 0
	for _, result := range results {
		if result.Terminal {
			terminal++
		}
		if result.Replayed {
			replayed++
		}
	}
	if terminal != 1 || replayed != 1 {
		t.Fatalf("terminal=%d replayed=%d results=%+v", terminal, replayed, results)
	}
	runs, err := repository.ListRunsByJob(context.Background(), task.OwnerUserID, task.JobID)
	if err != nil || len(runs) != 1 || runs[0].Status != automationdomain.RunStatusSkipped ||
		runs[0].FinishedAt == nil || runs[0].DeliveryStatus != automationdomain.DeliveryStatusNotAttempted {
		t.Fatalf("terminal runs=%+v err=%v", runs, err)
	}
	conflicting := build("run-skipped-conflict", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	if _, err = repository.ClaimScheduledTaskRun(context.Background(), conflicting); !errors.Is(err, automationdomain.ErrRuntimeCommandConflict) {
		t.Fatalf("terminal conflicting intent error=%v", err)
	}
	runs, err = repository.ListRunsByJob(context.Background(), task.OwnerUserID, task.JobID)
	if err != nil || len(runs) != 1 {
		t.Fatalf("terminal conflict created another run=%+v err=%v", runs, err)
	}
}

func newAtomicRuntimeRepository(
	t *testing.T,
	overlapPolicy string,
) (*sql.DB, *Repository, automationdomain.ScheduledTask) {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "automation-runtime.db"))
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
) VALUES ('atomic-agent', 'atomic-agent', 'Atomic Agent', '', '', 'active', '/tmp/atomic-agent')`); err != nil {
		t.Fatal(err)
	}
	repository := NewRepository(config.Config{DatabaseDriver: "sqlite"}, db)
	intervalSeconds := 3600
	task, err := repository.UpsertScheduledTask(context.Background(), automationdomain.ScheduledTask{
		JobID: "atomic-job", OwnerUserID: "atomic-owner", Name: "Atomic run", AgentID: "atomic-agent",
		Schedule:    automationdomain.Schedule{Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: &intervalSeconds, Timezone: "UTC"},
		Instruction: "run atomically", ExecutionKind: automationdomain.ExecutionKindAgent, PermissionMode: automationdomain.PermissionModeDefault,
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated, WakeMode: automationdomain.WakeModeNextHeartbeat},
		Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone}, Source: automationdomain.Source{Kind: automationdomain.SourceKindSystem},
		OverlapPolicy: overlapPolicy, SessionBindingState: automationdomain.TaskSessionBindingStateReady,
		PermissionPolicy: automationdomain.TaskPermissionPolicy{Version: 1, Revision: 1},
		PermissionState:  automationdomain.TaskPermissionStateReady,
		Enabled:          true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return db, repository, *task
}

func atomicRuntimeClaimInput(
	task automationdomain.ScheduledTask,
	runID string,
	requestID string,
	intentDigest string,
) InitialRunClaimInput {
	startedAt := time.Date(2026, 8, 27, 9, 0, 0, 0, time.UTC)
	permissionState := strings.TrimSpace(task.PermissionState)
	permissionRequestID := strings.TrimSpace(task.PendingPermissionRequestID)
	configurationVersion := task.ConfigurationVersion
	permissionRevision := task.PermissionPolicy.Revision
	return InitialRunClaimInput{
		Runtime: JobRuntimeClaimInput{
			OwnerUserID: task.OwnerUserID, JobID: task.JobID, RunID: runID,
			StartedAt: startedAt, OverlapPolicy: task.OverlapPolicy,
			ExpectedConfigurationVersion: &configurationVersion,
			ExpectedPermissionRevision:   &permissionRevision,
			ExpectedPermissionState:      &permissionState,
			ExpectedPermissionRequestID:  &permissionRequestID,
		},
		Run: RunPendingInput{
			RunID: runID, JobID: task.JobID, OwnerUserID: task.OwnerUserID,
			Status: automationdomain.RunStatusRunning, StartedAt: &startedAt, Attempts: 1,
			PermissionPolicyRevision: permissionRevision,
			ClientRequestID:          requestID, IntentDigest: intentDigest,
		},
	}
}

func assertAtomicRuntimeState(t *testing.T, db *sql.DB, jobID string, runID string, runCount int) {
	t.Helper()
	var running sql.NullString
	if err := db.QueryRow(`SELECT running_run_id FROM automation_scheduled_tasks WHERE job_id = ?`, jobID).Scan(&running); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(runID) == "" {
		if running.Valid && strings.TrimSpace(running.String) != "" {
			t.Fatalf("ghost running_run_id = %+v", running)
		}
	} else if !running.Valid || strings.TrimSpace(running.String) != runID {
		t.Fatalf("running_run_id = %+v, want %s", running, runID)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM automation_task_runs WHERE job_id = ?`, jobID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != runCount {
		t.Fatalf("run count = %d, want %d", count, runCount)
	}
}
