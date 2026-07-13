package automation

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"

	_ "modernc.org/sqlite"
)

func TestServiceAutoRetryDeliveryDeadLettersDisabledTask(t *testing.T) {
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
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "paused-delivery",
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
	if _, err = service.UpdateTaskStatus(context.Background(), task.JobID, false); err != nil {
		t.Fatalf("停用任务失败: %v", err)
	}

	runID := "run-disabled-delivery"
	dueAt := base.Add(time.Minute)
	deliveryError := "feishu temporary outage"
	if _, err = db.Exec(`
INSERT INTO automation_task_runs (
    run_id, job_id, owner_user_id, status, trigger_kind,
    delivery_mode, delivery_to, delivery_status, delivery_error,
    delivery_attempts, delivery_next_attempt_at, scheduled_for, finished_at,
    result_text, attempts
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		runID,
		task.JobID,
		task.OwnerUserID,
		automationdomain.RunStatusSucceeded,
		automationdomain.TriggerKindScheduled, automationdomain.DeliveryModeExplicit,
		"explicit:feishu:oc_group",
		automationdomain.DeliveryStatusFailed,
		deliveryError,
		1,
		dueAt,
		dueAt.Add(-time.Minute),
		dueAt,
		"日报正文",
		1,
	); err != nil {
		t.Fatalf("准备 disabled due delivery run 失败: %v", err)
	}

	service.nowFn = func() time.Time { return dueAt.Add(time.Second) }
	service.retryDueDeliveries(context.Background(), dueAt.Add(time.Second))

	runs, err := service.ListTaskRuns(context.Background(), task.JobID)
	if err != nil || len(runs) == 0 {
		t.Fatalf("读取自动重试跳过后的 run 失败: runs=%+v err=%v", runs, err)
	}
	updated := runs[0]
	if updated.RunID != runID ||
		updated.DeliveryStatus != automationdomain.DeliveryStatusFailed ||
		updated.DeliveryDeadLetterAt == nil ||
		updated.DeliveryNextAttemptAt != nil {
		t.Fatalf("停用任务的 due delivery 应进入死信并清理下一次重试: %+v", updated)
	}
	if updated.DeliveryAttempts != 1 {
		t.Fatalf("停用任务不应发生新的投递尝试，attempts=%d", updated.DeliveryAttempts)
	}
	if len(delivery.Calls()) != 0 {
		t.Fatalf("停用任务不应继续自动投递，calls=%+v", delivery.Calls())
	}
	dueRuns, err := service.repository.ListDueDeliveryRetries(context.Background(), dueAt.Add(2*time.Second), maxAutoDeliveryAttempts, deliveryRetryBatchLimit)
	if err != nil {
		t.Fatalf("重新读取 due delivery 失败: %v", err)
	}
	for _, dueRun := range dueRuns {
		if dueRun.RunID == runID {
			t.Fatalf("死信后的 disabled delivery 不应再次进入自动重试队列: %+v", dueRuns)
		}
	}

	events, err := service.ListTaskEvents(context.Background(), task.JobID, 20)
	if err != nil {
		t.Fatalf("读取自动重试跳过事件失败: %v", err)
	}
	for _, event := range events {
		if event.Action == automationdomain.TaskEventActionAutoRetryDelivery &&
			event.RunID == runID &&
			event.Detail["auto_retry_skipped_reason"] == "task_disabled" {
			return
		}
	}
	t.Fatalf("停用任务的自动重试跳过应写入审计事件: %+v", events)
}

func TestDeleteTaskDeadLettersPendingDeliveryRetries(t *testing.T) {
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
		Name:        "delete-with-failed-delivery",
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

	runID := "run-delete-delivery"
	nextAttemptAt := base.Add(10 * time.Minute)
	deliveryError := "feishu temporary outage"
	if _, err = db.Exec(`
INSERT INTO automation_task_runs (
    run_id, job_id, owner_user_id, status, trigger_kind,
    delivery_mode, delivery_to, delivery_status, delivery_error,
    delivery_attempts, delivery_next_attempt_at, scheduled_for, finished_at,
    result_text, attempts
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		runID,
		task.JobID,
		task.OwnerUserID,
		automationdomain.RunStatusSucceeded,
		automationdomain.TriggerKindScheduled, automationdomain.DeliveryModeExplicit,
		"explicit:feishu:oc_group",
		automationdomain.DeliveryStatusFailed,
		deliveryError,
		1,
		nextAttemptAt,
		base,
		base.Add(time.Minute),
		"日报正文",
		1,
	); err != nil {
		t.Fatalf("准备待补投递 run 失败: %v", err)
	}

	deletedAt := base.Add(time.Minute)
	service.nowFn = func() time.Time { return deletedAt }
	if _, err = service.DeleteTask(context.Background(), task.JobID); err != nil {
		t.Fatalf("DeleteTask 失败: %v", err)
	}

	runs, err := service.ListTaskRuns(context.Background(), task.JobID)
	if err != nil || len(runs) == 0 {
		t.Fatalf("删除后读取 run ledger 失败: runs=%+v err=%v", runs, err)
	}
	updated := runs[0]
	if updated.RunID != runID ||
		updated.DeliveryStatus != automationdomain.DeliveryStatusFailed ||
		updated.DeliveryDeadLetterAt == nil ||
		updated.DeliveryNextAttemptAt != nil ||
		updated.DeliveryError == nil ||
		!strings.Contains(*updated.DeliveryError, "deleted") {
		t.Fatalf("删除任务应立即把待补投递 run 标记为死信: %+v", updated)
	}
	if updated.DeliveryAttempts != 1 {
		t.Fatalf("删除任务不应新增投递尝试，attempts=%d", updated.DeliveryAttempts)
	}
	dueRuns, err := service.repository.ListDueDeliveryRetries(context.Background(), deletedAt.Add(time.Hour), maxAutoDeliveryAttempts, deliveryRetryBatchLimit)
	if err != nil {
		t.Fatalf("读取 due delivery 失败: %v", err)
	}
	for _, dueRun := range dueRuns {
		if dueRun.RunID == runID {
			t.Fatalf("删除任务后的死信 run 不应进入自动重试队列: %+v", dueRuns)
		}
	}
	if len(delivery.Calls()) != 0 {
		t.Fatalf("删除任务不应触发投递，calls=%+v", delivery.Calls())
	}
	report, err := service.GetDailyReport(context.Background(), automationdomain.ScheduledTaskDailyReportInput{
		Date:     "2026-05-21",
		Timezone: "UTC",
		JobID:    task.JobID,
	})
	if err != nil {
		t.Fatalf("删除任务后读取日报失败: %v", err)
	}
	if len(report.Tasks) != 1 {
		t.Fatalf("删除任务日报应返回一条任务明细: %+v", report)
	}
	dailyTask := report.Tasks[0]
	if !dailyTask.Deleted ||
		!containsString(dailyTask.Signals, "deleted") ||
		!containsString(dailyTask.Signals, "delivery_attention") ||
		!containsString(dailyTask.DeliveryDeadLetterRunIDs, runID) ||
		containsString(dailyTask.ManualRedeliveryRunIDs, runID) ||
		!containsString(dailyTask.SuggestedTools, "inspect_scheduled_task") ||
		containsString(dailyTask.SuggestedTools, "repair_scheduled_task") {
		t.Fatalf("删除任务日报应保留失败信号但不建议不可执行补投递: %+v", dailyTask)
	}

	events, err := service.ListTaskEvents(context.Background(), task.JobID, 20)
	if err != nil {
		t.Fatalf("删除后读取事件失败: %v", err)
	}
	for _, event := range events {
		if event.Action != automationdomain.TaskEventActionDelete {
			continue
		}
		values, ok := event.Detail["dead_lettered_delivery_run_ids"].([]any)
		if !ok || len(values) != 1 || values[0] != runID {
			t.Fatalf("delete 事件应记录被死信的投递 run: %+v", event.Detail)
		}
		return
	}
	t.Fatalf("删除任务应写入 delete 事件: %+v", events)
}

func TestServiceRetryRunDeliveryMarksDeadLetterAfterMaxAttempts(t *testing.T) {
	db := newAutomationTestDB(t)
	delivery := &fakeDeliveryRouter{err: fmt.Errorf("feishu temporary outage")}
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
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "dead-letter",
		AgentID:     "agent-1",
		Instruction: "send report",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
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
	if _, err = db.Exec(`
INSERT INTO automation_task_runs (
    run_id, job_id, owner_user_id, status, trigger_kind,
    delivery_mode, delivery_to, delivery_status, delivery_attempts,
    result_text, attempts
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"run-dead",
		task.JobID,
		task.OwnerUserID,
		automationdomain.RunStatusSucceeded,
		automationdomain.TriggerKindScheduled, automationdomain.DeliveryModeExplicit,
		"feishu:oc_group",
		automationdomain.DeliveryStatusFailed,
		maxAutoDeliveryAttempts-1,
		"日报正文",
		1,
	); err != nil {
		t.Fatalf("准备 run 失败: %v", err)
	}

	run, err := service.RetryRunDelivery(context.Background(), task.JobID, "run-dead")
	if err != nil {
		t.Fatalf("RetryRunDelivery 失败: %v", err)
	}
	if run.DeliveryStatus != automationdomain.DeliveryStatusFailed || run.DeliveryDeadLetterAt == nil || run.DeliveryNextAttemptAt != nil {
		t.Fatalf("达到最大重试后应进入死信且不再安排自动重试: %+v", run)
	}
	if run.DeliveryAttempts != maxAutoDeliveryAttempts {
		t.Fatalf("delivery_attempts = %d, 期望 %d", run.DeliveryAttempts, maxAutoDeliveryAttempts)
	}
}

func TestServiceRetryRunDeliveryRejectsAlreadyDeliveredRun(t *testing.T) {
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
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "delivered",
		AgentID:     "agent-1",
		Instruction: "send report",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
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
	if _, err = db.Exec(`
INSERT INTO automation_task_runs (
    run_id, job_id, owner_user_id, status, trigger_kind,
    delivery_mode, delivery_to, delivery_status, delivery_attempts,
    result_text, attempts
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"run-delivered",
		task.JobID,
		task.OwnerUserID,
		automationdomain.RunStatusSucceeded,
		automationdomain.TriggerKindScheduled, automationdomain.DeliveryModeExplicit,
		"feishu:oc_group",
		automationdomain.DeliveryStatusSucceeded,
		1,
		"日报正文",
		1,
	); err != nil {
		t.Fatalf("准备 run 失败: %v", err)
	}

	_, err = service.RetryRunDelivery(context.Background(), task.JobID, "run-delivered")
	if err == nil || !strings.Contains(err.Error(), "delivery_status must be failed") {
		t.Fatalf("期望拒绝重复补投已成功 run，实际 err=%v", err)
	}
	if calls := delivery.Calls(); len(calls) != 0 {
		t.Fatalf("不应重复调用投递，calls=%+v", calls)
	}
	var attempts int
	if err = db.QueryRow(`SELECT delivery_attempts FROM automation_task_runs WHERE run_id = ?`, "run-delivered").Scan(&attempts); err != nil {
		t.Fatalf("读取 delivery_attempts 失败: %v", err)
	}
	if attempts != 1 {
		t.Fatalf("delivery_attempts 不应变化，实际 %d", attempts)
	}
}
