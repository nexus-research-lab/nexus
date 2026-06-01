package goal

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
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

type fakeRuntimeInterrupter struct {
	sessionKeys []string
}

func (i *fakeRuntimeInterrupter) InterruptGoalRuntime(_ context.Context, sessionKey string) error {
	i.sessionKeys = append(i.sessionKeys, sessionKey)
	return nil
}

type fakeContinuationDispatcher struct {
	deferSessions map[string]bool
	plans         []protocol.GoalContinuation
	dispatchErr   error
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
	return d.dispatchErr
}
