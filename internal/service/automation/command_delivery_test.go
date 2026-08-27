package automation

import (
	"slices"
	"strings"
	"testing"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
)

func TestAutomationCommandCreateFromPairedFeishuDMDefaultsDeliveryToCurrentSession(t *testing.T) {
	fixture := newAutomationCommandFixture(t, "今日新闻摘要")
	ownerCtx := automationCommandTestOwnerContext(fixture.ServerContext.OwnerUserID)
	now := time.Date(2026, 5, 22, 9, 0, 0, 0, time.UTC)
	fixture.Service.nowFn = func() time.Time { return now }

	feishuSessionKey := protocol.BuildAgentSessionKey(
		fixture.ServerContext.AgentID,
		protocol.SessionChannelFeishuSegment,
		protocol.RoomTypeDM,
		"oc_group_123",
		"",
	)
	sctx := fixture.ServerContext
	sctx.SessionKey = feishuSessionKey
	sctx.SessionLabel = "飞书私聊 oc_group_123"
	sctx.SourceContextType = "agent_paired"
	if _, err := fixture.Router.RememberSessionRoute(ownerCtx, sctx.AgentID, feishuSessionKey, channels.DeliveryTarget{
		Mode:    channels.DeliveryModeExplicit,
		Channel: channels.ChannelTypeFeishu,
		To:      "oc_group_123",
	}); err != nil {
		t.Fatalf("记录飞书会话回投路由失败: %v", err)
	}

	createResult, isError := callAutomationCommand(t, fixture.Service, sctx, "create_scheduled_task", map[string]any{
		"name":              "飞书群每日新闻",
		"instruction":       "每天搜索重要新闻并发到这个飞书群",
		"execution_mode":    "dedicated",
		"named_session_key": "feishu-group-news",
		"reply_mode":        "channel",
		"schedule": map[string]any{
			"kind":       "daily",
			"daily_time": "09:00",
			"timezone":   "Asia/Shanghai",
		},
	})
	if isError {
		t.Fatalf("create_scheduled_task 不应失败: %s", automationCommandText(t, createResult))
	}
	created := decodeAutomationCommandJSON[automationdomain.ScheduledTask](t, createResult)
	if created.Delivery.Mode != automationdomain.DeliveryModeLast ||
		created.Delivery.SessionKey != feishuSessionKey {
		t.Fatalf("飞书群上下文创建任务应默认回投当前群: %+v", created.Delivery)
	}
	if created.Source.SessionKey != feishuSessionKey || created.Source.SessionLabel != "飞书私聊 oc_group_123" {
		t.Fatalf("任务来源应保留飞书私聊会话上下文: %+v", created.Source)
	}

	runResult, isError := callAutomationCommand(t, fixture.Service, sctx, "run_scheduled_task", map[string]any{
		"query": "飞书群每日新闻",
	})
	if isError {
		t.Fatalf("run_scheduled_task by query 不应失败: %s", automationCommandText(t, runResult))
	}
	runNow := decodeAutomationCommandJSON[automationdomain.ExecutionResult](t, runResult)
	if runNow.RunID == nil || *runNow.RunID == "" {
		t.Fatalf("run_scheduled_task 应返回 run_id: %+v", runNow)
	}
	runID := *runNow.RunID

	waitFor(t, 2*time.Second, func() bool {
		runs, err := fixture.Service.ListTaskRuns(ownerCtx, created.JobID)
		return err == nil && len(runs) > 0 && runs[0].RunID == runID &&
			runs[0].DeliveryStatus == automationdomain.DeliveryStatusRetrying &&
			runs[0].DeliveryError != nil
	})
	runs, err := fixture.Service.ListTaskRuns(ownerCtx, created.JobID)
	if err != nil || len(runs) == 0 {
		t.Fatalf("读取飞书群默认投递 run 失败: runs=%+v err=%v", runs, err)
	}
	run := runs[0]
	if run.Status != automationdomain.RunStatusSucceeded ||
		run.DeliveryTo != "explicit:feishu:oc_group_123" ||
		run.DeliveryAttempts != 1 ||
		run.DeliveryNextAttemptAt != nil ||
		run.DeliveryError == nil ||
		!strings.Contains(*run.DeliveryError, "feishu") {
		t.Fatalf("飞书 router 返回后结果未知时应保留待人工核对 ledger: %+v", run)
	}
}

func TestAutomationCommandReportAndRetryFailedDeliveryToCurrentSession(t *testing.T) {
	fixture := newAutomationCommandFixture(t, "今日新闻摘要")
	ownerCtx := automationCommandTestOwnerContext(fixture.ServerContext.OwnerUserID)
	now := time.Date(2026, 5, 22, 9, 0, 0, 0, time.UTC)
	fixture.Service.nowFn = func() time.Time { return now }
	sctx := fixture.ServerContext
	sctx.SessionKey = protocol.BuildAgentSessionKey(
		fixture.ServerContext.AgentID,
		protocol.SessionChannelFeishuSegment,
		protocol.RoomTypeDM,
		"oc_missing_group",
		"",
	)
	sctx.SessionLabel = "飞书私聊 oc_missing_group"
	sctx.SourceContextType = "agent_paired"

	createResult, isError := callAutomationCommand(t, fixture.Service, sctx, "create_scheduled_task", map[string]any{
		"name":              "飞书新闻投递",
		"instruction":       "搜索新闻并投递到飞书群",
		"execution_mode":    "dedicated",
		"named_session_key": "feishu-news",
		"reply_mode":        "channel",
		"reply_channel":     protocol.SessionChannelFeishu,
		"reply_to":          "oc_missing_group",
		"schedule": map[string]any{
			"kind":       "daily",
			"daily_time": "09:00",
			"timezone":   "Asia/Shanghai",
		},
	})
	if isError {
		t.Fatalf("create_scheduled_task 不应失败: %s", automationCommandText(t, createResult))
	}
	created := decodeAutomationCommandJSON[automationdomain.ScheduledTask](t, createResult)

	runResult, isError := callAutomationCommand(t, fixture.Service, sctx, "run_scheduled_task", map[string]any{
		"query": "飞书新闻投递",
	})
	if isError {
		t.Fatalf("run_scheduled_task by query 不应失败: %s", automationCommandText(t, runResult))
	}
	runNow := decodeAutomationCommandJSON[automationdomain.ExecutionResult](t, runResult)
	if runNow.RunID == nil || *runNow.RunID == "" {
		t.Fatalf("run_scheduled_task 应返回 run_id: %+v", runNow)
	}
	runID := *runNow.RunID

	waitFor(t, 2*time.Second, func() bool {
		runs, err := fixture.Service.ListTaskRuns(ownerCtx, created.JobID)
		return err == nil && len(runs) > 0 && runs[0].RunID == runID &&
			runs[0].DeliveryStatus == automationdomain.DeliveryStatusRetrying &&
			runs[0].DeliveryError != nil
	})
	failedRuns, err := fixture.Service.ListTaskRuns(ownerCtx, created.JobID)
	if err != nil || len(failedRuns) == 0 {
		t.Fatalf("读取飞书投递失败 run 失败: runs=%+v err=%v", failedRuns, err)
	}
	failedRun := failedRuns[0]
	if failedRun.Status != automationdomain.RunStatusSucceeded || failedRun.DeliveryError == nil {
		t.Fatalf("飞书发送失败不应影响执行成功，但应记录 delivery_error: %+v", failedRun)
	}
	if failedRun.DeliveryAttempts != 1 || failedRun.DeliveryNextAttemptAt != nil {
		t.Fatalf("飞书发送结果未知应记录尝试但不得自动重试: %+v", failedRun)
	}

	reportResult, isError := callAutomationCommand(t, fixture.Service, sctx, "get_scheduled_task_report", map[string]any{
		"query":    "飞书新闻投递",
		"date":     "2026-05-22",
		"timezone": "UTC",
	})
	if isError {
		t.Fatalf("get_scheduled_task_report by query 不应失败: %s", automationCommandText(t, reportResult))
	}
	report := decodeAutomationCommandJSON[automationdomain.ScheduledTaskDailyReport](t, reportResult)
	if len(report.Tasks) != 1 {
		t.Fatalf("日报应定位到唯一任务: %+v", report)
	}
	taskReport := report.Tasks[0]
	if !slices.Contains(taskReport.Signals, "delivery_unverified") ||
		!slices.Contains(taskReport.SuggestedTools, runtimeAutomationInspectSuggestion) ||
		!slices.Contains(taskReport.DeliveryUnverifiedRunIDs, runID) ||
		slices.Contains(taskReport.ManualRedeliveryRunIDs, runID) {
		t.Fatalf("日报应指出结果未知且先核对、不得建议普通补发: %+v", taskReport)
	}

	recipientSessionKey := fixture.ServerContext.SessionKey
	repairActor := fixture.ServerContext
	repairActor.IsMainAgent = true
	updateResult, isError := callAutomationCommand(t, fixture.Service, repairActor, "update_scheduled_task", map[string]any{
		"query":                      "飞书新闻投递",
		"reply_mode":                 "selected",
		"selected_reply_session_key": recipientSessionKey,
	})
	if isError {
		t.Fatalf("update_scheduled_task 修正投递目标不应失败: %s", automationCommandText(t, updateResult))
	}
	updated := decodeAutomationCommandJSON[automationdomain.ScheduledTask](t, updateResult)
	if updated.Delivery.Channel != protocol.SessionChannelInternalSegment || updated.Delivery.To != recipientSessionKey {
		t.Fatalf("应把失败任务投递目标修正到真实当前会话: %+v", updated.Delivery)
	}

	retryResult, isError := callAutomationCommand(t, fixture.Service, sctx, "repair_scheduled_task", map[string]any{
		"action": "retry_delivery",
		"query":  "飞书新闻投递",
		"run_id": runID,
	})
	if !isError || !strings.Contains(automationCommandText(t, retryResult), "unverified") {
		t.Fatalf("普通 Automation CLI 不得重放结果未知的外投: isError=%v result=%s", isError, automationCommandText(t, retryResult))
	}

	statusResult, isError := callAutomationCommand(t, fixture.Service, sctx, "inspect_scheduled_task", map[string]any{
		"query":     "飞书新闻投递",
		"run_limit": 3,
	})
	if isError {
		t.Fatalf("重投递后 inspect_scheduled_task 不应失败: %s", automationCommandText(t, statusResult))
	}
	status := decodeAutomationCommandJSON[automationdomain.ScheduledTaskStatus](t, statusResult)
	if status.Job.LastDeliveryStatus != automationdomain.DeliveryStatusRetrying ||
		status.Health.ManualRedeliveryAvailable ||
		status.Health.DeliveryUnverifiedRunCount != 1 ||
		!slices.Contains(status.Health.Signals, "delivery_unverified") {
		t.Fatalf("结果未知时状态必须要求先核对且不开放普通补投: %+v", status)
	}
}

func TestAutomationCommandDeletedTaskReportDoesNotSuggestRedelivery(t *testing.T) {
	fixture := newAutomationCommandFixture(t, "今日新闻摘要")
	ownerCtx := automationCommandTestOwnerContext(fixture.ServerContext.OwnerUserID)
	now := time.Date(2026, 5, 22, 9, 0, 0, 0, time.UTC)
	fixture.Service.nowFn = func() time.Time { return now }
	sctx := fixture.ServerContext
	sctx.SessionKey = protocol.BuildAgentSessionKey(
		fixture.ServerContext.AgentID,
		protocol.SessionChannelFeishuSegment,
		protocol.RoomTypeDM,
		"oc_missing_group",
		"",
	)
	sctx.SessionLabel = "飞书私聊 oc_missing_group"
	sctx.SourceContextType = "agent_paired"

	createResult, isError := callAutomationCommand(t, fixture.Service, sctx, "create_scheduled_task", map[string]any{
		"name":              "已删飞书新闻投递",
		"instruction":       "搜索新闻并投递到飞书群",
		"execution_mode":    "dedicated",
		"named_session_key": "deleted-feishu-news",
		"reply_mode":        "channel",
		"reply_channel":     protocol.SessionChannelFeishu,
		"reply_to":          "oc_missing_group",
		"schedule": map[string]any{
			"kind":       "daily",
			"daily_time": "09:00",
			"timezone":   "Asia/Shanghai",
		},
	})
	if isError {
		t.Fatalf("create_scheduled_task 不应失败: %s", automationCommandText(t, createResult))
	}
	created := decodeAutomationCommandJSON[automationdomain.ScheduledTask](t, createResult)

	runResult, isError := callAutomationCommand(t, fixture.Service, sctx, "run_scheduled_task", map[string]any{
		"query": "已删飞书新闻投递",
	})
	if isError {
		t.Fatalf("run_scheduled_task by query 不应失败: %s", automationCommandText(t, runResult))
	}
	runNow := decodeAutomationCommandJSON[automationdomain.ExecutionResult](t, runResult)
	if runNow.RunID == nil || *runNow.RunID == "" {
		t.Fatalf("run_scheduled_task 应返回 run_id: %+v", runNow)
	}
	runID := *runNow.RunID
	waitFor(t, 2*time.Second, func() bool {
		runs, err := fixture.Service.ListTaskRuns(ownerCtx, created.JobID)
		return err == nil && len(runs) > 0 && runs[0].RunID == runID &&
			runs[0].DeliveryStatus == automationdomain.DeliveryStatusRetrying &&
			runs[0].DeliveryError != nil
	})

	deleteResult, isError := callAutomationCommand(t, fixture.Service, sctx, "delete_scheduled_task", map[string]any{
		"query": "已删飞书新闻投递",
	})
	if isError {
		t.Fatalf("delete_scheduled_task by query 不应失败: %s", automationCommandText(t, deleteResult))
	}
	deleted := decodeAutomationCommandJSON[automationdomain.DeleteJobResult](t, deleteResult)
	if deleted.JobID != created.JobID || !deleted.Deleted {
		t.Fatalf("delete_scheduled_task 应删除原任务: %+v", deleted)
	}

	reportResult, isError := callAutomationCommand(t, fixture.Service, sctx, "get_scheduled_task_report", map[string]any{
		"query":    "已删飞书新闻投递",
		"date":     "2026-05-22",
		"timezone": "UTC",
	})
	if isError {
		t.Fatalf("get_scheduled_task_report by query 不应失败: %s", automationCommandText(t, reportResult))
	}
	report := decodeAutomationCommandJSON[automationdomain.ScheduledTaskDailyReport](t, reportResult)
	if len(report.Tasks) != 1 {
		t.Fatalf("日报应定位到唯一已删任务: %+v", report)
	}
	taskReport := report.Tasks[0]
	if !taskReport.Deleted ||
		!slices.Contains(taskReport.Signals, "deleted") ||
		!slices.Contains(taskReport.Signals, "delivery_attention") ||
		!slices.Contains(taskReport.DeliveryDeadLetterRunIDs, runID) ||
		slices.Contains(taskReport.ManualRedeliveryRunIDs, runID) ||
		!slices.Contains(taskReport.SuggestedTools, runtimeAutomationInspectSuggestion) ||
		slices.Contains(taskReport.SuggestedTools, runtimeAutomationApplySuggestion) {
		t.Fatalf("已删任务日报应保留失败诊断但不建议补发: %+v", taskReport)
	}
}
