package goal

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

type claimingContinuationDispatcher struct {
	service     *Service
	dispatchErr error
	plans       []protocol.GoalContinuation
}

func (d *claimingContinuationDispatcher) ShouldDeferGoalContinuation(context.Context, string) bool {
	return false
}

func (d *claimingContinuationDispatcher) DispatchGoalContinuation(ctx context.Context, plan protocol.GoalContinuation) error {
	d.plans = append(d.plans, plan)
	if _, err := d.service.ClaimContinuationPlan(ctx, plan); err != nil {
		return err
	}
	return d.dispatchErr
}

func TestServiceRepairCurrentGoalPreviewsUsesDurableOwner(t *testing.T) {
	repo := newMemoryRepository()
	repo.goals["goal-current"] = protocol.Goal{
		ID:         "goal-current",
		SessionKey: "agent:nexus:ws:dm:conversation-current",
		Objective:  "Repair current Goal title",
		Status:     protocol.GoalStatusPaused,
		Metadata: map[string]any{
			protocol.GoalMetadataOwnerUserID: "owner-current",
		},
	}
	repo.goals["goal-complete"] = protocol.Goal{
		ID:         "goal-complete",
		SessionKey: "agent:nexus:ws:dm:conversation-complete",
		Objective:  "Do not replay completed Goal title",
		Status:     protocol.GoalStatusComplete,
		Metadata: map[string]any{
			protocol.GoalMetadataOwnerUserID: "owner-complete",
		},
	}
	repo.goals["goal-ownerless"] = protocol.Goal{
		ID:         "goal-ownerless",
		SessionKey: "agent:nexus:ws:dm:conversation-ownerless",
		Objective:  "Do not guess legacy owner",
		Status:     protocol.GoalStatusActive,
	}
	service := NewService(config.Config{GoalEnabled: true}, repo)
	preview := &fakePreviewFiller{}
	service.SetPreviewFiller(preview)

	if err := service.RepairCurrentGoalPreviews(context.Background()); err != nil {
		t.Fatalf("RepairCurrentGoalPreviews() error = %v", err)
	}
	if len(preview.repairs) != 1 ||
		preview.repairs[0].goal.ID != "goal-current" ||
		preview.repairs[0].ownerUserID != "owner-current" {
		t.Fatalf("preview repairs = %#v, want exact current Goal owner", preview.repairs)
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
	if current.ContinuationCount != 1 || current.EmptyProgressCount != goalContinuationSuppressionThreshold {
		t.Fatalf("goal counts = continuation %d empty %d, want failed continuation recorded", current.ContinuationCount, current.EmptyProgressCount)
	}
	if current.LastError != dispatchErr.Error() {
		t.Fatalf("LastError = %q, want dispatch error", current.LastError)
	}
	if got := repo.events[len(repo.events)-1]; got.EventType != "continuation_failed" {
		t.Fatalf("last event = %#v, want continuation_failed", got)
	}
}

func TestServiceRunAutoResumeOnceDurablyRetriesStartupFailure(t *testing.T) {
	repo := newDurableMemoryGoalRepository()
	service := NewService(config.Config{GoalEnabled: true, GoalAutoContinueEnabled: true}, repo)
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	service.nowFn = func() time.Time { return now }
	service.idFactory = sequentialID()
	ctx := context.Background()
	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:durable-auto-resume",
		Objective:  "retry runtime startup without suspending",
	})
	if err != nil {
		t.Fatal(err)
	}
	dispatchErr := errors.New("runtime registration unavailable")
	dispatcher := &claimingContinuationDispatcher{service: service, dispatchErr: dispatchErr}
	if err := service.RunAutoResumeOnce(ctx, dispatcher); err != nil {
		t.Fatalf("RunAutoResumeOnce: %v", err)
	}
	current, err := service.Current(ctx, created.SessionKey)
	if err != nil || current.Status != protocol.GoalStatusActive || current.LastError != "" || current.EmptyProgressCount != 0 || current.ContinuationCount != 1 {
		t.Fatalf("Goal after durable retry = %#v, %v", current, err)
	}
	if err := service.RunAutoResumeOnce(ctx, dispatcher); err != nil || len(dispatcher.plans) != 1 {
		t.Fatalf("retry before due: plans=%d err=%v", len(dispatcher.plans), err)
	}
	now = now.Add(goalContinuationRetryBase)
	if err := service.RunAutoResumeOnce(ctx, dispatcher); err != nil || len(dispatcher.plans) != 2 {
		t.Fatalf("retry when due: plans=%d err=%v", len(dispatcher.plans), err)
	}
	current, _ = service.Current(ctx, created.SessionKey)
	if current.ContinuationCount != 1 {
		t.Fatalf("ContinuationCount = %d, want one durable reservation", current.ContinuationCount)
	}
}

func TestServiceRunAutoResumeOnceContinuesAfterOneGoalFails(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{
		GoalEnabled:             true,
		GoalAutoContinueEnabled: true,
	}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	first := protocol.Goal{
		ID:         "goal-malformed-first",
		SessionKey: "malformed-session-key",
		Objective:  "first fails validation",
		Status:     protocol.GoalStatusActive,
		Version:    1,
		UpdatedAt:  fixedClock()(),
	}
	repo.goals[first.ID] = first
	second, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:second",
		Objective:  "second still dispatches",
	})
	if err != nil {
		t.Fatal(err)
	}
	dispatcher := &fakeContinuationDispatcher{}

	if err := service.RunAutoResumeOnce(ctx, dispatcher); err == nil ||
		!strings.Contains(err.Error(), first.ID) {
		t.Fatalf("RunAutoResumeOnce() error = %v, want aggregated malformed Goal error", err)
	}
	if len(dispatcher.plans) != 1 || dispatcher.plans[0].Goal.ID != second.ID {
		t.Fatalf("plans = %#v, want second Goal attempted after first failure", dispatcher.plans)
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
		CreatedBy:  "model",
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
