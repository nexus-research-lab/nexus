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

func TestParseAgentSessionKeyWithAccountScope(t *testing.T) {
	raw := BuildAgentAccountSessionKey("alpha", "weixin-personal", "dm", "wx-account-1", "wx-user:1", "ctx:1")
	if raw != "agent:alpha:weixin-personal:dm:acct:wx-account-1:wx-user:1:topic:ctx:1" {
		t.Fatalf("account-scoped session_key 构建错误: %s", raw)
	}
	parsed := ParseSessionKey(raw)
	if !parsed.IsStructured {
		t.Fatalf("session_key 应合法: %+v", parsed)
	}
	if parsed.AgentID != "alpha" ||
		parsed.Channel != "weixin-personal" ||
		parsed.ChatType != "dm" ||
		parsed.AccountID != "wx-account-1" ||
		parsed.Ref != "wx-user:1" ||
		parsed.ThreadID != "ctx:1" {
		t.Fatalf("account-scoped session_key 解析错误: %+v", parsed)
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

func TestNormalizePersonalWeixinChannel(t *testing.T) {
	if got := NormalizeStoredChannelType("weixin-personal"); got != SessionChannelWeixinPersonal {
		t.Fatalf("个人微信通道应保持规范名称，实际 %q", got)
	}
	if got := NormalizeSessionKeyChannelSegment("weixin-personal"); got != SessionChannelWeixinPersonalSegment {
		t.Fatalf("个人微信 session_key 段应保持规范名称，实际 %q", got)
	}
}
