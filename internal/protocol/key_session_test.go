package protocol

import "testing"

func TestParseAgentSessionKeyWithTopicAndColonRef(t *testing.T) {
	raw := "agent:alpha:dg:group:123:456:topic:789"
	parsed := ParseSessionKey(raw)
	if !parsed.IsStructured {
		t.Fatalf("session_key 应合法: %+v", parsed)
	}
	if parsed.Kind != SessionKeyKindAgent {
		t.Fatalf("kind 解析错误: %+v", parsed)
	}
	if parsed.AgentID != "alpha" || parsed.Channel != "dg" || parsed.ChatType != "group" {
		t.Fatalf("基础字段解析错误: %+v", parsed)
	}
	if parsed.Ref != "123:456" || parsed.ThreadID != "789" {
		t.Fatalf("ref/thread 解析错误: %+v", parsed)
	}
}

func TestParseRoomSharedSessionKey(t *testing.T) {
	raw := "room:group:conversation_1"
	parsed := ParseSessionKey(raw)
	if !parsed.IsStructured || !parsed.IsShared {
		t.Fatalf("room 共享 key 解析错误: %+v", parsed)
	}
	if parsed.Kind != SessionKeyKindRoom || parsed.ConversationID != "conversation_1" {
		t.Fatalf("conversation_id 解析错误: %+v", parsed)
	}
	if !IsRoomSharedSessionKey(raw) {
		t.Fatalf("IsRoomSharedSessionKey 判断错误")
	}
}

func TestRequireStructuredSessionKeyRejectsPlainShape(t *testing.T) {
	if _, err := RequireStructuredSessionKey("plain-session-id"); err == nil {
		t.Fatal("非结构化 key 不应通过校验")
	}
}
