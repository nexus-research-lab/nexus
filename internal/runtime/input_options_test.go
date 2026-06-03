package runtime

import (
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestRuntimeInputOptionsForPurposeClearsContinuationControlFields(t *testing.T) {
	options := RuntimeInputOptionsForPurpose(sdkprotocol.OutboundMessageOptions{
		Meta:           true,
		HiddenFromUser: true,
		Synthetic:      true,
		Purpose:        "goal_continuation",
		Priority:       "internal",
		Metadata:       map[string]string{"goal_id": "goal-1"},
	}, "goal_continuation")

	if options.Meta || options.HiddenFromUser || options.Synthetic || options.Purpose != "" || options.Priority != "" || options.Metadata != nil {
		t.Fatalf("options = %#v, want continuation runtime input control fields cleared", options)
	}
}

func TestRuntimeInputOptionsForPurposePreservesOtherPurposes(t *testing.T) {
	options := sdkprotocol.OutboundMessageOptions{
		Meta:           true,
		HiddenFromUser: true,
		Synthetic:      true,
		Purpose:        "other",
		Priority:       "internal",
		Metadata:       map[string]string{"key": "value"},
	}
	got := RuntimeInputOptionsForPurpose(options, "goal_continuation")

	if !got.Meta || !got.HiddenFromUser || !got.Synthetic || got.Purpose != "other" || got.Priority != "internal" || got.Metadata["key"] != "value" {
		t.Fatalf("options = %#v, want non-matching purpose preserved", got)
	}
}
