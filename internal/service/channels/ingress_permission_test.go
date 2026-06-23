package channels

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	sqliterepo "github.com/nexus-research-lab/nexus/internal/storage/sqlite"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

func TestIngressServiceFeishuAllowsScheduledTaskSkillWithRestrictiveAgentTools(t *testing.T) {
	cfg := newIngressTestConfig(t)
	db := migrateIngressSQLite(t, cfg.DatabaseURL)
	defer func() { _ = db.Close() }()

	agentService := agentsvc.NewService(cfg, sqliterepo.NewAgentRepository(db))
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

	skillDecision, err := handler.requests[0].PermissionHandler(context.Background(), sdkpermission.Request{
		ToolName: "Skill",
		Input:    map[string]any{"name": "scheduled-task-manager"},
	})
	if err != nil {
		t.Fatalf("Skill 权限处理失败: %v", err)
	}
	if skillDecision.Behavior != sdkpermission.BehaviorAllow {
		t.Fatalf("限制 allowlist 时仍应允许加载托管定时任务 skill: %+v", skillDecision)
	}

	reportDecision, err := handler.requests[0].PermissionHandler(context.Background(), sdkpermission.Request{
		ToolName: "mcp__nexus_automation__get_scheduled_task_daily_report",
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

func TestIngressServiceAcceptTelegramAllowsScheduledTaskToolsOnly(t *testing.T) {
	cfg := newIngressTestConfig(t)
	db := migrateIngressSQLite(t, cfg.DatabaseURL)
	defer func() { _ = db.Close() }()

	agentService := agentsvc.NewService(cfg, sqliterepo.NewAgentRepository(db))
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
	if createTaskDecision.Behavior != sdkpermission.BehaviorAllow {
		t.Fatalf("telegram ingress 的 create_scheduled_task 应自动允许: %+v", createTaskDecision)
	}

	mcpDeleteTaskDecision, err := handler.requests[0].PermissionHandler(context.Background(), sdkpermission.Request{
		ToolName: "mcp__nexus_automation__delete_scheduled_task",
		Input:    map[string]any{"job_id": "job-1"},
	})
	if err != nil {
		t.Fatalf("mcp delete_scheduled_task 权限处理失败: %v", err)
	}
	if mcpDeleteTaskDecision.Behavior != sdkpermission.BehaviorAllow {
		t.Fatalf("telegram ingress 的 nexus_automation delete_scheduled_task 应自动允许: %+v", mcpDeleteTaskDecision)
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

func TestIngressServiceAutoApproveToolsCanAllowNexusAutomationServer(t *testing.T) {
	cfg := newIngressTestConfig(t)
	db := migrateIngressSQLite(t, cfg.DatabaseURL)
	defer func() { _ = db.Close() }()

	agentService := agentsvc.NewService(cfg, sqliterepo.NewAgentRepository(db))
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
		ToolName: "mcp__nexus_automation__disable_scheduled_task",
		Input:    map[string]any{"job_id": "job-1"},
	})
	if err != nil {
		t.Fatalf("nexus_automation 权限处理失败: %v", err)
	}
	if decision.Behavior != sdkpermission.BehaviorAllow {
		t.Fatalf("auto_approve_tools=nexus_automation 应允许 MCP 前缀工具: %+v", decision)
	}
	historyDecision, err := handler.requests[0].PermissionHandler(context.Background(), sdkpermission.Request{
		ToolName: "mcp__nexus_automation__search_scheduled_task_history",
		Input:    map[string]any{"query": "每日新闻"},
	})
	if err != nil {
		t.Fatalf("nexus_automation history search 权限处理失败: %v", err)
	}
	if historyDecision.Behavior != sdkpermission.BehaviorAllow {
		t.Fatalf("auto_approve_tools=nexus_automation 应允许历史搜索工具: %+v", historyDecision)
	}
}
