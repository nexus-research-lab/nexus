package channels

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

func TestPairedExternalDMUsesSameAgentInteractiveAuthorityAcrossChannels(t *testing.T) {
	channelTypes := []string{
		ChannelTypeDiscord,
		ChannelTypeTelegram,
		ChannelTypeDingTalk,
		ChannelTypeWeChat,
		ChannelTypeWeixinPersonal,
		ChannelTypeFeishu,
	}
	for index, channelType := range channelTypes {
		t.Run(channelType, func(t *testing.T) {
			cfg := newIngressTestConfig(t)
			db := migrateIngressSQLite(t, cfg.DatabaseURL)
			defer func() { _ = db.Close() }()

			agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
			defaultAgent, err := agentService.GetDefaultAgent(context.Background())
			if err != nil {
				t.Fatalf("初始化默认 Agent 失败: %v", err)
			}
			target := fmt.Sprintf("paired-user-%d", index)
			accountID := ""
			if channelType == ChannelTypeWeixinPersonal {
				accountID = "weixin-account"
			}
			if _, err = agentService.UpdateAgent(context.Background(), defaultAgent.AgentID, protocol.UpdateRequest{
				Options: &protocol.Options{AllowedTools: []string{"nexus_automation"}},
			}); err != nil {
				t.Fatalf("配置 %s Agent 工具权限失败: %v", channelType, err)
			}
			handler := &fakeIngressDMHandler{}
			router := NewRouter(cfg, db, agentService, permissionctx.NewContext())
			control := NewControlService(cfg, db, agentService, router)
			if _, err = control.CreatePairing(context.Background(), "", CreatePairingRequest{
				ChannelType: channelType,
				AccountID:   accountID,
				ChatType:    protocol.RoomTypeDM,
				ExternalRef: target,
				AgentID:     defaultAgent.AgentID,
				Status:      PairingStatusActive,
			}); err != nil {
				t.Fatalf("创建 %s pairing 失败: %v", channelType, err)
			}
			service := NewIngressService(cfg, agentService, handler, router)
			service.SetControlService(control)

			_, err = service.Accept(context.Background(), IngressRequest{
				Channel:   channelType,
				AccountID: accountID,
				ChatType:  protocol.RoomTypeDM,
				Ref:       target,
				Content:   "创建一个每天提醒我的定时任务",
				ReqID:     "paired-message",
				Delivery: &DeliveryTarget{
					Mode:      DeliveryModeExplicit,
					Channel:   channelType,
					To:        target,
					AccountID: accountID,
				},
			})
			if err != nil {
				t.Fatalf("%s paired DM Accept 失败: %v", channelType, err)
			}
			if len(handler.requests) != 1 || !handler.requests[0].TrustedExternalInteractiveContext {
				t.Fatalf("%s 未签发 paired interactive context: %+v", channelType, handler.requests)
			}
			decision, err := handler.requests[0].PermissionHandler(context.Background(), sdkpermission.Request{
				ToolName: "mcp__nexus_automation__create_scheduled_task",
				Input:    map[string]any{"name": "每日提醒"},
			})
			if err != nil || decision.Behavior != sdkpermission.BehaviorAllow {
				t.Fatalf("%s paired DM 未获得同 Agent Automation mutation: decision=%+v err=%v", channelType, decision, err)
			}
		})
	}
}

func TestPairedExternalDMRequestsRuntimePermissionAndResolvesBySlashAcrossChannels(t *testing.T) {
	channelTypes := []string{
		ChannelTypeDiscord,
		ChannelTypeTelegram,
		ChannelTypeDingTalk,
		ChannelTypeWeChat,
		ChannelTypeWeixinPersonal,
		ChannelTypeFeishu,
	}
	for index, channelType := range channelTypes {
		t.Run(channelType, func(t *testing.T) {
			cfg := newIngressTestConfig(t)
			db := migrateIngressSQLite(t, cfg.DatabaseURL)
			defer func() { _ = db.Close() }()

			agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
			defaultAgent, err := agentService.GetDefaultAgent(context.Background())
			if err != nil {
				t.Fatalf("初始化默认 Agent 失败: %v", err)
			}
			target := fmt.Sprintf("runtime-permission-user-%d", index)
			accountID := ""
			if channelType == ChannelTypeWeixinPersonal {
				accountID = "runtime-permission-weixin-account"
			}
			permission := permissionctx.NewContext()
			handler := &fakeIngressDMHandler{}
			router := NewRouter(cfg, db, agentService, permission)
			delivery := &recordingDeliveryChannel{channelType: channelType}
			router.RegisterForOwner("", delivery)
			if err = router.Start(context.Background()); err != nil {
				t.Fatalf("启动 %s router 失败: %v", channelType, err)
			}
			defer router.Stop(context.Background())
			control := NewControlService(cfg, db, agentService, router)
			if _, err = control.CreatePairing(context.Background(), "", CreatePairingRequest{
				ChannelType: channelType,
				AccountID:   accountID,
				ChatType:    protocol.RoomTypeDM,
				ExternalRef: target,
				AgentID:     defaultAgent.AgentID,
				Status:      PairingStatusActive,
			}); err != nil {
				t.Fatalf("创建 %s pairing 失败: %v", channelType, err)
			}
			service := NewIngressService(cfg, agentService, handler, router)
			service.SetControlService(control)
			service.SetRuntimePermissionContext(permission)

			initial, err := service.Accept(context.Background(), IngressRequest{
				Channel:   channelType,
				AccountID: accountID,
				ChatType:  protocol.RoomTypeDM,
				Ref:       target,
				Content:   "请写入测试文件",
				ReqID:     "runtime-permission-message",
				Delivery: &DeliveryTarget{
					Mode:      DeliveryModeExplicit,
					Channel:   channelType,
					To:        target,
					AccountID: accountID,
				},
			})
			if err != nil || initial == nil || len(handler.requests) != 1 {
				t.Fatalf("%s 初始 ingress 失败: result=%+v requests=%+v err=%v", channelType, initial, handler.requests, err)
			}

			decisionCtx, cancelDecision := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancelDecision()
			decisions := make(chan sdkpermission.Decision, 1)
			go func() {
				decision, _ := handler.requests[0].PermissionHandler(decisionCtx, sdkpermission.Request{
					ToolName: "Write",
					Input:    map[string]any{"file_path": "permission-test.txt"},
				})
				decisions <- decision
			}()

			notice := waitForRuntimePermissionNotice(t, delivery)
			if !strings.Contains(notice, "/y：允许本次") || strings.Contains(notice, runtimePermissionRequestIDPrefix) {
				t.Fatalf("%s 权限通知应隐藏内部 request ID: %q", channelType, notice)
			}
			if _, err = service.Accept(context.Background(), IngressRequest{
				Channel:   channelType,
				AccountID: accountID,
				ChatType:  protocol.RoomTypeDM,
				Ref:       target,
				Content:   "/y",
				ReqID:     "runtime-permission-approval",
				Delivery: &DeliveryTarget{
					Mode:      DeliveryModeExplicit,
					Channel:   channelType,
					To:        target,
					AccountID: accountID,
				},
			}); err != nil {
				t.Fatalf("%s /y 失败: %v", channelType, err)
			}

			select {
			case decision := <-decisions:
				if decision.Behavior != sdkpermission.BehaviorAllow {
					t.Fatalf("%s /y 未释放 runtime: %+v", channelType, decision)
				}
			case <-time.After(3 * time.Second):
				t.Fatalf("%s /y 后 runtime 仍在等待", channelType)
			}
			if len(handler.requests) != 1 {
				t.Fatalf("%s 权限命令不应进入 Agent 对话: %+v", channelType, handler.requests)
			}
		})
	}
}

func TestShortPermissionCommandRejectsRuntimeAndAutomationAmbiguity(t *testing.T) {
	cfg := newIngressTestConfig(t)
	db := migrateIngressSQLite(t, cfg.DatabaseURL)
	defer func() { _ = db.Close() }()

	agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	defaultAgent, err := agentService.GetDefaultAgent(context.Background())
	if err != nil {
		t.Fatalf("初始化默认 Agent 失败: %v", err)
	}
	permission := permissionctx.NewContext()
	dm := &fakeIngressDMHandler{}
	router := NewRouter(cfg, db, agentService, permission)
	delivery := &recordingDeliveryChannel{channelType: ChannelTypeFeishu}
	router.RegisterForOwner("", delivery)
	if err = router.Start(context.Background()); err != nil {
		t.Fatalf("启动 channel router 失败: %v", err)
	}
	defer router.Stop(context.Background())
	control := NewControlService(cfg, db, agentService, router)
	const target = "runtime-automation-ambiguity"
	if _, err = control.CreatePairing(context.Background(), "", CreatePairingRequest{
		ChannelType: ChannelTypeFeishu,
		ChatType:    protocol.RoomTypeDM,
		ExternalRef: target,
		AgentID:     defaultAgent.AgentID,
		Status:      PairingStatusActive,
	}); err != nil {
		t.Fatalf("创建 pairing 失败: %v", err)
	}
	commands := &recordingIngressCommandHandler{
		pendingCount: 1,
		result: IngressCommandResult{
			Handled: true,
			Reply:   "不应执行",
		},
	}
	service := NewIngressService(cfg, agentService, dm, router)
	service.SetControlService(control)
	service.SetRuntimePermissionContext(permission)

	initial, err := service.Accept(context.Background(), IngressRequest{
		Channel:  ChannelTypeFeishu,
		ChatType: protocol.RoomTypeDM,
		Ref:      target,
		Content:  "触发普通工具权限",
		ReqID:    "ambiguity-trigger",
		Delivery: &DeliveryTarget{Mode: DeliveryModeExplicit, Channel: ChannelTypeFeishu, To: target},
	})
	if err != nil || initial == nil || len(dm.requests) != 1 {
		t.Fatalf("初始 ingress 失败: requests=%+v err=%v", dm.requests, err)
	}
	service.SetCommandHandler(commands)
	requestCtx, cancel := context.WithCancel(t.Context())
	defer cancel()
	decisions := make(chan sdkpermission.Decision, 1)
	go func() {
		decision, _ := dm.requests[0].PermissionHandler(requestCtx, sdkpermission.Request{ToolName: "Write"})
		decisions <- decision
	}()
	_ = waitForRuntimePermissionNotice(t, delivery)

	if _, err = service.Accept(context.Background(), IngressRequest{
		Channel:  ChannelTypeFeishu,
		ChatType: protocol.RoomTypeDM,
		Ref:      target,
		Content:  "/y",
		ReqID:    "ambiguity-decision",
		Delivery: &DeliveryTarget{Mode: DeliveryModeExplicit, Channel: ChannelTypeFeishu, To: target},
	}); err != nil {
		t.Fatalf("歧义 /y 处理失败: %v", err)
	}
	if got := permission.CountSessionPermissionRequests(initial.SessionKey, ""); got != 1 {
		t.Fatalf("歧义 /y 不应消费 runtime 请求: pending=%d", got)
	}
	if len(commands.requests) != 0 {
		t.Fatalf("歧义 /y 不应消费 Automation 请求: %+v", commands.requests)
	}
	select {
	case decision := <-decisions:
		t.Fatalf("歧义 /y 不应释放 runtime: %+v", decision)
	case <-time.After(100 * time.Millisecond):
	}
	delivery.mu.Lock()
	texts := append([]string(nil), delivery.texts...)
	delivery.mu.Unlock()
	if !slices.ContainsFunc(texts, func(text string) bool { return strings.Contains(text, "多个待确认请求") }) {
		t.Fatalf("歧义 /y 未回投安全提示: %+v", texts)
	}
}

func waitForRuntimePermissionNotice(t *testing.T, channel *recordingDeliveryChannel) string {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		channel.mu.Lock()
		texts := append([]string(nil), channel.texts...)
		channel.mu.Unlock()
		for _, text := range texts {
			if strings.Contains(text, "【Nexus 权限确认】") && strings.Contains(text, "/y：允许本次") {
				return text
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("等待 IM runtime 权限通知超时")
	return ""
}

func TestExternalIngressExplicitAgentIDCannotBypassPairing(t *testing.T) {
	cfg := newIngressTestConfig(t)
	db := migrateIngressSQLite(t, cfg.DatabaseURL)
	defer func() { _ = db.Close() }()

	agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	defaultAgent, err := agentService.GetDefaultAgent(context.Background())
	if err != nil {
		t.Fatalf("初始化默认 Agent 失败: %v", err)
	}
	handler := &fakeIngressDMHandler{}
	router := NewRouter(cfg, db, agentService, permissionctx.NewContext())
	service := NewIngressService(cfg, agentService, handler, router)
	service.SetControlService(NewControlService(cfg, db, agentService, router))

	_, err = service.Accept(context.Background(), IngressRequest{
		Channel:  ChannelTypeTelegram,
		ChatType: protocol.RoomTypeDM,
		Ref:      "unpaired-user",
		AgentID:  defaultAgent.AgentID,
		Content:  "尝试绕过 pairing",
	})
	if err == nil || len(handler.requests) != 0 {
		t.Fatalf("显式 agent_id 不得绕过 pairing: err=%v requests=%+v", err, handler.requests)
	}
}

func TestScheduledTaskMutationToolMatchesRuntimeWrappers(t *testing.T) {
	for _, toolName := range []string{
		"create_scheduled_task",
		"mcp__nexus_automation__delete_scheduled_task",
		"nexus_automation.update_scheduled_task",
		"nexus_automation/run_scheduled_task",
		"custom_wrapper__repair_scheduled_task",
	} {
		if !isScheduledTaskMutationTool(toolName) {
			t.Fatalf("mutation wrapper %q was not denied", toolName)
		}
	}
	for _, toolName := range []string{
		"find_scheduled_tasks",
		"mcp__nexus_automation__inspect_scheduled_task",
		"mcp__nexus_automation__get_scheduled_task_report",
	} {
		if isScheduledTaskMutationTool(toolName) {
			t.Fatalf("read tool %q was classified as a mutation", toolName)
		}
	}
}

func TestIngressServiceFeishuAllowsManagedToolsWithRestrictiveAgentTools(t *testing.T) {
	cfg := newIngressTestConfig(t)
	db := migrateIngressSQLite(t, cfg.DatabaseURL)
	defer func() { _ = db.Close() }()

	agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	if _, err := agentService.UpdateAgent(context.Background(), cfg.DefaultAgentID, protocol.UpdateRequest{
		Options: &protocol.Options{AllowedTools: []string{"nexus_automation"}},
	}); err != nil {
		t.Fatalf("收紧默认 agent 工具权限失败: %v", err)
	}
	handler := &fakeIngressDMHandler{}
	router := NewRouter(cfg, db, agentService, permissionctx.NewContext())
	service := NewIngressService(cfg, agentService, handler, router)

	if _, err := service.Accept(context.Background(), IngressRequest{
		Channel:  "feishu",
		ChatType: "group",
		Ref:      "oc_group_123",
		Content:  "检查今天的定时任务发送情况",
	}); err != nil {
		t.Fatalf("Accept 失败: %v", err)
	}
	if len(handler.requests) != 1 || handler.requests[0].PermissionHandler == nil {
		t.Fatalf("未下发带权限处理器的请求: %+v", handler.requests)
	}

	reportDecision, err := handler.requests[0].PermissionHandler(context.Background(), sdkpermission.Request{
		ToolName: "mcp__nexus_automation__get_scheduled_task_report",
		Input:    map[string]any{"date": "today"},
	})
	if err != nil {
		t.Fatalf("日报工具权限处理失败: %v", err)
	}
	if reportDecision.Behavior != sdkpermission.BehaviorAllow {
		t.Fatalf("限制 allowlist 时仍应允许托管定时任务工具: %+v", reportDecision)
	}
	goalSkillDecision, err := handler.requests[0].PermissionHandler(context.Background(), sdkpermission.Request{
		ToolName: "Skill",
		Input:    map[string]any{"name": "goal-manager"},
	})
	if err != nil {
		t.Fatalf("Goal Skill 权限处理失败: %v", err)
	}
	if goalSkillDecision.Behavior != sdkpermission.BehaviorAllow {
		t.Fatalf("限制 allowlist 时仍应允许加载托管 Goal skill: %+v", goalSkillDecision)
	}

	goalDecision, err := handler.requests[0].PermissionHandler(context.Background(), sdkpermission.Request{
		ToolName: "mcp__nexus_goal__create_goal",
		Input:    map[string]any{"objective": "完成发送问题排查"},
	})
	if err != nil {
		t.Fatalf("Goal 工具权限处理失败: %v", err)
	}
	if goalDecision.Behavior != sdkpermission.BehaviorAllow {
		t.Fatalf("限制 allowlist 时仍应允许托管 Goal 工具: %+v", goalDecision)
	}

	readDecision, err := handler.requests[0].PermissionHandler(context.Background(), sdkpermission.Request{
		ToolName: "Read",
		Input:    map[string]any{"file_path": "README.md"},
	})
	if err != nil {
		t.Fatalf("Read 权限处理失败: %v", err)
	}
	if readDecision.Behavior != sdkpermission.BehaviorDeny {
		t.Fatalf("限制 allowlist 时不应顺带放开普通只读工具: %+v", readDecision)
	}
}

func TestIngressServiceAcceptTelegramAllowsScheduledTaskQueriesButDeniesMutations(t *testing.T) {
	cfg := newIngressTestConfig(t)
	db := migrateIngressSQLite(t, cfg.DatabaseURL)
	defer func() { _ = db.Close() }()

	agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	handler := &fakeIngressDMHandler{}
	router := NewRouter(cfg, db, agentService, permissionctx.NewContext())
	service := NewIngressService(cfg, agentService, handler, router)

	result, err := service.Accept(context.Background(), IngressRequest{
		Channel:  "telegram",
		ChatType: "group",
		Ref:      "-100123456",
		ThreadID: "12",
		Content:  "群组消息",
	})
	if err != nil {
		t.Fatalf("Accept 失败: %v", err)
	}

	if result.SessionKey != "agent:nexus:tg:group:-100123456:topic:12" {
		t.Fatalf("telegram session_key 不正确: %s", result.SessionKey)
	}
	route, err := router.GetLastRoute(context.Background(), cfg.DefaultAgentID)
	if err != nil {
		t.Fatalf("读取 last route 失败: %v", err)
	}
	if route == nil || route.Channel != ChannelTypeTelegram || route.To != "-100123456" || route.ThreadID != "12" {
		t.Fatalf("telegram route 记忆不正确: %+v", route)
	}

	readDecision, err := handler.requests[0].PermissionHandler(context.Background(), sdkpermission.Request{
		ToolName: "Read",
		Input:    map[string]any{"file_path": "README.md"},
	})
	if err != nil {
		t.Fatalf("Read 权限处理失败: %v", err)
	}
	if readDecision.Behavior != sdkpermission.BehaviorAllow {
		t.Fatalf("telegram ingress 的 Read 应自动允许: %+v", readDecision)
	}

	createTaskDecision, err := handler.requests[0].PermissionHandler(context.Background(), sdkpermission.Request{
		ToolName: "create_scheduled_task",
		Input:    map[string]any{"name": "新闻日报"},
	})
	if err != nil {
		t.Fatalf("create_scheduled_task 权限处理失败: %v", err)
	}
	if createTaskDecision.Behavior != sdkpermission.BehaviorDeny {
		t.Fatalf("telegram ingress 的 create_scheduled_task 必须拒绝: %+v", createTaskDecision)
	}

	mcpDeleteTaskDecision, err := handler.requests[0].PermissionHandler(context.Background(), sdkpermission.Request{
		ToolName: "mcp__nexus_automation__delete_scheduled_task",
		Input:    map[string]any{"job_id": "job-1"},
	})
	if err != nil {
		t.Fatalf("mcp delete_scheduled_task 权限处理失败: %v", err)
	}
	if mcpDeleteTaskDecision.Behavior != sdkpermission.BehaviorDeny {
		t.Fatalf("telegram ingress 的 nexus_automation delete_scheduled_task 必须拒绝: %+v", mcpDeleteTaskDecision)
	}

	writeDecision, err := handler.requests[0].PermissionHandler(context.Background(), sdkpermission.Request{
		ToolName: "Write",
		Input:    map[string]any{"file_path": "README.md"},
	})
	if err != nil {
		t.Fatalf("Write 权限处理失败: %v", err)
	}
	if writeDecision.Behavior != sdkpermission.BehaviorDeny {
		t.Fatalf("telegram ingress 的 Write 应默认拒绝: %+v", writeDecision)
	}
}

func TestIngressServiceAutoApproveToolsCannotOpenAutomationMutations(t *testing.T) {
	cfg := newIngressTestConfig(t)
	db := migrateIngressSQLite(t, cfg.DatabaseURL)
	defer func() { _ = db.Close() }()

	agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	handler := &fakeIngressDMHandler{}
	router := NewRouter(cfg, db, agentService, permissionctx.NewContext())
	service := NewIngressService(cfg, agentService, handler, router)

	if _, err := service.Accept(context.Background(), IngressRequest{
		Channel:          "feishu",
		ChatType:         "group",
		Ref:              "oc_group_123",
		Content:          "停止每日新闻定时任务",
		AutoApproveTools: []string{"nexus_automation"},
	}); err != nil {
		t.Fatalf("Accept 失败: %v", err)
	}
	if len(handler.requests) != 1 || handler.requests[0].PermissionHandler == nil {
		t.Fatalf("未下发带权限处理器的请求: %+v", handler.requests)
	}
	decision, err := handler.requests[0].PermissionHandler(context.Background(), sdkpermission.Request{
		ToolName: "mcp__nexus_automation__update_scheduled_task",
		Input:    map[string]any{"job_id": "job-1"},
	})
	if err != nil {
		t.Fatalf("nexus_automation 权限处理失败: %v", err)
	}
	if decision.Behavior != sdkpermission.BehaviorDeny {
		t.Fatalf("auto_approve_tools=nexus_automation 不能开放 mutation: %+v", decision)
	}
	historyDecision, err := handler.requests[0].PermissionHandler(context.Background(), sdkpermission.Request{
		ToolName: "mcp__nexus_automation__find_scheduled_tasks",
		Input:    map[string]any{"query": "每日新闻"},
	})
	if err != nil {
		t.Fatalf("nexus_automation history search 权限处理失败: %v", err)
	}
	if historyDecision.Behavior != sdkpermission.BehaviorAllow {
		t.Fatalf("auto_approve_tools=nexus_automation 应允许历史搜索工具: %+v", historyDecision)
	}
}

func TestIngressServiceAutoApproveAllCannotOpenAutomationMutations(t *testing.T) {
	cfg := newIngressTestConfig(t)
	db := migrateIngressSQLite(t, cfg.DatabaseURL)
	defer func() { _ = db.Close() }()

	agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	handler := &fakeIngressDMHandler{}
	router := NewRouter(cfg, db, agentService, permissionctx.NewContext())
	service := NewIngressService(cfg, agentService, handler, router)

	if _, err := service.Accept(context.Background(), IngressRequest{
		Channel:        "feishu",
		ChatType:       "group",
		Ref:            "oc_group_123",
		Content:        "创建并立即运行一个定时任务",
		AutoApproveAll: true,
	}); err != nil {
		t.Fatalf("Accept 失败: %v", err)
	}
	for _, toolName := range []string{
		"create_scheduled_task",
		"mcp__nexus_automation__update_scheduled_task",
		"mcp__nexus_automation__run_scheduled_task",
	} {
		decision, err := handler.requests[0].PermissionHandler(context.Background(), sdkpermission.Request{
			ToolName: toolName,
			Input:    map[string]any{"job_id": "job-1"},
		})
		if err != nil {
			t.Fatalf("%s permission error: %v", toolName, err)
		}
		if decision.Behavior != sdkpermission.BehaviorDeny {
			t.Fatalf("auto_approve_all 不能开放 %s: %+v", toolName, decision)
		}
	}
}
