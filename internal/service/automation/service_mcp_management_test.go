package automation

import (
	"testing"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/protocol"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestAutomationMCPManageTaskLifecycleByQuery(t *testing.T) {
	fixture := newAutomationMCPFixture(t, "今日新闻摘要")
	ownerCtx := automationMCPTestOwnerContext(fixture.ServerContext.OwnerUserID)

	createResult, isError := callAutomationMCPTool(t, fixture.Service, fixture.ServerContext, "create_scheduled_task", map[string]any{
		"name":              "新闻投递到智能体",
		"instruction":       "每天搜索新闻并输出摘要",
		"execution_mode":    "dedicated",
		"named_session_key": "news-search",
		"reply_mode":        "agent",
		"reply_agent_id":    "agent-2",
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

	agent3InboxKey := protocol.BuildAgentSessionKey(
		"agent-3",
		protocol.SessionChannelInternalSegment,
		"dm",
		protocol.AutomationInboxSessionRef,
		"",
	)
	updateResult, isError := callAutomationMCPTool(t, fixture.Service, fixture.ServerContext, "update_scheduled_task", map[string]any{
		"query":          "新闻投递到智能体",
		"name":           "AI 新闻投递到智能体",
		"instruction":    "每天搜索 AI 新闻并输出三条摘要",
		"reply_agent_id": "agent-3",
		"schedule": map[string]any{
			"kind":       "daily",
			"daily_time": "10:30",
			"timezone":   "Asia/Shanghai",
		},
	})
	if isError {
		t.Fatalf("update_scheduled_task by query 不应失败: %s", automationMCPToolText(t, updateResult))
	}
	updated := decodeAutomationMCPJSON[automationdomain.CronJob](t, updateResult)
	if updated.JobID != created.JobID {
		t.Fatalf("update query 应定位原任务，updated=%+v created=%+v", updated, created)
	}
	if updated.Name != "AI 新闻投递到智能体" || updated.Instruction != "每天搜索 AI 新闻并输出三条摘要" {
		t.Fatalf("update 未写入新名称/指令: %+v", updated)
	}
	if updated.Schedule.CronExpression == nil || *updated.Schedule.CronExpression != "30 10 * * *" {
		t.Fatalf("update 未写入新的 daily 调度: %+v", updated.Schedule)
	}
	if updated.Delivery.Channel != protocol.SessionChannelInternalSegment || updated.Delivery.To != agent3InboxKey {
		t.Fatalf("update 未把投递目标切到 agent-3 收件箱: %+v", updated.Delivery)
	}

	disableResult, isError := callAutomationMCPTool(t, fixture.Service, fixture.ServerContext, "disable_scheduled_task", map[string]any{
		"query": "AI 新闻投递到智能体",
	})
	if isError {
		t.Fatalf("disable_scheduled_task by query 不应失败: %s", automationMCPToolText(t, disableResult))
	}
	disabled := decodeAutomationMCPJSON[automationdomain.CronJob](t, disableResult)
	if disabled.JobID != created.JobID || disabled.Enabled {
		t.Fatalf("disable query 应停用原任务，实际 %+v", disabled)
	}

	enableResult, isError := callAutomationMCPTool(t, fixture.Service, fixture.ServerContext, "enable_scheduled_task", map[string]any{
		"query": "AI 新闻投递到智能体",
	})
	if isError {
		t.Fatalf("enable_scheduled_task by query 不应失败: %s", automationMCPToolText(t, enableResult))
	}
	enabled := decodeAutomationMCPJSON[automationdomain.CronJob](t, enableResult)
	if enabled.JobID != created.JobID || !enabled.Enabled {
		t.Fatalf("enable query 应重新启用原任务，实际 %+v", enabled)
	}

	statusResult, isError := callAutomationMCPTool(t, fixture.Service, fixture.ServerContext, "get_scheduled_task_status", map[string]any{
		"query":       "AI 新闻投递到智能体",
		"event_limit": 5,
	})
	if isError {
		t.Fatalf("get_scheduled_task_status by query 不应失败: %s", automationMCPToolText(t, statusResult))
	}
	status := decodeAutomationMCPJSON[automationdomain.CronTaskStatus](t, statusResult)
	if status.Job.JobID != created.JobID || !status.Job.Enabled {
		t.Fatalf("status query 应看到重新启用后的任务状态，实际 %+v", status.Job)
	}

	deleteResult, isError := callAutomationMCPTool(t, fixture.Service, fixture.ServerContext, "delete_scheduled_task", map[string]any{
		"query": "AI 新闻投递到智能体",
	})
	if isError {
		t.Fatalf("delete_scheduled_task by query 不应失败: %s", automationMCPToolText(t, deleteResult))
	}
	deleted := decodeAutomationMCPJSON[automationdomain.DeleteJobResult](t, deleteResult)
	if deleted.JobID != created.JobID || !deleted.Deleted {
		t.Fatalf("delete query 应删除原任务，实际 %+v", deleted)
	}
	taskAfterDelete, err := fixture.Service.GetTask(ownerCtx, created.JobID)
	if err != nil {
		t.Fatalf("删除后读取任务失败: %v", err)
	}
	if taskAfterDelete != nil {
		t.Fatalf("delete 后任务不应仍可作为 active 任务读取: %+v", taskAfterDelete)
	}

	events, err := fixture.Service.ListTaskEvents(ownerCtx, created.JobID, 10)
	if err != nil {
		t.Fatalf("delete 后应仍能读取管理审计: %v", err)
	}
	assertTaskLifecycleEvents(t, events, created.JobID)
}

func TestAutomationMCPDisableCanStopActiveRunByQuery(t *testing.T) {
	fixture := newAutomationMCPFixture(t, "这条迟到结果不应覆盖 cancelled")
	fixture.DM.delay = 300 * time.Millisecond
	ownerCtx := automationMCPTestOwnerContext(fixture.ServerContext.OwnerUserID)

	createResult, isError := callAutomationMCPTool(t, fixture.Service, fixture.ServerContext, "create_scheduled_task", map[string]any{
		"name":              "正在运行的新闻任务",
		"instruction":       "搜索新闻，模拟长时间运行",
		"execution_mode":    "dedicated",
		"named_session_key": "running-news",
		"reply_mode":        "none",
		"schedule": map[string]any{
			"kind":           "interval",
			"interval_value": 1,
			"interval_unit":  "hours",
			"timezone":       "Asia/Shanghai",
		},
	})
	if isError {
		t.Fatalf("create_scheduled_task 不应失败: %s", automationMCPToolText(t, createResult))
	}
	created := decodeAutomationMCPJSON[automationdomain.CronJob](t, createResult)

	runResult, isError := callAutomationMCPTool(t, fixture.Service, fixture.ServerContext, "run_scheduled_task", map[string]any{
		"query": "正在运行的新闻任务",
	})
	if isError {
		t.Fatalf("run_scheduled_task by query 不应失败: %s", automationMCPToolText(t, runResult))
	}
	runNow := decodeAutomationMCPJSON[automationdomain.ExecutionResult](t, runResult)
	if runNow.RunID == nil || *runNow.RunID == "" {
		t.Fatalf("run_scheduled_task 应返回 active run_id: %+v", runNow)
	}
	runID := *runNow.RunID

	waitFor(t, 2*time.Second, func() bool {
		task, err := fixture.Service.GetTask(ownerCtx, created.JobID)
		return err == nil && task != nil && task.Running && task.RunningRunID == runID
	})

	disableResult, isError := callAutomationMCPTool(t, fixture.Service, fixture.ServerContext, "disable_scheduled_task", map[string]any{
		"query":             "正在运行的新闻任务",
		"cancel_active_run": true,
	})
	if isError {
		t.Fatalf("disable_scheduled_task cancel_active_run 不应失败: %s", automationMCPToolText(t, disableResult))
	}
	stopped := decodeAutomationMCPJSON[automationdomain.CronJob](t, disableResult)
	if stopped.Enabled || stopped.Running || stopped.RunningRunID != "" {
		t.Fatalf("停止当前 run 后任务应停用且清空 running: %+v", stopped)
	}
	if stopped.LastRunStatus != automationdomain.RunStatusCancelled {
		t.Fatalf("停止当前 run 后 last_run_status 应为 cancelled: %+v", stopped)
	}
	interrupts := fixture.DM.Interrupts()
	if len(interrupts) != 1 || interrupts[0].SessionKey != runNow.SessionKey {
		t.Fatalf("停止当前 run 应中断真实 DM 会话: interrupts=%+v run=%+v", interrupts, runNow)
	}

	waitFor(t, 2*time.Second, func() bool {
		runs, err := fixture.Service.ListTaskRuns(ownerCtx, created.JobID)
		return err == nil && len(runs) > 0 && runs[0].RunID == runID && runs[0].Status == automationdomain.RunStatusCancelled
	})
	time.Sleep(350 * time.Millisecond)
	runs, err := fixture.Service.ListTaskRuns(ownerCtx, created.JobID)
	if err != nil || len(runs) == 0 {
		t.Fatalf("读取停止后的 run 失败: runs=%+v err=%v", runs, err)
	}
	if runs[0].RunID != runID || runs[0].Status != automationdomain.RunStatusCancelled {
		t.Fatalf("迟到执行结果不应覆盖 cancelled run: %+v", runs)
	}

	events, err := fixture.Service.ListTaskEvents(ownerCtx, created.JobID, 10)
	if err != nil {
		t.Fatalf("停止后应能读取管理审计: %v", err)
	}
	assertTaskEventsInclude(t, events, created.JobID,
		automationdomain.TaskEventActionCreate,
		automationdomain.TaskEventActionRunNow,
		automationdomain.TaskEventActionDisable,
		automationdomain.TaskEventActionRecover,
	)
}

func TestAutomationMCPTaskEventsRecordCurrentActorAgent(t *testing.T) {
	fixture := newAutomationMCPFixture(t, "ok")
	ownerCtx := automationMCPTestOwnerContext(fixture.ServerContext.OwnerUserID)
	creatorCtx := fixture.ServerContext
	creatorCtx.CurrentAgentID = "agent-2"
	creatorCtx.CurrentAgentName = "子智能体"
	creatorCtx.CurrentSessionKey = protocol.BuildAgentSessionKey("agent-2", protocol.SessionChannelInternalSegment, "dm", "operator", "")
	creatorCtx.SourceContextID = "agent-2"
	creatorCtx.SourceContextLabel = "子智能体"

	createResult, isError := callAutomationMCPTool(t, fixture.Service, creatorCtx, "create_scheduled_task", map[string]any{
		"name":           "子智能体日报",
		"instruction":    "每天整理日报",
		"execution_mode": "temporary",
		"reply_mode":     "none",
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
	if created.AgentID != "agent-2" || created.Source.CreatorAgentID != "agent-2" {
		t.Fatalf("测试前置任务归属不正确: %+v", created)
	}

	mainCtx := fixture.ServerContext
	mainCtx.CurrentAgentID = "nexus"
	mainCtx.CurrentAgentName = "主智能体"
	mainCtx.IsMainAgent = true
	mainCtx.SourceContextID = "nexus"
	mainCtx.SourceContextLabel = "主智能体"
	disableResult, isError := callAutomationMCPTool(t, fixture.Service, mainCtx, "disable_scheduled_task", map[string]any{
		"job_id": created.JobID,
	})
	if isError {
		t.Fatalf("主智能体停用子智能体任务不应失败: %s", automationMCPToolText(t, disableResult))
	}

	events, err := fixture.Service.ListTaskEvents(ownerCtx, created.JobID, 10)
	if err != nil {
		t.Fatalf("读取管理审计失败: %v", err)
	}
	actorsByAction := map[string]string{}
	for _, event := range events {
		actorsByAction[event.Action] = event.ActorAgentID
	}
	if actorsByAction[automationdomain.TaskEventActionCreate] != "agent-2" {
		t.Fatalf("创建事件应记录创建 Agent，events=%+v", events)
	}
	if actorsByAction[automationdomain.TaskEventActionDisable] != "nexus" {
		t.Fatalf("停用事件应记录本次调用的主智能体，而不是原创建者: %+v", events)
	}
}

func assertTaskLifecycleEvents(t *testing.T, events []automationdomain.CronTaskEvent, jobID string) {
	t.Helper()
	assertTaskEventsInclude(t, events, jobID,
		automationdomain.TaskEventActionCreate,
		automationdomain.TaskEventActionUpdate,
		automationdomain.TaskEventActionDisable,
		automationdomain.TaskEventActionDelete,
	)
}

func assertTaskEventsInclude(t *testing.T, events []automationdomain.CronTaskEvent, jobID string, expectedActions ...string) {
	t.Helper()
	actions := map[string]bool{}
	for _, event := range events {
		if event.JobID != jobID || event.AgentID != "agent-1" {
			t.Fatalf("管理事件归属不正确: %+v", event)
		}
		if event.ActorUserID != "user-1" || event.ActorAgentID != "agent-1" {
			t.Fatalf("管理事件 actor 不正确: %+v", event)
		}
		actions[event.Action] = true
	}
	for _, action := range expectedActions {
		if !actions[action] {
			t.Fatalf("缺少管理事件 action=%s: %+v", action, events)
		}
	}
}
