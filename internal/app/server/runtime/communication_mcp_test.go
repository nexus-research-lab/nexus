package runtime

import (
	"context"
	nexusmcp "github.com/nexus-research-lab/nexus/internal/mcp"
	"slices"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
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
	builder := NewCommunicationToolBuilder(
		svc, nil, stubRuntimeAgentResolver{record: worker}, nil,
	)
	sessionKey := protocol.BuildAgentSessionKey(
		worker.AgentID, protocol.SessionChannelWebSocketSegment, protocol.RoomTypeDM, "main", "",
	)
	ctx := runtimectx.WithRuntimeRoundLease(ownerContext, sessionKey, "round-worker")
	tools := builder(ctx, nexusmcp.RoundContext{
		SessionKey: sessionKey, RoundID: "round-worker", SourceContextType: "agent",
		SourceContextID: worker.AgentID, CommandContext: runtimectx.RuntimeCommandContext{Agent: worker},
	})
	if len(tools) == 0 {
		t.Fatal("ordinary Agent must receive Nexus communication tools")
	}
	if got := toolDefinitionNames(tools); !slices.Equal(got, []string{"list_targets", "send_message"}) {
		t.Fatalf("communication tools = %+v", got)
	}

	main := &protocol.Agent{AgentID: "nexus", OwnerUserID: "owner", IsMain: true}
	mainBuilder := NewCommunicationToolBuilder(
		svc, nil, stubRuntimeAgentResolver{record: main}, nil,
	)
	mainSession := protocol.BuildAgentSessionKey(
		main.AgentID, protocol.SessionChannelWebSocketSegment, protocol.RoomTypeDM, "main", "",
	)
	mainContext := runtimectx.WithRuntimeRoundLease(ownerContext, mainSession, "round-main")
	if got := mainBuilder(mainContext, nexusmcp.RoundContext{
		SessionKey: mainSession, RoundID: "round-main", SourceContextType: "agent",
		SourceContextID: main.AgentID, CommandContext: runtimectx.RuntimeCommandContext{Agent: main},
	}); len(got) != 0 {
		t.Fatalf("main Agent must not receive communication tools: %+v", got)
	}
	if got := builder(ctx, nexusmcp.RoundContext{
		SessionKey: sessionKey, RoundID: "round-worker", SourceContextType: "agent_external",
		SourceContextID: worker.AgentID, CommandContext: runtimectx.RuntimeCommandContext{Agent: worker},
	}); len(got) == 0 {
		t.Fatal("external Agent runtime must receive communication tools")
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
	if got := builder(roomContext, nexusmcp.RoundContext{
		SessionKey: roomSession, RoundID: "root-round-room", SourceContextType: "room_untrusted",
		SourceContextID: "room-1", CommandContext: runtimectx.RuntimeCommandContext{Agent: worker},
	}); len(got) == 0 {
		t.Fatal("Room handoff runtime must receive communication tools")
	}
}

func toolDefinitionNames(tools []sdktool.Tool) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		names = append(names, tool.Name)
	}
	return names
}

type capturingRoomCommunicationService struct {
	request protocol.CreateRoomDirectedMessageRequest
}

func (s *capturingRoomCommunicationService) HandleDirectedMessage(
	_ context.Context,
	_ string,
	_ string,
	request protocol.CreateRoomDirectedMessageRequest,
) (*protocol.RoomDirectedMessageRecord, error) {
	s.request = request
	return &protocol.RoomDirectedMessageRecord{MessageID: "message-1"}, nil
}

func (*capturingRoomCommunicationService) HandlePublicMessage(
	context.Context,
	string,
	string,
	protocol.CreateRoomPublicMessageRequest,
) (protocol.Message, error) {
	return protocol.Message{"message_id": "public-1"}, nil
}

func (*capturingRoomCommunicationService) MarkPublicMessagePublished(
	context.Context,
	string,
	string,
	string,
) error {
	return nil
}

func TestCommunicationMCPBuilderInjectsCurrentRoomIntoSameSendTool(t *testing.T) {
	communicationService := communicationsvc.NewService(nil, nil, nil, nil)
	roomService := &capturingRoomCommunicationService{}
	agentValue := &protocol.Agent{AgentID: "agent-1", OwnerUserID: "user-1"}
	builder := NewCommunicationToolBuilder(
		communicationService,
		roomService,
		stubRuntimeAgentResolver{record: agentValue},
		func(context.Context, string) (*protocol.RoomAggregate, error) {
			return &protocol.RoomAggregate{Room: protocol.RoomRecord{}}, nil
		},
	)
	runtimeContext := runtimectx.WithRuntimeRoundLease(
		authctx.WithPrincipal(context.Background(), &authctx.Principal{UserID: "user-1"}),
		protocol.BuildRoomAgentSessionKey("conversation-1", "agent-1", protocol.RoomTypeGroup),
		"agent-round-1",
	)
	tools := builder(runtimeContext, nexusmcp.RoundContext{
		SessionKey: protocol.BuildRoomSharedSessionKey("conversation-1"),
		RoundID:    "root-round-1", SourceContextType: "room", SourceContextID: "room-1",
		CommandContext: runtimectx.RuntimeCommandContext{Agent: agentValue},
	})
	if got := toolDefinitionNames(tools); !slices.Equal(got, []string{"list_targets", "send_message"}) {
		t.Fatalf("Room communication tools = %v", got)
	}
	server := sdktool.NewSimpleSDKMCPServer(nexusMCPServerName, "1.0.0", tools)
	if _, err := server.HandleMessage(context.Background(), map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "send_message",
			"arguments": map[string]any{
				"destination": "current_room",
				"visibility":  "private",
				"recipients":  []any{"agent-2"},
				"content":     "查验谁？",
			},
		},
	}); err != nil {
		t.Fatalf("tools/call 失败: %v", err)
	}
	if roomService.request.SourceAgentRoundID != "agent-round-1" {
		t.Fatalf("physical agent round = %q", roomService.request.SourceAgentRoundID)
	}
}
