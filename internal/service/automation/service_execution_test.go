package automation

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	_ "modernc.org/sqlite"
)

func TestServiceRunTaskNowUpdatesRunLedger(t *testing.T) {
	db := newAutomationTestDB(t)
	workspacePath := t.TempDir()
	permission := permissionctx.NewContext()
	dm := &fakeDMRunner{
		permission:    permission,
		assistantText: "assistant answer",
		resultText:    "runtime result",
	}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspacePath},
		db,
		nil,
		dm,
		nil,
		permission,
		&fakeWorkspaceReader{},
		nil,
	)

	task, err := service.CreateTask(context.Background(), protocol.CreateJobInput{
		Name:        "日报同步",
		AgentID:     "agent-1",
		Instruction: "整理今天的进展",
		Schedule: protocol.Schedule{
			Kind:            protocol.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: protocol.SessionTarget{
			Kind:            protocol.SessionTargetBound,
			BoundSessionKey: protocol.BuildAgentSessionKey("agent-1", "ws", "dm", "manual", ""),
		},
		Delivery: protocol.DeliveryTarget{Mode: protocol.DeliveryModeNone},
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	result, err := service.RunTaskNow(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("RunTaskNow 失败: %v", err)
	}
	if result.Status != protocol.RunStatusRunning {
		t.Fatalf("期望立即返回 running，实际为 %s", result.Status)
	}

	waitFor(t, 2*time.Second, func() bool {
		items, listErr := service.ListTaskRuns(context.Background(), task.JobID)
		if listErr != nil || len(items) == 0 {
			return false
		}
		return items[0].Status == protocol.RunStatusSucceeded
	})

	runs, err := service.ListTaskRuns(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("ListTaskRuns 失败: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("期望 1 条 run 记录，实际 %d", len(runs))
	}
	if runs[0].Status != protocol.RunStatusSucceeded {
		t.Fatalf("期望 run 成功，实际 %s", runs[0].Status)
	}
	if runs[0].AssistantText == nil || *runs[0].AssistantText != "assistant answer" {
		t.Fatalf("assistant_text 未持久化: %+v", runs[0].AssistantText)
	}
	if runs[0].ResultText == nil || *runs[0].ResultText != "runtime result" {
		t.Fatalf("result_text 未持久化: %+v", runs[0].ResultText)
	}
	if runs[0].ResultSummary == nil || *runs[0].ResultSummary != "runtime result" {
		t.Fatalf("result_summary 未优先使用 runtime result: %+v", runs[0].ResultSummary)
	}
	if runs[0].DeliveryStatus != protocol.DeliveryStatusNotRequired {
		t.Fatalf("delivery_status 未记录无需投递: %s", runs[0].DeliveryStatus)
	}
	if runs[0].ArtifactPath == nil || !strings.HasPrefix(*runs[0].ArtifactPath, ".nexus/automation/runs/") {
		t.Fatalf("artifact_path 未持久化: %+v", runs[0].ArtifactPath)
	}
	artifactContent, readErr := os.ReadFile(filepath.Join(workspacePath, filepath.FromSlash(*runs[0].ArtifactPath)))
	if readErr != nil {
		t.Fatalf("读取运行产物失败: %v", readErr)
	}
	if content := string(artifactContent); !strings.Contains(content, "runtime result") || !strings.Contains(content, "assistant answer") {
		t.Fatalf("运行产物内容不完整: %s", content)
	}

	requests := dm.Requests()
	if len(requests) != 1 {
		t.Fatalf("期望 dm runner 收到 1 次请求，实际 %d", len(requests))
	}
	if requests[0].Content != "整理今天的进展" {
		t.Fatalf("下发指令不正确: %s", requests[0].Content)
	}
	if requests[0].PermissionHandler == nil {
		t.Fatal("定时任务 DM 请求应使用非交互权限处理器")
	}
	if requests[0].PermissionMode != sdkpermission.ModeDefault {
		t.Fatalf("定时任务 DM 请求应由后台权限处理器接管授权，实际 mode=%s", requests[0].PermissionMode)
	}
	askDecision, err := requests[0].PermissionHandler(context.Background(), sdkpermission.Request{ToolName: "AskUserQuestion"})
	if err != nil {
		t.Fatalf("AskUserQuestion 权限处理失败: %v", err)
	}
	if askDecision.Behavior != sdkpermission.BehaviorDeny {
		t.Fatalf("后台定时任务不应等待交互式提问: %+v", askDecision)
	}
	writeDecision, err := requests[0].PermissionHandler(context.Background(), sdkpermission.Request{ToolName: "Write"})
	if err != nil {
		t.Fatalf("Write 权限处理失败: %v", err)
	}
	if writeDecision.Behavior != sdkpermission.BehaviorDeny {
		t.Fatalf("后台定时任务未预授权工具时应立即拒绝: %+v", writeDecision)
	}
}

func TestServiceRunTaskNowCanRunDisabledTaskWithoutReenabling(t *testing.T) {
	db := newAutomationTestDB(t)
	workspacePath := t.TempDir()
	permission := permissionctx.NewContext()
	dm := &fakeDMRunner{
		permission:    permission,
		assistantText: "manual run answer",
		resultText:    "manual run result",
	}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspacePath},
		db,
		nil,
		dm,
		nil,
		permission,
		&fakeWorkspaceReader{},
		nil,
	)

	task, err := service.CreateTask(context.Background(), protocol.CreateJobInput{
		Name:        "暂停新闻日报",
		AgentID:     "agent-1",
		Instruction: "手动补跑今天新闻",
		Schedule: protocol.Schedule{
			Kind:            protocol.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: protocol.SessionTarget{
			Kind:            protocol.SessionTargetBound,
			BoundSessionKey: protocol.BuildAgentSessionKey("agent-1", "ws", "dm", "manual", ""),
		},
		Delivery: protocol.DeliveryTarget{Mode: protocol.DeliveryModeNone},
		Enabled:  false,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	result, err := service.RunTaskNow(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("RunTaskNow disabled task 失败: %v", err)
	}
	if result.Status != protocol.RunStatusRunning || result.RunID == nil {
		t.Fatalf("disabled task manual run should start once: %+v", result)
	}
	waitFor(t, 2*time.Second, func() bool {
		items, listErr := service.ListTaskRuns(context.Background(), task.JobID)
		return listErr == nil && len(items) == 1 && items[0].Status == protocol.RunStatusSucceeded
	})

	jobs, err := service.ListTasks(context.Background(), "agent-1")
	if err != nil {
		t.Fatalf("ListTasks 失败: %v", err)
	}
	if len(jobs) != 1 || jobs[0].Enabled {
		t.Fatalf("manual run must not re-enable disabled task: %+v", jobs)
	}
	runs, err := service.ListTaskRuns(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("ListTaskRuns 失败: %v", err)
	}
	if len(runs) != 1 || runs[0].ResultSummary == nil || *runs[0].ResultSummary != "manual run result" {
		t.Fatalf("manual run ledger 不正确: %+v", runs)
	}
}

func TestServiceRunTaskNowRecordsPermissionDeniedToolAsFailedRun(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	dm := &fakeDMRunner{
		permission:   permission,
		requiredTool: "WebSearch",
	}
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
		Name:        "新闻搜索",
		AgentID:     "agent-1",
		Instruction: "搜索今天的 AI 新闻并总结",
		Schedule: protocol.Schedule{
			Kind:            protocol.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: protocol.SessionTarget{
			Kind:            protocol.SessionTargetBound,
			BoundSessionKey: protocol.BuildAgentSessionKey("agent-1", "ws", "dm", "permission-denied", ""),
		},
		Delivery: protocol.DeliveryTarget{Mode: protocol.DeliveryModeNone},
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	result, err := service.RunTaskNow(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("RunTaskNow 下发失败: %v", err)
	}
	if result.Status != protocol.RunStatusRunning {
		t.Fatalf("期望立即返回 running，实际为 %s", result.Status)
	}

	waitFor(t, 2*time.Second, func() bool {
		runs, listErr := service.ListTaskRuns(context.Background(), task.JobID)
		return listErr == nil && len(runs) == 1 && runs[0].Status == protocol.RunStatusFailed
	})
	runs, err := service.ListTaskRuns(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("ListTaskRuns 失败: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("期望 1 条 run，实际 %d", len(runs))
	}
	run := runs[0]
	if run.ErrorMessage == nil || !strings.Contains(*run.ErrorMessage, "WebSearch") {
		t.Fatalf("权限拒绝应写入 run error_message: %+v", run)
	}
	if run.ResultText == nil || !strings.Contains(*run.ResultText, "WebSearch") {
		t.Fatalf("权限拒绝仍应保留 runtime 结果文本: %+v", run)
	}

	updatedTask, err := service.GetTask(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("GetTask 失败: %v", err)
	}
	if updatedTask == nil || updatedTask.LastRunStatus != protocol.RunStatusFailed || updatedTask.FailureStreak != 1 {
		t.Fatalf("任务运行态未记录权限失败: %+v", updatedTask)
	}
	if updatedTask.LastError == nil || !strings.Contains(*updatedTask.LastError, "WebSearch") {
		t.Fatalf("任务 last_error 应包含权限失败原因: %+v", updatedTask)
	}

	status, err := service.GetTaskStatus(context.Background(), task.JobID, 10, 10)
	if err != nil {
		t.Fatalf("GetTaskStatus 失败: %v", err)
	}
	if status.Health.State != "attention" || status.Health.LatestExecutionError == nil ||
		!strings.Contains(*status.Health.LatestExecutionError, "WebSearch") {
		t.Fatalf("任务健康摘要应暴露权限失败: %+v", status.Health)
	}
	if !containsString(status.Health.Signals, "recent_execution_failed") ||
		!containsString(status.Health.ExecutionFailedRunIDs, run.RunID) {
		t.Fatalf("任务健康摘要缺少失败信号或 run_id: %+v", status.Health)
	}
	if !containsString(status.Health.SuggestedTools, "update_scheduled_task") ||
		!containsString(status.Health.SuggestedTools, "run_scheduled_task") {
		t.Fatalf("任务健康摘要缺少执行失败补救工具: %+v", status.Health)
	}

	report, err := service.GetDailyReport(context.Background(), protocol.CronDailyReportInput{
		Date:     "today",
		Timezone: "Asia/Shanghai",
		JobID:    task.JobID,
	})
	if err != nil {
		t.Fatalf("GetDailyReport 失败: %v", err)
	}
	if report.Totals.FailedRunCount != 1 || len(report.Tasks) != 1 ||
		report.Tasks[0].LatestExecutionError == nil ||
		!strings.Contains(*report.Tasks[0].LatestExecutionError, "WebSearch") {
		t.Fatalf("日报应暴露权限失败: %+v", report)
	}
	if !containsString(report.Tasks[0].SuggestedTools, "update_scheduled_task") ||
		!containsString(report.Tasks[0].SuggestedTools, "run_scheduled_task") {
		t.Fatalf("日报应提示执行失败补救工具: %+v", report.Tasks[0])
	}
}

func TestServiceDispatchRoomTaskUsesNonInteractivePermissionHandler(t *testing.T) {
	roomRunner := &fakeRoomRunner{}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		nil,
		nil,
		nil,
		roomRunner,
		nil,
		&fakeWorkspaceReader{},
		nil,
	)
	sessionKey := protocol.BuildRoomSharedSessionKey("conversation-1")
	if err := service.dispatchToSession(context.Background(), sessionKey, "round-room-1", "agent-1", "整理 Room 今日进展"); err != nil {
		t.Fatalf("dispatchToSession 失败: %v", err)
	}

	requests := roomRunner.Requests()
	if len(requests) != 1 {
		t.Fatalf("期望 room runner 收到 1 次请求，实际 %d", len(requests))
	}
	if requests[0].SessionKey != sessionKey || requests[0].ConversationID != "conversation-1" {
		t.Fatalf("Room 请求路由不正确: %+v", requests[0])
	}
	if requests[0].PermissionHandler == nil {
		t.Fatal("Room 定时任务请求应使用非交互权限处理器")
	}
	if requests[0].PermissionMode != sdkpermission.ModeDefault {
		t.Fatalf("Room 定时任务请求应由后台权限处理器接管授权，实际 mode=%s", requests[0].PermissionMode)
	}
	decision, err := requests[0].PermissionHandler(context.Background(), sdkpermission.Request{ToolName: "AskUserQuestion"})
	if err != nil {
		t.Fatalf("AskUserQuestion 权限处理失败: %v", err)
	}
	if decision.Behavior != sdkpermission.BehaviorDeny {
		t.Fatalf("Room 后台定时任务不应等待交互式提问: %+v", decision)
	}
}

func TestServiceRunTaskNowSupportsBoundRoomSessionTarget(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	roomRunner := &fakeRoomRunner{permission: permission, resultText: "Room 定时总结完成"}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		nil,
		roomRunner,
		permission,
		&fakeWorkspaceReader{},
		nil,
	)
	sessionKey := protocol.BuildRoomSharedSessionKey("conversation-1")
	task, err := service.CreateTask(context.Background(), protocol.CreateJobInput{
		Name:        "room-summary",
		AgentID:     "agent-1",
		Instruction: "整理 Room 今日进展",
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

	result, err := service.RunTaskNow(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("RunTaskNow 不应拒绝 Room 执行会话: %v", err)
	}
	if result.SessionKey != sessionKey || result.Status != protocol.RunStatusRunning {
		t.Fatalf("Room 定时任务下发结果不正确: %+v", result)
	}
	requests := roomRunner.Requests()
	if len(requests) != 1 || requests[0].SessionKey != sessionKey || requests[0].ConversationID != "conversation-1" {
		t.Fatalf("Room runner 请求不正确: %+v", requests)
	}

	waitFor(t, 2*time.Second, func() bool {
		runs, listErr := service.ListTaskRuns(context.Background(), task.JobID)
		return listErr == nil && len(runs) == 1 && runs[0].Status == protocol.RunStatusSucceeded
	})
	runs, err := service.ListTaskRuns(context.Background(), task.JobID)
	if err != nil || len(runs) != 1 {
		t.Fatalf("读取 Room 定时任务 run 失败: runs=%+v err=%v", runs, err)
	}
	if runs[0].ResultText == nil || *runs[0].ResultText != "Room 定时总结完成" {
		t.Fatalf("Room 定时任务应持久化执行结果: %+v", runs[0])
	}
}

func TestScheduledTaskPermissionHandlerApprovesAgentAllowedTools(t *testing.T) {
	handler := scheduledTaskPermissionHandler(protocol.Options{
		AllowedTools:    []string{"WebSearch", "nexus_automation"},
		DisallowedTools: []string{"Write"},
	})

	searchDecision, err := handler(context.Background(), sdkpermission.Request{ToolName: "WebSearch"})
	if err != nil {
		t.Fatalf("WebSearch 权限处理失败: %v", err)
	}
	if searchDecision.Behavior != sdkpermission.BehaviorAllow {
		t.Fatalf("Agent 已授权的 WebSearch 应允许后台执行: %+v", searchDecision)
	}
	wrappedSearchDecision, err := handler(context.Background(), sdkpermission.Request{ToolName: "mcp__brave_search__brave_web_search"})
	if err != nil {
		t.Fatalf("包装后的 WebSearch 权限处理失败: %v", err)
	}
	if wrappedSearchDecision.Behavior != sdkpermission.BehaviorAllow {
		t.Fatalf("WebSearch 授权应匹配常见搜索 MCP 工具名: %+v", wrappedSearchDecision)
	}

	wrappedDecision, err := handler(context.Background(), sdkpermission.Request{ToolName: "mcp__nexus_automation__get_scheduled_task_daily_report"})
	if err != nil {
		t.Fatalf("包装后的 nexus_automation 工具权限处理失败: %v", err)
	}
	if wrappedDecision.Behavior != sdkpermission.BehaviorAllow {
		t.Fatalf("nexus_automation 授权应匹配包装工具名: %+v", wrappedDecision)
	}

	writeDecision, err := handler(context.Background(), sdkpermission.Request{ToolName: "Write"})
	if err != nil {
		t.Fatalf("Write 权限处理失败: %v", err)
	}
	if writeDecision.Behavior != sdkpermission.BehaviorDeny {
		t.Fatalf("Agent 禁用的 Write 不应被后台授权: %+v", writeDecision)
	}

	questionDecision, err := handler(context.Background(), sdkpermission.Request{ToolName: "AskUserQuestion"})
	if err != nil {
		t.Fatalf("AskUserQuestion 权限处理失败: %v", err)
	}
	if questionDecision.Behavior != sdkpermission.BehaviorDeny {
		t.Fatalf("后台定时任务不应允许 AskUserQuestion: %+v", questionDecision)
	}
}

func TestServiceRunTaskNowExecutesScriptTaskWithoutAgentRunner(t *testing.T) {
	db := newAutomationTestDB(t)
	workspacePath := t.TempDir()
	service := NewService(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspacePath},
		db,
		nil,
		nil,
		nil,
		permissionctx.NewContext(),
		&fakeWorkspaceReader{},
		nil,
	)

	task, err := service.CreateTask(context.Background(), protocol.CreateJobInput{
		Name:          "脚本巡检",
		AgentID:       "agent-1",
		Instruction:   "echo automation-script-output",
		ExecutionKind: protocol.ExecutionKindScript,
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
	if task.ExecutionKind != protocol.ExecutionKindScript {
		t.Fatalf("execution_kind = %q, 期望 script", task.ExecutionKind)
	}

	result, err := service.RunTaskNow(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("RunTaskNow 失败: %v", err)
	}
	if result.Status != protocol.RunStatusRunning {
		t.Fatalf("期望脚本任务立即返回 running，实际 %s", result.Status)
	}

	waitFor(t, 2*time.Second, func() bool {
		items, listErr := service.ListTaskRuns(context.Background(), task.JobID)
		if listErr != nil || len(items) == 0 {
			return false
		}
		return items[0].Status == protocol.RunStatusSucceeded
	})
	runs, err := service.ListTaskRuns(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("ListTaskRuns 失败: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("期望 1 条 run 记录，实际 %d", len(runs))
	}
	run := runs[0]
	if run.SessionKey != "" || run.RoundID != "" || run.SessionID != nil {
		t.Fatalf("脚本任务不应绑定 Agent 会话: %+v", run)
	}
	if run.ResultText == nil || !strings.Contains(*run.ResultText, "automation-script-output") {
		t.Fatalf("脚本输出未持久化: %+v", run.ResultText)
	}
	if run.ArtifactPath == nil {
		t.Fatalf("脚本任务缺少运行产物路径")
	}
	artifactContent, readErr := os.ReadFile(filepath.Join(workspacePath, filepath.FromSlash(*run.ArtifactPath)))
	if readErr != nil {
		t.Fatalf("读取脚本运行产物失败: %v", readErr)
	}
	if !strings.Contains(string(artifactContent), "automation-script-output") {
		t.Fatalf("脚本运行产物缺少输出: %s", string(artifactContent))
	}
}

func TestServiceListTasksScopesByOwnerUserID(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		nil,
		nil,
		nil,
		&fakeWorkspaceReader{},
		nil,
	)
	ctxUser1 := authctx.WithPrincipal(context.Background(), &authctx.Principal{UserID: "user-1", Username: "user-1"})
	ctxUser2 := authctx.WithPrincipal(context.Background(), &authctx.Principal{UserID: "user-2", Username: "user-2"})

	taskUser1, err := service.CreateTask(ctxUser1, protocol.CreateJobInput{
		Name:        "用户 1 任务",
		AgentID:     "agent-1",
		Instruction: "user 1",
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
		t.Fatalf("创建 user-1 任务失败: %v", err)
	}
	if _, err = service.CreateTask(ctxUser2, protocol.CreateJobInput{
		Name:        "用户 2 任务",
		AgentID:     "agent-1",
		Instruction: "user 2",
		Schedule: protocol.Schedule{
			Kind:            protocol.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: protocol.SessionTarget{Kind: protocol.SessionTargetIsolated},
		Delivery:      protocol.DeliveryTarget{Mode: protocol.DeliveryModeNone},
		Enabled:       true,
	}); err != nil {
		t.Fatalf("创建 user-2 任务失败: %v", err)
	}

	user1Tasks, err := service.ListTasks(ctxUser1, "")
	if err != nil {
		t.Fatalf("ListTasks user-1 失败: %v", err)
	}
	if len(user1Tasks) != 1 || user1Tasks[0].JobID != taskUser1.JobID {
		t.Fatalf("user-1 scope 不正确: %+v", user1Tasks)
	}
	user2View, err := service.GetTask(ctxUser2, taskUser1.JobID)
	if err != nil {
		t.Fatalf("GetTask user-2 失败: %v", err)
	}
	if user2View != nil {
		t.Fatalf("user-2 不应读取 user-1 任务: %+v", user2View)
	}
	globalTasks, err := service.ListTasks(context.Background(), "")
	if err != nil {
		t.Fatalf("ListTasks global 失败: %v", err)
	}
	if len(globalTasks) != 2 {
		t.Fatalf("global scope 应看到 2 个任务，实际 %d", len(globalTasks))
	}
}

func TestServiceRunTaskNowRecordsOverlapSkippedRun(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	dm := &fakeDMRunner{permission: permission, delay: 200 * time.Millisecond}
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
		Name:        "重叠保护",
		AgentID:     "agent-1",
		Instruction: "慢任务",
		Schedule: protocol.Schedule{
			Kind:            protocol.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: protocol.SessionTarget{
			Kind:            protocol.SessionTargetBound,
			BoundSessionKey: protocol.BuildAgentSessionKey("agent-1", "ws", "dm", "overlap", ""),
		},
		Delivery:      protocol.DeliveryTarget{Mode: protocol.DeliveryModeNone},
		OverlapPolicy: protocol.OverlapPolicySkip,
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	first, err := service.RunTaskNow(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("第一次 RunTaskNow 失败: %v", err)
	}
	if first.Status != protocol.RunStatusRunning {
		t.Fatalf("第一次应返回 running，实际 %s", first.Status)
	}
	second, err := service.RunTaskNow(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("第二次 RunTaskNow 不应报错，应记录 skipped: %v", err)
	}
	if second.Status != protocol.RunStatusSkipped {
		t.Fatalf("第二次应返回 skipped，实际 %s", second.Status)
	}

	waitFor(t, 2*time.Second, func() bool {
		items, listErr := service.ListTaskRuns(context.Background(), task.JobID)
		if listErr != nil || len(items) != 2 {
			return false
		}
		hasSuccess := false
		hasSkipped := false
		for _, item := range items {
			hasSuccess = hasSuccess || item.Status == protocol.RunStatusSucceeded
			hasSkipped = hasSkipped || item.Status == protocol.RunStatusSkipped
		}
		return hasSuccess && hasSkipped
	})

	runs, err := service.ListTaskRuns(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("ListTaskRuns 失败: %v", err)
	}
	var skipped, succeeded *protocol.CronRun
	for i := range runs {
		switch runs[i].Status {
		case protocol.RunStatusSkipped:
			skipped = &runs[i]
		case protocol.RunStatusSucceeded:
			succeeded = &runs[i]
		}
	}
	if skipped == nil || skipped.ErrorMessage == nil {
		t.Fatalf("skipped run 应包含错误说明: %+v", runs)
	}
	if skipped.TriggerKind != "manual" {
		t.Fatalf("skipped run trigger_kind 不正确: %+v", skipped)
	}
	if succeeded == nil || succeeded.SessionKey == "" || succeeded.RoundID == "" || succeeded.SessionID == nil || succeeded.MessageCount == 0 {
		t.Fatalf("succeeded run 缺少执行诊断字段: %+v", succeeded)
	}
	if succeeded.ResultSummary == nil || strings.TrimSpace(*succeeded.ResultSummary) == "" {
		t.Fatalf("succeeded run 缺少 result_summary: %+v", succeeded)
	}
}

func TestServiceStartRunsDueTask(t *testing.T) {
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
	service.nowFn = func() time.Time {
		return time.Now().UTC()
	}

	_, err := service.CreateTask(context.Background(), protocol.CreateJobInput{
		Name:        "定时巡检",
		AgentID:     "agent-1",
		Instruction: "执行自动巡检",
		Schedule: protocol.Schedule{
			Kind:            protocol.ScheduleKindEvery,
			IntervalSeconds: intRef(1),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: protocol.SessionTarget{
			Kind:            protocol.SessionTargetBound,
			BoundSessionKey: protocol.BuildAgentSessionKey("agent-1", "ws", "dm", "scheduler", ""),
		},
		Delivery: protocol.DeliveryTarget{Mode: protocol.DeliveryModeNone},
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	if err = service.Start(context.Background()); err != nil {
		t.Fatalf("Start 失败: %v", err)
	}
	defer service.Stop()

	waitFor(t, 3*time.Second, func() bool {
		return len(dm.Requests()) > 0
	})
}

func TestRunTaskNowForMainTargetEnqueuesCronTextPayload(t *testing.T) {
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
	if _, err := service.UpdateHeartbeat(context.Background(), "agent-1", protocol.HeartbeatUpdateInput{
		Enabled:      true,
		EverySeconds: 3600,
		TargetMode:   protocol.HeartbeatTargetNone,
		AckMaxChars:  300,
	}); err != nil {
		t.Fatalf("UpdateHeartbeat 失败: %v", err)
	}

	task, err := service.CreateTask(context.Background(), protocol.CreateJobInput{
		Name:        "Main payload",
		AgentID:     "agent-1",
		Instruction: "follow up in main session",
		Schedule: protocol.Schedule{
			Kind:            protocol.ScheduleKindEvery,
			IntervalSeconds: intRef(60),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: protocol.SessionTarget{
			Kind:     protocol.SessionTargetMain,
			WakeMode: protocol.WakeModeNow,
		},
		Delivery: protocol.DeliveryTarget{Mode: protocol.DeliveryModeNone},
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	result, err := service.RunTaskNow(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("RunTaskNow 失败: %v", err)
	}
	if result.RunID == nil || result.Status != protocol.RunStatusQueuedToMain {
		t.Fatalf("main target 应返回 queued run: %+v", result)
	}

	var rawPayload string
	row := db.QueryRow(`SELECT payload FROM automation_system_events WHERE event_type='cron.trigger' ORDER BY created_at DESC, event_id DESC LIMIT 1`)
	if err = row.Scan(&rawPayload); err != nil {
		t.Fatalf("读取 cron.trigger payload 失败: %v", err)
	}
	payload := map[string]any{}
	if err = json.Unmarshal([]byte(rawPayload), &payload); err != nil {
		t.Fatalf("解析 cron.trigger payload 失败: %v", err)
	}
	if strings.TrimSpace(anyString(payload["text"])) != "follow up in main session" {
		t.Fatalf("cron.trigger payload.text 不正确: %v", payload)
	}
	if _, exists := payload["instruction"]; exists {
		t.Fatalf("cron.trigger 不应写 instruction 字段: %v", payload)
	}
	runs, err := service.ListTaskRuns(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("ListTaskRuns 失败: %v", err)
	}
	if len(runs) != 1 || runs[0].Status != protocol.RunStatusQueuedToMain || runs[0].SessionKey == "" {
		t.Fatalf("main target run ledger 不正确: %+v", runs)
	}
}

func TestRunTaskNowMarksMainEventFailedWhenWakeValidationFails(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		&fakeDMRunner{permission: permission},
		nil,
		permission,
		&fakeWorkspaceReader{},
		nil,
	)
	task, err := service.CreateTask(context.Background(), protocol.CreateJobInput{
		Name:        "main-wake-fail",
		AgentID:     "agent-1",
		Instruction: "wake failed",
		Schedule: protocol.Schedule{
			Kind:            protocol.ScheduleKindEvery,
			IntervalSeconds: intRef(60),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: protocol.SessionTarget{Kind: protocol.SessionTargetMain, WakeMode: protocol.WakeModeNow},
		Delivery:      protocol.DeliveryTarget{Mode: protocol.DeliveryModeNone},
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	if _, execErr := db.Exec(`UPDATE automation_cron_jobs SET wake_mode='bad-mode' WHERE job_id=?`, task.JobID); execErr != nil {
		t.Fatalf("写入坏 wake_mode 失败: %v", execErr)
	}

	if _, err = service.RunTaskNow(context.Background(), task.JobID); err == nil {
		t.Fatalf("期望 RunTaskNow 失败")
	}

	var status string
	row := db.QueryRow(
		`SELECT status FROM automation_system_events WHERE event_type='cron.trigger' ORDER BY created_at DESC, event_id DESC LIMIT 1`,
	)
	if scanErr := row.Scan(&status); scanErr != nil {
		t.Fatalf("读取 system event 状态失败: %v", scanErr)
	}
	if strings.TrimSpace(status) != "failed" {
		t.Fatalf("wake 失败后 event 应标记 failed，实际 %s", status)
	}
}
