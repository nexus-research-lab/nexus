package goal

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

var errTestUsageUnavailable = errors.New("test durable usage unavailable")

type unavailableFinalizationRepository struct {
	*memoryRepository
}

func (r *unavailableFinalizationRepository) FinalizeGoalUsage(
	context.Context,
	protocol.Goal,
	int64,
	protocol.GoalEvent,
) (*protocol.Goal, error) {
	return nil, errTestUsageUnavailable
}

func (r *unavailableFinalizationRepository) IsGoalUsageUnavailable(err error) bool {
	return errors.Is(err, errTestUsageUnavailable)
}

func TestServiceClassifiesDurableUsageUnavailable(t *testing.T) {
	now := time.Date(2026, 7, 27, 19, 0, 0, 0, time.UTC)
	repo := &unavailableFinalizationRepository{memoryRepository: newMemoryRepository()}
	repo.goals["goal-unavailable"] = protocol.Goal{
		ID:          "goal-unavailable",
		SessionKey:  "room:group:usage-unavailable",
		Objective:   "never fabricate missing usage",
		Status:      protocol.GoalStatusComplete,
		Version:     1,
		CreatedAt:   now,
		UpdatedAt:   now,
		CompletedAt: &now,
	}
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = func() time.Time { return now.Add(time.Minute) }

	if _, err := service.FinalizeUsageForGoal(
		context.Background(),
		"goal-unavailable",
		protocol.GoalUsage{},
		"round-unavailable",
	); !errors.Is(err, ErrGoalUsageUnavailable) {
		t.Fatalf("FinalizeUsageForGoal() error = %v, want ErrGoalUsageUnavailable", err)
	}
	if stored := repo.goals["goal-unavailable"]; stored.UsageFinalized {
		t.Fatalf("goal after unavailable finalization = %#v, want fence open", stored)
	}
}

func TestServiceFinalizeUsageForCompletedGoalKeepsReplacementIsolated(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	broadcaster := &fakeGoalBroadcaster{}
	service.SetEventBroadcaster(broadcaster)
	ctx := context.Background()
	sessionKey := "agent:nexus:ws:dm:final-usage"

	original, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: sessionKey,
		Objective:  "finish original",
	})
	if err != nil {
		t.Fatal(err)
	}
	completed, err := service.CompleteByModel(ctx, original.ID, protocol.CompleteGoalRequest{
		RoundID: "round-old",
	})
	if err != nil {
		t.Fatal(err)
	}
	if completed.UsageFinalized || completed.UsageFinalizedAt != nil {
		t.Fatalf("completed = %#v, want terminal settlement still pending", completed)
	}
	current, err := service.CurrentOptional(ctx, sessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current != nil {
		t.Fatalf("current after complete = %#v, want nil", current)
	}

	replacement, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: sessionKey,
		Objective:  "start replacement",
	})
	if err != nil {
		t.Fatal(err)
	}
	finalized, err := service.FinalizeUsageForGoal(ctx, original.ID, protocol.GoalUsage{
		InputTokens:          10,
		OutputTokens:         2,
		CacheReadInputTokens: 30,
		ActualTotalTokens:    42,
		ActualTotalKnown:     true,
		RuntimeSeconds:       5,
	}, "round-old")
	if err != nil {
		t.Fatal(err)
	}
	if !finalized.UsageFinalized || finalized.UsageFinalizedAt == nil ||
		finalized.Usage.BudgetTokens() != 12 ||
		finalized.Usage.ActualTokens() != 42 ||
		finalized.TimeUsedSeconds != 5 {
		t.Fatalf("finalized = %#v, want exact terminal aggregate and fence", finalized)
	}

	report, err := service.UsageByGoalID(ctx, original.ID)
	if err != nil {
		t.Fatal(err)
	}
	if report.GoalID != original.ID || !report.UsageFinalized ||
		report.Usage.ActualTokens() != 42 || report.Usage.BudgetTokens() != 12 {
		t.Fatalf("report = %#v, want finalized original Goal usage", report)
	}
	current, err = service.Current(ctx, sessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current.ID != replacement.ID || current.Usage.ActualTokens() != 0 ||
		current.Usage.BudgetTokens() != 0 || current.UsageFinalized {
		t.Fatalf("current = %#v, want untouched replacement Goal", current)
	}

	if _, err := service.RecordUsageForGoal(ctx, original.ID, protocol.GoalUsage{
		ActualTotalTokens: 1,
		ActualTotalKnown:  true,
	}, "round-old"); !errors.Is(err, ErrGoalInvalidState) {
		t.Fatalf("late RecordUsageForGoal() error = %v, want ErrGoalInvalidState", err)
	}
	current, err = service.Current(ctx, sessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current.ID != replacement.ID || current.Usage.ActualTokens() != 0 {
		t.Fatalf("replacement after late event = %#v, want no old usage", current)
	}

	var finalizedEvents int
	for _, event := range repo.events {
		if event.GoalID == original.ID && event.EventType == "usage_finalized" {
			finalizedEvents++
			if event.RoundID != "round-old" || event.Payload["usage_finalized"] != true {
				t.Fatalf("usage_finalized event = %#v, want terminal fence payload", event)
			}
		}
	}
	if finalizedEvents != 1 {
		t.Fatalf("usage_finalized events = %d, want 1; all = %#v", finalizedEvents, repo.events)
	}
	var broadcastFinalized int
	for _, event := range broadcaster.events {
		if event.EventType != protocol.EventTypeGoalProgress ||
			event.Data["goal_event_type"] != "usage_finalized" {
			continue
		}
		broadcastFinalized++
		eventGoal, ok := event.Data["goal"].(protocol.Goal)
		if !ok || eventGoal.ID != original.ID || !eventGoal.UsageFinalized ||
			eventGoal.Usage.ActualTokens() != 42 {
			t.Fatalf("usage_finalized broadcast = %#v, want finalized original Goal", event)
		}
	}
	if broadcastFinalized != 1 {
		t.Fatalf("usage_finalized broadcasts = %d, want 1; all = %#v", broadcastFinalized, broadcaster.events)
	}
}

func TestServiceDoesNotFinalizeActiveBlockedOrUsageLimitedGoal(t *testing.T) {
	tests := []struct {
		name   string
		settle func(context.Context, *Service, *protocol.Goal) (*protocol.Goal, error)
	}{
		{
			name: "active round terminal",
			settle: func(_ context.Context, _ *Service, item *protocol.Goal) (*protocol.Goal, error) {
				return item, nil
			},
		},
		{
			name: "blocked",
			settle: func(ctx context.Context, service *Service, item *protocol.Goal) (*protocol.Goal, error) {
				return service.BlockByModel(ctx, item.ID, protocol.BlockGoalRequest{
					BlockerID:   "needs-input",
					Reason:      "needs input",
					NeededInput: "provide input",
					RoundID:     "round-1",
				})
			},
		},
		{
			name: "usage limited",
			settle: func(ctx context.Context, service *Service, item *protocol.Goal) (*protocol.Goal, error) {
				return service.UsageLimitForGoal(ctx, item.ID, "round-1", "provider limit")
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			repo := newMemoryRepository()
			service := NewService(config.Config{GoalEnabled: true}, repo)
			service.nowFn = fixedClock()
			service.idFactory = sequentialID()
			ctx := context.Background()
			created, err := service.Create(ctx, protocol.CreateGoalRequest{
				SessionKey: "agent:nexus:ws:dm:finalization",
				Objective:  "keep resumable Goal open",
			})
			if err != nil {
				t.Fatal(err)
			}
			item, err := testCase.settle(ctx, service, created)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := service.FinalizeUsageForGoal(
				ctx,
				item.ID,
				protocol.GoalUsage{},
				"round-1",
			); !errors.Is(err, ErrGoalInvalidState) {
				t.Fatalf("FinalizeUsageForGoal(%s) error = %v, want ErrGoalInvalidState", item.Status, err)
			}
			stored, err := repo.GetGoal(ctx, item.ID)
			if err != nil {
				t.Fatal(err)
			}
			if stored.UsageFinalized || stored.UsageFinalizedAt != nil {
				t.Fatalf("stored = %#v, want resumable Goal unfinalized", stored)
			}
		})
	}
}
