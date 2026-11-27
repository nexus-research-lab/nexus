// execution_context_test.go 覆盖缓存面、命令上下文、上下文用量、输入块排序、中断与结果 Hook 测试。
package runtime

import (
	"context"
	"testing"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestRuntimeCommandContextReadsCurrentSDKSessionIdentity(t *testing.T) {
	identity := NewSDKSessionIdentityState("")
	commandContext := RuntimeCommandContext{SDKSessionIdentity: identity}
	if got := commandContext.CurrentSDKSessionID(); got != "" {
		t.Fatalf("initial SDK session identity = %q", got)
	}
	identity.Set(" sdk-session-fresh ")
	if got := commandContext.CurrentSDKSessionID(); got != "sdk-session-fresh" {
		t.Fatalf("updated SDK session identity = %q", got)
	}
	identity.Set("sdk-session-forked")
	if got := commandContext.CurrentSDKSessionID(); got != "sdk-session-forked" {
		t.Fatalf("forked SDK session identity = %q", got)
	}
}

func TestManagerRecordContextUsageAfterRuntimeReplacement(t *testing.T) {
	manager := NewManager()
	sessionKey := "agent:agent-a:ws:dm:session-a"
	manager.mu.Lock()
	manager.ensureStateLocked(sessionKey).ContextUsageByAgent = nil
	manager.mu.Unlock()

	manager.RecordContextUsage(sessionKey, "agent-a", protocol.ContextUsageData{
		TotalTokens: 10,
		MaxTokens:   100,
		Percentage:  10,
	})

	if snapshots := manager.ContextUsageSnapshots(sessionKey); len(snapshots) != 1 {
		t.Fatalf("ContextUsageSnapshots() = %#v, want replacement runtime snapshot", snapshots)
	}
}

func TestNormalizeContextUsageKeepsNamedCategories(t *testing.T) {
	usage, valid := normalizeContextUsage(agentclient.ContextUsageResponse{
		TotalTokens: 30,
		MaxTokens:   100,
		Categories: []agentclient.ContextUsageCategory{
			{Name: " Messages ", Tokens: 30, Color: "#fff"},
			{Name: "", Tokens: 5},
			{Name: "Skills", Tokens: 0},
			{Name: "Deferred MCP tools", Tokens: 12, IsDeferred: true},
		},
	})
	if !valid {
		t.Fatal("normalizeContextUsage() valid = false, want true")
	}
	want := []protocol.ContextUsageCategory{
		{Name: "Messages", Tokens: 30},
		{Name: "Deferred MCP tools", Tokens: 12, Deferred: true},
	}
	if len(usage.Categories) != len(want) {
		t.Fatalf("categories = %#v, want %#v", usage.Categories, want)
	}
	for index := range want {
		if usage.Categories[index] != want[index] {
			t.Fatalf("categories = %#v, want %#v", usage.Categories, want)
		}
	}
}

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
