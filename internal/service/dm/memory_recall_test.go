package dm

import (
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestRoundRunnerRuntimeInputOptionsUsesCanonicalUserText(t *testing.T) {
	runner := &roundRunner{
		content: "  用户原始问题  ",
		inputOptions: sdkprotocol.OutboundMessageOptions{
			RecallQuery: "stale wrapped prompt",
		},
	}

	options := runner.runtimeInputOptions()
	if options.RecallQuery != "用户原始问题" {
		t.Fatalf("RecallQuery = %q, want canonical user text", options.RecallQuery)
	}
}

func TestRoundRunnerRuntimeInputOptionsSkipsInternalInput(t *testing.T) {
	runner := &roundRunner{
		content:  "继续内部任务",
		internal: true,
		inputOptions: sdkprotocol.OutboundMessageOptions{
			Purpose:     "goal_continuation",
			RecallQuery: "must be cleared",
		},
	}

	if query := runner.runtimeInputOptions().RecallQuery; query != "" {
		t.Fatalf("RecallQuery = %q, want internal input skipped", query)
	}
}
