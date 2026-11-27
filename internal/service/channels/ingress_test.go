// ingress_test.go 覆盖渠道入站消息、命令、租约、权限与重试测试。
package channels

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
	dmsvc "github.com/nexus-research-lab/nexus/internal/service/dm"
	"github.com/nexus-research-lab/nexus/internal/storage"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"
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

func newIngressTestConfig(t *testing.T) config.Config {
	t.Helper()
	root := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", filepath.Join(root, ".nexus"))
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(root, ".nexus"))
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

	handlertest.MigrateSQLiteFromDir(t, databaseURL, ingressMigrationDir(t))
	// 与生产保持相同的单连接、事务和外键语义，避免并发审批使用裸连接竞争写锁。
	db, err := storage.OpenDB(config.Config{DatabaseDriver: "sqlite", DatabaseURL: databaseURL})
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
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

type recordingIngressCommandHandler struct {
	requests     []IngressCommandRequest
	result       IngressCommandResult
	err          error
	pendingCount int
	pendingErr   error
}

func TestIngressControlCommandsCoverEveryExternalIMChannel(t *testing.T) {
	channels := []string{
		ChannelTypeDiscord,
		ChannelTypeTelegram,
		ChannelTypeDingTalk,
		ChannelTypeWeChat,
		ChannelTypeWeixinPersonal,
		ChannelTypeFeishu,
	}
	for index, channelType := range channels {
		t.Run(channelType, func(t *testing.T) {
			cfg := newIngressTestConfig(t)
			db := migrateIngressSQLite(t, cfg.DatabaseURL)
			defer func() { _ = db.Close() }()

			agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
			defaultAgent, err := agentService.GetDefaultAgent(context.Background())
			if err != nil {
				t.Fatalf("初始化默认 Agent 失败: %v", err)
			}
			target := fmt.Sprintf("approval-target-%d", index)
			accountID := ""
			if channelType == ChannelTypeWeixinPersonal {
				accountID = "weixin-account"
			}
			dm := &fakeIngressDMHandler{}
			router := NewRouter(cfg, db, agentService, permissionctx.NewContext())
			deliveryChannel := &recordingDeliveryChannel{channelType: channelType}
			router.RegisterForOwner("", deliveryChannel)
			if err = router.Start(context.Background()); err != nil {
				t.Fatalf("启动 channel router 失败: %v", err)
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
			commands := &recordingIngressCommandHandler{result: IngressCommandResult{
				Handled: true,
				Reply:   "权限请求已批准",
			}}
			service := NewIngressService(cfg, agentService, dm, router)
			service.SetControlService(control)
			service.SetCommandHandler(commands)

			_, err = service.Accept(context.Background(), IngressRequest{
				Channel:   channelType,
				AccountID: accountID,
				ChatType:  protocol.RoomTypeDM,
				Ref:       target,
				Content:   "/y permission_123",
				RoundID:   "round-command",
				ReqID:     "message-command",
				Delivery: &DeliveryTarget{
					Mode:      DeliveryModeExplicit,
					Channel:   channelType,
					To:        target,
					AccountID: accountID,
				},
			})
			if err != nil {
				t.Fatalf("%s Accept 控制命令失败: %v", channelType, err)
			}
			if len(commands.requests) != 1 || len(dm.requests) != 0 ||
				deliveryChannel.sentCount() != 1 || deliveryChannel.texts[0] != "权限请求已批准" {
				t.Fatalf("%s 命令链路不完整: commands=%+v dm=%+v sends=%d texts=%+v", channelType, commands.requests, dm.requests, deliveryChannel.sentCount(), deliveryChannel.texts)
			}
		})
	}
}

func (h *recordingIngressCommandHandler) HandleIngressCommand(
	_ context.Context,
	request IngressCommandRequest,
) (IngressCommandResult, error) {
	h.requests = append(h.requests, request)
	return h.result, h.err
}

func (h *recordingIngressCommandHandler) CountPendingIngressPermissionRequests(
	_ context.Context,
	_ IngressCommandRequest,
) (int, error) {
	return h.pendingCount, h.pendingErr
}

type leaseAwareChannel struct {
	recordingDeliveryChannel

	mu      sync.Mutex
	ingress IngressAcceptor
}

func (c *leaseAwareChannel) SetIngress(ingress IngressAcceptor) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ingress = ingress
}

func (c *leaseAwareChannel) ingressSnapshot() IngressAcceptor {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ingress
}

type recordingIngressAcceptor struct {
	mu       sync.Mutex
	requests []IngressRequest
}

func (a *recordingIngressAcceptor) Accept(
	_ context.Context,
	request IngressRequest,
) (*IngressResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.requests = append(a.requests, request)
	return &IngressResult{
		Channel: request.Channel,
		AgentID: request.AgentID,
	}, nil
}

func (a *recordingIngressAcceptor) requestSnapshot() []IngressRequest {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]IngressRequest(nil), a.requests...)
}

func TestRouterIngressLeaseRevokesStoppedGenerationBeforeCleanup(t *testing.T) {
	router := NewRouter(config.Config{DatabaseDriver: "sqlite"}, newChannelTestDB(t), nil, nil)
	delegate := &recordingIngressAcceptor{}
	router.SetIngress(delegate)
	if err := router.Start(t.Context()); err != nil {
		t.Fatalf("start router: %v", err)
	}
	defer router.Stop(t.Context())

	stopErr := errors.New("adapter stop failed")
	channel := &leaseAwareChannel{recordingDeliveryChannel: recordingDeliveryChannel{
		channelType: ChannelTypeTelegram,
		stopErr:     stopErr,
	}}
	if err := router.RegisterAndStartForOwner(t.Context(), "owner-a", channel); err != nil {
		t.Fatalf("register channel: %v", err)
	}
	staleIngress := channel.ingressSnapshot()
	if staleIngress == nil {
		t.Fatal("registered channel did not receive ingress lease")
	}
	if _, err := staleIngress.Accept(t.Context(), IngressRequest{
		Channel:     ChannelTypeTelegram,
		OwnerUserID: "forged-owner",
		Content:     "before delete",
	}); err != nil {
		t.Fatalf("active lease rejected ingress: %v", err)
	}

	if err := router.UnregisterForOwner(t.Context(), "owner-a", ChannelTypeTelegram); !errors.Is(err, stopErr) {
		t.Fatalf("unregister error = %v, want %v", err, stopErr)
	}
	if _, err := staleIngress.Accept(t.Context(), IngressRequest{
		Channel: ChannelTypeTelegram,
		Content: "after delete",
	}); !errors.Is(err, ErrIngressLeaseRevoked) {
		t.Fatalf("stale ingress error = %v, want revoked lease", err)
	}

	requests := delegate.requestSnapshot()
	if len(requests) != 1 {
		t.Fatalf("delegate accepted %d requests, want exactly the pre-delete request", len(requests))
	}
	if requests[0].OwnerUserID != "owner-a" || requests[0].Channel != ChannelTypeTelegram {
		t.Fatalf("lease did not bind server-side owner/channel: %+v", requests[0])
	}
}

func TestRouterIngressLeaseRejectsOldGenerationAfterHotReplacement(t *testing.T) {
	router := NewRouter(config.Config{DatabaseDriver: "sqlite"}, newChannelTestDB(t), nil, nil)
	var logs bytes.Buffer
	router.SetLogger(slog.New(slog.NewJSONHandler(&logs, nil)))
	delegate := &recordingIngressAcceptor{}
	router.SetIngress(delegate)
	if err := router.Start(t.Context()); err != nil {
		t.Fatalf("start router: %v", err)
	}
	defer router.Stop(t.Context())

	oldChannel := &leaseAwareChannel{recordingDeliveryChannel: recordingDeliveryChannel{
		channelType: ChannelTypeTelegram,
	}}
	if err := router.RegisterAndStartForOwner(t.Context(), "owner-a", oldChannel); err != nil {
		t.Fatalf("register old channel: %v", err)
	}
	oldIngress := oldChannel.ingressSnapshot()

	newChannel := &leaseAwareChannel{recordingDeliveryChannel: recordingDeliveryChannel{
		channelType: ChannelTypeTelegram,
	}}
	if err := router.RegisterAndStartForOwner(t.Context(), "owner-a", newChannel); err != nil {
		t.Fatalf("register replacement channel: %v", err)
	}
	newIngress := newChannel.ingressSnapshot()

	if _, err := oldIngress.Accept(t.Context(), IngressRequest{
		Channel:   ChannelTypeTelegram,
		AccountID: "account-a",
		Content:   "stale",
	}); !errors.Is(err, ErrIngressLeaseRevoked) {
		t.Fatalf("old generation error = %v, want revoked lease", err)
	}
	for _, field := range []string{`"msg":"channel ingress lease rejected"`, `"owner_user_id":"owner-a"`, `"account_id":"account-a"`, `"generation":`} {
		if !strings.Contains(logs.String(), field) {
			t.Fatalf("入口拒绝日志缺少 %s: %s", field, logs.String())
		}
	}
	if strings.Contains(logs.String(), "stale") {
		t.Fatalf("入口拒绝日志不应包含消息正文: %s", logs.String())
	}
	if _, err := newIngress.Accept(t.Context(), IngressRequest{
		Channel: ChannelTypeTelegram,
		Content: "current",
	}); err != nil {
		t.Fatalf("current generation rejected ingress: %v", err)
	}
	if _, err := newIngress.Accept(t.Context(), IngressRequest{
		Channel: ChannelTypeDiscord,
		Content: "cross-channel",
	}); !errors.Is(err, ErrIngressLeaseRevoked) {
		t.Fatalf("cross-channel error = %v, want revoked lease", err)
	}
	if requests := delegate.requestSnapshot(); len(requests) != 1 || requests[0].Content != "current" {
		t.Fatalf("delegate requests = %+v, want current generation only", requests)
	}
}

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
				Options: &protocol.Options{AllowedTools: []string{"Bash"}},
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
				ToolName: "mcp__nexus__command",
				Input: map[string]any{
					"domain": "automation", "action": "plan", "operation": "create",
				},
			})
			if err != nil || decision.Behavior != sdkpermission.BehaviorAllow {
				t.Fatalf("%s paired DM 未继承同 Agent 的 runtime command 权限: decision=%+v err=%v", channelType, decision, err)
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

func TestIngressServiceFeishuAllowsManagedSemanticSkillsWithRestrictiveAgentTools(t *testing.T) {
	cfg := newIngressTestConfig(t)
	db := migrateIngressSQLite(t, cfg.DatabaseURL)
	defer func() { _ = db.Close() }()

	agentService := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	if _, err := agentService.UpdateAgent(context.Background(), cfg.DefaultAgentID, protocol.UpdateRequest{
		Options: &protocol.Options{AllowedTools: []string{"Skill"}},
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

	for _, skillName := range []string{"goal-manager", "execution-orchestrator"} {
		decision, err := handler.requests[0].PermissionHandler(context.Background(), sdkpermission.Request{
			ToolName: "Skill",
			Input:    map[string]any{"name": skillName},
		})
		if err != nil || decision.Behavior != sdkpermission.BehaviorAllow {
			t.Fatalf("限制 allowlist 时仍应允许加载托管 Skill %q: decision=%+v err=%v", skillName, decision, err)
		}
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

func TestIngressServiceRetiredAutomationFamilyIsNotManaged(t *testing.T) {
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
		ToolName: "mcp__nexus_automation__automation_update",
		Input:    map[string]any{"job_id": "job-1"},
	})
	if err != nil {
		t.Fatalf("nexus_automation 权限处理失败: %v", err)
	}
	if decision.Behavior != sdkpermission.BehaviorDeny {
		t.Fatalf("auto_approve_tools=nexus_automation 不能开放 mutation: %+v", decision)
	}
	historyDecision, err := handler.requests[0].PermissionHandler(context.Background(), sdkpermission.Request{
		ToolName: "mcp__nexus_automation__automation_query",
		Input:    map[string]any{"query": "每日新闻"},
	})
	if err != nil {
		t.Fatalf("nexus_automation history search 权限处理失败: %v", err)
	}
	if historyDecision.Behavior != sdkpermission.BehaviorDeny {
		t.Fatalf("retired nexus_automation family must not expand to old query tools: %+v", historyDecision)
	}
}

func TestIngressServiceAutoApproveAllRemainsGeneric(t *testing.T) {
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
	decision, err := handler.requests[0].PermissionHandler(context.Background(), sdkpermission.Request{
		ToolName: "Write",
		Input:    map[string]any{"file_path": "report.md"},
	})
	if err != nil || decision.Behavior != sdkpermission.BehaviorAllow {
		t.Fatalf("auto_approve_all should remain a generic explicit grant: decision=%+v err=%v", decision, err)
	}
}

func TestIngressRetryMarkerStopsBeforeRuntimeDispatch(t *testing.T) {
	cfg := newIngressTestConfig(t)
	db := migrateIngressSQLite(t, cfg.DatabaseURL)
	defer db.Close()
	agents := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	handler := &fakeIngressDMHandler{err: errors.New("runtime admission uncertain")}
	router := NewRouter(cfg, db, agents, permissionctx.NewContext())
	service := NewIngressService(cfg, agents, handler, router)
	service.SetControlService(NewControlService(cfg, db, agents, router))
	var retryable *channelcontract.RetryableIngressError
	if _, err := service.Accept(t.Context(), IngressRequest{Channel: "internal", Ref: "chat"}); !errors.As(err, &retryable) {
		t.Fatalf("准备失败应可安全重试: %v", err)
	}
	_, err := service.Accept(t.Context(), IngressRequest{Channel: "internal", Ref: "chat", Content: "hello", ReqID: "message", RoundID: "round"})
	if err == nil || errors.As(err, &retryable) || len(handler.requests) != 1 {
		t.Fatalf("运行时错误不能标为安全重试: %v calls=%d", err, len(handler.requests))
	}
}
