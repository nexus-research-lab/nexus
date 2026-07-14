// INPUT: active、paused 或缺失的当前 Goal，以及模型从用户纠正中整理出的 objective。
// OUTPUT: 验证同一 Goal 的安全重定向、审计来源、round 绑定和投影刷新。
// POS: RetargetByModel 的服务层回归测试。
package goal

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func TestServiceRetargetByModelPreservesGoalAndRefreshesProjection(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	dispatcher := &fakeGuidanceDispatcher{}
	service.SetGuidanceDispatcher(dispatcher)
	preview := &fakePreviewFiller{}
	service.SetPreviewFiller(preview)
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Analyze M3 and M4",
		CreatedBy:  "model",
	})
	if err != nil {
		t.Fatal(err)
	}
	stored := repo.goals[created.ID]
	stored.Usage = protocol.GoalUsage{InputTokens: 100, OutputTokens: 20, TotalTokens: 120}
	stored.TimeUsedSeconds = 12
	stored.ContinuationCount = 2
	stored.EmptyProgressCount = 1
	repo.goals[created.ID] = stored
	preview.items = nil
	preview.titleSchedules = nil

	updated, err := service.RetargetByModel(ctx, created.SessionKey, protocol.RetargetGoalRequest{
		Objective: "Analyze M4 and M5",
		RoundID:   "round-correction",
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != created.ID || updated.Objective != "Analyze M4 and M5" || updated.Status != protocol.GoalStatusActive {
		t.Fatalf("updated = %#v, want same active goal with corrected objective", updated)
	}
	if updated.Usage != stored.Usage || updated.TimeUsedSeconds != stored.TimeUsedSeconds {
		t.Fatalf("usage = %#v/%d, want preserved %#v/%d", updated.Usage, updated.TimeUsedSeconds, stored.Usage, stored.TimeUsedSeconds)
	}
	if updated.ContinuationCount != 0 || updated.EmptyProgressCount != 0 {
		t.Fatalf("continuation counters = %d/%d, want reset", updated.ContinuationCount, updated.EmptyProgressCount)
	}
	if len(repo.events) != 2 {
		t.Fatalf("events = %#v, want created + updated", repo.events)
	}
	event := repo.events[1]
	if event.EventType != "updated" || event.Source != protocol.GoalUpdateSourceModel || event.RoundID != "round-correction" || !eventPayloadBool(event.Payload, "objective_updated") {
		t.Fatalf("event = %#v, want model updated/objective_updated audit", event)
	}
	if len(dispatcher.items) != 0 {
		t.Fatalf("guidance = %#v, want model retarget tool result to carry the correction", dispatcher.items)
	}
	if len(preview.items) != 1 || preview.items[0].title != "Analyze M4 and M5" || len(preview.titleSchedules) != 1 {
		t.Fatalf("preview = %#v schedules=%#v, want corrected objective projection", preview.items, preview.titleSchedules)
	}
	progressed, err := service.RecordContinuationProgress(ctx, created.ID, "round-correction", true)
	if err != nil {
		t.Fatal(err)
	}
	if progressed.EmptyProgressCount != 0 || progressed.ContinuationCount != 0 {
		t.Fatalf("continuation counters after retarget progress = %d/%d, want reset", progressed.EmptyProgressCount, progressed.ContinuationCount)
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

func (r *concurrentRetargetRepository) AppendEvent(ctx context.Context, event protocol.GoalEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.memoryRepository.AppendEvent(ctx, event)
}

func TestServiceRetargetByModelRequiresFreshRoomCollaboration(t *testing.T) {
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
		AgentID:   "agent-peer",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !RoomCollaborationRequired(*updated) || RoomCollaborationObserved(*updated) {
		t.Fatalf("collaboration metadata = %#v, want required without stale evidence", updated.Metadata)
	}
	if len(dispatcher.items) != 1 || dispatcher.items[0].excludedAgentID != "agent-peer" || dispatcher.items[0].contextName != "goal" || dispatcher.items[0].objectiveRevision != updated.ObjectiveRevision() {
		t.Fatalf("guidance = %#v, want Room retarget propagated except caller", dispatcher.items)
	}
	for _, key := range []string{
		protocol.GoalMetadataRoomGoalCollaborationObserved,
		protocol.GoalMetadataRoomGoalCollaborationAgentID,
		protocol.GoalMetadataRoomGoalCollaborationRoundID,
		protocol.GoalMetadataRoomGoalCollaborationObservedAt,
	} {
		if _, ok := updated.Metadata[key]; ok {
			t.Fatalf("collaboration metadata retained stale %q: %#v", key, updated.Metadata)
		}
	}
	if _, err := service.CompleteByModel(ctx, created.ID, protocol.CompleteGoalRequest{}); !errors.Is(err, ErrGoalInvalidState) {
		t.Fatalf("completion without fresh collaboration error = %v, want ErrGoalInvalidState", err)
	}
	if _, err := service.RecordRoomGoalCollaborationEvidence(ctx, created.ID, "round-peer-stale", "agent-peer", created.ObjectiveRevision()); !errors.Is(err, ErrGoalRevisionStale) {
		t.Fatalf("stale collaboration error = %v, want ErrGoalRevisionStale", err)
	}
	if _, err := service.RecordRoomGoalCollaborationEvidence(ctx, created.ID, "round-peer-new", "agent-peer", updated.ObjectiveRevision()); err != nil {
		t.Fatal(err)
	}
	if _, err := service.CompleteByModel(ctx, created.ID, protocol.CompleteGoalRequest{ExpectedObjectiveRevision: created.ObjectiveRevision()}); !errors.Is(err, ErrGoalRevisionStale) {
		t.Fatalf("stale completion error = %v, want ErrGoalRevisionStale", err)
	}
	if _, err := service.CompleteByModel(ctx, created.ID, protocol.CompleteGoalRequest{ExpectedObjectiveRevision: updated.ObjectiveRevision()}); err != nil {
		t.Fatalf("completion with fresh collaboration error = %v", err)
	}
}

func TestServiceObjectiveRevisionIgnoresUsageVersionBumps(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Analyze M4 and M5",
	})
	if err != nil {
		t.Fatal(err)
	}
	used, err := service.RecordUsageForGoal(ctx, created.ID, protocol.GoalUsage{InputTokens: 10, OutputTokens: 2}, "round-usage")
	if err != nil {
		t.Fatal(err)
	}
	if used.Version == created.Version || used.ObjectiveRevision() != created.ObjectiveRevision() {
		t.Fatalf("versions = goal:%d->%d objective:%d->%d", created.Version, used.Version, created.ObjectiveRevision(), used.ObjectiveRevision())
	}
	metadataUpdated, err := service.Update(ctx, created.ID, protocol.UpdateGoalRequest{
		Metadata: map[string]any{protocol.GoalMetadataObjectiveRevision: int64(99)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if metadataUpdated.ObjectiveRevision() != created.ObjectiveRevision() {
		t.Fatalf("metadata update changed objective revision to %d", metadataUpdated.ObjectiveRevision())
	}
	if _, err := service.CompleteByModel(ctx, created.ID, protocol.CompleteGoalRequest{ExpectedObjectiveRevision: created.ObjectiveRevision()}); err != nil {
		t.Fatalf("completion after usage bump error = %v", err)
	}
}

func TestServiceRetargetByModelRejectsInvalidOrNonActiveGoal(t *testing.T) {
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
	if _, err := service.RetargetByModel(ctx, created.SessionKey, protocol.RetargetGoalRequest{Objective: "New"}); !errors.Is(err, ErrGoalInvalidState) {
		t.Fatalf("paused goal error = %v, want ErrGoalInvalidState", err)
	}
	if _, err := service.RetargetByModel(ctx, created.SessionKey, protocol.RetargetGoalRequest{}); !errors.Is(err, ErrGoalInvalidInput) {
		t.Fatalf("empty objective error = %v, want ErrGoalInvalidInput", err)
	}
}
