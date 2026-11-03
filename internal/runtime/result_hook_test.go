package runtime

import (
	"context"
	"testing"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type resultCapabilityClient struct {
	*fakeRuntimeClient
	ack, policy bool
}

func (c *resultCapabilityClient) Supports(capability agentclient.Capability) bool {
	return capability == agentclient.CapabilityHookResponseAck && c.ack || capability == agentclient.CapabilityMessageExecutionPolicy && c.policy
}

func TestResultHookRequiresCapabilitiesAndRoutesLoneRound(t *testing.T) {
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

func TestResultHookRoutesConcurrentRoundsByDeclaredRound(t *testing.T) {
	manager := NewManagerWithFactory(&fakeRuntimeFactory{client: &fakeRuntimeClient{}})
	callback := manager.WithWorkResultHook(agentclient.Options{}, "session").Hooks.Matchers[sdkhook.EventStop][0].Hooks[0]
	routed := map[string]int{}
	register := func(round string) {
		manager.SetWorkResultHook("session", round, func(context.Context, sdkhook.Input, string) (sdkhook.Output, error) {
			routed[round]++
			return sdkhook.Output{}, nil
		})
	}
	register("first")
	register("second")
	declare := func(round string) string {
		return "done\n" + protocol.ExecutionResultMarker + "\n{\"version\":1,\"round_id\":\"" + round + "\",\"delivery\":{\"result_summary\":\"x\"}}\n-->"
	}
	output, err := callback(t.Context(), sdkhook.Input{LastAssistantMessage: declare("second")}, "")
	if err != nil || output.Decision != "" || routed["second"] != 1 || routed["first"] != 0 {
		t.Fatalf("declared round was not routed: output=%+v routed=%v err=%v", output, routed, err)
	}
	output, _ = callback(t.Context(), sdkhook.Input{LastAssistantMessage: declare("other")}, "")
	if output.Decision != "block" || routed["first"]+routed["second"] != 1 {
		t.Fatalf("unmatched declaration must be blocked, not dropped: %+v", output)
	}
	output, _ = callback(t.Context(), sdkhook.Input{LastAssistantMessage: "plain reply"}, "")
	if output.Decision != "" {
		t.Fatalf("plain reply without declaration must pass: %+v", output)
	}
}
