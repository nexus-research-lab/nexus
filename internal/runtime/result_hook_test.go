package runtime

import (
	"context"
	"testing"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
)

type resultCapabilityClient struct {
	*fakeRuntimeClient
	ack, policy bool
}

func (c *resultCapabilityClient) Supports(capability agentclient.Capability) bool {
	return capability == agentclient.CapabilityHookResponseAck && c.ack || capability == agentclient.CapabilityMessageExecutionPolicy && c.policy
}

func TestResultHookRequiresCapabilitiesAndUnambiguousRound(t *testing.T) {
	for _, ack := range []bool{false, true} {
		for _, policy := range []bool{false, true} {
			if got := SupportsWorkResultHooks(&resultCapabilityClient{ack: ack, policy: policy}); got != (ack && policy) {
				t.Fatalf("capabilities ack=%v policy=%v: %v", ack, policy, got)
			}
		}
	}
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: &fakeRuntimeClient{}})
	options := manager.WithWorkResultHook(agentclient.Options{}, "session")
	callback := options.Hooks.Matchers[sdkhook.EventStop][0].Hooks[0]
	calls := 0
	record := func(context.Context, sdkhook.Input, string) (sdkhook.Output, error) {
		calls++
		return sdkhook.Output{}, nil
	}
	invoke := func(input sdkhook.Input) {
		t.Helper()
		if _, err := callback(t.Context(), input, ""); err != nil {
			t.Fatal(err)
		}
	}
	invoke(sdkhook.Input{})
	manager.SetWorkResultHook("session", "first", record)
	invoke(sdkhook.Input{AgentID: "child"})
	manager.SetWorkResultHook("session", "second", record)
	invoke(sdkhook.Input{})
	manager.ClearWorkResultHook("session", "first")
	invoke(sdkhook.Input{})
	manager.ClearWorkResultHook("session", "second")
	invoke(sdkhook.Input{})
	if calls != 1 {
		t.Fatalf("ambiguous, child or cleared round was routed: calls=%d", calls)
	}
}
