// protocol_test.go 覆盖协议事件、身份游标、会话键、轮次交接、IM 权限命令、失败识别与工具结果测试。
package protocol

import (
	"testing"
)

func TestChatPendingSnapshotCarriesRoomReplayFence(t *testing.T) {
	event := NewChatPendingSnapshotEvent(
		"room:session",
		"round-root",
		42,
		[]ChatAckPendingSlot{},
		[]string{},
	)
	if event.Data["pending_snapshot"] != true {
		t.Fatalf("pending snapshot marker missing: %+v", event.Data)
	}
	if event.Data["snapshot_room_seq"] != int64(42) {
		t.Fatalf("snapshot_room_seq = %#v, want 42", event.Data["snapshot_room_seq"])
	}

	clamped := NewChatPendingSnapshotEvent("", "", -1, nil, nil)
	if clamped.Data["snapshot_room_seq"] != int64(0) {
		t.Fatalf("negative snapshot_room_seq must clamp to zero: %+v", clamped.Data)
	}
}

func TestContextUsageEventCarriesCategories(t *testing.T) {
	event := NewContextUsageEvent("agent:session", "agent-1", ContextUsageData{
		TotalTokens: 17900,
		MaxTokens:   1000000,
		Percentage:  2,
		Categories: []ContextUsageCategory{
			{Name: "System prompt", Tokens: 3200},
			{Name: "MCP tools", Tokens: 800, Deferred: true},
		},
	})
	categories, ok := event.Data["categories"].([]map[string]any)
	if !ok || len(categories) != 2 {
		t.Fatalf("categories missing from payload: %+v", event.Data)
	}
	if categories[0]["name"] != "System prompt" || categories[0]["tokens"] != 3200 {
		t.Fatalf("unexpected first category: %+v", categories[0])
	}
	if _, present := categories[0]["deferred"]; present {
		t.Fatalf("non-deferred category should omit deferred: %+v", categories[0])
	}
	if categories[1]["deferred"] != true {
		t.Fatalf("deferred flag lost: %+v", categories[1])
	}

	empty := NewContextUsageEvent("agent:session", "agent-1", ContextUsageData{
		TotalTokens: 1, MaxTokens: 10, Percentage: 10,
	})
	if _, present := empty.Data["categories"]; present {
		t.Fatalf("empty categories should be omitted: %+v", empty.Data)
	}
}

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

func TestNormalizePublicHandoffReplyRequiresExactIdentity(t *testing.T) {
	valid := NormalizePublicHandoffReply(map[string]any{
		"handoff_id": " rh-1 ", "source_message_id": " msg-1 ",
		"source_agent_id": " agent-lead ", "goal_id": "must-not-project",
	})
	if valid == nil || valid.HandoffID != "rh-1" ||
		valid.SourceMessageID != "msg-1" || valid.SourceAgentID != "agent-lead" {
		t.Fatalf("valid handoff reply was not normalized: %+v", valid)
	}

	for _, value := range []any{
		nil,
		map[string]any{"handoff_id": "rh-1", "source_message_id": "msg-1"},
		map[string]any{"handoff_id": "rh-1", "source_agent_id": "agent-lead"},
		map[string]any{"source_message_id": "msg-1", "source_agent_id": "agent-lead"},
	} {
		if reply := NormalizePublicHandoffReply(value); reply != nil {
			t.Fatalf("incomplete handoff reply must fail closed: input=%+v reply=%+v", value, reply)
		}
	}
}

func TestIsProviderTokenLimitError(t *testing.T) {
	for _, signal := range []string{
		"context_length_exceeded",
		"Maximum context length is 128000 tokens",
		"request_too_large",
		"You've hit your usage limit",
		"out of tokens",
		"insufficient_quota",
		"上下文长度超过上限",
	} {
		if !IsProviderTokenLimitError(signal) {
			t.Fatalf("IsProviderTokenLimitError(%q) = false, want true", signal)
		}
	}
}

func TestParseMutationResultChangedReadsOnlyExplicitEnvelopeRefs(t *testing.T) {
	t.Parallel()

	got := ParseMutationResultChanged(map[string]any{
		"structuredContent": map[string]any{
			"outcome":      "applied",
			"execution_id": "execution-1",
			"changed": []any{
				" assignment:assignment-1 ",
				"attempt:attempt-1",
				"assignment:assignment-1",
			},
		},
	})
	if len(got) != 2 || got[0] != "assignment:assignment-1" ||
		got[1] != "attempt:attempt-1" {
		t.Fatalf("ParseMutationResultChanged() = %#v", got)
	}
	envelope, ok := ParseMutationResultEnvelope(map[string]any{
		"outcome":      "applied",
		"execution_id": "execution-1",
		"changed":      []string{"assignment:assignment-1"},
	})
	if !ok || envelope.ExecutionID != "execution-1" ||
		len(envelope.Changed) != 1 || envelope.Changed[0] != "assignment:assignment-1" {
		t.Fatalf("mutation identity envelope = %+v/%t", envelope, ok)
	}
	if refs := ParseMutationResultChanged(map[string]any{
		"changed": []any{"assignment:untrusted"},
	}); len(refs) != 0 {
		t.Fatalf("non-envelope changed refs = %#v", refs)
	}
}

func TestParseGoalStatusResultReadsOnlyTerminalGoalStatus(t *testing.T) {
	for _, test := range []struct {
		name   string
		value  any
		want   GoalStatus
		wantOK bool
	}{
		{name: "complete structured", value: map[string]any{"goal": map[string]any{"status": "complete"}}, want: GoalStatusComplete, wantOK: true},
		{name: "blocked text", value: `{"goal":{"status":"blocked"}}`, want: GoalStatusBlocked, wantOK: true},
		{name: "unrelated status", value: map[string]any{"status": "complete"}},
		{name: "active goal", value: map[string]any{"goal": map[string]any{"status": "active"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, ok := ParseGoalStatusResult(test.value)
			if got != test.want || ok != test.wantOK {
				t.Fatalf("ParseGoalStatusResult() = %q/%v, want %q/%v", got, ok, test.want, test.wantOK)
			}
		})
	}
}

func TestParseGoalIDResultReadsOnlyExplicitGoalIdentity(t *testing.T) {
	for _, test := range []struct {
		name   string
		value  any
		want   string
		wantOK bool
	}{
		{name: "camel case", value: map[string]any{"goalId": " goal-1 "}, want: "goal-1", wantOK: true},
		{name: "snake case text", value: `{"goal_id":"goal-2"}`, want: "goal-2", wantOK: true},
		{name: "nested goal identity", value: map[string]any{"goal": map[string]any{"id": "goal-3"}}, want: "goal-3", wantOK: true},
		{name: "wrapped", value: map[string]any{"structuredContent": map[string]any{"goalId": "goal-4"}}, want: "goal-4", wantOK: true},
		{name: "unrelated id", value: map[string]any{"id": "tool-1"}},
		{name: "null goal id", value: map[string]any{"goalId": nil}},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, ok := ParseGoalIDResult(test.value)
			if got != test.want || ok != test.wantOK {
				t.Fatalf("ParseGoalIDResult() = %q/%v, want %q/%v", got, ok, test.want, test.wantOK)
			}
		})
	}
}

// ParseMutationResultChanged extracts only the server-issued entity refs from
// a mutation result. Callers must still resolve those refs against an
// authoritative snapshot before treating them as WorkGraph identity.
func ParseMutationResultChanged(values ...any) []string {
	if result, ok := ParseMutationResultEnvelope(values...); ok {
		return result.Changed
	}
	return nil
}
