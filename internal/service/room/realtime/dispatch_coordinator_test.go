package realtime

import (
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRoomDispatchStateUsesConversationBoundary(t *testing.T) {
	var registry roomRoundRegistry
	first := registry.acquireDispatch(roomDispatchStateKey("room:shared:conversation-a", "conversation-a"))
	if got := registry.state("conversation-a", false); got != first.state {
		t.Fatal("dispatch lease 未绑定到 conversation state")
	}

	sameWaiting := make(chan struct{})
	sameAcquired := make(chan struct{})
	sameDone := make(chan struct{})
	go func() {
		close(sameWaiting)
		lease := registry.acquireDispatch(roomDispatchStateKey("room:agent:conversation-a", "conversation-a"))
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
		lease := registry.acquireDispatch(roomDispatchStateKey("room:shared:conversation-b", "conversation-b"))
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

	registry.mu.RLock()
	defer registry.mu.RUnlock()
	if len(registry.conversations) != 0 {
		t.Fatalf("dispatch lease 释放后仍残留 conversation state: %d", len(registry.conversations))
	}
}

func TestRoomDispatchStateKeyNormalizesSharedAndAgentSession(t *testing.T) {
	conversationID := "conversation-dispatch-key"
	shared := protocol.BuildRoomSharedSessionKey(conversationID)
	agent := protocol.BuildRoomAgentSessionKey(conversationID, "agent-a", protocol.RoomTypeGroup)
	if got, want := roomDispatchStateKey(shared, ""), roomDispatchStateKey(agent, ""); got != want {
		t.Fatalf("shared/agent session dispatch keys differ: got=%q want=%q", got, want)
	}
	dmAgent := protocol.BuildRoomAgentSessionKey("dm-ref", "agent-a", "dm")
	if got := roomConversationIDFromSessionKey(dmAgent); got != "" {
		t.Fatalf("DM agent session was treated as Room conversation: %q", got)
	}
}

func TestRoomDispatchStateKeepsConversationStateUntilRelease(t *testing.T) {
	registry := newRoomRoundRegistry()
	roundValue := &activeRoomRound{
		ConversationID: "conversation-active",
		SessionKey:     "room:shared:conversation-active",
		RoundID:        "round-1",
	}
	registry.register(roundValue)

	lease := registry.acquireDispatch(roomDispatchStateKey(roundValue.SessionKey, roundValue.ConversationID))
	registry.unregister(roundValue)

	registry.mu.RLock()
	state := registry.conversations[roundValue.ConversationID]
	registry.mu.RUnlock()
	if state == nil {
		t.Fatal("dispatch lease 持有期间不应删除 conversation state")
	}

	lease.Unlock()
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	if registry.conversations[roundValue.ConversationID] != nil {
		t.Fatal("dispatch 释放后应清理空 conversation state")
	}
}

func TestRoomDispatchStateSeparatesUnknownSessions(t *testing.T) {
	registry := newRoomRoundRegistry()
	firstSession := "legacy-session-a"
	secondSession := "legacy-session-b"
	if got := roomConversationIDFromSessionKey(firstSession); got != "" {
		t.Fatalf("测试 session 意外解析出 conversation: %q", got)
	}
	first := registry.acquireDispatch(roomDispatchStateKey(firstSession, ""))
	secondAcquired := make(chan struct{})
	secondDone := make(chan struct{})
	go func() {
		second := registry.acquireDispatch(roomDispatchStateKey(secondSession, ""))
		close(secondAcquired)
		second.Unlock()
		close(secondDone)
	}()

	select {
	case <-secondAcquired:
	case <-time.After(time.Second):
		t.Fatal("无法解析 conversation 的不同 session 不应共享 dispatch 锁")
	}
	first.Unlock()
	select {
	case <-secondDone:
	case <-time.After(time.Second):
		t.Fatal("unknown session dispatch goroutine 未完成")
	}
}
