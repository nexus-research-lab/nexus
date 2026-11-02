package realtime

import (
	"github.com/nexus-research-lab/nexus/internal/protocol"
	"testing"
)

func TestSharedInputHonorsStoppedNonPrimaryTarget(t *testing.T) {
	entries := []roomInputQueueEntry{
		{Item: protocol.InputQueueItem{AgentID: "primary", TargetAgentIDs: []string{"primary", "stopped"}}},
		{Item: protocol.InputQueueItem{AgentID: "other", TargetAgentIDs: []string{"other"}}},
	}
	projectRoomQueuePause(entries, map[string]bool{"stopped": true})
	if !entries[0].Item.DispatchPaused || entries[1].Item.DispatchPaused {
		t.Fatalf("wrong pause projection: %+v", entries)
	}
	service := &Service{}
	if service.canDispatchInputQueueItem("session", "conversation", nil, entries[0].Item) {
		t.Fatal("shared item bypassed stopped member")
	}
}
