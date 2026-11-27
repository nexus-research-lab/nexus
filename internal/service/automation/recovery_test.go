// recovery_test.go 覆盖自动化任务启动恢复与清理测试。
package automation

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
	_ "modernc.org/sqlite"
)

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
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "删除运行中的 Room 任务",
		AgentID:     "agent-1",
		Instruction: "在 Room 中执行后删除",
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

	runID := "run-delete-room-active"
	startedAt := time.Date(2026, 5, 21, 8, 0, 0, 0, time.UTC)
	if err = service.repository.InsertRunPending(context.Background(), automationstore.RunPendingInput{
		RunID:       runID,
		JobID:       task.JobID,
		OwnerUserID: task.OwnerUserID,
		TriggerKind: automationdomain.TriggerKindManual,
		Status:      automationdomain.RunStatusPending,
		SessionKey:  sessionKey,
		RoundID:     "round-delete-room-active",
	}); err != nil {
		t.Fatalf("预置 pending run 失败: %v", err)
	}
	if err = service.repository.MarkRunRunning(context.Background(), runID, startedAt); err != nil {
		t.Fatalf("预置 running run 失败: %v", err)
	}
	completeAttempt := service.registerPhysicalAttempt(runID, "round-delete-room-active")
	defer completeAttempt()
	runningJob := *task
	runningJob.Running = true
	runningJob.RunningRunID = runID
	runningJob.RunningStartedAt = &startedAt
	runningJob.LastRunStatus = automationdomain.RunStatusRunning
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
	workspacePath := newAutomationOwnerWorkspace(t, authctx.SystemUserID, "agent-1")
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
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "cleanup-target",
		AgentID:     "agent-1",
		Instruction: "cleanup",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(60),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated},
		Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	store := workspacestore.NewSessionFileStore(workspacePath)
	now := time.Now().UTC()
	matchingA := protocol.BuildAgentSessionKey("agent-1", "automation", "dm", "scheduled-task:"+task.JobID+":run-a", "")
	matchingB := protocol.BuildAgentSessionKey("agent-1", "automation", "dm", "scheduled-task:"+task.JobID+":run-b", "")
	unrelated := protocol.BuildAgentSessionKey("agent-1", "ws", "dm", "keep", "")
	for _, sessionKey := range []string{matchingA, matchingB, unrelated} {
		cleanupSessionID := "sdk-" + sessionKey
		if _, upsertErr := store.UpsertSession(workspacePath, protocol.Session{
			SessionKey:   sessionKey,
			AgentID:      "agent-1",
			SessionID:    &cleanupSessionID,
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
	coordinator := &fakeSessionArtifactDeletionCoordinator{
		deleteFn: func(_ context.Context, call sessionArtifactDeletionCall) error {
			_, deleteErr := store.ForOwner(call.ownerUserID).DeleteSession(
				call.workspacePath,
				call.sessionKey,
			)
			return deleteErr
		},
	}
	service.SetSessionArtifactDeletionCoordinator(coordinator)

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
	calls := coordinator.Calls()
	if len(calls) != 2 {
		t.Fatalf("期望统一清理 2 个 isolated 会话，实际 %+v", calls)
	}
	for _, call := range calls {
		if call.ownerUserID != authctx.SystemUserID ||
			call.workspacePath != workspacePath ||
			(call.sessionKey != matchingA && call.sessionKey != matchingB) ||
			call.cleanupSessionID != "sdk-"+call.sessionKey {
			t.Fatalf("Session artifact 删除参数不完整: %+v", call)
		}
	}
	item, _, findErr := store.FindSession(paths, unrelated)
	if findErr != nil {
		t.Fatalf("查询保留会话失败: %v", findErr)
	}
	if item == nil {
		t.Fatalf("不应删除非 automation 会话")
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
	sessionKey := protocol.BuildAgentSessionKey("agent-1", "automation", "dm", "scheduled-task:"+task.JobID+":"+runID, "")
	startedAt := time.Date(2026, 5, 21, 8, 0, 0, 0, time.UTC)
	if err = service.repository.InsertRunPending(context.Background(), automationstore.RunPendingInput{
		RunID:       runID,
		JobID:       task.JobID,
		OwnerUserID: task.OwnerUserID,
		TriggerKind: automationdomain.TriggerKindManual,
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
		t.Fatalf("repair_scheduled_task 应中断真实 DM 运行，实际 interrupts=%+v", interrupts)
	}
	if interrupts[0].SessionKey != sessionKey || interrupts[0].RoundID != roundID {
		t.Fatalf("DM 中断请求不正确: %+v", interrupts[0])
	}

	var runStatus string
	var runError sql.NullString
	if err = db.QueryRow(`SELECT status, error_message FROM automation_task_runs WHERE run_id = ?`, runID).Scan(&runStatus, &runError); err != nil {
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
	if err = db.QueryRow(`SELECT status FROM automation_task_runs WHERE run_id = ?`, runID).Scan(&runStatus); err != nil {
		t.Fatalf("再次读取 run 状态失败: %v", err)
	}
	if runStatus != automationdomain.RunStatusCancelled {
		t.Fatalf("迟到完成覆盖了 run 状态: %s", runStatus)
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
		TriggerKind: automationdomain.TriggerKindScheduled,
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
	if err = db.QueryRow(`SELECT status, error_message FROM automation_task_runs WHERE run_id = ?`, runID).Scan(&runStatus, &runError); err != nil {
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
	var recoverEvent *automationdomain.ScheduledTaskEvent
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
