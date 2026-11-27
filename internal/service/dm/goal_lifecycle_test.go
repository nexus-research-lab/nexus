// goal_lifecycle_test.go 覆盖 DM Goal 续跑与终态结算测试。
package dm

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
	_ "modernc.org/sqlite"
)

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

type fakeDMGoalUsageFinalizer struct {
	*fakeGoalContextProvider
	report                 protocol.GoalUsageReport
	finalizeCalls          int
	finalizeDeltas         []protocol.GoalUsage
	claimFailuresRemaining int
	claimAttempts          int
	claimSuccesses         int
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
