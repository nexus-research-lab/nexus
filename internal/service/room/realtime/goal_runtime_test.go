package realtime

import (
	"context"
	"errors"
	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	exec "github.com/nexus-research-lab/nexus/internal/runtime/exec"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	workspacestore "github.com/nexus-research-lab/nexus/internal/storage/workspace"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRecordGoalUsageForRoomSlotUsesToolCompletionDelta(t *testing.T) {
	goalProvider := &fakeRoomGoalContextProvider{}
	service := &Service{goals: goalProvider}
	slot := &activeRoomSlot{
		RuntimeSessionKey: "agent:nexus:ws:room:test",
		AgentRoundID:      "round-1",
	}
	slot.setGoalBinding("", "goal-1")
	slot.setGoalUsageAccumulator(goalsvc.NewRuntimeUsageAccumulator(true))

	service.recordGoalUsageFromSlotAssistantMessage(context.Background(), slot, roomGoalToolResultAssistantMessage("tool-1", "read_file", 4, 1))
	service.recordGoalUsageForSlot(context.Background(), slot, exec.RoundExecutionResult{
		Usage: sdkprotocol.TokenUsage{
			InputTokens:  6,
			OutputTokens: 3,
			TotalTokens:  9,
		},
	}, nil)

	usages := goalProvider.recordedUsage()
	if len(usages) != 2 {
		t.Fatalf("len(usages) = %d, want 2", len(usages))
	}
	if usages[0].InputTokens != 4 || usages[0].OutputTokens != 1 || usages[0].Total() != 5 {
		t.Fatalf("first usage = %#v, want 4/1", usages[0])
	}
	if usages[1].InputTokens != 2 || usages[1].OutputTokens != 2 || usages[1].Total() != 4 {
		t.Fatalf("second usage = %#v, want remaining 2/2", usages[1])
	}
}

func TestRecordGoalUsageForRoomSlotUsesAssistantSnapshotOnAbort(t *testing.T) {
	goalProvider := &fakeRoomGoalContextProvider{}
	service := &Service{goals: goalProvider}
	slot := &activeRoomSlot{
		RuntimeSessionKey: "agent:nexus:ws:room:test",
		AgentRoundID:      "round-1",
	}
	slot.setGoalBinding("", "goal-1")
	slot.setGoalUsageAccumulator(goalsvc.NewRuntimeUsageAccumulator(true))

	service.recordGoalUsageFromSlotAssistantMessage(context.Background(), slot, roomGoalToolResultAssistantMessage("tool-1", "read_file", 4, 1))
	service.recordGoalUsageForSlot(context.Background(), slot, exec.RoundExecutionResult{}, roomGoalAssistantUsageMessage(9, 4))

	usages := goalProvider.recordedUsage()
	if len(usages) != 2 {
		t.Fatalf("len(usages) = %d, want 2", len(usages))
	}
	if usages[1].InputTokens != 5 || usages[1].OutputTokens != 3 || usages[1].Total() != 8 {
		t.Fatalf("abort usage = %#v, want remaining 5/3", usages[1])
	}
}

func TestRoomSlotRecordsUsageToSharedGoalAfterCreateGoalTool(t *testing.T) {
	for _, toolName := range []string{"create_goal", "mcp__nexus_goal__create_goal"} {
		t.Run(toolName, func(t *testing.T) {
			sharedSessionKey := "room:group:conversation-1"
			goalProvider := &fakeRoomGoalContextProvider{}
			service := &Service{goals: goalProvider}
			slot := &activeRoomSlot{
				RuntimeSessionKey: "agent:nexus:ws:group:conversation-1",
				AgentRoundID:      "round-1:agent-1",
			}
			slot.setGoalBinding(sharedSessionKey, "")
			slot.setGoalUsageAccumulator(goalsvc.NewRuntimeUsageAccumulator(false))

			service.recordGoalUsageFromSlotAssistantMessage(context.Background(), slot, roomGoalToolResultAssistantMessage("tool-1", toolName, 4, 1))
			service.recordGoalUsageForSlot(context.Background(), slot, exec.RoundExecutionResult{
				Usage: sdkprotocol.TokenUsage{
					InputTokens:  9,
					OutputTokens: 3,
					TotalTokens:  12,
				},
			}, nil)

			usages := goalProvider.recordedUsage()
			if len(usages) != 1 {
				t.Fatalf("len(usages) = %d, want post-create delta", len(usages))
			}
			if usages[0].InputTokens != 5 || usages[0].OutputTokens != 2 || usages[0].Total() != 7 {
				t.Fatalf("usage = %#v, want 5/2 delta after create_goal baseline", usages[0])
			}
			if len(goalProvider.usageSessionKeys) != 1 || goalProvider.usageSessionKeys[0] != sharedSessionKey {
				t.Fatalf("usageSessionKeys = %#v, want shared room goal session", goalProvider.usageSessionKeys)
			}
		})
	}
}

func TestRegisterSlotGoalRuntimeMakesGoalGuidanceQueueable(t *testing.T) {
	manager := runtimectx.NewManager()
	service := &Service{runtime: manager}
	slot := &activeRoomSlot{
		RuntimeSessionKey: "agent:nexus:ws:room:conversation-1:agent-1",
		AgentRoundID:      "room-round-1:agent-1",
	}
	manager.StartRound(slot.RuntimeSessionKey, slot.AgentRoundID, nil)

	cleanup := service.registerSlotGoalRuntime(slot)
	roundIDs, err := manager.QueueGuidanceInput(context.Background(), slot.RuntimeSessionKey, "goal-event-1", "budget reached")
	if err != nil {
		t.Fatalf("QueueGuidanceInput() error = %v", err)
	}
	if len(roundIDs) != 1 || roundIDs[0] != slot.AgentRoundID {
		t.Fatalf("roundIDs = %#v, want slot round", roundIDs)
	}
	if count := manager.PendingGuidanceCount(slot.RuntimeSessionKey); count != 1 {
		t.Fatalf("PendingGuidanceCount = %d, want 1", count)
	}
	roundIDs = manager.ClearGoalAccounting(slot.RuntimeSessionKey)
	if len(roundIDs) != 1 || roundIDs[0] != slot.AgentRoundID {
		t.Fatalf("ClearGoalAccounting roundIDs = %#v, want slot round", roundIDs)
	}

	cleanup()
	manager.MarkRoundFinished(slot.RuntimeSessionKey, slot.AgentRoundID)
	if _, err := manager.QueueGuidanceInput(context.Background(), slot.RuntimeSessionKey, "goal-event-2", "late guidance"); !errors.Is(err, runtimectx.ErrNoRunningRound) {
		t.Fatalf("QueueGuidanceInput() after cleanup error = %v, want ErrNoRunningRound", err)
	}
}

func TestRegisterSlotGoalRuntimeUsesGoalSessionKey(t *testing.T) {
	manager := runtimectx.NewManager()
	service := &Service{runtime: manager}
	slot := &activeRoomSlot{
		RuntimeSessionKey: "agent:nexus:ws:group:conversation-1",
		AgentRoundID:      "room-round-1:agent-1",
	}
	slot.setGoalBinding("room:group:conversation-1", "")

	cleanup := service.registerSlotGoalRuntime(slot)
	goalSessionKey := slot.goalSessionKey()
	if roundIDs := manager.GetRunningRoundIDs(goalSessionKey); len(roundIDs) != 0 {
		t.Fatalf("Goal accounting 不应伪造 shared running round: %#v", roundIDs)
	}
	if roundIDs, err := manager.FlushGoalAccounting(context.Background(), goalSessionKey); err != nil || len(roundIDs) != 1 || roundIDs[0] != slot.AgentRoundID {
		t.Fatalf("FlushGoalAccounting() = %#v, %v, want slot accounting", roundIDs, err)
	}
	if roundIDs := manager.ClearGoalAccounting(goalSessionKey); len(roundIDs) != 1 || roundIDs[0] != slot.AgentRoundID {
		t.Fatalf("ClearGoalAccounting() = %#v, want slot accounting", roundIDs)
	}
	if roundIDs, err := manager.ActivateGoalAccounting(context.Background(), goalSessionKey); err != nil || len(roundIDs) != 1 || roundIDs[0] != slot.AgentRoundID {
		t.Fatalf("ActivateGoalAccounting() = %#v, %v, want slot accounting", roundIDs, err)
	}
	if _, err := manager.QueueGuidanceInput(context.Background(), goalSessionKey, "goal-event-1", "budget reached"); !errors.Is(err, runtimectx.ErrNoRunningRound) {
		t.Fatalf("shared Goal accounting 不应伪装 guidance runtime: %v", err)
	}

	cleanup()
	if roundIDs, err := manager.FlushGoalAccounting(context.Background(), goalSessionKey); err != nil || len(roundIDs) != 0 {
		t.Fatalf("cleanup 后 FlushGoalAccounting() = %#v, %v", roundIDs, err)
	}
}

func TestQueueRoomContextualGuidanceTargetsEveryActiveSlotExceptCaller(t *testing.T) {
	manager := runtimectx.NewManager()
	sessionKey := "room:group:conversation-1"
	lead := &activeRoomSlot{
		AgentID:           "agent-lead",
		AgentRoundID:      "round-root:agent-lead",
		RuntimeSessionKey: "agent:lead:ws:group:conversation-1",
	}
	caller := &activeRoomSlot{
		AgentID:           "agent-peer",
		AgentRoundID:      "round-root:agent-peer",
		RuntimeSessionKey: "agent:peer:ws:group:conversation-1",
	}
	manager.StartRound(lead.RuntimeSessionKey, lead.AgentRoundID, nil)
	manager.StartRound(caller.RuntimeSessionKey, caller.AgentRoundID, nil)
	service := &Service{
		runtime: manager,
		rounds: newRoomRoundRegistryFromRounds(map[string]*activeRoomRound{
			"round-root": {
				SessionKey:  sessionKey,
				RoundID:     "round-root",
				RootRoundID: "round-root",
				Slots: map[string]*activeRoomSlot{
					lead.AgentID:   lead,
					caller.AgentID: caller,
				},
			},
		}),
	}
	revision := service.GoalObjectiveRevisionState(sessionKey, "round-root", lead.AgentID, 1)
	if revision == nil || revision.Load() != 1 {
		t.Fatalf("initial revision = %v, want shared state at 1", revision)
	}

	roundIDs, err := service.QueueRoomContextualGuidanceInput(
		context.Background(),
		sessionKey,
		"goal-event-1",
		"goal",
		"The objective changed.",
		caller.AgentID,
		2,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(roundIDs) != 1 || roundIDs[0] != lead.AgentRoundID {
		t.Fatalf("roundIDs = %#v, want lead only", roundIDs)
	}
	if got := manager.PendingGuidanceCount(lead.RuntimeSessionKey); got != 1 {
		t.Fatalf("lead pending guidance = %d, want 1", got)
	}
	if got := manager.PendingGuidanceCount(caller.RuntimeSessionKey); got != 0 {
		t.Fatalf("caller pending guidance = %d, want 0", got)
	}
	if got := revision.Load(); got != 1 {
		t.Fatalf("revision before guidance consumption = %d, want 1", got)
	}
	options := manager.WithGuidanceHook(agentclient.Options{}, lead.RuntimeSessionKey)
	if _, err := options.Hooks.Matchers[sdkhook.EventPostToolUse][0].Hooks[0](
		context.Background(),
		sdkhook.Input{EventName: sdkhook.EventPostToolUse},
		"tool-before-retarget",
	); err != nil {
		t.Fatal(err)
	}
	if got := revision.Load(); got != 2 || lead.currentGoalObjectiveRevision() != 2 {
		t.Fatalf("revision after guidance consumption = pointer:%d slot:%d, want 2", got, lead.currentGoalObjectiveRevision())
	}
	lead.adoptGoalObjectiveRevision(1)
	if got := revision.Load(); got != 2 {
		t.Fatalf("an older guidance callback regressed revision to %d, want 2", got)
	}
}

func TestQueueRoomContextualGuidanceContinuesAfterUnavailableTarget(t *testing.T) {
	manager := runtimectx.NewManager()
	sessionKey := "room:group:conversation-best-effort"
	unavailable := &activeRoomSlot{
		AgentID:           "agent-unavailable",
		AgentRoundID:      "round-root:agent-unavailable",
		RuntimeSessionKey: "agent:a-unavailable:ws:group:conversation-best-effort",
	}
	active := &activeRoomSlot{
		AgentID:           "agent-active",
		AgentRoundID:      "round-root:agent-active",
		RuntimeSessionKey: "agent:b-active:ws:group:conversation-best-effort",
	}
	manager.StartRound(active.RuntimeSessionKey, active.AgentRoundID, nil)
	service := &Service{
		runtime: manager,
		rounds: newRoomRoundRegistryFromRounds(map[string]*activeRoomRound{
			"round-root": {
				SessionKey:  sessionKey,
				RoundID:     "round-root",
				RootRoundID: "round-root",
				Slots: map[string]*activeRoomSlot{
					unavailable.AgentID: unavailable,
					active.AgentID:      active,
				},
			},
		}),
	}

	roundIDs, err := service.QueueRoomContextualGuidanceInput(
		context.Background(),
		sessionKey,
		"goal-event-2",
		"goal",
		"Use the corrected objective.",
		"",
		2,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(roundIDs) != 1 || roundIDs[0] != active.AgentRoundID {
		t.Fatalf("roundIDs = %#v, want active recipient despite earlier unavailable target", roundIDs)
	}
	if got := manager.PendingGuidanceCount(active.RuntimeSessionKey); got != 1 {
		t.Fatalf("active pending guidance = %d, want 1", got)
	}
}

func TestResolveGoalRuntimeContextForSlotPrefersSharedRoomGoal(t *testing.T) {
	sharedSessionKey := "room:group:conversation-1"
	runtimeSessionKey := "agent:nexus:ws:group:conversation-1"
	service := &Service{goals: &fakeRoomGoalContextProvider{
		runtimeContexts: map[string]string{
			sharedSessionKey:  "shared goal context",
			runtimeSessionKey: "runtime goal context",
		},
		runtimeGoals: map[string]*protocol.Goal{
			sharedSessionKey: {
				ID:         "goal-shared",
				SessionKey: sharedSessionKey,
				Status:     protocol.GoalStatusActive,
				Metadata:   map[string]any{protocol.GoalMetadataObjectiveRevision: int64(4)},
			},
			runtimeSessionKey: {
				ID:         "goal-runtime",
				SessionKey: runtimeSessionKey,
				Status:     protocol.GoalStatusActive,
			},
		},
	}}
	slot := &activeRoomSlot{RuntimeSessionKey: runtimeSessionKey}

	prompt, goalContext, goalID, goalSessionKey, _ := service.resolveGoalRuntimeContextForSlot(
		context.Background(),
		&activeRoomRound{SessionKey: sharedSessionKey},
		slot,
		"base prompt",
	)

	if goalID != "goal-shared" || goalSessionKey != sharedSessionKey {
		t.Fatalf("goalID=%q goalSessionKey=%q, want shared goal", goalID, goalSessionKey)
	}
	if got := slot.currentGoalObjectiveRevision(); got != 4 {
		t.Fatalf("slot objective revision = %d, want 4", got)
	}
	if prompt != "base prompt" {
		t.Fatalf("prompt = %q, want unchanged system prompt", prompt)
	}
	if !strings.Contains(goalContext, "shared goal context") || strings.Contains(goalContext, "runtime goal context") {
		t.Fatalf("goalContext = %q, want only shared goal context", goalContext)
	}
}

func TestResolveGoalRuntimeContextForSlotKeepsBudgetLimitedSharedGoalTarget(t *testing.T) {
	sharedSessionKey := "room:group:conversation-1"
	runtimeSessionKey := "agent:nexus:ws:group:conversation-1"
	service := &Service{goals: &fakeRoomGoalContextProvider{
		runtimeContexts: map[string]string{
			runtimeSessionKey: "runtime goal context",
		},
		runtimeGoals: map[string]*protocol.Goal{
			sharedSessionKey: {
				ID:         "goal-shared-budget",
				SessionKey: sharedSessionKey,
				Status:     protocol.GoalStatusBudgetLimited,
			},
			runtimeSessionKey: {
				ID:         "goal-runtime",
				SessionKey: runtimeSessionKey,
				Status:     protocol.GoalStatusActive,
			},
		},
	}}
	slot := &activeRoomSlot{RuntimeSessionKey: runtimeSessionKey}

	prompt, goalContext, goalID, goalSessionKey, _ := service.resolveGoalRuntimeContextForSlot(
		context.Background(),
		&activeRoomRound{SessionKey: sharedSessionKey},
		slot,
		"base prompt",
	)

	if goalID != "goal-shared-budget" || goalSessionKey != sharedSessionKey {
		t.Fatalf("goalID=%q goalSessionKey=%q, want budget-limited shared usage target", goalID, goalSessionKey)
	}
	if prompt != "base prompt" {
		t.Fatalf("prompt = %q, want unchanged system prompt", prompt)
	}
	if goalContext != "" {
		t.Fatalf("goalContext = %q, want no injected context for budget_limited goal", goalContext)
	}
}

func TestResolveGoalRuntimeContextForSlotDoesNotFallBackFromSharedRoomToRuntimeGoal(t *testing.T) {
	sharedSessionKey := "room:group:conversation-1"
	runtimeSessionKey := "agent:nexus:ws:group:conversation-1"
	service := &Service{goals: &fakeRoomGoalContextProvider{
		runtimeContexts: map[string]string{
			runtimeSessionKey: "runtime goal context",
		},
		runtimeGoals: map[string]*protocol.Goal{
			runtimeSessionKey: {
				ID:         "goal-runtime",
				SessionKey: runtimeSessionKey,
				Status:     protocol.GoalStatusActive,
			},
		},
	}}
	slot := &activeRoomSlot{RuntimeSessionKey: runtimeSessionKey}

	prompt, goalContext, goalID, goalSessionKey, _ := service.resolveGoalRuntimeContextForSlot(
		context.Background(),
		&activeRoomRound{SessionKey: sharedSessionKey},
		slot,
		"base prompt",
	)

	if goalID != "" || goalSessionKey != sharedSessionKey {
		t.Fatalf("goalID=%q goalSessionKey=%q, want empty goal on shared room session", goalID, goalSessionKey)
	}
	if prompt != "base prompt" {
		t.Fatalf("prompt = %q, want unchanged system prompt", prompt)
	}
	if goalContext != "" {
		t.Fatalf("goalContext = %q, want no private runtime goal fallback", goalContext)
	}
}

func TestResolveGoalRuntimeContextForSlotFallsBackToRuntimeGoalForLegacyRound(t *testing.T) {
	legacySessionKey := "legacy-room-session"
	runtimeSessionKey := "agent:nexus:ws:group:conversation-1"
	service := &Service{goals: &fakeRoomGoalContextProvider{
		runtimeContexts: map[string]string{
			runtimeSessionKey: "runtime goal context",
		},
		runtimeGoals: map[string]*protocol.Goal{
			runtimeSessionKey: {
				ID:         "goal-runtime",
				SessionKey: runtimeSessionKey,
				Status:     protocol.GoalStatusActive,
			},
		},
	}}
	slot := &activeRoomSlot{RuntimeSessionKey: runtimeSessionKey}

	prompt, goalContext, goalID, goalSessionKey, _ := service.resolveGoalRuntimeContextForSlot(
		context.Background(),
		&activeRoomRound{SessionKey: legacySessionKey},
		slot,
		"base prompt",
	)

	if goalID != "goal-runtime" || goalSessionKey != runtimeSessionKey {
		t.Fatalf("goalID=%q goalSessionKey=%q, want runtime goal fallback", goalID, goalSessionKey)
	}
	if prompt != "base prompt" {
		t.Fatalf("prompt = %q, want unchanged system prompt", prompt)
	}
	if !strings.Contains(goalContext, "runtime goal context") {
		t.Fatalf("goalContext = %q, want runtime goal context", goalContext)
	}
}

func TestResolveGoalRuntimeContextForSlotKeepsSharedSessionForFutureRoomGoal(t *testing.T) {
	sharedSessionKey := "room:group:conversation-1"
	runtimeSessionKey := "agent:nexus:ws:group:conversation-1"
	service := &Service{goals: &fakeRoomGoalContextProvider{}}
	slot := &activeRoomSlot{RuntimeSessionKey: runtimeSessionKey}

	prompt, goalContext, goalID, goalSessionKey, _ := service.resolveGoalRuntimeContextForSlot(
		context.Background(),
		&activeRoomRound{SessionKey: sharedSessionKey},
		slot,
		"base prompt",
	)

	if goalID != "" || goalContext != "" {
		t.Fatalf("goalID=%q goalContext=%q, want no current goal", goalID, goalContext)
	}
	if goalSessionKey != sharedSessionKey {
		t.Fatalf("goalSessionKey = %q, want shared session for future room goal", goalSessionKey)
	}
	if prompt != "base prompt" {
		t.Fatalf("prompt = %q, want unchanged system prompt", prompt)
	}
}

func TestClearGoalUsageForRoomSlotStopsLaterAccounting(t *testing.T) {
	goalProvider := &fakeRoomGoalContextProvider{}
	service := &Service{goals: goalProvider}
	slot := &activeRoomSlot{
		RuntimeSessionKey: "agent:nexus:ws:room:test",
		AgentRoundID:      "round-1",
	}
	slot.setGoalBinding("", "goal-1")
	slot.setGoalUsageAccumulator(goalsvc.NewRuntimeUsageAccumulator(true))

	clearGoalUsageForSlot(slot)
	service.recordGoalUsageForSlot(context.Background(), slot, exec.RoundExecutionResult{
		Usage: sdkprotocol.TokenUsage{
			InputTokens:  6,
			OutputTokens: 3,
			TotalTokens:  9,
		},
	}, nil)

	if usages := goalProvider.recordedUsage(); len(usages) != 0 {
		t.Fatalf("usages = %#v, want none after clear", usages)
	}
}

func TestActivateGoalUsageForRoomSlotRestartsFromCurrentSnapshot(t *testing.T) {
	goalProvider := &fakeRoomGoalContextProvider{}
	service := &Service{goals: goalProvider}
	slot := &activeRoomSlot{
		RuntimeSessionKey: "agent:nexus:ws:room:test",
		AgentRoundID:      "round-1",
	}
	slot.setGoalBinding("", "goal-1")
	slot.setGoalUsageAccumulator(goalsvc.NewRuntimeUsageAccumulator(true))

	service.recordGoalUsageFromSlotAssistantMessage(context.Background(), slot, roomGoalToolResultAssistantMessage("tool-1", "read_file", 4, 1))
	clearGoalUsageForSlot(slot)
	slot.rememberGoalAssistantMessage(roomGoalToolResultAssistantMessage("tool-2", "read_file", 7, 3))
	activateGoalUsageForSlot(context.Background(), slot)
	service.recordGoalUsageForSlot(context.Background(), slot, exec.RoundExecutionResult{
		Usage: sdkprotocol.TokenUsage{
			InputTokens:  10,
			OutputTokens: 5,
			TotalTokens:  15,
		},
	}, nil)

	usages := goalProvider.recordedUsage()
	if len(usages) != 2 {
		t.Fatalf("len(usages) = %d, want initial usage and post-activate delta", len(usages))
	}
	if usages[1].InputTokens != 3 || usages[1].OutputTokens != 2 || usages[1].Total() != 5 {
		t.Fatalf("post-activate usage = %#v, want 3/2", usages[1])
	}
}

func TestRecordGoalUsageLimitForRoomSlotUsesGoalSessionKey(t *testing.T) {
	goalProvider := &fakeRoomGoalContextProvider{}
	service := &Service{goals: goalProvider}
	slot := &activeRoomSlot{
		RuntimeSessionKey: "agent:nexus:ws:group:conversation-1",
		AgentRoundID:      "round-1",
	}
	slot.setGoalBinding("room:group:conversation-1", "")
	goalSessionKey := slot.goalSessionKey()

	service.recordGoalUsageLimitForSlot(context.Background(), slot, exec.RoundExecutionResult{
		UsageLimitReached: true,
		UsageLimitReason:  "The usage limit has been reached",
	})

	if len(goalProvider.usageLimitKeys) != 1 || goalProvider.usageLimitKeys[0] != goalSessionKey {
		t.Fatalf("usageLimitKeys = %#v, want shared goal session", goalProvider.usageLimitKeys)
	}
}

func TestRecordGoalUsageLimitForRoomSlot(t *testing.T) {
	goalProvider := &fakeRoomGoalContextProvider{}
	service := &Service{goals: goalProvider}
	slot := &activeRoomSlot{
		RuntimeSessionKey: "agent:nexus:ws:room:test",
		AgentRoundID:      "round-1",
	}

	service.recordGoalUsageLimitForSlot(context.Background(), slot, exec.RoundExecutionResult{
		UsageLimitReached: true,
		UsageLimitReason:  "The usage limit has been reached",
	})

	reasons := goalProvider.recordedUsageLimitReasons()
	if len(reasons) != 1 || reasons[0] != "The usage limit has been reached" {
		t.Fatalf("usage limit reasons = %#v, want runtime reason", reasons)
	}
}

func TestRoomSlotIgnoresGoalRuntimeInPlanMode(t *testing.T) {
	goalProvider := &fakeRoomGoalContextProvider{}
	service := &Service{goals: goalProvider}
	slot := &activeRoomSlot{
		RuntimeSessionKey: "room:agent:runtime",
		AgentRoundID:      "round-plan",
	}
	slot.setGoalBinding("room:group:conversation-1", "goal-plan")
	slot.setGoalRuntimeIgnored(true)
	slot.setGoalUsageAccumulator(goalsvc.NewRuntimeUsageAccumulator(true))

	beginGoalUsageForSlot(slot)
	service.recordGoalUsageFromSlotAssistantMessage(context.Background(), slot, roomGoalToolResultAssistantMessage("tool-1", "read_file", 4, 1))
	service.recordGoalUsageForSlot(context.Background(), slot, exec.RoundExecutionResult{
		Usage: sdkprotocol.TokenUsage{
			InputTokens:  10,
			OutputTokens: 2,
		},
		ElapsedTimeSeconds: 3,
	}, protocol.Message{})
	service.recordGoalUsageLimitForSlot(context.Background(), slot, exec.RoundExecutionResult{
		UsageLimitReached: true,
		UsageLimitReason:  "usage limit",
	})
	service.recordGoalContinuationProgressForSlot(context.Background(), slot, &activeRoomRound{
		InputOptions: sdkprotocol.OutboundMessageOptions{Purpose: "goal_continuation"},
	}, exec.RoundExecutionResult{}, nil)

	if usages := goalProvider.recordedUsage(); len(usages) != 0 {
		t.Fatalf("plan mode recorded room goal usage: %#v", usages)
	}
	if reasons := goalProvider.recordedUsageLimitReasons(); len(reasons) != 0 {
		t.Fatalf("plan mode recorded room usage limit: %#v", reasons)
	}
	if progress := goalProvider.recordedProgress(); len(progress) != 0 {
		t.Fatalf("plan mode recorded room continuation progress: %#v", progress)
	}
}

// Goal 运行时测试替身与消息构造器。

type fakeRoomGoalContextProvider struct {
	mu               sync.Mutex
	runtimeContexts  map[string]string
	runtimeGoals     map[string]*protocol.Goal
	usage            []protocol.GoalUsage
	usageSessionKeys []string
	usageLimitReason []string
	usageLimitKeys   []string
	progress         []bool
	failures         []string
	completionMisses []string
	activities       []string
	collabRequired   []string
	collabEvidence   []string
	plan             *protocol.GoalContinuation
	planCalls        int
	stillCurrent     bool
	claimCalls       int
	releaseCalls     int
	onPlan           func()
}

func (p *fakeRoomGoalContextProvider) RuntimeContext(_ context.Context, sessionKey string) (string, *protocol.Goal, error) {
	goal := p.runtimeGoals[sessionKey]
	if goal == nil {
		return "", nil, goalsvc.ErrGoalNotFound
	}
	value := *goal
	return p.runtimeContexts[sessionKey], &value, nil
}

func (p *fakeRoomGoalContextProvider) RecordUsageForSession(_ context.Context, sessionKey string, usage protocol.GoalUsage, _ string) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.usageSessionKeys = append(p.usageSessionKeys, sessionKey)
	p.usage = append(p.usage, usage)
	return nil, nil
}

func (p *fakeRoomGoalContextProvider) RecordUsageForGoal(_ context.Context, _ string, usage protocol.GoalUsage, _ string) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.usage = append(p.usage, usage)
	return nil, nil
}

func (p *fakeRoomGoalContextProvider) UsageLimitForSession(_ context.Context, sessionKey string, _ string, reason string) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.usageLimitKeys = append(p.usageLimitKeys, sessionKey)
	p.usageLimitReason = append(p.usageLimitReason, reason)
	return nil, nil
}

func (p *fakeRoomGoalContextProvider) RecordContinuationProgress(_ context.Context, _ string, _ string, progressed bool, _ ...int64) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.progress = append(p.progress, progressed)
	return nil, nil
}

func (p *fakeRoomGoalContextProvider) RecordContinuationFailure(_ context.Context, _ string, _ string, reason string, _ ...int64) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.failures = append(p.failures, strings.TrimSpace(reason))
	return nil, nil
}

func (p *fakeRoomGoalContextProvider) RecordCompletionToolMiss(_ context.Context, _ string, _ string, reason string, _ ...int64) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.completionMisses = append(p.completionMisses, strings.TrimSpace(reason))
	return nil, nil
}

func (p *fakeRoomGoalContextProvider) RecordGoalActivity(_ context.Context, _ string, roundID string, _ ...int64) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.activities = append(p.activities, strings.TrimSpace(roundID))
	return nil, nil
}

func (p *fakeRoomGoalContextProvider) RecordRoomGoalCollaborationRequired(_ context.Context, _ string, roundID string) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.collabRequired = append(p.collabRequired, strings.TrimSpace(roundID))
	return nil, nil
}

func (p *fakeRoomGoalContextProvider) RecordRoomGoalCollaborationEvidence(_ context.Context, _ string, roundID string, agentID string, _ ...int64) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.collabEvidence = append(p.collabEvidence, strings.TrimSpace(roundID)+":"+strings.TrimSpace(agentID))
	return nil, nil
}

func (p *fakeRoomGoalContextProvider) PlanContinuationForSession(context.Context, string, string) (*protocol.GoalContinuation, error) {
	p.mu.Lock()
	p.planCalls++
	onPlan := p.onPlan
	plan := p.plan
	p.mu.Unlock()
	if onPlan != nil {
		onPlan()
	}
	return plan, nil
}

func (p *fakeRoomGoalContextProvider) GoalContinuationStillCurrent(context.Context, protocol.GoalContinuation) (bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.stillCurrent, nil
}

func (p *fakeRoomGoalContextProvider) ClaimContinuationPlan(context.Context, protocol.GoalContinuation) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.claimCalls++
	return nil, nil
}

func (p *fakeRoomGoalContextProvider) ReleaseContinuationPlan(context.Context, protocol.GoalContinuation, string) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.releaseCalls++
	return nil, nil
}

func (p *fakeRoomGoalContextProvider) recordedUsage() []protocol.GoalUsage {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]protocol.GoalUsage(nil), p.usage...)
}

func (p *fakeRoomGoalContextProvider) recordedUsageLimitReasons() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.usageLimitReason...)
}

func (p *fakeRoomGoalContextProvider) recordedProgress() []bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]bool(nil), p.progress...)
}

func (p *fakeRoomGoalContextProvider) recordedFailures() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.failures...)
}

func (p *fakeRoomGoalContextProvider) recordedCompletionMisses() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.completionMisses...)
}

func roomGoalToolResultAssistantMessage(
	toolUseID string,
	toolName string,
	inputTokens int64,
	outputTokens int64,
) protocol.Message {
	return protocol.Message{
		"role": "assistant",
		"usage": map[string]any{
			"input_tokens":  inputTokens,
			"output_tokens": outputTokens,
			"total_tokens":  inputTokens + outputTokens,
		},
		"content": []map[string]any{
			{"type": "tool_use", "id": toolUseID, "name": toolName},
			{"type": "tool_result", "tool_use_id": toolUseID},
		},
	}
}

func roomGoalCompletionToolMissAssistantMessage() protocol.Message {
	return protocol.Message{
		"role": "assistant",
		"content": []map[string]any{
			{"type": "text", "text": "任务已经完成，但我没有看到 mcp__nexus_goal__update_goal 工具，无法调用它来标记完成。"},
		},
	}
}

func roomGoalTextAssistantMessage(messageID string, text string) protocol.Message {
	return protocol.Message{
		"message_id": messageID,
		"role":       "assistant",
		"content": []map[string]any{
			{"type": "text", "text": text},
		},
	}
}

func roomGoalAssistantUsageMessage(inputTokens int64, outputTokens int64) protocol.Message {
	return protocol.Message{
		"role": "assistant",
		"usage": map[string]any{
			"input_tokens":  inputTokens,
			"output_tokens": outputTokens,
			"total_tokens":  inputTokens + outputTokens,
		},
	}
}

const (
	realRoomCancellationContent = "@Amy 算了不用了"
	realRoomSessionKey          = "room:group:91c68883cc96"
	realGoalID                  = "goal-real-room-review"
)

type cancellationGoalProvider struct {
	current   *protocol.Goal
	clearCall int
	planCall  int
}

func (p *cancellationGoalProvider) CurrentOptional(context.Context, string) (*protocol.Goal, error) {
	return p.current, nil
}

func (p *cancellationGoalProvider) Clear(_ context.Context, goalID string) (bool, error) {
	p.clearCall++
	if p.current == nil || p.current.ID != goalID {
		return false, nil
	}
	p.current = nil
	return true, nil
}

func (p *cancellationGoalProvider) PlanContinuationForSession(context.Context, string, string) (*protocol.GoalContinuation, error) {
	p.planCall++
	if p.current == nil {
		return nil, nil
	}
	return &protocol.GoalContinuation{
		Goal:    *p.current,
		RoundID: "goal_continuation_after_cancel",
	}, nil
}

func (p *cancellationGoalProvider) GoalContinuationStillCurrent(context.Context, protocol.GoalContinuation) (bool, error) {
	return p.current != nil, nil
}

func (p *cancellationGoalProvider) ClaimContinuationPlan(context.Context, protocol.GoalContinuation) (*protocol.Goal, error) {
	return p.current, nil
}

func (p *cancellationGoalProvider) RuntimeContext(context.Context, string) (string, *protocol.Goal, error) {
	return "", p.current, nil
}

func (p *cancellationGoalProvider) RecordUsageForSession(context.Context, string, protocol.GoalUsage, string) (*protocol.Goal, error) {
	return p.current, nil
}

func (p *cancellationGoalProvider) RecordUsageForGoal(context.Context, string, protocol.GoalUsage, string) (*protocol.Goal, error) {
	return p.current, nil
}

func (p *cancellationGoalProvider) UsageLimitForSession(context.Context, string, string, string) (*protocol.Goal, error) {
	return p.current, nil
}

func (p *cancellationGoalProvider) RecordContinuationProgress(context.Context, string, string, bool, ...int64) (*protocol.Goal, error) {
	return p.current, nil
}

func (p *cancellationGoalProvider) RecordContinuationFailure(context.Context, string, string, string, ...int64) (*protocol.Goal, error) {
	return p.current, nil
}

func (p *cancellationGoalProvider) RecordCompletionToolMiss(context.Context, string, string, string, ...int64) (*protocol.Goal, error) {
	return p.current, nil
}

func (p *cancellationGoalProvider) RecordGoalActivity(context.Context, string, string, ...int64) (*protocol.Goal, error) {
	return p.current, nil
}

func (p *cancellationGoalProvider) RecordRoomGoalCollaborationRequired(context.Context, string, string) (*protocol.Goal, error) {
	return p.current, nil
}

func (p *cancellationGoalProvider) RecordRoomGoalCollaborationEvidence(context.Context, string, string, string, ...int64) (*protocol.Goal, error) {
	return p.current, nil
}

func TestRealRoomCancellationClearsGoalBeforeContinuation(t *testing.T) {
	provider := &cancellationGoalProvider{current: &protocol.Goal{
		ID:         realGoalID,
		SessionKey: realRoomSessionKey,
		Status:     protocol.GoalStatusActive,
	}}
	service := &Service{goals: provider}

	if !isGoalCancellationRequest(realRoomCancellationContent) {
		t.Fatal("真实 Room 引导内容应被识别为明确取消意图")
	}
	if err := service.cancelActiveRoomGoalForUser(
		context.Background(),
		realRoomSessionKey,
		realRoomCancellationContent,
	); err != nil {
		t.Fatalf("清除 active Goal 失败: %v", err)
	}
	if provider.clearCall != 1 || provider.current != nil {
		t.Fatalf("取消应只清除一次 active Goal: calls=%d current=%+v", provider.clearCall, provider.current)
	}

	service.dispatchPostRoundWork(context.Background(), &activeRoomRound{
		SessionKey: realRoomSessionKey,
		RoundID:    "round_after_cancel",
	})
	if provider.planCall != 1 {
		t.Fatalf("取消后应只检查一次续跑且不生成计划: planCall=%d", provider.planCall)
	}
}

func TestGoalCancellationIntentDoesNotMatchOrdinaryDiscussion(t *testing.T) {
	for _, content := range []string{
		"停止后继续执行",
		"请说明任务为什么停止",
		"这个任务已经完成",
	} {
		if isGoalCancellationRequest(content) {
			t.Fatalf("普通讨论不应被识别为取消: %q", content)
		}
	}
}

func TestPublishPublicMessageSuppressesTheSameSlotFinalReply(t *testing.T) {
	slot := &activeRoomSlot{
		AgentID: "agent-amy",
	}
	slot.setPendingStream([]protocol.EventMessage{{EventType: protocol.EventTypeStream}})
	slot.beginNoReplyCandidate()
	service := &Service{rounds: newRoomRoundRegistryFromRounds(map[string]*activeRoomRound{
		"round-1": {
			SessionKey:  "room:group:conversation-1",
			RootRoundID: "round-1",
			Slots: map[string]*activeRoomSlot{
				"slot-1": slot,
			},
		},
	})}

	if err := service.MarkPublicMessagePublished(
		context.Background(),
		"room:group:conversation-1",
		"round-1",
		"agent-amy",
	); err != nil {
		t.Fatalf("标记主动广播失败: %v", err)
	}
	if !slot.publicMessageWasPublished() || !slot.shouldSuppressOutput() {
		t.Fatalf("主动广播后 slot 必须进入 suppress 状态: %+v", slot)
	}
	if events := slot.eventsReadyForEmission(protocol.EventMessage{EventType: protocol.EventTypeStream}); len(events) != 0 {
		t.Fatalf("主动广播后不应继续向公区发流事件: %+v", events)
	}
}

// Goal 完成就绪测试。

func TestActiveRoomGoalBlockerExcludesCallerSlotButKeepsRunningWork(t *testing.T) {
	const conversationID = "conversation-goal-ready"
	sessionKey := protocol.BuildRoomSharedSessionKey(conversationID)
	caller := &activeRoomSlot{
		AgentID:      "agent-lead",
		AgentRoundID: "agent-round-lead",
	}
	caller.setStatus("running")
	roundValue := &activeRoomRound{
		SessionKey:     sessionKey,
		ConversationID: conversationID,
		RoundID:        "room-round",
		RootRoundID:    "root-round",
		Slots:          map[string]*activeRoomSlot{"caller": caller},
	}
	service := &Service{rounds: newRoomRoundRegistryFromRounds(map[string]*activeRoomRound{"round": roundValue})}

	if blocker := service.activeRoomGoalBlocker(sessionKey, conversationID, "agent-lead", "agent-round-lead"); blocker != "" {
		t.Fatalf("caller current slot blocker = %q, want empty", blocker)
	}
	if blocker := service.activeRoomGoalBlocker(sessionKey, conversationID, "agent-lead", ""); !strings.Contains(blocker, "active Room slot") {
		t.Fatalf("caller without precise round blocker = %q, want fail-closed active slot", blocker)
	}

	caller.setSubagentTasks(map[string]struct{}{"task-running": {}})
	if blocker := service.activeRoomGoalBlocker(sessionKey, conversationID, "agent-lead", "agent-round-lead"); !strings.Contains(blocker, "running subagent work") {
		t.Fatalf("caller subagent blocker = %q, want running subagent work", blocker)
	}
	caller.setSubagentTasks(nil)

	peer := &activeRoomSlot{AgentID: "agent-peer", AgentRoundID: "agent-round-peer"}
	peer.setStatus("running")
	roundValue.Slots["peer"] = peer
	if blocker := service.activeRoomGoalBlocker(sessionKey, conversationID, "agent-lead", "agent-round-lead"); !strings.Contains(blocker, "agent-peer") {
		t.Fatalf("peer slot blocker = %q, want active peer", blocker)
	}

	peer.setStatus("finished")
	peer.setSubagentTasks(map[string]struct{}{"peer-task": {}})
	if blocker := service.activeRoomGoalBlocker(sessionKey, conversationID, "agent-lead", "agent-round-lead"); !strings.Contains(blocker, "agent-peer still has running subagent work") {
		t.Fatalf("peer subagent blocker = %q, want peer subagent even after main slot terminal", blocker)
	}
	peer.setSubagentTasks(nil)

	service.rounds.enqueuePublicMention(roundValue, publicMentionWake{TargetAgentID: "agent-peer"})
	if blocker := service.activeRoomGoalBlocker(sessionKey, conversationID, "agent-lead", "agent-round-lead"); !strings.Contains(blocker, "public-mention wake") {
		t.Fatalf("public mention blocker = %q, want pending wake", blocker)
	}
}

func TestRoomGoalInputQueueBlockerClearsOnlyAfterConsumption(t *testing.T) {
	root := t.TempDir()
	store := workspacestore.NewInputQueueStore(root)
	const (
		conversationID = "conversation-goal-queue"
		roomID         = "room-goal-queue"
		agentID        = "agent-peer"
	)
	location := workspacestore.InputQueueLocation{
		Scope:          protocol.InputQueueScopeRoom,
		WorkspacePath:  root,
		SessionKey:     protocol.BuildRoomAgentSessionKey(conversationID, agentID, protocol.RoomTypeGroup),
		RoomID:         roomID,
		ConversationID: conversationID,
	}
	if _, err := store.Enqueue(location, protocol.InputQueueItem{
		ID:             "queued-directed-message",
		AgentID:        agentID,
		SourceAgentID:  "agent-lead",
		Source:         protocol.InputQueueSourceAgentRoomMessage,
		Content:        "continue the delegated comparison",
		DeliveryPolicy: protocol.ChatDeliveryPolicyGuide,
	}); err != nil {
		t.Fatal(err)
	}
	contextValue := &protocol.ConversationContextAggregate{
		Room:         protocol.RoomRecord{ID: roomID, RoomType: protocol.RoomTypeGroup},
		Conversation: protocol.ConversationRecord{ID: conversationID, RoomID: roomID},
		Members: []protocol.MemberRecord{{
			MemberType: protocol.MemberTypeAgent, MemberAgentID: agentID,
		}},
		MemberAgents: []protocol.Agent{{AgentID: agentID, WorkspacePath: root}},
	}
	service := &Service{inputQueue: store}

	blocker, err := service.roomGoalInputQueueBlocker(context.Background(), contextValue)
	if err != nil || !strings.Contains(blocker, "queued-directed-message") {
		t.Fatalf("queued blocker = %q err=%v, want pending item", blocker, err)
	}
	if _, err = store.Dispatch(location, "queued-directed-message"); err != nil {
		t.Fatal(err)
	}
	blocker, err = service.roomGoalInputQueueBlocker(context.Background(), contextValue)
	if err != nil || blocker != "" {
		t.Fatalf("dispatched blocker = %q err=%v, want empty", blocker, err)
	}
}

func TestRoomGoalDelayedWakeBlockerClearsAfterWakeStarts(t *testing.T) {
	root := t.TempDir()
	t.Setenv("NEXUS_CONFIG_DIR", root)
	store := workspacestore.NewRoomDirectedMessageWakeStore(root)
	const conversationID = "conversation-goal-delayed-wake"
	wake := workspacestore.RoomDirectedMessageWake{
		WakeID: "wake-goal",
		Message: protocol.RoomDirectedMessageRecord{
			MessageID:      "wake-goal",
			RoomID:         "room-goal",
			ConversationID: conversationID,
			WakePolicy:     protocol.RoomWakePolicyDelayed,
		},
		DueAt: time.Now().Add(time.Minute).UnixMilli(),
	}
	if err := store.Schedule(wake); err != nil {
		t.Fatal(err)
	}
	service := &Service{directedWakes: store}

	blocker, err := service.roomGoalDelayedWakeBlocker(conversationID)
	if err != nil || !strings.Contains(blocker, wake.WakeID) {
		t.Fatalf("pending wake blocker = %q err=%v, want wake ID", blocker, err)
	}
	if err = store.Complete(wake.WakeID); err != nil {
		t.Fatal(err)
	}
	blocker, err = service.roomGoalDelayedWakeBlocker(conversationID)
	if err != nil || blocker != "" {
		t.Fatalf("completed wake blocker = %q err=%v, want empty", blocker, err)
	}
}
