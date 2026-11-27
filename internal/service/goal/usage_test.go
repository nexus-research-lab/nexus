// usage_test.go 覆盖 Goal 用量预算、终态结算、作用域记账与墙钟预算测试。
package goal

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/infra/authctx"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalappserver "github.com/nexus-research-lab/nexus/internal/service/goal/appserver"
)

func TestServiceLateUsageLimitForCompletedGoalDoesNotAffectReplacement(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()
	sessionKey := "agent:nexus:ws:dm:late-limit"

	original, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: sessionKey,
		Objective:  "finish original",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CompleteByModel(ctx, original.ID, protocol.CompleteGoalRequest{RoundID: "round-old"}); err != nil {
		t.Fatal(err)
	}
	replacement, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: sessionKey,
		Objective:  "new work",
	})
	if err != nil {
		t.Fatal(err)
	}

	settled, err := service.UsageLimitForGoal(ctx, original.ID, "round-old", "late provider limit")
	if err != nil {
		t.Fatal(err)
	}
	if settled.Status != protocol.GoalStatusComplete {
		t.Fatalf("old status = %q, want complete", settled.Status)
	}
	settled, err = service.RecordUsageForGoal(ctx, original.ID, protocol.GoalUsage{
		ActualTotalTokens: 25,
		ActualTotalKnown:  true,
	}, "round-old")
	if err != nil {
		t.Fatal(err)
	}
	if settled.Usage.ActualTokens() != 25 {
		t.Fatalf("old settled actual = %d, want 25", settled.Usage.ActualTokens())
	}
	current, err := service.Current(ctx, sessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current.ID != replacement.ID || current.Status != protocol.GoalStatusActive {
		t.Fatalf("current = %#v, want replacement active", current)
	}
	if current.Usage.ActualTokens() != 0 {
		t.Fatalf("replacement actual = %d, want no old-round usage", current.Usage.ActualTokens())
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
	if len(accountant.sessionKeys) != 2 ||
		accountant.sessionKeys[0] != created.SessionKey ||
		accountant.sessionKeys[1] != created.SessionKey {
		t.Fatalf("accountant sessionKeys = %#v, want checkpoint and boundary flush", accountant.sessionKeys)
	}
	if len(accountant.settlementBoundaries) != 2 ||
		accountant.settlementBoundaries[0] ||
		!accountant.settlementBoundaries[1] {
		t.Fatalf("settlement boundaries = %#v, want ordinary then boundary flush", accountant.settlementBoundaries)
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
	service := NewService(config.Config{
		GoalEnabled:                true,
		GoalMaxContinuationsPerRun: 20,
	}, repo)
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

type atomicUsageScopeMemoryRepository struct {
	*memoryRepository
	binding      protocol.GoalUsageScopeBinding
	createdEvent protocol.GoalEvent
	atomicCalls  int
	appendCalls  int
}

func (r *atomicUsageScopeMemoryRepository) AppendEvent(ctx context.Context, event protocol.GoalEvent) error {
	r.appendCalls++
	return r.memoryRepository.AppendEvent(ctx, event)
}

func (r *atomicUsageScopeMemoryRepository) CreateGoalWithUsageScope(
	ctx context.Context,
	item protocol.Goal,
	createdEvent protocol.GoalEvent,
	binding protocol.GoalUsageScopeBinding,
) (protocol.GoalUsageScopeCreateResult, error) {
	r.atomicCalls++
	r.binding = binding
	r.createdEvent = createdEvent
	created, err := r.memoryRepository.CreateGoal(ctx, item)
	if err != nil {
		return protocol.GoalUsageScopeCreateResult{}, err
	}
	created.Usage = protocol.GoalUsage{
		ActualTotalTokens: 13,
		ActualTotalKnown:  true,
		BudgetTotalKnown:  true,
	}
	created.Version++
	r.goals[created.ID] = *created
	usageEvent := protocol.GoalEvent{
		ID:         binding.UsageEventID,
		GoalID:     created.ID,
		SessionKey: created.SessionKey,
		EventType:  "usage_recorded",
		Source:     protocol.GoalUpdateSourceSystem,
		RoundID:    binding.ScopeRoundID,
		Payload:    map[string]any{"actual_tokens": int64(13)},
		CreatedAt:  binding.BoundAt,
	}
	r.events = append(r.events, createdEvent, usageEvent)
	return protocol.GoalUsageScopeCreateResult{
		Goal:            created,
		UsageEvent:      &usageEvent,
		AttributedDelta: 13,
	}, nil
}

func TestServiceCreateAtomicallyBindsModelGoalUsageScope(t *testing.T) {
	for _, test := range []struct {
		name             string
		requestOwner     string
		wantBindingOwner string
	}{
		{
			name:             "request owner wins",
			requestOwner:     "owner-request",
			wantBindingOwner: "owner-request",
		},
		{
			name:             "authenticated owner fallback",
			wantBindingOwner: "owner-context",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := &atomicUsageScopeMemoryRepository{memoryRepository: newMemoryRepository()}
			service := NewService(config.Config{GoalEnabled: true}, repo)
			service.nowFn = fixedClock()
			service.idFactory = sequentialID()
			broadcaster := &fakeGoalBroadcaster{}
			service.SetEventBroadcaster(broadcaster)
			ctx := authctx.WithPrincipal(context.Background(), &authctx.Principal{UserID: "owner-context"})

			created, err := service.Create(ctx, protocol.CreateGoalRequest{
				SessionKey:  "agent:nexus:ws:dm:conversation-1",
				Objective:   "Recover child usage after restart",
				CreatedBy:   "model",
				RoundID:     " scope-round-1 ",
				OwnerUserID: test.requestOwner,
			})
			if err != nil {
				t.Fatal(err)
			}
			if repo.atomicCalls != 1 || repo.appendCalls != 0 {
				t.Fatalf("atomic calls = %d, append calls = %d, want 1 and 0", repo.atomicCalls, repo.appendCalls)
			}
			if len(repo.events) != 2 ||
				repo.events[0].EventType != "created" ||
				repo.events[1].EventType != "usage_recorded" {
				t.Fatalf("stored events = %+v, want one created and one usage event", repo.events)
			}
			if repo.binding.OwnerUserID != test.wantBindingOwner ||
				repo.binding.GoalSessionKey != created.SessionKey ||
				repo.binding.SourceKind != protocol.GoalUsageSourceKindNXSTask ||
				repo.binding.ScopeRoundID != "scope-round-1" ||
				repo.binding.GoalID != created.ID ||
				repo.binding.UsageEventID == "" {
				t.Fatalf("scope binding = %+v, want durable model round binding", repo.binding)
			}
			if repo.createdEvent.ID == "" || repo.createdEvent.ID == repo.binding.UsageEventID {
				t.Fatalf("created event = %+v, binding = %+v, want distinct event identities", repo.createdEvent, repo.binding)
			}
			if created.Usage.ActualTokens() != 13 {
				t.Fatalf("created usage = %+v, want atomic pending attribution", created.Usage)
			}
			if len(broadcaster.events) != 2 ||
				broadcaster.events[0].EventType != protocol.EventTypeGoalCreated ||
				broadcaster.events[1].EventType != protocol.EventTypeGoalProgress {
				t.Fatalf("broadcast events = %+v, want created then usage progress", broadcaster.events)
			}
			broadcastGoal, ok := broadcaster.events[0].Data["goal"].(protocol.Goal)
			if !ok || broadcastGoal.Usage.ActualTokens() != 13 {
				t.Fatalf("created broadcast goal = %+v, want latest atomically attributed usage", broadcaster.events[0].Data["goal"])
			}
		})
	}
}

type capturingUsageSourceRepository struct {
	*memoryRepository
	snapshot       protocol.GoalUsageSourceSnapshot
	claim          protocol.GoalUsageSourceRoundClaim
	parentSnapshot protocol.GoalUsageParentSnapshot
	binding        protocol.GoalUsageScopeBinding
}

func (r *capturingUsageSourceRepository) ApplyUsageSourceSnapshot(
	_ context.Context,
	snapshot protocol.GoalUsageSourceSnapshot,
) (protocol.GoalUsageSourceResult, error) {
	r.snapshot = snapshot
	return protocol.GoalUsageSourceResult{}, nil
}

func (r *capturingUsageSourceRepository) ClaimUsageSourceRound(
	_ context.Context,
	claim protocol.GoalUsageSourceRoundClaim,
) (protocol.GoalUsageSourceResult, error) {
	r.claim = claim
	return protocol.GoalUsageSourceResult{}, nil
}

func (r *capturingUsageSourceRepository) RecordUsageParentSnapshot(
	_ context.Context,
	snapshot protocol.GoalUsageParentSnapshot,
) (protocol.GoalUsageParentResult, error) {
	r.parentSnapshot = snapshot
	return protocol.GoalUsageParentResult{}, nil
}

func (r *capturingUsageSourceRepository) BindUsageScopeFromNow(
	_ context.Context,
	binding protocol.GoalUsageScopeBinding,
) (protocol.GoalUsageScopeBindResult, error) {
	r.binding = binding
	return protocol.GoalUsageScopeBindResult{}, nil
}

func TestServiceUsageSourceNormalizesDurableScopeRound(t *testing.T) {
	repo := &capturingUsageSourceRepository{memoryRepository: newMemoryRepository()}
	service := NewService(config.Config{GoalEnabled: true}, repo)

	if _, err := service.RecordUsageSourceSnapshot(context.Background(), protocol.GoalUsageSourceSnapshot{
		OwnerUserID:            " owner-1 ",
		RuntimeSessionKey:      " runtime-1 ",
		SourceKind:             " nxs_task ",
		SourceID:               " task-1 ",
		CumulativeActualTokens: 12,
		GoalSessionKey:         " agent:nexus:ws:dm:conversation-1 ",
		RoundID:                " round-1 ",
	}); err != nil {
		t.Fatal(err)
	}
	if repo.snapshot.OwnerUserID != "owner-1" ||
		repo.snapshot.GoalSessionKey != "agent:nexus:ws:dm:conversation-1" ||
		repo.snapshot.ScopeRoundID != "round-1" {
		t.Fatalf("snapshot = %+v, want normalized durable scope", repo.snapshot)
	}

	if _, err := service.ClaimUsageSourceRound(context.Background(), protocol.GoalUsageSourceRoundClaim{
		OwnerUserID:    " owner-1 ",
		SourceKind:     " nxs_task ",
		RoundID:        " round-2 ",
		GoalID:         " goal-1 ",
		GoalSessionKey: " agent:nexus:ws:dm:conversation-1 ",
	}); err != nil {
		t.Fatal(err)
	}
	if repo.claim.ScopeRoundID != "round-2" || repo.claim.RuntimeSessionKey != "" {
		t.Fatalf("claim = %+v, want scope fallback independent of runtime session", repo.claim)
	}
}

func TestServiceUsageSourceRequiresNXSTaskScopeIdentity(t *testing.T) {
	repo := &capturingUsageSourceRepository{memoryRepository: newMemoryRepository()}
	service := NewService(config.Config{GoalEnabled: true}, repo)
	validSnapshot := protocol.GoalUsageSourceSnapshot{
		OwnerUserID:            "owner-1",
		RuntimeSessionKey:      "runtime-1",
		SourceKind:             protocol.GoalUsageSourceKindNXSTask,
		SourceID:               "task-1",
		CumulativeActualTokens: 1,
		GoalSessionKey:         "agent:nexus:ws:dm:conversation-1",
		ScopeRoundID:           "scope-1",
	}
	for _, mutate := range []func(*protocol.GoalUsageSourceSnapshot){
		func(item *protocol.GoalUsageSourceSnapshot) { item.OwnerUserID = "" },
		func(item *protocol.GoalUsageSourceSnapshot) { item.GoalSessionKey = "" },
		func(item *protocol.GoalUsageSourceSnapshot) { item.ScopeRoundID = "" },
		func(item *protocol.GoalUsageSourceSnapshot) { item.SourceKind = "other" },
	} {
		item := validSnapshot
		mutate(&item)
		if _, err := service.RecordUsageSourceSnapshot(context.Background(), item); !errors.Is(err, ErrGoalInvalidInput) {
			t.Fatalf("snapshot = %+v, error = %v, want invalid input", item, err)
		}
	}

	validClaim := protocol.GoalUsageSourceRoundClaim{
		OwnerUserID:    "owner-1",
		SourceKind:     protocol.GoalUsageSourceKindNXSTask,
		ScopeRoundID:   "scope-1",
		GoalID:         "goal-1",
		GoalSessionKey: "agent:nexus:ws:dm:conversation-1",
	}
	for _, mutate := range []func(*protocol.GoalUsageSourceRoundClaim){
		func(item *protocol.GoalUsageSourceRoundClaim) { item.OwnerUserID = "" },
		func(item *protocol.GoalUsageSourceRoundClaim) { item.GoalSessionKey = "" },
		func(item *protocol.GoalUsageSourceRoundClaim) { item.ScopeRoundID = "" },
		func(item *protocol.GoalUsageSourceRoundClaim) { item.SourceKind = "other" },
	} {
		item := validClaim
		mutate(&item)
		if _, err := service.ClaimUsageSourceRound(context.Background(), item); !errors.Is(err, ErrGoalInvalidInput) {
			t.Fatalf("claim = %+v, error = %v, want invalid input", item, err)
		}
	}
}

func TestServiceNormalizesRoomParentLedgerAndFromNowBinding(t *testing.T) {
	repo := &capturingUsageSourceRepository{memoryRepository: newMemoryRepository()}
	service := NewService(config.Config{GoalEnabled: true}, repo)

	if _, err := service.RecordUsageParentSnapshot(
		context.Background(),
		protocol.GoalUsageParentSnapshot{
			OwnerUserID:        " owner-1 ",
			GoalSessionKey:     " room:group:conversation-1 ",
			ScopeRoundID:       " root-round-1 ",
			SourceRoundID:      " slot-round-1 ",
			TokenUsageObserved: true,
		},
	); err != nil {
		t.Fatal(err)
	}
	if repo.parentSnapshot.OwnerUserID != "owner-1" ||
		repo.parentSnapshot.GoalSessionKey != "room:group:conversation-1" ||
		repo.parentSnapshot.ScopeRoundID != "root-round-1" ||
		repo.parentSnapshot.SourceRoundID != "slot-round-1" ||
		repo.parentSnapshot.EventID == "" ||
		repo.parentSnapshot.ObservedAt.IsZero() {
		t.Fatalf("parent snapshot = %+v, want normalized identity and generated audit fields", repo.parentSnapshot)
	}

	if _, err := service.BindUsageScopeFromNow(
		context.Background(),
		protocol.GoalUsageScopeBinding{
			OwnerUserID:    " owner-1 ",
			GoalSessionKey: " room:group:conversation-1 ",
			ScopeRoundID:   " root-round-1 ",
			GoalID:         " goal-1 ",
		},
	); err != nil {
		t.Fatal(err)
	}
	if repo.binding.OwnerUserID != "owner-1" ||
		repo.binding.GoalSessionKey != "room:group:conversation-1" ||
		repo.binding.SourceKind != protocol.GoalUsageSourceKindNXSTask ||
		repo.binding.ScopeRoundID != "root-round-1" ||
		repo.binding.GoalID != "goal-1" ||
		repo.binding.BoundAt.IsZero() {
		t.Fatalf("from-now binding = %+v, want normalized durable scope", repo.binding)
	}
}

func TestServiceRequiresRoomParentLedgerIdentity(t *testing.T) {
	repo := &capturingUsageSourceRepository{memoryRepository: newMemoryRepository()}
	service := NewService(config.Config{GoalEnabled: true}, repo)
	valid := protocol.GoalUsageParentSnapshot{
		OwnerUserID:        "owner-1",
		GoalSessionKey:     "room:group:conversation-1",
		ScopeRoundID:       "root-round-1",
		SourceRoundID:      "slot-round-1",
		TokenUsageObserved: true,
	}
	for _, mutate := range []func(*protocol.GoalUsageParentSnapshot){
		func(item *protocol.GoalUsageParentSnapshot) { item.OwnerUserID = "" },
		func(item *protocol.GoalUsageParentSnapshot) { item.GoalSessionKey = "" },
		func(item *protocol.GoalUsageParentSnapshot) { item.ScopeRoundID = "" },
		func(item *protocol.GoalUsageParentSnapshot) { item.SourceRoundID = "" },
	} {
		item := valid
		mutate(&item)
		if _, err := service.RecordUsageParentSnapshot(context.Background(), item); !errors.Is(err, ErrGoalInvalidInput) {
			t.Fatalf("parent snapshot = %+v, error = %v, want invalid input", item, err)
		}
	}
}

func TestServiceCreatesCompletedExternalGoalWithFinalizedZeroUsage(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(testConfig(), repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()
	complete := goalappserver.ThreadGoalStatusComplete
	objective := "Already complete"

	completed, err := service.SetFromThreadGoalParams(ctx, goalappserver.ThreadGoalSetParams{
		ThreadID:  "agent:nexus:ws:dm:external-complete-idle",
		Objective: &objective,
		Status:    &complete,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !completed.UsageFinalized || completed.Usage.ActualTokens() != 0 {
		t.Fatalf("completed = %#v, want authoritative finalized zero usage", completed)
	}
}

func testConfig() config.Config {
	return config.Config{GoalEnabled: true}
}

func TestRuntimeUsageAccumulatorAddsDistinctTurnsAndReconcilesFinal(t *testing.T) {
	accumulator := NewRuntimeUsageAccumulator(true)

	first, ok := accumulator.Delta(runtimeTurnSnapshot("turn-a", 90, 10))
	if ok {
		t.Fatalf("first delta = %#v, ok = true, want all token fields deferred", first)
	}
	if duplicate, ok := accumulator.Delta(runtimeTurnSnapshot("turn-a", 90, 10)); ok {
		t.Fatalf("duplicate turn delta = %#v, want none", duplicate)
	}
	second, ok := accumulator.Delta(runtimeTurnSnapshot("turn-b", 70, 10))
	if ok {
		t.Fatalf("second delta = %#v, ok = true, want all token fields deferred", second)
	}
	final, ok := accumulator.Delta(RuntimeUsageSnapshot{
		Usage:      protocol.GoalUsage{InputTokens: 160, OutputTokens: 20, ActualTotalTokens: 180},
		Cumulative: true,
		Terminal:   true,
	})
	if !ok || final.ActualTokens() != 180 || final.BudgetTokens() != 180 {
		t.Fatalf("reconciled final delta = %#v, ok = %v, want complete terminal 180", final, ok)
	}
}

func TestRuntimeUsageAccumulatorModelAndExternalActivationUseDifferentBaselines(t *testing.T) {
	modelCreated := NewRuntimeUsageAccumulator(false)
	if delta, ok := modelCreated.Delta(runtimeTurnSnapshot("turn-a", 90, 10)); ok {
		t.Fatalf("inactive delta = %#v, want none", delta)
	}
	backlog, ok := modelCreated.ActivateFromRoundStart()
	if ok {
		t.Fatalf("model-created backlog = %#v, ok = true, want tokens deferred", backlog)
	}
	next, ok := modelCreated.Delta(runtimeTurnSnapshot("turn-b", 70, 10))
	if ok {
		t.Fatalf("model-created next delta = %#v, ok = true, want tokens deferred", next)
	}

	externalCreated := NewRuntimeUsageAccumulator(false)
	if delta, ok := externalCreated.Delta(runtimeTurnSnapshot("turn-a", 90, 10)); ok {
		t.Fatalf("inactive delta = %#v, want none", delta)
	}
	externalCreated.Reset(runtimeTurnSnapshot("turn-a", 90, 10))
	next, ok = externalCreated.Delta(runtimeTurnSnapshot("turn-b", 70, 10))
	if ok {
		t.Fatalf("external-created delta = %#v, ok = true, want tokens deferred", next)
	}
}

func TestRuntimeUsageAccumulatorUnboundTerminalEligibilityIsPristineOnly(t *testing.T) {
	pristine := NewRuntimeUsageAccumulator(false)
	pristine.PrepareDelta(RuntimeUsageSnapshot{TokenUsageObserved: true})
	if !pristine.EligibleForUnboundTerminal() {
		t.Fatal("observed but never-bound accumulator was not eligible for unbound terminal")
	}

	activated := NewRuntimeUsageAccumulator(false)
	activated.PrepareActivationFromRoundStart()
	if activated.EligibleForUnboundTerminal() {
		t.Fatal("activated accumulator remained eligible for unbound terminal")
	}

	reset := NewRuntimeUsageAccumulator(false)
	reset.Reset(RuntimeUsageSnapshot{})
	if reset.EligibleForUnboundTerminal() {
		t.Fatal("Reset accumulator remained eligible for unbound terminal")
	}

	closed := NewRuntimeUsageAccumulator(false)
	closed.Close()
	if closed.EligibleForUnboundTerminal() {
		t.Fatal("closed accumulator remained eligible for unbound terminal")
	}
	if delta, ok := closed.PrepareActivationFromRoundStart(); ok || closed.Active() {
		t.Fatalf("closed accumulator reactivated with delta %#v", delta)
	}

	reset.Close()
	if delta, ok := reset.PrepareActivationFromRoundStart(); ok || reset.Active() {
		t.Fatalf("previously activated accumulator reactivated with delta %#v", delta)
	}
}

func runtimeTurnSnapshot(turnID string, inputTokens int64, outputTokens int64) RuntimeUsageSnapshot {
	return RuntimeUsageSnapshot{
		TurnID: turnID,
		Usage: protocol.GoalUsage{
			InputTokens:       inputTokens,
			OutputTokens:      outputTokens,
			ActualTotalTokens: inputTokens + outputTokens,
		},
	}
}

func TestServiceWallClockAccountingStopsWhileContinuationSuppressed(t *testing.T) {
	repo := newMemoryRepository()
	clock := newMutableClock(time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC))
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = clock.Now
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:wall-clock-empty-continuation",
		Objective:  "Do not count idle continuation hold time",
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := service.RecordUsageForSession(ctx, created.SessionKey, protocol.GoalUsage{RuntimeSeconds: 5}, "round-1"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.RecordContinuationProgress(ctx, created.ID, "goal-continuation-1", false); err != nil {
		t.Fatal(err)
	}
	if _, err := service.RecordContinuationProgress(ctx, created.ID, "goal-continuation-2", false); err != nil {
		t.Fatal(err)
	}

	clock.Advance(20 * time.Second)
	objective := "Do not count idle continuation hold time, updated"
	updated, err := service.Update(ctx, created.ID, protocol.UpdateGoalRequest{Objective: &objective})
	if err != nil {
		t.Fatal(err)
	}
	if updated.TimeUsedSeconds != 5 {
		t.Fatalf("updated.TimeUsedSeconds = %d, want only counted runtime before continuation hold", updated.TimeUsedSeconds)
	}

	if _, err := service.RecordContinuationProgress(ctx, created.ID, "round-2", true); err != nil {
		t.Fatal(err)
	}
	clock.Advance(3 * time.Second)
	_, goal, err := service.RuntimeContext(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if goal == nil || goal.TimeUsedSeconds != 8 {
		t.Fatalf("runtime goal = %#v, want wall clock to resume after real progress", goal)
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
	interrupter := &fakeRuntimeInterrupter{}
	service.SetRuntimeInterrupter(interrupter)

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
	if len(accountant.clearedSessionKeys) != 0 {
		t.Fatalf("accountant clearedSessionKeys = %#v, pause must retain terminal accounting", accountant.clearedSessionKeys)
	}
	if len(accountant.finalizingSessionKeys) != 1 ||
		accountant.finalizingSessionKeys[0] != created.SessionKey {
		t.Fatalf("accountant finalizingSessionKeys = %#v, want current session", accountant.finalizingSessionKeys)
	}
	if len(interrupter.sessionKeys) != 1 || interrupter.sessionKeys[0] != created.SessionKey {
		t.Fatalf("interrupter sessionKeys = %#v, want current session", interrupter.sessionKeys)
	}
	if len(interrupter.roundIDs) != 1 || strings.Join(interrupter.roundIDs[0], ",") != "round-running" {
		t.Fatalf("interrupter roundIDs = %#v, want exact Goal-accounting round", interrupter.roundIDs)
	}
	if len(repo.events) != 3 || repo.events[1].EventType != "usage_recorded" || repo.events[2].EventType != "paused" {
		t.Fatalf("events = %#v, want usage_recorded before paused", repo.events)
	}
}
