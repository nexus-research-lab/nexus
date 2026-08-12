package realtime

import (
	"context"
	"database/sql"
	"sync"
	"testing"

	sdkprotocol "github.com/nexus-research-lab/nexus-agent-sdk-bridge/protocol"
	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	runtimectx "github.com/nexus-research-lab/nexus/internal/runtime"
	exec "github.com/nexus-research-lab/nexus/internal/runtime/exec"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	goalappserver "github.com/nexus-research-lab/nexus/internal/service/goal/appserver"
)

func TestRoomActiveGoalUpdateDoesNotResetDeferredActual(t *testing.T) {
	fixture := newRoomGoalBoundaryFixture(t, "active-update")
	defer fixture.cleanup()

	fixture.recordAssistantUsage(90, 10)
	replacement := "Keep the same Goal while the Room slot is running"
	if _, err := fixture.goals.Update(
		context.Background(),
		fixture.goal.ID,
		protocol.UpdateGoalRequest{Objective: &replacement},
	); err != nil {
		t.Fatal(err)
	}

	fixture.room.finalizeGoalUsageForSlot(
		context.Background(),
		fixture.slot,
		exec.RoundExecutionResult{Usage: sdkprotocol.TokenUsage{
			InputTokens:  90,
			OutputTokens: 10,
			TotalTokens:  100,
		}},
		nil,
	)
	stored, err := fixture.repo.GetGoal(context.Background(), fixture.goal.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored == nil ||
		stored.Usage.BudgetTokens() != 100 ||
		stored.Usage.ActualTokens() != 100 ||
		stored.Usage.ActualTokensAreEstimated() {
		t.Fatalf("usage after same-Goal activate = %#v, want exact terminal actual 100", stored)
	}
}

func TestRoomExternalClearSettlesObservedActualBeforeBindingClear(t *testing.T) {
	fixture := newRoomGoalBoundaryFixture(t, "external-clear")
	defer fixture.cleanup()

	fixture.recordAssistantUsage(90, 10)
	cleared, err := fixture.goals.Clear(context.Background(), fixture.goal.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !cleared {
		t.Fatal("Clear() = false, want true")
	}
	deleted := fixture.repo.deletedGoal()
	if deleted == nil ||
		deleted.Usage.BudgetTokens() != 100 ||
		deleted.Usage.ActualTokens() != 100 ||
		deleted.Usage.ActualTokensAreEstimated() {
		t.Fatalf("deleted Goal usage = %#v, want settled observed actual 100", deleted)
	}
	if fixture.slot.goalIDForUsage() != "" || fixture.slot.goalUsageActive() {
		t.Fatal("external clear left the Room slot Goal binding active")
	}
}

func TestRoomExternalCompleteKeepsBindingUntilTerminalThenFinalizes(t *testing.T) {
	fixture := newRoomGoalBoundaryFixture(t, "external-complete")

	fixture.recordAssistantUsage(90, 10)
	complete := goalappserver.ThreadGoalStatusComplete
	completed, err := fixture.goals.SetFromThreadGoalParams(
		context.Background(),
		goalappserver.ThreadGoalSetParams{
			ThreadID: fixture.goal.SessionKey,
			Status:   &complete,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if completed.UsageFinalized {
		t.Fatal("external completion finalized usage before the running Room slot terminal")
	}
	if fixture.slot.goalIDForUsage() != fixture.goal.ID || !fixture.slot.goalUsageActive() {
		t.Fatal("external completion cleared the Room slot Goal binding before terminal usage")
	}

	fixture.slot.setStatus("finished")
	fixture.room.finalizeGoalUsageForSlot(
		context.Background(),
		fixture.slot,
		exec.RoundExecutionResult{Usage: sdkprotocol.TokenUsage{
			InputTokens:  90,
			OutputTokens: 10,
			TotalTokens:  100,
		}},
		nil,
	)
	if !fixture.room.finalizeCompletedRoomGoalUsage(context.Background(), fixture.round) {
		t.Fatal("shared Room Goal usage did not finalize after the parent terminal")
	}
	stored, err := fixture.repo.GetGoal(context.Background(), fixture.goal.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored == nil ||
		!stored.UsageFinalized ||
		stored.Usage.ActualTokens() != 100 ||
		stored.Usage.ActualTokensAreEstimated() {
		t.Fatalf("finalized Goal = %#v, want exact terminal actual 100", stored)
	}

	fixture.cleanup()
	if rounds := fixture.manager.BeginGoalAccountingFinalizing(fixture.goal.SessionKey); len(rounds) != 0 {
		t.Fatalf("cleanup left Room finalization hooks registered: %#v", rounds)
	}
}

type roomGoalBoundaryFixture struct {
	room    *Service
	goals   *goalsvc.Service
	manager *runtimectx.Manager
	repo    *roomGoalBoundaryRepository
	goal    *protocol.Goal
	slot    *activeRoomSlot
	round   *activeRoomRound
	cleanup func()
}

func (f roomGoalBoundaryFixture) recordAssistantUsage(inputTokens int64, outputTokens int64) {
	message := roomGoalAssistantUsageMessage(inputTokens, outputTokens)
	f.slot.rememberGoalAssistantMessage(message)
	f.room.recordGoalUsageFromSlotAssistantMessage(context.Background(), f.slot, message)
}

func newRoomGoalBoundaryFixture(t *testing.T, suffix string) roomGoalBoundaryFixture {
	t.Helper()
	sessionKey := protocol.BuildRoomSharedSessionKey("conversation-" + suffix)
	slot := &activeRoomSlot{
		AgentID:      "agent-" + suffix,
		AgentRoundID: "agent-round-" + suffix,
		RuntimeSessionKey: protocol.BuildRoomAgentSessionKey(
			"conversation-"+suffix,
			"agent-"+suffix,
			protocol.RoomTypeGroup,
		),
	}
	slot.setGoalBinding(sessionKey, "")
	slot.setGoalUsageAccumulator(goalsvc.NewRuntimeUsageAccumulator(false))
	slot.setStatus("running")
	roundValue := &activeRoomRound{
		SessionKey:     sessionKey,
		ConversationID: "conversation-" + suffix,
		RoundID:        "room-round-" + suffix,
		OwnerUserID:    "owner-" + suffix,
		Slots:          map[string]*activeRoomSlot{"worker": slot},
	}
	manager := runtimectx.NewManager()
	repo := newRoomGoalBoundaryRepository()
	goalService := goalsvc.NewService(config.Config{GoalEnabled: true}, repo)
	goalService.SetExternalMutationAccountant(manager)
	roomService := &Service{
		goals:   goalService,
		runtime: manager,
		rounds: newRoomRoundRegistryFromRounds(map[string]*activeRoomRound{
			roundValue.RoundID: roundValue,
		}),
	}
	cleanup := roomService.registerSlotGoalRuntime(slot)
	created, err := goalService.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey: sessionKey,
		Objective:  "Exercise Room Goal boundary " + suffix,
		CreatedBy:  "user",
	})
	if err != nil {
		cleanup()
		t.Fatal(err)
	}
	if slot.goalIDForUsage() != created.ID || !slot.goalUsageActive() {
		cleanup()
		t.Fatalf("external create did not activate Room slot Goal %q", created.ID)
	}
	return roomGoalBoundaryFixture{
		room:    roomService,
		goals:   goalService,
		manager: manager,
		repo:    repo,
		goal:    created,
		slot:    slot,
		round:   roundValue,
		cleanup: cleanup,
	}
}

type roomGoalBoundaryRepository struct {
	mu      sync.Mutex
	goals   map[string]protocol.Goal
	events  []protocol.GoalEvent
	deleted *protocol.Goal
}

func newRoomGoalBoundaryRepository() *roomGoalBoundaryRepository {
	return &roomGoalBoundaryRepository{goals: make(map[string]protocol.Goal)}
}

func (r *roomGoalBoundaryRepository) CreateGoal(
	_ context.Context,
	item protocol.Goal,
) (*protocol.Goal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.goals[item.ID] = item
	return cloneRoomBoundaryGoal(item), nil
}

func (r *roomGoalBoundaryRepository) CreateGoalWithEvent(
	_ context.Context,
	item protocol.Goal,
	event protocol.GoalEvent,
) (*protocol.Goal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.goals[item.ID] = item
	r.events = append(r.events, event)
	return cloneRoomBoundaryGoal(item), nil
}

func (r *roomGoalBoundaryRepository) GetGoal(
	_ context.Context,
	goalID string,
) (*protocol.Goal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.goals[goalID]
	if !ok {
		return nil, nil
	}
	return cloneRoomBoundaryGoal(item), nil
}

func (r *roomGoalBoundaryRepository) GetCurrentGoal(
	_ context.Context,
	sessionKey string,
) (*protocol.Goal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, item := range r.goals {
		if item.SessionKey == sessionKey && protocol.IsCurrentGoalStatus(item.Status) {
			return cloneRoomBoundaryGoal(item), nil
		}
	}
	return nil, nil
}

func (r *roomGoalBoundaryRepository) ListGoals(
	_ context.Context,
) ([]protocol.Goal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := make([]protocol.Goal, 0, len(r.goals))
	for _, item := range r.goals {
		items = append(items, item)
	}
	return items, nil
}

func (r *roomGoalBoundaryRepository) ListCurrentGoals(
	_ context.Context,
) ([]protocol.Goal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := make([]protocol.Goal, 0, len(r.goals))
	for _, item := range r.goals {
		if protocol.IsCurrentGoalStatus(item.Status) {
			items = append(items, item)
		}
	}
	return items, nil
}

func (r *roomGoalBoundaryRepository) ListRunnableGoals(
	_ context.Context,
	limit int,
) ([]protocol.Goal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := make([]protocol.Goal, 0, len(r.goals))
	for _, item := range r.goals {
		if protocol.NormalizeGoalStatus(item.Status) == protocol.GoalStatusActive {
			items = append(items, item)
		}
	}
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

func (r *roomGoalBoundaryRepository) UpdateGoal(
	_ context.Context,
	item protocol.Goal,
	expectedVersion int64,
) (*protocol.Goal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	current, ok := r.goals[item.ID]
	if !ok || current.Version != expectedVersion {
		return nil, sql.ErrNoRows
	}
	r.goals[item.ID] = item
	return cloneRoomBoundaryGoal(item), nil
}

func (r *roomGoalBoundaryRepository) UpdateGoalWithEvents(
	_ context.Context,
	item protocol.Goal,
	expectedVersion int64,
	events []protocol.GoalEvent,
) (*protocol.Goal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	current, ok := r.goals[item.ID]
	if !ok || current.Version != expectedVersion {
		return nil, sql.ErrNoRows
	}
	r.goals[item.ID] = item
	r.events = append(r.events, events...)
	return cloneRoomBoundaryGoal(item), nil
}

func (r *roomGoalBoundaryRepository) FinalizeGoalUsage(
	_ context.Context,
	item protocol.Goal,
	expectedVersion int64,
	event protocol.GoalEvent,
) (*protocol.Goal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	current, ok := r.goals[item.ID]
	if !ok ||
		current.Version != expectedVersion ||
		current.UsageFinalized ||
		protocol.NormalizeGoalStatus(current.Status) != protocol.GoalStatusComplete {
		return nil, sql.ErrNoRows
	}
	r.goals[item.ID] = item
	r.events = append(r.events, event)
	return cloneRoomBoundaryGoal(item), nil
}

func (r *roomGoalBoundaryRepository) DeleteGoal(
	_ context.Context,
	goalID string,
) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.goals[goalID]
	if !ok {
		return false, nil
	}
	r.deleted = cloneRoomBoundaryGoal(item)
	delete(r.goals, goalID)
	return true, nil
}

func (r *roomGoalBoundaryRepository) AppendEvent(
	_ context.Context,
	event protocol.GoalEvent,
) error {
	r.mu.Lock()
	r.events = append(r.events, event)
	r.mu.Unlock()
	return nil
}

func (r *roomGoalBoundaryRepository) ListEvents(
	_ context.Context,
	goalID string,
	_ int,
) ([]protocol.GoalEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	items := make([]protocol.GoalEvent, 0)
	for _, event := range r.events {
		if event.GoalID == goalID {
			items = append(items, event)
		}
	}
	return items, nil
}

func (r *roomGoalBoundaryRepository) deletedGoal() *protocol.Goal {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.deleted == nil {
		return nil
	}
	return cloneRoomBoundaryGoal(*r.deleted)
}

func cloneRoomBoundaryGoal(item protocol.Goal) *protocol.Goal {
	cloned := item
	return &cloned
}
