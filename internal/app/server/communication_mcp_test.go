package server

import (
	"context"
	"slices"
	"strings"
	"testing"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	communicationmcp "github.com/nexus-research-lab/nexus/internal/mcp/communication"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	communicationsvc "github.com/nexus-research-lab/nexus/internal/service/communication"
)

func TestCommunicationMCPBuilderInjectsForEveryLiveOrdinaryAgentContext(t *testing.T) {
	svc := communicationsvc.NewService(nil, nil, nil, nil)
	ownerContext := authctx.WithPrincipal(context.Background(), &authctx.Principal{
		UserID: "owner", Role: authctx.RoleOwner, AuthMethod: authctx.AuthMethodPassword,
	})
	worker := &protocol.Agent{AgentID: "worker", OwnerUserID: "owner"}
	builder := newCommunicationMCPBuilder(svc, stubRuntimeAgentResolver{record: worker})
	sessionKey := protocol.BuildAgentSessionKey(
		worker.AgentID, protocol.SessionChannelWebSocketSegment, protocol.RoomTypeDM, "main", "",
	)
	ctx := runtimectx.WithRuntimeRoundLease(ownerContext, sessionKey, "round-worker")
	servers := builder(
		ctx, worker, sessionKey, "round-worker", "agent", worker.AgentID, "", nil,
		sdkpermission.ModeDefault,
	)
	config, ok := servers[communicationmcp.ServerName].(sdkmcp.SDKServerConfig)
	if !ok || config.Instance == nil {
		t.Fatalf("ordinary Agent must receive nexus_comms: %+v", servers)
	}
	if got := communicationToolNames(t, config); !slices.Equal(got, []string{"list_address_book", "send_message"}) {
		t.Fatalf("communication tools = %+v", got)
	}

	main := &protocol.Agent{AgentID: "nexus", OwnerUserID: "owner", IsMain: true}
	mainBuilder := newCommunicationMCPBuilder(svc, stubRuntimeAgentResolver{record: main})
	mainSession := protocol.BuildAgentSessionKey(
		main.AgentID, protocol.SessionChannelWebSocketSegment, protocol.RoomTypeDM, "main", "",
	)
	mainContext := runtimectx.WithRuntimeRoundLease(ownerContext, mainSession, "round-main")
	if got := mainBuilder(
		mainContext, main, mainSession, "round-main", "agent", main.AgentID, "", nil,
		sdkpermission.ModeDefault,
	); len(got) != 0 {
		t.Fatalf("main Agent must not receive nexus_comms: %+v", got)
	}
	if got := builder(
		ctx, worker, sessionKey, "round-worker", "agent_external", worker.AgentID, "", nil,
		sdkpermission.ModeDefault,
	); len(got) == 0 {
		t.Fatal("external Agent runtime must receive nexus_comms")
	}

	roomSession := protocol.BuildRoomSharedSessionKey("conversation-1")
	roomLeaseSession := protocol.BuildRoomAgentSessionKey(
		"conversation-1", worker.AgentID, protocol.RoomTypeGroup,
	)
	roomContext := runtimectx.WithRuntimeRoundLease(ownerContext, roomLeaseSession, "agent-round-room")
	goalAuthority := runtimectx.NewGoalAuthorityState("goal-room", 1, "")
	responsibility := runtimectx.NewResponsibilityAuthorityState(
		goalAuthority,
		"",
		nil,
		nil,
	)
	roomContext = runtimectx.WithResponsibilityAuthorityState(roomContext, responsibility)
	actor, ok := communicationRuntimeActor(
		roomContext,
		stubRuntimeAgentResolver{record: worker},
		worker,
		roomSession,
		"root-round-room",
		"room_untrusted",
		"room-1",
	)
	if !ok || actor.ConversationID != "conversation-1" || actor.GoalCollaborationBinding == nil {
		t.Fatalf("Room communication actor = %+v, ok=%t", actor, ok)
	}
	if binding := actor.GoalCollaborationBinding(); binding == nil ||
		binding.GoalID != "goal-room" || binding.ObjectiveRevision != 1 {
		t.Fatalf("initial Goal collaboration binding = %+v", binding)
	}
	if !responsibility.ApplyGoalMutation(protocol.Goal{
		ID: "goal-room",
		Metadata: map[string]any{
			protocol.GoalMetadataObjectiveRevision: int64(2),
			protocol.GoalMetadataExecutionMode:     string(protocol.GoalExecutionModeGoalOnly),
		},
	}) {
		t.Fatal("retarget responsibility authority")
	}
	if binding := actor.GoalCollaborationBinding(); binding == nil ||
		binding.GoalID != "goal-room" || binding.ObjectiveRevision != 2 {
		t.Fatalf("retargeted Goal collaboration binding = %+v", binding)
	}
	if got := builder(
		roomContext, worker, roomSession, "root-round-room", "room_untrusted", "room-1", "", nil,
		sdkpermission.ModeDefault,
	); len(got) == 0 {
		t.Fatal("Room handoff runtime must receive nexus_comms")
	}
}

func communicationToolNames(t *testing.T, config sdkmcp.SDKServerConfig) []string {
	t.Helper()
	response, err := config.Instance.HandleMessage(t.Context(), map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/list",
	})
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	result, _ := response["result"].(map[string]any)
	tools, _ := result["tools"].([]map[string]any)
	names := make([]string, 0, len(tools))
	for _, item := range tools {
		name, _ := item["name"].(string)
		names = append(names, strings.TrimSpace(name))
	}
	return names
}
