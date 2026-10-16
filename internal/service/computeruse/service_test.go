package computeruse

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	nexuscua "github.com/nexus-research-lab/nexus-cua/sdk/go"
)

type fakeSupervisor struct {
	client      *fakeClient
	next        *fakeClient
	artifact    string
	starts      int
	stops       int
	restarts    int
	restartNext bool
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

type fakeClient struct {
	performCalls   int
	reconcileCalls int
	closeCalls     int
	indeterminate  bool
	observation    nexuscua.WindowObservation
}

func (c *fakeClient) GetCapabilities(context.Context, time.Duration) (nexuscua.DriverCapabilities, error) {
	return nexuscua.DriverCapabilities{
		ProtocolVersion: nexuscua.ProtocolVersion,
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

func (c *fakeClient) OpenSession(context.Context, nexuscua.OpenSessionInput, time.Duration) (nexuscua.OpenSessionOutput, error) {
	return nexuscua.OpenSessionOutput{SessionID: "session-1"}, nil
}

func (c *fakeClient) CloseSession(context.Context, nexuscua.SessionID, time.Duration) error {
	c.closeCalls++
	return nil
}

func (c *fakeClient) ListApps(context.Context, nexuscua.SessionID, time.Duration) ([]nexuscua.ApplicationSummary, error) {
	return nil, nil
}

func (c *fakeClient) ListWindows(context.Context, nexuscua.SessionID, *nexuscua.AppRef, time.Duration) ([]nexuscua.WindowSummary, error) {
	return []nexuscua.WindowSummary{{WindowRef: "window-1", Title: "Fixture", Visible: true}}, nil
}

func (c *fakeClient) ObserveWindow(context.Context, nexuscua.ObserveWindowInput, time.Duration) (nexuscua.WindowObservation, error) {
	if c.observation.ObservationID != "" {
		return c.observation, nil
	}
	return nexuscua.WindowObservation{ObservationID: "observation-1", WindowRef: "window-1"}, nil
}

func (c *fakeClient) VerifyState(context.Context, nexuscua.VerifyStateInput, time.Duration) (nexuscua.VerificationOutput, error) {
	return nexuscua.VerificationOutput{Matched: true, Evidence: "fixture"}, nil
}

func (c *fakeClient) PerformAction(context.Context, nexuscua.PerformActionInput, time.Duration) (nexuscua.ActionOutput, *nexuscua.ActionRequest, error) {
	c.performCalls++
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
	if _, err := round.SelectTarget(context.Background(), "Fixture"); err != nil {
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

func TestActionApprovalRequiredOnlyForNewMutation(t *testing.T) {
	service, client, _ := newFakeService(t)
	round := prepareRound(t, service)
	required, err := round.ActionApprovalRequired("action-20261013-approval")
	if err != nil || !required {
		t.Fatalf("new action approval = %v, %v; want true", required, err)
	}
	if _, err := round.PerformAction(context.Background(), "action-20261013-approval", ActionInput{Kind: "click_point", X: 1, Y: 2}); err != nil {
		t.Fatal(err)
	}
	required, err = round.ActionApprovalRequired("action-20261013-approval")
	if err != nil || required {
		t.Fatalf("completed action approval = %v, %v; want false", required, err)
	}
	if _, err := round.Observe(context.Background(), ObserveInput{}); err != nil {
		t.Fatal(err)
	}
	client.indeterminate = true
	if _, err := round.PerformAction(context.Background(), "action-20261013-pending", ActionInput{Kind: "click_point", X: 3, Y: 4}); err == nil {
		t.Fatal("indeterminate action unexpectedly succeeded")
	}
	required, err = round.ActionApprovalRequired("action-20261013-pending")
	if err != nil || required {
		t.Fatalf("pending action approval = %v, %v; want false", required, err)
	}
}

func TestPerformActionWithApprovalSkipsApprovalOnReconcile(t *testing.T) {
	service, client, _ := newFakeService(t)
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
	if approvals != 1 || client.performCalls != 1 || client.reconcileCalls != 0 {
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
	if approvals != 2 || client.performCalls != 2 || client.reconcileCalls != 1 {
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
	if _, err := round.SelectTarget(context.Background(), "Fixture"); err != nil {
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
