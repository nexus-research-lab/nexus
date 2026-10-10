package permission

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestContextProjectsRoutedRoundStatusAndListsRoomActivityRoutes(t *testing.T) {
	ctx := NewContext()
	broadcaster := newPermissionTestRoomBroadcaster()
	ctx.SetRoomBroadcaster(broadcaster)
	ctx.BindSessionRoute("session-b", RouteContext{
		DispatchSessionKey: "session-b",
		RoomID:             "room-1",
		ConversationID:     "conversation-b",
		AgentID:            "agent-b",
	})
	ctx.BindSessionRoute("session-a", RouteContext{
		DispatchSessionKey: "session-a",
		RoomID:             "room-1",
		ConversationID:     "conversation-a",
		AgentID:            "agent-a",
	})
	ctx.BindSessionRoute("session-other", RouteContext{
		RoomID:         "room-2",
		ConversationID: "conversation-other",
	})

	event := protocol.NewRoundStatusEvent("session-a", "round-a", protocol.RoundStatusRunning, "")
	if errs := ctx.BroadcastEvent(t.Context(), "session-a", event); len(errs) != 0 {
		t.Fatalf("Room 生命周期投影失败: %v", errs)
	}
	if roomID := <-broadcaster.roomIDs; roomID != "room-1" {
		t.Fatalf("Room 投影目标错误: %q", roomID)
	}
	projected := <-broadcaster.events
	if projected.RoomID != "room-1" || projected.ConversationID != "conversation-a" {
		t.Fatalf("Room 生命周期缺少精确 route: %+v", projected)
	}
	if projected.DeliveryMode != protocol.DeliveryModeDurable {
		t.Fatalf("Room 生命周期必须封住订阅竞态: %+v", projected)
	}

	routes := ctx.SessionActivityRoutesForRoom("room-1")
	if len(routes) != 2 || routes[0].SessionKey != "session-a" || routes[1].SessionKey != "session-b" {
		t.Fatalf("Room route 快照必须稳定且隔离: %+v", routes)
	}
}
