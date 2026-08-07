package realtime

import (
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestRoomSlotRuntimeInputOptionsUsesDirectTrigger(t *testing.T) {
	roundValue := &activeRoomRound{
		InputOptions: sdkprotocol.OutboundMessageOptions{
			RecallQuery: "wrapped public feed",
		},
	}
	slot := &activeRoomSlot{
		Trigger: roomTrigger{Content: "  用户直接问题  "},
	}

	options := roomSlotRuntimeInputOptions(roundValue, slot)
	if options.RecallQuery != "用户直接问题" {
		t.Fatalf("RecallQuery = %q, want direct slot trigger", options.RecallQuery)
	}
}

func TestRoomSlotRuntimeInputOptionsSkipsGoalContinuation(t *testing.T) {
	roundValue := &activeRoomRound{
		Internal: true,
		InputOptions: sdkprotocol.OutboundMessageOptions{
			Purpose:     "goal_continuation",
			RecallQuery: "must be cleared",
		},
	}
	slot := &activeRoomSlot{
		Trigger: roomTrigger{TriggerType: "goal_continuation", Content: "继续"},
	}

	if query := roomSlotRuntimeInputOptions(roundValue, slot).RecallQuery; query != "" {
		t.Fatalf("RecallQuery = %q, want goal continuation skipped", query)
	}
}
