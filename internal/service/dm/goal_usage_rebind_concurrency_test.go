package dm

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	exec "github.com/nexus-research-lab/nexus/internal/runtime/exec"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

type blockingDMGoalUsageProvider struct {
	*fakeGoalContextProvider
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

type failOnceDMGoalUsageProvider struct {
	*fakeGoalContextProvider
	mu       sync.Mutex
	failNext bool
}

type failNDMGoalUsageProvider struct {
	*fakeGoalContextProvider
	mu                sync.Mutex
	failuresRemaining int
}

func (p *failNDMGoalUsageProvider) RecordUsageForGoal(
	ctx context.Context,
	goalID string,
	usage protocol.GoalUsage,
	roundID string,
) (*protocol.Goal, error) {
	p.mu.Lock()
	if p.failuresRemaining > 0 {
		p.failuresRemaining--
		p.mu.Unlock()
		return nil, errors.New("transient usage write failure")
	}
	p.mu.Unlock()
	return p.fakeGoalContextProvider.RecordUsageForGoal(ctx, goalID, usage, roundID)
}

func (p *failOnceDMGoalUsageProvider) RecordUsageForGoal(
	ctx context.Context,
	goalID string,
	usage protocol.GoalUsage,
	roundID string,
) (*protocol.Goal, error) {
	p.mu.Lock()
	fail := p.failNext
	p.failNext = false
	p.mu.Unlock()
	if fail {
		return nil, errors.New("transient usage write failure")
	}
	return p.fakeGoalContextProvider.RecordUsageForGoal(ctx, goalID, usage, roundID)
}

func (p *blockingDMGoalUsageProvider) RecordUsageForGoal(
	ctx context.Context,
	goalID string,
	usage protocol.GoalUsage,
	roundID string,
) (*protocol.Goal, error) {
	p.once.Do(func() { close(p.entered) })
	<-p.release
	return p.fakeGoalContextProvider.RecordUsageForGoal(ctx, goalID, usage, roundID)
}

func TestRoundRunnerSerializesUsageSettlementWithExternalGoalRebind(t *testing.T) {
	provider := &blockingDMGoalUsageProvider{
		fakeGoalContextProvider: &fakeGoalContextProvider{},
		entered:                 make(chan struct{}),
		release:                 make(chan struct{}),
	}
	runner := &roundRunner{
		service:        &Service{goals: provider},
		sessionKey:     "agent:nexus:ws:dm:rebind",
		roundID:        "round-old",
		goalIDForUsage: "goal-old",
		goalUsage:      goalsvc.NewRuntimeUsageAccumulator(true),
	}

	recorded := make(chan struct{})
	go func() {
		runner.recordGoalUsageSnapshot(context.Background(), goalsvc.RuntimeUsageSnapshot{
			Usage:              protocol.GoalUsage{InputTokens: 10, OutputTokens: 2, ActualTotalTokens: 12},
			TokenUsageObserved: true,
			Cumulative:         true,
			SettlementBoundary: true,
		})
		close(recorded)
	}()
	<-provider.entered

	activationStarted := make(chan struct{})
	activationDone := make(chan struct{})
	go func() {
		close(activationStarted)
		_ = runner.activateGoalUsage(context.Background(), "goal-new")
		close(activationDone)
	}()
	<-activationStarted
	select {
	case <-activationDone:
		t.Fatal("external Goal rebind completed before old usage settlement")
	case <-time.After(25 * time.Millisecond):
	}

	close(provider.release)
	select {
	case <-recorded:
	case <-time.After(time.Second):
		t.Fatal("old Goal usage settlement did not finish")
	}
	select {
	case <-activationDone:
	case <-time.After(time.Second):
		t.Fatal("external Goal rebind did not resume")
	}

	provider.mu.Lock()
	gotIDs := append([]string(nil), provider.usageGoalIDs...)
	provider.mu.Unlock()
	if len(gotIDs) != 1 || gotIDs[0] != "goal-old" {
		t.Fatalf("usage Goal IDs = %#v, want old delta fixed to goal-old", gotIDs)
	}
	runner.goalUsageMu.Lock()
	gotBinding := runner.goalIDForUsage
	runner.goalUsageMu.Unlock()
	if gotBinding != "goal-new" {
		t.Fatalf("Goal binding = %q, want goal-new after settlement", gotBinding)
	}
}

func TestRoundRunnerRetriesUncommittedUsageAtTerminal(t *testing.T) {
	base := &fakeGoalContextProvider{}
	provider := &failOnceDMGoalUsageProvider{
		fakeGoalContextProvider: base,
		failNext:                true,
	}
	runner := &roundRunner{
		service:        &Service{goals: provider},
		sessionKey:     "agent:nexus:ws:dm:retry",
		roundID:        "round-retry",
		goalIDForUsage: "goal-retry",
		goalUsage:      goalsvc.NewRuntimeUsageAccumulator(true),
	}

	runner.recordGoalUsageSnapshot(context.Background(), goalsvc.RuntimeUsageSnapshot{
		TurnID:         "turn-a",
		ElapsedSeconds: 1,
		Usage: protocol.GoalUsage{
			InputTokens:       90,
			OutputTokens:      10,
			ActualTotalTokens: 100,
			ActualTotalKnown:  true,
		},
	})
	runner.recordGoalUsageSnapshot(context.Background(), goalsvc.RuntimeUsageSnapshot{
		Usage: protocol.GoalUsage{
			InputTokens:       140,
			OutputTokens:      10,
			ActualTotalTokens: 150,
			ActualTotalKnown:  true,
		},
		Cumulative: true,
		Terminal:   true,
	})

	usages := base.recordedUsage()
	if len(usages) != 1 || usages[0].BudgetTokens() != 150 || usages[0].ActualTokens() != 150 {
		t.Fatalf("persisted usage = %#v, want one complete terminal retry of 150", usages)
	}
}

func TestRoundRunnerRetainsTerminalDeltaAfterRetryWindow(t *testing.T) {
	base := &fakeGoalContextProvider{}
	provider := &failNDMGoalUsageProvider{
		fakeGoalContextProvider: base,
		failuresRemaining:       goalUsagePersistAttempts,
	}
	runner := &roundRunner{
		service:        &Service{goals: provider},
		sessionKey:     "agent:nexus:ws:dm:retry-window",
		roundID:        "round-retry-window",
		goalIDForUsage: "goal-retry-window",
		goalUsage:      goalsvc.NewRuntimeUsageAccumulator(true),
	}
	snapshot := goalsvc.RuntimeUsageSnapshot{
		Usage: protocol.GoalUsage{
			InputTokens:       140,
			OutputTokens:      10,
			ActualTotalTokens: 150,
			ActualTotalKnown:  true,
		},
		Cumulative: true,
		Terminal:   true,
	}

	runner.finalizeGoalUsage(context.Background(), exec.RoundExecutionResult{
		Usage: sdkprotocol.TokenUsage{
			InputTokens:  140,
			OutputTokens: 10,
			TotalTokens:  150,
		},
	}, nil)
	if !runner.goalUsage.Active() {
		t.Fatal("terminal persistence failure closed the accumulator")
	}
	if !runner.finalizeCompletedGoalUsageAfterSubagents(context.Background()) {
		t.Fatal("retained terminal delta did not settle on the later retry")
	}
	if runner.goalUsage.Active() {
		t.Fatal("settled terminal accumulator remains active")
	}
	usages := base.recordedUsage()
	if len(usages) != 1 || usages[0].BudgetTokens() != snapshot.Usage.BudgetTokens() ||
		usages[0].ActualTokens() != snapshot.Usage.ActualTokens() {
		t.Fatalf("persisted usage = %#v, want one retained terminal delta %#v", usages, snapshot.Usage)
	}
}

func TestRoundRunnerNewerTerminalSnapshotPreventsOlderSettlementFromClosing(t *testing.T) {
	base := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: base},
		sessionKey:     "agent:nexus:ws:dm:terminal-version-handoff",
		roundID:        "round-terminal-version-handoff",
		goalIDForUsage: "goal-terminal-version-handoff",
		goalUsage:      goalsvc.NewRuntimeUsageAccumulator(true),
	}
	first := goalsvc.RuntimeUsageSnapshot{
		Usage: protocol.GoalUsage{
			InputTokens:       90,
			OutputTokens:      10,
			ActualTotalTokens: 100,
			ActualTotalKnown:  true,
		},
		Cumulative: true,
		Terminal:   true,
	}
	firstVersion := runner.rememberTerminalGoalUsageSnapshot(first)
	if !runner.settleTerminalGoalUsageSnapshot(context.Background(), first) {
		t.Fatal("first terminal snapshot did not settle")
	}

	second := goalsvc.RuntimeUsageSnapshot{
		Usage: protocol.GoalUsage{
			InputTokens:       135,
			OutputTokens:      15,
			ActualTotalTokens: 150,
			ActualTotalKnown:  true,
		},
		Cumulative: true,
		Terminal:   true,
	}
	secondVersion := runner.rememberTerminalGoalUsageSnapshot(second)
	if runner.clearTerminalGoalUsageSnapshot(firstVersion) {
		t.Fatal("older settlement cleared a newer terminal snapshot")
	}
	if runner.closeGoalUsageIfNoTerminalSnapshotPending() {
		t.Fatal("older settlement closed accounting while a newer terminal snapshot was pending")
	}
	if !runner.goalUsage.Active() {
		t.Fatal("newer terminal handoff did not keep the accumulator active")
	}

	if !runner.settleTerminalGoalUsageSnapshot(context.Background(), second) {
		t.Fatal("newer terminal snapshot did not settle")
	}
	if !runner.clearTerminalGoalUsageSnapshot(secondVersion) {
		t.Fatal("newer terminal snapshot could not clear its own pending version")
	}
	if !runner.closeGoalUsageIfNoTerminalSnapshotPending() {
		t.Fatal("accounting did not close after the newest terminal snapshot settled")
	}
	if runner.goalUsage.Active() {
		t.Fatal("accumulator remained active after the newest terminal settlement")
	}

	usages := base.recordedUsage()
	if len(usages) != 2 {
		t.Fatalf("terminal handoff usages = %#v, want 100 then 50", usages)
	}
	total := usages[0].Add(usages[1])
	if total.BudgetTokens() != 150 || total.ActualTokens() != 150 {
		t.Fatalf("terminal handoff total = %#v, want latest cumulative 150", total)
	}
}
