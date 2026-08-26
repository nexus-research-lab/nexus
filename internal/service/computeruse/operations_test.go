// INPUT: round-scoped semantic operations and a deterministic typed RuntimeClient.
// OUTPUT: end-to-end policy evidence for target selection, observation, reconciliation, verification, and teardown.
// POS: portable Agent-command-to-typed-client integration test; native drivers remain nexus-cua's gate.
package computeruse

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	nexuscua "github.com/nexus-research-lab/nexus-cua/sdk/go"

	"github.com/nexus-research-lab/nexus/internal/cli/runtimecommand"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
)

type fixedComputerPreferences struct{ enabled bool }

func (preferences fixedComputerPreferences) Get(context.Context, string) (preferencessvc.Preferences, error) {
	return preferencessvc.Preferences{ComputerUseEnabled: preferences.enabled}, nil
}

type fakeActionHandle struct{ id string }

func (handle fakeActionHandle) RequestID() string { return handle.id }
func (fakeActionHandle) privateActionHandle()     {}

type fakeComputerClient struct {
	RuntimeClient
	mu             sync.Mutex
	performCalls   int
	reconcileCalls int
	closeCalls     int
	observation    int
}

func (client *fakeComputerClient) GetCapabilities(context.Context, time.Duration) (nexuscua.DriverCapabilities, error) {
	return nexuscua.DriverCapabilities{
		ProtocolVersion: ProtocolVersion, RuntimeVersion: packageTestVersion,
		Platform: nexuscua.Platform(normalizedReleaseOS("darwin")),
		Actions:  []nexuscua.ActionKind{nexuscua.ActionFocusWindow, nexuscua.ActionTypeText},
	}, nil
}

func (client *fakeComputerClient) DiscoverApplications(context.Context, time.Duration) (nexuscua.DiscoverApplicationsOutput, error) {
	return nexuscua.DiscoverApplicationsOutput{Complete: true, Applications: []nexuscua.DiscoveredApplication{{
		DiscoveryRef: "discovery-1", Name: "Notes", ApplicationID: "com.example.notes",
		Provenance: nexuscua.ApplicationProvenance{Platform: nexuscua.PlatformMacOS},
	}}}, nil
}

func (client *fakeComputerClient) OpenSession(context.Context, nexuscua.OpenSessionInput, time.Duration) (nexuscua.OpenSessionOutput, error) {
	return nexuscua.OpenSessionOutput{SessionID: "session-1"}, nil
}

func (client *fakeComputerClient) CloseSession(context.Context, nexuscua.SessionID, time.Duration) error {
	client.mu.Lock()
	client.closeCalls++
	client.mu.Unlock()
	return nil
}

func (client *fakeComputerClient) ListApps(context.Context, nexuscua.SessionID, time.Duration) ([]nexuscua.ApplicationSummary, error) {
	return []nexuscua.ApplicationSummary{{AppRef: "app-1", Name: "Notes", ApplicationID: "com.example.notes"}}, nil
}

func (client *fakeComputerClient) ListWindows(context.Context, nexuscua.SessionID, *nexuscua.AppRef, time.Duration) ([]nexuscua.WindowSummary, error) {
	return []nexuscua.WindowSummary{{WindowRef: "window-1", AppRef: "app-1", Title: "Notes", Visible: true}}, nil
}

func (client *fakeComputerClient) ObserveWindow(context.Context, nexuscua.ObserveWindowInput, time.Duration) (nexuscua.WindowObservation, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	client.observation++
	return nexuscua.WindowObservation{
		ObservationID: nexuscua.ObservationID(fmt.Sprintf("observation-%d", client.observation)),
		WindowRef:     "window-1", CapturedAt: "2026-08-26T00:00:00Z", ElementsComplete: true,
	}, nil
}

func (client *fakeComputerClient) VerifyState(context.Context, nexuscua.VerifyStateInput, time.Duration) (nexuscua.VerificationOutput, error) {
	return nexuscua.VerificationOutput{Matched: true, Evidence: "window title contains Notes"}, nil
}

func (client *fakeComputerClient) PerformAction(context.Context, nexuscua.PerformActionInput, time.Duration) (nexuscua.ActionOutput, ActionHandle, error) {
	client.mu.Lock()
	client.performCalls++
	client.mu.Unlock()
	return nexuscua.ActionOutput{}, fakeActionHandle{id: "native-action-1"}, &nexuscua.CUAError{
		Code: nexuscua.ErrorDeadlineExceeded, Message: "action wait expired", Retryable: true,
		MutationStatus: nexuscua.MutationIndeterminate,
	}
}

func (client *fakeComputerClient) ReconcileAction(context.Context, ActionHandle, time.Duration) (nexuscua.ActionOutput, error) {
	client.mu.Lock()
	client.reconcileCalls++
	client.mu.Unlock()
	return nexuscua.ActionOutput{Dispatched: true, ObservationInvalidated: true}, nil
}

func TestComputerOperationsSelectReconcileVerifyAndRoundCleanup(t *testing.T) {
	client := &fakeComputerClient{}
	supervisor := &Supervisor{
		packages: &PackageManager{},
		config:   SupervisorConfig{GOOS: "darwin", Now: time.Now},
		status:   SidecarStatus{State: SidecarReady, Epoch: 1},
		process: &supervisedProcess{
			epoch: 1, client: client,
			capabilities: nexuscua.DriverCapabilities{
				ProtocolVersion: ProtocolVersion, RuntimeVersion: packageTestVersion,
				Platform: nexuscua.PlatformMacOS, Actions: []nexuscua.ActionKind{nexuscua.ActionFocusWindow},
			},
			stdout: newTailBuffer(1024), stderr: newTailBuffer(1024),
		},
	}
	service := NewService(true, fixedComputerPreferences{enabled: true}, nil, supervisor)
	resources := runtimecommand.NewRoundResources()
	actor := runtimecommand.Actor{
		OwnerUserID: "owner-1", AgentID: "agent-1", WorkspacePath: t.TempDir(),
		SessionKey: "session-1", RoundID: "round-1", LeaseSessionKey: "session-1", LeaseRoundID: "round-1",
		Round: runtimecommand.RoundContext{Resources: resources},
	}
	operations := service.Operations(actor)

	selectOperation, _ := runtimecommand.FindOperation(operations, "select_target")
	selected, err := selectOperation.Invoke(context.Background(), map[string]any{
		"application_selector": "Notes", "include_screenshot": false,
	}, &runtimecommand.CallContext{RequestID: "select-target-1"})
	if err != nil || selected.IsError {
		t.Fatalf("select_target = %+v, %v", selected, err)
	}

	actionOperation, _ := runtimecommand.FindOperation(operations, "perform_action")
	first, err := actionOperation.Invoke(context.Background(), map[string]any{"kind": "focus_window"}, &runtimecommand.CallContext{RequestID: "action-request-1"})
	if err != nil || !first.IsError || first.StructuredContent["mutation_status"] != string(nexuscua.MutationIndeterminate) {
		t.Fatalf("first perform_action = %+v, %v", first, err)
	}
	second, err := actionOperation.Invoke(context.Background(), map[string]any{"kind": "focus_window"}, &runtimecommand.CallContext{RequestID: "action-request-1"})
	if err != nil || second.IsError || second.StructuredContent["outcome"] != "applied" {
		t.Fatalf("reconciled perform_action = %+v, %v", second, err)
	}
	conflict, _ := actionOperation.Invoke(context.Background(), map[string]any{"kind": "type_text", "text": "different"}, &runtimecommand.CallContext{RequestID: "action-request-1"})
	if !conflict.IsError || conflict.StructuredContent["reason_code"] != "request_conflict" {
		t.Fatalf("conflicting perform_action = %+v", conflict)
	}

	verifyOperation, _ := runtimecommand.FindOperation(operations, "verify_state")
	verified, err := verifyOperation.Invoke(context.Background(), map[string]any{"kind": "window_title_contains", "text": "Notes"}, &runtimecommand.CallContext{RequestID: "verify-state-1"})
	if err != nil || verified.IsError || verified.StructuredContent["matched"] != true {
		t.Fatalf("verify_state = %+v, %v", verified, err)
	}

	client.mu.Lock()
	if client.performCalls != 1 || client.reconcileCalls != 1 {
		t.Fatalf("native calls = perform %d reconcile %d", client.performCalls, client.reconcileCalls)
	}
	client.mu.Unlock()
	resources.Close()
	client.mu.Lock()
	closeCalls := client.closeCalls
	client.mu.Unlock()
	if closeCalls != 1 {
		t.Fatalf("CloseSession calls = %d", closeCalls)
	}
	if service.findRound(actor) != nil {
		t.Fatal("round state survived physical round cleanup")
	}
}

func TestComputerOperationsRejectDisabledOwnerWithoutStartingSidecar(t *testing.T) {
	service := NewService(true, fixedComputerPreferences{enabled: false}, nil, &Supervisor{})
	actor := runtimecommand.Actor{
		OwnerUserID: "owner-1", AgentID: "agent-1", SessionKey: "session-1", RoundID: "round-1",
		LeaseSessionKey: "session-1", LeaseRoundID: "round-1", Round: runtimecommand.RoundContext{Resources: runtimecommand.NewRoundResources()},
	}
	operation, _ := runtimecommand.FindOperation(service.Operations(actor), "list_applications")
	result, err := operation.Invoke(context.Background(), map[string]any{}, &runtimecommand.CallContext{RequestID: "list-apps-1"})
	if err != nil || !result.IsError || result.StructuredContent["reason_code"] != "disabled" {
		t.Fatalf("list_applications = %+v, %v", result, err)
	}
}
