package automation

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	automationdomain "github.com/nexus-research-lab/nexus/internal/automation"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"github.com/nexus-research-lab/nexus/internal/runtime/mcp/automation/contract"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	_ "modernc.org/sqlite"
)

func TestDeliverJobObservationUsesTaskOwnerContext(t *testing.T) {
	delivery := &fakeDeliveryRouter{}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		nil,
		nil,
		nil,
		nil,
		nil,
		&fakeWorkspaceReader{},
		delivery,
	)
	job := protocol.CronJob{
		JobID:       "job-owner",
		AgentID:     "agent-1",
		OwnerUserID: "user-1",
		Delivery: protocol.DeliveryTarget{
			Mode:    protocol.DeliveryModeExplicit,
			Channel: "feishu",
			To:      "oc_group",
		},
	}

	deliveryResult := service.deliverJobObservation(context.Background(), job, "", automationdomain.ExecutionObservation{
		Status:     protocol.RunStatusSucceeded,
		ResultText: "今日新闻摘要",
	})
	if deliveryResult.Status != protocol.DeliveryStatusSucceeded || deliveryResult.Error != nil {
		t.Fatalf("投递状态异常: status=%s err=%v", deliveryResult.Status, deliveryResult.Error)
	}
	if deliveryResult.deliveryTo(job.Delivery) != "explicit:feishu:oc_group" {
		t.Fatalf("投递应记录实际目标，实际 %q", deliveryResult.deliveryTo(job.Delivery))
	}
	owners := delivery.OwnerUserIDs()
	if len(owners) != 1 || owners[0] != "user-1" {
		t.Fatalf("投递应使用任务 owner 上下文，实际 owners=%+v", owners)
	}
}

func TestDeliverJobObservationRecordsDeliveryReceipt(t *testing.T) {
	delivery := &fakeDeliveryRouter{
		receipt: channelmessage.NewReceipt(channelmessage.ReceiptParams{
			Channel:  channels.ChannelTypeTelegram,
			Target:   "-1001",
			ThreadID: "12",
			Parts:    []channelmessage.ReceiptPart{channelmessage.TextPart("42")},
		}),
	}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		nil,
		nil,
		nil,
		nil,
		nil,
		&fakeWorkspaceReader{},
		delivery,
	)
	job := protocol.CronJob{
		JobID:   "job-receipt",
		AgentID: "agent-1",
		Delivery: protocol.DeliveryTarget{
			Mode:     protocol.DeliveryModeExplicit,
			Channel:  channels.ChannelTypeTelegram,
			To:       "-1001",
			ThreadID: "12",
		},
	}

	deliveryResult := service.deliverJobObservation(context.Background(), job, "", automationdomain.ExecutionObservation{
		Status:     protocol.RunStatusSucceeded,
		ResultText: "今日新闻摘要",
	})
	if deliveryResult.Receipt == nil || deliveryResult.Receipt.PrimaryPlatformMessageID != "42" {
		t.Fatalf("投递结果未记录平台回执: %+v", deliveryResult.Receipt)
	}
	if got := deliveryResult.deliveryTo(job.Delivery); got != "explicit:telegram:-1001:thread:12:message:42" {
		t.Fatalf("投递摘要未记录平台 message_id，实际 %q", got)
	}
}

func TestDeliverJobObservationPassesSourceSessionForLastDelivery(t *testing.T) {
	delivery := &fakeDeliveryRouter{}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		nil,
		nil,
		nil,
		nil,
		nil,
		&fakeWorkspaceReader{},
		delivery,
	)
	sourceSessionKey := protocol.BuildAgentSessionKey("agent-1", channels.ChannelTypeWeixinPersonal, "dm", "wx-user-1", "")
	job := protocol.CronJob{
		JobID:   "job-session-last",
		AgentID: "agent-1",
		Delivery: protocol.DeliveryTarget{
			Mode: protocol.DeliveryModeLast,
		},
		Source: protocol.Source{
			SessionKey: sourceSessionKey,
		},
	}

	deliveryResult := service.deliverJobObservation(context.Background(), job, "", automationdomain.ExecutionObservation{
		Status:     protocol.RunStatusSucceeded,
		ResultText: "定时提醒",
	})
	if deliveryResult.Status != protocol.DeliveryStatusSucceeded || deliveryResult.Error != nil {
		t.Fatalf("投递状态异常: status=%s err=%v", deliveryResult.Status, deliveryResult.Error)
	}
	calls := delivery.Calls()
	if len(calls) != 1 ||
		calls[0].Mode != channels.DeliveryModeLast ||
		calls[0].SessionKey != sourceSessionKey {
		t.Fatalf("last 投递应携带来源 IM session_key: %+v", calls)
	}
}

func TestServiceRunTaskNowDeliversToRememberedWebSocketRoute(t *testing.T) {
	workspacePath := t.TempDir()
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	dm := &fakeDMRunner{
		permission: permission,
		resultText: "巡检完成：CPU 使用率正常",
	}
	router := channels.NewRouter(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspacePath},
		db,
		&testAgentResolver{workspacePath: workspacePath},
		permission,
	)
	store := workspacestore.NewSessionFileStore(workspacePath)
	sessionKey := protocol.BuildAgentSessionKey("agent-1", "ws", "dm", "delivery", "")
	now := time.Now().UTC()
	if _, err := store.UpsertSession(workspacePath, protocol.Session{
		SessionKey:   sessionKey,
		AgentID:      "agent-1",
		ChannelType:  "websocket",
		ChatType:     "dm",
		Status:       "active",
		CreatedAt:    now,
		LastActivity: now,
		Title:        "Delivery",
		Options:      map[string]any{},
		IsActive:     true,
	}); err != nil {
		t.Fatalf("准备目标会话失败: %v", err)
	}
	if err := router.RememberWebSocketRoute(context.Background(), sessionKey); err != nil {
		t.Fatalf("RememberWebSocketRoute 失败: %v", err)
	}

	service := NewService(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspacePath},
		db,
		nil,
		dm,
		nil,
		permission,
		&fakeWorkspaceReader{},
		router,
	)

	task, err := service.CreateTask(context.Background(), protocol.CreateJobInput{
		Name:        "主动巡检播报",
		AgentID:     "agent-1",
		Instruction: "执行巡检并输出结果",
		Schedule: protocol.Schedule{
			Kind:            protocol.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: protocol.SessionTarget{
			Kind:            protocol.SessionTargetNamed,
			NamedSessionKey: "ops-bot",
		},
		Delivery: protocol.DeliveryTarget{Mode: protocol.DeliveryModeLast},
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	if _, err = service.RunTaskNow(context.Background(), task.JobID); err != nil {
		t.Fatalf("RunTaskNow 失败: %v", err)
	}
	waitFor(t, 2*time.Second, func() bool {
		items, listErr := service.ListTaskRuns(context.Background(), task.JobID)
		if listErr != nil || len(items) == 0 {
			return false
		}
		return items[0].Status == protocol.RunStatusSucceeded
	})

	sessionValue, _, err := store.FindSession([]string{workspacePath}, sessionKey)
	if err != nil {
		t.Fatalf("读取投递目标 session 失败: %v", err)
	}
	if sessionValue == nil {
		t.Fatalf("投递目标 session 不存在")
	}
	assertDeliveredAgentMessage(t, workspacePath, *sessionValue, "巡检完成：CPU 使用率正常", "投递目标")
	updatedTask, err := service.GetTask(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("读取任务运行态失败: %v", err)
	}
	if updatedTask == nil || updatedTask.LastDeliveryStatus != protocol.DeliveryStatusSucceeded {
		t.Fatalf("last_delivery_status 未记录投递成功: %+v", updatedTask)
	}
	deliveredRun := assertRunDeliveredTo(t, service, task.JobID, "explicit:websocket:"+sessionKey)
	if deliveredRun.ArtifactPath == nil || strings.TrimSpace(*deliveredRun.ArtifactPath) == "" {
		t.Fatalf("run 应记录产物路径: %+v", deliveredRun)
	}
	artifact, err := os.ReadFile(filepath.Join(workspacePath, filepath.FromSlash(*deliveredRun.ArtifactPath)))
	if err != nil {
		t.Fatalf("读取 run artifact 失败: %v", err)
	}
	if !strings.Contains(string(artifact), "Delivery Target: explicit:websocket:"+sessionKey) {
		t.Fatalf("run artifact 应记录实际投递目标: %s", string(artifact))
	}
}

func TestServiceRunTaskNowDeliversToAgentAutomationInbox(t *testing.T) {
	workspacePath := t.TempDir()
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	dm := &fakeDMRunner{
		permission: permission,
		resultText: "今日新闻摘要",
	}
	router := channels.NewRouter(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspacePath},
		db,
		&testAgentResolver{workspacePath: workspacePath},
		permission,
	)
	service := NewService(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspacePath},
		db,
		nil,
		dm,
		nil,
		permission,
		&fakeWorkspaceReader{},
		router,
	)

	inboxKey := protocol.BuildAgentSessionKey(
		"agent-2",
		protocol.SessionChannelInternalSegment,
		"dm",
		protocol.AutomationInboxSessionRef,
		"",
	)
	task, err := service.CreateTask(context.Background(), protocol.CreateJobInput{
		Name:        "新闻投递到智能体",
		AgentID:     "agent-1",
		Instruction: "搜索新闻并输出摘要",
		Schedule: protocol.Schedule{
			Kind:            protocol.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: protocol.SessionTarget{
			Kind:            protocol.SessionTargetNamed,
			NamedSessionKey: "news",
		},
		Delivery: protocol.DeliveryTarget{
			Mode:    protocol.DeliveryModeExplicit,
			Channel: protocol.SessionChannelInternalSegment,
			To:      inboxKey,
		},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	if _, err = service.RunTaskNow(context.Background(), task.JobID); err != nil {
		t.Fatalf("RunTaskNow 失败: %v", err)
	}
	waitFor(t, 2*time.Second, func() bool {
		items, listErr := service.ListTaskRuns(context.Background(), task.JobID)
		if listErr != nil || len(items) == 0 {
			return false
		}
		return items[0].DeliveryStatus == protocol.DeliveryStatusSucceeded
	})

	store := workspacestore.NewSessionFileStore(workspacePath)
	sessionValue, _, err := store.FindSession([]string{workspacePath}, inboxKey)
	if err != nil {
		t.Fatalf("读取智能体收件箱 session 失败: %v", err)
	}
	if sessionValue == nil {
		t.Fatal("投递到智能体时应自动创建定时任务收件箱")
	}
	if sessionValue.AgentID != "agent-2" {
		t.Fatalf("收件箱应归属目标智能体，实际 %+v", sessionValue)
	}
	if sessionValue.Title != "定时任务收件箱" || sessionValue.ChannelType != protocol.SessionChannelInternalSegment {
		t.Fatalf("收件箱元数据不正确: %+v", sessionValue)
	}

	assertDeliveredAgentMessage(t, workspacePath, *sessionValue, "今日新闻摘要", "智能体收件箱")
	assertRunDeliveredTo(t, service, task.JobID, "explicit:internal:"+inboxKey)
}

func TestAutomationMCPCreateRunAndInspectDeliversToAgentInbox(t *testing.T) {
	workspacePath := t.TempDir()
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	dm := &fakeDMRunner{
		permission: permission,
		resultText: "今日新闻摘要",
	}
	router := channels.NewRouter(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspacePath},
		db,
		&testAgentResolver{workspacePath: workspacePath},
		permission,
	)
	service := NewService(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspacePath},
		db,
		nil,
		dm,
		nil,
		permission,
		&fakeWorkspaceReader{},
		router,
	)
	sctx := contract.ServerContext{
		CurrentAgentID:      "agent-1",
		CurrentAgentName:    "新闻智能体",
		OwnerUserID:         "user-1",
		CurrentSessionKey:   protocol.BuildAgentSessionKey("agent-1", protocol.SessionChannelInternalSegment, "dm", "operator", ""),
		CurrentSessionLabel: "用户对话",
		SourceContextType:   "agent",
		SourceContextID:     "agent-1",
		SourceContextLabel:  "新闻智能体",
		DefaultTimezone:     "Asia/Shanghai",
	}
	inboxKey := protocol.BuildAgentSessionKey(
		"agent-2",
		protocol.SessionChannelInternalSegment,
		"dm",
		protocol.AutomationInboxSessionRef,
		"",
	)

	createResult, isError := callAutomationMCPTool(t, service, sctx, "create_scheduled_task", map[string]any{
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
	created := decodeAutomationMCPJSON[protocol.CronJob](t, createResult)
	if created.AgentID != "agent-1" {
		t.Fatalf("MCP 创建任务应归属调用智能体，实际 %+v", created)
	}
	if created.Delivery.Mode != protocol.DeliveryModeExplicit ||
		created.Delivery.Channel != protocol.SessionChannelInternalSegment ||
		created.Delivery.To != inboxKey {
		t.Fatalf("MCP reply_mode=agent 应解析为目标智能体收件箱，实际 %+v", created.Delivery)
	}
	if created.Source.Kind != protocol.SourceKindAgent || created.Source.CreatorAgentID != "agent-1" {
		t.Fatalf("MCP 创建任务应记录 Agent 来源，实际 %+v", created.Source)
	}

	runResult, isError := callAutomationMCPTool(t, service, sctx, "run_scheduled_task", map[string]any{
		"query": "新闻投递到智能体",
	})
	if isError {
		t.Fatalf("run_scheduled_task by query 不应失败: %s", automationMCPToolText(t, runResult))
	}
	runNow := decodeAutomationMCPJSON[protocol.ExecutionResult](t, runResult)
	if runNow.JobID != created.JobID {
		t.Fatalf("query 应定位到刚创建的任务，run=%+v created=%+v", runNow, created)
	}

	ownerCtx := automationMCPTestOwnerContext(sctx.OwnerUserID)
	waitFor(t, 2*time.Second, func() bool {
		items, listErr := service.ListTaskRuns(ownerCtx, created.JobID)
		if listErr != nil || len(items) == 0 {
			return false
		}
		return items[0].DeliveryStatus == protocol.DeliveryStatusSucceeded
	})

	store := workspacestore.NewSessionFileStore(workspacePath)
	sessionValue, _, err := store.FindSession([]string{workspacePath}, inboxKey)
	if err != nil {
		t.Fatalf("读取 MCP 创建的智能体收件箱 session 失败: %v", err)
	}
	if sessionValue == nil {
		t.Fatal("MCP 创建并运行后应自动创建目标智能体收件箱")
	}
	if sessionValue.AgentID != "agent-2" {
		t.Fatalf("MCP 投递收件箱应归属目标智能体，实际 %+v", sessionValue)
	}
	assertDeliveredAgentMessage(t, workspacePath, *sessionValue, "今日新闻摘要", "MCP 智能体收件箱")
	assertRunDeliveredToContext(t, ownerCtx, service, created.JobID, "explicit:internal:"+inboxKey)

	statusResult, isError := callAutomationMCPTool(t, service, sctx, "get_scheduled_task_status", map[string]any{
		"query":       "新闻投递到智能体",
		"run_limit":   5,
		"event_limit": 5,
	})
	if isError {
		t.Fatalf("get_scheduled_task_status by query 不应失败: %s", automationMCPToolText(t, statusResult))
	}
	status := decodeAutomationMCPJSON[protocol.CronTaskStatus](t, statusResult)
	if status.Job.JobID != created.JobID || status.Job.LastDeliveryStatus != protocol.DeliveryStatusSucceeded {
		t.Fatalf("MCP 状态应能看到任务最新投递成功，实际 %+v", status.Job)
	}
	if len(status.RecentRuns) == 0 || status.RecentRuns[0].DeliveryTo != "explicit:internal:"+inboxKey {
		t.Fatalf("MCP 状态应返回最近投递目标，实际 %+v", status.RecentRuns)
	}
}

func assertDeliveredAgentMessage(t *testing.T, workspacePath string, session protocol.Session, expectedText string, label string) {
	t.Helper()
	history := workspacestore.NewAgentHistoryStore(workspacePath)
	messages, err := history.ReadMessages(workspacePath, session, nil)
	if err != nil {
		t.Fatalf("读取%s消息失败: %v", label, err)
	}
	if len(messages) != 1 {
		t.Fatalf("期望%s写入 1 条消息，实际 %d", label, len(messages))
	}
	if firstNonEmptyString(stringFromMessage(messages[0], "content")) != expectedText {
		t.Fatalf("%s正文不正确: %+v", label, messages[0])
	}
	summary, ok := messages[0]["result_summary"].(map[string]any)
	if !ok {
		t.Fatalf("%s应挂载 result_summary: %+v", label, messages[0])
	}
	if firstNonEmptyString(stringFromMessage(summary, "subtype")) != "success" {
		t.Fatalf("%s投递终态不正确: %+v", label, messages[0])
	}
}

func assertRunDeliveredTo(t *testing.T, service *Service, jobID string, expectedDeliveryTo string) protocol.CronRun {
	t.Helper()
	return assertRunDeliveredToContext(t, context.Background(), service, jobID, expectedDeliveryTo)
}

func assertRunDeliveredToContext(t *testing.T, ctx context.Context, service *Service, jobID string, expectedDeliveryTo string) protocol.CronRun {
	t.Helper()
	deliveredRuns, err := service.ListTaskRuns(ctx, jobID)
	if err != nil || len(deliveredRuns) == 0 {
		t.Fatalf("读取投递 run 失败: runs=%+v err=%v", deliveredRuns, err)
	}
	if deliveredRuns[0].DeliveryStatus != protocol.DeliveryStatusSucceeded {
		t.Fatalf("run delivery_status 未记录投递成功: %+v", deliveredRuns[0])
	}
	if deliveredRuns[0].DeliveryTo != expectedDeliveryTo {
		t.Fatalf("run delivery_to 应记录实际解析后的投递目标，实际 %q", deliveredRuns[0].DeliveryTo)
	}
	if deliveredRuns[0].DeliveryAttempts != 1 || deliveredRuns[0].DeliveredAt == nil {
		t.Fatalf("run 投递观测信息不完整: %+v", deliveredRuns[0])
	}
	return deliveredRuns[0]
}

func TestServiceDeliveryFailureDoesNotFailExecutionAndCanRetry(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	delivery := &fakeDeliveryRouter{err: fmt.Errorf("feishu send message failed: bad chat_id")}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		&fakeDMRunner{permission: permission, resultText: "今日新闻摘要"},
		nil,
		permission,
		&fakeWorkspaceReader{},
		delivery,
	)
	task, err := service.CreateTask(context.Background(), protocol.CreateJobInput{
		Name:        "news",
		AgentID:     "agent-1",
		Instruction: "search news",
		Schedule: protocol.Schedule{
			Kind:            protocol.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: protocol.SessionTarget{Kind: protocol.SessionTargetNamed, NamedSessionKey: "news"},
		Delivery: protocol.DeliveryTarget{
			Mode:    protocol.DeliveryModeExplicit,
			Channel: "feishu",
			To:      "oc_bad",
		},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	if _, err = service.RunTaskNow(context.Background(), task.JobID); err != nil {
		t.Fatalf("RunTaskNow 失败: %v", err)
	}
	waitFor(t, 2*time.Second, func() bool {
		items, listErr := service.ListTaskRuns(context.Background(), task.JobID)
		return listErr == nil && len(items) > 0 && items[0].DeliveryStatus == protocol.DeliveryStatusFailed
	})

	runs, err := service.ListTaskRuns(context.Background(), task.JobID)
	if err != nil || len(runs) == 0 {
		t.Fatalf("读取 run 失败: runs=%+v err=%v", runs, err)
	}
	if runs[0].Status != protocol.RunStatusSucceeded {
		t.Fatalf("投递失败不应把执行状态改成 failed: %+v", runs[0])
	}
	if runs[0].DeliveryError == nil || !strings.Contains(*runs[0].DeliveryError, "bad chat_id") {
		t.Fatalf("delivery_error 未记录失败原因: %+v", runs[0])
	}
	if runs[0].DeliveryAttempts != 1 {
		t.Fatalf("delivery_attempts = %d, 期望 1", runs[0].DeliveryAttempts)
	}
	if runs[0].DeliveryNextAttemptAt == nil || runs[0].DeliveryDeadLetterAt != nil {
		t.Fatalf("投递失败后应安排自动重试且不进入死信: %+v", runs[0])
	}
	updatedTask, err := service.GetTask(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("读取任务失败: %v", err)
	}
	if updatedTask.LastRunStatus != protocol.RunStatusSucceeded || updatedTask.FailureStreak != 0 {
		t.Fatalf("投递失败不应触发任务级执行失败退避: %+v", updatedTask)
	}
	if updatedTask.LastDeliveryStatus != protocol.DeliveryStatusFailed {
		t.Fatalf("last_delivery_status 未记录失败: %+v", updatedTask)
	}

	updatedDelivery := protocol.DeliveryTarget{
		Mode:    protocol.DeliveryModeExplicit,
		Channel: "feishu",
		To:      "oc_good",
	}
	if _, err = service.UpdateTask(context.Background(), task.JobID, protocol.UpdateJobInput{Delivery: &updatedDelivery}); err != nil {
		t.Fatalf("修正投递目标失败: %v", err)
	}
	delivery.err = nil
	dueAt := runs[0].DeliveryNextAttemptAt.UTC().Add(time.Second)
	service.nowFn = func() time.Time { return dueAt }
	service.retryDueDeliveries(context.Background(), dueAt)
	redeliveredRuns, err := service.ListTaskRuns(context.Background(), task.JobID)
	if err != nil {
		t.Fatalf("读取自动重试后的 run 失败: %v", err)
	}
	redelivered := redeliveredRuns[0]
	if redelivered.DeliveryStatus != protocol.DeliveryStatusSucceeded || redelivered.DeliveryError != nil || redelivered.DeliveredAt == nil {
		t.Fatalf("重试投递后状态不正确: %+v", redelivered)
	}
	if redelivered.DeliveryTo != "explicit:feishu:oc_good" {
		t.Fatalf("重试投递应记录修正后的目标，实际 delivery_to=%q", redelivered.DeliveryTo)
	}
	calls := delivery.Calls()
	if len(calls) < 2 || calls[len(calls)-1].To != "oc_good" {
		t.Fatalf("重试投递应使用修正后的目标，calls=%+v", calls)
	}
	if redelivered.DeliveryAttempts != 2 {
		t.Fatalf("重试后 delivery_attempts = %d, 期望 2", redelivered.DeliveryAttempts)
	}
	if redelivered.DeliveryNextAttemptAt != nil || redelivered.DeliveryDeadLetterAt != nil {
		t.Fatalf("投递成功后应清理重试/死信时间: %+v", redelivered)
	}
	events, err := service.ListTaskEvents(context.Background(), task.JobID, 20)
	if err != nil {
		t.Fatalf("读取自动重试审计失败: %v", err)
	}
	var autoRetryEvent *protocol.CronTaskEvent
	for index := range events {
		if events[index].Action == protocol.TaskEventActionAutoRetryDelivery {
			autoRetryEvent = &events[index]
			break
		}
	}
	if autoRetryEvent == nil {
		t.Fatalf("自动投递重试应写入审计事件: %+v", events)
	}
	if autoRetryEvent.RunID != runs[0].RunID || autoRetryEvent.ActorUserID != authctx.SystemUserID {
		t.Fatalf("自动重试事件应关联 run 且 actor 为系统: %+v", autoRetryEvent)
	}
	if autoRetryEvent.Detail["delivery_status"] != protocol.DeliveryStatusSucceeded ||
		autoRetryEvent.Detail["delivery_to"] != "explicit:feishu:oc_good" {
		t.Fatalf("自动重试事件应记录投递结果和实际目标: %+v", autoRetryEvent.Detail)
	}
	if attempts, ok := autoRetryEvent.Detail["delivery_attempts"].(float64); !ok || int(attempts) != 2 {
		t.Fatalf("自动重试事件应记录 attempts=2: %+v", autoRetryEvent.Detail)
	}
}

func TestServiceRunDueOnceRetriesDueDelivery(t *testing.T) {
	db := newAutomationTestDB(t)
	delivery := &fakeDeliveryRouter{}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		nil,
		nil,
		nil,
		&fakeWorkspaceReader{},
		delivery,
	)
	base := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
	service.nowFn = func() time.Time { return base }
	task, err := service.CreateTask(context.Background(), protocol.CreateJobInput{
		Name:        "auto-redelivery",
		AgentID:     "agent-1",
		Instruction: "send report",
		Schedule: protocol.Schedule{
			Kind:            protocol.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "UTC",
		},
		SessionTarget: protocol.SessionTarget{Kind: protocol.SessionTargetNamed, NamedSessionKey: "reports"},
		Delivery: protocol.DeliveryTarget{
			Mode:    protocol.DeliveryModeExplicit,
			Channel: "feishu",
			To:      "oc_group",
		},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	runID := "run-due-delivery"
	dueAt := base.Add(5 * time.Minute)
	scheduledFor := base.Add(-time.Minute)
	deliveryError := "feishu temporary outage"
	if _, err = db.Exec(`
INSERT INTO automation_cron_runs (
    run_id, job_id, owner_user_id, status, trigger_kind,
    delivery_mode, delivery_to, delivery_status, delivery_error,
    delivery_attempts, delivery_next_attempt_at, scheduled_for, finished_at,
    result_text, attempts
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		runID,
		task.JobID,
		task.OwnerUserID,
		protocol.RunStatusSucceeded,
		"cron",
		protocol.DeliveryModeExplicit,
		"explicit:feishu:oc_old",
		protocol.DeliveryStatusFailed,
		deliveryError,
		1,
		dueAt,
		scheduledFor,
		scheduledFor.Add(time.Minute),
		"日报正文",
		1,
	); err != nil {
		t.Fatalf("准备 due delivery run 失败: %v", err)
	}

	service.nowFn = func() time.Time { return dueAt.Add(time.Second) }
	service.runDueOnce()

	waitFor(t, 2*time.Second, func() bool {
		runs, listErr := service.ListTaskRuns(context.Background(), task.JobID)
		return listErr == nil &&
			len(runs) > 0 &&
			runs[0].RunID == runID &&
			runs[0].DeliveryStatus == protocol.DeliveryStatusSucceeded
	})
	runs, err := service.ListTaskRuns(context.Background(), task.JobID)
	if err != nil || len(runs) == 0 {
		t.Fatalf("读取调度自动重试后的 run 失败: runs=%+v err=%v", runs, err)
	}
	redelivered := runs[0]
	if redelivered.DeliveryAttempts != 2 || redelivered.DeliveryError != nil || redelivered.DeliveryNextAttemptAt != nil {
		t.Fatalf("调度 tick 自动重试后状态不正确: %+v", redelivered)
	}
	if redelivered.DeliveryTo != "explicit:feishu:oc_group" {
		t.Fatalf("自动重试应使用任务当前投递目标，实际 delivery_to=%q", redelivered.DeliveryTo)
	}
	calls := delivery.Calls()
	if len(calls) != 1 || calls[0].To != "oc_group" {
		t.Fatalf("调度 tick 应自动投递到当前目标，calls=%+v", calls)
	}
	events, err := service.ListTaskEvents(context.Background(), task.JobID, 20)
	if err != nil {
		t.Fatalf("读取自动重试事件失败: %v", err)
	}
	for _, event := range events {
		if event.Action == protocol.TaskEventActionAutoRetryDelivery && event.RunID == runID {
			return
		}
	}
	t.Fatalf("调度 tick 自动重试应写入审计事件: %+v", events)
}

func TestServiceAutoRetryDeliveryDeadLettersDisabledTask(t *testing.T) {
	db := newAutomationTestDB(t)
	delivery := &fakeDeliveryRouter{}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		nil,
		nil,
		nil,
		&fakeWorkspaceReader{},
		delivery,
	)
	base := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
	task, err := service.CreateTask(context.Background(), protocol.CreateJobInput{
		Name:        "paused-delivery",
		AgentID:     "agent-1",
		Instruction: "send report",
		Schedule: protocol.Schedule{
			Kind:            protocol.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "UTC",
		},
		SessionTarget: protocol.SessionTarget{Kind: protocol.SessionTargetNamed, NamedSessionKey: "reports"},
		Delivery: protocol.DeliveryTarget{
			Mode:    protocol.DeliveryModeExplicit,
			Channel: "feishu",
			To:      "oc_group",
		},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	if _, err = service.UpdateTaskStatus(context.Background(), task.JobID, false); err != nil {
		t.Fatalf("停用任务失败: %v", err)
	}

	runID := "run-disabled-delivery"
	dueAt := base.Add(time.Minute)
	deliveryError := "feishu temporary outage"
	if _, err = db.Exec(`
INSERT INTO automation_cron_runs (
    run_id, job_id, owner_user_id, status, trigger_kind,
    delivery_mode, delivery_to, delivery_status, delivery_error,
    delivery_attempts, delivery_next_attempt_at, scheduled_for, finished_at,
    result_text, attempts
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		runID,
		task.JobID,
		task.OwnerUserID,
		protocol.RunStatusSucceeded,
		"cron",
		protocol.DeliveryModeExplicit,
		"explicit:feishu:oc_group",
		protocol.DeliveryStatusFailed,
		deliveryError,
		1,
		dueAt,
		dueAt.Add(-time.Minute),
		dueAt,
		"日报正文",
		1,
	); err != nil {
		t.Fatalf("准备 disabled due delivery run 失败: %v", err)
	}

	service.nowFn = func() time.Time { return dueAt.Add(time.Second) }
	service.retryDueDeliveries(context.Background(), dueAt.Add(time.Second))

	runs, err := service.ListTaskRuns(context.Background(), task.JobID)
	if err != nil || len(runs) == 0 {
		t.Fatalf("读取自动重试跳过后的 run 失败: runs=%+v err=%v", runs, err)
	}
	updated := runs[0]
	if updated.RunID != runID ||
		updated.DeliveryStatus != protocol.DeliveryStatusFailed ||
		updated.DeliveryDeadLetterAt == nil ||
		updated.DeliveryNextAttemptAt != nil {
		t.Fatalf("停用任务的 due delivery 应进入死信并清理下一次重试: %+v", updated)
	}
	if updated.DeliveryAttempts != 1 {
		t.Fatalf("停用任务不应发生新的投递尝试，attempts=%d", updated.DeliveryAttempts)
	}
	if len(delivery.Calls()) != 0 {
		t.Fatalf("停用任务不应继续自动投递，calls=%+v", delivery.Calls())
	}
	dueRuns, err := service.repository.ListDueDeliveryRetries(context.Background(), dueAt.Add(2*time.Second), maxAutoDeliveryAttempts, deliveryRetryBatchLimit)
	if err != nil {
		t.Fatalf("重新读取 due delivery 失败: %v", err)
	}
	for _, dueRun := range dueRuns {
		if dueRun.RunID == runID {
			t.Fatalf("死信后的 disabled delivery 不应再次进入自动重试队列: %+v", dueRuns)
		}
	}

	events, err := service.ListTaskEvents(context.Background(), task.JobID, 20)
	if err != nil {
		t.Fatalf("读取自动重试跳过事件失败: %v", err)
	}
	for _, event := range events {
		if event.Action == protocol.TaskEventActionAutoRetryDelivery &&
			event.RunID == runID &&
			event.Detail["auto_retry_skipped_reason"] == "task_disabled" {
			return
		}
	}
	t.Fatalf("停用任务的自动重试跳过应写入审计事件: %+v", events)
}

func TestDeleteTaskDeadLettersPendingDeliveryRetries(t *testing.T) {
	db := newAutomationTestDB(t)
	delivery := &fakeDeliveryRouter{}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		nil,
		nil,
		nil,
		&fakeWorkspaceReader{},
		delivery,
	)
	base := time.Date(2026, 5, 21, 9, 0, 0, 0, time.UTC)
	service.nowFn = func() time.Time { return base }
	task, err := service.CreateTask(context.Background(), protocol.CreateJobInput{
		Name:        "delete-with-failed-delivery",
		AgentID:     "agent-1",
		Instruction: "send report",
		Schedule: protocol.Schedule{
			Kind:            protocol.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "UTC",
		},
		SessionTarget: protocol.SessionTarget{Kind: protocol.SessionTargetNamed, NamedSessionKey: "reports"},
		Delivery: protocol.DeliveryTarget{
			Mode:    protocol.DeliveryModeExplicit,
			Channel: "feishu",
			To:      "oc_group",
		},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	runID := "run-delete-delivery"
	nextAttemptAt := base.Add(10 * time.Minute)
	deliveryError := "feishu temporary outage"
	if _, err = db.Exec(`
INSERT INTO automation_cron_runs (
    run_id, job_id, owner_user_id, status, trigger_kind,
    delivery_mode, delivery_to, delivery_status, delivery_error,
    delivery_attempts, delivery_next_attempt_at, scheduled_for, finished_at,
    result_text, attempts
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		runID,
		task.JobID,
		task.OwnerUserID,
		protocol.RunStatusSucceeded,
		"cron",
		protocol.DeliveryModeExplicit,
		"explicit:feishu:oc_group",
		protocol.DeliveryStatusFailed,
		deliveryError,
		1,
		nextAttemptAt,
		base,
		base.Add(time.Minute),
		"日报正文",
		1,
	); err != nil {
		t.Fatalf("准备待补投递 run 失败: %v", err)
	}

	deletedAt := base.Add(time.Minute)
	service.nowFn = func() time.Time { return deletedAt }
	if _, err = service.DeleteTask(context.Background(), task.JobID); err != nil {
		t.Fatalf("DeleteTask 失败: %v", err)
	}

	runs, err := service.ListTaskRuns(context.Background(), task.JobID)
	if err != nil || len(runs) == 0 {
		t.Fatalf("删除后读取 run ledger 失败: runs=%+v err=%v", runs, err)
	}
	updated := runs[0]
	if updated.RunID != runID ||
		updated.DeliveryStatus != protocol.DeliveryStatusFailed ||
		updated.DeliveryDeadLetterAt == nil ||
		updated.DeliveryNextAttemptAt != nil ||
		updated.DeliveryError == nil ||
		!strings.Contains(*updated.DeliveryError, "deleted") {
		t.Fatalf("删除任务应立即把待补投递 run 标记为死信: %+v", updated)
	}
	if updated.DeliveryAttempts != 1 {
		t.Fatalf("删除任务不应新增投递尝试，attempts=%d", updated.DeliveryAttempts)
	}
	dueRuns, err := service.repository.ListDueDeliveryRetries(context.Background(), deletedAt.Add(time.Hour), maxAutoDeliveryAttempts, deliveryRetryBatchLimit)
	if err != nil {
		t.Fatalf("读取 due delivery 失败: %v", err)
	}
	for _, dueRun := range dueRuns {
		if dueRun.RunID == runID {
			t.Fatalf("删除任务后的死信 run 不应进入自动重试队列: %+v", dueRuns)
		}
	}
	if len(delivery.Calls()) != 0 {
		t.Fatalf("删除任务不应触发投递，calls=%+v", delivery.Calls())
	}
	report, err := service.GetDailyReport(context.Background(), protocol.CronDailyReportInput{
		Date:     "2026-05-21",
		Timezone: "UTC",
		JobID:    task.JobID,
	})
	if err != nil {
		t.Fatalf("删除任务后读取日报失败: %v", err)
	}
	if len(report.Tasks) != 1 {
		t.Fatalf("删除任务日报应返回一条任务明细: %+v", report)
	}
	dailyTask := report.Tasks[0]
	if !dailyTask.Deleted ||
		!containsString(dailyTask.Signals, "deleted") ||
		!containsString(dailyTask.Signals, "delivery_attention") ||
		!containsString(dailyTask.DeliveryDeadLetterRunIDs, runID) ||
		containsString(dailyTask.ManualRedeliveryRunIDs, runID) ||
		!containsString(dailyTask.SuggestedTools, "get_scheduled_task_events") ||
		containsString(dailyTask.SuggestedTools, "retry_scheduled_task_delivery") {
		t.Fatalf("删除任务日报应保留失败信号但不建议不可执行补投递: %+v", dailyTask)
	}

	events, err := service.ListTaskEvents(context.Background(), task.JobID, 20)
	if err != nil {
		t.Fatalf("删除后读取事件失败: %v", err)
	}
	for _, event := range events {
		if event.Action != protocol.TaskEventActionDelete {
			continue
		}
		values, ok := event.Detail["dead_lettered_delivery_run_ids"].([]any)
		if !ok || len(values) != 1 || values[0] != runID {
			t.Fatalf("delete 事件应记录被死信的投递 run: %+v", event.Detail)
		}
		return
	}
	t.Fatalf("删除任务应写入 delete 事件: %+v", events)
}

func TestServiceRetryRunDeliveryMarksDeadLetterAfterMaxAttempts(t *testing.T) {
	db := newAutomationTestDB(t)
	delivery := &fakeDeliveryRouter{err: fmt.Errorf("feishu temporary outage")}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		nil,
		nil,
		nil,
		&fakeWorkspaceReader{},
		delivery,
	)
	task, err := service.CreateTask(context.Background(), protocol.CreateJobInput{
		Name:        "dead-letter",
		AgentID:     "agent-1",
		Instruction: "send report",
		Schedule: protocol.Schedule{
			Kind:            protocol.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: protocol.SessionTarget{Kind: protocol.SessionTargetNamed, NamedSessionKey: "reports"},
		Delivery: protocol.DeliveryTarget{
			Mode:    protocol.DeliveryModeExplicit,
			Channel: "feishu",
			To:      "oc_group",
		},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	if _, err = db.Exec(`
INSERT INTO automation_cron_runs (
    run_id, job_id, owner_user_id, status, trigger_kind,
    delivery_mode, delivery_to, delivery_status, delivery_attempts,
    result_text, attempts
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"run-dead",
		task.JobID,
		task.OwnerUserID,
		protocol.RunStatusSucceeded,
		"cron",
		protocol.DeliveryModeExplicit,
		"feishu:oc_group",
		protocol.DeliveryStatusFailed,
		maxAutoDeliveryAttempts-1,
		"日报正文",
		1,
	); err != nil {
		t.Fatalf("准备 run 失败: %v", err)
	}

	run, err := service.RetryRunDelivery(context.Background(), task.JobID, "run-dead")
	if err != nil {
		t.Fatalf("RetryRunDelivery 失败: %v", err)
	}
	if run.DeliveryStatus != protocol.DeliveryStatusFailed || run.DeliveryDeadLetterAt == nil || run.DeliveryNextAttemptAt != nil {
		t.Fatalf("达到最大重试后应进入死信且不再安排自动重试: %+v", run)
	}
	if run.DeliveryAttempts != maxAutoDeliveryAttempts {
		t.Fatalf("delivery_attempts = %d, 期望 %d", run.DeliveryAttempts, maxAutoDeliveryAttempts)
	}
}

func TestServiceRetryRunDeliveryRejectsAlreadyDeliveredRun(t *testing.T) {
	db := newAutomationTestDB(t)
	delivery := &fakeDeliveryRouter{}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		nil,
		nil,
		nil,
		&fakeWorkspaceReader{},
		delivery,
	)
	task, err := service.CreateTask(context.Background(), protocol.CreateJobInput{
		Name:        "delivered",
		AgentID:     "agent-1",
		Instruction: "send report",
		Schedule: protocol.Schedule{
			Kind:            protocol.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: protocol.SessionTarget{Kind: protocol.SessionTargetNamed, NamedSessionKey: "reports"},
		Delivery: protocol.DeliveryTarget{
			Mode:    protocol.DeliveryModeExplicit,
			Channel: "feishu",
			To:      "oc_group",
		},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}
	if _, err = db.Exec(`
INSERT INTO automation_cron_runs (
    run_id, job_id, owner_user_id, status, trigger_kind,
    delivery_mode, delivery_to, delivery_status, delivery_attempts,
    result_text, attempts
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"run-delivered",
		task.JobID,
		task.OwnerUserID,
		protocol.RunStatusSucceeded,
		"cron",
		protocol.DeliveryModeExplicit,
		"feishu:oc_group",
		protocol.DeliveryStatusSucceeded,
		1,
		"日报正文",
		1,
	); err != nil {
		t.Fatalf("准备 run 失败: %v", err)
	}

	_, err = service.RetryRunDelivery(context.Background(), task.JobID, "run-delivered")
	if err == nil || !strings.Contains(err.Error(), "delivery_status must be failed") {
		t.Fatalf("期望拒绝重复补投已成功 run，实际 err=%v", err)
	}
	if calls := delivery.Calls(); len(calls) != 0 {
		t.Fatalf("不应重复调用投递，calls=%+v", calls)
	}
	var attempts int
	if err = db.QueryRow(`SELECT delivery_attempts FROM automation_cron_runs WHERE run_id = ?`, "run-delivered").Scan(&attempts); err != nil {
		t.Fatalf("读取 delivery_attempts 失败: %v", err)
	}
	if attempts != 1 {
		t.Fatalf("delivery_attempts 不应变化，实际 %d", attempts)
	}
}

func TestRunTaskNowSkipsDuplicateExplicitDeliveryWhenTargetMatchesExecutionSession(t *testing.T) {
	db := newAutomationTestDB(t)
	permission := permissionctx.NewContext()
	delivery := &fakeDeliveryRouter{}
	service := NewService(
		config.Config{DatabaseDriver: "sqlite"},
		db,
		nil,
		&fakeDMRunner{permission: permission, resultText: "done"},
		nil,
		permission,
		&fakeWorkspaceReader{},
		delivery,
	)
	sessionKey := protocol.BuildAgentSessionKey("agent-1", "ws", "dm", "existing-chat", "")
	task, err := service.CreateTask(context.Background(), protocol.CreateJobInput{
		Name:        "dup-delivery",
		AgentID:     "agent-1",
		Instruction: "run once",
		Schedule: protocol.Schedule{
			Kind:            protocol.ScheduleKindEvery,
			IntervalSeconds: intRef(60),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: protocol.SessionTarget{
			Kind:            protocol.SessionTargetBound,
			BoundSessionKey: sessionKey,
		},
		Delivery: protocol.DeliveryTarget{
			Mode:    protocol.DeliveryModeExplicit,
			Channel: "websocket",
			To:      sessionKey,
		},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateTask 失败: %v", err)
	}

	if _, err = service.RunTaskNow(context.Background(), task.JobID); err != nil {
		t.Fatalf("RunTaskNow 失败: %v", err)
	}
	waitFor(t, 2*time.Second, func() bool {
		items, listErr := service.ListTaskRuns(context.Background(), task.JobID)
		return listErr == nil && len(items) > 0 && items[0].Status == protocol.RunStatusSucceeded
	})

	if len(delivery.Calls()) != 0 {
		t.Fatalf("execution 会话与显式回传目标一致时不应重复投递")
	}
}
