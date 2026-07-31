package realtime

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	exec "github.com/nexus-research-lab/nexus/internal/runtime/exec"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
)

func TestSharedRoomGoalWaitsForFailedUsageClaimThenFinalizesOnce(t *testing.T) {
	const (
		conversationID = "conversation-claim-retry"
		goalID         = "goal-claim-retry"
	)
	sessionKey := protocol.BuildRoomSharedSessionKey(conversationID)
	slot := &activeRoomSlot{
		AgentID:      "agent-claim-retry",
		AgentRoundID: "round-claim-retry",
		RuntimeSessionKey: protocol.BuildRoomAgentSessionKey(
			conversationID,
			"agent-claim-retry",
			protocol.RoomTypeGroup,
		),
	}
	slot.setRuntimeKind("nxs")
	grantTestRoomGoalAuthority(slot, sessionKey, goalID)
	slot.setGoalUsageAccumulator(goalsvc.NewRuntimeUsageAccumulator(true))
	slot.setStatus("finished")
	roundValue := &activeRoomRound{
		SessionKey:     sessionKey,
		ConversationID: conversationID,
		RoundID:        "room-round-claim-retry",
		RootRoundID:    "root-round-claim-retry",
		OwnerUserID:    "owner-claim-retry",
		Slots:          map[string]*activeRoomSlot{"claim": slot},
	}
	provider := &fakeRoomGoalUsageFinalizer{
		fakeRoomGoalContextProvider: &fakeRoomGoalContextProvider{},
		report: protocol.GoalUsageReport{
			GoalID:     goalID,
			SessionKey: sessionKey,
			Status:     protocol.GoalStatusComplete,
		},
		claimFailuresRemaining: goalUsagePersistAttempts * 2,
	}
	service := &Service{
		goals: provider,
		rounds: newRoomRoundRegistryFromRounds(map[string]*activeRoomRound{
			roundValue.RoundID: roundValue,
		}),
	}
	accelerateRoomGoalUsageRetry(service)

	if service.claimSubagentGoalUsageForRoomSlot(
		context.Background(),
		slot,
		goalID,
		sessionKey,
	) {
		t.Fatal("initial Room usage claim unexpectedly succeeded")
	}
	if !slot.goalUsageClaimPending() {
		t.Fatal("failed Room model-create claim did not remain pending")
	}
	if service.finalizeCompletedRoomGoalUsage(context.Background(), roundValue) {
		t.Fatal("shared Goal finalized while the pending Room claim kept failing")
	}
	if calls := provider.finalizeCallCount(); calls != 0 {
		t.Fatalf("FinalizeUsageForGoal calls before Room claim success = %d, want 0", calls)
	}
	if successes := provider.claimSuccessCount(); successes != 0 {
		t.Fatalf("successful Room claims before recovery = %d, want 0", successes)
	}

	if !service.finalizeCompletedRoomGoalUsage(context.Background(), roundValue) {
		t.Fatal("shared Goal did not finalize after Room claim persistence recovered")
	}
	if slot.goalUsageClaimPending() {
		t.Fatal("successful Room claim retry left the slot pending")
	}
	attempts := provider.claimAttemptCount()
	if attempts != goalUsagePersistAttempts*2+1 {
		t.Fatalf("Room claim attempts = %d, want %d failures plus one success", attempts, goalUsagePersistAttempts*2)
	}
	if successes := provider.claimSuccessCount(); successes != 1 {
		t.Fatalf("successful Room claims after recovery = %d, want exactly 1", successes)
	}
	if calls := provider.finalizeCallCount(); calls != 1 {
		t.Fatalf("FinalizeUsageForGoal calls after Room recovery = %d, want exactly 1", calls)
	}

	if !service.finalizeCompletedRoomGoalUsage(context.Background(), roundValue) {
		t.Fatal("repeated Room claim/finalization join should be an idempotent success")
	}
	if got := provider.claimAttemptCount(); got != attempts {
		t.Fatalf("repeated Room join retried an already successful claim: attempts %d -> %d", attempts, got)
	}
	if successes := provider.claimSuccessCount(); successes != 1 {
		t.Fatalf("successful Room claims after repeated join = %d, want exactly 1", successes)
	}
	if calls := provider.finalizeCallCount(); calls != 1 {
		t.Fatalf("FinalizeUsageForGoal calls after repeated Room join = %d, want exactly 1", calls)
	}
}

func TestTerminalRoomGoalUsageMustSettleBeforeCompletionOrContinuation(t *testing.T) {
	const (
		conversationID = "conversation-terminal-settlement"
		goalID         = "goal-terminal-settlement"
	)
	sessionKey := protocol.BuildRoomSharedSessionKey(conversationID)
	caller := &activeRoomSlot{
		AgentID:      "agent-lead",
		AgentRoundID: "round-lead",
	}
	peer := &activeRoomSlot{
		AgentID:      "agent-peer",
		AgentRoundID: "round-peer",
	}
	for _, slot := range []*activeRoomSlot{caller, peer} {
		slot.setGoalBinding(sessionKey, goalID)
		slot.setGoalUsageAccumulator(goalsvc.NewRuntimeUsageAccumulator(true))
		slot.setStatus("finished")
	}
	caller.setGoalUsageTerminalSettled(true)
	roundValue := &activeRoomRound{
		SessionKey:     sessionKey,
		ConversationID: conversationID,
		RoundID:        "room-round",
		RootRoundID:    "root-round",
		Slots: map[string]*activeRoomSlot{
			"caller": caller,
			"peer":   peer,
		},
	}
	service := &Service{
		rounds: newRoomRoundRegistryFromRounds(map[string]*activeRoomRound{
			"room-round": roundValue,
		}),
	}

	blocker := service.activeRoomGoalBlocker(
		sessionKey,
		conversationID,
		caller.AgentID,
		caller.AgentRoundID,
	)
	if !strings.Contains(blocker, "terminal Goal usage is not settled") {
		t.Fatalf("completion blocker = %q, want unsettled terminal usage", blocker)
	}
	contextValue := &protocol.ConversationContextAggregate{
		Conversation: protocol.ConversationRecord{ID: conversationID},
	}
	if !service.shouldDeferGoalContinuationForTargetStateLocked(
		context.Background(),
		sessionKey,
		contextValue,
	) {
		t.Fatal("Goal continuation was not deferred for unsettled terminal usage")
	}

	peer.setGoalUsageTerminalSettled(true)
	if blocker := service.activeRoomGoalBlocker(
		sessionKey,
		conversationID,
		caller.AgentID,
		caller.AgentRoundID,
	); blocker != "" {
		t.Fatalf("completion blocker after terminal settlement = %q, want empty", blocker)
	}
	if service.shouldDeferGoalContinuationForTargetStateLocked(
		context.Background(),
		sessionKey,
		contextValue,
	) {
		t.Fatal("Goal continuation remained deferred after every terminal usage settled")
	}
}

func TestSharedRoomGoalFinalizerWaitsForEverySlotAndRunsOnce(t *testing.T) {
	const (
		conversationID = "conversation-shared-finalizer"
		goalID         = "goal-shared-finalizer"
	)
	sessionKey := protocol.BuildRoomSharedSessionKey(conversationID)
	first := &activeRoomSlot{
		AgentID:      "agent-first",
		AgentRoundID: "round-first",
	}
	second := &activeRoomSlot{
		AgentID:      "agent-second",
		AgentRoundID: "round-second",
	}
	for _, slot := range []*activeRoomSlot{first, second} {
		slot.setGoalBinding(sessionKey, goalID)
		slot.setGoalUsageAccumulator(goalsvc.NewRuntimeUsageAccumulator(true))
	}
	first.setStatus("finished")
	first.setGoalUsageTerminalSettled(true)
	second.setStatus("running")

	roundValue := &activeRoomRound{
		SessionKey:     sessionKey,
		ConversationID: conversationID,
		RoundID:        "room-round",
		RootRoundID:    "root-round",
		Slots: map[string]*activeRoomSlot{
			"first":  first,
			"second": second,
		},
	}
	base := &fakeRoomGoalContextProvider{}
	provider := &fakeRoomGoalUsageFinalizer{
		fakeRoomGoalContextProvider: base,
		report: protocol.GoalUsageReport{
			GoalID:     goalID,
			SessionKey: sessionKey,
			Status:     protocol.GoalStatusComplete,
		},
	}
	provider.beforeFinalize = func() {
		for name, slot := range roundValue.Slots {
			if !slot.isTerminal() {
				t.Fatalf("FinalizeUsageForGoal observed non-terminal slot %s", name)
			}
			if !slot.goalUsageTerminalSettled() {
				t.Fatalf("FinalizeUsageForGoal observed unsettled slot %s", name)
			}
			if slot.hasRunningSubagentTask() {
				t.Fatalf("FinalizeUsageForGoal observed running child in slot %s", name)
			}
		}
	}
	service := &Service{
		goals: provider,
		rounds: newRoomRoundRegistryFromRounds(map[string]*activeRoomRound{
			"room-round": roundValue,
		}),
	}

	if service.finalizeCompletedRoomGoalUsage(context.Background(), roundValue) {
		t.Fatal("shared finalizer succeeded while a bound slot was still running")
	}
	if calls := provider.finalizeCallCount(); calls != 0 {
		t.Fatalf("FinalizeUsageForGoal calls with running slot = %d, want 0", calls)
	}

	second.setStatus("finished")
	second.setSubagentTasks(map[string]struct{}{"child-task": {}})
	if service.finalizeCompletedRoomGoalUsage(context.Background(), roundValue) {
		t.Fatal("shared finalizer succeeded while a bound slot still had a running child")
	}
	if calls := provider.finalizeCallCount(); calls != 0 {
		t.Fatalf("FinalizeUsageForGoal calls with running child = %d, want 0", calls)
	}

	second.setSubagentTasks(nil)
	if !service.finalizeCompletedRoomGoalUsage(context.Background(), roundValue) {
		t.Fatal("shared finalizer did not succeed after all slot and child work settled")
	}
	if !second.goalUsageTerminalSettled() {
		t.Fatal("shared finalizer did not settle the last terminal slot before fencing usage")
	}
	if calls := provider.finalizeCallCount(); calls != 1 {
		t.Fatalf("FinalizeUsageForGoal calls after convergence = %d, want 1", calls)
	}

	if !service.finalizeCompletedRoomGoalUsage(context.Background(), roundValue) {
		t.Fatal("repeated shared finalization should be an idempotent success")
	}
	if calls := provider.finalizeCallCount(); calls != 1 {
		t.Fatalf("FinalizeUsageForGoal calls after repeated convergence = %d, want exactly 1", calls)
	}
}

func TestRoomSubagentUsageRetryRecoversWithoutAnotherRuntimeMessage(t *testing.T) {
	const (
		conversationID = "conversation-source-background-retry"
		goalID         = "goal-source-background-retry"
	)
	sessionKey := protocol.BuildRoomSharedSessionKey(conversationID)
	slot := &activeRoomSlot{
		AgentID:      "agent-source-background-retry",
		AgentRoundID: "round-source-background-retry",
		RuntimeSessionKey: protocol.BuildRoomAgentSessionKey(
			conversationID,
			"agent-source-background-retry",
			protocol.RoomTypeGroup,
		),
	}
	slot.setRuntimeKind("nxs")
	grantTestRoomGoalAuthority(slot, sessionKey, goalID)
	slot.setGoalUsageAccumulator(goalsvc.NewRuntimeUsageAccumulator(true))
	slot.setGoalUsageTerminalSettled(true)
	slot.setStatus("finished")
	slot.rememberSubagentTaskMessage(protocol.Message{"metadata": map[string]any{
		"subtype": "task_started", "task_id": "task-1", "agent_id": "agent-1", "agent_type": "worker",
	}})
	roundValue := &activeRoomRound{
		SessionKey:     sessionKey,
		ConversationID: conversationID,
		RoundID:        "room-round-source-background-retry",
		OwnerUserID:    "owner-source-background-retry",
		Slots:          map[string]*activeRoomSlot{"worker": slot},
	}
	roundValue.RunningSubagents.Store(true)
	provider := &retryingRoomGoalUsageSourceProvider{
		fakeRoomGoalUsageFinalizer: &fakeRoomGoalUsageFinalizer{
			fakeRoomGoalContextProvider: &fakeRoomGoalContextProvider{},
			report: protocol.GoalUsageReport{
				GoalID:     goalID,
				SessionKey: sessionKey,
				Status:     protocol.GoalStatusComplete,
			},
		},
		failuresRemaining:         goalUsagePersistAttempts,
		finalizeFailuresRemaining: goalUsagePersistAttempts,
		persisted:                 make(chan struct{}),
	}
	service := &Service{
		goals: provider,
		rounds: newRoomRoundRegistryFromRounds(map[string]*activeRoomRound{
			roundValue.RoundID: roundValue,
		}),
	}
	accelerateRoomGoalUsageRetry(service)
	terminalMessage := protocol.Message{"metadata": map[string]any{
		"subtype": "task_notification", "task_id": "task-1", "agent_id": "agent-1",
		"agent_type": "worker", "status": "completed",
		"usage": map[string]any{"total_tokens": int64(240)},
	}}

	settled := service.recordSubagentGoalUsageForSlot(context.Background(), slot, terminalMessage)
	slot.rememberSubagentTaskMessage(terminalMessage)
	for _, settlement := range settled {
		slot.clearSubagentUsagePending(settlement.taskID, settlement.cumulativeTotal)
	}
	if len(settled) != 0 || !slot.hasRunningSubagentTask() {
		t.Fatalf("failed synchronous persistence settled=%#v running=%v, want pending barrier", settled, slot.hasRunningSubagentTask())
	}
	service.startRoomSubagentUsageRetry(roundValue, slot)

	select {
	case <-provider.persisted:
	case <-time.After(2 * time.Second):
		t.Fatal("background Room child usage retry did not recover without another runtime message")
	}
	deadline := time.Now().Add(2 * time.Second)
	for provider.finalizeCallCount() != 1 || roundValue.RunningSubagents.Load() {
		if time.Now().After(deadline) {
			t.Fatalf(
				"background recovery finalize calls=%d running_subagents=%v pending=%#v",
				provider.finalizeCallCount(),
				roundValue.RunningSubagents.Load(),
				slot.subagentUsagePendingSnapshot(),
			)
		}
		time.Sleep(time.Millisecond)
	}
	if slot.hasRunningSubagentTask() {
		t.Fatalf("background recovery left child barrier: %#v", slot.subagentUsagePendingSnapshot())
	}
	if got := provider.lastPersistedTotal(); got != 240 {
		t.Fatalf("background persisted total = %d, want latest 240", got)
	}
	if attempts := provider.finalizeAttemptCount(); attempts != goalUsagePersistAttempts+1 {
		t.Fatalf(
			"background finalize attempts = %d, want %d failures plus one success",
			attempts,
			goalUsagePersistAttempts,
		)
	}
	if !service.finalizeCompletedRoomGoalUsage(context.Background(), roundValue) {
		t.Fatal("repeated shared finalization should be an idempotent success")
	}
	if calls := provider.finalizeCallCount(); calls != 1 {
		t.Fatalf("shared Goal finalized %d times, want exactly once", calls)
	}
}

func TestRoomSubagentUsageRetryDoesNotFinalizeWhileParentRuns(t *testing.T) {
	const goalID = "goal-source-running-parent"
	slot := &activeRoomSlot{
		AgentID:           "agent-running-parent",
		AgentRoundID:      "round-running-parent",
		RuntimeSessionKey: "agent:running-parent",
	}
	slot.setRuntimeKind("nxs")
	slot.setGoalBinding("room:group:running-parent", goalID)
	slot.setGoalUsageAccumulator(goalsvc.NewRuntimeUsageAccumulator(true))
	slot.setStatus("running")
	roundValue := &activeRoomRound{
		SessionKey:  "room:group:running-parent",
		RoundID:     "room-round-running-parent",
		OwnerUserID: "owner-running-parent",
		Slots:       map[string]*activeRoomSlot{"worker": slot},
	}
	roundValue.RunningSubagents.Store(true)
	provider := &retryingRoomGoalUsageSourceProvider{
		fakeRoomGoalUsageFinalizer: &fakeRoomGoalUsageFinalizer{
			fakeRoomGoalContextProvider: &fakeRoomGoalContextProvider{},
			report: protocol.GoalUsageReport{
				GoalID:     goalID,
				SessionKey: roundValue.SessionKey,
				Status:     protocol.GoalStatusComplete,
			},
		},
		persisted: make(chan struct{}),
	}
	service := &Service{goals: provider}
	accelerateRoomGoalUsageRetry(service)
	slot.markSubagentUsagePending("task-running-parent", 10)
	service.startRoomSubagentUsageRetry(roundValue, slot)

	select {
	case <-provider.persisted:
	case <-time.After(time.Second):
		t.Fatal("background Room usage retry did not persist running-parent child")
	}
	time.Sleep(30 * time.Millisecond)
	if calls := provider.finalizeCallCount(); calls != 0 {
		t.Fatalf("child retry finalized shared Goal while parent was running: %d calls", calls)
	}
	if !roundValue.RunningSubagents.Load() {
		t.Fatal("child retry released post-round state while parent was running")
	}
}

func TestRoomParentUsageRetryRecoversWithoutChildOrRuntimeMessage(t *testing.T) {
	const (
		conversationID = "conversation-parent-background-retry"
		goalID         = "goal-parent-background-retry"
	)
	sessionKey := protocol.BuildRoomSharedSessionKey(conversationID)
	slot := &activeRoomSlot{
		AgentID:      "agent-parent-background-retry",
		AgentRoundID: "round-parent-background-retry",
		RuntimeSessionKey: protocol.BuildRoomAgentSessionKey(
			conversationID,
			"agent-parent-background-retry",
			protocol.RoomTypeGroup,
		),
	}
	grantTestRoomGoalAuthority(slot, sessionKey, goalID)
	slot.setGoalUsageAccumulator(goalsvc.NewRuntimeUsageAccumulator(true))
	slot.setStatus("finished")
	roundValue := &activeRoomRound{
		SessionKey:     sessionKey,
		ConversationID: conversationID,
		RoundID:        "room-round-parent-background-retry",
		OwnerUserID:    "owner-parent-background-retry",
		Slots:          map[string]*activeRoomSlot{"worker": slot},
	}
	base := &fakeRoomGoalContextProvider{}
	provider := &retryingRoomGoalUsageSourceProvider{
		fakeRoomGoalUsageFinalizer: &fakeRoomGoalUsageFinalizer{
			fakeRoomGoalContextProvider: base,
			report: protocol.GoalUsageReport{
				GoalID:     goalID,
				SessionKey: sessionKey,
				Status:     protocol.GoalStatusComplete,
			},
		},
		parentUsageFailuresRemaining: goalUsagePersistAttempts * 2,
		finalizeFailuresRemaining:    goalUsagePersistAttempts,
		parentUsagePersisted:         make(chan struct{}),
	}
	service := &Service{
		goals: provider,
		rounds: newRoomRoundRegistryFromRounds(map[string]*activeRoomRound{
			roundValue.RoundID: roundValue,
		}),
	}
	accelerateRoomGoalUsageRetry(service)
	result := exec.RoundExecutionResult{Usage: sdkprotocol.TokenUsage{
		InputTokens:  90,
		OutputTokens: 10,
		TotalTokens:  100,
	}}

	// 模拟 slot 自身 terminal settlement 的同步窗口先耗尽。
	service.finalizeGoalUsageForSlot(context.Background(), slot, result, nil)
	if slot.goalUsageTerminalSettled() {
		t.Fatal("terminal parent usage unexpectedly settled inside the first retry window")
	}
	if service.settleCompletedRoomGoalUsage(context.Background(), roundValue) {
		t.Fatal("round settlement unexpectedly succeeded inside the second retry window")
	}
	if !roundValue.RunningSubagents.Load() {
		t.Fatal("failed parent settlement did not establish the post-round barrier")
	}

	select {
	case <-provider.parentUsagePersisted:
	case <-time.After(3 * time.Second):
		t.Fatal("background Room parent usage retry did not recover without another runtime message")
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		base.mu.Lock()
		planCalls := base.planCalls
		base.mu.Unlock()
		if provider.finalizeCallCount() == 1 &&
			!roundValue.RunningSubagents.Load() &&
			roundValue.postRoundDispatched.Load() &&
			planCalls == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf(
				"background parent recovery finalize_calls=%d barrier=%v post_round=%v plan_calls=%d",
				provider.finalizeCallCount(),
				roundValue.RunningSubagents.Load(),
				roundValue.postRoundDispatched.Load(),
				planCalls,
			)
		}
		time.Sleep(time.Millisecond)
	}

	if !slot.goalUsageTerminalSettled() {
		t.Fatal("background parent retry left terminal usage unsettled")
	}
	usages := base.recordedUsage()
	if len(usages) != 1 || usages[0].ActualTokens() != 100 || usages[0].BudgetTokens() != 100 {
		t.Fatalf("persisted parent usage = %#v, want exact 100 once", usages)
	}
	if attempts := provider.parentUsageAttemptCount(); attempts != goalUsagePersistAttempts*2+1 {
		t.Fatalf(
			"parent usage attempts = %d, want %d failures plus one background success",
			attempts,
			goalUsagePersistAttempts*2,
		)
	}
	if attempts := provider.finalizeAttemptCount(); attempts != goalUsagePersistAttempts+1 {
		t.Fatalf(
			"shared finalization attempts = %d, want %d failures plus one background success",
			attempts,
			goalUsagePersistAttempts,
		)
	}

	// runRound 与 worker 即使在 barrier 释放处竞争，也只能派发一次。
	service.dispatchPostRoundWorkOnce(context.Background(), roundValue)
	base.mu.Lock()
	planCalls := base.planCalls
	base.mu.Unlock()
	if planCalls != 1 {
		t.Fatalf("post-round plan calls = %d, want exactly once", planCalls)
	}
}

func TestRoomPostRoundDispatchRunsOnceUnderRace(t *testing.T) {
	base := &fakeRoomGoalContextProvider{}
	service := &Service{goals: base}
	roundValue := &activeRoomRound{
		SessionKey: "room:group:post-round-once",
		RoundID:    "round-post-round-once",
	}
	attachTestRoomGoalAuthority(roundValue, "goal-post-round-once")

	var waitGroup sync.WaitGroup
	for range 32 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			service.dispatchPostRoundWorkOnce(context.Background(), roundValue)
		}()
	}
	waitGroup.Wait()

	base.mu.Lock()
	planCalls := base.planCalls
	base.mu.Unlock()
	if planCalls != 1 || !roundValue.postRoundDispatched.Load() {
		t.Fatalf(
			"post-round plan calls=%d dispatched=%v, want exactly once",
			planCalls,
			roundValue.postRoundDispatched.Load(),
		)
	}
}

func TestRoomParentTerminalHandoffRestartsWorkerAfterSkippedStart(t *testing.T) {
	const goalID = "goal-parent-terminal-handoff"
	slot := &activeRoomSlot{
		AgentID:      "agent-parent-terminal-handoff",
		AgentRoundID: "agent-round-parent-terminal-handoff",
	}
	slot.setGoalBinding("room:group:parent-terminal-handoff", goalID)
	slot.setGoalUsageAccumulator(goalsvc.NewRuntimeUsageAccumulator(true))
	slot.setGoalUsageTerminalSettled(true)
	slot.setStatus("running")
	roundValue := &activeRoomRound{
		SessionKey: "room:group:parent-terminal-handoff",
		RoundID:    "round-parent-terminal-handoff",
		Slots:      map[string]*activeRoomSlot{"worker": slot},
	}
	roundValue.RunningSubagents.Store(true)
	provider := &fakeRoomGoalUsageFinalizer{
		fakeRoomGoalContextProvider: &fakeRoomGoalContextProvider{},
		report: protocol.GoalUsageReport{
			GoalID:     goalID,
			SessionKey: roundValue.SessionKey,
			Status:     protocol.GoalStatusComplete,
		},
	}
	service := &Service{goals: provider}
	accelerateRoomGoalUsageRetry(service)

	// 旧 worker 尚未清 flag，runRound 的 terminal start 因而先跳过。
	if !slot.tryStartGoalUsageRetry() {
		t.Fatal("failed to install the simulated old usage worker")
	}
	service.startRoomGoalUsageRetry(roundValue, slot)
	slot.setStatus("finished")

	// 旧 worker 的 defer 清 flag 后必须重新检查 terminal 状态并接棒。
	service.finishRoomGoalUsageRetryWorker(roundValue, slot)
	deadline := time.Now().Add(time.Second)
	for provider.finalizeCallCount() != 1 || roundValue.RunningSubagents.Load() {
		if time.Now().After(deadline) {
			t.Fatalf(
				"terminal handoff finalize_calls=%d barrier=%v",
				provider.finalizeCallCount(),
				roundValue.RunningSubagents.Load(),
			)
		}
		time.Sleep(time.Millisecond)
	}
}

type fakeRoomGoalUsageFinalizer struct {
	*fakeRoomGoalContextProvider
	report                 protocol.GoalUsageReport
	finalizeCalls          int
	beforeFinalize         func()
	claimFailuresRemaining int
	claimAttempts          int
	claimSuccesses         int
}

type retryingRoomGoalUsageSourceProvider struct {
	*fakeRoomGoalUsageFinalizer
	sourceMu                     sync.Mutex
	failuresRemaining            int
	parentUsageFailuresRemaining int
	parentUsageAttempts          int
	finalizeFailuresRemaining    int
	finalizeAttempts             int
	persistedTotal               int64
	persisted                    chan struct{}
	persistedOnce                sync.Once
	parentUsagePersisted         chan struct{}
	parentUsagePersistedOnce     sync.Once
}

func (p *retryingRoomGoalUsageSourceProvider) RecordUsageForGoal(
	ctx context.Context,
	goalID string,
	usage protocol.GoalUsage,
	roundID string,
) (*protocol.Goal, error) {
	p.sourceMu.Lock()
	p.parentUsageAttempts++
	if p.parentUsageFailuresRemaining > 0 {
		p.parentUsageFailuresRemaining--
		p.sourceMu.Unlock()
		return nil, errors.New("injected Room parent usage persistence failure")
	}
	p.sourceMu.Unlock()
	updated, err := p.fakeRoomGoalUsageFinalizer.RecordUsageForGoal(ctx, goalID, usage, roundID)
	if err == nil && p.parentUsagePersisted != nil {
		p.parentUsagePersistedOnce.Do(func() { close(p.parentUsagePersisted) })
	}
	return updated, err
}

func (p *retryingRoomGoalUsageSourceProvider) parentUsageAttemptCount() int {
	p.sourceMu.Lock()
	defer p.sourceMu.Unlock()
	return p.parentUsageAttempts
}

func (p *retryingRoomGoalUsageSourceProvider) RecordUsageSourceSnapshot(
	_ context.Context,
	snapshot protocol.GoalUsageSourceSnapshot,
) (protocol.GoalUsageSourceResult, error) {
	p.sourceMu.Lock()
	if p.failuresRemaining > 0 {
		p.failuresRemaining--
		p.sourceMu.Unlock()
		return protocol.GoalUsageSourceResult{}, errors.New("injected Room source persistence failure")
	}
	p.persistedTotal = snapshot.CumulativeActualTokens
	p.sourceMu.Unlock()
	p.persistedOnce.Do(func() { close(p.persisted) })
	return protocol.GoalUsageSourceResult{}, nil
}

func (p *retryingRoomGoalUsageSourceProvider) lastPersistedTotal() int64 {
	p.sourceMu.Lock()
	defer p.sourceMu.Unlock()
	return p.persistedTotal
}

func (p *retryingRoomGoalUsageSourceProvider) FinalizeUsageForGoal(
	ctx context.Context,
	goalID string,
	delta protocol.GoalUsage,
	roundID string,
) (*protocol.Goal, error) {
	p.sourceMu.Lock()
	p.finalizeAttempts++
	if p.finalizeFailuresRemaining > 0 {
		p.finalizeFailuresRemaining--
		p.sourceMu.Unlock()
		return nil, errors.New("injected Room Goal finalization failure")
	}
	p.sourceMu.Unlock()
	return p.fakeRoomGoalUsageFinalizer.FinalizeUsageForGoal(ctx, goalID, delta, roundID)
}

func (p *retryingRoomGoalUsageSourceProvider) finalizeAttemptCount() int {
	p.sourceMu.Lock()
	defer p.sourceMu.Unlock()
	return p.finalizeAttempts
}

func (p *fakeRoomGoalUsageFinalizer) UsageByGoalID(
	_ context.Context,
	_ string,
) (*protocol.GoalUsageReport, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	report := p.report
	return &report, nil
}

func (p *fakeRoomGoalUsageFinalizer) FinalizeUsageForGoal(
	_ context.Context,
	goalID string,
	delta protocol.GoalUsage,
	_ string,
) (*protocol.Goal, error) {
	p.mu.Lock()
	beforeFinalize := p.beforeFinalize
	p.mu.Unlock()
	if beforeFinalize != nil {
		beforeFinalize()
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.finalizeCalls++
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

func (p *fakeRoomGoalUsageFinalizer) ClaimUsageSourceRound(
	_ context.Context,
	_ protocol.GoalUsageSourceRoundClaim,
) (protocol.GoalUsageSourceResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.claimAttempts++
	if p.claimFailuresRemaining > 0 {
		p.claimFailuresRemaining--
		return protocol.GoalUsageSourceResult{}, errors.New("injected Room usage claim failure")
	}
	p.claimSuccesses++
	return protocol.GoalUsageSourceResult{}, nil
}

func (p *fakeRoomGoalUsageFinalizer) finalizeCallCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.finalizeCalls
}

func (p *fakeRoomGoalUsageFinalizer) claimAttemptCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.claimAttempts
}

func (p *fakeRoomGoalUsageFinalizer) claimSuccessCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.claimSuccesses
}
