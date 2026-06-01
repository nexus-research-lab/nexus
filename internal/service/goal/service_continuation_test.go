package goal

import (
	"context"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestServiceQueuesObjectiveUpdateSteering(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	dispatcher := &fakeGuidanceDispatcher{}
	service.SetGuidanceDispatcher(dispatcher)
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Original",
	})
	if err != nil {
		t.Fatal(err)
	}
	updatedObjective := "Updated <goal>"
	if _, err := service.Update(ctx, created.ID, protocol.UpdateGoalRequest{Objective: &updatedObjective}); err != nil {
		t.Fatal(err)
	}
	if len(dispatcher.items) != 1 {
		t.Fatalf("guidance = %#v, want one objective update steering item", dispatcher.items)
	}
	item := dispatcher.items[0]
	if item.sessionKey != created.SessionKey ||
		item.contextName != "goal" ||
		!strings.Contains(item.content, "active thread goal objective") ||
		!strings.Contains(item.content, "existing, tracked Goal") ||
		!strings.Contains(item.content, "Updated &lt;goal&gt;") ||
		strings.Contains(item.content, "Nexus Goal") {
		t.Fatalf("guidance item = %#v, want escaped objective update steering", item)
	}
}

func TestServiceUpdateSameObjectiveDoesNotQueueObjectiveSteering(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	dispatcher := &fakeGuidanceDispatcher{}
	service.SetGuidanceDispatcher(dispatcher)
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Same objective",
	})
	if err != nil {
		t.Fatal(err)
	}
	sameObjective := "Same objective"
	updated, err := service.Update(ctx, created.ID, protocol.UpdateGoalRequest{Objective: &sameObjective})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Version != created.Version {
		t.Fatalf("updated version = %d, want unchanged %d", updated.Version, created.Version)
	}
	if len(dispatcher.items) != 0 {
		t.Fatalf("guidance = %#v, want no objective update steering", dispatcher.items)
	}
	if len(repo.events) != 1 {
		t.Fatalf("events = %#v, want no update event for unchanged objective", repo.events)
	}
}

func TestServiceQueuesBudgetLimitSteering(t *testing.T) {
	repo := newMemoryRepository()
	budget := int64(10)
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	dispatcher := &fakeGuidanceDispatcher{}
	service.SetGuidanceDispatcher(dispatcher)
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey:  "agent:nexus:ws:dm:chat",
		Objective:   "Budget <work>",
		TokenBudget: &budget,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.RecordUsageForSession(ctx, created.SessionKey, protocol.GoalUsage{TotalTokens: 10}, "round-1"); err != nil {
		t.Fatal(err)
	}
	if len(dispatcher.items) != 1 {
		t.Fatalf("guidance = %#v, want one budget limit steering item", dispatcher.items)
	}
	if item := dispatcher.items[0]; item.sessionKey != created.SessionKey ||
		item.contextName != "goal" ||
		!strings.Contains(item.content, "active thread goal") ||
		!strings.Contains(item.content, "existing, tracked Goal") ||
		!strings.Contains(item.content, "reached its token budget") ||
		!strings.Contains(item.content, "<objective>") ||
		!strings.Contains(item.content, "</objective>") ||
		!strings.Contains(item.content, "budget_limited") ||
		!strings.Contains(item.content, "Budget &lt;work&gt;") ||
		strings.Contains(item.content, "<untrusted_objective>") ||
		strings.Contains(item.content, "Budget <work>") ||
		strings.Contains(item.content, "Nexus Goal") {
		t.Fatalf("guidance item = %#v, want budget limit steering", item)
	}
}

func TestServicePlanContinuationForSession(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{
		GoalEnabled:                true,
		GoalAutoContinueEnabled:    true,
		GoalMaxContinuationsPerRun: 3,
	}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Complete parity",
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := service.PlanContinuationForSession(ctx, created.SessionKey, "round-1")
	if err != nil {
		t.Fatal(err)
	}
	if plan == nil || plan.RoundID != "goal_continuation_3" {
		t.Fatalf("plan = %#v, want hidden continuation round", plan)
	}
	if !plan.HiddenFromUser || !plan.Synthetic || plan.Purpose != goalContinuationPurpose {
		t.Fatalf("plan visibility = %#v, want hidden synthetic goal continuation", plan)
	}
	for _, want := range []string{
		"Continue working toward the active thread goal.",
		"Runtime note: this is an existing, tracked Goal",
		"First compare the current state against the objective.",
		"choose the next concrete, evidence-backed step and execute it",
		"Do not ask the user which direction to take when there is an obvious next step",
		"Do not mention hidden continuations",
		"Complete parity",
		"Completion audit:",
		"Blocked audit:",
		"If the runtime exposes the Goal update tool under a qualified MCP name",
		"Tokens remaining:",
	} {
		if !strings.Contains(plan.Prompt, want) {
			t.Fatalf("continuation prompt missing %q: %s", want, plan.Prompt)
		}
	}
	for _, forbidden := range []string{"active Nexus Goal", "Nexus runtime:", "PreviousRoundID:"} {
		if strings.Contains(plan.Prompt, forbidden) {
			t.Fatalf("continuation prompt contains legacy runtime wording %q: %s", forbidden, plan.Prompt)
		}
	}
	if strings.Contains(strings.ToLower(plan.Prompt), "absence of a new user message") {
		t.Fatalf("continuation prompt should not mention missing user messages: %s", plan.Prompt)
	}
	current, err := service.Current(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current.ContinuationCount != 1 {
		t.Fatalf("ContinuationCount = %d, want 1", current.ContinuationCount)
	}
	if len(repo.events) != 2 || repo.events[1].EventType != "continuation_scheduled" || repo.events[1].RoundID != plan.RoundID {
		t.Fatalf("events = %#v, want continuation_scheduled", repo.events)
	}
}

func TestServicePlanContinuationRetriesVersionStale(t *testing.T) {
	repo := &staleOnceVersionRepository{
		memoryRepository: newMemoryRepository(),
		mutate: func(item protocol.Goal) protocol.Goal {
			item.Objective = "Concurrent room slot update"
			return item
		},
	}
	service := NewService(config.Config{
		GoalEnabled:             true,
		GoalAutoContinueEnabled: true,
	}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "room:group:continuation-plan-race",
		Objective:  "Plan continuation",
	})
	if err != nil {
		t.Fatal(err)
	}
	repo.staleGoalID = created.ID

	plan, err := service.PlanContinuationForSession(ctx, created.SessionKey, "room-round-1")
	if err != nil {
		t.Fatal(err)
	}
	if !repo.injected {
		t.Fatal("stale version repository did not inject a version conflict")
	}
	if plan == nil {
		t.Fatal("plan = nil, want retried continuation")
	}
	if plan.Goal.Objective != "Concurrent room slot update" || !strings.Contains(plan.Prompt, "Concurrent room slot update") {
		t.Fatalf("plan = %#v, want continuation from reloaded room goal", plan)
	}
	current, err := service.Current(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current.ContinuationCount != 1 || current.Objective != "Concurrent room slot update" {
		t.Fatalf("current = %#v, want retried continuation update on reloaded goal", current)
	}
	if got := repo.events[len(repo.events)-1]; got.EventType != "continuation_scheduled" || got.RoundID != plan.RoundID {
		t.Fatalf("last event = %#v, want continuation_scheduled after retry", got)
	}
}

func TestServiceGoalContinuationStillCurrentRejectsStaleGoal(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{
		GoalEnabled:             true,
		GoalAutoContinueEnabled: true,
	}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Skip stale continuation",
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := service.PlanContinuationForSession(ctx, created.SessionKey, "round-1")
	if err != nil {
		t.Fatal(err)
	}
	current, err := service.GoalContinuationStillCurrent(ctx, *plan)
	if err != nil {
		t.Fatal(err)
	}
	if !current {
		t.Fatal("GoalContinuationStillCurrent() = false, want true for current active goal")
	}

	stale := repo.goals[created.ID]
	stale.Status = protocol.GoalStatusPaused
	stale.Version++
	repo.goals[created.ID] = stale
	current, err = service.GoalContinuationStillCurrent(ctx, *plan)
	if err != nil {
		t.Fatal(err)
	}
	if current {
		t.Fatal("GoalContinuationStillCurrent() = true, want false after goal is no longer active")
	}
}

func TestServicePlanContinuationStopsWhenBudgetExhausted(t *testing.T) {
	repo := newMemoryRepository()
	budget := int64(10)
	service := NewService(config.Config{
		GoalEnabled:             true,
		GoalAutoContinueEnabled: true,
	}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey:  "agent:nexus:ws:dm:chat",
		Objective:   "Budgeted work",
		TokenBudget: &budget,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.RecordUsageForSession(ctx, created.SessionKey, protocol.GoalUsage{TotalTokens: 10, RuntimeSeconds: 7}, "round-1"); err != nil {
		t.Fatal(err)
	}
	plan, err := service.PlanContinuationForSession(ctx, created.SessionKey, "round-1")
	if err != nil {
		t.Fatal(err)
	}
	if plan != nil {
		t.Fatalf("plan = %#v, want nil after budget exhaustion", plan)
	}
	current, err := service.Current(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current.Status != protocol.GoalStatusBudgetLimited || current.LastError == "" || current.TimeUsedSeconds != 7 {
		t.Fatalf("current = %#v, want budget_limited with last error and runtime", current)
	}
}

func TestServicePlanContinuationStopsAtUsageLimit(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{
		GoalEnabled:                true,
		GoalAutoContinueEnabled:    true,
		GoalMaxContinuationsPerRun: 1,
	}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Limited work",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.PlanContinuationForSession(ctx, created.SessionKey, "round-1"); err != nil {
		t.Fatal(err)
	}
	plan, err := service.PlanContinuationForSession(ctx, created.SessionKey, "round-2")
	if err != nil {
		t.Fatal(err)
	}
	if plan != nil {
		t.Fatalf("plan = %#v, want nil after usage limit", plan)
	}
	current, err := service.Current(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current.Status != protocol.GoalStatusUsageLimited || current.LastError == "" {
		t.Fatalf("current = %#v, want usage_limited with last error", current)
	}
}

func TestServiceResumeUsageLimitedGoalStartsFreshContinuationRun(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{
		GoalEnabled:                true,
		GoalAutoContinueEnabled:    true,
		GoalMaxContinuationsPerRun: 1,
	}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Resume after continuation cap",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.PlanContinuationForSession(ctx, created.SessionKey, "round-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.PlanContinuationForSession(ctx, created.SessionKey, "round-2"); err != nil {
		t.Fatal(err)
	}
	limited, err := service.Current(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if limited.Status != protocol.GoalStatusUsageLimited || limited.ContinuationCount != 1 {
		t.Fatalf("limited = %#v, want usage_limited after one continuation", limited)
	}

	resumed, err := service.Resume(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Status != protocol.GoalStatusActive || resumed.ContinuationCount != 0 {
		t.Fatalf("resumed = %#v, want active with continuation count reset", resumed)
	}
	next, err := service.PlanContinuationForSession(ctx, created.SessionKey, "round-3")
	if err != nil {
		t.Fatal(err)
	}
	if next == nil {
		t.Fatal("next plan = nil, want fresh continuation after resume")
	}
	if next.Goal.ContinuationCount != 1 {
		t.Fatalf("next continuation count = %d, want 1 for fresh run", next.Goal.ContinuationCount)
	}
}

func TestServiceRecordGoalActivityResetsContinuationRun(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{
		GoalEnabled:             true,
		GoalAutoContinueEnabled: true,
	}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "User activity restarts the run",
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := service.PlanContinuationForSession(ctx, created.SessionKey, "round-1")
	if err != nil {
		t.Fatal(err)
	}
	if plan == nil {
		t.Fatal("plan = nil, want continuation")
	}
	if _, err := service.RecordContinuationProgress(ctx, created.ID, plan.RoundID, false); err != nil {
		t.Fatal(err)
	}
	updated, err := service.RecordGoalActivity(ctx, created.ID, "round-user")
	if err != nil {
		t.Fatal(err)
	}
	if updated.ContinuationCount != 0 || updated.EmptyProgressCount != 0 || updated.LastError != "" {
		t.Fatalf("updated = %#v, want explicit activity to reset continuation run", updated)
	}
	if got := repo.events[len(repo.events)-1]; got.EventType != "continuation_reset" || got.RoundID != "round-user" {
		t.Fatalf("last event = %#v, want continuation_reset for user activity", got)
	}
}

func TestServicePlanContinuationSuppressesAfterEmptyProgress(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{
		GoalEnabled:             true,
		GoalAutoContinueEnabled: true,
	}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Stop empty loop",
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := service.PlanContinuationForSession(ctx, created.SessionKey, "round-1")
	if err != nil {
		t.Fatal(err)
	}
	if plan == nil {
		t.Fatal("plan = nil, want first continuation")
	}
	if _, err := service.RecordContinuationProgress(ctx, created.ID, plan.RoundID, false); err != nil {
		t.Fatal(err)
	}
	if next, err := service.PlanContinuationForSession(ctx, created.SessionKey, plan.RoundID); err != nil {
		t.Fatal(err)
	} else if next != nil {
		t.Fatalf("next plan = %#v, want nil after empty continuation progress", next)
	}
	current, err := service.Current(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current.EmptyProgressCount != 1 || current.ContinuationCount != 1 {
		t.Fatalf("current = %#v, want empty progress suppression without extra continuation", current)
	}
	if got := repo.events[len(repo.events)-1]; got.EventType != "continuation_suppressed" || got.RoundID != plan.RoundID {
		t.Fatalf("last event = %#v, want continuation_suppressed for continuation round", got)
	}
}

func TestServiceResumeActiveGoalClearsEmptyProgressAndDispatchesContinuation(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{
		GoalEnabled:             true,
		GoalAutoContinueEnabled: true,
	}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	dispatcher := &fakeContinuationDispatcher{}
	service.SetContinuationDispatcher(dispatcher)
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Resume suppressed active goal",
	})
	if err != nil {
		t.Fatal(err)
	}
	dispatcher.plans = nil
	if _, err := service.RecordContinuationProgress(ctx, created.ID, "goal_continuation_1", false); err != nil {
		t.Fatal(err)
	}

	resumed, err := service.Resume(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Status != protocol.GoalStatusActive || resumed.EmptyProgressCount != 0 {
		t.Fatalf("resumed = %#v, want active goal with empty progress cleared", resumed)
	}
	if len(dispatcher.plans) != 1 || dispatcher.plans[0].Goal.ID != created.ID {
		t.Fatalf("plans = %#v, want resumed active goal to dispatch continuation", dispatcher.plans)
	}
	if got := repo.events[len(repo.events)-2]; got.EventType != "resumed" {
		t.Fatalf("event before continuation = %#v, want resumed", got)
	}
}

func TestServiceRecordContinuationFailureStoresLastError(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{
		GoalEnabled:             true,
		GoalAutoContinueEnabled: true,
	}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Surface provider errors",
	})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := service.RecordContinuationFailure(ctx, created.ID, "goal_continuation_1", "Failed to authenticate. API Error: 401")
	if err != nil {
		t.Fatal(err)
	}
	if updated.LastError != "Failed to authenticate. API Error: 401" || updated.EmptyProgressCount != 1 {
		t.Fatalf("updated = %#v, want last_error and empty progress suppression", updated)
	}
	next, err := service.PlanContinuationForSession(ctx, created.SessionKey, "goal_continuation_1")
	if err != nil {
		t.Fatal(err)
	}
	if next != nil {
		t.Fatalf("next = %#v, want nil after continuation failure", next)
	}
	if got := repo.events[len(repo.events)-1]; got.EventType != "continuation_failed" || got.RoundID != "goal_continuation_1" {
		t.Fatalf("last event = %#v, want continuation_failed", got)
	}
}

func TestServiceRecordContinuationProgressRetriesVersionStale(t *testing.T) {
	repo := &staleOnceVersionRepository{
		memoryRepository: newMemoryRepository(),
		mutate: func(item protocol.Goal) protocol.Goal {
			item.Objective = "Concurrent room slot update"
			return item
		},
	}
	service := NewService(config.Config{
		GoalEnabled:             true,
		GoalAutoContinueEnabled: true,
	}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "room:group:continuation-race",
		Objective:  "Retry continuation progress",
	})
	if err != nil {
		t.Fatal(err)
	}
	repo.staleGoalID = created.ID

	updated, err := service.RecordContinuationProgress(ctx, created.ID, "goal_continuation_1", false)
	if err != nil {
		t.Fatal(err)
	}
	if !repo.injected {
		t.Fatal("stale version repository did not inject a version conflict")
	}
	if updated.EmptyProgressCount != 1 || updated.Objective != "Concurrent room slot update" {
		t.Fatalf("updated = %#v, want retried empty-progress update on reloaded goal", updated)
	}
	if got := repo.events[len(repo.events)-1]; got.EventType != "continuation_suppressed" || got.RoundID != "goal_continuation_1" {
		t.Fatalf("last event = %#v, want continuation_suppressed after retry", got)
	}
}

func TestServiceContinuationProgressResetAllowsNextContinuation(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{
		GoalEnabled:             true,
		GoalAutoContinueEnabled: true,
	}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Reset empty loop",
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := service.PlanContinuationForSession(ctx, created.SessionKey, "round-1")
	if err != nil {
		t.Fatal(err)
	}
	if plan == nil {
		t.Fatal("plan = nil, want first continuation")
	}
	if _, err := service.RecordContinuationProgress(ctx, created.ID, plan.RoundID, false); err != nil {
		t.Fatal(err)
	}
	if _, err := service.RecordContinuationProgress(ctx, created.ID, "round-user", true); err != nil {
		t.Fatal(err)
	}
	next, err := service.PlanContinuationForSession(ctx, created.SessionKey, "round-user")
	if err != nil {
		t.Fatal(err)
	}
	if next == nil {
		t.Fatal("next plan = nil, want continuation after progress reset")
	}
	current, err := service.Current(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current.EmptyProgressCount != 0 || current.ContinuationCount != 2 {
		t.Fatalf("current = %#v, want reset empty progress and second continuation", current)
	}
}
