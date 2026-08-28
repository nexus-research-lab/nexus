package communication

import (
	"context"
	"encoding/json"
	"slices"
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

func TestBuildToolsExposeOnlyTargetsAndSend(t *testing.T) {
	dmTools := listTools(t, BuildTools(nil, nil, RuntimeContext{}))
	if got := listedToolNames(dmTools); !slices.Equal(got, []string{"list_targets", "send_message"}) {
		t.Fatalf("DM communication tools = %v", got)
	}
	dmDestinations := toolProperty(t, dmTools, "send_message", "destination")["enum"].([]string)
	if slices.Contains(dmDestinations, destinationCurrentRoom) {
		t.Fatalf("DM schema 不应暴露当前 Room 目标: %v", dmDestinations)
	}
	if hasToolProperty(dmTools, "send_message", "recipients") {
		t.Fatal("DM schema 不应暴露 Room 私域投递参数")
	}

	roomTools := listTools(t, BuildTools(nil, &stubRoomService{}, RuntimeContext{
		CurrentRoomAvailable: true,
	}))
	roomDestinations := toolProperty(t, roomTools, "send_message", "destination")["enum"].([]string)
	if !slices.Contains(roomDestinations, destinationCurrentRoom) {
		t.Fatalf("Room schema 缺少当前 Room 目标: %v", roomDestinations)
	}
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

func TestRoomCommandIDIsStableAndRoundScoped(t *testing.T) {
	sctx := roomRuntimeContext()
	input := map[string]any{
		"destination": destinationCurrentRoom,
		"visibility":  visibilityPrivate,
		"recipients":  []any{"agent-amy"},
		"content":     "hello",
	}
	first, err := roomCommandID(sctx, nil, input)
	if err != nil {
		t.Fatal(err)
	}
	same, err := roomCommandID(sctx, nil, input)
	if err != nil || same != first {
		t.Fatalf("相同调用 command id 不稳定: first=%q same=%q err=%v", first, same, err)
	}
	sctx.CurrentAgentRoundID = "agent-round-2"
	next, err := roomCommandID(sctx, nil, input)
	if err != nil || next == first {
		t.Fatalf("跨 physical round command id 必须隔离: first=%q next=%q err=%v", first, next, err)
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

func listTools(t *testing.T, tools []sdktool.Tool) []map[string]any {
	t.Helper()
	server := sdktool.NewSimpleSDKMCPServer("nexus", "1.0.0", tools)
	response, err := server.HandleMessage(context.Background(), map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/list",
	})
	if err != nil {
		t.Fatal(err)
	}
	return response["result"].(map[string]any)["tools"].([]map[string]any)
}

func listedToolNames(tools []map[string]any) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		name, _ := tool["name"].(string)
		names = append(names, name)
	}
	return names
}

func toolProperty(
	t *testing.T,
	tools []map[string]any,
	toolName string,
	propertyName string,
) map[string]any {
	t.Helper()
	for _, tool := range tools {
		if tool["name"] != toolName {
			continue
		}
		schema := tool["inputSchema"].(map[string]any)
		return schema["properties"].(map[string]any)[propertyName].(map[string]any)
	}
	t.Fatalf("missing tool %s", toolName)
	return nil
}

func hasToolProperty(tools []map[string]any, toolName string, propertyName string) bool {
	for _, tool := range tools {
		if tool["name"] != toolName {
			continue
		}
		schema := tool["inputSchema"].(map[string]any)
		_, ok := schema["properties"].(map[string]any)[propertyName]
		return ok
	}
	return false
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
