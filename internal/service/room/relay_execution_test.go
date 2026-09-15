package room_test

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/app"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRelayExecutionRoomKeepsOneLocalAgentAndStableBinding(t *testing.T) {
	cfg := newRoomTestConfig(t)
	migrateRoomSQLite(t, cfg.DatabaseURL)
	agents, db, err := newRoomTestAgentService(t, cfg)
	if err != nil {
		t.Fatal(err)
	}
	service := app.NewRoomServiceWithDB(cfg, db, agents)
	ctx := t.Context()
	agent := createTestAgent(t, agents, ctx, "在线执行助手")
	first, err := service.EnsureRelayExecutionRoom(ctx, "scope:online-room", agent.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	again, err := service.EnsureRelayExecutionRoom(ctx, "scope:online-room", agent.AgentID)
	if err != nil || first.Room.ID != again.Room.ID || first.Conversation.ID != again.Conversation.ID || first.Room.PrivateMessagesEnabled || first.Room.HostAutoReplyEnabled {
		t.Fatalf("unstable or unsafe room: %+v %v", again, err)
	}
	other, err := service.EnsureRelayExecutionRoom(ctx, "other-scope:online-room", agent.AgentID)
	if err != nil || other.Room.ID == first.Room.ID {
		t.Fatalf("account scopes mixed: %+v %v", other, err)
	}
	second := createTestAgent(t, agents, ctx, "其他助手")
	if _, err = service.AddRoomMember(ctx, first.Room.ID, protocol.AddRoomMemberRequest{AgentID: second.AgentID}); err != nil {
		t.Fatal(err)
	}
	if _, err = service.EnsureRelayExecutionRoom(ctx, "scope:online-room", agent.AgentID); err == nil {
		t.Fatal("must not silently reuse modified execution members")
	}
}
