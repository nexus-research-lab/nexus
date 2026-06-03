package runtime

import (
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestInternalInputOptionsForPurposeMarksContinuationMetaHidden(t *testing.T) {
	options := InternalInputOptionsForPurpose(sdkprotocol.OutboundMessageOptions{
		Purpose:        "goal_continuation",
		Metadata:       map[string]string{"goal_id": "goal-1"},
	}, "goal_continuation")

	if !options.Meta || !options.HiddenFromUser || !options.Synthetic || options.Purpose != "goal_continuation" || options.Priority != "internal" || options.Metadata["goal_id"] != "goal-1" {
		t.Fatalf("options = %#v, want internal continuation runtime input", options)
	}
}

func TestInternalInputOptionsForPurposePreservesOtherPurposes(t *testing.T) {
	options := sdkprotocol.OutboundMessageOptions{
		HiddenFromUser: true,
		Synthetic:      true,
		Purpose:        "other",
		Priority:       "internal",
		Metadata:       map[string]string{"key": "value"},
	}
	got := InternalInputOptionsForPurpose(options, "goal_continuation")

	if !got.HiddenFromUser || !got.Synthetic || got.Purpose != "other" || got.Priority != "internal" || got.Metadata["key"] != "value" {
		t.Fatalf("options = %#v, want non-matching purpose preserved", got)
	}
}
