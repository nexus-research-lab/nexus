package memorymaintenance

// 本文件验证 Nexus 只唤醒启用且到期的 Agent，并遵守 nxs 返回的 next check。

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	agentsvc "github.com/nexus-research-lab/nexus/internal/service/agent"
	preferencessvc "github.com/nexus-research-lab/nexus/internal/service/preferences"
	runtimeselectionsvc "github.com/nexus-research-lab/nexus/internal/service/runtimeselection"
)

type fakeAgentCatalog struct {
	agents []protocol.Agent
}

func (f fakeAgentCatalog) ListAllAgentRecordsForMaintenance(context.Context) ([]protocol.Agent, error) {
	return append([]protocol.Agent(nil), f.agents...), nil
}

func (f fakeAgentCatalog) EnsureRuntimeSettingsProjection(agentValue protocol.Agent) error {
	return agentsvc.EnsureRuntimeSettingsProjection(agentValue)
}

func (f fakeAgentCatalog) LoadRuntimeSettingsProjection(agentValue protocol.Agent) (map[string]any, error) {
	return agentsvc.LoadRuntimeSettingsProjection(agentValue.WorkspacePath)
}

type fakeDreamRunner struct {
	mu     sync.Mutex
	calls  []string
	result agentclient.AutoDreamResult
	done   chan struct{}
}

type fakePreferencesService struct {
	preferences preferencessvc.Preferences
}

func (f fakePreferencesService) Get(context.Context, string) (preferencessvc.Preferences, error) {
	return f.preferences, nil
}

func (f *fakeDreamRunner) tryAutoDream(_ context.Context, agentValue protocol.Agent) (agentclient.AutoDreamResult, error) {
	f.mu.Lock()
	f.calls = append(f.calls, agentValue.AgentID)
	f.mu.Unlock()
	select {
	case f.done <- struct{}{}:
	default:
	}
	return f.result, nil
}

func (f *fakeDreamRunner) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func TestCoordinatorRunOnceRespectsEnabledAndNextCheck(t *testing.T) {
	now := time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)
	enabledAgent := newDreamTestAgent(t, "agent-enabled", true)
	disabledAgent := newDreamTestAgent(t, "agent-disabled", false)
	runner := &fakeDreamRunner{
		done: make(chan struct{}, 2),
		result: agentclient.AutoDreamResult{
			Status:        agentclient.AutoDreamStatusSkipped,
			Reason:        "time_gate",
			NextCheckAtMS: now.Add(time.Hour).UnixMilli(),
		},
	}
	coordinator := newCoordinator(config.MemoryMaintenanceConfig{
		MaxConcurrent: 1,
		RunTimeout:    time.Minute,
		SweepInterval: 10 * time.Minute,
	}, fakeAgentCatalog{agents: []protocol.Agent{enabledAgent, disabledAgent}}, fakePreferencesService{}, runner)
	coordinator.now = func() time.Time { return now }

	if err := coordinator.runOnce(context.Background()); err != nil {
		t.Fatalf("runOnce() error = %v", err)
	}
	waitDreamCall(t, runner.done)
	waitForCallCount(t, runner, 1)
	if err := coordinator.runOnce(context.Background()); err != nil {
		t.Fatalf("second RunOnce() error = %v", err)
	}
	time.Sleep(20 * time.Millisecond)
	if got := runner.callCount(); got != 1 {
		t.Fatalf("call count = %d, want next-check suppression", got)
	}
	coordinator.Stop()
}

func TestCoordinatorBacksOffUnavailableProvider(t *testing.T) {
	now := time.Date(2026, 8, 4, 17, 0, 0, 0, time.UTC)
	agentValue := newDreamTestAgent(t, "agent-unavailable", true)
	runner := &fakeDreamRunner{
		done: make(chan struct{}, 2),
		result: agentclient.AutoDreamResult{
			Status: agentclient.AutoDreamStatusSkipped,
			Reason: autoDreamProviderUnavailableReason,
		},
	}
	coordinator := newCoordinator(config.MemoryMaintenanceConfig{
		MaxConcurrent: 1,
		RunTimeout:    time.Minute,
		SweepInterval: 10 * time.Minute,
	}, fakeAgentCatalog{agents: []protocol.Agent{agentValue}}, fakePreferencesService{}, runner)
	coordinator.now = func() time.Time { return now }

	if err := coordinator.runOnce(context.Background()); err != nil {
		t.Fatalf("runOnce() error = %v", err)
	}
	waitDreamCall(t, runner.done)
	coordinator.wg.Wait()
	key := agentKey(agentValue)
	coordinator.mu.Lock()
	nextCheck := coordinator.nextChecks[key]
	coordinator.mu.Unlock()
	wantNextCheck := now.Add(autoDreamProviderUnavailableRetryInterval)
	if !nextCheck.Equal(wantNextCheck) {
		t.Fatalf("next check = %s, want %s", nextCheck, wantNextCheck)
	}

	now = wantNextCheck.Add(-time.Minute)
	if err := coordinator.runOnce(context.Background()); err != nil {
		t.Fatalf("early runOnce() error = %v", err)
	}
	if got := runner.callCount(); got != 1 {
		t.Fatalf("call count = %d, want provider backoff suppression", got)
	}
}

func TestCoordinatorRunOnceRespectsUserAutoDreamPreference(t *testing.T) {
	agentValue := newDreamTestAgent(t, "agent-disabled-by-user", true)
	disabled := false
	runner := &fakeDreamRunner{done: make(chan struct{}, 1)}
	coordinator := newCoordinator(config.MemoryMaintenanceConfig{
		MaxConcurrent: 1,
		RunTimeout:    time.Minute,
		SweepInterval: 10 * time.Minute,
	}, fakeAgentCatalog{agents: []protocol.Agent{agentValue}}, fakePreferencesService{
		preferences: preferencessvc.Preferences{
			RuntimeSettings: preferencessvc.RuntimeSettings{
				"nxs": {AutoDreamEnabled: &disabled},
			},
		},
	}, runner)

	if err := coordinator.runOnce(context.Background()); err != nil {
		t.Fatalf("runOnce() error = %v", err)
	}
	if got := runner.callCount(); got != 0 {
		t.Fatalf("call count = %d, want user preference suppression", got)
	}
}

func TestRuntimeDreamRunnerPrefersOwnerBackgroundSelection(t *testing.T) {
	runner := &runtimeDreamRunner{preferences: fakePreferencesService{preferences: preferencessvc.Preferences{
		DefaultBackgroundModelSelection: preferencessvc.ModelSelection{
			Provider: "background-provider",
			Model:    "background-model",
		},
	}}}
	provider, model, available, err := runner.backgroundSelection(context.Background(), "owner-1", runtimeselectionsvc.Selection{
		Provider: "agent-provider",
		Model:    "agent-model",
	})
	if err != nil {
		t.Fatalf("backgroundSelection() error = %v", err)
	}
	if !available || provider != "background-provider" || model != "background-model" {
		t.Fatalf("background selection = %s/%s, want owner background provider/model", provider, model)
	}
}

func TestRuntimeDreamRunnerFallsBackToAgentSelection(t *testing.T) {
	runner := &runtimeDreamRunner{preferences: fakePreferencesService{}}
	provider, model, available, err := runner.backgroundSelection(context.Background(), "owner-1", runtimeselectionsvc.Selection{
		Provider: "agent-provider",
		Model:    "agent-model",
	})
	if err != nil {
		t.Fatalf("backgroundSelection() error = %v", err)
	}
	if !available || provider != "agent-provider" || model != "agent-model" {
		t.Fatalf("background selection = %s/%s, want Agent fallback", provider, model)
	}
}

func TestRuntimeDreamRunnerSkipsUnavailableSelection(t *testing.T) {
	runner := &runtimeDreamRunner{preferences: fakePreferencesService{}}
	provider, model, available, err := runner.backgroundSelection(
		context.Background(),
		"owner-1",
		runtimeselectionsvc.Selection{},
	)
	if err != nil {
		t.Fatalf("backgroundSelection() error = %v", err)
	}
	if available || provider != "" || model != "" {
		t.Fatalf("background selection = %s/%s available=%t, want unavailable", provider, model, available)
	}
}

func TestRuntimeDreamRunnerSkipsDisabledAutoDreamBeforeWorkspaceSetup(t *testing.T) {
	disabled := false
	preferences := fakePreferencesService{preferences: preferencessvc.Preferences{
		RuntimeSettings: preferencessvc.RuntimeSettings{
			"nxs": {AutoDreamEnabled: &disabled},
		},
	}}
	runner := &runtimeDreamRunner{
		preferences: preferences,
		selector:    runtimeselectionsvc.NewService(preferences),
	}

	result, err := runner.tryAutoDream(context.Background(), protocol.Agent{OwnerUserID: "owner-1"})
	if err != nil {
		t.Fatalf("tryAutoDream() error = %v", err)
	}
	if result.Status != agentclient.AutoDreamStatusSkipped || result.Reason != "gate_closed" {
		t.Fatalf("tryAutoDream() result = %+v, want disabled gate", result)
	}
}

func TestRuntimeDreamRunnerMaintainsClaudeAgentThroughNXS(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("NEXUS_STATE_ROOT", stateRoot)
	cfg := config.Config{WorkspacePath: filepath.Join(stateRoot, "users")}
	agentValue := protocol.Agent{
		AgentID:       "claude-agent",
		OwnerUserID:   "owner-1",
		WorkspacePath: agentsvc.ResolveWorkspacePath(cfg, "owner-1", "claude-agent"),
	}
	if err := os.MkdirAll(agentValue.WorkspacePath, 0o755); err != nil {
		t.Fatalf("MkdirAll(workspace) error = %v", err)
	}
	prefs := fakePreferencesService{preferences: preferencessvc.Preferences{AgentRuntimeKind: "claude"}}
	runner := &runtimeDreamRunner{
		config:      cfg,
		preferences: prefs,
		selector:    runtimeselectionsvc.NewService(prefs),
	}

	result, err := runner.tryAutoDream(context.Background(), agentValue)
	if err != nil {
		t.Fatalf("tryAutoDream() error = %v", err)
	}
	if result.Reason != autoDreamProviderUnavailableReason {
		t.Fatalf("tryAutoDream() reason = %q, want nxs maintenance to reach provider selection", result.Reason)
	}
}

func TestDreamSessionCancellationForcesClose(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	closed := make(chan struct{})
	stop := closeDreamSessionOnCancellation(ctx, func() {
		close(closed)
	})
	cancel()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatal("AutoDream admission 撤销后未强制关闭 bridge session")
	}
	stop()
}

func newDreamTestAgent(t *testing.T, agentID string, enabled bool) protocol.Agent {
	t.Helper()
	workspace := t.TempDir()
	settingsPath := filepath.Join(workspace, ".nexus", "settings.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	payload, err := json.Marshal(map[string]any{
		"memory": map[string]any{
			"dream": map[string]any{"enabled": enabled},
		},
	})
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if err = os.WriteFile(settingsPath, payload, 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return protocol.Agent{
		AgentID:       agentID,
		OwnerUserID:   "owner-1",
		WorkspacePath: workspace,
		Options:       protocol.Options{Provider: "provider", Model: "model"},
	}
}

func waitDreamCall(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Dream call")
	}
}

func waitForCallCount(t *testing.T, runner *fakeDreamRunner, want int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if runner.callCount() == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("call count = %d, want %d", runner.callCount(), want)
}
