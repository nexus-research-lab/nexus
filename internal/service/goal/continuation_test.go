// continuation_test.go 覆盖 Goal 续跑计划、进度与对齐测试。
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
