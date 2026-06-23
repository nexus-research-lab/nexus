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
