package channels

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	sqliterepo "github.com/nexus-research-lab/nexus/internal/storage/sqlite"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

type fakeIngressDMHandler struct {
	requests []dmsvc.Request
	err      error
}

type externalSessionNotifyCall struct {
	agentID    string
	sessionKey string
}

type fakeExternalSessionNotifier struct {
	calls []externalSessionNotifyCall
}

func (f *fakeIngressDMHandler) HandleChat(_ context.Context, request dmsvc.Request) error {
	f.requests = append(f.requests, request)
	if f.err != nil {
		return f.err
	}
	return nil
}

func (f *fakeExternalSessionNotifier) NotifyExternalSessionUpdated(_ context.Context, agentID string, sessionKey string) {
	f.calls = append(f.calls, externalSessionNotifyCall{
		agentID:    agentID,
		sessionKey: sessionKey,
	})
}

func TestIngressServiceAcceptInternalBuildsSessionAndRemembersRoute(t *testing.T) {
	cfg := newIngressTestConfig(t)
	db := migrateIngressSQLite(t, cfg.DatabaseURL)
	defer func() { _ = db.Close() }()

	agentService := agentsvc.NewService(cfg, sqliterepo.NewAgentRepository(db))
	handler := &fakeIngressDMHandler{}
	notifier := &fakeExternalSessionNotifier{}
	router := NewRouter(cfg, db, agentService, permissionctx.NewContext())
	service := NewIngressService(cfg, agentService, handler, router)
	service.SetExternalSessionNotifier(notifier)

	result, err := service.Accept(context.Background(), IngressRequest{
		Channel: "internal",
		Ref:     "chat",
		Content: "来自内部系统的消息",
	})
	if err != nil {
		t.Fatalf("Accept 失败: %v", err)
	}

	if result.SessionKey != "agent:nexus:internal:dm:chat" {
		t.Fatalf("session_key 不正确: %s", result.SessionKey)
	}
	if len(handler.requests) != 1 {
		t.Fatalf("聊天请求数量不正确: %d", len(handler.requests))
	}
	if handler.requests[0].SessionKey != result.SessionKey {
		t.Fatalf("聊天请求 session_key 不正确: %+v", handler.requests[0])
	}
	if handler.requests[0].PermissionHandler == nil {
		t.Fatal("internal ingress 应注入权限处理器")
	}
	if handler.requests[0].ExternalReplyTarget != nil {
		t.Fatalf("internal ingress 不应携带外部回复目标: %+v", handler.requests[0].ExternalReplyTarget)
	}

	route, err := router.GetLastRoute(context.Background(), cfg.DefaultAgentID)
	if err != nil {
		t.Fatalf("读取 last route 失败: %v", err)
	}
	if route == nil || route.Channel != ChannelTypeInternal || route.SessionKey != result.SessionKey {
		t.Fatalf("internal route 记忆不正确: %+v", route)
	}
	if len(notifier.calls) != 0 {
		t.Fatalf("internal ingress 不应触发外部 session 通知: %+v", notifier.calls)
	}

	decision, err := handler.requests[0].PermissionHandler(context.Background(), sdkpermission.Request{
		ToolName: "Read",
		Input:    map[string]any{"file_path": "README.md"},
	})
	if err != nil {
		t.Fatalf("执行权限处理器失败: %v", err)
	}
	if decision.Behavior != sdkpermission.BehaviorAllow {
		t.Fatalf("internal ingress 的 Read 应自动允许: %+v", decision)
	}
	writeDecision, err := handler.requests[0].PermissionHandler(context.Background(), sdkpermission.Request{
		ToolName: "Write",
		Input:    map[string]any{"file_path": "README.md"},
	})
	if err != nil {
		t.Fatalf("执行 Write 权限处理器失败: %v", err)
	}
	if writeDecision.Behavior != sdkpermission.BehaviorDeny {
		t.Fatalf("internal ingress 的 Write 应默认拒绝: %+v", writeDecision)
	}
}

func TestIngressServiceAcceptFeishuBuildsSessionAndRemembersRoute(t *testing.T) {
	cfg := newIngressTestConfig(t)
	db := migrateIngressSQLite(t, cfg.DatabaseURL)
	defer func() { _ = db.Close() }()

	agentService := agentsvc.NewService(cfg, sqliterepo.NewAgentRepository(db))
	handler := &fakeIngressDMHandler{}
	notifier := &fakeExternalSessionNotifier{}
	router := NewRouter(cfg, db, agentService, permissionctx.NewContext())
	service := NewIngressService(cfg, agentService, handler, router)
	service.SetExternalSessionNotifier(notifier)

	result, err := service.Accept(context.Background(), IngressRequest{
		Channel:  "feishu",
		ChatType: "group",
		Ref:      "oc_group_123",
		Content:  "检查今天的定时任务发送情况",
	})
	if err != nil {
		t.Fatalf("Accept 失败: %v", err)
	}

	if result.SessionKey != "agent:nexus:fs:group:oc_group_123" {
		t.Fatalf("feishu session_key 不正确: %s", result.SessionKey)
	}
	if result.RememberedDelivery == nil {
		t.Fatal("feishu ingress 应记录回投目标")
	}
	if len(handler.requests) != 1 {
		t.Fatalf("聊天请求数量不正确: %d", len(handler.requests))
	}
	if !handler.requests[0].BroadcastUserMessage {
		t.Fatal("feishu ingress 应实时广播用户输入")
	}
	replyTarget := handler.requests[0].ExternalReplyTarget
	if replyTarget == nil || replyTarget.Channel != ChannelTypeFeishu || replyTarget.To != "oc_group_123" {
		t.Fatalf("feishu DM 请求应携带外部回复目标: %+v", replyTarget)
	}
	route, err := router.GetLastRoute(context.Background(), cfg.DefaultAgentID)
	if err != nil {
		t.Fatalf("读取 last route 失败: %v", err)
	}
	if route == nil || route.Channel != ChannelTypeFeishu || route.To != "oc_group_123" {
		t.Fatalf("feishu route 记忆不正确: %+v", route)
	}
}

func TestIngressServiceAcceptWeixinPersonalPassesReplyTarget(t *testing.T) {
	cfg := newIngressTestConfig(t)
	db := migrateIngressSQLite(t, cfg.DatabaseURL)
	defer func() { _ = db.Close() }()

	agentService := agentsvc.NewService(cfg, sqliterepo.NewAgentRepository(db))
	handler := &fakeIngressDMHandler{}
	notifier := &fakeExternalSessionNotifier{}
	router := NewRouter(cfg, db, agentService, permissionctx.NewContext())
	service := NewIngressService(cfg, agentService, handler, router)
	service.SetExternalSessionNotifier(notifier)

	result, err := service.Accept(context.Background(), IngressRequest{
		Channel:  ChannelTypeWeixinPersonal,
		ChatType: "dm",
		Ref:      "wx-user-1",
		Content:  "你好",
		Delivery: &DeliveryTarget{
			Mode:      DeliveryModeExplicit,
			Channel:   ChannelTypeWeixinPersonal,
			To:        "wx-user-1",
			AccountID: "bot-agent-1",
			ThreadID:  "context-token-1",
		},
	})
	if err != nil {
		t.Fatalf("Accept 失败: %v", err)
	}

	if result.SessionKey != "agent:nexus:weixin-personal:dm:wx-user-1" {
		t.Fatalf("个人微信 session_key 不正确: %s", result.SessionKey)
	}
	if len(handler.requests) != 1 {
		t.Fatalf("聊天请求数量不正确: %d", len(handler.requests))
	}
	if !handler.requests[0].BroadcastUserMessage {
		t.Fatal("个人微信 ingress 应实时广播用户输入")
	}
	replyTarget := handler.requests[0].ExternalReplyTarget
	if replyTarget == nil {
		t.Fatal("个人微信 DM 请求应携带外部回复目标")
	}
	if replyTarget.Channel != ChannelTypeWeixinPersonal ||
		replyTarget.To != "wx-user-1" ||
		replyTarget.AccountID != "bot-agent-1" ||
		replyTarget.ThreadID != "context-token-1" {
		t.Fatalf("个人微信外部回复目标不正确: %+v", replyTarget)
	}
	if len(notifier.calls) != 1 {
		t.Fatalf("个人微信 ingress 应触发一次外部 session 通知，实际: %+v", notifier.calls)
	}
	if notifier.calls[0].agentID != cfg.DefaultAgentID || notifier.calls[0].sessionKey != result.SessionKey {
		t.Fatalf("个人微信外部 session 通知内容不正确: %+v", notifier.calls[0])
	}
}

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

func TestIngressServiceDeduplicatesReqID(t *testing.T) {
	cfg := newIngressTestConfig(t)
	db := migrateIngressSQLite(t, cfg.DatabaseURL)
	defer func() { _ = db.Close() }()

	agentService := agentsvc.NewService(cfg, sqliterepo.NewAgentRepository(db))
	handler := &fakeIngressDMHandler{}
	router := NewRouter(cfg, db, agentService, permissionctx.NewContext())
	control := NewControlService(cfg, db, agentService, router)
	service := NewIngressService(cfg, agentService, handler, router)
	service.SetControlService(control)

	request := IngressRequest{
		Channel: "internal",
		Ref:     "chat",
		Content: "创建每天九点的新闻定时任务",
		RoundID: "evt-1",
		ReqID:   "message-1",
	}
	first, err := service.Accept(context.Background(), request)
	if err != nil {
		t.Fatalf("第一次 Accept 失败: %v", err)
	}
	second, err := service.Accept(context.Background(), request)
	if err != nil {
		t.Fatalf("重复 Accept 不应失败: %v", err)
	}
	if len(handler.requests) != 1 {
		t.Fatalf("重复 req_id 不应再次下发 DM，实际请求数: %d", len(handler.requests))
	}
	if second == nil || !second.Duplicate {
		t.Fatalf("重复消息应返回 duplicate=true: %+v", second)
	}
	if second.SessionKey != first.SessionKey || second.RoundID != first.RoundID || second.ReqID != first.ReqID {
		t.Fatalf("重复消息返回的原始结果不一致: first=%+v second=%+v", first, second)
	}
}

func TestIngressServiceRetriesFailedReqID(t *testing.T) {
	cfg := newIngressTestConfig(t)
	db := migrateIngressSQLite(t, cfg.DatabaseURL)
	defer func() { _ = db.Close() }()

	agentService := agentsvc.NewService(cfg, sqliterepo.NewAgentRepository(db))
	handler := &fakeIngressDMHandler{err: errors.New("dm temporarily unavailable")}
	router := NewRouter(cfg, db, agentService, permissionctx.NewContext())
	control := NewControlService(cfg, db, agentService, router)
	service := NewIngressService(cfg, agentService, handler, router)
	service.SetControlService(control)

	request := IngressRequest{
		Channel: "internal",
		Ref:     "chat",
		Content: "停止每日新闻定时任务",
		RoundID: "evt-1",
		ReqID:   "message-1",
	}
	if _, err := service.Accept(context.Background(), request); err == nil {
		t.Fatal("第一次 DM 失败应返回错误")
	}
	handler.err = nil
	result, err := service.Accept(context.Background(), request)
	if err != nil {
		t.Fatalf("失败后的同 req_id 应允许重试: %v", err)
	}
	if result == nil || result.Duplicate {
		t.Fatalf("失败重试成功不应标记 duplicate: %+v", result)
	}
	if len(handler.requests) != 2 {
		t.Fatalf("失败后重试应再次下发 DM，实际请求数: %d", len(handler.requests))
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

func newIngressTestConfig(t *testing.T) config.Config {
	t.Helper()
	root := t.TempDir()
	return config.Config{
		Host:           "127.0.0.1",
		Port:           18040,
		ProjectName:    "nexus-channel-test",
		APIPrefix:      "/nexus/v1",
		WebSocketPath:  "/nexus/v1/chat/ws",
		DefaultAgentID: "nexus",
		WorkspacePath:  filepath.Join(root, "workspace"),
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(root, "nexus.db"),
	}
}

func migrateIngressSQLite(t *testing.T, databaseURL string) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", databaseURL)
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	if err = goose.SetDialect("sqlite3"); err != nil {
		t.Fatalf("设置 goose 方言失败: %v", err)
	}
	if err = goose.Up(db, ingressMigrationDir(t)); err != nil {
		t.Fatalf("执行 migration 失败: %v", err)
	}
	return db
}

func ingressMigrationDir(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("无法定位当前测试文件")
	}
	return filepath.Join(filepath.Dir(filename), "..", "..", "..", "db", "migrations", "sqlite")
}
