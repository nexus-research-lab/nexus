package channels

import (
	"context"
	"testing"

	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
)

func TestIngressServiceAcceptInternalBuildsSessionAndRemembersRoute(t *testing.T) {
	cfg := newIngressTestConfig(t)
	db := migrateIngressSQLite(t, cfg.DatabaseURL)
	defer func() { _ = db.Close() }()

	agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
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

	agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
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

	agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
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

	agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
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
