package goal

import (
	"context"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestServiceRuntimeContextSkipsStoppedGoals(t *testing.T) {
	for _, tc := range []struct {
		name       string
		mutateGoal func(context.Context, *Service, protocol.Goal) error
	}{
		{
			name: "paused",
			mutateGoal: func(ctx context.Context, service *Service, item protocol.Goal) error {
				_, err := service.Pause(ctx, item.ID)
				return err
			},
		},
		{
			name: "blocked",
			mutateGoal: func(ctx context.Context, service *Service, item protocol.Goal) error {
				_, err := service.BlockByModel(ctx, item.ID, protocol.BlockGoalRequest{RoundID: "round-1"})
				return err
			},
		},
		{
			name: "usage_limited",
			mutateGoal: func(ctx context.Context, service *Service, item protocol.Goal) error {
				_, err := service.UsageLimitForSession(ctx, item.SessionKey, "round-1", "usage limit")
				return err
			},
		},
		{
			name: "completed",
			mutateGoal: func(ctx context.Context, service *Service, item protocol.Goal) error {
				_, err := service.CompleteByModel(ctx, item.ID, protocol.CompleteGoalRequest{RoundID: "round-1"})
				return err
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newMemoryRepository()
			budget := int64(10)
			service := NewService(config.Config{GoalEnabled: true}, repo)
			service.nowFn = fixedClock()
			service.idFactory = sequentialID()
			ctx := context.Background()

			created, err := service.Create(ctx, protocol.CreateGoalRequest{
				SessionKey:  "agent:nexus:ws:dm:" + tc.name,
				Objective:   "Stopped work",
				TokenBudget: &budget,
			})
			if err != nil {
				t.Fatal(err)
			}
			if err := tc.mutateGoal(ctx, service, *created); err != nil {
				t.Fatal(err)
			}
			contextText, goal, err := service.RuntimeContext(ctx, created.SessionKey)
			if err != nil {
				t.Fatal(err)
			}
			if contextText != "" || goal != nil {
				t.Fatalf("RuntimeContext() = %q, %#v; want no runtime context for stopped goal", contextText, goal)
			}
		})
	}
}

func TestServiceRuntimeContextKeepsBudgetLimitedGoalForUsageAccounting(t *testing.T) {
	repo := newMemoryRepository()
	budget := int64(10)
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey:  "agent:nexus:ws:dm:budget-limited-context",
		Objective:   "Account budget-limited wrap-up",
		TokenBudget: &budget,
	})
	if err != nil {
		t.Fatal(err)
	}
	limited, err := service.RecordUsageForSession(ctx, created.SessionKey, protocol.GoalUsage{TotalTokens: 10}, "round-1")
	if err != nil {
		t.Fatal(err)
	}
	if limited.Status != protocol.GoalStatusBudgetLimited {
		t.Fatalf("limited status = %q, want budget_limited", limited.Status)
	}

	contextText, goal, err := service.RuntimeContext(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if contextText != "" {
		t.Fatalf("RuntimeContext() context = %q, want no injected context for budget_limited goal", contextText)
	}
	if goal == nil || goal.ID != limited.ID || goal.Status != protocol.GoalStatusBudgetLimited {
		t.Fatalf("RuntimeContext() goal = %#v, want budget_limited usage target", goal)
	}
}

func TestServiceRuntimeContextAccountsWallClockUsage(t *testing.T) {
	repo := newMemoryRepository()
	clock := newMutableClock(time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC))
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = clock.Now
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:wall-clock-context",
		Objective:  "Account wall clock in runtime context",
	})
	if err != nil {
		t.Fatal(err)
	}

	clock.Advance(12 * time.Second)
	contextText, goal, err := service.RuntimeContext(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if contextText != "" || goal == nil || goal.TimeUsedSeconds != 12 {
		t.Fatalf("RuntimeContext() = (%q, %#v), want 12s wall-clock usage without injected context", contextText, goal)
	}

	clock.Advance(3 * time.Second)
	_, goal, err = service.RuntimeContext(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if goal == nil || goal.TimeUsedSeconds != 15 {
		t.Fatalf("second RuntimeContext goal = %#v, want cumulative 15s wall-clock usage", goal)
	}
}

func TestServiceRuntimeContextRetriesWallClockVersionStale(t *testing.T) {
	repo := &staleOnceVersionRepository{
		memoryRepository: newMemoryRepository(),
		mutate: func(item protocol.Goal) protocol.Goal {
			item.Objective = "Concurrent objective update"
			return item
		},
	}
	clock := newMutableClock(time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC))
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = clock.Now
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:wall-clock-stale",
		Objective:  "Retry wall clock accounting",
	})
	if err != nil {
		t.Fatal(err)
	}
	repo.staleGoalID = created.ID

	clock.Advance(5 * time.Second)
	_, goal, err := service.RuntimeContext(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if !repo.injected {
		t.Fatal("stale version repository did not inject a version conflict")
	}
	if goal == nil || goal.TimeUsedSeconds != 5 || goal.Objective != "Concurrent objective update" {
		t.Fatalf("RuntimeContext goal = %#v, want retried wall-clock update on reloaded goal", goal)
	}
}
