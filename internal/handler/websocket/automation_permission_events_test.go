package websocket

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
)

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
