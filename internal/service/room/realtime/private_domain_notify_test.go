package realtime

import (
	"slices"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestDirectedMessageAgentIDsCoversSourceAndRecipientsOnce(t *testing.T) {
	got := directedMessageAgentIDs(protocol.RoomDirectedMessageRecord{
		SourceAgentID: " agent-b ",
		Recipients:    []string{"agent-a", "agent-b", "", "agent-c"},
	})
	want := []string{"agent-a", "agent-b", "agent-c"}
	if !slices.Equal(got, want) {
		t.Fatalf("私域失效应覆盖发送方与接收方且去重: got=%v want=%v", got, want)
	}
}
