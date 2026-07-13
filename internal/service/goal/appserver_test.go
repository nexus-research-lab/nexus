package goal

import (
	"context"
	"errors"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalappserver "github.com/nexus-research-lab/nexus/internal/service/goal/appserver"
)

func TestServiceSetFromThreadGoalParamsCreatesAndUpdatesGoal(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()
	threadID := "agent:nexus:ws:dm:chat"
	objective := "Ship app-server parity"
	paused := goalappserver.ThreadGoalStatusPaused
	budget := int64(120)

	created, err := service.SetFromThreadGoalParams(ctx, goalappserver.ThreadGoalSetParams{
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

	usageLimited := goalappserver.ThreadGoalStatusUsageLimited
	updated, err := service.SetFromThreadGoalParams(ctx, goalappserver.ThreadGoalSetParams{
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
	paused := goalappserver.ThreadGoalStatusPaused

	created, err := service.SetFromThreadGoalParams(ctx, goalappserver.ThreadGoalSetParams{
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

func TestServiceSetFromThreadGoalParamsCompleteIsNotCurrentGoal(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()
	threadID := "agent:nexus:ws:dm:chat"
	objective := "Complete through app-server set"
	complete := goalappserver.ThreadGoalStatusComplete

	created, err := service.SetFromThreadGoalParams(ctx, goalappserver.ThreadGoalSetParams{
		ThreadID:  threadID,
		Objective: &objective,
		Status:    &complete,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Status != protocol.GoalStatusComplete || created.CompletedAt == nil {
		t.Fatalf("created = %#v, want completed goal", created)
	}
	current, err := service.CurrentOptional(ctx, threadID)
	if err != nil {
		t.Fatal(err)
	}
	if current != nil {
		t.Fatalf("current = %#v, want nil after app-server complete", current)
	}
	stored, err := repo.GetGoal(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored == nil || stored.Status != protocol.GoalStatusComplete {
		t.Fatalf("stored = %#v, want completed history record", stored)
	}
}

func TestServiceSetFromThreadGoalParamsRequiresObjectiveWhenMissing(t *testing.T) {
	service := NewService(config.Config{GoalEnabled: true}, newMemoryRepository())
	active := goalappserver.ThreadGoalStatusActive

	_, err := service.SetFromThreadGoalParams(context.Background(), goalappserver.ThreadGoalSetParams{
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
	budgetLimited := goalappserver.ThreadGoalStatusBudgetLimited

	created, err := service.SetFromThreadGoalParams(ctx, goalappserver.ThreadGoalSetParams{
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

	for _, status := range []goalappserver.ThreadGoalStatus{
		goalappserver.ThreadGoalStatusPaused,
		goalappserver.ThreadGoalStatusBlocked,
	} {
		updated, err := service.SetFromThreadGoalParams(ctx, goalappserver.ThreadGoalSetParams{
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

	created, err := service.SetFromThreadGoalParams(ctx, goalappserver.ThreadGoalSetParams{
		ThreadID:  threadID,
		Objective: &objective,
	})
	if err != nil {
		t.Fatal(err)
	}

	cleared, err := service.ClearFromThreadGoalParams(ctx, goalappserver.ThreadGoalClearParams{ThreadID: threadID})
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

	cleared, err = service.ClearFromThreadGoalParams(ctx, goalappserver.ThreadGoalClearParams{ThreadID: threadID})
	if err != nil {
		t.Fatal(err)
	}
	if cleared {
		t.Fatal("second ClearFromThreadGoalParams() cleared = true, want false")
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
	paused := goalappserver.ThreadGoalStatusPaused

	created, err := service.SetFromThreadGoalParams(ctx, goalappserver.ThreadGoalSetParams{
		ThreadID:  threadID,
		Objective: &objective,
		Status:    &paused,
	})
	if err != nil {
		t.Fatal(err)
	}
	accountant := &fakeExternalMutationAccountant{roundID: "round-running"}
	service.SetExternalMutationAccountant(accountant)
	active := goalappserver.ThreadGoalStatusActive

	updated, err := service.SetFromThreadGoalParams(ctx, goalappserver.ThreadGoalSetParams{
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

	created, err := service.SetFromThreadGoalParams(ctx, goalappserver.ThreadGoalSetParams{
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

func TestServiceSetFromThreadGoalParamsCanSuppressContinuationUntilResponse(t *testing.T) {
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
	objective := "Start after response ordering"

	created, err := service.SetFromThreadGoalParams(WithActiveGoalContinuationSuppressed(ctx), goalappserver.ThreadGoalSetParams{
		ThreadID:  threadID,
		Objective: &objective,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(dispatcher.plans) != 0 {
		t.Fatalf("plans = %#v, want suppressed continuation before response", dispatcher.plans)
	}
	current, err := service.Current(ctx, threadID)
	if err != nil {
		t.Fatal(err)
	}
	if current.ContinuationCount != 0 {
		t.Fatalf("ContinuationCount = %d, want 0 before explicit dispatch", current.ContinuationCount)
	}

	service.DispatchActiveGoalContinuation(ctx, *created)
	if len(dispatcher.plans) != 1 || dispatcher.plans[0].Goal.ID != created.ID {
		t.Fatalf("plans = %#v, want explicit continuation for active goal %q", dispatcher.plans, created.ID)
	}
	current, err = service.Current(ctx, threadID)
	if err != nil {
		t.Fatal(err)
	}
	if current.ContinuationCount != 1 {
		t.Fatalf("ContinuationCount = %d, want 1 after explicit dispatch", current.ContinuationCount)
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
	status := goalappserver.ThreadGoalStatusActive

	created, err := service.SetFromThreadGoalParams(ctx, goalappserver.ThreadGoalSetParams{
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

	created, err := service.SetFromThreadGoalParams(ctx, goalappserver.ThreadGoalSetParams{
		ThreadID:  threadID,
		Objective: &objective,
	})
	if err != nil {
		t.Fatal(err)
	}
	preview.items = nil

	updatedObjective := "Updated app-server goal"
	updated, err := service.SetFromThreadGoalParams(ctx, goalappserver.ThreadGoalSetParams{
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

	created, err := service.SetFromThreadGoalParams(ctx, goalappserver.ThreadGoalSetParams{
		ThreadID:  threadID,
		Objective: &objective,
	})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := service.SetFromThreadGoalParams(ctx, goalappserver.ThreadGoalSetParams{
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
	paused := goalappserver.ThreadGoalStatusPaused

	if _, err := service.SetFromThreadGoalParams(ctx, goalappserver.ThreadGoalSetParams{
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
