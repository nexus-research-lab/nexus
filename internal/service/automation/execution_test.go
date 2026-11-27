// execution_test.go 覆盖自动化任务运行、目标解析、配置控制、命令 Schema、脚本边界与迁移测试。
package automation

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	automationexec "github.com/nexus-research-lab/nexus/internal/automation"
	automationdomain "github.com/nexus-research-lab/nexus/internal/automation/types"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/mcp/command"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	automationstore "github.com/nexus-research-lab/nexus/internal/storage/automation"
	"github.com/pressly/goose/v3"
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
	job := automationdomain.ScheduledTask{AgentID: "agent-1", Instruction: "整理 Room 今日进展"}
	if err := service.dispatchJobToSession(context.Background(), job, "", sessionKey, "round-room-1", nil, nil); err != nil {
		t.Fatalf("dispatchJobToSession 失败: %v", err)
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
	if len(requests[0].TargetAgentIDs) != 1 || requests[0].TargetAgentIDs[0] != "agent-1" {
		t.Fatalf("Room 定时任务应显式指定目标成员: %+v", requests[0].TargetAgentIDs)
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
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "room-summary",
		AgentID:     "agent-1",
		Instruction: "整理 Room 今日进展",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetBound,
			BoundSessionKey: sessionKey,
		},
		Delivery: automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	result, err := service.RunTaskNow(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("RunTaskNow 不应拒绝 Room 执行会话: %v", err)
	}
	if result.SessionKey != sessionKey || result.Status != automationdomain.RunStatusRunning {
		t.Fatalf("Room 定时任务下发结果不正确: %+v", result)
	}
	requests := roomRunner.Requests()
	if len(requests) != 1 || requests[0].SessionKey != sessionKey || requests[0].ConversationID != "conversation-1" {
		t.Fatalf("Room runner 请求不正确: %+v", requests)
	}
	if len(requests[0].TargetAgentIDs) != 1 || requests[0].TargetAgentIDs[0] != "agent-1" {
		t.Fatalf("Room runner 应收到显式目标成员: %+v", requests[0].TargetAgentIDs)
	}

	waitFor(t, 2*time.Second, func() bool {
		runs, listErr := service.ListTaskRuns(context.Background(), task.JobID)
		return listErr == nil && len(runs) == 1 && runs[0].Status == automationdomain.RunStatusSucceeded
	})
	runs, err := service.ListTaskRuns(context.Background(), task.JobID)
	if err != nil || len(runs) != 1 {
		t.Fatalf("读取 Room 定时任务 run 失败: runs=%+v err=%v", runs, err)
	}
	if runs[0].ResultText == nil || *runs[0].ResultText != "Room 定时总结完成" {
		t.Fatalf("Room 定时任务应持久化执行结果: %+v", runs[0])
	}
}

func TestServiceCreateTaskRejectsRoomTargetAgentOutsideMembers(t *testing.T) {
	db := newAutomationTestDB(t)
	roomRunner := &fakeRoomRunner{
		contexts: map[string]*protocol.ConversationContextAggregate{
			"conversation-1": {
				Room: protocol.RoomRecord{ID: "room-1", RoomType: protocol.RoomTypeGroup},
				Members: []protocol.MemberRecord{
					{MemberType: protocol.MemberTypeAgent, MemberAgentID: "agent-2"},
				},
				Conversation: protocol.ConversationRecord{ID: "conversation-1", RoomID: "room-1"},
			},
		},
	}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		nil,
		roomRunner,
		nil,
		&fakeWorkspaceReader{},
		nil,
	)
	_, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "room-summary",
		AgentID:     "agent-1",
		Instruction: "整理 Room 今日进展",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetBound,
			BoundSessionKey: protocol.BuildRoomSharedSessionKey("conversation-1"),
		},
		Delivery: automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Enabled:  true,
	})
	if err == nil || !strings.Contains(err.Error(), "不是目标 Room 的成员") {
		t.Fatalf("非 Room 成员不应创建 Room 定时任务: %v", err)
	}
}

func TestServiceRunTaskNowExecutesScriptTaskWithoutAgentRunner(t *testing.T) {
	db := newAutomationTestDB(t)
	workspacePath := newAutomationOwnerWorkspace(t, authctx.SystemUserID, "agent-1")
	service := NewService(
		config.Config{
			DatabaseDriver: "sqlite",
			WorkspacePath:  workspacePath,
			AppMode:        "desktop",
		},
		db,
		nil,
		nil,
		nil,
		permissionctx.NewContext(),
		&fakeWorkspaceReader{},
		nil,
	)

	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:          "脚本巡检",
		AgentID:       "agent-1",
		Instruction:   "echo automation-script-output",
		ExecutionKind: automationdomain.ExecutionKindScript,
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated},
		Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	if task.ExecutionKind != automationdomain.ExecutionKindScript {
		t.Fatalf("execution_kind = %q, 期望 script", task.ExecutionKind)
	}

	result, err := service.RunTaskNow(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("RunTaskNow 失败: %v", err)
	}
	if result.Status != automationdomain.RunStatusRunning {
		t.Fatalf("期望脚本任务立即返回 running，实际 %s", result.Status)
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

func TestDeleteTaskCancelsActiveScriptBeforeFinalizing(t *testing.T) {
	db := newAutomationTestDB(t)
	workspacePath := newAutomationOwnerWorkspace(t, authctx.SystemUserID, "agent-1")
	service := NewService(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspacePath, AppMode: "desktop"},
		db, nil, nil, nil, permissionctx.NewContext(), &fakeWorkspaceReader{}, nil,
	)
	startedName := "delete-script-started"
	sideEffectName := "delete-script-side-effect"
	// The delayed write is performed by a background pipeline, not the shell
	// itself. Killing only /bin/sh would let that child outlive deletion.
	script := "touch " + startedName + "; (sleep 1; touch " + sideEffectName + ") | cat >/dev/null & wait"
	if runtime.GOOS == "windows" {
		script = "echo started>" + startedName + " & timeout /T 2 /NOBREAK >NUL & echo side-effect>" + sideEffectName
	}
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name: "cancel active script", AgentID: "agent-1", Instruction: script,
		ExecutionKind: automationdomain.ExecutionKindScript,
		Schedule:      automationdomain.Schedule{Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: intRef(3600), Timezone: "UTC"},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated},
		Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone}, Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateTask(): %v", err)
	}
	result, err := service.RunTaskNow(context.Background(), task.JobID)
	if err != nil || result == nil || result.RunID == nil {
		t.Fatalf("RunTaskNow(): result=%+v err=%v", result, err)
	}
	startedPath := filepath.Join(workspacePath, startedName)
	sideEffectPath := filepath.Join(workspacePath, sideEffectName)
	waitFor(t, 2*time.Second, func() bool {
		_, statErr := os.Stat(startedPath)
		return statErr == nil
	})
	deleted, err := service.DeleteTaskAtVersion(context.Background(), task.JobID, task.ConfigurationVersion)
	if err != nil {
		if !TaskDeletionPrepared(err) || !errors.Is(err, ErrExecutionAttemptOwnershipUnconfirmed) {
			t.Fatalf("unconfirmed process-tree cancellation must require review: %v", err)
		}
		claimed, loadErr := service.repository.GetScheduledTask(context.Background(), task.OwnerUserID, task.JobID)
		if loadErr != nil || claimed == nil || claimed.DeletionState != automationdomain.TaskDeletionStateReviewRequired {
			t.Fatalf("uncertain cancellation must retain review claim: task=%+v err=%v", claimed, loadErr)
		}
		waitDuration := 1200 * time.Millisecond
		if runtime.GOOS == "windows" {
			waitDuration = 2200 * time.Millisecond
		}
		time.Sleep(waitDuration)
		if _, statErr := os.Stat(sideEffectPath); !os.IsNotExist(statErr) {
			t.Fatalf("cancelled process-tree child continued side effects: stat err=%v", statErr)
		}
		return
	}
	if deleted == nil || !deleted.CancelledActiveRun || deleted.CancelledRunID != *result.RunID {
		t.Fatalf("delete result lost the script run stopped by cleanup: %+v", deleted)
	}
	time.Sleep(1200 * time.Millisecond)
	if _, statErr := os.Stat(sideEffectPath); !os.IsNotExist(statErr) {
		t.Fatalf("script continued side effects after delete: stat err=%v", statErr)
	}
	run, err := service.repository.GetRun(context.Background(), task.OwnerUserID, task.JobID, *result.RunID)
	if err != nil || run == nil || run.Status != automationdomain.RunStatusCancelled {
		t.Fatalf("deleted script run was not durably cancelled: run=%+v err=%v", run, err)
	}
}

func TestServiceRunTaskNowNeverFallsBackToHostForServerScript(t *testing.T) {
	db := newAutomationTestDB(t)
	workspacePath := newAutomationOwnerWorkspace(t, authctx.SystemUserID, "agent-1")
	service := NewService(
		config.Config{
			DatabaseDriver:       "sqlite",
			WorkspacePath:        workspacePath,
			AppMode:              "server",
			RuntimeIsolationMode: "off",
		},
		db,
		nil,
		nil,
		nil,
		permissionctx.NewContext(),
		&fakeWorkspaceReader{},
		nil,
	)

	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:          "server script isolation",
		AgentID:       "agent-1",
		Instruction:   "printf should-not-run > host-fallback-marker.txt",
		ExecutionKind: automationdomain.ExecutionKindScript,
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated},
		Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Enabled:       true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	result, err := service.RunTaskNow(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("RunTaskNow 失败: %v", err)
	}
	if result.Status != automationdomain.RunStatusRunning {
		t.Fatalf("期望脚本任务立即返回 running，实际 %s", result.Status)
	}

	waitFor(t, 2*time.Second, func() bool {
		runs, listErr := service.ListTaskRuns(context.Background(), task.JobID)
		return listErr == nil &&
			len(runs) == 1 &&
			runs[0].Status == automationdomain.RunStatusFailed
	})
	runs, err := service.ListTaskRuns(context.Background(), task.JobID)
	if err != nil || len(runs) != 1 {
		t.Fatalf("读取 server 脚本 run 失败: runs=%+v err=%v", runs, err)
	}
	if runs[0].ErrorMessage == nil ||
		!strings.Contains(*runs[0].ErrorMessage, "requires runtime isolation enforce") {
		t.Fatalf("server 脚本应因隔离未启用而失败: %+v", runs[0].ErrorMessage)
	}
	if _, statErr := os.Stat(filepath.Join(workspacePath, "host-fallback-marker.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("server 脚本不应回退到宿主 shell，marker stat=%v", statErr)
	}
}

func TestCreateTaskRequestIDIsIdempotentBeforeCapacityCheck(t *testing.T) {
	service := NewService(
		config.Config{
			DatabaseDriver:                   "sqlite",
			AutomationMaxEnabledTasksPerUser: 1,
		},
		newAutomationTestDB(t),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	input := automationConfigurationTaskInput("stable-create")
	input.RequestID = "request-stable-create"

	first, err := service.CreateTask(context.Background(), input)
	if err != nil {
		t.Fatalf("first CreateTask: %v", err)
	}
	replayed, err := service.CreateTask(context.Background(), input)
	if err != nil {
		t.Fatalf("idempotent replay must bypass capacity check: %v", err)
	}
	if replayed.JobID != first.JobID || replayed.ConfigurationVersion != first.ConfigurationVersion {
		t.Fatalf("replay = %+v, want same task/version as %+v", replayed, first)
	}
	events, err := service.ListTaskEvents(context.Background(), first.JobID, 20)
	if err != nil {
		t.Fatalf("ListTaskEvents: %v", err)
	}
	createEvents := 0
	for _, event := range events {
		if event.Action == automationdomain.TaskEventActionCreate {
			createEvents++
		}
	}
	if createEvents != 1 {
		t.Fatalf("create events = %d, want 1", createEvents)
	}

	conflicting := input
	conflicting.Name = "different intent"
	if _, err = service.CreateTask(context.Background(), conflicting); !errors.Is(err, automationdomain.ErrCreateRequestConflict) {
		t.Fatalf("conflicting replay error = %v, want ErrCreateRequestConflict", err)
	}
}

func TestCreateTaskReplayReportsCommittedTaskWasDeleted(t *testing.T) {
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		newAutomationTestDB(t),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	input := automationConfigurationTaskInput("deleted-create-result")
	input.RequestID = "request-deleted-create-result"
	created, err := service.CreateTask(context.Background(), input)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	ownerCtx := contextForOwner(context.Background(), created.OwnerUserID)
	status, err := service.GetTaskCreateRequestStatus(ownerCtx, input.RequestID)
	if err != nil {
		t.Fatalf("GetTaskCreateRequestStatus committed: %v", err)
	}
	if status.Status != automationdomain.TaskCreateRequestStatusCommitted ||
		status.Task == nil || status.Task.JobID != created.JobID {
		t.Fatalf("committed create status = %+v", status)
	}
	if err = service.repository.DeleteScheduledTask(context.Background(), created.OwnerUserID, created.JobID); err != nil {
		t.Fatalf("DeleteScheduledTask: %v", err)
	}
	if _, err = service.CreateTask(context.Background(), input); !errors.Is(err, automationdomain.ErrCreateRequestResultGone) {
		t.Fatalf("deleted replay error = %v, want ErrCreateRequestResultGone", err)
	}
	status, err = service.GetTaskCreateRequestStatus(ownerCtx, input.RequestID)
	if err != nil {
		t.Fatalf("GetTaskCreateRequestStatus gone: %v", err)
	}
	if status.Status != automationdomain.TaskCreateRequestStatusGone || status.Task != nil {
		t.Fatalf("gone create status = %+v", status)
	}
	missing, err := service.GetTaskCreateRequestStatus(ownerCtx, "request-never-accepted")
	if err != nil {
		t.Fatalf("GetTaskCreateRequestStatus missing: %v", err)
	}
	if missing.Status != automationdomain.TaskCreateRequestStatusNotFound || missing.Task != nil {
		t.Fatalf("missing create status = %+v", missing)
	}
}

func TestScheduledTaskCASRejectsStaleUpdateAndDelete(t *testing.T) {
	db := newAutomationTestDB(t)
	firstService := NewService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil, nil, nil, nil, nil)
	secondService := NewService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil, nil, nil, nil, nil)

	created, err := firstService.CreateTask(context.Background(), automationConfigurationTaskInput("versioned"))
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if created.ConfigurationVersion != 1 {
		t.Fatalf("initial version = %d, want 1", created.ConfigurationVersion)
	}
	stale, err := secondService.GetTask(context.Background(), created.JobID)
	if err != nil {
		t.Fatalf("GetTask stale snapshot: %v", err)
	}
	name := "updated"
	updated, err := firstService.UpdateTaskAtVersion(
		context.Background(),
		created.JobID,
		created.ConfigurationVersion,
		automationdomain.UpdateJobInput{Name: &name},
	)
	if err != nil {
		t.Fatalf("UpdateTaskAtVersion: %v", err)
	}
	if updated.ConfigurationVersion != created.ConfigurationVersion+1 {
		t.Fatalf("updated version = %d, want %d", updated.ConfigurationVersion, created.ConfigurationVersion+1)
	}
	staleName := "stale overwrite"
	if _, err = secondService.UpdateTaskAtVersion(
		context.Background(),
		created.JobID,
		stale.ConfigurationVersion,
		automationdomain.UpdateJobInput{Name: &staleName},
	); !errors.Is(err, automationdomain.ErrConfigurationVersionConflict) {
		t.Fatalf("stale update error = %v, want version conflict", err)
	}
	if _, err = secondService.DeleteTaskAtVersion(
		context.Background(),
		created.JobID,
		stale.ConfigurationVersion,
	); !errors.Is(err, automationdomain.ErrConfigurationVersionConflict) {
		t.Fatalf("stale delete error = %v, want version conflict", err)
	}
	if _, err = secondService.DeleteTaskAtVersion(
		context.Background(),
		created.JobID,
		updated.ConfigurationVersion,
	); err != nil {
		t.Fatalf("current-version delete: %v", err)
	}
	persisted, err := firstService.GetTask(context.Background(), created.JobID)
	if err != nil {
		t.Fatalf("GetTask after delete: %v", err)
	}
	if persisted != nil {
		t.Fatalf("task still exists after versioned delete: %+v", persisted)
	}
}

func TestVersionedManualRunRechecksConfigurationBeforeRuntimeClaim(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil, nil, nil, nil, nil)
	created, err := service.CreateTask(context.Background(), automationConfigurationTaskInput("run-version"))
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	name := "new configuration"
	if _, err = service.UpdateTaskAtVersion(
		context.Background(),
		created.JobID,
		created.ConfigurationVersion,
		automationdomain.UpdateJobInput{Name: &name},
	); err != nil {
		t.Fatalf("UpdateTaskAtVersion: %v", err)
	}

	staleVersion := created.ConfigurationVersion
	result, err := service.startJobExecutionControlledAtVersion(
		context.Background(),
		*created,
		automationdomain.TriggerKindManual,
		time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC),
		&staleVersion,
	)
	if result != nil || !errors.Is(err, automationdomain.ErrConfigurationVersionConflict) {
		t.Fatalf("stale manual run = (%+v, %v), want nil/version conflict", result, err)
	}
	var runningRunID sql.NullString
	if err = db.QueryRow(
		`SELECT running_run_id FROM automation_scheduled_tasks WHERE job_id = ?`,
		created.JobID,
	).Scan(&runningRunID); err != nil {
		t.Fatalf("read runtime after stale run: %v", err)
	}
	if runningRunID.Valid && runningRunID.String != "" {
		t.Fatalf("stale manual run changed running_run_id: %+v", runningRunID)
	}
}

func TestScheduledTaskDeleteRollsBackRunFinalizationWhenDeleteFails(t *testing.T) {
	db := newAutomationTestDB(t)
	service := NewService(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil, nil, nil, nil, nil)
	task, err := service.CreateTask(context.Background(), automationConfigurationTaskInput("delete-atomic"))
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	activeRunID := "run-delete-active"
	if err = service.repository.InsertRunPending(context.Background(), automationstore.RunPendingInput{
		RunID:       activeRunID,
		JobID:       task.JobID,
		OwnerUserID: task.OwnerUserID,
		TriggerKind: automationdomain.TriggerKindManual,
		Status:      automationdomain.RunStatusPending,
	}); err != nil {
		t.Fatalf("InsertRunPending active: %v", err)
	}
	if err = service.repository.MarkRunRunning(context.Background(), activeRunID, time.Now().UTC()); err != nil {
		t.Fatalf("MarkRunRunning: %v", err)
	}
	if _, err = db.Exec(
		`UPDATE automation_scheduled_tasks SET running_run_id = ?, running_started_at = CURRENT_TIMESTAMP WHERE job_id = ?`,
		activeRunID,
		task.JobID,
	); err != nil {
		t.Fatalf("set active task runtime: %v", err)
	}
	deliveryRunID := "run-delete-delivery"
	if err = service.repository.InsertRunPending(context.Background(), automationstore.RunPendingInput{
		RunID:          deliveryRunID,
		JobID:          task.JobID,
		OwnerUserID:    task.OwnerUserID,
		TriggerKind:    automationdomain.TriggerKindScheduled,
		Status:         automationdomain.RunStatusSucceeded,
		DeliveryMode:   automationdomain.DeliveryModeExplicit,
		DeliveryStatus: automationdomain.DeliveryStatusFailed,
	}); err != nil {
		t.Fatalf("InsertRunPending delivery: %v", err)
	}
	if _, err = db.Exec(`
CREATE TRIGGER fail_atomic_task_delete
BEFORE DELETE ON automation_scheduled_tasks
WHEN OLD.job_id = '` + task.JobID + `'
BEGIN
    SELECT RAISE(FAIL, 'injected task delete failure');
END;
`); err != nil {
		t.Fatalf("create delete failure trigger: %v", err)
	}

	if _, err = service.DeleteTaskAtVersion(context.Background(), task.JobID, task.ConfigurationVersion); err == nil {
		t.Fatal("DeleteTaskAtVersion should fail when delete trigger aborts")
	} else if !TaskDeletionPrepared(err) {
		t.Fatalf("delete failure should preserve prepared-stage evidence: %v", err)
	}
	persisted, err := service.repository.GetScheduledTask(context.Background(), task.OwnerUserID, task.JobID)
	if err != nil || persisted == nil {
		t.Fatalf("durable deletion claim must remain: task=%+v err=%v", persisted, err)
	}
	if persisted.DeletionState != automationdomain.TaskDeletionStateDeleting ||
		persisted.DeletionToken == "" || persisted.Enabled ||
		persisted.ConfigurationVersion != task.ConfigurationVersion+1 {
		t.Fatalf("delete failure lost its exact durable claim: %+v", persisted)
	}
	activeRun, err := service.repository.GetRun(context.Background(), task.OwnerUserID, task.JobID, activeRunID)
	if err != nil || activeRun == nil || activeRun.Status != automationdomain.RunStatusRunning {
		t.Fatalf("active run cancellation must roll back: run=%+v err=%v", activeRun, err)
	}
	deliveryRun, err := service.repository.GetRun(context.Background(), task.OwnerUserID, task.JobID, deliveryRunID)
	if err != nil || deliveryRun == nil ||
		deliveryRun.DeliveryStatus != automationdomain.DeliveryStatusFailed ||
		deliveryRun.DeliveryDeadLetterAt != nil {
		t.Fatalf("delivery dead-letter must roll back: run=%+v err=%v", deliveryRun, err)
	}
	if _, err = db.Exec(`DROP TRIGGER fail_atomic_task_delete`); err != nil {
		t.Fatalf("drop delete failure trigger: %v", err)
	}
	if _, err = service.DeleteTaskAtVersion(context.Background(), task.JobID, task.ConfigurationVersion); err != nil {
		t.Fatalf("repeated DELETE must resume the exact deletion claim: %v", err)
	}
	persisted, err = service.repository.GetScheduledTask(context.Background(), task.OwnerUserID, task.JobID)
	if err != nil || persisted != nil {
		t.Fatalf("repeated DELETE did not finish durable cleanup: task=%+v err=%v", persisted, err)
	}
}

func TestScheduledTaskUpdateAcceptsUnchangedHistoricalIMDelivery(t *testing.T) {
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		newAutomationTestDB(t),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	grant := &mutableAutomationDeliveryGrant{allowed: true}
	service.SetDeliveryGrantResolver(grant)
	input := automationConfigurationTaskInput("historical-im")
	input.Source = automationdomain.Source{Kind: automationdomain.SourceKindUserPage}
	input.Delivery = automationdomain.DeliveryTarget{
		Mode:       automationdomain.DeliveryModeLast,
		SessionKey: "agent:agent-1:weixin-personal:dm:acct:old-account:old-contact",
	}
	created, err := service.CreateTask(context.Background(), input)
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	grant.setAllowed(false)
	updatedName := "historical-im-edited"
	updated, err := service.UpdateTask(
		context.Background(),
		created.JobID,
		automationdomain.UpdateJobInput{
			Name:     &updatedName,
			Delivery: &created.Delivery,
			Source:   &created.Source,
		},
	)
	if err != nil {
		t.Fatalf("unchanged historical IM route should remain editable: %v", err)
	}
	if updated.Name != updatedName || updated.Delivery != created.Delivery ||
		updated.Source != created.Source {
		t.Fatalf("historical task update changed route/source: %+v", updated)
	}

	changedDelivery := created.Delivery
	changedDelivery.SessionKey = "agent:agent-1:weixin-personal:dm:acct:new-account:new-contact"
	if _, err = service.UpdateTask(
		context.Background(),
		created.JobID,
		automationdomain.UpdateJobInput{Delivery: &changedDelivery},
	); !errors.Is(err, automationdomain.ErrTaskDeliverySessionUnavailable) {
		t.Fatalf("changed IM route must return the stable unavailable-session error, got %v", err)
	}
}

func TestScheduledTaskConcurrentCASHasSingleWinner(t *testing.T) {
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		newAutomationTestDB(t),
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
	)
	created, err := service.CreateTask(context.Background(), automationConfigurationTaskInput("race"))
	if err != nil {
		t.Fatalf("CreateTask: %v", err)
	}

	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, value := range []string{"left", "right"} {
		value := value
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, updateErr := service.UpdateTaskAtVersion(
				context.Background(),
				created.JobID,
				created.ConfigurationVersion,
				automationdomain.UpdateJobInput{Name: &value},
			)
			results <- updateErr
		}()
	}
	wg.Wait()
	close(results)

	successes := 0
	conflicts := 0
	for result := range results {
		switch {
		case result == nil:
			successes++
		case errors.Is(result, automationdomain.ErrConfigurationVersionConflict):
			conflicts++
		default:
			t.Fatalf("unexpected concurrent update error: %v", result)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d, want 1/1", successes, conflicts)
	}
}

func TestAutomationCommandCreateRetryReplaysSameTask(t *testing.T) {
	fixture := newAutomationCommandFixture(t, "ok")
	createArgs := map[string]any{
		"request_id":     "mcp-create-stable",
		"name":           "stable",
		"instruction":    "run once",
		"execution_mode": "temporary",
		"reply_mode":     "none",
		"schedule": map[string]any{
			"kind":           "interval",
			"interval_value": 10,
			"interval_unit":  "minutes",
		},
	}
	firstResult, firstError := callAutomationCommand(
		t,
		fixture.Service,
		fixture.ServerContext,
		"create_scheduled_task",
		createArgs,
	)
	if firstError {
		t.Fatalf("first create_scheduled_task: %s", automationCommandText(t, firstResult))
	}
	secondResult, secondError := callAutomationCommand(
		t,
		fixture.Service,
		fixture.ServerContext,
		"create_scheduled_task",
		createArgs,
	)
	if secondError {
		t.Fatalf("replayed create_scheduled_task: %s", automationCommandText(t, secondResult))
	}
	first := decodeAutomationCommandJSON[automationdomain.ScheduledTask](t, firstResult)
	second := decodeAutomationCommandJSON[automationdomain.ScheduledTask](t, secondResult)
	if first.JobID != second.JobID || first.ConfigurationVersion != second.ConfigurationVersion {
		t.Fatalf("command replay created a different task: first=%+v second=%+v", first, second)
	}
	tasks, err := fixture.Service.ListTasks(
		automationCommandTestOwnerContext(fixture.ServerContext.OwnerUserID),
		fixture.ServerContext.AgentID,
	)
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("task count after replay = %d, want 1", len(tasks))
	}
}

func automationConfigurationTaskInput(name string) automationdomain.CreateJobInput {
	interval := 300
	return automationdomain.CreateJobInput{
		Name:        name,
		AgentID:     "agent-1",
		Schedule:    automationdomain.Schedule{Kind: automationdomain.ScheduleKindEvery, IntervalSeconds: &interval, Timezone: "UTC"},
		Instruction: "run",
		SessionTarget: automationdomain.SessionTarget{
			Kind: automationdomain.SessionTargetIsolated,
		},
		Delivery: automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Source: automationdomain.Source{
			Kind: automationdomain.SourceKindAgent,
		},
		Enabled: true,
	}
}

func TestAutomationExactContractAndRawInputAgree(t *testing.T) {
	fixture := newAutomationCommandFixture(t, "ok")
	contract, err := fixture.Service.RuntimeCommandContract(context.Background(), fixture.ServerContext, "create")
	if err != nil || len(contract.Operations) != 1 {
		t.Fatalf("contract=%+v err=%v", contract, err)
	}
	schema := contract.Operations["create"].InputSchema
	valid := map[string]any{"name": "test", "instruction": "test", "schedule": map[string]any{"kind": "interval", "interval_value": 5}}
	if err := command.ValidateInput(schema, valid); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		field string
		value any
	}{
		{"agent_id", "other"}, {"enabled", "true"}, {"context_mode", "unknown"},
		{"schedule", map[string]any{"kind": "interval", "interval_value": 0}},
		{"schedule", map[string]any{"kind": "interval", "interval_value": 5, "extra": true}},
	} {
		input := map[string]any{}
		for k, v := range valid {
			input[k] = v
		}
		input[test.field] = test.value
		if err := ValidateRuntimeCommandInput(fixture.ServerContext, "plan", "create", input); err == nil {
			t.Fatalf("accepted invalid %s=%v", test.field, test.value)
		}
	}
	if _, err := fixture.Service.RuntimeCommandContract(context.Background(), fixture.ServerContext, "unknown"); err == nil {
		t.Fatal("unknown operation accepted")
	}
	if err := ValidateRuntimeCommandInput(fixture.ServerContext, "invoke", "list", nil); err == nil || !strings.Contains(err.Error(), "action=inspect") {
		t.Fatalf("query correction: %v", err)
	}
	if err := ValidateRuntimeCommandInput(fixture.ServerContext, "invoke", "create", valid); err == nil || !strings.Contains(err.Error(), "action=plan") {
		t.Fatalf("mutation correction: %v", err)
	}
}

func TestAutomationPlanViewCanBeReusedWithoutHiddenFields(t *testing.T) {
	fixture := newAutomationCommandFixture(t, "ok")
	input := automationdomain.AutomationCommandInput{Name: "test", Instruction: "test", Schedule: &automationdomain.AutomationCommandSchedule{Kind: "interval", IntervalValue: 60}}
	plan, err := fixture.Service.PlanRuntimeCommand(context.Background(), fixture.ServerContext, "create", input)
	if err != nil {
		t.Fatal(err)
	}
	view, err := RuntimeCommandPlanView(fixture.ServerContext, *plan)
	if err != nil {
		t.Fatal(err)
	}
	raw := view["input"].(map[string]any)
	if _, ok := raw["agent_id"]; ok {
		t.Fatal("hidden agent_id leaked into reusable input")
	}
	if err := ValidateRuntimeCommandInput(fixture.ServerContext, "apply", "create", raw); err != nil {
		t.Fatal(err)
	}
	bytes, _ := json.Marshal(raw)
	var reused automationdomain.AutomationCommandInput
	if err := json.Unmarshal(bytes, &reused); err != nil {
		t.Fatal(err)
	}
	replanned, err := fixture.Service.PlanRuntimeCommand(context.Background(), fixture.ServerContext, "create", reused)
	if err != nil {
		t.Fatal(err)
	}
	request := automationdomain.AutomationCommandRequest{RequestID: "schema-test-001", ExpectedRevision: plan.CurrentRevision, PlanDigest: plan.PlanDigest}
	if err := ValidateRuntimeCommandPlan(request, *replanned); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"request_id", "expected_revision", "plan_digest", "stale_revision", "stale_digest"} {
		bad := request
		switch field {
		case "request_id":
			bad.RequestID = ""
		case "expected_revision":
			bad.ExpectedRevision = ""
		case "plan_digest":
			bad.PlanDigest = ""
		case "stale_revision":
			bad.ExpectedRevision = "old"
		case "stale_digest":
			bad.PlanDigest = "old"
		}
		if err := ValidateRuntimeCommandPlan(bad, *replanned); err == nil {
			t.Fatalf("accepted %s", field)
		}
	}
}

func TestServiceRejectsAgentActorAtEveryScriptControlEntry(t *testing.T) {
	service := newScriptControlBoundaryService(t)
	agentCtx := automationexec.WithActorAgentID(context.Background(), "agent-1")

	if _, err := service.CreateTask(agentCtx, scriptControlTaskInput("agent-script-create", automationdomain.ExecutionKindScript)); !errors.Is(err, errAgentScriptControl) {
		t.Fatalf("Agent script create error = %v, want boundary rejection", err)
	}

	for _, test := range []struct {
		name string
		call func(string) error
	}{
		{
			name: "update",
			call: func(jobID string) error {
				name := "changed"
				_, err := service.UpdateTask(agentCtx, jobID, automationdomain.UpdateJobInput{Name: &name})
				return err
			},
		},
		{
			name: "delete",
			call: func(jobID string) error {
				_, err := service.DeleteTask(agentCtx, jobID)
				return err
			},
		},
		{
			name: "run",
			call: func(jobID string) error {
				_, err := service.RunTaskNow(agentCtx, jobID)
				return err
			},
		},
		{
			name: "retry_delivery",
			call: func(jobID string) error {
				_, err := service.RetryRunDelivery(agentCtx, jobID, "missing-run")
				return err
			},
		},
		{
			name: "recover",
			call: func(jobID string) error {
				_, err := service.RecoverTaskRunningRun(agentCtx, jobID, "")
				return err
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			task, err := service.CreateTask(context.Background(), scriptControlTaskInput("script-"+test.name, automationdomain.ExecutionKindScript))
			if err != nil {
				t.Fatalf("human script create failed: %v", err)
			}
			if err = test.call(task.JobID); !errors.Is(err, errAgentScriptControl) {
				t.Fatalf("%s error = %v, want boundary rejection", test.name, err)
			}
		})
	}
}

func TestServiceSerializesConcurrentHumanScriptTransitionBeforeAgentRun(t *testing.T) {
	service := newScriptControlBoundaryService(t)
	task, err := service.CreateTask(context.Background(), scriptControlTaskInput("concurrent-agent", automationdomain.ExecutionKindAgent))
	if err != nil {
		t.Fatalf("create Agent task: %v", err)
	}

	humanPersisted := make(chan struct{})
	releaseHuman := make(chan struct{})
	service.SetTaskEventNotifier(TaskEventNotifierFunc(func(_ context.Context, event automationdomain.ScheduledTaskEvent) {
		if event.JobID != task.JobID || event.Action != automationdomain.TaskEventActionUpdate {
			return
		}
		close(humanPersisted)
		<-releaseHuman
	}))

	humanDone := make(chan error, 1)
	go func() {
		scriptKind := automationdomain.ExecutionKindScript
		_, updateErr := service.UpdateTask(context.Background(), task.JobID, automationdomain.UpdateJobInput{
			ExecutionKind: &scriptKind,
		})
		humanDone <- updateErr
	}()
	<-humanPersisted

	agentDone := make(chan error, 1)
	go func() {
		agentCtx := automationexec.WithActorAgentID(context.Background(), "agent-1")
		_, runErr := service.RunTaskNow(agentCtx, task.JobID)
		agentDone <- runErr
	}()
	select {
	case runErr := <-agentDone:
		close(releaseHuman)
		<-humanDone
		t.Fatalf("Agent run crossed the in-flight human control boundary: %v", runErr)
	case <-time.After(50 * time.Millisecond):
	}

	close(releaseHuman)
	if err = <-humanDone; err != nil {
		t.Fatalf("human script transition failed: %v", err)
	}
	if err = <-agentDone; !errors.Is(err, errAgentScriptControl) {
		t.Fatalf("Agent run after script transition error = %v, want boundary rejection", err)
	}
}

func newScriptControlBoundaryService(t *testing.T) *Service {
	t.Helper()
	return NewService(
		config.Config{DatabaseDriver: "sqlite"},
		newAutomationTestDB(t),
		nil,
		nil,
		nil,
		permissionctx.NewContext(),
		&fakeWorkspaceReader{},
		nil,
	)
}

func scriptControlTaskInput(name string, executionKind string) automationdomain.CreateJobInput {
	return automationdomain.CreateJobInput{
		Name:          name,
		AgentID:       "agent-1",
		Instruction:   "echo safe",
		ExecutionKind: executionKind,
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{Kind: automationdomain.SessionTargetIsolated},
		Delivery:      automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeNone},
		Enabled:       true,
	}
}

func TestSQLiteAutomationDeliveryRouteMigrationUpgradesNewerParallelLedger(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "nexus.db"))
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("设置 goose 方言失败: %v", err)
	}
	dir := automationMigrationDir(t)
	if err = goose.UpTo(db, dir, 86); err != nil {
		t.Fatalf("迁移到 Automation 权限 schema 失败: %v", err)
	}
	for version := int64(87); version <= 92; version++ {
		if _, err = db.Exec(
			"INSERT INTO goose_db_version (version_id, is_applied) VALUES (?, TRUE)",
			version,
		); err != nil {
			t.Fatalf("模拟并行分支 migration %d 失败: %v", version, err)
		}
	}
	if err = goose.Up(db, dir); err != nil {
		t.Fatalf("从较新并行账本执行 Automation 投递 migration 失败: %v", err)
	}
	version, err := goose.GetDBVersion(db)
	if err != nil {
		t.Fatalf("读取 migration 版本失败: %v", err)
	}
	migrations, err := goose.CollectMigrations(dir, 0, math.MaxInt64)
	if err != nil || len(migrations) == 0 {
		t.Fatalf("collect current migrations: %v", err)
	}
	wantVersion := migrations[len(migrations)-1].Version
	if version != wantVersion {
		t.Fatalf("migration version = %d, want %d", version, wantVersion)
	}
	for _, target := range []struct {
		table  string
		column string
	}{
		{table: "automation_scheduled_tasks", column: "delivery_session_key"},
		{table: "automation_scheduled_tasks", column: "permission_mode"},
		{table: "automation_scheduled_tasks", column: "session_binding_state"},
		{table: "automation_scheduled_tasks", column: "invalidated_session_keys_json"},
		{table: "automation_scheduled_tasks", column: "delivery_grant_json"},
		{table: "automation_task_runs", column: "delivery_target_json"},
		{table: "automation_delivery_routes", column: "context_token"},
		{table: "automation_permission_requests", column: "delivery_session_key"},
	} {
		var found string
		if err = db.QueryRow(
			"SELECT name FROM pragma_table_info(?) WHERE name = ?",
			target.table,
			target.column,
		).Scan(&found); err != nil {
			t.Fatalf("缺少升级字段 %s.%s: %v", target.table, target.column, err)
		}
	}
}

func automationMigrationDir(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("无法定位当前测试文件")
	}
	return filepath.Join(filepath.Dir(filename), "..", "..", "..", "db", "migrations", "sqlite")
}
