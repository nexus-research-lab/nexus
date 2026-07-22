package room

import "testing"

func TestWrapRoundStatusErrorEventCarriesRoomIdentity(t *testing.T) {
	event := WrapRoundStatusErrorEvent(
		"room:group:conversation-1",
		"room-1",
		"conversation-1",
		"round-1",
		"provider unavailable",
	)

	if event.Data["status"] != "error" || event.Data["message"] != "provider unavailable" {
		t.Fatalf("error round status data = %#v", event.Data)
	}
	if event.DeliveryMode != "durable" || event.RoomID != "room-1" || event.ConversationID != "conversation-1" {
		t.Fatalf("error round status identity = %+v", event)
	}
}
