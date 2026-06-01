package room

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
)

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
	activities       []string
	plan             *protocol.GoalContinuation
	planCalls        int
	stillCurrent     bool
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

func (p *fakeRoomGoalContextProvider) RecordContinuationProgress(_ context.Context, _ string, _ string, progressed bool) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.progress = append(p.progress, progressed)
	return nil, nil
}

func (p *fakeRoomGoalContextProvider) RecordContinuationFailure(_ context.Context, _ string, _ string, reason string) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.failures = append(p.failures, strings.TrimSpace(reason))
	return nil, nil
}

func (p *fakeRoomGoalContextProvider) RecordGoalActivity(_ context.Context, _ string, roundID string) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.activities = append(p.activities, strings.TrimSpace(roundID))
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

func (p *fakeRoomGoalContextProvider) ReleaseContinuationPlan(context.Context, protocol.GoalContinuation, string) (*protocol.Goal, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.releaseCalls++
	return nil, nil
}

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
	service.recordGoalUsageForSlot(context.Background(), slot, runtimectx.RoundExecutionResult{
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
	service.recordGoalUsageForSlot(context.Background(), slot, runtimectx.RoundExecutionResult{}, roomGoalAssistantUsageMessage(9, 4))

	usages := goalProvider.recordedUsage()
	if len(usages) != 2 {
		t.Fatalf("len(usages) = %d, want 2", len(usages))
	}
	if usages[1].InputTokens != 5 || usages[1].OutputTokens != 3 || usages[1].Total() != 8 {
		t.Fatalf("abort usage = %#v, want remaining 5/3", usages[1])
	}
}

func TestRoomSlotRecordsUsageToSharedGoalAfterCreateGoalTool(t *testing.T) {
	sharedSessionKey := "room:group:conversation-1"
	goalProvider := &fakeRoomGoalContextProvider{}
	service := &RealtimeService{goals: goalProvider}
	slot := &activeRoomSlot{
		RuntimeSessionKey: "agent:nexus:ws:group:conversation-1",
		GoalSessionKey:    sharedSessionKey,
		AgentRoundID:      "round-1:agent-1",
		GoalUsage:         goalsvc.NewRuntimeUsageAccumulator(false),
	}

	service.recordGoalUsageFromSlotAssistantMessage(context.Background(), slot, roomGoalToolResultAssistantMessage("tool-1", "create_goal", 4, 1))
	service.recordGoalUsageForSlot(context.Background(), slot, runtimectx.RoundExecutionResult{
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
}

func TestRoomSlotRecordsUsageToSharedGoalAfterMCPCreateGoalTool(t *testing.T) {
	sharedSessionKey := "room:group:conversation-1"
	goalProvider := &fakeRoomGoalContextProvider{}
	service := &RealtimeService{goals: goalProvider}
	slot := &activeRoomSlot{
		RuntimeSessionKey: "agent:nexus:ws:group:conversation-1",
		GoalSessionKey:    sharedSessionKey,
		AgentRoundID:      "round-1:agent-1",
		GoalUsage:         goalsvc.NewRuntimeUsageAccumulator(false),
	}

	service.recordGoalUsageFromSlotAssistantMessage(context.Background(), slot, roomGoalToolResultAssistantMessage("tool-1", "mcp__nexus_goal__create_goal", 4, 1))
	service.recordGoalUsageForSlot(context.Background(), slot, runtimectx.RoundExecutionResult{
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
		t.Fatalf("usage = %#v, want 5/2 delta after MCP create_goal baseline", usages[0])
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
			},
			runtimeSessionKey: {
				ID:         "goal-runtime",
				SessionKey: runtimeSessionKey,
				Status:     protocol.GoalStatusActive,
			},
		},
	}}
	slot := &activeRoomSlot{RuntimeSessionKey: runtimeSessionKey}

	prompt, goalContext, goalID, goalSessionKey := service.resolveGoalRuntimeContextForSlot(
		context.Background(),
		&activeRoomRound{SessionKey: sharedSessionKey},
		slot,
		"base prompt",
	)

	if goalID != "goal-shared" || goalSessionKey != sharedSessionKey {
		t.Fatalf("goalID=%q goalSessionKey=%q, want shared goal", goalID, goalSessionKey)
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

	prompt, goalContext, goalID, goalSessionKey := service.resolveGoalRuntimeContextForSlot(
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

	prompt, goalContext, goalID, goalSessionKey := service.resolveGoalRuntimeContextForSlot(
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

	prompt, goalContext, goalID, goalSessionKey := service.resolveGoalRuntimeContextForSlot(
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

	prompt, goalContext, goalID, goalSessionKey := service.resolveGoalRuntimeContextForSlot(
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
	service.recordGoalUsageForSlot(context.Background(), slot, runtimectx.RoundExecutionResult{
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
	service.recordGoalUsageForSlot(context.Background(), slot, runtimectx.RoundExecutionResult{
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

	service.recordGoalUsageLimitForSlot(context.Background(), slot, runtimectx.RoundExecutionResult{
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

	service.recordGoalUsageLimitForSlot(context.Background(), slot, runtimectx.RoundExecutionResult{
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
	service.recordGoalUsageForSlot(context.Background(), slot, runtimectx.RoundExecutionResult{
		Usage: sdkprotocol.TokenUsage{
			InputTokens:  10,
			OutputTokens: 2,
		},
		ElapsedTimeSeconds: 3,
	}, protocol.Message{})
	service.recordGoalUsageLimitForSlot(context.Background(), slot, runtimectx.RoundExecutionResult{
		UsageLimitReached: true,
		UsageLimitReason:  "usage limit",
	})
	service.recordGoalContinuationProgressForSlot(context.Background(), slot, &activeRoomRound{
		InputOptions: sdkprotocol.OutboundMessageOptions{Purpose: "goal_continuation"},
	}, runtimectx.RoundExecutionResult{}, nil)

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

func TestRecordGoalContinuationProgressForRoomSlotSuppressesEmptyContinuation(t *testing.T) {
	goalProvider := &fakeRoomGoalContextProvider{}
	service := &RealtimeService{goals: goalProvider}
	slot := &activeRoomSlot{
		RuntimeSessionKey: "agent:nexus:ws:room:test",
		AgentRoundID:      "goal_continuation_1",
		GoalIDForUsage:    "goal-1",
	}
	roundValue := &activeRoomRound{
		InputOptions: sdkprotocol.OutboundMessageOptions{
			Purpose: "goal_continuation",
		},
	}

	service.recordGoalContinuationProgressForSlot(context.Background(), slot, roundValue, runtimectx.RoundExecutionResult{}, nil)

	progress := goalProvider.recordedProgress()
	if len(progress) != 1 || progress[0] {
		t.Fatalf("progress = %#v, want one false continuation progress", progress)
	}
}

func TestRecordGoalContinuationProgressForRoomSlotRecordsFailure(t *testing.T) {
	goalProvider := &fakeRoomGoalContextProvider{}
	service := &RealtimeService{goals: goalProvider}
	slot := &activeRoomSlot{
		RuntimeSessionKey: "agent:nexus:ws:room:test",
		AgentRoundID:      "goal_continuation_1",
		GoalIDForUsage:    "goal-1",
	}
	roundValue := &activeRoomRound{
		InputOptions: sdkprotocol.OutboundMessageOptions{
			Purpose: "goal_continuation",
		},
	}

	service.recordGoalContinuationProgressForSlot(
		context.Background(),
		slot,
		roundValue,
		runtimectx.RoundExecutionResult{
			TerminalStatus: "error",
			ResultSubtype:  "error",
			ErrorMessage:   "Failed to authenticate. API Error: 401",
		},
		nil,
	)

	failures := goalProvider.recordedFailures()
	if len(failures) != 1 || failures[0] != "Failed to authenticate. API Error: 401" {
		t.Fatalf("failures = %#v, want provider error", failures)
	}
	if progress := goalProvider.recordedProgress(); len(progress) != 0 {
		t.Fatalf("progress = %#v, want failure path instead of empty progress", progress)
	}
}

func TestRecordGoalContinuationProgressForRoomSlotCountsToolProgress(t *testing.T) {
	goalProvider := &fakeRoomGoalContextProvider{}
	service := &RealtimeService{goals: goalProvider}
	slot := &activeRoomSlot{
		RuntimeSessionKey: "agent:nexus:ws:room:test",
		AgentRoundID:      "goal_continuation_1",
		GoalIDForUsage:    "goal-1",
		GoalUsage:         goalsvc.NewRuntimeUsageAccumulator(true),
	}
	roundValue := &activeRoomRound{
		InputOptions: sdkprotocol.OutboundMessageOptions{
			Purpose: "goal_continuation",
		},
	}

	service.recordGoalUsageFromSlotAssistantMessage(context.Background(), slot, roomGoalToolResultAssistantMessage("tool-1", "read_file", 4, 1))
	service.recordGoalContinuationProgressForSlot(context.Background(), slot, roundValue, runtimectx.RoundExecutionResult{}, nil)

	progress := goalProvider.recordedProgress()
	if len(progress) != 1 || !progress[0] {
		t.Fatalf("progress = %#v, want one true continuation progress", progress)
	}
}

func TestRecordGoalContinuationProgressForRoomSlotRecordsUserActivity(t *testing.T) {
	goalProvider := &fakeRoomGoalContextProvider{}
	service := &RealtimeService{goals: goalProvider}
	slot := &activeRoomSlot{
		RuntimeSessionKey: "agent:nexus:ws:room:test",
		AgentRoundID:      "round-user",
		GoalIDForUsage:    "goal-1",
	}
	roundValue := &activeRoomRound{}

	service.recordGoalContinuationProgressForSlot(context.Background(), slot, roundValue, runtimectx.RoundExecutionResult{}, nil)

	goalProvider.mu.Lock()
	defer goalProvider.mu.Unlock()
	if len(goalProvider.activities) != 1 || goalProvider.activities[0] != "round-user" {
		t.Fatalf("activities = %#v, want explicit room goal activity", goalProvider.activities)
	}
	if len(goalProvider.progress) != 0 {
		t.Fatalf("progress = %#v, want no continuation progress for user room round", goalProvider.progress)
	}
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
