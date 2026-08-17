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
	registry.mu.Lock()
	remaining := registry.senderTokens[sender.Key()]["agent-1"].refCount
	registry.mu.Unlock()
	if remaining != 1 {
		t.Fatalf("重复订阅引用计数不正确: %d", remaining)
	}

	registry.Unsubscribe(sender, "agent-1")
	registry.mu.Lock()
	_, exists := registry.senderTokens[sender.Key()]
	registry.mu.Unlock()
	if exists {
		t.Fatalf("最后一个引用取消后仍保留 sender token: %+v", registry.senderTokens)
	}
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
