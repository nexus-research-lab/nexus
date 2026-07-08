package goal

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

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

func TestServiceCreateFillsEmptyPreviewFromGoal(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	preview := &fakePreviewFiller{}
	service.SetPreviewFiller(preview)

	created, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Ship goal mode",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.items) != 1 || preview.items[0].sessionKey != created.SessionKey || preview.items[0].title != created.Objective {
		t.Fatalf("preview items = %#v, want created goal objective", preview.items)
	}
	if len(preview.titleSchedules) != 1 || preview.titleSchedules[0].fallbackTitle != created.Objective {
		t.Fatalf("title schedules = %#v, want goal title generation", preview.titleSchedules)
	}
}

func TestServiceCreateUsesLoopTitleAsPreviewFallback(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	preview := &fakePreviewFiller{}
	service.SetPreviewFiller(preview)

	created, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey:  "room:group:conversation_1",
		Objective:   "按 Loop「Knip Until Clean」推进这个 Room Goal。\n\n目标\n执行 knip 清理。",
		OwnerUserID: "owner-1",
		Metadata: map[string]any{
			protocol.GoalMetadataRoomGoalLoopTitle: "Knip Until Clean",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.items) != 1 || preview.items[0].title != "Knip Until Clean" {
		t.Fatalf("preview items = %#v, want loop title fallback", preview.items)
	}
	if len(preview.titleSchedules) != 1 ||
		preview.titleSchedules[0].goal.ID != created.ID ||
		preview.titleSchedules[0].ownerUserID != "owner-1" ||
		preview.titleSchedules[0].fallbackTitle != "Knip Until Clean" {
		t.Fatalf("title schedules = %#v, want loop title generation", preview.titleSchedules)
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

func TestServiceBroadcastsContinuationSuppressedEvent(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	broadcaster := &fakeGoalBroadcaster{}
	service.SetEventBroadcaster(broadcaster)
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Broadcast suppression",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.RecordContinuationProgress(ctx, created.ID, "goal_continuation_1", false); err != nil {
		t.Fatal(err)
	}
	if len(broadcaster.events) != 2 || broadcaster.events[1].EventType != protocol.EventTypeGoalContinuation {
		t.Fatalf("events = %#v, want goal_continuation for suppressed continuation", broadcaster.events)
	}
	if broadcaster.events[1].Data["goal_event_type"] != "continuation_suppressed" {
		t.Fatalf("payload = %#v, want continuation_suppressed goal_event_type", broadcaster.events[1].Data)
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

func TestServiceUpdateObjectiveFillsEmptyPreviewFromGoal(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	preview := &fakePreviewFiller{}
	service.SetPreviewFiller(preview)
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Initial goal objective",
	})
	if err != nil {
		t.Fatal(err)
	}
	preview.items = nil

	updatedObjective := "Revised goal objective"
	updated, err := service.Update(ctx, created.ID, protocol.UpdateGoalRequest{
		Objective: &updatedObjective,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.items) != 1 || preview.items[0].sessionKey != updated.SessionKey || preview.items[0].title != updated.Objective {
		t.Fatalf("preview items = %#v, want updated goal objective", preview.items)
	}
}

func TestServiceModelStatusUpdateFlushesButDoesNotClearRuntimeAccountingEarly(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:chat",
		Objective:  "Complete from tool",
	})
	if err != nil {
		t.Fatal(err)
	}
	accountant := &fakeExternalMutationAccountant{
		service: service,
		usage:   protocol.GoalUsage{InputTokens: 4, OutputTokens: 5},
		roundID: "round-running",
	}
	service.SetExternalMutationAccountant(accountant)

	completed, err := service.CompleteByModel(ctx, created.ID, protocol.CompleteGoalRequest{RoundID: "round-running"})
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != protocol.GoalStatusComplete {
		t.Fatalf("completed status = %q, want complete", completed.Status)
	}
	if len(accountant.sessionKeys) != 1 || accountant.sessionKeys[0] != created.SessionKey {
		t.Fatalf("accountant flush=%#v, want one best-effort flush for model update", accountant.sessionKeys)
	}
	if len(accountant.clearedSessionKeys) != 0 {
		t.Fatalf("accountant clear=%#v, want no early clear for model update", accountant.clearedSessionKeys)
	}
}

func TestServiceCompleteByModelRequiresRoomGoalCollaborationEvidence(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()

	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "room:group:conversation-1",
		Objective:  "完成房间协作目标",
		Metadata: map[string]any{
			protocol.GoalMetadataRoomGoalScope:                 "room",
			protocol.GoalMetadataRoomGoalLeadAgentID:           "agent-lead",
			protocol.GoalMetadataRoomGoalCollaborationRequired: true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err = service.CompleteByModel(ctx, created.ID, protocol.CompleteGoalRequest{RoundID: "round-lead"}); !errors.Is(err, ErrGoalInvalidState) {
		t.Fatalf("CompleteByModel error = %v, want ErrGoalInvalidState before collaborator evidence", err)
	}
	current, err := service.Current(ctx, created.SessionKey)
	if err != nil {
		t.Fatal(err)
	}
	if current.Status != protocol.GoalStatusActive {
		t.Fatalf("status = %q, want active after rejected completion", current.Status)
	}

	if _, err = service.RecordRoomGoalCollaborationEvidence(ctx, created.ID, "round-peer", "agent-peer"); err != nil {
		t.Fatal(err)
	}
	completed, err := service.CompleteByModel(ctx, created.ID, protocol.CompleteGoalRequest{RoundID: "round-lead-final"})
	if err != nil {
		t.Fatal(err)
	}
	if completed.Status != protocol.GoalStatusComplete {
		t.Fatalf("status = %q, want complete after collaborator evidence", completed.Status)
	}
	if !protocol.GoalRoomCollaborationObserved(*completed) {
		t.Fatalf("metadata = %#v, want collaboration observed", completed.Metadata)
	}
}

func TestServiceBlockByModelAllowsEmptyReason(t *testing.T) {
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
	blocked, err := service.BlockByModel(ctx, created.ID, protocol.BlockGoalRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if blocked.Status != protocol.GoalStatusBlocked || blocked.BlockedAt == nil {
		t.Fatalf("blocked = %#v, want blocked status", blocked)
	}
	if len(repo.events) != 2 || repo.events[1].EventType != "blocked" {
		t.Fatalf("events = %#v, want blocked event", repo.events)
	}
	if _, ok := repo.events[1].Payload["reason"]; ok {
		t.Fatalf("blocked payload = %#v, want no synthetic reason", repo.events[1].Payload)
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
