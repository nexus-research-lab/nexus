package goal

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestServiceCreateAndCurrentGoal(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()

	created, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Ship goal mode",
		CreatedBy:  "user",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != "goal_1" || created.Status != protocol.GoalStatusActive {
		t.Fatalf("created = %#v, want active goal_1", created)
	}
	if created.TokenBudget != nil {
		t.Fatalf("TokenBudget = %#v, want nil when omitted", created.TokenBudget)
	}

	current, err := service.Current(context.Background(), "agent:nexus:ws:dm:chat")
	if err != nil {
		t.Fatal(err)
	}
	if current.ID != created.ID {
		t.Fatalf("Current ID = %q, want %q", current.ID, created.ID)
	}
	if _, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Second",
	}); !errors.Is(err, ErrGoalConflict) {
		t.Fatalf("duplicate create error = %v, want ErrGoalConflict", err)
	}
}

func TestServiceCurrentOptionalAllowsMissingGoal(t *testing.T) {
	service := NewService(config.Config{GoalEnabled: true}, newMemoryRepository())

	current, err := service.CurrentOptional(context.Background(), "agent:nexus:ws:dm:chat")
	if err != nil {
		t.Fatal(err)
	}
	if current != nil {
		t.Fatalf("CurrentOptional() = %#v, want nil", current)
	}
	if _, err := service.Current(context.Background(), "agent:nexus:ws:dm:chat"); !errors.Is(err, ErrGoalNotFound) {
		t.Fatalf("Current() error = %v, want ErrGoalNotFound", err)
	}
}

func TestServiceBroadcastsGoalEvents(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	broadcaster := &fakeGoalBroadcaster{}
	service.SetEventBroadcaster(broadcaster)

	created, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Broadcast status",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(broadcaster.events) != 1 || broadcaster.events[0].EventType != protocol.EventTypeGoalCreated {
		t.Fatalf("events = %#v, want goal_created", broadcaster.events)
	}

	if _, err := service.Pause(context.Background(), created.ID); err != nil {
		t.Fatal(err)
	}
	if len(broadcaster.events) != 2 || broadcaster.events[1].EventType != protocol.EventTypeGoalStatusChanged {
		t.Fatalf("events = %#v, want goal_status_changed", broadcaster.events)
	}
	if broadcaster.events[1].Data["goal_event_type"] != "paused" {
		t.Fatalf("payload = %#v, want paused goal_event_type", broadcaster.events[1].Data)
	}
}

func TestServiceStateTransitions(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Long task",
	})
	if err != nil {
		t.Fatal(err)
	}
	paused, err := service.Pause(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if paused.Status != protocol.GoalStatusPaused {
		t.Fatalf("paused status = %q, want paused", paused.Status)
	}
	if _, err := service.CompleteByModel(ctx, created.ID, protocol.CompleteGoalRequest{}); !errors.Is(err, ErrGoalInvalidState) {
		t.Fatalf("model complete paused error = %v, want ErrGoalInvalidState", err)
	}
	resumed, err := service.Resume(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	completed, err := service.CompleteByModel(ctx, resumed.ID, protocol.CompleteGoalRequest{Summary: "done", RoundID: "round-1"})
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != protocol.GoalStatusComplete || completed.CompletedAt == nil {
		t.Fatalf("completed = %#v, want terminal complete", completed)
	}
	if _, err := service.Resume(ctx, completed.ID); !errors.Is(err, ErrGoalInvalidState) {
		t.Fatalf("resume complete error = %v, want ErrGoalInvalidState", err)
	}
}

func TestServiceBlockByModelAllowsEmptyReason(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Wait for external input",
	})
	if err != nil {
		t.Fatal(err)
	}
	blocked, err := service.BlockByModel(ctx, created.ID, protocol.BlockGoalRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if blocked.Status != protocol.GoalStatusBlocked || blocked.BlockedAt == nil {
		t.Fatalf("blocked = %#v, want blocked status", blocked)
	}
	if len(repo.events) != 2 || repo.events[1].EventType != "blocked" {
		t.Fatalf("events = %#v, want blocked event", repo.events)
	}
	if _, ok := repo.events[1].Payload["reason"]; ok {
		t.Fatalf("blocked payload = %#v, want no synthetic reason", repo.events[1].Payload)
	}
}

func TestServiceRecordUsageForCompletedGoal(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Complete with final usage",
	})
	if err != nil {
		t.Fatal(err)
	}
	completed, err := service.CompleteByModel(ctx, created.ID, protocol.CompleteGoalRequest{})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := service.RecordUsageForGoal(ctx, completed.ID, protocol.GoalUsage{
		TotalTokens:    12,
		RuntimeSeconds: 5,
	}, "round-1")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != protocol.GoalStatusComplete || updated.Usage.Total() != 12 || updated.TimeUsedSeconds != 5 {
		t.Fatalf("updated = %#v, want completed goal with final usage", updated)
	}
	if len(repo.events) != 3 || repo.events[2].EventType != "usage_recorded" || repo.events[2].RoundID != "round-1" {
		t.Fatalf("events = %#v, want usage_recorded after completion", repo.events)
	}
}

func TestServiceRejectsOversizedObjective(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()
	oversized := strings.Repeat("x", maxGoalObjectiveRunes+1)

	if _, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  oversized,
	}); !errors.Is(err, ErrGoalInvalidInput) {
		t.Fatalf("Create oversized objective error = %v, want ErrGoalInvalidInput", err)
	}

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Valid goal",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Update(ctx, created.ID, protocol.UpdateGoalRequest{
		Objective: &oversized,
	}); !errors.Is(err, ErrGoalInvalidInput) {
		t.Fatalf("Update oversized objective error = %v, want ErrGoalInvalidInput", err)
	}
}

func TestServiceRejectsNonPositiveBudget(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()
	zero := int64(0)
	negative := int64(-1)

	if _, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey:  "agent:nexus:ws:dm:chat",
		Objective:   "Invalid budget",
		TokenBudget: &zero,
	}); !errors.Is(err, ErrGoalInvalidInput) {
		t.Fatalf("Create zero budget error = %v, want ErrGoalInvalidInput", err)
	}

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Valid budget target",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Update(ctx, created.ID, protocol.UpdateGoalRequest{
		TokenBudget: optionalBudget(negative),
	}); !errors.Is(err, ErrGoalInvalidInput) {
		t.Fatalf("Update negative budget error = %v, want ErrGoalInvalidInput", err)
	}
}

func TestServiceUpdateBudgetSteersLimitedStatus(t *testing.T) {
	repo := newMemoryRepository()
	initialBudget := int64(10)
	raisedBudget := int64(20)
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
		TokenBudget: &initialBudget,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.RecordUsageForSession(ctx, created.SessionKey, protocol.GoalUsage{TotalTokens: 10}, "round-1"); err != nil {
		t.Fatal(err)
	}
	current, err := service.Current(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current.Status != protocol.GoalStatusBudgetLimited {
		t.Fatalf("current status = %q, want budget_limited", current.Status)
	}

	resumed, err := service.Update(ctx, created.ID, protocol.UpdateGoalRequest{TokenBudget: optionalBudget(raisedBudget)})
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Status != protocol.GoalStatusActive || resumed.LastError != "" {
		t.Fatalf("resumed = %#v, want active with cleared error", resumed)
	}
	if resumed.TokenBudget == nil || *resumed.TokenBudget != raisedBudget {
		t.Fatalf("TokenBudget = %#v, want %d", resumed.TokenBudget, raisedBudget)
	}
}

func TestServiceRecordUsageUsesGoalBudgetTokenAccounting(t *testing.T) {
	repo := newMemoryRepository()
	budget := int64(50)
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey:  "agent:nexus:ws:dm:chat",
		Objective:   "Budget accounting",
		TokenBudget: &budget,
	})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := service.RecordUsageForSession(ctx, created.SessionKey, protocol.GoalUsage{
		InputTokens:              10,
		OutputTokens:             20,
		CacheCreationInputTokens: 80,
		CacheReadInputTokens:     90,
		ReasoningTokens:          40,
		TotalTokens:              240,
	}, "round-1")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != protocol.GoalStatusActive {
		t.Fatalf("status = %q, want active", updated.Status)
	}
	if updated.Usage.TotalTokens != 30 || updated.Usage.Total() != 30 {
		t.Fatalf("usage = %#v, want budget total 30", updated.Usage)
	}
	remaining := updated.RemainingTokens()
	if remaining == nil || *remaining != 20 {
		t.Fatalf("RemainingTokens() = %#v, want 20", remaining)
	}
}

func TestServiceFlushesGoalAccountingBeforeExternalMutation(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Pause after accounting",
	})
	if err != nil {
		t.Fatal(err)
	}
	accountant := &fakeExternalMutationAccountant{
		service: service,
		usage:   protocol.GoalUsage{InputTokens: 4, OutputTokens: 5},
		roundID: "round-running",
	}
	service.SetExternalMutationAccountant(accountant)

	paused, err := service.Pause(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if paused.Status != protocol.GoalStatusPaused || paused.Usage.Total() != 9 {
		t.Fatalf("paused = %#v, want paused after usage accounting", paused)
	}
	if len(accountant.sessionKeys) != 1 || accountant.sessionKeys[0] != created.SessionKey {
		t.Fatalf("accountant sessionKeys = %#v, want current session", accountant.sessionKeys)
	}
	if len(repo.events) != 3 || repo.events[1].EventType != "usage_recorded" || repo.events[2].EventType != "paused" {
		t.Fatalf("events = %#v, want usage_recorded before paused", repo.events)
	}
}

func TestServiceAllowsGoalCompletionAfterExternalFlushHitsBudget(t *testing.T) {
	repo := newMemoryRepository()
	budget := int64(5)
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey:  "agent:nexus:ws:dm:chat",
		Objective:   "Complete despite budget crossing",
		TokenBudget: &budget,
	})
	if err != nil {
		t.Fatal(err)
	}
	service.SetExternalMutationAccountant(&fakeExternalMutationAccountant{
		service: service,
		usage:   protocol.GoalUsage{InputTokens: 6, OutputTokens: 1},
		roundID: "round-running",
	})

	completed, err := service.CompleteByModel(ctx, created.ID, protocol.CompleteGoalRequest{RoundID: "round-running"})
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != protocol.GoalStatusComplete || completed.Usage.Total() != 7 {
		t.Fatalf("completed = %#v, want complete after budget-limited accounting", completed)
	}
	if len(repo.events) != 4 ||
		repo.events[1].EventType != "usage_recorded" ||
		repo.events[2].EventType != "budget_limited" ||
		repo.events[3].EventType != "completed" {
		t.Fatalf("events = %#v, want usage, budget_limited, completed", repo.events)
	}
}

func TestServiceUsageLimitForSessionTransitionsActiveAndBudgetLimitedGoal(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Runtime usage limit",
	})
	if err != nil {
		t.Fatal(err)
	}
	limited, err := service.UsageLimitForSession(ctx, created.SessionKey, "round-1", "You've hit your usage limit.")
	if err != nil {
		t.Fatal(err)
	}
	if limited.Status != protocol.GoalStatusUsageLimited || limited.LastError != "You've hit your usage limit." {
		t.Fatalf("limited = %#v, want usage_limited with reason", limited)
	}
	if len(repo.events) != 2 || repo.events[1].EventType != "usage_limited" || repo.events[1].RoundID != "round-1" {
		t.Fatalf("events = %#v, want usage_limited event", repo.events)
	}

	resumed, err := service.Resume(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Status != protocol.GoalStatusActive {
		t.Fatalf("resumed status = %q, want active", resumed.Status)
	}
	if _, err := service.RecordUsageForSession(ctx, created.SessionKey, protocol.GoalUsage{TotalTokens: 1}, "round-2"); err != nil {
		t.Fatal(err)
	}
	lowBudget := int64(1)
	budgetLimited, err := service.Update(ctx, created.ID, protocol.UpdateGoalRequest{TokenBudget: optionalBudget(lowBudget)})
	if err != nil {
		t.Fatal(err)
	}
	if budgetLimited.Status != protocol.GoalStatusBudgetLimited {
		t.Fatalf("budgetLimited status = %q, want budget_limited", budgetLimited.Status)
	}

	limited, err = service.UsageLimitForSession(ctx, created.SessionKey, "round-3", "usage limit")
	if err != nil {
		t.Fatal(err)
	}
	if limited.Status != protocol.GoalStatusUsageLimited {
		t.Fatalf("budget-limited transition status = %q, want usage_limited", limited.Status)
	}
}

func TestServiceUpdateBudgetClearResumesLimitedGoal(t *testing.T) {
	repo := newMemoryRepository()
	initialBudget := int64(10)
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey:  "agent:nexus:ws:dm:chat",
		Objective:   "Clear budget",
		TokenBudget: &initialBudget,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.RecordUsageForSession(ctx, created.SessionKey, protocol.GoalUsage{TotalTokens: 10}, "round-1"); err != nil {
		t.Fatal(err)
	}
	resumed, err := service.Update(ctx, created.ID, protocol.UpdateGoalRequest{TokenBudget: clearBudget()})
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Status != protocol.GoalStatusActive || resumed.TokenBudget != nil {
		t.Fatalf("resumed = %#v, want active with cleared budget", resumed)
	}
}

func TestServiceUpdateBudgetLimitsActiveGoal(t *testing.T) {
	repo := newMemoryRepository()
	initialBudget := int64(100)
	loweredBudget := int64(25)
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey:  "agent:nexus:ws:dm:chat",
		Objective:   "Lower budget",
		TokenBudget: &initialBudget,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.RecordUsageForSession(ctx, created.SessionKey, protocol.GoalUsage{TotalTokens: 30}, "round-1"); err != nil {
		t.Fatal(err)
	}
	limited, err := service.Update(ctx, created.ID, protocol.UpdateGoalRequest{TokenBudget: optionalBudget(loweredBudget)})
	if err != nil {
		t.Fatal(err)
	}
	if limited.Status != protocol.GoalStatusBudgetLimited || limited.LastError == "" {
		t.Fatalf("limited = %#v, want budget_limited with error", limited)
	}
}

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
	if item.sessionKey != created.SessionKey || !strings.Contains(item.content, "objective was edited") || !strings.Contains(item.content, "Updated &lt;goal&gt;") {
		t.Fatalf("guidance item = %#v, want escaped objective update steering", item)
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
		Objective:   "Budget work",
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
	if item := dispatcher.items[0]; item.sessionKey != created.SessionKey || !strings.Contains(item.content, "budget_limited") || !strings.Contains(item.content, "Budget work") {
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
		"Complete parity",
		"PreviousRoundID: round-1",
		"Completion audit:",
		"Blocked audit:",
		"Tokens remaining:",
	} {
		if !strings.Contains(plan.Prompt, want) {
			t.Fatalf("continuation prompt missing %q: %s", want, plan.Prompt)
		}
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

func TestServiceRunAutoResumeOnceDispatchesActiveGoal(t *testing.T) {
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
		Objective:  "Resume after restart",
	})
	if err != nil {
		t.Fatal(err)
	}
	dispatcher := &fakeContinuationDispatcher{}
	if err := service.RunAutoResumeOnce(ctx, dispatcher); err != nil {
		t.Fatal(err)
	}
	if len(dispatcher.plans) != 1 || dispatcher.plans[0].Goal.ID != created.ID {
		t.Fatalf("plans = %#v, want one resumed goal", dispatcher.plans)
	}
	current, err := service.Current(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current.ContinuationCount != 1 {
		t.Fatalf("ContinuationCount = %d, want 1", current.ContinuationCount)
	}
}

func TestServiceRunAutoResumeOnceSkipsDeferredGoal(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{
		GoalEnabled:             true,
		GoalAutoContinueEnabled: true,
	}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	if _, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Deferred goal",
	}); err != nil {
		t.Fatal(err)
	}
	dispatcher := &fakeContinuationDispatcher{deferSessions: map[string]bool{"agent:nexus:ws:dm:chat": true}}
	if err := service.RunAutoResumeOnce(ctx, dispatcher); err != nil {
		t.Fatal(err)
	}
	if len(dispatcher.plans) != 0 {
		t.Fatalf("plans = %#v, want no dispatch for busy session", dispatcher.plans)
	}
}

func TestServiceCheckpointUpdatesRuntimeContext(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Long work",
	})
	if err != nil {
		t.Fatal(err)
	}
	checkpoint, err := service.CreateCheckpointByModel(ctx, created.ID, protocol.CreateGoalCheckpointRequest{
		Summary: "Repository and service are wired.",
		RoundID: "round-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if checkpoint.ID != "goal_checkpoint_3" || checkpoint.Summary == "" {
		t.Fatalf("checkpoint = %#v, want persisted checkpoint", checkpoint)
	}
	contextText, _, err := service.RuntimeContext(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(contextText, "LatestCheckpoint") ||
		!strings.Contains(contextText, "Repository and service are wired.") {
		t.Fatalf("runtime context missing checkpoint: %s", contextText)
	}
	if len(repo.events) != 2 || repo.events[1].EventType != "checkpoint_created" {
		t.Fatalf("events = %#v, want checkpoint_created", repo.events)
	}
}

func TestServiceDisabled(t *testing.T) {
	service := NewService(config.Config{}, newMemoryRepository())
	_, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "disabled",
	})
	if !errors.Is(err, ErrGoalDisabled) {
		t.Fatalf("Create disabled error = %v, want ErrGoalDisabled", err)
	}
}

func TestBuildRuntimeContext(t *testing.T) {
	budget := int64(100)
	contextText := BuildRuntimeContext(protocol.Goal{
		Objective:   "Complete parity",
		Status:      protocol.GoalStatusActive,
		TokenBudget: &budget,
		Usage: protocol.GoalUsage{
			InputTokens:  10,
			OutputTokens: 20,
			TotalTokens:  30,
		},
	})
	for _, want := range []string{"<nexus_goal>", "Complete parity", "TokenBudget: 100", "RemainingTokens: 70", "update_goal with status=complete"} {
		if !strings.Contains(contextText, want) {
			t.Fatalf("RuntimeContext missing %q: %s", want, contextText)
		}
	}
}

type memoryRepository struct {
	goals       map[string]protocol.Goal
	events      []protocol.GoalEvent
	checkpoints []protocol.GoalCheckpoint
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{goals: map[string]protocol.Goal{}}
}

func (r *memoryRepository) CreateGoal(_ context.Context, item protocol.Goal) (*protocol.Goal, error) {
	for _, current := range r.goals {
		if current.SessionKey == item.SessionKey && protocol.IsCurrentGoalStatus(current.Status) {
			return nil, ErrGoalConflict
		}
	}
	r.goals[item.ID] = item
	return cloneGoal(item), nil
}

func (r *memoryRepository) GetGoal(_ context.Context, goalID string) (*protocol.Goal, error) {
	item, ok := r.goals[goalID]
	if !ok {
		return nil, nil
	}
	return cloneGoal(item), nil
}

func (r *memoryRepository) GetCurrentGoal(_ context.Context, sessionKey string) (*protocol.Goal, error) {
	for _, item := range r.goals {
		if item.SessionKey == sessionKey && protocol.IsCurrentGoalStatus(item.Status) {
			return cloneGoal(item), nil
		}
	}
	return nil, nil
}

func (r *memoryRepository) ListRunnableGoals(_ context.Context, limit int) ([]protocol.Goal, error) {
	items := make([]protocol.Goal, 0)
	for _, item := range r.goals {
		if item.Status == protocol.GoalStatusActive {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i int, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].UpdatedAt.Before(items[j].UpdatedAt)
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (r *memoryRepository) UpdateGoal(_ context.Context, item protocol.Goal, expectedVersion int64) (*protocol.Goal, error) {
	current, ok := r.goals[item.ID]
	if !ok || current.Version != expectedVersion {
		return nil, sql.ErrNoRows
	}
	r.goals[item.ID] = item
	return cloneGoal(item), nil
}

func (r *memoryRepository) AppendEvent(_ context.Context, event protocol.GoalEvent) error {
	r.events = append(r.events, event)
	return nil
}

func (r *memoryRepository) ListEvents(_ context.Context, goalID string, _ int) ([]protocol.GoalEvent, error) {
	items := make([]protocol.GoalEvent, 0)
	for _, event := range r.events {
		if event.GoalID == goalID {
			items = append(items, event)
		}
	}
	return items, nil
}

func (r *memoryRepository) CreateCheckpoint(_ context.Context, checkpoint protocol.GoalCheckpoint) (*protocol.GoalCheckpoint, error) {
	r.checkpoints = append(r.checkpoints, checkpoint)
	clone := checkpoint
	return &clone, nil
}

func (r *memoryRepository) LatestCheckpoint(_ context.Context, goalID string) (*protocol.GoalCheckpoint, error) {
	for index := len(r.checkpoints) - 1; index >= 0; index-- {
		if r.checkpoints[index].GoalID == goalID {
			clone := r.checkpoints[index]
			return &clone, nil
		}
	}
	return nil, nil
}

func cloneGoal(item protocol.Goal) *protocol.Goal {
	clone := item
	return &clone
}

func fixedClock() func() time.Time {
	now := time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC)
	return func() time.Time {
		return now
	}
}

func sequentialID() func(string) string {
	next := 0
	return func(prefix string) string {
		next++
		return prefix + "_" + string(rune('0'+next))
	}
}

func optionalBudget(value int64) protocol.OptionalInt64 {
	return protocol.OptionalInt64{Present: true, Value: &value}
}

func clearBudget() protocol.OptionalInt64 {
	return protocol.OptionalInt64{Present: true}
}

type fakeGoalBroadcaster struct {
	events []protocol.EventMessage
}

func (b *fakeGoalBroadcaster) BroadcastEvent(_ context.Context, _ string, event protocol.EventMessage) []error {
	b.events = append(b.events, event)
	return nil
}

type fakeGuidanceDispatcher struct {
	items []fakeGuidanceItem
}

type fakeGuidanceItem struct {
	sessionKey string
	roundID    string
	content    string
}

func (d *fakeGuidanceDispatcher) QueueGuidanceInput(_ context.Context, sessionKey string, roundID string, content string) ([]string, error) {
	d.items = append(d.items, fakeGuidanceItem{
		sessionKey: sessionKey,
		roundID:    roundID,
		content:    content,
	})
	return []string{"round-running"}, nil
}

type fakeExternalMutationAccountant struct {
	service     *Service
	usage       protocol.GoalUsage
	roundID     string
	sessionKeys []string
}

func (a *fakeExternalMutationAccountant) FlushGoalAccounting(ctx context.Context, sessionKey string) ([]string, error) {
	a.sessionKeys = append(a.sessionKeys, sessionKey)
	if a.service != nil {
		if _, err := a.service.RecordUsageForSession(ctx, sessionKey, a.usage, a.roundID); err != nil {
			return nil, err
		}
	}
	return []string{a.roundID}, nil
}

type fakeContinuationDispatcher struct {
	deferSessions map[string]bool
	plans         []protocol.GoalContinuation
}

func (d *fakeContinuationDispatcher) ShouldDeferGoalContinuation(_ context.Context, sessionKey string) bool {
	return d.deferSessions[sessionKey]
}

func (d *fakeContinuationDispatcher) DispatchGoalContinuation(_ context.Context, plan protocol.GoalContinuation) error {
	d.plans = append(d.plans, plan)
	return nil
}
