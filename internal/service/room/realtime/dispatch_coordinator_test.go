package realtime

import (
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRoomDispatchRegistryUsesConversationBoundary(t *testing.T) {
	var registry roomDispatchRegistry
	first := registry.acquire(roomDispatchKey("room:shared:conversation-a", "conversation-a"))

	sameWaiting := make(chan struct{})
	sameAcquired := make(chan struct{})
	sameDone := make(chan struct{})
	go func() {
		close(sameWaiting)
		lease := registry.acquire(roomDispatchKey("room:agent:conversation-a", "conversation-a"))
		close(sameAcquired)
		lease.Unlock()
		close(sameDone)
	}()
	<-sameWaiting

	select {
	case <-sameAcquired:
		t.Fatal("同一 conversation 的 dispatch 不应并发通过")
	default:
	}

	differentAcquired := make(chan struct{})
	differentDone := make(chan struct{})
	go func() {
		lease := registry.acquire(roomDispatchKey("room:shared:conversation-b", "conversation-b"))
		close(differentAcquired)
		lease.Unlock()
		close(differentDone)
	}()
	select {
	case <-differentAcquired:
	case <-time.After(time.Second):
		t.Fatal("不同 conversation 不应被同一把 dispatch 锁阻塞")
	}
	<-differentDone

	first.Unlock()
	select {
	case <-sameDone:
	case <-time.After(time.Second):
		t.Fatal("释放 conversation dispatch 后等待者未获得闸门")
	}

	registry.mu.Lock()
	defer registry.mu.Unlock()
	if len(registry.entries) != 0 {
		t.Fatalf("dispatch lease 释放后仍残留 entry: %d", len(registry.entries))
	}
}

func TestRoomDispatchKeyNormalizesSharedAndAgentSession(t *testing.T) {
	conversationID := "conversation-dispatch-key"
	shared := protocol.BuildRoomSharedSessionKey(conversationID)
	agent := protocol.BuildRoomAgentSessionKey(conversationID, "agent-a", protocol.RoomTypeGroup)
	if got, want := roomDispatchKey(shared, ""), roomDispatchKey(agent, ""); got != want {
		t.Fatalf("shared/agent session dispatch keys differ: got=%q want=%q", got, want)
	}
	dmAgent := protocol.BuildRoomAgentSessionKey("dm-ref", "agent-a", "dm")
	if got := roomConversationIDFromSessionKey(dmAgent); got != "" {
		t.Fatalf("DM agent session was treated as Room conversation: %q", got)
	}
}
