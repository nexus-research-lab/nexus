// usage_test.go 覆盖 Goal 用量预算、终态结算、作用域记账与墙钟预算测试。
package goal

import (
	"context"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

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

func testConfig() config.Config {
	return config.Config{GoalEnabled: true}
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
