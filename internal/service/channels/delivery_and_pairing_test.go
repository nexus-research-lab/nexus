// delivery_and_pairing_test.go 覆盖渠道路由投递、IM 回复与配对生命周期测试。
package channels

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	channelcontract "github.com/nexus-research-lab/nexus/internal/service/channels/contract"
	management "github.com/nexus-research-lab/nexus/internal/service/channels/management"
	channelmessage "github.com/nexus-research-lab/nexus/internal/service/channels/message"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	roomrealtime "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
	"github.com/nexus-research-lab/nexus/internal/storage/agentrepo"
	"github.com/nexus-research-lab/nexus/internal/storage/imdelivery"
	"github.com/nexus-research-lab/nexus/internal/storage/roomrepo"
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
	stopErr     error

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
	return c.stopErr
}

func (c *recordingDeliveryChannel) SendDeliveryMessage(_ context.Context, target DeliveryTarget, text string) (DeliveryResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.targets = append(c.targets, target)
	c.texts = append(c.texts, text)
	return channelcontract.NewDeliveryResult(target, nil), nil
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
	return channelcontract.NewDeliveryResult(target, c.receipt), nil
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

func newChannelTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_")))
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	schema := `
	CREATE TABLE channel_control_versions (
	    owner_user_id VARCHAR(64) NOT NULL PRIMARY KEY,
	    version INTEGER NOT NULL DEFAULT 1,
	    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL
	);
	CREATE TABLE automation_delivery_routes (
	    route_id VARCHAR(64) NOT NULL PRIMARY KEY,
	    agent_id VARCHAR(64) NOT NULL,
	    session_key VARCHAR(512) NOT NULL DEFAULT '',
	    target_session_key VARCHAR(512) NOT NULL DEFAULT '',
	    mode VARCHAR(32) NOT NULL,
	    channel VARCHAR(64),
	    "to" VARCHAR(255),
	    account_id VARCHAR(64),
	    thread_id VARCHAR(255),
	    context_token TEXT,
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
	    sync_cursor TEXT NOT NULL DEFAULT '',
	    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
	    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
	    PRIMARY KEY (owner_user_id, channel_type, account_id)
	);
	CREATE TABLE im_pairings (
 target_room_id TEXT NOT NULL DEFAULT '',
 target_conversation_id TEXT NOT NULL DEFAULT '',
 binding_version BIGINT NOT NULL DEFAULT 1,
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
	    session_key VARCHAR(512) NOT NULL DEFAULT '',
	    session_materialized BOOLEAN NOT NULL DEFAULT FALSE,
	    last_message_at DATETIME,
	    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
	    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
		    UNIQUE (owner_user_id, channel_type, account_id, chat_type, external_ref, thread_id)
	);`
	if _, err = db.Exec(schema); err != nil {
		t.Fatalf("初始化 delivery schema 失败: %v", err)
	}
	if _, err = db.Exec(`CREATE TABLE im_pairing_sessions (
    owner_user_id TEXT NOT NULL,
    pairing_id TEXT NOT NULL,
    channel_type TEXT NOT NULL,
    account_id TEXT NOT NULL DEFAULT '',
    chat_type TEXT NOT NULL,
    external_ref TEXT NOT NULL,
    thread_id TEXT NOT NULL DEFAULT '',
    session_key TEXT NOT NULL,
    session_materialized BOOLEAN NOT NULL DEFAULT FALSE,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    PRIMARY KEY (owner_user_id, channel_type, account_id, chat_type, external_ref, thread_id),
    UNIQUE (owner_user_id, session_key)
)`); err != nil {
		t.Fatalf("初始化 pairing session schema 失败: %v", err)
	}
	if _, err = db.Exec(`CREATE TABLE im_ingress_messages(owner_user_id TEXT NOT NULL,channel_type TEXT NOT NULL,account_id TEXT NOT NULL DEFAULT '',req_id TEXT NOT NULL,agent_id TEXT NOT NULL,session_key TEXT NOT NULL,round_id TEXT NOT NULL,dispatch_phase TEXT NOT NULL DEFAULT 'dispatching',payload_hash TEXT NOT NULL DEFAULT '',binding_version BIGINT NOT NULL DEFAULT 0,status TEXT NOT NULL,error_message TEXT,created_at DATETIME DEFAULT CURRENT_TIMESTAMP,updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,completed_at DATETIME,PRIMARY KEY(owner_user_id,channel_type,account_id,req_id))`); err != nil {
		t.Fatal(err)
	}
	migration, readErr := os.ReadFile("../../../db/migrations/sqlite/00141_im_delivery_replies.sql")
	if readErr != nil {
		t.Fatal(readErr)
	}
	if _, execErr := db.Exec(strings.Split(string(migration), "-- +goose Down")[0]); execErr != nil {
		t.Fatal(execErr)
	}
	return db
}

func newChannelOwnerWorkspace(t *testing.T, ownerUserID string, agentID string) (string, string) {
	t.Helper()
	stateRoot := filepath.Join(t.TempDir(), ".nexus")
	t.Setenv(appfs.NexusStateRootEnvName, stateRoot)
	t.Setenv("NEXUS_CONFIG_DIR", "")
	return filepath.Join(stateRoot, "users"), filepath.Join(
		appfs.UserWorkspaceRootAt(stateRoot, ownerUserID),
		agentID,
	)
}

func TestRouterDoesNotReuseLegacyExternalAgentRouteWithoutSessionKey(t *testing.T) {
	db := newChannelTestDB(t)
	router := NewRouter(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)

	if _, err := router.RememberRoute(context.Background(), "agent-a", DeliveryTarget{
		Mode:    DeliveryModeExplicit,
		Channel: ChannelTypeFeishu,
		To:      "oc-legacy",
	}); err != nil {
		t.Fatalf("记录 legacy 外部 route 失败: %v", err)
	}
	last, err := router.GetLastRoute(context.Background(), "agent-a")
	if err != nil {
		t.Fatalf("读取 legacy 外部 route 失败: %v", err)
	}
	if last != nil {
		t.Fatalf("缺少 exact Session key 的外部 route 必须 fail closed: %+v", last)
	}
}

func TestRouterFailsClosedWhenIMGrantValidatorIsMissing(t *testing.T) {
	db := newChannelTestDB(t)
	router := NewRouter(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	// Wiring the durable IM delivery store opts this Router into the full
	// lifecycle contract. A missing pairing validator must not silently turn a
	// structured external Session into a sendable target.
	router.SetIMDeliverySupport(imdelivery.NewRepository(config.Config{DatabaseDriver: "sqlite"}, db), nil)
	sessionKey := protocol.BuildAgentAccountSessionKey(
		"agent-a", protocol.SessionChannelFeishu, protocol.RoomTypeDM, "account-a", "ou-a", "",
	)
	_, err := router.RememberRoute(context.Background(), "agent-a", DeliveryTarget{
		Mode:       DeliveryModeExplicit,
		Channel:    ChannelTypeFeishu,
		To:         "ou-a",
		SessionKey: sessionKey,
	})
	if !errors.Is(err, ErrExternalSessionGrantUnavailable) {
		t.Fatalf("缺少 IM grant validator 时必须 fail closed: %v", err)
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

func TestRouterDeliverMessageRejectsMissingOrdinaryInternalSession(t *testing.T) {
	workspaceRoot, workspacePath := newChannelOwnerWorkspace(t, authctx.SystemUserID, "agent-1")
	router := NewRouter(
		config.Config{DatabaseDriver: "sqlite", WorkspacePath: workspaceRoot},
		newChannelTestDB(t),
		&stubAgentResolver{agentByID: map[string]*protocol.Agent{
			"agent-1": {
				AgentID: "agent-1", OwnerUserID: authctx.SystemUserID,
				WorkspacePath: workspacePath,
			},
		}},
		nil,
	)
	if err := router.Start(context.Background()); err != nil {
		t.Fatalf("启动 router 失败: %v", err)
	}
	defer router.Stop(context.Background())

	sessionKey := protocol.BuildAgentSessionKey(
		"agent-1", protocol.SessionChannelInternalSegment, protocol.RoomTypeDM,
		"missing-real-session", "",
	)
	_, err := router.DeliverMessage(context.Background(), "agent-1", "不能创建隐藏会话", DeliveryTarget{
		Mode: DeliveryModeExplicit, Channel: ChannelTypeInternal, To: sessionKey,
	})
	if err == nil || !strings.Contains(err.Error(), "delivery target session is not available") {
		t.Fatalf("missing ordinary internal session must fail closed, got %v", err)
	}
	stored, _, findErr := workspacestore.NewSessionFileStore(workspaceRoot).
		ForOwner(authctx.SystemUserID).
		FindSession([]string{workspacePath}, sessionKey)
	if findErr != nil || stored != nil {
		t.Fatalf("missing session must not be synthesized: session=%+v err=%v", stored, findErr)
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

type databaseBackedDeliverySessionResolver struct {
	session protocol.Session
}

func (r databaseBackedDeliverySessionResolver) ResolveDeliverySession(
	_ context.Context,
	sessionKey string,
) (*protocol.Session, error) {
	if strings.TrimSpace(sessionKey) != strings.TrimSpace(r.session.SessionKey) {
		return nil, nil
	}
	result := r.session
	return &result, nil
}

type imTestSessions map[string]protocol.Session

func (s imTestSessions) ResolveDeliverySession(_ context.Context, key string) (*protocol.Session, error) {
	v, ok := s[key]
	if !ok {
		return nil, nil
	}
	return &v, nil
}

func TestTrackedIMDeliveryKeepsExactOriginAndNeverReplaysUnknown(t *testing.T) {
	root, workspace := newChannelOwnerWorkspace(t, authctx.SystemUserID, "agent-a")
	db := newChannelTestDB(t)
	defer db.Close()
	cfg := config.Config{DatabaseDriver: "sqlite", WorkspacePath: root}
	agents := &stubAgentResolver{agentByID: map[string]*protocol.Agent{"agent-a": {AgentID: "agent-a", OwnerUserID: authctx.SystemUserID, WorkspacePath: workspace}}}
	router := NewRouter(cfg, db, agents, nil)
	external := &recordingDeliveryChannel{channelType: ChannelTypeWeixinPersonal}
	router.RegisterForOwner(authctx.SystemUserID, external)
	if err := router.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer router.Stop(context.Background())
	control := NewControlService(cfg, db, agents, router)
	paired, err := control.CreatePairing(context.Background(), authctx.SystemUserID, CreatePairingRequest{ChannelType: ChannelTypeWeixinPersonal, AccountID: "account", ChatType: "dm", ExternalRef: "person", AgentID: "agent-a", Status: PairingStatusActive})
	if err != nil {
		t.Fatal(err)
	}
	sourceA := "agent:agent-a:ws:dm:task-a"
	sourceB := "agent:agent-a:ws:dm:task-b"
	target := paired.SessionKey
	now := time.Now().UTC()
	sessions := imTestSessions{}
	for _, key := range []string{sourceA, sourceB, target} {
		sessions[key] = protocol.Session{SessionKey: key, AgentID: "agent-a", CreatedAt: now, ChatType: "dm", Title: key}
	}
	router.SetSessionProjectionResolver(sessions)
	store := imdelivery.NewRepository(cfg, db)
	router.SetIMDeliverySupport(store, control)
	ctx := ingressTestOwnerContext(authctx.SystemUserID)
	if _, err = router.RememberSessionRoute(ctx, "agent-a", target, DeliveryTarget{Mode: DeliveryModeExplicit, Channel: ChannelTypeWeixinPersonal, AccountID: "account", To: "person", SessionKey: target}); err != nil {
		t.Fatal(err)
	}
	send := func(source, call string) (DeliveryResult, error) {
		return control.SendAgentExternalSessionMessage(WithIMDeliverySource(ctx, imdelivery.Source{Kind: "tool", AgentID: "agent-a", SessionKey: source, RoundID: "round", CallID: call}), authctx.SystemUserID, "agent-a", target, "相同的草案")
	}
	first, err := send(sourceA, "call")
	if err != nil {
		t.Fatal(err)
	}
	retry, err := send(sourceA, "call")
	if err != nil || retry.DeliveryID != first.DeliveryID || external.sentCount() != 1 {
		t.Fatalf("duplicate physical send %+v %v count=%d", retry, err, external.sentCount())
	}
	second, err := send(sourceB, "call")
	if err != nil || second.DeliveryID == first.DeliveryID {
		t.Fatalf("different sessions collapsed %v", err)
	}
	rows, err := store.List(ctx, authctx.SystemUserID, target, "草案", 0, 10)
	if err != nil || len(rows) != 2 {
		t.Fatalf("query %v %v", rows, err)
	}
	pending, err := router.stageIMDelivery(ctx, imdelivery.Source{Kind: "tool", AgentID: "agent-a", SessionKey: sourceA, RoundID: "round", CallID: "uncertain"}, "agent-a", target, "相同的草案")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.ClaimSend(ctx, authctx.SystemUserID, pending.ID, false); err != nil {
		t.Fatal(err)
	}
	// 请求取消后仍记录已确认分段，重试只返回回执而不重发。
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	progress := channelcontract.NewDeliveryResult(DeliveryTarget{Mode: DeliveryModeLast, SessionKey: target}, channelmessage.NewReceipt(channelmessage.ReceiptParams{Channel: ChannelTypeWeixinPersonal, Target: "person", Parts: []channelmessage.ReceiptPart{channelmessage.TextPart("part-1")}}))
	if err = channelcontract.RecordDeliveryProgress(router.withIMDeliveryProgress(cancelled, pending), progress); err != nil {
		t.Fatal(err)
	}
	uncertain, retryErr := send(sourceA, "uncertain")
	if retryErr == nil || uncertain.Receipt == nil || uncertain.Receipt.PrimaryPlatformMessageID != "part-1" {
		t.Fatalf("未知结果未保留已确认分段: %+v %v", uncertain, retryErr)
	}
	if _, err = send(sourceA, "uncertain"); err == nil || external.sentCount() != 2 {
		t.Fatal("unknown delivery replayed")
	}

	// The evidence comes from ingress, not transcript text or an arbitrary round.
	if _, err = db.Exec(`INSERT INTO im_ingress_messages(owner_user_id,channel_type,account_id,req_id,agent_id,session_key,round_id,status) VALUES (?,?,?,?,?,?,?,?)`, authctx.SystemUserID, ChannelTypeWeixinPersonal, "account", "human-message", "agent-a", target, "human-round", "processing"); err != nil {
		t.Fatal(err)
	}
	request := normalizedIngressRequest{ownerUserID: authctx.SystemUserID, agentID: "agent-a", sessionKey: target, parsed: protocol.ParseSessionKey(target), roundID: "human-round", reqID: "human-message", content: "确认第一份"}
	if err = control.recordDeliveryInput(ctx, request); err != nil {
		t.Fatal(err)
	}
	if _, err = control.IMDeliveryInput(ctx, authctx.SystemUserID, "agent-a", target, "human-round", "确认第一份"); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []struct{ round, content string }{{"other-round", "确认第一份"}, {"human-round", "伪造批准"}} {
		if _, err = control.IMDeliveryInput(ctx, authctx.SystemUserID, "agent-a", target, bad.round, bad.content); err == nil {
			t.Fatal("forged human input accepted")
		}
	}
	if _, err = db.Exec(`UPDATE im_ingress_messages SET status='failed' WHERE req_id='human-message'`); err != nil {
		t.Fatal(err)
	}
	if _, err = control.IMDeliveryInput(ctx, authctx.SystemUserID, "agent-a", target, "human-round", "确认第一份"); err == nil {
		t.Fatal("failed input accepted")
	}

	// 历史 failed 也没有证明副作用未发生，不能重领并换掉人类输入证据。
	claimed, duplicate, err := control.claimIngressMessage(ctx, ingressMessageClaimInput{OwnerUserID: authctx.SystemUserID, Channel: ChannelTypeWeixinPersonal, AccountID: "account", ReqID: "human-message", AgentID: "agent-a", SessionKey: target, RoundID: "retry-round"})
	if claimed || !errors.Is(err, ErrIngressOutcomeUnknown) || duplicate == nil || duplicate.RoundID != "human-round" {
		t.Fatalf("legacy failed claim changed evidence: %v %+v %v", claimed, duplicate, err)
	}
	if _, err = control.IMDeliveryInput(ctx, authctx.SystemUserID, "agent-a", target, "retry-round", "确认第一份"); err == nil {
		t.Fatal("unknown input became a new approval")
	}
	status := PairingStatusDisabled
	if _, err = control.UpdatePairing(ctx, authctx.SystemUserID, paired.PairingID, UpdatePairingRequest{Status: &status}); err != nil {
		t.Fatal(err)
	}
	status = PairingStatusActive
	if _, err = control.UpdatePairing(ctx, authctx.SystemUserID, paired.PairingID, UpdatePairingRequest{Status: &status}); err != nil {
		t.Fatal(err)
	}
	old, err := store.GetDelivery(ctx, authctx.SystemUserID, first.DeliveryID)
	if err != nil || !old.ReturnRevoked || old.State != "sent" {
		t.Fatalf("pairing revival restored old return address or changed send fact %+v %v", old, err)
	}
	if _, err = send(sourceA, "call"); !errors.Is(err, imdelivery.ErrConflict) && err == nil {
		t.Fatal("revoked invocation unexpectedly succeeded")
	}
}

func TestValidateExternalSessionGrantRejectsRotatedSessionKey(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()
	router := NewRouter(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	deleted := map[string]bool{}
	router.SetSessionProjectionResolver(deletedSessionResolver{deleted: deleted})
	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, router)
	created, err := service.CreatePairing(context.Background(), "owner-a", CreatePairingRequest{
		ChannelType: ChannelTypeWeChat,
		ChatType:    protocol.RoomTypeDM,
		ExternalRef: "wx-user-a",
		AgentID:     "agent-a",
		Status:      PairingStatusActive,
	})
	if err != nil {
		t.Fatalf("创建配对失败: %v", err)
	}
	if _, err = db.Exec("UPDATE im_pairings SET session_materialized = 1 WHERE pairing_id = ?", created.PairingID); err != nil {
		t.Fatalf("准备已物化 Session 失败: %v", err)
	}
	oldKey := created.SessionKey
	deleted[oldKey] = true
	_, rotatedKey, err := service.ResolveIngressSession(context.Background(), IngressRequest{
		OwnerUserID: "owner-a",
		Channel:     ChannelTypeWeChat,
		ChatType:    protocol.RoomTypeDM,
		Ref:         "wx-user-a",
	})
	if err != nil || rotatedKey == oldKey || protocol.ParseSessionKey(rotatedKey).Generation == "" {
		t.Fatalf("删除后应轮换 Session: old=%q new=%q err=%v", oldKey, rotatedKey, err)
	}
	if err = service.ValidateExternalSessionGrant(context.Background(), "owner-a", "agent-a", oldKey); err == nil {
		t.Fatal("已轮换的旧 Session key 不得通过 IM grant 校验")
	}
	if err = service.ValidateExternalSessionGrant(context.Background(), "owner-a", "agent-a", rotatedKey); err != nil {
		t.Fatalf("当前 Session key 应保持可授权: %v", err)
	}
}

func TestSendAgentExternalSessionMessageRevalidatesAndProjects(t *testing.T) {
	workspaceRoot, workspacePath := newChannelOwnerWorkspace(t, authctx.SystemUserID, "agent-a")
	db := newChannelTestDB(t)
	defer db.Close()
	agents := &stubAgentResolver{agentByID: map[string]*protocol.Agent{
		"agent-a": {
			AgentID: "agent-a", OwnerUserID: authctx.SystemUserID, WorkspacePath: workspacePath,
		},
	}}
	router := NewRouter(config.Config{
		DatabaseDriver: "sqlite", WorkspacePath: workspaceRoot,
	}, db, agents, nil)
	external := &recordingDeliveryChannel{channelType: ChannelTypeWeixinPersonal}
	router.RegisterForOwner(authctx.SystemUserID, external)
	if err := router.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer router.Stop(context.Background())
	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, agents, router)
	router.SetIMDeliverySupport(imdelivery.NewRepository(config.Config{DatabaseDriver: "sqlite"}, db), service)
	pairing, err := service.CreatePairing(context.Background(), authctx.SystemUserID, CreatePairingRequest{
		ChannelType: ChannelTypeWeixinPersonal,
		AccountID:   "weixin-account",
		ChatType:    protocol.RoomTypeDM,
		ExternalRef: "weixin-user",
		AgentID:     "agent-a",
		Status:      PairingStatusActive,
	})
	if err != nil {
		t.Fatal(err)
	}
	sessionKey := pairing.SessionKey
	now := time.Now().UTC()
	router.SetSessionProjectionResolver(databaseBackedDeliverySessionResolver{session: protocol.Session{
		SessionKey: sessionKey, AgentID: "agent-a",
		ChannelType: ChannelTypeWeixinPersonal, ChatType: protocol.RoomTypeDM,
		Status: "closed", CreatedAt: now, LastActivity: now, Title: "微信私聊",
	}})
	if _, err = router.RememberSessionRoute(context.Background(), "agent-a", sessionKey, DeliveryTarget{
		Mode: DeliveryModeExplicit, Channel: ChannelTypeWeixinPersonal,
		To: "weixin-user", AccountID: "weixin-account", SessionKey: sessionKey,
	}); err != nil {
		t.Fatal(err)
	}

	result, err := service.SendAgentExternalSessionMessage(
		context.Background(), authctx.SystemUserID, "agent-a", sessionKey, "主动提醒",
	)
	if err != nil {
		t.Fatalf("主动外部私聊投递失败: %v", err)
	}
	if result.Target.Channel != ChannelTypeWeixinPersonal || external.sentCount() != 1 {
		t.Fatalf("外部通道投递不正确: result=%+v sent=%d", result, external.sentCount())
	}
	stored, _, err := workspacestore.NewSessionFileStore(workspaceRoot).
		ForOwner(authctx.SystemUserID).
		FindSession([]string{workspacePath}, sessionKey)
	if err != nil || stored == nil {
		t.Fatalf("目标 Session 未物化: session=%+v err=%v", stored, err)
	}
	messages, err := workspacestore.NewAgentHistoryStore(workspaceRoot).
		ForOwner(authctx.SystemUserID).
		ReadMessages(workspacePath, *stored, nil)
	if err != nil || len(messages) != 1 || extractAssistantText(messages[0]) != "主动提醒" {
		t.Fatalf("目标 Session 投影不正确: messages=%+v err=%v", messages, err)
	}
	status := PairingStatusDisabled
	if _, err = service.UpdatePairing(context.Background(), authctx.SystemUserID, pairing.PairingID, UpdatePairingRequest{
		Status: &status,
	}); err != nil {
		t.Fatal(err)
	}
	_, err = service.SendAgentExternalSessionMessage(
		context.Background(), authctx.SystemUserID, "agent-a", sessionKey, "不应发出",
	)
	if !errors.Is(err, ErrExternalSessionGrantUnavailable) ||
		!strings.Contains(err.Error(), "pairing is not active") || external.sentCount() != 1 {
		t.Fatalf("撤权后必须 fail closed: err=%v sent=%d", err, external.sentCount())
	}
	_, err = router.DeliverMessage(ingressTestOwnerContext(authctx.SystemUserID), "agent-a", "旧 round 回复不应发出", DeliveryTarget{
		Mode: DeliveryModeExplicit, Channel: ChannelTypeWeixinPersonal, To: "weixin-user",
		AccountID: "weixin-account", SessionKey: sessionKey,
	})
	if !errors.Is(err, ErrExternalSessionGrantUnavailable) || external.sentCount() != 1 {
		t.Fatalf("普通 DM 回复在 pairing 撤权后也必须 fail closed: err=%v sent=%d", err, external.sentCount())
	}
	if err = router.SetTyping(ingressTestOwnerContext(authctx.SystemUserID), "agent-a", DeliveryTarget{
		Mode: DeliveryModeExplicit, Channel: ChannelTypeWeixinPersonal, To: "weixin-user",
		AccountID: "weixin-account", SessionKey: sessionKey,
	}, true); !errors.Is(err, ErrExternalSessionGrantUnavailable) {
		t.Fatalf("typing 在 pairing 撤权后必须 fail closed: %v", err)
	}
}

func TestListAgentExternalSessionsReturnsOnlyActivePairedDMs(t *testing.T) {
	db := newChannelTestDB(t)
	defer db.Close()
	router := NewRouter(config.Config{DatabaseDriver: "sqlite"}, db, nil, nil)
	service := NewControlService(config.Config{DatabaseDriver: "sqlite"}, db, nil, router)
	activeDM, err := service.CreatePairing(context.Background(), "owner-a", CreatePairingRequest{
		ChannelType:  ChannelTypeWeixinPersonal,
		AccountID:    "weixin-account",
		ChatType:     protocol.RoomTypeDM,
		ExternalRef:  "weixin-user",
		ExternalName: "捷哥",
		AgentID:      "agent-a",
		Status:       PairingStatusActive,
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, request := range []CreatePairingRequest{
		{ChannelType: ChannelTypeWeixinPersonal, ChatType: protocol.RoomTypeGroup, ExternalRef: "group", AgentID: "agent-a", Status: PairingStatusActive},
		{ChannelType: ChannelTypeWeixinPersonal, ChatType: protocol.RoomTypeDM, ExternalRef: "other-agent", AgentID: "agent-b", Status: PairingStatusActive},
		{ChannelType: ChannelTypeWeixinPersonal, ChatType: protocol.RoomTypeDM, ExternalRef: "disabled", AgentID: "agent-a", Status: PairingStatusDisabled},
	} {
		if _, err = service.CreatePairing(context.Background(), "owner-a", request); err != nil {
			t.Fatal(err)
		}
	}
	router.SetSessionProjectionResolver(databaseBackedDeliverySessionResolver{session: protocol.Session{
		SessionKey: activeDM.SessionKey,
		AgentID:    "agent-a",
		Title:      "真实微信私聊",
	}})
	items, err := service.ListAgentExternalSessions(
		context.Background(), "owner-a", "agent-a", ChannelTypeWeixinPersonal,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].SessionKey != activeDM.SessionKey || items[0].Label != "捷哥" {
		t.Fatalf("Agent delivery sessions = %+v", items)
	}
	if _, err = service.ListAgentExternalSessions(
		context.Background(), "owner-a", "agent-a", ChannelTypeWebSocket,
	); !errors.Is(err, ErrExternalSessionGrantUnavailable) {
		t.Fatalf("non-IM delivery directory error = %v", err)
	}
}

func TestPairingRoomTargetSwitchKeepsSessionAndFencesOldDelivery(t *testing.T) {
	cfg := newIngressTestConfig(t)
	db := migrateIngressSQLite(t, cfg.DatabaseURL)
	defer db.Close()
	ctx := contextWithIngressOwner(context.Background(), "owner-room-binding")
	agents := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	agent, err := agents.CreateAgent(ctx, protocol.CreateRequest{Name: "负责人"})
	if err != nil {
		t.Fatal(err)
	}
	rooms := roomsvc.NewService(cfg, agents, roomrepo.NewSQLRepository("sqlite", db))
	room, err := rooms.CreateRoom(ctx, protocol.CreateRoomRequest{AgentIDs: []string{agent.AgentID}, Name: "项目", PrivateMessagesEnabled: true})
	if err != nil {
		t.Fatal(err)
	}
	control := NewControlService(cfg, db, agents, nil)
	control.SetRoomService(rooms)
	pairing, err := control.CreatePairing(ctx, authctx.OwnerUserID(ctx), CreatePairingRequest{ChannelType: ChannelTypeFeishu, ChatType: "dm", ExternalRef: "person", AgentID: agent.AgentID})
	if err != nil {
		t.Fatal(err)
	}
	original := pairing.SessionKey
	target := DeliveryTarget{Mode: DeliveryModeExplicit, Channel: ChannelTypeFeishu, To: "person", SessionKey: original, PairingID: pairing.PairingID, BindingVersion: pairing.BindingVersion}
	binding := management.PairingSessionTarget{RoomID: room.Room.ID, ConversationID: room.Conversation.ID}
	next, err := control.UpdatePairing(ctx, authctx.OwnerUserID(ctx), pairing.PairingID, UpdatePairingRequest{SessionTarget: &binding, BindingVersion: &pairing.BindingVersion})
	if err != nil {
		t.Fatal(err)
	}
	if next.SessionKey != original || next.BindingVersion != pairing.BindingVersion+1 || next.SessionTarget.RoomName != "项目" {
		t.Fatalf("切换不应新建传输会话: %+v", next)
	}
	if _, err := control.ValidateBindingDelivery(ctx, authctx.OwnerUserID(ctx), agent.AgentID, target); !errors.Is(err, ErrExternalSessionGrantUnavailable) {
		t.Fatalf("旧回复必须失效: %v", err)
	}
	target.BindingVersion = next.BindingVersion
	if bound, err := control.ValidateBindingDelivery(ctx, authctx.OwnerUserID(ctx), agent.AgentID, target); err != nil || !bound {
		t.Fatalf("新 Room 目标未生效: %v %v", bound, err)
	}
	if _, err := control.UpdatePairing(ctx, authctx.OwnerUserID(ctx), pairing.PairingID, UpdatePairingRequest{SessionTarget: &binding, BindingVersion: &pairing.BindingVersion}); err == nil {
		t.Fatal("旧版本更新必须拒绝")
	}
	independent := management.PairingSessionTarget{}
	restored, err := control.UpdatePairing(ctx, authctx.OwnerUserID(ctx), pairing.PairingID, UpdatePairingRequest{SessionTarget: &independent, BindingVersion: &next.BindingVersion})
	if err != nil {
		t.Fatal(err)
	}
	if restored.SessionKey != original || restored.SessionTarget.RoomID != "" {
		t.Fatalf("独立会话历史身份丢失: %+v", restored)
	}
	if _, err := control.ValidateBindingDelivery(ctx, authctx.OwnerUserID(ctx), agent.AgentID, target); !errors.Is(err, ErrExternalSessionGrantUnavailable) {
		t.Fatalf("Room 旧轮次不能再回信: %v", err)
	}
	bad := binding
	bad.RoomID = "another-room"
	if _, err := control.UpdatePairing(ctx, authctx.OwnerUserID(ctx), pairing.PairingID, UpdatePairingRequest{SessionTarget: &bad, BindingVersion: &restored.BindingVersion}); err == nil {
		t.Fatal("错误 Room / conversation 组合必须拒绝")
	}
	otherCtx := contextWithIngressOwner(ctx, "other-owner")
	if err := control.validatePairingRoom(otherCtx, agent.AgentID, binding); err == nil {
		t.Fatal("跨 owner 目标必须拒绝")
	}
}

func TestPairingDMReusesSelectedConversationAndExternalReplyTarget(t *testing.T) {
	cfg := newIngressTestConfig(t)
	db := migrateIngressSQLite(t, cfg.DatabaseURL)
	defer db.Close()
	ctx := contextWithIngressOwner(t.Context(), "owner-dm")
	agents := agentsvc.NewService(cfg, agentrepo.NewSQLRepository("sqlite", db))
	agent, err := agents.CreateAgent(ctx, protocol.CreateRequest{Name: "Amy"})
	if err != nil {
		t.Fatal(err)
	}
	rooms := roomsvc.NewService(cfg, agents, roomrepo.NewSQLRepository("sqlite", db))
	dm, err := rooms.EnsureDirectRoom(ctx, agent.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	control := NewControlService(cfg, db, agents, nil)
	control.SetRoomService(rooms)
	pairing, err := control.CreatePairing(ctx, "owner-dm", CreatePairingRequest{ChannelType: ChannelTypeFeishu, ChatType: "dm", ExternalRef: "person", AgentID: agent.AgentID})
	if err != nil {
		t.Fatal(err)
	}
	target := management.PairingSessionTarget{RoomID: dm.Room.ID, ConversationID: dm.Conversation.ID}
	paired, err := control.UpdatePairing(ctx, "owner-dm", pairing.PairingID, UpdatePairingRequest{SessionTarget: &target, BindingVersion: &pairing.BindingVersion})
	if err != nil {
		t.Fatal(err)
	}
	handler := &fakeIngressDMHandler{}
	ingress := NewIngressService(cfg, agents, handler, nil)
	ingress.SetControlService(control)
	ingress.SetRoomRealtime(&roomrealtime.Service{})
	request, err := ingress.normalizeRequest(ctx, IngressRequest{Channel: ChannelTypeFeishu, Ref: "person", Content: "继续之前的工作", ReqID: "input", RoundID: "round"})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ingress.claimIngress(ctx, request); err != nil {
		t.Fatal(err)
	}
	if err := control.beginIngressDispatch(ctx, request); err != nil {
		t.Fatal(err)
	}
	if err := ingress.dispatchIngress(ctx, request); err != nil {
		t.Fatal(err)
	}
	expected := protocol.BuildRoomAgentSessionKey(dm.Conversation.ID, agent.AgentID, protocol.RoomTypeDM)
	if len(handler.requests) != 1 {
		t.Fatalf("requests=%v", handler.requests)
	}
	actual := handler.requests[0]
	if actual.SessionKey != expected || actual.ExternalReplyTarget == nil || actual.ExternalReplyTarget.SessionKey != paired.SessionKey || actual.ExternalReplyTarget.BindingVersion != paired.BindingVersion {
		t.Fatalf("wrong DM route: %+v", actual)
	}
	ingress.SetRoundIndexReader(func(_ context.Context, session string) (*protocol.SessionRoundIndex, error) {
		if session != expected {
			t.Fatalf("recovery read wrong session: %s", session)
		}
		return &protocol.SessionRoundIndex{Items: []protocol.SessionRoundIndexItem{{RoundID: "round", HasUserMessage: true}}}, nil
	})
	if ok, err := ingress.recoverIngress(ctx, request); err != nil || !ok {
		t.Fatalf("recovery: %v %v", ok, err)
	}
	if len(handler.requests) != 1 {
		t.Fatal("recovery reran DM")
	}
	other, err := agents.CreateAgent(ctx, protocol.CreateRequest{Name: "Other"})
	if err != nil {
		t.Fatal(err)
	}
	if err := control.validatePairingRoom(ctx, other.AgentID, target); err == nil {
		t.Fatal("accepted another agent's DM")
	}
	switched, err := control.UpdatePairing(ctx, "owner-dm", pairing.PairingID, UpdatePairingRequest{AgentID: &other.AgentID, BindingVersion: &paired.BindingVersion})
	if err != nil {
		t.Fatal(err)
	}
	if switched.SessionTarget.RoomID != "" {
		t.Fatal("agent switch retained old DM")
	}
}
