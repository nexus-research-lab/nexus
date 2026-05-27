package goal

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestServiceCreateAndCurrentGoal(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()

	created, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Ship goal mode",
		CreatedBy:  "user",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID != "goal_1" || created.Status != protocol.GoalStatusActive {
		t.Fatalf("created = %#v, want active goal_1", created)
	}
	if created.TokenBudget != nil {
		t.Fatalf("TokenBudget = %#v, want nil when omitted", created.TokenBudget)
	}

	current, err := service.Current(context.Background(), "agent:nexus:ws:dm:chat")
	if err != nil {
		t.Fatal(err)
	}
	if current.ID != created.ID {
		t.Fatalf("Current ID = %q, want %q", current.ID, created.ID)
	}
	if _, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Second",
	}); !errors.Is(err, ErrGoalConflict) {
		t.Fatalf("duplicate create error = %v, want ErrGoalConflict", err)
	}
}

func TestServiceCreateGoalEventSourceFollowsCreator(t *testing.T) {
	for _, tc := range []struct {
		name      string
		createdBy string
		roundID   string
		want      protocol.GoalUpdateSource
	}{
		{name: "user default", createdBy: "", want: protocol.GoalUpdateSourceUser},
		{name: "model tool", createdBy: "model", roundID: "round-model", want: protocol.GoalUpdateSourceModel},
		{name: "app server", createdBy: "app_server", want: protocol.GoalUpdateSourceExternal},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newMemoryRepository()
			service := NewService(config.Config{GoalEnabled: true}, repo)
			service.nowFn = fixedClock()
			service.idFactory = sequentialID()

			if _, err := service.Create(context.Background(), protocol.CreateGoalRequest{
				SessionKey: "agent:nexus:ws:dm:" + strings.ReplaceAll(tc.name, " ", "-"),
				Objective:  "Ship goal mode",
				CreatedBy:  tc.createdBy,
				RoundID:    tc.roundID,
			}); err != nil {
				t.Fatal(err)
			}
			if len(repo.events) != 1 || repo.events[0].Source != tc.want {
				t.Fatalf("events = %#v, want source %q", repo.events, tc.want)
			}
			if repo.events[0].RoundID != tc.roundID {
				t.Fatalf("event round_id = %q, want %q", repo.events[0].RoundID, tc.roundID)
			}
		})
	}
}

func TestServiceCreateFillsEmptyPreviewFromGoal(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	preview := &fakePreviewFiller{}
	service.SetPreviewFiller(preview)

	created, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Ship goal mode",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.items) != 1 || preview.items[0].sessionKey != created.SessionKey || preview.items[0].title != created.Objective {
		t.Fatalf("preview items = %#v, want created goal objective", preview.items)
	}
}

func TestServiceCurrentOptionalAllowsMissingGoal(t *testing.T) {
	service := NewService(config.Config{GoalEnabled: true}, newMemoryRepository())

	current, err := service.CurrentOptional(context.Background(), "agent:nexus:ws:dm:chat")
	if err != nil {
		t.Fatal(err)
	}
	if current != nil {
		t.Fatalf("CurrentOptional() = %#v, want nil", current)
	}
	if _, err := service.Current(context.Background(), "agent:nexus:ws:dm:chat"); !errors.Is(err, ErrGoalNotFound) {
		t.Fatalf("Current() error = %v, want ErrGoalNotFound", err)
	}
}

func TestServiceBroadcastsGoalEvents(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	broadcaster := &fakeGoalBroadcaster{}
	service.SetEventBroadcaster(broadcaster)

	created, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Broadcast status",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(broadcaster.events) != 1 || broadcaster.events[0].EventType != protocol.EventTypeGoalCreated {
		t.Fatalf("events = %#v, want goal_created", broadcaster.events)
	}

	if _, err := service.Pause(context.Background(), created.ID); err != nil {
		t.Fatal(err)
	}
	if len(broadcaster.events) != 2 || broadcaster.events[1].EventType != protocol.EventTypeGoalStatusChanged {
		t.Fatalf("events = %#v, want goal_status_changed", broadcaster.events)
	}
	if broadcaster.events[1].Data["goal_event_type"] != "paused" {
		t.Fatalf("payload = %#v, want paused goal_event_type", broadcaster.events[1].Data)
	}

	if _, err := service.Clear(context.Background(), created.ID); err != nil {
		t.Fatal(err)
	}
	if len(broadcaster.events) != 3 || broadcaster.events[2].EventType != protocol.EventTypeGoalCleared {
		t.Fatalf("events = %#v, want goal_cleared", broadcaster.events)
	}
	if broadcaster.events[2].Data["goal_event_type"] != "cleared" {
		t.Fatalf("payload = %#v, want cleared goal_event_type", broadcaster.events[2].Data)
	}
}

func TestServiceBroadcastsContinuationSuppressedEvent(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	broadcaster := &fakeGoalBroadcaster{}
	service.SetEventBroadcaster(broadcaster)
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Broadcast suppression",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.RecordContinuationProgress(ctx, created.ID, "goal_continuation_1", false); err != nil {
		t.Fatal(err)
	}
	if len(broadcaster.events) != 2 || broadcaster.events[1].EventType != protocol.EventTypeGoalContinuation {
		t.Fatalf("events = %#v, want goal_continuation for suppressed continuation", broadcaster.events)
	}
	if broadcaster.events[1].Data["goal_event_type"] != "continuation_suppressed" {
		t.Fatalf("payload = %#v, want continuation_suppressed goal_event_type", broadcaster.events[1].Data)
	}
}

func TestServiceStateTransitions(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Long task",
	})
	if err != nil {
		t.Fatal(err)
	}
	paused, err := service.Pause(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if paused.Status != protocol.GoalStatusPaused {
		t.Fatalf("paused status = %q, want paused", paused.Status)
	}
	if _, err := service.CompleteByModel(ctx, created.ID, protocol.CompleteGoalRequest{}); !errors.Is(err, ErrGoalInvalidState) {
		t.Fatalf("model complete paused error = %v, want ErrGoalInvalidState", err)
	}
	resumed, err := service.Resume(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	completed, err := service.CompleteByModel(ctx, resumed.ID, protocol.CompleteGoalRequest{Summary: "done", RoundID: "round-1"})
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != protocol.GoalStatusComplete || completed.CompletedAt == nil {
		t.Fatalf("completed = %#v, want terminal complete", completed)
	}
	if _, err := service.Resume(ctx, completed.ID); !errors.Is(err, ErrGoalInvalidState) {
		t.Fatalf("resume complete error = %v, want ErrGoalInvalidState", err)
	}
	current, err := service.Current(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current.Status != protocol.GoalStatusComplete {
		t.Fatalf("current = %#v, want complete goal still visible", current)
	}
	cleared, err := service.Clear(ctx, completed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !cleared {
		t.Fatal("Clear() = false, want true")
	}
	current, err = service.CurrentOptional(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current != nil {
		t.Fatalf("current after clear = %#v, want nil", current)
	}
	deleted, err := repo.GetGoal(ctx, completed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != nil {
		t.Fatalf("deleted = %#v, want nil after clear", deleted)
	}
}

func TestServiceEditCompletedGoalReactivatesIt(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Finish first objective",
	})
	if err != nil {
		t.Fatal(err)
	}
	completed, err := service.CompleteByModel(ctx, created.ID, protocol.CompleteGoalRequest{})
	if err != nil {
		t.Fatal(err)
	}
	updatedObjective := "Continue with revised objective"
	edited, err := service.Update(ctx, completed.ID, protocol.UpdateGoalRequest{
		Objective: &updatedObjective,
	})
	if err != nil {
		t.Fatal(err)
	}
	if edited.Status != protocol.GoalStatusActive || edited.Objective != updatedObjective || edited.CompletedAt != nil {
		t.Fatalf("edited = %#v, want active revised goal without completed_at", edited)
	}
}

func TestServiceUpdateObjectiveFillsEmptyPreviewFromGoal(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	preview := &fakePreviewFiller{}
	service.SetPreviewFiller(preview)
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Initial goal objective",
	})
	if err != nil {
		t.Fatal(err)
	}
	preview.items = nil

	updatedObjective := "Revised goal objective"
	updated, err := service.Update(ctx, created.ID, protocol.UpdateGoalRequest{
		Objective: &updatedObjective,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.items) != 1 || preview.items[0].sessionKey != updated.SessionKey || preview.items[0].title != updated.Objective {
		t.Fatalf("preview items = %#v, want updated goal objective", preview.items)
	}
}

func TestServiceModelStatusUpdateFlushesButDoesNotClearRuntimeAccountingEarly(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Complete from tool",
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

	completed, err := service.CompleteByModel(ctx, created.ID, protocol.CompleteGoalRequest{RoundID: "round-running"})
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != protocol.GoalStatusComplete {
		t.Fatalf("completed status = %q, want complete", completed.Status)
	}
	if len(accountant.sessionKeys) != 1 || accountant.sessionKeys[0] != created.SessionKey {
		t.Fatalf("accountant flush=%#v, want one best-effort flush for model update", accountant.sessionKeys)
	}
	if len(accountant.clearedSessionKeys) != 0 {
		t.Fatalf("accountant clear=%#v, want no early clear for model update", accountant.clearedSessionKeys)
	}
}

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

func TestServiceSetFromThreadGoalParamsCreatesAndUpdatesGoal(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()
	threadID := "agent:nexus:ws:dm:chat"
	objective := "Ship app-server parity"
	paused := protocol.ThreadGoalStatusPaused
	budget := int64(120)

	created, err := service.SetFromThreadGoalParams(ctx, protocol.ThreadGoalSetParams{
		ThreadID:    threadID,
		Objective:   &objective,
		Status:      &paused,
		TokenBudget: optionalBudget(budget),
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.SessionKey != threadID || created.Status != protocol.GoalStatusPaused || created.TokenBudget == nil || *created.TokenBudget != budget {
		t.Fatalf("created = %#v, want paused app-server goal with budget", created)
	}

	usageLimited := protocol.ThreadGoalStatusUsageLimited
	updated, err := service.SetFromThreadGoalParams(ctx, protocol.ThreadGoalSetParams{
		ThreadID: threadID,
		Status:   &usageLimited,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != created.ID || updated.Status != protocol.GoalStatusUsageLimited {
		t.Fatalf("updated = %#v, want same goal usage_limited", updated)
	}
}

func TestServiceSetFromThreadGoalParamsCreatesFinalStatusDirectly(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	broadcaster := &fakeGoalBroadcaster{}
	service.SetEventBroadcaster(broadcaster)
	ctx := context.Background()
	threadID := "agent:nexus:ws:dm:chat"
	objective := "Create paused without active flicker"
	paused := protocol.ThreadGoalStatusPaused

	created, err := service.SetFromThreadGoalParams(ctx, protocol.ThreadGoalSetParams{
		ThreadID:  threadID,
		Objective: &objective,
		Status:    &paused,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Status != protocol.GoalStatusPaused {
		t.Fatalf("created status = %q, want paused", created.Status)
	}
	if len(repo.events) != 1 || repo.events[0].EventType != "created" {
		t.Fatalf("events = %#v, want a single created event", repo.events)
	}
	if len(broadcaster.events) != 1 || broadcaster.events[0].EventType != protocol.EventTypeGoalCreated {
		t.Fatalf("broadcast events = %#v, want one final created event", broadcaster.events)
	}
	goal, _ := broadcaster.events[0].Data["goal"].(protocol.Goal)
	if goal.Status != protocol.GoalStatusPaused {
		t.Fatalf("broadcast goal = %#v, want paused final status", broadcaster.events[0].Data["goal"])
	}
}

func TestServiceSetFromThreadGoalParamsRequiresObjectiveWhenMissing(t *testing.T) {
	service := NewService(config.Config{GoalEnabled: true}, newMemoryRepository())
	active := protocol.ThreadGoalStatusActive

	_, err := service.SetFromThreadGoalParams(context.Background(), protocol.ThreadGoalSetParams{
		ThreadID: "agent:nexus:ws:dm:missing",
		Status:   &active,
	})
	if !errors.Is(err, ErrGoalNotFound) {
		t.Fatalf("SetFromThreadGoalParams() error = %v, want ErrGoalNotFound", err)
	}
	want := "cannot update goal for thread agent:nexus:ws:dm:missing: no goal exists"
	if err.Error() != want {
		t.Fatalf("SetFromThreadGoalParams() error text = %q, want %q", err.Error(), want)
	}
}

func TestServiceSetFromThreadGoalParamsPreservesBudgetLimitedGoal(t *testing.T) {
	service := NewService(config.Config{GoalEnabled: true}, newMemoryRepository())
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()
	threadID := "agent:nexus:ws:dm:chat"
	objective := "Keep polishing"
	budget := int64(10)
	budgetLimited := protocol.ThreadGoalStatusBudgetLimited

	created, err := service.SetFromThreadGoalParams(ctx, protocol.ThreadGoalSetParams{
		ThreadID:    threadID,
		Objective:   &objective,
		Status:      &budgetLimited,
		TokenBudget: optionalBudget(budget),
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Status != protocol.GoalStatusBudgetLimited {
		t.Fatalf("created status = %q, want budget_limited", created.Status)
	}

	for _, status := range []protocol.ThreadGoalStatus{
		protocol.ThreadGoalStatusPaused,
		protocol.ThreadGoalStatusBlocked,
	} {
		updated, err := service.SetFromThreadGoalParams(ctx, protocol.ThreadGoalSetParams{
			ThreadID: threadID,
			Status:   &status,
		})
		if err != nil {
			t.Fatal(err)
		}
		if updated.Status != protocol.GoalStatusBudgetLimited {
			t.Fatalf("status %q updated goal to %q, want budget_limited", status, updated.Status)
		}
	}
}

func TestServicePauseAndModelBlockPreserveBudgetLimitedGoal(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(context.Context, *Service, protocol.Goal) (*protocol.Goal, error)
	}{
		{
			name: "user pause",
			run: func(ctx context.Context, service *Service, item protocol.Goal) (*protocol.Goal, error) {
				return service.Pause(ctx, item.ID)
			},
		},
		{
			name: "model block",
			run: func(ctx context.Context, service *Service, item protocol.Goal) (*protocol.Goal, error) {
				return service.BlockByModel(ctx, item.ID, protocol.BlockGoalRequest{RoundID: "round-blocked"})
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := newMemoryRepository()
			service := NewService(config.Config{GoalEnabled: true}, repo)
			service.nowFn = fixedClock()
			service.idFactory = sequentialID()
			ctx := context.Background()
			budget := int64(10)

			created, err := service.Create(ctx, protocol.CreateGoalRequest{
				SessionKey:  "agent:nexus:ws:dm:" + strings.ReplaceAll(tc.name, " ", "-"),
				Objective:   "Preserve budget limit",
				TokenBudget: &budget,
			})
			if err != nil {
				t.Fatal(err)
			}
			limited, err := service.RecordUsageForSession(ctx, created.SessionKey, protocol.GoalUsage{TotalTokens: 10}, "round-budget")
			if err != nil {
				t.Fatal(err)
			}
			if limited.Status != protocol.GoalStatusBudgetLimited {
				t.Fatalf("limited status = %q, want budget_limited", limited.Status)
			}

			updated, err := tc.run(ctx, service, *limited)
			if err != nil {
				t.Fatal(err)
			}
			if updated.Status != protocol.GoalStatusBudgetLimited {
				t.Fatalf("updated status = %q, want budget_limited", updated.Status)
			}
			for _, event := range repo.events {
				if event.EventType == "paused" || event.EventType == "blocked" {
					t.Fatalf("events = %#v, want no paused/blocked event after budget_limited", repo.events)
				}
			}
		})
	}
}

func TestServiceClearFromThreadGoalParamsDeletesCurrentGoal(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()
	threadID := "agent:nexus:ws:dm:chat"
	objective := "Clear through app-server"
	accountant := &fakeExternalMutationAccountant{}
	service.SetExternalMutationAccountant(accountant)

	created, err := service.SetFromThreadGoalParams(ctx, protocol.ThreadGoalSetParams{
		ThreadID:  threadID,
		Objective: &objective,
	})
	if err != nil {
		t.Fatal(err)
	}

	cleared, err := service.ClearFromThreadGoalParams(ctx, protocol.ThreadGoalClearParams{ThreadID: threadID})
	if err != nil {
		t.Fatal(err)
	}
	if !cleared {
		t.Fatal("ClearFromThreadGoalParams() cleared = false, want true")
	}
	current, err := service.CurrentOptional(ctx, threadID)
	if err != nil {
		t.Fatal(err)
	}
	if current != nil {
		t.Fatalf("current = %#v, want nil after clear", current)
	}
	deleted, err := repo.GetGoal(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != nil {
		t.Fatalf("deleted = %#v, want hard-deleted app-server goal", deleted)
	}
	if len(accountant.clearedSessionKeys) != 1 || accountant.clearedSessionKeys[0] != threadID {
		t.Fatalf("clearedSessionKeys = %#v, want app-server clear to stop runtime accounting", accountant.clearedSessionKeys)
	}

	cleared, err = service.ClearFromThreadGoalParams(ctx, protocol.ThreadGoalClearParams{ThreadID: threadID})
	if err != nil {
		t.Fatal(err)
	}
	if cleared {
		t.Fatal("second ClearFromThreadGoalParams() cleared = true, want false")
	}
}

func TestServiceBlockByModelAllowsEmptyReason(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Wait for external input",
	})
	if err != nil {
		t.Fatal(err)
	}
	blocked, err := service.BlockByModel(ctx, created.ID, protocol.BlockGoalRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if blocked.Status != protocol.GoalStatusBlocked || blocked.BlockedAt == nil {
		t.Fatalf("blocked = %#v, want blocked status", blocked)
	}
	if len(repo.events) != 2 || repo.events[1].EventType != "blocked" {
		t.Fatalf("events = %#v, want blocked event", repo.events)
	}
	if _, ok := repo.events[1].Payload["reason"]; ok {
		t.Fatalf("blocked payload = %#v, want no synthetic reason", repo.events[1].Payload)
	}
}

func TestServiceRecordUsageForCompletedGoal(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Complete with final usage",
	})
	if err != nil {
		t.Fatal(err)
	}
	completed, err := service.CompleteByModel(ctx, created.ID, protocol.CompleteGoalRequest{})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := service.RecordUsageForGoal(ctx, completed.ID, protocol.GoalUsage{
		TotalTokens:    12,
		RuntimeSeconds: 5,
	}, "round-1")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != protocol.GoalStatusComplete || updated.Usage.Total() != 12 || updated.TimeUsedSeconds != 5 {
		t.Fatalf("updated = %#v, want completed goal with final usage", updated)
	}
	if len(repo.events) != 3 || repo.events[2].EventType != "usage_recorded" || repo.events[2].RoundID != "round-1" {
		t.Fatalf("events = %#v, want usage_recorded after completion", repo.events)
	}
}

func TestServiceRejectsOversizedObjective(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()
	oversized := strings.Repeat("x", maxGoalObjectiveRunes+1)

	_, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "   ",
	})
	assertGoalInvalidInputMessage(t, err, goalObjectiveEmptyMessage)

	_, err = service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  oversized,
	})
	assertGoalInvalidInputMessage(t, err, goalObjectiveTooLongMessage)

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Valid goal",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Update(ctx, created.ID, protocol.UpdateGoalRequest{
		Objective: &oversized,
	}); err != nil {
		assertGoalInvalidInputMessage(t, err, goalObjectiveTooLongMessage)
	} else {
		t.Fatal("Update oversized objective error = nil, want ErrGoalInvalidInput")
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

func TestServiceRecordUsageUsesGoalBudgetTokenAccounting(t *testing.T) {
	repo := newMemoryRepository()
	budget := int64(50)
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey:  "agent:nexus:ws:dm:chat",
		Objective:   "Budget accounting",
		TokenBudget: &budget,
	})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := service.RecordUsageForSession(ctx, created.SessionKey, protocol.GoalUsage{
		InputTokens:              10,
		OutputTokens:             20,
		CacheCreationInputTokens: 80,
		CacheReadInputTokens:     90,
		ReasoningTokens:          40,
		TotalTokens:              240,
	}, "round-1")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Status != protocol.GoalStatusActive {
		t.Fatalf("status = %q, want active", updated.Status)
	}
	if updated.Usage.TotalTokens != 20 || updated.Usage.Total() != 20 {
		t.Fatalf("usage = %#v, want budget total 20", updated.Usage)
	}
	remaining := updated.RemainingTokens()
	if remaining == nil || *remaining != 30 {
		t.Fatalf("RemainingTokens() = %#v, want 30", remaining)
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
	if len(accountant.sessionKeys) != 1 || accountant.sessionKeys[0] != created.SessionKey {
		t.Fatalf("accountant sessionKeys = %#v, want current session", accountant.sessionKeys)
	}
	if len(repo.events) != 3 ||
		repo.events[1].EventType != "usage_recorded" ||
		repo.events[2].EventType != "budget_limited" {
		t.Fatalf("events = %#v, want usage_recorded then budget_limited only", repo.events)
	}
}

func TestServiceSetFromThreadGoalParamsActivatesAccountingWhenGoalBecomesActive(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()
	threadID := "agent:nexus:ws:dm:chat"
	objective := "Resume external accounting"
	paused := protocol.ThreadGoalStatusPaused

	created, err := service.SetFromThreadGoalParams(ctx, protocol.ThreadGoalSetParams{
		ThreadID:  threadID,
		Objective: &objective,
		Status:    &paused,
	})
	if err != nil {
		t.Fatal(err)
	}
	accountant := &fakeExternalMutationAccountant{roundID: "round-running"}
	service.SetExternalMutationAccountant(accountant)
	active := protocol.ThreadGoalStatusActive

	updated, err := service.SetFromThreadGoalParams(ctx, protocol.ThreadGoalSetParams{
		ThreadID: threadID,
		Status:   &active,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != created.ID || updated.Status != protocol.GoalStatusActive {
		t.Fatalf("updated = %#v, want same active goal", updated)
	}
	if len(accountant.sessionKeys) != 1 || accountant.sessionKeys[0] != threadID {
		t.Fatalf("flush sessionKeys = %#v, want current session", accountant.sessionKeys)
	}
	if len(accountant.activatedSessionKeys) != 1 || accountant.activatedSessionKeys[0] != threadID {
		t.Fatalf("activated sessionKeys = %#v, want current session", accountant.activatedSessionKeys)
	}
	if len(accountant.clearedSessionKeys) != 0 {
		t.Fatalf("cleared sessionKeys = %#v, want no clear for active goal", accountant.clearedSessionKeys)
	}
}

func TestServiceAllowsGoalCompletionAfterExternalFlushHitsBudget(t *testing.T) {
	repo := newMemoryRepository()
	budget := int64(5)
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
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
}

func TestServiceUsageLimitForSessionTransitionsActiveAndBudgetLimitedGoal(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
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

func TestServiceUpdateBudgetClearResumesLimitedGoal(t *testing.T) {
	repo := newMemoryRepository()
	initialBudget := int64(10)
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey:  "agent:nexus:ws:dm:chat",
		Objective:   "Clear budget",
		TokenBudget: &initialBudget,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.RecordUsageForSession(ctx, created.SessionKey, protocol.GoalUsage{TotalTokens: 10}, "round-1"); err != nil {
		t.Fatal(err)
	}
	resumed, err := service.Update(ctx, created.ID, protocol.UpdateGoalRequest{TokenBudget: clearBudget()})
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Status != protocol.GoalStatusActive || resumed.TokenBudget != nil {
		t.Fatalf("resumed = %#v, want active with cleared budget", resumed)
	}
}

func TestServiceUpdateBudgetLimitsActiveGoal(t *testing.T) {
	repo := newMemoryRepository()
	initialBudget := int64(100)
	loweredBudget := int64(25)
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey:  "agent:nexus:ws:dm:chat",
		Objective:   "Lower budget",
		TokenBudget: &initialBudget,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.RecordUsageForSession(ctx, created.SessionKey, protocol.GoalUsage{TotalTokens: 30}, "round-1"); err != nil {
		t.Fatal(err)
	}
	limited, err := service.Update(ctx, created.ID, protocol.UpdateGoalRequest{TokenBudget: optionalBudget(loweredBudget)})
	if err != nil {
		t.Fatal(err)
	}
	if limited.Status != protocol.GoalStatusBudgetLimited || limited.LastError == "" {
		t.Fatalf("limited = %#v, want budget_limited with error", limited)
	}
}

func TestServiceQueuesObjectiveUpdateSteering(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	dispatcher := &fakeGuidanceDispatcher{}
	service.SetGuidanceDispatcher(dispatcher)
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Original",
	})
	if err != nil {
		t.Fatal(err)
	}
	updatedObjective := "Updated <goal>"
	if _, err := service.Update(ctx, created.ID, protocol.UpdateGoalRequest{Objective: &updatedObjective}); err != nil {
		t.Fatal(err)
	}
	if len(dispatcher.items) != 1 {
		t.Fatalf("guidance = %#v, want one objective update steering item", dispatcher.items)
	}
	item := dispatcher.items[0]
	if item.sessionKey != created.SessionKey ||
		item.contextName != "goal_context" ||
		!strings.Contains(item.content, "active thread goal objective was edited") ||
		!strings.Contains(item.content, "Updated &lt;goal&gt;") ||
		strings.Contains(item.content, "Nexus Goal") {
		t.Fatalf("guidance item = %#v, want escaped objective update steering", item)
	}
}

func TestServiceUpdateSameObjectiveDoesNotQueueObjectiveSteering(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	dispatcher := &fakeGuidanceDispatcher{}
	service.SetGuidanceDispatcher(dispatcher)
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Same objective",
	})
	if err != nil {
		t.Fatal(err)
	}
	sameObjective := "Same objective"
	updated, err := service.Update(ctx, created.ID, protocol.UpdateGoalRequest{Objective: &sameObjective})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Version != created.Version {
		t.Fatalf("updated version = %d, want unchanged %d", updated.Version, created.Version)
	}
	if len(dispatcher.items) != 0 {
		t.Fatalf("guidance = %#v, want no objective update steering", dispatcher.items)
	}
	if len(repo.events) != 1 {
		t.Fatalf("events = %#v, want no update event for unchanged objective", repo.events)
	}
}

func TestServiceQueuesBudgetLimitSteering(t *testing.T) {
	repo := newMemoryRepository()
	budget := int64(10)
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	dispatcher := &fakeGuidanceDispatcher{}
	service.SetGuidanceDispatcher(dispatcher)
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey:  "agent:nexus:ws:dm:chat",
		Objective:   "Budget <work>",
		TokenBudget: &budget,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.RecordUsageForSession(ctx, created.SessionKey, protocol.GoalUsage{TotalTokens: 10}, "round-1"); err != nil {
		t.Fatal(err)
	}
	if len(dispatcher.items) != 1 {
		t.Fatalf("guidance = %#v, want one budget limit steering item", dispatcher.items)
	}
	if item := dispatcher.items[0]; item.sessionKey != created.SessionKey ||
		item.contextName != "goal_context" ||
		!strings.Contains(item.content, "active thread goal has reached its token budget") ||
		!strings.Contains(item.content, "<objective>") ||
		!strings.Contains(item.content, "</objective>") ||
		!strings.Contains(item.content, "budget_limited") ||
		!strings.Contains(item.content, "Budget &lt;work&gt;") ||
		strings.Contains(item.content, "<untrusted_objective>") ||
		strings.Contains(item.content, "Budget <work>") ||
		strings.Contains(item.content, "Nexus Goal") {
		t.Fatalf("guidance item = %#v, want budget limit steering", item)
	}
}

func TestServicePlanContinuationForSession(t *testing.T) {
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
		Objective:  "Complete parity",
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := service.PlanContinuationForSession(ctx, created.SessionKey, "round-1")
	if err != nil {
		t.Fatal(err)
	}
	if plan == nil || plan.RoundID != "goal_continuation_3" {
		t.Fatalf("plan = %#v, want hidden continuation round", plan)
	}
	if !plan.HiddenFromUser || !plan.Synthetic || plan.Purpose != goalContinuationPurpose {
		t.Fatalf("plan visibility = %#v, want hidden synthetic goal continuation", plan)
	}
	for _, want := range []string{
		"Continue working toward the active thread goal.",
		"Complete parity",
		"Completion audit:",
		"Blocked audit:",
		"Do not call update_goal unless the goal is complete or the strict blocked audit above is satisfied.",
		"Tokens remaining:",
	} {
		if !strings.Contains(plan.Prompt, want) {
			t.Fatalf("continuation prompt missing %q: %s", want, plan.Prompt)
		}
	}
	for _, forbidden := range []string{"active Nexus Goal", "Nexus runtime:", "PreviousRoundID:"} {
		if strings.Contains(plan.Prompt, forbidden) {
			t.Fatalf("continuation prompt contains legacy runtime wording %q: %s", forbidden, plan.Prompt)
		}
	}
	current, err := service.Current(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current.ContinuationCount != 1 {
		t.Fatalf("ContinuationCount = %d, want 1", current.ContinuationCount)
	}
	if len(repo.events) != 2 || repo.events[1].EventType != "continuation_scheduled" || repo.events[1].RoundID != plan.RoundID {
		t.Fatalf("events = %#v, want continuation_scheduled", repo.events)
	}
}

func TestServiceGoalContinuationStillCurrentRejectsStaleGoal(t *testing.T) {
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
		Objective:  "Skip stale continuation",
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := service.PlanContinuationForSession(ctx, created.SessionKey, "round-1")
	if err != nil {
		t.Fatal(err)
	}
	current, err := service.GoalContinuationStillCurrent(ctx, *plan)
	if err != nil {
		t.Fatal(err)
	}
	if !current {
		t.Fatal("GoalContinuationStillCurrent() = false, want true for current active goal")
	}

	stale := repo.goals[created.ID]
	stale.Status = protocol.GoalStatusPaused
	stale.Version++
	repo.goals[created.ID] = stale
	current, err = service.GoalContinuationStillCurrent(ctx, *plan)
	if err != nil {
		t.Fatal(err)
	}
	if current {
		t.Fatal("GoalContinuationStillCurrent() = true, want false after goal is no longer active")
	}
}

func TestServicePlanContinuationStopsWhenBudgetExhausted(t *testing.T) {
	repo := newMemoryRepository()
	budget := int64(10)
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
		TokenBudget: &budget,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.RecordUsageForSession(ctx, created.SessionKey, protocol.GoalUsage{TotalTokens: 10, RuntimeSeconds: 7}, "round-1"); err != nil {
		t.Fatal(err)
	}
	plan, err := service.PlanContinuationForSession(ctx, created.SessionKey, "round-1")
	if err != nil {
		t.Fatal(err)
	}
	if plan != nil {
		t.Fatalf("plan = %#v, want nil after budget exhaustion", plan)
	}
	current, err := service.Current(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current.Status != protocol.GoalStatusBudgetLimited || current.LastError == "" || current.TimeUsedSeconds != 7 {
		t.Fatalf("current = %#v, want budget_limited with last error and runtime", current)
	}
}

func TestServicePlanContinuationStopsAtUsageLimit(t *testing.T) {
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
		Objective:  "Limited work",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.PlanContinuationForSession(ctx, created.SessionKey, "round-1"); err != nil {
		t.Fatal(err)
	}
	plan, err := service.PlanContinuationForSession(ctx, created.SessionKey, "round-2")
	if err != nil {
		t.Fatal(err)
	}
	if plan != nil {
		t.Fatalf("plan = %#v, want nil after usage limit", plan)
	}
	current, err := service.Current(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current.Status != protocol.GoalStatusUsageLimited || current.LastError == "" {
		t.Fatalf("current = %#v, want usage_limited with last error", current)
	}
}

func TestServicePlanContinuationSuppressesAfterEmptyProgress(t *testing.T) {
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
	if next, err := service.PlanContinuationForSession(ctx, created.SessionKey, plan.RoundID); err != nil {
		t.Fatal(err)
	} else if next != nil {
		t.Fatalf("next plan = %#v, want nil after empty continuation progress", next)
	}
	current, err := service.Current(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current.EmptyProgressCount != 1 || current.ContinuationCount != 1 {
		t.Fatalf("current = %#v, want empty progress suppression without extra continuation", current)
	}
	if got := repo.events[len(repo.events)-1]; got.EventType != "continuation_suppressed" || got.RoundID != plan.RoundID {
		t.Fatalf("last event = %#v, want continuation_suppressed for continuation round", got)
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

func TestServiceRunAutoResumeOnceDispatchesActiveGoal(t *testing.T) {
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
		Objective:  "Resume after restart",
	})
	if err != nil {
		t.Fatal(err)
	}
	dispatcher := &fakeContinuationDispatcher{}
	if err := service.RunAutoResumeOnce(ctx, dispatcher); err != nil {
		t.Fatal(err)
	}
	if len(dispatcher.plans) != 1 || dispatcher.plans[0].Goal.ID != created.ID {
		t.Fatalf("plans = %#v, want one resumed goal", dispatcher.plans)
	}
	current, err := service.Current(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current.ContinuationCount != 1 {
		t.Fatalf("ContinuationCount = %d, want 1", current.ContinuationCount)
	}
}

func TestServiceRunAutoResumeOnceSkipsDeferredGoal(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{
		GoalEnabled:             true,
		GoalAutoContinueEnabled: true,
	}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	if _, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Deferred goal",
	}); err != nil {
		t.Fatal(err)
	}
	dispatcher := &fakeContinuationDispatcher{deferSessions: map[string]bool{"agent:nexus:ws:dm:chat": true}}
	if err := service.RunAutoResumeOnce(ctx, dispatcher); err != nil {
		t.Fatal(err)
	}
	if len(dispatcher.plans) != 0 {
		t.Fatalf("plans = %#v, want no dispatch for busy session", dispatcher.plans)
	}
}

func TestServiceRunAutoResumeOnceSkipsStaleContinuationBeforeDispatch(t *testing.T) {
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
		Objective:  "Do not dispatch stale plan",
	})
	if err != nil {
		t.Fatal(err)
	}
	dispatcher := &fakeContinuationDispatcher{
		onShouldDefer: func(call int, _ string) {
			if call != 2 {
				return
			}
			stale := repo.goals[created.ID]
			stale.Status = protocol.GoalStatusPaused
			stale.Version++
			repo.goals[created.ID] = stale
		},
	}
	if err := service.RunAutoResumeOnce(ctx, dispatcher); err != nil {
		t.Fatal(err)
	}
	if len(dispatcher.plans) != 0 {
		t.Fatalf("plans = %#v, want no dispatch after goal changed before launch", dispatcher.plans)
	}
}

func TestServiceSetFromThreadGoalParamsDispatchesActiveGoalImmediately(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{
		GoalEnabled:             true,
		GoalAutoContinueEnabled: true,
	}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	dispatcher := &fakeContinuationDispatcher{}
	service.SetContinuationDispatcher(dispatcher)
	ctx := context.Background()
	threadID := "agent:nexus:ws:dm:chat"
	objective := "Start after app-server set"

	created, err := service.SetFromThreadGoalParams(ctx, protocol.ThreadGoalSetParams{
		ThreadID:  threadID,
		Objective: &objective,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(dispatcher.plans) != 1 || dispatcher.plans[0].Goal.ID != created.ID {
		t.Fatalf("plans = %#v, want immediate continuation for active goal %q", dispatcher.plans, created.ID)
	}
	current, err := service.Current(ctx, threadID)
	if err != nil {
		t.Fatal(err)
	}
	if current.ContinuationCount != 1 {
		t.Fatalf("ContinuationCount = %d, want 1", current.ContinuationCount)
	}
}

func TestServiceSetFromThreadGoalParamsFillsEmptyPreview(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true, GoalAutoContinueEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	preview := &fakePreviewFiller{}
	service.SetPreviewFiller(preview)
	ctx := context.Background()
	objective := "Ship app-server RPC parity"
	status := protocol.ThreadGoalStatusActive

	created, err := service.SetFromThreadGoalParams(ctx, protocol.ThreadGoalSetParams{
		ThreadID:    "room:group:conversation-1",
		Objective:   &objective,
		Status:      &status,
		TokenBudget: protocol.OptionalInt64{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.items) != 1 || preview.items[0].sessionKey != created.SessionKey || preview.items[0].title != created.Objective {
		t.Fatalf("preview items = %#v, want app-server created goal objective", preview.items)
	}
}

func TestServiceSetFromThreadGoalParamsUpdateFillsEmptyPreview(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	preview := &fakePreviewFiller{}
	service.SetPreviewFiller(preview)
	ctx := context.Background()
	threadID := "room:group:conversation-1"
	objective := "Initial app-server goal"

	created, err := service.SetFromThreadGoalParams(ctx, protocol.ThreadGoalSetParams{
		ThreadID:  threadID,
		Objective: &objective,
	})
	if err != nil {
		t.Fatal(err)
	}
	preview.items = nil

	updatedObjective := "Updated app-server goal"
	updated, err := service.SetFromThreadGoalParams(ctx, protocol.ThreadGoalSetParams{
		ThreadID:  threadID,
		Objective: &updatedObjective,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != created.ID || updated.Objective != updatedObjective {
		t.Fatalf("updated = %#v, want existing goal with revised objective", updated)
	}
	if len(preview.items) != 1 || preview.items[0].sessionKey != updated.SessionKey || preview.items[0].title != updated.Objective {
		t.Fatalf("preview items = %#v, want app-server updated goal objective", preview.items)
	}
}

func TestServiceSetFromThreadGoalParamsSameObjectiveDoesNotQueueSteering(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	dispatcher := &fakeGuidanceDispatcher{}
	service.SetGuidanceDispatcher(dispatcher)
	ctx := context.Background()
	threadID := "room:group:conversation-1"
	objective := "Stable app-server goal"

	created, err := service.SetFromThreadGoalParams(ctx, protocol.ThreadGoalSetParams{
		ThreadID:  threadID,
		Objective: &objective,
	})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := service.SetFromThreadGoalParams(ctx, protocol.ThreadGoalSetParams{
		ThreadID:  threadID,
		Objective: &objective,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Version != created.Version+1 {
		t.Fatalf("updated version = %d, want refreshed %d", updated.Version, created.Version+1)
	}
	if len(dispatcher.items) != 0 {
		t.Fatalf("guidance = %#v, want no objective update steering", dispatcher.items)
	}
	if len(repo.events) != 2 || repo.events[1].EventType != "updated" {
		t.Fatalf("events = %#v, want app-server update event for explicit unchanged objective", repo.events)
	}
	if eventPayloadBool(repo.events[1].Payload, "objective_updated") {
		t.Fatalf("event payload = %#v, want no objective update marker for unchanged objective", repo.events[1].Payload)
	}
}

func TestServiceSetFromThreadGoalParamsDoesNotDispatchPausedGoal(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{
		GoalEnabled:             true,
		GoalAutoContinueEnabled: true,
	}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	dispatcher := &fakeContinuationDispatcher{}
	service.SetContinuationDispatcher(dispatcher)
	ctx := context.Background()
	threadID := "agent:nexus:ws:dm:chat"
	objective := "Do not start paused goal"
	paused := protocol.ThreadGoalStatusPaused

	if _, err := service.SetFromThreadGoalParams(ctx, protocol.ThreadGoalSetParams{
		ThreadID:  threadID,
		Objective: &objective,
		Status:    &paused,
	}); err != nil {
		t.Fatal(err)
	}
	if len(dispatcher.plans) != 0 {
		t.Fatalf("plans = %#v, want no continuation for paused goal", dispatcher.plans)
	}
}

func TestServiceCheckpointDoesNotInjectRuntimeContext(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Long work",
	})
	if err != nil {
		t.Fatal(err)
	}
	checkpoint, err := service.CreateCheckpointByModel(ctx, created.ID, protocol.CreateGoalCheckpointRequest{
		Summary: "Repository and service are wired.",
		RoundID: "round-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if checkpoint.ID != "goal_checkpoint_3" || checkpoint.Summary == "" {
		t.Fatalf("checkpoint = %#v, want persisted checkpoint", checkpoint)
	}
	contextText, goal, err := service.RuntimeContext(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if contextText != "" || goal == nil || goal.ID != created.ID {
		t.Fatalf("RuntimeContext() = (%q, %#v), want accounting target without checkpoint prompt", contextText, goal)
	}
	if len(repo.events) != 2 || repo.events[1].EventType != "checkpoint_created" {
		t.Fatalf("events = %#v, want checkpoint_created", repo.events)
	}
}

func TestServiceDisabled(t *testing.T) {
	service := NewService(config.Config{}, newMemoryRepository())
	_, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "disabled",
	})
	if !errors.Is(err, ErrGoalDisabled) {
		t.Fatalf("Create disabled error = %v, want ErrGoalDisabled", err)
	}
}

type memoryRepository struct {
	goals       map[string]protocol.Goal
	events      []protocol.GoalEvent
	checkpoints []protocol.GoalCheckpoint
}

func newMemoryRepository() *memoryRepository {
	return &memoryRepository{goals: map[string]protocol.Goal{}}
}

func (r *memoryRepository) CreateGoal(_ context.Context, item protocol.Goal) (*protocol.Goal, error) {
	for _, current := range r.goals {
		if current.SessionKey == item.SessionKey && protocol.IsCurrentGoalStatus(current.Status) {
			return nil, ErrGoalConflict
		}
	}
	r.goals[item.ID] = item
	return cloneGoal(item), nil
}

func (r *memoryRepository) GetGoal(_ context.Context, goalID string) (*protocol.Goal, error) {
	item, ok := r.goals[goalID]
	if !ok {
		return nil, nil
	}
	return cloneGoal(item), nil
}

func (r *memoryRepository) GetCurrentGoal(_ context.Context, sessionKey string) (*protocol.Goal, error) {
	for _, item := range r.goals {
		if item.SessionKey == sessionKey && protocol.IsCurrentGoalStatus(item.Status) {
			return cloneGoal(item), nil
		}
	}
	return nil, nil
}

func (r *memoryRepository) ListRunnableGoals(_ context.Context, limit int) ([]protocol.Goal, error) {
	items := make([]protocol.Goal, 0)
	for _, item := range r.goals {
		if item.Status == protocol.GoalStatusActive {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i int, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].UpdatedAt.Before(items[j].UpdatedAt)
	})
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (r *memoryRepository) UpdateGoal(_ context.Context, item protocol.Goal, expectedVersion int64) (*protocol.Goal, error) {
	current, ok := r.goals[item.ID]
	if !ok || current.Version != expectedVersion {
		return nil, sql.ErrNoRows
	}
	r.goals[item.ID] = item
	return cloneGoal(item), nil
}

func (r *memoryRepository) DeleteGoal(_ context.Context, goalID string) (bool, error) {
	if _, ok := r.goals[goalID]; !ok {
		return false, nil
	}
	delete(r.goals, goalID)
	return true, nil
}

func (r *memoryRepository) AppendEvent(_ context.Context, event protocol.GoalEvent) error {
	r.events = append(r.events, event)
	return nil
}

func (r *memoryRepository) ListEvents(_ context.Context, goalID string, _ int) ([]protocol.GoalEvent, error) {
	items := make([]protocol.GoalEvent, 0)
	for _, event := range r.events {
		if event.GoalID == goalID {
			items = append(items, event)
		}
	}
	return items, nil
}

func (r *memoryRepository) CreateCheckpoint(_ context.Context, checkpoint protocol.GoalCheckpoint) (*protocol.GoalCheckpoint, error) {
	r.checkpoints = append(r.checkpoints, checkpoint)
	clone := checkpoint
	return &clone, nil
}

func cloneGoal(item protocol.Goal) *protocol.Goal {
	clone := item
	return &clone
}

func fixedClock() func() time.Time {
	now := time.Date(2026, 5, 22, 10, 0, 0, 0, time.UTC)
	return func() time.Time {
		return now
	}
}

type mutableClock struct {
	now time.Time
}

func newMutableClock(now time.Time) *mutableClock {
	return &mutableClock{now: now}
}

func (c *mutableClock) Now() time.Time {
	return c.now
}

func (c *mutableClock) Advance(duration time.Duration) {
	c.now = c.now.Add(duration)
}

func sequentialID() func(string) string {
	next := 0
	return func(prefix string) string {
		next++
		return prefix + "_" + string(rune('0'+next))
	}
}

func optionalBudget(value int64) protocol.OptionalInt64 {
	return protocol.OptionalInt64{Present: true, Value: &value}
}

func clearBudget() protocol.OptionalInt64 {
	return protocol.OptionalInt64{Present: true}
}

func assertGoalInvalidInputMessage(t *testing.T, err error, want string) {
	t.Helper()
	if !errors.Is(err, ErrGoalInvalidInput) {
		t.Fatalf("error = %v, want ErrGoalInvalidInput", err)
	}
	if err.Error() != want {
		t.Fatalf("error text = %q, want %q", err.Error(), want)
	}
}

type fakeGoalBroadcaster struct {
	events []protocol.EventMessage
}

func (b *fakeGoalBroadcaster) BroadcastEvent(_ context.Context, _ string, event protocol.EventMessage) []error {
	b.events = append(b.events, event)
	return nil
}

type fakePreviewFiller struct {
	items []fakePreviewItem
}

type fakePreviewItem struct {
	sessionKey string
	title      string
}

func (f *fakePreviewFiller) FillEmptyPreviewFromGoal(_ context.Context, sessionKey string, title string) error {
	f.items = append(f.items, fakePreviewItem{sessionKey: sessionKey, title: title})
	return nil
}

type fakeGuidanceDispatcher struct {
	items []fakeGuidanceItem
}

type fakeGuidanceItem struct {
	sessionKey  string
	roundID     string
	contextName string
	content     string
}

func (d *fakeGuidanceDispatcher) QueueGuidanceInput(_ context.Context, sessionKey string, roundID string, content string) ([]string, error) {
	d.items = append(d.items, fakeGuidanceItem{
		sessionKey: sessionKey,
		roundID:    roundID,
		content:    content,
	})
	return []string{"round-running"}, nil
}

func (d *fakeGuidanceDispatcher) QueueContextualGuidanceInput(_ context.Context, sessionKey string, roundID string, contextName string, content string) ([]string, error) {
	d.items = append(d.items, fakeGuidanceItem{
		sessionKey:  sessionKey,
		roundID:     roundID,
		contextName: contextName,
		content:     content,
	})
	return []string{"round-running"}, nil
}

type fakeExternalMutationAccountant struct {
	service              *Service
	usage                protocol.GoalUsage
	roundID              string
	sessionKeys          []string
	clearedSessionKeys   []string
	activatedSessionKeys []string
}

func (a *fakeExternalMutationAccountant) FlushGoalAccounting(ctx context.Context, sessionKey string) ([]string, error) {
	a.sessionKeys = append(a.sessionKeys, sessionKey)
	if a.service != nil {
		if _, err := a.service.RecordUsageForSession(ctx, sessionKey, a.usage, a.roundID); err != nil {
			return nil, err
		}
	}
	return []string{a.roundID}, nil
}

func (a *fakeExternalMutationAccountant) ClearGoalAccounting(sessionKey string) []string {
	a.clearedSessionKeys = append(a.clearedSessionKeys, sessionKey)
	return []string{a.roundID}
}

func (a *fakeExternalMutationAccountant) ActivateGoalAccounting(_ context.Context, sessionKey string) ([]string, error) {
	a.activatedSessionKeys = append(a.activatedSessionKeys, sessionKey)
	return []string{a.roundID}, nil
}

type fakeContinuationDispatcher struct {
	deferSessions map[string]bool
	plans         []protocol.GoalContinuation
	deferCalls    int
	onShouldDefer func(call int, sessionKey string)
}

func (d *fakeContinuationDispatcher) ShouldDeferGoalContinuation(_ context.Context, sessionKey string) bool {
	d.deferCalls++
	if d.onShouldDefer != nil {
		d.onShouldDefer(d.deferCalls, sessionKey)
	}
	return d.deferSessions[sessionKey]
}

func (d *fakeContinuationDispatcher) DispatchGoalContinuation(_ context.Context, plan protocol.GoalContinuation) error {
	d.plans = append(d.plans, plan)
	return nil
}
