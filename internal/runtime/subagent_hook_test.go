package runtime

import (
	"context"
	"strings"
	"testing"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestSubagentAdmissionHookFailsClosedWithoutCurrentCallbacks(t *testing.T) {
	manager := NewManager()
	options := manager.WithSubagentAdmissionHooks(agentclient.Options{}, "runtime-session")
	matchers := options.Hooks.Matchers[sdkhook.EventPreToolUse]
	if len(matchers) != 1 || matchers[0].Matcher != "Agent" {
		t.Fatalf("PreToolUse matchers = %#v", matchers)
	}
	output, err := matchers[0].Hooks[0](context.Background(), sdkhook.Input{
		EventName: sdkhook.EventPreToolUse,
		ToolName:  "Agent",
	}, "tool-1")
	if err != nil {
		t.Fatal(err)
	}
	if output.SpecificOutput == nil ||
		!strings.Contains(output.SpecificOutput.PermissionDecisionReason, subagentHookUnavailableCode) {
		t.Fatalf("missing fail-closed denial: %#v", output)
	}
}

func TestSubagentAdmissionHookUsesCurrentPhysicalRoundCallbacks(t *testing.T) {
	manager := NewManager()
	options := manager.WithSubagentAdmissionHooks(agentclient.Options{}, "runtime-session")
	var firstCalls, secondCalls int
	manager.SetSubagentHookCallbacks("runtime-session", "round-1", SubagentHookCallbacks{
		PreToolUse: func(context.Context, sdkhook.Input, string) (sdkhook.Output, error) {
			firstCalls++
			return sdkhook.Output{}, nil
		},
	})
	callback := options.Hooks.Matchers[sdkhook.EventPreToolUse][0].Hooks[0]
	if _, err := callback(context.Background(), sdkhook.Input{ToolName: "Agent"}, "tool-1"); err != nil {
		t.Fatal(err)
	}
	manager.ClearSubagentHookCallbacks("runtime-session", "round-1")
	manager.SetSubagentHookCallbacks("runtime-session", "round-2", SubagentHookCallbacks{
		PreToolUse: func(context.Context, sdkhook.Input, string) (sdkhook.Output, error) {
			secondCalls++
			return sdkhook.Output{}, nil
		},
	})
	if _, err := callback(context.Background(), sdkhook.Input{ToolName: "Agent"}, "tool-2"); err != nil {
		t.Fatal(err)
	}
	if firstCalls != 1 || secondCalls != 1 {
		t.Fatalf("callback calls = first:%d second:%d", firstCalls, secondCalls)
	}
}

func TestSubagentAdmissionHookIgnoresNonAgentTool(t *testing.T) {
	manager := NewManager()
	options := manager.WithSubagentAdmissionHooks(agentclient.Options{}, "runtime-session")
	called := false
	manager.SetSubagentHookCallbacks("runtime-session", "round-1", SubagentHookCallbacks{
		PreToolUse: func(context.Context, sdkhook.Input, string) (sdkhook.Output, error) {
			called = true
			return sdkhook.Output{}, nil
		},
	})
	output, err := options.Hooks.Matchers[sdkhook.EventPreToolUse][0].Hooks[0](
		context.Background(),
		sdkhook.Input{ToolName: "Read"},
		"tool-1",
	)
	if err != nil {
		t.Fatal(err)
	}
	if called || output.SpecificOutput != nil || output.Continue != nil {
		t.Fatalf("non-Agent hook should be a no-op: called=%t output=%#v", called, output)
	}
}

func TestSubagentLifecycleHookFailsClosedWithoutBindingCallback(t *testing.T) {
	manager := NewManager()
	options := manager.WithSubagentAdmissionHooks(agentclient.Options{}, "runtime-session")
	output, err := options.Hooks.Matchers[sdkhook.EventSubagentStart][0].Hooks[0](
		context.Background(),
		sdkhook.Input{EventName: sdkhook.EventSubagentStart, AgentID: "sdk-agent-1"},
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	if output.Continue == nil || *output.Continue ||
		!strings.Contains(output.StopReason, subagentHookCorrelationCode) {
		t.Fatalf("SubagentStart should fail closed: %#v", output)
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

func TestSubagentLifecycleFailsClosedWhenIdentityIsAmbiguous(t *testing.T) {
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
	if _, err := preToolUse(
		context.Background(),
		sdkhook.Input{ToolName: "Agent"},
		"tool-old",
	); err != nil {
		t.Fatal(err)
	}
	if _, err := subagentStart(
		context.Background(),
		sdkhook.Input{AgentID: "reused-sdk-agent"},
		"",
	); err != nil {
		t.Fatal(err)
	}
	if _, err := subagentStop(
		context.Background(),
		sdkhook.Input{AgentID: "reused-sdk-agent"},
		"",
	); err != nil {
		t.Fatal(err)
	}
	manager.ClearSubagentHookCallbacks("runtime-session", "round-old")

	manager.SetSubagentHookCallbacks("runtime-session", "round-new", lifecycleTestCallbacks(
		&newStarts,
		&newStops,
	))
	if _, err := preToolUse(
		context.Background(),
		sdkhook.Input{ToolName: "Agent"},
		"tool-new",
	); err != nil {
		t.Fatal(err)
	}
	output, err := subagentStart(
		context.Background(),
		sdkhook.Input{AgentID: "reused-sdk-agent"},
		"",
	)
	if err != nil {
		t.Fatal(err)
	}
	if output.Continue == nil || *output.Continue ||
		!strings.Contains(output.StopReason, subagentHookAmbiguousCode) {
		t.Fatalf("ambiguous lifecycle must fail closed: %#v", output)
	}
	if oldStarts != 1 || oldStops != 1 || newStarts != 0 || newStops != 0 {
		t.Fatalf(
			"ambiguous lifecycle was misrouted old=(%d,%d) new=(%d,%d)",
			oldStarts,
			oldStops,
			newStarts,
			newStops,
		)
	}
}

func TestSubagentLifecycleReconcilesMissingTerminalEventAfterParentRoundExit(t *testing.T) {
	manager := NewManager()
	options := manager.WithSubagentAdmissionHooks(agentclient.Options{}, "runtime-session")
	preToolUse := options.Hooks.Matchers[sdkhook.EventPreToolUse][0].Hooks[0]
	subagentStart := options.Hooks.Matchers[sdkhook.EventSubagentStart][0].Hooks[0]
	subagentStop := options.Hooks.Matchers[sdkhook.EventSubagentStop][0].Hooks[0]

	var failures, stops, schedules int
	manager.SetSubagentHookCallbacks("runtime-session", "round-1", SubagentHookCallbacks{
		PreToolUse: func(context.Context, sdkhook.Input, string) (sdkhook.Output, error) {
			return sdkhook.Output{}, nil
		},
		PostToolUseFailure: func(
			_ context.Context,
			input sdkhook.Input,
			toolUseID string,
		) (sdkhook.Output, error) {
			failures++
			if toolUseID != "tool-1" ||
				!input.IsInterrupt ||
				!strings.Contains(input.Error, "parent runtime round ended") {
				t.Fatalf("reconcile input = %#v tool=%q", input, toolUseID)
			}
			return sdkhook.Output{}, nil
		},
		SubagentStart: func(context.Context, sdkhook.Input, string) (sdkhook.Output, error) {
			return sdkhook.Output{}, nil
		},
		SubagentStop: func(context.Context, sdkhook.Input, string) (sdkhook.Output, error) {
			stops++
			return sdkhook.Output{}, nil
		},
		ParentRoundExit: func(
			_ context.Context,
			input SubagentRoundExitInput,
		) error {
			schedules++
			if input.ToolUseID != "tool-1" ||
				input.SDKSessionID != "sdk-session" ||
				input.SDKAgentID != "sdk-agent" ||
				input.ParentRoundExitedAt.IsZero() ||
				input.ReconcileAfter.Sub(input.ParentRoundExitedAt) !=
					protocol.SubagentReconciliationGrace {
				t.Fatalf("durable reconciliation input = %#v", input)
			}
			return nil
		},
	})
	if _, err := preToolUse(
		context.Background(),
		sdkhook.Input{ToolName: "Agent", SessionID: "sdk-session"},
		"tool-1",
	); err != nil {
		t.Fatal(err)
	}
	if _, err := subagentStart(
		context.Background(),
		sdkhook.Input{SessionID: "sdk-session", AgentID: "sdk-agent"},
		"",
	); err != nil {
		t.Fatal(err)
	}
	manager.ClearSubagentHookCallbacks("runtime-session", "round-1")
	if schedules != 1 {
		t.Fatalf("durable reconciliation schedules = %d, want 1", schedules)
	}
	manager.expireSubagentHookBinding(
		"runtime-session",
		subagentHookBinding{
			Sequence:  1,
			RoundID:   "round-1",
			ToolUseID: "tool-1",
		},
		1,
	)
	if failures != 1 {
		t.Fatalf("reconcile failures = %d, want 1", failures)
	}
	if _, err := subagentStop(
		context.Background(),
		sdkhook.Input{SessionID: "sdk-session", AgentID: "sdk-agent"},
		"tool-1",
	); err != nil {
		t.Fatal(err)
	}
	if stops != 1 {
		t.Fatalf("late Stop calls = %d, want 1", stops)
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
