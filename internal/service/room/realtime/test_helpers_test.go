package realtime_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	serverapp "github.com/nexus-research-lab/nexus/internal/app/server"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/handler/handlertest"
	"github.com/nexus-research-lab/nexus/internal/infra/appfs"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	"github.com/nexus-research-lab/nexus/internal/service/conversation/titlegen"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	realtimesvc "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
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

func mentionCountFromTestValue(value any) int {
	switch typed := value.(type) {
	case []protocol.AgentMention:
		return len(typed)
	case []map[string]any:
		return len(typed)
	case []any:
		return len(typed)
	default:
		return 0
	}
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

func assertRoomStreamBlockIndex(t *testing.T, events []protocol.EventMessage, messageID string, blockType string, expectedIndex int) {
	t.Helper()
	for _, event := range events {
		if event.EventType != protocol.EventTypeStream || event.MessageID != messageID {
			continue
		}
		contentBlock, ok := event.Data["content_block"].(map[string]any)
		if !ok || contentBlock["type"] != blockType {
			continue
		}
		if event.Data["index"] != expectedIndex {
			t.Fatalf("Room %s stream index 不正确: got=%v want=%d event=%+v", blockType, event.Data["index"], expectedIndex, event)
		}
		return
	}
	t.Fatalf("未找到 Room block_type=%s message_id=%s 的 stream 事件: %+v", blockType, messageID, events)
}

func findRoomAssistantMessagePayload(t *testing.T, events []protocol.EventMessage, messageID string) protocol.Message {
	t.Helper()
	for _, event := range events {
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
	agentService, db, err := serverapp.NewAgentService(cfg)
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
