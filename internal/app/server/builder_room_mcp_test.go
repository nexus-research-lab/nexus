package server

import (
	"context"
	"testing"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type stubRoomMCPService struct{}

func (stubRoomMCPService) HandleDirectedMessage(
	context.Context,
	string,
	string,
	protocol.CreateRoomDirectedMessageRequest,
) (*protocol.RoomDirectedMessageRecord, error) {
	return &protocol.RoomDirectedMessageRecord{}, nil
}

func (stubRoomMCPService) HandlePublicMessage(
	context.Context,
	string,
	string,
	protocol.CreateRoomPublicMessageRequest,
) (protocol.Message, error) {
	return protocol.Message{}, nil
}

func TestRoomMCPBuilderOnlyAddsServerForRoomRuntime(t *testing.T) {
	builder := newRoomMCPBuilder(stubRoomMCPService{}, nil)

	servers := builder("agent-1", protocol.BuildRoomSharedSessionKey("conversation-1"), "room", "room-1", "狼人杀")
	if _, ok := servers["nexus_room"].(sdkmcp.SDKServerConfig); !ok {
		t.Fatalf("Room runtime 应注入 nexus_room SDK server: %+v", servers)
	}

	if dmServers := builder("agent-1", "agent:agent-1:ws:dm:session-1", "agent", "agent-1", "Agent"); len(dmServers) != 0 {
		t.Fatalf("非 Room runtime 不应注入 nexus_room: %+v", dmServers)
	}
}
