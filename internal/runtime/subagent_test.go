// subagent_test.go 覆盖子代理控制绑定与生命周期 Hook 准入测试。
package runtime

import (
	"context"
	"strings"
	"testing"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
)

type subagentControlClient struct {
	fakeRuntimeClient
	calls int
}

func (c *subagentControlClient) ControlSubagent(_ context.Context, id, operation string, _ map[string]any) (map[string]any, error) {
	c.calls++
	return map[string]any{"id": id, "operation": operation}, nil
}
func TestSubagentControlCannotBorrowSuccessorRound(t *testing.T) {
	manager := NewManager()
	client := &subagentControlClient{}
	manager.sessions["parent"] = &sessionState{Client: client, SubagentHooks: map[string]SubagentHookCallbacks{"round-1": {}}}
	control := manager.BindSubagentControl("parent", "round-1")
	if _, err := control(context.Background(), "tool-1", "list", nil); err != nil {
		t.Fatal(err)
	}
	delete(manager.sessions["parent"].SubagentHooks, "round-1")
	manager.sessions["parent"].SubagentHooks["round-2"] = SubagentHookCallbacks{}
	if _, err := control(context.Background(), "tool-2", "spawn", nil); err == nil {
		t.Fatal("old round borrowed successor")
	}
	if client.calls != 1 {
		t.Fatal("expired round reached runtime")
	}
	if _, err := manager.BindSubagentControl("other-parent", "round-2")(context.Background(), "tool-3", "list", nil); err == nil {
		t.Fatal("foreign session accepted")
	}
}

func TestSubagentLifecycleKeepsImmutableParentAcrossSuccessorRound(t *testing.T) {
	manager := NewManager()
	options := manager.WithSubagentAdmissionHooks(agentclient.Options{}, "runtime-session")
	preToolUse := options.Hooks.Matchers[sdkhook.EventPreToolUse][0].Hooks[0]
	subagentStart := options.Hooks.Matchers[sdkhook.EventSubagentStart][0].Hooks[0]
	subagentStop := options.Hooks.Matchers[sdkhook.EventSubagentStop][0].Hooks[0]

	var oldStarts, oldStops, newStarts, newStops int
	manager.SetSubagentHookCallbacks("runtime-session", "round-old", lifecycleTestCallbacks(
		&oldStarts,
		&oldStops,
	))
	if _, err := preToolUse(context.Background(), sdkhook.Input{
		ToolName:  "Agent",
		SessionID: "sdk-session",
	}, "tool-old"); err != nil {
		t.Fatal(err)
	}
	if _, err := subagentStart(context.Background(), sdkhook.Input{
		SessionID: "sdk-session",
		AgentID:   "sdk-agent-old",
	}, ""); err != nil {
		t.Fatal(err)
	}
	manager.ClearSubagentHookCallbacks("runtime-session", "round-old")

	manager.SetSubagentHookCallbacks("runtime-session", "round-new", lifecycleTestCallbacks(
		&newStarts,
		&newStops,
	))
	if _, err := preToolUse(context.Background(), sdkhook.Input{
		ToolName:  "Agent",
		SessionID: "sdk-session",
	}, "tool-new"); err != nil {
		t.Fatal(err)
	}
	lateStart, err := subagentStart(context.Background(), sdkhook.Input{
		SessionID: "sdk-session",
		AgentID:   "sdk-agent-old",
	}, "")
	if err != nil {
		t.Fatal(err)
	}
	if lateStart.Continue == nil || *lateStart.Continue ||
		!strings.Contains(lateStart.StopReason, subagentHookAmbiguousCode) {
		t.Fatalf("late old Start must not bind to the successor: %#v", lateStart)
	}
	if _, err := subagentStart(context.Background(), sdkhook.Input{
		SessionID: "sdk-session",
		AgentID:   "sdk-agent-new",
	}, ""); err != nil {
		t.Fatal(err)
	}

	for range 2 {
		output, err := subagentStop(context.Background(), sdkhook.Input{
			SessionID: "sdk-session",
			AgentID:   "sdk-agent-old",
		}, "")
		if err != nil {
			t.Fatal(err)
		}
		if output.Continue != nil && !*output.Continue {
			t.Fatalf("late old lifecycle should stay admitted: %#v", output)
		}
	}
	if oldStarts != 1 || oldStops != 2 || newStarts != 1 || newStops != 0 {
		t.Fatalf(
			"lifecycle calls old=(%d,%d) new=(%d,%d)",
			oldStarts,
			oldStops,
			newStarts,
			newStops,
		)
	}
}

func lifecycleTestCallbacks(starts *int, stops *int) SubagentHookCallbacks {
	return SubagentHookCallbacks{
		PreToolUse: func(context.Context, sdkhook.Input, string) (sdkhook.Output, error) {
			return sdkhook.Output{}, nil
		},
		SubagentStart: func(context.Context, sdkhook.Input, string) (sdkhook.Output, error) {
			*starts++
			return sdkhook.Output{}, nil
		},
		SubagentStop: func(context.Context, sdkhook.Input, string) (sdkhook.Output, error) {
			*stops++
			return sdkhook.Output{}, nil
		},
	}
}
