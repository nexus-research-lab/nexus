package protocol

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCreateRoomActionRequestIgnoresSourceAgentIDJSON(t *testing.T) {
	raw := []byte(`{"action_type":"private_note","source_agent_id":"forged-agent","content":"note"}`)
	var request CreateRoomActionRequest
	if err := json.Unmarshal(raw, &request); err != nil {
		t.Fatalf("解析 Room action 请求失败: %v", err)
	}
	if request.SourceAgentID != "" {
		t.Fatalf("source_agent_id 不应从 JSON body 注入: %+v", request)
	}

	request.SourceAgentID = "runtime-agent"
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("序列化 Room action 请求失败: %v", err)
	}
	if strings.Contains(string(encoded), "source_agent_id") {
		t.Fatalf("Room action 请求 JSON 不应包含 source_agent_id: %s", encoded)
	}
}
