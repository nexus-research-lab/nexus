package room

import (
	"testing"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
)

type fakeRoomStageToolRouter struct {
	requestedSessionKey string
}

func (f *fakeRoomStageToolRouter) WithStageOpenRoutingHook(
	options agentclient.Options,
	_ string,
	_ string,
) agentclient.Options {
	return options
}

func (f *fakeRoomStageToolRouter) StageRuntimeContext(sessionKey string) []runtimectx.ContextualInputBlock {
	f.requestedSessionKey = sessionKey
	return []runtimectx.ContextualInputBlock{
		runtimectx.NewContextualInputBlock("operation_stage", "Stage is open.", 20, nil),
	}
}

func TestRoomSlotContextIncludesLiveStageCapability(t *testing.T) {
	t.Parallel()

	router := &fakeRoomStageToolRouter{}
	execution := &slotExecution{
		service: &RealtimeService{stageTools: router},
		slot: &activeRoomSlot{
			RuntimeSessionKey: "agent:agent-1:ws:group:conversation-1",
		},
	}

	blocks := execution.contextualInputs()
	if router.requestedSessionKey != execution.slot.RuntimeSessionKey {
		t.Fatalf("Stage session key = %q, want %q", router.requestedSessionKey, execution.slot.RuntimeSessionKey)
	}
	if len(blocks) != 1 || blocks[0].Name != "operation_stage" {
		t.Fatalf("contextual inputs = %#v, want Stage context", blocks)
	}
}
