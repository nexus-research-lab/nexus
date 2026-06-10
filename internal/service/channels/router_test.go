package channels

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
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

func (c *recordingDeliveryChannel) SendDeliveryText(_ context.Context, target DeliveryTarget, text string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.targets = append(c.targets, target)
	c.texts = append(c.texts, text)
	return nil
}

func (c *recordingDeliveryChannel) sentCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.targets)
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

func TestRouterDeliverTextUsesOwnerScopedChannel(t *testing.T) {
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

	if _, err := router.DeliverText(context.Background(), "agent-a", "给 A", DeliveryTarget{
		Mode:    DeliveryModeExplicit,
		Channel: ChannelTypeTelegram,
		To:      "chat-a",
	}); err != nil {
		t.Fatalf("owner-a 投递失败: %v", err)
	}
	if channelA.sentCount() != 1 || channelB.sentCount() != 0 {
		t.Fatalf("owner-a 投递应只进入 A 通道，A=%d B=%d", channelA.sentCount(), channelB.sentCount())
	}

	if _, err := router.DeliverText(context.Background(), "agent-b", "给 B", DeliveryTarget{
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

	if _, err := router.DeliverText(context.Background(), "agent-a", "失败通道", DeliveryTarget{
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

func TestRouterDeliverTextUsesRememberedWebSocketRoute(t *testing.T) {
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
	target, err := router.DeliverText(context.Background(), "agent-1", "自动提醒", DeliveryTarget{Mode: DeliveryModeLast})
	if err != nil {
		t.Fatalf("DeliverText 失败: %v", err)
	}
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

func TestRouterDeliverTextPersistsSharedRoomDelivery(t *testing.T) {
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

	target, err := router.DeliverText(context.Background(), "agent-1", "今日新闻摘要", DeliveryTarget{
		Mode:    DeliveryModeExplicit,
		Channel: ChannelTypeWebSocket,
		To:      sessionKey,
	})
	if err != nil {
		t.Fatalf("Room 共享投递失败: %v", err)
	}
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

func TestRouterDeliverTextCreatesInternalAutomationInbox(t *testing.T) {
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
	target, err := router.DeliverText(context.Background(), "agent-1", "今日新闻摘要", DeliveryTarget{
		Mode:    DeliveryModeExplicit,
		Channel: ChannelTypeInternal,
		To:      sessionKey,
	})
	if err != nil {
		t.Fatalf("internal 投递失败: %v", err)
	}
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

func TestDiscordChannelSendDeliveryText(t *testing.T) {
	requests := make([]*http.Request, 0)
	channel := newDiscordChannel("token-1", &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requests = append(requests, request)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
				Header:     make(http.Header),
			}, nil
		}),
	})
	channel.baseURL = "https://discord.test/api/v10"

	text := strings.Repeat("a", 2400)
	if err := channel.SendDeliveryText(context.Background(), DeliveryTarget{
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
}

func TestTelegramChannelSendDeliveryText(t *testing.T) {
	requests := make([]*http.Request, 0)
	channel := newTelegramChannel("token-2", &http.Client{
		Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
			requests = append(requests, request)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(`{}`)),
				Header:     make(http.Header),
			}, nil
		}),
	})
	channel.baseURL = "https://telegram.test"

	if err := channel.SendDeliveryText(context.Background(), DeliveryTarget{
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
}

func TestFeishuChannelSendDeliveryText(t *testing.T) {
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
	if err := channel.SendDeliveryText(context.Background(), DeliveryTarget{
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

func TestDingTalkChannelSendDeliveryText(t *testing.T) {
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

	if err := channel.SendDeliveryText(context.Background(), DeliveryTarget{
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

func TestWeChatChannelSendDeliveryText(t *testing.T) {
	var tokenRequests int
	var messagePayload map[string]any
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/cgi-bin/gettoken":
			tokenRequests++
			if request.URL.Query().Get("corpid") != "ww_corp" || request.URL.Query().Get("corpsecret") != "corp-secret" {
				return nil, fmt.Errorf("企业微信 token 请求凭据不正确: %s", request.URL.RawQuery)
			}
			return jsonResponse(`{"errcode":0,"errmsg":"ok","access_token":"wechat-token","expires_in":7200}`), nil
		case "/cgi-bin/message/send":
			if request.URL.Query().Get("access_token") != "wechat-token" {
				return nil, fmt.Errorf("企业微信 access_token 不正确: %s", request.URL.RawQuery)
			}
			if err := json.NewDecoder(request.Body).Decode(&messagePayload); err != nil {
				return nil, fmt.Errorf("解析企业微信消息请求失败: %w", err)
			}
			return jsonResponse(`{"errcode":0,"errmsg":"ok"}`), nil
		default:
			return nil, fmt.Errorf("未知企业微信请求路径: %s", request.URL.Path)
		}
	})}

	channel := newWeChatChannel("ww_corp", "corp-secret", "100001", client)
	channel.baseURL = "https://wechat.test"

	if err := channel.SendDeliveryText(context.Background(), DeliveryTarget{
		Mode:      DeliveryModeExplicit,
		Channel:   ChannelTypeWeChat,
		To:        "zhangsan",
		AccountID: "touser",
	}, "今日新闻摘要"); err != nil {
		t.Fatalf("企业微信发送失败: %v", err)
	}
	if tokenRequests != 1 {
		t.Fatalf("企业微信 token 请求次数不正确: %d", tokenRequests)
	}
	if messagePayload["touser"] != "zhangsan" || messagePayload["msgtype"] != "text" {
		t.Fatalf("企业微信消息路由不正确: %+v", messagePayload)
	}
	if int(messagePayload["agentid"].(float64)) != 100001 {
		t.Fatalf("企业微信 agentid 不正确: %+v", messagePayload)
	}
	textPayload, ok := messagePayload["text"].(map[string]any)
	if !ok || textPayload["content"] != "今日新闻摘要" {
		t.Fatalf("企业微信消息正文不正确: %+v", messagePayload)
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
	CREATE TABLE im_pairings (
	    pairing_id VARCHAR(64) NOT NULL PRIMARY KEY,
	    owner_user_id VARCHAR(64) NOT NULL,
	    channel_type VARCHAR(32) NOT NULL,
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
	    UNIQUE (owner_user_id, channel_type, chat_type, external_ref, thread_id)
	);`
	if _, err = db.Exec(schema); err != nil {
		t.Fatalf("初始化 delivery schema 失败: %v", err)
	}
	return db
}
