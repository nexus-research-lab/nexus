package websocket

import (
	"context"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type fakeWorkspaceRegistrySender struct {
	key    string
	events chan protocol.EventMessage
	closed bool
}

func newFakeWorkspaceRegistrySender(key string) *fakeWorkspaceRegistrySender {
	return &fakeWorkspaceRegistrySender{
		key:    key,
		events: make(chan protocol.EventMessage, 16),
	}
}

func (s *fakeWorkspaceRegistrySender) Key() string    { return s.key }
func (s *fakeWorkspaceRegistrySender) IsClosed() bool { return s.closed }
func (s *fakeWorkspaceRegistrySender) SendEvent(_ context.Context, event protocol.EventMessage) error {
	s.events <- event
	return nil
}

func TestWorkspaceSubscriptionRegistryKeepsDuplicateSenderSubscription(t *testing.T) {
	ctx := context.Background()
	snapshot := RuntimeSnapshot{
		AgentID:          "agent-1",
		RunningTaskCount: 1,
		Status:           "running",
	}
	registry := newWorkspaceSubscriptionRegistry(nil, func(string) RuntimeSnapshot {
		return snapshot
	})
	sender := newFakeWorkspaceRegistrySender("sender-1")

	if err := registry.Subscribe(ctx, sender, "agent-1"); err != nil {
		t.Fatalf("首次 subscribe_workspace 失败: %v", err)
	}
	readWorkspaceRegistryEvent(t, sender.events)
	if err := registry.Subscribe(ctx, sender, "agent-1"); err != nil {
		t.Fatalf("重复 subscribe_workspace 失败: %v", err)
	}
	readWorkspaceRegistryEvent(t, sender.events)

	registry.Unsubscribe(sender, "agent-1")
	snapshot.RunningTaskCount = 2
	registry.broadcastRuntimeChanges()

	event := readWorkspaceRegistryEvent(t, sender.events)
	if event.EventType != protocol.EventTypeAgentRuntimeEvent {
		t.Fatalf("事件类型不正确: %+v", event)
	}
	if event.AgentID != "agent-1" {
		t.Fatalf("agent_id 不正确: %+v", event)
	}
	if event.Data["running_task_count"] != 2 {
		t.Fatalf("running_task_count 不正确: %+v", event.Data)
	}

	registry.Unsubscribe(sender, "agent-1")
	snapshot.RunningTaskCount = 3
	registry.broadcastRuntimeChanges()
	assertNoWorkspaceRegistryEvent(t, sender.events)
}

func TestWorkspaceSubscriptionRegistryUnregisterSenderClearsAllReferences(t *testing.T) {
	ctx := context.Background()
	snapshot := RuntimeSnapshot{
		AgentID:          "agent-1",
		RunningTaskCount: 1,
		Status:           "running",
	}
	registry := newWorkspaceSubscriptionRegistry(nil, func(string) RuntimeSnapshot {
		return snapshot
	})
	sender := newFakeWorkspaceRegistrySender("sender-1")

	if err := registry.Subscribe(ctx, sender, "agent-1"); err != nil {
		t.Fatalf("首次 subscribe_workspace 失败: %v", err)
	}
	readWorkspaceRegistryEvent(t, sender.events)
	if err := registry.Subscribe(ctx, sender, "agent-1"); err != nil {
		t.Fatalf("重复 subscribe_workspace 失败: %v", err)
	}
	readWorkspaceRegistryEvent(t, sender.events)

	registry.UnregisterSender(sender)
	snapshot.RunningTaskCount = 2
	registry.broadcastRuntimeChanges()
	assertNoWorkspaceRegistryEvent(t, sender.events)
	if len(registry.senderTokens[sender.Key()]) != 0 {
		t.Fatalf("sender token 未清理: %+v", registry.senderTokens)
	}
}

func readWorkspaceRegistryEvent(t *testing.T, events <-chan protocol.EventMessage) protocol.EventMessage {
	t.Helper()
	select {
	case event := <-events:
		return event
	case <-time.After(2 * time.Second):
		t.Fatal("等待 workspace registry 事件超时")
		return protocol.EventMessage{}
	}
}

func assertNoWorkspaceRegistryEvent(t *testing.T, events <-chan protocol.EventMessage) {
	t.Helper()
	select {
	case event := <-events:
		t.Fatalf("不应收到 workspace registry 事件: %+v", event)
	case <-time.After(80 * time.Millisecond):
	}
}
