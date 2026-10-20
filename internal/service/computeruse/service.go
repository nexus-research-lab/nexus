// INPUT: Nexus round identity and the pinned typed nexus-cua client.
// OUTPUT: owner-scoped Computer Use sessions, opaque target selection, bounded observations, and one-shot mutation reconciliation.
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
	defaultSessionTTL     = 120
	maxScreenshotBytes    = 16 << 20
	maxActionKeys         = 32
	maxActionTextBytes    = 32 << 10
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
	service       *Service
	owner         string
	id            string
	client        Client
	mu            sync.Mutex
	mutationMu    sync.Mutex
	closed        bool
	application   *nexuscua.DiscoveredApplication
	sessionID     nexuscua.SessionID
	window        *nexuscua.WindowSummary
	observationID nexuscua.ObservationID
	pending       map[string]*nexuscua.ActionRequest
	completed     map[string]ActionResult
	completedIDs  []string
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
	r.completed = make(map[string]ActionResult)
	r.completedIDs = nil
	r.mu.Unlock()
}

func (r *Round) ID() string { return r.id }

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
		"protocol_version":   capabilities.ProtocolVersion,
		"runtime_version":    capabilities.RuntimeVersion,
		"platform":           capabilities.Platform,
		"capture_modes":      capabilities.CaptureModes,
		"accessibility_tree": capabilities.AccessibilityTree,
		"input_routes":       capabilities.InputRoutes,
		"actions":            capabilities.Actions,
		"permissions": map[string]any{
			"screen_capture": permissions.ScreenCapture,
			"accessibility":  permissions.Accessibility,
			"input_control":  permissions.InputControl,
		},
		"session_active": r.HasSession(),
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

func (r *Round) SelectTarget(ctx context.Context, selector string) (map[string]any, error) {
	r.mutationMu.Lock()
	defer r.mutationMu.Unlock()
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
	selected, err := nexuscua.SelectApplication(apps.Applications, strings.TrimSpace(selector))
	if err != nil {
		return nil, err
	}
	capabilities, err := r.client.GetCapabilities(ctx, defaultCommandTimeout)
	if err != nil {
		return nil, err
	}
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
	session, err := r.client.OpenSession(ctx, nexuscua.OpenSessionInput{Manifest: nexuscua.CapabilityManifest{
		Mode: nexuscua.PermissionBounded, ApplicationRefs: []nexuscua.DiscoveryRef{selected.DiscoveryRef},
		AllowedActions: capabilities.Actions, AllowForegroundInput: true, TTLSeconds: defaultSessionTTL,
	}}, defaultCommandTimeout)
	if err != nil {
		return nil, err
	}
	r.mu.Lock()
	r.application = selected
	r.sessionID = session.SessionID
	r.window = nil
	r.observationID = ""
	r.mu.Unlock()
	return map[string]any{
		"name": selected.Name, "application_id": selected.ApplicationID,
		"foreground": selected.Foreground, "session_active": true,
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
			"minimized":  window.Minimized,
		})
	}
	return map[string]any{"windows": values}, nil
}

type ObserveInput struct {
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
		window, err = selectWindowByRefOrTitle(windows, selector)
		if err != nil {
			return Observation{}, err
		}
	}
	accessibility, err := parseAccessibility(input.Accessibility)
	if err != nil {
		return Observation{}, err
	}
	value, err := r.client.ObserveWindow(ctx, nexuscua.ObserveWindowInput{
		SessionID: sessionID, WindowRef: window.WindowRef,
		IncludeScreenshot: input.IncludeScreenshot, Accessibility: accessibility,
	}, defaultCommandTimeout)
	if err != nil {
		r.handleSessionError(err)
		return Observation{}, err
	}
	r.mu.Lock()
	r.window = window
	r.observationID = value.ObservationID
	r.mu.Unlock()
	data := observationView(value)
	result := Observation{Data: data}
	if value.Screenshot != nil && input.IncludeScreenshot {
		result.Data["screenshot"] = map[string]any{
			"artifact_ref": value.Screenshot.ArtifactRef,
			"mime_type":    value.Screenshot.MIMEType,
			"byte_length":  value.Screenshot.ByteLength,
			"sha256":       value.Screenshot.SHA256,
			"mapping":      value.Screenshot.Mapping,
		}
		if encoded, mime, readErr := r.readArtifact(value.Screenshot.Path); readErr != nil {
			r.mu.Lock()
			if r.observationID == value.ObservationID {
				r.observationID = ""
			}
			r.mu.Unlock()
			return Observation{}, readErr
		} else {
			result.ImageData, result.ImageMIME = encoded, mime
		}
	}
	return result, nil
}

func selectWindowByRefOrTitle(windows []nexuscua.WindowSummary, selector string) (*nexuscua.WindowSummary, error) {
	matches := make([]int, 0, 1)
	for index := range windows {
		if string(windows[index].WindowRef) == selector {
			matches = append(matches, index)
		}
	}
	if len(matches) == 1 {
		return &windows[matches[0]], nil
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("nexus-cua: window ref %q has %d matches", selector, len(matches))
	}
	return nexuscua.SelectWindow(windows, selector)
}

type ActionInput struct {
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
	Data map[string]any
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
		result := actionView(output)
		r.mu.Lock()
		delete(r.pending, requestID)
		r.observationID = ""
		r.rememberActionResultLocked(requestID, result)
		r.mu.Unlock()
		return result, nil
	}
	if len(r.pending) > 0 {
		r.mu.Unlock()
		return ActionResult{}, ErrMutationPending
	}
	if r.sessionID == "" || r.window == nil {
		r.mu.Unlock()
		return ActionResult{}, ErrNoTarget
	}
	if r.observationID == "" {
		r.mu.Unlock()
		return ActionResult{}, ErrNoObservation
	}
	if len(r.pending) >= maxPendingActions {
		r.mu.Unlock()
		return ActionResult{}, errors.New("Computer Use pending mutation 超出上限，请先对账或关闭 target")
	}
	sessionID, windowRef, observationID := r.sessionID, r.window.WindowRef, r.observationID
	r.mu.Unlock()
	action, err := buildAction(input)
	if err != nil {
		return ActionResult{}, err
	}
	if approve != nil {
		if err := approve(); err != nil {
			return ActionResult{}, err
		}
	}
	output, actionRequest, err := r.client.PerformAction(ctx, nexuscua.PerformActionInput{
		SessionID: sessionID, WindowRef: windowRef, ObservationID: observationID, Action: action,
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
	result := actionView(output)
	r.mu.Lock()
	r.observationID = ""
	r.rememberActionResultLocked(requestID, result)
	r.mu.Unlock()
	return result, nil
}

func (r *Round) rememberActionResultLocked(requestID string, result ActionResult) {
	if r.completed == nil {
		r.completed = make(map[string]ActionResult)
	}
	if _, exists := r.completed[requestID]; !exists {
		r.completedIDs = append(r.completedIDs, requestID)
	}
	r.completed[requestID] = cloneActionResult(result)
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
	return ActionResult{Data: data}
}

// ActionApprovalRequired reports whether requestID would dispatch a new
// mutation. Completed actions and indeterminate actions already admitted to
// the sidecar are reconciled under the original request identity and must not
// prompt the human a second time.
func (r *Round) ActionApprovalRequired(requestID string) (bool, error) {
	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return false, errors.New("Computer Use perform_action 需要 request_id")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err := r.ensureOpenLocked(); err != nil {
		return false, err
	}
	if _, ok := r.completed[requestID]; ok {
		return false, nil
	}
	if _, ok := r.pending[requestID]; ok {
		return false, nil
	}
	if len(r.pending) > 0 {
		return false, ErrMutationPending
	}
	if r.sessionID == "" || r.window == nil {
		return false, ErrNoTarget
	}
	if r.observationID == "" {
		return false, ErrNoObservation
	}
	return true, nil
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
	r.mutationMu.Lock()
	defer r.mutationMu.Unlock()
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return nil
	}
	r.closed = true
	sessionID := r.sessionID
	r.sessionID = ""
	r.application = nil
	r.window = nil
	r.observationID = ""
	r.pending = make(map[string]*nexuscua.ActionRequest)
	r.completed = make(map[string]ActionResult)
	r.completedIDs = nil
	r.mu.Unlock()
	if sessionID == "" {
		return nil
	}
	err := r.client.CloseSession(ctx, sessionID, defaultCommandTimeout)
	if isSessionUnavailable(err) {
		return nil
	}
	return err
}

func (r *Round) HasSession() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return !r.closed && r.sessionID != ""
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
	if len(input.Keys) > maxActionKeys {
		return nil, errors.New("Computer Use keys 超出上限")
	}
	if len(input.Value) > maxActionTextBytes || len(input.Text) > maxActionTextBytes {
		return nil, errors.New("Computer Use 文本动作超出上限")
	}
	switch kind {
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
	case "click_point":
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
		return nexuscua.ClickPoint{Point: nexuscua.ScreenshotPoint{X: input.X, Y: input.Y}, Button: button, Count: input.Count}, nil
	case "move_pointer":
		return nexuscua.MovePointer{Point: nexuscua.ScreenshotPoint{X: input.X, Y: input.Y}, DurationMS: input.DurationMS}, nil
	case "type_text":
		return nexuscua.TypeText{Text: nexuscua.NewSensitiveText(input.Text)}, nil
	case "press_keys":
		return nexuscua.PressKeys{Keys: append([]string(nil), input.Keys...)}, nil
	case "scroll":
		return nexuscua.Scroll{DeltaX: float64(input.DeltaX), DeltaY: float64(input.DeltaY)}, nil
	case "drag":
		return nexuscua.Drag{From: nexuscua.ScreenshotPoint{X: input.X, Y: input.Y}, To: nexuscua.ScreenshotPoint{X: input.ToX, Y: input.ToY}, DurationMS: input.DurationMS}, nil
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
