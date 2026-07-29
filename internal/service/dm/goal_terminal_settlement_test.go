package dm

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	exec "github.com/nexus-research-lab/nexus/internal/runtime/exec"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

func TestDMParentTerminalUsageBackgroundRetryRecoversWithoutChildOrNewMessage(t *testing.T) {
	const (
		sessionKey = "agent:nexus:ws:dm:parent-terminal-background-retry"
		goalID     = "goal-parent-terminal-background-retry"
		roundID    = "round-parent-terminal-background-retry"
	)
	provider := newRetryingDMGoalUsageProvider(sessionKey, goalID, 0)
	provider.claimFailuresRemaining = goalUsagePersistAttempts * 2
	provider.finalizeFailuresRemaining = goalUsagePersistAttempts * 2
	var dispatches atomic.Int64
	runner := &roundRunner{
		service:                   &Service{goals: provider},
		sessionKey:                sessionKey,
		roundID:                   roundID,
		ownerUserID:               "owner-parent-terminal-background-retry",
		goalIDForUsage:            goalID,
		childGoalIDForUsage:       goalID,
		goalUsage:                 goalsvc.NewRuntimeUsageAccumulator(true),
		runtimeKind:               "nxs",
		subagentUsageClaimPending: true,
		postRoundDispatchHook:     func() { dispatches.Add(1) },
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
	if attempts := provider.claimAttemptCount(); attempts != goalUsagePersistAttempts {
		t.Fatalf("claim attempts before parent terminal = %d, want %d", attempts, goalUsagePersistAttempts)
	}
	runner.startGoalUsageRetryWorker()
	waitForDMGoalUsageRetry(t, func() bool {
		runner.goalUsageMu.Lock()
		defer runner.goalUsageMu.Unlock()
		return !runner.goalUsageRetryRunning
	})
	if attempts := provider.claimAttemptCount(); attempts != goalUsagePersistAttempts {
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
			provider.finalizeCallCount() == goalUsagePersistAttempts*2+1 &&
			dmGoalUsageRetryStopped(runner)
	})
	if attempts := provider.claimAttemptCount(); attempts != goalUsagePersistAttempts*2+1 {
		t.Fatalf("claim attempts = %d, want two failed windows plus one background success", attempts)
	}
	if calls := provider.finalizeCallCount(); calls != goalUsagePersistAttempts*2+1 {
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

func TestDMParentTerminalUsageBackgroundRetryNeverDispatchesAbnormalRound(t *testing.T) {
	for _, terminal := range []string{
		subagentParentTerminalFailed,
		subagentParentTerminalInterrupted,
	} {
		t.Run(terminal, func(t *testing.T) {
			const (
				sessionKey = "agent:nexus:ws:dm:abnormal-parent-terminal-retry"
				goalID     = "goal-abnormal-parent-terminal-retry"
				roundID    = "round-abnormal-parent-terminal-retry"
			)
			provider := newRetryingDMGoalUsageProvider(sessionKey, goalID, 0)
			provider.finalizeFailuresRemaining = goalUsagePersistAttempts * 2
			var dispatches atomic.Int64
			runner := &roundRunner{
				service:               &Service{goals: provider},
				sessionKey:            sessionKey,
				roundID:               roundID,
				goalIDForUsage:        goalID,
				childGoalIDForUsage:   goalID,
				goalUsage:             goalsvc.NewRuntimeUsageAccumulator(true),
				postRoundDispatchHook: func() { dispatches.Add(1) },
			}
			accelerateDMGoalUsageRetry(runner)
			runner.finalizeGoalUsage(context.Background(), exec.RoundExecutionResult{
				Usage: sdkprotocol.TokenUsage{InputTokens: 9, OutputTokens: 1, TotalTokens: 10},
			}, nil)
			runner.markSubagentParentTerminal(terminal)
			if runner.completeSubagentJoinAfterParentTerminal() {
				t.Fatal("abnormal terminal usage unexpectedly settled inside the exhausted synchronous window")
			}

			waitForDMGoalUsageRetry(t, func() bool {
				return provider.finalizeCallCount() == goalUsagePersistAttempts*2+1
			})
			if got := dispatches.Load(); got != 0 {
				t.Fatalf("abnormal parent post-round dispatches = %d, want 0", got)
			}
			runner.goalUsageMu.Lock()
			dispatched := runner.subagentPostRoundDispatched
			runner.goalUsageMu.Unlock()
			if dispatched {
				t.Fatal("abnormal parent claimed normal post-round dispatch")
			}
		})
	}
}

func TestDMSubagentUsageBackgroundRetryRecoversAndDispatchesNormalParentOnce(t *testing.T) {
	const (
		sessionKey = "agent:nexus:ws:dm:source-background-retry"
		goalID     = "goal-source-background-retry"
		roundID    = "round-source-background-retry"
	)
	provider := newRetryingDMGoalUsageProvider(sessionKey, goalID, 6)
	var dispatches atomic.Int64
	runner := &roundRunner{
		service:                &Service{goals: provider},
		sessionKey:             sessionKey,
		roundID:                roundID,
		ownerUserID:            "owner-source-background-retry",
		goalIDForUsage:         goalID,
		childGoalIDForUsage:    goalID,
		goalUsage:              goalsvc.NewRuntimeUsageAccumulator(true),
		runtimeKind:            "nxs",
		goalTokenUsageObserved: true,
		postRoundDispatchHook: func() {
			dispatches.Add(1)
		},
	}
	accelerateDMGoalUsageRetry(runner)
	runner.markSubagentParentTerminal(subagentParentTerminalNormal)
	runner.rememberSubagentTaskMessage(dmSubagentTaskMessage("task_started", "running"))
	terminalMessage := dmSubagentTaskMessage("task_notification", "completed")
	terminalMessage["metadata"].(map[string]any)["usage"] = map[string]any{"total_tokens": int64(100)}

	if settled := runner.recordSubagentGoalUsage(context.Background(), terminalMessage); len(settled) != 0 {
		t.Fatalf("synchronous failed checkpoints settled = %#v, want none", settled)
	}
	runner.rememberSubagentTaskMessage(terminalMessage)
	if got := dispatches.Load(); got != 0 {
		t.Fatalf("post-round dispatch before retry recovery = %d, want 0", got)
	}

	waitForDMGoalUsageRetry(t, func() bool {
		return dispatches.Load() == 1 &&
			provider.finalizeCallCount() == 1 &&
			dmGoalUsageRetryStopped(runner)
	})
	if attempts := provider.sourceAttemptCount(); attempts != 7 {
		t.Fatalf("source attempts = %d, want 6 failures plus one background success", attempts)
	}
	if runner.hasRunningSubagentTask() {
		t.Fatal("background retry success left the child join barrier pending")
	}
	if got := dispatches.Load(); got != 1 {
		t.Fatalf("post-round dispatches after retry settled = %d, want exactly 1", got)
	}
	if calls := provider.finalizeCallCount(); calls != 1 {
		t.Fatalf("finalization calls after retry settled = %d, want exactly 1", calls)
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
		service:                &Service{goals: provider},
		sessionKey:             sessionKey,
		roundID:                roundID,
		ownerUserID:            "owner-finalize-background-retry",
		goalIDForUsage:         goalID,
		childGoalIDForUsage:    goalID,
		goalUsage:              goalsvc.NewRuntimeUsageAccumulator(true),
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

func TestDMSubagentUsageBackgroundRetryDoesNotDispatchFailedOrInterruptedParent(t *testing.T) {
	for _, terminal := range []string{
		subagentParentTerminalFailed,
		subagentParentTerminalInterrupted,
	} {
		t.Run(terminal, func(t *testing.T) {
			const (
				sessionKey = "agent:nexus:ws:dm:abnormal-source-background-retry"
				goalID     = "goal-abnormal-source-background-retry"
				roundID    = "round-abnormal-source-background-retry"
			)
			provider := newRetryingDMGoalUsageProvider(sessionKey, goalID, 6)
			var dispatches atomic.Int64
			runner := &roundRunner{
				service:                &Service{goals: provider},
				sessionKey:             sessionKey,
				roundID:                roundID,
				ownerUserID:            "owner-abnormal-source-background-retry",
				goalIDForUsage:         goalID,
				childGoalIDForUsage:    goalID,
				goalUsage:              goalsvc.NewRuntimeUsageAccumulator(true),
				runtimeKind:            "nxs",
				goalTokenUsageObserved: true,
				postRoundDispatchHook: func() {
					dispatches.Add(1)
				},
			}
			accelerateDMGoalUsageRetry(runner)
			runner.markSubagentParentTerminal(terminal)
			runner.rememberSubagentTaskMessage(dmSubagentTaskMessage("task_started", "running"))
			terminalMessage := dmSubagentTaskMessage("task_notification", "completed")
			terminalMessage["metadata"].(map[string]any)["usage"] = map[string]any{"total_tokens": int64(100)}

			runner.recordSubagentGoalUsage(context.Background(), terminalMessage)
			runner.rememberSubagentTaskMessage(terminalMessage)

			waitForDMGoalUsageRetry(t, func() bool {
				return provider.finalizeCallCount() == 1
			})
			if got := dispatches.Load(); got != 0 {
				t.Fatalf("abnormal parent post-round dispatches = %d, want 0", got)
			}
			if runner.subagentPostRoundDispatched {
				t.Fatal("abnormal parent claimed normal post-round dispatch")
			}
		})
	}
}

func TestCompletedDMGoalWaitsForFailedUsageClaimThenFinalizesOnce(t *testing.T) {
	const (
		sessionKey = "agent:nexus:ws:dm:claim-retry"
		goalID     = "goal-claim-retry"
		roundID    = "round-claim-retry"
	)
	provider := &fakeDMGoalUsageFinalizer{
		fakeGoalContextProvider: &fakeGoalContextProvider{},
		report: protocol.GoalUsageReport{
			GoalID:     goalID,
			SessionKey: sessionKey,
			Status:     protocol.GoalStatusComplete,
		},
		claimFailuresRemaining: goalUsagePersistAttempts * 2,
	}
	runner := &roundRunner{
		service:                &Service{goals: provider},
		sessionKey:             sessionKey,
		roundID:                roundID,
		ownerUserID:            "owner-claim-retry",
		goalIDForUsage:         goalID,
		childGoalIDForUsage:    goalID,
		goalUsage:              goalsvc.NewRuntimeUsageAccumulator(true),
		runtimeKind:            "nxs",
		goalTokenUsageObserved: true,
	}
	accelerateDMGoalUsageRetry(runner)

	runner.claimSubagentGoalUsageRound(context.Background(), goalID)
	if !runner.subagentGoalUsageClaimPending() {
		t.Fatal("failed model-create claim did not remain pending")
	}
	if runner.finalizeCompletedGoalUsageAfterSubagents(context.Background()) {
		t.Fatal("completed Goal finalized while the pending claim kept failing")
	}
	if calls := provider.finalizeCallCount(); calls != 0 {
		t.Fatalf("FinalizeUsageForGoal calls before claim success = %d, want 0", calls)
	}
	if successes := provider.claimSuccessCount(); successes != 0 {
		t.Fatalf("successful claims before recovery = %d, want 0", successes)
	}

	if !runner.finalizeCompletedGoalUsageAfterSubagents(context.Background()) {
		t.Fatal("completed Goal did not finalize after claim persistence recovered")
	}
	if runner.subagentGoalUsageClaimPending() {
		t.Fatal("successful retry left the usage claim pending")
	}
	attempts := provider.claimAttemptCount()
	if attempts != goalUsagePersistAttempts*2+1 {
		t.Fatalf("claim attempts = %d, want %d failures plus one success", attempts, goalUsagePersistAttempts*2)
	}
	if successes := provider.claimSuccessCount(); successes != 1 {
		t.Fatalf("successful claims after recovery = %d, want exactly 1", successes)
	}
	if calls := provider.finalizeCallCount(); calls != 1 {
		t.Fatalf("FinalizeUsageForGoal calls after recovery = %d, want exactly 1", calls)
	}

	if !runner.finalizeCompletedGoalUsageAfterSubagents(context.Background()) {
		t.Fatal("repeated claim/finalization join should be an idempotent success")
	}
	if got := provider.claimAttemptCount(); got != attempts {
		t.Fatalf("repeated join retried an already successful claim: attempts %d -> %d", attempts, got)
	}
	if successes := provider.claimSuccessCount(); successes != 1 {
		t.Fatalf("successful claims after repeated join = %d, want exactly 1", successes)
	}
	if calls := provider.finalizeCallCount(); calls != 1 {
		t.Fatalf("FinalizeUsageForGoal calls after repeated join = %d, want exactly 1", calls)
	}
}

func TestCompletedDMGoalFinalizesOnlyAfterRunningChildDrains(t *testing.T) {
	const (
		sessionKey = "agent:nexus:ws:dm:terminal-child"
		goalID     = "goal-terminal-child"
		roundID    = "round-terminal-child"
	)
	base := &fakeGoalContextProvider{
		usageGoal: &protocol.Goal{
			ID:         goalID,
			SessionKey: sessionKey,
			Status:     protocol.GoalStatusComplete,
		},
	}
	provider := &fakeDMGoalUsageFinalizer{
		fakeGoalContextProvider: base,
		report: protocol.GoalUsageReport{
			GoalID:     goalID,
			SessionKey: sessionKey,
			Status:     protocol.GoalStatusComplete,
		},
	}
	runner := &roundRunner{
		service:             &Service{goals: provider},
		sessionKey:          sessionKey,
		roundID:             roundID,
		goalIDForUsage:      goalID,
		childGoalIDForUsage: goalID,
		goalUsage:           goalsvc.NewRuntimeUsageAccumulator(true),
		subagentTasks:       map[string]struct{}{"child-task": {}},
		runtimeKind:         "nxs",
	}

	runner.finalizeGoalUsage(context.Background(), exec.RoundExecutionResult{
		Usage: sdkprotocol.TokenUsage{
			InputTokens:  9,
			OutputTokens: 3,
			TotalTokens:  12,
		},
	}, nil)

	usages := base.recordedUsage()
	if len(usages) != 1 || usages[0].BudgetTokens() != 12 || usages[0].ActualTokens() != 12 {
		t.Fatalf("parent terminal usage = %#v, want one persisted 12-token delta", usages)
	}
	if calls := provider.finalizeCallCount(); calls != 0 {
		t.Fatalf("FinalizeUsageForGoal calls with running child = %d, want 0", calls)
	}
	if runner.finalizeCompletedGoalUsageAfterSubagents(context.Background()) {
		t.Fatal("child join finalized while child task was still running")
	}

	childTerminal := protocol.Message{"metadata": map[string]any{
		"task_id":   "child-task",
		"task_type": "local_agent",
		"subtype":   "task_notification",
		"status":    "completed",
		"usage":     map[string]any{"total_tokens": int64(7)},
	}}
	for _, settlement := range runner.recordSubagentGoalUsage(context.Background(), childTerminal) {
		runner.clearSubagentUsageObservationPending(settlement.taskID, settlement.observation)
	}
	runner.rememberSubagentTaskMessage(childTerminal)
	if runner.hasRunningSubagentTask() {
		t.Fatal("child task map still contains completed task")
	}
	if !runner.finalizeCompletedGoalUsageAfterSubagents(context.Background()) {
		t.Fatal("child join did not finalize completed Goal usage")
	}
	if calls := provider.finalizeCallCount(); calls != 1 {
		t.Fatalf("FinalizeUsageForGoal calls after child drain = %d, want 1", calls)
	}
	if deltas := provider.finalizedDeltas(); len(deltas) != 1 || !isZeroGoalUsage(deltas[0]) {
		t.Fatalf("finalization deltas = %#v, want one zero delta after parent usage was persisted", deltas)
	}

	if !runner.finalizeCompletedGoalUsageAfterSubagents(context.Background()) {
		t.Fatal("repeated finalized join should be an idempotent success")
	}
	if calls := provider.finalizeCallCount(); calls != 1 {
		t.Fatalf("FinalizeUsageForGoal calls after repeated join = %d, want exactly 1", calls)
	}
}

func TestDMUnavailableChildEvidenceStopsWorkerAndReleasesPostRoundOnce(t *testing.T) {
	const (
		sessionKey = "agent:nexus:ws:dm:child-unavailable"
		goalID     = "goal-child-unavailable"
		roundID    = "round-child-unavailable"
	)
	provider := newEvidenceAwareDMGoalProvider(sessionKey, goalID)
	var dispatches atomic.Int64
	runner := &roundRunner{
		service:                &Service{goals: provider},
		ownerUserID:            "owner-child-unavailable",
		sessionKey:             sessionKey,
		roundID:                roundID,
		goalIDForUsage:         goalID,
		childGoalIDForUsage:    goalID,
		goalUsage:              goalsvc.NewRuntimeUsageAccumulator(true),
		goalTokenUsageObserved: true,
		runtimeKind:            "nxs",
		postRoundDispatchHook:  func() { dispatches.Add(1) },
	}
	taskMessage := func(taskID string, subtype string, status string, total any) protocol.Message {
		metadata := map[string]any{
			"task_id":   taskID,
			"task_type": "local_agent",
			"subtype":   subtype,
			"status":    status,
		}
		if total != nil {
			metadata["usage"] = map[string]any{"total_tokens": total}
		}
		return protocol.Message{"metadata": metadata}
	}
	record := func(message protocol.Message) {
		t.Helper()
		for _, settlement := range runner.recordSubagentGoalUsage(context.Background(), message) {
			runner.clearSubagentUsageObservationPending(
				settlement.taskID,
				settlement.observation,
			)
		}
		runner.rememberSubagentTaskMessage(message)
	}

	record(taskMessage("child-observed", "task_started", "running", nil))
	record(taskMessage("child-observed", "task_notification", "completed", int64(100)))
	record(taskMessage("child-missing", "task_started", "running", nil))
	record(taskMessage("child-missing", "task_progress", "running", int64(50)))
	record(taskMessage("child-missing", "task_notification", "completed", int64(0)))
	if runner.hasRunningSubagentTask() {
		t.Fatal("terminal child evidence left a live in-memory join barrier")
	}

	runner.markSubagentParentTerminal(subagentParentTerminalNormal)
	runner.startGoalUsageRetryWorker()
	waitForDMGoalUsageRetry(t, func() bool {
		runner.goalUsageMu.Lock()
		running := runner.goalUsageRetryRunning
		runner.goalUsageMu.Unlock()
		return dispatches.Load() == 1 && !running
	})
	if calls := provider.finalizeCallCount(); calls != 1 {
		t.Fatalf("unavailable finalization calls = %d, want one non-retryable attempt", calls)
	}
	provider.mu.Lock()
	finalized := provider.report.UsageFinalized
	provider.mu.Unlock()
	if finalized {
		t.Fatal("multi-child Goal finalized despite one terminal child missing provider usage")
	}
	if got := provider.childEvidence("child-observed"); !got.terminal || !got.tokenObserved {
		t.Fatalf("observed child evidence = %#v, want authoritative terminal", got)
	}
	if got := provider.childEvidence("child-missing"); !got.terminal || got.tokenObserved {
		t.Fatalf("missing child evidence = %#v, want terminal unavailable", got)
	}

	// A stray worker start after post-round release must not retry the same
	// durable unavailable conclusion or dispatch downstream work twice.
	runner.startGoalUsageRetryWorker()
	waitForDMGoalUsageRetry(t, func() bool {
		runner.goalUsageMu.Lock()
		defer runner.goalUsageMu.Unlock()
		return !runner.goalUsageRetryRunning
	})
	if calls := provider.finalizeCallCount(); calls != 1 {
		t.Fatalf("stray worker retried unavailable finalization: %d calls", calls)
	}
	if got := dispatches.Load(); got != 1 {
		t.Fatalf("post-round dispatches = %d, want exactly one", got)
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
		service:          &Service{goals: provider},
		sessionKey:       sessionKey,
		roundID:          "round-missing-terminal-usage",
		goalIDForUsage:   goalID,
		goalUsage:        goalsvc.NewRuntimeUsageAccumulator(true),
		goalUsageStarted: time.Now().Add(-2 * time.Second),
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
	if runner.goalUsage.Active() {
		t.Fatal("missing-usage terminal settlement left accumulator active for infinite retry")
	}
}

func TestCompletedDMGoalExplicitZeroTokenUsageCanFinalize(t *testing.T) {
	const (
		sessionKey = "agent:nexus:ws:dm:explicit-zero-terminal-usage"
		goalID     = "goal-explicit-zero-terminal-usage"
	)
	provider := &fakeDMGoalUsageFinalizer{
		fakeGoalContextProvider: &fakeGoalContextProvider{},
		report: protocol.GoalUsageReport{
			GoalID:     goalID,
			SessionKey: sessionKey,
			Status:     protocol.GoalStatusComplete,
		},
	}
	runner := &roundRunner{
		service:        &Service{goals: provider},
		sessionKey:     sessionKey,
		roundID:        "round-explicit-zero-terminal-usage",
		goalIDForUsage: goalID,
		goalUsage:      goalsvc.NewRuntimeUsageAccumulator(true),
	}

	runner.finalizeGoalUsage(context.Background(), exec.RoundExecutionResult{
		Usage: sdkprotocol.TokenUsage{Raw: map[string]any{"total_tokens": int64(0)}},
	}, nil)

	if calls := provider.finalizeCallCount(); calls != 1 {
		t.Fatalf("FinalizeUsageForGoal calls = %d, want 1 for explicit provider zero", calls)
	}
	provider.mu.Lock()
	finalized := provider.report.UsageFinalized
	provider.mu.Unlock()
	if !finalized {
		t.Fatal("explicit provider zero did not establish finalization fence")
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
				service:        &Service{goals: provider},
				ownerUserID:    "owner-durable-parent-replay",
				sessionKey:     sessionKey,
				roundID:        roundID,
				goalIDForUsage: goalID,
				goalUsage:      goalsvc.NewRuntimeUsageAccumulator(true),
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
				if !isZeroGoalUsage(delta) {
					t.Fatalf("finalization delta = %#v, want zero after durable parent attribution", delta)
				}
			}
		})
	}
}

func TestDMTerminalUsageAlreadyFinalizedFenceWinsOverUncommittedLocalDelta(t *testing.T) {
	const (
		sessionKey = "agent:nexus:ws:dm:finalize-commit-response-lost"
		goalID     = "goal-finalize-commit-response-lost"
	)
	provider := newCommitThenErrorDMGoalProvider(sessionKey, goalID)
	runner := &roundRunner{
		service:        &Service{goals: provider},
		sessionKey:     sessionKey,
		roundID:        "round-finalize-commit-response-lost",
		goalIDForUsage: goalID,
		goalUsage:      goalsvc.NewRuntimeUsageAccumulator(true),
	}
	snapshot := goalsvc.RuntimeUsageSnapshot{
		Usage: protocol.GoalUsage{
			InputTokens:       9,
			OutputTokens:      1,
			ActualTotalTokens: 10,
			ActualTotalKnown:  true,
		},
		TokenUsageObserved: true,
		Cumulative:         true,
		Terminal:           true,
	}

	if runner.settleTerminalGoalUsageSnapshot(context.Background(), snapshot) {
		t.Fatal("first settlement unexpectedly observed the lost finalization response")
	}
	if !runner.settleTerminalGoalUsageSnapshot(context.Background(), snapshot) {
		t.Fatal("durable finalization fence did not close replayed local delta")
	}
	if calls := provider.finalizeCallCount(); calls != 1 {
		t.Fatalf("finalization calls = %d, want exactly one committed call", calls)
	}
	provider.mu.Lock()
	usage := provider.report.Usage
	provider.mu.Unlock()
	if usage.BudgetTokens() != 10 || usage.ActualTokens() != 10 {
		t.Fatalf("final usage = %#v, want exactly one 10-token settlement", usage)
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

type dmChildEvidenceState struct {
	terminal      bool
	tokenObserved bool
}

type evidenceAwareDMGoalProvider struct {
	*durableDMParentProvider
	sourceSnapshots   []protocol.GoalUsageSourceSnapshot
	childEvidenceByID map[string]dmChildEvidenceState
}

func newEvidenceAwareDMGoalProvider(
	sessionKey string,
	goalID string,
) *evidenceAwareDMGoalProvider {
	return &evidenceAwareDMGoalProvider{
		durableDMParentProvider: newDurableDMParentProvider(sessionKey, goalID),
		childEvidenceByID:       make(map[string]dmChildEvidenceState),
	}
}

func (p *evidenceAwareDMGoalProvider) RecordUsageSourceSnapshot(
	_ context.Context,
	snapshot protocol.GoalUsageSourceSnapshot,
) (protocol.GoalUsageSourceResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.sourceSnapshots = append(p.sourceSnapshots, snapshot)
	if snapshot.EvidenceRequired {
		current := p.childEvidenceByID[snapshot.SourceID]
		current.terminal = current.terminal || snapshot.Terminal
		current.tokenObserved = current.tokenObserved ||
			(snapshot.Terminal && snapshot.TokenUsageObserved)
		p.childEvidenceByID[snapshot.SourceID] = current
	}
	return protocol.GoalUsageSourceResult{}, nil
}

func (p *evidenceAwareDMGoalProvider) FinalizeUsageForGoal(
	_ context.Context,
	goalID string,
	delta protocol.GoalUsage,
	_ string,
) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.finalizeCalls++
	p.finalizeDeltas = append(p.finalizeDeltas, delta)
	for _, evidence := range p.childEvidenceByID {
		if !evidence.terminal {
			return nil, errors.New("injected durable child usage pending")
		}
		if !evidence.tokenObserved {
			return nil, goalsvc.ErrGoalUsageUnavailable
		}
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

func (p *evidenceAwareDMGoalProvider) childEvidence(taskID string) dmChildEvidenceState {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.childEvidenceByID[taskID]
}

type commitThenErrorDMGoalProvider struct {
	*fakeDMGoalUsageFinalizer
	responseLost bool
}

func newCommitThenErrorDMGoalProvider(
	sessionKey string,
	goalID string,
) *commitThenErrorDMGoalProvider {
	return &commitThenErrorDMGoalProvider{
		fakeDMGoalUsageFinalizer: &fakeDMGoalUsageFinalizer{
			fakeGoalContextProvider: &fakeGoalContextProvider{},
			report: protocol.GoalUsageReport{
				GoalID:     goalID,
				SessionKey: sessionKey,
				Status:     protocol.GoalStatusComplete,
			},
		},
	}
}

func (p *commitThenErrorDMGoalProvider) FinalizeUsageForGoal(
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
	if !p.responseLost {
		p.responseLost = true
		return nil, errors.New("injected committed DM finalization response failure")
	}
	return &protocol.Goal{
		ID:             goalID,
		SessionKey:     p.report.SessionKey,
		Status:         protocol.GoalStatusComplete,
		Usage:          p.report.Usage,
		UsageFinalized: true,
	}, nil
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

func (p *retryingDMGoalUsageProvider) sourceAttemptCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.sourceAttempts
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
	runner.goalUsageMu.Lock()
	defer runner.goalUsageMu.Unlock()
	return !runner.goalUsageRetryRunning
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

func (p *fakeDMGoalUsageFinalizer) claimSuccessCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.claimSuccesses
}

func (r *roundRunner) subagentGoalUsageClaimPending() bool {
	r.goalUsageMu.Lock()
	defer r.goalUsageMu.Unlock()
	return r.subagentUsageClaimPending
}
