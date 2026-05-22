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

func TestServicePlanContinuationPausesWhenBudgetExhausted(t *testing.T) {
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
	if _, err := service.RecordUsageForSession(ctx, created.SessionKey, protocol.GoalUsage{TotalTokens: 10}, "round-1"); err != nil {
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
	if current.Status != protocol.GoalStatusPaused || current.LastError == "" {
		t.Fatalf("current = %#v, want paused with last error", current)
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
	for _, want := range []string{"<nexus_goal>", "Complete parity", "TokenBudget: 100", "调用 Goal 工具标记完成"} {
		if !strings.Contains(contextText, want) {
			t.Fatalf("RuntimeContext missing %q: %s", want, contextText)
		}
	}
}

type memoryRepository struct {
	goals  map[string]protocol.Goal
	events []protocol.GoalEvent
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
