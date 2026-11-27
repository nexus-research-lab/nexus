// usage_test.go 覆盖 Goal 用量预算、终态结算、作用域记账与墙钟预算测试。
package goal

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalappserver "github.com/nexus-research-lab/nexus/internal/service/goal/appserver"
)

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
