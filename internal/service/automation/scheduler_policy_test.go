package automation

import (
	"context"
	"strings"
	"testing"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
)

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

func TestSchedulerLeaseReleaseRequiresOwner(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db, nil, nil, nil, permissionctx.NewContext(), &fakeWorkspaceReader{}, nil,
	)
	service.schedulerOwnerID = "scheduler-owner"
	now := time.Date(2026, 6, 11, 10, 0, 0, 0, time.UTC)
	if held, _, err := service.refreshSchedulerLease(context.Background(), now); err != nil || !held {
		t.Fatalf("获取租约失败: held=%v err=%v", held, err)
	}
	if err := service.repository.ReleaseSchedulerLease(context.Background(), automationSchedulerLeaseName, "other-owner"); err != nil {
		t.Fatalf("非 owner 释放租约失败: %v", err)
	}
	if held, _, err := service.refreshSchedulerLease(context.Background(), now.Add(11*time.Second)); err != nil || !held {
		t.Fatalf("非 owner 不应影响现有租约: held=%v err=%v", held, err)
	}
	if err := service.releaseSchedulerLease(context.Background()); err != nil {
		t.Fatalf("owner 释放租约失败: %v", err)
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

func TestSchedulerRunsMisfireOnceAndAdvancesFromNow(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	dm := &fakeDMRunner{permission: permission}
	service := NewService(
		config.Config{
			DatabaseDriver:                "sqlite",
			AutomationMisfirePolicy:       "run_once",
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
	task, err := service.CreateTask(context.Background(), schedulerPolicyTaskInput("补跑一次", true))
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	scheduledFor := now.Add(-5 * time.Minute)
	service.mu.Lock()
	service.jobStates[task.JobID].NextRunAt = &scheduledFor
	service.mu.Unlock()

	service.runDueOnce()
	waitFor(t, 2*time.Second, func() bool {
		runs, listErr := service.repository.ListRunsByJob(context.Background(), task.OwnerUserID, task.JobID)
		return listErr == nil && len(runs) == 1 && runs[0].Status == automationdomain.RunStatusSucceeded
	})
	runs, err := service.repository.ListRunsByJob(context.Background(), task.OwnerUserID, task.JobID)
	if err != nil {
		t.Fatalf("ListRunsByJob 失败: %v", err)
	}
	if len(runs) != 1 || runs[0].TriggerKind != automationdomain.TriggerKindMisfire {
		t.Fatalf("应只记录一次 misfire 补跑: %+v", runs)
	}
	current, err := service.GetTask(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("GetTask 失败: %v", err)
	}
	wantNext := now.Add(time.Hour)
	if current.NextRunAt == nil || !current.NextRunAt.Equal(wantNext) {
		t.Fatalf("补跑后应从当前时间推进调度: got=%v want=%s", current.NextRunAt, wantNext)
	}
	service.runDueOnce()
	if got := len(dm.Requests()); got != 1 {
		t.Fatalf("补跑后不应继续追赶历史窗口: requests=%d", got)
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
	events, err := service.ListTaskEvents(context.Background(), task.JobID, 10)
	if err != nil {
		t.Fatalf("ListTaskEvents 失败: %v", err)
	}
	foundExpire := false
	for _, event := range events {
		if event.Action == automationdomain.TaskEventActionExpire {
			foundExpire = true
			break
		}
	}
	if !foundExpire {
		t.Fatalf("应记录 expire 审计事件: %+v", events)
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

func TestRecurringJitterKeepsStablePhaseAfterScheduledTrigger(t *testing.T) {
	service := NewService(
		config.Config{
			DatabaseDriver:                   "sqlite",
			AutomationRecurringJitterSeconds: 900,
		},
		newAutomationTestDB(t),
		nil,
		nil,
		nil,
		permissionctx.NewContext(),
		&fakeWorkspaceReader{},
		nil,
	)
	job := automationdomain.ScheduledTask{
		JobID:   "stable-job",
		Enabled: true,
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
	}
	scheduledFor := time.Date(2026, 6, 11, 10, 7, 0, 0, time.UTC)
	next := service.nextRunAfterScheduledTrigger(job, automationdomain.TriggerKindScheduled, scheduledFor)
	want := scheduledFor.Add(time.Hour)
	if next == nil || !next.Equal(want) {
		t.Fatalf("循环任务应保留首次 jitter 建立的相位: got=%v want=%s", next, want)
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
