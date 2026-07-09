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
	"github.com/nexus-research-lab/nexus/internal/mcp/automation/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	"github.com/nexus-research-lab/nexus/internal/service/channels"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	_ "modernc.org/sqlite"
)

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

	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "主动巡检播报",
		AgentID:     "agent-1",
		Instruction: "执行巡检并输出结果",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetNamed,
			NamedSessionKey: "ops-bot",
		},
		Delivery: automationdomain.DeliveryTarget{Mode: automationdomain.DeliveryModeLast},
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
		return items[0].Status == automationdomain.RunStatusSucceeded
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
	if updatedTask == nil || updatedTask.LastDeliveryStatus != automationdomain.DeliveryStatusSucceeded {
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
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "新闻投递到智能体",
		AgentID:     "agent-1",
		Instruction: "搜索新闻并输出摘要",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(3600),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetNamed,
			NamedSessionKey: "news",
		},
		Delivery: automationdomain.DeliveryTarget{
			Mode:    automationdomain.DeliveryModeExplicit,
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
		return items[0].DeliveryStatus == automationdomain.DeliveryStatusSucceeded
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
	created := decodeAutomationMCPJSON[automationdomain.CronJob](t, createResult)
	if created.AgentID != "agent-1" {
		t.Fatalf("MCP 创建任务应归属调用智能体，实际 %+v", created)
	}
	if created.Delivery.Mode != automationdomain.DeliveryModeExplicit ||
		created.Delivery.Channel != protocol.SessionChannelInternalSegment ||
		created.Delivery.To != inboxKey {
		t.Fatalf("MCP reply_mode=agent 应解析为目标智能体收件箱，实际 %+v", created.Delivery)
	}
	if created.Source.Kind != automationdomain.SourceKindAgent || created.Source.CreatorAgentID != "agent-1" {
		t.Fatalf("MCP 创建任务应记录 Agent 来源，实际 %+v", created.Source)
	}

	runResult, isError := callAutomationMCPTool(t, service, sctx, "run_scheduled_task", map[string]any{
		"query": "新闻投递到智能体",
	})
	if isError {
		t.Fatalf("run_scheduled_task by query 不应失败: %s", automationMCPToolText(t, runResult))
	}
	runNow := decodeAutomationMCPJSON[automationdomain.ExecutionResult](t, runResult)
	if runNow.JobID != created.JobID {
		t.Fatalf("query 应定位到刚创建的任务，run=%+v created=%+v", runNow, created)
	}

	ownerCtx := automationMCPTestOwnerContext(sctx.OwnerUserID)
	waitFor(t, 2*time.Second, func() bool {
		items, listErr := service.ListTaskRuns(ownerCtx, created.JobID)
		if listErr != nil || len(items) == 0 {
			return false
		}
		return items[0].DeliveryStatus == automationdomain.DeliveryStatusSucceeded
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
	status := decodeAutomationMCPJSON[automationdomain.CronTaskStatus](t, statusResult)
	if status.Job.JobID != created.JobID || status.Job.LastDeliveryStatus != automationdomain.DeliveryStatusSucceeded {
		t.Fatalf("MCP 状态应能看到任务最新投递成功，实际 %+v", status.Job)
	}
	if len(status.RecentRuns) == 0 || status.RecentRuns[0].DeliveryTo != "explicit:internal:"+inboxKey {
		t.Fatalf("MCP 状态应返回最近投递目标，实际 %+v", status.RecentRuns)
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
	task, err := service.CreateTask(context.Background(), automationdomain.CreateJobInput{
		Name:        "dup-delivery",
		AgentID:     "agent-1",
		Instruction: "run once",
		Schedule: automationdomain.Schedule{
			Kind:            automationdomain.ScheduleKindEvery,
			IntervalSeconds: intRef(60),
			Timezone:        "Asia/Shanghai",
		},
		SessionTarget: automationdomain.SessionTarget{
			Kind:            automationdomain.SessionTargetBound,
			BoundSessionKey: sessionKey,
		},
		Delivery: automationdomain.DeliveryTarget{
			Mode:    automationdomain.DeliveryModeExplicit,
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
		return listErr == nil && len(items) > 0 && items[0].Status == automationdomain.RunStatusSucceeded
	})

	if len(delivery.Calls()) != 0 {
		t.Fatalf("execution 会话与显式回传目标一致时不应重复投递")
	}
}
