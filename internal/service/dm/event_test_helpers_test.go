package dm

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	"github.com/nexus-research-lab/nexus/internal/service/conversation/titlegen"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
)

type dmTestSender struct {
	key    string
	events chan protocol.EventMessage
}

func newDMTestSender(key string) *dmTestSender {
	return &dmTestSender{
		key:    key,
		events: make(chan protocol.EventMessage, 32),
	}
}

func (s *dmTestSender) Key() string { return s.key }

func (s *dmTestSender) IsClosed() bool { return false }

func (s *dmTestSender) SendEvent(_ context.Context, event protocol.EventMessage) error {
	s.events <- event
	return nil
}

type fakeDMTitleScheduler struct {
	mu       sync.Mutex
	requests []titlegen.Request
}

func (s *fakeDMTitleScheduler) Schedule(_ context.Context, request titlegen.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, request)
}

func (s *fakeDMTitleScheduler) LastRequest() titlegen.Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.requests) == 0 {
		return titlegen.Request{}
	}
	return s.requests[len(s.requests)-1]
}

type fakeDMPreferencesService struct {
	prefs preferencessvc.Preferences
}

func (s fakeDMPreferencesService) Get(_ context.Context, _ string) (preferencessvc.Preferences, error) {
	return s.prefs, nil
}

type blockingDMTestSender struct {
	key  string
	done chan struct{}
	once sync.Once
}

func (s *blockingDMTestSender) Key() string { return s.key }

func (s *blockingDMTestSender) IsClosed() bool { return false }

func (s *blockingDMTestSender) SendEvent(ctx context.Context, _ protocol.EventMessage) error {
	<-ctx.Done()
	s.once.Do(func() {
		close(s.done)
	})
	return ctx.Err()
}

func waitForEvent(t *testing.T, events <-chan protocol.EventMessage, eventType protocol.EventType, status string) {
	t.Helper()
	_ = collectEventsUntil(t, events, func(event protocol.EventMessage) bool {
		if event.EventType != eventType {
			return false
		}
		if status == "" {
			return true
		}
		return event.Data["status"] == status
	})
}

func collectEventsUntil(
	t *testing.T,
	events <-chan protocol.EventMessage,
	stop func(protocol.EventMessage) bool,
) []protocol.EventMessage {
	t.Helper()
	result := make([]protocol.EventMessage, 0, 8)
	timeout := time.After(3 * time.Second)
	for {
		select {
		case event := <-events:
			result = append(result, event)
			if stop(event) {
				return result
			}
		case <-timeout:
			t.Fatalf("等待事件超时，当前事件: %+v", result)
		}
	}
}

func waitForDMRuntimeIdle(t *testing.T, runtimeManager *runtimectx.Manager, sessionKey string) {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		if len(runtimeManager.GetRunningRoundIDs(sessionKey)) == 0 {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("等待 DM round 结束超时，running_rounds=%+v", runtimeManager.GetRunningRoundIDs(sessionKey))
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func assertEventTypes(t *testing.T, events []protocol.EventMessage, expected []protocol.EventType) {
	t.Helper()
	if len(events) < len(expected) {
		t.Fatalf("事件数量不足: got=%d want>=%d", len(events), len(expected))
	}
	for index, eventType := range expected {
		if events[index].EventType != eventType {
			t.Fatalf("第 %d 个事件类型不正确: got=%s want=%s all=%+v", index, events[index].EventType, eventType, events)
		}
	}
}

func assertContainsRoundStatus(t *testing.T, events []protocol.EventMessage, status string) {
	t.Helper()
	for _, event := range events {
		if event.EventType == protocol.EventTypeRoundStatus && event.Data["status"] == status {
			return
		}
	}
	t.Fatalf("未找到 round_status=%s: %+v", status, events)
}

func assertContainsStreamEventType(t *testing.T, events []protocol.EventMessage, streamType string) {
	t.Helper()
	for _, event := range events {
		if event.EventType == protocol.EventTypeStream && event.Data["type"] == streamType {
			return
		}
	}
	t.Fatalf("未找到 stream.type=%s: %+v", streamType, events)
}

func assertContainsResultSubtype(t *testing.T, events []protocol.EventMessage, subtype string) {
	t.Helper()
	for _, event := range events {
		if event.EventType != protocol.EventTypeMessage {
			continue
		}
		if event.Data["role"] == "result" && event.Data["subtype"] == subtype {
			return
		}
		if event.Data["role"] == "assistant" {
			summary, ok := event.Data["result_summary"].(map[string]any)
			if ok && summary["subtype"] == subtype {
				return
			}
		}
	}
	t.Fatalf("未找到 result.subtype=%s: %+v", subtype, events)
}

func assertContainsErrorEventForMessage(t *testing.T, events []protocol.EventMessage, messageID string) {
	t.Helper()
	for _, event := range events {
		if event.EventType == protocol.EventTypeError && event.MessageID == messageID {
			return
		}
	}
	t.Fatalf("未找到绑定消息 %s 的 error 事件: %+v", messageID, events)
}

func assertStreamBlockIndex(t *testing.T, events []protocol.EventMessage, blockType string, expectedIndex int) {
	t.Helper()
	for _, event := range events {
		if event.EventType != protocol.EventTypeStream {
			continue
		}
		contentBlock, ok := event.Data["content_block"].(map[string]any)
		if !ok || contentBlock["type"] != blockType {
			continue
		}
		if event.Data["index"] != expectedIndex {
			t.Fatalf("%s stream index 不正确: got=%v want=%d event=%+v", blockType, event.Data["index"], expectedIndex, event)
		}
		return
	}
	t.Fatalf("未找到 block_type=%s 的 stream 事件: %+v", blockType, events)
}

func findAssistantMessagePayload(t *testing.T, events []protocol.EventMessage, messageID string) protocol.Message {
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
	t.Fatalf("未找到 assistant message_id=%s 的 durable 消息: %+v", messageID, events)
	return nil
}

func findLatestAssistantMessagePayload(t *testing.T, events []protocol.EventMessage, messageID string) protocol.Message {
	t.Helper()
	var latest protocol.Message
	for _, event := range events {
		if event.EventType != protocol.EventTypeMessage || event.MessageID != messageID {
			continue
		}
		if event.Data["role"] != "assistant" {
			continue
		}
		latest = protocol.Message(event.Data)
	}
	if latest != nil {
		return latest
	}
	t.Fatalf("未找到 assistant message_id=%s 的最后 durable 消息: %+v", messageID, events)
	return nil
}

func contentBlocksFromPayload(t *testing.T, payload map[string]any) []map[string]any {
	t.Helper()
	rawBlocks, ok := payload["content"]
	if !ok {
		t.Fatalf("消息缺少 content: %+v", payload)
	}
	switch typed := rawBlocks.(type) {
	case []map[string]any:
		return typed
	case []any:
		result := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			block, ok := item.(map[string]any)
			if !ok {
				t.Fatalf("content block 类型不正确: %+v", payload)
			}
			result = append(result, block)
		}
		return result
	default:
		t.Fatalf("content 类型不正确: %+v", payload)
		return nil
	}
}

func assertContentBlockTypes(t *testing.T, blocks []map[string]any, expected []string) {
	t.Helper()
	if len(blocks) != len(expected) {
		t.Fatalf("content block 数量不正确: got=%d want=%d blocks=%+v", len(blocks), len(expected), blocks)
	}
	for index, expectedType := range expected {
		if blocks[index]["type"] != expectedType {
			t.Fatalf("第 %d 个 content block 类型不正确: got=%v want=%s blocks=%+v", index, blocks[index]["type"], expectedType, blocks)
		}
	}
}

func assertToolResultIDs(t *testing.T, blocks []map[string]any, expected []string) {
	t.Helper()
	resultIDs := make([]string, 0, len(expected))
	for _, block := range blocks {
		if block["type"] != "tool_result" {
			continue
		}
		resultIDs = append(resultIDs, anyToString(block["tool_use_id"]))
	}
	if len(resultIDs) != len(expected) {
		t.Fatalf("tool_result 数量不正确: got=%+v want=%+v blocks=%+v", resultIDs, expected, blocks)
	}
	for index, expectedID := range expected {
		if resultIDs[index] != expectedID {
			t.Fatalf("tool_result 顺序不正确: got=%+v want=%+v blocks=%+v", resultIDs, expected, blocks)
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
