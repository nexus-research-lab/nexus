package server

import (
	"context"
	"testing"

	sdkmcp "github.com/nexus-research-lab/nexus-agent-sdk-bridge/mcp"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

type stubRoomMCPService struct{}

type capturingRoomMCPService struct {
	stubRoomMCPService
	request protocol.CreateRoomDirectedMessageRequest
}

func (s *capturingRoomMCPService) HandleDirectedMessage(
	_ context.Context,
	_ string,
	_ string,
	request protocol.CreateRoomDirectedMessageRequest,
) (*protocol.RoomDirectedMessageRecord, error) {
	s.request = request
	return &protocol.RoomDirectedMessageRecord{MessageID: "message-1"}, nil
}

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
	return protocol.Message{"message_id": "public-1"}, nil
}

func (stubRoomMCPService) MarkPublicMessagePublished(context.Context, string, string, string) error {
	return nil
}

func TestRoomMCPBuilderOnlyAddsServerForRoomRuntime(t *testing.T) {
	builder := newRoomMCPBuilder(stubRoomMCPService{}, nil)
	agentValue := &protocol.Agent{AgentID: "agent-1", OwnerUserID: "user-1"}

	servers := builder(
		context.Background(),
		agentValue,
		protocol.BuildRoomSharedSessionKey("conversation-1"),
		"round-1",
		"room",
		"room-1",
		"狼人杀",
	)
	if _, ok := servers["nexus_room"].(sdkmcp.SDKServerConfig); !ok {
		t.Fatalf("Room runtime 应注入 nexus_room SDK server: %+v", servers)
	}
	untrustedServers := builder(
		context.Background(),
		agentValue,
		protocol.BuildRoomSharedSessionKey("conversation-1"),
		"round-2",
		"room_untrusted",
		"room-1",
		"狼人杀",
	)
	if _, ok := untrustedServers["nexus_room"].(sdkmcp.SDKServerConfig); !ok {
		t.Fatalf("Room 自动续跑仍应注入 nexus_room SDK server: %+v", untrustedServers)
	}

	if dmServers := builder(
		context.Background(),
		agentValue,
		"agent:agent-1:ws:dm:session-1",
		"round-1",
		"agent",
		"agent-1",
		"Agent",
	); len(dmServers) != 0 {
		t.Fatalf("非 Room runtime 不应注入 nexus_room: %+v", dmServers)
	}
}

func TestRoomMCPBuilderCarriesPhysicalAgentRound(t *testing.T) {
	svc := &capturingRoomMCPService{}
	builder := newRoomMCPBuilder(svc, nil)
	servers := builder(
		runtimectx.WithRuntimeRoundLease(context.Background(), "runtime-session", "agent-round-1"),
		&protocol.Agent{AgentID: "agent-1", OwnerUserID: "user-1"},
		protocol.BuildRoomSharedSessionKey("conversation-1"),
		"root-round-1",
		"room",
		"room-1",
		"狼人杀",
	)
	config := servers["nexus_room"].(sdkmcp.SDKServerConfig)
	if _, err := config.Instance.HandleMessage(context.Background(), map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params": map[string]any{
			"name": "send_directed_message",
			"arguments": map[string]any{
				"recipients": []any{"agent-2"},
				"content":    "查验谁？",
			},
		},
	}); err != nil {
		t.Fatalf("tools/call 失败: %v", err)
	}
	if svc.request.SourceAgentRoundID != "agent-round-1" {
		t.Fatalf("physical agent round = %q, want agent-round-1", svc.request.SourceAgentRoundID)
	}
}

func TestRoomMCPBuilderUsesRoomPrivateMessageSetting(t *testing.T) {
	builder := newRoomMCPBuilder(
		stubRoomMCPService{},
		func(context.Context, string) (*protocol.RoomAggregate, error) {
			return &protocol.RoomAggregate{
				Room: protocol.RoomRecord{PrivateMessagesEnabled: true},
			}, nil
		},
	)

	servers := builder(
		context.Background(),
		&protocol.Agent{AgentID: "agent-1", OwnerUserID: "user-1"},
		protocol.BuildRoomSharedSessionKey("conversation-1"),
		"round-1",
		"room",
		"room-1",
		"狼人杀",
	)
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
	if !names["publish_public_message"] {
		t.Fatalf("Room 开启私信时应暴露特殊流程的 publish_public_message: %+v", tools)
	}
}

func TestRoomMCPBuilderSkipsInternalContactChannel(t *testing.T) {
	builder := newRoomMCPBuilder(
		stubRoomMCPService{},
		func(context.Context, string) (*protocol.RoomAggregate, error) {
			return &protocol.RoomAggregate{
				Room: protocol.RoomRecord{IsContactChannel: true, PrivateMessagesEnabled: true},
			}, nil
		},
	)
	servers := builder(
		context.Background(),
		&protocol.Agent{AgentID: "agent-1", OwnerUserID: "user-1"},
		protocol.BuildRoomSharedSessionKey("conversation-1"),
		"round-1", "room", "room-1", "联系人通道",
	)
	if len(servers) != 0 {
		t.Fatalf("联系人内部通道不应注入 nexus_room: %+v", servers)
	}
}
