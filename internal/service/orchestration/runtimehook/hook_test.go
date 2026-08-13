package runtimehook

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
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

func TestCallbacksReadDynamicActorAtEachSubagentBoundary(t *testing.T) {
	provider := &fakeProvider{admitResult: orchestration.SubagentAdmissionResult{Allowed: true}}
	current := orchestration.ActorContext{AgentID: "agent-lead"}
	callbacks := Callbacks(provider, Context{
		Actor:         orchestration.ActorContext{AgentID: "stale-agent"},
		ActorProvider: func() orchestration.ActorContext { return current },
	})
	if _, err := callbacks.PreToolUse(context.Background(), sdkhook.Input{}, "tool-1"); err != nil {
		t.Fatal(err)
	}
	if provider.actor.AgentID != "agent-lead" || provider.actor.WorkBinding != nil {
		t.Fatalf("first dynamic actor = %+v", provider.actor)
	}
	current.WorkBinding = &protocol.ExecutionWorkBinding{
		ExecutionID: "execution-1", PlanID: "plan-1", WorkItemID: "work-1",
		SpecID: "spec-1", AssignmentID: "assignment-1", AttemptID: "attempt-1",
	}
	if _, err := callbacks.PreToolUse(context.Background(), sdkhook.Input{}, "tool-2"); err != nil {
		t.Fatal(err)
	}
	if provider.actor.WorkBinding == nil || provider.actor.WorkBinding.AssignmentID != "assignment-1" {
		t.Fatalf("bound dynamic actor = %+v", provider.actor)
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

func TestCallbacksAllowRuntimeOnlySubagentWithoutDurableBinding(t *testing.T) {
	provider := &fakeProvider{
		admitResult: orchestration.SubagentAdmissionResult{
			Allowed: true,
			Mode:    orchestration.SubagentAdmissionRuntimeOnly,
		},
		parentResult: orchestration.SubagentAdmissionResult{
			Allowed: true,
			Mode:    orchestration.SubagentAdmissionRuntimeOnly,
		},
	}
	callbacks := Callbacks(provider, Context{})
	output, err := callbacks.PreToolUse(
		context.Background(),
		sdkhook.Input{},
		"runtime-only-tool",
	)
	if err != nil {
		t.Fatal(err)
	}
	if output.SpecificOutput != nil || output.Continue != nil {
		t.Fatalf("runtime-only output = %#v, want no-op", output)
	}
	now := time.Date(2030, time.January, 2, 3, 4, 5, 0, time.UTC)
	if err = callbacks.ParentRoundExit(context.Background(), orchestrationRoundExitInput(now)); err != nil {
		t.Fatalf("runtime-only parent exit = %v", err)
	}
}

func TestCallbacksLogPersistenceCauseButReturnSanitizedDenial(t *testing.T) {
	var logs bytes.Buffer
	provider := &fakeProvider{admitErr: errors.New("database is locked (SQLITE_BUSY)")}
	output, err := Callbacks(provider, Context{
		Logger: slog.New(slog.NewJSONHandler(&logs, nil)),
	}).PreToolUse(context.Background(), sdkhook.Input{}, "tool-1")
	if err != nil {
		t.Fatal(err)
	}
	if output.SpecificOutput == nil ||
		output.SpecificOutput.PermissionDecisionReason !=
			"[subagent_admission_error] authoritative subagent admission state could not be persisted" {
		t.Fatalf("persistence denial = %#v", output)
	}
	if strings.Contains(output.SpecificOutput.PermissionDecisionReason, "SQLITE_BUSY") ||
		!strings.Contains(logs.String(), "SQLITE_BUSY") ||
		!strings.Contains(logs.String(), "PreToolUse") {
		t.Fatalf("sanitized denial = %q, internal log = %q", output.SpecificOutput.PermissionDecisionReason, logs.String())
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
	admitResult  orchestration.SubagentAdmissionResult
	admitErr     error
	startResult  orchestration.SubagentAdmissionResult
	stopResult   orchestration.SubagentAdmissionResult
	parentResult orchestration.SubagentAdmissionResult
	launch       orchestration.SubagentLaunchInput
	start        orchestration.SubagentLifecycleInput
	stop         orchestration.SubagentLifecycleInput
	actor        orchestration.ActorContext
}

func (f *fakeProvider) AdmitSubagentLaunch(
	_ context.Context,
	actor orchestration.ActorContext,
	input orchestration.SubagentLaunchInput,
) (orchestration.SubagentAdmissionResult, error) {
	f.actor = actor
	f.launch = input
	return f.admitResult, f.admitErr
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
	if f.parentResult.Allowed || f.parentResult.ReasonCode != "" {
		return f.parentResult, nil
	}
	return orchestration.SubagentAdmissionResult{
		Allowed: true,
		Binding: &orchestration.SubagentAttemptBinding{AttemptID: "attempt-child"},
	}, nil
}

func orchestrationRoundExitInput(now time.Time) runtimectx.SubagentRoundExitInput {
	return runtimectx.SubagentRoundExitInput{
		ParentRoundExitedAt: now,
		ReconcileAfter:      now.Add(30 * time.Second),
	}
}
