package automation

import (
	"context"
	"database/sql"
	"testing"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"

	_ "modernc.org/sqlite"
)

func TestServiceCreateTaskPersistsRuntimeState(t *testing.T) {
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
	now := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
	service.nowFn = func() time.Time {
		return now
	}

	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "持久运行态",
		AgentID:     "agent-1",
		Instruction: "记录下一次运行",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(90),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind: automationdomain.SessionTargetIsolated,
		},
		Delivery: automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	if task.NextRunAt == nil {
		t.Fatalf("返回结果缺少 next_run_at")
	}

	var nextRunAt sql.NullTime
	var failureStreak int
	if err = db.QueryRow(`SELECT next_run_at, failure_streak FROM automation_scheduled_tasks WHERE job_id = ?`, task.JobID).Scan(&nextRunAt, &failureStreak); err != nil {
		t.Fatalf("读取持久运行态失败: %v", err)
	}
	if !nextRunAt.Valid {
		t.Fatalf("next_run_at 未持久化")
	}
	if got := nextRunAt.Time.UTC(); !got.Equal(now.Add(90 * time.Second)) {
		t.Fatalf("next_run_at = %s, 期望 %s", got, now.Add(90*time.Second))
	}
	if failureStreak != 0 {
		t.Fatalf("failure_streak = %d, 期望 0", failureStreak)
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
