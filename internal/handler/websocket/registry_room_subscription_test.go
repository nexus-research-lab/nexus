package websocket

import (
	"context"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type fakeRoomRegistrySender struct {
	key    string
	events chan protocol.EventMessage
}

func newFakeRoomRegistrySender(key string) *fakeRoomRegistrySender {
	return &fakeRoomRegistrySender{
		key:    key,
		events: make(chan protocol.EventMessage, 16),
	}
}

func (s *fakeRoomRegistrySender) Key() string    { return s.key }
func (s *fakeRoomRegistrySender) IsClosed() bool { return false }
func (s *fakeRoomRegistrySender) SendEvent(_ context.Context, event protocol.EventMessage) error {
	s.events <- event
	return nil
}

func TestRoomSubscriptionRegistryReplaysDurableEvents(t *testing.T) {
	registry := newRoomSubscriptionRegistry(8)
	ctx := context.Background()

	senderA := newFakeRoomRegistrySender("sender-a")
	if err := registry.SubscribeRoom(ctx, senderA, "chat-1", "conv-1", nil); err != nil {
		t.Fatalf("首次 subscribe_room 失败: %v", err)
	}
	registry.Broadcast(ctx, "chat-1", durableRoomTestEvent(protocol.EventTypeMessage, "conv-1"))
	registry.Broadcast(ctx, "chat-1", durableRoomTestEvent(protocol.EventTypeRoundStatus, "conv-1"))

	senderB := newFakeRoomRegistrySender("sender-b")
	lastSeenRoomSeq := int64(1)
	if err := registry.SubscribeRoom(ctx, senderB, "chat-1", "conv-1", &lastSeenRoomSeq); err != nil {
		t.Fatalf("重连 subscribe_room 失败: %v", err)
	}

	event := readRoomRegistryEvent(t, senderB.events)
	if event.EventType != protocol.EventTypeRoundStatus {
		t.Fatalf("回放事件类型不正确: %+v", event)
	}
	if event.RoomSeq == nil || *event.RoomSeq != 2 {
		t.Fatalf("回放 room_seq 不正确: %+v", event)
	}
}

func TestRoomSubscriptionRegistryRequestsResyncWhenReplayBufferMissed(t *testing.T) {
	registry := newRoomSubscriptionRegistry(1)
	ctx := context.Background()

	registry.Broadcast(ctx, "chat-1", durableRoomTestEvent(protocol.EventTypeMessage, "conv-1"))
	registry.Broadcast(ctx, "chat-1", durableRoomTestEvent(protocol.EventTypeRoundStatus, "conv-1"))
	registry.Broadcast(ctx, "chat-1", durableRoomTestEvent(protocol.EventTypeMessage, "conv-1"))

	sender := newFakeRoomRegistrySender("sender-c")
	lastSeenRoomSeq := int64(1)
	if err := registry.SubscribeRoom(ctx, sender, "chat-1", "conv-1", &lastSeenRoomSeq); err != nil {
		t.Fatalf("subscribe_room 失败: %v", err)
	}

	event := readRoomRegistryEvent(t, sender.events)
	if event.EventType != protocol.EventTypeRoomResyncRequired {
		t.Fatalf("期望 room_resync_required，实际: %+v", event)
	}
	if event.Data["latest_room_seq"] != int64(3) && event.Data["latest_room_seq"] != float64(3) {
		t.Fatalf("latest_room_seq 不正确: %+v", event.Data)
	}
	if event.Data["buffer_start_room_seq"] != int64(3) && event.Data["buffer_start_room_seq"] != float64(3) {
		t.Fatalf("buffer_start_room_seq 不正确: %+v", event.Data)
	}
}

func durableRoomTestEvent(eventType protocol.EventType, conversationID string) protocol.EventMessage {
	event := protocol.NewEvent(eventType, map[string]any{
		"conversation_id": conversationID,
	})
	event.DeliveryMode = "durable"
	event.ConversationID = conversationID
	return event
}

func readRoomRegistryEvent(t *testing.T, events <-chan protocol.EventMessage) protocol.EventMessage {
	t.Helper()
	select {
	case event := <-events:
		return event
	case <-time.After(2 * time.Second):
		t.Fatal("等待 Room registry 事件超时")
		return protocol.EventMessage{}
	}
}
