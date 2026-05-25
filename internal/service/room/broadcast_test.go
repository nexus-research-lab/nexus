package room

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
)

type recordingRoomBroadcaster struct {
	events []protocol.EventMessage
}

func (b *recordingRoomBroadcaster) Broadcast(_ context.Context, _ string, event protocol.EventMessage) []error {
	b.events = append(b.events, event)
	return nil
}

type recordingPermissionSender struct {
	events []protocol.EventMessage
}

func (s *recordingPermissionSender) Key() string {
	return "permission-sender"
}

func (s *recordingPermissionSender) IsClosed() bool {
	return false
}

func (s *recordingPermissionSender) SendEvent(_ context.Context, event protocol.EventMessage) error {
	s.events = append(s.events, event)
	return nil
}

func TestBroadcastSharedEventMirrorsToRoundObserverWhenBroadcasterIsConfigured(t *testing.T) {
	service := &RealtimeService{
		permission:   permissionctx.NewContext(),
		activeRounds: make(map[string]*activeRoomRound),
	}
	broadcaster := &recordingRoomBroadcaster{}
	service.SetRoomBroadcaster(broadcaster)

	sessionKey := protocol.BuildRoomSharedSessionKey("conversation-1")
	observed := make(chan protocol.EventMessage, 1)
	service.registerRound(&activeRoomRound{
		SessionKey:     sessionKey,
		RoomID:         "room-1",
		ConversationID: "conversation-1",
		RoundID:        "round-1",
		EventObserver: func(_ context.Context, event protocol.EventMessage) {
			observed <- event
		},
		Slots: map[string]*activeRoomSlot{},
		Done:  make(chan struct{}),
	})

	event := protocol.NewRoundStatusEvent(sessionKey, "round-1", "finished", "success")
	service.broadcastSharedEvent(context.Background(), sessionKey, "room-1", event)

	if len(broadcaster.events) != 1 {
		t.Fatalf("Room broadcaster 应收到事件，实际 %d", len(broadcaster.events))
	}
	select {
	case mirrored := <-observed:
		if mirrored.EventType != protocol.EventTypeRoundStatus || mirrored.SessionKey != sessionKey {
			t.Fatalf("观察器收到事件不正确: %+v", mirrored)
		}
	default:
		t.Fatal("配置 broadcaster 时也应镜像事件给内部观察器")
	}
}

func TestBroadcastSharedEventDoesNotDuplicateObserverWhenUsingPermissionBroadcast(t *testing.T) {
	permission := permissionctx.NewContext()
	service := &RealtimeService{
		permission:   permission,
		activeRounds: make(map[string]*activeRoomRound),
	}

	sessionKey := protocol.BuildRoomSharedSessionKey("conversation-1")
	sender := &recordingPermissionSender{}
	permission.BindSession(sessionKey, sender, "client-1", false)

	observed := make(chan protocol.EventMessage, 1)
	service.registerRound(&activeRoomRound{
		SessionKey:     sessionKey,
		RoomID:         "room-1",
		ConversationID: "conversation-1",
		RoundID:        "round-1",
		EventObserver: func(_ context.Context, event protocol.EventMessage) {
			observed <- event
		},
		Slots: map[string]*activeRoomSlot{},
		Done:  make(chan struct{}),
	})

	event := protocol.NewRoundStatusEvent(sessionKey, "round-1", "finished", "success")
	service.broadcastSharedEvent(context.Background(), sessionKey, "", event)

	if len(sender.events) != 1 {
		t.Fatalf("permission sender 应收到一次事件，实际 %d", len(sender.events))
	}
	select {
	case mirrored := <-observed:
		t.Fatalf("permission 广播路径不应额外调用内部观察器: %+v", mirrored)
	default:
	}
}
