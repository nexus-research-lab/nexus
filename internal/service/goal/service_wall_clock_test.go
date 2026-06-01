package goal

import (
	"context"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestServiceExternalMutationAccountsWallClockWithoutRunningRound(t *testing.T) {
	repo := newMemoryRepository()
	clock := newMutableClock(time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC))
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = clock.Now
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:wall-clock-update",
		Objective:  "Account before external mutation",
	})
	if err != nil {
		t.Fatal(err)
	}

	clock.Advance(9 * time.Second)
	objective := "Account before external mutation, updated"
	updated, err := service.Update(ctx, created.ID, protocol.UpdateGoalRequest{Objective: &objective})
	if err != nil {
		t.Fatal(err)
	}
	if updated.TimeUsedSeconds != 9 {
		t.Fatalf("updated.TimeUsedSeconds = %d, want 9", updated.TimeUsedSeconds)
	}
}

func TestServiceExternalMutationDoesNotDoubleCountWallClockAfterRuntimeFlush(t *testing.T) {
	repo := newMemoryRepository()
	clock := newMutableClock(time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC))
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = clock.Now
	service.idFactory = sequentialID()
	service.SetExternalMutationAccountant(&fakeExternalMutationAccountant{roundID: "round-running"})
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:wall-clock-flush",
		Objective:  "Do not double count running round",
	})
	if err != nil {
		t.Fatal(err)
	}

	clock.Advance(9 * time.Second)
	objective := "Do not double count running round, updated"
	updated, err := service.Update(ctx, created.ID, protocol.UpdateGoalRequest{Objective: &objective})
	if err != nil {
		t.Fatal(err)
	}
	if updated.TimeUsedSeconds != 0 {
		t.Fatalf("updated.TimeUsedSeconds = %d, want no wall-clock fallback after runtime flush", updated.TimeUsedSeconds)
	}
}

func TestServiceRuntimeUsageAdvancesWallClockBaseline(t *testing.T) {
	repo := newMemoryRepository()
	clock := newMutableClock(time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC))
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = clock.Now
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:wall-clock-runtime-usage",
		Objective:  "Avoid double-counting runtime usage",
	})
	if err != nil {
		t.Fatal(err)
	}

	clock.Advance(5 * time.Second)
	if _, err := service.RecordUsageForSession(ctx, created.SessionKey, protocol.GoalUsage{TotalTokens: 7, RuntimeSeconds: 5}, "round-1"); err != nil {
		t.Fatal(err)
	}

	clock.Advance(3 * time.Second)
	objective := "Avoid double-counting runtime usage, updated"
	updated, err := service.Update(ctx, created.ID, protocol.UpdateGoalRequest{Objective: &objective})
	if err != nil {
		t.Fatal(err)
	}
	if updated.TimeUsedSeconds != 8 {
		t.Fatalf("updated.TimeUsedSeconds = %d, want 8 without double-counting first 5 seconds", updated.TimeUsedSeconds)
	}
}

func TestServiceWallClockAccountingResetsAcrossPauseAndResume(t *testing.T) {
	repo := newMemoryRepository()
	clock := newMutableClock(time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC))
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = clock.Now
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:wall-clock-resume",
		Objective:  "Reset paused wall clock",
	})
	if err != nil {
		t.Fatal(err)
	}

	clock.Advance(5 * time.Second)
	paused, err := service.Pause(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if paused.TimeUsedSeconds != 5 {
		t.Fatalf("paused.TimeUsedSeconds = %d, want 5", paused.TimeUsedSeconds)
	}

	clock.Advance(20 * time.Second)
	resumed, err := service.Resume(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.TimeUsedSeconds != 5 {
		t.Fatalf("resumed.TimeUsedSeconds = %d, want paused time to stay 5", resumed.TimeUsedSeconds)
	}

	clock.Advance(3 * time.Second)
	_, goal, err := service.RuntimeContext(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if goal == nil || goal.TimeUsedSeconds != 8 {
		t.Fatalf("runtime goal = %#v, want 8s after resumed active time", goal)
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
	if len(accountant.clearedSessionKeys) != 1 || accountant.clearedSessionKeys[0] != created.SessionKey {
		t.Fatalf("accountant clearedSessionKeys = %#v, want current session", accountant.clearedSessionKeys)
	}
	if len(repo.events) != 3 || repo.events[1].EventType != "usage_recorded" || repo.events[2].EventType != "paused" {
		t.Fatalf("events = %#v, want usage_recorded before paused", repo.events)
	}
}
