package goal

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type durableMemoryGoalRepository struct {
	*memoryRepository
	plans map[string]protocol.GoalContinuationPlan
}

func newDurableMemoryGoalRepository() *durableMemoryGoalRepository {
	return &durableMemoryGoalRepository{memoryRepository: newMemoryRepository(), plans: map[string]protocol.GoalContinuationPlan{}}
}

func (r *durableMemoryGoalRepository) ReserveGoalContinuation(ctx context.Context, item protocol.Goal, expectedVersion int64, event protocol.GoalEvent, plan protocol.GoalContinuationPlan) (*protocol.Goal, error) {
	for _, current := range r.plans {
		if current.GoalID == plan.GoalID && current.ObjectiveRevision == plan.ObjectiveRevision &&
			(current.Status == protocol.GoalContinuationPlanStatusScheduled || current.Status == protocol.GoalContinuationPlanStatusClaimed || current.Status == protocol.GoalContinuationPlanStatusStarted) {
			return nil, sql.ErrNoRows
		}
	}
	updated, err := r.memoryRepository.UpdateGoalWithEvents(ctx, item, expectedVersion, []protocol.GoalEvent{event})
	if err != nil {
		return nil, err
	}
	r.plans[plan.RoundID] = plan
	return updated, nil
}

func (r *durableMemoryGoalRepository) GetOpenGoalContinuation(_ context.Context, goalID string, revision int64) (*protocol.GoalContinuationPlan, error) {
	for _, plan := range r.plans {
		if plan.GoalID == goalID && plan.ObjectiveRevision == revision &&
			(plan.Status == protocol.GoalContinuationPlanStatusScheduled || plan.Status == protocol.GoalContinuationPlanStatusClaimed || plan.Status == protocol.GoalContinuationPlanStatusStarted) {
			copy := plan
			return &copy, nil
		}
	}
	return nil, nil
}

func (r *durableMemoryGoalRepository) ClaimGoalContinuation(_ context.Context, roundID string, now time.Time, leaseEnd time.Time) (*protocol.GoalContinuationPlan, error) {
	plan, ok := r.plans[roundID]
	if !ok || (plan.Status == protocol.GoalContinuationPlanStatusScheduled && plan.NextAttemptAt != nil && plan.NextAttemptAt.After(now)) ||
		((plan.Status == protocol.GoalContinuationPlanStatusClaimed || plan.Status == protocol.GoalContinuationPlanStatusStarted) && plan.ClaimExpiresAt != nil && plan.ClaimExpiresAt.After(now)) {
		return nil, sql.ErrNoRows
	}
	plan.Status = protocol.GoalContinuationPlanStatusClaimed
	plan.AttemptCount++
	plan.Version++
	plan.NextAttemptAt = nil
	plan.ClaimExpiresAt = &leaseEnd
	plan.UpdatedAt = now
	r.plans[roundID] = plan
	return &plan, nil
}

func (r *durableMemoryGoalRepository) MarkGoalContinuationStarted(_ context.Context, roundID string, now, recoveryAt time.Time) error {
	plan, ok := r.plans[roundID]
	if !ok {
		return sql.ErrNoRows
	}
	if plan.Status == protocol.GoalContinuationPlanStatusStarted {
		return nil
	}
	if plan.Status != protocol.GoalContinuationPlanStatusClaimed {
		return sql.ErrNoRows
	}
	plan.Status = protocol.GoalContinuationPlanStatusStarted
	plan.ClaimExpiresAt = &recoveryAt
	plan.SettledAt = nil
	plan.UpdatedAt = now
	r.plans[roundID] = plan
	return nil
}

func (r *durableMemoryGoalRepository) SettleGoalContinuation(_ context.Context, goalID, roundID string, revision int64, now time.Time) error {
	plan, ok := r.plans[roundID]
	if !ok || plan.GoalID != goalID || plan.ObjectiveRevision != revision ||
		(plan.Status != protocol.GoalContinuationPlanStatusClaimed && plan.Status != protocol.GoalContinuationPlanStatusStarted) {
		if ok && plan.Status == protocol.GoalContinuationPlanStatusSettled {
			return nil
		}
		return sql.ErrNoRows
	}
	plan.Status = protocol.GoalContinuationPlanStatusSettled
	plan.ClaimExpiresAt = nil
	plan.SettledAt = &now
	plan.UpdatedAt = now
	r.plans[roundID] = plan
	return nil
}

func (r *durableMemoryGoalRepository) RetryGoalContinuation(_ context.Context, roundID string, reason string, next time.Time, now time.Time) error {
	plan, ok := r.plans[roundID]
	if !ok || plan.Status != protocol.GoalContinuationPlanStatusClaimed {
		return sql.ErrNoRows
	}
	plan.Status = protocol.GoalContinuationPlanStatusScheduled
	plan.ClaimExpiresAt = nil
	plan.NextAttemptAt = &next
	plan.LastError = reason
	plan.UpdatedAt = now
	r.plans[roundID] = plan
	return nil
}

func (r *durableMemoryGoalRepository) ReleaseGoalContinuation(ctx context.Context, item protocol.Goal, expectedVersion int64, event protocol.GoalEvent, roundID string, now time.Time) (*protocol.Goal, error) {
	plan, ok := r.plans[roundID]
	if !ok || (plan.Status != protocol.GoalContinuationPlanStatusScheduled && plan.Status != protocol.GoalContinuationPlanStatusClaimed) {
		return nil, sql.ErrNoRows
	}
	updated, err := r.memoryRepository.UpdateGoalWithEvents(ctx, item, expectedVersion, []protocol.GoalEvent{event})
	if err != nil {
		return nil, err
	}
	plan.Status = protocol.GoalContinuationPlanStatusReleased
	plan.SettledAt = &now
	r.plans[roundID] = plan
	return updated, nil
}

func TestServiceDurableContinuationRecoversScheduleAndExpiredClaimWithoutRecount(t *testing.T) {
	repo := newDurableMemoryGoalRepository()
	service := NewService(config.Config{
		GoalEnabled: true, GoalAutoContinueEnabled: true, GoalMaxContinuationsPerRun: 3,
	}, repo)
	now := time.Date(2026, 8, 14, 10, 0, 0, 0, time.UTC)
	service.nowFn = func() time.Time { return now }
	service.idFactory = sequentialID()
	ctx := context.Background()
	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:durable-service", Objective: "recover exactly once",
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.PlanContinuationForSession(ctx, created.SessionKey, "round-before")
	if err != nil || first == nil {
		t.Fatalf("first plan = %#v, %v", first, err)
	}
	if len(continuationReservations(first.Goal.Metadata)) != 0 {
		t.Fatalf("durable prompt identity leaked into Goal metadata: %#v", first.Goal.Metadata)
	}
	recovered, err := service.PlanContinuationForSession(ctx, created.SessionKey, "ignored-after-crash")
	if err != nil || recovered == nil || recovered.RoundID != first.RoundID || recovered.Prompt != first.Prompt {
		t.Fatalf("schedule recovery = %#v, %v; want original plan", recovered, err)
	}
	if recovered.Goal.ContinuationCount != 1 {
		t.Fatalf("schedule recovery count = %d, want 1", recovered.Goal.ContinuationCount)
	}
	if _, err = service.ClaimContinuationPlan(ctx, *recovered); err != nil {
		t.Fatal(err)
	}
	if early, err := service.PlanContinuationForSession(ctx, created.SessionKey, ""); err != nil || early != nil {
		t.Fatalf("live lease recovery = %#v, %v, want deferred", early, err)
	}
	now = now.Add(goalContinuationClaimLease + time.Second)
	afterClaimCrash, err := service.PlanContinuationForSession(ctx, created.SessionKey, "")
	if err != nil || afterClaimCrash == nil || afterClaimCrash.RoundID != first.RoundID {
		t.Fatalf("expired claim recovery = %#v, %v", afterClaimCrash, err)
	}
	current, err := service.Current(ctx, created.SessionKey)
	if err != nil || current.ContinuationCount != 1 {
		t.Fatalf("count after claim recovery = %#v, %v", current, err)
	}
}

func TestServiceDurableContinuationRecoversStartedCrashWithoutRecount(t *testing.T) {
	repo := newDurableMemoryGoalRepository()
	service := NewService(config.Config{
		GoalEnabled: true, GoalAutoContinueEnabled: true, GoalMaxContinuationsPerRun: 3,
	}, repo)
	now := time.Date(2026, 8, 14, 10, 30, 0, 0, time.UTC)
	service.nowFn = func() time.Time { return now }
	service.idFactory = sequentialID()
	ctx := context.Background()
	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:started-crash", Objective: "recover a registered runtime",
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := service.PlanContinuationForSession(ctx, created.SessionKey, "round-before")
	if err != nil || plan == nil {
		t.Fatalf("plan = %#v, %v", plan, err)
	}
	if _, err = service.ClaimContinuationPlan(ctx, *plan); err != nil {
		t.Fatal(err)
	}
	if err = service.MarkContinuationPlanStarted(ctx, *plan); err != nil {
		t.Fatal(err)
	}
	if beforeExpiry, err := service.PlanContinuationForSession(ctx, created.SessionKey, ""); err != nil || beforeExpiry != nil {
		t.Fatalf("started owner before expiry = %#v, %v, want deferred", beforeExpiry, err)
	}
	now = now.Add(goalContinuationStartedLease + time.Second)
	recovered, err := service.PlanContinuationForSession(ctx, created.SessionKey, "")
	if err != nil || recovered == nil || recovered.RoundID != plan.RoundID || recovered.Prompt != plan.Prompt {
		t.Fatalf("started crash recovery = %#v, %v; want original plan", recovered, err)
	}
	current, err := service.Current(ctx, created.SessionKey)
	if err != nil || current.ContinuationCount != 1 {
		t.Fatalf("count after started crash = %#v, %v, want 1", current, err)
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

func TestServiceReleaseConcurrentContinuationReservationsExactlyOnce(t *testing.T) {
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
		Objective:  "Finish the audit",
	})
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.PlanContinuationForSession(ctx, created.SessionKey, "round-1")
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.PlanContinuationForSession(ctx, created.SessionKey, "round-2")
	if err != nil {
		t.Fatal(err)
	}
	if first == nil || second == nil || second.Goal.ContinuationCount != 2 {
		t.Fatalf("plans = %#v / %#v, want two reservations", first, second)
	}

	if _, err := service.ReleaseContinuationPlan(ctx, *first, "explicit input won"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ReleaseContinuationPlan(ctx, *second, "explicit input won"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ReleaseContinuationPlan(ctx, *first, "duplicate release"); err != nil {
		t.Fatal(err)
	}
	current, err := service.Current(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current.ContinuationCount != 0 || len(continuationReservations(current.Metadata)) != 0 {
		t.Fatalf("current = %#v, want both reservations released exactly once", current)
	}
}

func TestServiceClaimContinuationKeepsCountAndMakesReleaseNoop(t *testing.T) {
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
		SessionKey: "agent:nexus:ws:dm:claim",
		Objective:  "Finish exactly once",
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := service.PlanContinuationForSession(ctx, created.SessionKey, "round-before")
	if err != nil || plan == nil {
		t.Fatalf("plan = %#v err=%v", plan, err)
	}
	if _, err := service.ClaimContinuationPlan(ctx, *plan); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ClaimContinuationPlan(ctx, *plan); !errors.Is(err, ErrGoalRevisionStale) {
		t.Fatalf("duplicate claim error = %v, want ErrGoalRevisionStale", err)
	}
	if _, err := service.ReleaseContinuationPlan(ctx, *plan, "duplicate dispatch saw runtime running"); err != nil {
		t.Fatal(err)
	}
	current, err := service.Current(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current.ContinuationCount != 1 || len(continuationReservations(current.Metadata)) != 0 {
		t.Fatalf("current = %#v, want claimed continuation counted once without pending reservation", current)
	}
	startedEvents := 0
	deferredEvents := 0
	for _, event := range repo.events {
		switch event.EventType {
		case "continuation_started":
			startedEvents++
		case "continuation_deferred":
			deferredEvents++
		}
	}
	if startedEvents != 1 || deferredEvents != 0 {
		t.Fatalf("started=%d deferred=%d, want idempotent claim and no deferred event", startedEvents, deferredEvents)
	}
}

func TestServiceClaimContinuationRejectsRetargetAfterValidation(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{
		GoalEnabled:             true,
		GoalAutoContinueEnabled: true,
	}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:claim-retarget",
		Objective:  "Analyze M3 and M4",
		CreatedBy:  "model",
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := service.PlanContinuationForSession(ctx, created.SessionKey, "round-before")
	if err != nil || plan == nil {
		t.Fatalf("plan = %#v err=%v", plan, err)
	}
	current, err := service.GoalContinuationStillCurrent(ctx, *plan)
	if err != nil || !current {
		t.Fatalf("pre-claim validation = %v err=%v, want current", current, err)
	}
	retargeted, err := service.RetargetByModel(ctx, created.SessionKey, protocol.RetargetGoalRequest{
		Objective:                 "Analyze M4 and M5",
		RoundID:                   "round-correction",
		ExpectedObjectiveRevision: plan.Goal.ObjectiveRevision(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.ClaimContinuationPlan(ctx, *plan); !errors.Is(err, ErrGoalRevisionStale) {
		t.Fatalf("claim after retarget error = %v, want ErrGoalRevisionStale", err)
	}
	currentGoal, err := service.Current(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if currentGoal.Objective != retargeted.Objective || currentGoal.ObjectiveRevision() != retargeted.ObjectiveRevision() ||
		currentGoal.ContinuationCount != 0 || len(continuationReservations(currentGoal.Metadata)) != 0 {
		t.Fatalf("current = %#v, want retargeted Goal without stale reservation", currentGoal)
	}
	for _, event := range repo.events {
		if event.EventType == "continuation_started" && event.RoundID == plan.RoundID {
			t.Fatalf("stale continuation emitted started event: %#v", event)
		}
	}
}

func TestServiceResumeUsageLimitedGoalCannotReopenSameContinuationEpoch(t *testing.T) {
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
	if !errors.Is(err, ErrGoalInvalidInput) || resumed != nil {
		t.Fatalf("Resume() = %#v, %v; want explicit retarget guidance", resumed, err)
	}
	current, currentErr := service.Current(ctx, created.SessionKey)
	if currentErr != nil {
		t.Fatal(currentErr)
	}
	if current.Status != protocol.GoalStatusUsageLimited || current.ContinuationCount != 1 {
		t.Fatalf("current = %#v, want exhausted epoch unchanged", current)
	}
}
