package realtime

import (
	"testing"
)

// TestExternalReplyContextIsTurnScoped 确认 IM 回信说明走本轮上下文，不进入 system prompt。
func TestExternalReplyContextIsTurnScoped(t *testing.T) {
	execution := &slotExecution{
		round:                &activeRoomRound{},
		slot:                 &activeRoomSlot{AgentID: "amy", RuntimeSessionKey: "agent:amy:room"},
		externalReplyContext: "reply to the paired IM user",
	}
	for _, internal := range []bool{false, true} {
		execution.round.Internal = internal
		found := false
		for _, block := range execution.contextualInputs() {
			if block.Name == "im_delivery" && block.Content == execution.externalReplyContext {
				found = true
			}
		}
		if !found {
			t.Fatalf("internal=%v: IM reply context missing from contextual inputs", internal)
		}
	}
}
