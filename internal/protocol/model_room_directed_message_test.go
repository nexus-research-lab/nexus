package protocol

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCreateRoomDirectedMessageRequestIgnoresSourceAgentIDJSON(t *testing.T) {
	raw := []byte(`{"source_agent_id":"forged-agent","recipients":["agent-devin"],"content":"note"}`)
	var request CreateRoomDirectedMessageRequest
	if err := json.Unmarshal(raw, &request); err != nil {
		t.Fatalf("解析 Room directed message 请求失败: %v", err)
	}
	if request.SourceAgentID != "" {
		t.Fatalf("source_agent_id 不应从 JSON body 注入: %+v", request)
	}

	request.SourceAgentID = "runtime-agent"
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("序列化 Room directed message 请求失败: %v", err)
	}
	if strings.Contains(string(encoded), "source_agent_id") {
		t.Fatalf("Room directed message 请求 JSON 不应包含 source_agent_id: %s", encoded)
	}
}

func TestCreateRoomPublicMessageRequestIgnoresSourceAgentIDJSON(t *testing.T) {
	raw := []byte(`{"source_agent_id":"forged-agent","content":"hello"}`)
	var request CreateRoomPublicMessageRequest
	if err := json.Unmarshal(raw, &request); err != nil {
		t.Fatalf("解析 Room public message 请求失败: %v", err)
	}
	if request.SourceAgentID != "" {
		t.Fatalf("source_agent_id 不应从 JSON body 注入: %+v", request)
	}

	request.SourceAgentID = "runtime-agent"
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("序列化 Room public message 请求失败: %v", err)
	}
	if strings.Contains(string(encoded), "source_agent_id") {
		t.Fatalf("Room public message 请求 JSON 不应包含 source_agent_id: %s", encoded)
	}
}

func TestRoomReplyRouteSupportsNextReplyRoute(t *testing.T) {
	raw := []byte(`{"mode":"private","recipients":["agent-amy"],"wake_policy":"immediate","next_reply_route":{"mode":"public"}}`)
	var route RoomReplyRoute
	if err := json.Unmarshal(raw, &route); err != nil {
		t.Fatalf("解析 next_reply_route 失败: %v", err)
	}
	if route.NextReplyRoute == nil || route.NextReplyRoute.Mode != RoomReplyRoutePublic {
		t.Fatalf("next_reply_route 未保留: %+v", route)
	}

	encoded, err := json.Marshal(route)
	if err != nil {
		t.Fatalf("序列化 next_reply_route 失败: %v", err)
	}
	if !strings.Contains(string(encoded), `"next_reply_route":{"mode":"public"}`) {
		t.Fatalf("next_reply_route JSON 不正确: %s", encoded)
	}
}
