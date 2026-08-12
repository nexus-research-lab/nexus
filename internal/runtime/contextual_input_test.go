package runtime

import (
	"strings"
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestRenderContextualInputBlockUsesInternalSourceEnvelope(t *testing.T) {
	for _, test := range []struct {
		name    string
		content string
	}{
		{name: ContextualInputNameRoundRecovery, content: "Recorded terminal reason: content_filtered."},
		{name: ContextualInputNameExecution, content: `<nexus_execution_context execution_version="4"></nexus_execution_context>`},
		{name: ContextualInputNameTransport, content: `<nexus_transport_context transport="im" channel="weixin-personal" chat_type="dm" route_binding="host" />`},
	} {
		rendered := renderContextualInputBlock(NewContextualInputBlock(test.name, test.content, 0, nil))
		if !strings.Contains(rendered, `<internal_context source="`+test.name+`">`) ||
			!strings.Contains(rendered, test.content) {
			t.Fatalf("context 未保留来源或正文: %q", rendered)
		}
	}
}

func TestRuntimeInputOptionsForPurpose(t *testing.T) {
	options := RuntimeInputOptionsForPurpose(sdkprotocol.OutboundMessageOptions{
		Meta:           true,
		HiddenFromUser: true,
		Synthetic:      true,
		RecallQuery:    "do not recall",
		Purpose:        "goal_continuation",
		Priority:       "internal",
		Metadata:       map[string]string{"goal_id": "goal-1"},
	}, "goal_continuation")

	if options.Meta || options.HiddenFromUser || options.Synthetic || options.RecallQuery != "" || options.Purpose != "" || options.Priority != "" || options.Metadata != nil {
		t.Fatalf("options = %#v, want continuation runtime input control fields cleared", options)
	}
	other := sdkprotocol.OutboundMessageOptions{
		Meta:           true,
		HiddenFromUser: true,
		Synthetic:      true,
		Purpose:        "other",
		Priority:       "internal",
		Metadata:       map[string]string{"key": "value"},
	}
	got := RuntimeInputOptionsForPurpose(other, "goal_continuation")

	if !got.Meta || !got.HiddenFromUser || !got.Synthetic || got.Purpose != "other" || got.Priority != "internal" || got.Metadata["key"] != "value" {
		t.Fatalf("options = %#v, want non-matching purpose preserved", got)
	}
}
