package runtime

import (
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestVisibleInputOptionsForPurposeClearsLocalOnlyFlags(t *testing.T) {
	options := VisibleInputOptionsForPurpose(sdkprotocol.OutboundMessageOptions{
		Meta:           true,
		HiddenFromUser: true,
		Synthetic:      true,
		Purpose:        "goal_continuation",
		Priority:       "internal",
		Metadata:       map[string]string{"goal_id": "goal-1"},
	}, "goal_continuation")

	if options.Meta || options.HiddenFromUser || options.Synthetic || options.Purpose != "" || options.Priority != "" || len(options.Metadata) > 0 {
		t.Fatalf("options = %#v, want normal visible runtime input", options)
	}
}

func TestVisibleInputOptionsForPurposePreservesOtherPurposes(t *testing.T) {
	options := sdkprotocol.OutboundMessageOptions{
		HiddenFromUser: true,
		Synthetic:      true,
		Purpose:        "other",
		Priority:       "internal",
		Metadata:       map[string]string{"key": "value"},
	}
	got := VisibleInputOptionsForPurpose(options, "goal_continuation")

	if !got.HiddenFromUser || !got.Synthetic || got.Purpose != "other" || got.Priority != "internal" || got.Metadata["key"] != "value" {
		t.Fatalf("options = %#v, want non-matching purpose preserved", got)
	}
}
