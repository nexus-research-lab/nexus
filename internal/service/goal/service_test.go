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
	service := NewService(config.Config{GoalEnabled: true, GoalDefaultTokenBudget: 100}, repo)
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
	if created.TokenBudget == nil || *created.TokenBudget != 100 {
		t.Fatalf("TokenBudget = %#v, want default 100", created.TokenBudget)
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
	if !strings.Contains(plan.Prompt, "Complete parity") || !strings.Contains(plan.Prompt, "PreviousRoundID: round-1") {
		t.Fatalf("continuation prompt missing context: %s", plan.Prompt)
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

func TestServiceRunAutoResumeOnceSkipsBusyGoal(t *testing.T) {
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
		Objective:  "Busy goal",
	}); err != nil {
		t.Fatal(err)
	}
	dispatcher := &fakeContinuationDispatcher{busy: map[string]bool{"agent:nexus:ws:dm:chat": true}}
	if err := service.RunAutoResumeOnce(ctx, dispatcher); err != nil {
		t.Fatal(err)
	}
	if len(dispatcher.plans) != 0 {
		t.Fatalf("plans = %#v, want no dispatch for busy session", dispatcher.plans)
	}
}

func TestServiceCheckpointUpdatesRuntimeContext(t *testing.T) {
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
	contextText, _, err := service.RuntimeContext(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(contextText, "LatestCheckpoint") ||
		!strings.Contains(contextText, "Repository and service are wired.") {
		t.Fatalf("runtime context missing checkpoint: %s", contextText)
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

func TestBuildRuntimeContext(t *testing.T) {
	budget := int64(100)
	contextText := BuildRuntimeContext(protocol.Goal{
		Objective:   "Complete parity",
		Status:      protocol.GoalStatusActive,
		TokenBudget: &budget,
		Usage: protocol.GoalUsage{
			InputTokens:  10,
			OutputTokens: 20,
			TotalTokens:  30,
		},
	})
	for _, want := range []string{"<nexus_goal>", "Complete parity", "TokenBudget: 100", "RemainingTokens: 70", "update_goal with status=complete"} {
		if !strings.Contains(contextText, want) {
			t.Fatalf("RuntimeContext missing %q: %s", want, contextText)
		}
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

func (r *memoryRepository) LatestCheckpoint(_ context.Context, goalID string) (*protocol.GoalCheckpoint, error) {
	for index := len(r.checkpoints) - 1; index >= 0; index-- {
		if r.checkpoints[index].GoalID == goalID {
			clone := r.checkpoints[index]
			return &clone, nil
		}
	}
	return nil, nil
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

func sequentialID() func(string) string {
	next := 0
	return func(prefix string) string {
		next++
		return prefix + "_" + string(rune('0'+next))
	}
}

type fakeGoalBroadcaster struct {
	events []protocol.EventMessage
}

func (b *fakeGoalBroadcaster) BroadcastEvent(_ context.Context, _ string, event protocol.EventMessage) []error {
	b.events = append(b.events, event)
	return nil
}

type fakeContinuationDispatcher struct {
	busy  map[string]bool
	plans []protocol.GoalContinuation
}

func (d *fakeContinuationDispatcher) IsGoalSessionBusy(sessionKey string) bool {
	return d.busy[sessionKey]
}

func (d *fakeContinuationDispatcher) DispatchGoalContinuation(_ context.Context, plan protocol.GoalContinuation) error {
	d.plans = append(d.plans, plan)
	return nil
}
