// INPUT: Nexus round identity and the pinned typed nexus-cua client.
// OUTPUT: owner-scoped Computer Use sessions, distinct target/observation readiness, bounded observations, and one-shot mutation reconciliation.
// POS: Nexus' only Computer Use service boundary; transport credentials and sidecar process details stay below this package.
package computeruse

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	nexuscua "github.com/nexus-research-lab/nexus-cua/sdk/go"
	"github.com/nexus-research-lab/nexus/internal/config"
)

const (
	protocolVersion       = nexuscua.ProtocolVersion
	defaultCommandTimeout = 30 * time.Second
	readinessTimeout      = 10 * time.Second
	defaultSessionTTL     = 30 * 60
	maxScreenshotBytes    = 16 << 20
	maxPendingActions     = 64
	maxCompletedActions   = 128
)

var (
	ErrDisabled        = errors.New("Computer Use 未启用")
	ErrRoundClosed     = errors.New("Computer Use round 已关闭")
	ErrNoTarget        = errors.New("Computer Use 尚未选择目标应用")
	ErrNoObservation   = errors.New("Computer Use 尚未取得可用于动作的最新 observation")
	ErrOwnerConflict   = errors.New("Computer Use 当前 sidecar 已绑定其他 owner")
	ErrNotReady        = errors.New("Computer Use sidecar 尚未就绪")
	ErrMutationPending = errors.New("Computer Use 存在待对账 mutation，请先复用原 request_id")
)

// Client 是 Nexus 使用的最小 typed CUA client 面；不把 generic wire command
// 暴露给上层，也不允许调用方读取 token。
type Client interface {
	GetCapabilities(context.Context, time.Duration) (nexuscua.DriverCapabilities, error)
	GetPermissionStatus(context.Context, time.Duration) (nexuscua.PermissionStatus, error)
	DiscoverApplications(context.Context, time.Duration) (nexuscua.DiscoverApplicationsOutput, error)
	OpenSession(context.Context, nexuscua.OpenSessionInput, time.Duration) (nexuscua.OpenSessionOutput, error)
	CloseSession(context.Context, nexuscua.SessionID, time.Duration) error
	ListApps(context.Context, nexuscua.SessionID, time.Duration) ([]nexuscua.ApplicationSummary, error)
	ListWindows(context.Context, nexuscua.SessionID, *nexuscua.AppRef, time.Duration) ([]nexuscua.WindowSummary, error)
	ObserveWindow(context.Context, nexuscua.ObserveWindowInput, time.Duration) (nexuscua.WindowObservation, error)
	VerifyState(context.Context, nexuscua.VerifyStateInput, time.Duration) (nexuscua.VerificationOutput, error)
	PerformAction(context.Context, nexuscua.PerformActionInput, time.Duration) (nexuscua.ActionOutput, *nexuscua.ActionRequest, error)
	ReconcileAction(context.Context, *nexuscua.ActionRequest, time.Duration) (nexuscua.ActionOutput, error)
}

// Supervisor owns exactly one sidecar epoch. Implementations must generate a
// fresh token and discard old sessions when an epoch ends.
type Supervisor interface {
	Start(context.Context) (Client, error)
	Stop(context.Context) error
	Restart(context.Context) (Client, error)
	ArtifactRoot() string
}

// Factory is injectable for contract tests. Production uses the pinned SDK and
// process supervisor below.
type Factory func(nexuscua.Config) (Client, error)

// Service owns the application-wide sidecar and the round-scoped CUA state.
type Service struct {
	mu                   sync.Mutex
	enabled              bool
	owner                string
	supervisor           Supervisor
	client               Client
	rounds               map[string]*Round
	installer            *CUAInstaller
	configuredBinaryPath string
}

// NewService constructs a fail-closed service. ComputerUseEnabled is false by
// default in config, so installing Nexus does not silently gain desktop input.
func NewService(cfg config.Config) *Service {
	installer := NewCUAInstaller(cfg)
	binaryPath := strings.TrimSpace(cfg.ComputerUseBinaryPath)
	if binaryPath == "" {
		binaryPath = installer.InstalledBinaryPath()
	}
	enabled := cfg.ComputerUseEnabled
	if installer.Enabled() {
		enabled = true
	}
	return &Service{
		enabled:              enabled,
		supervisor:           NewProcessSupervisorWithBinaryPath(cfg, binaryPath),
		rounds:               make(map[string]*Round),
		installer:            installer,
		configuredBinaryPath: binaryPath,
	}
}

// NewServiceWithSupervisor is used by tests and by hosts that already own a
// verified sidecar lifecycle.
func NewServiceWithSupervisor(enabled bool, supervisor Supervisor) *Service {
	return &Service{enabled: enabled, supervisor: supervisor, rounds: make(map[string]*Round)}
}

// SetLogger supplies the host logger used for bounded sidecar lifecycle
// diagnostics. Raw sidecar output is never emitted.
func (s *Service) SetLogger(logger *slog.Logger) {
	if configured, ok := s.supervisor.(interface{ SetLogger(*slog.Logger) }); ok {
		configured.SetLogger(logger)
	}
}

// Install downloads and verifies the latest nexus-cua GitHub Release, then
// switches future sidecar epochs to the new binary. Installation never turns
// Computer Use on by itself; the settings toggle remains an explicit action.
func (s *Service) Install(ctx context.Context) (CUAInstallationStatus, error) {
	if s.installer == nil {
		return CUAInstallationStatus{}, ErrCUANotInstalled
	}
	result, err := s.installer.Install(ctx)
	if err != nil {
		return s.InstallationStatus(), err
	}
	if err := s.replaceBinary(ctx, result.BinaryPath); err != nil {
		return s.InstallationStatus(), err
	}
	s.mu.Lock()
	s.configuredBinaryPath = result.BinaryPath
	s.mu.Unlock()
	return s.InstallationStatus(), nil
}

// InstallationStatus returns host installation state without exposing private
// paths or sidecar transport credentials.
func (s *Service) InstallationStatus() CUAInstallationStatus {
	if s.installer == nil {
		return CUAInstallationStatus{}
	}
	status := s.installer.Status()
	s.mu.Lock()
	status.Enabled = s.enabled
	configured := s.configuredBinaryPath
	s.mu.Unlock()
	if !status.Installed && isExecutableRegularFile(configured) {
		status.Installed = true
		status.Source = "configured"
		status.Message = ""
	}
	return status
}

// CheckPermissions starts the enabled sidecar if needed and reads the native
// permission projection. The runtime never attempts to grant these permissions.
func (s *Service) CheckPermissions(ctx context.Context) (nexuscua.PermissionStatus, error) {
	s.mu.Lock()
	if !s.enabled {
		s.mu.Unlock()
		return nexuscua.PermissionStatus{}, ErrDisabled
	}
	client := s.client
	s.mu.Unlock()
	if s.supervisor != nil {
		started, err := s.supervisor.Start(ctx)
		if err != nil {
			return nexuscua.PermissionStatus{}, err
		}
		client = started
		s.mu.Lock()
		s.client = client
		s.mu.Unlock()
	}
	if client == nil {
		return nexuscua.PermissionStatus{}, ErrNotReady
	}
	return client.GetPermissionStatus(ctx, defaultCommandTimeout)
}

func (s *Service) Enabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.enabled
}

// SetEnabled closes all active rounds before disabling admission. Re-enabling
// starts a fresh sidecar epoch on the next round.
func (s *Service) SetEnabled(ctx context.Context, enabled bool) error {
	s.mu.Lock()
	configured := s.configuredBinaryPath
	installer := s.installer
	s.mu.Unlock()
	if enabled && installer != nil && installer.InstalledBinaryPath() == "" && !isExecutableRegularFile(configured) {
		return ErrCUANotInstalled
	}
	if installer != nil && installer.InstalledBinaryPath() != "" {
		if err := installer.SetEnabled(enabled); err != nil {
			return err
		}
	}
	s.mu.Lock()
	if s.enabled == enabled {
		s.mu.Unlock()
		return nil
	}
	s.enabled = enabled
	if enabled {
		s.owner = ""
		s.client = nil
		s.mu.Unlock()
		return nil
	}
	rounds := s.takeRoundsLocked()
	s.client = nil
	s.owner = ""
	s.mu.Unlock()
	return s.closeRoundsAndStop(ctx, rounds)
}

func (s *Service) replaceBinary(ctx context.Context, path string) error {
	if !isExecutableRegularFile(path) {
		return errors.New("Computer Use 安装结果不是可执行普通文件")
	}
	s.mu.Lock()
	rounds := s.takeRoundsLocked()
	s.client = nil
	s.owner = ""
	s.mu.Unlock()
	if err := s.closeRoundsAndStop(ctx, rounds); err != nil {
		return err
	}
	setter, ok := s.supervisor.(interface{ SetBinaryPath(string) error })
	if !ok {
		return errors.New("Computer Use supervisor 不支持热切换安装包")
	}
	if err := setter.SetBinaryPath(path); err != nil {
		return err
	}
	return nil
}

// SwitchOwner revokes the previous owner's rounds and starts the next owner
// from a clean epoch. It is intentionally explicit; a round cannot silently
// inherit another user's authority.
func (s *Service) SwitchOwner(ctx context.Context, owner string) error {
	owner = strings.TrimSpace(owner)
	if owner == "" {
		return errors.New("Computer Use owner 不能为空")
	}
	s.mu.Lock()
	if !s.enabled {
		s.mu.Unlock()
		return ErrDisabled
	}
	if s.owner == "" || s.owner == owner {
		s.owner = owner
		s.mu.Unlock()
		return nil
	}
	rounds := s.takeRoundsLocked()
	s.client = nil
	s.owner = owner
	s.mu.Unlock()
	return s.closeRoundsAndStop(ctx, rounds)
}

// Round returns a round-scoped handle. The first call starts the supervised
// sidecar; subsequent calls for the same round reuse its client and state.
func (s *Service) Round(ctx context.Context, owner, roundID string) (*Round, error) {
	owner = strings.TrimSpace(owner)
	roundID = strings.TrimSpace(roundID)
	if owner == "" || roundID == "" {
		return nil, errors.New("Computer Use round identity 不能为空")
	}
	s.mu.Lock()
	if !s.enabled {
		s.mu.Unlock()
		return nil, ErrDisabled
	}
	if s.owner != "" && s.owner != owner {
		s.mu.Unlock()
		return nil, ErrOwnerConflict
	}
	if s.owner == "" {
		s.owner = owner
	}
	client := s.client
	s.mu.Unlock()

	if s.supervisor != nil {
		// Start is idempotent for a live epoch and repairs a crashed process.
		// Calling it for every command also lets a round discover that its
		// previous opaque session died with the sidecar.
		started, err := s.supervisor.Start(ctx)
		if err != nil {
			return nil, fmt.Errorf("启动 Computer Use sidecar: %w", err)
		}
		client = started
	}
	if client == nil {
		return nil, ErrNotReady
	}

	var stale []*Round
	round := &Round{
		service: s, owner: owner, id: roundID, client: client,
		deadline:  time.Now().Add(defaultSessionTTL * time.Second),
		pending:   make(map[string]*nexuscua.ActionRequest),
		completed: make(map[string]ActionResult),
	}
	s.mu.Lock()
	if !s.enabled {
		s.mu.Unlock()
		return nil, ErrDisabled
	}
	if s.owner != "" && s.owner != owner {
		s.mu.Unlock()
		return nil, ErrOwnerConflict
	}
	if s.client != nil && !sameClient(s.client, client) {
		stale = s.takeRoundsLocked()
	}
	s.client = client
	if existing := s.rounds[roundID]; existing != nil {
		s.mu.Unlock()
		for _, old := range stale {
			old.invalidate()
		}
		return existing, nil
	}
	s.rounds[roundID] = round
	s.mu.Unlock()
	for _, old := range stale {
		old.invalidate()
	}
	return round, nil
}

// CloseRoundByID is used by the trusted runtime lifecycle observer after a
// physical round has finished. The round ID is already host-owned; no model
// input can reach this helper, so it closes the matching owner-bound round
// without accepting an owner override.
func (s *Service) CloseRoundByID(ctx context.Context, roundID string) error {
	roundID = strings.TrimSpace(roundID)
	if roundID == "" {
		return nil
	}
	s.mu.Lock()
	round := s.rounds[roundID]
	delete(s.rounds, roundID)
	s.mu.Unlock()
	if round == nil {
		return nil
	}
	return round.Close(ctx)
}

// CloseRound closes its CUA session and forgets all observations and pending
// mutations. It is safe to call more than once.
func (s *Service) CloseRound(ctx context.Context, owner, roundID string) error {
	s.mu.Lock()
	round := s.rounds[roundID]
	if round != nil && round.owner != strings.TrimSpace(owner) {
		s.mu.Unlock()
		return ErrOwnerConflict
	}
	delete(s.rounds, roundID)
	s.mu.Unlock()
	if round == nil {
		return nil
	}
	return round.Close(ctx)
}

// Close revokes admission, closes every round, and stops the sidecar.
func (s *Service) Close(ctx context.Context) error {
	s.mu.Lock()
	rounds := s.takeRoundsLocked()
	s.enabled = false
	s.owner = ""
	s.client = nil
	s.mu.Unlock()
	return s.closeRoundsAndStop(ctx, rounds)
}

func (s *Service) takeRoundsLocked() []*Round {
	rounds := make([]*Round, 0, len(s.rounds))
	for _, round := range s.rounds {
		rounds = append(rounds, round)
	}
	s.rounds = make(map[string]*Round)
	return rounds
}

func (s *Service) closeRoundsAndStop(ctx context.Context, rounds []*Round) error {
	var joined error
	for _, round := range rounds {
		joined = errors.Join(joined, round.Close(ctx))
	}
	if s.supervisor != nil {
		joined = errors.Join(joined, s.supervisor.Stop(ctx))
	}
	return joined
}

// Round is the only object command operations receive. Native refs never leave
// this object except as opaque values in structured results.
type Round struct {
	launchables           map[nexuscua.LaunchRef]nexuscua.LaunchableApplication
	pendingLaunches       map[string]*nexuscua.LaunchRequest
	service               *Service
	owner                 string
	id                    string
	client                Client
	mu                    sync.Mutex
	mutationMu            sync.Mutex
	closed                bool
	deadline              time.Time
	sessionExpiresAt      time.Time
	application           *nexuscua.DiscoveredApplication
	sessionID             nexuscua.SessionID
	window                *nexuscua.WindowSummary
	observationID         nexuscua.ObservationID
	userActive            bool
	observeAfterSupported bool
	observationOptions    *nexuscua.ObserveAfterOptions
	pending               map[string]*nexuscua.ActionRequest
	completed             map[string]ActionResult
	completedIDs          []string
}

// invalidate is used when the supervised sidecar epoch ends unexpectedly.
// Its session and all opaque refs belong to the dead process and cannot be
// carried into the replacement epoch.
func (r *Round) invalidate() {
	r.mutationMu.Lock()
	defer r.mutationMu.Unlock()
	r.mu.Lock()
	r.closed = true
	r.sessionID = ""
	r.application = nil
	r.window = nil
	r.observationID = ""
	r.pending = make(map[string]*nexuscua.ActionRequest)
	r.launchables = nil
	r.pendingLaunches = nil
	r.completed = make(map[string]ActionResult)
	r.completedIDs = nil
	r.mu.Unlock()
}

func (r *Round) ID() string { return r.id }

func (r *Round) UserActive() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.userActive
}

func (r *Round) GetComputer(ctx context.Context) (map[string]any, error) {
	r.mu.Lock()
	if err := r.ensureOpenLocked(); err != nil {
		r.mu.Unlock()
		return nil, err
	}
	r.mu.Unlock()
	capabilities, err := r.client.GetCapabilities(ctx, defaultCommandTimeout)
	if err != nil {
		return nil, err
	}
	permissions, err := r.client.GetPermissionStatus(ctx, defaultCommandTimeout)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"protocol_version":      capabilities.ProtocolVersion,
		"runtime_version":       capabilities.RuntimeVersion,
		"platform":              capabilities.Platform,
		"primary_modifier":      capabilities.PrimaryModifier,
		"capture_modes":         capabilities.CaptureModes,
		"accessibility_tree":    capabilities.AccessibilityTree,
		"observe_after":         capabilities.ObserveAfter,
		"input_routes":          capabilities.InputRoutes,
		"actions":               supportedRoundActions(capabilities.Actions),
		"action_limits":         capabilities.ActionLimits,
		"application_lifecycle": capabilities.ApplicationLifecycle,
		"permissions": map[string]any{
			"screen_capture": permissions.ScreenCapture,
			"accessibility":  permissions.Accessibility,
			"input_control":  permissions.InputControl,
		},
		"session_active": r.HasSession(),
		"user_active":    r.UserActive(),
	}, nil
}

func (r *Round) ListApplications(ctx context.Context) (map[string]any, error) {
	r.mu.Lock()
	if err := r.ensureOpenLocked(); err != nil {
		r.mu.Unlock()
		return nil, err
	}
	r.mu.Unlock()
	apps, err := r.client.DiscoverApplications(ctx, defaultCommandTimeout)
	if err != nil {
		return nil, err
	}
	values := make([]map[string]any, 0, len(apps.Applications))
	for _, app := range apps.Applications {
		values = append(values, map[string]any{
			"name": app.Name, "application_id": app.ApplicationID,
			"foreground": app.Foreground, "discovery_ref": app.DiscoveryRef,
			"provenance": app.Provenance,
		})
	}
	return map[string]any{"complete": apps.Complete, "applications": values}, nil
}

// supportedRoundActions grants only operations exposed by this host. New runtime
// capabilities, especially application termination, never widen an existing host grant.
func supportedRoundActions(available []nexuscua.ActionKind) []nexuscua.ActionKind {
	exposed := map[nexuscua.ActionKind]bool{
		nexuscua.ActionFocusWindow: true, nexuscua.ActionFocusElement: true,
		nexuscua.ActionInvokeElement: true, nexuscua.ActionClickPoint: true,
		nexuscua.ActionSetValue: true, nexuscua.ActionToggleElement: true,
		nexuscua.ActionSelectElement: true, nexuscua.ActionSetExpanded: true,
		nexuscua.ActionMovePointer: true, nexuscua.ActionTypeText: true,
		nexuscua.ActionPressKeys: true, nexuscua.ActionScroll: true, nexuscua.ActionDrag: true,
		nexuscua.ActionSetWindowMinimized: true, nexuscua.ActionSetWindowBounds: true,
		nexuscua.ActionSetRangeValue: true, nexuscua.ActionPressPoint: true, nexuscua.ActionTerminateApplication: true,
	}
	actions := make([]nexuscua.ActionKind, 0, len(exposed))
	for _, action := range available {
		if exposed[action] {
			actions = append(actions, action)
		}
	}
	return actions
}

func (r *Round) SelectTarget(ctx context.Context, selector string, options TargetOptions, approve func(TargetGrant) error) (map[string]any, error) {
	r.mutationMu.Lock()
	defer r.mutationMu.Unlock()
	r.mu.Lock()
	if err := r.ensureOpenLocked(); err != nil {
		r.mu.Unlock()
		return nil, err
	}
	if len(r.pending) > 0 || len(r.pendingLaunches) > 0 {
		r.mu.Unlock()
		return nil, ErrMutationPending
	}
	r.mu.Unlock()
	apps, err := r.client.DiscoverApplications(ctx, defaultCommandTimeout)
	if err != nil {
		return nil, err
	}
	selected, err := nexuscua.SelectApplication(apps.Applications, strings.TrimSpace(selector))
	if err != nil {
		return nil, err
	}
	capabilities, err := r.client.GetCapabilities(ctx, defaultCommandTimeout)
	if err != nil {
		return nil, err
	}
	grant, err := r.targetGrant(*selected, capabilities, options)
	if err != nil {
		return nil, err
	}
	if approve == nil {
		return nil, errors.New("Computer Use select_target 缺少宿主授权通道")
	}
	if err := approve(grant); err != nil {
		return nil, err
	}
	// Discovery references may expire while a human decides. Re-resolve the
	// approved application identity, never widen it to a similarly named app.
	fresh, err := r.client.DiscoverApplications(ctx, defaultCommandTimeout)
	if err != nil {
		return nil, err
	}
	refreshed, err := nexuscua.SelectApplication(fresh.Applications, selected.ApplicationID)
	if err != nil || !reflect.DeepEqual(refreshed.Provenance, selected.Provenance) {
		return nil, errors.New("Computer Use 应用身份已变化，请重新 select_target")
	}
	selected = refreshed
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil, ErrRoundClosed
	}
	old := r.sessionID
	r.mu.Unlock()
	if old != "" {
		if err := r.client.CloseSession(ctx, old, defaultCommandTimeout); err != nil {
			// The runtime may have expired/reaped the session already.  Treat
			// that as a successful local cleanup and continue opening the fresh
			// target session; otherwise every retry keeps attempting to close the
			// same dead opaque session forever.
			if !isSessionUnavailable(err) {
				return nil, fmt.Errorf("关闭旧 Computer Use session: %w", err)
			}
			r.invalidateSession()
		}
	}
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil, ErrRoundClosed
	}
	r.sessionID = ""
	r.window = nil
	r.observationID = ""
	r.mu.Unlock()
	// Subtract both approval and old-session cleanup time from the grant.
	remaining := time.Until(grant.ExpiresAt)
	if remaining < time.Second {
		return nil, errors.New("Computer Use 授权已过期，请重新 select_target")
	}
	ttl := uint32(remaining / time.Second)
	session, err := r.client.OpenSession(ctx, nexuscua.OpenSessionInput{Manifest: nexuscua.CapabilityManifest{
		Mode: nexuscua.PermissionBounded, ApplicationRefs: []nexuscua.DiscoveryRef{selected.DiscoveryRef},
		AllowedActions: grant.Actions, AllowForegroundInput: grant.AllowForegroundInput, TTLSeconds: ttl,
	}}, defaultCommandTimeout)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		closeErr := r.client.CloseSession(context.WithoutCancel(ctx), session.SessionID, defaultCommandTimeout)
		if isSessionUnavailable(closeErr) {
			closeErr = nil
		}
		return nil, errors.Join(ErrRoundClosed, closeErr)
	}
	r.application = selected
	r.userActive = false
	r.observeAfterSupported = capabilities.ObserveAfter
	r.observationOptions = nil
	r.sessionID = session.SessionID
	r.sessionExpiresAt = grant.ExpiresAt
	r.window = nil
	r.observationID = ""
	r.mu.Unlock()
	return map[string]any{
		"name": selected.Name, "application_id": selected.ApplicationID,
		"foreground": selected.Foreground, "session_active": true,
		"ttl_seconds": ttl, "expires_at": session.ExpiresAt,
		"allowed_actions": grant.Actions, "allow_foreground_input": grant.AllowForegroundInput,
	}, nil
}

// ListWindows exposes only the selected application's current top-level
// window summaries. It deliberately runs before observe so callers can obtain
// an exact title/ref when an application owns multiple windows.
func (r *Round) ListWindows(ctx context.Context) (map[string]any, error) {
	r.mu.Lock()
	if err := r.ensureOpenLocked(); err != nil {
		r.mu.Unlock()
		return nil, err
	}
	if r.sessionID == "" || r.application == nil {
		r.mu.Unlock()
		return nil, ErrNoTarget
	}
	sessionID := r.sessionID
	r.mu.Unlock()
	windows, err := r.client.ListWindows(ctx, sessionID, nil, defaultCommandTimeout)
	if err != nil {
		r.handleSessionError(err)
		return nil, err
	}
	values := make([]map[string]any, 0, len(windows))
	for _, window := range windows {
		values = append(values, map[string]any{
			"window_ref": window.WindowRef,
			"title":      window.Title,
			"visible":    window.Visible,
			"foreground": window.Foreground,
			"state":      window.State,
		})
	}
	return map[string]any{"windows": values}, nil
}

type ObserveInput struct {
	ApplicationScene  bool
	WindowSelector    string
	IncludeScreenshot bool
	Accessibility     string
}

type Observation struct {
	Data      map[string]any
	ImageData string
	ImageMIME string
}

func (r *Round) Observe(ctx context.Context, input ObserveInput) (Observation, error) {
	r.mutationMu.Lock()
	defer r.mutationMu.Unlock()
	r.mu.Lock()
	if err := r.ensureOpenLocked(); err != nil {
		r.mu.Unlock()
		return Observation{}, err
	}
	if r.sessionID == "" || r.application == nil {
		r.mu.Unlock()
		return Observation{}, ErrNoTarget
	}
	sessionID := r.sessionID
	selector := strings.TrimSpace(input.WindowSelector)
	r.mu.Unlock()
	windows, err := r.client.ListWindows(ctx, sessionID, nil, defaultCommandTimeout)
	if err != nil {
		r.handleSessionError(err)
		return Observation{}, err
	}
	var window *nexuscua.WindowSummary
	if selector == "" && len(windows) == 1 {
		window = &windows[0]
	} else {
		window, err = nexuscua.SelectWindow(windows, selector)
		if err != nil {
			return Observation{}, err
		}
	}
	accessibility, err := parseAccessibility(input.Accessibility)
	if err != nil {
		return Observation{}, err
	}
	observe := r.client.ObserveWindow
	if input.ApplicationScene {
		scene, ok := r.client.(interface {
			ObserveApplicationScene(context.Context, nexuscua.ObserveWindowInput, time.Duration) (nexuscua.WindowObservation, error)
		})
		if !ok {
			return Observation{}, errors.New("Computer Use runtime 不支持应用场景观察")
		}
		observe = scene.ObserveApplicationScene
	}
	value, err := observe(ctx, nexuscua.ObserveWindowInput{
		SessionID: sessionID, WindowRef: window.WindowRef,
		IncludeScreenshot: input.IncludeScreenshot, Accessibility: accessibility,
	}, defaultCommandTimeout)
	if err != nil {
		r.handleSessionError(err)
		return Observation{}, err
	}
	result, err := r.projectObservation(value)
	if err != nil {
		return Observation{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.sessionID != sessionID {
		return Observation{}, ErrRoundClosed
	}
	r.window = window
	r.observationID = value.ObservationID
	r.observationOptions = &nexuscua.ObserveAfterOptions{
		IncludeScreenshot: input.IncludeScreenshot, Accessibility: accessibility,
		ApplicationScene: input.ApplicationScene,
	}
	return result, nil
}

// projectObservation consumes no authority and never exposes the private artifact path.
func (r *Round) projectObservation(value nexuscua.WindowObservation) (Observation, error) {
	result := Observation{Data: observationView(value)}
	if value.Screenshot != nil {
		encoded, mime, err := r.readArtifact(value.Screenshot.Path)
		if err != nil {
			return Observation{}, err
		}
		result.Data["screenshot"] = map[string]any{
			"artifact_ref": value.Screenshot.ArtifactRef, "mime_type": value.Screenshot.MIMEType,
			"byte_length": value.Screenshot.ByteLength, "sha256": value.Screenshot.SHA256,
			"mapping": value.Screenshot.Mapping,
		}
		result.ImageData, result.ImageMIME = encoded, mime
	}
	return result, nil
}

type ActionInput struct {
	Bounds     *nexuscua.ScreenRect
	RangeValue *float64
	Kind       string
	ElementRef string
	Value      string
	Text       string
	Expanded   bool
	X          uint32
	Y          uint32
	ToX        uint32
	ToY        uint32
	Button     string
	Count      uint8
	DurationMS uint32
	DeltaX     int32
	DeltaY     int32
	Keys       []string
}

type ActionResult struct {
	Data      map[string]any
	ImageData string
	ImageMIME string
}

func (r *Round) PerformAction(ctx context.Context, requestID string, input ActionInput) (ActionResult, error) {
	return r.performAction(ctx, requestID, input, nil)
}

// PerformActionWithApproval serializes the approval decision with mutation
// admission. The callback runs only for a new mutation; completed and pending
// request IDs go directly to cache lookup or reconciliation.
func (r *Round) PerformActionWithApproval(ctx context.Context, requestID string, input ActionInput, approve func() error) (ActionResult, error) {
	return r.performAction(ctx, requestID, input, approve)
}

func (r *Round) performAction(ctx context.Context, requestID string, input ActionInput, approve func() error) (ActionResult, error) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return ActionResult{}, errors.New("Computer Use perform_action 需要 request_id")
	}
	r.mutationMu.Lock()
	defer r.mutationMu.Unlock()
	r.mu.Lock()
	if err := r.ensureOpenLocked(); err != nil {
		r.mu.Unlock()
		return ActionResult{}, err
	}
	if completed, ok := r.completed[requestID]; ok {
		r.mu.Unlock()
		return cloneActionResult(completed), nil
	}
	pending := r.pending[requestID]
	if pending != nil {
		r.mu.Unlock()
		output, err := r.client.ReconcileAction(ctx, pending, defaultCommandTimeout)
		if err != nil {
			r.handleSessionError(err)
			return ActionResult{}, err
		}
		result := r.acceptActionOutput(output)
		r.mu.Lock()
		delete(r.pending, requestID)
		r.rememberActionResultLocked(requestID, result)
		r.mu.Unlock()
		return result, nil
	}
	if r.userActive {
		r.mu.Unlock()
		recovery := "request_host_authority"
		return ActionResult{}, &nexuscua.CUAError{
			Code: nexuscua.ErrorUserActive, Message: "用户已接管，新的明确授权之前停止自动操作",
			MutationStatus: nexuscua.MutationNotDispatched, RecoveryAction: &recovery,
		}
	}
	if len(r.pending) > 0 || len(r.pendingLaunches) > 0 {
		r.mu.Unlock()
		return ActionResult{}, ErrMutationPending
	}
	if !r.sessionExpiresAt.IsZero() && !time.Now().Before(r.sessionExpiresAt) {
		r.mu.Unlock()
		return ActionResult{}, errors.New("Computer Use session 已过期，请重新 select_target")
	}
	if r.sessionID == "" || r.application == nil {
		r.mu.Unlock()
		return ActionResult{}, ErrNoTarget
	}
	if r.window == nil || r.observationID == "" {
		r.mu.Unlock()
		return ActionResult{}, ErrNoObservation
	}
	if len(r.pending) >= maxPendingActions {
		r.mu.Unlock()
		return ActionResult{}, errors.New("Computer Use pending mutation 超出上限，请先对账或关闭 target")
	}
	sessionID, windowRef, observationID := r.sessionID, r.window.WindowRef, r.observationID
	var after *nexuscua.ObserveAfterOptions
	if r.observeAfterSupported && r.observationOptions != nil {
		options := *r.observationOptions
		after = &options
	}
	r.mu.Unlock()
	action, err := buildAction(input)
	if err != nil {
		return ActionResult{}, err
	}
	if strings.EqualFold(strings.TrimSpace(input.Kind), "terminate_application") && approve == nil {
		return ActionResult{}, errors.New("退出应用需要单独审批")
	}
	if approve != nil {
		if err := approve(); err != nil {
			return ActionResult{}, err
		}
	}
	output, actionRequest, err := r.client.PerformAction(ctx, nexuscua.PerformActionInput{
		SessionID: sessionID, WindowRef: windowRef, ObservationID: observationID, Action: action, ObserveAfter: after,
	}, defaultCommandTimeout)
	if err != nil {
		var indeterminate *nexuscua.MutationIndeterminateError
		if errors.As(err, &indeterminate) && actionRequest != nil {
			r.mu.Lock()
			r.pending[requestID] = actionRequest
			r.mu.Unlock()
		}
		r.handleSessionError(err)
		return ActionResult{}, err
	}
	result := r.acceptActionOutput(output)
	r.mu.Lock()
	r.rememberActionResultLocked(requestID, result)
	r.mu.Unlock()
	return result, nil
}

// acceptActionOutput preserves successful mutation evidence even when its follow-up read fails.
func (r *Round) acceptActionOutput(output nexuscua.ActionOutput) ActionResult {
	result := actionView(output)
	r.mu.Lock()
	r.observationID = ""
	open := !r.closed
	window := r.window
	r.mu.Unlock()
	if output.ObservationError != nil {
		result.Data["observation_error"] = output.ObservationError
	}
	if !open || output.Observation == nil || window == nil || output.Observation.WindowRef != window.WindowRef {
		return result
	}
	observed, err := r.projectObservation(*output.Observation)
	if err != nil {
		result.Data["observation_error"] = map[string]any{
			"code": "driver_failure", "message": "动作已执行，后续截图读取失败，请重新 observe",
			"mutation_status": "not_applicable", "recovery_action": "observe_window",
		}
		return result
	}
	r.mu.Lock()
	if !r.closed {
		r.observationID = output.Observation.ObservationID
	}
	r.mu.Unlock()
	result.Data["observation"] = observed.Data
	result.ImageData, result.ImageMIME = observed.ImageData, observed.ImageMIME
	return result
}

func (r *Round) rememberActionResultLocked(requestID string, result ActionResult) {
	if r.completed == nil {
		r.completed = make(map[string]ActionResult)
	}
	if _, exists := r.completed[requestID]; !exists {
		r.completedIDs = append(r.completedIDs, requestID)
	}
	cached := cloneActionResult(result)
	// Keep mutation evidence only. Do not retain screenshots or AX trees for 128 actions.
	if _, hasObservation := cached.Data["observation"]; hasObservation {
		delete(cached.Data, "observation")
		cached.Data["fresh_observation_required"] = true
	}
	cached.ImageData, cached.ImageMIME = "", ""
	r.completed[requestID] = cached
	for len(r.completedIDs) > maxCompletedActions {
		oldest := r.completedIDs[0]
		r.completedIDs = r.completedIDs[1:]
		delete(r.completed, oldest)
	}
}

func cloneActionResult(result ActionResult) ActionResult {
	data := make(map[string]any, len(result.Data))
	for key, value := range result.Data {
		data[key] = value
	}
	return ActionResult{Data: data, ImageData: result.ImageData, ImageMIME: result.ImageMIME}
}

func (r *Round) VerifyState(ctx context.Context, predicate map[string]any) (map[string]any, error) {
	r.mu.Lock()
	if err := r.ensureOpenLocked(); err != nil {
		r.mu.Unlock()
		return nil, err
	}
	if r.sessionID == "" || r.window == nil {
		r.mu.Unlock()
		return nil, ErrNoTarget
	}
	sessionID, windowRef := r.sessionID, r.window.WindowRef
	r.mu.Unlock()
	parsed, err := buildPredicate(predicate)
	if err != nil {
		return nil, err
	}
	value, err := r.client.VerifyState(ctx, nexuscua.VerifyStateInput{SessionID: sessionID, WindowRef: windowRef, Predicate: parsed}, defaultCommandTimeout)
	if err != nil {
		r.handleSessionError(err)
		return nil, err
	}
	return map[string]any{"matched": value.Matched, "evidence": value.Evidence}, nil
}

func (r *Round) Close(ctx context.Context) error {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil
	}
	r.closed = true
	sessionID := r.sessionID
	r.mu.Unlock()
	// Revocation must reach the runtime while an input action still owns
	// mutationMu. Waiting for that lock first lets a held input run to completion.
	var closeErr error
	if sessionID != "" {
		closeErr = r.client.CloseSession(ctx, sessionID, defaultCommandTimeout)
		if isSessionUnavailable(closeErr) {
			closeErr = nil
		}
	}
	r.mutationMu.Lock()
	defer r.mutationMu.Unlock()
	r.mu.Lock()
	r.sessionID = ""
	r.application = nil
	r.window = nil
	r.observationID = ""
	r.pending = make(map[string]*nexuscua.ActionRequest)
	r.launchables = nil
	r.pendingLaunches = nil
	r.completed = make(map[string]ActionResult)
	r.completedIDs = nil
	r.mu.Unlock()
	return closeErr
}

func (r *Round) HasSession() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return !r.closed && r.sessionID != "" && (r.sessionExpiresAt.IsZero() || time.Now().Before(r.sessionExpiresAt))
}

func (r *Round) ensureOpenLocked() error {
	if r.closed {
		return ErrRoundClosed
	}
	if r.client == nil {
		return ErrNotReady
	}
	return nil
}

// invalidateSession drops all runtime-owned opaque references while keeping
// the host round reusable. A runtime session can disappear independently due
// to TTL expiry, sidecar restart, or an application-generation change. The
// old session must never remain visible as active, and a later select_target
// must be able to open a new one.
func (r *Round) invalidateSession() {
	r.mu.Lock()
	r.sessionID = ""
	r.application = nil
	r.window = nil
	r.observationID = ""
	r.pending = make(map[string]*nexuscua.ActionRequest)
	r.completed = make(map[string]ActionResult)
	r.completedIDs = nil
	r.mu.Unlock()
}

func (r *Round) handleSessionError(err error) {
	var cuaErr *nexuscua.CUAError
	if errors.As(err, &cuaErr) && cuaErr.Code == nexuscua.ErrorUserActive {
		r.mu.Lock()
		r.userActive = true
		r.observationID = ""
		r.mu.Unlock()
	}
	if isSessionUnavailable(err) {
		r.invalidateSession()
	}
}

func isSessionUnavailable(err error) bool {
	var cuaErr *nexuscua.CUAError
	return errors.As(err, &cuaErr) && cuaErr.Code == nexuscua.ErrorSessionUnavailable
}

func (r *Round) readArtifact(path string) (string, string, error) {
	root := ""
	if r.service != nil && r.service.supervisor != nil {
		root = r.service.supervisor.ArtifactRoot()
	}
	cleanPath := filepath.Clean(strings.TrimSpace(path))
	if root == "" || cleanPath == "." {
		return "", "", errors.New("Computer Use screenshot artifact root 未配置")
	}
	cleanRoot, err := filepath.Abs(root)
	if err != nil {
		return "", "", err
	}
	cleanPath, err = filepath.Abs(cleanPath)
	if err != nil {
		return "", "", err
	}
	resolvedRoot, err := filepath.EvalSymlinks(cleanRoot)
	if err != nil {
		return "", "", err
	}
	resolvedPath, err := filepath.EvalSymlinks(cleanPath)
	if err != nil {
		return "", "", err
	}
	relative, err := filepath.Rel(resolvedRoot, resolvedPath)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", "", errors.New("Computer Use screenshot artifact escaped its private root")
	}
	info, err := os.Stat(cleanPath)
	if err != nil {
		return "", "", err
	}
	if !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > maxScreenshotBytes {
		return "", "", errors.New("Computer Use screenshot artifact exceeds the bounded file policy")
	}
	file, err := os.Open(cleanPath)
	if err != nil {
		return "", "", err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxScreenshotBytes+1))
	if err != nil {
		return "", "", err
	}
	if len(data) > maxScreenshotBytes {
		return "", "", errors.New("Computer Use screenshot artifact exceeds the bounded file policy")
	}
	mime := "image/png"
	if len(data) >= 8 && string(data[:8]) != "\x89PNG\r\n\x1a\n" {
		mime = "application/octet-stream"
	}
	return base64.StdEncoding.EncodeToString(data), mime, nil
}

func observationView(value nexuscua.WindowObservation) map[string]any {
	elements := make([]map[string]any, 0, len(value.Elements))
	for _, element := range value.Elements {
		elements = append(elements, map[string]any{
			"element_ref": element.ElementRef, "parent_ref": element.ParentRef,
			"role": element.Role, "name": element.Name, "value": element.Value,
			"screen_bounds": element.ScreenBounds, "enabled": element.Enabled,
			"focused": element.Focused, "actions": element.Actions,
		})
	}
	return map[string]any{
		"observation_id": value.ObservationID, "window_ref": value.WindowRef,
		"captured_at": value.CapturedAt, "window_screen_bounds": value.WindowScreenBounds,
		"elements": elements, "elements_complete": value.ElementsComplete,
		"elements_truncation": value.ElementsTruncation,
	}
}

func actionView(value nexuscua.ActionOutput) ActionResult {
	return ActionResult{Data: map[string]any{
		"delivery_mode":           value.DeliveryMode,
		"dispatched":              value.Dispatched,
		"effect_status":           value.EffectStatus,
		"evidence_kind":           value.EvidenceKind,
		"foreground_restoration":  value.ForegroundRestoration,
		"observation_invalidated": value.ObservationInvalidated,
	}}
}

func parseAccessibility(value string) (nexuscua.AccessibilityMode, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "interactive":
		return nexuscua.AccessibilityInteractive, nil
	case "disabled":
		return nexuscua.AccessibilityDisabled, nil
	case "full":
		return nexuscua.AccessibilityFull, nil
	default:
		return "", fmt.Errorf("Computer Use accessibility 模式无效: %q", value)
	}
}

func buildAction(input ActionInput) (nexuscua.Action, error) {
	kind := strings.ToLower(strings.TrimSpace(input.Kind))
	if input.Count == 0 {
		input.Count = 1
	}
	if kind == "click_point" || kind == "press_point" || kind == "move_pointer" || kind == "scroll" || kind == "drag" {
		if input.X > 1000 || input.Y > 1000 || (kind == "drag" && (input.ToX > 1000 || input.ToY > 1000)) {
			return nil, errors.New("Computer Use 坐标必须在 0..1000 内")
		}
	}
	switch kind {
	case "restore_window":
		return nexuscua.SetWindowMinimized{Minimized: false}, nil
	case "set_window_bounds":
		if input.Bounds == nil {
			return nil, errors.New("set_window_bounds 需要 bounds")
		}
		return nexuscua.SetWindowBounds{Bounds: *input.Bounds}, nil
	case "set_range_value":
		if input.ElementRef == "" || input.RangeValue == nil {
			return nil, errors.New("set_range_value 需要 element_ref 和 range_value")
		}
		return nexuscua.SetRangeValue{ElementRef: nexuscua.ElementRef(input.ElementRef), Value: *input.RangeValue}, nil
	case "terminate_application":
		return nexuscua.TerminateApplication{}, nil
	case "focus_window":
		return nexuscua.FocusWindow{}, nil
	case "focus_element":
		if strings.TrimSpace(input.ElementRef) == "" {
			return nil, errors.New("Computer Use focus_element 需要 element_ref")
		}
		return nexuscua.FocusElement{ElementRef: nexuscua.ElementRef(input.ElementRef)}, nil
	case "invoke_element":
		if strings.TrimSpace(input.ElementRef) == "" {
			return nil, errors.New("Computer Use invoke_element 需要 element_ref")
		}
		return nexuscua.InvokeElement{ElementRef: nexuscua.ElementRef(input.ElementRef)}, nil
	case "set_value":
		if strings.TrimSpace(input.ElementRef) == "" {
			return nil, errors.New("Computer Use set_value 需要 element_ref")
		}
		return nexuscua.SetValue{ElementRef: nexuscua.ElementRef(input.ElementRef), Value: nexuscua.NewSensitiveText(input.Value)}, nil
	case "toggle_element":
		if strings.TrimSpace(input.ElementRef) == "" {
			return nil, errors.New("Computer Use toggle_element 需要 element_ref")
		}
		return nexuscua.ToggleElement{ElementRef: nexuscua.ElementRef(input.ElementRef)}, nil
	case "select_element":
		if strings.TrimSpace(input.ElementRef) == "" {
			return nil, errors.New("Computer Use select_element 需要 element_ref")
		}
		return nexuscua.SelectElement{ElementRef: nexuscua.ElementRef(input.ElementRef)}, nil
	case "set_expanded":
		if strings.TrimSpace(input.ElementRef) == "" {
			return nil, errors.New("Computer Use set_expanded 需要 element_ref")
		}
		return nexuscua.SetExpanded{ElementRef: nexuscua.ElementRef(input.ElementRef), Expanded: input.Expanded}, nil
	case "click_point", "press_point":
		button := nexuscua.PointerLeft
		switch strings.ToLower(strings.TrimSpace(input.Button)) {
		case "", "left":
		case "middle":
			button = nexuscua.PointerMiddle
		case "right":
			button = nexuscua.PointerRight
		default:
			return nil, fmt.Errorf("Computer Use click button 无效: %q", input.Button)
		}
		if kind == "press_point" {
			return nexuscua.PressPoint{Point: nexuscua.NormalizedPoint{X: float64(input.X), Y: float64(input.Y)}, Button: button, DurationMS: input.DurationMS}, nil
		}
		return nexuscua.ClickPoint{Point: nexuscua.NormalizedPoint{X: float64(input.X), Y: float64(input.Y)}, Button: button, Count: input.Count}, nil
	case "move_pointer":
		return nexuscua.MovePointer{Point: nexuscua.NormalizedPoint{X: float64(input.X), Y: float64(input.Y)}, DurationMS: input.DurationMS}, nil
	case "type_text":
		return nexuscua.TypeText{Text: nexuscua.NewSensitiveText(input.Text)}, nil
	case "press_keys":
		return nexuscua.PressKeys{Keys: append([]string(nil), input.Keys...)}, nil
	case "scroll":
		return nexuscua.Scroll{Point: nexuscua.NormalizedPoint{X: float64(input.X), Y: float64(input.Y)}, DeltaX: float64(input.DeltaX), DeltaY: float64(input.DeltaY)}, nil
	case "drag":
		return nexuscua.Drag{From: nexuscua.NormalizedPoint{X: float64(input.X), Y: float64(input.Y)}, To: nexuscua.NormalizedPoint{X: float64(input.ToX), Y: float64(input.ToY)}, DurationMS: input.DurationMS}, nil
	default:
		return nil, fmt.Errorf("Computer Use action kind 不受支持: %q", input.Kind)
	}
}

func buildPredicate(input map[string]any) (nexuscua.StatePredicate, error) {
	kind, _ := input["kind"].(string)
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "window_title_contains":
		text, _ := input["text"].(string)
		if strings.TrimSpace(text) == "" {
			return nil, errors.New("window_title_contains 需要非空 text")
		}
		return nexuscua.WindowTitleContains{Text: text}, nil
	case "element_exists":
		var role, name *string
		if value, ok := input["role"].(string); ok && strings.TrimSpace(value) != "" {
			value = strings.TrimSpace(value)
			role = &value
		}
		if value, ok := input["name"].(string); ok && strings.TrimSpace(value) != "" {
			value = strings.TrimSpace(value)
			name = &value
		}
		if role == nil && name == nil {
			return nil, errors.New("element_exists 需要 role 或 name")
		}
		return nexuscua.ElementExists{Role: role, Name: name}, nil
	default:
		return nil, fmt.Errorf("Computer Use predicate kind 无效: %q", kind)
	}
}

// ProcessSupervisor is the production local supervisor. It starts only an
// already installed, host-verified binary configured by Nexus.
type ProcessSupervisor struct {
	mu         sync.Mutex
	startMu    sync.Mutex
	binaryPath string
	stateRoot  string
	endpoint   string
	tokenFile  string
	artifact   string
	process    *managedProcess
	client     Client
	factory    Factory
	logger     *slog.Logger
}

type managedProcess struct {
	cmd         *exec.Cmd
	done        chan struct{}
	err         error
	stdoutStats *sidecarOutputStats
	stderrStats *sidecarOutputStats
}

// sidecarOutputStats records only bounded counters. Sidecar output may contain
// driver or application data and must never be copied into Nexus logs.
type sidecarOutputStats struct {
	mu    sync.Mutex
	bytes int64
	lines int64
}

func (s *sidecarOutputStats) Write(value []byte) (int, error) {
	s.mu.Lock()
	s.bytes += int64(len(value))
	for _, char := range value {
		if char == '\n' {
			s.lines++
		}
	}
	s.mu.Unlock()
	return len(value), nil
}

func (s *sidecarOutputStats) snapshot() (int64, int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.bytes, s.lines
}

func NewProcessSupervisor(cfg config.Config) *ProcessSupervisor {
	return NewProcessSupervisorWithBinaryPath(cfg, cfg.ComputerUseBinaryPath)
}

// cuaStateRoot anchors every local Computer Use artifact beside the SQLite
// database file. DATABASE_URL may carry a sqlite:// scheme prefix; the DSN
// normalization in storage intentionally rewrites those URLs into driver
// form, so this anchors on the raw path instead of reusing it.
func cuaStateRoot(cfg config.Config) string {
	return filepath.Join(filepath.Dir(trimDatabaseScheme(cfg.DatabaseURL)), "computer-use")
}

// trimDatabaseScheme removes one optional sqlite:// scheme prefix while
// keeping the remainder verbatim, so sqlite:///abs paths stay absolute.
func trimDatabaseScheme(value string) string {
	value = strings.TrimSpace(value)
	if lower := strings.ToLower(value); strings.HasPrefix(lower, "sqlite://") {
		return value[len("sqlite://"):]
	}
	return value
}

func NewProcessSupervisorWithBinaryPath(cfg config.Config, binaryPath string) *ProcessSupervisor {
	stateRoot := cuaStateRoot(cfg)
	return &ProcessSupervisor{
		binaryPath: strings.TrimSpace(binaryPath),
		stateRoot:  stateRoot,
		factory: func(value nexuscua.Config) (Client, error) {
			return nexuscua.NewClient(value)
		},
		logger: slog.Default(),
	}
}

func (s *ProcessSupervisor) SetLogger(logger *slog.Logger) {
	if logger == nil {
		logger = slog.Default()
	}
	s.mu.Lock()
	s.logger = logger
	s.mu.Unlock()
}

// SetBinaryPath changes the executable used by the next sidecar epoch. The
// caller must stop the current process first so existing opaque refs cannot
// survive an installation switch.
func (s *ProcessSupervisor) SetBinaryPath(path string) error {
	path = strings.TrimSpace(path)
	if !isExecutableRegularFile(path) {
		return errors.New("Computer Use binary path 必须指向可执行普通文件")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.process != nil {
		select {
		case <-s.process.done:
		default:
			return errors.New("Computer Use sidecar 仍在运行")
		}
	}
	s.binaryPath = path
	return nil
}

func (s *ProcessSupervisor) ArtifactRoot() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.artifact
}

func (s *ProcessSupervisor) Start(ctx context.Context) (Client, error) {
	s.startMu.Lock()
	defer s.startMu.Unlock()
	s.mu.Lock()
	if s.process != nil {
		select {
		case <-s.process.done:
			// The watcher has already reaped this child. A new epoch gets a
			// fresh token, endpoint and artifact root below.
			s.process = nil
			s.client = nil
			s.endpoint = ""
			s.tokenFile = ""
			s.artifact = ""
		default:
		}
	}
	if s.client != nil && s.process != nil {
		client := s.client
		s.mu.Unlock()
		return client, nil
	}
	if s.binaryPath == "" {
		s.mu.Unlock()
		return nil, errors.New("Computer Use binary path 未配置")
	}
	stateRoot, binaryPath, factory := s.stateRoot, s.binaryPath, s.factory
	s.mu.Unlock()
	if !filepath.IsAbs(binaryPath) {
		return nil, errors.New("Computer Use binary path 必须是绝对路径")
	}
	info, err := os.Lstat(binaryPath)
	if err != nil {
		return nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, errors.New("Computer Use binary path 必须指向普通文件")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		return nil, errors.New("Computer Use binary path 不可执行")
	}
	if err := os.MkdirAll(stateRoot, 0o700); err != nil {
		return nil, err
	}
	if err := os.Chmod(stateRoot, 0o700); err != nil {
		return nil, err
	}
	endpoint := filepath.Join(stateRoot, "service.sock")
	if runtime.GOOS == "windows" {
		endpoint = `\\.\pipe\nexus-cua-` + randomSuffix()
	}
	tokenFile := filepath.Join(stateRoot, "token")
	artifactRoot := filepath.Join(stateRoot, "artifacts")
	_ = os.Remove(endpoint)
	_ = os.Remove(tokenFile)
	if err := os.RemoveAll(artifactRoot); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(artifactRoot, 0o700); err != nil {
		return nil, err
	}
	token := make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return nil, err
	}
	if err := os.WriteFile(tokenFile, []byte(hex.EncodeToString(token)), 0o600); err != nil {
		return nil, err
	}
	cmd := exec.Command(binaryPath, "serve", "--endpoint", endpoint, "--token-file", tokenFile, "--artifact-root", artifactRoot)
	stdoutStats := &sidecarOutputStats{}
	stderrStats := &sidecarOutputStats{}
	cmd.Stdout = stdoutStats
	cmd.Stderr = stderrStats
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	process := &managedProcess{
		cmd: cmd, done: make(chan struct{}),
		stdoutStats: stdoutStats, stderrStats: stderrStats,
	}
	s.mu.Lock()
	s.process = process
	s.client = nil
	s.endpoint = endpoint
	s.tokenFile = tokenFile
	s.artifact = artifactRoot
	logger := s.logger
	s.mu.Unlock()
	if logger != nil {
		logger.InfoContext(ctx, "Computer Use sidecar started")
	}
	go s.waitProcess(process)
	client, err := factory(nexuscua.Config{Endpoint: endpoint, TokenFile: tokenFile})
	if err != nil {
		_ = s.stopManagedProcessWithTimeout(process, 2*time.Second)
		return nil, err
	}
	if err := waitUntilReady(ctx, client); err != nil {
		_ = s.stopManagedProcessWithTimeout(process, 2*time.Second)
		return nil, err
	}
	s.mu.Lock()
	if s.process != process {
		s.mu.Unlock()
		return nil, ErrNotReady
	}
	s.client = client
	s.mu.Unlock()
	return client, nil
}

func (s *ProcessSupervisor) Stop(ctx context.Context) error {
	s.mu.Lock()
	process := s.process
	s.process, s.client = nil, nil
	s.endpoint, s.tokenFile, s.artifact = "", "", ""
	s.mu.Unlock()
	if process == nil {
		return nil
	}
	return s.stopManagedProcess(ctx, process)
}

func (s *ProcessSupervisor) Restart(ctx context.Context) (Client, error) {
	if err := s.Stop(ctx); err != nil {
		return nil, err
	}
	return s.Start(ctx)
}

func waitUntilReady(ctx context.Context, client Client) error {
	deadline := time.Now().Add(readinessTimeout)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return err
		}
		probeCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
		capabilities, err := client.GetCapabilities(probeCtx, 500*time.Millisecond)
		cancel()
		if err == nil && capabilities.ProtocolVersion == protocolVersion {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	return ErrNotReady
}

func (s *ProcessSupervisor) waitProcess(process *managedProcess) {
	err := process.cmd.Wait()
	s.mu.Lock()
	process.err = normalizeProcessExit(err)
	logger := s.logger
	if s.process == process {
		s.process = nil
		s.client = nil
		s.endpoint, s.tokenFile, s.artifact = "", "", ""
	}
	close(process.done)
	s.mu.Unlock()
	if logger != nil {
		stdoutBytes, stdoutLines := process.stdoutStats.snapshot()
		stderrBytes, stderrLines := process.stderrStats.snapshot()
		logger.Warn("Computer Use sidecar exited",
			"error", process.err,
			"stdout_bytes", stdoutBytes,
			"stdout_lines", stdoutLines,
			"stderr_bytes", stderrBytes,
			"stderr_lines", stderrLines,
		)
	}
}

func (s *ProcessSupervisor) stopManagedProcess(ctx context.Context, process *managedProcess) error {
	if process == nil || process.cmd == nil || process.cmd.Process == nil {
		return nil
	}
	if runtime.GOOS == "windows" {
		_ = process.cmd.Process.Kill()
	} else {
		_ = process.cmd.Process.Signal(os.Interrupt)
	}
	select {
	case <-process.done:
		s.mu.Lock()
		err := process.err
		s.mu.Unlock()
		return err
	case <-ctx.Done():
		_ = process.cmd.Process.Kill()
		<-process.done
		return ctx.Err()
	}
}

func (s *ProcessSupervisor) stopManagedProcessWithTimeout(process *managedProcess, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.stopManagedProcess(ctx, process)
}

func sameClient(left, right Client) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	leftValue, rightValue := reflect.ValueOf(left), reflect.ValueOf(right)
	if leftValue.Type() != rightValue.Type() || !leftValue.Type().Comparable() {
		return false
	}
	return leftValue.Interface() == rightValue.Interface()
}

func normalizeProcessExit(err error) error {
	if err == nil {
		return nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ProcessState != nil {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() && status.Signal() == syscall.SIGINT {
			return nil
		}
	}
	return err
}

func randomSuffix() string {
	var data [8]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "fallback"
	}
	return hex.EncodeToString(data[:])
}
