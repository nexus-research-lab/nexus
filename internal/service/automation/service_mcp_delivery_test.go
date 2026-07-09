package automation

import (
	"slices"
	"strings"
	"testing"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/protocol"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func TestAutomationMCPCreateFromFeishuGroupDefaultsDeliveryToCurrentGroup(t *testing.T) {
	fixture := newAutomationMCPFixture(t, "今日新闻摘要")
	ownerCtx := automationMCPTestOwnerContext(fixture.ServerContext.OwnerUserID)
	now := time.Date(2026, 5, 22, 9, 0, 0, 0, time.UTC)
	fixture.Service.nowFn = func() time.Time { return now }

	feishuSessionKey := protocol.BuildAgentSessionKey(
		fixture.ServerContext.CurrentAgentID,
		protocol.SessionChannelFeishuSegment,
		"group",
		"oc_group_123",
		"",
	)
	sctx := fixture.ServerContext
	sctx.CurrentSessionKey = feishuSessionKey
	sctx.CurrentSessionLabel = "飞书群 oc_group_123"

	createResult, isError := callAutomationMCPTool(t, fixture.Service, sctx, "create_scheduled_task", map[string]any{
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
		t.Fatalf("create_scheduled_task 不应失败: %s", automationMCPToolText(t, createResult))
	}
	created := decodeAutomationMCPJSON[automationdomain.CronJob](t, createResult)
	if created.Delivery.Mode != automationdomain.DeliveryModeExplicit ||
		created.Delivery.Channel != protocol.SessionChannelFeishu ||
		created.Delivery.To != "oc_group_123" {
		t.Fatalf("飞书群上下文创建任务应默认回投当前群: %+v", created.Delivery)
	}
	if created.Source.SessionKey != feishuSessionKey || created.Source.SessionLabel != "飞书群 oc_group_123" {
		t.Fatalf("任务来源应保留飞书群会话上下文: %+v", created.Source)
	}

	runResult, isError := callAutomationMCPTool(t, fixture.Service, sctx, "run_scheduled_task", map[string]any{
		"query": "飞书群每日新闻",
	})
	if isError {
		t.Fatalf("run_scheduled_task by query 不应失败: %s", automationMCPToolText(t, runResult))
	}
	runNow := decodeAutomationMCPJSON[automationdomain.ExecutionResult](t, runResult)
	if runNow.RunID == nil || *runNow.RunID == "" {
		t.Fatalf("run_scheduled_task 应返回 run_id: %+v", runNow)
	}
	runID := *runNow.RunID

	waitFor(t, 2*time.Second, func() bool {
		runs, err := fixture.Service.ListTaskRuns(ownerCtx, created.JobID)
		return err == nil && len(runs) > 0 && runs[0].RunID == runID && runs[0].DeliveryStatus == automationdomain.DeliveryStatusFailed
	})
	runs, err := fixture.Service.ListTaskRuns(ownerCtx, created.JobID)
	if err != nil || len(runs) == 0 {
		t.Fatalf("读取飞书群默认投递 run 失败: runs=%+v err=%v", runs, err)
	}
	run := runs[0]
	if run.Status != automationdomain.RunStatusSucceeded ||
		run.DeliveryTo != "explicit:feishu:oc_group_123" ||
		run.DeliveryAttempts != 1 ||
		run.DeliveryNextAttemptAt == nil ||
		run.DeliveryError == nil ||
		!strings.Contains(*run.DeliveryError, "feishu") {
		t.Fatalf("飞书未配置时应只标记投递失败并保留可补救 ledger: %+v", run)
	}
}

func TestAutomationMCPReportAndRetryFailedDeliveryToAgentInbox(t *testing.T) {
	fixture := newAutomationMCPFixture(t, "今日新闻摘要")
	ownerCtx := automationMCPTestOwnerContext(fixture.ServerContext.OwnerUserID)
	now := time.Date(2026, 5, 22, 9, 0, 0, 0, time.UTC)
	fixture.Service.nowFn = func() time.Time { return now }

	createResult, isError := callAutomationMCPTool(t, fixture.Service, fixture.ServerContext, "create_scheduled_task", map[string]any{
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
		t.Fatalf("create_scheduled_task 不应失败: %s", automationMCPToolText(t, createResult))
	}
	created := decodeAutomationMCPJSON[automationdomain.CronJob](t, createResult)

	runResult, isError := callAutomationMCPTool(t, fixture.Service, fixture.ServerContext, "run_scheduled_task", map[string]any{
		"query": "飞书新闻投递",
	})
	if isError {
		t.Fatalf("run_scheduled_task by query 不应失败: %s", automationMCPToolText(t, runResult))
	}
	runNow := decodeAutomationMCPJSON[automationdomain.ExecutionResult](t, runResult)
	if runNow.RunID == nil || *runNow.RunID == "" {
		t.Fatalf("run_scheduled_task 应返回 run_id: %+v", runNow)
	}
	runID := *runNow.RunID

	waitFor(t, 2*time.Second, func() bool {
		runs, err := fixture.Service.ListTaskRuns(ownerCtx, created.JobID)
		return err == nil && len(runs) > 0 && runs[0].RunID == runID && runs[0].DeliveryStatus == automationdomain.DeliveryStatusFailed
	})
	failedRuns, err := fixture.Service.ListTaskRuns(ownerCtx, created.JobID)
	if err != nil || len(failedRuns) == 0 {
		t.Fatalf("读取飞书投递失败 run 失败: runs=%+v err=%v", failedRuns, err)
	}
	failedRun := failedRuns[0]
	if failedRun.Status != automationdomain.RunStatusSucceeded || failedRun.DeliveryError == nil {
		t.Fatalf("飞书发送失败不应影响执行成功，但应记录 delivery_error: %+v", failedRun)
	}
	if failedRun.DeliveryAttempts != 1 || failedRun.DeliveryNextAttemptAt == nil {
		t.Fatalf("飞书发送失败应记录投递尝试并安排重试: %+v", failedRun)
	}

	reportResult, isError := callAutomationMCPTool(t, fixture.Service, fixture.ServerContext, "get_scheduled_task_daily_report", map[string]any{
		"query":    "飞书新闻投递",
		"date":     "2026-05-22",
		"timezone": "UTC",
	})
	if isError {
		t.Fatalf("get_scheduled_task_daily_report by query 不应失败: %s", automationMCPToolText(t, reportResult))
	}
	report := decodeAutomationMCPJSON[automationdomain.CronDailyReport](t, reportResult)
	if len(report.Tasks) != 1 {
		t.Fatalf("日报应定位到唯一任务: %+v", report)
	}
	taskReport := report.Tasks[0]
	if !slices.Contains(taskReport.Signals, "delivery_attention") ||
		!slices.Contains(taskReport.SuggestedTools, "retry_scheduled_task_delivery") ||
		!slices.Contains(taskReport.ManualRedeliveryRunIDs, runID) {
		t.Fatalf("日报应直接指出失败投递 run 和补救工具: %+v", taskReport)
	}

	inboxKey := protocol.BuildAgentSessionKey(
		"agent-2",
		protocol.SessionChannelInternalSegment,
		"dm",
		protocol.AutomationInboxSessionRef,
		"",
	)
	updateResult, isError := callAutomationMCPTool(t, fixture.Service, fixture.ServerContext, "update_scheduled_task", map[string]any{
		"query":          "飞书新闻投递",
		"reply_agent_id": "agent-2",
	})
	if isError {
		t.Fatalf("update_scheduled_task 修正投递目标不应失败: %s", automationMCPToolText(t, updateResult))
	}
	updated := decodeAutomationMCPJSON[automationdomain.CronJob](t, updateResult)
	if updated.Delivery.Channel != protocol.SessionChannelInternalSegment || updated.Delivery.To != inboxKey {
		t.Fatalf("应把失败任务投递目标修正到 agent-2 收件箱: %+v", updated.Delivery)
	}

	retryResult, isError := callAutomationMCPTool(t, fixture.Service, fixture.ServerContext, "retry_scheduled_task_delivery", map[string]any{
		"query":  "飞书新闻投递",
		"run_id": runID,
	})
	if isError {
		t.Fatalf("retry_scheduled_task_delivery by query 不应失败: %s", automationMCPToolText(t, retryResult))
	}
	redelivered := decodeAutomationMCPJSON[automationdomain.CronRun](t, retryResult)
	if redelivered.RunID != runID ||
		redelivered.DeliveryStatus != automationdomain.DeliveryStatusSucceeded ||
		redelivered.DeliveryError != nil ||
		redelivered.DeliveryTo != "explicit:internal:"+inboxKey {
		t.Fatalf("重投递应复用原 run 并记录新的实际目标: %+v", redelivered)
	}
	if redelivered.DeliveryAttempts != 2 || redelivered.DeliveryNextAttemptAt != nil || redelivered.DeliveryDeadLetterAt != nil {
		t.Fatalf("重投递成功后应清理重试计划: %+v", redelivered)
	}

	store := workspacestore.NewSessionFileStore(fixture.WorkspacePath)
	sessionValue, _, err := store.FindSession([]string{fixture.WorkspacePath}, inboxKey)
	if err != nil {
		t.Fatalf("读取重投递智能体收件箱 session 失败: %v", err)
	}
	if sessionValue == nil {
		t.Fatal("重投递到智能体时应自动创建目标收件箱")
	}
	assertDeliveredAgentMessage(t, fixture.WorkspacePath, *sessionValue, "今日新闻摘要", "重投递智能体收件箱")

	statusResult, isError := callAutomationMCPTool(t, fixture.Service, fixture.ServerContext, "get_scheduled_task_status", map[string]any{
		"query":     "飞书新闻投递",
		"run_limit": 3,
	})
	if isError {
		t.Fatalf("重投递后 get_scheduled_task_status 不应失败: %s", automationMCPToolText(t, statusResult))
	}
	status := decodeAutomationMCPJSON[automationdomain.CronTaskStatus](t, statusResult)
	if status.Job.LastDeliveryStatus != automationdomain.DeliveryStatusSucceeded ||
		status.Health.ManualRedeliveryAvailable ||
		status.Health.DeliveryFailedRunCount != 0 {
		t.Fatalf("重投递成功后状态应清除可手动补投提示: %+v", status)
	}
}

func TestAutomationMCPDeletedTaskReportDoesNotSuggestRedelivery(t *testing.T) {
	fixture := newAutomationMCPFixture(t, "今日新闻摘要")
	ownerCtx := automationMCPTestOwnerContext(fixture.ServerContext.OwnerUserID)
	now := time.Date(2026, 5, 22, 9, 0, 0, 0, time.UTC)
	fixture.Service.nowFn = func() time.Time { return now }

	createResult, isError := callAutomationMCPTool(t, fixture.Service, fixture.ServerContext, "create_scheduled_task", map[string]any{
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
		t.Fatalf("create_scheduled_task 不应失败: %s", automationMCPToolText(t, createResult))
	}
	created := decodeAutomationMCPJSON[automationdomain.CronJob](t, createResult)

	runResult, isError := callAutomationMCPTool(t, fixture.Service, fixture.ServerContext, "run_scheduled_task", map[string]any{
		"query": "已删飞书新闻投递",
	})
	if isError {
		t.Fatalf("run_scheduled_task by query 不应失败: %s", automationMCPToolText(t, runResult))
	}
	runNow := decodeAutomationMCPJSON[automationdomain.ExecutionResult](t, runResult)
	if runNow.RunID == nil || *runNow.RunID == "" {
		t.Fatalf("run_scheduled_task 应返回 run_id: %+v", runNow)
	}
	runID := *runNow.RunID
	waitFor(t, 2*time.Second, func() bool {
		runs, err := fixture.Service.ListTaskRuns(ownerCtx, created.JobID)
		return err == nil && len(runs) > 0 && runs[0].RunID == runID && runs[0].DeliveryStatus == automationdomain.DeliveryStatusFailed
	})

	deleteResult, isError := callAutomationMCPTool(t, fixture.Service, fixture.ServerContext, "delete_scheduled_task", map[string]any{
		"query": "已删飞书新闻投递",
	})
	if isError {
		t.Fatalf("delete_scheduled_task by query 不应失败: %s", automationMCPToolText(t, deleteResult))
	}
	deleted := decodeAutomationMCPJSON[automationdomain.DeleteJobResult](t, deleteResult)
	if deleted.JobID != created.JobID || !deleted.Deleted {
		t.Fatalf("delete_scheduled_task 应删除原任务: %+v", deleted)
	}

	reportResult, isError := callAutomationMCPTool(t, fixture.Service, fixture.ServerContext, "get_scheduled_task_daily_report", map[string]any{
		"query":    "已删飞书新闻投递",
		"date":     "2026-05-22",
		"timezone": "UTC",
	})
	if isError {
		t.Fatalf("get_scheduled_task_daily_report by query 不应失败: %s", automationMCPToolText(t, reportResult))
	}
	report := decodeAutomationMCPJSON[automationdomain.CronDailyReport](t, reportResult)
	if len(report.Tasks) != 1 {
		t.Fatalf("日报应定位到唯一已删任务: %+v", report)
	}
	taskReport := report.Tasks[0]
	if !taskReport.Deleted ||
		!slices.Contains(taskReport.Signals, "deleted") ||
		!slices.Contains(taskReport.Signals, "delivery_attention") ||
		!slices.Contains(taskReport.DeliveryDeadLetterRunIDs, runID) ||
		slices.Contains(taskReport.ManualRedeliveryRunIDs, runID) ||
		!slices.Contains(taskReport.SuggestedTools, "get_scheduled_task_events") ||
		slices.Contains(taskReport.SuggestedTools, "retry_scheduled_task_delivery") {
		t.Fatalf("已删任务日报应保留失败诊断但不建议补发: %+v", taskReport)
	}
}
