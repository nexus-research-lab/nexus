package protocol

import "testing"

func TestAgentSessionKeyGenerationPreservesExternalRoute(t *testing.T) {
	key := BuildAgentAccountSessionKeyWithGeneration(
		"agent-a", SessionChannelFeishuSegment, "group", "cli-a", "oc-chat", "om-thread", "session-2",
	)
	parsed := ParseSessionKey(key)
	if !parsed.IsStructured || parsed.AgentID != "agent-a" || parsed.Channel != SessionChannelFeishuSegment ||
		parsed.ChatType != "group" || parsed.AccountID != "cli-a" || parsed.Ref != "oc-chat" ||
		parsed.ThreadID != "om-thread" || parsed.Generation != "session-2" {
		t.Fatalf("代次 session key 解析错误: key=%q parsed=%+v", key, parsed)
	}
	if got, err := RequireStructuredSessionKey(key); err != nil || got != key {
		t.Fatalf("代次 session key 应通过结构化校验: got=%q err=%v", got, err)
	}
	if legacy := LegacySessionDirectoryIdentity(key); legacy == LegacySessionDirectoryIdentity(BuildAgentAccountSessionKey("agent-a", SessionChannelFeishuSegment, "group", "cli-a", "oc-chat", "om-thread")) {
		t.Fatalf("不同 Session 代次不得复用物理目录: %q", legacy)
	}
}
