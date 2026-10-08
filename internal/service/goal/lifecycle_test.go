package goal

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
)

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
