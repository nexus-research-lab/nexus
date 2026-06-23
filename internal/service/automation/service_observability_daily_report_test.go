package automation

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"

	_ "modernc.org/sqlite"
)

func TestServiceDailyReportAggregatesExecutionAndDelivery(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(
		config.Config{DatabaseDriver: "sqlite", DefaultTimezone: "UTC"},
		db,
		nil,
		nil,
		nil,
		nil,
		&fakeWorkspaceReader{},
		nil,
	)
	task, err := service.CreateTask(context.Background(), protocol.CreateJobInput{
		Name:        "新闻日报",
		AgentID:     "agent-1",
		Instruction: "搜索今天的重要新闻",
		Schedule: protocol.Schedule{
			Kind:            protocol.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "UTC",
		},
		SessionTarget: protocol.SessionTarget{Kind: protocol.SessionTargetIsolated},
		Delivery:      protocol.DeliveryTarget{Mode: protocol.DeliveryModeExplicit, Channel: "feishu", To: "oc_group"},
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	okAt := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
	failedAt := time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC)
	executionFailedAt := time.Date(2026, 5, 21, 11, 0, 0, 0, time.UTC)
	pendingAt := time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC)
	skippedAt := time.Date(2026, 5, 21, 13, 0, 0, 0, time.UTC)
	oldAt := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	deliveryError := "feishu chat not found"
	executionError := "news search failed"
	deadLetterAt := failedAt.Add(30 * time.Minute)
	for _, run := range []struct {
		runID                string
		scheduledFor         time.Time
		status               string
		errorMessage         *string
		deliveryStatus       string
		deliveryError        *string
		deliveryDeadLetterAt *time.Time
	}{
		{runID: "run-ok", scheduledFor: okAt, status: protocol.RunStatusSucceeded, deliveryStatus: protocol.DeliveryStatusSucceeded},
		{runID: "run-delivery-failed", scheduledFor: failedAt, status: protocol.RunStatusSucceeded, deliveryStatus: protocol.DeliveryStatusFailed, deliveryError: &deliveryError, deliveryDeadLetterAt: &deadLetterAt},
		{runID: "run-execution-failed", scheduledFor: executionFailedAt, status: protocol.RunStatusFailed, errorMessage: &executionError, deliveryStatus: protocol.DeliveryStatusNotAttempted},
		{runID: "run-delivery-skipped", scheduledFor: skippedAt, status: protocol.RunStatusSucceeded, deliveryStatus: protocol.DeliveryStatusSkipped},
		{runID: "run-old", scheduledFor: oldAt, status: protocol.RunStatusSucceeded, deliveryStatus: protocol.DeliveryStatusSucceeded},
	} {
		if err = service.repository.InsertRunPending(context.Background(), automationstore.RunPendingInput{
			RunID:        run.runID,
			JobID:        task.JobID,
			OwnerUserID:  task.OwnerUserID,
			ScheduledFor: &run.scheduledFor,
			TriggerKind:  "cron",
			DeliveryMode: protocol.DeliveryModeExplicit,
			DeliveryTo:   "feishu:oc_group",
		}); err != nil {
			t.Fatalf("插入 run %s 失败: %v", run.runID, err)
		}
		if err = service.repository.MarkRunFinished(context.Background(), automationstore.RunFinishInput{
			RunID:                run.runID,
			Status:               run.status,
			FinishedAt:           run.scheduledFor.Add(time.Minute),
			ErrorMessage:         run.errorMessage,
			DeliveryStatus:       run.deliveryStatus,
			DeliveryError:        run.deliveryError,
			DeliveryAttempted:    true,
			DeliveryDeadLetterAt: run.deliveryDeadLetterAt,
		}); err != nil {
			t.Fatalf("结束 run %s 失败: %v", run.runID, err)
		}
	}
	if err = service.repository.InsertRunPending(context.Background(), automationstore.RunPendingInput{
		RunID:        "run-delivery-pending",
		JobID:        task.JobID,
		OwnerUserID:  task.OwnerUserID,
		ScheduledFor: &pendingAt,
		TriggerKind:  "cron",
		DeliveryMode: protocol.DeliveryModeExplicit,
		DeliveryTo:   "feishu:oc_group",
		Status:       protocol.RunStatusPending,
	}); err != nil {
		t.Fatalf("插入 pending run 失败: %v", err)
	}

	report, err := service.GetDailyReport(context.Background(), protocol.CronDailyReportInput{
		Date:     "2026-05-21",
		Timezone: "UTC",
	})
	if err != nil {
		t.Fatalf("GetDailyReport 失败: %v", err)
	}
	if report.Totals.TaskCount != 1 || report.Totals.RunCount != 5 {
		t.Fatalf("日报总数不正确: %+v", report.Totals)
	}
	if report.Totals.DeliveredRunCount != 1 ||
		report.Totals.FailedRunCount != 1 ||
		report.Totals.DeliveryFailedRunCount != 1 ||
		report.Totals.DeliveryPendingRunCount != 1 ||
		report.Totals.DeliverySkippedRunCount != 1 ||
		report.Totals.DeliveryNotNeededCount != 0 {
		t.Fatalf("日报投递统计不正确: %+v", report.Totals)
	}
	if len(report.Tasks) != 1 || len(report.Tasks[0].Runs) != 5 {
		t.Fatalf("日报任务明细不正确: %+v", report.Tasks)
	}
	dailyTask := report.Tasks[0]
	if !slices.Contains(dailyTask.Signals, "delivery_attention") ||
		!slices.Contains(dailyTask.SuggestedTools, "retry_scheduled_task_delivery") {
		t.Fatalf("日报任务应直接提示投递补救动作: %+v", dailyTask)
	}
	if !slices.Contains(dailyTask.Signals, "recent_execution_failed") ||
		!slices.Contains(dailyTask.SuggestedTools, "update_scheduled_task") ||
		!slices.Contains(dailyTask.SuggestedTools, "run_scheduled_task") {
		t.Fatalf("日报任务应直接提示执行失败补救动作: %+v", dailyTask)
	}
	if !slices.Contains(dailyTask.ManualRedeliveryRunIDs, "run-delivery-failed") {
		t.Fatalf("日报任务应直接给出可补投 run_id: %+v", dailyTask)
	}
	if !slices.Contains(dailyTask.DeliveryDeadLetterRunIDs, "run-delivery-failed") {
		t.Fatalf("日报任务应直接给出死信 run_id: %+v", dailyTask)
	}
	if !slices.Contains(dailyTask.ExecutionFailedRunIDs, "run-execution-failed") ||
		!slices.Contains(dailyTask.DeliveryPendingRunIDs, "run-delivery-pending") ||
		!slices.Contains(dailyTask.DeliverySkippedRunIDs, "run-delivery-skipped") {
		t.Fatalf("日报任务应直接给出异常 run_id 摘要: %+v", dailyTask)
	}
	if dailyTask.LatestExecutionError == nil || *dailyTask.LatestExecutionError != executionError ||
		dailyTask.LatestDeliveryError == nil || *dailyTask.LatestDeliveryError != deliveryError {
		t.Fatalf("日报任务应直接暴露最近错误: %+v", dailyTask)
	}
}

func TestServiceDailyReportAndRunsSurviveDeletedTask(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(
		config.Config{DatabaseDriver: "sqlite", DefaultTimezone: "UTC"},
		db,
		nil,
		nil,
		nil,
		nil,
		&fakeWorkspaceReader{},
		nil,
	)
	task, err := service.CreateTask(context.Background(), protocol.CreateJobInput{
		Name:        "删除前日报",
		AgentID:     "agent-1",
		Instruction: "搜索新闻并发送",
		Schedule: protocol.Schedule{
			Kind:            protocol.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "UTC",
		},
		SessionTarget: protocol.SessionTarget{Kind: protocol.SessionTargetIsolated},
		Delivery:      protocol.DeliveryTarget{Mode: protocol.DeliveryModeExplicit, Channel: "feishu", To: "oc_group"},
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	scheduledFor := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
	if err = service.repository.InsertRunPending(context.Background(), automationstore.RunPendingInput{
		RunID:        "run-before-delete",
		JobID:        task.JobID,
		OwnerUserID:  task.OwnerUserID,
		ScheduledFor: &scheduledFor,
		TriggerKind:  "cron",
		DeliveryMode: protocol.DeliveryModeExplicit,
		DeliveryTo:   "feishu:oc_group",
	}); err != nil {
		t.Fatalf("插入删除前 run 失败: %v", err)
	}
	if err = service.repository.MarkRunFinished(context.Background(), automationstore.RunFinishInput{
		RunID:             "run-before-delete",
		Status:            protocol.RunStatusSucceeded,
		FinishedAt:        scheduledFor.Add(time.Minute),
		DeliveryStatus:    protocol.DeliveryStatusSucceeded,
		DeliveryAttempted: true,
	}); err != nil {
		t.Fatalf("结束删除前 run 失败: %v", err)
	}
	if _, err = service.DeleteTask(context.Background(), task.JobID); err != nil {
		t.Fatalf("DeleteTask 失败: %v", err)
	}

	runs, err := service.ListTaskRuns(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("删除任务后仍应可读取 run ledger: %v", err)
	}
	if len(runs) != 1 || runs[0].RunID != "run-before-delete" {
		t.Fatalf("删除任务后 run ledger 不正确: %+v", runs)
	}
	report, err := service.GetDailyReport(context.Background(), protocol.CronDailyReportInput{
		Date:     "2026-05-21",
		Timezone: "UTC",
		JobID:    task.JobID,
	})
	if err != nil {
		t.Fatalf("删除任务后 GetDailyReport 失败: %v", err)
	}
	if report.Totals.TaskCount != 1 || report.Totals.RunCount != 1 || report.Totals.DeliveredRunCount != 1 {
		t.Fatalf("删除任务日报统计不正确: %+v", report.Totals)
	}
	if len(report.Tasks) != 1 {
		t.Fatalf("删除任务日报明细数量不正确: %+v", report.Tasks)
	}
	dailyTask := report.Tasks[0]
	if !dailyTask.Deleted || dailyTask.Name != "删除前日报" || dailyTask.AgentID != "agent-1" {
		t.Fatalf("删除任务日报未保留任务快照: %+v", dailyTask)
	}
	if !slices.Contains(dailyTask.Signals, "deleted") ||
		!slices.Contains(dailyTask.SuggestedTools, "get_scheduled_task_events") {
		t.Fatalf("删除任务日报应提示查询审计事件: %+v", dailyTask)
	}
	if len(dailyTask.Runs) != 1 || dailyTask.Runs[0].RunID != "run-before-delete" {
		t.Fatalf("删除任务日报未保留 run 明细: %+v", dailyTask.Runs)
	}
}

func TestServiceDailyReportIncludesRecoveryRunID(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(
		config.Config{DatabaseDriver: "sqlite", DefaultTimezone: "UTC"},
		db,
		nil,
		nil,
		nil,
		nil,
		&fakeWorkspaceReader{},
		nil,
	)
	task, err := service.CreateTask(context.Background(), protocol.CreateJobInput{
		Name:        "运行中日报任务",
		AgentID:     "agent-1",
		Instruction: "检查运行态",
		Schedule: protocol.Schedule{
			Kind:            protocol.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "UTC",
		},
		SessionTarget: protocol.SessionTarget{Kind: protocol.SessionTargetIsolated},
		Delivery:      protocol.DeliveryTarget{Mode: protocol.DeliveryModeNone},
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	runID := "run-running-daily-report"
	startedAt := time.Date(2026, 5, 21, 8, 0, 0, 0, time.UTC)
	if err = service.repository.InsertRunPending(context.Background(), automationstore.RunPendingInput{
		RunID:        runID,
		JobID:        task.JobID,
		OwnerUserID:  task.OwnerUserID,
		ScheduledFor: &startedAt,
		TriggerKind:  "cron",
		Status:       protocol.RunStatusPending,
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

	report, err := service.GetDailyReport(context.Background(), protocol.CronDailyReportInput{
		Date:     "2026-05-21",
		Timezone: "UTC",
	})
	if err != nil {
		t.Fatalf("GetDailyReport 失败: %v", err)
	}
	if report.Totals.RunningTaskCount != 1 || len(report.Tasks) != 1 {
		t.Fatalf("日报运行中统计不正确: %+v", report)
	}
	dailyTask := report.Tasks[0]
	if dailyTask.RecoveryRunID != runID {
		t.Fatalf("日报任务应直接给出可恢复 run_id: %+v", dailyTask)
	}
	if !slices.Contains(dailyTask.Signals, "running") ||
		!slices.Contains(dailyTask.SuggestedTools, "recover_scheduled_task") {
		t.Fatalf("日报任务应直接提示恢复动作: %+v", dailyTask)
	}
}
