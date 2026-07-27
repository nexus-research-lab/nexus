package realtime

import (
	"context"
	"errors"
	"sync"
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	exec "github.com/nexus-research-lab/nexus/internal/runtime/exec"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
)

type roomParentLedgerProvider struct {
	*fakeRoomGoalContextProvider

	mu              sync.Mutex
	parentSnapshots []protocol.GoalUsageParentSnapshot
	bindings        []protocol.GoalUsageScopeBinding
	bindErr         error
	parentErr       error
	parentGoal      *protocol.Goal
	usageReport     *protocol.GoalUsageReport
	finalizeErr     error
	finalizeCalls   int
}

func (p *roomParentLedgerProvider) RecordUsageParentSnapshot(
	_ context.Context,
	snapshot protocol.GoalUsageParentSnapshot,
) (protocol.GoalUsageParentResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.parentSnapshots = append(p.parentSnapshots, snapshot)
	if p.parentErr != nil {
		return protocol.GoalUsageParentResult{}, p.parentErr
	}
	return protocol.GoalUsageParentResult{
		AttributedUsage: snapshot.Usage,
		Goal:            cloneRoomGoal(p.parentGoal),
	}, nil
}

func (p *roomParentLedgerProvider) BindUsageScopeFromNow(
	_ context.Context,
	binding protocol.GoalUsageScopeBinding,
) (protocol.GoalUsageScopeBindResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.bindings = append(p.bindings, binding)
	return protocol.GoalUsageScopeBindResult{}, p.bindErr
}

func (p *roomParentLedgerProvider) snapshots() []protocol.GoalUsageParentSnapshot {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]protocol.GoalUsageParentSnapshot(nil), p.parentSnapshots...)
}

func (p *roomParentLedgerProvider) scopeBindings() []protocol.GoalUsageScopeBinding {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]protocol.GoalUsageScopeBinding(nil), p.bindings...)
}

func (p *roomParentLedgerProvider) UsageByGoalID(
	_ context.Context,
	_ string,
) (*protocol.GoalUsageReport, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.usageReport == nil {
		return nil, nil
	}
	value := *p.usageReport
	return &value, nil
}

func (p *roomParentLedgerProvider) FinalizeUsageForGoal(
	_ context.Context,
	_ string,
	_ protocol.GoalUsage,
	_ string,
) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.finalizeCalls++
	if p.finalizeErr != nil {
		return nil, p.finalizeErr
	}
	return &protocol.Goal{UsageFinalized: true}, nil
}

func (p *roomParentLedgerProvider) finalizeCallCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.finalizeCalls
}

func TestRoomUnboundTerminalWritesDurableRootParentLedger(t *testing.T) {
	provider := &roomParentLedgerProvider{
		fakeRoomGoalContextProvider: &fakeRoomGoalContextProvider{},
	}
	service := &Service{goals: provider}
	slot := roomParentLedgerSlot("slot-before-handoff", "root-handoff")
	slot.setGoalUsageAccumulator(goalsvc.NewRuntimeUsageAccumulator(false))

	service.finalizeGoalUsageForSlot(
		context.Background(),
		slot,
		exec.RoundExecutionResult{
			Usage: sdkprotocol.TokenUsage{
				InputTokens:  11,
				OutputTokens: 4,
				TotalTokens:  21,
			},
			ElapsedTimeSeconds: 6,
		},
		nil,
	)

	snapshots := provider.snapshots()
	if len(snapshots) != 1 {
		t.Fatalf("parent snapshots = %#v, want one terminal ledger row", snapshots)
	}
	got := snapshots[0]
	if got.OwnerUserID != "owner-room" ||
		got.GoalSessionKey != "room:group:conversation-parent-ledger" ||
		got.ScopeRoundID != "root-handoff" ||
		got.SourceRoundID != "slot-before-handoff" ||
		got.GoalID != "" ||
		!got.TokenUsageObserved ||
		got.Usage.InputTokens != 11 ||
		got.Usage.OutputTokens != 4 ||
		got.Usage.ActualTokens() != 21 ||
		got.Usage.RuntimeSeconds != 6 {
		t.Fatalf("unbound parent snapshot = %#v", got)
	}
	if !slot.goalUsageTerminalSettled() || slot.goalUsageActive() {
		t.Fatalf(
			"terminal state settled/active = %v/%v, want true/false",
			slot.goalUsageTerminalSettled(),
			slot.goalUsageActive(),
		)
	}
	if direct := provider.recordedUsage(); len(direct) != 0 {
		t.Fatalf("direct Goal usage = %#v, want durable ledger only", direct)
	}
}

func TestRoomBoundExplicitZeroTerminalStillWritesAuthoritativeLedgerEvidence(t *testing.T) {
	provider := &roomParentLedgerProvider{
		fakeRoomGoalContextProvider: &fakeRoomGoalContextProvider{},
	}
	service := &Service{goals: provider}
	slot := roomParentLedgerSlot("slot-bound-zero", "root-bound-zero")
	slot.setGoalBinding("room:group:conversation-parent-ledger", "goal-bound-zero")
	slot.setGoalUsageAccumulator(goalsvc.NewRuntimeUsageAccumulator(true))

	service.finalizeGoalUsageForSlot(
		context.Background(),
		slot,
		exec.RoundExecutionResult{
			Usage: sdkprotocol.TokenUsage{
				Raw: map[string]any{"total_tokens": int64(0)},
			},
		},
		nil,
	)

	snapshots := provider.snapshots()
	if len(snapshots) != 1 {
		t.Fatalf("parent snapshots = %#v, want explicit-zero terminal row", snapshots)
	}
	got := snapshots[0]
	if got.GoalID != "goal-bound-zero" ||
		!got.TokenUsageObserved ||
		got.Usage.ActualTokens() != 0 ||
		got.Usage.BudgetTokens() != 0 {
		t.Fatalf("explicit-zero parent snapshot = %#v", got)
	}
}

func TestRoomMissingProviderUsagePersistsUnavailableEvidence(t *testing.T) {
	provider := &roomParentLedgerProvider{
		fakeRoomGoalContextProvider: &fakeRoomGoalContextProvider{},
	}
	service := &Service{goals: provider}
	slot := roomParentLedgerSlot("slot-missing-usage", "root-missing-usage")
	slot.setGoalUsageAccumulator(goalsvc.NewRuntimeUsageAccumulator(false))

	service.finalizeGoalUsageForSlot(
		context.Background(),
		slot,
		exec.RoundExecutionResult{ElapsedTimeSeconds: 9},
		nil,
	)

	snapshots := provider.snapshots()
	if len(snapshots) != 1 {
		t.Fatalf("parent snapshots = %#v, want unavailable terminal evidence", snapshots)
	}
	if snapshots[0].TokenUsageObserved ||
		snapshots[0].Usage.ActualTokens() != 0 ||
		snapshots[0].Usage.RuntimeSeconds != 9 {
		t.Fatalf("missing-usage parent snapshot = %#v", snapshots[0])
	}
}

func TestRoomClosedAccumulatorDoesNotReopenUnboundTerminalLedger(t *testing.T) {
	provider := &roomParentLedgerProvider{
		fakeRoomGoalContextProvider: &fakeRoomGoalContextProvider{},
	}
	service := &Service{goals: provider}
	slot := roomParentLedgerSlot("slot-after-clear", "root-after-clear")
	accumulator := goalsvc.NewRuntimeUsageAccumulator(false)
	accumulator.Close()
	slot.setGoalUsageAccumulator(accumulator)

	service.finalizeGoalUsageForSlot(
		context.Background(),
		slot,
		exec.RoundExecutionResult{
			Usage: sdkprotocol.TokenUsage{
				InputTokens:  20,
				OutputTokens: 5,
				TotalTokens:  25,
			},
			ElapsedTimeSeconds: 7,
		},
		nil,
	)

	if snapshots := provider.snapshots(); len(snapshots) != 0 {
		t.Fatalf("closed accumulator parent snapshots = %#v, want none", snapshots)
	}
	if direct := provider.recordedUsage(); len(direct) != 0 {
		t.Fatalf("closed accumulator direct usage = %#v, want none", direct)
	}
}

func TestRoomExternalActivationBindFailureKeepsOldGoalAndBaseline(t *testing.T) {
	provider := &roomParentLedgerProvider{
		fakeRoomGoalContextProvider: &fakeRoomGoalContextProvider{},
		bindErr:                     errors.New("temporary binding failure"),
	}
	service := &Service{goals: provider}
	slot := roomParentLedgerSlot("slot-external-activate", "root-external-activate")
	slot.setGoalBinding("room:group:conversation-parent-ledger", "goal-old")
	slot.setGoalUsageAccumulator(goalsvc.NewRuntimeUsageAccumulator(true))

	err := service.activateGoalUsageForSlot(context.Background(), slot, "goal-external")
	if err == nil {
		t.Fatal("activateGoalUsageForSlot() error = nil, want durable bind failure")
	}
	bindings := provider.scopeBindings()
	if len(bindings) != goalUsagePersistAttempts {
		t.Fatalf("scope binding attempts = %d, want %d", len(bindings), goalUsagePersistAttempts)
	}
	for _, binding := range bindings {
		if binding.OwnerUserID != "owner-room" ||
			binding.GoalSessionKey != "room:group:conversation-parent-ledger" ||
			binding.ScopeRoundID != "root-external-activate" ||
			binding.GoalID != "goal-external" {
			t.Fatalf("external scope binding = %#v", binding)
		}
	}
	if !slot.goalUsageActiveForGoal("goal-old") || slot.goalIDForUsage() != "goal-old" {
		t.Fatalf(
			"failed durable activation changed old Goal/baseline: goal=%q active_old=%v",
			slot.goalIDForUsage(),
			slot.goalUsageActiveForGoal("goal-old"),
		)
	}
	if slot.goalUsageActiveForGoal("goal-external") {
		t.Fatal("failed durable activation reset accounting to the rejected Goal")
	}
}

func TestRoomDurableParentUnavailableStopsFinalizationRetry(t *testing.T) {
	provider := &roomParentLedgerProvider{
		fakeRoomGoalContextProvider: &fakeRoomGoalContextProvider{},
		usageReport: &protocol.GoalUsageReport{
			GoalID: "goal-parent-unavailable",
			Status: protocol.GoalStatusComplete,
		},
		finalizeErr: goalsvc.ErrGoalUsageUnavailable,
	}
	service := &Service{goals: provider}

	if !service.finalizeCompletedRoomGoalWithRetry(
		context.Background(),
		provider,
		"goal-parent-unavailable",
		"round-parent-unavailable",
	) {
		t.Fatal("durable unavailable evidence should be a settled non-finalized terminal")
	}
	if calls := provider.finalizeCallCount(); calls != 1 {
		t.Fatalf("FinalizeUsageForGoal calls = %d, want one non-retryable unavailable result", calls)
	}
}

func roomParentLedgerSlot(sourceRoundID string, scopeRoundID string) *activeRoomSlot {
	slot := &activeRoomSlot{
		RoomSessionID:         "room-session-parent-ledger",
		OwnerUserID:           "owner-room",
		AgentID:               "agent-parent-ledger",
		AgentRoundID:          sourceRoundID,
		GoalUsageScopeRoundID: scopeRoundID,
		RuntimeSessionKey:     "agent:agent-parent-ledger:workspace:group:conversation-parent-ledger",
	}
	slot.setGoalBinding("room:group:conversation-parent-ledger", "")
	return slot
}
