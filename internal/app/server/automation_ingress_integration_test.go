package server

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestFeishuIngressCanCreateScheduledTaskThroughDMMCPRuntime(t *testing.T) {
	fixture := newAutomationIngressFixture(t)
	fixture.acceptFeishuMessage("每天 9 点搜索重要新闻并发到这个飞书群", "feishu-event-1", "feishu-message-1")

	var created protocol.CronJob
	waitForAutomationIngress(t, 3*time.Second, func() bool {
		items, err := fixture.automation.ListTasks(context.Background(), fixture.cfg.DefaultAgentID)
		if err != nil || len(items) != 1 {
			return false
		}
		created = items[0]
		return true
	})
	if created.Name != "飞书群每日新闻" ||
		created.Schedule.Kind != protocol.ScheduleKindCron ||
		created.Schedule.CronExpression == nil ||
		*created.Schedule.CronExpression != "0 9 * * *" {
		t.Fatalf("通过 Feishu ingress 创建的任务配置不正确: %+v", created)
	}
	if created.Delivery.Mode != protocol.DeliveryModeExplicit ||
		created.Delivery.Channel != protocol.SessionChannelFeishu ||
		created.Delivery.To != "oc_group_123" {
		t.Fatalf("通过 Feishu ingress 创建的任务应回投当前群: %+v", created.Delivery)
	}
	expectedSessionKey := protocol.BuildAgentSessionKey(
		fixture.cfg.DefaultAgentID,
		protocol.SessionChannelFeishuSegment,
		"group",
		"oc_group_123",
		"",
	)
	if created.Source.Kind != protocol.SourceKindAgent ||
		created.Source.SessionKey != expectedSessionKey {
		t.Fatalf("通过 Feishu ingress 创建的任务应保留来源会话: %+v", created.Source)
	}
	if !fixture.runtimeFactory.AllowedTool("create_scheduled_task") {
		t.Fatal("Feishu ingress runtime 应自动允许 nexus_automation 创建定时任务工具")
	}

	runResult, err := fixture.automation.RunTaskNow(context.Background(), created.JobID)
	if err != nil {
		t.Fatalf("立即运行 Feishu ingress 创建的任务失败: %v", err)
	}
	if runResult.RunID == nil || *runResult.RunID == "" {
		t.Fatalf("立即运行应返回 run_id: %+v", runResult)
	}
	runID := *runResult.RunID
	waitForAutomationIngress(t, 3*time.Second, func() bool {
		runs, listErr := fixture.automation.ListTaskRuns(context.Background(), created.JobID)
		return listErr == nil &&
			len(runs) == 1 &&
			runs[0].RunID == runID &&
			runs[0].DeliveryStatus == protocol.DeliveryStatusFailed
	})
	runs, err := fixture.automation.ListTaskRuns(context.Background(), created.JobID)
	if err != nil || len(runs) != 1 {
		t.Fatalf("读取任务运行记录失败: runs=%+v err=%v", runs, err)
	}
	run := runs[0]
	if run.Status != protocol.RunStatusSucceeded ||
		run.DeliveryStatus != protocol.DeliveryStatusFailed ||
		run.DeliveryTo != "explicit:feishu:oc_group_123" ||
		run.DeliveryError == nil ||
		!strings.Contains(*run.DeliveryError, "feishu") ||
		run.DeliveryAttempts != 1 ||
		run.DeliveryNextAttemptAt == nil {
		t.Fatalf("Feishu 未配置时应只让投递进入可恢复失败 ledger: %+v", run)
	}
}

func TestFeishuIngressCanManageScheduledTaskThroughDMMCPRuntime(t *testing.T) {
	fixture := newAutomationIngressFixture(t)
	seeded := fixture.seedDailyFeishuTask("飞书群每日新闻", "0 9 * * *")

	fixture.acceptFeishuMessage("把飞书群每日新闻改成每天 10 点", "feishu-event-update", "feishu-message-update")
	var updated *protocol.CronJob
	waitForAutomationIngress(t, 3*time.Second, func() bool {
		var err error
		updated, err = fixture.automation.GetTask(context.Background(), seeded.JobID)
		return err == nil &&
			updated != nil &&
			updated.Schedule.CronExpression != nil &&
			*updated.Schedule.CronExpression == "0 10 * * *"
	})
	if !fixture.runtimeFactory.AllowedTool("update_scheduled_task") {
		t.Fatal("Feishu ingress runtime 应自动允许 update_scheduled_task")
	}

	fixture.acceptFeishuMessage("暂停飞书群每日新闻", "feishu-event-disable", "feishu-message-disable")
	waitForAutomationIngress(t, 3*time.Second, func() bool {
		var err error
		updated, err = fixture.automation.GetTask(context.Background(), seeded.JobID)
		return err == nil && updated != nil && !updated.Enabled
	})
	if !fixture.runtimeFactory.AllowedTool("disable_scheduled_task") {
		t.Fatal("Feishu ingress runtime 应自动允许 disable_scheduled_task")
	}

	fixture.acceptFeishuMessage("恢复飞书群每日新闻", "feishu-event-enable", "feishu-message-enable")
	waitForAutomationIngress(t, 3*time.Second, func() bool {
		var err error
		updated, err = fixture.automation.GetTask(context.Background(), seeded.JobID)
		return err == nil && updated != nil && updated.Enabled
	})
	if !fixture.runtimeFactory.AllowedTool("enable_scheduled_task") {
		t.Fatal("Feishu ingress runtime 应自动允许 enable_scheduled_task")
	}

	fixture.acceptFeishuMessage("删除飞书群每日新闻", "feishu-event-delete", "feishu-message-delete")
	waitForAutomationIngress(t, 3*time.Second, func() bool {
		deleted, err := fixture.automation.GetTask(context.Background(), seeded.JobID)
		return err == nil && deleted == nil
	})
	if !fixture.runtimeFactory.AllowedTool("delete_scheduled_task") {
		t.Fatal("Feishu ingress runtime 应自动允许 delete_scheduled_task")
	}
	history, err := fixture.automation.SearchTaskHistory(context.Background(), protocol.CronTaskHistorySearchInput{
		Query:          "飞书群每日新闻",
		AgentID:        fixture.cfg.DefaultAgentID,
		IncludeActive:  true,
		IncludeDeleted: true,
		Limit:          10,
	})
	if err != nil {
		t.Fatalf("搜索删除后的任务历史失败: %v", err)
	}
	if len(history) != 1 || history[0].JobID != seeded.JobID || !history[0].Deleted {
		t.Fatalf("删除后的任务应仍可通过对话线索追溯: %+v", history)
	}
}

func TestFeishuIngressCanInspectAndRetryScheduledTaskDeliveryThroughDMMCPRuntime(t *testing.T) {
	fixture := newAutomationIngressFixture(t)
	seeded := fixture.seedDailyFeishuTask("飞书群每日新闻", "0 9 * * *")
	runID := fixture.runTaskUntilDeliveryFailed(seeded.JobID)

	fixture.acceptFeishuMessage("检查今天飞书群每日新闻发送情况", "feishu-event-report", "feishu-message-report")
	if !fixture.runtimeFactory.AllowedTool("get_scheduled_task_daily_report") ||
		!fixture.runtimeFactory.AllowedTool("get_scheduled_task_status") {
		t.Fatalf("Feishu ingress runtime 应自动允许检查发送情况工具: %+v", fixture.runtimeFactory.AllowedTools())
	}

	fixture.acceptFeishuMessage("把飞书群每日新闻失败发送改到智能体收件箱并补发", "feishu-event-redeliver", "feishu-message-redeliver")
	if !fixture.runtimeFactory.AllowedTool("update_scheduled_task") ||
		!fixture.runtimeFactory.AllowedTool("retry_scheduled_task_delivery") {
		t.Fatalf("Feishu ingress runtime 应自动允许修正目标并补发: %+v", fixture.runtimeFactory.AllowedTools())
	}

	inboxKey := protocol.BuildAgentSessionKey(
		fixture.cfg.DefaultAgentID,
		protocol.SessionChannelInternalSegment,
		"dm",
		protocol.AutomationInboxSessionRef,
		"",
	)
	waitForAutomationIngress(t, 3*time.Second, func() bool {
		runs, err := fixture.automation.ListTaskRuns(context.Background(), seeded.JobID)
		return err == nil &&
			len(runs) == 1 &&
			runs[0].RunID == runID &&
			runs[0].DeliveryStatus == protocol.DeliveryStatusSucceeded &&
			runs[0].DeliveryTo == "explicit:internal:"+inboxKey
	})
	runs, err := fixture.automation.ListTaskRuns(context.Background(), seeded.JobID)
	if err != nil || len(runs) != 1 {
		t.Fatalf("读取补发后的 run 失败: runs=%+v err=%v", runs, err)
	}
	if runs[0].DeliveryAttempts != 2 || runs[0].DeliveryError != nil || runs[0].DeliveryNextAttemptAt != nil {
		t.Fatalf("补发成功后应清理失败状态并保留尝试次数: %+v", runs[0])
	}
	fixture.assertAgentInboxReceived(inboxKey, "今日新闻摘要")
}

func TestFeishuIngressCanStopRunningScheduledTaskThroughDMMCPRuntime(t *testing.T) {
	fixture := newAutomationIngressFixture(t)
	seeded := fixture.seedLongRunningFeishuTask("飞书群每日新闻", "0 9 * * *")

	runResult, err := fixture.automation.RunTaskNow(context.Background(), seeded.JobID)
	if err != nil {
		t.Fatalf("立即运行长任务失败: %v", err)
	}
	if runResult.RunID == nil || *runResult.RunID == "" {
		t.Fatalf("立即运行长任务应返回 run_id: %+v", runResult)
	}
	runID := *runResult.RunID
	waitForAutomationIngress(t, 3*time.Second, func() bool {
		task, getErr := fixture.automation.GetTask(context.Background(), seeded.JobID)
		return getErr == nil && task != nil && task.Running && task.RunningRunID == runID
	})

	fixture.acceptFeishuMessage("停止正在运行的飞书群每日新闻", "feishu-event-stop-running", "feishu-message-stop-running")
	if !fixture.runtimeFactory.AllowedTool("disable_scheduled_task") {
		t.Fatalf("Feishu ingress runtime 应自动允许停止当前运行任务: %+v", fixture.runtimeFactory.AllowedTools())
	}

	var stopped *protocol.CronJob
	waitForAutomationIngress(t, 3*time.Second, func() bool {
		var getErr error
		stopped, getErr = fixture.automation.GetTask(context.Background(), seeded.JobID)
		return getErr == nil &&
			stopped != nil &&
			!stopped.Enabled &&
			!stopped.Running &&
			stopped.RunningRunID == "" &&
			stopped.LastRunStatus == protocol.RunStatusCancelled
	})
	runs, err := fixture.automation.ListTaskRuns(context.Background(), seeded.JobID)
	if err != nil || len(runs) != 1 {
		t.Fatalf("读取停止后的 run 失败: runs=%+v err=%v", runs, err)
	}
	if runs[0].RunID != runID || runs[0].Status != protocol.RunStatusCancelled {
		t.Fatalf("停止当前运行任务应把 active run 标记为 cancelled: %+v", runs)
	}
	time.Sleep(350 * time.Millisecond)
	runs, err = fixture.automation.ListTaskRuns(context.Background(), seeded.JobID)
	if err != nil || len(runs) != 1 || runs[0].Status != protocol.RunStatusCancelled {
		t.Fatalf("迟到执行结果不应覆盖 cancelled run: runs=%+v err=%v", runs, err)
	}
}
