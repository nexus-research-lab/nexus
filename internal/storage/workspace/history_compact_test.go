package workspace

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestCompactMessagesMergesDurableUserWithoutMovingItAfterAssistant(t *testing.T) {
	rows := []protocol.Message{
		{
			"message_id": "user-1",
			"role":       "user",
			"round_id":   "round-1",
			"content":    "transcript content",
			"session_id": "runtime-session",
			"timestamp":  int64(1000),
		},
		{
			"message_id": "assistant-1",
			"role":       "assistant",
			"round_id":   "round-1",
			"parent_id":  "user-1",
			"content":    "reply",
			"timestamp":  int64(2000),
		},
		{
			"message_id":     "user-1",
			"role":           "user",
			"round_id":       "round-1",
			"agent_round_id": "agent-round-1",
			"content":        "durable marker content",
			"timestamp":      int64(3000),
			"metadata":       map[string]string{"source": "marker"},
		},
	}

	compacted := compactMessages(rows)
	if len(compacted) != 2 {
		t.Fatalf("expected two compacted messages, got %+v", compacted)
	}
	user := compacted[0]
	if stringFromAny(user["role"]) != "user" || messageTimestamp(user) != 1000 {
		t.Fatalf("durable user must retain the earliest input timestamp: %+v", compacted)
	}
	if stringFromAny(user["content"]) != "durable marker content" {
		t.Fatalf("durable marker content must win: %+v", user)
	}
	if stringFromAny(user["agent_round_id"]) != "agent-round-1" ||
		stringFromAny(user["session_id"]) != "runtime-session" {
		t.Fatalf("user snapshots must merge durable and runtime identity: %+v", user)
	}
	if stringFromAny(compacted[1]["role"]) != "assistant" {
		t.Fatalf("assistant must remain after the user: %+v", compacted)
	}
}
