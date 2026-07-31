package runtimehook

import (
	"context"
	"testing"

	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	orchestration "github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

func TestCallbacksBindPreToolUseToHostIdentity(t *testing.T) {
	provider := &fakeProvider{
		admitResult: orchestration.SubagentAdmissionResult{
			Allowed: true,
			Binding: &orchestration.SubagentAttemptBinding{AttemptID: "attempt-child"},
		},
	}
	callbacks := Callbacks(provider, Context{
		Actor: orchestration.ActorContext{
			OwnerUserID: "owner-1",
			SessionKey:  "scope-session",
			ExecutionID: "execution-1",
			AgentID:     "agent-1",
		},
		RuntimeSessionKey: "runtime-session",
		RoomSessionID:     "room-session",
	})
	output, err := callbacks.PreToolUse(context.Background(), sdkhook.Input{
		SessionID: "sdk-session",
		ToolUseID: "input-tool-id",
	}, "callback-tool-id")
	if err != nil {
		t.Fatal(err)
	}
	if output.SpecificOutput != nil || output.Continue != nil {
		t.Fatalf("allowed output = %#v, want no-op", output)
	}
	if provider.launch.ToolUseID != "callback-tool-id" ||
		provider.launch.RuntimeSessionKey != "runtime-session" ||
		provider.launch.RoomSessionID != "room-session" ||
		provider.launch.SDKSessionID != "sdk-session" {
		t.Fatalf("launch input = %#v", provider.launch)
	}
}

func TestCallbacksProjectStructuredAdmissionDenial(t *testing.T) {
	provider := &fakeProvider{
		admitResult: orchestration.SubagentAdmissionResult{
			ReasonCode: orchestration.ErrorCodeAmbiguousAssignment,
			Message:    "choose one Work Item",
		},
	}
	output, err := Callbacks(provider, Context{}).PreToolUse(
		context.Background(),
		sdkhook.Input{},
		"tool-1",
	)
	if err != nil {
		t.Fatal(err)
	}
	if output.SpecificOutput == nil ||
		output.SpecificOutput.PermissionDecision != sdkpermission.BehaviorDeny ||
		output.SpecificOutput.PermissionDecisionReason !=
			"[ambiguous_assignment] choose one Work Item" {
		t.Fatalf("denial output = %#v", output)
	}
}

func TestCallbacksForwardSubagentStopWithoutInventingTaskIdentity(t *testing.T) {
	provider := &fakeProvider{
		stopResult: orchestration.SubagentAdmissionResult{
			Allowed: true,
			Binding: &orchestration.SubagentAttemptBinding{AttemptID: "attempt-child"},
		},
	}
	output, err := Callbacks(provider, Context{}).SubagentStop(
		context.Background(),
		sdkhook.Input{
			SessionID:            "sdk-session",
			AgentID:              "sdk-agent",
			AgentType:            "researcher",
			ToolUseID:            "unrelated-stop-tool",
			AgentTranscriptPath:  "/tmp/subagent.jsonl",
			LastAssistantMessage: "done",
		},
		"unrelated-callback-tool",
	)
	if err != nil {
		t.Fatal(err)
	}
	if output.Continue != nil || output.SpecificOutput != nil {
		t.Fatalf("stop output = %#v, want no-op", output)
	}
	if provider.stop.SDKAgentID != "sdk-agent" ||
		provider.stop.AgentType != "researcher" ||
		provider.stop.AgentTranscriptPath != "/tmp/subagent.jsonl" ||
		provider.stop.ToolUseID != "" {
		t.Fatalf("stop input = %#v", provider.stop)
	}
}

type fakeProvider struct {
	admitResult orchestration.SubagentAdmissionResult
	startResult orchestration.SubagentAdmissionResult
	stopResult  orchestration.SubagentAdmissionResult
	launch      orchestration.SubagentLaunchInput
	start       orchestration.SubagentLifecycleInput
	stop        orchestration.SubagentLifecycleInput
}

func (f *fakeProvider) AdmitSubagentLaunch(
	_ context.Context,
	_ orchestration.ActorContext,
	input orchestration.SubagentLaunchInput,
) (orchestration.SubagentAdmissionResult, error) {
	f.launch = input
	return f.admitResult, nil
}

func (f *fakeProvider) ObserveSubagentStart(
	_ context.Context,
	_ orchestration.ActorContext,
	input orchestration.SubagentLifecycleInput,
) (orchestration.SubagentAdmissionResult, error) {
	f.start = input
	return f.startResult, nil
}

func (f *fakeProvider) ObserveSubagentStop(
	_ context.Context,
	_ orchestration.ActorContext,
	input orchestration.SubagentLifecycleInput,
) (orchestration.SubagentAdmissionResult, error) {
	f.stop = input
	return f.stopResult, nil
}

func (f *fakeProvider) ObserveSubagentParentRoundExit(
	_ context.Context,
	_ orchestration.ActorContext,
	_ orchestration.SubagentParentRoundExitInput,
) (orchestration.SubagentAdmissionResult, error) {
	return orchestration.SubagentAdmissionResult{
		Allowed: true,
		Binding: &orchestration.SubagentAttemptBinding{AttemptID: "attempt-child"},
	}, nil
}
