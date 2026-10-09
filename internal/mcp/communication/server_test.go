package communication

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/mcp/sdktool"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	communicationsvc "github.com/nexus-research-lab/nexus/internal/service/communication"
	roomsvc "github.com/nexus-research-lab/nexus/internal/service/room"
)

type stubRoomService struct {
	directedRequest        protocol.CreateRoomDirectedMessageRequest
	directedRoomID         string
	directedConversationID string
	directedOwnerUserID    string
	directedErr            error
	publicRequest          protocol.CreateRoomPublicMessageRequest
	publicMessagePublished bool
}

func (s *stubRoomService) HandleDirectedMessage(
	ctx context.Context,
	roomID string,
	conversationID string,
	request protocol.CreateRoomDirectedMessageRequest,
) (*protocol.RoomDirectedMessageRecord, error) {
	s.directedRequest = request
	s.directedRoomID = roomID
	s.directedConversationID = conversationID
	s.directedOwnerUserID, _ = authctx.CurrentUserID(ctx)
	if s.directedErr != nil {
		return nil, s.directedErr
	}
	return &protocol.RoomDirectedMessageRecord{
		MessageID:  "private-1",
		WakePolicy: request.WakePolicy,
	}, nil
}

func (s *stubRoomService) HandlePublicMessage(
	_ context.Context,
	_ string,
	_ string,
	request protocol.CreateRoomPublicMessageRequest,
) (protocol.Message, error) {
	s.publicRequest = request
	return protocol.Message{"message_id": "public-1"}, nil
}

func (s *stubRoomService) MarkPublicMessagePublished(
	context.Context,
	string,
	string,
	string,
) error {
	s.publicMessagePublished = true
	return nil
}

func TestSendMessageRoutesCurrentRoomPrivate(t *testing.T) {
	svc := &stubRoomService{}
	sctx := roomRuntimeContext()
	result, isError := callTool(t, BuildTools(nil, svc, sctx), "send_message", map[string]any{
		"destination":  destinationCurrentRoom,
		"visibility":   visibilityPrivate,
		"recipients":   []any{"agent-amy"},
		"wake_targets": []any{"agent-amy"},
		"content":      "今晚查验谁？",
		"wake_policy":  "immediate",
		"reply_route": map[string]any{
			"mode":        "private",
			"recipients":  []any{"agent-host"},
			"wake_policy": "immediate",
			"next_reply_route": map[string]any{
				"mode": "public",
			},
		},
	})
	if isError {
		t.Fatalf("Room 私信不应失败: %s", extractText(t, result))
	}
	if svc.directedRoomID != "room-1" || svc.directedConversationID != "conversation-1" ||
		svc.directedOwnerUserID != "user-1" {
		t.Fatalf("Room scope 未由宿主注入: %+v", svc)
	}
	request := svc.directedRequest
	if request.SourceAgentID != "agent-host" || request.SourceAgentRoundID != "agent-round-1" ||
		request.RootRoundID != "root-round-1" || strings.TrimSpace(request.CommandID) == "" {
		t.Fatalf("Room runtime 身份未完整注入: %+v", request)
	}
	if request.ReplyRoute.NextReplyRoute == nil ||
		request.ReplyRoute.NextReplyRoute.Mode != protocol.RoomReplyRoutePublic {
		t.Fatalf("reply route 未解析: %+v", request.ReplyRoute)
	}
	if request.GoalCollaborationBinding == nil ||
		request.GoalCollaborationBinding.GoalID != "goal-1" {
		t.Fatalf("Goal 协作归因未注入: %+v", request.GoalCollaborationBinding)
	}
	if !strings.Contains(extractText(t, result), `"status":"queued"`) {
		t.Fatalf("Room 私信回执不正确: %s", extractText(t, result))
	}
}

func TestSendMessageRoutesCurrentRoomPublicAndSuppressesFinal(t *testing.T) {
	svc := &stubRoomService{}
	result, isError := callTool(t, BuildTools(nil, svc, roomRuntimeContext()), "send_message", map[string]any{
		"destination": destinationCurrentRoom,
		"visibility":  visibilityPublic,
		"content":     "公开结论",
	})
	if isError {
		t.Fatalf("Room 公区发送不应失败: %s", extractText(t, result))
	}
	if svc.publicRequest.SourceAgentID != "agent-host" ||
		svc.publicRequest.SourceAgentRoundID != "agent-round-1" ||
		svc.publicRequest.RootRoundID != "root-round-1" ||
		!svc.publicMessagePublished {
		t.Fatalf("Room 公区发送未完整收口: %+v", svc)
	}
}

func TestSendMessageDefersAutomaticPrivateReplyToFinal(t *testing.T) {
	result, isError := callTool(t, BuildTools(nil, &stubRoomService{
		directedErr: roomsvc.ErrDirectedReplyAutoRouted,
	}, roomRuntimeContext()), "send_message", map[string]any{
		"destination": destinationCurrentRoom,
		"visibility":  visibilityPrivate,
		"recipients":  []any{"agent-amy"},
		"content":     "查验结果",
	})
	if isError || !strings.Contains(extractText(t, result), `"status":"reply_via_final"`) {
		t.Fatalf("自动私域回复应交给 final route: %+v", result)
	}
}

func TestSendMessageRejectsFieldsFromAnotherDestination(t *testing.T) {
	result, isError := callTool(t, BuildTools(nil, &stubRoomService{}, roomRuntimeContext()), "send_message", map[string]any{
		"destination": destinationCurrentRoom,
		"visibility":  visibilityPublic,
		"recipients":  []any{"agent-amy"},
		"content":     "不应接受 recipients",
	})
	if !isError || !strings.Contains(extractText(t, result), "recipients 不适用于当前消息目标") {
		t.Fatalf("跨分支字段必须 fail closed: %+v", result)
	}
}

func TestSendMessageDoesNotBypassCurrentRoomPolicyThroughRoomTarget(t *testing.T) {
	result, isError := callTool(t, BuildTools(nil, &stubRoomService{}, roomRuntimeContext()), "send_message", map[string]any{
		"destination": "room",
		"target_id":   "room-1",
		"content":     "绕过当前 Room 分支",
	})
	if !isError || !strings.Contains(extractText(t, result), "destination=current_room") {
		t.Fatalf("当前 Room 必须走受控分支: %+v", result)
	}
}

func roomRuntimeContext() RuntimeContext {
	return RuntimeContext{
		Actor: communicationsvc.Actor{
			OwnerUserID: "user-1", AgentID: "agent-host",
			SessionKey: "room:group:conversation-1", RoundID: "root-round-1",
			ContextKind: communicationsvc.ContextKindRoom,
			RoomID:      "room-1", ConversationID: "conversation-1",
			GoalCollaborationBinding: func() *protocol.GoalCollaborationBinding {
				return &protocol.GoalCollaborationBinding{GoalID: "goal-1", ObjectiveRevision: 1}
			},
		},
		CurrentAgentRoundID:  "agent-round-1",
		CurrentRoomAvailable: true,
	}
}

func callTool(
	t *testing.T,
	tools []sdktool.Tool,
	name string,
	args map[string]any,
) (map[string]any, bool) {
	t.Helper()
	server := sdktool.NewSimpleSDKMCPServer("nexus", "1.0.0", tools)
	response, err := server.HandleMessage(context.Background(), map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  map[string]any{"name": name, "arguments": args},
	})
	if err != nil {
		t.Fatal(err)
	}
	result := response["result"].(map[string]any)
	isError, _ := result["isError"].(bool)
	return result, isError
}

func extractText(t *testing.T, result map[string]any) string {
	t.Helper()
	content := result["content"].([]map[string]any)
	if len(content) == 0 {
		t.Fatalf("empty content: %+v", result)
	}
	payload, _ := content[0]["text"].(string)
	var decoded any
	if !resultBool(result, "isError") && json.Unmarshal([]byte(payload), &decoded) != nil {
		t.Fatalf("result is not JSON: %s", payload)
	}
	return payload
}

func resultBool(result map[string]any, key string) bool {
	value, _ := result[key].(bool)
	return value
}
