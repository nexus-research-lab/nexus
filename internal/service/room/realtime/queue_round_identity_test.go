package realtime_test

import (
	"context"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/app"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	permissionctx "github.com/nexus-research-lab/nexus/internal/runtime/permission"
	realtimesvc "github.com/nexus-research-lab/nexus/internal/service/room/realtime"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

// Surviving queue claims are consumed when shared history already holds the
// queued round. Host recovery input is internal and never persisted as a user
// message, so that check relies on the coordinator reply carrying the queued
// round ID; lock that contract here.
func TestInternalQueuedRoundReplyCarriesQueuedRoundID(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)
	agentService, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	roomService := app.NewRoomServiceWithDB(cfg, db, agentService)
	agentValue := createTestAgent(t, agentService, context.Background(), "助手甲")
	roomContext, err := roomService.CreateRoom(context.Background(), protocol.CreateRoomRequest{AgentIDs: []string{agentValue.AgentID}, Name: "queued-round-id", Title: "主对话"})
	if err != nil {
		t.Fatal(err)
	}
	client := newFakeRoomClient()
	client.onQuery = func(_ context.Context, _ string) error {
		go sendFakeAssistantResult(client, "assistant-probe", "已核对")
		return nil
	}
	permission := permissionctx.NewContext()
	runtimeManager := runtimectx.NewManager()
	service := realtimesvc.NewServiceWithFactory(cfg, roomService, agentService, runtimeManager, permission, &fakeRoomFactory{clients: []*fakeRoomClient{client}})
	shared := protocol.BuildRoomSharedSessionKey(roomContext.Conversation.ID)
	registerRealtimeServiceCleanup(t, service, runtimeManager, shared)
	sender := newRealtimeTestSender("probe")
	permission.BindSession(shared, sender)
	if err = service.HandleChat(context.Background(), realtimesvc.ChatRequest{
		SessionKey: shared, RoomID: roomContext.Room.ID, ConversationID: roomContext.Conversation.ID,
		Content: "inspect", RoundID: "queue_probe", TargetAgentIDs: []string{agentValue.AgentID}, Internal: true,
	}); err != nil {
		t.Fatal(err)
	}
	_ = collectRoomEventsUntil(t, sender.events, func(_ []protocol.EventMessage, event protocol.EventMessage) bool {
		return event.EventType == protocol.EventTypeRoundStatus && event.Data["round_id"] == "queue_probe" && event.Data["status"] != "running"
	})
	time.Sleep(200 * time.Millisecond)
	messages, err := workspacestore.NewRoomHistoryStore(cfg.WorkspacePath).ReadMessages(roomContext.Room.OwnerUserID, roomContext.Conversation.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range messages {
		if protocol.MessageRole(m) == "assistant" && protocol.MessageRoundID(m) == "queue_probe" {
			return
		}
	}
	t.Fatalf("internal queued round left no reply with its round ID: %+v", messages)
}
