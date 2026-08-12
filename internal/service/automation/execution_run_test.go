package automation

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	_ "modernc.org/sqlite"
)

func TestServiceRunTaskNowUpdatesRunLedger(t *testing.T) {
	db := newAutomationTestDB(t)
	workspacePath := newAutomationOwnerWorkspace(t, authctx.SystemUserID, "agent-1")
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
		Name:           "日报同步",
		AgentID:        "agent-1",
		Instruction:    "整理今天的进展",
		PermissionMode: automationdomain.PermissionModePlan,
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
	if task.PermissionMode != automationdomain.PermissionModePlan {
		t.Fatalf("permission_mode 未持久化: %+v", task)
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
	if requests[0].Content != "整理今天的进展" {
		t.Fatalf("下发指令不正确: %s", requests[0].Content)
	}
	if requests[0].AutomationRun == nil ||
		requests[0].AutomationRun.JobID != task.JobID ||
		requests[0].AutomationRun.RunID != runs[0].RunID ||
		requests[0].AutomationRun.JobName != task.Name {
		t.Fatalf("定时任务身份应通过可信上下文下发: %+v", requests[0].AutomationRun)
	}
	if requests[0].PermissionHandler == nil {
		t.Fatal("定时任务 DM 请求应使用非交互权限处理器")
	}
	if requests[0].PermissionMode != sdkpermission.ModePlan {
		t.Fatalf("定时任务 DM 请求未透传任务 permission_mode，实际 mode=%s", requests[0].PermissionMode)
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

func TestServiceRunTaskNowPersistsPermissionRequestAndResumesSameRun(t *testing.T) {
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
		return listErr == nil && len(runs) == 1 &&
			runs[0].Status == automationdomain.RunStatusPending &&
			runs[0].BlockState == automationdomain.RunBlockStateAwaitingApproval
	})
	runs, err := service.ListTaskRuns(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("ListTaskRuns 失败: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("期望 1 条 run，实际 %d", len(runs))
	}
	run := runs[0]
	if run.BlockedRequestID == "" || run.FinishedAt != nil || run.EffectStarted {
		t.Fatalf("权限等待应保留可恢复 run 且尚未越过副作用边界: %+v", run)
	}

	updatedTask, err := service.GetTask(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("GetTask 失败: %v", err)
	}
	if updatedTask == nil || updatedTask.PermissionState != automationdomain.TaskPermissionStateAwaitingApproval ||
		updatedTask.PendingPermissionRequestID != run.BlockedRequestID || updatedTask.FailureStreak != 0 {
		t.Fatalf("任务应进入待审批状态且不累计执行失败: %+v", updatedTask)
	}

	ownerCtx := contextForOwner(context.Background(), task.OwnerUserID)
	requests, err := service.ListPermissionRequests(ownerCtx, automationdomain.PermissionRequestStatusPending, task.JobID)
	if err != nil {
		t.Fatalf("ListPermissionRequests 失败: %v", err)
	}
	if len(requests) != 1 || requests[0].RequestID != run.BlockedRequestID ||
		requests[0].Capability.ToolName != "WebSearch" || !requests[0].ResumeSafe {
		t.Fatalf("持久审批请求不完整: %+v", requests)
	}
	decision, err := service.ResolvePermissionRequest(
		ownerCtx,
		requests[0].RequestID,
		permissionDecisionInputForRequest(requests[0], automationdomain.PermissionDecisionAllowTask),
	)
	if err != nil {
		t.Fatalf("ResolvePermissionRequest 失败: %v", err)
	}
	if !decision.ResumeStarted || decision.Request == nil ||
		decision.Request.Status != automationdomain.PermissionRequestStatusApproved {
		t.Fatalf("allow_task 应自动恢复同一 logical run: %+v", decision)
	}
	dispatched := dm.Requests()
	if len(dispatched) != 2 || dispatched[1].Content != task.Instruction ||
		dispatched[1].AutomationRun == nil ||
		dispatched[1].AutomationRun.JobID != task.JobID ||
		dispatched[1].AutomationRun.RunID != run.RunID ||
		dispatched[1].AutomationRun.ResumeToolName != "WebSearch" {
		t.Fatalf("审批后的新 attempt 必须通过可信上下文要求重试原工具: %+v", dispatched)
	}

	waitFor(t, 2*time.Second, func() bool {
		items, listErr := service.ListTaskRuns(context.Background(), task.JobID)
		return listErr == nil && len(items) == 1 && items[0].Status == automationdomain.RunStatusSucceeded
	})
	runs, err = service.ListTaskRuns(context.Background(), task.JobID)
	if err != nil || len(runs) != 1 {
		t.Fatalf("读取恢复后的 run 失败: runs=%+v err=%v", runs, err)
	}
	if runs[0].RunID != run.RunID || runs[0].Attempts != 2 || runs[0].Status != automationdomain.RunStatusSucceeded {
		t.Fatalf("审批恢复必须复用 logical run_id 并创建新 attempt: %+v", runs[0])
	}
	updatedTask, err = service.GetTask(context.Background(), task.JobID)
	if err != nil || updatedTask == nil || updatedTask.PermissionState != automationdomain.TaskPermissionStateReady ||
		updatedTask.FailureStreak != 0 {
		t.Fatalf("审批恢复成功后任务状态不正确: task=%+v err=%v", updatedTask, err)
	}
	if !slices.ContainsFunc(updatedTask.PermissionPolicy.Grants, func(grant automationdomain.TaskPermissionGrant) bool {
		return grant.Source == automationdomain.PermissionGrantSourceUserApproval &&
			grant.Capability.ToolName == "WebSearch"
	}) {
		t.Fatalf("allow_task 未写入任务级 capability grant: %+v", updatedTask.PermissionPolicy)
	}
	second, err := service.RunTaskNow(context.Background(), task.JobID)
	if err != nil || second.RunID == nil {
		t.Fatalf("持久授权后的第二次运行启动失败: result=%+v err=%v", second, err)
	}
	waitFor(t, 2*time.Second, func() bool {
		items, listErr := service.ListTaskRuns(context.Background(), task.JobID)
		return listErr == nil && len(items) == 2 && items[0].Status == automationdomain.RunStatusSucceeded
	})
	pendingRequests, err := service.ListPermissionRequests(ownerCtx, automationdomain.PermissionRequestStatusPending, task.JobID)
	if err != nil || len(pendingRequests) != 0 {
		t.Fatalf("任务级授权后不应重复请求相同 capability: requests=%+v err=%v", pendingRequests, err)
	}
}

func TestPermissionResumeFailsWhenAgentDoesNotRetryApprovedTool(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	dm := &fakeDMRunner{
		permission:               permission,
		requiredTool:             "WebSearch",
		skipPermissionAfterFirst: true,
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
		Name:        "权限续跑证据校验",
		AgentID:     "agent-1",
		Instruction: "搜索今天的 AI 新闻",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetBound,
			BoundSessionKey: protocol.BuildAgentSessionKey("agent-1", "ws", "dm", "permission-resume-evidence", ""),
		},
		Delivery: automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	if _, err = service.RunTaskNow(context.Background(), task.JobID); err != nil {
		t.Fatalf("RunTaskNow 失败: %v", err)
	}
	ownerCtx := contextForOwner(context.Background(), task.OwnerUserID)
	waitFor(t, 2*time.Second, func() bool {
		requests, listErr := service.ListPermissionRequests(ownerCtx, automationdomain.PermissionRequestStatusPending, task.JobID)
		return listErr == nil && len(requests) == 1
	})
	requests, err := service.ListPermissionRequests(ownerCtx, automationdomain.PermissionRequestStatusPending, task.JobID)
	if err != nil || len(requests) != 1 {
		t.Fatalf("读取审批请求失败: requests=%+v err=%v", requests, err)
	}
	decision, err := service.ResolvePermissionRequest(
		ownerCtx,
		requests[0].RequestID,
		permissionDecisionInputForRequest(requests[0], automationdomain.PermissionDecisionAllowTask),
	)
	if err != nil || decision == nil || !decision.ResumeStarted {
		t.Fatalf("审批续跑启动失败: decision=%+v err=%v", decision, err)
	}
	waitFor(t, 2*time.Second, func() bool {
		runs, listErr := service.ListTaskRuns(context.Background(), task.JobID)
		return listErr == nil && len(runs) == 1 && runs[0].Status == automationdomain.RunStatusFailed
	})
	runs, err := service.ListTaskRuns(context.Background(), task.JobID)
	if err != nil || len(runs) != 1 || runs[0].ErrorMessage == nil ||
		!strings.Contains(*runs[0].ErrorMessage, "没有重新调用已授权工具 WebSearch") {
		t.Fatalf("未重试工具的续跑不能记录为成功: runs=%+v err=%v", runs, err)
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
	var skipped, succeeded *automationdomain.ScheduledTaskRun
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
	if skipped.TriggerKind != automationdomain.TriggerKindManual {
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
