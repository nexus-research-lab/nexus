package goal

import (
	"context"
	"errors"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

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

func TestServiceRunAutoResumeOnceReleasesPlanWhenDispatchDefers(t *testing.T) {
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
		Objective:  "Do not count deferred continuation",
	})
	if err != nil {
		t.Fatal(err)
	}
	dispatcher := &fakeContinuationDispatcher{}
	dispatcher.onShouldDefer = func(call int, sessionKey string) {
		if call == 2 {
			dispatcher.deferSessions = map[string]bool{sessionKey: true}
		}
	}
	if err := service.RunAutoResumeOnce(ctx, dispatcher); err != nil {
		t.Fatal(err)
	}
	if len(dispatcher.plans) != 0 {
		t.Fatalf("plans = %#v, want no dispatch after second defer", dispatcher.plans)
	}
	current, err := service.Current(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current.ContinuationCount != 0 {
		t.Fatalf("ContinuationCount = %d, want deferred continuation released", current.ContinuationCount)
	}
	if got := repo.events[len(repo.events)-1]; got.EventType != "continuation_deferred" {
		t.Fatalf("last event = %#v, want continuation_deferred", got)
	}
}

func TestServiceRunAutoResumeOnceRecordsFailureWhenDispatchFails(t *testing.T) {
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
		Objective:  "Do not count failed continuation dispatch",
	})
	if err != nil {
		t.Fatal(err)
	}
	dispatchErr := errors.New("runtime start failed")
	dispatcher := &fakeContinuationDispatcher{dispatchErr: dispatchErr}
	if err := service.RunAutoResumeOnce(ctx, dispatcher); err != nil {
		t.Fatalf("RunAutoResumeOnce error = %v, want nil after recording dispatch failure", err)
	}
	if len(dispatcher.plans) != 1 {
		t.Fatalf("plans = %#v, want attempted dispatch", dispatcher.plans)
	}
	current, err := service.Current(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current.ContinuationCount != 1 || current.EmptyProgressCount != 1 {
		t.Fatalf("goal counts = continuation %d empty %d, want failed continuation recorded", current.ContinuationCount, current.EmptyProgressCount)
	}
	if current.LastError != dispatchErr.Error() {
		t.Fatalf("LastError = %q, want dispatch error", current.LastError)
	}
	if got := repo.events[len(repo.events)-1]; got.EventType != "continuation_failed" {
		t.Fatalf("last event = %#v, want continuation_failed", got)
	}
}

func TestServiceRunAutoResumeOnceIgnoresStaleDispatchAfterRetarget(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{
		GoalEnabled:             true,
		GoalAutoContinueEnabled: true,
	}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:stale-auto-resume",
		Objective:  "Analyze M3 and M4",
	})
	if err != nil {
		t.Fatal(err)
	}
	dispatcher := &fakeContinuationDispatcher{}
	dispatcher.onDispatch = func(plan protocol.GoalContinuation) error {
		if _, retargetErr := service.RetargetByModel(ctx, created.SessionKey, protocol.RetargetGoalRequest{
			Objective:                 "Analyze M4 and M5",
			RoundID:                   "round-correction",
			ExpectedObjectiveRevision: plan.Goal.ObjectiveRevision(),
		}); retargetErr != nil {
			return retargetErr
		}
		return ErrGoalRevisionStale
	}
	if err := service.RunAutoResumeOnce(ctx, dispatcher); err != nil {
		t.Fatalf("RunAutoResumeOnce error = %v, want stale dispatch ignored", err)
	}
	current, err := service.Current(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current.Objective != "Analyze M4 and M5" || current.EmptyProgressCount != 0 || current.ContinuationCount != 0 || current.LastError != "" {
		t.Fatalf("current = %#v, want corrected Goal untouched by stale dispatch failure", current)
	}
	for _, event := range repo.events {
		if event.EventType == "continuation_failed" {
			t.Fatalf("stale dispatch recorded failure: %#v", event)
		}
	}
}

func TestServiceRunAutoResumeOnceClearsMissingContinuationTarget(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{
		GoalEnabled:             true,
		GoalAutoContinueEnabled: true,
	}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:deleted-agent:ws:dm:chat",
		Objective:  "Clean stale goal after agent deletion",
	})
	if err != nil {
		t.Fatal(err)
	}
	dispatcher := &fakeContinuationDispatcher{missingSessions: map[string]bool{created.SessionKey: true}}
	if err := service.RunAutoResumeOnce(ctx, dispatcher); err != nil {
		t.Fatalf("RunAutoResumeOnce error = %v", err)
	}
	if len(dispatcher.plans) != 0 {
		t.Fatalf("plans = %#v, want no dispatch for missing target", dispatcher.plans)
	}
	if _, ok := repo.goals[created.ID]; ok {
		t.Fatal("missing target goal still exists")
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
	current := repo.goals[created.ID]
	if current.ContinuationCount != 0 {
		t.Fatalf("ContinuationCount = %d, want stale unstarted continuation released", current.ContinuationCount)
	}
	if got := repo.events[len(repo.events)-1]; got.EventType != "continuation_deferred" {
		t.Fatalf("last event = %#v, want continuation_deferred for stale unstarted plan", got)
	}
}
