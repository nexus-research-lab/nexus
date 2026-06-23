package goal

import (
	"context"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

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

func TestServiceCompletionToolMissAllowsOneFinalizationRetry(t *testing.T) {
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
		Objective:  "Finish with a proper Goal update",
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := service.PlanContinuationForSession(ctx, created.SessionKey, "round-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.RecordCompletionToolMiss(ctx, created.ID, plan.RoundID, "assistant could not call update_goal"); err != nil {
		t.Fatal(err)
	}
	retry, err := service.PlanContinuationForSession(ctx, created.SessionKey, plan.RoundID)
	if err != nil {
		t.Fatal(err)
	}
	if retry == nil {
		t.Fatal("retry = nil, want finalization retry continuation")
	}
	for _, want := range []string{
		"Completion finalization retry:",
		"previous goal-continuation response",
		"mcp__nexus_goal__update_goal",
		"before any final response",
	} {
		if !strings.Contains(retry.Prompt, want) {
			t.Fatalf("retry prompt missing %q: %s", want, retry.Prompt)
		}
	}
	current, err := service.Current(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current.EmptyProgressCount != 0 || goalCompletionToolRetryCount(current.Metadata) != 1 {
		t.Fatalf("current = %#v, want one retry without empty-progress suppression", current)
	}
	if got := repo.events[len(repo.events)-2]; got.EventType != "completion_tool_retry" || got.RoundID != plan.RoundID {
		t.Fatalf("retry event = %#v, want completion_tool_retry for first miss", got)
	}
}

func TestServiceCompletionToolMissCompletesAfterRetry(t *testing.T) {
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
	})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := service.PlanContinuationForSession(ctx, created.SessionKey, "round-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.RecordCompletionToolMiss(ctx, created.ID, plan.RoundID, "first miss"); err != nil {
		t.Fatal(err)
	}
	retry, err := service.PlanContinuationForSession(ctx, created.SessionKey, plan.RoundID)
	if err != nil {
		t.Fatal(err)
	}
	if retry == nil {
		t.Fatal("retry = nil, want finalization retry continuation")
	}
	if _, err := service.RecordCompletionToolMiss(ctx, created.ID, retry.RoundID, "second miss"); err != nil {
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
	if completed.Status != protocol.GoalStatusComplete || goalCompletionToolRetryCount(completed.Metadata) != 0 {
		t.Fatalf("completed = %#v, want complete with retry metadata cleared", completed)
	}
	if got := repo.events[len(repo.events)-1]; got.EventType != "completed" || got.Payload["reason"] != "second miss" {
		t.Fatalf("last event = %#v, want completed event with second miss reason", got)
	}
}

func TestServicePlanContinuationCompletesStaleCompletionToolMissSuppression(t *testing.T) {
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
	})
	if err != nil {
		t.Fatal(err)
	}
	stale := repo.goals[created.ID]
	stale.EmptyProgressCount = 1
	stale.Metadata = map[string]any{goalCompletionToolRetryMetadataKey: 1}
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
	if completed.Status != protocol.GoalStatusComplete || goalCompletionToolRetryCount(completed.Metadata) != 0 {
		t.Fatalf("completed = %#v, want complete with retry metadata cleared", completed)
	}
	if got := repo.events[len(repo.events)-1]; got.EventType != "completed" || got.RoundID != "retry-round" {
		t.Fatalf("last event = %#v, want completed event for stale retry suppression", got)
	}
}

func TestServiceResumeActiveGoalClearsEmptyProgressAndDispatchesContinuation(t *testing.T) {
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

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Resume suppressed active goal",
	})
	if err != nil {
		t.Fatal(err)
	}
	dispatcher.plans = nil
	if _, err := service.RecordContinuationProgress(ctx, created.ID, "goal_continuation_1", false); err != nil {
		t.Fatal(err)
	}

	resumed, err := service.Resume(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Status != protocol.GoalStatusActive || resumed.EmptyProgressCount != 0 {
		t.Fatalf("resumed = %#v, want active goal with empty progress cleared", resumed)
	}
	if len(dispatcher.plans) != 1 || dispatcher.plans[0].Goal.ID != created.ID {
		t.Fatalf("plans = %#v, want resumed active goal to dispatch continuation", dispatcher.plans)
	}
	if got := repo.events[len(repo.events)-2]; got.EventType != "resumed" {
		t.Fatalf("event before continuation = %#v, want resumed", got)
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
	if updated.LastError != "Failed to authenticate. API Error: 401" || updated.EmptyProgressCount != 1 {
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
	if got := repo.events[len(repo.events)-1]; got.EventType != "continuation_suppressed" || got.RoundID != "goal_continuation_1" {
		t.Fatalf("last event = %#v, want continuation_suppressed after retry", got)
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
