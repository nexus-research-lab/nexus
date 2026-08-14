package protocol

import "testing"

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
