package automation

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"

	_ "modernc.org/sqlite"
)

type reentrantTaskUpdateDMRunner struct {
	service *Service
	jobID   string
	version int64
	done    chan error
}

func (r *reentrantTaskUpdateDMRunner) HandleChat(context.Context, dmsvc.Request) error {
	name := "unrelated task updated during dispatch"
	_, err := r.service.UpdateTaskAtVersion(
		context.Background(), r.jobID, r.version, automationdomain.UpdateJobInput{Name: &name},
	)
	r.done <- err
	return errors.New("stop test dispatch after reentrant task update")
}

func TestManualRunDispatchDoesNotHoldGlobalTaskControlLock(t *testing.T) {
	db := newAutomationTestDB(t)
	runner := &reentrantTaskUpdateDMRunner{done: make(chan error, 1)}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"}, db, nil, runner, nil,
		permissionctx.NewContext(), &fakeWorkspaceReader{}, nil,
	)
	runner.service = service
	jobIDs := []string{"job_manual_run_dispatch", "job_unrelated_configuration"}
	jobIndex := 0
	service.idFactory = func(kind string) string {
		if kind == "job" && jobIndex < len(jobIDs) {
			id := jobIDs[jobIndex]
			jobIndex++
			return id
		}
		return automationexec.NewID(kind)
	}
	createTask := func(name string) *automationdomain.ScheduledTask {
		t.Helper()
		task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
			Name: name, AgentID: "agent-1", Instruction: "dispatch without global lock",
			Schedule: automationdomain.Schedule{
				Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: intRef(3600), Timezone: "UTC",
			},
			SessionTarget: automationdomain.SessionTarget{
				Kind:            automationdomain.SessionTargetNamed,
				NamedSessionKey: protocol.BuildAgentSessionKey("agent-1", "ws", "dm", name, ""),
			},
			Delivery: automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
			Enabled:  true,
		})
		if err != nil {
			t.Fatalf("CreateTask(%s): %v", name, err)
		}
		return task
	}
	runTask := createTask("manual-run-dispatch")
	unrelated := createTask("unrelated-configuration")
	runner.jobID = unrelated.JobID
	runner.version = unrelated.ConfigurationVersion

	runResult := make(chan error, 1)
	go func() {
		_, err := service.RunTaskNowAtVersionWithRequest(
			context.Background(), runTask.JobID, runTask.ConfigurationVersion,
			"web-run:reentrant-control-lock",
		)
		runResult <- err
	}()
	select {
	case err := <-runner.done:
		if err != nil {
			t.Fatalf("reentrant unrelated update: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("manual runtime dispatch retained the global task control lock")
	}
	select {
	case <-runResult:
	case <-time.After(time.Second):
		t.Fatal("manual runtime dispatch did not settle")
	}
}

func TestRepositoryClaimScheduledTaskRuntimePreventsDuplicateExternalClaims(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		nil,
		nil,
		permissionctx.NewContext(),
		&fakeWorkspaceReader{},
		nil,
	)
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "领取防重",
		AgentID:     "agent-1",
		Instruction: "只应领取一次",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind: automationdomain.SessionTargetIsolated,
		},
		Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		OverlapPolicy: automationdomain.OverlapPolicySkip,
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	startedAt := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
	nextRunAt := startedAt.Add(time.Hour)
	claimed, err := service.repository.ClaimScheduledTaskRuntime(context.Background(), automationstore.JobRuntimeClaimInput{
		JobID:         task.JobID,
		RunID:         "run-1",
		StartedAt:     startedAt,
		NextRunAt:     &nextRunAt,
		OverlapPolicy: automationdomain.OverlapPolicySkip,
	})
	if err != nil {
		t.Fatalf("第一次领取失败: %v", err)
	}
	if !claimed {
		t.Fatalf("第一次领取应成功")
	}
	claimed, err = service.repository.ClaimScheduledTaskRuntime(context.Background(), automationstore.JobRuntimeClaimInput{
		JobID:         task.JobID,
		RunID:         "run-2",
		StartedAt:     startedAt.Add(time.Second),
		NextRunAt:     &nextRunAt,
		OverlapPolicy: automationdomain.OverlapPolicySkip,
	})
	if err != nil {
		t.Fatalf("第二次领取失败: %v", err)
	}
	if claimed {
		t.Fatalf("overlap=skip 下 running_run_id 未清理时不应允许第二次领取")
	}

	var runningRunID sql.NullString
	if err = db.QueryRow(`SELECT running_run_id FROM automation_scheduled_tasks WHERE job_id = ?`, task.JobID).Scan(&runningRunID); err != nil {
		t.Fatalf("读取 running_run_id 失败: %v", err)
	}
	if !runningRunID.Valid || runningRunID.String != "run-1" {
		t.Fatalf("running_run_id = %+v, 期望 run-1", runningRunID)
	}

	result, err := service.startJobExecution(context.Background(), *task, automationdomain.TriggerKindScheduled, startedAt.Add(2*time.Second))
	if err != nil {
		t.Fatalf("外部领取后本进程触发应返回当前运行态而不是报错: %v", err)
	}
	if result == nil || result.Status != automationdomain.RunStatusRunning || result.RunID == nil || *result.RunID != "run-1" {
		t.Fatalf("外部领取后的触发结果 = %+v, 期望 running/run-1", result)
	}
	if err = db.QueryRow(`SELECT running_run_id FROM automation_scheduled_tasks WHERE job_id = ?`, task.JobID).Scan(&runningRunID); err != nil {
		t.Fatalf("再次读取 running_run_id 失败: %v", err)
	}
	if !runningRunID.Valid || runningRunID.String != "run-1" {
		t.Fatalf("外部领取标记被错误清理: %+v", runningRunID)
	}
	var runCount int
	if err = db.QueryRow(`SELECT COUNT(*) FROM automation_task_runs WHERE job_id = ?`, task.JobID).Scan(&runCount); err != nil {
		t.Fatalf("读取 run 数量失败: %v", err)
	}
	if runCount != 0 {
		t.Fatalf("外部调度器已领取时，本进程不应写入 skipped run，实际 %d", runCount)
	}
}

func TestRepositoryClaimScheduledTaskRuntimeRejectsStaleConfigurationVersion(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		nil,
		nil,
		permissionctx.NewContext(),
		&fakeWorkspaceReader{},
		nil,
	)
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "版本领取",
		AgentID:     "agent-1",
		Instruction: "只运行已确认版本",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated},
		Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		OverlapPolicy: automationdomain.OverlapPolicySkip,
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	staleVersion := task.ConfigurationVersion
	if _, err = db.Exec(
		`UPDATE automation_scheduled_tasks
SET configuration_version = configuration_version + 1
WHERE job_id = ?`,
		task.JobID,
	); err != nil {
		t.Fatalf("advance configuration version: %v", err)
	}

	claimed, err := service.repository.ClaimScheduledTaskRuntime(
		context.Background(),
		automationstore.JobRuntimeClaimInput{
			JobID:                        task.JobID,
			RunID:                        "run-stale-version",
			StartedAt:                    time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC),
			OverlapPolicy:                automationdomain.OverlapPolicySkip,
			ExpectedConfigurationVersion: &staleVersion,
		},
	)
	if claimed || !errors.Is(err, automationdomain.ErrConfigurationVersionConflict) {
		t.Fatalf("stale claim = (%v, %v), want false/version conflict", claimed, err)
	}
	var runningRunID sql.NullString
	if err = db.QueryRow(
		`SELECT running_run_id FROM automation_scheduled_tasks WHERE job_id = ?`,
		task.JobID,
	).Scan(&runningRunID); err != nil {
		t.Fatalf("read runtime after stale claim: %v", err)
	}
	if runningRunID.Valid && runningRunID.String != "" {
		t.Fatalf("stale claim changed running_run_id: %+v", runningRunID)
	}
}

func TestRepositoryRuntimeClaimCannotClearNewerPermissionState(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		nil,
		nil,
		permissionctx.NewContext(),
		&fakeWorkspaceReader{},
		nil,
	)
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "权限领取",
		AgentID:     "agent-1",
		Instruction: "不得清除更新的权限请求",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated},
		Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		OverlapPolicy: automationdomain.OverlapPolicySkip,
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if _, err = db.Exec(
		`UPDATE automation_scheduled_tasks
SET permission_state = ?, pending_permission_request_id = ?
WHERE job_id = ?`,
		automationdomain.TaskPermissionStateAwaitingApproval,
		"permission-new",
		task.JobID,
	); err != nil {
		t.Fatalf("install newer permission state: %v", err)
	}

	expectedState := automationdomain.TaskPermissionStateDenied
	expectedRequestID := ""
	expectedRevision := task.PermissionPolicy.Revision
	expectedVersion := task.ConfigurationVersion
	claimed, err := service.repository.ClaimScheduledTaskRuntime(
		context.Background(),
		automationstore.JobRuntimeClaimInput{
			JobID:                        task.JobID,
			RunID:                        "run-permission-stale",
			StartedAt:                    time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC),
			OverlapPolicy:                automationdomain.OverlapPolicySkip,
			AllowDisabled:                true,
			ExpectedConfigurationVersion: &expectedVersion,
			ExpectedPermissionRevision:   &expectedRevision,
			ExpectedPermissionState:      &expectedState,
			ExpectedPermissionRequestID:  &expectedRequestID,
			ResetDeniedPermission:        true,
		},
	)
	if claimed || !errors.Is(err, automationdomain.ErrPermissionRequestStale) {
		t.Fatalf("stale permission claim = (%v, %v), want false/stale", claimed, err)
	}
	var permissionState string
	var pendingRequestID sql.NullString
	var runningRunID sql.NullString
	if err = db.QueryRow(
		`SELECT permission_state, pending_permission_request_id, running_run_id
FROM automation_scheduled_tasks WHERE job_id = ?`,
		task.JobID,
	).Scan(&permissionState, &pendingRequestID, &runningRunID); err != nil {
		t.Fatalf("read task after stale permission claim: %v", err)
	}
	if permissionState != automationdomain.TaskPermissionStateAwaitingApproval ||
		!pendingRequestID.Valid || pendingRequestID.String != "permission-new" ||
		(runningRunID.Valid && runningRunID.String != "") {
		t.Fatalf("stale claim changed permission/runtime: state=%q request=%+v run=%+v", permissionState, pendingRequestID, runningRunID)
	}
}

func TestScriptJobExternalClaimDoesNotRecordSkippedRun(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		nil,
		nil,
		permissionctx.NewContext(),
		&fakeWorkspaceReader{},
		nil,
	)
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:          "脚本领取防重",
		AgentID:       "agent-1",
		Instruction:   "echo should-not-run",
		ExecutionKind: automationdomain.ExecutionKindScript,
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated},
		Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		OverlapPolicy: automationdomain.OverlapPolicySkip,
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	startedAt := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
	nextRunAt := startedAt.Add(time.Hour)
	claimed, err := service.repository.ClaimScheduledTaskRuntime(context.Background(), automationstore.JobRuntimeClaimInput{
		JobID:         task.JobID,
		RunID:         "run-script-1",
		StartedAt:     startedAt,
		NextRunAt:     &nextRunAt,
		OverlapPolicy: automationdomain.OverlapPolicySkip,
	})
	if err != nil {
		t.Fatalf("脚本任务外部领取失败: %v", err)
	}
	if !claimed {
		t.Fatal("脚本任务外部领取应成功")
	}
	result, err := service.startJobExecution(context.Background(), *task, automationdomain.TriggerKindScheduled, startedAt.Add(2*time.Second))
	if err != nil {
		t.Fatalf("脚本任务外部领取后触发失败: %v", err)
	}
	if result == nil || result.Status != automationdomain.RunStatusRunning || result.RunID == nil || *result.RunID != "run-script-1" {
		t.Fatalf("脚本任务外部领取后的触发结果 = %+v, 期望 running/run-script-1", result)
	}
	var runCount int
	if err = db.QueryRow(`SELECT COUNT(*) FROM automation_task_runs WHERE job_id = ?`, task.JobID).Scan(&runCount); err != nil {
		t.Fatalf("读取脚本 run 数量失败: %v", err)
	}
	if runCount != 0 {
		t.Fatalf("脚本任务被其他调度器领取时，本进程不应写入 skipped run，实际 %d", runCount)
	}
}

func TestExecutionPathsRollBackRuntimeWhenInitialRunInsertFails(t *testing.T) {
	for _, testCase := range []struct {
		name          string
		executionKind string
		targetKind    string
	}{
		{name: "runtime", executionKind: automationdomain.ExecutionKindAgent, targetKind: automationdomain.SessionTargetIsolated},
		{name: "main queued", executionKind: automationdomain.ExecutionKindAgent, targetKind: automationdomain.SessionTargetMain},
		{name: "script", executionKind: automationdomain.ExecutionKindScript, targetKind: automationdomain.SessionTargetIsolated},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			db := newAutomationTestDB(t)
			service := NewService(
				config.Config{DatabaseDriver: "sqlite"},
				db,
				nil,
				nil,
				nil,
				permissionctx.NewContext(),
				&fakeWorkspaceReader{},
				nil,
			)
			task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
				Name: "atomic " + testCase.name, AgentID: "agent-1", Instruction: "must not dispatch",
				Schedule:      automationdomain.Schedule{Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: intRef(3600), Timezone: "UTC"},
				SessionTarget: automationdomain.SessionTarget{Kind: testCase.targetKind},
				Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
				Enabled:       true,
			})
			if err != nil {
				t.Fatal(err)
			}
			if testCase.executionKind == automationdomain.ExecutionKindScript {
				if _, err = db.Exec(`UPDATE automation_scheduled_tasks SET execution_kind = ? WHERE job_id = ?`, testCase.executionKind, task.JobID); err != nil {
					t.Fatal(err)
				}
				task.ExecutionKind = testCase.executionKind
			}
			if err = service.repository.InsertRunPending(context.Background(), automationstore.RunPendingInput{
				RunID: "run-insert-conflict", JobID: task.JobID, OwnerUserID: task.OwnerUserID,
				Status: automationdomain.RunStatusSucceeded,
			}); err != nil {
				t.Fatal(err)
			}
			service.idFactory = func(prefix string) string {
				if prefix == "run" {
					return "run-insert-conflict"
				}
				return prefix + "-atomic-test"
			}
			if _, err = service.startJobExecution(
				context.Background(), *task, automationdomain.TriggerKindScheduled, time.Now().UTC(),
			); err == nil {
				t.Fatal("initial run insert conflict unexpectedly dispatched")
			}
			var runningRunID sql.NullString
			if err = db.QueryRow(`SELECT running_run_id FROM automation_scheduled_tasks WHERE job_id = ?`, task.JobID).Scan(&runningRunID); err != nil {
				t.Fatal(err)
			}
			if runningRunID.Valid && runningRunID.String != "" {
				t.Fatalf("ghost runtime occupancy = %+v", runningRunID)
			}
			var runCount int
			if err = db.QueryRow(`SELECT COUNT(*) FROM automation_task_runs WHERE job_id = ?`, task.JobID).Scan(&runCount); err != nil || runCount != 1 {
				t.Fatalf("run count=%d err=%v", runCount, err)
			}
			var eventCount int
			if err = db.QueryRow(`SELECT COUNT(*) FROM automation_system_events`).Scan(&eventCount); err != nil {
				t.Fatal(err)
			}
			if eventCount != 0 {
				t.Fatalf("dispatch occurred before commit: system events=%d", eventCount)
			}
			service.mu.Lock()
			state := service.jobStates[task.JobID]
			running := state != nil && state.Running
			service.mu.Unlock()
			if running {
				t.Fatal("in-memory runtime registered before durable commit")
			}
		})
	}
}

func TestManualRunRequestUsesDatabaseOverlapInsteadOfStaleProcessProjection(t *testing.T) {
	t.Run("database active while local cache is idle", func(t *testing.T) {
		db := newAutomationTestDB(t)
		serviceA := NewService(
			config.Config{DatabaseDriver: "sqlite"}, db, nil, nil, nil,
			permissionctx.NewContext(), &fakeWorkspaceReader{}, nil,
		)
		serviceB := NewService(
			config.Config{DatabaseDriver: "sqlite"}, db, nil, nil, nil,
			permissionctx.NewContext(), &fakeWorkspaceReader{}, nil,
		)
		task, err := serviceA.CreateTask(context.Background(), automationdomain.CreateJobInput{
			Name: "cross instance active", AgentID: "agent-1", Instruction: "run once",
			Schedule:      automationdomain.Schedule{Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: intRef(3600), Timezone: "UTC"},
			SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetMain},
			Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
			OverlapPolicy: automationdomain.OverlapPolicySkip, Enabled: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		serviceB.ensureJobState(*task)
		if _, err = serviceA.startJobExecution(
			context.Background(), *task, automationdomain.TriggerKindScheduled, time.Now().UTC(),
		); err != nil {
			t.Fatal(err)
		}
		result, err := serviceB.RunTaskNowAtVersionWithRequest(
			context.Background(), task.JobID, task.ConfigurationVersion, "web-run:cross-instance-active",
		)
		if err != nil || result.Status != automationdomain.RunStatusSkipped || result.RunID == nil {
			t.Fatalf("manual overlap result=%+v err=%v", result, err)
		}
		run, err := serviceB.repository.GetRun(context.Background(), task.OwnerUserID, task.JobID, *result.RunID)
		if err != nil || run.ClientRequestID != "web-run:cross-instance-active" || run.FinishedAt == nil ||
			run.DeliveryStatus != automationdomain.DeliveryStatusNotAttempted {
			t.Fatalf("manual overlap ledger=%+v err=%v", run, err)
		}
	})

	t.Run("local cache active while database is idle", func(t *testing.T) {
		db := newAutomationTestDB(t)
		service := NewService(
			config.Config{DatabaseDriver: "sqlite"}, db, nil, nil, nil,
			permissionctx.NewContext(), &fakeWorkspaceReader{}, nil,
		)
		task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
			Name: "stale local active", AgentID: "agent-1", Instruction: "must execute",
			Schedule:      automationdomain.Schedule{Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: intRef(3600), Timezone: "UTC"},
			SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetMain},
			Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
			OverlapPolicy: automationdomain.OverlapPolicySkip, Enabled: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		state := service.ensureJobState(*task)
		service.mu.Lock()
		state.Running = true
		state.RunningCount = 1
		state.RunningRunID = "stale-local-run"
		service.mu.Unlock()
		result, err := service.RunTaskNowAtVersionWithRequest(
			context.Background(), task.JobID, task.ConfigurationVersion, "web-run:stale-local-active",
		)
		if err != nil || result.Status != automationdomain.RunStatusQueuedToMain || result.RunID == nil {
			t.Fatalf("manual stale-cache result=%+v err=%v", result, err)
		}
		run, err := service.repository.GetRun(context.Background(), task.OwnerUserID, task.JobID, *result.RunID)
		if err != nil || run.Status != automationdomain.RunStatusQueuedToMain || run.ClientRequestID != "web-run:stale-local-active" {
			t.Fatalf("manual stale-cache ledger=%+v err=%v", run, err)
		}
		service.mu.Lock()
		runningCount := state.RunningCount
		runningRunID := state.RunningRunID
		service.mu.Unlock()
		if runningCount != 1 || runningRunID != *result.RunID {
			t.Fatalf("reconciled runtime count=%d run_id=%q, want 1/%q", runningCount, runningRunID, *result.RunID)
		}
	})
}

func TestManualOverlapRequestUsesDatabaseAcrossRuntimeAndScriptPaths(t *testing.T) {
	for _, testCase := range []struct {
		name          string
		executionKind string
	}{
		{name: "agent runtime", executionKind: automationdomain.ExecutionKindAgent},
		{name: "script", executionKind: automationdomain.ExecutionKindScript},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			db := newAutomationTestDB(t)
			service := NewService(
				config.Config{DatabaseDriver: "sqlite"}, db, nil, nil, nil,
				permissionctx.NewContext(), &fakeWorkspaceReader{}, nil,
			)
			task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
				Name: "database overlap " + testCase.name, AgentID: "agent-1", Instruction: "do not dispatch",
				Schedule:      automationdomain.Schedule{Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: intRef(3600), Timezone: "UTC"},
				SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated},
				Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
				OverlapPolicy: automationdomain.OverlapPolicySkip, Enabled: true,
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = db.Exec(`UPDATE automation_scheduled_tasks
SET execution_kind = ?, running_run_id = 'remote-active-run', running_started_at = CURRENT_TIMESTAMP
WHERE job_id = ?`, testCase.executionKind, task.JobID); err != nil {
				t.Fatal(err)
			}
			const requestID = "web-run:cross-path-overlap"
			first, err := service.RunTaskNowAtVersionWithRequest(
				context.Background(), task.JobID, task.ConfigurationVersion, requestID,
			)
			if err != nil || first == nil || first.RunID == nil || first.Status != automationdomain.RunStatusSkipped {
				t.Fatalf("first overlap receipt=%+v err=%v", first, err)
			}
			second, err := service.RunTaskNowAtVersionWithRequest(
				context.Background(), task.JobID, task.ConfigurationVersion, requestID,
			)
			if err != nil || second == nil || second.RunID == nil || *second.RunID != *first.RunID || !second.Replayed {
				t.Fatalf("replayed overlap receipt=%+v err=%v", second, err)
			}
			var runCount int
			if err = db.QueryRow(`SELECT COUNT(*) FROM automation_task_runs
WHERE job_id = ? AND client_request_id = ?`, task.JobID, requestID).Scan(&runCount); err != nil || runCount != 1 {
				t.Fatalf("request run count=%d err=%v", runCount, err)
			}
		})
	}
}
