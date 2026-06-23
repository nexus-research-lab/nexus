package channels

import (
	"context"
	"testing"

	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	sqliterepo "github.com/nexus-research-lab/nexus/internal/storage/sqlite"
)

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
