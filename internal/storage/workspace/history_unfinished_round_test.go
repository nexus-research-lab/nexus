package workspace

import (
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestBuildRoomTranscriptReferenceRejectsResultRole(t *testing.T) {
	row := buildRoomTranscriptReference(protocol.Message{
		"message_id": "result-1",
		"session_id": "sdk-session-1",
		"agent_id":   "nexus",
		"round_id":   "round-1",
		"role":       "result",
		"timestamp":  1000,
	}, "/tmp/workspace", "agent:nexus:ws:dm:test")
	if row != nil {
		t.Fatalf("result 行不应生成 transcript_ref: %+v", row)
	}
}
