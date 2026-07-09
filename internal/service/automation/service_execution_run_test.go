package automation

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"
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

	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "日报同步",
		AgentID:     "agent-1",
		Instruction: "整理今天的进展",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetBound,
			BoundSessionKey: protocol.BuildAgentSessionKey("agent-1", "ws", "dm", "manual", ""),
		},
		Delivery: automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	result, err := service.RunTaskNow(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("RunTaskNow 失败: %v", err)
	}
	if result.Status != automationdomain.RunStatusRunning {
		t.Fatalf("期望立即返回 running，实际为 %s", result.Status)
	}

	waitFor(t, 2*time.Second, func() bool {
		items, listErr := service.ListTaskRuns(context.Background(), task.JobID)
		if listErr != nil || len(items) == 0 {
			return false
		}
		return items[0].Status == automationdomain.RunStatusSucceeded
	})

	runs, err := service.ListTaskRuns(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("ListTaskRuns 失败: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("期望 1 条 run 记录，实际 %d", len(runs))
	}
	if runs[0].Status != automationdomain.RunStatusSucceeded {
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
	if runs[0].DeliveryStatus != automationdomain.DeliveryStatusNotRequired {
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
	if !strings.HasPrefix(requests[0].Content, "[cron:"+task.JobID+" 日报同步] ") ||
		!strings.Contains(requests[0].Content, "整理今天的进展") {
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

	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "暂停新闻日报",
		AgentID:     "agent-1",
		Instruction: "手动补跑今天新闻",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetBound,
			BoundSessionKey: protocol.BuildAgentSessionKey("agent-1", "ws", "dm", "manual", ""),
		},
		Delivery: automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Enabled:  false,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	result, err := service.RunTaskNow(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("RunTaskNow disabled task 失败: %v", err)
	}
	if result.Status != automationdomain.RunStatusRunning || result.RunID == nil {
		t.Fatalf("disabled task manual run should start once: %+v", result)
	}
	waitFor(t, 2*time.Second, func() bool {
		items, listErr := service.ListTaskRuns(context.Background(), task.JobID)
		return listErr == nil && len(items) == 1 && items[0].Status == automationdomain.RunStatusSucceeded
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

	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "新闻搜索",
		AgentID:     "agent-1",
		Instruction: "搜索今天的 AI 新闻并总结",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetBound,
			BoundSessionKey: protocol.BuildAgentSessionKey("agent-1", "ws", "dm", "permission-denied", ""),
		},
		Delivery: automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	result, err := service.RunTaskNow(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("RunTaskNow 下发失败: %v", err)
	}
	if result.Status != automationdomain.RunStatusRunning {
		t.Fatalf("期望立即返回 running，实际为 %s", result.Status)
	}

	waitFor(t, 2*time.Second, func() bool {
		runs, listErr := service.ListTaskRuns(context.Background(), task.JobID)
		return listErr == nil && len(runs) == 1 && runs[0].Status == automationdomain.RunStatusFailed
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
	if updatedTask == nil || updatedTask.LastRunStatus != automationdomain.RunStatusFailed || updatedTask.FailureStreak != 1 {
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

	report, err := service.GetDailyReport(context.Background(), automationdomain.CronDailyReportInput{
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

	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "重叠保护",
		AgentID:     "agent-1",
		Instruction: "慢任务",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetBound,
			BoundSessionKey: protocol.BuildAgentSessionKey("agent-1", "ws", "dm", "overlap", ""),
		},
		Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		OverlapPolicy: automationdomain.OverlapPolicySkip,
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	first, err := service.RunTaskNow(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("第一次 RunTaskNow 失败: %v", err)
	}
	if first.Status != automationdomain.RunStatusRunning {
		t.Fatalf("第一次应返回 running，实际 %s", first.Status)
	}
	second, err := service.RunTaskNow(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("第二次 RunTaskNow 不应报错，应记录 skipped: %v", err)
	}
	if second.Status != automationdomain.RunStatusSkipped {
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
			hasSuccess = hasSuccess || item.Status == automationdomain.RunStatusSucceeded
			hasSkipped = hasSkipped || item.Status == automationdomain.RunStatusSkipped
		}
		return hasSuccess && hasSkipped
	})

	runs, err := service.ListTaskRuns(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("ListTaskRuns 失败: %v", err)
	}
	var skipped, succeeded *automationdomain.CronRun
	for i := range runs {
		switch runs[i].Status {
		case automationdomain.RunStatusSkipped:
			skipped = &runs[i]
		case automationdomain.RunStatusSucceeded:
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

	_, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "定时巡检",
		AgentID:     "agent-1",
		Instruction: "执行自动巡检",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(1),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetBound,
			BoundSessionKey: protocol.BuildAgentSessionKey("agent-1", "ws", "dm", "scheduler", ""),
		},
		Delivery: automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
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
