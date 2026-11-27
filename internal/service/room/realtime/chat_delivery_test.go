// chat_delivery_test.go 覆盖 Room 黑盒测试夹具、消息投递、运行时交互与定向消息测试。
package realtime_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/app"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	authsvc "github.com/nexus-research-lab/nexus/internal/service/auth"
	"github.com/nexus-research-lab/nexus/internal/service/conversation/titlegen"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	realtimesvc "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
	subscriptionsvc "github.com/nexus-research-lab/nexus/internal/service/subscription"
	usagesvc "github.com/nexus-research-lab/nexus/internal/service/usage"
	goalstore "github.com/nexus-research-lab/nexus/internal/storage/goal"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
	_ "modernc.org/sqlite"
)

var NewServiceWithFactory = realtimesvc.NewServiceWithFactory

type noopSessionArtifactDeletionCoordinator struct{}

func (noopSessionArtifactDeletionCoordinator) DeleteSessionArtifacts(
	context.Context,
	string,
	string,
	string,
	string,
) error {
	return nil
}

func TestMain(m *testing.M) {
	os.Exit(handlertest.RunWithMinimalAppRoot(m))
}

func registerRealtimeServiceCleanup(
	t *testing.T,
	service *realtimesvc.Service,
	runtimeManager *runtimectx.Manager,
	sessionKey string,
) {
	t.Helper()
	stopSchedulers, err := service.StartDelayedWakeScheduler(context.Background())
	if err != nil {
		t.Fatalf("启动 Room 测试调度器失败: %v", err)
	}
	t.Cleanup(func() {
		stopSchedulers()
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := runtimeManager.CloseSession(cleanupCtx, sessionKey); err != nil &&
			!runtimectx.IsRuntimeTransportClosedError(err) {
			t.Errorf("清理 Room 测试 runtime 失败: %v", err)
		}
	})
}

func createSingleAgentGroupRoom(
	ctx context.Context,
	service *roomsvc.Service,
	agentID string,
) (*protocol.ConversationContextAggregate, error) {
	return service.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentID},
	})
}

type fakeRoomClient struct {
	mu             sync.Mutex
	sessionID      string
	connectErr     error
	messages       chan sdkprotocol.ReceivedMessage
	interruptCalls int
	disconnects    int
	stoppedTasks   []string
	taskMessages   []string
	queryPrompts   []string
	sentContents   []string
	onQuery        func(context.Context, string) error
	onInterrupt    func(context.Context)
}

func newFakeRoomClient() *fakeRoomClient {
	return &fakeRoomClient{
		sessionID: "room-sdk-session",
		messages:  make(chan sdkprotocol.ReceivedMessage, 32),
	}
}

func sendFakeRoomThinkingStream(client *fakeRoomClient, messageID string, thinking string) {
	client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeStreamEvent,
		SessionID: client.sessionID,
		Stream: &sdkprotocol.StreamEvent{Event: map[string]any{
			"type":    "message_start",
			"message": map[string]any{"id": messageID},
		}},
	}
	client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeStreamEvent,
		SessionID: client.sessionID,
		Stream: &sdkprotocol.StreamEvent{Event: map[string]any{
			"type":  "content_block_start",
			"index": 0,
			"content_block": map[string]any{
				"type": "thinking", "thinking": thinking,
			},
		}},
	}
}

func (c *fakeRoomClient) Connect(context.Context) error { return c.connectErr }

func (c *fakeRoomClient) Query(ctx context.Context, prompt string) error {
	c.mu.Lock()
	c.queryPrompts = append(c.queryPrompts, prompt)
	c.mu.Unlock()
	if c.onQuery != nil {
		return c.onQuery(ctx, prompt)
	}
	return nil
}

func (c *fakeRoomClient) ReceiveMessages(context.Context) <-chan sdkprotocol.ReceivedMessage {
	return c.messages
}

func (c *fakeRoomClient) SendContent(_ context.Context, content any, _ *string, _ string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if text, ok := content.(string); ok {
		c.sentContents = append(c.sentContents, text)
	}
	return nil
}

func (c *fakeRoomClient) Interrupt(ctx context.Context) error {
	c.mu.Lock()
	c.interruptCalls++
	callback := c.onInterrupt
	c.mu.Unlock()
	if callback != nil {
		callback(ctx)
	}
	return nil
}

func (c *fakeRoomClient) StopTask(_ context.Context, taskID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stoppedTasks = append(c.stoppedTasks, taskID)
	return nil
}

func (c *fakeRoomClient) SendTaskMessage(_ context.Context, taskID string, _ string, _ string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.taskMessages = append(c.taskMessages, taskID)
	return nil
}

func (c *fakeRoomClient) RemoveMessages(context.Context, []string) error { return nil }

func (c *fakeRoomClient) SetPermissionMode(context.Context, sdkpermission.Mode) error { return nil }

func (c *fakeRoomClient) Retire() {}

func (c *fakeRoomClient) Disconnect(context.Context) error {
	c.mu.Lock()
	c.disconnects++
	c.mu.Unlock()
	return nil
}

func (c *fakeRoomClient) Reconfigure(context.Context, agentclient.Options) error {
	return nil
}

func (c *fakeRoomClient) SessionID() string { return c.sessionID }

type fakeRoomFactory struct {
	mu      sync.Mutex
	clients []*fakeRoomClient
	index   int
	options []agentclient.Options
}

func (f *fakeRoomFactory) New(options agentclient.Options) runtimectx.Client {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.options = append(f.options, options)
	if f.index >= len(f.clients) {
		return newFakeRoomClient()
	}
	client := f.clients[f.index]
	f.index++
	return client
}

func (f *fakeRoomFactory) LastOptions() agentclient.Options {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.options) == 0 {
		return agentclient.Options{}
	}
	return f.options[len(f.options)-1]
}

func (f *fakeRoomFactory) Options() []agentclient.Options {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]agentclient.Options(nil), f.options...)
}

func sendFakeAssistantResult(client *fakeRoomClient, messageID string, text string) {
	sendFakeAssistantResultWithUsage(client, messageID, text, nil)
}

type fakeRoomTitleScheduler struct {
	mu       sync.Mutex
	requests []titlegen.Request
}

func (s *fakeRoomTitleScheduler) Schedule(_ context.Context, request titlegen.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, request)
}

func (s *fakeRoomTitleScheduler) LastRequest() titlegen.Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.requests) == 0 {
		return titlegen.Request{}
	}
	return s.requests[len(s.requests)-1]
}

func sendFakeTerminalAssistantAndClose(client *fakeRoomClient, messageID string, text string, usage map[string]any) {
	client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeAssistant,
		SessionID: client.sessionID,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				ID:         messageID,
				Model:      "sonnet",
				StopReason: "end_turn",
				Usage:      usage,
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.TextBlock{Text: text},
				},
			},
		},
	}
	close(client.messages)
}

func sendFakeAssistantResultWithUsage(client *fakeRoomClient, messageID string, text string, usage map[string]any) {
	client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeAssistant,
		SessionID: client.sessionID,
		Assistant: &sdkprotocol.AssistantMessage{
			Message: sdkprotocol.ConversationEnvelope{
				ID:    messageID,
				Model: "sonnet",
				Content: []sdkprotocol.ContentBlock{
					sdkprotocol.TextBlock{Text: text},
				},
			},
		},
	}
	client.messages <- sdkprotocol.ReceivedMessage{
		Type:      sdkprotocol.MessageTypeResult,
		SessionID: client.sessionID,
		UUID:      messageID + "-result",
		Result: &sdkprotocol.ResultMessage{
			Subtype:    "success",
			DurationMS: 1,
			NumTurns:   1,
			Result:     "done",
			Usage:      usage,
		},
	}
}

type realtimeTestSender struct {
	key    string
	events chan protocol.EventMessage
}

func newRealtimeTestSender(key string) *realtimeTestSender {
	return &realtimeTestSender{
		key:    key,
		events: make(chan protocol.EventMessage, 64),
	}
}

func (s *realtimeTestSender) Key() string { return s.key }

func (s *realtimeTestSender) IsClosed() bool { return false }

func (s *realtimeTestSender) SendEvent(_ context.Context, event protocol.EventMessage) error {
	s.events <- event
	return nil
}

func collectRoomEventsUntil(
	t *testing.T,
	events <-chan protocol.EventMessage,
	stop func([]protocol.EventMessage, protocol.EventMessage) bool,
) []protocol.EventMessage {
	t.Helper()
	result := make([]protocol.EventMessage, 0, 16)
	timeout := time.After(3 * time.Second)
	for {
		select {
		case event := <-events:
			result = append(result, event)
			if stop(result, event) {
				return result
			}
		case <-timeout:
			t.Fatalf("等待 Room 事件超时，当前事件: %+v", result)
		}
	}
}

func readRoomPrivateHistory(
	t *testing.T,
	root string,
	ownerUserID string,
	workspacePath string,
	sessionKey string,
	agentID string,
	sessionID string,
) []protocol.Message {
	t.Helper()
	historyStore := workspacestore.NewAgentHistoryStore(root).ForOwner(ownerUserID)
	rows, err := historyStore.ReadMessages(workspacePath, protocol.Session{
		SessionKey: sessionKey,
		AgentID:    agentID,
		SessionID:  stringPointer(sessionID),
		Options:    map[string]any{},
	}, nil)
	if err != nil {
		t.Fatalf("读取 room transcript 历史失败: %v", err)
	}
	return rows
}

func writeRoomTranscriptFixture(
	t *testing.T,
	ownerUserID string,
	workspacePath string,
	sessionID string,
	rows []map[string]any,
) {
	t.Helper()
	if strings.TrimSpace(sessionID) == "" {
		t.Fatal("session_id 为空，无法写入 room transcript fixture")
	}
	projectDir := filepath.Join(
		appfs.UserRuntimeRoot(ownerUserID),
		"projects",
		workspacestore.TranscriptProjectDirectoryName(workspacePath),
	)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("创建 room transcript 目录失败: %v", err)
	}
	file, err := os.Create(filepath.Join(projectDir, sessionID+".jsonl"))
	if err != nil {
		t.Fatalf("创建 room transcript fixture 失败: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	for _, row := range rows {
		if err := encoder.Encode(row); err != nil {
			t.Fatalf("写入 room transcript fixture 失败: %v", err)
		}
	}
}

func anyToInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case float32:
		return int(typed)
	default:
		return 0
	}
}

func anyToString(value any) string {
	if typed, ok := value.(string); ok {
		return typed
	}
	return ""
}

func assertRoomEventTypes(t *testing.T, events []protocol.EventMessage, expected []protocol.EventType) {
	t.Helper()
	if len(events) < len(expected) {
		t.Fatalf("Room 事件数量不足: got=%d want>=%d all=%+v", len(events), len(expected), events)
	}
	for index, eventType := range expected {
		if events[index].EventType != eventType {
			t.Fatalf("第 %d 个 Room 事件类型不正确: got=%s want=%s all=%+v", index, events[index].EventType, eventType, events)
		}
	}
}

func countEventType(events []protocol.EventMessage, target protocol.EventType) int {
	count := 0
	for _, event := range events {
		if event.EventType == target {
			count++
		}
	}
	return count
}

func hasAgentRoundStatus(events []protocol.EventMessage, agentID string, status string) bool {
	for _, event := range events {
		if event.EventType != protocol.EventTypeAgentRoundStatus {
			continue
		}
		if event.Data["agent_id"] == agentID && event.Data["status"] == status {
			return true
		}
	}
	return false
}

func countRoomResultSubtype(events []protocol.EventMessage, subtype string) int {
	count := 0
	for _, event := range events {
		if event.EventType != protocol.EventTypeMessage {
			continue
		}
		if event.Data["role"] == "result" && event.Data["subtype"] == subtype {
			count++
			continue
		}
		if event.Data["role"] == "assistant" {
			summary, ok := event.Data["result_summary"].(map[string]any)
			if ok && summary["subtype"] == subtype {
				count++
			}
		}
	}
	return count
}

func hasChatAckPendingAgent(events []protocol.EventMessage, agentID string) bool {
	for _, event := range events {
		if event.EventType != protocol.EventTypeChatAck {
			continue
		}
		pending, ok := event.Data["pending"].([]protocol.ChatAckPendingSlot)
		if !ok {
			continue
		}
		for _, item := range pending {
			if item.AgentID == agentID {
				return true
			}
		}
	}
	return false
}

func inputQueueItemsFromEvent(event protocol.EventMessage) []protocol.InputQueueItem {
	switch items := event.Data["items"].(type) {
	case []protocol.InputQueueItem:
		return items
	case []any:
		result := make([]protocol.InputQueueItem, 0, len(items))
		for _, item := range items {
			payload, err := json.Marshal(item)
			if err != nil {
				continue
			}
			var parsed protocol.InputQueueItem
			if err = json.Unmarshal(payload, &parsed); err != nil {
				continue
			}
			result = append(result, parsed)
		}
		return result
	default:
		return nil
	}
}

func hasAgentPublicMessage(events []protocol.EventMessage, agentID string) bool {
	for _, event := range events {
		if event.EventType != protocol.EventTypeMessage {
			continue
		}
		if event.Data["agent_id"] == agentID &&
			(event.Data["role"] == "assistant" || event.Data["role"] == "result") {
			return true
		}
	}
	return false
}

func hasStreamText(events []protocol.EventMessage, text string) bool {
	for _, event := range events {
		if event.EventType != protocol.EventTypeStream {
			continue
		}
		block, _ := event.Data["content_block"].(map[string]any)
		if strings.Contains(normalizePendingValue(block["text"]), text) {
			return true
		}
	}
	return false
}

func roomTestStringSliceContains(values []string, target string) bool {
	return slices.Contains(values, target)
}

func normalizePendingValue(value any) string {
	if typed, ok := value.(string); ok {
		return typed
	}
	return ""
}

func findRoomAssistantMessagePayload(t *testing.T, events []protocol.EventMessage, messageID string) protocol.Message {
	t.Helper()
	// 同一消息可能被终态投影更新，校验最后到达的快照。
	for index := len(events) - 1; index >= 0; index-- {
		event := events[index]
		if event.EventType != protocol.EventTypeMessage || event.MessageID != messageID {
			continue
		}
		if event.Data["role"] != "assistant" {
			continue
		}
		return protocol.Message(event.Data)
	}
	t.Fatalf("未找到 Room assistant message_id=%s 的 durable 消息: %+v", messageID, events)
	return nil
}

func roomContentBlocksFromPayload(t *testing.T, payload map[string]any) []map[string]any {
	t.Helper()
	rawBlocks, ok := payload["content"]
	if !ok {
		t.Fatalf("Room 消息缺少 content: %+v", payload)
	}
	switch typed := rawBlocks.(type) {
	case []map[string]any:
		return typed
	case []any:
		result := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			block, ok := item.(map[string]any)
			if !ok {
				t.Fatalf("Room content block 类型不正确: %+v", payload)
			}
			result = append(result, block)
		}
		return result
	default:
		t.Fatalf("Room content 类型不正确: %+v", payload)
		return nil
	}
}

// 共享 Room 测试夹具。

type fakeRoomRuntimeCloser struct {
	keys []string
}

func (f *fakeRoomRuntimeCloser) CloseSession(_ context.Context, sessionKey string) error {
	f.keys = append(f.keys, sessionKey)
	return nil
}

func createTestAgent(
	t *testing.T,
	service *agentsvc.Service,
	ctx context.Context,
	name string,
) *protocol.Agent {
	t.Helper()

	item, err := service.CreateAgent(ctx, protocol.CreateRequest{Name: name})
	if err != nil {
		t.Fatalf("创建测试 agent 失败: %v", err)
	}
	return item
}

func newRoomTestConfig(t *testing.T) config.Config {
	t.Helper()

	root := t.TempDir()
	t.Setenv("HOME", root)
	t.Setenv("NEXUS_STATE_ROOT", filepath.Join(root, ".nexus"))
	t.Setenv("NEXUS_CONFIG_DIR", filepath.Join(root, ".nexus"))
	return config.Config{
		Host:           "127.0.0.1",
		Port:           18011,
		ProjectName:    "nexus-room-test",
		APIPrefix:      "/nexus/v1",
		WebSocketPath:  "/nexus/v1/chat/ws",
		DefaultAgentID: "nexus",
		WorkspacePath:  filepath.Join(root, "workspace"),
		DatabaseDriver: "sqlite",
		DatabaseURL:    filepath.Join(root, "nexus.db"),
	}
}

func migrateRoomSQLite(t *testing.T, databaseURL string) {
	t.Helper()
	handlertest.MigrateSQLiteFromDir(t, databaseURL, roomMigrationDir(t))
}

func newRoomTestAgentService(
	t *testing.T,
	cfg config.Config,
) (*agentsvc.Service, *sql.DB, error) {
	t.Helper()
	agentService, db, err := app.NewAgentService(cfg)
	if err != nil {
		return nil, nil, err
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("关闭 Room 测试数据库失败: %v", err)
		}
	})
	return agentService, db, nil
}

func assertRuntimeClosedKeys(t *testing.T, got []string, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("关闭 runtime keys 数量不一致: got=%v want=%v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("关闭 runtime keys 不一致: got=%v want=%v", got, want)
		}
	}
}

func stringPointer(value string) *string {
	return &value
}

func roomMigrationDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("定位测试文件失败")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "db", "migrations", "sqlite")
}

func TestRealtimeServiceHandleGroupChatWithExplicitAgentMention(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}

	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "user-room-usage",
		Username: "room-owner",
		Role:     authsvc.RoleOwner,
	})
	memberAgent := createTestAgent(t, agentService, ctx, "单聊助手")
	roomContext, err := createSingleAgentGroupRoom(ctx, roomService, memberAgent.AgentID)
	if err != nil {
		t.Fatalf("创建单成员 room 失败: %v", err)
	}

	client := newFakeRoomClient()
	client.onQuery = func(_ context.Context, _ string) error {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeAssistant,
				SessionID: client.sessionID,
				Assistant: &sdkprotocol.AssistantMessage{
					Message: sdkprotocol.ConversationEnvelope{
						ID:    "assistant-sdk-1",
						Model: "sonnet",
						Content: []sdkprotocol.ContentBlock{
							sdkprotocol.TextBlock{Text: "已收到，正在处理。"},
						},
					},
				},
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "result-sdk-1",
				Result: &sdkprotocol.ResultMessage{
					Subtype:       "success",
					DurationMS:    15,
					DurationAPIMS: 11,
					NumTurns:      1,
					Result:        "done",
					Usage: map[string]any{
						"input_tokens":  3,
						"output_tokens": 5,
					},
				},
			}
		}()
		return nil
	}

	permission := permissionctx.NewContext()
	runtimeManager := runtimectx.NewManager()
	factory := &fakeRoomFactory{clients: []*fakeRoomClient{client}}
	service := realtimesvc.NewServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimeManager,
		permission,
		factory,
	)
	usageService := usagesvc.NewServiceWithDB(cfg, db)
	service.SetUsageRecorder(usageService)
	roomHistory := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath)

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-1")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:      sharedSessionKey,
		RoomID:          roomContext.Room.ID,
		ConversationID:  roomContext.Conversation.ID,
		ClientRequestID: "request-room-user-1",
		ClientMessageID: "local-room-user-1",
		Content:         "@单聊助手 你好",
		RoundID:         "room-round-1",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	events := collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	assertRoomEventTypes(t, events, []protocol.EventType{
		protocol.EventTypeMessage,
		protocol.EventTypeRoundStatus,
		protocol.EventTypeChatAck,
		protocol.EventTypeSessionStatus,
		protocol.EventTypeAgentRoundStatus,
		protocol.EventTypeStreamStart,
		protocol.EventTypeMessage,
		protocol.EventTypeMessage,
		protocol.EventTypeAgentRoundStatus,
		protocol.EventTypeStreamEnd,
		protocol.EventTypeRoundStatus,
	})
	client.mu.Lock()
	queryPrompts := append([]string(nil), client.queryPrompts...)
	client.mu.Unlock()
	if len(queryPrompts) != 1 {
		t.Fatalf("期望发送 1 条 Room runtime query，实际 %d", len(queryPrompts))
	}
	for _, expected := range []string{
		"<public_anchor>",
		"<public_feed>",
		`<latest_trigger type="public_chat">`,
	} {
		if !strings.Contains(queryPrompts[0], expected) {
			t.Fatalf("Room runtime query 缺少动态上下文 %q:\n%s", expected, queryPrompts[0])
		}
	}
	if strings.Contains(queryPrompts[0], "<nexus_runtime_context>") {
		t.Fatalf("默认关闭情绪系统时不应注入动态上下文:\n%s", queryPrompts[0])
	}

	pendingMsgID := ""
	foundCorrelatedUserEvent := false
	for _, event := range events {
		if event.EventType == protocol.EventTypeMessage && event.Data["role"] == "user" {
			foundCorrelatedUserEvent = event.Data["client_message_id"] == "local-room-user-1"
		}
		if event.EventType == protocol.EventTypeChatAck {
			if pending, ok := event.Data["pending"].([]protocol.ChatAckPendingSlot); ok && len(pending) > 0 {
				pendingMsgID = pending[0].MsgID
			}
		}
		if event.EventType == protocol.EventTypeMessage && event.MessageID == "assistant-sdk-1" {
			if pendingMsgID == "" {
				t.Fatal("未拿到 pending slot msg_id")
			}
			if event.MessageID == pendingMsgID {
				t.Fatalf("assistant message_id 不应回退成 slot msg_id: %s", pendingMsgID)
			}
		}
	}
	if !foundCorrelatedUserEvent {
		t.Fatal("Room durable user 广播必须携带 client_message_id 以原位替换 optimistic 消息")
	}

	roomSystemPrompt := factory.LastOptions().System.Append
	roomPromptOptions := factory.LastOptions().System
	if !strings.Contains(roomPromptOptions.AppendStatic, "## Execution Orchestration") ||
		!strings.Contains(roomPromptOptions.AppendStatic, "# Nexus Room") ||
		!strings.Contains(roomPromptOptions.AppendStatic, "<room_member_directory>") {
		t.Fatalf("Room 稳定 prompt 应包含 execution contract、房间规则与成员目录: %q", roomPromptOptions.AppendStatic)
	}
	if strings.Contains(roomPromptOptions.AppendDynamic, "## Execution Orchestration") ||
		strings.Contains(roomPromptOptions.AppendDynamic, "# Nexus Room") ||
		strings.Contains(roomPromptOptions.AppendDynamic, "<room_member_directory>") {
		t.Fatalf("Room 动态 prompt 不应重复房间稳定段: %q", roomPromptOptions.AppendDynamic)
	}
	if strings.Count(roomPromptOptions.AppendStatic, "## Execution Orchestration") != 1 ||
		strings.Count(roomSystemPrompt, "## Execution Orchestration") != 1 {
		t.Fatalf("Room execution contract 应只注入一次: static=%q combined=%q", roomPromptOptions.AppendStatic, roomSystemPrompt)
	}
	for _, expected := range []string{
		"# Nexus Room",
		"You are a member in a multi-member Nexus Room",
		"Each turn includes <public_feed>",
		"Before substantial execution, assess separability",
		"members may use local subagents",
		"Current-Room private messaging is disabled",
		"host-issued room_host_default trigger when the owner has enabled host auto-reply",
		"managed Plan and assign_work through execution-orchestrator",
		"never substitute raw @",
		"If a private message wakes you, answer once in the final reply",
		"The final reply may be persisted or projected verbatim",
		"# Nexus Room Member Directory",
		"<room_member_directory>",
		"- name=单聊助手 agent_id=" + memberAgent.AgentID,
	} {
		if !strings.Contains(roomSystemPrompt, expected) {
			t.Fatalf("Room 固定规则应注入 SDK append system prompt，缺少 %q:\n%s", expected, roomSystemPrompt)
		}
	}
	for _, unexpected := range []string{
		`nexus.send_message`,
		"以成员 单聊助手",
		"<current_room_member>",
	} {
		if strings.Contains(roomSystemPrompt, unexpected) {
			t.Fatalf("Room 固定规则不应包含动态变量 %q:\n%s", unexpected, roomSystemPrompt)
		}
	}
	if got := strings.TrimSpace(factory.LastOptions().Env["NEXUS_PROJECT_ROOT"]); got != "" {
		t.Fatalf("Room runtime 不应再注入项目根目录: got=%q", got)
	}
	for _, key := range []string{"NEXUSCTL_COMMAND_PATH", "NEXUSCTL_USER_ID", "NEXUSCTL_WORKSPACE_PATH"} {
		if got := strings.TrimSpace(factory.LastOptions().Env[key]); got != "" {
			t.Fatalf("Room runtime 不得获得 owner-scoped CLI 环境 %s=%q: %+v", key, got, factory.LastOptions().Env)
		}
	}

	privateSessionKey := protocol.BuildRoomAgentSessionKey(roomContext.Conversation.ID, memberAgent.AgentID, roomContext.Room.RoomType)
	cursor, ok, err := workspacestore.NewAgentHistoryStore(cfg.WorkspacePath).ReadRoomPublicCursor(
		memberAgent.WorkspacePath,
		privateSessionKey,
		roomContext.Conversation.ID,
		memberAgent.AgentID,
	)
	if err != nil {
		t.Fatalf("读取 Room 公区 cursor 失败: %v", err)
	}
	if !ok || !strings.HasPrefix(cursor.LastPublicMessageID, "msg_user_") {
		t.Fatalf("成功 round 应记录目标 agent 公区消费位置: ok=%v cursor=%+v", ok, cursor)
	}
	roomTranscriptBaseTime := time.Now().Add(-2 * time.Second).UTC()
	writeRoomTranscriptFixture(t, roomContext.Room.OwnerUserID, memberAgent.WorkspacePath, client.sessionID, []map[string]any{
		{
			"type":      "user",
			"uuid":      "room-user-1",
			"sessionId": client.sessionID,
			"timestamp": roomTranscriptBaseTime.Format(time.RFC3339Nano),
			"message": map[string]any{
				"role":    "user",
				"content": "dispatch prompt",
			},
		},
		{
			"type":       "assistant",
			"uuid":       "assistant-sdk-1",
			"sessionId":  client.sessionID,
			"parentUuid": "room-user-1",
			"timestamp":  roomTranscriptBaseTime.Add(200 * time.Millisecond).Format(time.RFC3339Nano),
			"message": map[string]any{
				"role": "assistant",
				"content": []map[string]any{
					{"type": "text", "text": "已收到，正在处理。"},
				},
			},
		},
	})
	sharedMessages, err := roomHistory.ReadMessages(roomContext.Room.OwnerUserID, roomContext.Conversation.ID, nil)
	if err != nil {
		t.Fatalf("读取共享 Room 消息失败: %v", err)
	}
	if len(sharedMessages) != 2 {
		t.Fatalf("共享消息数量不正确: got=%d want=2", len(sharedMessages))
	}
	if _, exists := sharedMessages[0]["client_message_id"]; exists {
		t.Fatalf("client_message_id 不应写入 Room 历史消息: %+v", sharedMessages[0])
	}
	if sharedMessages[1]["message_id"] != "assistant-sdk-1" {
		t.Fatalf("共享 assistant message_id 不正确: %+v", sharedMessages[1])
	}
	sharedSummary, ok := sharedMessages[1]["result_summary"].(map[string]any)
	if !ok || anyToString(sharedSummary["result"]) != "done" {
		t.Fatalf("共享 result 摘要应挂在 assistant 上: %+v", sharedMessages[1])
	}
	privateMessages := readRoomPrivateHistory(
		t,
		cfg.WorkspacePath,
		roomContext.Room.OwnerUserID,
		memberAgent.WorkspacePath,
		privateSessionKey,
		memberAgent.AgentID,
		client.sessionID,
	)
	if len(privateMessages) != 2 {
		t.Fatalf("私有 runtime 消息数量不正确: got=%d want=2", len(privateMessages))
	}
	if privateMessages[0]["role"] != "user" || privateMessages[1]["role"] != "assistant" {
		t.Fatalf("私有 runtime 消息顺序不正确: %+v", privateMessages)
	}
	privateUserContent := anyToString(privateMessages[0]["content"])
	for _, expected := range []string{
		"<public_feed>",
		"User: @单聊助手 你好",
	} {
		if !strings.Contains(privateUserContent, expected) {
			t.Fatalf("私有 round marker 应记录实际 Room dispatch prompt，缺少 %q:\n%s", expected, privateUserContent)
		}
	}
	privateSummary, ok := privateMessages[1]["result_summary"].(map[string]any)
	if !ok || anyToInt(privateSummary["duration_ms"]) != 15 || anyToString(privateSummary["result"]) != "done" {
		t.Fatalf("私有 result 应保留 runtime 摘要: %+v", privateMessages[1])
	}
	usageSummary, err := usageService.Summary(ctx, "user-room-usage")
	if err != nil {
		t.Fatalf("读取 room token usage 失败: %v", err)
	}
	if usageSummary.InputTokens != 3 || usageSummary.OutputTokens != 5 || usageSummary.TotalTokens != 8 {
		t.Fatalf("room result usage 未写入 ledger: %+v", usageSummary)
	}
	if usageSummary.SessionCount != 1 || usageSummary.MessageCount != 1 {
		t.Fatalf("room usage 计数不正确: %+v", usageSummary)
	}
}

func TestRealtimeServiceRoutesUnmentionedGroupMessageToConfiguredHost(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)

	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "user-room-host-default",
		Username: "room-owner",
		Role:     authsvc.RoleOwner,
	})
	amy := createTestAgent(t, agentService, ctx, "Amy")
	devin := createTestAgent(t, agentService, ctx, "Devin")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs:             []string{amy.AgentID, devin.AgentID},
		Name:                 "群主接管测试房间",
		HostAgentID:          amy.AgentID,
		HostAutoReplyEnabled: true,
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	client := newFakeRoomClient()
	hostPrompt := make(chan string, 1)
	client.onQuery = func(_ context.Context, prompt string) error {
		hostPrompt <- prompt
		go sendFakeAssistantResult(client, "amy-room-host-default", "我来处理这条需求。")
		return nil
	}

	permission := permissionctx.NewContext()
	factory := &fakeRoomFactory{clients: []*fakeRoomClient{client}}
	service := realtimesvc.NewServiceWithFactory(cfg, roomService, agentService, runtimectx.NewManager(), permission, factory)
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-host-default")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:                  sharedSessionKey,
		RoomID:                      roomContext.Room.ID,
		ConversationID:              roomContext.Conversation.ID,
		Content:                     "帮我拆一下这个需求",
		RoundID:                     "room-round-host-default",
		TrustedConfigurationContext: true,
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	events := collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus &&
			event.Data["round_id"] == "room-round-host-default" &&
			event.Data["status"] == "finished"
	})
	select {
	case prompt := <-hostPrompt:
		if !strings.Contains(prompt, `type="room_host_default"`) || !strings.Contains(prompt, "帮我拆一下这个需求") {
			t.Fatalf("群主应收到接管触发和原始输入: %s", prompt)
		}
	default:
		t.Fatal("开启接管后，未 @ 消息必须唤醒已保存的群主")
	}
	if !hasChatAckPendingAgent(events, amy.AgentID) {
		t.Fatalf("未 @ 消息应创建群主 pending slot: %+v", events)
	}
	if hasChatAckPendingAgent(events, devin.AgentID) {
		t.Fatalf("未 @ 消息不应直接唤醒非群主成员: %+v", events)
	}
	roomHistory := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath)
	sharedMessages, err := roomHistory.ReadMessages(roomContext.Room.OwnerUserID, roomContext.Conversation.ID, nil)
	if err != nil {
		t.Fatalf("读取 Room 公区历史失败: %v", err)
	}
	foundUserMessage := false
	for _, message := range sharedMessages {
		if message["round_id"] == "room-round-host-default" && message["role"] == "user" {
			foundUserMessage = true
			if message["content"] != "帮我拆一下这个需求" {
				t.Fatalf("未 @ 用户输入内容不正确: %+v", message)
			}
		}
	}
	if !foundUserMessage {
		t.Fatalf("未 @ 用户输入仍应写入公区历史: %+v", sharedMessages)
	}
}

func TestRealtimeServiceAcksPublicMessageWithoutMention(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}

	ctx := context.Background()
	amy := createTestAgent(t, agentService, ctx, "Amy")
	devin := createTestAgent(t, agentService, ctx, "Devin")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{amy.AgentID, devin.AgentID},
		Name:     "公区无 @ 测试房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	permission := permissionctx.NewContext()
	runtimeManager := runtimectx.NewManager()
	service := NewServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimeManager,
		permission,
		&fakeRoomFactory{},
	)
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-no-mention")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:                  sharedSessionKey,
		RoomID:                      roomContext.Room.ID,
		ConversationID:              roomContext.Conversation.ID,
		Content:                     "先记一下这个背景",
		RoundID:                     "room-round-no-mention",
		TrustedConfigurationContext: true,
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	events := collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})
	if countEventType(events, protocol.EventTypeChatAck) != 1 {
		t.Fatalf("公区无 @ 消息也必须 ack，否则前端发送队列会卡住: %+v", events)
	}
}

func TestRealtimeServiceSuppressesNoReplyMarkerProjection(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}

	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "user-room-no-reply",
		Username: "room-owner",
		Role:     authsvc.RoleOwner,
	})
	agentValue := createTestAgent(t, agentService, ctx, "Amy")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{agentValue.AgentID},
		Name:     "无需回复测试房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	client := newFakeRoomClient()
	client.onQuery = func(_ context.Context, _ string) error {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeStreamEvent,
				SessionID: client.sessionID,
				Stream: &sdkprotocol.StreamEvent{
					Event: map[string]any{
						"type":  "content_block_start",
						"index": 0,
						"content_block": map[string]any{
							"type": "text",
							"text": "",
						},
					},
				},
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeStreamEvent,
				SessionID: client.sessionID,
				Stream: &sdkprotocol.StreamEvent{
					Event: map[string]any{
						"type":  "content_block_delta",
						"index": 0,
						"delta": map[string]any{
							"type": "text_delta",
							"text": "<nexus_room_no_reply/>",
						},
					},
				},
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeStreamEvent,
				SessionID: client.sessionID,
				Stream: &sdkprotocol.StreamEvent{
					Event: map[string]any{
						"type":  "content_block_stop",
						"index": 0,
					},
				},
			}
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeResult,
				SessionID: client.sessionID,
				UUID:      "amy-no-reply-result",
				Result: &sdkprotocol.ResultMessage{
					Subtype:    "success",
					Result:     "<nexus_room_no_reply/>",
					NumTurns:   1,
					DurationMS: 1,
					Usage: map[string]any{
						"input_tokens":  7,
						"output_tokens": 2,
					},
				},
			}
		}()
		return nil
	}

	permission := permissionctx.NewContext()
	runtimeManager := runtimectx.NewManager()
	service := NewServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimeManager,
		permission,
		&fakeRoomFactory{clients: []*fakeRoomClient{client}},
	)
	usageService := usagesvc.NewServiceWithDB(cfg, db)
	service.SetUsageRecorder(usageService)
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-no-reply")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@Amy 这条不用你回答",
		RoundID:        "room-round-no-reply",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	events := collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})
	if hasAgentPublicMessage(events, agentValue.AgentID) {
		t.Fatalf("无需回复标记不应投影到公区: %+v", events)
	}
	if hasStreamText(events, "<nexus_room_no_reply/>") {
		t.Fatalf("无需回复标记不应以流式文本暴露给前端: %+v", events)
	}
	if !hasAgentRoundStatus(events, agentValue.AgentID, "finished") {
		t.Fatalf("no-reply 仍须广播 slot 终态，让前端收口已发布的 thinking 快照: %+v", events)
	}
	usageSummary, err := usageService.Summary(ctx, "user-room-no-reply")
	if err != nil {
		t.Fatalf("读取 no-reply token usage 失败: %v", err)
	}
	if usageSummary.TotalTokens != 9 {
		t.Fatalf("no-reply result usage 也应写入 ledger: %+v", usageSummary)
	}
}

// 活跃目标投递测试。

func TestRealtimeServiceHostConsumesQueuedInputAsSoonAsItsSlotFinishes(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "owner-active-room-target",
		Username: "room-owner",
		Role:     authsvc.RoleOwner,
	})
	worker := createTestAgent(t, agentService, ctx, "Worker")
	host := createTestAgent(t, agentService, ctx, "Host")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs:             []string{worker.AgentID, host.AgentID},
		Name:                 "活跃插话目标测试",
		HostAgentID:          host.AgentID,
		HostAutoReplyEnabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	type followUpQuery struct {
		client *fakeRoomClient
		prompt string
	}
	hostFollowUpPrompt := make(chan followUpQuery, 1)
	newReusableClient := func() *fakeRoomClient {
		client := newFakeRoomClient()
		queryCount := 0
		client.onQuery = func(_ context.Context, prompt string) error {
			queryCount++
			if queryCount == 1 {
				return nil
			}
			hostFollowUpPrompt <- followUpQuery{client: client, prompt: prompt}
			go sendFakeAssistantResult(client, "assistant-host-follow-up", "已处理补充要求")
			return nil
		}
		return client
	}
	firstClient := newReusableClient()
	secondClient := newReusableClient()
	permission := permissionctx.NewContext()
	factory := &fakeRoomFactory{clients: []*fakeRoomClient{firstClient, secondClient}}
	service := NewServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permission,
		factory,
	)
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-active-target")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@Host @Worker 同时处理当前任务",
		RoundID:        "room-round-active-host-worker",
	}); err != nil {
		t.Fatalf("启动 Host/Worker shared round 失败: %v", err)
	}
	started := map[string]bool{}
	_ = collectRoomEventsUntil(t, sender.events, func(_ []protocol.EventMessage, event protocol.EventMessage) bool {
		if event.EventType == protocol.EventTypeStreamStart {
			started[event.AgentID] = true
		}
		return started[worker.AgentID] && started[host.AgentID]
	})
	options := factory.Options()
	if len(options) != 2 {
		t.Fatalf("shared round runtime 数量 = %d, want 2", len(options))
	}
	hostClient := firstClient
	workerClient := secondClient
	if options[1].CWD == host.WorkspacePath {
		hostClient, workerClient = secondClient, firstClient
	} else if options[0].CWD != host.WorkspacePath {
		t.Fatalf("无法按 workspace 识别 Host runtime: %+v", options)
	}

	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "然后重点检查一下边界条件",
		RoundID:        "room-round-unmentioned-host-queue",
		UserMessageID:  "msg-unmentioned-host-queue",
		DeliveryPolicy: protocol.ChatDeliveryPolicyQueue,
	}); err != nil {
		t.Fatalf("发送无 @ Host 队列输入失败: %v", err)
	}
	events := collectRoomEventsUntil(t, sender.events, func(_ []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeChatAck && event.Data["round_id"] == "room-round-unmentioned-host-queue"
	})
	for _, event := range events {
		if event.EventType == protocol.EventTypeChatAck && event.Data["round_id"] == "room-round-unmentioned-host-queue" {
			if committed, _ := event.Data["user_message_committed"].(bool); committed {
				t.Fatalf("Host 尚未消费时用户消息不应提交: %+v", event.Data)
			}
		}
	}

	queueStore := workspacestore.NewInputQueueStore(cfg.WorkspacePath)
	hostLocation := workspacestore.InputQueueLocation{
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  host.WorkspacePath,
		SessionKey:     protocol.BuildRoomAgentSessionKey(roomContext.Conversation.ID, host.AgentID, roomContext.Room.RoomType),
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
	}
	hostItems, err := queueStore.Snapshot(hostLocation)
	if err != nil {
		t.Fatal(err)
	}
	if len(hostItems) != 1 || hostItems[0].SourceMessageID != "msg-unmentioned-host-queue" ||
		len(hostItems[0].TargetAgentIDs) != 1 || hostItems[0].TargetAgentIDs[0] != host.AgentID {
		t.Fatalf("无 @ 输入应只进入 Host 消费队列: %+v", hostItems)
	}
	workerItems, err := queueStore.Snapshot(workspacestore.InputQueueLocation{
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  worker.WorkspacePath,
		SessionKey:     protocol.BuildRoomAgentSessionKey(roomContext.Conversation.ID, worker.AgentID, roomContext.Room.RoomType),
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(workerItems) != 0 {
		t.Fatalf("无 @ 输入不应被运行中的 Worker 抢走: %+v", workerItems)
	}

	go sendFakeAssistantResult(hostClient, "assistant-host-current", "Host 当前任务完成")
	select {
	case followUp := <-hostFollowUpPrompt:
		if followUp.client != hostClient {
			t.Fatal("Host 队列输入被其他 Agent runtime 消费")
		}
		if !strings.Contains(followUp.prompt, "然后重点检查一下边界条件") {
			t.Fatalf("Host follow-up prompt 缺少队列输入: %s", followUp.prompt)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Host slot 结束后未立即消费队列，仍在等待 Worker")
	}
	if service.CountRunningTasks(worker.AgentID) == 0 {
		t.Fatal("测试前提失效：Host 接力时 Worker 应仍在运行")
	}
	go sendFakeAssistantResult(workerClient, "assistant-worker-current", "Worker 当前任务完成")
	collectRoomEventsUntil(t, sender.events, func(_ []protocol.EventMessage, event protocol.EventMessage) bool {
		generating, _ := event.Data["is_generating"].(bool)
		return event.EventType == protocol.EventTypeSessionStatus && !generating
	})
}

func TestRealtimeServiceSerializesSiblingPublicMentionReturnsToFinishedHost(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}

	ctx := context.Background()
	host := createTestAgent(t, agentService, ctx, "Host")
	workerA := createTestAgent(t, agentService, ctx, "WorkerA")
	workerB := createTestAgent(t, agentService, ctx, "WorkerB")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs:    []string{host.AgentID, workerA.AgentID, workerB.AgentID},
		HostAgentID: host.AgentID,
		Name:        "成员并行回交 Host 测试房间",
		Title:       "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	hostClient := newFakeRoomClient()
	fastWorkerClient := newFakeRoomClient()
	slowWorkerClient := newFakeRoomClient()
	workerStarted := make(chan struct{}, 2)
	hostReturnPrompts := make(chan string, 2)
	releaseSlowWorker := make(chan struct{})
	releaseFirstHostReturn := make(chan struct{})
	var hostQueryCount atomic.Int32

	hostClient.onQuery = func(_ context.Context, prompt string) error {
		switch hostQueryCount.Add(1) {
		case 1:
			go sendFakeAssistantResult(
				hostClient,
				"host-delegates-two-workers",
				"@WorkerA 请完成 A 部分，@WorkerB 请完成 B 部分。",
			)
		case 2:
			hostReturnPrompts <- prompt
			go func() {
				<-releaseFirstHostReturn
				sendFakeAssistantResult(hostClient, "host-consumes-first-return", "第一份已收到，继续等待另一份。")
			}()
		case 3:
			hostReturnPrompts <- prompt
			go sendFakeAssistantResult(hostClient, "host-consumes-second-return", "两份回交均已收到。")
		}
		return nil
	}
	fastWorkerClient.onQuery = func(_ context.Context, _ string) error {
		workerStarted <- struct{}{}
		go sendFakeAssistantResult(fastWorkerClient, "worker-fast-return", "@Host 第一份成员回交。")
		return nil
	}
	slowWorkerClient.onQuery = func(_ context.Context, _ string) error {
		workerStarted <- struct{}{}
		go func() {
			<-releaseSlowWorker
			sendFakeAssistantResult(slowWorkerClient, "worker-slow-return", "@Host 第二份成员回交。")
		}()
		return nil
	}

	permission := permissionctx.NewContext()
	service := NewServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permission,
		&fakeRoomFactory{clients: []*fakeRoomClient{hostClient, fastWorkerClient, slowWorkerClient}},
	)

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := &realtimeTestSender{
		key:    "room-sender-sibling-returns-to-host",
		events: make(chan protocol.EventMessage, 512),
	}
	permission.BindSession(sharedSessionKey, sender)

	const rootRoundID = "room-round-sibling-returns-to-host"
	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@Host 请让两个成员并行处理后回交",
		RoundID:        rootRoundID,
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	for range 2 {
		select {
		case <-workerStarted:
		case <-time.After(2 * time.Second):
			t.Fatal("Host 委派后并非两个成员都进入 runtime")
		}
	}

	var firstReturnPrompt string
	select {
	case firstReturnPrompt = <-hostReturnPrompts:
		if !strings.Contains(firstReturnPrompt, "<latest_trigger type=\"public_mention\">\n") ||
			!strings.Contains(firstReturnPrompt, "@Host 第一份成员回交。") {
			t.Fatalf("Host 首次回交 prompt 缺少第一位成员触发: %s", firstReturnPrompt)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Host 完成初始委派后未被第一位成员再次唤醒")
	}
	if hostQueryCount.Load() != 2 {
		t.Fatalf("慢成员仍在运行时 Host 应只被再次唤醒一次: queries=%d", hostQueryCount.Load())
	}

	close(releaseSlowWorker)
	var queuedReturn protocol.InputQueueItem
	_ = collectRoomEventsUntil(t, sender.events, func(_ []protocol.EventMessage, event protocol.EventMessage) bool {
		if event.EventType != protocol.EventTypeInputQueue {
			return false
		}
		for _, item := range inputQueueItemsFromEvent(event) {
			if item.Source == protocol.InputQueueSourceAgentPublicMention &&
				item.AgentID == host.AgentID &&
				item.SourceMessageID == "worker-slow-return" {
				queuedReturn = item
				return true
			}
		}
		return false
	})
	if queuedReturn.DeliveryPolicy != protocol.ChatDeliveryPolicyQueue ||
		queuedReturn.HandoffID == "" ||
		queuedReturn.RootRoundID != rootRoundID {
		t.Fatalf("第二位成员回交必须按原 root 进入 Host 串行队列: %+v", queuedReturn)
	}
	if hostQueryCount.Load() != 2 {
		t.Fatalf("Host 忙时不得并发启动第二个回交 slot: queries=%d", hostQueryCount.Load())
	}
	select {
	case prompt := <-hostReturnPrompts:
		t.Fatalf("Host 首个回交尚未结束时不应消费第二个回交: %s", prompt)
	default:
	}

	hostQueueLocation := workspacestore.InputQueueLocation{
		OwnerUserID:    roomContext.Room.OwnerUserID,
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  host.WorkspacePath,
		SessionKey:     protocol.BuildRoomAgentSessionKey(roomContext.Conversation.ID, host.AgentID, roomContext.Room.RoomType),
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
	}
	hostQueueItems, err := workspacestore.NewInputQueueStore(cfg.WorkspacePath).Snapshot(hostQueueLocation)
	if err != nil {
		t.Fatalf("读取 Host 回交队列失败: %v", err)
	}
	if len(hostQueueItems) != 1 || hostQueueItems[0].ID != queuedReturn.ID {
		t.Fatalf("第二位成员回交未持久化到 Host 自己的队列: event=%+v stored=%+v", queuedReturn, hostQueueItems)
	}

	close(releaseFirstHostReturn)
	var secondReturnPrompt string
	select {
	case secondReturnPrompt = <-hostReturnPrompts:
		if !strings.Contains(secondReturnPrompt, "<latest_trigger type=\"public_mention\">\n") ||
			!strings.Contains(secondReturnPrompt, "@Host 第二份成员回交。") {
			t.Fatalf("Host 第二次回交 prompt 缺少排队成员触发: %s", secondReturnPrompt)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Host 首个回交结束后未串行消费第二位成员回交")
	}
	if hostQueryCount.Load() != 3 {
		t.Fatalf("Host 应按初始委派、第一份回交、第二份回交串行执行三次: queries=%d", hostQueryCount.Load())
	}

	handoffStore := workspacestore.NewRoomPublicHandoffStore(cfg.WorkspacePath)
	var handoffs []workspacestore.RoomPublicHandoff
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		handoffs, err = handoffStore.ListRoot(
			roomContext.Room.OwnerUserID,
			roomContext.Conversation.ID,
			rootRoundID,
		)
		if err != nil {
			t.Fatalf("读取成员回交 handoff ledger 失败: %v", err)
		}
		allFinished := len(handoffs) == 4
		for _, handoff := range handoffs {
			allFinished = allFinished && handoff.Status == "finished"
		}
		if allFinished {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(handoffs) != 4 {
		t.Fatalf("Host 委派两次且成员回交两次应产生四条 handoff: %+v", handoffs)
	}
	returnRoundIDs := make(map[string]struct{}, 2)
	returnSources := make(map[string]struct{}, 2)
	for _, handoff := range handoffs {
		if handoff.Status != "finished" {
			t.Fatalf("成员协作 handoff 未收口: %+v", handoff)
		}
		if handoff.TargetAgentID != host.AgentID {
			continue
		}
		if handoff.TargetRoundID == "" {
			t.Fatalf("回交 Host 的 handoff 没有实际 target round: %+v", handoff)
		}
		returnRoundIDs[handoff.TargetRoundID] = struct{}{}
		returnSources[handoff.SourceAgentID] = struct{}{}
	}
	if len(returnRoundIDs) != 2 || len(returnSources) != 2 {
		t.Fatalf("两个来源回交同一 Host 必须各自串行启动独立 round: %+v", handoffs)
	}
}

type fakeRoomQuotaChecker struct {
	err error
}

type testRoomGoalSessionOwnershipVerifier struct {
	agentNames map[string]string
}

func (v testRoomGoalSessionOwnershipVerifier) VerifyGoalSessionOwnership(
	_ context.Context,
	request goalsvc.GoalSessionOwnershipRequest,
) (goalsvc.GoalSessionOwnershipProof, error) {
	agentID := request.TrustedAgentID
	return goalsvc.GoalSessionOwnershipProof{
		TrustedAgentID:   agentID,
		TrustedAgentName: v.agentNames[agentID],
	}, nil
}

func TestRealtimeServiceHandleChatMarksInternalGoalUsageLimitedWhenQuotaExceeded(t *testing.T) {
	cfg := newRoomTestConfig(t)
	cfg.GoalEnabled = true
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "owner-room-internal-goal-quota",
		Username: "room-owner",
		Role:     authsvc.RoleOwner,
	})
	memberAgent := createTestAgent(t, agentService, ctx, "内部 Goal 额度助手")
	roomContext, err := createSingleAgentGroupRoom(ctx, roomService, memberAgent.AgentID)
	if err != nil {
		t.Fatalf("创建单成员 room 失败: %v", err)
	}

	factory := &fakeRoomFactory{clients: []*fakeRoomClient{newFakeRoomClient()}}
	service := realtimesvc.NewServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permissionctx.NewContext(),
		factory,
	)
	goalService := goalsvc.NewService(cfg, goalstore.NewRepository(cfg, db))
	goalService.SetSessionOwnershipVerifier(testRoomGoalSessionOwnershipVerifier{
		agentNames: map[string]string{memberAgent.AgentID: memberAgent.Name},
	})
	service.SetGoalContextProvider(goalService)
	quotaErr := subscriptionsvc.QuotaExceededError{UsedTokens: 10, LimitTokens: 10}
	service.SetQuotaChecker(fakeRoomQuotaChecker{err: quotaErr})

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	goal, err := goalService.Create(ctx, protocol.CreateGoalRequest{
		SessionKey:  sharedSessionKey,
		Objective:   "保持账号额度边界",
		OwnerUserID: "owner-room-internal-goal-quota",
	})
	if err != nil {
		t.Fatalf("创建 Room Goal 失败: %v", err)
	}
	err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "internal goal continuation",
		GoalID:         goal.ID,
		RoundID:        "room-round-internal-goal-quota",
		Internal:       true,
	})
	if !errors.Is(err, subscriptionsvc.ErrQuotaExceeded) {
		t.Fatalf("账号额度耗尽应阻止内部 Room Goal runtime，实际: %v", err)
	}
	limited, err := goalService.Current(ctx, sharedSessionKey)
	if err != nil {
		t.Fatalf("读取受限 Room Goal 失败: %v", err)
	}
	if limited.Status != protocol.GoalStatusUsageLimited || limited.LastError != quotaErr.ClientMessage() {
		t.Fatalf("受限 Room Goal = %#v, want usage_limited with actionable subscription quota message", limited)
	}
	if got := factory.LastOptions(); got.Model != "" {
		t.Fatalf("账号额度耗尽时不应创建内部 Room Goal runtime: %+v", got)
	}
}

func (f fakeRoomQuotaChecker) EnsureQuotaAvailable(context.Context, string) error {
	return f.err
}

func TestRealtimeServiceFailsRoomRoundWithoutResult(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	if err != nil {
		t.Fatalf("创建 room service 失败: %v", err)
	}

	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "user-room-assistant-terminal",
		Username: "room-owner",
		Role:     authsvc.RoleOwner,
	})
	memberAgent := createTestAgent(t, agentService, ctx, "终态助手")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{memberAgent.AgentID},
		Name:     "assistant 终态房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	client := newFakeRoomClient()
	client.onQuery = func(_ context.Context, _ string) error {
		go sendFakeTerminalAssistantAndClose(client, "assistant-terminal-no-result", "这是完整回复。", map[string]any{
			"input_tokens":  7,
			"output_tokens": 11,
		})
		return nil
	}

	permission := permissionctx.NewContext()
	runtimeManager := runtimectx.NewManager()
	service := realtimesvc.NewServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimeManager,
		permission,
		&fakeRoomFactory{clients: []*fakeRoomClient{client}},
	)
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-assistant-terminal")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@终态助手 写一句话",
		RoundID:        "room-round-assistant-terminal",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}

	events := collectRoomEventsUntil(t, sender.events, func(events []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "error"
	})
	for _, event := range events {
		if event.Data["status"] == "finished" {
			t.Fatalf("缺少 result 不能宣告 Room round 成功: %+v", event)
		}
	}
	assistantPayload := findRoomAssistantMessagePayload(t, events, "assistant-terminal-no-result")
	if assistantPayload["is_complete"] != true || assistantPayload["stop_reason"] != "error" {
		t.Fatalf("已有回复应投影为失败终态: %+v", assistantPayload)
	}
	blocks := roomContentBlocksFromPayload(t, assistantPayload)
	if len(blocks) != 1 || blocks[0]["text"] != "这是完整回复。" {
		t.Fatalf("失败收口不能丢失已生成的正文: %+v", assistantPayload)
	}
}

func TestRealtimeServiceKeepsSubagentRoomSlotInRuntimeManager(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)
	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := context.Background()
	memberAgent := createTestAgent(t, agentService, ctx, "子任务助手")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs: []string{memberAgent.AgentID},
		Name:     "subagent runtime 房间",
		Title:    "主对话",
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	client := newFakeRoomClient()
	client.onQuery = func(_ context.Context, _ string) error {
		go func() {
			client.messages <- sdkprotocol.ReceivedMessage{
				Type:      sdkprotocol.MessageTypeTaskStarted,
				SessionID: client.sessionID,
				TaskStarted: &sdkprotocol.TaskStartedMessage{
					TaskID:      "task-room-1",
					AgentID:     "sdk-subagent-1",
					AgentType:   "worker",
					Description: "检查 Room runtime",
					TaskType:    "local_agent",
					Additional: map[string]any{
						"child_session_id": "child-room-1",
						"name":             "Room runtime audit",
					},
				},
			}
			sendFakeAssistantResult(client, "assistant-room-subagent", "子任务已在后台执行。")
		}()
		return nil
	}

	permission := permissionctx.NewContext()
	runtimeManager := runtimectx.NewManager()
	service := realtimesvc.NewServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimeManager,
		permission,
		&fakeRoomFactory{clients: []*fakeRoomClient{client}},
	)
	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	sender := newRealtimeTestSender("room-sender-subagent-runtime")
	permission.BindSession(sharedSessionKey, sender)
	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@子任务助手 启动后台检查",
		RoundID:        "room-round-subagent-runtime",
	}); err != nil {
		t.Fatalf("HandleChat 失败: %v", err)
	}
	collectRoomEventsUntil(t, sender.events, func(_ []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == "finished"
	})

	runtimeSessionKey := protocol.BuildRoomAgentSessionKey(
		roomContext.Conversation.ID,
		memberAgent.AgentID,
		roomContext.Room.RoomType,
	)
	if !runtimeManager.HasSubagentHistory(runtimeSessionKey) {
		t.Fatal("Room slot 未在 runtime manager 保留 subagent history")
	}
	if rounds := runtimeManager.GetRunningRoundIDs(runtimeSessionKey); len(rounds) != 0 {
		t.Fatalf("父 round 结束后仍残留 running round: %+v", rounds)
	}
	if err = runtimeManager.StopTask(ctx, runtimeSessionKey, "task-room-1"); err != nil {
		t.Fatalf("Room task stop 未路由到 slot client: %v", err)
	}
	if err = runtimeManager.SendTaskMessage(ctx, runtimeSessionKey, "task-room-1", "继续检查", "继续检查"); err != nil {
		t.Fatalf("Room task follow-up 未路由到 slot client: %v", err)
	}
	client.mu.Lock()
	if client.disconnects != 0 || len(client.stoppedTasks) != 1 || len(client.taskMessages) != 1 {
		t.Fatalf("Room slot client 生命周期/控制不正确: disconnects=%d stopped=%+v messages=%+v", client.disconnects, client.stoppedTasks, client.taskMessages)
	}
	client.mu.Unlock()

	roomHistory := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath)
	messages, err := roomHistory.ReadMessages(roomContext.Room.OwnerUserID, roomContext.Conversation.ID, nil)
	if err != nil {
		t.Fatalf("读取 Room history 失败: %v", err)
	}
	var taskMetadata map[string]any
	for _, message := range messages {
		metadata, _ := message["metadata"].(map[string]any)
		if metadata["task_id"] == "task-room-1" {
			taskMetadata = metadata
			break
		}
	}
	if taskMetadata["runtime_kind"] != "nxs" || taskMetadata["child_session_id"] != "child-room-1" {
		t.Fatalf("Room task metadata 未保留 runtime/thread 身份: %+v", taskMetadata)
	}
	if err = runtimeManager.CloseSession(ctx, runtimeSessionKey); err != nil {
		t.Fatalf("清理 Room runtime 失败: %v", err)
	}
}

type roomDirectedMessageBroadcaster struct {
	mu     sync.Mutex
	events []protocol.EventMessage
}

func (b *roomDirectedMessageBroadcaster) Broadcast(_ context.Context, _ string, event protocol.EventMessage) []error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, event)
	return nil
}

func (b *roomDirectedMessageBroadcaster) Events() []protocol.EventMessage {
	b.mu.Lock()
	defer b.mu.Unlock()
	events := make([]protocol.EventMessage, len(b.events))
	copy(events, b.events)
	return events
}

func TestRealtimeServiceCreatesDirectedMessageWithoutPublicLeak(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "user-room-directed-message",
		Username: "room-owner",
		Role:     authsvc.RoleOwner,
	})
	amy := createTestAgent(t, agentService, ctx, "Amy")
	devin := createTestAgent(t, agentService, ctx, "Devin")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs:               []string{amy.AgentID, devin.AgentID},
		Name:                   "测试 Room",
		PrivateMessagesEnabled: true,
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	service := realtimesvc.NewService(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permissionctx.NewContext(),
	)
	broadcaster := &roomDirectedMessageBroadcaster{}
	service.SetRoomBroadcaster(broadcaster)

	message, err := service.HandleDirectedMessage(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomDirectedMessageRequest{
		SourceAgentID: amy.AgentID,
		Recipients:    []string{devin.AgentID},
		Content:       "只给 Devin 的提醒",
		ReplyRoute: protocol.RoomReplyRoute{
			Mode:       protocol.RoomReplyRoutePrivate,
			Recipients: []string{amy.AgentID},
			WakePolicy: protocol.RoomWakePolicyNone,
		},
	})
	if err != nil {
		t.Fatalf("创建 directed message 失败: %v", err)
	}
	if message.WakePolicy != protocol.RoomWakePolicyNone ||
		message.ReplyRoute.Mode != protocol.RoomReplyRoutePrivate ||
		len(message.Recipients) != 1 ||
		message.Recipients[0] != devin.AgentID {
		t.Fatalf("directed message 默认路由不正确: %+v", message)
	}

	event := waitForRoomBroadcastEvent(t, broadcaster, protocol.EventTypeRoomDirectedMessage)
	if event.RoomID != roomContext.Room.ID || event.ConversationID != roomContext.Conversation.ID {
		t.Fatalf("directed message 事件上下文不正确: %+v", event)
	}
	if event.Data["message_id"] != message.MessageID || event.Data["event_kind"] != "created" {
		t.Fatalf("directed message 创建事件不正确: %+v", event.Data)
	}
	if _, ok := event.Data["content"]; ok {
		t.Fatalf("directed message 事件不应泄漏正文: %+v", event.Data)
	}
	messageStore := workspacestore.NewRoomDirectedMessageStore(cfg.WorkspacePath)
	devinMessages, err := messageStore.ReadContextMessages(
		roomContext.Room.OwnerUserID,
		roomContext.Conversation.ID,
		devin.AgentID,
	)
	if err != nil {
		t.Fatalf("读取 Devin directed message 失败: %v", err)
	}
	if len(devinMessages) != 1 || devinMessages[0].Content != "只给 Devin 的提醒" {
		t.Fatalf("目标成员未读到 directed message: %+v", devinMessages)
	}

	roomHistory := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath)
	messages, err := roomHistory.ReadMessages(roomContext.Room.OwnerUserID, roomContext.Conversation.ID, nil)
	if err != nil {
		t.Fatalf("读取公区历史失败: %v", err)
	}
	for _, publicMessage := range messages {
		if strings.Contains(fmt.Sprint(publicMessage), "只给 Devin 的提醒") {
			t.Fatalf("directed message 正文不应写入公区 feed: %+v", messages)
		}
	}
}

func TestRealtimeServiceKeepsPublicMessageInSourceAgentRound(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)
	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID: "user-room-public-round", Username: "room-owner", Role: authsvc.RoleOwner,
	})
	source := createTestAgent(t, agentService, ctx, "Source")
	observer := createTestAgent(t, agentService, ctx, "Observer")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs:               []string{source.AgentID, observer.AgentID},
		Name:                   "公开消息归并 Room",
		PrivateMessagesEnabled: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	service := realtimesvc.NewService(
		cfg, roomService, agentService, runtimectx.NewManager(), permissionctx.NewContext(),
	)
	broadcaster := &roomDirectedMessageBroadcaster{}
	service.SetRoomBroadcaster(broadcaster)

	message, err := service.HandlePublicMessage(
		ctx,
		roomContext.Room.ID,
		roomContext.Conversation.ID,
		protocol.CreateRoomPublicMessageRequest{
			SourceAgentID:      source.AgentID,
			SourceAgentRoundID: "agent-round-public-1",
			RootRoundID:        "root-round-public-1",
			Content:            "公开结论",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if message["round_id"] != "root-round-public-1" ||
		message["agent_round_id"] != "agent-round-public-1" {
		t.Fatalf("公开消息未归并到来源 Agent 执行: %+v", message)
	}
	event := waitForRoomBroadcastEvent(t, broadcaster, protocol.EventTypeMessage)
	if event.RoundID != "root-round-public-1" || event.AgentRoundID != "agent-round-public-1" {
		t.Fatalf("公开消息事件身份不正确: %+v", event)
	}
}

func TestRealtimeServiceRechecksPrivateMessagingAtCallTime(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := app.NewAgentService(cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID: "user-room-private-revocation", Username: "room-owner", Role: authsvc.RoleOwner,
	})
	amy := createTestAgent(t, agentService, ctx, "Amy")
	devin := createTestAgent(t, agentService, ctx, "Devin")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs:               []string{amy.AgentID, devin.AgentID},
		Name:                   "撤权测试 Room",
		PrivateMessagesEnabled: true,
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}
	disabled := false
	if _, err = roomService.UpdateRoom(ctx, roomContext.Room.ID, protocol.UpdateRoomRequest{
		PrivateMessagesEnabled: &disabled,
	}); err != nil {
		t.Fatalf("关闭私域消息失败: %v", err)
	}

	service := realtimesvc.NewService(
		cfg, roomService, agentService, runtimectx.NewManager(), permissionctx.NewContext(),
	)
	request := protocol.CreateRoomDirectedMessageRequest{
		SourceAgentID: amy.AgentID, Recipients: []string{devin.AgentID},
		Content: "旧 runtime 不应继续发送", ReplyRoute: protocol.RoomReplyRoute{Mode: protocol.RoomReplyRouteNone},
	}
	if _, err = service.HandleDirectedMessage(ctx, roomContext.Room.ID, roomContext.Conversation.ID, request); err == nil ||
		!strings.Contains(err.Error(), "disabled") {
		t.Fatalf("关闭后旧 directed message 调用应被 service 拒绝，实际 err=%v", err)
	}
	if _, err = service.HandlePublicMessage(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomPublicMessageRequest{
		SourceAgentID: amy.AgentID, Content: "旧 runtime 不应继续主动广播",
	}); err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Fatalf("关闭后旧 public message 调用应被 service 拒绝，实际 err=%v", err)
	}
	if _, err = service.HandlePlatformPublicMessage(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomPublicMessageRequest{
		SourceAgentID: amy.AgentID, Content: "平台通讯群消息不依赖私域开关",
	}); err != nil {
		t.Fatalf("平台通讯群消息不应依赖私域开关: %v", err)
	}
}

func TestRealtimeServiceQueuesDirectedMessageWhenTargetRunning(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "user-room-directed-message-queue",
		Username: "room-owner",
		Role:     authsvc.RoleOwner,
	})
	amy := createTestAgent(t, agentService, ctx, "Amy")
	devin := createTestAgent(t, agentService, ctx, "Devin")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs:               []string{amy.AgentID, devin.AgentID},
		Name:                   "私信排队 Room",
		PrivateMessagesEnabled: true,
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	devinCurrentClient := newFakeRoomClient()
	devinInitialPrompt := make(chan string, 1)
	queuedPrompt := make(chan string, 1)
	var devinQueryCount atomic.Int32
	devinCurrentClient.onQuery = func(_ context.Context, prompt string) error {
		if devinQueryCount.Add(1) == 1 {
			devinInitialPrompt <- prompt
			return nil
		}
		queuedPrompt <- prompt
		go sendFakeAssistantResult(devinCurrentClient, "devin-directed-message-after-busy", "这是给 Amy 的排队私下回复")
		return nil
	}

	permission := permissionctx.NewContext()
	runtimeManager := runtimectx.NewManager()
	service := realtimesvc.NewServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimeManager,
		permission,
		&fakeRoomFactory{clients: []*fakeRoomClient{devinCurrentClient}},
	)

	sharedSessionKey := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	registerRealtimeServiceCleanup(t, service, runtimeManager, sharedSessionKey)
	sender := newRealtimeTestSender("room-sender-directed-message-queue")
	permission.BindSession(sharedSessionKey, sender)

	if err = service.HandleChat(ctx, realtimesvc.ChatRequest{
		SessionKey:     sharedSessionKey,
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
		Content:        "@Devin 先处理一个长任务",
		RoundID:        "room-round-devin-directed-busy",
	}); err != nil {
		t.Fatalf("启动 Devin 长任务失败: %v", err)
	}
	select {
	case prompt := <-devinInitialPrompt:
		if !strings.Contains(prompt, "@Devin 先处理一个长任务") {
			t.Fatalf("Devin 首轮 prompt 缺少长任务: %s", prompt)
		}
	case <-time.After(time.Second):
		t.Fatal("Devin 长任务未进入 runtime Query")
	}

	message, err := service.HandleDirectedMessage(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomDirectedMessageRequest{
		SourceAgentID: amy.AgentID,
		Recipients:    []string{devin.AgentID},
		Content:       "只给 Devin 的排队私信",
		WakePolicy:    protocol.RoomWakePolicyImmediate,
		ReplyRoute: protocol.RoomReplyRoute{
			Mode:       protocol.RoomReplyRoutePrivate,
			Recipients: []string{amy.AgentID},
			WakePolicy: protocol.RoomWakePolicyNone,
		},
	})
	if err != nil {
		t.Fatalf("创建 directed message 失败: %v", err)
	}

	targetQueueLocation := workspacestore.InputQueueLocation{
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  devin.WorkspacePath,
		SessionKey:     protocol.BuildRoomAgentSessionKey(roomContext.Conversation.ID, devin.AgentID, roomContext.Room.RoomType),
		RoomID:         roomContext.Room.ID,
		ConversationID: roomContext.Conversation.ID,
	}
	targetQueueItems, err := workspacestore.NewInputQueueStore(cfg.WorkspacePath).Snapshot(targetQueueLocation)
	if err != nil {
		t.Fatalf("读取目标 agent session 队列失败: %v", err)
	}
	if len(targetQueueItems) != 1 {
		t.Fatalf("directed message 应排队到目标 agent session: %+v", targetQueueItems)
	}
	queuedItem := targetQueueItems[0]
	if queuedItem.Source != protocol.InputQueueSourceAgentRoomMessage ||
		queuedItem.SourceAgentID != amy.AgentID ||
		queuedItem.SourceMessageID != message.MessageID ||
		len(queuedItem.TargetAgentIDs) != 1 ||
		queuedItem.TargetAgentIDs[0] != devin.AgentID ||
		queuedItem.ReplyRoute.Mode != protocol.RoomReplyRoutePrivate ||
		len(queuedItem.ReplyRoute.Recipients) != 1 ||
		queuedItem.ReplyRoute.Recipients[0] != amy.AgentID {
		t.Fatalf("directed message 队列项缺少来源、目标或 reply_route: %+v", queuedItem)
	}
	if strings.Contains(queuedItem.Content, "只给 Devin 的排队私信") {
		t.Fatalf("directed message 队列项不应泄漏私信正文: %+v", queuedItem)
	}
	select {
	case prompt := <-queuedPrompt:
		t.Fatalf("目标 agent 尚未空闲前不应启动 queued directed message: %s", prompt)
	default:
	}

	go sendFakeAssistantResult(devinCurrentClient, "devin-current-directed-task-done", "当前长任务完成。")
	select {
	case prompt := <-queuedPrompt:
		for _, expected := range []string{
			"<room_directed_messages>",
			"只给 Devin 的排队私信",
			"reply_route=private recipients=Amy",
		} {
			if !strings.Contains(prompt, expected) {
				t.Fatalf("queued directed message prompt 缺少片段 %q:\n%s", expected, prompt)
			}
		}
	case <-time.After(time.Second):
		t.Fatal("目标 agent 空闲后未派发 queued directed message")
	}

	messageStore := workspacestore.NewRoomDirectedMessageStore(cfg.WorkspacePath)
	waitForRoomDirectedMessageContent(t, messageStore, roomContext.Room.OwnerUserID, roomContext.Conversation.ID, amy.AgentID, "这是给 Amy 的排队私下回复")
}

func TestRealtimeServiceCarriesPublicRouteFromPrivateHandback(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "user-room-public-message-handback",
		Username: "room-owner",
		Role:     authsvc.RoleOwner,
	})
	amy := createTestAgent(t, agentService, ctx, "Amy")
	devin := createTestAgent(t, agentService, ctx, "Devin")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs:               []string{amy.AgentID, devin.AgentID},
		Name:                   "狼人杀测试 Room",
		PrivateMessagesEnabled: true,
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	witchClient := newFakeRoomClient()
	var witchSawPrompt atomic.Bool
	witchClient.onQuery = func(_ context.Context, prompt string) error {
		witchSawPrompt.Store(strings.Contains(prompt, "今晚被刀的是 Lucy") &&
			strings.Contains(prompt, "reply_route=private recipients=Amy"))
		sendFakeAssistantResult(witchClient, "devin-witch-reply", "救:Lucy；不毒")
		return nil
	}

	var service *realtimesvc.Service
	hostClient := newFakeRoomClient()
	var hostSawHandback atomic.Bool
	hostClient.onQuery = func(_ context.Context, prompt string) error {
		hostSawHandback.Store(strings.Contains(prompt, "救:Lucy；不毒") &&
			strings.Contains(prompt, "reply_route=public"))
		sendFakeAssistantResult(hostClient, "amy-after-private-handback", "<nexus_room_no_reply/>")
		return nil
	}

	service = realtimesvc.NewServiceWithFactory(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permissionctx.NewContext(),
		&fakeRoomFactory{clients: []*fakeRoomClient{witchClient, hostClient}},
	)
	broadcaster := &roomDirectedMessageBroadcaster{}
	service.SetRoomBroadcaster(broadcaster)

	_, err = service.HandleDirectedMessage(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomDirectedMessageRequest{
		SourceAgentID: amy.AgentID,
		Recipients:    []string{devin.AgentID},
		Content:       "今晚被刀的是 Lucy。是否用解药救？是否用毒药毒谁？格式：救:<名字>|不救；毒:<名字>|不毒。",
		WakePolicy:    protocol.RoomWakePolicyImmediate,
		ReplyRoute: protocol.RoomReplyRoute{
			Mode:       protocol.RoomReplyRoutePrivate,
			Recipients: []string{amy.AgentID},
			WakePolicy: protocol.RoomWakePolicyImmediate,
			NextReplyRoute: &protocol.RoomReplyRoute{
				Mode: protocol.RoomReplyRoutePublic,
			},
		},
	})
	if err != nil {
		t.Fatalf("创建女巫 directed message 失败: %v", err)
	}

	waitForAtomicBool(t, &witchSawPrompt, "女巫未看到私信问题")
	waitForAtomicBool(t, &hostSawHandback, "主持人未收到私域回交")
	messageStore := workspacestore.NewRoomDirectedMessageStore(cfg.WorkspacePath)
	amyMessages := waitForRoomDirectedMessageContent(t, messageStore, roomContext.Room.OwnerUserID, roomContext.Conversation.ID, amy.AgentID, "救:Lucy；不毒")
	if !roomDirectedMessageContentHasReplyRoute(amyMessages, "救:Lucy；不毒", protocol.RoomReplyRoutePublic) {
		t.Fatalf("私域回交应携带主持人下一跳公区路线: %+v", amyMessages)
	}
	roomHistory := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath)
	publicMessages, err := roomHistory.ReadMessages(roomContext.Room.OwnerUserID, roomContext.Conversation.ID, nil)
	if err != nil {
		t.Fatalf("读取公区历史失败: %v", err)
	}
	publicText := fmt.Sprint(publicMessages)
	if strings.Contains(publicText, "救:Lucy") {
		t.Fatalf("女巫私下回复不应泄漏到公区 feed: %+v", publicMessages)
	}
}

func TestRealtimeServiceRejectsInvalidDirectedMessageRoute(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)

	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatalf("创建 agent service 失败: %v", err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	ctx := authsvc.WithPrincipal(context.Background(), &authsvc.Principal{
		UserID:   "user-room-directed-message-invalid",
		Username: "room-owner",
		Role:     authsvc.RoleOwner,
	})
	amy := createTestAgent(t, agentService, ctx, "Amy")
	devin := createTestAgent(t, agentService, ctx, "Devin")
	roomContext, err := roomService.CreateRoom(ctx, protocol.CreateRoomRequest{
		AgentIDs:               []string{amy.AgentID, devin.AgentID},
		Name:                   "测试 Room",
		PrivateMessagesEnabled: true,
	})
	if err != nil {
		t.Fatalf("创建 room 失败: %v", err)
	}

	service := realtimesvc.NewService(
		cfg,
		roomService,
		agentService,
		runtimectx.NewManager(),
		permissionctx.NewContext(),
	)
	_, err = service.HandleDirectedMessage(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomDirectedMessageRequest{
		SourceAgentID: amy.AgentID,
		Recipients:    []string{"agent-not-member"},
		Content:       "非法目标",
	})
	if err == nil {
		t.Fatal("非 Room 成员不应成为 directed message recipient")
	}

	_, err = service.HandleDirectedMessage(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomDirectedMessageRequest{
		SourceAgentID: amy.AgentID,
		Recipients:    []string{devin.AgentID},
		WakeTargets:   []string{amy.AgentID},
		Content:       "唤醒目标越界",
		WakePolicy:    protocol.RoomWakePolicyImmediate,
	})
	if err == nil || !strings.Contains(err.Error(), "subset of recipients") {
		t.Fatalf("wake_targets 必须是 recipients 子集: %v", err)
	}

	_, err = service.HandleDirectedMessage(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomDirectedMessageRequest{
		SourceAgentID: amy.AgentID,
		Recipients:    []string{devin.AgentID},
		WakeTargets:   []string{devin.AgentID},
		Content:       "只记录却指定唤醒目标",
		WakePolicy:    protocol.RoomWakePolicyNone,
	})
	if err == nil || !strings.Contains(err.Error(), "wake_targets") {
		t.Fatalf("wake_policy=none 不应接受 wake_targets: %v", err)
	}

	_, err = service.HandleDirectedMessage(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomDirectedMessageRequest{
		SourceAgentID: amy.AgentID,
		Recipients:    []string{devin.AgentID},
		Content:       "非法 reply route",
		ReplyRoute: protocol.RoomReplyRoute{
			Mode: protocol.RoomReplyRoutePrivate,
		},
	})
	if err == nil || !strings.Contains(err.Error(), "reply_route private requires recipients") {
		t.Fatalf("reply_route=private 缺少 recipients 错误不正确: %v", err)
	}

	_, err = service.HandleDirectedMessage(ctx, roomContext.Room.ID, roomContext.Conversation.ID, protocol.CreateRoomDirectedMessageRequest{
		SourceAgentID: amy.AgentID,
		Recipients:    []string{devin.AgentID},
		Content:       "非法 next reply route",
		ReplyRoute: protocol.RoomReplyRoute{
			Mode:       protocol.RoomReplyRoutePrivate,
			Recipients: []string{amy.AgentID},
			WakePolicy: protocol.RoomWakePolicyNone,
			NextReplyRoute: &protocol.RoomReplyRoute{
				Mode: protocol.RoomReplyRoutePublic,
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "next_reply_route requires reply_route private wake_policy=immediate") {
		t.Fatalf("reply_route=private wake=none 不应接受 next_reply_route: %v", err)
	}
}

func waitForAtomicBool(t *testing.T, value *atomic.Bool, message string) {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for !value.Load() {
		select {
		case <-deadline:
			t.Fatal(message)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func roomDirectedMessageContentsContain(messages []protocol.RoomDirectedMessageRecord, content string) bool {
	return slices.ContainsFunc(messages, func(message protocol.RoomDirectedMessageRecord) bool {
		return message.Content == content
	})
}

func roomDirectedMessageContentHasReplyRoute(
	messages []protocol.RoomDirectedMessageRecord,
	content string,
	mode protocol.RoomReplyRouteMode,
) bool {
	return slices.ContainsFunc(messages, func(message protocol.RoomDirectedMessageRecord) bool {
		return message.Content == content && message.ReplyRoute.Mode == mode
	})
}

func waitForRoomDirectedMessageContent(
	t *testing.T,
	store *workspacestore.RoomDirectedMessageStore,
	ownerUserID string,
	conversationID string,
	agentID string,
	content string,
) []protocol.RoomDirectedMessageRecord {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		messages, err := store.ReadContextMessages(ownerUserID, conversationID, agentID)
		if err != nil {
			t.Fatalf("读取 Room directed message 失败: %v", err)
		}
		if roomDirectedMessageContentsContain(messages, content) {
			return messages
		}
		select {
		case <-deadline:
			t.Fatalf("Room directed message 未投影给目标成员: agent=%s content=%q messages=%+v", agentID, content, messages)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

func waitForRoomBroadcastEvent(
	t *testing.T,
	broadcaster *roomDirectedMessageBroadcaster,
	eventType protocol.EventType,
) protocol.EventMessage {
	t.Helper()
	return waitForRoomBroadcastEventMatching(t, broadcaster, eventType, func(protocol.EventMessage) bool {
		return true
	})
}

func waitForRoomBroadcastEventMatching(
	t *testing.T,
	broadcaster *roomDirectedMessageBroadcaster,
	eventType protocol.EventType,
	matches func(protocol.EventMessage) bool,
) protocol.EventMessage {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		for _, event := range broadcaster.Events() {
			if event.EventType == eventType && matches(event) {
				return event
			}
		}
		select {
		case <-deadline:
			t.Fatalf("未广播 Room 事件: %s", eventType)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}
