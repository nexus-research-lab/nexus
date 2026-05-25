package room

import (
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestRoomRoundInputOptionsMarksInternalContinuationHidden(t *testing.T) {
	roundValue := &activeRoomRound{
		Internal: true,
		InputOptions: sdkprotocol.OutboundMessageOptions{
			Purpose:  "goal_continuation",
			Metadata: map[string]string{"goal_id": "goal-room"},
		},
	}

	options := roomRoundInputOptions(roundValue)

	if !options.HiddenFromUser || !options.Synthetic || options.Priority != "internal" {
		t.Fatalf("options = %#v, want hidden synthetic internal continuation", options)
	}
	if options.Purpose != "goal_continuation" || options.Metadata["goal_id"] != "goal-room" {
		t.Fatalf("options = %#v, want continuation metadata preserved", options)
	}
}

func TestRoomRoundMarkerOptionsMarksInternalContinuationHidden(t *testing.T) {
	roundValue := &activeRoomRound{
		Internal: true,
		InputOptions: sdkprotocol.OutboundMessageOptions{
			Purpose:  "goal_continuation",
			Metadata: map[string]string{"goal_id": "goal-room"},
		},
	}

	options := roomRoundMarkerOptions(roundValue)

	if !options.HiddenFromUser || !options.Synthetic {
		t.Fatalf("options = %#v, want hidden synthetic round marker", options)
	}
	if options.Purpose != "goal_continuation" || options.Metadata["goal_id"] != "goal-room" {
		t.Fatalf("options = %#v, want continuation metadata preserved", options)
	}
}
