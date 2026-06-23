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
	builder := newRoomMCPBuilder(stubRoomMCPService{}, nil, nil)

	servers := builder("agent-1", protocol.BuildRoomSharedSessionKey("conversation-1"), "room", "room-1", "狼人杀")
	if _, ok := servers["nexus_room"].(sdkmcp.SDKServerConfig); !ok {
		t.Fatalf("Room runtime 应注入 nexus_room SDK server: %+v", servers)
	}

	if dmServers := builder("agent-1", "agent:agent-1:ws:dm:session-1", "agent", "agent-1", "Agent"); len(dmServers) != 0 {
		t.Fatalf("非 Room runtime 不应注入 nexus_room: %+v", dmServers)
	}
}

func TestRoomMCPBuilderUsesRoomPrivateMessageSetting(t *testing.T) {
	builder := newRoomMCPBuilder(
		stubRoomMCPService{},
		nil,
		func(context.Context, string) (*protocol.RoomAggregate, error) {
			return &protocol.RoomAggregate{
				Room: protocol.RoomRecord{PrivateMessagesEnabled: true},
			}, nil
		},
	)

	servers := builder("agent-1", protocol.BuildRoomSharedSessionKey("conversation-1"), "room", "room-1", "狼人杀")
	config, ok := servers["nexus_room"].(sdkmcp.SDKServerConfig)
	if !ok {
		t.Fatalf("Room runtime 应注入 nexus_room SDK server: %+v", servers)
	}

	response, err := config.Instance.HandleMessage(context.Background(), map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	})
	if err != nil {
		t.Fatalf("tools/list 失败: %v", err)
	}
	result := response["result"].(map[string]any)
	tools := result["tools"].([]map[string]any)
	names := map[string]bool{}
	for _, item := range tools {
		name, _ := item["name"].(string)
		names[name] = true
	}
	if !names["send_directed_message"] {
		t.Fatalf("Room 开启私信时应暴露 send_directed_message: %+v", tools)
	}
}
