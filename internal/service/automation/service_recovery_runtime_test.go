package automation

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/protocol"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"

	_ "modernc.org/sqlite"
)

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
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "中断恢复",
		AgentID:     "agent-1",
		Instruction: "恢复上次运行",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
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

	runID := "run-interrupted"
	startedAt := time.Date(2026, 5, 21, 8, 0, 0, 0, time.UTC)
	if _, err = db.Exec(
		`INSERT INTO automation_cron_runs (run_id, job_id, owner_user_id, status, trigger_kind, attempts, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		runID,
		task.JobID,
		task.OwnerUserID,
		automationdomain.RunStatusRunning,
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
		automationdomain.RunStatusRunning,
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
	if runStatus != automationdomain.RunStatusCancelled {
		t.Fatalf("run status = %s, 期望 %s", runStatus, automationdomain.RunStatusCancelled)
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
	if !lastRunStatus.Valid || lastRunStatus.String != automationdomain.RunStatusCancelled {
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
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "手动释放",
		AgentID:     "agent-1",
		Instruction: "恢复卡住运行",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
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

	runID := "run-stuck"
	roundID := "round-stuck"
	sessionKey := protocol.BuildAgentSessionKey("agent-1", "automation", "dm", "cron:"+task.JobID+":"+runID, "")
	startedAt := time.Date(2026, 5, 21, 8, 0, 0, 0, time.UTC)
	if err = service.repository.InsertRunPending(context.Background(), automationstore.RunPendingInput{
		RunID:       runID,
		JobID:       task.JobID,
		OwnerUserID: task.OwnerUserID,
		TriggerKind: "manual",
		Status:      automationdomain.RunStatusPending,
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
	runningJob.LastRunStatus = automationdomain.RunStatusRunning
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
	if recovered.LastRunStatus != automationdomain.RunStatusCancelled {
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
	if runStatus != automationdomain.RunStatusCancelled {
		t.Fatalf("run status = %s, 期望 cancelled", runStatus)
	}
	if !runError.Valid || !strings.Contains(runError.String, "手动释放") {
		t.Fatalf("run error 未记录手动释放原因: %+v", runError)
	}

	lateFinished, err := service.repository.MarkRunFinishedIfActive(context.Background(), automationstore.RunFinishInput{
		RunID:      runID,
		Status:     automationdomain.RunStatusSucceeded,
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
	if runStatus != automationdomain.RunStatusCancelled {
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
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "释放 Room 卡住运行",
		AgentID:     "agent-1",
		Instruction: "恢复 Room 卡住运行",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetBound,
			BoundSessionKey: sessionKey,
		},
		Delivery: automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
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
		Status:      automationdomain.RunStatusPending,
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
	runningJob.LastRunStatus = automationdomain.RunStatusRunning
	service.replaceJobRuntimeState(runningJob)

	recovered, err := service.RecoverTaskRunningRun(context.Background(), task.JobID, runID)
	if err != nil {
		t.Fatalf("RecoverTaskRunningRun 失败: %v", err)
	}
	if recovered.Running || recovered.RunningRunID != "" || recovered.LastRunStatus != automationdomain.RunStatusCancelled {
		t.Fatalf("Room 运行占用未释放: %+v", recovered)
	}
	interrupts := room.Interrupts()
	if len(interrupts) != 1 || interrupts[0].SessionKey != sessionKey {
		t.Fatalf("recover_scheduled_task 应中断真实 Room 运行，实际 interrupts=%+v", interrupts)
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
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "自动释放",
		AgentID:     "agent-1",
		Instruction: "恢复超时运行",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind: automationdomain.SessionTargetIsolated,
		},
		Delivery: automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Source:   automationdomain.Source{Kind: automationdomain.SourceKindAgent, CreatorAgentID: "agent-1", ContextType: "agent", ContextID: "agent-1"},
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
		Status:      automationdomain.RunStatusPending,
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
	runningJob.LastRunStatus = automationdomain.RunStatusRunning
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
	if recovered.LastRunStatus != automationdomain.RunStatusCancelled {
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
	if runStatus != automationdomain.RunStatusCancelled {
		t.Fatalf("run status = %s, 期望 cancelled", runStatus)
	}
	if !runError.Valid || !strings.Contains(runError.String, "自动释放运行占用") {
		t.Fatalf("run error 未记录自动恢复原因: %+v", runError)
	}

	events, err := service.ListTaskEvents(context.Background(), task.JobID, 10)
	if err != nil {
		t.Fatalf("读取自动恢复事件失败: %v", err)
	}
	var recoverEvent *automationdomain.CronTaskEvent
	for index := range events {
		if events[index].Action == automationdomain.TaskEventActionRecover {
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
