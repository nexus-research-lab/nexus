package room

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	agentclient "github.com/nexus-research-lab/nexus-agent-sdk-bridge/client"
	sdkhook "github.com/nexus-research-lab/nexus-agent-sdk-bridge/hook"
	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	exec "github.com/nexus-research-lab/nexus/internal/runtime/exec"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
)

func TestRecordGoalUsageForRoomSlotUsesToolCompletionDelta(t *testing.T) {
	goalProvider := &fakeRoomGoalContextProvider{}
	service := &RealtimeService{goals: goalProvider}
	slot := &activeRoomSlot{
		RuntimeSessionKey: "agent:nexus:ws:room:test",
		AgentRoundID:      "round-1",
		GoalIDForUsage:    "goal-1",
		GoalUsage:         goalsvc.NewRuntimeUsageAccumulator(true),
	}

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
	service := &RealtimeService{goals: goalProvider}
	slot := &activeRoomSlot{
		RuntimeSessionKey: "agent:nexus:ws:room:test",
		AgentRoundID:      "round-1",
		GoalIDForUsage:    "goal-1",
		GoalUsage:         goalsvc.NewRuntimeUsageAccumulator(true),
	}

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
			service := &RealtimeService{goals: goalProvider}
			slot := &activeRoomSlot{
				RuntimeSessionKey: "agent:nexus:ws:group:conversation-1",
				GoalSessionKey:    sharedSessionKey,
				AgentRoundID:      "round-1:agent-1",
				GoalUsage:         goalsvc.NewRuntimeUsageAccumulator(false),
			}

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
	service := &RealtimeService{runtime: manager}
	slot := &activeRoomSlot{
		RuntimeSessionKey: "agent:nexus:ws:room:conversation-1:agent-1",
		AgentRoundID:      "room-round-1:agent-1",
	}

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
	if _, err := manager.QueueGuidanceInput(context.Background(), slot.RuntimeSessionKey, "goal-event-2", "late guidance"); !errors.Is(err, runtimectx.ErrNoRunningRound) {
		t.Fatalf("QueueGuidanceInput() after cleanup error = %v, want ErrNoRunningRound", err)
	}
}

func TestRegisterSlotGoalRuntimeUsesGoalSessionKey(t *testing.T) {
	manager := runtimectx.NewManager()
	service := &RealtimeService{runtime: manager}
	slot := &activeRoomSlot{
		RuntimeSessionKey: "agent:nexus:ws:group:conversation-1",
		GoalSessionKey:    "room:group:conversation-1",
		AgentRoundID:      "room-round-1:agent-1",
	}

	cleanup := service.registerSlotGoalRuntime(slot)
	roundIDs, err := manager.QueueGuidanceInput(context.Background(), slot.GoalSessionKey, "goal-event-1", "budget reached")
	if err != nil {
		t.Fatalf("QueueGuidanceInput() error = %v", err)
	}
	if len(roundIDs) != 1 || roundIDs[0] != slot.AgentRoundID {
		t.Fatalf("roundIDs = %#v, want slot round", roundIDs)
	}
	if count := manager.PendingGuidanceCount(slot.GoalSessionKey); count != 1 {
		t.Fatalf("PendingGuidanceCount = %d, want 1", count)
	}

	cleanup()
	if _, err := manager.QueueGuidanceInput(context.Background(), slot.GoalSessionKey, "goal-event-2", "late guidance"); !errors.Is(err, runtimectx.ErrNoRunningRound) {
		t.Fatalf("QueueGuidanceInput() after cleanup error = %v, want ErrNoRunningRound", err)
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
	service := &RealtimeService{
		runtime: manager,
		activeRounds: map[string]*activeRoomRound{
			"round-root": {
				SessionKey:  sessionKey,
				RoundID:     "round-root",
				RootRoundID: "round-root",
				Slots: map[string]*activeRoomSlot{
					lead.AgentID:   lead,
					caller.AgentID: caller,
				},
			},
		},
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
	service := &RealtimeService{
		runtime: manager,
		activeRounds: map[string]*activeRoomRound{
			"round-root": {
				SessionKey:  sessionKey,
				RoundID:     "round-root",
				RootRoundID: "round-root",
				Slots: map[string]*activeRoomSlot{
					unavailable.AgentID: unavailable,
					active.AgentID:      active,
				},
			},
		},
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
	service := &RealtimeService{goals: &fakeRoomGoalContextProvider{
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
	service := &RealtimeService{goals: &fakeRoomGoalContextProvider{
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
	service := &RealtimeService{goals: &fakeRoomGoalContextProvider{
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
	service := &RealtimeService{goals: &fakeRoomGoalContextProvider{
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
	service := &RealtimeService{goals: &fakeRoomGoalContextProvider{}}
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
	service := &RealtimeService{goals: goalProvider}
	slot := &activeRoomSlot{
		RuntimeSessionKey: "agent:nexus:ws:room:test",
		AgentRoundID:      "round-1",
		GoalIDForUsage:    "goal-1",
		GoalUsage:         goalsvc.NewRuntimeUsageAccumulator(true),
	}

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
	service := &RealtimeService{goals: goalProvider}
	slot := &activeRoomSlot{
		RuntimeSessionKey: "agent:nexus:ws:room:test",
		AgentRoundID:      "round-1",
		GoalIDForUsage:    "goal-1",
		GoalUsage:         goalsvc.NewRuntimeUsageAccumulator(true),
	}

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
	service := &RealtimeService{goals: goalProvider}
	slot := &activeRoomSlot{
		RuntimeSessionKey: "agent:nexus:ws:group:conversation-1",
		GoalSessionKey:    "room:group:conversation-1",
		AgentRoundID:      "round-1",
	}

	service.recordGoalUsageLimitForSlot(context.Background(), slot, exec.RoundExecutionResult{
		UsageLimitReached: true,
		UsageLimitReason:  "The usage limit has been reached",
	})

	if len(goalProvider.usageLimitKeys) != 1 || goalProvider.usageLimitKeys[0] != slot.GoalSessionKey {
		t.Fatalf("usageLimitKeys = %#v, want shared goal session", goalProvider.usageLimitKeys)
	}
}

func TestRecordGoalUsageLimitForRoomSlot(t *testing.T) {
	goalProvider := &fakeRoomGoalContextProvider{}
	service := &RealtimeService{goals: goalProvider}
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
	service := &RealtimeService{goals: goalProvider}
	slot := &activeRoomSlot{
		RuntimeSessionKey:  "room:agent:runtime",
		GoalSessionKey:     "room:group:conversation-1",
		AgentRoundID:       "round-plan",
		GoalIDForUsage:     "goal-plan",
		GoalRuntimeIgnored: true,
		GoalUsage:          goalsvc.NewRuntimeUsageAccumulator(true),
		GoalUsageStartedAt: time.Now(),
	}

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
