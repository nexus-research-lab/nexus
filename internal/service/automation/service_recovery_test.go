package automation

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

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

	task, err := service.CreateTask(context.Background(), protocol.CreateJobInput{
		Name:        "持久运行态",
		AgentID:     "agent-1",
		Instruction: "记录下一次运行",
		Schedule: protocol.Schedule{
			Kind:            protocol.ScheduleKindEvery,
			IntervalSeconds: intRef(90),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: protocol.SessionTarget{
			Kind: protocol.SessionTargetIsolated,
		},
		Delivery: protocol.DeliveryTarget{Mode: protocol.DeliveryModeNone},
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
	if err = db.QueryRow(`SELECT next_run_at, failure_streak FROM automation_cron_jobs WHERE job_id = ?`, task.JobID).Scan(&nextRunAt, &failureStreak); err != nil {
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

func TestRepositoryClaimCronJobRuntimePreventsDuplicateExternalClaims(t *testing.T) {
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
	task, err := service.CreateTask(context.Background(), protocol.CreateJobInput{
		Name:        "领取防重",
		AgentID:     "agent-1",
		Instruction: "只应领取一次",
		Schedule: protocol.Schedule{
			Kind:            protocol.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: protocol.SessionTarget{
			Kind: protocol.SessionTargetIsolated,
		},
		Delivery:      protocol.DeliveryTarget{Mode: protocol.DeliveryModeNone},
		OverlapPolicy: protocol.OverlapPolicySkip,
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	startedAt := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
	nextRunAt := startedAt.Add(time.Hour)
	claimed, err := service.repository.ClaimCronJobRuntime(context.Background(), automationstore.JobRuntimeClaimInput{
		JobID:         task.JobID,
		RunID:         "run-1",
		StartedAt:     startedAt,
		NextRunAt:     &nextRunAt,
		OverlapPolicy: protocol.OverlapPolicySkip,
	})
	if err != nil {
		t.Fatalf("第一次领取失败: %v", err)
	}
	if !claimed {
		t.Fatalf("第一次领取应成功")
	}
	claimed, err = service.repository.ClaimCronJobRuntime(context.Background(), automationstore.JobRuntimeClaimInput{
		JobID:         task.JobID,
		RunID:         "run-2",
		StartedAt:     startedAt.Add(time.Second),
		NextRunAt:     &nextRunAt,
		OverlapPolicy: protocol.OverlapPolicySkip,
	})
	if err != nil {
		t.Fatalf("第二次领取失败: %v", err)
	}
	if claimed {
		t.Fatalf("overlap=skip 下 running_run_id 未清理时不应允许第二次领取")
	}

	var runningRunID sql.NullString
	if err = db.QueryRow(`SELECT running_run_id FROM automation_cron_jobs WHERE job_id = ?`, task.JobID).Scan(&runningRunID); err != nil {
		t.Fatalf("读取 running_run_id 失败: %v", err)
	}
	if !runningRunID.Valid || runningRunID.String != "run-1" {
		t.Fatalf("running_run_id = %+v, 期望 run-1", runningRunID)
	}

	result, err := service.startJobExecution(context.Background(), *task, "cron", startedAt.Add(2*time.Second))
	if err != nil {
		t.Fatalf("外部领取后本进程触发应返回当前运行态而不是报错: %v", err)
	}
	if result == nil || result.Status != protocol.RunStatusRunning || result.RunID == nil || *result.RunID != "run-1" {
		t.Fatalf("外部领取后的触发结果 = %+v, 期望 running/run-1", result)
	}
	if err = db.QueryRow(`SELECT running_run_id FROM automation_cron_jobs WHERE job_id = ?`, task.JobID).Scan(&runningRunID); err != nil {
		t.Fatalf("再次读取 running_run_id 失败: %v", err)
	}
	if !runningRunID.Valid || runningRunID.String != "run-1" {
		t.Fatalf("外部领取标记被错误清理: %+v", runningRunID)
	}
	var runCount int
	if err = db.QueryRow(`SELECT COUNT(*) FROM automation_cron_runs WHERE job_id = ?`, task.JobID).Scan(&runCount); err != nil {
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
	task, err := service.CreateTask(context.Background(), protocol.CreateJobInput{
		Name:          "脚本领取防重",
		AgentID:       "agent-1",
		Instruction:   "echo should-not-run",
		ExecutionKind: protocol.ExecutionKindScript,
		Schedule: protocol.Schedule{
			Kind:            protocol.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: protocol.SessionTarget{Kind: protocol.SessionTargetIsolated},
		Delivery:      protocol.DeliveryTarget{Mode: protocol.DeliveryModeNone},
		OverlapPolicy: protocol.OverlapPolicySkip,
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	startedAt := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
	nextRunAt := startedAt.Add(time.Hour)
	claimed, err := service.repository.ClaimCronJobRuntime(context.Background(), automationstore.JobRuntimeClaimInput{
		JobID:         task.JobID,
		RunID:         "run-script-1",
		StartedAt:     startedAt,
		NextRunAt:     &nextRunAt,
		OverlapPolicy: protocol.OverlapPolicySkip,
	})
	if err != nil {
		t.Fatalf("脚本任务外部领取失败: %v", err)
	}
	if !claimed {
		t.Fatal("脚本任务外部领取应成功")
	}
	result, err := service.startJobExecution(context.Background(), *task, "cron", startedAt.Add(2*time.Second))
	if err != nil {
		t.Fatalf("脚本任务外部领取后触发失败: %v", err)
	}
	if result == nil || result.Status != protocol.RunStatusRunning || result.RunID == nil || *result.RunID != "run-script-1" {
		t.Fatalf("脚本任务外部领取后的触发结果 = %+v, 期望 running/run-script-1", result)
	}
	var runCount int
	if err = db.QueryRow(`SELECT COUNT(*) FROM automation_cron_runs WHERE job_id = ?`, task.JobID).Scan(&runCount); err != nil {
		t.Fatalf("读取脚本 run 数量失败: %v", err)
	}
	if runCount != 0 {
		t.Fatalf("脚本任务被其他调度器领取时，本进程不应写入 skipped run，实际 %d", runCount)
	}
}

func TestServiceBootstrapRecoversInterruptedTaskRuntime(t *testing.T) {
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
	task, err := service.CreateTask(context.Background(), protocol.CreateJobInput{
		Name:        "中断恢复",
		AgentID:     "agent-1",
		Instruction: "恢复上次运行",
		Schedule: protocol.Schedule{
			Kind:            protocol.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: protocol.SessionTarget{
			Kind: protocol.SessionTargetIsolated,
		},
		Delivery: protocol.DeliveryTarget{Mode: protocol.DeliveryModeNone},
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	runID := "run-interrupted"
	startedAt := time.Date(2026, 5, 21, 8, 0, 0, 0, time.UTC)
	if _, err = db.Exec(
		`INSERT INTO automation_cron_runs (run_id, job_id, owner_user_id, status, trigger_kind, attempts, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		runID,
		task.JobID,
		task.OwnerUserID,
		protocol.RunStatusRunning,
		"cron",
	); err != nil {
		t.Fatalf("预置 running run 失败: %v", err)
	}
	if _, err = db.Exec(
		`UPDATE automation_cron_jobs
SET running_run_id = ?, running_started_at = ?, last_run_status = ?, failure_streak = 0, next_run_at = NULL
WHERE job_id = ?`,
		runID,
		startedAt,
		protocol.RunStatusRunning,
		task.JobID,
	); err != nil {
		t.Fatalf("预置 running job 失败: %v", err)
	}

	recoveredAt := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
	recoveredService := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		nil,
		nil,
		permissionctx.NewContext(),
		&fakeWorkspaceReader{},
		nil,
	)
	recoveredService.nowFn = func() time.Time {
		return recoveredAt
	}
	if err = recoveredService.bootstrapRuntime(context.Background()); err != nil {
		t.Fatalf("bootstrapRuntime 失败: %v", err)
	}

	var runStatus string
	var runError sql.NullString
	var finishedAt sql.NullTime
	if err = db.QueryRow(`SELECT status, error_message, finished_at FROM automation_cron_runs WHERE run_id = ?`, runID).Scan(&runStatus, &runError, &finishedAt); err != nil {
		t.Fatalf("读取恢复后的 run 失败: %v", err)
	}
	if runStatus != protocol.RunStatusCancelled {
		t.Fatalf("run status = %s, 期望 %s", runStatus, protocol.RunStatusCancelled)
	}
	if !runError.Valid || !strings.Contains(runError.String, "scheduler restarted") {
		t.Fatalf("run error 未记录重启原因: %+v", runError)
	}
	if !finishedAt.Valid {
		t.Fatalf("run finished_at 未记录")
	}

	var runningRunID sql.NullString
	var nextRunAt sql.NullTime
	var lastRunStatus sql.NullString
	var failureStreak int
	var lastError sql.NullString
	if err = db.QueryRow(
		`SELECT running_run_id, next_run_at, last_run_status, failure_streak, last_error
FROM automation_cron_jobs WHERE job_id = ?`,
		task.JobID,
	).Scan(&runningRunID, &nextRunAt, &lastRunStatus, &failureStreak, &lastError); err != nil {
		t.Fatalf("读取恢复后的 job 失败: %v", err)
	}
	if runningRunID.Valid {
		t.Fatalf("running_run_id 未清理: %s", runningRunID.String)
	}
	if !nextRunAt.Valid || !nextRunAt.Time.UTC().Equal(recoveredAt.Add(30*time.Second)) {
		t.Fatalf("next_run_at = %+v, 期望 %s", nextRunAt, recoveredAt.Add(30*time.Second))
	}
	if !lastRunStatus.Valid || lastRunStatus.String != protocol.RunStatusCancelled {
		t.Fatalf("last_run_status = %+v, 期望 cancelled", lastRunStatus)
	}
	if failureStreak != 1 {
		t.Fatalf("failure_streak = %d, 期望 1", failureStreak)
	}
	if !lastError.Valid || !strings.Contains(lastError.String, "scheduler restarted") {
		t.Fatalf("last_error 未记录重启原因: %+v", lastError)
	}
}

func TestServiceRecoverTaskRunningRunReleasesStuckRuntime(t *testing.T) {
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
	task, err := service.CreateTask(context.Background(), protocol.CreateJobInput{
		Name:        "手动释放",
		AgentID:     "agent-1",
		Instruction: "恢复卡住运行",
		Schedule: protocol.Schedule{
			Kind:            protocol.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: protocol.SessionTarget{
			Kind: protocol.SessionTargetIsolated,
		},
		Delivery: protocol.DeliveryTarget{Mode: protocol.DeliveryModeNone},
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	runID := "run-stuck"
	roundID := "round-stuck"
	sessionKey := protocol.BuildAgentSessionKey("agent-1", "automation", "dm", "cron:"+task.JobID+":"+runID, "")
	startedAt := time.Date(2026, 5, 21, 8, 0, 0, 0, time.UTC)
	if err = service.repository.InsertRunPending(context.Background(), automationstore.RunPendingInput{
		RunID:       runID,
		JobID:       task.JobID,
		OwnerUserID: task.OwnerUserID,
		TriggerKind: "manual",
		Status:      protocol.RunStatusPending,
		SessionKey:  sessionKey,
		RoundID:     roundID,
	}); err != nil {
		t.Fatalf("预置 pending run 失败: %v", err)
	}
	if err = service.repository.MarkRunRunning(context.Background(), runID, startedAt); err != nil {
		t.Fatalf("预置 running run 失败: %v", err)
	}
	runningJob := *task
	runningJob.Running = true
	runningJob.RunningRunID = runID
	runningJob.RunningStartedAt = &startedAt
	runningJob.LastRunStatus = protocol.RunStatusRunning
	service.replaceJobRuntimeState(runningJob)

	recoveredAt := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
	service.nowFn = func() time.Time {
		return recoveredAt
	}
	recovered, err := service.RecoverTaskRunningRun(context.Background(), task.JobID, runID)
	if err != nil {
		t.Fatalf("RecoverTaskRunningRun 失败: %v", err)
	}
	if recovered.Running || recovered.RunningRunID != "" || recovered.RunningStartedAt != nil {
		t.Fatalf("运行占用未释放: %+v", recovered)
	}
	if recovered.LastRunStatus != protocol.RunStatusCancelled {
		t.Fatalf("last_run_status = %s, 期望 cancelled", recovered.LastRunStatus)
	}
	if recovered.FailureStreak != 1 {
		t.Fatalf("failure_streak = %d, 期望 1", recovered.FailureStreak)
	}
	if recovered.LastError == nil || !strings.Contains(*recovered.LastError, "手动释放") {
		t.Fatalf("last_error 未记录手动释放原因: %+v", recovered.LastError)
	}
	interrupts := dm.Interrupts()
	if len(interrupts) != 1 {
		t.Fatalf("recover_scheduled_task 应中断真实 DM 运行，实际 interrupts=%+v", interrupts)
	}
	if interrupts[0].SessionKey != sessionKey || interrupts[0].RoundID != roundID {
		t.Fatalf("DM 中断请求不正确: %+v", interrupts[0])
	}

	var runStatus string
	var runError sql.NullString
	if err = db.QueryRow(`SELECT status, error_message FROM automation_cron_runs WHERE run_id = ?`, runID).Scan(&runStatus, &runError); err != nil {
		t.Fatalf("读取恢复后的 run 失败: %v", err)
	}
	if runStatus != protocol.RunStatusCancelled {
		t.Fatalf("run status = %s, 期望 cancelled", runStatus)
	}
	if !runError.Valid || !strings.Contains(runError.String, "手动释放") {
		t.Fatalf("run error 未记录手动释放原因: %+v", runError)
	}

	lateFinished, err := service.repository.MarkRunFinishedIfActive(context.Background(), automationstore.RunFinishInput{
		RunID:      runID,
		Status:     protocol.RunStatusSucceeded,
		FinishedAt: recoveredAt.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("迟到完成写入失败: %v", err)
	}
	if lateFinished {
		t.Fatalf("手动释放后的 run 不应再被迟到完成覆盖")
	}
	if err = db.QueryRow(`SELECT status FROM automation_cron_runs WHERE run_id = ?`, runID).Scan(&runStatus); err != nil {
		t.Fatalf("再次读取 run 状态失败: %v", err)
	}
	if runStatus != protocol.RunStatusCancelled {
		t.Fatalf("迟到完成覆盖了 run 状态: %s", runStatus)
	}
}

func TestServiceRecoverTaskRunningRunInterruptsRoomRuntime(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	room := &fakeRoomRunner{permission: permission}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		nil,
		room,
		permission,
		&fakeWorkspaceReader{},
		nil,
	)
	sessionKey := protocol.BuildRoomSharedSessionKey("conversation-1")
	task, err := service.CreateTask(context.Background(), protocol.CreateJobInput{
		Name:        "释放 Room 卡住运行",
		AgentID:     "agent-1",
		Instruction: "恢复 Room 卡住运行",
		Schedule: protocol.Schedule{
			Kind:            protocol.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: protocol.SessionTarget{
			Kind:            protocol.SessionTargetBound,
			BoundSessionKey: sessionKey,
		},
		Delivery: protocol.DeliveryTarget{Mode: protocol.DeliveryModeNone},
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	runID := "run-room-stuck"
	startedAt := time.Date(2026, 5, 21, 8, 0, 0, 0, time.UTC)
	if err = service.repository.InsertRunPending(context.Background(), automationstore.RunPendingInput{
		RunID:       runID,
		JobID:       task.JobID,
		OwnerUserID: task.OwnerUserID,
		TriggerKind: "manual",
		Status:      protocol.RunStatusPending,
		SessionKey:  sessionKey,
		RoundID:     "round-room-stuck",
	}); err != nil {
		t.Fatalf("预置 pending run 失败: %v", err)
	}
	if err = service.repository.MarkRunRunning(context.Background(), runID, startedAt); err != nil {
		t.Fatalf("预置 running run 失败: %v", err)
	}
	runningJob := *task
	runningJob.Running = true
	runningJob.RunningRunID = runID
	runningJob.RunningStartedAt = &startedAt
	runningJob.LastRunStatus = protocol.RunStatusRunning
	service.replaceJobRuntimeState(runningJob)

	recovered, err := service.RecoverTaskRunningRun(context.Background(), task.JobID, runID)
	if err != nil {
		t.Fatalf("RecoverTaskRunningRun 失败: %v", err)
	}
	if recovered.Running || recovered.RunningRunID != "" || recovered.LastRunStatus != protocol.RunStatusCancelled {
		t.Fatalf("Room 运行占用未释放: %+v", recovered)
	}
	interrupts := room.Interrupts()
	if len(interrupts) != 1 || interrupts[0].SessionKey != sessionKey {
		t.Fatalf("recover_scheduled_task 应中断真实 Room 运行，实际 interrupts=%+v", interrupts)
	}
}

func TestDisableTaskPreservesActiveRunForRecovery(t *testing.T) {
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
	task, err := service.CreateTask(context.Background(), protocol.CreateJobInput{
		Name:        "停用运行中任务",
		AgentID:     "agent-1",
		Instruction: "正在运行时被停用",
		Schedule: protocol.Schedule{
			Kind:            protocol.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: protocol.SessionTarget{Kind: protocol.SessionTargetIsolated},
		Delivery:      protocol.DeliveryTarget{Mode: protocol.DeliveryModeNone},
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	runID := "run-disable-active"
	startedAt := time.Date(2026, 5, 21, 8, 0, 0, 0, time.UTC)
	if err = service.repository.InsertRunPending(context.Background(), automationstore.RunPendingInput{
		RunID:       runID,
		JobID:       task.JobID,
		OwnerUserID: task.OwnerUserID,
		TriggerKind: "manual",
		Status:      protocol.RunStatusPending,
	}); err != nil {
		t.Fatalf("预置 pending run 失败: %v", err)
	}
	if err = service.repository.MarkRunRunning(context.Background(), runID, startedAt); err != nil {
		t.Fatalf("预置 running run 失败: %v", err)
	}
	runningJob := *task
	runningJob.Running = true
	runningJob.RunningRunID = runID
	runningJob.RunningStartedAt = &startedAt
	runningJob.LastRunStatus = protocol.RunStatusRunning
	service.replaceJobRuntimeState(runningJob)

	disabled, err := service.UpdateTaskStatus(context.Background(), task.JobID, false)
	if err != nil {
		t.Fatalf("UpdateTaskStatus 失败: %v", err)
	}
	if disabled.Enabled {
		t.Fatal("任务应已停用")
	}
	if !disabled.Running || disabled.RunningRunID != runID || disabled.RunningStartedAt == nil {
		t.Fatalf("停用不应隐藏 active run: %+v", disabled)
	}
	if disabled.NextRunAt != nil {
		t.Fatalf("停用后不应安排下一次运行: %+v", disabled.NextRunAt)
	}

	var runningRunID sql.NullString
	var nextRunAt sql.NullTime
	if err = db.QueryRow(`SELECT running_run_id, next_run_at FROM automation_cron_jobs WHERE job_id = ?`, task.JobID).Scan(&runningRunID, &nextRunAt); err != nil {
		t.Fatalf("读取停用后的 job runtime 失败: %v", err)
	}
	if !runningRunID.Valid || runningRunID.String != runID {
		t.Fatalf("running_run_id 应保留 active run，实际 %+v", runningRunID)
	}
	if nextRunAt.Valid {
		t.Fatalf("停用后 next_run_at 应为空，实际 %+v", nextRunAt)
	}

	events, err := service.ListTaskEvents(context.Background(), task.JobID, 10)
	if err != nil {
		t.Fatalf("ListTaskEvents 失败: %v", err)
	}
	var disableEvent *protocol.CronTaskEvent
	for index := range events {
		if events[index].Action == protocol.TaskEventActionDisable {
			disableEvent = &events[index]
			break
		}
	}
	if disableEvent == nil {
		t.Fatalf("缺少 disable 事件: %+v", events)
	}
	if disableEvent.RunID != runID || disableEvent.Detail["active_run_id"] != runID {
		t.Fatalf("disable 事件未关联 active run: %+v", disableEvent)
	}

	recovered, err := service.RecoverTaskRunningRun(context.Background(), task.JobID, runID)
	if err != nil {
		t.Fatalf("RecoverTaskRunningRun 失败: %v", err)
	}
	if recovered.Running || recovered.RunningRunID != "" {
		t.Fatalf("恢复后 running 应清空: %+v", recovered)
	}
	var runStatus string
	if err = db.QueryRow(`SELECT status FROM automation_cron_runs WHERE run_id = ?`, runID).Scan(&runStatus); err != nil {
		t.Fatalf("读取恢复后的 run 失败: %v", err)
	}
	if runStatus != protocol.RunStatusCancelled {
		t.Fatalf("run status = %s, 期望 cancelled", runStatus)
	}
}

func TestDeleteTaskCancelsActiveRun(t *testing.T) {
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
	task, err := service.CreateTask(context.Background(), protocol.CreateJobInput{
		Name:        "删除运行中任务",
		AgentID:     "agent-1",
		Instruction: "正在运行时被删除",
		Schedule: protocol.Schedule{
			Kind:            protocol.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: protocol.SessionTarget{Kind: protocol.SessionTargetIsolated},
		Delivery:      protocol.DeliveryTarget{Mode: protocol.DeliveryModeNone},
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	runID := "run-delete-active"
	roundID := "round-delete-active"
	sessionKey := protocol.BuildAgentSessionKey("agent-1", "automation", "dm", "cron:"+task.JobID+":"+runID, "")
	startedAt := time.Date(2026, 5, 21, 8, 0, 0, 0, time.UTC)
	if err = service.repository.InsertRunPending(context.Background(), automationstore.RunPendingInput{
		RunID:       runID,
		JobID:       task.JobID,
		OwnerUserID: task.OwnerUserID,
		TriggerKind: "manual",
		Status:      protocol.RunStatusPending,
		SessionKey:  sessionKey,
		RoundID:     roundID,
	}); err != nil {
		t.Fatalf("预置 pending run 失败: %v", err)
	}
	if err = service.repository.MarkRunRunning(context.Background(), runID, startedAt); err != nil {
		t.Fatalf("预置 running run 失败: %v", err)
	}
	runningJob := *task
	runningJob.Running = true
	runningJob.RunningRunID = runID
	runningJob.RunningStartedAt = &startedAt
	runningJob.LastRunStatus = protocol.RunStatusRunning
	service.replaceJobRuntimeState(runningJob)

	deletedAt := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
	service.nowFn = func() time.Time {
		return deletedAt
	}
	result, err := service.DeleteTask(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("DeleteTask 失败: %v", err)
	}
	if result.JobID != task.JobID || !result.Deleted || result.ActiveRunID != runID ||
		result.CancelledRunID != runID || !result.CancelledActiveRun {
		t.Fatalf("DeleteTask 返回结果未记录 active run 取消: %+v", result)
	}
	deletedJob, err := service.GetTask(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("删除后 GetTask 失败: %v", err)
	}
	if deletedJob != nil {
		t.Fatalf("任务应已删除: %+v", deletedJob)
	}

	var runStatus string
	var runError sql.NullString
	var finishedAt sql.NullTime
	if err = db.QueryRow(`SELECT status, error_message, finished_at FROM automation_cron_runs WHERE run_id = ?`, runID).Scan(&runStatus, &runError, &finishedAt); err != nil {
		t.Fatalf("读取删除后的 run 失败: %v", err)
	}
	if runStatus != protocol.RunStatusCancelled {
		t.Fatalf("run status = %s, 期望 cancelled", runStatus)
	}
	if !runError.Valid || !strings.Contains(runError.String, "deleted") {
		t.Fatalf("run error 未记录删除原因: %+v", runError)
	}
	if !finishedAt.Valid {
		t.Fatalf("run finished_at 未记录")
	}
	interrupts := dm.Interrupts()
	if len(interrupts) != 1 {
		t.Fatalf("删除 active run 应中断真实 DM 运行，实际 interrupts=%+v", interrupts)
	}
	if interrupts[0].SessionKey != sessionKey || interrupts[0].RoundID != roundID {
		t.Fatalf("DM 中断请求不正确: %+v", interrupts[0])
	}

	lateFinished, err := service.repository.MarkRunFinishedIfActive(context.Background(), automationstore.RunFinishInput{
		RunID:      runID,
		Status:     protocol.RunStatusSucceeded,
		FinishedAt: deletedAt.Add(time.Minute),
	})
	if err != nil {
		t.Fatalf("迟到完成写入失败: %v", err)
	}
	if lateFinished {
		t.Fatalf("删除取消后的 run 不应再被迟到完成覆盖")
	}
	if err = db.QueryRow(`SELECT status FROM automation_cron_runs WHERE run_id = ?`, runID).Scan(&runStatus); err != nil {
		t.Fatalf("再次读取 run 状态失败: %v", err)
	}
	if runStatus != protocol.RunStatusCancelled {
		t.Fatalf("迟到完成覆盖了 run status: %s", runStatus)
	}

	events, err := service.ListTaskEvents(context.Background(), task.JobID, 10)
	if err != nil {
		t.Fatalf("删除后 ListTaskEvents 失败: %v", err)
	}
	var deleteEvent *protocol.CronTaskEvent
	for index := range events {
		if events[index].Action == protocol.TaskEventActionDelete {
			deleteEvent = &events[index]
			break
		}
	}
	if deleteEvent == nil {
		t.Fatalf("缺少 delete 事件: %+v", events)
	}
	if deleteEvent.RunID != runID {
		t.Fatalf("delete 事件 run_id = %q, 期望 %q", deleteEvent.RunID, runID)
	}
	if deleteEvent.Detail["cancelled_run_id"] != runID || deleteEvent.Detail["cancelled_active_run"] != true {
		t.Fatalf("delete 事件未记录取消 run: %+v", deleteEvent.Detail)
	}
}

func TestDeleteTaskInterruptsActiveRoomRun(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	room := &fakeRoomRunner{permission: permission}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		nil,
		room,
		permission,
		&fakeWorkspaceReader{},
		nil,
	)
	sessionKey := protocol.BuildRoomSharedSessionKey("conversation-1")
	task, err := service.CreateTask(context.Background(), protocol.CreateJobInput{
		Name:        "删除运行中的 Room 任务",
		AgentID:     "agent-1",
		Instruction: "在 Room 中执行后删除",
		Schedule: protocol.Schedule{
			Kind:            protocol.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: protocol.SessionTarget{
			Kind:            protocol.SessionTargetBound,
			BoundSessionKey: sessionKey,
		},
		Delivery: protocol.DeliveryTarget{Mode: protocol.DeliveryModeNone},
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	runID := "run-delete-room-active"
	startedAt := time.Date(2026, 5, 21, 8, 0, 0, 0, time.UTC)
	if err = service.repository.InsertRunPending(context.Background(), automationstore.RunPendingInput{
		RunID:       runID,
		JobID:       task.JobID,
		OwnerUserID: task.OwnerUserID,
		TriggerKind: "manual",
		Status:      protocol.RunStatusPending,
		SessionKey:  sessionKey,
		RoundID:     "round-delete-room-active",
	}); err != nil {
		t.Fatalf("预置 pending run 失败: %v", err)
	}
	if err = service.repository.MarkRunRunning(context.Background(), runID, startedAt); err != nil {
		t.Fatalf("预置 running run 失败: %v", err)
	}
	runningJob := *task
	runningJob.Running = true
	runningJob.RunningRunID = runID
	runningJob.RunningStartedAt = &startedAt
	runningJob.LastRunStatus = protocol.RunStatusRunning
	service.replaceJobRuntimeState(runningJob)

	result, err := service.DeleteTask(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("DeleteTask 失败: %v", err)
	}
	if !result.CancelledActiveRun || result.CancelledRunID != runID {
		t.Fatalf("DeleteTask 未记录 Room active run 取消: %+v", result)
	}
	interrupts := room.Interrupts()
	if len(interrupts) != 1 || interrupts[0].SessionKey != sessionKey {
		t.Fatalf("删除 active Room run 应中断共享 Room 会话，实际 interrupts=%+v", interrupts)
	}
}

func TestServiceWatchdogRecoversTimedOutRunningRun(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(
		config.Config{DatabaseDriver: "sqlite", AutomationRunTimeoutSeconds: 3600},
		db,
		nil,
		nil,
		nil,
		permissionctx.NewContext(),
		&fakeWorkspaceReader{},
		nil,
	)
	task, err := service.CreateTask(context.Background(), protocol.CreateJobInput{
		Name:        "自动释放",
		AgentID:     "agent-1",
		Instruction: "恢复超时运行",
		Schedule: protocol.Schedule{
			Kind:            protocol.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: protocol.SessionTarget{
			Kind: protocol.SessionTargetIsolated,
		},
		Delivery: protocol.DeliveryTarget{Mode: protocol.DeliveryModeNone},
		Source:   protocol.Source{Kind: protocol.SourceKindAgent, CreatorAgentID: "agent-1", ContextType: "agent", ContextID: "agent-1"},
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	runID := "run-timeout"
	startedAt := time.Date(2026, 5, 21, 7, 0, 0, 0, time.UTC)
	recoveredAt := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
	if err = service.repository.InsertRunPending(context.Background(), automationstore.RunPendingInput{
		RunID:       runID,
		JobID:       task.JobID,
		OwnerUserID: task.OwnerUserID,
		TriggerKind: "cron",
		Status:      protocol.RunStatusPending,
	}); err != nil {
		t.Fatalf("预置 pending run 失败: %v", err)
	}
	if err = service.repository.MarkRunRunning(context.Background(), runID, startedAt); err != nil {
		t.Fatalf("预置 running run 失败: %v", err)
	}
	runningJob := *task
	runningJob.Running = true
	runningJob.RunningRunID = runID
	runningJob.RunningStartedAt = &startedAt
	runningJob.LastRunStatus = protocol.RunStatusRunning
	service.replaceJobRuntimeState(runningJob)
	service.nowFn = func() time.Time {
		return recoveredAt
	}

	service.recoverStaleRunningJobs(context.Background(), recoveredAt)

	recovered, err := service.GetTask(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("读取自动恢复后的任务失败: %v", err)
	}
	if recovered == nil {
		t.Fatal("任务不存在")
	}
	if recovered.Running || recovered.RunningRunID != "" || recovered.RunningStartedAt != nil {
		t.Fatalf("超时运行占用未释放: %+v", recovered)
	}
	if recovered.LastRunStatus != protocol.RunStatusCancelled {
		t.Fatalf("last_run_status = %s, 期望 cancelled", recovered.LastRunStatus)
	}
	if recovered.LastError == nil || !strings.Contains(*recovered.LastError, "自动释放运行占用") {
		t.Fatalf("last_error 未记录自动恢复原因: %+v", recovered.LastError)
	}

	var runStatus string
	var runError sql.NullString
	if err = db.QueryRow(`SELECT status, error_message FROM automation_cron_runs WHERE run_id = ?`, runID).Scan(&runStatus, &runError); err != nil {
		t.Fatalf("读取恢复后的 run 失败: %v", err)
	}
	if runStatus != protocol.RunStatusCancelled {
		t.Fatalf("run status = %s, 期望 cancelled", runStatus)
	}
	if !runError.Valid || !strings.Contains(runError.String, "自动释放运行占用") {
		t.Fatalf("run error 未记录自动恢复原因: %+v", runError)
	}

	events, err := service.ListTaskEvents(context.Background(), task.JobID, 10)
	if err != nil {
		t.Fatalf("读取自动恢复事件失败: %v", err)
	}
	var recoverEvent *protocol.CronTaskEvent
	for index := range events {
		if events[index].Action == protocol.TaskEventActionRecover {
			recoverEvent = &events[index]
			break
		}
	}
	if recoverEvent == nil {
		t.Fatalf("缺少自动恢复事件: %+v", events)
	}
	if recoverEvent.RunID != runID || recoverEvent.Detail["reason"] != "timeout" {
		t.Fatalf("自动恢复事件不完整: %+v", recoverEvent)
	}
}

func TestDeleteTaskCleansIsolatedAutomationSessions(t *testing.T) {
	workspacePath := t.TempDir()
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	service := NewService(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspacePath},
		db,
		nil,
		&fakeDMRunner{permission: permission},
		nil,
		permission,
		&fakeWorkspaceReader{},
		nil,
	)
	runtimeCloser := &fakeRuntimeSessionCloser{}
	service.SetRuntimeSessionCloser(runtimeCloser)
	task, err := service.CreateTask(context.Background(), protocol.CreateJobInput{
		Name:        "cleanup-target",
		AgentID:     "agent-1",
		Instruction: "cleanup",
		Schedule: protocol.Schedule{
			Kind:            protocol.ScheduleKindEvery,
			IntervalSeconds: intRef(60),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: protocol.SessionTarget{Kind: protocol.SessionTargetIsolated},
		Delivery:      protocol.DeliveryTarget{Mode: protocol.DeliveryModeNone},
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	store := workspacestore.NewSessionFileStore(workspacePath)
	now := time.Now().UTC()
	matchingA := protocol.BuildAgentSessionKey("agent-1", "automation", "dm", "cron:"+task.JobID+":run-a", "")
	matchingB := protocol.BuildAgentSessionKey("agent-1", "automation", "dm", "cron:"+task.JobID+":run-b", "")
	unrelated := protocol.BuildAgentSessionKey("agent-1", "ws", "dm", "keep", "")
	for _, sessionKey := range []string{matchingA, matchingB, unrelated} {
		if _, upsertErr := store.UpsertSession(workspacePath, protocol.Session{
			SessionKey:   sessionKey,
			AgentID:      "agent-1",
			ChannelType:  "automation",
			ChatType:     "dm",
			Status:       "active",
			CreatedAt:    now,
			LastActivity: now,
			Title:        "session",
			Options:      map[string]any{},
			IsActive:     true,
		}); upsertErr != nil {
			t.Fatalf("准备测试会话失败: %v", upsertErr)
		}
	}

	if _, err = service.DeleteTask(context.Background(), task.JobID); err != nil {
		t.Fatalf("DeleteTask 失败: %v", err)
	}

	paths := []string{workspacePath}
	for _, removedKey := range []string{matchingA, matchingB} {
		item, _, findErr := store.FindSession(paths, removedKey)
		if findErr != nil {
			t.Fatalf("查询会话失败: %v", findErr)
		}
		if item != nil {
			t.Fatalf("期望会话被清理: %s", removedKey)
		}
	}
	closed := runtimeCloser.Calls()
	if len(closed) != 2 {
		t.Fatalf("期望关闭 2 个 isolated 会话，实际 %d", len(closed))
	}
	item, _, findErr := store.FindSession(paths, unrelated)
	if findErr != nil {
		t.Fatalf("查询保留会话失败: %v", findErr)
	}
	if item == nil {
		t.Fatalf("不应删除非 automation 会话")
	}
}
