package computeruse

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	nexuscua "github.com/nexus-research-lab/nexus-cua/sdk/go"
)

type fakeSupervisor struct {
	client          *fakeClient
	next            *fakeClient
	artifact        string
	starts          int
	stops           int
	restarts        int
	restartNext     bool
	foregroundCalls int
}

func (s *fakeSupervisor) Start(context.Context) (Client, error) {
	if s.restartNext {
		s.restartNext = false
		s.starts++
		if s.next != nil {
			s.client = s.next
		}
		return s.client, nil
	}
	if s.starts > 0 {
		return s.client, nil
	}
	s.starts++
	return s.client, nil
}

func (s *fakeSupervisor) Stop(context.Context) error {
	s.stops++
	s.starts = 0
	return nil
}

func (s *fakeSupervisor) Restart(context.Context) (Client, error) {
	s.restarts++
	s.starts++
	return s.client, nil
}

func (s *fakeSupervisor) ArtifactRoot() string { return s.artifact }

func (s *fakeSupervisor) prepareForeground(context.Context) { s.foregroundCalls++ }

type fakeClient struct {
	windows        []nexuscua.WindowSummary
	lastObserve    nexuscua.ObserveWindowInput
	observeAfter   bool
	actionOutput   *nexuscua.ActionOutput
	actionError    error
	lastInput      nexuscua.PerformActionInput
	sceneCalls     int
	openCalls      int
	manifest       nexuscua.CapabilityManifest
	performCalls   int
	reconcileCalls int
	closeCalls     int
	closeErr       error
	listWindowsErr error
	observeErr     error
	indeterminate  bool
	observation    nexuscua.WindowObservation
}

func (c *fakeClient) GetCapabilities(context.Context, time.Duration) (nexuscua.DriverCapabilities, error) {
	return nexuscua.DriverCapabilities{
		ProtocolVersion: nexuscua.ProtocolVersion,
		RuntimeVersion:  pinnedCUAVersion,
		ObserveAfter:    c.observeAfter,
		ActionLimits:    nexuscua.ActionLimits{MaxSessionTTLSeconds: 3600},
		Platform:        nexuscua.PlatformMacOS,
		Actions:         []nexuscua.ActionKind{nexuscua.ActionClickPoint, nexuscua.ActionTypeText},
	}, nil
}

func (c *fakeClient) GetPermissionStatus(context.Context, time.Duration) (nexuscua.PermissionStatus, error) {
	return nexuscua.PermissionStatus{
		ScreenCapture: nexuscua.PermissionGranted,
		Accessibility: nexuscua.PermissionGranted,
		InputControl:  nexuscua.PermissionGranted,
	}, nil
}

func (c *fakeClient) DiscoverApplications(context.Context, time.Duration) (nexuscua.DiscoverApplicationsOutput, error) {
	return nexuscua.DiscoverApplicationsOutput{
		Complete: true,
		Applications: []nexuscua.DiscoveredApplication{{
			DiscoveryRef: "discovery-1", Name: "Fixture", ApplicationID: "com.example.fixture",
		}},
	}, nil
}

func (c *fakeClient) OpenSession(_ context.Context, input nexuscua.OpenSessionInput, _ time.Duration) (nexuscua.OpenSessionOutput, error) {
	c.openCalls++
	c.manifest = input.Manifest
	return nexuscua.OpenSessionOutput{SessionID: "session-1"}, nil
}

func (c *fakeClient) CloseSession(context.Context, nexuscua.SessionID, time.Duration) error {
	c.closeCalls++
	return c.closeErr
}

func (c *fakeClient) ListApps(context.Context, nexuscua.SessionID, time.Duration) ([]nexuscua.ApplicationSummary, error) {
	return nil, nil
}

func (c *fakeClient) ListWindows(context.Context, nexuscua.SessionID, *nexuscua.AppRef, time.Duration) ([]nexuscua.WindowSummary, error) {
	if c.listWindowsErr != nil {
		return nil, c.listWindowsErr
	}
	if c.windows != nil {
		return c.windows, nil
	}
	return []nexuscua.WindowSummary{{WindowRef: "window-1", Title: "Fixture", Visible: true}}, nil
}

func (c *fakeClient) ObserveWindow(_ context.Context, input nexuscua.ObserveWindowInput, _ time.Duration) (nexuscua.WindowObservation, error) {
	c.lastObserve = input
	if c.observeErr != nil {
		return nexuscua.WindowObservation{}, c.observeErr
	}
	if c.observation.ObservationID != "" {
		return c.observation, nil
	}
	return nexuscua.WindowObservation{ObservationID: "observation-1", WindowRef: input.WindowRef}, nil
}

func TestObserveRetainsExactWindowWithoutGuessingReplacement(t *testing.T) {
	for _, tc := range []struct {
		name     string
		windows  []nexuscua.WindowSummary
		selector string
		want     nexuscua.WindowRef
	}{
		{"selected survives", []nexuscua.WindowSummary{{WindowRef: "window-2"}, {WindowRef: "window-1"}}, "", "window-1"},
		{"explicit wins", []nexuscua.WindowSummary{{WindowRef: "window-1"}, {WindowRef: "window-2"}}, "window-2", "window-2"},
		{"sole replacement", []nexuscua.WindowSummary{{WindowRef: "window-2"}}, "", "window-2"},
		{"ambiguous replacement", []nexuscua.WindowSummary{{WindowRef: "window-2", Title: "Fixture"}, {WindowRef: "window-3"}}, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, client, _ := newFakeService(t)
			round := prepareRound(t, service)
			client.windows = tc.windows
			client.lastObserve = nexuscua.ObserveWindowInput{}
			_, err := round.Observe(context.Background(), ObserveInput{WindowSelector: tc.selector})
			if tc.want == "" {
				if err == nil || client.lastObserve.WindowRef != "" {
					t.Fatal("ambiguous replacement was observed")
				}
			} else if err != nil || client.lastObserve.WindowRef != tc.want {
				t.Fatalf("window=%s, err=%v; want %s", client.lastObserve.WindowRef, err, tc.want)
			}
		})
	}
}

func (c *fakeClient) VerifyState(context.Context, nexuscua.VerifyStateInput, time.Duration) (nexuscua.VerificationOutput, error) {
	return nexuscua.VerificationOutput{Matched: true, Evidence: "fixture"}, nil
}

func TestMetadataObservationPreservesUnknownWindowState(t *testing.T) {
	service, client, _ := newFakeService(t)
	round := prepareRound(t, service)
	client.windows = []nexuscua.WindowSummary{{
		WindowRef: "window-1", State: nexuscua.WindowUnknown, Visible: true,
		ScreenBounds: nexuscua.ScreenRect{Width: 131, Height: 160},
	}}
	observation, err := round.Observe(context.Background(), ObserveInput{Accessibility: "disabled"})
	if err != nil {
		t.Fatal(err)
	}
	status := observation.Data["window_status"].(map[string]any)
	if status["state"] != nexuscua.WindowUnknown || status["foreground"] != false || status["source"] != "list_windows_before_observe" {
		t.Fatalf("lost discovery evidence: %#v", status)
	}
	if client.lastObserve.IncludeScreenshot || client.lastObserve.Accessibility != nexuscua.AccessibilityDisabled {
		t.Fatal("metadata recovery unexpectedly requested capture or AX")
	}
	if !strings.Contains(observation.Data["window_state_hint"].(string), "one authorized focus_window") {
		t.Fatal("unknown thumbnail state lacks focus recovery guidance")
	}
	client.actionError = &nexuscua.CUAError{Code: nexuscua.ErrorTargetUnavailable, MutationStatus: nexuscua.MutationNotDispatched}
	if _, err := round.PerformAction(context.Background(), "restore-failed", ActionInput{Kind: "restore_window"}); err == nil {
		t.Fatal("preflight rejection unexpectedly succeeded")
	}
	client.actionError = nil
	if _, err := round.Observe(context.Background(), ObserveInput{Accessibility: "disabled"}); err != nil {
		t.Fatal(err)
	}
	if _, err := round.PerformAction(context.Background(), "focus-recovery", ActionInput{Kind: "focus_window"}); err != nil {
		t.Fatalf("known preflight rejection blocked recovery: %v", err)
	}
	if client.openCalls != 1 || client.reconcileCalls != 0 {
		t.Fatal("recovery reopened authority or treated preflight as indeterminate")
	}
}

func (c *fakeClient) PerformAction(_ context.Context, input nexuscua.PerformActionInput, _ time.Duration) (nexuscua.ActionOutput, *nexuscua.ActionRequest, error) {
	c.performCalls++
	c.lastInput = input
	if c.actionError != nil {
		return nexuscua.ActionOutput{}, nil, c.actionError
	}
	if c.actionOutput != nil {
		return *c.actionOutput, nil, nil
	}
	if c.indeterminate {
		return nexuscua.ActionOutput{}, &nexuscua.ActionRequest{}, &nexuscua.MutationIndeterminateError{
			Request: &nexuscua.ActionRequest{}, Cause: errors.New("transport timeout"),
		}
	}
	return nexuscua.ActionOutput{Dispatched: true, ObservationInvalidated: true}, nil, nil
}

func (c *fakeClient) ReconcileAction(context.Context, *nexuscua.ActionRequest, time.Duration) (nexuscua.ActionOutput, error) {
	c.reconcileCalls++
	return nexuscua.ActionOutput{Dispatched: true, ObservationInvalidated: true}, nil
}

func newFakeService(t *testing.T) (*Service, *fakeClient, *fakeSupervisor) {
	t.Helper()
	root := t.TempDir()
	client := &fakeClient{}
	supervisor := &fakeSupervisor{client: client, artifact: root}
	return NewServiceWithSupervisor(true, supervisor), client, supervisor
}

func prepareRound(t *testing.T, service *Service) *Round {
	t.Helper()
	round, err := service.Round(context.Background(), "owner-a", "round-a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := round.SelectTarget(context.Background(), "Fixture", TargetOptions{AllowForegroundInput: true}, func(TargetGrant) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := round.Observe(context.Background(), ObserveInput{}); err != nil {
		t.Fatal(err)
	}
	return round
}

func TestServiceDisabledFailsClosed(t *testing.T) {
	service, _, supervisor := newFakeService(t)
	if err := service.SetEnabled(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Round(context.Background(), "owner-a", "round-a"); !errors.Is(err, ErrDisabled) {
		t.Fatalf("Round error = %v, want ErrDisabled", err)
	}
	if supervisor.starts != 0 {
		t.Fatalf("sidecar starts = %d, want 0", supervisor.starts)
	}
}

func TestServiceDisableClosesRoundsAndStopsSupervisor(t *testing.T) {
	service, client, supervisor := newFakeService(t)
	round := prepareRound(t, service)
	if err := service.SetEnabled(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	if client.closeCalls != 1 || supervisor.stops != 1 || round.HasSession() {
		t.Fatalf("disable cleanup close=%d stops=%d session=%v", client.closeCalls, supervisor.stops, round.HasSession())
	}
	if _, err := service.Round(context.Background(), "owner-a", "round-next"); !errors.Is(err, ErrDisabled) {
		t.Fatalf("disabled Round error = %v", err)
	}
}

func TestServiceOwnerAndRoundIsolation(t *testing.T) {
	service, _, supervisor := newFakeService(t)
	roundA, err := service.Round(context.Background(), "owner-a", "round-a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Round(context.Background(), "owner-b", "round-b"); !errors.Is(err, ErrOwnerConflict) {
		t.Fatalf("owner conflict error = %v", err)
	}
	roundB, err := service.Round(context.Background(), "owner-a", "round-b")
	if err != nil {
		t.Fatal(err)
	}
	if roundA == roundB {
		t.Fatal("different round ids shared one round object")
	}
	if supervisor.starts != 1 {
		t.Fatalf("sidecar starts = %d, want one shared epoch", supervisor.starts)
	}
	if err := service.CloseRound(context.Background(), "owner-b", "round-a"); !errors.Is(err, ErrOwnerConflict) {
		t.Fatalf("cross-owner close error = %v", err)
	}
	if err := service.CloseRound(context.Background(), "owner-a", "round-a"); err != nil {
		t.Fatal(err)
	}
	if roundB.HasSession() {
		t.Fatal("closing round-a changed round-b state")
	}
}

func TestServiceCloseRoundByIDCleansTrustedRuntimeFinish(t *testing.T) {
	service, client, _ := newFakeService(t)
	round := prepareRound(t, service)
	if err := service.CloseRoundByID(context.Background(), round.ID()); err != nil {
		t.Fatal(err)
	}
	if round.HasSession() || client.closeCalls != 1 {
		t.Fatalf("round finish cleanup session=%v close_calls=%d", round.HasSession(), client.closeCalls)
	}
	if err := service.CloseRoundByID(context.Background(), round.ID()); err != nil {
		t.Fatal(err)
	}
}

func TestServiceSwitchOwnerRevokesPreviousRounds(t *testing.T) {
	service, _, supervisor := newFakeService(t)
	oldRound := prepareRound(t, service)
	if err := service.SwitchOwner(context.Background(), "owner-b"); err != nil {
		t.Fatal(err)
	}
	if oldRound.HasSession() {
		t.Fatal("old owner round remained active after owner switch")
	}
	if _, err := service.Round(context.Background(), "owner-a", "round-old"); !errors.Is(err, ErrOwnerConflict) {
		t.Fatalf("old owner Round error = %v", err)
	}
	if _, err := service.Round(context.Background(), "owner-b", "round-new"); err != nil {
		t.Fatal(err)
	}
	if supervisor.stops != 1 {
		t.Fatalf("supervisor stops = %d, want one", supervisor.stops)
	}
}

func TestServiceReplacesRoundsAfterSupervisorRestart(t *testing.T) {
	service, _, supervisor := newFakeService(t)
	oldRound := prepareRound(t, service)
	replacement := &fakeClient{}
	supervisor.next = replacement
	supervisor.restartNext = true

	newRound, err := service.Round(context.Background(), "owner-a", "round-a")
	if err != nil {
		t.Fatal(err)
	}
	if newRound == oldRound {
		t.Fatal("sidecar restart reused the old round object")
	}
	if oldRound.HasSession() {
		t.Fatal("old round remained usable after sidecar restart")
	}
	if supervisor.starts != 2 {
		t.Fatalf("sidecar starts = %d, want two epochs", supervisor.starts)
	}
}

func TestSelectTargetRecoversWhenOldRuntimeSessionAlreadyExpired(t *testing.T) {
	service, client, _ := newFakeService(t)
	round := prepareRound(t, service)
	client.closeErr = &nexuscua.CUAError{Code: nexuscua.ErrorSessionUnavailable, Message: "expired"}

	if _, err := round.SelectTarget(context.Background(), "Fixture", TargetOptions{AllowForegroundInput: true}, func(TargetGrant) error { return nil }); err != nil {
		t.Fatalf("SelectTarget after expired session = %v", err)
	}
	if !round.HasSession() {
		t.Fatal("SelectTarget did not establish a replacement session")
	}
}

func TestSessionUnavailableClearsRoundState(t *testing.T) {
	service, client, _ := newFakeService(t)
	round := prepareRound(t, service)
	client.listWindowsErr = &nexuscua.CUAError{Code: nexuscua.ErrorSessionUnavailable, Message: "expired"}
	if _, err := round.Observe(context.Background(), ObserveInput{}); err == nil {
		t.Fatal("Observe unexpectedly succeeded")
	}
	if round.HasSession() {
		t.Fatal("expired runtime session remained visible as active")
	}
}

func TestObserveAcceptsWindowRefReturnedByListWindows(t *testing.T) {
	service, _, _ := newFakeService(t)
	round, err := service.Round(context.Background(), "owner-a", "round-a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := round.SelectTarget(context.Background(), "Fixture", TargetOptions{AllowForegroundInput: true}, func(TargetGrant) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := round.Observe(context.Background(), ObserveInput{WindowSelector: "window-1"}); err != nil {
		t.Fatalf("Observe by listed window_ref = %v", err)
	}
}

func TestSelectedTargetWithoutObservationDoesNotRequireReauthorization(t *testing.T) {
	for _, failedRead := range []bool{false, true} {
		t.Run(fmt.Sprintf("failed_read_%t", failedRead), func(t *testing.T) {
			service, client, _ := newFakeService(t)
			round, err := service.Round(context.Background(), "owner-a", "round-a")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := round.SelectTarget(context.Background(), "Fixture", TargetOptions{}, func(TargetGrant) error { return nil }); err != nil {
				t.Fatal(err)
			}
			if failedRead {
				client.observeErr = &nexuscua.CUAError{Code: nexuscua.ErrorTargetUnavailable, Message: "matching accessibility window is unavailable"}
				if _, err := round.Observe(context.Background(), ObserveInput{}); !errors.Is(err, client.observeErr) {
					t.Fatalf("Observe error = %v", err)
				}
			}
			if !round.HasSession() {
				t.Fatal("selected target session was lost without a successful observation")
			}
			approvals := 0
			_, err = round.PerformActionWithApproval(context.Background(), "no-observation", ActionInput{Kind: "focus_window"}, func() error {
				approvals++
				return nil
			})
			if !errors.Is(err, ErrNoObservation) || approvals != 0 || client.performCalls != 0 {
				t.Fatalf("error/approval/dispatch = %v/%d/%d, want ErrNoObservation/0/0", err, approvals, client.performCalls)
			}
			client.observeErr = nil
			if _, err := round.Observe(context.Background(), ObserveInput{Accessibility: "disabled"}); err != nil {
				t.Fatal(err)
			}
			if _, err := round.PerformAction(context.Background(), "recovered-observation", ActionInput{Kind: "focus_window"}); err != nil {
				t.Fatal(err)
			}
			if client.openCalls != 1 || client.performCalls != 1 {
				t.Fatalf("session/dispatch = %d/%d, want one of each", client.openCalls, client.performCalls)
			}
		})
	}
}

func TestActionViewPreservesEffectEvidenceAndForegroundRestoration(t *testing.T) {
	value := nexuscua.ActionOutput{
		DeliveryMode:           nexuscua.DeliveryForeground,
		Dispatched:             true,
		EffectStatus:           nexuscua.ActionEffectConfirmed,
		EvidenceKind:           nexuscua.ActionEvidenceNativeReadback,
		ForegroundRestoration:  nexuscua.ForegroundRestorationRestored,
		ObservationInvalidated: true,
	}
	got := actionView(value).Data
	for key, want := range map[string]any{
		"effect_status":          value.EffectStatus,
		"evidence_kind":          value.EvidenceKind,
		"foreground_restoration": value.ForegroundRestoration,
	} {
		if got[key] != want {
			t.Fatalf("%s = %#v, want %#v", key, got[key], want)
		}
	}
}

func TestPerformActionWithApprovalSkipsApprovalOnReconcile(t *testing.T) {
	service, client, supervisor := newFakeService(t)
	round := prepareRound(t, service)
	approvals := 0
	approve := func() error {
		approvals++
		return nil
	}
	if _, err := round.PerformActionWithApproval(context.Background(), "action-20261013-callback", ActionInput{Kind: "click_point", X: 1, Y: 2}, approve); err != nil {
		t.Fatal(err)
	}
	if _, err := round.Observe(context.Background(), ObserveInput{}); err != nil {
		t.Fatal(err)
	}
	if _, err := round.PerformActionWithApproval(context.Background(), "action-20261013-callback", ActionInput{Kind: "click_point", X: 9, Y: 9}, approve); err != nil {
		t.Fatal(err)
	}
	if approvals != 1 || supervisor.foregroundCalls != 1 || client.performCalls != 1 || client.reconcileCalls != 0 {
		t.Fatalf("completed callback/dispatch = %d/%d/%d, want 1/1/0", approvals, client.performCalls, client.reconcileCalls)
	}
	if _, err := round.Observe(context.Background(), ObserveInput{}); err != nil {
		t.Fatal(err)
	}
	client.indeterminate = true
	if _, err := round.PerformActionWithApproval(context.Background(), "action-20261013-pending-callback", ActionInput{Kind: "click_point", X: 3, Y: 4}, approve); err == nil {
		t.Fatal("indeterminate action unexpectedly succeeded")
	}
	client.indeterminate = false
	if _, err := round.PerformActionWithApproval(context.Background(), "action-20261013-pending-callback", ActionInput{Kind: "click_point", X: 3, Y: 4}, approve); err != nil {
		t.Fatal(err)
	}
	if approvals != 2 || supervisor.foregroundCalls != 2 || client.performCalls != 2 || client.reconcileCalls != 1 {
		t.Fatalf("pending callback/dispatch = %d/%d/%d, want 2/2/1", approvals, client.performCalls, client.reconcileCalls)
	}
}

func TestPerformActionReconcilesSameRequestWithoutRedispatch(t *testing.T) {
	service, client, _ := newFakeService(t)
	round := prepareRound(t, service)
	client.indeterminate = true
	if _, err := round.PerformAction(context.Background(), "action-20261013-1", ActionInput{Kind: "click_point", X: 1, Y: 2}); err == nil {
		t.Fatal("first action unexpectedly succeeded")
	}
	client.indeterminate = false
	if _, err := round.PerformAction(context.Background(), "action-20261013-1", ActionInput{Kind: "click_point", X: 1, Y: 2}); err != nil {
		t.Fatal(err)
	}
	if client.performCalls != 1 || client.reconcileCalls != 1 {
		t.Fatalf("dispatch/reconcile = %d/%d, want 1/1", client.performCalls, client.reconcileCalls)
	}
}

func TestPerformActionBlocksDifferentRequestWhilePending(t *testing.T) {
	service, client, _ := newFakeService(t)
	round := prepareRound(t, service)
	client.indeterminate = true
	if _, err := round.PerformAction(context.Background(), "action-20261013-a", ActionInput{Kind: "click_point", X: 1, Y: 2}); err == nil {
		t.Fatal("first action unexpectedly succeeded")
	}
	if _, err := round.PerformAction(context.Background(), "action-20261013-b", ActionInput{Kind: "click_point", X: 3, Y: 4}); !errors.Is(err, ErrMutationPending) {
		t.Fatalf("different request error = %v, want ErrMutationPending", err)
	}
	if client.performCalls != 1 {
		t.Fatalf("dispatch count = %d, want one", client.performCalls)
	}
}

func TestPerformActionCachesCompletedRequest(t *testing.T) {
	service, client, _ := newFakeService(t)
	round := prepareRound(t, service)
	first, err := round.PerformAction(context.Background(), "action-20261013-cache", ActionInput{Kind: "click_point", X: 1, Y: 2})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := round.Observe(context.Background(), ObserveInput{}); err != nil {
		t.Fatal(err)
	}
	second, err := round.PerformAction(context.Background(), "action-20261013-cache", ActionInput{Kind: "click_point", X: 9, Y: 9})
	if err != nil {
		t.Fatal(err)
	}
	if client.performCalls != 1 || first.Data["dispatched"] != second.Data["dispatched"] {
		t.Fatalf("dispatch count/results = %d, %#v/%#v", client.performCalls, first.Data, second.Data)
	}
}

func TestScreenshotArtifactCannotEscapePrivateRoot(t *testing.T) {
	service, client, _ := newFakeService(t)
	round, err := service.Round(context.Background(), "owner-a", "round-a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := round.SelectTarget(context.Background(), "Fixture", TargetOptions{AllowForegroundInput: true}, func(TargetGrant) error { return nil }); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.png")
	if err := os.WriteFile(outside, []byte("not a screenshot"), 0o600); err != nil {
		t.Fatal(err)
	}
	client.observation = nexuscua.WindowObservation{
		ObservationID: "observation-1", WindowRef: "window-1",
		Screenshot: &nexuscua.ScreenshotArtifact{Path: outside},
	}
	if _, err := round.Observe(context.Background(), ObserveInput{IncludeScreenshot: true}); err == nil || !strings.Contains(err.Error(), "escaped") {
		t.Fatalf("artifact error = %v, want private-root rejection", err)
	}
}

func TestCloseCleansRoundsAndSupervisor(t *testing.T) {
	service, client, supervisor := newFakeService(t)
	round := prepareRound(t, service)
	if err := service.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if client.closeCalls != 1 || supervisor.stops != 1 || round.HasSession() {
		t.Fatalf("session close=%d supervisor stop=%d session active=%v", client.closeCalls, supervisor.stops, round.HasSession())
	}
	if _, err := service.Round(context.Background(), "owner-a", "round-next"); !errors.Is(err, ErrDisabled) {
		t.Fatalf("post-close Round error = %v", err)
	}
}

func TestNormalizedActionsPreserveCoordinatesAndBoundScroll(t *testing.T) {
	action, err := buildAction(ActionInput{Kind: "scroll", X: 750, Y: 250, DeltaY: 300})
	if err != nil {
		t.Fatal(err)
	}
	scroll, ok := action.(nexuscua.Scroll)
	if !ok || scroll.Point != (nexuscua.NormalizedPoint{X: 750, Y: 250}) || scroll.DeltaY != 300 {
		t.Fatalf("unexpected normalized scroll: %#v", action)
	}
	for _, input := range []ActionInput{
		{Kind: "click_point", X: 1001},
		{Kind: "drag", X: 500, Y: 500, ToX: 1001},
	} {
		if _, err := buildAction(input); err == nil {
			t.Fatalf("accepted out-of-bounds action: %s", input.Kind)
		}
	}
}

func TestRoundAuthorityDoesNotInheritUnexposedCapabilities(t *testing.T) {
	actions := supportedRoundActions([]nexuscua.ActionKind{nexuscua.ActionClickPoint, nexuscua.ActionKind("future_unknown_action")})
	if len(actions) != 1 || actions[0] != nexuscua.ActionClickPoint {
		t.Fatalf("unexpected authority: %v", actions)
	}
}

func TestTargetApprovalScopesAuthorityAndRoundDeadline(t *testing.T) {
	service, client, _ := newFakeService(t)
	round, err := service.Round(context.Background(), "owner-a", "round-a")
	if err != nil {
		t.Fatal(err)
	}
	round.deadline = time.Now().Add(90 * time.Second)
	options := TargetOptions{Actions: []nexuscua.ActionKind{nexuscua.ActionClickPoint}, TTLSeconds: 600}
	if _, err := round.SelectTarget(context.Background(), "Fixture", options, func(TargetGrant) error { return errors.New("deny") }); err == nil || client.openCalls != 0 {
		t.Fatal("denial created authority")
	}
	if _, err := round.SelectTarget(context.Background(), "Fixture", options, nil); err == nil || client.openCalls != 0 {
		t.Fatal("missing approval created authority")
	}
	_, err = round.SelectTarget(context.Background(), "Fixture", options, func(grant TargetGrant) error {
		if grant.Application.ApplicationID != "com.example.fixture" || grant.AllowForegroundInput || grant.TTLSeconds > 90 || len(grant.Actions) != 1 {
			t.Fatalf("wrong grant: %#v", grant)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if client.manifest.AllowForegroundInput || client.manifest.TTLSeconds > 90 || len(client.manifest.AllowedActions) != 1 {
		t.Fatalf("grant widened: %#v", client.manifest)
	}
	round.sessionExpiresAt = time.Now().Add(-time.Second)
	if _, err := round.PerformAction(context.Background(), "expired", ActionInput{Kind: "click_point"}); err == nil || client.performCalls != 0 {
		t.Fatal("expired session dispatched")
	}
	if client.openCalls != 1 {
		t.Fatal("session silently renewed")
	}
}

func (c *fakeClient) ObserveApplicationScene(ctx context.Context, input nexuscua.ObserveWindowInput, timeout time.Duration) (nexuscua.WindowObservation, error) {
	c.sceneCalls++
	return c.ObserveWindow(ctx, input, timeout)
}

func TestPostActionObservationAdvancesGuardAndDoesNotCachePixels(t *testing.T) {
	service, client, supervisor := newFakeService(t)
	client.observeAfter = true
	round := prepareRound(t, service)
	if _, err := round.Observe(context.Background(), ObserveInput{ApplicationScene: true}); err != nil {
		t.Fatal(err)
	}
	image := filepath.Join(supervisor.artifact, "after.png")
	if err := os.WriteFile(image, []byte("\x89PNG\r\n\x1a\nfixture"), 0600); err != nil {
		t.Fatal(err)
	}
	client.actionOutput = &nexuscua.ActionOutput{Dispatched: true, ObservationInvalidated: true,
		Observation: &nexuscua.WindowObservation{ObservationID: "after", WindowRef: "window-1",
			Screenshot: &nexuscua.ScreenshotArtifact{Path: image, MIMEType: "image/png"}},
	}
	result, err := round.PerformAction(context.Background(), "post-action", ActionInput{Kind: "click_point"})
	if err != nil {
		t.Fatal(err)
	}
	if result.ImageData == "" || result.Data["observation"] == nil || round.observationID != "after" {
		t.Fatal("fresh observation was not delivered")
	}
	if client.lastInput.ObserveAfter == nil || !client.lastInput.ObserveAfter.ApplicationScene || client.sceneCalls != 1 {
		t.Fatal("observation options were not preserved")
	}
	cached, err := round.PerformAction(context.Background(), "post-action", ActionInput{Kind: "click_point"})
	if err != nil || client.performCalls != 1 || cached.ImageData != "" || cached.Data["observation"] != nil || cached.Data["fresh_observation_required"] != true {
		t.Fatal("replay retained pixels or redispatched mutation")
	}
	if _, err := round.PerformAction(context.Background(), "next-action", ActionInput{Kind: "click_point"}); err != nil {
		t.Fatal(err)
	}
	if client.lastInput.ObservationID != "after" {
		t.Fatal("next action did not use returned guard")
	}
}

func TestPostActionScreenshotFailurePreservesDispatchAndRequiresFreshRead(t *testing.T) {
	service, client, _ := newFakeService(t)
	round := prepareRound(t, service)
	client.actionOutput = &nexuscua.ActionOutput{Dispatched: true, ObservationInvalidated: true,
		Observation: &nexuscua.WindowObservation{ObservationID: "after", WindowRef: "window-1", Screenshot: &nexuscua.ScreenshotArtifact{Path: "/missing/private.png"}},
	}
	result, err := round.PerformAction(context.Background(), "read-failed", ActionInput{Kind: "click_point"})
	if err != nil || result.Data["dispatched"] != true || result.Data["observation_error"] == nil || round.observationID != "" {
		t.Fatal("post-read failure lost action result or retained guard")
	}
	if _, err := round.PerformAction(context.Background(), "read-failed", ActionInput{Kind: "click_point"}); err != nil || client.performCalls != 1 {
		t.Fatal("post-read failure replayed action")
	}
}

func TestUserTakeoverLatchesUntilNewApprovedTarget(t *testing.T) {
	service, client, _ := newFakeService(t)
	round := prepareRound(t, service)
	client.actionError = &nexuscua.CUAError{Code: nexuscua.ErrorUserActive, MutationStatus: nexuscua.MutationNotDispatched}
	_, _ = round.PerformAction(context.Background(), "interrupted", ActionInput{Kind: "click_point"})
	if !round.UserActive() {
		t.Fatal("user takeover not latched")
	}
	client.actionError = nil
	if _, err := round.Observe(context.Background(), ObserveInput{}); err != nil {
		t.Fatal(err)
	}
	if _, err := round.PerformAction(context.Background(), "later", ActionInput{Kind: "click_point"}); err == nil || client.performCalls != 1 {
		t.Fatal("observation resumed authority after takeover")
	}
	approvals := 0
	if _, err := round.SelectTarget(context.Background(), "Fixture", TargetOptions{}, func(TargetGrant) error { approvals++; return nil }); err != nil {
		t.Fatal(err)
	}
	if round.UserActive() || approvals != 1 {
		t.Fatal("fresh host grant did not reset takeover")
	}
}

func TestForegroundHandoffDoesNotRunBeforeApproval(t *testing.T) {
	service, client, supervisor := newFakeService(t)
	round := prepareRound(t, service)
	denied := errors.New("denied by host")
	_, err := round.PerformActionWithApproval(context.Background(), "denied-handoff", ActionInput{Kind: "focus_window"}, func() error { return denied })
	if !errors.Is(err, denied) || supervisor.foregroundCalls != 0 || client.performCalls != 0 {
		t.Fatalf("denied mutation granted foreground or dispatched: err=%v handoffs=%d dispatch=%d", err, supervisor.foregroundCalls, client.performCalls)
	}
}
