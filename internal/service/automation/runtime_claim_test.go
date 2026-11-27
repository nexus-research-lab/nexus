// runtime_claim_test.go 覆盖自动化任务认领、运行时命令、调度策略与删除认领测试。
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

	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/mcp/command"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
	_ "modernc.org/sqlite"
)

type reentrantTaskUpdateDMRunner struct {
	noMainWindow
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
		{name: "main window", executionKind: automationdomain.ExecutionKindAgent, targetKind: automationdomain.SessionTargetMain},
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
			config.Config{DatabaseDriver: "sqlite"}, db, nil, &fakeDMRunner{permission: permissionctx.NewContext(), delay: 500 * time.Millisecond}, nil,
			permissionctx.NewContext(), &fakeWorkspaceReader{}, nil,
		)
		serviceB := NewService(
			config.Config{DatabaseDriver: "sqlite"}, db, nil, &fakeDMRunner{permission: permissionctx.NewContext(), delay: 500 * time.Millisecond}, nil,
			permissionctx.NewContext(), &fakeWorkspaceReader{}, nil,
		)
		task, err := serviceA.CreateTask(context.Background(), automationdomain.CreateJobInput{
			Name: "cross instance active", AgentID: "agent-1", Instruction: "run once",
			Schedule:      automationdomain.Schedule{Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: intRef(3600), Timezone: "UTC"},
			SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated},
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
			config.Config{DatabaseDriver: "sqlite"}, db, nil, &fakeDMRunner{permission: permissionctx.NewContext(), delay: 500 * time.Millisecond}, nil,
			permissionctx.NewContext(), &fakeWorkspaceReader{}, nil,
		)
		task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
			Name: "stale local active", AgentID: "agent-1", Instruction: "must execute",
			Schedule:      automationdomain.Schedule{Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: intRef(3600), Timezone: "UTC"},
			SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated},
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
		if err != nil || result.Status != automationdomain.RunStatusRunning || result.RunID == nil {
			t.Fatalf("manual stale-cache result=%+v err=%v", result, err)
		}
		run, err := service.repository.GetRun(context.Background(), task.OwnerUserID, task.JobID, *result.RunID)
		if err != nil || run.Status != automationdomain.RunStatusRunning || run.ClientRequestID != "web-run:stale-local-active" {
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

type fixedAutomationDeliveryAuthority struct {
	sessions []channels.AgentExternalSession
}

func (f fixedAutomationDeliveryAuthority) ValidateExternalSessionGrant(
	context.Context,
	string,
	string,
	string,
) error {
	return nil
}

func (f fixedAutomationDeliveryAuthority) ListAgentExternalSessions(
	context.Context,
	string,
	string,
	string,
) ([]channels.AgentExternalSession, error) {
	return append([]channels.AgentExternalSession(nil), f.sessions...), nil
}

type fixedAutomationDeliverySessionResolver map[string]protocol.Session

func (f fixedAutomationDeliverySessionResolver) ResolveDeliverySession(
	_ context.Context,
	sessionKey string,
) (*protocol.Session, error) {
	item, ok := f[sessionKey]
	if !ok {
		return nil, nil
	}
	return &item, nil
}

func TestAutomationCommandCronScheduleAcceptsStandardFiveField(t *testing.T) {
	for _, expression := range []string{"0 9 15 * *", "*/15 9-17 * * 1-5"} {
		schedule, err := automationCommandCronSchedule(expression, "Asia/Shanghai")
		if err != nil {
			t.Fatalf("automationCommandCronSchedule(%q): %v", expression, err)
		}
		if schedule.CronExpression == nil || *schedule.CronExpression != expression {
			t.Fatalf("cron_expression = %#v, want %q", schedule.CronExpression, expression)
		}
	}
	if _, err := automationCommandCronSchedule("0 25 15 * *", "Asia/Shanghai"); err == nil {
		t.Fatal("invalid cron expression unexpectedly accepted")
	}
	fixture := newAutomationCommandFixture(t, "ok")
	input := automationConfigurationTaskInput("invalid cron")
	expression := "0 25 15 * *"
	input.Schedule = automationdomain.Schedule{
		Kind: automationdomain.ScheduleKindCron, CronExpression: &expression, Timezone: "Asia/Shanghai",
	}
	if _, err := fixture.Service.CreateTask(context.Background(), input); err == nil {
		t.Fatal("CreateTask unexpectedly persisted invalid cron expression")
	}
}

func TestRuntimeCommandCreateUsesPlanConfirmationAndIdempotency(t *testing.T) {
	fixture := newAutomationCommandFixture(t, "ok")
	actor := fixture.ServerContext
	input := automationdomain.AutomationCommandInput{
		Name: "CLI 日报", Instruction: "生成工作日报",
		Schedule: &automationdomain.AutomationCommandSchedule{
			Kind: "daily", DailyTime: "09:00", Timezone: "Asia/Shanghai",
		},
	}
	plan, err := fixture.Service.PlanRuntimeCommand(
		context.Background(), actor, automationdomain.AutomationCommandOperationCreate, input,
	)
	if err != nil {
		t.Fatalf("PlanRuntimeCommand(create): %v", err)
	}
	if !plan.RequiresConfirmation || plan.PlanDigest == "" || plan.CurrentRevision != "new:agent-1" {
		t.Fatalf("create plan = %+v", plan)
	}
	request := automationdomain.AutomationCommandRequest{
		Action:    automationdomain.AutomationCommandActionApply,
		Operation: automationdomain.AutomationCommandOperationCreate,
		Input:     input, RequestID: "runtime-create-stable",
		ExpectedRevision: plan.CurrentRevision, PlanDigest: plan.PlanDigest,
	}
	if _, err = fixture.Service.ApplyRuntimeCommand(
		context.Background(), actor, request, RuntimeCommandApplyOptions{},
	); err == nil || !strings.Contains(err.Error(), "真人确认") {
		t.Fatalf("unconfirmed create error = %v", err)
	}
	first, err := fixture.Service.ApplyRuntimeCommand(
		context.Background(), actor, request, RuntimeCommandApplyOptions{HumanConfirmed: true, HumanApprovalRequestID: "test-approval"},
	)
	if err != nil {
		t.Fatalf("ApplyRuntimeCommand(create): %v", err)
	}
	created, ok := first.Data.(*automationdomain.ScheduledTask)
	if !ok || created == nil || created.JobID == "" || created.Source.CreatorAgentID != actor.AgentID {
		t.Fatalf("created task = %#v", first.Data)
	}
	commandRecord, err := fixture.Service.repository.GetRuntimeCommand(
		context.Background(), actor.OwnerUserID, request.RequestID,
	)
	if err != nil || commandRecord.ApprovalRequestID != "test-approval" {
		t.Fatalf("command approval audit = %+v err=%v", commandRecord, err)
	}
	replayed, err := fixture.Service.ApplyRuntimeCommand(
		context.Background(), actor, request, RuntimeCommandApplyOptions{HumanConfirmed: true, HumanApprovalRequestID: "test-approval"},
	)
	if err != nil {
		t.Fatalf("replayed create: %v", err)
	}
	replayedTask, ok := replayed.Data.(*automationdomain.ScheduledTask)
	if !ok || replayedTask.JobID != created.JobID {
		t.Fatalf("create replay = %#v, want job %s", replayed.Data, created.JobID)
	}
	conflicting := request
	conflicting.Input.Name = "different intent"
	if _, err = fixture.Service.ApplyRuntimeCommand(
		context.Background(), actor, conflicting, RuntimeCommandApplyOptions{HumanConfirmed: true, HumanApprovalRequestID: "test-approval"},
	); !errors.Is(err, automationdomain.ErrRuntimeCommandConflict) {
		t.Fatalf("conflicting request_id error = %v", err)
	}
	listed, err := fixture.Service.InspectRuntimeCommand(
		context.Background(), actor, automationdomain.AutomationCommandOperationList,
		automationdomain.AutomationCommandInput{Query: "CLI 日报"},
	)
	if err != nil {
		t.Fatalf("InspectRuntimeCommand(list): %v", err)
	}
	items, ok := listed.([]automationdomain.ScheduledTask)
	if !ok || len(items) != 1 || items[0].JobID != created.JobID {
		t.Fatalf("listed tasks = %#v", listed)
	}
}

func TestRuntimeCommandSelectsSameAgentPairedDMSession(t *testing.T) {
	fixture := newAutomationCommandFixture(t, "ok")
	actor := fixture.ServerContext
	sessionKey := protocol.BuildAgentAccountSessionKey(
		actor.AgentID,
		protocol.SessionChannelWeixinPersonal,
		protocol.RoomTypeDM,
		"weixin-account",
		"weixin-user",
		"",
	)
	fixture.Service.SetDeliveryGrantResolver(fixedAutomationDeliveryAuthority{sessions: []channels.AgentExternalSession{{
		SessionKey: sessionKey,
		Channel:    protocol.SessionChannelWeixinPersonal,
		Label:      "捷哥",
		AgentID:    actor.AgentID,
	}}})
	fixture.Service.SetDeliverySessionResolver(fixedAutomationDeliverySessionResolver{
		sessionKey: {
			SessionKey:  sessionKey,
			AgentID:     actor.AgentID,
			ChannelType: protocol.SessionChannelWeixinPersonal,
			ChatType:    protocol.RoomTypeDM,
			Title:       "捷哥",
		},
	})

	listed, err := fixture.Service.InspectRuntimeCommand(
		context.Background(),
		actor,
		automationdomain.AutomationCommandOperationDeliveryTargets,
		automationdomain.AutomationCommandInput{DeliveryChannel: protocol.SessionChannelWeixinPersonal},
	)
	if err != nil {
		t.Fatal(err)
	}
	targets, ok := listed.([]automationdomain.AutomationCommandDeliverySession)
	if !ok || len(targets) != 1 || targets[0].SessionKey != sessionKey || targets[0].Label != "捷哥" {
		t.Fatalf("delivery_targets = %#v", listed)
	}

	input := automationdomain.AutomationCommandInput{
		Name: "微信提醒", Instruction: "提醒捷哥跟进客户",
		Schedule: &automationdomain.AutomationCommandSchedule{
			Kind: "daily", DailyTime: "09:00", Timezone: "Asia/Shanghai",
		},
		DeliverySessionKey: sessionKey,
	}
	plan, err := fixture.Service.PlanRuntimeCommand(
		context.Background(), actor, automationdomain.AutomationCommandOperationCreate, input,
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err := fixture.Service.ApplyRuntimeCommand(
		context.Background(),
		actor,
		automationdomain.AutomationCommandRequest{
			Action: automationdomain.AutomationCommandActionApply, Operation: automationdomain.AutomationCommandOperationCreate,
			Input: input, RequestID: "runtime-weixin-delivery",
			ExpectedRevision: plan.CurrentRevision, PlanDigest: plan.PlanDigest,
		},
		RuntimeCommandApplyOptions{HumanConfirmed: true, HumanApprovalRequestID: "test-approval"},
	)
	if err != nil {
		t.Fatal(err)
	}
	created := result.Data.(*automationdomain.ScheduledTask)
	if created.Delivery.Mode != automationdomain.DeliveryModeLast ||
		created.Delivery.SessionKey != sessionKey ||
		created.DeliveryGrant.SessionKey != actor.SessionKey {
		t.Fatalf("created delivery = %+v grant=%+v", created.Delivery, created.DeliveryGrant)
	}
	otherAgentInput := input
	otherAgentInput.DeliverySessionKey = protocol.BuildAgentSessionKey(
		"agent-2",
		protocol.SessionChannelWeixinPersonal,
		protocol.RoomTypeDM,
		"other-user",
		"",
	)
	if _, err = fixture.Service.PlanRuntimeCommand(
		context.Background(), actor, automationdomain.AutomationCommandOperationCreate, otherAgentInput,
	); err == nil {
		t.Fatal("ordinary Agent unexpectedly selected another Agent delivery Session")
	}
}

func TestRuntimeCommandScopesCrossAgentAndBackgroundRun(t *testing.T) {
	fixture := newAutomationCommandFixture(t, "ok")
	actor := fixture.ServerContext
	if _, err := runtimeCommandAgentID(actor, "agent-2"); err == nil {
		t.Fatal("ordinary Agent unexpectedly received cross-Agent authority")
	}
	main := actor
	main.IsMainAgent = true
	if target, err := runtimeCommandAgentID(main, "agent-2"); err != nil || target != "agent-2" {
		t.Fatalf("main cross-Agent target = %q err=%v", target, err)
	}

	created := createRuntimeCommandTask(t, fixture, actor, "background task", "runtime-background-create")
	background := actor
	background.SourceContextType = "automation_run"
	background.CurrentJobID = created.JobID
	background.CurrentRunID = "run-current"
	if background.MutationAllowed() {
		t.Fatal("background scheduled run unexpectedly received mutation authority")
	}
	if _, err := fixture.Service.PlanRuntimeCommand(
		context.Background(), background, automationdomain.AutomationCommandOperationDelete,
		automationdomain.AutomationCommandInput{JobID: created.JobID},
	); err == nil {
		t.Fatal("background scheduled run planned a mutation")
	}
	if _, err := fixture.Service.InspectRuntimeCommand(
		context.Background(), background, automationdomain.AutomationCommandOperationGet,
		automationdomain.AutomationCommandInput{},
	); err != nil {
		t.Fatalf("background scheduled run could not inspect bound task: %v", err)
	}
	if _, err := fixture.Service.InspectRuntimeCommand(
		context.Background(), background, automationdomain.AutomationCommandOperationGet,
		automationdomain.AutomationCommandInput{JobID: "other-job"},
	); err == nil {
		t.Fatal("background scheduled run inspected another task")
	}
}

func TestRuntimeCommandApplyRejectsStaleTaskRevision(t *testing.T) {
	fixture := newAutomationCommandFixture(t, "ok")
	actor := fixture.ServerContext
	created := createRuntimeCommandTask(t, fixture, actor, "versioned task", "runtime-version-create")
	input := automationdomain.AutomationCommandInput{JobID: created.JobID, Enabled: boolCommandPointer(false)}
	plan, err := fixture.Service.PlanRuntimeCommand(
		context.Background(), actor, automationdomain.AutomationCommandOperationUpdate, input,
	)
	if err != nil {
		t.Fatalf("plan update: %v", err)
	}
	name := "concurrent update"
	if _, err = fixture.Service.UpdateTaskAtVersion(
		runtimeAutomationCommandContext(context.Background(), actor),
		created.JobID,
		created.ConfigurationVersion,
		automationdomain.UpdateJobInput{Name: &name},
	); err != nil {
		t.Fatalf("concurrent update: %v", err)
	}
	_, err = fixture.Service.ApplyRuntimeCommand(
		context.Background(), actor,
		automationdomain.AutomationCommandRequest{
			Action:    automationdomain.AutomationCommandActionApply,
			Operation: automationdomain.AutomationCommandOperationUpdate,
			Input:     input, RequestID: "runtime-update-stale",
			ExpectedRevision: plan.CurrentRevision, PlanDigest: plan.PlanDigest,
		},
		RuntimeCommandApplyOptions{HumanConfirmed: true, HumanApprovalRequestID: "test-approval"},
	)
	if err == nil || !strings.Contains(err.Error(), "状态已变化") {
		t.Fatalf("stale update error = %v", err)
	}
}

func TestRuntimeCommandReplayDoesNotDuplicateRun(t *testing.T) {
	fixture := newAutomationCommandFixture(t, "ok")
	actor := fixture.ServerContext
	created := createRuntimeCommandTask(t, fixture, actor, "replay task", "runtime-replay-create")
	runInput := automationdomain.AutomationCommandInput{JobID: created.JobID}
	runPlan, err := fixture.Service.PlanRuntimeCommand(
		context.Background(), actor, automationdomain.AutomationCommandOperationRun, runInput,
	)
	if err != nil {
		t.Fatal(err)
	}
	runRequest := automationdomain.AutomationCommandRequest{
		Action:    automationdomain.AutomationCommandActionApply,
		Operation: automationdomain.AutomationCommandOperationRun,
		Input:     runInput, RequestID: "runtime-run-replay",
		ExpectedRevision: runPlan.CurrentRevision, PlanDigest: runPlan.PlanDigest,
	}
	first, err := fixture.Service.ApplyRuntimeCommand(
		context.Background(), actor, runRequest, RuntimeCommandApplyOptions{HumanConfirmed: true, HumanApprovalRequestID: "test-approval"},
	)
	if err != nil {
		t.Fatal(err)
	}
	second, err := fixture.Service.ApplyRuntimeCommand(
		context.Background(), actor, runRequest, RuntimeCommandApplyOptions{HumanConfirmed: true, HumanApprovalRequestID: "test-approval"},
	)
	if err != nil || second.Outcome != "replayed" {
		t.Fatalf("run replay = %+v err=%v", second, err)
	}
	firstRun := first.Data.(*automationdomain.ExecutionResult)
	secondRun := second.Data.(*automationdomain.ExecutionResult)
	if firstRun.RunID == nil || secondRun.RunID == nil || *firstRun.RunID != *secondRun.RunID {
		t.Fatalf("run replay IDs: first=%+v second=%+v", firstRun, secondRun)
	}

}

func TestRuntimeCommandReplayRecoversAcceptedRunAfterReceiptLoss(t *testing.T) {
	fixture := newAutomationCommandFixture(t, "ok")
	actor := fixture.ServerContext
	created := createRuntimeCommandTask(t, fixture, actor, "accepted run", "runtime-accepted-create")
	request := automationdomain.AutomationCommandRequest{
		Action: automationdomain.AutomationCommandActionApply, Operation: automationdomain.AutomationCommandOperationRun,
		Input: automationdomain.AutomationCommandInput{JobID: created.JobID}, RequestID: "runtime-run-receipt-loss",
	}
	intentDigest, err := runtimeAutomationIntentDigest(actor, request.Operation, request.Input)
	if err != nil {
		t.Fatal(err)
	}
	if _, isNew, claimErr := fixture.Service.repository.ClaimRuntimeCommand(
		context.Background(),
		automationstore.RuntimeCommandRecord{
			OwnerUserID: actor.OwnerUserID, RequestID: request.RequestID,
			ActorAgentID: actor.AgentID, Operation: request.Operation, IntentDigest: intentDigest,
			ApprovalRequestID: "approval-receipt-loss",
		},
	); claimErr != nil || !isNew {
		t.Fatalf("claim runtime command isNew=%v err=%v", isNew, claimErr)
	}
	if err = fixture.Service.repository.InsertRunPending(
		context.Background(),
		automationstore.RunPendingInput{
			RunID: "run-receipt-loss", JobID: created.JobID, OwnerUserID: created.OwnerUserID,
			Status:          automationdomain.RunStatusRunning,
			ClientRequestID: request.RequestID, IntentDigest: intentDigest,
			PermissionPolicyRevision: created.PermissionPolicy.Revision,
		},
	); err != nil {
		t.Fatal(err)
	}
	replayed, found, err := fixture.Service.ReplayRuntimeCommand(context.Background(), actor, request)
	if err != nil || !found || replayed == nil || replayed.Outcome != "replayed" {
		t.Fatalf("receipt-loss replay=%+v found=%v err=%v", replayed, found, err)
	}
	run, ok := replayed.Data.(*automationdomain.ExecutionResult)
	if !ok || run.RunID == nil || *run.RunID != "run-receipt-loss" {
		t.Fatalf("replayed run=%#v", replayed.Data)
	}
	record, err := fixture.Service.repository.GetRuntimeCommand(context.Background(), actor.OwnerUserID, request.RequestID)
	if err != nil || record.Status != "applied" {
		t.Fatalf("reconciled command=%+v err=%v", record, err)
	}
}

func createRuntimeCommandTask(
	t *testing.T,
	fixture automationCommandFixture,
	actor command.Actor,
	name string,
	requestID string,
) *automationdomain.ScheduledTask {
	t.Helper()
	input := automationdomain.AutomationCommandInput{
		Name: name, Instruction: "do work",
		Schedule: &automationdomain.AutomationCommandSchedule{
			Kind: "interval", IntervalValue: 1, IntervalUnit: "hours",
		},
	}
	plan, err := fixture.Service.PlanRuntimeCommand(
		context.Background(), actor, automationdomain.AutomationCommandOperationCreate, input,
	)
	if err != nil {
		t.Fatal(err)
	}
	result, err := fixture.Service.ApplyRuntimeCommand(
		context.Background(), actor,
		automationdomain.AutomationCommandRequest{
			Action:    automationdomain.AutomationCommandActionApply,
			Operation: automationdomain.AutomationCommandOperationCreate,
			Input:     input, RequestID: requestID,
			ExpectedRevision: plan.CurrentRevision, PlanDigest: plan.PlanDigest,
		},
		RuntimeCommandApplyOptions{HumanConfirmed: true, HumanApprovalRequestID: "test-approval"},
	)
	if err != nil {
		t.Fatal(err)
	}
	task, ok := result.Data.(*automationdomain.ScheduledTask)
	if !ok || task == nil {
		t.Fatalf("created data = %#v", result.Data)
	}
	return task
}

func boolCommandPointer(value bool) *bool { return &value }

func TestNextAutomationDeadlineChoosesEarliestRunnableBoundary(t *testing.T) {
	db := newAutomationTestDB(t)
	now := time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC)
	service := NewService(
		config.Config{DatabaseDriver: "sqlite", AutomationRunTimeoutSeconds: 60},
		db, nil, nil, nil, permissionctx.NewContext(), &fakeWorkspaceReader{}, nil,
	)
	leaseAt := now.Add(20 * time.Second)
	taskAt := now.Add(30 * time.Second)
	expiresAt := now.Add(40 * time.Second)
	startedAt := now.Add(-55 * time.Second)
	service.schedulerLeaseRenewAt = leaseAt
	service.jobStates["job-deadline"] = &automationexec.JobRuntimeState{
		Job:     automationdomain.ScheduledTask{Enabled: true, ExpiresAt: &expiresAt},
		Running: true, RunningStartedAt: &startedAt, NextRunAt: &taskAt,
	}

	timeoutAt := now.Add(5 * time.Second)
	if deadline := service.nextAutomationDeadline(); deadline == nil || !deadline.Equal(timeoutAt) {
		t.Fatalf("next deadline = %v, want stale-run timeout %s", deadline, timeoutAt)
	}
}

func TestSchedulerLeaseAllowsSingleLeaderAndExpiryTakeover(t *testing.T) {
	db := newAutomationTestDB(t)
	newService := func(ownerID string) *Service {
		service := NewService(
			config.Config{DatabaseDriver: "sqlite", AutomationSchedulerLeaseSeconds: 3},
			db, nil, nil, nil, permissionctx.NewContext(), &fakeWorkspaceReader{}, nil,
		)
		service.schedulerOwnerID = ownerID
		return service
	}

	first := newService("scheduler-a")
	second := newService("scheduler-b")
	now := time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC)
	held, becameLeader, err := first.refreshSchedulerLease(context.Background(), now)
	if err != nil || !held || !becameLeader {
		t.Fatalf("第一个实例应获取租约: held=%v became=%v err=%v", held, becameLeader, err)
	}
	held, becameLeader, err = second.refreshSchedulerLease(context.Background(), now.Add(time.Second))
	if err != nil || held || becameLeader {
		t.Fatalf("租约有效期内第二个实例不应接管: held=%v became=%v err=%v", held, becameLeader, err)
	}
	held, becameLeader, err = second.refreshSchedulerLease(context.Background(), now.Add(4*time.Second))
	if err != nil || !held || !becameLeader {
		t.Fatalf("租约过期后第二个实例应接管: held=%v became=%v err=%v", held, becameLeader, err)
	}
	held, becameLeader, err = first.refreshSchedulerLease(context.Background(), now.Add(4*time.Second))
	if err != nil || held || becameLeader {
		t.Fatalf("原 leader 不应覆盖新租约: held=%v became=%v err=%v", held, becameLeader, err)
	}
}

func TestCreateAndEnableTaskRespectUserCapacity(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(
		config.Config{
			DatabaseDriver:                   "sqlite",
			AutomationMaxEnabledTasksPerUser: 1,
		},
		db,
		nil,
		nil,
		nil,
		permissionctx.NewContext(),
		&fakeWorkspaceReader{},
		nil,
	)
	first := schedulerPolicyTaskInput("第一个任务", true)
	if _, err := service.CreateTask(context.Background(), first); err != nil {
		t.Fatalf("创建第一个任务失败: %v", err)
	}
	if _, err := service.CreateTask(context.Background(), schedulerPolicyTaskInput("超限任务", true)); err == nil || !strings.Contains(err.Error(), "不能超过 1 个") {
		t.Fatalf("创建超限启用任务应失败: %v", err)
	}
	disabled, err := service.CreateTask(context.Background(), schedulerPolicyTaskInput("暂停任务", false))
	if err != nil {
		t.Fatalf("容量满时仍应允许创建暂停任务: %v", err)
	}
	if _, err = service.UpdateTaskStatus(context.Background(), disabled.JobID, true); err == nil || !strings.Contains(err.Error(), "不能超过 1 个") {
		t.Fatalf("启用超限任务应失败: %v", err)
	}
}

func TestSchedulerSkipsMisfireAndAdvancesFromNow(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	dm := &fakeDMRunner{permission: permission}
	service := NewService(
		config.Config{
			DatabaseDriver:                "sqlite",
			AutomationMisfirePolicy:       "skip",
			AutomationMisfireGraceSeconds: 10,
		},
		db,
		nil,
		dm,
		nil,
		permission,
		&fakeWorkspaceReader{},
		nil,
	)
	now := time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC)
	service.nowFn = func() time.Time { return now }
	task, err := service.CreateTask(context.Background(), schedulerPolicyTaskInput("错过窗口", true))
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	scheduledFor := now.Add(-5 * time.Minute)
	service.mu.Lock()
	service.jobStates[task.JobID].NextRunAt = &scheduledFor
	service.mu.Unlock()

	service.runDueOnce()
	if len(dm.Requests()) != 0 {
		t.Fatalf("misfire_policy=skip 不应启动 Agent: %+v", dm.Requests())
	}
	runs, err := service.repository.ListRunsByJob(context.Background(), task.OwnerUserID, task.JobID)
	if err != nil {
		t.Fatalf("ListRunsByJob 失败: %v", err)
	}
	if len(runs) != 1 || runs[0].Status != automationdomain.RunStatusSkipped || runs[0].TriggerKind != automationdomain.TriggerKindMisfire {
		t.Fatalf("应记录一条 misfire skipped run: %+v", runs)
	}
	current, err := service.GetTask(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("GetTask 失败: %v", err)
	}
	wantNext := now.Add(time.Hour)
	if current.NextRunAt == nil || !current.NextRunAt.Equal(wantNext) {
		t.Fatalf("跳过后应从当前时间推进调度: got=%v want=%s", current.NextRunAt, wantNext)
	}
}

func TestSchedulerDisablesExpiredTaskWithoutInterruptingExecution(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	dm := &fakeDMRunner{permission: permission}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		dm,
		nil,
		permission,
		&fakeWorkspaceReader{},
		nil,
	)
	now := time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC)
	service.nowFn = func() time.Time { return now }
	expiresAt := now.Add(time.Minute)
	input := schedulerPolicyTaskInput("有生命周期的任务", true)
	input.ExpiresAt = &expiresAt
	task, err := service.CreateTask(context.Background(), input)
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	now = expiresAt
	service.runDueOnce()
	if len(dm.Requests()) != 0 {
		t.Fatalf("过期任务不应启动 Agent: %+v", dm.Requests())
	}
	current, err := service.GetTask(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("GetTask 失败: %v", err)
	}
	if current.Enabled || current.NextRunAt != nil {
		t.Fatalf("过期任务应停用并清除下一次执行时间: %+v", current)
	}
	if current.ConfigurationVersion != task.ConfigurationVersion+1 {
		t.Fatalf("自动停用应只推进一次配置版本: got=%d want=%d", current.ConfigurationVersion, task.ConfigurationVersion+1)
	}
	if current.Name != task.Name || current.Instruction != task.Instruction ||
		!sameSchedule(current.Schedule, task.Schedule) {
		t.Fatalf("自动停用不应改写其他任务配置: before=%+v after=%+v", task, current)
	}
	changed, err := service.repository.DisableScheduledTaskAtVersion(
		context.Background(),
		task.OwnerUserID,
		task.JobID,
		task.ConfigurationVersion,
	)
	if err != nil || changed {
		t.Fatalf("重复提交同一停用意图应幂等: changed=%v err=%v", changed, err)
	}
	events, err := service.ListTaskEvents(context.Background(), task.JobID, 10)
	if err != nil {
		t.Fatalf("ListTaskEvents 失败: %v", err)
	}
	expireCount := 0
	for _, event := range events {
		if event.Action == automationdomain.TaskEventActionExpire {
			expireCount++
		}
	}
	if expireCount != 1 {
		t.Fatalf("应记录 expire 审计事件: %+v", events)
	}

	service.runDueOnce()
	repeated, err := service.repository.GetScheduledTask(context.Background(), task.OwnerUserID, task.JobID)
	if err != nil {
		t.Fatalf("重复调度后读取任务失败: %v", err)
	}
	if repeated == nil || repeated.ConfigurationVersion != current.ConfigurationVersion {
		t.Fatalf("重复调度不应再次推进版本: before=%+v after=%+v", current, repeated)
	}
	repeatedEvents, err := service.ListTaskEvents(context.Background(), task.JobID, 10)
	if err != nil {
		t.Fatalf("重复调度后读取事件失败: %v", err)
	}
	repeatedExpireCount := 0
	for _, event := range repeatedEvents {
		if event.Action == automationdomain.TaskEventActionExpire {
			repeatedExpireCount++
		}
	}
	if repeatedExpireCount != 1 {
		t.Fatalf("重复调度不应重复记录自动停用: %+v", repeatedEvents)
	}
}

func TestScheduledTaskReadsDoNotPersistPastOneShotDisable(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db, nil, nil, nil, permissionctx.NewContext(), &fakeWorkspaceReader{}, nil,
	)
	now := time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC)
	service.nowFn = func() time.Time { return now }
	runAt := now.Add(time.Minute)
	input := schedulerPolicyTaskInput("只读的一次性任务", true)
	runAtText := runAt.Format(time.RFC3339)
	input.Schedule = automationdomain.Schedule{
		Kind:     automationdomain.ScheduleKindAt,
		RunAt:    &runAtText,
		Timezone: "UTC",
	}
	task, err := service.CreateTask(context.Background(), input)
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	if _, err = db.ExecContext(
		context.Background(),
		"UPDATE automation_scheduled_tasks SET next_run_at = NULL WHERE job_id = ?",
		task.JobID,
	); err != nil {
		t.Fatalf("准备无 next_run_at 的持久快照失败: %v", err)
	}
	service.mu.Lock()
	delete(service.jobStates, task.JobID)
	service.mu.Unlock()
	now = runAt.Add(time.Second)

	before, err := service.repository.GetScheduledTask(context.Background(), task.OwnerUserID, task.JobID)
	if err != nil || before == nil {
		t.Fatalf("读取测试前快照失败: task=%+v err=%v", before, err)
	}
	if _, err = service.ListTasks(context.Background(), ""); err != nil {
		t.Fatalf("ListTasks 失败: %v", err)
	}
	if _, err = service.GetTask(context.Background(), task.JobID); err != nil {
		t.Fatalf("GetTask 失败: %v", err)
	}
	// 旧实现会在读取返回前登记异步整行 Upsert；等待后台任务可稳定捕获该回归。
	service.wg.Wait()
	after, err := service.repository.GetScheduledTask(context.Background(), task.OwnerUserID, task.JobID)
	if err != nil || after == nil {
		t.Fatalf("读取测试后快照失败: task=%+v err=%v", after, err)
	}
	if !after.Enabled || after.ConfigurationVersion != before.ConfigurationVersion {
		t.Fatalf("List/Get 只能投影，不能停用或推进版本: before=%+v after=%+v", before, after)
	}
}

func TestScheduledTaskDisableCASPreservesConcurrentConfiguration(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db, nil, nil, nil, permissionctx.NewContext(), &fakeWorkspaceReader{}, nil,
	)
	now := time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC)
	service.nowFn = func() time.Time { return now }
	expiresAt := now.Add(time.Minute)
	input := schedulerPolicyTaskInput("旧名称", true)
	input.ExpiresAt = &expiresAt
	task, err := service.CreateTask(context.Background(), input)
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	stale := *task
	updatedInput := stale
	updatedInput.Name = "用户刚保存的新名称"
	updatedInput.Instruction = "使用新的用户配置执行"
	updated, err := service.repository.UpdateScheduledTaskAtVersion(
		context.Background(),
		updatedInput,
		stale.ConfigurationVersion,
	)
	if err != nil {
		t.Fatalf("模拟并发配置更新失败: %v", err)
	}
	now = expiresAt

	changed, err := service.repository.DisableScheduledTaskAtVersion(
		context.Background(),
		stale.OwnerUserID,
		stale.JobID,
		stale.ConfigurationVersion,
	)
	if changed || !errors.Is(err, automationdomain.ErrConfigurationVersionConflict) {
		t.Fatalf("旧版本自动停用必须冲突: changed=%v err=%v", changed, err)
	}
	if err = service.disableScheduledTaskAtVersion(
		context.Background(),
		scheduledTaskDisableCandidate{job: stale, reason: scheduledTaskDisableReasonExpired},
		now,
	); err != nil {
		t.Fatalf("scheduler 应安全吸收版本冲突并刷新投影: %v", err)
	}
	current, err := service.repository.GetScheduledTask(context.Background(), task.OwnerUserID, task.JobID)
	if err != nil || current == nil {
		t.Fatalf("读取并发更新后的任务失败: task=%+v err=%v", current, err)
	}
	if !current.Enabled || current.ConfigurationVersion != updated.ConfigurationVersion ||
		current.Name != updated.Name || current.Instruction != updated.Instruction {
		t.Fatalf("旧 scheduler 快照覆盖了用户配置: want=%+v got=%+v", updated, current)
	}
	service.mu.Lock()
	projected := service.jobStates[task.JobID]
	if projected == nil || projected.Job.ConfigurationVersion != updated.ConfigurationVersion ||
		projected.Job.Name != updated.Name {
		service.mu.Unlock()
		t.Fatalf("冲突后应刷新到权威配置: state=%+v updated=%+v", projected, updated)
	}
	service.mu.Unlock()

	service.runDueOnce()
	resolved, err := service.repository.GetScheduledTask(context.Background(), task.OwnerUserID, task.JobID)
	if err != nil || resolved == nil {
		t.Fatalf("读取重新核对后的任务失败: task=%+v err=%v", resolved, err)
	}
	if resolved.Enabled || resolved.ConfigurationVersion != updated.ConfigurationVersion+1 ||
		resolved.Name != updated.Name || resolved.Instruction != updated.Instruction {
		t.Fatalf("scheduler 应用新版本停用时仍须保留用户配置: updated=%+v resolved=%+v", updated, resolved)
	}
}

func TestSchedulerDisablesCompletedOneShotThroughVersionedEntry(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db, nil, nil, nil, permissionctx.NewContext(), &fakeWorkspaceReader{}, nil,
	)
	now := time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC)
	service.nowFn = func() time.Time { return now }
	runAt := now.Add(time.Minute)
	runAtText := runAt.Format(time.RFC3339)
	input := schedulerPolicyTaskInput("已经完成的一次性任务", true)
	input.Schedule = automationdomain.Schedule{
		Kind:     automationdomain.ScheduleKindAt,
		RunAt:    &runAtText,
		Timezone: "UTC",
	}
	task, err := service.CreateTask(context.Background(), input)
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	service.mu.Lock()
	service.jobStates[task.JobID].NextRunAt = nil
	service.mu.Unlock()

	service.runDueOnce()
	current, err := service.repository.GetScheduledTask(context.Background(), task.OwnerUserID, task.JobID)
	if err != nil || current == nil {
		t.Fatalf("读取自动停用结果失败: task=%+v err=%v", current, err)
	}
	if current.Enabled || current.ConfigurationVersion != task.ConfigurationVersion+1 {
		t.Fatalf("one-shot 完成应由 scheduler 推进一次停用版本: before=%+v after=%+v", task, current)
	}
	events, err := service.ListTaskEvents(context.Background(), task.JobID, 10)
	if err != nil {
		t.Fatalf("读取 one-shot 事件失败: %v", err)
	}
	foundReason := false
	for _, event := range events {
		if event.Action == automationdomain.TaskEventActionDisable &&
			event.Detail["reason"] == scheduledTaskDisableReasonOneShotComplete {
			foundReason = true
			break
		}
	}
	if !foundReason {
		t.Fatalf("one-shot 自动停用应记录明确原因: %+v", events)
	}
}

func TestTaskExpirationMustBeInFutureAndCanBeCleared(t *testing.T) {
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
	now := time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC)
	service.nowFn = func() time.Time { return now }
	past := now.Add(-time.Second)
	invalid := schedulerPolicyTaskInput("已过期任务", true)
	invalid.ExpiresAt = &past
	if _, err := service.CreateTask(context.Background(), invalid); err == nil || !strings.Contains(err.Error(), "expires_at") {
		t.Fatalf("创建时应拒绝过去的 expires_at: %v", err)
	}

	future := now.Add(time.Hour)
	valid := schedulerPolicyTaskInput("可清除过期时间", true)
	valid.ExpiresAt = &future
	task, err := service.CreateTask(context.Background(), valid)
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	updated, err := service.UpdateTask(context.Background(), task.JobID, automationdomain.UpdateJobInput{ClearExpiresAt: true})
	if err != nil {
		t.Fatalf("清除 expires_at 失败: %v", err)
	}
	if updated.ExpiresAt != nil {
		t.Fatalf("expires_at 应已清除: %+v", updated.ExpiresAt)
	}
}

func schedulerPolicyTaskInput(name string, enabled bool) automationdomain.CreateJobInput {
	return automationdomain.CreateJobInput{
		Name:        name,
		AgentID:     "agent-1",
		Instruction: "执行检查",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated},
		Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Enabled:       enabled,
	}
}

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
