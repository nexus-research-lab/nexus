package dm

import (
	"context"
	"errors"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	sdkpermission "github.com/nexus-research-lab/nexus-agent-sdk-bridge/permission"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	nexusmcp "github.com/nexus-research-lab/nexus/internal/mcp"
	"github.com/nexus-research-lab/nexus/internal/mcp/command"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	exec "github.com/nexus-research-lab/nexus/internal/runtime/exec"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	"github.com/nexus-research-lab/nexus/internal/service/runtimehost"
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
				runner.RememberGoalAssistantMessage(goalCompletionCommandMissAssistantMessage())
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
				service:               &Service{goals: provider, Host: runtimehost.Host{Runtime: runtimectx.NewManager()}},
				sessionKey:            "agent:nexus:ws:dm:revision",
				roundID:               "round-revision",
				GoalRoundState:        runtimehost.GoalRoundState{IDForUsage: "goal-revision"},
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

func TestTerminalRoundStatusEventCarriesUsageLimitFailureCode(t *testing.T) {
	runner := &roundRunner{
		sessionKey: "agent:nexus:ws:dm:terminal-usage-limit",
		roundID:    "round-terminal-usage-limit",
		agent:      &protocol.Agent{AgentID: "nexus"},
	}
	event := terminalRoundStatusEvent(runner, exec.RoundExecutionResult{
		TerminalStatus:    "error",
		ResultSubtype:     "error",
		ErrorMessage:      "provider returned an invalid request",
		UsageLimitReached: true,
	})
	if event.Data["failure_code"] != protocol.ConversationFailureUsageLimited {
		t.Fatalf("failure_code = %#v, want %q", event.Data["failure_code"], protocol.ConversationFailureUsageLimited)
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
		service:               &Service{goals: &fakeGoalContextProvider{}, Host: runtimehost.Host{Runtime: runtimectx.NewManager()}},
		GoalRoundState:        runtimehost.GoalRoundState{IDForUsage: "goal-1"},
		goalObjectiveRevision: func() *atomic.Int64 { value := &atomic.Int64{}; value.Store(1); return value }(),
		responsibilityState:   goalOnly,
	}
	stageDMRuntimeCommandReceipt(runner, nexusmcp.CommandReceipt{
		Domain: command.DomainExecution, Operation: "submit_work",
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
	stageDMRuntimeCommandReceipt(runner, nexusmcp.CommandReceipt{
		Domain: command.DomainExecution, Operation: "submit_work",
		Outcome: string(protocol.MutationResultApplied), GoalBound: true,
	})
	runner.recordGoalUsageFromAssistantMessage(message)
	if !runner.hasGoalToolProgress() {
		t.Fatal("confirmed Goal-bound Execution mutation was not counted")
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
		service:               &Service{Host: runtimehost.Host{Runtime: manager}, goals: provider},
		sessionKey:            sessionKey,
		roundID:               roundID,
		GoalRoundState:        runtimehost.GoalRoundState{IDForUsage: goalID, ChildIDForUsage: goalID, Usage: goalsvc.NewRuntimeUsageAccumulator(true)},
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
	runner.Mu.Lock()
	binding := runner.IDForUsage
	runner.Mu.Unlock()
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

func TestDMExternalActivationDurableBindFailureKeepsOldBindingAndBaseline(t *testing.T) {
	bindConflict := errors.New("durable scope already bound")
	provider := &fakeDMScopeBindingGoalProvider{
		fakeGoalContextProvider: &fakeGoalContextProvider{},
		bindErr:                 bindConflict,
	}
	runner := &roundRunner{
		service:        &Service{goals: provider, Host: runtimehost.Host{Runtime: runtimectx.NewManager()}},
		ownerUserID:    "owner-bind-conflict",
		sessionKey:     "agent:nexus:ws:dm:bind-conflict",
		roundID:        "round-bind-conflict",
		GoalRoundState: runtimehost.GoalRoundState{IDForUsage: "goal-old", ChildIDForUsage: "goal-old", Usage: goalsvc.NewRuntimeUsageAccumulator(true), UsageScopeConsumed: true},
	}
	accelerateDMGoalUsageRetry(runner)
	runner.recordGoalUsageFromAssistantMessage(
		goalToolResultAssistantMessage("tool-old", "read_file", false, 4, 1),
	)

	err := runner.activateGoalUsage(context.Background(), "goal-new")
	if !errors.Is(err, bindConflict) {
		t.Fatalf("activateGoalUsage() error = %v, want durable bind conflict", err)
	}
	runner.Mu.Lock()
	binding := runner.IDForUsage
	consumed := runner.UsageScopeConsumed
	runner.Mu.Unlock()
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
	if runner.IDForUsage != "goal-new" {
		t.Fatalf("capability fallback binding = %q, want goal-new", runner.IDForUsage)
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
		service:        &Service{goals: provider, Host: runtimehost.Host{Runtime: runtimectx.NewManager()}},
		ownerUserID:    "owner-model-bind-window",
		sessionKey:     sessionKey,
		roundID:        roundID,
		GoalRoundState: runtimehost.GoalRoundState{Usage: goalsvc.NewRuntimeUsageAccumulator(false)},
		runtimeKind:    "nxs",
	}
	accelerateDMGoalUsageRetry(runner)
	runner.recordGoalUsageFromAssistantMessage(
		goalToolResultAssistantMessage("tool-before-create", "read_file", false, 4, 1),
	)

	if err := runner.activateGoalUsage(context.Background(), "goal-external"); !errors.Is(err, bindConflict) {
		t.Fatalf("external activation error = %v, want durable model-scope conflict", err)
	}
	runner.Mu.Lock()
	binding := runner.IDForUsage
	consumed := runner.UsageScopeConsumed
	active := runner.Usage.Active()
	runner.Mu.Unlock()
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
	stageDMAppliedGoalCommand(runner, command.GoalOperationCreate, modelGoal, "")
	runner.recordGoalUsageFromAssistantMessage(
		goalToolResultAssistantMessage("tool-create", "create_goal", false, 5, 1),
	)
	runner.Mu.Lock()
	binding = runner.IDForUsage
	consumed = runner.UsageScopeConsumed
	runner.Mu.Unlock()
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
			service := &Service{Host: runtimehost.Host{Runtime: manager}}
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
				GoalRoundState: runtimehost.GoalRoundState{IDForUsage: test.goalID, Usage: goalsvc.NewRuntimeUsageAccumulator(test.active)},
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
		service:        &Service{goals: goalProvider, Host: runtimehost.Host{Runtime: runtimectx.NewManager()}},
		sessionKey:     "agent:nexus:ws:dm:test",
		roundID:        "round-1",
		GoalRoundState: runtimehost.GoalRoundState{IDForUsage: "goal-1", Usage: goalsvc.NewRuntimeUsageAccumulator(true)},
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

func TestRoundRunnerSkipsEmptyGoalContinuationProgressWhileSubagentRuns(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: goalProvider, Host: runtimehost.Host{Runtime: runtimectx.NewManager()}},
		sessionKey:     "agent:nexus:ws:dm:test",
		roundID:        "goal_continuation_1",
		GoalRoundState: runtimehost.GoalRoundState{IDForUsage: "goal-1", SubagentTasks: map[string]struct{}{"task-1": {}}},
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

func TestRoundRunnerActivateSameGoalPreservesLowerExactTerminalCalibration(t *testing.T) {
	goalProvider := &fakeGoalContextProvider{}
	runner := &roundRunner{
		service:        &Service{goals: goalProvider, Host: runtimehost.Host{Runtime: runtimectx.NewManager()}},
		sessionKey:     "agent:nexus:ws:dm:same-goal-activate",
		roundID:        "round-same-goal-activate",
		GoalRoundState: runtimehost.GoalRoundState{IDForUsage: "goal-same", ChildIDForUsage: "goal-same", Usage: goalsvc.NewRuntimeUsageAccumulator(true)},
	}
	assistant := protocol.Message{
		"message_id": "assistant-same-goal",
		"role":       "assistant",
		"usage": map[string]any{
			"input_tokens":  int64(150),
			"output_tokens": int64(50),
		},
	}
	runner.RememberGoalAssistantMessage(assistant)
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

func TestRoundRunnerResetsGoalUsageAfterCreateGoal(t *testing.T) {
	t.Run("create_goal command", func(t *testing.T) {
		goalProvider := &fakeGoalContextProvider{}
		runner := &roundRunner{
			service:        &Service{goals: goalProvider, Host: runtimehost.Host{Runtime: runtimectx.NewManager()}},
			sessionKey:     "agent:nexus:ws:dm:test",
			roundID:        "round-1",
			GoalRoundState: runtimehost.GoalRoundState{Usage: goalsvc.NewRuntimeUsageAccumulator(false)},
		}

		stageDMAppliedGoalCommand(runner, command.GoalOperationCreate, "", "")
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
		service:        &Service{goals: goalProvider, Host: runtimehost.Host{Runtime: runtimectx.NewManager()}},
		sessionKey:     sessionKey,
		roundID:        "round-1",
		GoalRoundState: runtimehost.GoalRoundState{Usage: goalsvc.NewRuntimeUsageAccumulator(false)},
	}

	stageDMAppliedGoalCommand(runner, command.GoalOperationCreate, "goal-created", "")
	runner.recordGoalUsageFromAssistantMessage(goalToolResultAssistantMessage("tool-create", "Bash", false, 5, 1))
	stageDMAppliedGoalCommand(runner, command.GoalOperationUpdate, "goal-created", protocol.GoalStatusComplete)
	runner.recordGoalUsageFromAssistantMessage(goalToolResultAssistantMessage("tool-update", "Bash", false, 8, 2))
	runner.finalizeGoalUsage(context.Background(), exec.RoundExecutionResult{
		Usage: sdkprotocol.TokenUsage{
			InputTokens:  17,
			OutputTokens: 4,
			TotalTokens:  21,
		},
	}, nil)

	if runner.IDForUsage != "goal-created" {
		t.Fatalf("goalIDForUsage = %q, want fixed model-created Goal", runner.IDForUsage)
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
		service:        &Service{goals: goalProvider, Host: runtimehost.Host{Runtime: runtimectx.NewManager()}},
		sessionKey:     "agent:nexus:ws:dm:test",
		roundID:        "round-1",
		GoalRoundState: runtimehost.GoalRoundState{IDForUsage: "goal-1", Usage: goalsvc.NewRuntimeUsageAccumulator(true)},
		runtimeKind:    "nxs",
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

func TestDMExternalActivationFlushesPendingChildBeforeBindAndSkipsStaleRetry(t *testing.T) {
	provider := &orderedDMBindingGoalProvider{
		fakeGoalContextProvider: &fakeGoalContextProvider{},
	}
	runner := &roundRunner{
		service:        &Service{goals: provider, Host: runtimehost.Host{Runtime: runtimectx.NewManager()}},
		sessionKey:     "agent:nexus:ws:dm:bind-pending",
		roundID:        "round-bind-pending",
		ownerUserID:    "owner-bind-pending",
		runtimeKind:    "nxs",
		GoalRoundState: runtimehost.GoalRoundState{Usage: goalsvc.NewRuntimeUsageAccumulator(false)},
	}
	runner.markSubagentUsageObservationPending("task-pending", goalsvc.SubagentUsageObservation{
		CumulativeTotal: 75,
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
	if runner.HasRunningSubagentTask() {
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
		service:        &Service{goals: provider, Host: runtimehost.Host{Runtime: runtimectx.NewManager()}},
		sessionKey:     "agent:nexus:ws:dm:bind-pending-failure",
		roundID:        "round-bind-pending-failure",
		ownerUserID:    "owner-bind-pending-failure",
		runtimeKind:    "nxs",
		GoalRoundState: runtimehost.GoalRoundState{IDForUsage: "goal-old", ChildIDForUsage: "goal-old", Usage: goalsvc.NewRuntimeUsageAccumulator(true), UsageScopeConsumed: true},
	}
	accelerateDMGoalUsageRetry(runner)
	runner.markSubagentUsageObservationPending("task-pending", goalsvc.SubagentUsageObservation{
		CumulativeTotal: 75,
	})

	err := runner.activateGoalUsage(context.Background(), "goal-new")
	if !errors.Is(err, sourceErr) {
		t.Fatalf("activateGoalUsage() error = %v, want source checkpoint failure", err)
	}
	if len(provider.bindings) != 0 {
		t.Fatalf("bindings = %#v, bind must not run after source failure", provider.bindings)
	}
	if runner.IDForUsage != "goal-old" ||
		runner.ChildIDForUsage != "goal-old" ||
		!runner.UsageScopeConsumed ||
		!runner.Usage.Active() {
		t.Fatalf(
			"failed activation mutated old state: goal=%q child=%q consumed=%v active=%v",
			runner.IDForUsage,
			runner.ChildIDForUsage,
			runner.UsageScopeConsumed,
			runner.Usage.Active(),
		)
	}
	if pending := runner.SubagentUsagePending["task-pending"]; pending.CumulativeTotal != 75 {
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
		service:        &Service{goals: provider, Host: runtimehost.Host{Runtime: runtimectx.NewManager()}},
		sessionKey:     sessionKey,
		roundID:        "round-create",
		ownerUserID:    "owner-dm",
		runtimeKind:    "nxs",
		GoalRoundState: runtimehost.GoalRoundState{Usage: goalsvc.NewRuntimeUsageAccumulator(false), UsageStartedAt: time.Now()},
	}
	taskMessage := func(total int64) protocol.Message {
		return protocol.Message{"metadata": map[string]any{
			"task_id": "task-pre-create",
			"usage":   map[string]any{"total_tokens": total},
		}}
	}

	runner.recordSubagentGoalUsage(context.Background(), taskMessage(100))
	stageDMAppliedGoalCommand(runner, command.GoalOperationCreate, "goal-created", "")
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

func (r *roundRunner) recordGoalUsage(ctx context.Context, result exec.RoundExecutionResult, finalAssistant protocol.Message) {
	if r.service.goals == nil || r.ignoreGoalRuntime() {
		return
	}
	snapshot, ok := r.finalGoalUsageSnapshot(result, finalAssistant)
	if !ok {
		return
	}
	r.recordGoalUsageSnapshot(ctx, snapshot)
}
