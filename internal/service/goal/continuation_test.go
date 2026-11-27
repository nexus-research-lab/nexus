// continuation_test.go 覆盖 Goal 续跑计划、进度与对齐测试。
package goal

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

func TestServiceUserStopSettlesReceiptAndSuspendsContinuation(t *testing.T) {
	repo := newDurableMemoryGoalRepository()
	service := NewService(config.Config{
		GoalEnabled: true, GoalAutoContinueEnabled: true, GoalMaxContinuationsPerRun: 5,
	}, repo)
	now := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	service.nowFn = func() time.Time { return now }
	service.idFactory = sequentialID()
	ctx := context.Background()
	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:user-stop", Objective: "stop means stop",
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

	stopped, err := service.RecordContinuationUserStop(ctx, created.ID, continuationRuntimeIdentity(plan.RoundID), created.ObjectiveRevision())
	if err != nil {
		t.Fatal(err)
	}
	if stopped.Status != protocol.GoalStatusActive ||
		stopped.ContinuationState() != protocol.GoalContinuationStateSuspended ||
		stopped.ContinuationSuspensionReason() != protocol.GoalContinuationSuspensionUserStopped {
		t.Fatalf("after user stop = status %q state %q reason %q", stopped.Status, stopped.ContinuationState(), stopped.ContinuationSuspensionReason())
	}
	if open, err := repo.GetOpenGoalContinuation(ctx, created.ID, created.ObjectiveRevision()); err != nil || open != nil {
		t.Fatalf("open receipt after user stop = %#v, %v; want settled", open, err)
	}
	if next, err := service.PlanContinuationForSession(ctx, created.SessionKey, plan.RoundID); err != nil || next != nil {
		t.Fatalf("plan right after user stop = %#v, %v; want none", next, err)
	}
	now = now.Add(goalContinuationStartedLease + time.Minute)
	if replay, err := service.PlanContinuationForSession(ctx, created.SessionKey, ""); err != nil || replay != nil {
		t.Fatalf("plan after started lease expiry = %#v, %v; want no replay of the stopped round", replay, err)
	}
	if again, err := service.RecordContinuationUserStop(ctx, created.ID, continuationRuntimeIdentity(plan.RoundID), created.ObjectiveRevision()); err != nil || again.Version != stopped.Version {
		t.Fatalf("repeated user stop = %#v, %v; want idempotent", again, err)
	}

	resumed, err := service.Resume(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.ContinuationState() != protocol.GoalContinuationStateReady || continuationStoppedByUser(resumed.Metadata) {
		t.Fatalf("after resume = state %q metadata %#v; want ready without stop marker", resumed.ContinuationState(), resumed.Metadata)
	}
	next, err := service.PlanContinuationForSession(ctx, created.SessionKey, "")
	if err != nil || next == nil {
		t.Fatalf("plan after resume = %#v, %v; want a fresh continuation", next, err)
	}
	if next.RoundID == plan.RoundID {
		t.Fatalf("resume replayed the stopped round %q", plan.RoundID)
	}
}

func TestServiceNoProgressSuspensionClearsStaleUserStopMarker(t *testing.T) {
	repo := newDurableMemoryGoalRepository()
	service := NewService(config.Config{
		GoalEnabled: true, GoalAutoContinueEnabled: true, GoalMaxContinuationsPerRun: 10,
	}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()
	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:stale-stop", Objective: "reason stays accurate",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.RecordContinuationUserStop(ctx, created.ID, continuationRuntimeIdentity("round-stop"), created.ObjectiveRevision()); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Resume(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	var current *protocol.Goal
	for index := range goalContinuationSuppressionThreshold {
		current, err = service.RecordContinuationProgress(ctx, created.ID, fmt.Sprintf("round-empty-%d", index), false, created.ObjectiveRevision())
		if err != nil {
			t.Fatal(err)
		}
	}
	if current.ContinuationSuspensionReason() != protocol.GoalContinuationSuspensionNoProgress {
		t.Fatalf("suspension reason = %q, want no_progress", current.ContinuationSuspensionReason())
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

func TestServiceRecordRoomGoalCollaborationHandbackClearsSuppressionWithoutResettingContinuationLimit(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{
		GoalEnabled:             true,
		GoalAutoContinueEnabled: true,
	}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: protocol.BuildRoomSharedSessionKey("conversation-handback"),
		Objective:  "Continue after Room collaboration",
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := service.PlanContinuationForSession(ctx, created.SessionKey, "round-source")
	if err != nil || plan == nil {
		t.Fatalf("plan=%+v err=%v", plan, err)
	}
	if _, err := service.RecordContinuationProgress(
		ctx,
		created.ID,
		plan.RoundID,
		false,
		created.ObjectiveRevision(),
	); err != nil {
		t.Fatal(err)
	}
	updated, err := service.RecordRoomGoalCollaborationHandback(
		ctx,
		created.ID,
		"room-handoff-target",
		created.ObjectiveRevision(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if updated.EmptyProgressCount != 0 || updated.LastError != "" ||
		updated.ContinuationCount != 1 {
		t.Fatalf("updated=%+v, want suppression cleared and continuation count preserved", updated)
	}
	if got := repo.events[len(repo.events)-1]; got.EventType != "room_collaboration_handback" ||
		got.RoundID != "room-handoff-target" {
		t.Fatalf("last event=%+v", got)
	}
}

func TestServicePlanContinuationUsesOneRecoveryTurnBeforeSuppressing(t *testing.T) {
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
	next, err := service.PlanContinuationForSession(ctx, created.SessionKey, plan.RoundID)
	if err != nil {
		t.Fatal(err)
	}
	if next == nil || !strings.Contains(next.Prompt, "single automatic recovery turn") {
		t.Fatalf("next plan = %#v, want explicit recovery continuation", next)
	}
	if _, err = service.RecordContinuationProgress(ctx, created.ID, next.RoundID, false); err != nil {
		t.Fatal(err)
	}
	if suppressed, planErr := service.PlanContinuationForSession(ctx, created.SessionKey, next.RoundID); planErr != nil {
		t.Fatal(planErr)
	} else if suppressed != nil {
		t.Fatalf("suppressed plan = %#v, want nil after two empty continuation turns", suppressed)
	}
	current, err := service.Current(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current.EmptyProgressCount != 2 || current.ContinuationCount != 2 {
		t.Fatalf("current = %#v, want suppression after one recovery continuation", current)
	}
	if got := repo.events[len(repo.events)-1]; got.EventType != "continuation_suppressed" || got.RoundID != next.RoundID {
		t.Fatalf("last event = %#v, want continuation_suppressed for continuation round", got)
	}
}

func TestServiceCompletionCommandMissCompletesAfterRetry(t *testing.T) {
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
		Objective:  "Finish with a proper Goal update",
		CreatedBy:  "model",
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := service.PlanContinuationForSession(ctx, created.SessionKey, "round-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.RecordCompletionCommandMiss(ctx, created.ID, plan.RoundID, "first miss"); err != nil {
		t.Fatal(err)
	}
	retry, err := service.PlanContinuationForSession(ctx, created.SessionKey, plan.RoundID)
	if err != nil {
		t.Fatal(err)
	}
	if retry == nil {
		t.Fatal("retry = nil, want finalization retry continuation")
	}
	if _, err := service.RecordCompletionCommandMiss(ctx, created.ID, retry.RoundID, "second miss"); err != nil {
		t.Fatal(err)
	}
	if next, err := service.PlanContinuationForSession(ctx, created.SessionKey, retry.RoundID); err != nil {
		t.Fatal(err)
	} else if next != nil {
		t.Fatalf("next = %#v, want nil after system completion", next)
	}
	current, err := service.CurrentOptional(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current != nil {
		t.Fatalf("current = %#v, want nil after system completion", current)
	}
	completed, err := repo.GetGoal(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != protocol.GoalStatusComplete || goalCompletionCommandRetryCount(completed.Metadata) != 0 {
		t.Fatalf("completed = %#v, want complete with retry metadata cleared", completed)
	}
	if got := repo.events[len(repo.events)-1]; got.EventType != "completed" || got.Payload["reason"] != "second miss" {
		t.Fatalf("last event = %#v, want completed event with second miss reason", got)
	}
}

func TestServiceCompletionCommandMissCannotBypassRoomReadiness(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{
		GoalEnabled:             true,
		GoalAutoContinueEnabled: true,
	}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	readiness := &fakeRoomGoalCompletionReadiness{blocker: "work item W2 is not accepted"}
	service.SetRoomGoalCompletionReadiness(readiness)
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: protocol.BuildRoomSharedSessionKey("completion-bypass"),
		Objective:  "Finish all assigned Room work",
		CreatedBy:  "model",
		AgentID:    "agent-lead",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.RecordCompletionCommandMiss(ctx, created.ID, "round-miss-1", "first miss"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.RecordCompletionCommandMiss(ctx, created.ID, "round-miss-2", "second miss"); err != nil {
		t.Fatal(err)
	}

	current, err := service.Current(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current.Status != protocol.GoalStatusActive {
		t.Fatalf("Goal status = %q, want active while Room work is outstanding", current.Status)
	}
	if readiness.callCount != 1 ||
		readiness.goalID != created.ID ||
		readiness.agentID != "agent-lead" ||
		readiness.roundID != "round-miss-2" {
		t.Fatalf(
			"readiness call = count:%d goal:%q agent:%q round:%q",
			readiness.callCount,
			readiness.goalID,
			readiness.agentID,
			readiness.roundID,
		)
	}
}

func TestServiceCompletionCommandMissCannotBypassExecutionReadiness(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{
		GoalEnabled:             true,
		GoalAutoContinueEnabled: true,
	}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	readiness := &fakeExecutionGoalCompletionReadiness{blocker: "terminal Work Item is not accepted"}
	service.SetExecutionGoalCompletionReadiness(readiness)
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:completion-bypass",
		Objective:  "Finish the accepted WorkGraph",
		Metadata: map[string]any{
			protocol.GoalMetadataExecutionID: "execution-completion-bypass",
			protocol.GoalMetadataExecutionBindingState: string(
				protocol.GoalExecutionBindingStateConfirmed,
			),
			protocol.GoalMetadataCompletionCriteria: []string{"accepted WorkGraph delivered"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.RecordCompletionCommandMiss(ctx, created.ID, "round-miss-1", "first miss"); err != nil {
		t.Fatal(err)
	}
	if _, err = service.AuditObjectiveAlignmentByModel(ctx, created.ID, protocol.AuditGoalObjectiveAlignmentRequest{
		Report: protocol.ObjectiveAlignmentReport{
			Decision: protocol.ObjectiveAlignmentAligned,
			CriteriaResults: []protocol.ObjectiveAlignmentCriterionResult{{
				Criterion: "accepted WorkGraph delivered",
				Status:    protocol.ObjectiveAlignmentCriterionSatisfied,
				Evidence: []protocol.ObjectiveAlignmentEvidence{{
					Ref: "workgraph:execution-completion-bypass", Claim: "the Goal reached the Execution readiness gate",
				}},
			}},
			Summary: "Objective alignment is satisfied independently of Execution readiness.",
		},
		RoundID: "round-miss-2",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err = service.RecordCompletionCommandMiss(ctx, created.ID, "round-miss-2", "second miss"); err != nil {
		t.Fatal(err)
	}

	current, err := service.Current(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current.Status != protocol.GoalStatusActive || readiness.callCount != 1 || readiness.goalID != created.ID {
		t.Fatalf("current = %#v readiness = %#v", current, readiness)
	}
}

func TestServicePlanContinuationCompletesStaleCompletionCommandMissSuppression(t *testing.T) {
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
		Objective:  "Finish with a proper Goal update",
		CreatedBy:  "model",
	})
	if err != nil {
		t.Fatal(err)
	}
	stale := repo.goals[created.ID]
	stale.EmptyProgressCount = goalContinuationSuppressionThreshold
	stale.Metadata = map[string]any{goalCompletionCommandRetryMetadataKey: 1}
	stale.Version++
	repo.goals[created.ID] = stale

	next, err := service.PlanContinuationForSession(ctx, created.SessionKey, "retry-round")
	if err != nil {
		t.Fatal(err)
	}
	if next != nil {
		t.Fatalf("next = %#v, want nil after stale completion finalization", next)
	}
	current, err := service.CurrentOptional(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current != nil {
		t.Fatalf("current = %#v, want nil after stale completion finalization", current)
	}
	completed := repo.goals[created.ID]
	if completed.Status != protocol.GoalStatusComplete || goalCompletionCommandRetryCount(completed.Metadata) != 0 {
		t.Fatalf("completed = %#v, want complete with retry metadata cleared", completed)
	}
	if !completed.UsageFinalized || completed.UsageFinalizedAt == nil {
		t.Fatalf("completed = %#v, want stale retry usage finalized", completed)
	}
	completedEvent := repo.events[len(repo.events)-2]
	finalizedEvent := repo.events[len(repo.events)-1]
	if completedEvent.EventType != "completed" || completedEvent.RoundID != "retry-round" ||
		finalizedEvent.EventType != "usage_finalized" || finalizedEvent.RoundID != "retry-round" {
		t.Fatalf(
			"terminal events = %#v / %#v, want completed then usage_finalized",
			completedEvent,
			finalizedEvent,
		)
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
	if updated.LastError != "Failed to authenticate. API Error: 401" || updated.EmptyProgressCount != goalContinuationSuppressionThreshold {
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
	if got := repo.events[len(repo.events)-1]; got.EventType != "continuation_recovery_scheduled" || got.RoundID != "goal_continuation_1" {
		t.Fatalf("last event = %#v, want continuation_recovery_scheduled after retry", got)
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

func TestManagedGoalCompletionRejectsNonAlignedOrStaleAudit(t *testing.T) {
	for _, test := range []struct {
		name            string
		decision        protocol.ObjectiveAlignmentDecision
		status          protocol.ObjectiveAlignmentCriterionStatus
		auditRound      string
		completionRound string
	}{
		{
			name:            "confirmed gap",
			decision:        protocol.ObjectiveAlignmentNotAligned,
			status:          protocol.ObjectiveAlignmentCriterionUnsatisfied,
			auditRound:      "round-final",
			completionRound: "round-final",
		},
		{
			name:            "stale round",
			decision:        protocol.ObjectiveAlignmentAligned,
			status:          protocol.ObjectiveAlignmentCriterionSatisfied,
			auditRound:      "round-before",
			completionRound: "round-final",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := newMemoryRepository()
			service := NewService(config.Config{GoalEnabled: true}, repo)
			service.nowFn = fixedClock()
			service.idFactory = sequentialID()
			service.SetExecutionGoalCompletionReadiness(
				&fakeExecutionGoalCompletionReadiness{},
			)
			created, err := service.Create(
				context.Background(),
				protocol.CreateGoalRequest{
					SessionKey: "agent:nexus:ws:dm:" + strings.ReplaceAll(test.name, " ", "-"),
					Objective:  "Ship report",
					Metadata: map[string]any{
						protocol.GoalMetadataExecutionID: "execution-" + strings.ReplaceAll(test.name, " ", "-"),
						protocol.GoalMetadataExecutionBindingState: string(
							protocol.GoalExecutionBindingStateConfirmed,
						),
						protocol.GoalMetadataActivationOrigin: string(protocol.GoalActivationOriginAdaptiveInitial),
						protocol.GoalMetadataCompletionCriteria: []string{
							"report accepted",
						},
					},
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			result := protocol.ObjectiveAlignmentCriterionResult{
				Criterion: "report accepted",
				Status:    test.status,
			}
			if test.status == protocol.ObjectiveAlignmentCriterionSatisfied {
				result.Evidence = []protocol.ObjectiveAlignmentEvidence{{
					Ref:   "review:accepted",
					Claim: "the final report was accepted",
				}}
			} else {
				result.Gap = "the report has not been accepted"
			}
			if _, err = service.AuditObjectiveAlignmentByModel(
				context.Background(),
				created.ID,
				protocol.AuditGoalObjectiveAlignmentRequest{
					Report: protocol.ObjectiveAlignmentReport{
						Decision:        test.decision,
						CriteriaResults: []protocol.ObjectiveAlignmentCriterionResult{result},
						Summary:         "current alignment state",
					},
					RoundID:                   test.auditRound,
					AgentID:                   "agent-1",
					ExpectedObjectiveRevision: created.ObjectiveRevision(),
				},
			); err != nil {
				t.Fatal(err)
			}
			if _, err = service.CompleteByModel(
				context.Background(),
				created.ID,
				protocol.CompleteGoalRequest{
					RoundID:                   test.completionRound,
					AgentID:                   "agent-1",
					ExpectedObjectiveRevision: created.ObjectiveRevision(),
				},
			); !errors.Is(err, ErrGoalInvalidState) {
				t.Fatalf("completion error = %v, want ErrGoalInvalidState", err)
			}
		})
	}
}

func TestManagedGoalCompletionCommandMissCannotBypassObjectiveAlignment(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{
		GoalEnabled:             true,
		GoalAutoContinueEnabled: true,
	}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	service.SetExecutionGoalCompletionReadiness(
		&fakeExecutionGoalCompletionReadiness{},
	)
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:alignment-miss",
		Objective:  "Ship a verified report",
		Metadata: map[string]any{
			protocol.GoalMetadataExecutionID: "execution-alignment-miss",
			protocol.GoalMetadataExecutionBindingState: string(
				protocol.GoalExecutionBindingStateConfirmed,
			),
			protocol.GoalMetadataActivationOrigin: string(protocol.GoalActivationOriginUserExplicit),
			protocol.GoalMetadataCompletionCriteria: []string{
				"report verified",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.RecordCompletionCommandMiss(
		ctx,
		created.ID,
		"round-first",
		"first miss",
	); err != nil {
		t.Fatal(err)
	}
	if _, err = service.RecordCompletionCommandMiss(
		ctx,
		created.ID,
		"round-final",
		"second miss",
	); err != nil {
		t.Fatal(err)
	}

	current, err := service.Current(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current.Status != protocol.GoalStatusActive {
		t.Fatalf("status = %q, want active without aligned audit", current.Status)
	}
	if got := repo.events[len(repo.events)-1]; got.EventType != "continuation_suppressed" ||
		!strings.Contains(got.Payload["reason"].(string), "objective alignment audit") {
		t.Fatalf("last event = %#v, want alignment suppression", got)
	}
}

func TestManagedGoalCompletionCommandMissCanUseSameRoundAlignedAudit(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{
		GoalEnabled:             true,
		GoalAutoContinueEnabled: true,
	}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	service.SetExecutionGoalCompletionReadiness(
		&fakeExecutionGoalCompletionReadiness{},
	)
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:aligned-miss",
		Objective:  "Ship a verified report",
		Metadata: map[string]any{
			protocol.GoalMetadataExecutionID: "execution-aligned-miss",
			protocol.GoalMetadataExecutionBindingState: string(
				protocol.GoalExecutionBindingStateConfirmed,
			),
			protocol.GoalMetadataActivationOrigin: string(protocol.GoalActivationOriginUserExplicit),
			protocol.GoalMetadataCompletionCriteria: []string{
				"report verified",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.RecordCompletionCommandMiss(
		ctx,
		created.ID,
		"round-first",
		"first miss",
	); err != nil {
		t.Fatal(err)
	}
	const finalRound = "round-final"
	if _, err = service.AuditObjectiveAlignmentByModel(
		ctx,
		created.ID,
		protocol.AuditGoalObjectiveAlignmentRequest{
			Report: protocol.ObjectiveAlignmentReport{
				Decision: protocol.ObjectiveAlignmentAligned,
				CriteriaResults: []protocol.ObjectiveAlignmentCriterionResult{{
					Criterion: "report verified",
					Status:    protocol.ObjectiveAlignmentCriterionSatisfied,
					Evidence: []protocol.ObjectiveAlignmentEvidence{{
						Ref:   "command:make-check",
						Claim: "the report verification passed",
					}},
				}},
				Summary: "The report is verified.",
			},
			RoundID:                   finalRound,
			AgentID:                   "agent-1",
			ExpectedObjectiveRevision: created.ObjectiveRevision(),
		},
	); err != nil {
		t.Fatal(err)
	}
	if _, err = service.RecordCompletionCommandMiss(
		ctx,
		created.ID,
		finalRound,
		"second miss",
	); err != nil {
		t.Fatal(err)
	}

	completed, err := repo.GetGoal(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != protocol.GoalStatusComplete {
		t.Fatalf("status = %q, want system completion backed by aligned audit", completed.Status)
	}
}
