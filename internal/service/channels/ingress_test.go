package channels

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"
)

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

	parsedSession := protocol.ParseSessionKey(result.SessionKey)
	if !parsedSession.IsStructured || parsedSession.Kind != protocol.SessionKeyKindAgent ||
		parsedSession.AgentID != defaultAgent.AgentID ||
		parsedSession.Channel != protocol.SessionChannelFeishuSegment ||
		parsedSession.ChatType != "group" ||
		parsedSession.AccountID != "cli_a" ||
		parsedSession.Ref != "oc_group_123" ||
		parsedSession.ThreadID != "omt_thread_1" ||
		parsedSession.Generation == "" {
		t.Fatalf("飞书话题 session_key 不正确: %s (parsed=%+v)", result.SessionKey, parsedSession)
	}
	if len(handler.requests) != 1 || handler.requests[0].SessionKey != result.SessionKey {
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

func TestIngressDoesNotRepeatUncertainControlCommand(t *testing.T) {
	cfg := newIngressTestConfig(t)
	db := migrateIngressSQLite(t, cfg.DatabaseURL)
	defer db.Close()
	agents := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	router := NewRouter(cfg, db, agents, permissionctx.NewContext())
	service := NewIngressService(cfg, agents, &fakeIngressDMHandler{}, router)
	service.SetControlService(NewControlService(cfg, db, agents, router))
	commands := &recordingIngressCommandHandler{err: errors.New("命令执行结果未知")}
	service.SetCommandHandler(commands)
	request := IngressRequest{Channel: "internal", Ref: "chat", Content: "/stop", ReqID: "same-command"}
	if _, err := service.Accept(t.Context(), request); err == nil {
		t.Fatal("预期命令失败")
	}
	_, _ = service.Accept(t.Context(), request)
	if len(commands.requests) != 1 {
		t.Fatalf("未知控制命令被重复执行: %d", len(commands.requests))
	}
}

func TestIngressCrashRecoveryUsesOriginalRoundAndDurableEvidence(t *testing.T) {
	cfg := newIngressTestConfig(t)
	db := migrateIngressSQLite(t, cfg.DatabaseURL)
	defer db.Close()
	agents := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	handler := &fakeIngressDMHandler{}
	router := NewRouter(cfg, db, agents, permissionctx.NewContext())
	control := NewControlService(cfg, db, agents, router)
	service := NewIngressService(cfg, agents, handler, router)
	service.SetControlService(control)
	request := IngressRequest{Channel: "internal", Ref: "chat", Content: "hello", ReqID: "prepared", RoundID: "original"}
	normalized, err := service.normalizeRequest(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	if claimed, _, err := service.claimIngress(t.Context(), normalized); err != nil || !claimed {
		t.Fatalf("claim: %v %v", claimed, err)
	}
	// 模拟在路由准备阶段退出；重投必须沿原轮次继续。
	request.RoundID = "new-round"
	result, err := service.Accept(t.Context(), request)
	if err != nil || result.RoundID != "original" || len(handler.requests) != 1 || handler.requests[0].RoundID != "original" {
		t.Fatalf("prepared recovery: %+v %v", result, err)
	}
	if _, err = db.Exec(`UPDATE im_ingress_messages SET status='processing' WHERE req_id='prepared'`); err != nil {
		t.Fatal(err)
	}
	service.SetRoundIndexReader(func(_ context.Context, session string) (*protocol.SessionRoundIndex, error) {
		if session != normalized.sessionKey {
			t.Fatalf("wrong session: %s", session)
		}
		return &protocol.SessionRoundIndex{Items: []protocol.SessionRoundIndexItem{{RoundID: "original", HasUserMessage: true}}}, nil
	})
	result, err = service.Accept(t.Context(), request)
	if err != nil || !result.Duplicate || len(handler.requests) != 1 {
		t.Fatalf("accepted recovery reran: %+v %v", result, err)
	}
	request.Content = "different command"
	if _, err = service.Accept(t.Context(), request); err == nil {
		t.Fatal("同身份不能替换正文")
	}
}

func TestIngressDispatchBarrierHasOneWinner(t *testing.T) {
	cfg := newIngressTestConfig(t)
	db := migrateIngressSQLite(t, cfg.DatabaseURL)
	defer db.Close()
	agents := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	router := NewRouter(cfg, db, agents, permissionctx.NewContext())
	control := NewControlService(cfg, db, agents, router)
	service := NewIngressService(cfg, agents, &fakeIngressDMHandler{}, router)
	service.SetControlService(control)
	request, err := service.normalizeRequest(t.Context(), IngressRequest{Channel: "internal", Ref: "chat", Content: "hello", ReqID: "same"})
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if claimed, _, err := service.claimIngress(t.Context(), request); err != nil || !claimed {
			t.Fatalf("prepare: %v %v", claimed, err)
		}
	}
	if err = control.beginIngressDispatch(t.Context(), request); err != nil {
		t.Fatal(err)
	}
	if err = control.beginIngressDispatch(t.Context(), request); !errors.Is(err, ErrIngressOutcomeUnknown) {
		t.Fatalf("second dispatcher: %v", err)
	}
	if _, _, err = service.claimIngress(t.Context(), request); !errors.Is(err, ErrIngressOutcomeUnknown) {
		t.Fatalf("unknown accepted: %v", err)
	}
}

func TestIngressRecoveryScansPastUnknownWithoutPlatformRedelivery(t *testing.T) {
	cfg := newIngressTestConfig(t)
	db := migrateIngressSQLite(t, cfg.DatabaseURL)
	defer db.Close()
	agents := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	handler := &fakeIngressDMHandler{}
	router := NewRouter(cfg, db, agents, permissionctx.NewContext())
	control := NewControlService(cfg, db, agents, router)
	service := NewIngressService(cfg, agents, handler, router)
	service.SetControlService(control)
	request, err := service.normalizeRequest(t.Context(), IngressRequest{Channel: "internal", Ref: "chat", Content: "hello"})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i <= ingressRecoveryBatchSize; i++ {
		request.reqID, request.roundID = fmt.Sprintf("message-%03d", i), fmt.Sprintf("round-%03d", i)
		if _, _, err := service.claimIngress(t.Context(), request); err != nil {
			t.Fatal(err)
		}
		if err := control.beginIngressDispatch(t.Context(), request); err != nil {
			t.Fatal(err)
		}
	}
	service.SetRoundIndexReader(func(_ context.Context, session string) (*protocol.SessionRoundIndex, error) {
		if session != request.sessionKey {
			t.Fatalf("wrong session: %s", session)
		}
		return &protocol.SessionRoundIndex{Items: []protocol.SessionRoundIndexItem{
			{RoundID: "round-100", HasUserMessage: true}, {RoundID: "round-099", HasUserMessage: false},
		}}, nil
	})
	cursor := ingressMessageRow{}
	first, err := service.recoverIngressBatch(t.Context(), &cursor)
	if err != nil || !first.HasMore {
		t.Fatalf("first batch: %+v %v", first, err)
	}
	second, err := service.recoverIngressBatch(t.Context(), &cursor)
	if err != nil || second.HasMore || cursor.ReqID != "" {
		t.Fatalf("second batch: %+v %v", second, err)
	}
	var accepted int
	if err := db.QueryRow(`SELECT COUNT(*) FROM im_ingress_messages WHERE status='accepted'`).Scan(&accepted); err != nil {
		t.Fatal(err)
	}
	if accepted != 1 || len(handler.requests) != 0 {
		t.Fatalf("accepted=%d reruns=%d", accepted, len(handler.requests))
	}
	// 无证据的旧消息留在待核验状态，不能伪造受理或重跑。
	unknown, err := control.getIngressMessage(t.Context(), request.ownerUserID, request.channelStored, request.accountID, "message-099")
	if err != nil || unknown.Status != "processing" {
		t.Fatalf("unknown: %+v %v", unknown, err)
	}
}

func TestRoomExternalReplyPromptRequiresPersistedOriginalSession(t *testing.T) {
	cfg := newIngressTestConfig(t)
	db := migrateIngressSQLite(t, cfg.DatabaseURL)
	defer db.Close()
	service := NewIngressService(cfg, nil, nil, nil)
	service.SetControlService(NewControlService(cfg, db, nil, nil))
	ctx := ingressTestOwnerContext("owner")
	_, err := db.Exec(`INSERT INTO im_room_inputs (owner_user_id,root_round_id,pairing_id,binding_version,agent_id,room_id,conversation_id,target_json,content) VALUES ('owner','root','pair',1,'amy','room','topic','{}','hello')`)
	if err != nil {
		t.Fatal(err)
	}
	original := protocol.BuildRoomAgentSessionKey("topic", "amy", protocol.RoomTypeGroup)
	for _, session := range []string{original, "agent:amy:ws:dm:other"} {
		prompt, err := service.roomExternalReplyPrompt(ctx, "root", "amy", session)
		if err != nil {
			t.Fatal(err)
		}
		if (prompt != "") != (session == original) {
			t.Fatalf("prompt for %s: %q", session, prompt)
		}
	}
	prompt, err := service.roomExternalReplyPrompt(ctx, "unrelated", "amy", original)
	if err != nil || prompt != "" {
		t.Fatalf("unrelated input: %q %v", prompt, err)
	}
}
