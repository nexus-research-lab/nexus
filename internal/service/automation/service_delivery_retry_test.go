package automation

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"

	_ "modernc.org/sqlite"
)

func TestServiceDeliveryFailureDoesNotFailExecutionAndCanRetry(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	delivery := &fakeDeliveryRouter{err: fmt.Errorf("feishu send message failed: bad chat_id")}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		&fakeDMRunner{permission: permission, resultText: "今日新闻摘要"},
		nil,
		permission,
		&fakeWorkspaceReader{},
		delivery,
	)
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "news",
		AgentID:     "agent-1",
		Instruction: "search news",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetNamed, NamedSessionKey: "news"},
		Delivery: automationdomain.DeliveryTarget{
			Mode:    automationdomain.DeliveryModeExplicit,
			Channel: "feishu",
			To:      "oc_bad",
		},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	if _, err = service.RunTaskNow(context.Background(), task.JobID); err != nil {
		t.Fatalf("RunTaskNow 失败: %v", err)
	}
	waitFor(t, 2*time.Second, func() bool {
		items, listErr := service.ListTaskRuns(context.Background(), task.JobID)
		return listErr == nil && len(items) > 0 && items[0].DeliveryStatus == automationdomain.DeliveryStatusFailed
	})

	runs, err := service.ListTaskRuns(context.Background(), task.JobID)
	if err != nil || len(runs) == 0 {
		t.Fatalf("读取 run 失败: runs=%+v err=%v", runs, err)
	}
	if runs[0].Status != automationdomain.RunStatusSucceeded {
		t.Fatalf("投递失败不应把执行状态改成 failed: %+v", runs[0])
	}
	if runs[0].DeliveryError == nil || !strings.Contains(*runs[0].DeliveryError, "bad chat_id") {
		t.Fatalf("delivery_error 未记录失败原因: %+v", runs[0])
	}
	if runs[0].DeliveryAttempts != 1 {
		t.Fatalf("delivery_attempts = %d, 期望 1", runs[0].DeliveryAttempts)
	}
	if runs[0].DeliveryNextAttemptAt == nil || runs[0].DeliveryDeadLetterAt != nil {
		t.Fatalf("投递失败后应安排自动重试且不进入死信: %+v", runs[0])
	}
	updatedTask, err := service.GetTask(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("读取任务失败: %v", err)
	}
	if updatedTask.LastRunStatus != automationdomain.RunStatusSucceeded || updatedTask.FailureStreak != 0 {
		t.Fatalf("投递失败不应触发任务级执行失败退避: %+v", updatedTask)
	}
	if updatedTask.LastDeliveryStatus != automationdomain.DeliveryStatusFailed {
		t.Fatalf("last_delivery_status 未记录失败: %+v", updatedTask)
	}

	updatedDelivery := automationdomain.DeliveryTarget{
		Mode:    automationdomain.DeliveryModeExplicit,
		Channel: "feishu",
		To:      "oc_good",
	}
	if _, err = service.UpdateTask(context.Background(), task.JobID, automationdomain.UpdateJobInput{Delivery: &updatedDelivery}); err != nil {
		t.Fatalf("修正投递目标失败: %v", err)
	}
	delivery.err = nil
	dueAt := runs[0].DeliveryNextAttemptAt.UTC().Add(time.Second)
	service.nowFn = func() time.Time { return dueAt }
	service.retryDueDeliveries(context.Background(), dueAt)
	redeliveredRuns, err := service.ListTaskRuns(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("读取自动重试后的 run 失败: %v", err)
	}
	redelivered := redeliveredRuns[0]
	if redelivered.DeliveryStatus != automationdomain.DeliveryStatusSucceeded || redelivered.DeliveryError != nil || redelivered.DeliveredAt == nil {
		t.Fatalf("重试投递后状态不正确: %+v", redelivered)
	}
	if redelivered.DeliveryTo != "explicit:feishu:oc_good" {
		t.Fatalf("重试投递应记录修正后的目标，实际 delivery_to=%q", redelivered.DeliveryTo)
	}
	calls := delivery.Calls()
	if len(calls) < 2 || calls[len(calls)-1].To != "oc_good" {
		t.Fatalf("重试投递应使用修正后的目标，calls=%+v", calls)
	}
	if redelivered.DeliveryAttempts != 2 {
		t.Fatalf("重试后 delivery_attempts = %d, 期望 2", redelivered.DeliveryAttempts)
	}
	if redelivered.DeliveryNextAttemptAt != nil || redelivered.DeliveryDeadLetterAt != nil {
		t.Fatalf("投递成功后应清理重试/死信时间: %+v", redelivered)
	}
	events, err := service.ListTaskEvents(context.Background(), task.JobID, 20)
	if err != nil {
		t.Fatalf("读取自动重试审计失败: %v", err)
	}
	var autoRetryEvent *automationdomain.CronTaskEvent
	for index := range events {
		if events[index].Action == automationdomain.TaskEventActionAutoRetryDelivery {
			autoRetryEvent = &events[index]
			break
		}
	}
	if autoRetryEvent == nil {
		t.Fatalf("自动投递重试应写入审计事件: %+v", events)
	}
	if autoRetryEvent.RunID != runs[0].RunID || autoRetryEvent.ActorUserID != authctx.SystemUserID {
		t.Fatalf("自动重试事件应关联 run 且 actor 为系统: %+v", autoRetryEvent)
	}
	if autoRetryEvent.Detail["delivery_status"] != automationdomain.DeliveryStatusSucceeded ||
		autoRetryEvent.Detail["delivery_to"] != "explicit:feishu:oc_good" {
		t.Fatalf("自动重试事件应记录投递结果和实际目标: %+v", autoRetryEvent.Detail)
	}
	if attempts, ok := autoRetryEvent.Detail["delivery_attempts"].(float64); !ok || int(attempts) != 2 {
		t.Fatalf("自动重试事件应记录 attempts=2: %+v", autoRetryEvent.Detail)
	}
}

func TestServiceRunDueOnceRetriesDueDelivery(t *testing.T) {
	db := newAutomationTestDB(t)
	delivery := &fakeDeliveryRouter{}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		nil,
		nil,
		nil,
		&fakeWorkspaceReader{},
		delivery,
	)
	base := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
	service.nowFn = func() time.Time { return base }
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "auto-redelivery",
		AgentID:     "agent-1",
		Instruction: "send report",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "UTC",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetNamed, NamedSessionKey: "reports"},
		Delivery: automationdomain.DeliveryTarget{
			Mode:    automationdomain.DeliveryModeExplicit,
			Channel: "feishu",
			To:      "oc_group",
		},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	runID := "run-due-delivery"
	dueAt := base.Add(5 * time.Minute)
	scheduledFor := base.Add(-time.Minute)
	deliveryError := "feishu temporary outage"
	if _, err = db.Exec(`
INSERT INTO automation_cron_runs (
    run_id, job_id, owner_user_id, status, trigger_kind,
    delivery_mode, delivery_to, delivery_status, delivery_error,
    delivery_attempts, delivery_next_attempt_at, scheduled_for, finished_at,
    result_text, attempts
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		runID,
		task.JobID,
		task.OwnerUserID,
		automationdomain.RunStatusSucceeded,
		"cron",
		automationdomain.DeliveryModeExplicit,
		"explicit:feishu:oc_old",
		automationdomain.DeliveryStatusFailed,
		deliveryError,
		1,
		dueAt,
		scheduledFor,
		scheduledFor.Add(time.Minute),
		"日报正文",
		1,
	); err != nil {
		t.Fatalf("准备 due delivery run 失败: %v", err)
	}

	service.nowFn = func() time.Time { return dueAt.Add(time.Second) }
	service.runDueOnce()

	waitFor(t, 2*time.Second, func() bool {
		runs, listErr := service.ListTaskRuns(context.Background(), task.JobID)
		return listErr == nil &&
			len(runs) > 0 &&
			runs[0].RunID == runID &&
			runs[0].DeliveryStatus == automationdomain.DeliveryStatusSucceeded
	})
	runs, err := service.ListTaskRuns(context.Background(), task.JobID)
	if err != nil || len(runs) == 0 {
		t.Fatalf("读取调度自动重试后的 run 失败: runs=%+v err=%v", runs, err)
	}
	redelivered := runs[0]
	if redelivered.DeliveryAttempts != 2 || redelivered.DeliveryError != nil || redelivered.DeliveryNextAttemptAt != nil {
		t.Fatalf("调度 tick 自动重试后状态不正确: %+v", redelivered)
	}
	if redelivered.DeliveryTo != "explicit:feishu:oc_group" {
		t.Fatalf("自动重试应使用任务当前投递目标，实际 delivery_to=%q", redelivered.DeliveryTo)
	}
	calls := delivery.Calls()
	if len(calls) != 1 || calls[0].To != "oc_group" {
		t.Fatalf("调度 tick 应自动投递到当前目标，calls=%+v", calls)
	}
	events, err := service.ListTaskEvents(context.Background(), task.JobID, 20)
	if err != nil {
		t.Fatalf("读取自动重试事件失败: %v", err)
	}
	for _, event := range events {
		if event.Action == automationdomain.TaskEventActionAutoRetryDelivery && event.RunID == runID {
			return
		}
	}
	t.Fatalf("调度 tick 自动重试应写入审计事件: %+v", events)
}
