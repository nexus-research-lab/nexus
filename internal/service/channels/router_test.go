package channels

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
	dingchatbot "github.com/open-dingtalk/dingtalk-stream-sdk-go/chatbot"

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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type recordingIngressAcceptor struct {
	requests []IngressRequest
	err      error
}

func (r *recordingIngressAcceptor) Accept(_ context.Context, request IngressRequest) (*IngressResult, error) {
	r.requests = append(r.requests, request)
	if r.err != nil {
		return nil, r.err
	}
	return &IngressResult{
		Channel: request.Channel,
		AgentID: request.AgentID,
		ReqID:   request.ReqID,
	}, nil
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

func TestDiscordChannelSendDeliveryMessage(t *testing.T) {
	requests := make([]*http.Request, 0)
	payloads := make([]map[string]any, 0)
	channel := newDiscordChannel("token-1", &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requests = append(requests, request)
			var payload map[string]any
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				return nil, fmt.Errorf("解析 Discord 请求失败: %w", err)
			}
			payloads = append(payloads, payload)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
				Header:     make(http.Header),
			}, nil
		}),
	})
	channel.baseURL = "https://discord.test/api/v10"

	text := strings.Repeat("a", 2400)
	if _, err := channel.SendDeliveryMessage(context.Background(), DeliveryTarget{
		Mode:    DeliveryModeExplicit,
		Channel: ChannelTypeDiscord,
		To:      "123456",
	}, text); err != nil {
		t.Fatalf("Discord 发送失败: %v", err)
	}
	if len(requests) != 2 {
		t.Fatalf("期望分片发送 2 次，实际 %d", len(requests))
	}
	if got := requests[0].Header.Get("Authorization"); got != "Bot token-1" {
		t.Fatalf("Authorization 头不正确: %s", got)
	}
	if !strings.HasSuffix(requests[0].URL.Path, "/channels/123456/messages") {
		t.Fatalf("Discord 路径不正确: %s", requests[0].URL.Path)
	}
	allowedMentions, ok := payloads[0]["allowed_mentions"].(map[string]any)
	if !ok {
		t.Fatalf("Discord payload 应禁用 mention 解析: %+v", payloads[0])
	}
	parseValues, ok := allowedMentions["parse"].([]any)
	if !ok || len(parseValues) != 0 {
		t.Fatalf("Discord allowed_mentions.parse 应为空: %+v", allowedMentions)
	}
}

func TestDiscordChannelSendDeliveryTyping(t *testing.T) {
	requests := make([]*http.Request, 0)
	channel := newDiscordChannel("token-1", &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requests = append(requests, request)
			return &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       io.NopCloser(strings.NewReader(``)),
				Header:     make(http.Header),
			}, nil
		}),
	})
	channel.baseURL = "https://discord.test/api/v10"

	if err := channel.SendDeliveryTyping(context.Background(), DeliveryTarget{
		Mode:     DeliveryModeExplicit,
		Channel:  ChannelTypeDiscord,
		To:       "channel-1",
		ThreadID: "thread-1",
	}, false); err != nil {
		t.Fatalf("Discord typing stop 应静默忽略: %v", err)
	}
	if len(requests) != 0 {
		t.Fatalf("Discord typing stop 不应请求 API，实际 %d", len(requests))
	}

	if err := channel.SendDeliveryTyping(context.Background(), DeliveryTarget{
		Mode:     DeliveryModeExplicit,
		Channel:  ChannelTypeDiscord,
		To:       "channel-1",
		ThreadID: "thread-1",
	}, true); err != nil {
		t.Fatalf("Discord typing start 失败: %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("期望 typing 请求 1 次，实际 %d", len(requests))
	}
	if requests[0].Method != http.MethodPost || !strings.HasSuffix(requests[0].URL.Path, "/channels/thread-1/typing") {
		t.Fatalf("Discord typing 路径不正确: %s %s", requests[0].Method, requests[0].URL.Path)
	}
	if got := requests[0].Header.Get("Authorization"); got != "Bot token-1" {
		t.Fatalf("Discord typing Authorization 不正确: %s", got)
	}
}

func TestTelegramChannelSendDeliveryMessage(t *testing.T) {
	requests := make([]*http.Request, 0)
	var payload map[string]any
	channel := newTelegramChannel("token-2", &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requests = append(requests, request)
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				return nil, fmt.Errorf("解析 Telegram 请求失败: %w", err)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
				Header:     make(http.Header),
			}, nil
		}),
	})
	channel.baseURL = "https://telegram.test"

	if _, err := channel.SendDeliveryMessage(context.Background(), DeliveryTarget{
		Mode:     DeliveryModeExplicit,
		Channel:  ChannelTypeTelegram,
		To:       "-1001",
		ThreadID: "12",
	}, "hello"); err != nil {
		t.Fatalf("Telegram 发送失败: %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("期望发送 1 次，实际 %d", len(requests))
	}
	if !strings.HasSuffix(requests[0].URL.Path, "/bottoken-2/sendMessage") {
		t.Fatalf("Telegram 路径不正确: %s", requests[0].URL.Path)
	}
	if payload["chat_id"] != "-1001" || payload["message_thread_id"] != float64(12) {
		t.Fatalf("Telegram topic payload 不正确: %+v", payload)
	}
	if payload["disable_web_page_preview"] != true {
		t.Fatalf("Telegram 应关闭链接预览: %+v", payload)
	}
}

func TestTelegramChannelSendDeliveryMessageReturnsReceipt(t *testing.T) {
	channel := newTelegramChannel("token-2", &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{"ok":true,"result":{"message_id":42}}`)),
				Header:     make(http.Header),
			}, nil
		}),
	})
	channel.baseURL = "https://telegram.test"

	result, err := channel.SendDeliveryMessage(context.Background(), DeliveryTarget{
		Mode:     DeliveryModeExplicit,
		Channel:  ChannelTypeTelegram,
		To:       "-1001",
		ThreadID: "12",
	}, "hello")
	if err != nil {
		t.Fatalf("Telegram receipt 发送失败: %v", err)
	}
	receipt := result.Receipt
	if receipt == nil || receipt.PrimaryPlatformMessageID != "42" {
		t.Fatalf("Telegram receipt 未记录 message_id: %+v", receipt)
	}
	if receipt.Channel != ChannelTypeTelegram || receipt.Target != "-1001" || receipt.ThreadID != "12" {
		t.Fatalf("Telegram receipt 目标信息不正确: %+v", receipt)
	}
}

func TestTelegramChannelSendDeliveryTyping(t *testing.T) {
	requests := make([]*http.Request, 0)
	var payload map[string]any
	channel := newTelegramChannel("token-2", &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requests = append(requests, request)
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				return nil, fmt.Errorf("解析 Telegram typing 请求失败: %w", err)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
				Header:     make(http.Header),
			}, nil
		}),
	})
	channel.baseURL = "https://telegram.test"

	if err := channel.SendDeliveryTyping(context.Background(), DeliveryTarget{
		Mode:     DeliveryModeExplicit,
		Channel:  ChannelTypeTelegram,
		To:       "-1001",
		ThreadID: "12",
	}, false); err != nil {
		t.Fatalf("Telegram typing stop 应静默忽略: %v", err)
	}
	if len(requests) != 0 {
		t.Fatalf("Telegram typing stop 不应请求 API，实际 %d", len(requests))
	}

	if err := channel.SendDeliveryTyping(context.Background(), DeliveryTarget{
		Mode:     DeliveryModeExplicit,
		Channel:  ChannelTypeTelegram,
		To:       "-1001",
		ThreadID: "12",
	}, true); err != nil {
		t.Fatalf("Telegram typing start 失败: %v", err)
	}
	if len(requests) != 1 {
		t.Fatalf("期望 typing 请求 1 次，实际 %d", len(requests))
	}
	if !strings.HasSuffix(requests[0].URL.Path, "/bottoken-2/sendChatAction") {
		t.Fatalf("Telegram typing 路径不正确: %s", requests[0].URL.Path)
	}
	if payload["chat_id"] != "-1001" || payload["action"] != "typing" || payload["message_thread_id"] != float64(12) {
		t.Fatalf("Telegram typing payload 不正确: %+v", payload)
	}
}

func TestTelegramChannelSendDeliveryGeneralTopicHandling(t *testing.T) {
	var messagePayload map[string]any
	var typingPayload map[string]any
	requests := make([]*http.Request, 0, 2)
	channel := newTelegramChannel("token-2", &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requests = append(requests, request)
			var payload map[string]any
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				return nil, fmt.Errorf("解析 Telegram 请求失败: %w", err)
			}
			if strings.HasSuffix(request.URL.Path, "/sendMessage") {
				messagePayload = payload
			}
			if strings.HasSuffix(request.URL.Path, "/sendChatAction") {
				typingPayload = payload
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
				Header:     make(http.Header),
			}, nil
		}),
	})
	channel.baseURL = "https://telegram.test"
	target := DeliveryTarget{
		Mode:     DeliveryModeExplicit,
		Channel:  ChannelTypeTelegram,
		To:       "-1001",
		ThreadID: "1",
	}

	if _, err := channel.SendDeliveryMessage(context.Background(), target, "hello"); err != nil {
		t.Fatalf("Telegram General topic 发送失败: %v", err)
	}
	if err := channel.SendDeliveryTyping(context.Background(), target, true); err != nil {
		t.Fatalf("Telegram General topic typing 失败: %v", err)
	}
	if len(requests) != 2 {
		t.Fatalf("期望 Telegram 请求 2 次，实际 %d", len(requests))
	}
	if _, ok := messagePayload["message_thread_id"]; ok {
		t.Fatalf("Telegram sendMessage 不应携带 General topic thread_id=1: %+v", messagePayload)
	}
	if typingPayload["message_thread_id"] != float64(1) {
		t.Fatalf("Telegram sendChatAction 应携带 General topic thread_id=1: %+v", typingPayload)
	}
}

func TestTelegramFetchUpdatesSubscribesEditedMessages(t *testing.T) {
	var payload map[string]any
	channel := newTelegramChannel("token-2", &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				return nil, fmt.Errorf("解析 Telegram getUpdates 请求失败: %w", err)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(`{
					"ok": true,
					"result": [{
						"update_id": 4,
						"edited_message": {
							"message_id": 9,
							"text": "edited",
							"from": {"id": 8, "is_bot": false},
							"chat": {"id": 7, "type": "private"}
						}
					}]
				}`)),
				Header: make(http.Header),
			}, nil
		}),
	})
	channel.baseURL = "https://telegram.test"

	updates, nextOffset, err := channel.fetchUpdates(context.Background(), 3)
	if err != nil {
		t.Fatalf("Telegram getUpdates 失败: %v", err)
	}
	if len(updates) != 1 || updates[0].EditedMessage == nil || nextOffset != 5 {
		t.Fatalf("Telegram edited update 解析不正确: updates=%+v next=%d", updates, nextOffset)
	}
	allowed, ok := payload["allowed_updates"].([]any)
	if !ok {
		t.Fatalf("Telegram allowed_updates 未发送: %+v", payload)
	}
	foundEdited := false
	for _, item := range allowed {
		if item == "edited_message" {
			foundEdited = true
			break
		}
	}
	if !foundEdited {
		t.Fatalf("Telegram allowed_updates 应包含 edited_message: %+v", allowed)
	}
}

func TestTelegramFetchUpdatesRedactsBotTokenInErrors(t *testing.T) {
	token := "123456:secret-token"
	channel := newTelegramChannel(token, &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return nil, fmt.Errorf("boom %s", request.URL.String())
		}),
	})
	channel.baseURL = "https://telegram.test"

	_, _, err := channel.fetchUpdates(context.Background(), 0)
	if err == nil {
		t.Fatal("Telegram getUpdates 应返回错误")
	}
	if strings.Contains(err.Error(), token) {
		t.Fatalf("Telegram 错误不应包含 bot token: %s", err)
	}
	if !strings.Contains(err.Error(), "bot<redacted>") {
		t.Fatalf("Telegram 错误应标记 token 已脱敏: %s", err)
	}
}

func TestTelegramChannelHandleEditedUpdateUsesDistinctReqID(t *testing.T) {
	channel := newTelegramChannel("token-2", nil)
	ingress := &recordingIngressAcceptor{}
	channel.SetIngress(ingress)

	channel.handleUpdate(context.Background(), telegramUpdate{
		UpdateID: 10,
		Message: &telegramMessage{
			MessageID: 9,
			Text:      "original",
			From:      &telegramUser{ID: 8},
			Chat:      telegramChat{ID: 7, Type: "private"},
		},
	})
	channel.handleUpdate(context.Background(), telegramUpdate{
		UpdateID: 11,
		EditedMessage: &telegramMessage{
			MessageID: 9,
			Text:      "edited",
			From:      &telegramUser{ID: 8},
			Chat:      telegramChat{ID: 7, Type: "private"},
		},
	})

	if len(ingress.requests) != 2 {
		t.Fatalf("Telegram 原消息和编辑事件都应进入 ingress: %+v", ingress.requests)
	}
	if ingress.requests[0].ReqID == ingress.requests[1].ReqID {
		t.Fatalf("Telegram 编辑事件不应复用原消息 req_id: %+v", ingress.requests)
	}
	if ingress.requests[1].ReqID != "9:edited:11" {
		t.Fatalf("Telegram 编辑事件 req_id 不正确: %q", ingress.requests[1].ReqID)
	}
	if ingress.requests[1].Content != "edited" || !ingress.requests[1].Message.Edited {
		t.Fatalf("Telegram 编辑事件内容未保留: %+v", ingress.requests[1])
	}
}

func TestTelegramChannelHandleUpdateSendsPairingApprovalNotice(t *testing.T) {
	var outboundRequests int
	var outboundPayload map[string]any
	channel := newTelegramChannel("token-2", &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			outboundRequests++
			if !strings.HasSuffix(request.URL.Path, "/bottoken-2/sendMessage") {
				t.Fatalf("待配对提醒应调用 Telegram sendMessage，实际 path=%s", request.URL.Path)
			}
			if err := json.NewDecoder(request.Body).Decode(&outboundPayload); err != nil {
				t.Fatalf("解析 Telegram 待配对提醒失败: %v", err)
			}
			return jsonResponse(`{"ok":true,"result":{"message_id":42}}`), nil
		}),
	})
	channel.baseURL = "https://telegram.test"
	ingress := &recordingIngressAcceptor{err: &pairingApprovalError{
		PairingID: "pair_pending_1",
		Message:   "IM 对象尚未配对授权，请先在配对控制台批准",
	}}
	channel.SetIngress(ingress)

	channel.handleUpdate(context.Background(), telegramUpdate{
		Message: &telegramMessage{
			MessageID: 8,
			Text:      "hello",
			From:      &telegramUser{ID: 7},
			Chat:      telegramChat{ID: 7, Type: "private"},
		},
	})

	if len(ingress.requests) != 1 {
		t.Fatalf("Telegram 消息未进入 ingress: %+v", ingress.requests)
	}
	if outboundRequests != 1 {
		t.Fatalf("待配对授权应回发配对提醒，实际请求数: %d", outboundRequests)
	}
	text := fmt.Sprint(outboundPayload["text"])
	if !strings.Contains(text, "配对控制台") || !strings.Contains(text, "pair_pending_1") {
		t.Fatalf("待配对提醒文案不正确: %q", text)
	}
	if strings.Contains(text, "消息处理失败") {
		t.Fatalf("待配对提醒不应伪装成处理失败: %q", text)
	}
}

func TestFeishuChannelSendDeliveryMessage(t *testing.T) {
	var tokenRequests int
	var messagePayload map[string]string
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			tokenRequests++
			var payload map[string]string
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				return nil, fmt.Errorf("解析 token 请求失败: %w", err)
			}
			if payload["app_id"] != "cli_test" || payload["app_secret"] != "secret_test" {
				return nil, fmt.Errorf("token 请求凭据不正确: %+v", payload)
			}
			return jsonResponse(`{"code":0,"tenant_access_token":"tenant-token","expire":7200}`), nil
		case "/open-apis/im/v1/messages":
			if request.URL.Query().Get("receive_id_type") != "chat_id" {
				return nil, fmt.Errorf("receive_id_type 不正确: %s", request.URL.RawQuery)
			}
			if request.Header.Get("Authorization") != "Bearer tenant-token" {
				return nil, fmt.Errorf("Authorization 不正确: %s", request.Header.Get("Authorization"))
			}
			if err := json.NewDecoder(request.Body).Decode(&messagePayload); err != nil {
				return nil, fmt.Errorf("解析消息请求失败: %w", err)
			}
			return jsonResponse(`{"code":0,"msg":"ok"}`), nil
		default:
			return nil, fmt.Errorf("未知飞书请求路径: %s", request.URL.Path)
		}
	})}

	channel := newFeishuChannel("cli_test", "secret_test", client).WithConnectionMode("webhook")
	channel.baseURL = "https://feishu.test"
	if err := channel.Start(context.Background()); err != nil {
		t.Fatalf("飞书通道启动失败: %v", err)
	}
	if _, err := channel.SendDeliveryMessage(context.Background(), DeliveryTarget{
		Mode:    DeliveryModeExplicit,
		Channel: ChannelTypeFeishu,
		To:      "oc_group_123",
	}, "今日新闻摘要"); err != nil {
		t.Fatalf("飞书发送失败: %v", err)
	}
	if tokenRequests != 1 {
		t.Fatalf("token 请求次数不正确: %d", tokenRequests)
	}
	if messagePayload["receive_id"] != "oc_group_123" || messagePayload["msg_type"] != "text" {
		t.Fatalf("飞书消息请求不正确: %+v", messagePayload)
	}
	var content map[string]string
	if err := json.Unmarshal([]byte(messagePayload["content"]), &content); err != nil {
		t.Fatalf("解析飞书消息 content 失败: %v", err)
	}
	if content["text"] != "今日新闻摘要" {
		t.Fatalf("飞书消息正文不正确: %+v", content)
	}
}

func TestDingTalkChannelSendDeliveryMessage(t *testing.T) {
	var tokenRequests int
	var messagePayload map[string]string
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/v1.0/oauth2/accessToken":
			tokenRequests++
			var payload map[string]string
			if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
				return nil, fmt.Errorf("解析钉钉 token 请求失败: %w", err)
			}
			if payload["appKey"] != "ding-client" || payload["appSecret"] != "ding-secret" {
				return nil, fmt.Errorf("钉钉 token 请求凭据不正确: %+v", payload)
			}
			return jsonResponse(`{"accessToken":"ding-token","expireIn":7200}`), nil
		case "/v1.0/robot/groupMessages/send":
			if request.Header.Get("x-acs-dingtalk-access-token") != "ding-token" {
				return nil, fmt.Errorf("钉钉 Authorization 不正确: %s", request.Header.Get("x-acs-dingtalk-access-token"))
			}
			if err := json.NewDecoder(request.Body).Decode(&messagePayload); err != nil {
				return nil, fmt.Errorf("解析钉钉消息请求失败: %w", err)
			}
			return jsonResponse(`{}`), nil
		default:
			return nil, fmt.Errorf("未知钉钉请求路径: %s", request.URL.Path)
		}
	})}

	channel := newDingTalkChannel("ding-client", "ding-secret", "robot-code", client)
	channel.baseURL = "https://dingtalk.test"

	if _, err := channel.SendDeliveryMessage(context.Background(), DeliveryTarget{
		Mode:    DeliveryModeExplicit,
		Channel: ChannelTypeDingTalk,
		To:      "cid-group-1",
	}, "今日新闻摘要"); err != nil {
		t.Fatalf("钉钉发送失败: %v", err)
	}
	if tokenRequests != 1 {
		t.Fatalf("钉钉 token 请求次数不正确: %d", tokenRequests)
	}
	if messagePayload["robotCode"] != "robot-code" || messagePayload["openConversationId"] != "cid-group-1" {
		t.Fatalf("钉钉消息路由不正确: %+v", messagePayload)
	}
	if messagePayload["msgKey"] != "sampleText" {
		t.Fatalf("钉钉消息类型不正确: %+v", messagePayload)
	}
	var msgParam map[string]string
	if err := json.Unmarshal([]byte(messagePayload["msgParam"]), &msgParam); err != nil {
		t.Fatalf("解析钉钉 msgParam 失败: %v", err)
	}
	if msgParam["content"] != "今日新闻摘要" {
		t.Fatalf("钉钉消息正文不正确: %+v", msgParam)
	}
}

func TestDingTalkChannelAccessTokenRefreshUsesSingleflight(t *testing.T) {
	var callers int32
	var tokenRequests int32
	releaseTokenResponse := make(chan struct{})
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/v1.0/oauth2/accessToken" {
			return nil, fmt.Errorf("未知钉钉请求路径: %s", request.URL.Path)
		}
		atomic.AddInt32(&tokenRequests, 1)
		<-releaseTokenResponse
		return jsonResponse(`{"accessToken":"ding-token","expireIn":7200}`), nil
	})}
	channel := newDingTalkChannel("ding-client", "ding-secret", "robot-code", client)
	channel.baseURL = "https://dingtalk.test"

	const concurrency = 12
	start := make(chan struct{})
	errs := make(chan error, concurrency)
	var wg sync.WaitGroup
	for range concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			atomic.AddInt32(&callers, 1)
			token, err := channel.accessTokenForDelivery(context.Background())
			if err != nil {
				errs <- err
				return
			}
			if token != "ding-token" {
				errs <- fmt.Errorf("钉钉 token 不正确: %q", token)
			}
		}()
	}

	close(start)
	deadline := time.After(time.Second)
	for atomic.LoadInt32(&callers) < concurrency {
		select {
		case <-deadline:
			close(releaseTokenResponse)
			t.Fatalf("等待并发 token 请求进入调用路径超时，实际: %d", atomic.LoadInt32(&callers))
		default:
			time.Sleep(time.Millisecond)
		}
	}
	close(releaseTokenResponse)
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("钉钉 token 刷新失败: %v", err)
		}
	}
	if got := atomic.LoadInt32(&tokenRequests); got != 1 {
		t.Fatalf("并发刷新应只发起 1 次 token 请求，实际: %d", got)
	}
}

func TestDingTalkStreamMessageAcknowledgesWhenWebhookReportsIngressFailure(t *testing.T) {
	var webhookRequests int32
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.String() != "https://dingtalk.test/session-webhook" {
			return nil, fmt.Errorf("未知钉钉请求地址: %s", request.URL.String())
		}
		atomic.AddInt32(&webhookRequests, 1)
		var payload struct {
			MsgType string `json:"msgtype"`
			Text    struct {
				Content string `json:"content"`
			} `json:"text"`
		}
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			return nil, fmt.Errorf("解析钉钉 webhook 请求失败: %w", err)
		}
		if payload.MsgType != "text" || !strings.Contains(payload.Text.Content, "DingTalk 消息处理失败") {
			return nil, fmt.Errorf("钉钉 webhook 错误提示不正确: %+v", payload)
		}
		return jsonResponse(`{}`), nil
	})}
	channel := newDingTalkChannel("ding-client", "ding-secret", "", client)
	ingress := &recordingIngressAcceptor{err: errors.New("dm temporarily unavailable")}
	channel.SetIngress(ingress)

	response, err := channel.handleStreamMessage(context.Background(), &dingchatbot.BotCallbackDataModel{
		ConversationId:    "cid-group-1",
		ConversationType:  "2",
		ConversationTitle: "日报群",
		ChatbotCorpId:     "corp-1",
		MsgId:             "ding-message-1",
		SenderStaffId:     "staff-1",
		SenderNick:        "Alice",
		SessionWebhook:    "https://dingtalk.test/session-webhook",
		Text: dingchatbot.BotCallbackDataTextModel{
			Content: "检查今天日报",
		},
	})
	if err != nil {
		t.Fatalf("已通过 webhook 通知用户时不应向钉钉返回错误: %v", err)
	}
	if response == nil || string(response) != "" {
		t.Fatalf("钉钉 stream 应返回空 ACK: %q", string(response))
	}
	if len(ingress.requests) != 1 {
		t.Fatalf("钉钉 Stream 消息应先进入 ingress: %+v", ingress.requests)
	}
	if got := atomic.LoadInt32(&webhookRequests); got != 1 {
		t.Fatalf("钉钉错误 webhook 请求次数不正确: %d", got)
	}
}

func TestDingTalkStreamMessageRemembersSessionWebhookDelivery(t *testing.T) {
	channel := newDingTalkChannel("ding-client", "ding-secret", "", nil)
	ingress := &recordingIngressAcceptor{}
	channel.SetIngress(ingress)

	if _, err := channel.handleStreamMessage(context.Background(), &dingchatbot.BotCallbackDataModel{
		ConversationId:    "cid-group-1",
		ConversationType:  "2",
		ConversationTitle: "日报群",
		ChatbotCorpId:     "corp-1",
		MsgId:             "ding-message-1",
		SenderStaffId:     "staff-1",
		SenderNick:        "Alice",
		SessionWebhook:    "https://dingtalk.test/session-webhook",
		Text: dingchatbot.BotCallbackDataTextModel{
			Content: "检查今天日报",
		},
	}); err != nil {
		t.Fatalf("钉钉 Stream 消息处理失败: %v", err)
	}

	if len(ingress.requests) != 1 {
		t.Fatalf("钉钉 Stream 消息未进入 ingress: %+v", ingress.requests)
	}
	accepted := ingress.requests[0]
	if accepted.Ref != "cid-group-1" || accepted.ChatType != "group" || accepted.Content != "检查今天日报" {
		t.Fatalf("钉钉 Stream ingress 请求不正确: %+v", accepted)
	}
	if accepted.Delivery == nil ||
		accepted.Delivery.Channel != ChannelTypeDingTalk ||
		accepted.Delivery.To != "https://dingtalk.test/session-webhook" ||
		accepted.Delivery.AccountID != "corp-1" {
		t.Fatalf("钉钉 Stream 回投目标应使用 sessionWebhook: %+v", accepted.Delivery)
	}
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
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
