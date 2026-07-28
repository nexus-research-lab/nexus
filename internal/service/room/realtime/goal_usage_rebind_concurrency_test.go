package realtime

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

type blockingRoomGoalUsageProvider struct {
	*fakeRoomGoalContextProvider
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

type failOnceRoomGoalUsageProvider struct {
	*fakeRoomGoalContextProvider
	mu       sync.Mutex
	failNext bool
}

type failNRoomGoalUsageProvider struct {
	*fakeRoomGoalContextProvider
	mu                sync.Mutex
	failuresRemaining int
}

type blockingRoomChildBindProvider struct {
	*fakeRoomGoalContextProvider
	sourceEntered chan struct{}
	sourceRelease chan struct{}
	bindCalled    chan struct{}
	sourceOnce    sync.Once
	bindOnce      sync.Once
	mu            sync.Mutex
	snapshots     []protocol.GoalUsageSourceSnapshot
	order         []string
}

type failingRoomChildFlushProvider struct {
	*fakeRoomGoalContextProvider
	mu        sync.Mutex
	bindCalls int
}

type returningRoomChildGoalProvider struct {
	*fakeRoomGoalContextProvider
	goal protocol.Goal
}

func (p *returningRoomChildGoalProvider) RecordUsageSourceSnapshot(
	_ context.Context,
	_ protocol.GoalUsageSourceSnapshot,
) (protocol.GoalUsageSourceResult, error) {
	goal := p.goal
	return protocol.GoalUsageSourceResult{Goal: &goal}, nil
}

func (p *failingRoomChildFlushProvider) RecordUsageSourceSnapshot(
	_ context.Context,
	_ protocol.GoalUsageSourceSnapshot,
) (protocol.GoalUsageSourceResult, error) {
	return protocol.GoalUsageSourceResult{}, errors.New("child checkpoint unavailable")
}

func (p *failingRoomChildFlushProvider) BindUsageScopeFromNow(
	_ context.Context,
	_ protocol.GoalUsageScopeBinding,
) (protocol.GoalUsageScopeBindResult, error) {
	p.mu.Lock()
	p.bindCalls++
	p.mu.Unlock()
	return protocol.GoalUsageScopeBindResult{}, nil
}

func (p *blockingRoomChildBindProvider) RecordUsageSourceSnapshot(
	_ context.Context,
	snapshot protocol.GoalUsageSourceSnapshot,
) (protocol.GoalUsageSourceResult, error) {
	p.mu.Lock()
	p.snapshots = append(p.snapshots, snapshot)
	p.order = append(p.order, "source:"+snapshot.SourceID)
	callIndex := len(p.snapshots)
	p.mu.Unlock()
	if callIndex == 1 {
		p.sourceOnce.Do(func() { close(p.sourceEntered) })
		<-p.sourceRelease
	}
	return protocol.GoalUsageSourceResult{}, nil
}

func (p *blockingRoomChildBindProvider) BindUsageScopeFromNow(
	_ context.Context,
	_ protocol.GoalUsageScopeBinding,
) (protocol.GoalUsageScopeBindResult, error) {
	p.mu.Lock()
	p.order = append(p.order, "bind")
	p.mu.Unlock()
	p.bindOnce.Do(func() { close(p.bindCalled) })
	return protocol.GoalUsageScopeBindResult{}, nil
}

func (p *failNRoomGoalUsageProvider) RecordUsageForGoal(
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
	return p.fakeRoomGoalContextProvider.RecordUsageForGoal(ctx, goalID, usage, roundID)
}

func (p *failOnceRoomGoalUsageProvider) RecordUsageForGoal(
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
	return p.fakeRoomGoalContextProvider.RecordUsageForGoal(ctx, goalID, usage, roundID)
}

func (p *blockingRoomGoalUsageProvider) RecordUsageForGoal(
	ctx context.Context,
	goalID string,
	usage protocol.GoalUsage,
	roundID string,
) (*protocol.Goal, error) {
	p.once.Do(func() { close(p.entered) })
	<-p.release
	return p.fakeRoomGoalContextProvider.RecordUsageForGoal(ctx, goalID, usage, roundID)
}

func TestRoomChildPersistenceAndExternalBindShareRootScopeBoundary(t *testing.T) {
	provider := &blockingRoomChildBindProvider{
		fakeRoomGoalContextProvider: &fakeRoomGoalContextProvider{},
		sourceEntered:               make(chan struct{}),
		sourceRelease:               make(chan struct{}),
		bindCalled:                  make(chan struct{}),
	}
	const (
		ownerID   = "owner-room-child-bind"
		sessionID = "room:group:child-bind"
		scopeID   = "root-child-bind"
	)
	slot := &activeRoomSlot{
		OwnerUserID:           ownerID,
		AgentID:               "agent-current",
		AgentRoundID:          "slot-current",
		GoalUsageScopeRoundID: scopeID,
		RuntimeSessionKey:     "agent:current:workspace:group:child-bind",
	}
	peer := &activeRoomSlot{
		OwnerUserID:           ownerID,
		AgentID:               "agent-peer",
		AgentRoundID:          "slot-peer",
		GoalUsageScopeRoundID: scopeID,
		RuntimeSessionKey:     "agent:peer:workspace:group:child-bind",
	}
	for _, candidate := range []*activeRoomSlot{slot, peer} {
		candidate.setRuntimeKind("nxs")
		candidate.setGoalBinding(sessionID, "")
	}
	// peer 的 running child 只有内存 pending；activation 必须在 bind 前把
	// 它写成 evidence/checkpoint，且不能因为是 0 就丢掉 lifecycle。
	peer.markSubagentUsageObservationPending(
		roomSubagentUsageObservation{},
		"task-peer-running",
	)
	roundValue := &activeRoomRound{
		ConversationID: "child-bind",
		SessionKey:     sessionID,
		RoundID:        scopeID,
		Slots: map[string]*activeRoomSlot{
			slot.AgentID: slot,
			peer.AgentID: peer,
		},
	}
	service := &Service{
		goals:  provider,
		rounds: newRoomRoundRegistryFromRounds(map[string]*activeRoomRound{scopeID: roundValue}),
	}

	recorded := make(chan []roomSubagentUsageSettlement, 1)
	go func() {
		recorded <- service.recordSubagentGoalUsageForSlot(
			context.Background(),
			slot,
			protocol.Message{"metadata": map[string]any{
				"task_id":   "task-current",
				"task_type": "local_agent",
				"subtype":   "task_progress",
				"status":    "running",
				"usage":     map[string]any{"total_tokens": int64(40)},
			}},
		)
	}()
	select {
	case <-provider.sourceEntered:
	case <-time.After(time.Second):
		t.Fatal("pre-bind Room child checkpoint did not enter persistence")
	}

	activationDone := make(chan error, 1)
	go func() {
		activationDone <- service.activateGoalUsageForSlot(
			context.Background(),
			slot,
			"goal-new",
		)
	}()
	select {
	case <-provider.bindCalled:
		t.Fatal("external Room Goal bind overtook an in-flight child checkpoint")
	case <-time.After(25 * time.Millisecond):
	}

	close(provider.sourceRelease)
	select {
	case <-recorded:
	case <-time.After(time.Second):
		t.Fatal("pre-bind Room child checkpoint did not finish")
	}
	select {
	case err := <-activationDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("external Room Goal bind did not resume")
	}

	provider.mu.Lock()
	snapshots := append([]protocol.GoalUsageSourceSnapshot(nil), provider.snapshots...)
	order := append([]string(nil), provider.order...)
	provider.mu.Unlock()
	if len(snapshots) != 2 {
		t.Fatalf("source snapshots = %#v, want in-flight current + flushed peer", snapshots)
	}
	for _, snapshot := range snapshots {
		if snapshot.GoalID != "" {
			t.Fatalf("pre-bind snapshot GoalID = %q, want old unbound scope", snapshot.GoalID)
		}
		if !snapshot.EvidenceRequired || snapshot.Terminal {
			t.Fatalf("pre-bind running child evidence = %#v", snapshot)
		}
	}
	if got, want := order, []string{"source:task-current", "source:task-peer-running", "bind"}; len(got) != len(want) ||
		got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("persistence/bind order = %#v, want %#v", got, want)
	}
	if got := slot.goalIDForUsage(); got != "goal-new" {
		t.Fatalf("Goal binding = %q, want goal-new", got)
	}
}

func TestRoomExternalBindRequiresKnownChildPendingToFlush(t *testing.T) {
	provider := &failingRoomChildFlushProvider{
		fakeRoomGoalContextProvider: &fakeRoomGoalContextProvider{},
	}
	service := &Service{goals: provider}
	slot := &activeRoomSlot{
		OwnerUserID:           "owner-child-flush-failure",
		AgentRoundID:          "slot-child-flush-failure",
		GoalUsageScopeRoundID: "root-child-flush-failure",
		RuntimeSessionKey:     "agent:nexus:workspace:group:child-flush-failure",
	}
	slot.setRuntimeKind("nxs")
	slot.setGoalBinding("room:group:child-flush-failure", "")
	slot.markSubagentUsageObservationPending(
		roomSubagentUsageObservation{observedAt: time.Now().UTC()},
		"task-running",
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := service.activateGoalUsageForSlot(ctx, slot, "goal-new"); err == nil {
		t.Fatal("external Room Goal bind succeeded with an unflushed child pending snapshot")
	}
	provider.mu.Lock()
	bindCalls := provider.bindCalls
	provider.mu.Unlock()
	if bindCalls != 0 {
		t.Fatalf("BindUsageScopeFromNow calls = %d, want 0 before child flush succeeds", bindCalls)
	}
	if got := slot.goalIDForUsage(); got != "" {
		t.Fatalf("Goal binding = %q, want old unbound state after flush failure", got)
	}
	if pending := slot.subagentUsageObservationPendingSnapshot(); len(pending) != 1 {
		t.Fatalf("pending child snapshots = %#v, want retained retry state", pending)
	}
}

func TestRoomChildResultBindsOnlyMatchingRootScope(t *testing.T) {
	const sessionKey = "room:group:child-result-scope"
	provider := &returningRoomChildGoalProvider{
		fakeRoomGoalContextProvider: &fakeRoomGoalContextProvider{},
		goal: protocol.Goal{
			ID:         "goal-child-result-scope",
			SessionKey: sessionKey,
		},
	}
	origin := &activeRoomSlot{
		AgentID:               "agent-origin",
		AgentRoundID:          "slot-origin",
		GoalUsageScopeRoundID: "root-matching",
		RuntimeSessionKey:     "agent:origin:workspace:group:child-result-scope",
	}
	peer := &activeRoomSlot{
		AgentID:               "agent-peer",
		AgentRoundID:          "slot-peer",
		GoalUsageScopeRoundID: "root-matching",
		RuntimeSessionKey:     "agent:peer:workspace:group:child-result-scope",
	}
	unrelated := &activeRoomSlot{
		AgentID:               "agent-unrelated",
		AgentRoundID:          "slot-unrelated",
		GoalUsageScopeRoundID: "root-unrelated",
		RuntimeSessionKey:     "agent:unrelated:workspace:group:child-result-scope",
	}
	for _, slot := range []*activeRoomSlot{origin, peer, unrelated} {
		slot.setRuntimeKind("nxs")
		slot.setGoalBinding(sessionKey, "")
	}
	service := &Service{
		goals: provider,
		rounds: newRoomRoundRegistryFromRounds(map[string]*activeRoomRound{
			"matching": {
				ConversationID: "child-result-scope",
				SessionKey:     sessionKey,
				RootRoundID:    "root-matching",
				Slots: map[string]*activeRoomSlot{
					origin.AgentID: origin,
					peer.AgentID:   peer,
				},
			},
			"unrelated": {
				ConversationID: "child-result-scope",
				SessionKey:     sessionKey,
				RootRoundID:    "root-unrelated",
				Slots: map[string]*activeRoomSlot{
					unrelated.AgentID: unrelated,
				},
			},
		}),
	}
	settled := service.recordSubagentGoalUsageForSlot(
		context.Background(),
		origin,
		protocol.Message{"metadata": map[string]any{
			"task_id":   "task-child-result",
			"task_type": "local_agent",
			"subtype":   "task_progress",
			"status":    "running",
			"usage":     map[string]any{"total_tokens": int64(20)},
		}},
	)
	if len(settled) != 1 {
		t.Fatalf("settled child snapshots = %#v, want one", settled)
	}
	for _, slot := range []*activeRoomSlot{origin, peer} {
		if got := slot.goalIDForUsage(); got != provider.goal.ID {
			t.Fatalf("%s Goal binding = %q, want %q", slot.AgentID, got, provider.goal.ID)
		}
	}
	if got := unrelated.goalIDForUsage(); got != "" {
		t.Fatalf("unrelated root Goal binding = %q, want empty", got)
	}
}

func TestRoomSlotSerializesUsageSettlementWithExternalGoalRebind(t *testing.T) {
	provider := &blockingRoomGoalUsageProvider{
		fakeRoomGoalContextProvider: &fakeRoomGoalContextProvider{},
		entered:                     make(chan struct{}),
		release:                     make(chan struct{}),
	}
	service := &Service{goals: provider}
	slot := &activeRoomSlot{
		RuntimeSessionKey: "agent:nexus:ws:room:rebind",
		AgentRoundID:      "round-old",
	}
	slot.setGoalBinding("room:group:rebind", "goal-old")
	slot.setGoalUsageAccumulator(goalsvc.NewRuntimeUsageAccumulator(true))

	recorded := make(chan struct{})
	go func() {
		service.recordGoalUsageSnapshotForSlot(context.Background(), slot, goalsvc.RuntimeUsageSnapshot{
			Usage:              protocol.GoalUsage{InputTokens: 10, OutputTokens: 2, ActualTotalTokens: 12},
			Cumulative:         true,
			Terminal:           true,
			TokenUsageObserved: true,
		})
		close(recorded)
	}()
	<-provider.entered

	activationStarted := make(chan struct{})
	activationDone := make(chan struct{})
	go func() {
		close(activationStarted)
		activateGoalUsageForSlot(context.Background(), slot, "goal-new")
		close(activationDone)
	}()
	<-activationStarted
	select {
	case <-activationDone:
		t.Fatal("external Room Goal rebind completed before old usage settlement")
	case <-time.After(25 * time.Millisecond):
	}

	close(provider.release)
	select {
	case <-recorded:
	case <-time.After(time.Second):
		t.Fatal("old Room Goal usage settlement did not finish")
	}
	select {
	case <-activationDone:
	case <-time.After(time.Second):
		t.Fatal("external Room Goal rebind did not resume")
	}

	provider.mu.Lock()
	gotIDs := append([]string(nil), provider.usageGoalIDs...)
	provider.mu.Unlock()
	if len(gotIDs) != 1 || gotIDs[0] != "goal-old" {
		t.Fatalf("usage Goal IDs = %#v, want old delta fixed to goal-old", gotIDs)
	}
	if got := slot.goalIDForUsage(); got != "goal-new" {
		t.Fatalf("Goal binding = %q, want goal-new after settlement", got)
	}
}

func TestRoomSlotRetriesUncommittedUsageAtTerminal(t *testing.T) {
	base := &fakeRoomGoalContextProvider{}
	provider := &failOnceRoomGoalUsageProvider{
		fakeRoomGoalContextProvider: base,
		failNext:                    true,
	}
	service := &Service{goals: provider}
	slot := &activeRoomSlot{
		RuntimeSessionKey: "agent:nexus:ws:room:retry",
		AgentRoundID:      "round-retry",
	}
	slot.setGoalBinding("room:group:retry", "goal-retry")
	slot.setGoalUsageAccumulator(goalsvc.NewRuntimeUsageAccumulator(true))

	service.recordGoalUsageSnapshotForSlot(context.Background(), slot, goalsvc.RuntimeUsageSnapshot{
		TurnID: "turn-a",
		Usage: protocol.GoalUsage{
			InputTokens:       90,
			OutputTokens:      10,
			ActualTotalTokens: 100,
			ActualTotalKnown:  true,
		},
	})
	if !service.settleTerminalGoalUsageSnapshotForSlotWithRetry(
		context.Background(),
		slot,
		goalsvc.RuntimeUsageSnapshot{
			Usage: protocol.GoalUsage{
				InputTokens:       140,
				OutputTokens:      10,
				ActualTotalTokens: 150,
				ActualTotalKnown:  true,
			},
			Cumulative:         true,
			Terminal:           true,
			TokenUsageObserved: true,
		}) {
		t.Fatal("terminal usage retry did not settle")
	}

	usages := base.recordedUsage()
	if len(usages) != 1 || usages[0].BudgetTokens() != 150 || usages[0].ActualTokens() != 150 {
		t.Fatalf("persisted usage = %#v, want one complete terminal retry of 150", usages)
	}
}

func TestRoomSlotRetainsTerminalDeltaAfterRetryWindow(t *testing.T) {
	base := &fakeRoomGoalContextProvider{}
	provider := &failNRoomGoalUsageProvider{
		fakeRoomGoalContextProvider: base,
		failuresRemaining:           goalUsagePersistAttempts,
	}
	service := &Service{goals: provider, rounds: newRoomRoundRegistry()}
	slot := &activeRoomSlot{
		AgentID:           "agent-retry-window",
		RuntimeSessionKey: "agent:nexus:ws:room:retry-window",
		AgentRoundID:      "round-retry-window",
	}
	slot.setGoalBinding("room:group:retry-window", "goal-retry-window")
	slot.setGoalUsageAccumulator(goalsvc.NewRuntimeUsageAccumulator(true))

	service.finalizeGoalUsageForSlot(context.Background(), slot, exec.RoundExecutionResult{
		Usage: sdkprotocol.TokenUsage{
			InputTokens:  140,
			OutputTokens: 10,
			TotalTokens:  150,
		},
	}, nil)
	if !slot.goalUsageActive() || slot.goalUsageTerminalSettled() {
		t.Fatal("terminal persistence failure did not retain an unsettled accumulator")
	}
	slot.setStatus("finished")
	roundValue := &activeRoomRound{
		SessionKey: "room:group:retry-window",
		RoundID:    "round-retry-window",
		Slots:      map[string]*activeRoomSlot{slot.AgentID: slot},
	}
	if !service.finalizeCompletedRoomGoalUsage(context.Background(), roundValue) {
		t.Fatal("retained Room terminal delta did not settle on the round coordinator retry")
	}
	if slot.goalUsageActive() || !slot.goalUsageTerminalSettled() {
		t.Fatal("settled Room terminal accumulator did not close its barrier")
	}
	usages := base.recordedUsage()
	if len(usages) != 1 || usages[0].BudgetTokens() != 150 || usages[0].ActualTokens() != 150 {
		t.Fatalf("persisted usage = %#v, want one retained terminal delta of 150", usages)
	}
}
