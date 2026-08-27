package websocket

import (
	"context"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
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

func TestNotifyAutomationPermissionEventProjectsToSessionAndRoom(t *testing.T) {
	permission := permissionctx.NewContext()
	roomSubs := newRoomSubscriptionRegistry(8)
	handler := &Handler{permission: permission, roomSubs: roomSubs}
	sessionSender := newFakeRoomRegistrySender("session-sender")
	roomSender := newFakeRoomRegistrySender("room-sender")
	sessionKey := "agent:agent-recipient:ws:dm:conversation-1"
	permission.BindSession(sessionKey, sessionSender)
	if err := roomSubs.SubscribeRoom(
		context.Background(),
		roomSender,
		"room-1",
		"conversation-1",
		nil,
	); err != nil {
		t.Fatalf("SubscribeRoom() error = %v", err)
	}
	event := protocol.NewEvent(protocol.EventTypePermissionRequest, map[string]any{
		"request_id": "automation-permission-1",
	})
	event.SessionKey = sessionKey
	event.RoomID = "room-1"
	event.ConversationID = "conversation-1"

	handler.NotifyAutomationPermissionEvent(context.Background(), event)

	sessionEvent := readRoomRegistryEvent(t, sessionSender.events)
	if sessionEvent.EventType != protocol.EventTypePermissionRequest ||
		sessionEvent.SessionKey != sessionKey || sessionEvent.RoomSeq != nil {
		t.Fatalf("session event = %+v", sessionEvent)
	}
	roomEvent := readRoomRegistryEvent(t, roomSender.events)
	if roomEvent.EventType != protocol.EventTypePermissionRequest ||
		roomEvent.DeliveryMode != protocol.DeliveryModeDurable ||
		roomEvent.RoomSeq == nil || *roomEvent.RoomSeq != 1 {
		t.Fatalf("room event = %+v", roomEvent)
	}
}

func TestActiveChatActivitySourcesAggregatesExactConversationSessions(t *testing.T) {
	permission := permissionctx.NewContext()
	runtime := runtimectx.NewManager()
	handler := &Handler{permission: permission, runtime: runtime}

	bindRunningRound := func(runtimeSessionKey string, dispatchSessionKey string, conversationID string, roundID string) {
		t.Helper()
		permission.BindSessionRoute(runtimeSessionKey, permissionctx.RouteContext{
			DispatchSessionKey: dispatchSessionKey,
			RoomID:             "room-1",
			ConversationID:     conversationID,
		})
		if err := runtime.StartRound(context.Background(), runtimeSessionKey, roundID, func() {}); err != nil {
			t.Fatalf("StartRound(%q): %v", runtimeSessionKey, err)
		}
	}

	bindRunningRound("agent:cindy:ws:dm:conversation-a", "agent:cindy:ws:dm:conversation-a", "conversation-a", "round-a")
	bindRunningRound("agent:cindy:ws:dm:conversation-b", "agent:cindy:ws:dm:conversation-b", "conversation-b", "round-b")
	permission.BindSessionRoute("agent:idle:ws:dm:conversation-idle", permissionctx.RouteContext{
		DispatchSessionKey: "agent:idle:ws:dm:conversation-idle",
		RoomID:             "room-1",
		ConversationID:     "conversation-idle",
	})

	sources := handler.activeChatActivitySources("room-1")
	if len(sources) != 2 {
		t.Fatalf("active source count = %d, want 2: %+v", len(sources), sources)
	}
	if sources[0].SessionKey != "agent:cindy:ws:dm:conversation-a" ||
		sources[0].ConversationID != "conversation-a" ||
		len(sources[0].RunningRoundIDs) != 1 || sources[0].RunningRoundIDs[0] != "round-a" {
		t.Fatalf("first source = %+v", sources[0])
	}
	if sources[1].SessionKey != "agent:cindy:ws:dm:conversation-b" ||
		sources[1].ConversationID != "conversation-b" ||
		len(sources[1].RunningRoundIDs) != 1 || sources[1].RunningRoundIDs[0] != "round-b" {
		t.Fatalf("second source = %+v", sources[1])
	}
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

func TestRoomSubscriptionRegistryReplaysPendingInteractionAndDeliversResolution(t *testing.T) {
	registry := newRoomSubscriptionRegistry(8)
	ctx := context.Background()
	requestID := "permission-reconnect"
	request := protocol.NewEvent(protocol.EventTypePermissionRequest, map[string]any{
		"request_id": requestID,
	})
	request.ConversationID = "conv-1"
	request.DeliveryMode = protocol.DeliveryModeDurable
	registry.Broadcast(ctx, "room-1", request)

	reconnected := newFakeRoomRegistrySender("sender-reconnected")
	lastSeenRoomSeq := int64(0)
	if err := registry.SubscribeRoom(
		ctx,
		reconnected,
		"room-1",
		"",
		&lastSeenRoomSeq,
	); err != nil {
		t.Fatalf("侧栏重连 subscribe_room 失败: %v", err)
	}
	replayed := readRoomRegistryEvent(t, reconnected.events)
	if replayed.EventType != protocol.EventTypePermissionRequest || replayed.Data["request_id"] != requestID {
		t.Fatalf("重连未重放待确认请求: %+v", replayed)
	}
	if replayed.RoomSeq == nil || *replayed.RoomSeq != 1 {
		t.Fatalf("待确认重放 room_seq 不正确: %+v", replayed)
	}

	resolved := protocol.NewPermissionRequestResolvedEvent("room:group:conv-1", requestID, "answered")
	resolved.ConversationID = "conv-1"
	resolved.DeliveryMode = protocol.DeliveryModeDurable
	registry.Broadcast(ctx, "room-1", resolved)
	receivedResolution := readRoomRegistryEvent(t, reconnected.events)
	if receivedResolution.EventType != protocol.EventTypePermissionRequestResolved ||
		receivedResolution.Data["request_id"] != requestID {
		t.Fatalf("重连后未收到待确认结束事件: %+v", receivedResolution)
	}
	if receivedResolution.RoomSeq == nil || *receivedResolution.RoomSeq != 2 {
		t.Fatalf("待确认结束 room_seq 不正确: %+v", receivedResolution)
	}
}

func TestRoomSubscriptionRegistryReplaysFromSnapshotBoundaryZero(t *testing.T) {
	registry := newRoomSubscriptionRegistry(8)
	ctx := context.Background()
	serverPending := protocol.NewChatAckEvent(
		"room:group:conv-1",
		"",
		"",
		"root-1",
		"",
		false,
		[]protocol.ChatAckPendingSlot{{
			AgentID:      "agent-1",
			AgentRoundID: "agent-round-1",
			MsgID:        "slot-1",
			RoundID:      "root-1",
			Status:       "pending",
			Timestamp:    1,
		}},
	)
	serverPending.DeliveryMode = "durable"
	serverPending.ConversationID = "conv-1"
	registry.Broadcast(ctx, "chat-1", serverPending)

	sender := newFakeRoomRegistrySender("sender-boundary-zero")
	boundary := int64(0)
	if err := registry.SubscribeRoom(
		ctx,
		sender,
		"chat-1",
		"conv-1",
		&boundary,
	); err != nil {
		t.Fatalf("subscribe_room 失败: %v", err)
	}
	event := readRoomRegistryEvent(t, sender.events)
	if event.EventType != protocol.EventTypeChatAck {
		t.Fatalf("replayed event = %s, want chat_ack", event.EventType)
	}
	if event.RoomSeq == nil || *event.RoomSeq != 1 {
		t.Fatalf("replayed room_seq = %#v, want 1", event.RoomSeq)
	}
}

func TestRoomSubscriptionRegistryDoesNotReplayClientAck(t *testing.T) {
	registry := newRoomSubscriptionRegistry(8)
	ctx := context.Background()
	clientAck := protocol.NewChatAckEvent(
		"room:group:conv-1",
		"request-1",
		"client-message-1",
		"root-1",
		"user-message-1",
		true,
		nil,
	)
	clientAck.ConversationID = "conv-1"
	registry.Broadcast(ctx, "chat-1", clientAck)

	sender := newFakeRoomRegistrySender("sender-client-ack")
	boundary := int64(0)
	if err := registry.SubscribeRoom(
		ctx,
		sender,
		"chat-1",
		"conv-1",
		&boundary,
	); err != nil {
		t.Fatalf("subscribe_room 失败: %v", err)
	}
	assertNoRoomRegistryEvent(t, sender.events)
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

func TestRoomSubscriptionRegistryKeepsGlobalAndConversationSubscriptions(t *testing.T) {
	registry := newRoomSubscriptionRegistry(8)
	ctx := context.Background()
	sender := newFakeRoomRegistrySender("sender-global")

	if err := registry.SubscribeRoom(ctx, sender, "chat-1", "", nil); err != nil {
		t.Fatalf("全房间 subscribe_room 失败: %v", err)
	}
	if err := registry.SubscribeRoom(ctx, sender, "chat-1", "conv-1", nil); err != nil {
		t.Fatalf("会话 subscribe_room 失败: %v", err)
	}

	registry.Broadcast(ctx, "chat-1", durableRoomTestEvent(protocol.EventTypeMessage, "conv-2"))
	event := readRoomRegistryEvent(t, sender.events)
	if event.ConversationID != "conv-2" {
		t.Fatalf("全房间订阅未收到其他会话事件: %+v", event)
	}
	assertNoRoomRegistryEvent(t, sender.events)

	registry.UnsubscribeRoom(sender, "chat-1", "conv-1")
	registry.Broadcast(ctx, "chat-1", durableRoomTestEvent(protocol.EventTypeMessage, "conv-2"))
	event = readRoomRegistryEvent(t, sender.events)
	if event.ConversationID != "conv-2" {
		t.Fatalf("移除会话订阅后全房间订阅应继续生效: %+v", event)
	}

	registry.UnsubscribeRoom(sender, "chat-1", "")
	registry.Broadcast(ctx, "chat-1", durableRoomTestEvent(protocol.EventTypeMessage, "conv-2"))
	assertNoRoomRegistryEvent(t, sender.events)
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

func assertNoRoomRegistryEvent(t *testing.T, events <-chan protocol.EventMessage) {
	t.Helper()
	select {
	case event := <-events:
		t.Fatalf("不应收到 Room registry 事件: %+v", event)
	case <-time.After(80 * time.Millisecond):
	}
}
