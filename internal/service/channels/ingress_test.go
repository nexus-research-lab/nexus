package channels

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
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
	requests     []dmsvc.Request
	ownerUserIDs []string
	err          error
}

type externalSessionNotifyCall struct {
	agentID    string
	sessionKey string
}

type fakeExternalSessionNotifier struct {
	calls []externalSessionNotifyCall
}

func (f *fakeIngressDMHandler) HandleChat(ctx context.Context, request dmsvc.Request) error {
	f.requests = append(f.requests, request)
	f.ownerUserIDs = append(f.ownerUserIDs, authctx.OwnerUserID(ctx))
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
		RoundID:  "evt-1",
		ReqID:    "om_1",
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
	if result.Message == nil ||
		result.Message.Channel != ChannelTypeFeishu ||
		result.Message.Target != "oc_group_123" ||
		result.Message.Text != "检查今天的定时任务发送情况" {
		t.Fatalf("feishu ingress 应返回标准消息 envelope: %+v", result.Message)
	}
	if len(handler.requests) != 1 {
		t.Fatalf("聊天请求数量不正确: %d", len(handler.requests))
	}
	if !handler.requests[0].BroadcastUserMessage {
		t.Fatal("feishu ingress 应实时广播用户输入")
	}
	metadata := handler.requests[0].InputOptions.Metadata
	if metadata["im.platform_message_id"] != "om_1" ||
		metadata["im.channel"] != ChannelTypeFeishu ||
		metadata["im.target"] != "oc_group_123" {
		t.Fatalf("feishu ingress 应把消息 envelope 注入 DM metadata: %+v", metadata)
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

func TestIngressServiceAcceptFeishuThreadUsesGroupPairing(t *testing.T) {
	cfg := newIngressTestConfig(t)
	db := migrateIngressSQLite(t, cfg.DatabaseURL)
	defer func() { _ = db.Close() }()

	agentService := agentsvc.NewService(cfg, sqliterepo.NewAgentRepository(db))
	defaultAgent, err := agentService.GetDefaultAgent(context.Background())
	if err != nil {
		t.Fatalf("初始化默认 Agent 失败: %v", err)
	}
	handler := &fakeIngressDMHandler{}
	router := NewRouter(cfg, db, agentService, permissionctx.NewContext())
	control := NewControlService(cfg, db, agentService, router)
	if _, err = control.CreatePairing(context.Background(), "", CreatePairingRequest{
		ChannelType: ChannelTypeFeishu,
		ChatType:    "group",
		ExternalRef: "oc_group_123",
		AgentID:     defaultAgent.AgentID,
	}); err != nil {
		t.Fatalf("创建飞书群级配对失败: %v", err)
	}
	service := NewIngressService(cfg, agentService, handler, router)
	service.SetControlService(control)

	result, err := service.Accept(context.Background(), IngressRequest{
		Channel:   ChannelTypeFeishu,
		AccountID: "cli_a",
		ChatType:  "group",
		Ref:       "oc_group_123",
		ThreadID:  "omt_thread_1",
		Content:   "继续这个话题",
		RoundID:   "evt-thread-1",
		ReqID:     "om_reply_1",
		Delivery: &DeliveryTarget{
			Mode:      DeliveryModeExplicit,
			Channel:   ChannelTypeFeishu,
			To:        "oc_group_123",
			AccountID: "chat_id",
			ThreadID:  "om_reply_1",
		},
	})
	if err != nil {
		t.Fatalf("飞书话题消息应命中群级配对: %v", err)
	}

	expectedSessionKey := "agent:" + defaultAgent.AgentID + ":fs:group:acct:cli_a:oc_group_123:topic:omt_thread_1"
	if result.SessionKey != expectedSessionKey {
		t.Fatalf("飞书话题 session_key 不正确: %s", result.SessionKey)
	}
	if len(handler.requests) != 1 || handler.requests[0].SessionKey != expectedSessionKey {
		t.Fatalf("飞书话题消息未进入 DM 主链: %+v", handler.requests)
	}
	replyTarget := handler.requests[0].ExternalReplyTarget
	if replyTarget == nil ||
		replyTarget.Channel != ChannelTypeFeishu ||
		replyTarget.To != "oc_group_123" ||
		replyTarget.ThreadID != "om_reply_1" {
		t.Fatalf("飞书话题回复目标应指向当前消息: %+v", replyTarget)
	}
}

func TestIngressServiceAcceptPassesChannelOwnerToDM(t *testing.T) {
	cfg := newIngressTestConfig(t)
	db := migrateIngressSQLite(t, cfg.DatabaseURL)
	defer func() { _ = db.Close() }()

	agentService := agentsvc.NewService(cfg, sqliterepo.NewAgentRepository(db))
	ownerCtx := ingressTestOwnerContext("owner-a")
	ownerAgent, err := agentService.GetDefaultAgent(ownerCtx)
	if err != nil {
		t.Fatalf("初始化 owner agent 失败: %v", err)
	}
	handler := &fakeIngressDMHandler{}
	router := NewRouter(cfg, db, agentService, permissionctx.NewContext())
	service := NewIngressService(cfg, agentService, handler, router)

	result, err := service.Accept(context.Background(), IngressRequest{
		OwnerUserID: "owner-a",
		Channel:     "feishu",
		ChatType:    "group",
		Ref:         "oc_group_owner",
		Content:     "总结一下这个群今天讨论的事项",
	})
	if err != nil {
		t.Fatalf("Accept 失败: %v", err)
	}
	if result.AgentID != ownerAgent.AgentID {
		t.Fatalf("ingress 应解析 owner 作用域默认 agent: got=%s want=%s", result.AgentID, ownerAgent.AgentID)
	}
	if len(handler.ownerUserIDs) != 1 || handler.ownerUserIDs[0] != "owner-a" {
		t.Fatalf("DM handler 未收到 channel owner context: %+v", handler.ownerUserIDs)
	}
	expectedSessionKey := "agent:" + ownerAgent.AgentID + ":fs:group:oc_group_owner"
	if len(handler.requests) != 1 || handler.requests[0].SessionKey != expectedSessionKey {
		t.Fatalf("DM 请求不正确: %+v", handler.requests)
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

func TestIngressServiceAcceptsManyWeixinUsersAsSeparateAgentSessions(t *testing.T) {
	cfg := newIngressTestConfig(t)
	db := migrateIngressSQLite(t, cfg.DatabaseURL)
	defer func() { _ = db.Close() }()

	agentService := agentsvc.NewService(cfg, sqliterepo.NewAgentRepository(db))
	ownerCtx := ingressTestOwnerContext("owner-a")
	ownerAgent, err := agentService.GetDefaultAgent(ownerCtx)
	if err != nil {
		t.Fatalf("初始化 owner agent 失败: %v", err)
	}
	handler := &fakeIngressDMHandler{}
	notifier := &fakeExternalSessionNotifier{}
	router := NewRouter(cfg, db, agentService, permissionctx.NewContext())
	service := NewIngressService(cfg, agentService, handler, router)
	service.SetExternalSessionNotifier(notifier)

	cases := []struct {
		ref     string
		content string
		reqID   string
		token   string
	}{
		{ref: "wx-user-1", content: "第一位微信用户", reqID: "wx-msg-1", token: "context-token-1"},
		{ref: "wx-user-2", content: "第二位微信用户", reqID: "wx-msg-2", token: "context-token-2"},
	}
	seenSessionKeys := map[string]bool{}
	for _, tc := range cases {
		result, err := service.Accept(context.Background(), IngressRequest{
			OwnerUserID: "owner-a",
			Channel:     ChannelTypeWeixinPersonal,
			ChatType:    "dm",
			Ref:         tc.ref,
			Content:     tc.content,
			ReqID:       tc.reqID,
			Delivery: &DeliveryTarget{
				Mode:      DeliveryModeExplicit,
				Channel:   ChannelTypeWeixinPersonal,
				To:        tc.ref,
				AccountID: "bot-agent-1",
				ThreadID:  tc.token,
			},
		})
		if err != nil {
			t.Fatalf("多微信用户入站失败 ref=%s err=%v", tc.ref, err)
		}
		expectedSessionKey := "agent:" + ownerAgent.AgentID + ":weixin-personal:dm:" + tc.ref
		if result.AgentID != ownerAgent.AgentID || result.SessionKey != expectedSessionKey {
			t.Fatalf("多微信用户 session 解析不正确 ref=%s result=%+v want=%s", tc.ref, result, expectedSessionKey)
		}
		if seenSessionKeys[result.SessionKey] {
			t.Fatalf("不同微信用户不应复用同一个 IM session: %s", result.SessionKey)
		}
		seenSessionKeys[result.SessionKey] = true
	}

	if len(handler.requests) != len(cases) {
		t.Fatalf("每个微信用户消息都应进入 DM 主链，实际请求数: %d", len(handler.requests))
	}
	if len(handler.ownerUserIDs) != len(cases) {
		t.Fatalf("每个微信用户消息都应携带 owner context: %+v", handler.ownerUserIDs)
	}
	for index, tc := range cases {
		request := handler.requests[index]
		expectedSessionKey := "agent:" + ownerAgent.AgentID + ":weixin-personal:dm:" + tc.ref
		if handler.ownerUserIDs[index] != "owner-a" ||
			request.AgentID != ownerAgent.AgentID ||
			request.SessionKey != expectedSessionKey ||
			request.Content != tc.content {
			t.Fatalf("多微信用户 DM 请求不正确 index=%d request=%+v owners=%+v", index, request, handler.ownerUserIDs)
		}
		if request.ExternalReplyTarget == nil ||
			request.ExternalReplyTarget.Channel != ChannelTypeWeixinPersonal ||
			request.ExternalReplyTarget.To != tc.ref ||
			request.ExternalReplyTarget.ThreadID != tc.token {
			t.Fatalf("多微信用户外部回复目标不正确 index=%d target=%+v", index, request.ExternalReplyTarget)
		}
	}
	if len(notifier.calls) != len(cases) {
		t.Fatalf("每个微信用户 session 更新都应通知前端: %+v", notifier.calls)
	}
	for index, call := range notifier.calls {
		if call.agentID != ownerAgent.AgentID || !seenSessionKeys[call.sessionKey] {
			t.Fatalf("微信用户外部 session 通知不正确 index=%d call=%+v", index, call)
		}
	}
}

func TestIngressServiceScopesSameWeixinUserByAccount(t *testing.T) {
	cfg := newIngressTestConfig(t)
	db := migrateIngressSQLite(t, cfg.DatabaseURL)
	defer func() { _ = db.Close() }()

	agentService := agentsvc.NewService(cfg, sqliterepo.NewAgentRepository(db))
	ownerCtx := ingressTestOwnerContext("owner-a")
	ownerAgent, err := agentService.GetDefaultAgent(ownerCtx)
	if err != nil {
		t.Fatalf("初始化 owner agent 失败: %v", err)
	}
	handler := &fakeIngressDMHandler{}
	router := NewRouter(cfg, db, agentService, permissionctx.NewContext())
	service := NewIngressService(cfg, agentService, handler, router)

	for _, accountID := range []string{"wx-account-1", "wx-account-2"} {
		result, err := service.Accept(context.Background(), IngressRequest{
			OwnerUserID: "owner-a",
			Channel:     ChannelTypeWeixinPersonal,
			AccountID:   accountID,
			ChatType:    "dm",
			Ref:         "same-wx-user",
			Content:     "来自 " + accountID,
			ReqID:       "same-platform-msg",
			Delivery: &DeliveryTarget{
				Mode:      DeliveryModeExplicit,
				Channel:   ChannelTypeWeixinPersonal,
				To:        "same-wx-user",
				AccountID: accountID,
				ThreadID:  "context-token-" + accountID,
			},
		})
		if err != nil {
			t.Fatalf("账号隔离微信入站失败 account=%s err=%v", accountID, err)
		}
		expectedSessionKey := "agent:" + ownerAgent.AgentID + ":weixin-personal:dm:acct:" + accountID + ":same-wx-user"
		if result.SessionKey != expectedSessionKey {
			t.Fatalf("账号隔离微信 session_key 不正确 account=%s got=%s want=%s", accountID, result.SessionKey, expectedSessionKey)
		}
	}
	if len(handler.requests) != 2 {
		t.Fatalf("两个账号的消息都应进入 DM 主链，实际: %d", len(handler.requests))
	}
	if handler.requests[0].SessionKey == handler.requests[1].SessionKey {
		t.Fatalf("不同微信账号不应复用同一 IM session: %+v", handler.requests)
	}
	for index, request := range handler.requests {
		if request.ExternalReplyTarget == nil || request.ExternalReplyTarget.AccountID == "" {
			t.Fatalf("账号隔离微信回复目标应保留 account_id index=%d request=%+v", index, request)
		}
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

func ingressTestOwnerContext(ownerUserID string) context.Context {
	return authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID:     ownerUserID,
		Username:   ownerUserID,
		Role:       authctx.RoleOwner,
		AuthMethod: authctx.AuthMethodLocal,
	})
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
