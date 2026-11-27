// lifecycle_test.go 覆盖 Goal 公共夹具、生命周期、预检、清理、恢复与重定目标测试。
package goal

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalappserver "github.com/nexus-research-lab/nexus/internal/service/goal/appserver"
)

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

func (r *memoryRepository) CreateGoalWithEvent(
	_ context.Context,
	item protocol.Goal,
	event protocol.GoalEvent,
) (*protocol.Goal, error) {
	for _, current := range r.goals {
		if current.SessionKey == item.SessionKey && protocol.IsCurrentGoalStatus(current.Status) {
			return nil, ErrGoalConflict
		}
	}
	r.goals[item.ID] = item
	r.events = append(r.events, event)
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

func (r *memoryRepository) ListGoals(_ context.Context) ([]protocol.Goal, error) {
	items := make([]protocol.Goal, 0, len(r.goals))
	for _, item := range r.goals {
		items = append(items, item)
	}
	sort.Slice(items, func(i int, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].UpdatedAt.Before(items[j].UpdatedAt)
	})
	return items, nil
}

func (r *memoryRepository) ListCurrentGoals(_ context.Context) ([]protocol.Goal, error) {
	items := make([]protocol.Goal, 0, len(r.goals))
	for _, item := range r.goals {
		if protocol.IsCurrentGoalStatus(item.Status) {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i int, j int) bool {
		if items[i].UpdatedAt.Equal(items[j].UpdatedAt) {
			return items[i].ID < items[j].ID
		}
		return items[i].UpdatedAt.Before(items[j].UpdatedAt)
	})
	return items, nil
}

func (r *memoryRepository) ListRunnableGoals(_ context.Context, limit int) ([]protocol.Goal, error) {
	items := make([]protocol.Goal, 0)
	for _, item := range r.goals {
		if item.Status == protocol.GoalStatusActive &&
			item.ContinuationState() != protocol.GoalContinuationStateSuspended {
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

func (r *memoryRepository) UpdateGoalWithEvents(
	_ context.Context,
	item protocol.Goal,
	expectedVersion int64,
	events []protocol.GoalEvent,
) (*protocol.Goal, error) {
	current, ok := r.goals[item.ID]
	if !ok || current.Version != expectedVersion {
		return nil, sql.ErrNoRows
	}
	r.goals[item.ID] = item
	r.events = append(r.events, events...)
	return cloneGoal(item), nil
}

func (r *memoryRepository) FinalizeGoalUsage(
	_ context.Context,
	item protocol.Goal,
	expectedVersion int64,
	event protocol.GoalEvent,
) (*protocol.Goal, error) {
	current, ok := r.goals[item.ID]
	if !ok || current.Version != expectedVersion || current.UsageFinalized ||
		current.Status != protocol.GoalStatusComplete {
		return nil, sql.ErrNoRows
	}
	r.goals[item.ID] = item
	r.events = append(r.events, event)
	return cloneGoal(item), nil
}

func (r *memoryRepository) DeleteGoal(_ context.Context, goalID string) (bool, error) {
	if _, ok := r.goals[goalID]; !ok {
		return false, nil
	}
	delete(r.goals, goalID)
	events := r.events[:0]
	for _, event := range r.events {
		if event.GoalID != goalID {
			events = append(events, event)
		}
	}
	r.events = events
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

type staleOnceUsageRepository struct {
	*memoryRepository
	staleGoalID     string
	concurrentUsage protocol.GoalUsage
	injected        bool
}

func (r *staleOnceUsageRepository) UpdateGoal(ctx context.Context, item protocol.Goal, expectedVersion int64) (*protocol.Goal, error) {
	if !r.injected && item.ID == r.staleGoalID {
		r.injected = true
		current := r.goals[item.ID]
		current.Usage = current.Usage.Add(r.concurrentUsage)
		current.TimeUsedSeconds += r.concurrentUsage.RuntimeSeconds
		current.Version++
		r.goals[item.ID] = current
		return nil, sql.ErrNoRows
	}
	return r.memoryRepository.UpdateGoal(ctx, item, expectedVersion)
}

func (r *staleOnceUsageRepository) UpdateGoalWithEvents(
	ctx context.Context,
	item protocol.Goal,
	expectedVersion int64,
	events []protocol.GoalEvent,
) (*protocol.Goal, error) {
	if !r.injected && item.ID == r.staleGoalID {
		r.injected = true
		current := r.goals[item.ID]
		current.Usage = current.Usage.Add(r.concurrentUsage)
		current.TimeUsedSeconds += r.concurrentUsage.RuntimeSeconds
		current.Version++
		r.goals[item.ID] = current
		return nil, sql.ErrNoRows
	}
	return r.memoryRepository.UpdateGoalWithEvents(ctx, item, expectedVersion, events)
}

type staleOnceVersionRepository struct {
	*memoryRepository
	staleGoalID string
	injected    bool
	mutate      func(protocol.Goal) protocol.Goal
}

func (r *staleOnceVersionRepository) UpdateGoal(ctx context.Context, item protocol.Goal, expectedVersion int64) (*protocol.Goal, error) {
	if !r.injected && item.ID == r.staleGoalID {
		r.injected = true
		current := r.goals[item.ID]
		if r.mutate != nil {
			current = r.mutate(current)
		}
		current.Version++
		r.goals[item.ID] = current
		return nil, sql.ErrNoRows
	}
	return r.memoryRepository.UpdateGoal(ctx, item, expectedVersion)
}

func (r *staleOnceVersionRepository) UpdateGoalWithEvents(
	ctx context.Context,
	item protocol.Goal,
	expectedVersion int64,
	events []protocol.GoalEvent,
) (*protocol.Goal, error) {
	if !r.injected && item.ID == r.staleGoalID {
		r.injected = true
		current := r.goals[item.ID]
		if r.mutate != nil {
			current = r.mutate(current)
		}
		current.Version++
		r.goals[item.ID] = current
		return nil, sql.ErrNoRows
	}
	return r.memoryRepository.UpdateGoalWithEvents(ctx, item, expectedVersion, events)
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
	items          []fakePreviewItem
	titleSchedules []fakePreviewTitleSchedule
	repairs        []fakePreviewRepair
}

type fakePreviewItem struct {
	sessionKey string
	title      string
}

type fakePreviewTitleSchedule struct {
	goal          protocol.Goal
	ownerUserID   string
	fallbackTitle string
}

type fakePreviewRepair struct {
	goal        protocol.Goal
	ownerUserID string
}

func (f *fakePreviewFiller) FillEmptyPreviewFromGoal(_ context.Context, sessionKey string, title string) error {
	f.items = append(f.items, fakePreviewItem{sessionKey: sessionKey, title: title})
	return nil
}

func (f *fakePreviewFiller) ScheduleGoalTitleFromGoal(_ context.Context, goal protocol.Goal, ownerUserID string, fallbackTitle string) {
	f.titleSchedules = append(f.titleSchedules, fakePreviewTitleSchedule{
		goal:          goal,
		ownerUserID:   ownerUserID,
		fallbackTitle: fallbackTitle,
	})
}

func (f *fakePreviewFiller) RepairGoalTitleFromGoal(_ context.Context, goal protocol.Goal, ownerUserID string) error {
	f.repairs = append(f.repairs, fakePreviewRepair{goal: goal, ownerUserID: ownerUserID})
	return nil
}

type fakeGuidanceDispatcher struct {
	items []fakeGuidanceItem
}

type fakeGuidanceItem struct {
	sessionKey        string
	roundID           string
	contextName       string
	content           string
	excludedAgentID   string
	objectiveRevision int64
}

func (d *fakeGuidanceDispatcher) QueueGuidanceInput(_ context.Context, sessionKey string, roundID string, content string) ([]string, error) {
	d.items = append(d.items, fakeGuidanceItem{
		sessionKey: sessionKey,
		roundID:    roundID,
		content:    content,
	})
	return []string{"round-running"}, nil
}

func (d *fakeGuidanceDispatcher) QueueContextualGuidanceInput(_ context.Context, sessionKey string, roundID string, contextName string, content string, objectiveRevision int64) ([]string, error) {
	d.items = append(d.items, fakeGuidanceItem{
		sessionKey:        sessionKey,
		roundID:           roundID,
		contextName:       contextName,
		content:           content,
		objectiveRevision: objectiveRevision,
	})
	return []string{"round-running"}, nil
}

func (d *fakeGuidanceDispatcher) QueueRoomContextualGuidanceInput(_ context.Context, sessionKey string, roundID string, contextName string, content string, excludedAgentID string, objectiveRevision int64) ([]string, error) {
	d.items = append(d.items, fakeGuidanceItem{
		sessionKey:        sessionKey,
		roundID:           roundID,
		contextName:       contextName,
		content:           content,
		excludedAgentID:   excludedAgentID,
		objectiveRevision: objectiveRevision,
	})
	return []string{"round-running"}, nil
}

type fakeExternalMutationAccountant struct {
	service               *Service
	usage                 protocol.GoalUsage
	usageFlushed          bool
	roundID               string
	sessionKeys           []string
	settlementBoundaries  []bool
	clearedSessionKeys    []string
	finalizingSessionKeys []string
	activatedSessionKeys  []string
	createPreflightCalls  []string
	createConflicts       map[string][]string
	activateErr           error
	activationRollbacks   []string
}

func (a *fakeExternalMutationAccountant) FlushGoalAccounting(ctx context.Context, sessionKey string) ([]string, error) {
	a.sessionKeys = append(a.sessionKeys, sessionKey)
	a.settlementBoundaries = append(a.settlementBoundaries, RuntimeUsageSettlementBoundary(ctx))
	if a.service != nil && !a.usageFlushed {
		if _, err := a.service.RecordUsageForSession(ctx, sessionKey, a.usage, a.roundID); err != nil {
			return nil, err
		}
		a.usageFlushed = true
	}
	return []string{a.roundID}, nil
}

func (a *fakeExternalMutationAccountant) ClearGoalAccounting(sessionKey string) []string {
	a.clearedSessionKeys = append(a.clearedSessionKeys, sessionKey)
	return []string{a.roundID}
}

func (a *fakeExternalMutationAccountant) BeginGoalAccountingFinalizing(sessionKey string) []string {
	if strings.TrimSpace(a.roundID) == "" {
		return nil
	}
	a.finalizingSessionKeys = append(a.finalizingSessionKeys, sessionKey)
	return []string{a.roundID}
}

func (a *fakeExternalMutationAccountant) ActivateGoalAccounting(_ context.Context, sessionKey string, goalID string) ([]string, error) {
	a.activatedSessionKeys = append(a.activatedSessionKeys, sessionKey+":"+goalID)
	return []string{a.roundID}, a.activateErr
}

func (a *fakeExternalMutationAccountant) GoalAccountingCreateConflicts(sessionKey string, scopeRoundID string) []string {
	a.createPreflightCalls = append(a.createPreflightCalls, sessionKey+":"+scopeRoundID)
	return append([]string(nil), a.createConflicts[scopeRoundID]...)
}

func (a *fakeExternalMutationAccountant) ClearGoalAccountingRounds(sessionKey string, roundIDs []string) []string {
	for _, roundID := range roundIDs {
		a.activationRollbacks = append(a.activationRollbacks, sessionKey+":"+roundID)
	}
	return append([]string(nil), roundIDs...)
}

func (a *fakeExternalMutationAccountant) GoalAccountingRoundIDs(_ string, _ string) []string {
	if strings.TrimSpace(a.roundID) == "" {
		return nil
	}
	return []string{a.roundID}
}

type fakeRuntimeInterrupter struct {
	sessionKeys []string
	roundIDs    [][]string
}

func (i *fakeRuntimeInterrupter) InterruptGoalRuntime(_ context.Context, sessionKey string, roundIDs []string) error {
	i.sessionKeys = append(i.sessionKeys, sessionKey)
	i.roundIDs = append(i.roundIDs, append([]string(nil), roundIDs...))
	return nil
}

type fakeContinuationDispatcher struct {
	deferSessions   map[string]bool
	missingSessions map[string]bool
	plans           []protocol.GoalContinuation
	dispatchErr     error
	deferCalls      int
	onShouldDefer   func(call int, sessionKey string)
	onDispatch      func(protocol.GoalContinuation) error
}

func (d *fakeContinuationDispatcher) ShouldDeferGoalContinuation(_ context.Context, sessionKey string) bool {
	d.deferCalls++
	if d.onShouldDefer != nil {
		d.onShouldDefer(d.deferCalls, sessionKey)
	}
	return d.deferSessions[sessionKey]
}

func (d *fakeContinuationDispatcher) GoalContinuationTargetMissing(_ context.Context, sessionKey string) (bool, error) {
	return d.missingSessions[sessionKey], nil
}

func (d *fakeContinuationDispatcher) DispatchGoalContinuation(_ context.Context, plan protocol.GoalContinuation) error {
	d.plans = append(d.plans, plan)
	if d.onDispatch != nil {
		return d.onDispatch(plan)
	}
	return d.dispatchErr
}

type fakeRoomGoalCompletionReadiness struct {
	blocker   string
	err       error
	goalID    string
	agentID   string
	roundID   string
	callCount int
}

type fakeExecutionGoalCompletionReadiness struct {
	blocker   string
	err       error
	goalID    string
	callCount int
}

func (f *fakeExecutionGoalCompletionReadiness) ExecutionGoalCompletionBlocker(
	_ context.Context,
	item protocol.Goal,
) (string, error) {
	f.callCount++
	f.goalID = item.ID
	return f.blocker, f.err
}

func (f *fakeRoomGoalCompletionReadiness) RoomGoalCompletionReport(
	_ context.Context,
	item protocol.Goal,
	agentID string,
	roundID string,
) (RoomGoalCompletionReport, error) {
	f.callCount++
	f.goalID = item.ID
	f.agentID = agentID
	f.roundID = roundID
	return RoomGoalCompletionReport{Blocker: f.blocker}, f.err
}

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

func TestServicePlanContinuationKeepsLegacyExternalGoalStandalone(t *testing.T) {
	for _, sessionKey := range []string{
		"agent:nexus:ws:dm:legacy-reservation",
		"room:group:legacy-reservation",
	} {
		t.Run(sessionKey, func(t *testing.T) {
			repo := newMemoryRepository()
			legacy := protocol.Goal{
				ID:         "goal_legacy_external",
				SessionKey: sessionKey,
				Objective:  "Resume an existing Goal",
				Status:     protocol.GoalStatusActive,
				Version:    1,
			}
			repo.goals[legacy.ID] = legacy
			service := NewService(config.Config{
				GoalEnabled:             true,
				GoalAutoContinueEnabled: true,
			}, repo)
			service.nowFn = fixedClock()
			service.idFactory = sequentialID()

			plan, err := service.PlanContinuationForSession(context.Background(), legacy.SessionKey, "")
			if err != nil {
				t.Fatal(err)
			}
			if plan == nil {
				t.Fatal("plan = nil, want repaired continuation")
			}
			if got := protocol.GoalReservedExecutionID(plan.Goal); got != "" {
				t.Fatalf("plan execution = %q, want standalone Goal", got)
			}
			if got := protocol.GoalExecutionBindingStateFromGoal(plan.Goal); got !=
				protocol.GoalExecutionBindingStateStandalone {
				t.Fatalf("plan binding state = %q, want standalone", got)
			}
			stored, err := repo.GetGoal(context.Background(), legacy.ID)
			if err != nil {
				t.Fatal(err)
			}
			if stored == nil || protocol.GoalReservedExecutionID(*stored) != "" {
				t.Fatalf("stored = %#v, want no implicit execution reservation", stored)
			}
			if got := protocol.GoalExecutionBindingStateFromGoal(*stored); got !=
				protocol.GoalExecutionBindingStateStandalone {
				t.Fatalf("stored binding state = %q, want standalone", got)
			}
		})
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
	current, err := service.CurrentOptional(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current != nil {
		t.Fatalf("current = %#v, want nil after complete", current)
	}
	stored, err := repo.GetGoal(ctx, completed.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored == nil || stored.Status != protocol.GoalStatusComplete {
		t.Fatalf("stored = %#v, want completed history retained", stored)
	}
}

func TestServiceEditCompletedGoalDoesNotReactivateCurrentGoal(t *testing.T) {
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
	if _, err = service.Update(ctx, completed.ID, protocol.UpdateGoalRequest{
		Objective: &updatedObjective,
	}); !errors.Is(err, ErrGoalInvalidState) {
		t.Fatalf("update completed goal error = %v, want ErrGoalInvalidState", err)
	}
	current, err := service.CurrentOptional(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current != nil {
		t.Fatalf("current = %#v, want nil after completed goal update attempt", current)
	}
}

func TestServiceCompleteByModelKeepsRoomGoalActiveWhileRoomWorkIsOutstanding(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	readiness := &fakeRoomGoalCompletionReadiness{
		blocker: "agent agent-peer still has an active Room slot",
	}
	service.SetRoomGoalCompletionReadiness(readiness)
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: protocol.BuildRoomSharedSessionKey("conversation-readiness"),
		Objective:  "wait for all Room work",
		CreatedBy:  "model",
		AgentID:    "agent-lead",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.CompleteByModel(ctx, created.ID, protocol.CompleteGoalRequest{
		AgentID: "agent-lead",
		RoundID: "round-lead",
	})
	if !errors.Is(err, ErrGoalInvalidState) || !strings.Contains(err.Error(), readiness.blocker) {
		t.Fatalf("CompleteByModel error = %v, want outstanding Room work rejection", err)
	}
	current, currentErr := service.Current(ctx, created.SessionKey)
	if currentErr != nil {
		t.Fatal(currentErr)
	}
	if current.Status != protocol.GoalStatusActive {
		t.Fatalf("status = %q, want active after readiness rejection", current.Status)
	}
	if readiness.callCount != 1 || readiness.goalID != created.ID || readiness.agentID != "agent-lead" || readiness.roundID != "round-lead" {
		t.Fatalf("readiness call = count:%d goal:%q agent:%q round:%q", readiness.callCount, readiness.goalID, readiness.agentID, readiness.roundID)
	}

	readiness.blocker = ""
	completed, err := service.CompleteByModel(ctx, created.ID, protocol.CompleteGoalRequest{
		AgentID: "agent-lead",
		RoundID: "round-lead-final",
	})
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != protocol.GoalStatusComplete || readiness.callCount != 2 {
		t.Fatalf("completed = %#v calls=%d, want complete after Room work drains", completed, readiness.callCount)
	}
}

func TestRoomLeadCompletionKeepsWorkGraphReadinessIndependentFromCollaborationEvidence(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	executionReadiness := &fakeExecutionGoalCompletionReadiness{
		blocker: "work_item:W2:required_not_accepted",
	}
	service.SetExecutionGoalCompletionReadiness(executionReadiness)
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: protocol.BuildRoomSharedSessionKey("room-workgraph-readiness"),
		Objective:  "complete the managed Room delivery",
		CreatedBy:  "model",
		AgentID:    "agent-lead",
		Metadata: map[string]any{
			protocol.GoalMetadataExecutionID: "execution-room-readiness",
			protocol.GoalMetadataExecutionBindingState: string(
				protocol.GoalExecutionBindingStateConfirmed,
			),
			protocol.GoalMetadataCompletionCriteria: []string{
				"all required Work Items accepted",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	const roundID = "round-room-final"
	_, err = service.AuditObjectiveAlignmentByModel(ctx, created.ID, protocol.AuditGoalObjectiveAlignmentRequest{
		Report: protocol.ObjectiveAlignmentReport{
			Decision: protocol.ObjectiveAlignmentAligned,
			CriteriaResults: []protocol.ObjectiveAlignmentCriterionResult{{
				Criterion: "all required Work Items accepted",
				Status:    protocol.ObjectiveAlignmentCriterionSatisfied,
				Evidence: []protocol.ObjectiveAlignmentEvidence{{
					Ref:   "workgraph:execution-room-readiness",
					Claim: "the current round inspected the managed WorkGraph",
				}},
			}},
			Summary: "The objective is aligned; WorkGraph readiness remains authoritative.",
		},
		RoundID:                   roundID,
		AgentID:                   "agent-lead",
		ExpectedObjectiveRevision: created.ObjectiveRevision(),
	})
	if err != nil {
		t.Fatal(err)
	}
	completion := protocol.CompleteGoalRequest{
		RoundID:                   roundID,
		AgentID:                   "agent-lead",
		ExpectedObjectiveRevision: created.ObjectiveRevision(),
	}
	if _, err = service.CompleteByModel(ctx, created.ID, completion); !errors.Is(err, ErrGoalInvalidState) ||
		!errors.Is(err, ErrGoalExecutionNotReady) ||
		!strings.Contains(err.Error(), executionReadiness.blocker) {
		t.Fatalf("CompleteByModel error = %v, want WorkGraph readiness rejection", err)
	}
	current, err := service.Current(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current.Status != protocol.GoalStatusActive || RoomCollaborationObserved(*current) {
		t.Fatalf("current = %#v, want active Goal without manufactured collaboration evidence", current)
	}

	executionReadiness.blocker = ""
	completed, err := service.CompleteByModel(ctx, created.ID, completion)
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != protocol.GoalStatusComplete || RoomCollaborationObserved(*completed) {
		t.Fatalf("completed = %#v, want lead completion after WorkGraph readiness without collaboration gate", completed)
	}
}

func TestServiceCompleteByModelRetriesConcurrentGoalVersion(t *testing.T) {
	repo := &staleOnceUsageRepository{
		memoryRepository: newMemoryRepository(),
		concurrentUsage:  protocol.GoalUsage{TotalTokens: 7},
	}
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()
	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: protocol.BuildRoomSharedSessionKey("complete-version-race"),
		Objective:  "Complete after concurrent usage",
		CreatedBy:  "model",
		AgentID:    "agent-lead",
	})
	if err != nil {
		t.Fatal(err)
	}
	repo.staleGoalID = created.ID
	completed, err := service.CompleteByModel(ctx, created.ID, protocol.CompleteGoalRequest{AgentID: "agent-lead"})
	if err != nil {
		t.Fatal(err)
	}
	if !repo.injected || completed.Status != protocol.GoalStatusComplete || completed.Usage.Total() != 7 {
		t.Fatalf("completed = %#v injected=%v, want retried terminal mutation", completed, repo.injected)
	}
}

func TestServiceBlockByModelRequiresDurableRecoveryReason(t *testing.T) {
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
	if blocked, err := service.BlockByModel(ctx, created.ID, protocol.BlockGoalRequest{}); !errors.Is(err, ErrGoalInvalidInput) || blocked != nil {
		t.Fatalf("empty blocker = %#v, %v; want invalid input", blocked, err)
	}
	blocked, err := service.BlockByModel(ctx, created.ID, protocol.BlockGoalRequest{
		BlockerID:   "source-system-unavailable",
		Reason:      "source system is unavailable",
		NeededInput: "restore source-system access",
	})
	if err != nil {
		t.Fatal(err)
	}
	if blocked.Status != protocol.GoalStatusBlocked || blocked.BlockedAt == nil {
		t.Fatalf("blocked = %#v, want blocked status", blocked)
	}
	if len(repo.events) != 2 || repo.events[1].EventType != "blocked" {
		t.Fatalf("events = %#v, want blocked event", repo.events)
	}
	if repo.events[1].Payload["blocker_id"] != "source-system-unavailable" ||
		repo.events[1].Payload["reason"] != "source system is unavailable" ||
		repo.events[1].Payload["needed_input"] != "restore source-system access" {
		t.Fatalf("blocked payload = %#v, want durable recovery path", repo.events[1].Payload)
	}
	blocker, ok := protocol.GoalBlockerFromGoal(*blocked)
	if !ok || blocker.ID != "source-system-unavailable" ||
		blocker.Reason != "source system is unavailable" ||
		blocker.NeededInput != "restore source-system access" ||
		blocker.SinceObjectiveRevision != blocked.ObjectiveRevision() {
		t.Fatalf("blocked projection = %#v, ok=%v", blocker, ok)
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

type concurrentGoalCreateRepository struct {
	*memoryRepository
	winner protocol.Goal
}

func (r *concurrentGoalCreateRepository) CreateGoalWithEvent(
	_ context.Context,
	_ protocol.Goal,
	_ protocol.GoalEvent,
) (*protocol.Goal, error) {
	r.goals[r.winner.ID] = r.winner
	return nil, errors.New("current Goal unique constraint")
}

func TestServiceCreatePreflightUsesWholeSessionForExternalGoal(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(testConfig(), repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	accountant := &fakeExternalMutationAccountant{
		createConflicts: map[string][]string{
			"": {"round-live"},
		},
	}
	service.SetExternalMutationAccountant(accountant)

	_, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:create-preflight-external",
		Objective:  "External Goal starts from now",
		CreatedBy:  "user",
		RoundID:    "ignored-for-external",
	})
	if !errors.Is(err, ErrGoalConflict) {
		t.Fatalf("Create() error = %v, want ErrGoalConflict", err)
	}
	if len(accountant.createPreflightCalls) != 1 ||
		accountant.createPreflightCalls[0] != "agent:nexus:ws:dm:create-preflight-external:" {
		t.Fatalf("preflight calls = %#v, want whole-session check", accountant.createPreflightCalls)
	}
	if len(repo.goals) != 0 {
		t.Fatalf("goals = %#v, want no persistence before conflict", repo.goals)
	}
}

func TestServiceCreateClassifiesConcurrentCurrentGoalInsertAsConflict(t *testing.T) {
	sessionKey := "agent:nexus:ws:dm:create-concurrent-conflict"
	repo := &concurrentGoalCreateRepository{
		memoryRepository: newMemoryRepository(),
		winner: protocol.Goal{
			ID:         "goal-concurrent-winner",
			SessionKey: sessionKey,
			Objective:  "the concurrent winner",
			Status:     protocol.GoalStatusActive,
			Version:    1,
		},
	}
	service := NewService(testConfig(), repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()

	_, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey: sessionKey,
		Objective:  "the losing concurrent request",
		CreatedBy:  "user",
	})
	if !errors.Is(err, ErrGoalConflict) {
		t.Fatalf("Create() error = %v, want ErrGoalConflict", err)
	}
	if len(repo.events) != 0 {
		t.Fatalf("losing create events = %#v, want none", repo.events)
	}
}

func TestServiceCreatePropagatesActivationError(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(testConfig(), repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	activationErr := errors.New("bind runtime Goal scope")
	accountant := &fakeExternalMutationAccountant{
		roundID:     "round-activated-before-error",
		activateErr: activationErr,
	}
	service.SetExternalMutationAccountant(accountant)
	broadcaster := &fakeGoalBroadcaster{}
	service.SetEventBroadcaster(broadcaster)

	_, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:create-activation-error",
		Objective:  "Activation errors are visible",
		CreatedBy:  "user",
	})
	if !errors.Is(err, activationErr) {
		t.Fatalf("Create() error = %v, want activation error", err)
	}
	if len(accountant.activatedSessionKeys) != 1 {
		t.Fatalf("activation calls = %#v, want one", accountant.activatedSessionKeys)
	}
	if len(accountant.activationRollbacks) != 1 ||
		accountant.activationRollbacks[0] != "agent:nexus:ws:dm:create-activation-error:round-activated-before-error" {
		t.Fatalf("activation rollbacks = %#v, want successful runtime rounds cleared", accountant.activationRollbacks)
	}
	if len(repo.goals) != 0 || len(repo.events) != 0 {
		t.Fatalf("failed activation left goals=%d events=%d, want rollback", len(repo.goals), len(repo.events))
	}
	if len(broadcaster.events) != 0 {
		t.Fatalf("broadcast events = %#v, failed creation must stay invisible", broadcaster.events)
	}
}

func TestServiceThreadGoalSetUsesCreatePreflight(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(testConfig(), repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	accountant := &fakeExternalMutationAccountant{
		createConflicts: map[string][]string{
			"": {"room-slot-live"},
		},
	}
	service.SetExternalMutationAccountant(accountant)
	objective := "App-server must share the external creation fence"

	_, err := service.SetFromThreadGoalParams(context.Background(), goalappserver.ThreadGoalSetParams{
		ThreadID:  "agent:nexus:ws:dm:thread-goal-create-preflight",
		Objective: &objective,
	})
	if !errors.Is(err, ErrGoalConflict) {
		t.Fatalf("SetFromThreadGoalParams() error = %v, want ErrGoalConflict", err)
	}
	if len(repo.goals) != 0 || len(repo.events) != 0 {
		t.Fatalf("preflight persisted goals=%d events=%d, want zero", len(repo.goals), len(repo.events))
	}
}

func TestServiceThreadGoalSetRollsBackActivationFailure(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(testConfig(), repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	activationErr := errors.New("thread Goal runtime bind conflict")
	accountant := &fakeExternalMutationAccountant{activateErr: activationErr}
	service.SetExternalMutationAccountant(accountant)
	broadcaster := &fakeGoalBroadcaster{}
	service.SetEventBroadcaster(broadcaster)
	objective := "App-server activation must be atomic to callers"

	_, err := service.SetFromThreadGoalParams(context.Background(), goalappserver.ThreadGoalSetParams{
		ThreadID:  "agent:nexus:ws:dm:thread-goal-activation-error",
		Objective: &objective,
	})
	if !errors.Is(err, activationErr) {
		t.Fatalf("SetFromThreadGoalParams() error = %v, want activation error", err)
	}
	if len(repo.goals) != 0 || len(repo.events) != 0 || len(broadcaster.events) != 0 {
		t.Fatalf(
			"failed activation left goals=%d events=%d broadcasts=%d, want invisible rollback",
			len(repo.goals),
			len(repo.events),
			len(broadcaster.events),
		)
	}
}

func TestServiceDeleteGoalsForAgentRemovesAllAgentSessions(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	agentDM := createCleanupGoal(t, service, ctx, "agent:agent-a:ws:dm:chat-1")
	agentGroupTopic := createCleanupGoal(t, service, ctx, "agent:agent-a:ws:group:conversation-1:topic:thread-1")
	otherAgent := createCleanupGoal(t, service, ctx, "agent:agent-b:ws:dm:chat-1")
	roomGoal := createCleanupGoal(t, service, ctx, "room:group:conversation-1")

	deleted, err := service.DeleteGoalsForAgent(ctx, "agent-a")
	if err != nil {
		t.Fatalf("DeleteGoalsForAgent() error = %v", err)
	}
	if deleted != 2 {
		t.Fatalf("deleted = %d, want 2", deleted)
	}
	assertGoalDeleted(t, repo, agentDM.ID)
	assertGoalDeleted(t, repo, agentGroupTopic.ID)
	assertGoalExists(t, repo, otherAgent.ID)
	assertGoalExists(t, repo, roomGoal.ID)
}

func TestServiceDeleteGoalsForRoomConversationsRemovesSharedAndMemberGoals(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	shared := createCleanupGoal(t, service, ctx, "room:group:conversation-1")
	groupMember := createCleanupGoal(t, service, ctx, "agent:agent-a:ws:group:conversation-1")
	dmMember := createCleanupGoal(t, service, ctx, "agent:agent-a:ws:dm:conversation-1")
	otherConversation := createCleanupGoal(t, service, ctx, "agent:agent-a:ws:group:conversation-2")

	deleted, err := service.DeleteGoalsForRoomConversations(ctx, []string{"conversation-1"})
	if err != nil {
		t.Fatalf("DeleteGoalsForRoomConversations() error = %v", err)
	}
	if deleted != 3 {
		t.Fatalf("deleted = %d, want 3", deleted)
	}
	assertGoalDeleted(t, repo, shared.ID)
	assertGoalDeleted(t, repo, groupMember.ID)
	assertGoalDeleted(t, repo, dmMember.ID)
	assertGoalExists(t, repo, otherConversation.ID)
}

func TestServiceDeleteGoalsForRoomMemberOnlyRemovesThatMember(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	removedMember := createCleanupGoal(t, service, ctx, "agent:agent-a:ws:group:conversation-1")
	remainingMember := createCleanupGoal(t, service, ctx, "agent:agent-b:ws:group:conversation-1")
	shared := createCleanupGoal(t, service, ctx, "room:group:conversation-1")

	deleted, err := service.DeleteGoalsForRoomMember(ctx, "agent-a", []string{"conversation-1"})
	if err != nil {
		t.Fatalf("DeleteGoalsForRoomMember() error = %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}
	assertGoalDeleted(t, repo, removedMember.ID)
	assertGoalExists(t, repo, remainingMember.ID)
	assertGoalExists(t, repo, shared.ID)
}

func createCleanupGoal(t *testing.T, service *Service, ctx context.Context, sessionKey string) *protocol.Goal {
	t.Helper()
	item, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: sessionKey,
		Objective:  "cleanup " + sessionKey,
	})
	if err != nil {
		t.Fatalf("Create(%s) error = %v", sessionKey, err)
	}
	return item
}

func assertGoalDeleted(t *testing.T, repo *memoryRepository, goalID string) {
	t.Helper()
	if _, ok := repo.goals[goalID]; ok {
		t.Fatalf("goal %s still exists", goalID)
	}
}

func assertGoalExists(t *testing.T, repo *memoryRepository, goalID string) {
	t.Helper()
	if _, ok := repo.goals[goalID]; !ok {
		t.Fatalf("goal %s missing", goalID)
	}
}

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

func TestServiceRetargetByModelRetriesVersionStaleAndPreservesConcurrentUsage(t *testing.T) {
	base := newMemoryRepository()
	repo := &staleOnceUsageRepository{
		memoryRepository: base,
		concurrentUsage: protocol.GoalUsage{
			InputTokens:    7,
			OutputTokens:   3,
			RuntimeSeconds: 2,
		},
	}
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()
	if _, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: protocol.BuildRoomSharedSessionKey("missing-model-agent"),
		Objective:  "Ownerless model Goal",
		CreatedBy:  "model",
	}); !errors.Is(err, ErrGoalInvalidInput) {
		t.Fatalf("ownerless model create error = %v, want ErrGoalInvalidInput", err)
	}

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Analyze M3 and M4",
	})
	if err != nil {
		t.Fatal(err)
	}
	repo.staleGoalID = created.ID

	updated, err := service.RetargetByModel(ctx, created.SessionKey, protocol.RetargetGoalRequest{
		Objective:                 "Analyze M4 and M5",
		ExpectedObjectiveRevision: created.ObjectiveRevision(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !repo.injected || updated.ID != created.ID || updated.Objective != "Analyze M4 and M5" {
		t.Fatalf("updated = %#v injected=%v, want retried same Goal", updated, repo.injected)
	}
	if updated.Usage.InputTokens != 7 || updated.Usage.OutputTokens != 3 || updated.Usage.RuntimeSeconds != 2 {
		t.Fatalf("usage = %#v, want concurrent usage preserved", updated.Usage)
	}
	if updated.ObjectiveRevision() != created.ObjectiveRevision()+1 {
		t.Fatalf("objective revision = %d, want %d", updated.ObjectiveRevision(), created.ObjectiveRevision()+1)
	}
}

func TestServiceRetargetByModelRejectsOlderConcurrentObjective(t *testing.T) {
	base := newMemoryRepository()
	repo := &concurrentRetargetRepository{
		memoryRepository: base,
		blockedObjective: "Analyze M4 and M5",
		updateStarted:    make(chan struct{}),
		releaseUpdate:    make(chan struct{}),
	}
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Analyze M3 and M4",
	})
	if err != nil {
		t.Fatal(err)
	}
	expectedRevision := created.ObjectiveRevision()

	type retargetResult struct {
		goal *protocol.Goal
		err  error
	}
	olderResult := make(chan retargetResult, 1)
	go func() {
		goal, retargetErr := service.RetargetByModel(ctx, created.SessionKey, protocol.RetargetGoalRequest{
			Objective:                 "Analyze M4 and M5",
			RoundID:                   "round-older",
			ExpectedObjectiveRevision: expectedRevision,
		})
		olderResult <- retargetResult{goal: goal, err: retargetErr}
	}()

	<-repo.updateStarted
	newer, newerErr := service.RetargetByModel(ctx, created.SessionKey, protocol.RetargetGoalRequest{
		Objective:                 "Analyze M5 and M6",
		RoundID:                   "round-newer",
		ExpectedObjectiveRevision: expectedRevision,
	})
	close(repo.releaseUpdate)
	older := <-olderResult

	if newerErr != nil {
		t.Fatalf("newer retarget error = %v", newerErr)
	}
	if !errors.Is(older.err, ErrGoalRevisionStale) || older.goal != nil {
		t.Fatalf("older retarget = goal:%#v err:%v, want ErrGoalRevisionStale", older.goal, older.err)
	}
	current, err := service.Current(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current.Objective != "Analyze M5 and M6" || current.ObjectiveRevision() != expectedRevision+1 {
		t.Fatalf("current = %#v, want only newer objective at revision %d", current, expectedRevision+1)
	}
	if newer.Objective != current.Objective || newer.ObjectiveRevision() != current.ObjectiveRevision() {
		t.Fatalf("newer = %#v current = %#v, want same committed retarget", newer, current)
	}
	if len(repo.events) != 2 || repo.events[1].RoundID != "round-newer" {
		t.Fatalf("events = %#v, want only newer retarget audit", repo.events)
	}
}

type concurrentRetargetRepository struct {
	*memoryRepository
	mu               sync.Mutex
	blockOnce        sync.Once
	blockedObjective string
	updateStarted    chan struct{}
	releaseUpdate    chan struct{}
}

func (r *concurrentRetargetRepository) GetGoal(ctx context.Context, goalID string) (*protocol.Goal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.memoryRepository.GetGoal(ctx, goalID)
}

func (r *concurrentRetargetRepository) GetCurrentGoal(ctx context.Context, sessionKey string) (*protocol.Goal, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.memoryRepository.GetCurrentGoal(ctx, sessionKey)
}

func (r *concurrentRetargetRepository) UpdateGoal(ctx context.Context, item protocol.Goal, expectedVersion int64) (*protocol.Goal, error) {
	if item.Objective == r.blockedObjective {
		r.blockOnce.Do(func() {
			close(r.updateStarted)
			<-r.releaseUpdate
		})
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.memoryRepository.UpdateGoal(ctx, item, expectedVersion)
}

func (r *concurrentRetargetRepository) UpdateGoalWithEvents(
	ctx context.Context,
	item protocol.Goal,
	expectedVersion int64,
	events []protocol.GoalEvent,
) (*protocol.Goal, error) {
	if item.Objective == r.blockedObjective {
		r.blockOnce.Do(func() {
			close(r.updateStarted)
			<-r.releaseUpdate
		})
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.memoryRepository.UpdateGoalWithEvents(ctx, item, expectedVersion, events)
}

func (r *concurrentRetargetRepository) AppendEvent(ctx context.Context, event protocol.GoalEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.memoryRepository.AppendEvent(ctx, event)
}

func TestServiceRetargetByModelPreservesGoalLifecycleCollaboration(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	dispatcher := &fakeGuidanceDispatcher{}
	service.SetGuidanceDispatcher(dispatcher)
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: protocol.BuildRoomSharedSessionKey("conversation-1"),
		Objective:  "Analyze M3 and M4",
		AgentID:    "agent-lead",
		Metadata: map[string]any{
			protocol.GoalMetadataRoomGoalLeadAgentID:                   "agent-lead",
			protocol.GoalMetadataRoomGoalCollaborationRequired:         true,
			protocol.GoalMetadataRoomGoalCollaborationObserved:         true,
			protocol.GoalMetadataRoomGoalCollaborationAgentID:          "agent-peer",
			protocol.GoalMetadataRoomGoalCollaborationRoundID:          "round-peer-old",
			protocol.GoalMetadataRoomGoalCollaborationObservedAt:       "2026-07-13T10:00:00Z",
			protocol.GoalMetadataRoomGoalCollaborationRequirementRound: "round-required",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := service.RetargetByModel(ctx, created.SessionKey, protocol.RetargetGoalRequest{
		Objective: "Analyze M4 and M5",
		RoundID:   "round-correction",
		AgentID:   "agent-lead",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !RoomCollaborationObserved(*updated) {
		t.Fatalf("collaboration metadata = %#v, want Goal-lifecycle evidence retained", updated.Metadata)
	}
	if len(dispatcher.items) != 1 || dispatcher.items[0].excludedAgentID != "agent-lead" || dispatcher.items[0].contextName != "goal" || dispatcher.items[0].objectiveRevision != updated.ObjectiveRevision() {
		t.Fatalf("guidance = %#v, want Room retarget propagated except caller", dispatcher.items)
	}
	for key, want := range map[string]string{
		protocol.GoalMetadataRoomGoalCollaborationAgentID:    "agent-peer",
		protocol.GoalMetadataRoomGoalCollaborationRoundID:    "round-peer-old",
		protocol.GoalMetadataRoomGoalCollaborationObservedAt: "2026-07-13T10:00:00Z",
	} {
		if got := protocol.GoalMetadataString(updated.Metadata, key); got != want {
			t.Fatalf("collaboration metadata %q = %q, want %q", key, got, want)
		}
	}
	if _, err := service.RecordRoomGoalCollaborationEvidence(ctx, created.ID, "round-peer-stale", "agent-peer", created.ObjectiveRevision()); !errors.Is(err, ErrGoalRevisionStale) {
		t.Fatalf("stale collaboration error = %v, want ErrGoalRevisionStale", err)
	}
	if _, err := service.CompleteByModel(ctx, created.ID, protocol.CompleteGoalRequest{
		AgentID:                   "agent-lead",
		ExpectedObjectiveRevision: created.ObjectiveRevision(),
	}); !errors.Is(err, ErrGoalRevisionStale) {
		t.Fatalf("stale completion error = %v, want ErrGoalRevisionStale", err)
	}
	if _, err := service.CompleteByModel(ctx, created.ID, protocol.CompleteGoalRequest{
		AgentID:                   "agent-lead",
		ExpectedObjectiveRevision: updated.ObjectiveRevision(),
	}); err != nil {
		t.Fatalf("completion with earlier Goal collaboration error = %v", err)
	}
}

func TestServiceRoomGoalModelMutationsRequireLeadAgent(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: protocol.BuildRoomSharedSessionKey("ownership"),
		Objective:  "Original objective",
		CreatedBy:  "model",
		AgentID:    "agent-lead",
		Metadata: map[string]any{
			protocol.GoalMetadataRoomGoalLeadAgentID:   "agent-spoof",
			protocol.GoalMetadataRoomGoalLeadAgentName: "Spoof",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := protocol.GoalMetadataString(created.Metadata, protocol.GoalMetadataRoomGoalCreatorAgentID); got != "agent-lead" {
		t.Fatalf("creator agent = %q, want agent-lead", got)
	}
	if got := RoomLeadAgentID(*created); got != "agent-lead" {
		t.Fatalf("lead agent = %q, want agent-lead", got)
	}
	if got := RoomLeadAgentName(*created); got != "" {
		t.Fatalf("model-created lead name = %q, want directory reconciliation", got)
	}
	accountant := &fakeExternalMutationAccountant{service: service}
	service.SetExternalMutationAccountant(accountant)
	if _, err = service.RetargetByModel(ctx, created.SessionKey, protocol.RetargetGoalRequest{Objective: "Peer replacement", AgentID: "agent-peer"}); !errors.Is(err, ErrGoalForbidden) {
		t.Fatalf("peer retarget error = %v, want ErrGoalForbidden", err)
	}
	if _, err = service.BlockByModel(ctx, created.ID, protocol.BlockGoalRequest{
		BlockerID:   "waiting-owner-decision",
		Reason:      "waiting for a decision",
		NeededInput: "the owner decision",
		AgentID:     "agent-peer",
	}); !errors.Is(err, ErrGoalForbidden) {
		t.Fatalf("peer block error = %v, want ErrGoalForbidden", err)
	}
	if _, err = service.CompleteByModel(ctx, created.ID, protocol.CompleteGoalRequest{AgentID: "agent-peer"}); !errors.Is(err, ErrGoalForbidden) {
		t.Fatalf("peer complete error = %v, want ErrGoalForbidden", err)
	}
	if len(accountant.sessionKeys) != 0 {
		t.Fatalf("forbidden mutation flushed accounting for %v", accountant.sessionKeys)
	}
	updated, err := service.RetargetByModel(ctx, created.SessionKey, protocol.RetargetGoalRequest{Objective: "Lead replacement", AgentID: "agent-lead"})
	if err != nil || updated.Objective != "Lead replacement" {
		t.Fatalf("lead retarget = %#v err=%v", updated, err)
	}
	replaced, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey:      created.SessionKey,
		Objective:       "User replacement",
		ReplaceExisting: true,
	})
	if err != nil || RoomLeadAgentID(*replaced) != "agent-lead" || protocol.GoalMetadataString(replaced.Metadata, protocol.GoalMetadataRoomGoalCreatorAgentID) != "agent-lead" {
		t.Fatalf("user replacement ownership = %#v err=%v", replaced, err)
	}
	service.SetSessionOwnershipVerifier(staticGoalSessionOwnershipVerifier{
		trustedAgentID: "agent-peer", trustedAgentName: "Peer",
	})
	reassigned, err := service.SetRoomGoalLead(ctx, replaced.ID, "agent-peer")
	if err != nil || RoomLeadAgentID(*reassigned) != "agent-peer" || RoomLeadAgentName(*reassigned) != "Peer" || protocol.GoalMetadataString(reassigned.Metadata, protocol.GoalMetadataRoomGoalCreatorAgentID) != "agent-lead" {
		t.Fatalf("reassigned ownership = %#v err=%v", reassigned, err)
	}
	service.SetSessionOwnershipVerifier(nil)
	legacy, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: protocol.BuildRoomSharedSessionKey("legacy-ownership"),
		Objective:  "Legacy objective",
		Metadata: map[string]any{
			protocol.GoalMetadataRoomGoalCreatorAgentID: "agent-spoof",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := legacy.Metadata[protocol.GoalMetadataRoomGoalCreatorAgentID]; exists {
		t.Fatalf("user-created Goal accepted spoofed creator: %#v", legacy.Metadata)
	}
	if _, err = service.RetargetByModel(ctx, legacy.SessionKey, protocol.RetargetGoalRequest{Objective: "Claimed objective", AgentID: "agent-host"}); !errors.Is(err, ErrGoalForbidden) {
		t.Fatalf("ownerless legacy retarget error = %v, want ErrGoalForbidden", err)
	}
	service.SetSessionOwnershipVerifier(staticGoalSessionOwnershipVerifier{
		trustedAgentID: "agent-host", trustedAgentName: "Host",
	})
	if _, err = service.SetRoomGoalLead(ctx, legacy.ID, "agent-host"); err != nil {
		t.Fatal(err)
	}
	metadataUpdated, err := service.Update(ctx, legacy.ID, protocol.UpdateGoalRequest{Metadata: map[string]any{
		protocol.GoalMetadataRoomGoalCreatorAgentID: "agent-spoof",
		protocol.GoalMetadataRoomGoalLeadAgentID:    "agent-host",
		protocol.GoalMetadataRoomGoalLeadAgentName:  "Host",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := metadataUpdated.Metadata[protocol.GoalMetadataRoomGoalCreatorAgentID]; exists {
		t.Fatalf("metadata update injected creator: %#v", metadataUpdated.Metadata)
	}
	claimed, err := service.RetargetByModel(ctx, legacy.SessionKey, protocol.RetargetGoalRequest{Objective: "Claimed objective", AgentID: "agent-host"})
	if err != nil || RoomLeadAgentID(*claimed) != "agent-host" {
		t.Fatalf("reconciled legacy retarget = %#v err=%v, want host lead", claimed, err)
	}
}

func TestServiceRetargetByModelReplacesPausedGoalWithoutResume(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	if _, err := service.RetargetByModel(ctx, "agent:nexus:ws:dm:missing", protocol.RetargetGoalRequest{Objective: "New"}); !errors.Is(err, ErrGoalNotFound) {
		t.Fatalf("missing goal error = %v, want ErrGoalNotFound", err)
	}
	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Original",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Pause(ctx, created.ID); err != nil {
		t.Fatal(err)
	}
	retargeted, err := service.RetargetByModel(ctx, created.SessionKey, protocol.RetargetGoalRequest{Objective: "New"})
	if err != nil {
		t.Fatal(err)
	}
	if retargeted.ID != created.ID || retargeted.Objective != "New" || retargeted.Status != protocol.GoalStatusActive {
		t.Fatalf("retargeted = %#v, want same active Goal with replacement objective", retargeted)
	}
	if _, err := service.RetargetByModel(ctx, created.SessionKey, protocol.RetargetGoalRequest{}); !errors.Is(err, ErrGoalInvalidInput) {
		t.Fatalf("empty objective error = %v, want ErrGoalInvalidInput", err)
	}
}

func TestServiceRetargetByModelDoesNotBypassExhaustedBudget(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()
	budget := int64(10)

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey:  "agent:nexus:ws:dm:budget-limited-retarget",
		Objective:   "Original",
		TokenBudget: &budget,
	})
	if err != nil {
		t.Fatal(err)
	}
	limited, err := service.RecordUsageForSession(ctx, created.SessionKey, protocol.GoalUsage{TotalTokens: budget}, "round-budget")
	if err != nil {
		t.Fatal(err)
	}
	if limited.Status != protocol.GoalStatusBudgetLimited {
		t.Fatalf("status = %q, want budget_limited", limited.Status)
	}

	if _, err = service.RetargetByModel(ctx, created.SessionKey, protocol.RetargetGoalRequest{Objective: "Replacement"}); !errors.Is(err, ErrGoalInvalidState) {
		t.Fatalf("retarget error = %v, want ErrGoalInvalidState", err)
	}
	current, err := service.Current(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current.Objective != created.Objective || current.Status != protocol.GoalStatusBudgetLimited {
		t.Fatalf("current = %#v, want unchanged budget-limited Goal", current)
	}
}
