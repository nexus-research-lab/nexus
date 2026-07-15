package roommcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/mcp/room/contract"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type stubRoomService struct {
	directedRoomID         string
	directedConversationID string
	directedRequest        protocol.CreateRoomDirectedMessageRequest
	directedOwnerUserID    string
	publicMessagePublished bool
}

func (s *stubRoomService) HandleDirectedMessage(
	ctx context.Context,
	roomID string,
	conversationID string,
	request protocol.CreateRoomDirectedMessageRequest,
) (*protocol.RoomDirectedMessageRecord, error) {
	s.directedRoomID = roomID
	s.directedConversationID = conversationID
	s.directedRequest = request
	s.directedOwnerUserID, _ = authctx.CurrentUserID(ctx)
	return &protocol.RoomDirectedMessageRecord{
		MessageID:      "msg-1",
		RoomID:         roomID,
		ConversationID: conversationID,
		SourceAgentID:  request.SourceAgentID,
		Recipients:     request.Recipients,
		Content:        request.Content,
		WakePolicy:     request.WakePolicy,
		ReplyRoute:     request.ReplyRoute,
		DelaySeconds:   request.DelaySeconds,
		CorrelationID:  request.CorrelationID,
		Timestamp:      123,
	}, nil
}

func (s *stubRoomService) HandlePublicMessage(
	context.Context,
	string,
	string,
	protocol.CreateRoomPublicMessageRequest,
) (protocol.Message, error) {
	return protocol.Message{"message_id": "public-1"}, nil
}

func (s *stubRoomService) MarkPublicMessagePublished(context.Context, string, string, string) error {
	s.publicMessagePublished = true
	return nil
}

func TestToolsListOmitsPublicToolByDefault(t *testing.T) {
	tools := listRoomTools(t, &stubRoomService{}, contract.ServerContext{})
	names := map[string]bool{}
	for _, item := range tools {
		name, _ := item["name"].(string)
		names[name] = true
		meta, ok := item["_meta"].(map[string]any)
		if !ok {
			t.Fatalf("%s missing _meta", name)
		}
		if hint, _ := meta["anthropic/searchHint"].(string); strings.TrimSpace(hint) == "" {
			t.Fatalf("%s missing searchHint", name)
		}
		if _, ok := meta["anthropic/alwaysLoad"]; ok {
			t.Fatalf("%s should stay deferred", name)
		}
	}
	if names["publish_public_message"] {
		t.Fatalf("Room 普通运行时不应暴露 publish_public_message: %+v", tools)
	}
	if names["send_directed_message"] {
		t.Fatalf("Room 未开启私信时不应暴露 send_directed_message: %+v", tools)
	}
}

func TestToolsListIncludesDirectedToolWhenEnabled(t *testing.T) {
	tools := listRoomTools(t, &stubRoomService{}, contract.ServerContext{PrivateMessagesEnabled: true})
	names := map[string]bool{}
	for _, item := range tools {
		name, _ := item["name"].(string)
		names[name] = true
	}
	if !names["send_directed_message"] {
		t.Fatalf("missing room tool send_directed_message: %+v", tools)
	}
	if !names["publish_public_message"] {
		t.Fatalf("Room 特殊流程应暴露 publish_public_message: %+v", tools)
	}
}

func TestSendDirectedMessageUsesInjectedRoomScope(t *testing.T) {
	svc := &stubRoomService{}
	result, isError := callRoomTool(t, svc, contract.ServerContext{
		OwnerUserID:            "user-1",
		CurrentAgentID:         "agent-host",
		CurrentRoundID:         "round-root-1",
		RoomID:                 "room-1",
		ConversationID:         "conversation-1",
		SourceContextType:      "room",
		PrivateMessagesEnabled: true,
	}, "send_directed_message", map[string]any{
		"recipients":  []any{"agent-amy"},
		"content":     "今晚查验谁？",
		"wake_policy": "immediate",
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
		t.Fatalf("send_directed_message 不应失败: %s", extractRoomText(t, result))
	}
	if svc.directedRoomID != "room-1" || svc.directedConversationID != "conversation-1" {
		t.Fatalf("Room scope 未注入: room=%s conversation=%s", svc.directedRoomID, svc.directedConversationID)
	}
	if svc.directedOwnerUserID != "user-1" {
		t.Fatalf("owner user 未注入: %s", svc.directedOwnerUserID)
	}
	if svc.directedRequest.SourceAgentID != "agent-host" {
		t.Fatalf("source agent 不应来自工具入参: %+v", svc.directedRequest)
	}
	if svc.directedRequest.RootRoundID != "round-root-1" {
		t.Fatalf("root round 应由运行时注入: %+v", svc.directedRequest)
	}
	if svc.directedRequest.ReplyRoute.NextReplyRoute == nil ||
		svc.directedRequest.ReplyRoute.NextReplyRoute.Mode != protocol.RoomReplyRoutePublic {
		t.Fatalf("next_reply_route 未解析: %+v", svc.directedRequest.ReplyRoute)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(extractRoomText(t, result)), &payload); err != nil {
		t.Fatalf("工具输出不是 JSON: %v", err)
	}
	if payload["message_id"] == nil || payload["status"] != "queued" {
		t.Fatalf("工具应只返回最小确认: %+v", payload)
	}
	if payload["content"] != nil || payload["item"] != nil {
		t.Fatalf("工具输出不应泄漏 directed message 正文: %+v", payload)
	}
}

func callRoomTool(
	t *testing.T,
	svc contract.Service,
	sctx contract.ServerContext,
	name string,
	args map[string]any,
) (map[string]any, bool) {
	t.Helper()
	server := NewServer(svc, sctx)
	resp, err := server.HandleMessage(context.Background(), map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  map[string]any{"name": name, "arguments": args},
	})
	if err != nil {
		t.Fatalf("HandleMessage error: %v", err)
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing result, got %+v", resp)
	}
	isError, _ := result["isError"].(bool)
	return result, isError
}

func listRoomTools(t *testing.T, svc contract.Service, sctx contract.ServerContext) []map[string]any {
	t.Helper()
	server := NewServer(svc, sctx)
	resp, err := server.HandleMessage(context.Background(), map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/list",
	})
	if err != nil {
		t.Fatalf("HandleMessage error: %v", err)
	}
	result, ok := resp["result"].(map[string]any)
	if !ok {
		t.Fatalf("missing result, got %+v", resp)
	}
	tools, ok := result["tools"].([]map[string]any)
	if !ok {
		t.Fatalf("tools not []map, got %T", result["tools"])
	}
	return tools
}

func extractRoomText(t *testing.T, result map[string]any) string {
	t.Helper()
	content, ok := result["content"].([]map[string]any)
	if !ok || len(content) == 0 {
		t.Fatalf("content 格式不正确: %+v", result)
	}
	text, _ := content[0]["text"].(string)
	return text
}
