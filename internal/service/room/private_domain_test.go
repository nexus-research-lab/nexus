// INPUT: Agent contact channels, ordinary DM filters, and directed-message ledgers.
// OUTPUT: Global history includes incoming/outgoing contact facts independently of room_limit while preserving scope and owner boundaries.
// POS: Private-domain service regression using the real SQL repository and workspace store.
package room_test

import (
	"context"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/app"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
	"github.com/nexus-research-lab/nexus/internal/storage/roomrepo"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
)

func TestPrivateDomainIncludesAgentContactChannelsOutsideDirectoryLimit(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)
	agents, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	service := app.NewRoomServiceWithDB(cfg, db, agents)
	ctx := context.Background()
	a := createTestAgent(t, agents, ctx, "QA")
	b := createTestAgent(t, agents, ctx, "Polish")
	c := createTestAgent(t, agents, ctx, "Other")
	createContact := func(ids []string) *protocol.ConversationContextAggregate {
		t.Helper()
		value, err := service.CreateRoom(ctx, protocol.CreateRoomRequest{AgentIDs: ids, IsContactChannel: true, PrivateMessagesEnabled: true})
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	contact := createContact([]string{a.AgentID, b.AgentID})
	unrelated := createContact([]string{a.AgentID, c.AgentID})
	foreign := createContact([]string{b.AgentID, c.AgentID})
	if _, err = db.ExecContext(ctx, "UPDATE rooms SET owner_user_id = ? WHERE id = ?", "foreign-owner", foreign.Room.ID); err != nil {
		t.Fatal(err)
	}
	store := workspacestore.NewRoomDirectedMessageStore(cfg.WorkspacePath)
	for _, fact := range []struct {
		source, target, id string
		room               *protocol.ConversationContextAggregate
	}{
		{a.AgentID, b.AgentID, "incoming", contact}, {b.AgentID, a.AgentID, "outgoing", contact}, {a.AgentID, c.AgentID, "other", unrelated},
	} {
		if err = store.AppendMessage(fact.room.Room.OwnerUserID, protocol.RoomDirectedMessageRecord{MessageID: fact.id, RoomID: fact.room.Room.ID, ConversationID: fact.room.Conversation.ID, SourceAgentID: fact.source, Recipients: []string{fact.target}, Content: fact.id, Timestamp: 1}); err != nil {
			t.Fatal(err)
		}
	}
	// A newer ordinary room consumes the entire directory budget, but must not hide contact history.
	if _, err = service.CreateRoom(ctx, protocol.CreateRoomRequest{AgentIDs: []string{a.AgentID, c.AgentID}, Name: "Recent ordinary room"}); err != nil {
		t.Fatal(err)
	}
	page, err := service.ListAgentPrivateThreads(ctx, b.AgentID, roomsvc.AgentPrivateDomainQuery{RoomLimit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].MessageCount != 2 {
		t.Fatalf("global threads = %#v", page.Items)
	}
	events, err := service.ListAgentPrivateEvents(ctx, b.AgentID, page.Items[0].ThreadID, roomsvc.AgentPrivateDomainQuery{RoomLimit: 1})
	if err != nil || len(events.Items) != 2 {
		t.Fatalf("events = %#v, %v", events, err)
	}
	if events.Items[0].Direction != "incoming" || events.Items[1].Direction != "outgoing" {
		t.Fatalf("directions = %#v", events.Items)
	}
	scoped, err := service.ListAgentPrivateThreads(ctx, b.AgentID, roomsvc.AgentPrivateDomainQuery{RoomID: unrelated.Room.ID, ConversationID: unrelated.Conversation.ID})
	if err != nil || len(scoped.Items) != 0 {
		t.Fatalf("unrelated scope = %#v, %v", scoped, err)
	}
	rooms, err := service.ListRooms(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range rooms {
		if item.Room.IsContactChannel {
			t.Fatal("contact channel leaked into ordinary directory")
		}
	}
	repository := roomrepo.NewSQLRepository(cfg.DatabaseDriver, db)
	ids, err := repository.ListAgentContactRoomIDs(ctx, contact.Room.OwnerUserID, b.AgentID)
	if err != nil || len(ids) != 1 || ids[0] != contact.Room.ID {
		t.Fatalf("owner/member scope = %v, %v", ids, err)
	}
}
