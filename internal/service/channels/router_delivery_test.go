package channels

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func TestRouterRegisterAndStartLetsNewChannelAdoptReplaced(t *testing.T) {
	db := newChannelTestDB(t)
	router := NewRouter(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	if err := router.Start(context.Background()); err != nil {
		t.Fatalf("启动 router 失败: %v", err)
	}
	defer router.Stop(context.Background())

	replaced := &recordingDeliveryChannel{channelType: ChannelTypeWeixinPersonal}
	if err := router.RegisterAndStartForOwner(context.Background(), "owner-a", replaced); err != nil {
		t.Fatalf("注册旧通道失败: %v", err)
	}
	next := &adoptingDeliveryChannel{recordingDeliveryChannel: recordingDeliveryChannel{channelType: ChannelTypeWeixinPersonal}}
	if err := router.RegisterAndStartForOwner(context.Background(), "owner-a", next); err != nil {
		t.Fatalf("注册新通道失败: %v", err)
	}

	if next.adopted != replaced || replaced.stops != 0 || next.starts != 1 {
		t.Fatalf("通道接管状态不正确: adopted=%T stops=%d starts=%d", next.adopted, replaced.stops, next.starts)
	}
}

func TestRouterFailedHotReplacementKeepsReadyChannel(t *testing.T) {
	db := newChannelTestDB(t)
	router := NewRouter(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	if err := router.Start(context.Background()); err != nil {
		t.Fatalf("启动 router 失败: %v", err)
	}
	defer router.Stop(context.Background())

	current := &recordingDeliveryChannel{channelType: ChannelTypeTelegram}
	if err := router.RegisterAndStartForOwner(context.Background(), "owner-a", current); err != nil {
		t.Fatalf("注册当前通道失败: %v", err)
	}
	candidate := &recordingDeliveryChannel{
		channelType: ChannelTypeTelegram,
		startErr:    fmt.Errorf("candidate start failed"),
	}
	err := router.RegisterAndStartForOwner(context.Background(), "owner-a", candidate)
	if err == nil || err.Error() != "candidate start failed" {
		t.Fatalf("候选启动失败必须返回调用者: %v", err)
	}
	if got := router.GetForOwner("owner-a", ChannelTypeTelegram); got != current {
		t.Fatalf("失败候选不应替换当前实例: got=%T", got)
	}
	if !router.IsReadyForOwner("owner-a", ChannelTypeTelegram) {
		t.Fatal("失败热替换后当前实例应保持 ready")
	}
	if current.stops != 0 || candidate.stops != 1 {
		t.Fatalf("失败候选应自行清理且不得停止当前实例: current=%d candidate=%d", current.stops, candidate.stops)
	}
}

func TestRouterStaleStartCompletionCannotOverwriteNewGeneration(t *testing.T) {
	db := newChannelTestDB(t)
	router := NewRouter(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	release := make(chan struct{})
	stale := &blockingDeliveryChannel{
		recordingDeliveryChannel: recordingDeliveryChannel{
			channelType: ChannelTypeTelegram,
			startErr:    fmt.Errorf("stale generation failed"),
		},
		startEntered: make(chan struct{}),
		startRelease: release,
	}
	router.RegisterForOwner("owner-a", stale)

	startDone := make(chan error, 1)
	go func() {
		startDone <- router.Start(context.Background())
	}()
	<-stale.startEntered

	current := &recordingDeliveryChannel{channelType: ChannelTypeTelegram}
	if err := router.RegisterAndStartForOwner(context.Background(), "owner-a", current); err != nil {
		t.Fatalf("注册新 generation 失败: %v", err)
	}
	close(release)
	if err := <-startDone; err != nil {
		t.Fatalf("router 启动失败: %v", err)
	}
	defer router.Stop(context.Background())

	if got := router.GetForOwner("owner-a", ChannelTypeTelegram); got != current {
		t.Fatalf("旧 generation 完成后不应覆盖新实例: got=%T", got)
	}
	if !router.IsReadyForOwner("owner-a", ChannelTypeTelegram) {
		t.Fatal("旧 generation 的失败结果不应把新实例标成未就绪")
	}
}

func TestNewRouterHonorsChannelEnabledFlags(t *testing.T) {
	db := newChannelTestDB(t)
	router := NewRouter(config.Config{
		DatabaseDriver: "sqlite", DiscordEnabled: false, DiscordBotToken: "discord-token",
		TelegramEnabled: false, TelegramBotToken: "telegram-token",
	}, db, nil, nil)

	if router.GetForOwner("", ChannelTypeDiscord) != nil || router.GetForOwner("", ChannelTypeTelegram) != nil {
		t.Fatal("禁用的外部通道不应注册")
	}
	if router.GetForOwner("", ChannelTypeWebSocket) == nil || router.GetForOwner("", ChannelTypeInternal) == nil {
		t.Fatal("内置通道不应受外部通道开关影响")
	}
}

func TestRouterDeliverMessageUsesOwnerScopedChannel(t *testing.T) {
	db := newChannelTestDB(t)
	resolver := &stubAgentResolver{
		agentByID: map[string]*protocol.Agent{
			"agent-a": {AgentID: "agent-a", OwnerUserID: "owner-a"},
			"agent-b": {AgentID: "agent-b", OwnerUserID: "owner-b"},
		},
	}
	router := NewRouter(config.Config{DatabaseDriver: "sqlite"}, db, resolver, nil)
	channelA := &recordingDeliveryChannel{channelType: ChannelTypeTelegram}
	channelB := &recordingDeliveryChannel{channelType: ChannelTypeTelegram}
	router.RegisterForOwner("owner-a", channelA)
	router.RegisterForOwner("owner-b", channelB)
	if err := router.Start(context.Background()); err != nil {
		t.Fatalf("启动 router 失败: %v", err)
	}
	defer router.Stop(context.Background())

	if _, err := router.DeliverMessage(context.Background(), "agent-a", "给 A", DeliveryTarget{
		Mode:    DeliveryModeExplicit,
		Channel: ChannelTypeTelegram,
		To:      "chat-a",
	}); err != nil {
		t.Fatalf("owner-a 投递失败: %v", err)
	}
	if channelA.sentCount() != 1 || channelB.sentCount() != 0 {
		t.Fatalf("owner-a 投递应只进入 A 通道，A=%d B=%d", channelA.sentCount(), channelB.sentCount())
	}

	if _, err := router.DeliverMessage(context.Background(), "agent-b", "给 B", DeliveryTarget{
		Mode:    DeliveryModeExplicit,
		Channel: ChannelTypeTelegram,
		To:      "chat-b",
	}); err != nil {
		t.Fatalf("owner-b 投递失败: %v", err)
	}
	if channelA.sentCount() != 1 || channelB.sentCount() != 1 {
		t.Fatalf("owner-b 投递应只进入 B 通道，A=%d B=%d", channelA.sentCount(), channelB.sentCount())
	}
}

func TestRouterDeliverMessageReturnsReceipt(t *testing.T) {
	db := newChannelTestDB(t)
	resolver := &stubAgentResolver{
		agentByID: map[string]*protocol.Agent{
			"agent-a": {AgentID: "agent-a", OwnerUserID: "owner-a"},
		},
	}
	router := NewRouter(config.Config{DatabaseDriver: "sqlite"}, db, resolver, nil)
	channel := &recordingReceiptDeliveryChannel{
		recordingDeliveryChannel: recordingDeliveryChannel{channelType: ChannelTypeTelegram},
		receipt: channelmessage.NewReceipt(channelmessage.ReceiptParams{
			Channel: ChannelTypeTelegram,
			Target:  "chat-a",
			Parts:   []channelmessage.ReceiptPart{channelmessage.TextPart("42")},
		}),
	}
	router.RegisterForOwner("owner-a", channel)
	if err := router.Start(context.Background()); err != nil {
		t.Fatalf("启动 router 失败: %v", err)
	}
	defer router.Stop(context.Background())

	result, err := router.DeliverMessage(context.Background(), "agent-a", "给 A", DeliveryTarget{
		Mode:    DeliveryModeExplicit,
		Channel: ChannelTypeTelegram,
		To:      "chat-a",
	})
	if err != nil {
		t.Fatalf("receipt 投递失败: %v", err)
	}
	if result.Target.To != "chat-a" {
		t.Fatalf("投递目标未返回解析后结果: %+v", result.Target)
	}
	if result.Receipt == nil || result.Receipt.PrimaryPlatformMessageID != "42" {
		t.Fatalf("投递回执未返回平台 message_id: %+v", result.Receipt)
	}
	if channel.sentCount() != 1 {
		t.Fatalf("receipt-aware 通道应收到 1 次投递，实际 %d", channel.sentCount())
	}
}

func TestRouterDeliverMessageUsesSessionRememberedRouteBeforeAgentRoute(t *testing.T) {
	db := newChannelTestDB(t)
	resolver := &stubAgentResolver{
		agentByID: map[string]*protocol.Agent{
			"agent-a": {AgentID: "agent-a", OwnerUserID: "owner-a"},
		},
	}
	router := NewRouter(config.Config{DatabaseDriver: "sqlite"}, db, resolver, nil)
	channel := &recordingDeliveryChannel{channelType: ChannelTypeTelegram}
	router.RegisterForOwner("owner-a", channel)
	if err := router.Start(context.Background()); err != nil {
		t.Fatalf("启动 router 失败: %v", err)
	}
	defer router.Stop(context.Background())

	sessionA := protocol.BuildAgentSessionKey("agent-a", ChannelTypeTelegram, "dm", "user-a", "")
	sessionB := protocol.BuildAgentSessionKey("agent-a", ChannelTypeTelegram, "dm", "user-b", "")
	if _, err := router.RememberRoute(context.Background(), "agent-a", DeliveryTarget{
		Mode:    DeliveryModeExplicit,
		Channel: ChannelTypeTelegram,
		To:      "agent-latest",
	}); err != nil {
		t.Fatalf("记录 agent 最近目标失败: %v", err)
	}
	if _, err := router.RememberSessionRoute(context.Background(), "agent-a", sessionA, DeliveryTarget{
		Mode:    DeliveryModeExplicit,
		Channel: ChannelTypeTelegram,
		To:      "user-a",
	}); err != nil {
		t.Fatalf("记录 session A 目标失败: %v", err)
	}
	if _, err := router.RememberSessionRoute(context.Background(), "agent-a", sessionB, DeliveryTarget{
		Mode:    DeliveryModeExplicit,
		Channel: ChannelTypeTelegram,
		To:      "user-b",
	}); err != nil {
		t.Fatalf("记录 session B 目标失败: %v", err)
	}

	result, err := router.DeliverMessage(context.Background(), "agent-a", "给 A", DeliveryTarget{
		Mode:       DeliveryModeLast,
		SessionKey: sessionA,
	})
	if err != nil {
		t.Fatalf("session A last 投递失败: %v", err)
	}
	if result.Target.To != "user-a" {
		t.Fatalf("session A 应使用自己的最近目标，不应使用 agent 最近目标: %+v", result.Target)
	}

	result, err = router.DeliverMessage(context.Background(), "agent-a", "给 B", DeliveryTarget{
		Mode:       DeliveryModeLast,
		SessionKey: sessionB,
	})
	if err != nil {
		t.Fatalf("session B last 投递失败: %v", err)
	}
	if result.Target.To != "user-b" {
		t.Fatalf("session B 应使用自己的最近目标，不应被 session A 覆盖: %+v", result.Target)
	}

	result, err = router.DeliverMessage(context.Background(), "agent-a", "给最近 Agent", DeliveryTarget{
		Mode: DeliveryModeLast,
	})
	if err != nil {
		t.Fatalf("agent last 投递失败: %v", err)
	}
	if result.Target.To != "agent-latest" {
		t.Fatalf("未指定 session 时应仍使用 agent 最近目标: %+v", result.Target)
	}
}

func TestRouterPersistsStableContextButNotImmediateCallbackState(t *testing.T) {
	db := newChannelTestDB(t)
	router := NewRouter(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	sessionKey := protocol.BuildAgentAccountSessionKey(
		"agent-a",
		protocol.SessionChannelWeixinPersonal,
		"dm",
		"account-a",
		"user-a",
		"",
	)
	remembered, err := router.RememberSessionRoute(context.Background(), "agent-a", sessionKey, DeliveryTarget{
		Mode:           DeliveryModeExplicit,
		Channel:        ChannelTypeWeixinPersonal,
		To:             "user-a",
		AccountID:      "account-a",
		ContextToken:   "stable-context-token",
		ReplyContextID: "callback-request-id",
		StreamID:       "callback-stream-id",
	})
	if err != nil {
		t.Fatalf("记录 IM session route 失败: %v", err)
	}
	if remembered == nil || remembered.ContextToken != "stable-context-token" {
		t.Fatalf("稳定 context_token 未持久化: %+v", remembered)
	}
	loaded, err := router.GetSessionRoute(context.Background(), "agent-a", sessionKey)
	if err != nil {
		t.Fatalf("读取 IM session route 失败: %v", err)
	}
	if loaded == nil ||
		loaded.ContextToken != "stable-context-token" ||
		loaded.ReplyContextID != "" ||
		loaded.StreamID != "" {
		t.Fatalf("路由只能保存稳定上下文，不能跨定时执行复用 callback req/stream: %+v", loaded)
	}
}

func TestRememberWebSocketRouteDoesNotReplaceExternalIMSessionRoute(t *testing.T) {
	db := newChannelTestDB(t)
	router := NewRouter(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	sessionKey := protocol.BuildAgentAccountSessionKey(
		"agent-a",
		protocol.SessionChannelWeixinPersonal,
		protocol.RoomTypeDM,
		"account-a",
		"user-a",
		"",
	)
	target := DeliveryTarget{
		Mode:         DeliveryModeExplicit,
		Channel:      ChannelTypeWeixinPersonal,
		To:           "user-a",
		AccountID:    "account-a",
		ContextToken: "context-a",
	}
	if _, err := router.RememberRoute(context.Background(), "agent-a", target); err != nil {
		t.Fatalf("记录外部 IM last route 失败: %v", err)
	}
	if _, err := router.RememberSessionRoute(context.Background(), "agent-a", sessionKey, target); err != nil {
		t.Fatalf("记录外部 IM route 失败: %v", err)
	}
	if err := router.RememberWebSocketRoute(context.Background(), sessionKey); err != nil {
		t.Fatalf("浏览器查看外部 IM session 不应失败: %v", err)
	}
	loaded, err := router.GetSessionRoute(context.Background(), "agent-a", sessionKey)
	if err != nil {
		t.Fatalf("读取外部 IM route 失败: %v", err)
	}
	if loaded == nil || loaded.Channel != ChannelTypeWeixinPersonal ||
		loaded.To != "user-a" || loaded.AccountID != "account-a" ||
		loaded.ContextToken != "context-a" {
		t.Fatalf("浏览器订阅覆盖了外部 IM 投递 route: %+v", loaded)
	}
	last, err := router.GetLastRoute(context.Background(), "agent-a")
	if err != nil || last == nil || last.Channel != ChannelTypeWeixinPersonal || last.To != "user-a" {
		t.Fatalf("浏览器订阅覆盖了 Agent 最近 IM route: route=%+v err=%v", last, err)
	}
}

func TestRouterDoesNotDeliverToFailedOwnerChannel(t *testing.T) {
	db := newChannelTestDB(t)
	resolver := &stubAgentResolver{
		agentByID: map[string]*protocol.Agent{
			"agent-a": {AgentID: "agent-a", OwnerUserID: "owner-a"},
		},
	}
	router := NewRouter(config.Config{DatabaseDriver: "sqlite"}, db, resolver, nil)
	failedChannel := &recordingDeliveryChannel{
		channelType: ChannelTypeTelegram,
		startErr:    fmt.Errorf("boom"),
	}
	router.RegisterForOwner("owner-a", failedChannel)
	if err := router.Start(context.Background()); err != nil {
		t.Fatalf("启动 router 不应因单个 owner 通道失败而失败: %v", err)
	}
	defer router.Stop(context.Background())

	if _, err := router.DeliverMessage(context.Background(), "agent-a", "失败通道", DeliveryTarget{
		Mode:    DeliveryModeExplicit,
		Channel: ChannelTypeTelegram,
		To:      "chat-a",
	}); err == nil {
		t.Fatal("启动失败的 owner 通道不应可投递")
	}
	if failedChannel.sentCount() != 0 {
		t.Fatalf("启动失败通道不应收到投递，实际 %d", failedChannel.sentCount())
	}
}

func TestRouterDeliverMessageUsesRememberedWebSocketRoute(t *testing.T) {
	workspaceRoot, workspacePath := newChannelOwnerWorkspace(t, authctx.SystemUserID, "agent-1")
	db := newChannelTestDB(t)
	permission := permissionctx.NewContext()
	resolver := &stubAgentResolver{
		agentByID: map[string]*protocol.Agent{
			"agent-1": {
				AgentID:       "agent-1",
				OwnerUserID:   authctx.SystemUserID,
				WorkspacePath: workspacePath,
			},
		},
	}
	store := workspacestore.NewSessionFileStore(workspaceRoot)
	sessionKey := protocol.BuildAgentSessionKey("agent-1", "ws", "dm", "chat-1", "")
	now := time.Now().UTC()
	if _, err := store.UpsertSession(workspacePath, protocol.Session{
		SessionKey:   sessionKey,
		AgentID:      "agent-1",
		ChannelType:  "websocket",
		ChatType:     "dm",
		Status:       "active",
		CreatedAt:    now,
		LastActivity: now,
		Title:        "Test",
		Options:      map[string]any{},
		IsActive:     true,
	}); err != nil {
		t.Fatalf("创建测试 session 失败: %v", err)
	}

	router := NewRouter(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspaceRoot},
		db,
		resolver,
		permission,
	)
	sender := &stubPermissionSender{key: "sender-1"}
	permission.BindSession(sessionKey, sender)

	if err := router.RememberWebSocketRoute(context.Background(), sessionKey); err != nil {
		t.Fatalf("RememberWebSocketRoute 失败: %v", err)
	}
	result, err := router.DeliverMessage(context.Background(), "agent-1", "自动提醒", DeliveryTarget{Mode: DeliveryModeLast})
	if err != nil {
		t.Fatalf("DeliverMessage 失败: %v", err)
	}
	target := result.Target
	if target.Channel != ChannelTypeWebSocket || target.To != sessionKey {
		t.Fatalf("解析后的投递目标不正确: %+v", target)
	}

	sessionValue, _, err := store.FindSession([]string{workspacePath}, sessionKey)
	if err != nil {
		t.Fatalf("读取测试 session 失败: %v", err)
	}
	if sessionValue == nil {
		t.Fatalf("测试 session 不存在")
	}
	if sessionValue.Status != "closed" || sessionValue.IsActive {
		t.Fatalf("channel delivery 不应把空闲 session 标成 active: %+v", sessionValue)
	}
	history := workspacestore.NewAgentHistoryStore(workspaceRoot)
	messages, err := history.ReadMessages(workspacePath, *sessionValue, nil)
	if err != nil {
		t.Fatalf("读取消息失败: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("期望写入 1 条 assistant 消息，实际 %d", len(messages))
	}
	if stringValue(messages[0]["role"]) != "assistant" {
		t.Fatalf("投递消息角色不正确: %+v", messages)
	}
	if extractAssistantText(messages[0]) != "自动提醒" {
		t.Fatalf("assistant 正文不正确: %+v", messages[0])
	}
	if _, ok := messages[0]["result_summary"].(map[string]any); !ok {
		t.Fatalf("assistant 应挂载 result_summary: %+v", messages[0])
	}

	events := sender.Events()
	if len(events) != 1 {
		t.Fatalf("期望广播 1 条 durable message，实际 %d", len(events))
	}
	if events[0].EventType != protocol.EventTypeMessage {
		t.Fatalf("广播事件类型不正确: %+v", events)
	}
}

func TestRouterDeliverMessagePersistsSharedRoomDelivery(t *testing.T) {
	workspacePath := t.TempDir()
	stateRoot := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", stateRoot)
	db := newChannelTestDB(t)
	permission := permissionctx.NewContext()
	resolver := &stubAgentResolver{
		agentByID: map[string]*protocol.Agent{
			"agent-1": {
				AgentID:       "agent-1",
				WorkspacePath: workspacePath,
			},
		},
	}
	router := NewRouter(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspacePath},
		db,
		resolver,
		permission,
	)

	sessionKey := protocol.BuildRoomSharedSessionKey("conversation-1")
	sender := &stubPermissionSender{key: "room-sender-1"}
	permission.BindSession(sessionKey, sender)

	result, err := router.DeliverMessage(context.Background(), "agent-1", "今日新闻摘要", DeliveryTarget{
		Mode:    DeliveryModeExplicit,
		Channel: ChannelTypeWebSocket,
		To:      sessionKey,
	})
	if err != nil {
		t.Fatalf("Room 共享投递失败: %v", err)
	}
	target := result.Target
	if target.Channel != ChannelTypeWebSocket || target.To != sessionKey || target.SessionKey != sessionKey {
		t.Fatalf("解析后的投递目标不正确: %+v", target)
	}

	roomHistory := workspacestore.NewRoomHistoryStore(workspacePath)
	messages, err := roomHistory.ReadMessages("", "conversation-1", nil)
	if err != nil {
		t.Fatalf("读取 Room 共享历史失败: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("期望写入 1 条 Room assistant 消息，实际 %d", len(messages))
	}
	if stringValue(messages[0]["role"]) != "assistant" {
		t.Fatalf("Room 投递消息角色不正确: %+v", messages[0])
	}
	if stringValue(messages[0]["agent_id"]) != "agent-1" {
		t.Fatalf("Room 投递消息缺少 agent 归属: %+v", messages[0])
	}
	if stringValue(messages[0]["conversation_id"]) != "conversation-1" {
		t.Fatalf("Room 投递消息 conversation_id 不正确: %+v", messages[0])
	}
	if extractAssistantText(messages[0]) != "今日新闻摘要" {
		t.Fatalf("Room assistant 正文不正确: %+v", messages[0])
	}

	events := sender.Events()
	if len(events) != 1 {
		t.Fatalf("期望广播 1 条 Room durable message，实际 %d", len(events))
	}
	if events[0].EventType != protocol.EventTypeMessage ||
		events[0].SessionKey != sessionKey ||
		events[0].ConversationID != "conversation-1" ||
		events[0].AgentID != "agent-1" {
		t.Fatalf("Room 广播事件不正确: %+v", events[0])
	}
}

func TestRouterDeliverMessageCreatesInternalAutomationInbox(t *testing.T) {
	workspaceRoot, workspacePath := newChannelOwnerWorkspace(t, authctx.SystemUserID, "agent-1")
	db := newChannelTestDB(t)
	resolver := &stubAgentResolver{
		agentByID: map[string]*protocol.Agent{
			"agent-1": {
				AgentID:       "agent-1",
				OwnerUserID:   authctx.SystemUserID,
				WorkspacePath: workspacePath,
			},
		},
	}
	router := NewRouter(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspaceRoot},
		db,
		resolver,
		nil,
	)
	if err := router.Start(context.Background()); err != nil {
		t.Fatalf("启动 router 失败: %v", err)
	}
	defer router.Stop(context.Background())

	store := workspacestore.NewSessionFileStore(workspaceRoot)
	sessionKey := protocol.BuildAgentSessionKey(
		"agent-1",
		protocol.SessionChannelInternalSegment,
		"dm",
		protocol.AutomationInboxSessionRef,
		"",
	)
	result, err := router.DeliverMessage(context.Background(), "agent-1", "今日新闻摘要", DeliveryTarget{
		Mode:    DeliveryModeExplicit,
		Channel: ChannelTypeInternal,
		To:      sessionKey,
	})
	if err != nil {
		t.Fatalf("internal 投递失败: %v", err)
	}
	target := result.Target
	if target.Channel != ChannelTypeInternal || target.To != sessionKey || target.SessionKey != sessionKey {
		t.Fatalf("解析后的投递目标不正确: %+v", target)
	}

	sessionValue, _, err := store.FindSession([]string{workspacePath}, sessionKey)
	if err != nil {
		t.Fatalf("读取自动创建 session 失败: %v", err)
	}
	if sessionValue == nil {
		t.Fatal("internal 投递应自动创建定时任务收件箱 session")
	}
	if sessionValue.Title != "定时任务收件箱" || sessionValue.ChannelType != protocol.SessionChannelInternalSegment {
		t.Fatalf("自动创建 session 元数据不正确: %+v", sessionValue)
	}

	history := workspacestore.NewAgentHistoryStore(workspaceRoot)
	messages, err := history.ReadMessages(workspacePath, *sessionValue, nil)
	if err != nil {
		t.Fatalf("读取消息失败: %v", err)
	}
	if len(messages) != 1 || extractAssistantText(messages[0]) != "今日新闻摘要" {
		t.Fatalf("internal 投递历史不正确: %+v", messages)
	}
}

func TestRouterDeliverAutomationResultProjectsExternalSessionIdempotently(t *testing.T) {
	workspaceRoot, workspacePath := newChannelOwnerWorkspace(t, authctx.SystemUserID, "agent-1")
	db := newChannelTestDB(t)
	resolver := &stubAgentResolver{agentByID: map[string]*protocol.Agent{
		"agent-1": {
			AgentID: "agent-1", OwnerUserID: authctx.SystemUserID, WorkspacePath: workspacePath,
		},
	}}
	router := NewRouter(config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspaceRoot}, db, resolver, nil)
	external := &recordingReceiptDeliveryChannel{
		recordingDeliveryChannel: recordingDeliveryChannel{channelType: ChannelTypeTelegram},
		receipt: channelmessage.NewReceipt(channelmessage.ReceiptParams{
			Channel: ChannelTypeTelegram,
			Target:  "chat-1",
			Parts:   []channelmessage.ReceiptPart{channelmessage.TextPart("platform-1")},
		}),
	}
	router.RegisterForOwner(authctx.SystemUserID, external)
	if err := router.Start(context.Background()); err != nil {
		t.Fatalf("启动 router 失败: %v", err)
	}
	defer router.Stop(context.Background())

	sessionKey := protocol.BuildAgentSessionKey("agent-1", protocol.SessionChannelTelegramSegment, "dm", "chat-1", "")
	files := workspacestore.NewSessionFileStore(workspaceRoot).ForOwner(authctx.SystemUserID)
	now := time.Now().UTC()
	sessionValue, err := files.UpsertSession(workspacePath, protocol.Session{
		SessionKey:   sessionKey,
		AgentID:      "agent-1",
		ChannelType:  protocol.SessionChannelTelegram,
		ChatType:     "dm",
		Status:       "closed",
		CreatedAt:    now,
		LastActivity: now,
		Title:        "Telegram",
		Options:      map[string]any{},
	})
	if err != nil || sessionValue == nil {
		t.Fatalf("准备外部 session 失败: session=%+v err=%v", sessionValue, err)
	}
	target := DeliveryTarget{
		Mode:       DeliveryModeExplicit,
		Channel:    ChannelTypeTelegram,
		To:         "chat-1",
		SessionKey: sessionKey,
	}
	delivery := AutomationDeliveryContext{
		JobID:               "task-1",
		RunID:               "run-1",
		TaskName:            "测试任务",
		Instruction:         "测试数数 123",
		ExecutionSessionKey: "agent:agent-1:automation:dm:task-1:run-1",
		ExecutionRoundID:    "round-execution",
	}
	for attempt := 0; attempt < 2; attempt++ {
		if _, err = router.DeliverAutomationResult(context.Background(), "agent-1", "结果 123", target, delivery); err != nil {
			t.Fatalf("第 %d 次 Automation 投递失败: %v", attempt+1, err)
		}
	}

	sessionValue, _, err = files.FindSession([]string{workspacePath}, sessionKey)
	if err != nil || sessionValue == nil {
		t.Fatalf("读取外部 session 失败: session=%+v err=%v", sessionValue, err)
	}
	if sessionValue.MessageCount != 1 {
		t.Fatalf("同 run 重投不得重复增加会话消息数: %+v", sessionValue)
	}
	history := workspacestore.NewAgentHistoryStore(workspaceRoot).ForOwner(authctx.SystemUserID)
	messages, err := history.ReadMessages(workspacePath, *sessionValue, nil)
	if err != nil {
		t.Fatalf("读取投影历史失败: %v", err)
	}
	if len(messages) != 1 || extractAssistantText(messages[0]) != "结果 123" {
		t.Fatalf("Automation 投影应只有一条 assistant: %+v", messages)
	}
	metadata, _ := messages[0]["metadata"].(map[string]any)
	if metadata["source"] != automationDeliverySource || metadata["job_id"] != "task-1" || metadata["run_id"] != "run-1" {
		t.Fatalf("Automation 结构化标识不完整: %+v", metadata)
	}
	externalDelivery, _ := messages[0]["external_delivery"].(map[string]any)
	if externalDelivery["primary_platform_message_id"] != "platform-1" {
		t.Fatalf("平台回执未关联到投影消息: %+v", messages[0])
	}
}
