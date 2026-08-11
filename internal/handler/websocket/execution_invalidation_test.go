package websocket

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

func TestExecutionInvalidationRegistryIsolatesOwnerAndSession(t *testing.T) {
	registry := newExecutionInvalidationRegistry()
	ownerSender := &executionInvalidationTestSender{key: "owner"}
	otherOwnerSender := &executionInvalidationTestSender{key: "other-owner"}
	otherSessionSender := &executionInvalidationTestSender{key: "other-session"}
	registry.Bind("owner-a", "session-1", ownerSender)
	registry.Bind("owner-b", "session-1", otherOwnerSender)
	registry.Bind("owner-a", "session-2", otherSessionSender)

	event := protocol.NewExecutionInvalidatedEvent(
		"session-1",
		protocol.ExecutionInvalidationData{ExecutionID: "execution-1", Version: 4},
	)
	registry.Broadcast(context.Background(), "owner-a", "session-1", event)

	if len(ownerSender.events) != 1 ||
		ownerSender.events[0].EventType != protocol.EventTypeExecutionInvalidated {
		t.Fatalf("owner events = %#v", ownerSender.events)
	}
	if len(otherOwnerSender.events) != 0 || len(otherSessionSender.events) != 0 {
		t.Fatalf(
			"cross-scope events: other owner=%#v other session=%#v",
			otherOwnerSender.events,
			otherSessionSender.events,
		)
	}
}

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

func TestExecutionInvalidationFenceRefreshesBoundSessionWithoutGraphIdentity(t *testing.T) {
	sender := &executionInvalidationTestSender{key: "owner"}
	if err := sendExecutionInvalidationFence(
		context.Background(),
		sender,
		"room:group:conversation-1",
	); err != nil {
		t.Fatal(err)
	}

	if len(sender.events) != 1 {
		t.Fatalf("events = %#v", sender.events)
	}
	event := sender.events[0]
	if event.SessionKey != "room:group:conversation-1" ||
		event.Data["execution_id"] != "" ||
		event.Data["version"] != int64(0) {
		t.Fatalf("event = %#v", event)
	}
}

type executionInvalidationTestSender struct {
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
	s.events = append(s.events, event)
	return nil
}
