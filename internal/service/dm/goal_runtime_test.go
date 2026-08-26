package dm

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/cli/runtimecommand"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	exec "github.com/nexus-research-lab/nexus/internal/runtime/exec"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

type dmSettlementBoundaryContext struct {
	context.Context
}

func (dmSettlementBoundaryContext) Value(any) any {
	return true
}

func TestRoundRunnerGoalMutationsUseBoundObjectiveRevision(t *testing.T) {
	for _, testCase := range []struct {
		name string
		run  func(*roundRunner)
	}{
		{
			name: "continuation failure",
			run: func(runner *roundRunner) {
				runner.inputOptions.Purpose = "goal_continuation"
				runner.recordGoalContinuationProgress(exec.RoundExecutionResult{TerminalStatus: "error", ErrorMessage: "runtime failed"})
			},
		},
		{
			name: "explicit activity",
			run: func(runner *roundRunner) {
				runner.recordGoalContinuationProgress(exec.RoundExecutionResult{})
			},
		},
		{
			name: "completion command miss",
			run: func(runner *roundRunner) {
				runner.inputOptions.Purpose = "goal_continuation"
				runner.rememberGoalAssistantMessage(goalCompletionCommandMissAssistantMessage())
				runner.recordGoalContinuationProgress(exec.RoundExecutionResult{})
			},
		},
		{
			name: "continuation progress",
			run: func(runner *roundRunner) {
				runner.inputOptions.Purpose = "goal_continuation"
				runner.recordGoalContinuationProgress(exec.RoundExecutionResult{})
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			provider := &fakeGoalContextProvider{}
			revision := &atomic.Int64{}
			revision.Store(7)
			runner := &roundRunner{
				service:               &Service{goals: provider},
				sessionKey:            "agent:nexus:ws:dm:revision",
				roundID:               "round-revision",
				goalIDForUsage:        "goal-revision",
				goalObjectiveRevision: revision,
			}

			testCase.run(runner)

			provider.mu.Lock()
			revisions := append([]int64(nil), provider.progressRevisions...)
			revisions = append(revisions, provider.failureRevisions...)
			revisions = append(revisions, provider.completionRevisions...)
			revisions = append(revisions, provider.activityRevisions...)
			provider.mu.Unlock()
			if !slices.Equal(revisions, []int64{7}) {
				t.Fatalf("mutation revisions = %v, want [7]", revisions)
			}
		})
	}
}

func TestTerminalRoundStatusEventCarriesRuntimeError(t *testing.T) {
	runner := &roundRunner{
		sessionKey:   "agent:nexus:ws:dm:terminal-error",
		roundID:      "round-terminal-error",
		agentRoundID: "agent-round-terminal-error",
		agent:        &protocol.Agent{AgentID: "nexus"},
	}
	event := terminalRoundStatusEvent(runner, exec.RoundExecutionResult{
		TerminalStatus: "error",
		ResultSubtype:  "error",
		ErrorMessage:   "query failed: provider unavailable",
	})
	if event.EventType != protocol.EventTypeRoundStatus {
		t.Fatalf("event type = %s, want round_status", event.EventType)
	}
	if event.Data["status"] != "error" || event.Data["result_subtype"] != "error" {
		t.Fatalf("event data = %#v, want error terminal status", event.Data)
	}
	if event.Data["message"] != "query failed: provider unavailable" {
		t.Fatalf("event message = %#v, want runtime error", event.Data["message"])
	}
	if event.AgentID != "nexus" || event.AgentRoundID != runner.agentRoundID {
		t.Fatalf("event identity = agent=%q agent_round=%q", event.AgentID, event.AgentRoundID)
	}
}

func TestDMFinalGoalUsageSnapshotPrefersExplicitZeroResultOverAssistantUsage(t *testing.T) {
	runner := &roundRunner{}
	assistant := goalAssistantUsageMessage(80, 20)

	snapshot, ok := runner.finalGoalUsageSnapshot(exec.RoundExecutionResult{
		Usage: sdkprotocol.TokenUsage{
			Raw: map[string]any{"total_tokens": int64(0)},
		},
	}, assistant)

	if !ok {
		t.Fatal("explicit provider zero was treated as missing usage")
	}
	if !snapshot.Cumulative || !snapshot.Terminal || !snapshot.TokenUsageObserved {
		t.Fatalf(
			"snapshot flags = cumulative:%v terminal:%v observed:%v, want true/true/true",
			snapshot.Cumulative,
			snapshot.Terminal,
			snapshot.TokenUsageObserved,
		)
	}
	if snapshot.Usage.ActualTokens() != 0 ||
		snapshot.Usage.BudgetTokens() != 0 ||
		snapshot.Usage.InputTokens != 0 ||
		snapshot.Usage.OutputTokens != 0 {
		t.Fatalf("snapshot usage = %#v, want authoritative result zero instead of assistant 100", snapshot.Usage)
	}
}

func TestDMFinalGoalUsageSnapshotDistinguishesMissingTokenUsage(t *testing.T) {
	runner := &roundRunner{
		goalUsageStarted: time.Now().Add(-2 * time.Second),
	}

	snapshot, ok := runner.finalGoalUsageSnapshot(exec.RoundExecutionResult{}, nil)
	if !ok {
		t.Fatal("elapsed-only terminal snapshot was dropped")
	}
	if snapshot.TokenUsageObserved {
		t.Fatalf("snapshot = %#v, want missing token usage", snapshot)
	}
	if snapshot.ElapsedSeconds <= 0 {
		t.Fatalf("elapsed = %d, want positive fallback", snapshot.ElapsedSeconds)
	}
}

func TestRoundRunnerRecordsGoalUsageAtToolCompletion(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: goalProvider},
		sessionKey:     "agent:nexus:ws:dm:test",
		roundID:        "round-1",
		goalIDForUsage: "goal-1",
		goalUsage:      goalsvc.NewRuntimeUsageAccumulator(true),
	}

	runner.recordGoalUsageFromAssistantMessage(goalToolResultAssistantMessage("tool-1", "read_file", false, 4, 3))
	runner.recordGoalUsage(context.Background(), exec.RoundExecutionResult{
		Usage: sdkprotocol.TokenUsage{
			InputTokens:  10,
			OutputTokens: 5,
			TotalTokens:  15,
		},
	}, nil)

	usages := goalProvider.recordedUsage()
	if len(usages) != 1 {
		t.Fatalf("len(usages) = %d, want one terminal settlement", len(usages))
	}
	if usages[0].InputTokens != 10 || usages[0].OutputTokens != 5 ||
		usages[0].BudgetTokens() != 15 || usages[0].ActualTokens() != 15 {
		t.Fatalf("terminal usage = %#v, want cumulative 10/5", usages[0])
	}
}

func TestDMGoalProgressRequiresConfirmedGoalExecutionAuthority(t *testing.T) {
	message := goalToolResultAssistantMessage(
		"tool-workgraph",
		"Bash",
		false,
		4,
		1,
	)
	content := message["content"].([]map[string]any)
	content[1]["content"] = `{"outcome":"applied"}`

	goalOnly := runtimectx.NewResponsibilityAuthorityState(
		runtimectx.NewGoalAuthorityState("goal-1", 1, ""),
		"",
		nil,
		nil,
	)
	runner := &roundRunner{
		service:               &Service{goals: &fakeGoalContextProvider{}},
		goalIDForUsage:        "goal-1",
		goalObjectiveRevision: func() *atomic.Int64 { value := &atomic.Int64{}; value.Store(1); return value }(),
		responsibilityState:   goalOnly,
	}
	stageDMRuntimeCommandReceipt(runner, runtimecommand.Receipt{
		Domain: runtimecommand.DomainExecution, Operation: "submit_work",
		Outcome: string(protocol.MutationResultApplied), GoalBound: false,
	})
	runner.recordGoalUsageFromAssistantMessage(message)
	if runner.hasGoalToolProgress() {
		t.Fatal("Goal-only authority counted an unrelated WorkGraph mutation")
	}

	bound := runtimectx.NewResponsibilityAuthorityState(
		runtimectx.NewGoalAuthorityState("goal-1", 1, "execution-1"),
		"execution-1",
		nil,
		nil,
	)
	runner.responsibilityState = bound
	stageDMRuntimeCommandReceipt(runner, runtimecommand.Receipt{
		Domain: runtimecommand.DomainExecution, Operation: "submit_work",
		Outcome: string(protocol.MutationResultApplied), GoalBound: true,
	})
	runner.recordGoalUsageFromAssistantMessage(message)
	if !runner.hasGoalToolProgress() {
		t.Fatal("confirmed Goal-bound Execution mutation was not counted")
	}
}

func TestRoundRunnerRecordsAbortGoalUsageFromAssistantSnapshot(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: goalProvider},
		sessionKey:     "agent:nexus:ws:dm:test",
		roundID:        "round-1",
		goalIDForUsage: "goal-1",
		goalUsage:      goalsvc.NewRuntimeUsageAccumulator(true),
	}

	runner.recordGoalUsageFromAssistantMessage(goalToolResultAssistantMessage("tool-1", "read_file", false, 4, 1))
	runner.recordGoalUsage(context.Background(), exec.RoundExecutionResult{}, goalAssistantUsageMessage(7, 3))

	usages := goalProvider.recordedUsage()
	if len(usages) != 1 {
		t.Fatalf("len(usages) = %d, want one terminal settlement", len(usages))
	}
	if usages[0].InputTokens != 11 || usages[0].OutputTokens != 4 || usages[0].Total() != 15 {
		t.Fatalf("abort usage = %#v, want both observed turns 11/4", usages[0])
	}
}

func TestRoundRunnerMidRoundFlushDefersEstimatedActualUntilLowerExactTerminal(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: goalProvider},
		sessionKey:     "agent:nexus:ws:dm:estimated-checkpoint",
		roundID:        "round-estimated-checkpoint",
		goalIDForUsage: "goal-estimated-checkpoint",
		goalUsage:      goalsvc.NewRuntimeUsageAccumulator(true),
	}
	runner.rememberGoalAssistantMessage(protocol.Message{
		"message_id": "assistant-estimated-checkpoint",
		"role":       "assistant",
		"usage": map[string]any{
			"input_tokens":  int64(150),
			"output_tokens": int64(50),
		},
	})

	if err := runner.flushGoalUsage(context.Background()); err != nil {
		t.Fatalf("flushGoalUsage() error = %v", err)
	}
	usages := goalProvider.recordedUsage()
	if len(usages) != 0 {
		t.Fatalf("checkpoint usages = %#v, want no token persistence before terminal", usages)
	}

	runner.finalizeGoalUsage(context.Background(), exec.RoundExecutionResult{
		Usage: sdkprotocol.TokenUsage{
			InputTokens:  150,
			OutputTokens: 50,
			TotalTokens:  180,
		},
	}, nil)

	usages = goalProvider.recordedUsage()
	if len(usages) != 1 {
		t.Fatalf("terminal usages = %#v, want one terminal reconciliation", usages)
	}
	if usages[0].BudgetTokens() != 200 ||
		usages[0].ActualTokens() != 180 ||
		usages[0].ActualTokensAreEstimated() {
		t.Fatalf("reconciled usage = %#v, want budget 200 and exact actual 180", usages[0])
	}
}

func TestRoundRunnerSettlementBoundaryFlushesDeferredActualBeforeClear(t *testing.T) {
	for _, test := range []struct {
		name      string
		estimated bool
	}{
		{name: "provider actual"},
		{name: "estimated fallback", estimated: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			goalProvider := &fakeGoalContextProvider{}
			runner := &roundRunner{
				service:        &Service{goals: goalProvider},
				sessionKey:     "agent:nexus:ws:dm:settlement-boundary",
				roundID:        "round-settlement-boundary",
				goalIDForUsage: "goal-settlement-boundary",
				goalUsage:      goalsvc.NewRuntimeUsageAccumulator(true),
			}
			usage := map[string]any{
				"input_tokens":  int64(90),
				"output_tokens": int64(10),
			}
			if !test.estimated {
				usage["total_tokens"] = int64(100)
			}
			assistant := protocol.Message{
				"message_id": "assistant-settlement-boundary",
				"role":       "assistant",
				"usage":      usage,
			}
			runner.rememberGoalAssistantMessage(assistant)
			runner.recordGoalUsageFromAssistantMessage(assistant)

			if err := runner.flushGoalUsage(dmSettlementBoundaryContext{Context: context.Background()}); err != nil {
				t.Fatalf("settlement-boundary flush error = %v", err)
			}
			runner.clearGoalUsage()
			// Provider terminal arrives after the external clear. It must not
			// reopen accounting or attribute any later usage to another Goal.
			runner.recordGoalUsage(context.Background(), exec.RoundExecutionResult{
				Usage: sdkprotocol.TokenUsage{
					InputTokens:  105,
					OutputTokens: 15,
					TotalTokens:  120,
				},
			}, nil)

			usages := goalProvider.recordedUsage()
			if len(usages) != 1 {
				t.Fatalf("settlement-boundary usages = %#v, want one old-Goal settlement", usages)
			}
			if usages[0].BudgetTokens() != 100 ||
				usages[0].ActualTokens() != 100 ||
				usages[0].ActualTokensAreEstimated() != test.estimated {
				t.Fatalf("settlement-boundary total = %#v, want budget/actual 100 with estimated=%v", usages[0], test.estimated)
			}
			if !slices.Equal(goalProvider.usageGoalIDs, []string{"goal-settlement-boundary"}) {
				t.Fatalf("usage Goal IDs = %#v, want settlement fixed to the cleared Goal", goalProvider.usageGoalIDs)
			}
		})
	}
}

func TestRoundRunnerSettlementBoundaryFlushesDeferredUsageWithEmptySnapshot(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: goalProvider},
		sessionKey:     "agent:nexus:ws:dm:empty-settlement-boundary",
		roundID:        "round-empty-settlement-boundary",
		goalIDForUsage: "goal-empty-settlement-boundary",
		goalUsage:      goalsvc.NewRuntimeUsageAccumulator(true),
	}
	assistant := protocol.Message{
		"message_id": "assistant-empty-settlement-boundary",
		"role":       "assistant",
		"usage": map[string]any{
			"input_tokens":  int64(90),
			"output_tokens": int64(10),
			"total_tokens":  int64(100),
		},
	}
	// Usage 已进入 accumulator，但 runtime 尚未交付可作为 flush 快照的
	// last assistant。显式 settlement boundary 仍必须把 deferred usage 落库。
	runner.recordGoalUsageFromAssistantMessage(assistant)

	if err := runner.flushGoalUsage(dmSettlementBoundaryContext{Context: context.Background()}); err != nil {
		t.Fatalf("empty settlement-boundary flush error = %v", err)
	}
	usages := goalProvider.recordedUsage()
	if len(usages) != 1 || usages[0].BudgetTokens() != 100 || usages[0].ActualTokens() != 100 {
		t.Fatalf("empty-boundary usages = %#v, want one deferred 100-token settlement", usages)
	}
}

func TestDMRegisterRunnerWiresGoalFinalizingUntilRoundCleanup(t *testing.T) {
	const (
		sessionKey = "agent:nexus:ws:dm:finalizing-hook"
		roundID    = "round-finalizing-hook"
	)
	manager := runtimectx.NewManager()
	_ = manager.StartRound(context.Background(), sessionKey, roundID, func() {})
	accumulator := goalsvc.NewRuntimeUsageAccumulator(true)
	revision := &atomic.Int64{}
	provider := &fakeDMGoalUsageFinalizer{
		fakeGoalContextProvider: &fakeGoalContextProvider{},
		report: protocol.GoalUsageReport{
			GoalID:     "goal-finalizing-hook",
			SessionKey: sessionKey,
			Status:     protocol.GoalStatusComplete,
		},
	}
	service := &Service{
		runtime: manager,
		goals:   provider,
	}
	runner := &roundRunner{
		service:               service,
		sessionKey:            sessionKey,
		roundID:               roundID,
		goalIDForUsage:        "goal-finalizing-hook",
		goalUsage:             accumulator,
		goalObjectiveRevision: revision,
	}
	execution := &dmChatExecution{
		service:    runner.service,
		ctx:        context.Background(),
		sessionKey: sessionKey,
		agent:      &protocol.Agent{AgentID: "nexus"},
		request:    Request{RoundID: roundID},
		runner:     runner,
	}

	execution.registerRunner()
	if rounds := manager.BeginGoalAccountingFinalizing(sessionKey); !slices.Equal(rounds, []string{roundID}) {
		t.Fatalf("finalizing hook rounds = %#v, want [%s]", rounds, roundID)
	}
	if !accumulator.Active() {
		t.Fatal("external complete hook did not keep DM Goal accounting active for terminal reconciliation")
	}
	runner.finalizeGoalUsage(context.Background(), exec.RoundExecutionResult{
		Usage: sdkprotocol.TokenUsage{
			InputTokens:  12,
			OutputTokens: 3,
			TotalTokens:  15,
		},
	}, nil)
	if calls := provider.finalizeCallCount(); calls != 1 {
		t.Fatalf("external complete finalization calls = %d, want 1", calls)
	}
	provider.mu.Lock()
	finalized := provider.report.UsageFinalized
	finalUsage := provider.report.Usage
	provider.mu.Unlock()
	if !finalized || finalUsage.ActualTokens() != 15 {
		t.Fatalf("external complete report = finalized:%v usage:%#v, want finalized actual 15", finalized, finalUsage)
	}
	runner.goalUsageMu.Lock()
	binding := runner.goalIDForUsage
	runner.goalUsageMu.Unlock()
	if binding != "goal-finalizing-hook" {
		t.Fatalf("Goal binding after external complete terminal = %q, want fixed original Goal", binding)
	}

	manager.MarkRoundFinished(sessionKey, roundID)
	if rounds := manager.BeginGoalAccountingFinalizing(sessionKey); len(rounds) != 0 {
		t.Fatalf("finalizing hooks after round cleanup = %#v, want none", rounds)
	}
}

func TestDMRegisterRunnerGuardsConsumedScopeUntilRoundFinished(t *testing.T) {
	const (
		sessionKey = "agent:nexus:ws:dm:create-guard"
		roundID    = "round-create-guard"
		goalID     = "goal-existing"
	)
	manager := runtimectx.NewManager()
	_ = manager.StartRound(context.Background(), sessionKey, roundID, func() {})
	provider := &fakeDMGoalUsageFinalizer{
		fakeGoalContextProvider: &fakeGoalContextProvider{},
		report: protocol.GoalUsageReport{
			GoalID:     goalID,
			SessionKey: sessionKey,
			Status:     protocol.GoalStatusComplete,
		},
	}
	runner := &roundRunner{
		service:               &Service{runtime: manager, goals: provider},
		sessionKey:            sessionKey,
		roundID:               roundID,
		goalIDForUsage:        goalID,
		childGoalIDForUsage:   goalID,
		goalUsage:             goalsvc.NewRuntimeUsageAccumulator(true),
		goalObjectiveRevision: &atomic.Int64{},
	}
	execution := &dmChatExecution{
		service:    runner.service,
		ctx:        context.Background(),
		sessionKey: sessionKey,
		agent:      &protocol.Agent{AgentID: "nexus"},
		request:    Request{RoundID: roundID},
		runner:     runner,
	}
	execution.registerRunner()

	if rounds := manager.BeginGoalAccountingFinalizing(sessionKey); !slices.Equal(rounds, []string{roundID}) {
		t.Fatalf("finalizing rounds = %#v, want [%s]", rounds, roundID)
	}
	if conflicts := manager.GoalAccountingCreateConflicts(sessionKey, ""); !slices.Equal(conflicts, []string{roundID}) {
		t.Fatalf("external create conflicts = %#v, want live consumed round", conflicts)
	}
	if conflicts := manager.GoalAccountingCreateConflicts(sessionKey, roundID); !slices.Equal(conflicts, []string{roundID}) {
		t.Fatalf("model create conflicts = %#v, want same consumed scope", conflicts)
	}
	runner.goalUsageMu.Lock()
	binding := runner.goalIDForUsage
	runner.goalUsageMu.Unlock()
	if binding != goalID {
		t.Fatalf("preflight changed old Goal binding to %q, want %q", binding, goalID)
	}

	runner.clearGoalUsage()
	if conflicts := manager.GoalAccountingCreateConflicts(sessionKey, roundID); !slices.Equal(conflicts, []string{roundID}) {
		t.Fatalf("clear reset consumed guard: %#v", conflicts)
	}
	manager.MarkRoundFinished(sessionKey, roundID)
	if conflicts := manager.GoalAccountingCreateConflicts(sessionKey, ""); len(conflicts) != 0 {
		t.Fatalf("finished historical round still blocks create: %#v", conflicts)
	}
}

func TestDMGoalCreateGuardBecomesConsumedAfterExternalAndModelActivation(t *testing.T) {
	for _, test := range []struct {
		name     string
		activate func(*roundRunner, *runtimectx.Manager, string)
	}{
		{
			name: "external activation",
			activate: func(runner *roundRunner, manager *runtimectx.Manager, sessionKey string) {
				activated, err := manager.ActivateGoalAccounting(
					context.Background(),
					sessionKey,
					"goal-external",
				)
				if err != nil || !slices.Equal(activated, []string{runner.roundID}) {
					t.Fatalf("external activation = %#v, err=%v", activated, err)
				}
			},
		},
		{
			name: "model create",
			activate: func(runner *roundRunner, _ *runtimectx.Manager, _ string) {
				stageDMAppliedGoalCommand(runner, runtimecommand.GoalOperationCreate, "goal-model", "")
				runner.recordGoalUsageFromAssistantMessage(
					goalToolResultAssistantMessage("tool-create", "create_goal", false, 2, 1),
				)
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			sessionKey := "agent:nexus:ws:dm:create-guard-" + strings.ReplaceAll(test.name, " ", "-")
			roundID := "round-create-guard-" + strings.ReplaceAll(test.name, " ", "-")
			manager := runtimectx.NewManager()
			_ = manager.StartRound(context.Background(), sessionKey, roundID, func() {})
			provider := &fakeGoalContextProvider{
				runtimeGoal: &protocol.Goal{ID: "goal-model", SessionKey: sessionKey},
			}
			runner := &roundRunner{
				service:               &Service{runtime: manager, goals: provider},
				sessionKey:            sessionKey,
				roundID:               roundID,
				goalUsage:             goalsvc.NewRuntimeUsageAccumulator(false),
				goalObjectiveRevision: &atomic.Int64{},
			}
			execution := &dmChatExecution{
				service:    runner.service,
				ctx:        context.Background(),
				sessionKey: sessionKey,
				agent:      &protocol.Agent{AgentID: "nexus"},
				request:    Request{RoundID: roundID},
				runner:     runner,
			}
			execution.registerRunner()
			if conflicts := manager.GoalAccountingCreateConflicts(sessionKey, roundID); len(conflicts) != 0 {
				t.Fatalf("fresh scope conflicts = %#v, want none", conflicts)
			}

			test.activate(runner, manager, sessionKey)
			runner.clearGoalUsage()
			if conflicts := manager.GoalAccountingCreateConflicts(sessionKey, roundID); !slices.Equal(conflicts, []string{roundID}) {
				t.Fatalf("activation/clear consumed conflicts = %#v, want [%s]", conflicts, roundID)
			}
			manager.MarkRoundFinished(sessionKey, roundID)
		})
	}
}

func TestDMExternalActivationDurableBindFailureKeepsOldBindingAndBaseline(t *testing.T) {
	bindConflict := errors.New("durable scope already bound")
	provider := &fakeDMScopeBindingGoalProvider{
		fakeGoalContextProvider: &fakeGoalContextProvider{},
		bindErr:                 bindConflict,
	}
	runner := &roundRunner{
		service:                &Service{goals: provider},
		ownerUserID:            "owner-bind-conflict",
		sessionKey:             "agent:nexus:ws:dm:bind-conflict",
		roundID:                "round-bind-conflict",
		goalIDForUsage:         "goal-old",
		childGoalIDForUsage:    "goal-old",
		goalUsage:              goalsvc.NewRuntimeUsageAccumulator(true),
		goalUsageScopeConsumed: true,
	}
	accelerateDMGoalUsageRetry(runner)
	runner.recordGoalUsageFromAssistantMessage(
		goalToolResultAssistantMessage("tool-old", "read_file", false, 4, 1),
	)

	err := runner.activateGoalUsage(context.Background(), "goal-new")
	if !errors.Is(err, bindConflict) {
		t.Fatalf("activateGoalUsage() error = %v, want durable bind conflict", err)
	}
	runner.goalUsageMu.Lock()
	binding := runner.goalIDForUsage
	consumed := runner.goalUsageScopeConsumed
	runner.goalUsageMu.Unlock()
	if binding != "goal-old" || !consumed {
		t.Fatalf("binding/consumed = %q/%v, want old Goal retained and scope consumed", binding, consumed)
	}
	if err := runner.flushGoalUsage(dmSettlementBoundaryContext{Context: context.Background()}); err != nil {
		t.Fatal(err)
	}
	usages := provider.recordedUsage()
	if len(usages) != 1 || usages[0].ActualTokens() != 5 ||
		len(provider.usageGoalIDs) != 1 || provider.usageGoalIDs[0] != "goal-old" {
		t.Fatalf("post-conflict usage = %#v targets=%#v, want old baseline attributed to old Goal", usages, provider.usageGoalIDs)
	}

	provider.bindErr = goalsvc.ErrGoalInvalidState
	if err := runner.activateGoalUsage(context.Background(), "goal-new"); err != nil {
		t.Fatalf("capability fallback activation error = %v", err)
	}
	if runner.goalIDForUsage != "goal-new" {
		t.Fatalf("capability fallback binding = %q, want goal-new", runner.goalIDForUsage)
	}
}

func TestDMExternalActivationBindConflictBeforeModelResultKeepsScopeUnconsumed(t *testing.T) {
	const (
		sessionKey = "agent:nexus:ws:dm:model-bind-window"
		roundID    = "round-model-bind-window"
		modelGoal  = "goal-model-created"
	)
	bindConflict := errors.New("scope already belongs to model Goal")
	model := &protocol.Goal{ID: modelGoal, SessionKey: sessionKey}
	provider := &fakeDMScopeBindingGoalProvider{
		fakeGoalContextProvider: &fakeGoalContextProvider{
			runtimeGoal: model,
			usageGoal:   model,
		},
		bindErr: bindConflict,
	}
	runner := &roundRunner{
		service:     &Service{goals: provider},
		ownerUserID: "owner-model-bind-window",
		sessionKey:  sessionKey,
		roundID:     roundID,
		goalUsage:   goalsvc.NewRuntimeUsageAccumulator(false),
		runtimeKind: "nxs",
	}
	accelerateDMGoalUsageRetry(runner)
	runner.recordGoalUsageFromAssistantMessage(
		goalToolResultAssistantMessage("tool-before-create", "read_file", false, 4, 1),
	)

	if err := runner.activateGoalUsage(context.Background(), "goal-external"); !errors.Is(err, bindConflict) {
		t.Fatalf("external activation error = %v, want durable model-scope conflict", err)
	}
	runner.goalUsageMu.Lock()
	binding := runner.goalIDForUsage
	consumed := runner.goalUsageScopeConsumed
	active := runner.goalUsage.Active()
	runner.goalUsageMu.Unlock()
	if binding != "" || consumed || active {
		t.Fatalf(
			"failed external bind mutated pre-result scope: binding=%q consumed=%v active=%v",
			binding,
			consumed,
			active,
		)
	}

	// The original model create result can still claim the untouched round-start
	// baseline and becomes the scope's one consumed Goal.
	stageDMAppliedGoalCommand(runner, runtimecommand.GoalOperationCreate, modelGoal, "")
	runner.recordGoalUsageFromAssistantMessage(
		goalToolResultAssistantMessage("tool-create", "create_goal", false, 5, 1),
	)
	runner.goalUsageMu.Lock()
	binding = runner.goalIDForUsage
	consumed = runner.goalUsageScopeConsumed
	runner.goalUsageMu.Unlock()
	if binding != modelGoal || !consumed {
		t.Fatalf("model result binding/consumed = %q/%v, want %q/true", binding, consumed, modelGoal)
	}
}

func TestDMGoalFinalizingHookDeclinesIgnoredOrUnboundRound(t *testing.T) {
	for _, test := range []struct {
		name           string
		goalID         string
		active         bool
		permissionMode sdkpermission.Mode
		withProvider   bool
		withFinalizer  bool
	}{
		{
			name:          "no Goal binding",
			withFinalizer: true,
		},
		{
			name:           "ignored plan mode",
			goalID:         "goal-ignored-finalizing",
			active:         true,
			permissionMode: sdkpermission.ModePlan,
			withFinalizer:  true,
		},
		{
			name:   "no Goal provider",
			goalID: "goal-no-provider-finalizing",
			active: true,
		},
		{
			name:         "provider lacks finalization",
			goalID:       "goal-nonfinalizing-provider",
			active:       true,
			withProvider: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			const (
				sessionKey = "agent:nexus:ws:dm:declined-finalizing-hook"
				roundID    = "round-declined-finalizing-hook"
			)
			manager := runtimectx.NewManager()
			_ = manager.StartRound(context.Background(), sessionKey, roundID, func() {})
			service := &Service{runtime: manager}
			if test.withFinalizer {
				service.goals = &fakeDMGoalUsageFinalizer{
					fakeGoalContextProvider: &fakeGoalContextProvider{},
					report: protocol.GoalUsageReport{
						GoalID:     test.goalID,
						SessionKey: sessionKey,
						Status:     protocol.GoalStatusComplete,
					},
				}
			} else if test.withProvider {
				service.goals = &fakeGoalContextProvider{}
			}
			runner := &roundRunner{
				service:        service,
				sessionKey:     sessionKey,
				roundID:        roundID,
				goalIDForUsage: test.goalID,
				goalUsage:      goalsvc.NewRuntimeUsageAccumulator(test.active),
				permissionMode: test.permissionMode,
			}
			execution := &dmChatExecution{
				service:    service,
				ctx:        context.Background(),
				sessionKey: sessionKey,
				agent:      &protocol.Agent{AgentID: "nexus"},
				request:    Request{RoundID: roundID},
				runner:     runner,
			}

			execution.registerRunner()
			if rounds := manager.BeginGoalAccountingFinalizing(sessionKey); len(rounds) != 0 {
				t.Fatalf("declined finalizing hook rounds = %#v, want none", rounds)
			}
			manager.MarkRoundFinished(sessionKey, roundID)
		})
	}
}

func TestRoundRunnerMarksUsageLimitAfterAccounting(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: goalProvider},
		sessionKey:     "agent:nexus:ws:dm:test",
		roundID:        "round-1",
		goalIDForUsage: "goal-1",
		goalUsage:      goalsvc.NewRuntimeUsageAccumulator(true),
	}

	runner.recordGoalUsage(context.Background(), exec.RoundExecutionResult{
		Usage: sdkprotocol.TokenUsage{
			InputTokens:  3,
			OutputTokens: 2,
			TotalTokens:  5,
		},
		UsageLimitReached: true,
		UsageLimitReason:  "You've hit your usage limit.",
	}, nil)
	runner.recordGoalUsageLimit(exec.RoundExecutionResult{
		UsageLimitReached: true,
		UsageLimitReason:  "You've hit your usage limit.",
	})

	usages := goalProvider.recordedUsage()
	if len(usages) != 1 || usages[0].Total() != 5 {
		t.Fatalf("usages = %#v, want usage recorded before limit", usages)
	}
	reasons := goalProvider.recordedUsageLimitReasons()
	if len(reasons) != 1 || reasons[0] != "You've hit your usage limit." {
		t.Fatalf("usage limit reasons = %#v, want runtime reason", reasons)
	}
}

func TestRoundRunnerRecordsEmptyGoalContinuationProgress(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: goalProvider},
		sessionKey:     "agent:nexus:ws:dm:test",
		roundID:        "goal_continuation_1",
		goalIDForUsage: "goal-1",
		inputOptions: sdkprotocol.OutboundMessageOptions{
			Purpose: "goal_continuation",
		},
	}

	runner.recordGoalContinuationProgress(exec.RoundExecutionResult{})

	progress := goalProvider.recordedProgress()
	if len(progress) != 1 || progress[0] {
		t.Fatalf("progress = %#v, want one false continuation progress", progress)
	}
}

func TestRoundRunnerSkipsEmptyGoalContinuationProgressWhileSubagentRuns(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: goalProvider},
		sessionKey:     "agent:nexus:ws:dm:test",
		roundID:        "goal_continuation_1",
		goalIDForUsage: "goal-1",
		subagentTasks:  map[string]struct{}{"task-1": {}},
		inputOptions: sdkprotocol.OutboundMessageOptions{
			Purpose: "goal_continuation",
		},
	}

	runner.recordGoalContinuationProgress(exec.RoundExecutionResult{})

	if progress := goalProvider.recordedProgress(); len(progress) != 0 {
		t.Fatalf("progress = %#v, want running subagent to defer empty continuation progress", progress)
	}
	goalProvider.mu.Lock()
	settledCalls := goalProvider.settledCalls
	goalProvider.mu.Unlock()
	if settledCalls != 1 {
		t.Fatalf("settledCalls = %d, want runtime terminal to settle launch receipt while subagent work remains pending", settledCalls)
	}
}

func TestRoundRunnerRecordsGoalContinuationFailure(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: goalProvider},
		sessionKey:     "agent:nexus:ws:dm:test",
		roundID:        "goal_continuation_1",
		goalIDForUsage: "goal-1",
		inputOptions: sdkprotocol.OutboundMessageOptions{
			Purpose: "goal_continuation",
		},
	}

	runner.recordGoalContinuationProgress(exec.RoundExecutionResult{
		TerminalStatus: "error",
		ResultSubtype:  "error",
		ErrorMessage:   "Failed to authenticate. API Error: 401",
	})

	failures := goalProvider.recordedFailures()
	if len(failures) != 1 || failures[0] != "Failed to authenticate. API Error: 401" {
		t.Fatalf("failures = %#v, want provider error", failures)
	}
	if progress := goalProvider.recordedProgress(); len(progress) != 0 {
		t.Fatalf("progress = %#v, want failure path instead of empty progress", progress)
	}
}

func TestRoundRunnerOrdinaryToolDoesNotFakeGoalContinuationProgress(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: goalProvider},
		sessionKey:     "agent:nexus:ws:dm:test",
		roundID:        "goal_continuation_1",
		goalIDForUsage: "goal-1",
		goalUsage:      goalsvc.NewRuntimeUsageAccumulator(true),
		inputOptions: sdkprotocol.OutboundMessageOptions{
			Purpose: "goal_continuation",
		},
	}

	runner.recordGoalUsageFromAssistantMessage(goalToolResultAssistantMessage("tool-1", "read_file", false, 4, 1))
	runner.recordGoalContinuationProgress(exec.RoundExecutionResult{})

	progress := goalProvider.recordedProgress()
	if len(progress) != 1 || progress[0] {
		t.Fatalf("progress = %#v, want ordinary read to record empty progress", progress)
	}
}

func TestRoundRunnerRecordsGoalCompletionCommandMiss(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: goalProvider},
		sessionKey:     "agent:nexus:ws:dm:test",
		roundID:        "goal_continuation_1",
		goalIDForUsage: "goal-1",
		inputOptions: sdkprotocol.OutboundMessageOptions{
			Purpose: "goal_continuation",
		},
	}
	runner.rememberGoalAssistantMessage(goalCompletionCommandMissAssistantMessage())

	runner.recordGoalContinuationProgress(exec.RoundExecutionResult{})

	misses := goalProvider.recordedCompletionMisses()
	if len(misses) != 1 || !strings.Contains(misses[0], "nexus_runtime.command update_goal receipt") {
		t.Fatalf("completion misses = %#v, want one missing update_goal record", misses)
	}
	if progress := goalProvider.recordedProgress(); len(progress) != 0 {
		t.Fatalf("progress = %#v, want completion miss path instead of empty progress", progress)
	}
}

func TestRoundRunnerRecordsUserGoalActivityInsteadOfContinuationProgress(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: goalProvider},
		sessionKey:     "agent:nexus:ws:dm:test",
		roundID:        "round-user",
		goalIDForUsage: "goal-1",
	}

	runner.recordGoalContinuationProgress(exec.RoundExecutionResult{})

	goalProvider.mu.Lock()
	defer goalProvider.mu.Unlock()
	if len(goalProvider.activities) != 1 || goalProvider.activities[0] != "round-user" {
		t.Fatalf("activities = %#v, want explicit goal activity", goalProvider.activities)
	}
	if len(goalProvider.progress) != 0 {
		t.Fatalf("progress = %#v, want no continuation progress for user round", goalProvider.progress)
	}
}

func TestRoundRunnerFinalizesGoalUsageAfterUpdateGoal(t *testing.T) {
	t.Run("update_goal command", func(t *testing.T) {
		goalProvider := &fakeGoalContextProvider{}
		runner := &roundRunner{
			service:        &Service{goals: goalProvider},
			sessionKey:     "agent:nexus:ws:dm:test",
			roundID:        "round-1",
			goalIDForUsage: "goal-1",
			goalUsage:      goalsvc.NewRuntimeUsageAccumulator(true),
		}

		stageDMAppliedGoalCommand(runner, runtimecommand.GoalOperationUpdate, "goal-1", protocol.GoalStatusComplete)
		runner.recordGoalUsageFromAssistantMessage(goalToolResultAssistantMessage("tool-1", "Bash", false, 10, 2))
		runner.finalizeGoalUsage(context.Background(), exec.RoundExecutionResult{
			Usage: sdkprotocol.TokenUsage{
				InputTokens:  20,
				OutputTokens: 5,
				TotalTokens:  25,
			},
		}, nil)
		runner.recordGoalUsage(context.Background(), exec.RoundExecutionResult{
			Usage: sdkprotocol.TokenUsage{
				InputTokens:  30,
				OutputTokens: 6,
				TotalTokens:  36,
			},
		}, nil)

		usages := goalProvider.recordedUsage()
		if len(usages) != 1 {
			t.Fatalf("len(usages) = %d, want one terminal settlement", len(usages))
		}
		if usages[0].InputTokens != 20 || usages[0].OutputTokens != 5 ||
			usages[0].BudgetTokens() != 25 || usages[0].ActualTokens() != 25 {
			t.Fatalf("terminal usage = %#v, want cumulative 20/5", usages[0])
		}
	})
}

func TestRoundRunnerClearGoalUsageStopsLaterAccounting(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: goalProvider},
		sessionKey:     "agent:nexus:ws:dm:test",
		roundID:        "round-1",
		goalIDForUsage: "goal-1",
		goalUsage:      goalsvc.NewRuntimeUsageAccumulator(true),
	}

	runner.clearGoalUsage()
	runner.recordGoalUsage(context.Background(), exec.RoundExecutionResult{
		Usage: sdkprotocol.TokenUsage{
			InputTokens:  20,
			OutputTokens: 5,
			TotalTokens:  25,
		},
	}, nil)

	if usages := goalProvider.recordedUsage(); len(usages) != 0 {
		t.Fatalf("usages = %#v, want none after clear", usages)
	}
}

func TestRoundRunnerActivateSameGoalPreservesLowerExactTerminalCalibration(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:             &Service{goals: goalProvider},
		sessionKey:          "agent:nexus:ws:dm:same-goal-activate",
		roundID:             "round-same-goal-activate",
		goalIDForUsage:      "goal-same",
		childGoalIDForUsage: "goal-same",
		goalUsage:           goalsvc.NewRuntimeUsageAccumulator(true),
	}
	assistant := protocol.Message{
		"message_id": "assistant-same-goal",
		"role":       "assistant",
		"usage": map[string]any{
			"input_tokens":  int64(150),
			"output_tokens": int64(50),
		},
	}
	runner.rememberGoalAssistantMessage(assistant)
	runner.recordGoalUsageFromAssistantMessage(assistant)

	if err := runner.activateGoalUsage(context.Background(), "goal-same"); err != nil {
		t.Fatal(err)
	}
	runner.finalizeGoalUsage(context.Background(), exec.RoundExecutionResult{
		Usage: sdkprotocol.TokenUsage{
			InputTokens:  150,
			OutputTokens: 50,
			TotalTokens:  180,
		},
	}, nil)

	usages := goalProvider.recordedUsage()
	if len(usages) != 1 {
		t.Fatalf("same-goal usages = %#v, want one terminal settlement", usages)
	}
	if usages[0].BudgetTokens() != 200 ||
		usages[0].ActualTokens() != 180 ||
		usages[0].ActualTokensAreEstimated() {
		t.Fatalf("same-goal total = %#v, want budget 200 and lower exact terminal actual 180", usages[0])
	}
}

func TestRoundRunnerActivateGoalUsageRestartsFromCurrentSnapshot(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: goalProvider},
		sessionKey:     "agent:nexus:ws:dm:test",
		roundID:        "round-1",
		goalIDForUsage: "goal-old",
		goalUsage:      goalsvc.NewRuntimeUsageAccumulator(true),
	}

	oldAssistant := goalToolResultAssistantMessage("tool-1", "read_file", false, 4, 1)
	runner.rememberGoalAssistantMessage(oldAssistant)
	runner.recordGoalUsageFromAssistantMessage(oldAssistant)
	if err := runner.flushGoalUsage(dmSettlementBoundaryContext{Context: context.Background()}); err != nil {
		t.Fatalf("old Goal settlement error = %v", err)
	}
	runner.clearGoalUsage()
	runner.rememberGoalAssistantMessage(goalToolResultAssistantMessage("tool-2", "read_file", false, 7, 3))
	if err := runner.activateGoalUsage(context.Background(), "goal-new"); err != nil {
		t.Fatal(err)
	}
	runner.recordGoalUsage(context.Background(), exec.RoundExecutionResult{
		Usage: sdkprotocol.TokenUsage{
			InputTokens:  14,
			OutputTokens: 6,
			TotalTokens:  20,
		},
	}, nil)

	usages := goalProvider.recordedUsage()
	if len(usages) != 2 {
		t.Fatalf("len(usages) = %d, want initial usage and post-activate delta", len(usages))
	}
	if usages[1].InputTokens != 3 || usages[1].OutputTokens != 2 || usages[1].Total() != 5 {
		t.Fatalf("post-activate usage = %#v, want 3/2", usages[1])
	}
	if len(goalProvider.usageGoalIDs) != 2 ||
		goalProvider.usageGoalIDs[0] != "goal-old" ||
		goalProvider.usageGoalIDs[1] != "goal-new" {
		t.Fatalf("usage Goal IDs = %#v, want external activation rebound to goal-new", goalProvider.usageGoalIDs)
	}
}

func TestRoundRunnerResetsGoalUsageAfterCreateGoal(t *testing.T) {
	t.Run("create_goal command", func(t *testing.T) {
		goalProvider := &fakeGoalContextProvider{}
		runner := &roundRunner{
			service:    &Service{goals: goalProvider},
			sessionKey: "agent:nexus:ws:dm:test",
			roundID:    "round-1",
			goalUsage:  goalsvc.NewRuntimeUsageAccumulator(false),
		}

		stageDMAppliedGoalCommand(runner, runtimecommand.GoalOperationCreate, "", "")
		runner.recordGoalUsageFromAssistantMessage(goalToolResultAssistantMessage("tool-1", "Bash", false, 5, 1))
		runner.recordGoalUsage(context.Background(), exec.RoundExecutionResult{
			Usage: sdkprotocol.TokenUsage{
				InputTokens:  8,
				OutputTokens: 3,
				TotalTokens:  11,
			},
		}, nil)

		usages := goalProvider.recordedUsage()
		if len(usages) != 1 {
			t.Fatalf("len(usages) = %d, want one terminal settlement", len(usages))
		}
		if usages[0].InputTokens != 8 || usages[0].OutputTokens != 3 ||
			usages[0].BudgetTokens() != 11 || usages[0].ActualTokens() != 11 {
			t.Fatalf("usage = %#v, want complete first Goal round 8/3", usages[0])
		}
	})
}

func TestRoundRunnerBindsModelCreatedGoalThroughTerminalSettlement(t *testing.T) {
	sessionKey := "agent:nexus:ws:dm:test"
	goalProvider := &fakeGoalContextProvider{
		usageGoal:   &protocol.Goal{ID: "goal-created", SessionKey: sessionKey},
		runtimeGoal: &protocol.Goal{ID: "goal-created", SessionKey: sessionKey},
	}
	runner := &roundRunner{
		service:    &Service{goals: goalProvider},
		sessionKey: sessionKey,
		roundID:    "round-1",
		goalUsage:  goalsvc.NewRuntimeUsageAccumulator(false),
	}

	stageDMAppliedGoalCommand(runner, runtimecommand.GoalOperationCreate, "goal-created", "")
	runner.recordGoalUsageFromAssistantMessage(goalToolResultAssistantMessage("tool-create", "Bash", false, 5, 1))
	stageDMAppliedGoalCommand(runner, runtimecommand.GoalOperationUpdate, "goal-created", protocol.GoalStatusComplete)
	runner.recordGoalUsageFromAssistantMessage(goalToolResultAssistantMessage("tool-update", "Bash", false, 8, 2))
	runner.finalizeGoalUsage(context.Background(), exec.RoundExecutionResult{
		Usage: sdkprotocol.TokenUsage{
			InputTokens:  17,
			OutputTokens: 4,
			TotalTokens:  21,
		},
	}, nil)

	if runner.goalIDForUsage != "goal-created" {
		t.Fatalf("goalIDForUsage = %q, want fixed model-created Goal", runner.goalIDForUsage)
	}
	if len(goalProvider.usageSessionKeys) != 0 {
		t.Fatalf("session usage targets = %#v, want bound Goal-only terminal settlement", goalProvider.usageSessionKeys)
	}
	if len(goalProvider.usageGoalIDs) != 1 ||
		goalProvider.usageGoalIDs[0] != "goal-created" {
		t.Fatalf("goal usage targets = %#v, want terminal fixed to goal-created", goalProvider.usageGoalIDs)
	}
	total := protocol.GoalUsage{}
	for _, usage := range goalProvider.recordedUsage() {
		total = total.Add(usage)
	}
	if total.InputTokens != 17 || total.OutputTokens != 4 || total.ActualTokens() != 21 {
		t.Fatalf("settled usage = %#v, want complete round 17/4", total)
	}
}

func TestRoundRunnerRecordsNXSSubagentActualUsageWithoutDoubleCounting(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: goalProvider, runtime: runtimectx.NewManager()},
		sessionKey:     "agent:nexus:ws:dm:test",
		roundID:        "round-1",
		goalIDForUsage: "goal-1",
		runtimeKind:    "nxs",
		goalUsage:      goalsvc.NewRuntimeUsageAccumulator(true),
	}
	taskMessage := func(total int64) protocol.Message {
		return protocol.Message{"metadata": map[string]any{
			"task_id": "task-1",
			"usage":   map[string]any{"total_tokens": total},
		}}
	}

	runner.recordSubagentGoalUsage(context.Background(), taskMessage(100))
	runner.recordSubagentGoalUsage(context.Background(), taskMessage(150))
	runner.recordSubagentGoalUsage(context.Background(), taskMessage(150))

	usages := goalProvider.recordedUsage()
	if len(usages) != 2 || usages[0].ActualTokens() != 100 || usages[1].ActualTokens() != 50 {
		t.Fatalf("usages = %#v, want exact 100 + 50 child actual deltas", usages)
	}
	if usages[0].BudgetTokens() != 0 || usages[1].BudgetTokens() != 0 {
		t.Fatalf("usages = %#v, child total-only snapshots must not become budget tokens", usages)
	}
}

func TestRoundRunnerKeepsSubagentJoinBarrierWhileUsageCheckpointPersists(t *testing.T) {
	provider := &blockingPersistentDMGoalProvider{
		fakeGoalContextProvider: &fakeGoalContextProvider{},
		entered:                 make(chan struct{}),
		release:                 make(chan struct{}),
	}
	runner := &roundRunner{
		service:             &Service{goals: provider},
		sessionKey:          "agent:nexus:ws:dm:source-barrier",
		roundID:             "round-source-barrier",
		ownerUserID:         "owner-source-barrier",
		childGoalIDForUsage: "goal-source-barrier",
		runtimeKind:         "nxs",
	}
	runner.rememberSubagentTaskMessage(dmSubagentTaskMessage("task_started", "running"))
	terminalMessage := dmSubagentTaskMessage("task_notification", "completed")
	terminalMessage["metadata"].(map[string]any)["usage"] = map[string]any{"total_tokens": int64(100)}

	settled := make(chan []dmSubagentUsageSettlement, 1)
	go func() {
		settled <- runner.recordSubagentGoalUsage(context.Background(), terminalMessage)
	}()
	select {
	case <-provider.entered:
	case <-time.After(time.Second):
		t.Fatal("subagent usage checkpoint did not enter persistence")
	}

	runner.rememberSubagentTaskMessage(terminalMessage)
	if !runner.hasRunningSubagentTask() {
		t.Fatal("terminal lifecycle removed the task before its usage checkpoint settled")
	}

	close(provider.release)
	var settledSnapshots []dmSubagentUsageSettlement
	select {
	case settledSnapshots = <-settled:
	case <-time.After(time.Second):
		t.Fatal("subagent usage checkpoint did not finish")
	}
	for _, settled := range settledSnapshots {
		runner.clearSubagentUsageObservationPending(settled.taskID, settled.observation)
	}
	if runner.hasRunningSubagentTask() {
		t.Fatal("settled usage checkpoint did not release the child join barrier")
	}
}

func TestDMExternalActivationWaitsForInflightChildCheckpointBeforeBind(t *testing.T) {
	provider := &orderedDMBindingGoalProvider{
		fakeGoalContextProvider: &fakeGoalContextProvider{},
		sourceEntered:           make(chan struct{}),
		sourceRelease:           make(chan struct{}),
		bindEntered:             make(chan struct{}),
	}
	runner := &roundRunner{
		service:          &Service{goals: provider},
		sessionKey:       "agent:nexus:ws:dm:bind-source-race",
		roundID:          "round-bind-source-race",
		ownerUserID:      "owner-bind-source-race",
		runtimeKind:      "nxs",
		goalUsage:        goalsvc.NewRuntimeUsageAccumulator(false),
		goalUsageStarted: time.Now(),
	}
	progress := dmSubagentTaskMessage("task_progress", "running")
	progress["metadata"].(map[string]any)["usage"] = map[string]any{
		"total_tokens": int64(100),
	}

	recordDone := make(chan []dmSubagentUsageSettlement, 1)
	go func() {
		recordDone <- runner.recordSubagentGoalUsage(context.Background(), progress)
	}()
	select {
	case <-provider.sourceEntered:
	case <-time.After(time.Second):
		t.Fatal("pre-binding child checkpoint did not enter persistence")
	}

	activateDone := make(chan error, 1)
	go func() {
		activateDone <- runner.activateGoalUsage(context.Background(), "goal-new")
	}()
	select {
	case <-provider.bindEntered:
		t.Fatal("external bind overtook the in-flight pre-binding child checkpoint")
	case <-time.After(50 * time.Millisecond):
	}

	close(provider.sourceRelease)
	select {
	case <-recordDone:
	case <-time.After(time.Second):
		t.Fatal("pre-binding child checkpoint did not finish")
	}
	select {
	case err := <-activateDone:
		if err != nil {
			t.Fatalf("activateGoalUsage() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("external bind did not finish after child checkpoint")
	}

	if len(provider.snapshots) != 1 {
		t.Fatalf("source snapshots = %#v, want one pre-binding checkpoint", provider.snapshots)
	}
	snapshot := provider.snapshots[0]
	if snapshot.GoalID != "" ||
		snapshot.CumulativeActualTokens != 100 ||
		snapshot.Terminal ||
		snapshot.TokenUsageObserved ||
		!snapshot.EvidenceRequired ||
		snapshot.ObservedAt.IsZero() {
		t.Fatalf("pre-binding running child snapshot = %#v", snapshot)
	}
	if len(provider.bindings) != 1 || provider.bindings[0].GoalID != "goal-new" {
		t.Fatalf("bindings = %#v, want goal-new after source checkpoint", provider.bindings)
	}
	if snapshot.ObservedAt.After(provider.bindings[0].BoundAt) {
		t.Fatalf(
			"child observed_at = %v, want no later than bind boundary %v",
			snapshot.ObservedAt,
			provider.bindings[0].BoundAt,
		)
	}
}

func TestDMExternalActivationFlushesPendingChildBeforeBindAndSkipsStaleRetry(t *testing.T) {
	provider := &orderedDMBindingGoalProvider{
		fakeGoalContextProvider: &fakeGoalContextProvider{},
	}
	runner := &roundRunner{
		service:     &Service{goals: provider},
		sessionKey:  "agent:nexus:ws:dm:bind-pending",
		roundID:     "round-bind-pending",
		ownerUserID: "owner-bind-pending",
		runtimeKind: "nxs",
		goalUsage:   goalsvc.NewRuntimeUsageAccumulator(false),
	}
	runner.markSubagentUsageObservationPending("task-pending", dmSubagentUsageObservation{
		cumulativeTotal: 75,
	})

	pendingTaskIDs, done, _ := runner.pendingSubagentUsageForRetry()
	if done || len(pendingTaskIDs) != 1 {
		t.Fatalf("pending task IDs = %#v, done=%v", pendingTaskIDs, done)
	}
	if err := runner.activateGoalUsage(context.Background(), "goal-new"); err != nil {
		t.Fatal(err)
	}
	// Simulate a retry worker that copied the task ID before activation. It must
	// re-check pending under the lock and avoid replaying the old checkpoint.
	if err := runner.retryPendingSubagentUsageObservation(
		context.Background(),
		provider,
		pendingTaskIDs[0],
	); err != nil {
		t.Fatal(err)
	}

	if len(provider.order) != 2 ||
		provider.order[0] != "source" ||
		provider.order[1] != "bind" {
		t.Fatalf("persistence order = %#v, want source then bind", provider.order)
	}
	if len(provider.snapshots) != 1 || provider.snapshots[0].GoalID != "" {
		t.Fatalf("source snapshots = %#v, stale retry must not target new Goal", provider.snapshots)
	}
	if runner.hasRunningSubagentTask() {
		t.Fatal("successfully flushed pending checkpoint still holds the in-memory join barrier")
	}
}

func TestDMExternalActivationStopsWhenPendingChildCheckpointCannotPersist(t *testing.T) {
	sourceErr := errors.New("source checkpoint unavailable")
	provider := &orderedDMBindingGoalProvider{
		fakeGoalContextProvider: &fakeGoalContextProvider{},
		sourceErr:               sourceErr,
	}
	runner := &roundRunner{
		service:                &Service{goals: provider},
		sessionKey:             "agent:nexus:ws:dm:bind-pending-failure",
		roundID:                "round-bind-pending-failure",
		ownerUserID:            "owner-bind-pending-failure",
		runtimeKind:            "nxs",
		goalIDForUsage:         "goal-old",
		childGoalIDForUsage:    "goal-old",
		goalUsage:              goalsvc.NewRuntimeUsageAccumulator(true),
		goalUsageScopeConsumed: true,
	}
	accelerateDMGoalUsageRetry(runner)
	runner.markSubagentUsageObservationPending("task-pending", dmSubagentUsageObservation{
		cumulativeTotal: 75,
	})

	err := runner.activateGoalUsage(context.Background(), "goal-new")
	if !errors.Is(err, sourceErr) {
		t.Fatalf("activateGoalUsage() error = %v, want source checkpoint failure", err)
	}
	if len(provider.bindings) != 0 {
		t.Fatalf("bindings = %#v, bind must not run after source failure", provider.bindings)
	}
	if runner.goalIDForUsage != "goal-old" ||
		runner.childGoalIDForUsage != "goal-old" ||
		!runner.goalUsageScopeConsumed ||
		!runner.goalUsage.Active() {
		t.Fatalf(
			"failed activation mutated old state: goal=%q child=%q consumed=%v active=%v",
			runner.goalIDForUsage,
			runner.childGoalIDForUsage,
			runner.goalUsageScopeConsumed,
			runner.goalUsage.Active(),
		)
	}
	if pending := runner.subagentUsagePending["task-pending"]; pending.cumulativeTotal != 75 {
		t.Fatalf("failed source checkpoint lost pending observation: %#v", pending)
	}
	if len(provider.snapshots) != goalUsagePersistAttempts {
		t.Fatalf("source attempts = %d, want %d", len(provider.snapshots), goalUsagePersistAttempts)
	}
	observedAt := provider.snapshots[0].ObservedAt
	if observedAt.IsZero() {
		t.Fatal("pending observation did not preserve its capture time")
	}
	for index, snapshot := range provider.snapshots[1:] {
		if !snapshot.ObservedAt.Equal(observedAt) {
			t.Fatalf(
				"retry snapshot[%d] observed_at = %v, want stable %v",
				index+1,
				snapshot.ObservedAt,
				observedAt,
			)
		}
	}
}

func TestRoundRunnerClaimsPreCreateSubagentUsageAndKeepsChildBoundAfterTerminal(t *testing.T) {
	sessionKey := "agent:nexus:ws:dm:child-round-start"
	base := &fakeGoalContextProvider{
		runtimeGoal: &protocol.Goal{
			ID:         "goal-created",
			SessionKey: sessionKey,
			Status:     protocol.GoalStatusActive,
		},
	}
	provider := &fakePersistentDMGoalProvider{fakeGoalContextProvider: base}
	runner := &roundRunner{
		service:          &Service{goals: provider},
		sessionKey:       sessionKey,
		roundID:          "round-create",
		ownerUserID:      "owner-dm",
		runtimeKind:      "nxs",
		goalUsage:        goalsvc.NewRuntimeUsageAccumulator(false),
		goalUsageStarted: time.Now(),
	}
	taskMessage := func(total int64) protocol.Message {
		return protocol.Message{"metadata": map[string]any{
			"task_id": "task-pre-create",
			"usage":   map[string]any{"total_tokens": total},
		}}
	}

	runner.recordSubagentGoalUsage(context.Background(), taskMessage(100))
	stageDMAppliedGoalCommand(runner, runtimecommand.GoalOperationCreate, "goal-created", "")
	runner.recordGoalUsageFromAssistantMessage(
		goalToolResultAssistantMessage("tool-create", "create_goal", false, 0, 0),
	)

	if len(provider.snapshots) != 1 ||
		provider.snapshots[0].GoalID != "" ||
		provider.snapshots[0].RoundID != "round-create" ||
		provider.snapshots[0].ScopeRoundID != "round-create" {
		t.Fatalf("pre-create snapshots = %#v, want one unbound checkpoint observation", provider.snapshots)
	}
	if len(provider.claims) != 1 {
		t.Fatalf("claims = %#v, want one model-create round claim", provider.claims)
	}
	claim := provider.claims[0]
	if claim.OwnerUserID != "owner-dm" ||
		claim.RuntimeSessionKey != sessionKey ||
		claim.RoundID != "round-create" ||
		claim.ScopeRoundID != "round-create" ||
		claim.GoalID != "goal-created" ||
		claim.GoalSessionKey != sessionKey {
		t.Fatalf("claim = %#v, want exact DM round/Goal identity", claim)
	}

	runner.finalizeGoalUsage(context.Background(), exec.RoundExecutionResult{}, nil)
	runner.recordSubagentGoalUsage(context.Background(), taskMessage(150))
	if len(provider.snapshots) != 2 || provider.snapshots[1].GoalID != "goal-created" {
		t.Fatalf("post-terminal snapshots = %#v, want child fixed to original Goal", provider.snapshots)
	}
}

func TestDMPersistsNXSChildLifecycleEvidenceWithTerminalOnlyTokenPresence(t *testing.T) {
	const (
		sessionKey = "agent:nexus:ws:dm:child-evidence"
		roundID    = "round-child-evidence"
		goalID     = "goal-child-evidence"
	)
	provider := &fakePersistentDMGoalProvider{
		fakeGoalContextProvider: &fakeGoalContextProvider{},
	}
	runner := &roundRunner{
		service:             &Service{goals: provider},
		ownerUserID:         "owner-child-evidence",
		sessionKey:          sessionKey,
		roundID:             roundID,
		runtimeKind:         "nxs",
		goalIDForUsage:      goalID,
		childGoalIDForUsage: goalID,
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

	record(taskMessage("task-positive", "task_started", "running", nil))
	record(taskMessage("task-positive", "task_started", "running", nil))
	record(taskMessage("task-positive", "task_notification", "completed", int64(42)))
	record(taskMessage("task-missing", "task_started", "running", nil))
	record(taskMessage("task-missing", "task_progress", "running", int64(23)))
	record(taskMessage("task-missing", "task_notification", "completed", int64(0)))

	if len(provider.snapshots) != 6 {
		t.Fatalf("child evidence snapshots = %#v, want start + restart + terminal and start + progress + terminal", provider.snapshots)
	}
	for index, snapshot := range provider.snapshots {
		if !snapshot.EvidenceRequired ||
			snapshot.OwnerUserID != "owner-child-evidence" ||
			snapshot.RuntimeSessionKey != sessionKey ||
			snapshot.GoalSessionKey != sessionKey ||
			snapshot.RoundID != roundID ||
			snapshot.ScopeRoundID != roundID ||
			snapshot.GoalID != goalID ||
			snapshot.ObservedAt.IsZero() {
			t.Fatalf("snapshot[%d] identity/evidence = %#v", index, snapshot)
		}
	}
	for _, index := range []int{0, 1, 3} {
		snapshot := provider.snapshots[index]
		if snapshot.Terminal || snapshot.TokenUsageObserved ||
			snapshot.CumulativeActualTokens != 0 {
			t.Fatalf("start/restart snapshot[%d] = %#v, want required nonterminal zero", index, snapshot)
		}
	}
	positiveTerminal := provider.snapshots[2]
	if !positiveTerminal.Terminal ||
		!positiveTerminal.TokenUsageObserved ||
		positiveTerminal.CumulativeActualTokens != 42 {
		t.Fatalf("positive terminal = %#v, want authoritative terminal 42", positiveTerminal)
	}
	progress := provider.snapshots[4]
	if progress.Terminal ||
		progress.TokenUsageObserved ||
		progress.CumulativeActualTokens != 23 {
		t.Fatalf("positive progress = %#v, want checkpoint without terminal evidence", progress)
	}
	missingTerminal := provider.snapshots[5]
	if !missingTerminal.Terminal ||
		missingTerminal.TokenUsageObserved ||
		missingTerminal.CumulativeActualTokens != 0 {
		t.Fatalf("placeholder terminal = %#v, want terminal unavailable despite positive progress", missingTerminal)
	}
}

func TestRoundRunnerIgnoresGoalRuntimeInPlanMode(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:          &Service{goals: goalProvider},
		sessionKey:       "agent:nexus:ws:dm:test-goal-plan-runtime",
		roundID:          "round-plan",
		goalIDForUsage:   "goal-plan",
		goalUsage:        goalsvc.NewRuntimeUsageAccumulator(true),
		goalUsageStarted: time.Now(),
		permissionMode:   sdkpermission.ModePlan,
	}

	runner.recordGoalUsageFromAssistantMessage(goalToolResultAssistantMessage("tool-1", "read_file", false, 4, 1))
	runner.recordGoalUsage(context.Background(), exec.RoundExecutionResult{
		Usage: sdkprotocol.TokenUsage{
			InputTokens:  10,
			OutputTokens: 2,
		},
		ElapsedTimeSeconds: 3,
	}, protocol.Message{})
	runner.recordGoalUsageLimit(exec.RoundExecutionResult{
		UsageLimitReached: true,
		UsageLimitReason:  "usage limit",
	})
	runner.recordGoalContinuationProgress(exec.RoundExecutionResult{})

	if usages := goalProvider.recordedUsage(); len(usages) != 0 {
		t.Fatalf("plan mode recorded goal usage: %#v", usages)
	}
	if reasons := goalProvider.recordedUsageLimitReasons(); len(reasons) != 0 {
		t.Fatalf("plan mode recorded usage limit: %#v", reasons)
	}
	if progress := goalProvider.recordedProgress(); len(progress) != 0 {
		t.Fatalf("plan mode recorded continuation progress: %#v", progress)
	}
}

type fakePersistentDMGoalProvider struct {
	*fakeGoalContextProvider
	snapshots []protocol.GoalUsageSourceSnapshot
	claims    []protocol.GoalUsageSourceRoundClaim
}

type fakeDMScopeBindingGoalProvider struct {
	*fakeGoalContextProvider
	bindings []protocol.GoalUsageScopeBinding
	bindErr  error
}

func (p *fakeDMScopeBindingGoalProvider) BindUsageScopeFromNow(
	_ context.Context,
	binding protocol.GoalUsageScopeBinding,
) (protocol.GoalUsageScopeBindResult, error) {
	p.bindings = append(p.bindings, binding)
	return protocol.GoalUsageScopeBindResult{}, p.bindErr
}

type blockingPersistentDMGoalProvider struct {
	*fakeGoalContextProvider
	entered chan struct{}
	release chan struct{}
}

type orderedDMBindingGoalProvider struct {
	*fakeGoalContextProvider
	sourceEntered chan struct{}
	sourceRelease chan struct{}
	bindEntered   chan struct{}
	sourceErr     error
	bindErr       error
	snapshots     []protocol.GoalUsageSourceSnapshot
	bindings      []protocol.GoalUsageScopeBinding
	order         []string
}

func (p *orderedDMBindingGoalProvider) RecordUsageSourceSnapshot(
	_ context.Context,
	snapshot protocol.GoalUsageSourceSnapshot,
) (protocol.GoalUsageSourceResult, error) {
	p.snapshots = append(p.snapshots, snapshot)
	p.order = append(p.order, "source")
	if p.sourceEntered != nil {
		close(p.sourceEntered)
	}
	if p.sourceRelease != nil {
		<-p.sourceRelease
	}
	return protocol.GoalUsageSourceResult{}, p.sourceErr
}

func (p *orderedDMBindingGoalProvider) BindUsageScopeFromNow(
	_ context.Context,
	binding protocol.GoalUsageScopeBinding,
) (protocol.GoalUsageScopeBindResult, error) {
	p.bindings = append(p.bindings, binding)
	p.order = append(p.order, "bind")
	if p.bindEntered != nil {
		close(p.bindEntered)
	}
	return protocol.GoalUsageScopeBindResult{}, p.bindErr
}

func (p *blockingPersistentDMGoalProvider) RecordUsageSourceSnapshot(
	_ context.Context,
	_ protocol.GoalUsageSourceSnapshot,
) (protocol.GoalUsageSourceResult, error) {
	close(p.entered)
	<-p.release
	return protocol.GoalUsageSourceResult{}, nil
}

func (p *fakePersistentDMGoalProvider) RecordUsageSourceSnapshot(
	_ context.Context,
	snapshot protocol.GoalUsageSourceSnapshot,
) (protocol.GoalUsageSourceResult, error) {
	p.snapshots = append(p.snapshots, snapshot)
	return protocol.GoalUsageSourceResult{}, nil
}

func (p *fakePersistentDMGoalProvider) ClaimUsageSourceRound(
	_ context.Context,
	claim protocol.GoalUsageSourceRoundClaim,
) (protocol.GoalUsageSourceResult, error) {
	p.claims = append(p.claims, claim)
	return protocol.GoalUsageSourceResult{}, nil
}
