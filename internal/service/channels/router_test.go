package channels

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	_ "modernc.org/sqlite"
)

type stubAgentResolver struct {
	agentByID map[string]*protocol.Agent
}

func (r *stubAgentResolver) GetAgent(_ context.Context, agentID string) (*protocol.Agent, error) {
	item := r.agentByID[strings.TrimSpace(agentID)]
	if item == nil {
		return nil, fmt.Errorf("agent not found: %s", agentID)
	}
	return item, nil
}

func (r *stubAgentResolver) GetDefaultAgent(_ context.Context) (*protocol.Agent, error) {
	for _, item := range r.agentByID {
		if item != nil && item.IsMain {
			return item, nil
		}
	}
	for _, item := range r.agentByID {
		if item != nil {
			return item, nil
		}
	}
	return nil, nil
}

type stubPermissionSender struct {
	key    string
	mu     sync.Mutex
	events []protocol.EventMessage
}

func (s *stubPermissionSender) Key() string {
	return s.key
}

func (s *stubPermissionSender) IsClosed() bool {
	return false
}

func (s *stubPermissionSender) SendEvent(_ context.Context, event protocol.EventMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
	return nil
}

func (s *stubPermissionSender) Events() []protocol.EventMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]protocol.EventMessage, len(s.events))
	copy(result, s.events)
	return result
}

type recordingDeliveryChannel struct {
	channelType string
	startErr    error

	mu      sync.Mutex
	starts  int
	stops   int
	targets []DeliveryTarget
	texts   []string
}

func (c *recordingDeliveryChannel) ChannelType() string {
	return c.channelType
}

func (c *recordingDeliveryChannel) Start(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.starts++
	return c.startErr
}

func (c *recordingDeliveryChannel) Stop(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stops++
	return nil
}

func (c *recordingDeliveryChannel) SendDeliveryMessage(_ context.Context, target DeliveryTarget, text string) (DeliveryResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.targets = append(c.targets, target)
	c.texts = append(c.texts, text)
	return newDeliveryResult(target, nil), nil
}

func (c *recordingDeliveryChannel) sentCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.targets)
}

type recordingReceiptDeliveryChannel struct {
	recordingDeliveryChannel
	receipt *channelmessage.Receipt
}

func (c *recordingReceiptDeliveryChannel) SendDeliveryMessage(
	ctx context.Context,
	target DeliveryTarget,
	text string,
) (DeliveryResult, error) {
	if _, err := c.recordingDeliveryChannel.SendDeliveryMessage(ctx, target, text); err != nil {
		return DeliveryResult{}, err
	}
	return newDeliveryResult(target, c.receipt), nil
}

type adoptingDeliveryChannel struct {
	recordingDeliveryChannel
	adopted DeliveryChannel
}

func (c *adoptingDeliveryChannel) AdoptReplacedChannel(replaced DeliveryChannel) bool {
	c.adopted = replaced
	return true
}

func extractAssistantText(message protocol.Message) string {
	items, ok := message["content"].([]map[string]any)
	if !ok {
		rawItems, ok := message["content"].([]any)
		if !ok {
			return ""
		}
		items = make([]map[string]any, 0, len(rawItems))
		for _, raw := range rawItems {
			payload, ok := raw.(map[string]any)
			if ok {
				items = append(items, payload)
			}
		}
	}
	parts := make([]string, 0, len(items))
	for _, item := range items {
		if stringValue(item["type"]) != "text" {
			continue
		}
		text := stringValue(item["text"])
		if text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
}

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
	next := &adoptingDeliveryChannel{
		recordingDeliveryChannel: recordingDeliveryChannel{channelType: ChannelTypeWeixinPersonal},
	}
	if err := router.RegisterAndStartForOwner(context.Background(), "owner-a", next); err != nil {
		t.Fatalf("注册新通道失败: %v", err)
	}

	if next.adopted != replaced {
		t.Fatalf("新通道应接管旧通道: got=%T", next.adopted)
	}
	if replaced.stops != 0 {
		t.Fatalf("接管成功时旧通道不应被停止，stops=%d", replaced.stops)
	}
	if next.starts != 1 {
		t.Fatalf("新通道仍应启动，starts=%d", next.starts)
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
	workspacePath := t.TempDir()
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
	store := workspacestore.NewSessionFileStore(workspacePath)
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
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspacePath},
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
	history := workspacestore.NewAgentHistoryStore(workspacePath)
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
	t.Setenv("NEXUS_CONFIG_DIR", t.TempDir())
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
	messages, err := roomHistory.ReadMessages("conversation-1", nil)
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
	workspacePath := t.TempDir()
	db := newChannelTestDB(t)
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
		nil,
	)
	if err := router.Start(context.Background()); err != nil {
		t.Fatalf("启动 router 失败: %v", err)
	}
	defer router.Stop(context.Background())

	store := workspacestore.NewSessionFileStore(workspacePath)
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

	history := workspacestore.NewAgentHistoryStore(workspacePath)
	messages, err := history.ReadMessages(workspacePath, *sessionValue, nil)
	if err != nil {
		t.Fatalf("读取消息失败: %v", err)
	}
	if len(messages) != 1 || extractAssistantText(messages[0]) != "今日新闻摘要" {
		t.Fatalf("internal 投递历史不正确: %+v", messages)
	}
}

func TestNewRouterHonorsChannelEnabledFlags(t *testing.T) {
	db := newChannelTestDB(t)
	router := NewRouter(
		config.Config{
			DatabaseDriver:   "sqlite",
			DiscordEnabled:   false,
			DiscordBotToken:  "discord-token",
			TelegramEnabled:  false,
			TelegramBotToken: "telegram-token",
		},
		db,
		nil,
		nil,
	)

	if router.Get(ChannelTypeDiscord) != nil {
		t.Fatal("DISCORD_ENABLED=false 时不应注册 discord 通道")
	}
	if router.Get(ChannelTypeTelegram) != nil {
		t.Fatal("TELEGRAM_ENABLED=false 时不应注册 telegram 通道")
	}
	if router.Get(ChannelTypeWebSocket) == nil {
		t.Fatal("websocket 通道不应受开关影响")
	}
	if router.Get(ChannelTypeInternal) == nil {
		t.Fatal("internal 通道不应受开关影响")
	}
}

func newChannelTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_")))
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	schema := `
	CREATE TABLE automation_delivery_routes (
	    route_id VARCHAR(64) NOT NULL PRIMARY KEY,
	    agent_id VARCHAR(64) NOT NULL,
	    session_key VARCHAR(512) NOT NULL DEFAULT '',
	    mode VARCHAR(32) NOT NULL,
	    channel VARCHAR(64),
	    "to" VARCHAR(255),
	    account_id VARCHAR(64),
	    thread_id VARCHAR(255),
	    enabled BOOLEAN NOT NULL,
	    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
	    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL
	);
	CREATE TABLE im_channel_configs (
	    owner_user_id VARCHAR(64) NOT NULL,
	    channel_type VARCHAR(32) NOT NULL,
	    agent_id VARCHAR(64) NOT NULL,
	    status VARCHAR(32) NOT NULL DEFAULT 'configured',
	    config_json TEXT NOT NULL DEFAULT '{}',
	    credentials_encrypted TEXT,
	    last_error TEXT,
	    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
	    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
	    PRIMARY KEY (owner_user_id, channel_type)
	);
	CREATE TABLE im_channel_accounts (
	    owner_user_id VARCHAR(64) NOT NULL,
	    channel_type VARCHAR(32) NOT NULL,
	    account_id VARCHAR(255) NOT NULL,
	    user_id VARCHAR(255) NOT NULL DEFAULT '',
	    status VARCHAR(32) NOT NULL DEFAULT 'connected',
	    config_json TEXT NOT NULL DEFAULT '{}',
	    credentials_encrypted TEXT,
	    last_error TEXT,
	    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
	    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
	    PRIMARY KEY (owner_user_id, channel_type, account_id)
	);
	CREATE TABLE im_pairings (
	    pairing_id VARCHAR(64) NOT NULL PRIMARY KEY,
		    owner_user_id VARCHAR(64) NOT NULL,
		    channel_type VARCHAR(32) NOT NULL,
		    account_id VARCHAR(255) NOT NULL DEFAULT '',
		    chat_type VARCHAR(16) NOT NULL,
	    external_ref VARCHAR(255) NOT NULL,
	    thread_id VARCHAR(255) NOT NULL DEFAULT '',
	    external_name VARCHAR(255),
	    agent_id VARCHAR(64) NOT NULL,
	    status VARCHAR(32) NOT NULL DEFAULT 'pending',
	    source VARCHAR(32) NOT NULL DEFAULT 'manual',
	    last_message_at DATETIME,
	    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
	    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
		    UNIQUE (owner_user_id, channel_type, account_id, chat_type, external_ref, thread_id)
		);`
	if _, err = db.Exec(schema); err != nil {
		t.Fatalf("初始化 delivery schema 失败: %v", err)
	}
	return db
}
