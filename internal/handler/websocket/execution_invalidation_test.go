package websocket

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

func TestExecutionInvalidationRegistryFailsClosedAndReleasesSenderScopes(t *testing.T) {
	registry := newExecutionInvalidationRegistry()
	sender := &executionInvalidationTestSender{key: "sender-1"}
	registry.Bind("", "session-1", sender)
	registry.Bind("owner-a", "", sender)
	registry.Bind("owner-a", "session-1", sender)
	registry.Bind("owner-a", "session-2", sender)

	event := protocol.NewExecutionInvalidatedEvent(
		"session-1",
		protocol.ExecutionInvalidationData{ExecutionID: "execution-1"},
	)
	registry.Broadcast(context.Background(), "", "session-1", event)
	registry.Broadcast(context.Background(), "owner-a", "", event)
	if len(sender.events) != 0 {
		t.Fatalf("incomplete scope broadcast events = %#v", sender.events)
	}

	registry.Unbind("owner-a", "session-1", sender)
	registry.Broadcast(context.Background(), "owner-a", "session-1", event)
	if len(sender.events) != 0 {
		t.Fatalf("unbound scope events = %#v", sender.events)
	}

	registry.UnregisterSender(sender)
	registry.Broadcast(context.Background(), "owner-a", "session-2", event)
	if len(sender.events) != 0 || len(registry.byScope) != 0 || len(registry.senderScopes) != 0 {
		t.Fatalf(
			"sender registry leaked: events=%#v by_scope=%#v sender_scopes=%#v",
			sender.events,
			registry.byScope,
			registry.senderScopes,
		)
	}
}

func TestHandlerProjectsExecutionInvalidationPayload(t *testing.T) {
	registry := newExecutionInvalidationRegistry()
	sender := &executionInvalidationTestSender{key: "owner"}
	registry.Bind("owner-a", "session-1", sender)
	handler := &Handler{executionInvalidations: registry}

	handler.InvalidateExecution(context.Background(), orchestrationsvc.ExecutionInvalidation{
		OwnerUserID: "owner-a",
		SessionKey:  "session-1",
		ExecutionID: "execution-1",
		Version:     7,
	})

	if len(sender.events) != 1 {
		t.Fatalf("events = %#v", sender.events)
	}
	event := sender.events[0]
	if event.SessionKey != "session-1" ||
		event.Data["execution_id"] != "execution-1" ||
		event.Data["version"] != int64(7) {
		t.Fatalf("event = %#v", event)
	}
}

func TestExecutionInvalidationRegistryCoalescesBurstsPerScope(t *testing.T) {
	registry := newExecutionInvalidationRegistry()
	registry.window = 40 * time.Millisecond
	sender := &executionInvalidationTestSender{key: "sender-1"}
	other := &executionInvalidationTestSender{key: "sender-2"}
	registry.Bind("owner-a", "session-1", sender)
	registry.Bind("owner-a", "session-2", other)

	for version := int64(1); version <= 20; version++ {
		registry.Invalidate(context.Background(), orchestrationsvc.ExecutionInvalidation{
			OwnerUserID: "owner-a", SessionKey: "session-1", ExecutionID: "execution-1", Version: version,
		})
	}
	registry.Invalidate(context.Background(), orchestrationsvc.ExecutionInvalidation{
		OwnerUserID: "owner-a", SessionKey: "session-2", ExecutionID: "execution-2",
	})
	// 首个失效立即投递，另一个 scope 不受该窗口影响。
	if got := sender.snapshot(); len(got) != 1 || got[0].Data["version"] != int64(1) {
		t.Fatalf("leading events = %#v", got)
	}
	if got := other.snapshot(); len(got) != 1 {
		t.Fatalf("independent scope events = %#v", got)
	}

	deadline := time.Now().Add(2 * time.Second)
	for len(sender.snapshot()) < 2 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	got := sender.snapshot()
	if len(got) != 2 || got[1].Data["version"] != int64(20) {
		t.Fatalf("trailing events = %#v, want one carrying the latest version", got)
	}

	// 空窗口结束后释放 scope，下一次失效重新立即投递。
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		registry.mu.Lock()
		idle := len(registry.throttles) == 0
		registry.mu.Unlock()
		if idle {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	registry.Invalidate(context.Background(), orchestrationsvc.ExecutionInvalidation{
		OwnerUserID: "owner-a", SessionKey: "session-1", ExecutionID: "execution-1", Version: 21,
	})
	if got := sender.snapshot(); len(got) != 3 || got[2].Data["version"] != int64(21) {
		t.Fatalf("post-idle events = %#v", got)
	}
}

type executionInvalidationTestSender struct {
	mu     sync.Mutex
	key    string
	closed bool
	events []protocol.EventMessage
}

func (s *executionInvalidationTestSender) Key() string    { return s.key }
func (s *executionInvalidationTestSender) IsClosed() bool { return s.closed }
func (s *executionInvalidationTestSender) SendEvent(
	_ context.Context,
	event protocol.EventMessage,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
	return nil
}

func (s *executionInvalidationTestSender) snapshot() []protocol.EventMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]protocol.EventMessage(nil), s.events...)
}
