// lifecycle_test.go 覆盖 Goal 公共夹具、生命周期、预检、清理、恢复与重定目标测试。
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
