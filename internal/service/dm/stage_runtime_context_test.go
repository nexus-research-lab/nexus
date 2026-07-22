package dm

import (
	"testing"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

type fakeDMStageToolRouter struct {
	requestedSessionKey string
}

func (f *fakeDMStageToolRouter) WithStageOpenRoutingHook(
	options agentclient.Options,
	_ string,
	_ string,
) agentclient.Options {
	return options
}

func (f *fakeDMStageToolRouter) StageRuntimeContext(sessionKey string) []runtimectx.ContextualInputBlock {
	f.requestedSessionKey = sessionKey
	return []runtimectx.ContextualInputBlock{
		runtimectx.NewContextualInputBlock("operation_stage", "Stage is open.", 20, nil),
	}
}

func TestDMRoundContextIncludesLiveStageCapability(t *testing.T) {
	t.Parallel()

	router := &fakeDMStageToolRouter{}
	runner := &roundRunner{
		service:        &Service{stageTools: router},
		sessionKey:     "agent:agent-1:ws:dm:conversation-1",
		goalContext:    "Continue the goal.",
		goalIDForUsage: "goal-1",
	}

	blocks := runner.contextualInputs()
	if router.requestedSessionKey != runner.sessionKey {
		t.Fatalf("Stage session key = %q, want %q", router.requestedSessionKey, runner.sessionKey)
	}
	if len(blocks) != 2 || blocks[0].Name != "goal" || blocks[1].Name != "operation_stage" {
		t.Fatalf("contextual inputs = %#v, want Goal followed by Stage", blocks)
	}
}
