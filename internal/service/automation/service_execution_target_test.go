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
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	_ "modernc.org/sqlite"
)

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
	if len(requests[0].TargetAgentIDs) != 1 || requests[0].TargetAgentIDs[0] != "agent-1" {
		t.Fatalf("Room runner 应收到显式目标成员: %+v", requests[0].TargetAgentIDs)
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
	_, err := service.CreateTask(context.Background(), protocol.CreateJobInput{
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
			BoundSessionKey: protocol.BuildRoomSharedSessionKey("conversation-1"),
		},
		Delivery: protocol.DeliveryTarget{Mode: protocol.DeliveryModeNone},
		Enabled:  true,
	})
	if err == nil || !strings.Contains(err.Error(), "不是目标 Room 的成员") {
		t.Fatalf("非 Room 成员不应创建 Room 定时任务: %v", err)
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
	if !strings.HasPrefix(strings.TrimSpace(anyString(payload["text"])), "[cron:"+task.JobID+" Main payload] ") ||
		!strings.Contains(strings.TrimSpace(anyString(payload["text"])), "follow up in main session") {
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
