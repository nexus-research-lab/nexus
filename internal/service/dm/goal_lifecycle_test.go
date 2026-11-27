// goal_lifecycle_test.go 覆盖 DM Goal 续跑与终态结算测试。
package dm

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	exec "github.com/nexus-research-lab/nexus/internal/runtime/exec"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
	_ "modernc.org/sqlite"
)

func TestDMParentTerminalUsageBackgroundRetryRecoversWithoutChildOrNewMessage(t *testing.T) {
	const (
		sessionKey = "agent:nexus:ws:dm:parent-terminal-background-retry"
		goalID     = "goal-parent-terminal-background-retry"
		roundID    = "round-parent-terminal-background-retry"
	)
	provider := newRetryingDMGoalUsageProvider(sessionKey, goalID, 0)
	provider.claimFailuresRemaining = runtimehost.GoalUsagePersistAttempts * 2
	provider.finalizeFailuresRemaining = runtimehost.GoalUsagePersistAttempts * 2
	var dispatches atomic.Int64
	runner := &roundRunner{
		service:               &Service{goals: provider, Host: runtimehost.Host{Runtime: runtimectx.NewManager()}},
		sessionKey:            sessionKey,
		roundID:               roundID,
		ownerUserID:           "owner-parent-terminal-background-retry",
		GoalRoundState:        runtimehost.GoalRoundState{IDForUsage: goalID, ChildIDForUsage: goalID, Usage: goalsvc.NewRuntimeUsageAccumulator(true), UsageClaimPending: true},
		runtimeKind:           "nxs",
		postRoundDispatchHook: func() { dispatches.Add(1) },
	}
	accelerateDMGoalUsageRetry(runner)
	result := exec.RoundExecutionResult{
		Usage: sdkprotocol.TokenUsage{
			InputTokens:  12,
			OutputTokens: 3,
			TotalTokens:  15,
		},
	}

	// The first synchronous window exhausts while the parent is still running.
	// A manually requested worker must not settle or dispatch before terminal.
	runner.finalizeGoalUsage(context.Background(), result, nil)
	if attempts := provider.claimAttemptCount(); attempts != runtimehost.GoalUsagePersistAttempts {
		t.Fatalf("claim attempts before parent terminal = %d, want %d", attempts, runtimehost.GoalUsagePersistAttempts)
	}
	runner.startGoalUsageRetryWorker()
	waitForDMGoalUsageRetry(t, func() bool {
		runner.Mu.Lock()
		defer runner.Mu.Unlock()
		return !runner.UsageRetrying
	})
	if attempts := provider.claimAttemptCount(); attempts != runtimehost.GoalUsagePersistAttempts {
		t.Fatalf("parent-active retry advanced claim attempts to %d", attempts)
	}
	if got := dispatches.Load(); got != 0 {
		t.Fatalf("parent-active post-round dispatches = %d, want 0", got)
	}

	// No child and no subsequent runtime message arrives. The terminal join
	// exhausts its second synchronous window, then the single background worker
	// must recover the claim, terminal delta, finalization fence, and dispatch.
	runner.markSubagentParentTerminal(subagentParentTerminalNormal)
	if runner.completeSubagentJoinAfterParentTerminal() {
		t.Fatal("terminal usage unexpectedly settled inside the exhausted synchronous window")
	}
	var starters sync.WaitGroup
	for range 16 {
		starters.Add(1)
		go func() {
			defer starters.Done()
			runner.startGoalUsageRetryWorker()
		}()
	}
	starters.Wait()

	waitForDMGoalUsageRetry(t, func() bool {
		return dispatches.Load() == 1 &&
			provider.finalizeCallCount() == runtimehost.GoalUsagePersistAttempts*2+1 &&
			dmGoalUsageRetryStopped(runner)
	})
	if attempts := provider.claimAttemptCount(); attempts != runtimehost.GoalUsagePersistAttempts*2+1 {
		t.Fatalf("claim attempts = %d, want two failed windows plus one background success", attempts)
	}
	if calls := provider.finalizeCallCount(); calls != runtimehost.GoalUsagePersistAttempts*2+1 {
		t.Fatalf("finalization calls = %d, want two failed windows plus one background success", calls)
	}
	deltas := provider.finalizedDeltas()
	if len(deltas) == 0 || deltas[len(deltas)-1].ActualTokens() != 15 {
		t.Fatalf("terminal finalization deltas = %#v, want retained provider total 15", deltas)
	}
	if got := dispatches.Load(); got != 1 {
		t.Fatalf("post-round dispatches after terminal recovery = %d, want exactly 1", got)
	}
}

func TestDMSubagentUsageBackgroundRetryAlsoRetriesFinalizationUntilDispatch(t *testing.T) {
	const (
		sessionKey = "agent:nexus:ws:dm:finalize-background-retry"
		goalID     = "goal-finalize-background-retry"
		roundID    = "round-finalize-background-retry"
	)
	provider := newRetryingDMGoalUsageProvider(sessionKey, goalID, 6)
	provider.finalizeFailuresRemaining = 6
	var dispatches atomic.Int64
	runner := &roundRunner{
		service:                &Service{goals: provider, Host: runtimehost.Host{Runtime: runtimectx.NewManager()}},
		sessionKey:             sessionKey,
		roundID:                roundID,
		ownerUserID:            "owner-finalize-background-retry",
		GoalRoundState:         runtimehost.GoalRoundState{IDForUsage: goalID, ChildIDForUsage: goalID, Usage: goalsvc.NewRuntimeUsageAccumulator(true)},
		runtimeKind:            "nxs",
		goalTokenUsageObserved: true,
		postRoundDispatchHook:  func() { dispatches.Add(1) },
	}
	accelerateDMGoalUsageRetry(runner)
	runner.markSubagentParentTerminal(subagentParentTerminalNormal)
	runner.rememberSubagentTaskMessage(dmSubagentTaskMessage("task_started", "running"))
	terminalMessage := dmSubagentTaskMessage("task_notification", "completed")
	terminalMessage["metadata"].(map[string]any)["usage"] = map[string]any{"total_tokens": int64(100)}

	runner.recordSubagentGoalUsage(context.Background(), terminalMessage)
	runner.rememberSubagentTaskMessage(terminalMessage)

	waitForDMGoalUsageRetry(t, func() bool {
		return dispatches.Load() == 1 && dmGoalUsageRetryStopped(runner)
	})
	if calls := provider.finalizeCallCount(); calls != 7 {
		t.Fatalf("finalization calls = %d, want 6 failures plus one background success", calls)
	}
	if got := dispatches.Load(); got != 1 {
		t.Fatalf("post-round dispatches after finalization recovery = %d, want exactly 1", got)
	}
}

func TestCompletedDMGoalWithoutTokenUsageSettlesWithoutFinalizationFence(t *testing.T) {
	const (
		sessionKey = "agent:nexus:ws:dm:missing-terminal-usage"
		goalID     = "goal-missing-terminal-usage"
	)
	base := &fakeGoalContextProvider{}
	provider := &fakeDMGoalUsageFinalizer{
		fakeGoalContextProvider: base,
		report: protocol.GoalUsageReport{
			GoalID:     goalID,
			SessionKey: sessionKey,
			Status:     protocol.GoalStatusComplete,
		},
	}
	runner := &roundRunner{
		service:        &Service{goals: provider, Host: runtimehost.Host{Runtime: runtimectx.NewManager()}},
		sessionKey:     sessionKey,
		roundID:        "round-missing-terminal-usage",
		GoalRoundState: runtimehost.GoalRoundState{IDForUsage: goalID, Usage: goalsvc.NewRuntimeUsageAccumulator(true), UsageStartedAt: time.Now().Add(-2 * time.Second)},
	}

	runner.finalizeGoalUsage(context.Background(), exec.RoundExecutionResult{}, nil)

	if calls := provider.finalizeCallCount(); calls != 0 {
		t.Fatalf("FinalizeUsageForGoal calls = %d, want 0 without token evidence", calls)
	}
	provider.mu.Lock()
	finalized := provider.report.UsageFinalized
	provider.mu.Unlock()
	if finalized {
		t.Fatal("missing terminal token usage was frozen as authoritative zero")
	}
	usages := base.recordedUsage()
	if len(usages) != 1 || usages[0].RuntimeSeconds <= 0 ||
		usages[0].BudgetTokens() != 0 || usages[0].ActualTokens() != 0 {
		t.Fatalf("persisted usage = %#v, want elapsed-only settlement", usages)
	}
	if runner.Usage.Active() {
		t.Fatal("missing-usage terminal settlement left accumulator active for infinite retry")
	}
}

func TestDMTerminalUsageDurableParentLedgerReplaysWithoutDoubleAttribution(t *testing.T) {
	for _, test := range []struct {
		name              string
		parentCommitError bool
		finalizeError     bool
		wantFinalizeCalls int
	}{
		{
			name:              "parent commit response lost",
			parentCommitError: true,
			wantFinalizeCalls: 1,
		},
		{
			name:              "finalization retry",
			finalizeError:     true,
			wantFinalizeCalls: 2,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			const (
				sessionKey = "agent:nexus:ws:dm:durable-parent-replay"
				goalID     = "goal-durable-parent-replay"
				roundID    = "round-durable-parent-replay"
			)
			provider := newDurableDMParentProvider(sessionKey, goalID)
			if test.parentCommitError {
				provider.parentCommitErrorsRemaining = 1
			}
			if test.finalizeError {
				provider.finalizeFailuresRemaining = 1
			}
			runner := &roundRunner{
				service:        &Service{goals: provider, Host: runtimehost.Host{Runtime: runtimectx.NewManager()}},
				ownerUserID:    "owner-durable-parent-replay",
				sessionKey:     sessionKey,
				roundID:        roundID,
				GoalRoundState: runtimehost.GoalRoundState{IDForUsage: goalID, Usage: goalsvc.NewRuntimeUsageAccumulator(true)},
			}
			snapshot := goalsvc.RuntimeUsageSnapshot{
				Usage: protocol.GoalUsage{
					InputTokens:       12,
					OutputTokens:      3,
					ActualTotalTokens: 15,
					ActualTotalKnown:  true,
				},
				TokenUsageObserved: true,
				Cumulative:         true,
				Terminal:           true,
			}

			if runner.settleTerminalGoalUsageSnapshot(context.Background(), snapshot) {
				t.Fatal("first settlement unexpectedly succeeded despite injected response failure")
			}
			if !runner.settleTerminalGoalUsageSnapshot(context.Background(), snapshot) {
				t.Fatal("terminal replay did not converge")
			}

			if calls := provider.parentCallCount(); calls != 2 {
				t.Fatalf("parent ledger calls = %d, want replayed twice", calls)
			}
			if usage := provider.parentAttributedUsage(); usage.BudgetTokens() != 15 ||
				usage.ActualTokens() != 15 {
				t.Fatalf("durably attributed usage = %#v, want exactly one 15-token terminal", usage)
			}
			if calls := provider.finalizeCallCount(); calls != test.wantFinalizeCalls {
				t.Fatalf("finalization calls = %d, want %d", calls, test.wantFinalizeCalls)
			}
			for _, delta := range provider.finalizedDeltas() {
				if !delta.IsZero() {
					t.Fatalf("finalization delta = %#v, want zero after durable parent attribution", delta)
				}
			}
		})
	}
}

type fakeDMGoalUsageFinalizer struct {
	*fakeGoalContextProvider
	report                 protocol.GoalUsageReport
	finalizeCalls          int
	finalizeDeltas         []protocol.GoalUsage
	claimFailuresRemaining int
	claimAttempts          int
	claimSuccesses         int
}

type durableDMParentProvider struct {
	*fakeDMGoalUsageFinalizer
	parentSnapshots             []protocol.GoalUsageParentSnapshot
	parentLedger                map[string]protocol.GoalUsage
	parentAttributed            protocol.GoalUsage
	parentCommitErrorsRemaining int
	finalizeFailuresRemaining   int
}

func newDurableDMParentProvider(sessionKey string, goalID string) *durableDMParentProvider {
	return &durableDMParentProvider{
		fakeDMGoalUsageFinalizer: &fakeDMGoalUsageFinalizer{
			fakeGoalContextProvider: &fakeGoalContextProvider{},
			report: protocol.GoalUsageReport{
				GoalID:     goalID,
				SessionKey: sessionKey,
				Status:     protocol.GoalStatusComplete,
			},
		},
		parentLedger: make(map[string]protocol.GoalUsage),
	}
}

func (p *durableDMParentProvider) RecordUsageParentSnapshot(
	_ context.Context,
	snapshot protocol.GoalUsageParentSnapshot,
) (protocol.GoalUsageParentResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.parentSnapshots = append(p.parentSnapshots, snapshot)
	key := snapshot.OwnerUserID + "\x00" +
		snapshot.GoalSessionKey + "\x00" +
		snapshot.ScopeRoundID + "\x00" +
		snapshot.SourceRoundID
	if _, exists := p.parentLedger[key]; !exists {
		p.parentLedger[key] = snapshot.Usage
		p.parentAttributed = p.parentAttributed.Add(snapshot.Usage)
		p.report.Usage = p.report.Usage.Add(snapshot.Usage)
	}
	if p.parentCommitErrorsRemaining > 0 {
		p.parentCommitErrorsRemaining--
		return protocol.GoalUsageParentResult{}, errors.New("injected committed DM parent response failure")
	}
	return protocol.GoalUsageParentResult{
		AttributedUsage: p.parentLedger[key],
	}, nil
}

func (p *durableDMParentProvider) FinalizeUsageForGoal(
	_ context.Context,
	goalID string,
	delta protocol.GoalUsage,
	_ string,
) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.finalizeCalls++
	p.finalizeDeltas = append(p.finalizeDeltas, delta)
	if p.finalizeFailuresRemaining > 0 {
		p.finalizeFailuresRemaining--
		return nil, errors.New("injected DM durable finalization failure")
	}
	p.report.Usage = p.report.Usage.Add(delta)
	p.report.UsageFinalized = true
	return &protocol.Goal{
		ID:             goalID,
		SessionKey:     p.report.SessionKey,
		Status:         protocol.GoalStatusComplete,
		Usage:          p.report.Usage,
		UsageFinalized: true,
	}, nil
}

func (p *durableDMParentProvider) parentCallCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.parentSnapshots)
}

func (p *durableDMParentProvider) parentAttributedUsage() protocol.GoalUsage {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.parentAttributed
}

type retryingDMGoalUsageProvider struct {
	*fakeDMGoalUsageFinalizer
	sourceFailuresRemaining   int
	sourceAttempts            int
	finalizeFailuresRemaining int
}

func newRetryingDMGoalUsageProvider(
	sessionKey string,
	goalID string,
	sourceFailures int,
) *retryingDMGoalUsageProvider {
	return &retryingDMGoalUsageProvider{
		fakeDMGoalUsageFinalizer: &fakeDMGoalUsageFinalizer{
			fakeGoalContextProvider: &fakeGoalContextProvider{},
			report: protocol.GoalUsageReport{
				GoalID:     goalID,
				SessionKey: sessionKey,
				Status:     protocol.GoalStatusComplete,
			},
		},
		sourceFailuresRemaining: sourceFailures,
	}
}

func (p *retryingDMGoalUsageProvider) RecordUsageSourceSnapshot(
	_ context.Context,
	_ protocol.GoalUsageSourceSnapshot,
) (protocol.GoalUsageSourceResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sourceAttempts++
	if p.sourceFailuresRemaining > 0 {
		p.sourceFailuresRemaining--
		return protocol.GoalUsageSourceResult{}, errors.New("injected DM source snapshot failure")
	}
	return protocol.GoalUsageSourceResult{}, nil
}

func (p *retryingDMGoalUsageProvider) FinalizeUsageForGoal(
	_ context.Context,
	goalID string,
	delta protocol.GoalUsage,
	_ string,
) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.finalizeCalls++
	p.finalizeDeltas = append(p.finalizeDeltas, delta)
	if p.finalizeFailuresRemaining > 0 {
		p.finalizeFailuresRemaining--
		return nil, errors.New("injected DM usage finalization failure")
	}
	p.report.Usage = p.report.Usage.Add(delta)
	p.report.UsageFinalized = true
	return &protocol.Goal{
		ID:             goalID,
		SessionKey:     p.report.SessionKey,
		Status:         protocol.GoalStatusComplete,
		Usage:          p.report.Usage,
		UsageFinalized: true,
	}, nil
}

func waitForDMGoalUsageRetry(t *testing.T, ready func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if ready() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for DM Goal usage background retry")
}

func dmGoalUsageRetryStopped(runner *roundRunner) bool {
	if runner == nil {
		return true
	}
	runner.Mu.Lock()
	defer runner.Mu.Unlock()
	return !runner.UsageRetrying
}

func (p *fakeDMGoalUsageFinalizer) UsageByGoalID(
	_ context.Context,
	_ string,
) (*protocol.GoalUsageReport, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	report := p.report
	return &report, nil
}

func (p *fakeDMGoalUsageFinalizer) FinalizeUsageForGoal(
	_ context.Context,
	goalID string,
	delta protocol.GoalUsage,
	_ string,
) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.finalizeCalls++
	p.finalizeDeltas = append(p.finalizeDeltas, delta)
	p.report.Usage = p.report.Usage.Add(delta)
	p.report.UsageFinalized = true
	return &protocol.Goal{
		ID:             goalID,
		SessionKey:     p.report.SessionKey,
		Status:         protocol.GoalStatusComplete,
		Usage:          p.report.Usage,
		UsageFinalized: true,
	}, nil
}

func (p *fakeDMGoalUsageFinalizer) ClaimUsageSourceRound(
	_ context.Context,
	_ protocol.GoalUsageSourceRoundClaim,
) (protocol.GoalUsageSourceResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.claimAttempts++
	if p.claimFailuresRemaining > 0 {
		p.claimFailuresRemaining--
		return protocol.GoalUsageSourceResult{}, errors.New("injected DM usage claim failure")
	}
	p.claimSuccesses++
	return protocol.GoalUsageSourceResult{}, nil
}

func (p *fakeDMGoalUsageFinalizer) finalizeCallCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.finalizeCalls
}

func (p *fakeDMGoalUsageFinalizer) finalizedDeltas() []protocol.GoalUsage {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]protocol.GoalUsage(nil), p.finalizeDeltas...)
}

func (p *fakeDMGoalUsageFinalizer) claimAttemptCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.claimAttempts
}
