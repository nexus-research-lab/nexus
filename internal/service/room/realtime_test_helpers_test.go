package room_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/service/conversation/titlegen"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	_ "modernc.org/sqlite"
)

var NewRealtimeServiceWithFactory = roomsvc.NewRealtimeServiceWithFactory

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

var roomTranscriptSanitizePattern = regexp.MustCompile(`[^a-zA-Z0-9]`)

func readRoomPrivateHistory(
	t *testing.T,
	root string,
	workspacePath string,
	sessionKey string,
	agentID string,
	sessionID string,
) []protocol.Message {
	t.Helper()
	historyStore := workspacestore.NewAgentHistoryStore(root)
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
	workspacePath string,
	sessionID string,
	rows []map[string]any,
) {
	t.Helper()
	if strings.TrimSpace(sessionID) == "" {
		t.Fatal("session_id 为空，无法写入 room transcript fixture")
	}
	projectDir := filepath.Join(
		os.Getenv("NEXUS_CONFIG_DIR"),
		"projects",
		sanitizeRoomTranscriptPath(canonicalizeRoomTranscriptPath(workspacePath)),
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

func canonicalizeRoomTranscriptPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	absolutePath, err := filepath.Abs(path)
	if err == nil {
		path = absolutePath
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	return path
}

func sanitizeRoomTranscriptPath(path string) string {
	const maxLength = 200
	sanitized := roomTranscriptSanitizePattern.ReplaceAllString(path, "-")
	if len(sanitized) <= maxLength {
		return sanitized
	}
	return sanitized[:maxLength] + "-" + roomTranscriptHash(path)
}

func roomTranscriptHash(value string) string {
	var hash int32
	for _, character := range value {
		hash = hash*31 + int32(character)
	}

	number := int64(hash)
	if number < 0 {
		number = -number
	}
	if number == 0 {
		return "0"
	}

	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	result := make([]byte, 0, 8)
	for number > 0 {
		result = append(result, digits[number%36])
		number /= 36
	}
	for left, right := 0, len(result)-1; left < right; left, right = left+1, right-1 {
		result[left], result[right] = result[right], result[left]
	}
	return string(result)
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
