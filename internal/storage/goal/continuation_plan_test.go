package goal

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"

	_ "modernc.org/sqlite"
)

func TestRepositoryGoalContinuationPlanSurvivesReopenAndFencesDuplicateWorkers(t *testing.T) {
	ctx := context.Background()
	databaseURL := filepath.Join(t.TempDir(), "goal-continuation.sqlite")
	db := openGoalContinuationTestDB(t, databaseURL)
	applyGoalMigration(t, db)
	repository := NewRepository(config.Config{DatabaseDriver: "sqlite"}, db)
	now := time.Date(2026, 8, 14, 8, 0, 0, 0, time.UTC)
	goal := protocol.Goal{
		ID: "goal-durable", SessionKey: "agent:nexus:ws:dm:durable",
		Objective: "finish durable recovery", Status: protocol.GoalStatusActive,
		ContinuationCount: 1, Version: 2, CreatedAt: now, UpdatedAt: now,
	}
	if _, err := repository.CreateGoal(ctx, protocol.Goal{
		ID: goal.ID, SessionKey: goal.SessionKey, Objective: goal.Objective,
		Status: goal.Status, Version: 1, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	event := protocol.GoalEvent{
		ID: "event-scheduled", GoalID: goal.ID, SessionKey: goal.SessionKey,
		EventType: "continuation_scheduled", Source: protocol.GoalUpdateSourceSystem,
		RoundID: "round-durable", CreatedAt: now,
	}
	next := now
	plan := protocol.GoalContinuationPlan{
		RoundID: "round-durable", GoalID: goal.ID, SessionKey: goal.SessionKey,
		ObjectiveRevision: 1, PreviousRoundID: "round-before",
		Prompt: "secret server prompt", Purpose: "goal_continuation",
		Metadata: map[string]string{"goal_id": goal.ID},
		Status:   protocol.GoalContinuationPlanStatusScheduled, Version: 1,
		NextAttemptAt: &next, CreatedAt: now, UpdatedAt: now,
	}
	if _, err := repository.ReserveGoalContinuation(ctx, goal, 1, event, plan); err != nil {
		t.Fatal(err)
	}
	deadline, err := repository.NextGoalContinuationAt(ctx)
	if err != nil || deadline == nil || !deadline.Equal(next) {
		t.Fatalf("scheduled continuation deadline = %v, err=%v", deadline, err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db = openGoalContinuationTestDB(t, databaseURL)
	repository = NewRepository(config.Config{DatabaseDriver: "sqlite"}, db)
	recovered, err := repository.GetOpenGoalContinuation(ctx, goal.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if recovered == nil || recovered.Prompt != plan.Prompt || recovered.PreviousRoundID != plan.PreviousRoundID {
		t.Fatalf("recovered = %#v, want complete server-only plan", recovered)
	}
	leaseEnd := now.Add(time.Minute)
	claimed, err := repository.ClaimGoalContinuation(ctx, plan.RoundID, now, leaseEnd)
	if err != nil {
		t.Fatal(err)
	}
	if claimed.Status != protocol.GoalContinuationPlanStatusClaimed || claimed.AttemptCount != 1 {
		t.Fatalf("claimed = %#v", claimed)
	}
	deadline, err = repository.NextGoalContinuationAt(ctx)
	if err != nil || deadline == nil || !deadline.Equal(leaseEnd) {
		t.Fatalf("claimed continuation deadline = %v, err=%v", deadline, err)
	}
	if _, err := repository.ClaimGoalContinuation(ctx, plan.RoundID, now, leaseEnd); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("duplicate claim error = %v, want CAS miss", err)
	}
	if _, err := repository.ClaimGoalContinuation(ctx, plan.RoundID, leaseEnd.Add(time.Second), leaseEnd.Add(time.Minute)); err != nil {
		t.Fatalf("expired lease recovery: %v", err)
	}
	startedAt := leaseEnd.Add(2 * time.Second)
	startedRecoveryAt := startedAt.Add(15 * time.Minute)
	if err := repository.MarkGoalContinuationStarted(ctx, plan.RoundID, startedAt, startedRecoveryAt); err != nil {
		t.Fatal(err)
	}
	deadline, err = repository.NextGoalContinuationAt(ctx)
	if err != nil || deadline == nil || !deadline.Equal(startedRecoveryAt) {
		t.Fatalf("started continuation deadline = %v, err=%v", deadline, err)
	}
	if err := repository.MarkGoalContinuationStarted(ctx, plan.RoundID, leaseEnd.Add(3*time.Second), startedRecoveryAt); err != nil {
		t.Fatalf("idempotent started settlement: %v", err)
	}
	if open, err := repository.GetOpenGoalContinuation(ctx, goal.ID, 1); err != nil || open == nil || open.Status != protocol.GoalContinuationPlanStatusStarted {
		t.Fatalf("open after started = %#v, %v; want started receipt owned by continuation recovery", open, err)
	}
	if _, err := repository.ClaimGoalContinuation(ctx, plan.RoundID, startedAt.Add(time.Minute), startedAt.Add(2*time.Minute)); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("live started receipt claim error = %v, want CAS miss", err)
	}
	if recovered, err := repository.ClaimGoalContinuation(ctx, plan.RoundID, startedRecoveryAt.Add(time.Second), startedRecoveryAt.Add(time.Minute)); err != nil || recovered.Status != protocol.GoalContinuationPlanStatusClaimed || recovered.AttemptCount != 3 {
		t.Fatalf("expired started recovery = %#v, %v", recovered, err)
	}
	if err := repository.SettleGoalContinuation(ctx, goal.ID, plan.RoundID, 1, startedRecoveryAt.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	deadline, err = repository.NextGoalContinuationAt(ctx)
	if err != nil || deadline != nil {
		t.Fatalf("settled continuation deadline = %v, err=%v", deadline, err)
	}
	if err := repository.SettleGoalContinuation(ctx, goal.ID, plan.RoundID, 1, startedRecoveryAt.Add(3*time.Second)); err != nil {
		t.Fatalf("idempotent terminal settlement: %v", err)
	}
	if err := repository.SettleGoalContinuation(ctx, "goal-wrong", plan.RoundID, 1, startedRecoveryAt.Add(4*time.Second)); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("mismatched terminal settlement error = %v, want fail closed", err)
	}
	if open, err := repository.GetOpenGoalContinuation(ctx, goal.ID, 1); err != nil || open != nil {
		t.Fatalf("open after terminal settlement = %#v, %v", open, err)
	}
}

func TestRepositoryGoalContinuationRetryPreservesCountAndCancelsOnRevisionChange(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 14, 9, 0, 0, 0, time.UTC)
	created, err := repository.CreateGoal(ctx, protocol.Goal{
		ID: "goal-retry", SessionKey: "agent:nexus:ws:dm:retry", Objective: "retry",
		Status: protocol.GoalStatusActive, Version: 1, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	created.ContinuationCount = 1
	created.Version++
	next := now
	plan := protocol.GoalContinuationPlan{
		RoundID: "round-retry", GoalID: created.ID, SessionKey: created.SessionKey,
		ObjectiveRevision: 1, Prompt: "retry prompt", Purpose: "goal_continuation",
		Status: protocol.GoalContinuationPlanStatusScheduled, Version: 1,
		NextAttemptAt: &next, CreatedAt: now, UpdatedAt: now,
	}
	event := protocol.GoalEvent{ID: "event-retry", GoalID: created.ID, SessionKey: created.SessionKey, EventType: "continuation_scheduled", Source: protocol.GoalUpdateSourceSystem, RoundID: plan.RoundID, CreatedAt: now}
	if _, err = repository.ReserveGoalContinuation(ctx, *created, 1, event, plan); err != nil {
		t.Fatal(err)
	}
	if _, err = repository.ClaimGoalContinuation(ctx, plan.RoundID, now, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err = repository.RetryGoalContinuation(ctx, plan.RoundID, "temporary startup failure", now.Add(time.Minute), now); err != nil {
		t.Fatal(err)
	}
	current, err := repository.GetGoal(ctx, created.ID)
	if err != nil || current.ContinuationCount != 1 || current.LastError != "" {
		t.Fatalf("Goal after launch retry = %#v, %v", current, err)
	}
	current.Objective = "retargeted"
	current.Metadata = map[string]any{protocol.GoalMetadataObjectiveRevision: int64(2)}
	current.Version++
	current.UpdatedAt = now.Add(2 * time.Minute)
	retargetEvent := protocol.GoalEvent{ID: "event-retarget", GoalID: current.ID, SessionKey: current.SessionKey, EventType: "updated", Source: protocol.GoalUpdateSourceModel, CreatedAt: current.UpdatedAt}
	if _, err = repository.UpdateGoalWithEvents(ctx, *current, 2, []protocol.GoalEvent{retargetEvent}); err != nil {
		t.Fatal(err)
	}
	if open, err := repository.GetOpenGoalContinuation(ctx, current.ID, 1); err != nil || open != nil {
		t.Fatalf("old revision open plan = %#v, %v", open, err)
	}
}

func TestRepositoryStartedContinuationKeepsOneOpenPlanFenceAcrossReopen(t *testing.T) {
	ctx := context.Background()
	databaseURL := filepath.Join(t.TempDir(), "goal-continuation-started.sqlite")
	db := openGoalContinuationTestDB(t, databaseURL)
	applyGoalMigration(t, db)
	repository := NewRepository(config.Config{DatabaseDriver: "sqlite"}, db)
	now := time.Date(2026, 8, 14, 8, 30, 0, 0, time.UTC)
	created, err := repository.CreateGoal(ctx, protocol.Goal{
		ID: "goal-started-fence", SessionKey: "agent:nexus:ws:dm:started-fence",
		Objective: "keep one plan", Status: protocol.GoalStatusActive,
		Version: 1, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	created.ContinuationCount, created.Version = 1, 2
	next := now
	plan := protocol.GoalContinuationPlan{
		RoundID: "round-started-fence", GoalID: created.ID, SessionKey: created.SessionKey,
		ObjectiveRevision: 1, Prompt: "recover me", Purpose: "goal_continuation",
		Status: protocol.GoalContinuationPlanStatusScheduled, Version: 1,
		NextAttemptAt: &next, CreatedAt: now, UpdatedAt: now,
	}
	event := protocol.GoalEvent{ID: "event-started-fence", GoalID: created.ID, SessionKey: created.SessionKey, EventType: "continuation_scheduled", Source: protocol.GoalUpdateSourceSystem, RoundID: plan.RoundID, CreatedAt: now}
	if _, err = repository.ReserveGoalContinuation(ctx, *created, 1, event, plan); err != nil {
		t.Fatal(err)
	}
	if _, err = repository.ClaimGoalContinuation(ctx, plan.RoundID, now, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err = repository.MarkGoalContinuationStarted(ctx, plan.RoundID, now.Add(time.Second), now.Add(15*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}

	db = openGoalContinuationTestDB(t, databaseURL)
	repository = NewRepository(config.Config{DatabaseDriver: "sqlite"}, db)
	recovered, err := repository.GetOpenGoalContinuation(ctx, created.ID, 1)
	if err != nil || recovered == nil || recovered.Status != protocol.GoalContinuationPlanStatusStarted || recovered.RoundID != plan.RoundID {
		t.Fatalf("started recovery after reopen = %#v, %v", recovered, err)
	}
	created, err = repository.GetGoal(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	created.ContinuationCount++
	created.Version++
	created.UpdatedAt = now.Add(2 * time.Second)
	second := plan
	second.RoundID = "round-duplicate-started-fence"
	second.CreatedAt, second.UpdatedAt = created.UpdatedAt, created.UpdatedAt
	second.NextAttemptAt = &created.UpdatedAt
	secondEvent := event
	secondEvent.ID, secondEvent.RoundID, secondEvent.CreatedAt = "event-duplicate-started-fence", second.RoundID, created.UpdatedAt
	if _, err = repository.ReserveGoalContinuation(ctx, *created, 2, secondEvent, second); err == nil {
		t.Fatal("reserve while started receipt owns revision = nil, want unique open-plan failure")
	}
	current, err := repository.GetGoal(ctx, created.ID)
	if err != nil || current.ContinuationCount != 1 || current.Version != 2 {
		t.Fatalf("Goal after started-fence rollback = %#v, %v", current, err)
	}
}

func TestRepositoryGoalContinuationLifecycleTransitionCancelsOpenReceipt(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 14, 10, 30, 0, 0, time.UTC)
	created, err := repository.CreateGoal(ctx, protocol.Goal{
		ID: "goal-terminal", SessionKey: "agent:nexus:ws:dm:terminal", Objective: "stop",
		Status: protocol.GoalStatusActive, Version: 1, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	created.ContinuationCount, created.Version = 1, 2
	next := now
	plan := protocol.GoalContinuationPlan{
		RoundID: "round-terminal", GoalID: created.ID, SessionKey: created.SessionKey,
		ObjectiveRevision: 1, Prompt: "must not start", Purpose: "goal_continuation",
		Status: protocol.GoalContinuationPlanStatusScheduled, Version: 1,
		NextAttemptAt: &next, CreatedAt: now, UpdatedAt: now,
	}
	event := protocol.GoalEvent{ID: "event-terminal-scheduled", GoalID: created.ID, SessionKey: created.SessionKey, EventType: "continuation_scheduled", Source: protocol.GoalUpdateSourceSystem, RoundID: plan.RoundID, CreatedAt: now}
	created, err = repository.ReserveGoalContinuation(ctx, *created, 1, event, plan)
	if err != nil {
		t.Fatal(err)
	}
	created.Status = protocol.GoalStatusBlocked
	created.Version++
	created.UpdatedAt = now.Add(time.Minute)
	blockedEvent := protocol.GoalEvent{ID: "event-terminal-blocked", GoalID: created.ID, SessionKey: created.SessionKey, EventType: "blocked", Source: protocol.GoalUpdateSourceModel, CreatedAt: created.UpdatedAt}
	if _, err = repository.UpdateGoalWithEvents(ctx, *created, 2, []protocol.GoalEvent{blockedEvent}); err != nil {
		t.Fatal(err)
	}
	if open, err := repository.GetOpenGoalContinuation(ctx, created.ID, 1); err != nil || open != nil {
		t.Fatalf("open after blocked = %#v, %v", open, err)
	}
}

func TestRepositoryGoalContinuationScheduleIsAtomicWhenPlanInsertFails(t *testing.T) {
	repository := newTestRepository(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 14, 13, 0, 0, 0, time.UTC)
	created, err := repository.CreateGoal(ctx, protocol.Goal{
		ID: "goal-atomic-plan", SessionKey: "agent:nexus:ws:dm:atomic-plan", Objective: "atomic",
		Status: protocol.GoalStatusActive, Version: 1, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	created.ContinuationCount, created.Version = 1, 2
	next := now
	plan := protocol.GoalContinuationPlan{
		RoundID: "round-atomic", GoalID: created.ID, SessionKey: created.SessionKey,
		ObjectiveRevision: 1, Prompt: "prompt", Purpose: "goal_continuation",
		Status: protocol.GoalContinuationPlanStatusScheduled, Version: 1,
		NextAttemptAt: &next, CreatedAt: now, UpdatedAt: now,
	}
	event := protocol.GoalEvent{ID: "event-atomic", GoalID: created.ID, SessionKey: created.SessionKey, EventType: "continuation_scheduled", Source: protocol.GoalUpdateSourceSystem, RoundID: plan.RoundID, CreatedAt: now}
	if _, err = repository.ReserveGoalContinuation(ctx, *created, 1, event, plan); err != nil {
		t.Fatal(err)
	}
	created.Version, created.ContinuationCount = 3, 2
	created.UpdatedAt = now.Add(time.Second)
	secondEvent := event
	secondEvent.ID = "event-atomic-second"
	secondEvent.RoundID = "round-atomic-second"
	secondPlan := plan
	secondPlan.RoundID = secondEvent.RoundID
	secondPlan.UpdatedAt = created.UpdatedAt
	if _, err = repository.ReserveGoalContinuation(ctx, *created, 2, secondEvent, secondPlan); err == nil {
		t.Fatal("second open plan insert error = nil, want unique open receipt failure")
	}
	current, err := repository.GetGoal(ctx, created.ID)
	if err != nil || current.Version != 2 || current.ContinuationCount != 1 {
		t.Fatalf("Goal after rolled-back plan insert = %#v, %v", current, err)
	}
	events, err := repository.ListEvents(ctx, created.ID, 10)
	if err != nil || len(events) != 1 || events[0].ID != event.ID {
		t.Fatalf("events after rolled-back plan insert = %#v, %v", events, err)
	}
}

func openGoalContinuationTestDB(t *testing.T, databaseURL string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	return db
}
