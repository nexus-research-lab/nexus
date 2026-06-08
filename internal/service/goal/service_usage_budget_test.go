package goal

import (
	"context"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestServicePauseAndModelBlockPreserveBudgetLimitedGoal(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(context.Context, *Service, protocol.Goal) (*protocol.Goal, error)
	}{
		{
			name: "user pause",
			run: func(ctx context.Context, service *Service, item protocol.Goal) (*protocol.Goal, error) {
				return service.Pause(ctx, item.ID)
			},
		},
		{
			name: "model block",
			run: func(ctx context.Context, service *Service, item protocol.Goal) (*protocol.Goal, error) {
				return service.BlockByModel(ctx, item.ID, protocol.BlockGoalRequest{RoundID: "round-blocked"})
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newMemoryRepository()
			service := NewService(config.Config{GoalEnabled: true}, repo)
			service.nowFn = fixedClock()
			service.idFactory = sequentialID()
			ctx := context.Background()
			budget := int64(10)

			created, err := service.Create(ctx, protocol.CreateGoalRequest{
				SessionKey:  "agent:nexus:ws:dm:" + strings.ReplaceAll(tc.name, " ", "-"),
				Objective:   "Preserve budget limit",
				TokenBudget: &budget,
			})
			if err != nil {
				t.Fatal(err)
			}
			limited, err := service.RecordUsageForSession(ctx, created.SessionKey, protocol.GoalUsage{TotalTokens: 10}, "round-budget")
			if err != nil {
				t.Fatal(err)
			}
			if limited.Status != protocol.GoalStatusBudgetLimited {
				t.Fatalf("limited status = %q, want budget_limited", limited.Status)
			}

			updated, err := tc.run(ctx, service, *limited)
			if err != nil {
				t.Fatal(err)
			}
			if updated.Status != protocol.GoalStatusBudgetLimited {
				t.Fatalf("updated status = %q, want budget_limited", updated.Status)
			}
			for _, event := range repo.events {
				if event.EventType == "paused" || event.EventType == "blocked" {
					t.Fatalf("events = %#v, want no paused/blocked event after budget_limited", repo.events)
				}
			}
		})
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

func TestServiceRecordUsageRetriesVersionStale(t *testing.T) {
	repo := &staleOnceUsageRepository{
		memoryRepository: newMemoryRepository(),
		concurrentUsage:  protocol.GoalUsage{TotalTokens: 3, RuntimeSeconds: 2},
	}
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()
	budget := int64(7)

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey:  "room:group:usage-race",
		Objective:   "Count parallel room agents",
		TokenBudget: &budget,
	})
	if err != nil {
		t.Fatal(err)
	}
	repo.staleGoalID = created.ID

	updated, err := service.RecordUsageForGoal(ctx, created.ID, protocol.GoalUsage{
		TotalTokens:    5,
		RuntimeSeconds: 4,
	}, "round-agent-b")
	if err != nil {
		t.Fatal(err)
	}
	if !repo.injected {
		t.Fatal("stale usage repository did not inject a version conflict")
	}
	if updated.Usage.Total() != 8 || updated.TimeUsedSeconds != 6 {
		t.Fatalf("updated usage = %#v time=%d, want concurrent + retried delta", updated.Usage, updated.TimeUsedSeconds)
	}
	if updated.Status != protocol.GoalStatusBudgetLimited {
		t.Fatalf("updated status = %q, want budget_limited after retried delta", updated.Status)
	}
	if len(repo.events) != 3 ||
		repo.events[1].EventType != "usage_recorded" ||
		repo.events[2].EventType != "budget_limited" {
		t.Fatalf("events = %#v, want usage_recorded and budget_limited after retry", repo.events)
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

	_, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey:  "agent:nexus:ws:dm:chat",
		Objective:   "Invalid budget",
		TokenBudget: &zero,
	})
	assertGoalInvalidInputMessage(t, err, goalBudgetPositiveMessage)

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Valid budget target",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Update(ctx, created.ID, protocol.UpdateGoalRequest{
		TokenBudget: optionalBudget(negative),
	}); err != nil {
		assertGoalInvalidInputMessage(t, err, goalBudgetPositiveMessage)
	} else {
		t.Fatal("Update negative budget error = nil, want ErrGoalInvalidInput")
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

	dispatcher := &fakeContinuationDispatcher{}
	service.SetContinuationDispatcher(dispatcher)
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
	if len(dispatcher.plans) != 1 || dispatcher.plans[0].Goal.ID != resumed.ID {
		t.Fatalf("plans = %#v, want continuation after budget resume", dispatcher.plans)
	}
}

func TestServiceResumePreservesExhaustedBudgetLimitedGoal(t *testing.T) {
	repo := newMemoryRepository()
	budget := int64(10)
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey:  "agent:nexus:ws:dm:chat",
		Objective:   "Respect exhausted budget",
		TokenBudget: &budget,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.RecordUsageForSession(ctx, created.SessionKey, protocol.GoalUsage{TotalTokens: 10}, "round-1"); err != nil {
		t.Fatal(err)
	}

	resumed, err := service.Resume(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Status != protocol.GoalStatusBudgetLimited {
		t.Fatalf("resumed status = %q, want budget_limited", resumed.Status)
	}
	if len(repo.events) != 3 || repo.events[len(repo.events)-1].EventType != "budget_limited" {
		t.Fatalf("events = %#v, want no resumed event while budget is exhausted", repo.events)
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
	if updated.Usage.TotalTokens != 20 || updated.Usage.Total() != 20 {
		t.Fatalf("usage = %#v, want budget total 20", updated.Usage)
	}
	remaining := updated.RemainingTokens()
	if remaining == nil || *remaining != 30 {
		t.Fatalf("RemainingTokens() = %#v, want 30", remaining)
	}
}

func TestServicePauseAfterBudgetLimitAccountingKeepsBudgetLimited(t *testing.T) {
	repo := newMemoryRepository()
	budget := int64(5)
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey:  "agent:nexus:ws:dm:chat",
		Objective:   "Pause after budget limit",
		TokenBudget: &budget,
	})
	if err != nil {
		t.Fatal(err)
	}
	accountant := &fakeExternalMutationAccountant{
		service: service,
		usage:   protocol.GoalUsage{InputTokens: 4, OutputTokens: 2},
		roundID: "round-running",
	}
	service.SetExternalMutationAccountant(accountant)

	paused, err := service.Pause(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if paused.Status != protocol.GoalStatusBudgetLimited || paused.Usage.Total() != 6 {
		t.Fatalf("paused = %#v, want budget_limited after accounting crosses budget", paused)
	}
	if len(accountant.sessionKeys) != 1 || accountant.sessionKeys[0] != created.SessionKey {
		t.Fatalf("accountant sessionKeys = %#v, want current session", accountant.sessionKeys)
	}
	if len(repo.events) != 3 ||
		repo.events[1].EventType != "usage_recorded" ||
		repo.events[2].EventType != "budget_limited" {
		t.Fatalf("events = %#v, want usage_recorded then budget_limited only", repo.events)
	}
}

func TestServiceAllowsGoalCompletionAfterExternalFlushHitsBudget(t *testing.T) {
	repo := newMemoryRepository()
	budget := int64(5)
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	dispatcher := &fakeGuidanceDispatcher{}
	service.SetGuidanceDispatcher(dispatcher)
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
	if len(dispatcher.items) != 0 {
		t.Fatalf("guidance = %#v, want suppressed budget steering while update_goal completes", dispatcher.items)
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
