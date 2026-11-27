// service_test.go 覆盖 Computer Use 服务、目录、产物、进程监督、撤销与原生集成测试。
package computeruse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	nexuscua "github.com/nexus-research-lab/nexus-cua/sdk/go"
	"github.com/nexus-research-lab/nexus/internal/config"
)

func NewServiceWithSupervisor(enabled bool, supervisor Supervisor) *Service {
	return &Service{enabled: enabled, supervisor: supervisor, rounds: make(map[string]*Round)}
}

func NewProcessSupervisor(cfg config.Config) *ProcessSupervisor {
	return NewProcessSupervisorWithBinaryPath(cfg, cfg.ComputerUseBinaryPath)
}

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

func (c *fakeClient) VerifyState(context.Context, nexuscua.VerifyStateInput, time.Duration) (nexuscua.VerificationOutput, error) {
	return nexuscua.VerificationOutput{Matched: true, Evidence: "fixture"}, nil
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

func TestForegroundHandoffDoesNotRunBeforeApproval(t *testing.T) {
	service, client, supervisor := newFakeService(t)
	round := prepareRound(t, service)
	denied := errors.New("denied by host")
	_, err := round.PerformActionWithApproval(context.Background(), "denied-handoff", ActionInput{Kind: "focus_window"}, func() error { return denied })
	if !errors.Is(err, denied) || supervisor.foregroundCalls != 0 || client.performCalls != 0 {
		t.Fatalf("denied mutation granted foreground or dispatched: err=%v handoffs=%d dispatch=%d", err, supervisor.foregroundCalls, client.performCalls)
	}
}

type catalogClient struct {
	*fakeClient
	installed []nexuscua.LaunchableApplication
}

func (c *catalogClient) DiscoverLaunchableApplications(context.Context, time.Duration) (nexuscua.DiscoverLaunchableApplicationsOutput, error) {
	return nexuscua.DiscoverLaunchableApplicationsOutput{Applications: c.installed, Complete: false}, nil
}

func (c *catalogClient) DiscoverApplications(context.Context, time.Duration) (nexuscua.DiscoverApplicationsOutput, error) {
	apps := make([]nexuscua.DiscoveredApplication, 0, len(c.installed))
	for _, app := range c.installed {
		apps = append(apps, nexuscua.DiscoveredApplication{Name: app.Name, ApplicationID: app.ApplicationID, Provenance: app.Provenance})
	}
	return nexuscua.DiscoverApplicationsOutput{Applications: apps, Complete: false}, nil
}

func (*catalogClient) LaunchApplication(context.Context, nexuscua.LaunchApplicationInput, time.Duration) (nexuscua.ApplicationLaunchOutput, *nexuscua.LaunchRequest, error) {
	panic("catalog discovery must not launch an application")
}

func (*catalogClient) ReconcileLaunch(context.Context, *nexuscua.LaunchRequest, time.Duration) (nexuscua.ApplicationLaunchOutput, error) {
	panic("catalog discovery must not reconcile a mutation")
}

func TestApplicationQueryAvoidsFullCatalogWithoutSelectingOrWideningAuthority(t *testing.T) {
	service, base, _ := newFakeService(t)
	round, err := service.Round(context.Background(), "owner-a", "round-a")
	if err != nil {
		t.Fatal(err)
	}
	client := &catalogClient{fakeClient: base}
	for i := range 144 {
		client.installed = append(client.installed, nexuscua.LaunchableApplication{
			LaunchRef: nexuscua.LaunchRef(fmt.Sprintf("launch-%d", i)),
			Name:      fmt.Sprintf("Application %d", i), ApplicationID: fmt.Sprintf("dev.fixture.%d", i),
		})
	}
	bundleID := "com.google.Chrome"
	chrome := nexuscua.LaunchableApplication{
		LaunchRef: "chrome-exact-ref", Name: "Google Chrome", ApplicationID: bundleID,
		ExpiresAt:  "2026-11-03T10:00:00Z",
		Provenance: nexuscua.ApplicationProvenance{Platform: nexuscua.PlatformMacOS, BundleID: &bundleID},
	}
	client.installed[89] = chrome
	round.client = client
	for _, query := range []string{"CHROME", "com.google.chrome"} {
		result, err := round.ListLaunchableApplications(context.Background(), query)
		if err != nil {
			t.Fatal(err)
		}
		apps := result["applications"].([]nexuscua.LaunchableApplication)
		encoded, _ := json.Marshal(result)
		if len(apps) != 1 || !reflect.DeepEqual(apps[0], chrome) || len(encoded) > 4096 || result["complete"] != false {
			t.Fatal("filtered catalog lost exact identity, exceeded a small page, or hid native incompleteness")
		}
		running, err := round.ListApplications(context.Background(), query)
		if err != nil || len(running["applications"].([]map[string]any)) != 1 || running["complete"] != false {
			t.Fatalf("running catalog differs: %v, %v", running, err)
		}
	}
	approved := false
	_, err = round.LaunchApplication(context.Background(), "hidden-app", nexuscua.LaunchApplicationInput{LaunchRef: "launch-0"}, func(nexuscua.LaunchableApplication) error { approved = true; return nil })
	if err == nil || approved || base.openCalls != 0 || base.performCalls != 0 || len(round.launchables) != 1 {
		t.Fatal("query silently selected/granted authority or admitted an undisclosed launch ref")
	}
	for query, want := range map[string]int{"missing": 0, "": 144} {
		result, err := round.ListLaunchableApplications(context.Background(), query)
		if err != nil || len(result["applications"].([]nexuscua.LaunchableApplication)) != want {
			t.Fatalf("query %q did not preserve empty/full discovery behavior: %v", query, err)
		}
	}
}

func TestProcessSupervisorRestartsAfterChildExit(t *testing.T) {
	root := t.TempDir()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("NEXUS_CUA_TEST_EXIT_CHILD", "1")
	supervisor := NewProcessSupervisor(config.Config{
		ComputerUseStateDir:   filepath.Join(root, "computer-use"),
		ComputerUseBinaryPath: binary,
	})
	var factoryCalls atomic.Int32
	supervisor.factory = func(nexuscua.Config) (Client, error) {
		factoryCalls.Add(1)
		return &fakeClient{}, nil
	}

	first, err := supervisor.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && supervisor.ArtifactRoot() != "" {
		time.Sleep(10 * time.Millisecond)
	}
	if supervisor.ArtifactRoot() != "" {
		t.Fatal("supervisor did not observe the crashed sidecar")
	}
	second, err := supervisor.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("restart reused the dead sidecar client")
	}
	if factoryCalls.Load() != 2 {
		t.Fatalf("client factory calls = %d, want two epochs", factoryCalls.Load())
	}
	_ = supervisor.Stop(context.Background())
}

func TestMain(m *testing.M) {
	if os.Getenv("NEXUS_CUA_TEST_EXIT_CHILD") == "1" && len(os.Args) > 1 && os.Args[1] == "serve" {
		time.Sleep(100 * time.Millisecond)
		os.Exit(1)
	}
	os.Exit(m.Run())
}

func TestPackagedCUANativeActionLoop(t *testing.T) {
	binary := os.Getenv("NEXUS_CUA_TEST_BINARY")
	if binary == "" {
		t.Skip("set NEXUS_CUA_TEST_BINARY and open the owned native fixture")
	}
	if !filepath.IsAbs(binary) {
		t.Fatal("NEXUS_CUA_TEST_BINARY must be absolute")
	}
	ctx := context.Background()
	root, err := os.MkdirTemp("", "nexus-cua-live-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	service := NewService(config.Config{
		ComputerUseStateDir: filepath.Join(root, "computer-use"),
		ComputerUseEnabled:  true, ComputerUseBinaryPath: binary,
	})
	t.Cleanup(func() { _ = service.Close(ctx) })
	round, err := service.Round(ctx, "native-test-owner", "native-test-round")
	if err != nil {
		t.Fatal(err)
	}
	computer, err := round.GetComputer(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if computer["runtime_version"] != pinnedCUAVersion || computer["observe_after"] != true {
		t.Fatal("packaged runtime version/capabilities do not match pinned SDK")
	}
	approved := false
	fixtureID := os.Getenv("NEXUS_CUA_TEST_APPLICATION_ID")
	if fixtureID == "" {
		fixtureID = "dev.nexus.cua.native-fixture"
	}
	_, err = round.SelectTarget(ctx, fixtureID, TargetOptions{
		Actions: []nexuscua.ActionKind{nexuscua.ActionInvokeElement}, TTLSeconds: 60,
	}, func(grant TargetGrant) error {
		approved = grant.Application.ApplicationID == fixtureID && !grant.AllowForegroundInput
		return nil
	})
	if err != nil || !approved {
		t.Fatalf("bounded fixture selection: %v", err)
	}
	observed, err := round.Observe(ctx, ObserveInput{
		WindowSelector: "Generation", IncludeScreenshot: true, Accessibility: "full",
	})
	if err != nil {
		t.Fatal(err)
	}
	if observed.ImageData == "" || observed.ImageMIME != "image/png" {
		t.Fatal("initial image missing")
	}
	findButton := func(data map[string]any) string {
		for _, element := range data["elements"].([]map[string]any) {
			if element["name"] == "Increment Counter" {
				return string(element["element_ref"].(nexuscua.ElementRef))
			}
		}
		t.Fatal("owned fixture button missing")
		return ""
	}
	for _, id := range []string{"native-first", "native-second"} {
		result, err := round.PerformAction(ctx, id, ActionInput{Kind: "invoke_element", ElementRef: findButton(observed.Data)})
		if err != nil {
			t.Fatal(err)
		}
		if result.Data["dispatched"] != true || result.Data["observation_error"] != nil || result.ImageData == "" {
			t.Fatal("action did not return successful evidence and a fresh image")
		}
		after, ok := result.Data["observation"].(map[string]any)
		if !ok || after["observation_id"] == observed.Data["observation_id"] {
			t.Fatal("guard did not advance")
		}
		if shot, ok := after["screenshot"].(map[string]any); !ok || shot["path"] != nil {
			t.Fatal("screenshot missing or leaked its private path")
		}
		observed = Observation{Data: after}
	}
	if _, err := round.PerformAction(ctx, "native-first", ActionInput{Kind: "invoke_element", ElementRef: findButton(observed.Data)}); err != nil {
		t.Fatal(err)
	}
	if err := service.CloseRound(ctx, "native-test-owner", "native-test-round"); err != nil {
		t.Fatal(err)
	}
	if round.HasSession() {
		t.Fatal("round close retained native authority")
	}
	if _, err := round.Observe(ctx, ObserveInput{}); err == nil || !strings.Contains(err.Error(), "关闭") {
		t.Fatal("closed round admitted a read")
	}
	t.Log("pinned runtime, bounded host approval, image delivery, two chained actions, replay and close verified")
}
