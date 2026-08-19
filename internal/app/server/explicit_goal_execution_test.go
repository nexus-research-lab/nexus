package server

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

func TestExplicitCreateGoalRejectsCurrentTransientExecution(t *testing.T) {
	request := explicitGoalCreateRequest()
	executions := &stubExplicitExecutionService{
		current: explicitExecutionSnapshot(request.SessionKey, request.Objective),
	}
	goals := &stubExplicitGoalLifecycleService{}
	coordinator := newExplicitGoalExecutionCoordinator(goals, executions)

	_, err := coordinator.Create(context.Background(), request)
	if err == nil || !strings.Contains(err.Error(), "promote_execution_to_goal") {
		t.Fatalf("Create() error = %v, want explicit promotion guidance", err)
	}
	if goals.createCalls != 0 || executions.bindCalls != 0 {
		t.Fatalf("create calls=%d bind calls=%d", goals.createCalls, executions.bindCalls)
	}
}

func TestExplicitCreateGoalStaysStandaloneUntilPlanPreflightOwnsBinding(t *testing.T) {
	request := explicitGoalCreateRequest()
	goals := &stubExplicitGoalLifecycleService{}
	executions := &stubExplicitExecutionService{}
	coordinator := newExplicitGoalExecutionCoordinator(goals, executions)
	const expectedExecutionID = "execution-proposal-owned"

	created, err := coordinator.Create(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if got := protocol.GoalMetadataString(
		created.Metadata,
		protocol.GoalMetadataExecutionID,
	); got != "" {
		t.Fatalf("new Goal reservation = %q, want none", got)
	}
	if got := protocol.GoalExecutionBindingStateFromGoal(*created); got !=
		protocol.GoalExecutionBindingStateStandalone {
		t.Fatalf("new Goal binding state = %q, want standalone", got)
	}
	activation, err := coordinator.ResolveExplicitGoalActivation(
		context.Background(),
		orchestrationsvc.ExplicitGoalActivationRequest{
			OwnerUserID: request.OwnerUserID,
			SessionKey:  request.SessionKey,
			ScopeKind:   protocol.ExecutionScopeDM,
			AgentID:     request.AgentID,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if activation == nil || activation.ReservedExecutionID != "" ||
		activation.Objective != request.Objective {
		t.Fatalf("proposal activation = %#v", activation)
	}
	binding, err := coordinator.PrepareExplicitGoalBinding(
		context.Background(),
		orchestrationsvc.ExplicitGoalBindingRequest{
			CandidateExecutionID: expectedExecutionID,
			OwnerUserID:          request.OwnerUserID,
			SessionKey:           request.SessionKey,
			ScopeKind:            protocol.ExecutionScopeDM,
			Objective:            request.Objective,
			CompletionCriteria:   []string{"report accepted"},
			AgentID:              request.AgentID,
			RootRoundID:          "round-plan",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if binding.ExecutionID != expectedExecutionID ||
		binding.GoalID != created.ID ||
		binding.ActivationOrigin != protocol.GoalActivationOriginUserExplicit {
		t.Fatalf("binding = %#v", binding)
	}
	if goals.bindCalls != 1 {
		t.Fatalf("Goal metadata bind calls = %d", goals.bindCalls)
	}
	if got := protocol.GoalMetadataString(
		goals.current.Metadata,
		protocol.GoalMetadataExecutionID,
	); got != expectedExecutionID {
		t.Fatalf("reserved Goal execution metadata = %q", got)
	}
	if got := protocol.GoalExecutionBindingStateFromGoal(*goals.current); got !=
		protocol.GoalExecutionBindingStatePending {
		t.Fatalf("prepared Goal binding state = %q, want pending", got)
	}

	// A retry after Goal preflight reuses the now-durable proposal identity.
	replayed, err := coordinator.PrepareExplicitGoalBinding(
		context.Background(),
		orchestrationsvc.ExplicitGoalBindingRequest{
			CandidateExecutionID: "execution-new-random-id",
			OwnerUserID:          request.OwnerUserID,
			SessionKey:           request.SessionKey,
			ScopeKind:            protocol.ExecutionScopeDM,
			Objective:            request.Objective,
			CompletionCriteria:   []string{"report accepted"},
			AgentID:              request.AgentID,
			RootRoundID:          "round-plan",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.ExecutionID != expectedExecutionID {
		t.Fatalf("replayed binding = %#v", replayed)
	}
}

func TestExplicitCreateGoalRetryReusesStandaloneReceipt(t *testing.T) {
	request := explicitGoalCreateRequest()
	commandID := explicitGoalCommandID(request, request.Objective)
	goals := &stubExplicitGoalLifecycleService{current: &protocol.Goal{
		ID:         "goal-explicit",
		SessionKey: request.SessionKey,
		Objective:  request.Objective,
		Status:     protocol.GoalStatusActive,
		Version:    1,
		CreatedBy:  request.CreatedBy,
		Metadata:   explicitGoalMetadata(request.Metadata, nil, commandID),
	}}
	executions := &stubExplicitExecutionService{}
	coordinator := newExplicitGoalExecutionCoordinator(goals, executions)

	repaired, err := coordinator.Create(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if repaired.ID != "goal-explicit" ||
		goals.createCalls != 0 ||
		executions.bindCalls != 0 ||
		protocol.GoalExecutionBindingStateFromGoal(*repaired) != protocol.GoalExecutionBindingStateStandalone {
		t.Fatalf(
			"repaired=%#v createCalls=%d bindCalls=%d",
			repaired,
			goals.createCalls,
			executions.bindCalls,
		)
	}
}

func TestExplicitCreateGoalIgnoresTerminalExecution(t *testing.T) {
	request := explicitGoalCreateRequest()
	execution := explicitExecutionSnapshot(request.SessionKey, "old objective")
	execution.Execution.Status = protocol.ExecutionStatusCompleted
	goals := &stubExplicitGoalLifecycleService{}
	coordinator := newExplicitGoalExecutionCoordinator(
		goals,
		&stubExplicitExecutionService{current: execution},
	)
	created, err := coordinator.Create(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if created == nil || goals.createCalls != 1 ||
		protocol.GoalExecutionMode(protocol.GoalMetadataString(
			created.Metadata,
			protocol.GoalMetadataExecutionMode,
		)) != protocol.GoalExecutionModeGoalOnly {
		t.Fatalf("created=%#v calls=%d", created, goals.createCalls)
	}
}

func TestExplicitGoalPreflightRejectsExecutionBoundToDifferentGoal(t *testing.T) {
	request := explicitGoalCreateRequest()
	goals := &stubExplicitGoalLifecycleService{}
	coordinator := newExplicitGoalExecutionCoordinator(
		goals,
		&stubExplicitExecutionService{},
	)
	if _, err := coordinator.Create(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	_, err := coordinator.PrepareExplicitGoalBinding(
		context.Background(),
		orchestrationsvc.ExplicitGoalBindingRequest{
			CandidateExecutionID: "execution-current",
			OwnerUserID:          request.OwnerUserID,
			ExistingExecution:    true,
			ExistingGoalID:       "goal-other",
			SessionKey:           request.SessionKey,
			ScopeKind:            protocol.ExecutionScopeDM,
			Objective:            request.Objective,
			AgentID:              request.AgentID,
		},
	)
	if !errors.Is(err, orchestrationsvc.ErrExplicitGoalBindingConflict) {
		t.Fatalf("PrepareExplicitGoalBinding() error = %v", err)
	}
	if goals.bindCalls != 0 {
		t.Fatalf("conflicting preflight mutated Goal metadata %d times", goals.bindCalls)
	}
}

func TestGoalObjectiveRetargetSagaRecoversEveryDurablePhase(t *testing.T) {
	injected := errors.New("injected phase failure")
	for _, test := range []struct {
		name  string
		setup func(*stubExplicitGoalLifecycleService, *stubExplicitExecutionService)
	}{
		{
			name: "prepare",
			setup: func(goals *stubExplicitGoalLifecycleService, _ *stubExplicitExecutionService) {
				goals.prepareErrOnce = injected
			},
		},
		{
			name: "supersede",
			setup: func(_ *stubExplicitGoalLifecycleService, executions *stubExplicitExecutionService) {
				executions.supersedeErrOnce = injected
			},
		},
		{
			name: "commit",
			setup: func(goals *stubExplicitGoalLifecycleService, _ *stubExplicitExecutionService) {
				goals.commitErrOnce = injected
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			goal := retargetSagaGoal()
			oldGoal := *cloneExplicitGoal(goal)
			executions := &stubExplicitExecutionService{
				current: retargetSagaExecution(goal),
			}
			goals := &stubExplicitGoalLifecycleService{current: goal}
			test.setup(goals, executions)
			coordinator := newExplicitGoalExecutionCoordinator(goals, executions)
			command := goalsvc.ObjectiveRetargetCommand{
				Goal:                      oldGoal,
				Objective:                 "Deliver the revised verified report",
				Reason:                    "user changed the requested outcome",
				ExpectedObjectiveRevision: oldGoal.ObjectiveRevision(),
				Source:                    protocol.GoalUpdateSourceUser,
				OwnerUserID:               "owner-1",
			}

			if _, err := coordinator.RetargetGoalObjective(context.Background(), command); !errors.Is(err, injected) {
				t.Fatalf("first retarget error = %v, want injected failure", err)
			}
			if test.name == "supersede" || test.name == "commit" {
				transition, ok := goalsvc.ObjectiveTransitionFromGoal(*goals.current)
				if !ok || transition.Phase != goalsvc.ObjectiveTransitionPrepared ||
					goals.current.Objective != oldGoal.Objective {
					t.Fatalf("durable prepared transition = %#v goal=%#v", transition, goals.current)
				}
			}
			if test.name == "commit" &&
				executions.current.Execution.Status != protocol.ExecutionStatusSuperseded {
				t.Fatalf("old Execution status after commit failure = %s", executions.current.Execution.Status)
			}

			committed, err := coordinator.RetargetGoalObjective(context.Background(), command)
			if err != nil {
				t.Fatal(err)
			}
			transition, ok := goalsvc.ObjectiveTransitionFromGoal(*committed)
			if !ok || transition.Phase != goalsvc.ObjectiveTransitionAwaitingPlan ||
				transition.OldExecutionID != "execution-old" ||
				transition.SuccessorExecutionID == "" ||
				committed.Objective != command.Objective ||
				committed.ObjectiveRevision() != oldGoal.ObjectiveRevision()+1 ||
				protocol.GoalMetadataString(
					committed.Metadata,
					protocol.GoalMetadataExecutionID,
				) != transition.SuccessorExecutionID {
				t.Fatalf("committed Goal=%#v transition=%#v", committed, transition)
			}
			if _, exists := committed.Metadata[protocol.GoalMetadataCompletionCriteria]; exists {
				t.Fatalf("retarget carried old completion criteria: %#v", committed.Metadata)
			}
			if executions.current.Execution.Status != protocol.ExecutionStatusSuperseded ||
				executions.lastSupersede.SuccessorExecutionID != transition.SuccessorExecutionID ||
				executions.lastSupersede.OldGoalObjectiveRevision != transition.OldRevision ||
				executions.lastSupersede.NewGoalObjectiveRevision != transition.NewRevision ||
				executions.lastSupersede.ActorID != "owner-1" {
				t.Fatalf(
					"old graph=%#v supersede=%#v",
					executions.current.Execution,
					executions.lastSupersede,
				)
			}

			version := committed.Version
			replayed, err := coordinator.RetargetGoalObjective(context.Background(), command)
			if err != nil {
				t.Fatal(err)
			}
			replayedTransition, ok := goalsvc.ObjectiveTransitionFromGoal(*replayed)
			if !ok || replayed.Version != version ||
				replayedTransition.ID != transition.ID ||
				replayedTransition.SuccessorExecutionID != transition.SuccessorExecutionID ||
				goals.lastRetarget.CommandID != transition.CommandID {
				t.Fatalf("retarget replay=%#v transition=%#v", replayed, replayedTransition)
			}
		})
	}
}

func TestGoalObjectiveRetargetCommitsAfterTerminalExecution(t *testing.T) {
	goal := retargetSagaGoal()
	oldGoal := *cloneExplicitGoal(goal)
	completedExecution := retargetSagaExecution(goal)
	completedExecution.Execution.Status = protocol.ExecutionStatusCompleted
	executions := &stubExplicitExecutionService{current: completedExecution}
	goals := &stubExplicitGoalLifecycleService{current: goal}
	coordinator := newExplicitGoalExecutionCoordinator(goals, executions)
	command := goalsvc.ObjectiveRetargetCommand{
		Goal:                      oldGoal,
		Objective:                 "Deliver a second verified phase",
		Reason:                    "user extended the Goal after terminal acceptance",
		ExpectedObjectiveRevision: oldGoal.ObjectiveRevision(),
		Source:                    protocol.GoalUpdateSourceUser,
		OwnerUserID:               "owner-1",
	}

	committed, err := coordinator.RetargetGoalObjective(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	transition, ok := goalsvc.ObjectiveTransitionFromGoal(*committed)
	if !ok || transition.Phase != goalsvc.ObjectiveTransitionAwaitingPlan ||
		committed.ObjectiveRevision() != oldGoal.ObjectiveRevision()+1 {
		t.Fatalf("terminal retarget Goal=%#v transition=%#v", committed, transition)
	}
	if executions.current.Execution.Status != protocol.ExecutionStatusCompleted {
		t.Fatalf("terminal predecessor status = %s, want completed", executions.current.Execution.Status)
	}
	if executions.lastSupersede.SuccessorExecutionID != transition.SuccessorExecutionID ||
		executions.lastSupersede.OldGoalObjectiveRevision != transition.OldRevision ||
		executions.lastSupersede.NewGoalObjectiveRevision != transition.NewRevision {
		t.Fatalf("terminal predecessor reservation = %#v", executions.lastSupersede)
	}
}

func TestGoalObjectiveRetargetRejectsAnotherHTTPOwnerBeforePrepare(t *testing.T) {
	goal := retargetSagaGoal()
	goals := &stubExplicitGoalLifecycleService{current: goal}
	executions := &stubExplicitExecutionService{current: retargetSagaExecution(goal)}
	coordinator := newExplicitGoalExecutionCoordinator(goals, executions)

	_, err := coordinator.RetargetGoalObjective(
		context.Background(),
		goalsvc.ObjectiveRetargetCommand{
			Goal:                      *cloneExplicitGoal(goal),
			RequestedObjective:        "another owner's replacement",
			Objective:                 "another owner's replacement",
			Reason:                    "HTTP update",
			ExpectedObjectiveRevision: goal.ObjectiveRevision(),
			Source:                    protocol.GoalUpdateSourceUser,
			OwnerUserID:               "owner-other",
		},
	)
	if !errors.Is(err, goalsvc.ErrGoalForbidden) {
		t.Fatalf("retarget error = %v, want ErrGoalForbidden", err)
	}
	if goals.prepareCalls != 0 || executions.supersedeCalls != 0 ||
		executions.current.Execution.Status != protocol.ExecutionStatusActive {
		t.Fatalf(
			"unauthorized retarget mutated state: prepare=%d supersede=%d execution=%#v",
			goals.prepareCalls,
			executions.supersedeCalls,
			executions.current.Execution,
		)
	}
}

func TestGoalObjectiveRetargetFencesOrphanReservationAndPlansFreshSuccessor(t *testing.T) {
	goal := retargetSagaGoal()
	goal.Metadata[protocol.GoalMetadataOwnerUserID] = "owner-1"
	goals := &stubExplicitGoalLifecycleService{current: goal}
	executions := &stubExplicitExecutionService{}
	coordinator := newExplicitGoalExecutionCoordinator(goals, executions)

	committed, err := coordinator.RetargetGoalObjective(
		context.Background(),
		goalsvc.ObjectiveRetargetCommand{
			Goal:                      *cloneExplicitGoal(goal),
			RequestedObjective:        "fresh objective after orphan reservation",
			Objective:                 "fresh objective after orphan reservation",
			Reason:                    "user changed the objective",
			ExpectedObjectiveRevision: goal.ObjectiveRevision(),
			Source:                    protocol.GoalUpdateSourceUser,
			OwnerUserID:               "owner-1",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	transition, ok := goalsvc.ObjectiveTransitionFromGoal(*committed)
	if !ok || !transition.OldExecutionFenced ||
		transition.Phase != goalsvc.ObjectiveTransitionAwaitingPlan {
		t.Fatalf("committed orphan transition = %#v", transition)
	}
	binding, err := coordinator.PrepareExplicitGoalBinding(
		context.Background(),
		orchestrationsvc.ExplicitGoalBindingRequest{
			CandidateExecutionID: transition.SuccessorExecutionID,
			OwnerUserID:          "owner-1",
			SessionKey:           committed.SessionKey,
			ScopeKind:            protocol.ExecutionScopeDM,
			Objective:            committed.Objective,
			AgentID:              "agent-lead",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if binding == nil || binding.ExecutionID != transition.SuccessorExecutionID ||
		binding.ReplacesExecutionID != "" {
		t.Fatalf("fresh successor binding = %#v", binding)
	}
}

func TestGoalExecutionPreflightPreservesAdaptiveProvenanceAcrossDMAndRoom(t *testing.T) {
	for _, test := range []struct {
		name         string
		sessionKey   string
		scope        protocol.ExecutionScopeKind
		conversation string
		origin       protocol.GoalActivationOrigin
		reason       protocol.GoalActivationReason
	}{
		{
			name:       "adaptive initial DM",
			sessionKey: "agent:nexus:ws:dm:adaptive-initial",
			scope:      protocol.ExecutionScopeDM,
			origin:     protocol.GoalActivationOriginAdaptiveInitial,
			reason:     protocol.GoalActivationReasonContextBoundary,
		},
		{
			name:         "adaptive promoted Room",
			sessionKey:   "room:group:conversation-1",
			scope:        protocol.ExecutionScopeRoom,
			conversation: "conversation-1",
			origin:       protocol.GoalActivationOriginAdaptivePromoted,
			reason:       protocol.GoalActivationReasonObservedBoundary,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			goal := &protocol.Goal{
				ID:         "goal-adaptive",
				SessionKey: test.sessionKey,
				Objective:  "Deliver a verified report",
				Status:     protocol.GoalStatusActive,
				Version:    1,
				Metadata: map[string]any{
					protocol.GoalMetadataOwnerUserID:      "owner-1",
					protocol.GoalMetadataActivationOrigin: string(test.origin),
					protocol.GoalMetadataActivationReason: string(test.reason),
				},
			}
			if test.scope == protocol.ExecutionScopeRoom {
				goal.Metadata[protocol.GoalMetadataRoomGoalLeadAgentID] = "agent-lead"
			}
			goals := &stubExplicitGoalLifecycleService{current: goal}
			coordinator := newExplicitGoalExecutionCoordinator(
				goals,
				&stubExplicitExecutionService{},
			)
			binding, err := coordinator.PrepareExplicitGoalBinding(
				context.Background(),
				orchestrationsvc.ExplicitGoalBindingRequest{
					CandidateExecutionID: "execution-adaptive",
					OwnerUserID:          "owner-1",
					SessionKey:           test.sessionKey,
					ScopeKind:            test.scope,
					ConversationID:       test.conversation,
					Objective:            goal.Objective,
					CompletionCriteria:   []string{"report accepted"},
					AgentID:              "agent-lead",
					RootRoundID:          "round-plan",
				},
			)
			if err != nil {
				t.Fatal(err)
			}
			if binding.ActivationOrigin != test.origin ||
				binding.ActivationReason != test.reason ||
				binding.ExecutionID != "execution-adaptive" {
				t.Fatalf("adaptive binding = %#v", binding)
			}
		})
	}
}

func TestResolveExplicitGoalActivationProjectsExistingReservedExecution(t *testing.T) {
	goal := &protocol.Goal{
		ID:         "goal-reserved",
		SessionKey: "agent:nexus:ws:dm:reserved",
		Objective:  "Deliver a verified report",
		Status:     protocol.GoalStatusActive,
		Version:    3,
		Metadata: map[string]any{
			protocol.GoalMetadataOwnerUserID:      "owner-1",
			protocol.GoalMetadataActivationOrigin: string(protocol.GoalActivationOriginUserExplicit),
			protocol.GoalMetadataActivationReason: string(protocol.GoalActivationReasonPersistenceRequested),
			protocol.GoalMetadataExecutionID:      "execution-reserved",
			protocol.GoalMetadataObjectiveTransition: map[string]any{
				"transition_id":          "transition-1",
				"command_id":             "command-1",
				"phase":                  string(goalsvc.ObjectiveTransitionAwaitingPlan),
				"old_revision":           int64(2),
				"new_revision":           int64(3),
				"old_execution_id":       "execution-old",
				"old_execution_fenced":   false,
				"successor_execution_id": "execution-reserved",
				"target_objective":       "Deliver a verified report",
			},
		},
	}
	coordinator := newExplicitGoalExecutionCoordinator(
		&stubExplicitGoalLifecycleService{current: goal},
		&stubExplicitExecutionService{},
	)
	activation, err := coordinator.ResolveExplicitGoalActivation(
		context.Background(),
		orchestrationsvc.ExplicitGoalActivationRequest{
			ExistingGoalID:        goal.ID,
			GoalObjectiveRevision: goal.ObjectiveRevision(),
			OwnerUserID:           "owner-1",
			SessionKey:            goal.SessionKey,
			ScopeKind:             protocol.ExecutionScopeDM,
			AgentID:               "agent-lead",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if activation == nil || activation.ReservedExecutionID != "execution-reserved" ||
		activation.Objective != goal.Objective ||
		activation.ReplacesExecutionID != "execution-old" {
		t.Fatalf("activation = %#v, want reserved Execution projection", activation)
	}
}

func TestResolveExplicitGoalActivationRecoversLegacyExplicitReservation(t *testing.T) {
	const commandID = "explicit_goal_legacy_command"
	goal := &protocol.Goal{
		ID:         "goal-legacy-reservation",
		SessionKey: "agent:nexus:ws:dm:legacy-reservation",
		Objective:  "Deliver a verified report",
		Status:     protocol.GoalStatusActive,
		Version:    1,
		Metadata: map[string]any{
			protocol.GoalMetadataOwnerUserID:      "owner-1",
			protocol.GoalMetadataExplicitCommand:  commandID,
			protocol.GoalMetadataActivationOrigin: string(protocol.GoalActivationOriginUserExplicit),
			protocol.GoalMetadataActivationReason: string(protocol.GoalActivationReasonPersistenceRequested),
		},
	}
	coordinator := newExplicitGoalExecutionCoordinator(
		&stubExplicitGoalLifecycleService{current: goal},
		&stubExplicitExecutionService{},
	)
	activation, err := coordinator.ResolveExplicitGoalActivation(
		context.Background(),
		orchestrationsvc.ExplicitGoalActivationRequest{
			OwnerUserID: "owner-1",
			SessionKey:  goal.SessionKey,
			ScopeKind:   protocol.ExecutionScopeDM,
			AgentID:     "agent-lead",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	expected := protocol.ExplicitGoalReservedExecutionID(commandID)
	if activation == nil || activation.ReservedExecutionID != expected {
		t.Fatalf("legacy activation = %#v, want reservation %q", activation, expected)
	}
}

func TestResolveExplicitGoalActivationRejectsCrossOwnerGoal(t *testing.T) {
	goal := &protocol.Goal{
		ID:         "goal-other-owner",
		SessionKey: "agent:nexus:ws:dm:owner-fence",
		Objective:  "Deliver a verified report",
		Status:     protocol.GoalStatusActive,
		Version:    1,
		Metadata: map[string]any{
			protocol.GoalMetadataOwnerUserID:      "owner-other",
			protocol.GoalMetadataActivationOrigin: string(protocol.GoalActivationOriginUserExplicit),
			protocol.GoalMetadataActivationReason: string(protocol.GoalActivationReasonPersistenceRequested),
		},
	}
	coordinator := newExplicitGoalExecutionCoordinator(
		&stubExplicitGoalLifecycleService{current: goal},
		&stubExplicitExecutionService{},
	)
	_, err := coordinator.ResolveExplicitGoalActivation(
		context.Background(),
		orchestrationsvc.ExplicitGoalActivationRequest{
			OwnerUserID: "owner-1",
			SessionKey:  goal.SessionKey,
			ScopeKind:   protocol.ExecutionScopeDM,
			AgentID:     "agent-lead",
		},
	)
	if !errors.Is(err, orchestrationsvc.ErrExplicitGoalBindingConflict) {
		t.Fatalf("ResolveExplicitGoalActivation() error = %v, want owner binding conflict", err)
	}
}

func retargetSagaGoal() *protocol.Goal {
	return &protocol.Goal{
		ID:         "goal-retarget",
		SessionKey: "agent:nexus:ws:dm:retarget",
		Objective:  "Deliver the original verified report",
		Status:     protocol.GoalStatusActive,
		Version:    1,
		Metadata: map[string]any{
			protocol.GoalMetadataExecutionID:        "execution-old",
			protocol.GoalMetadataActivationOrigin:   string(protocol.GoalActivationOriginAdaptivePromoted),
			protocol.GoalMetadataActivationReason:   string(protocol.GoalActivationReasonObservedBoundary),
			protocol.GoalMetadataCompletionCriteria: []string{"old report accepted"},
		},
	}
}

func retargetSagaExecution(goal *protocol.Goal) *protocol.ExecutionSnapshot {
	return &protocol.ExecutionSnapshot{Execution: protocol.Execution{
		ID:                    "execution-old",
		OwnerUserID:           "owner-1",
		SessionKey:            goal.SessionKey,
		ScopeKind:             protocol.ExecutionScopeDM,
		CoordinatorAgentID:    "agent-lead",
		Objective:             goal.Objective,
		CompletionCriteria:    []string{"old report accepted"},
		Status:                protocol.ExecutionStatusActive,
		Version:               4,
		GoalID:                goal.ID,
		GoalObjectiveRevision: goal.ObjectiveRevision(),
		GoalActivationOrigin:  protocol.GoalActivationOriginAdaptivePromoted,
		GoalActivationReason:  protocol.GoalActivationReasonObservedBoundary,
	}}
}

type stubExplicitGoalLifecycleService struct {
	current        *protocol.Goal
	createCalls    int
	bindCalls      int
	prepareCalls   int
	commitCalls    int
	confirmCalls   int
	lastRetarget   goalsvc.ObjectiveRetargetCommand
	prepareErrOnce error
	commitErrOnce  error
}

func (s *stubExplicitGoalLifecycleService) Create(
	_ context.Context,
	request protocol.CreateGoalRequest,
) (*protocol.Goal, error) {
	s.createCalls++
	if s.current != nil {
		return nil, goalsvc.ErrGoalConflict
	}
	metadata := cloneExplicitGoalMetadata(request.Metadata)
	if ownerUserID := strings.TrimSpace(request.OwnerUserID); ownerUserID != "" {
		metadata[protocol.GoalMetadataOwnerUserID] = ownerUserID
	}
	s.current = &protocol.Goal{
		ID:          "goal-explicit",
		SessionKey:  request.SessionKey,
		Objective:   request.Objective,
		Status:      protocol.GoalStatusActive,
		TokenBudget: request.TokenBudget,
		Version:     1,
		CreatedBy:   request.CreatedBy,
		Metadata:    metadata,
	}
	return cloneExplicitGoal(s.current), nil
}

func (s *stubExplicitGoalLifecycleService) Current(
	context.Context,
	string,
) (*protocol.Goal, error) {
	if s.current == nil {
		return nil, goalsvc.ErrGoalNotFound
	}
	return cloneExplicitGoal(s.current), nil
}

func (s *stubExplicitGoalLifecycleService) CurrentOptional(
	context.Context,
	string,
) (*protocol.Goal, error) {
	return cloneExplicitGoal(s.current), nil
}

func (s *stubExplicitGoalLifecycleService) RetargetByModel(
	context.Context,
	string,
	protocol.RetargetGoalRequest,
) (*protocol.Goal, error) {
	return nil, errors.New("unexpected RetargetByModel")
}

func (s *stubExplicitGoalLifecycleService) AuditObjectiveAlignmentByModel(
	context.Context,
	string,
	protocol.AuditGoalObjectiveAlignmentRequest,
) (*protocol.GoalObjectiveAlignmentRecord, error) {
	return nil, errors.New("unexpected AuditObjectiveAlignmentByModel")
}

func (s *stubExplicitGoalLifecycleService) CompleteByModel(
	context.Context,
	string,
	protocol.CompleteGoalRequest,
) (*protocol.Goal, error) {
	return nil, errors.New("unexpected CompleteByModel")
}

func (s *stubExplicitGoalLifecycleService) BlockByModel(
	context.Context,
	string,
	protocol.BlockGoalRequest,
) (*protocol.Goal, error) {
	return nil, errors.New("unexpected BlockByModel")
}

func (s *stubExplicitGoalLifecycleService) BindExplicitExecution(
	_ context.Context,
	binding goalsvc.ExplicitExecutionBinding,
) (*protocol.Goal, error) {
	s.bindCalls++
	if s.current == nil {
		return nil, goalsvc.ErrGoalNotFound
	}
	s.current.Metadata = cloneExplicitGoalMetadata(s.current.Metadata)
	s.current.Metadata[protocol.GoalMetadataExecutionID] = binding.ExecutionID
	s.current.Metadata[protocol.GoalMetadataExecutionMode] =
		string(protocol.GoalExecutionModeManaged)
	s.current.Metadata[protocol.GoalMetadataExecutionBindingState] =
		string(protocol.GoalExecutionBindingStatePending)
	s.current.Metadata[protocol.GoalMetadataCompletionCriteria] = append(
		[]string(nil),
		binding.CompletionCriteria...,
	)
	s.current.Version++
	return cloneExplicitGoal(s.current), nil
}

func (s *stubExplicitGoalLifecycleService) PrepareObjectiveRetarget(
	_ context.Context,
	command goalsvc.ObjectiveRetargetCommand,
) (*protocol.Goal, error) {
	s.prepareCalls++
	s.lastRetarget = command
	if s.prepareErrOnce != nil {
		err := s.prepareErrOnce
		s.prepareErrOnce = nil
		return nil, err
	}
	if s.current == nil {
		return nil, goalsvc.ErrGoalNotFound
	}
	if transition, ok := goalsvc.ObjectiveTransitionFromGoal(*s.current); ok {
		if transition.ID == command.TransitionID &&
			transition.CommandID == command.CommandID &&
			transition.TargetObjective == command.Objective &&
			transition.SuccessorExecutionID == command.SuccessorExecutionID {
			return cloneExplicitGoal(s.current), nil
		}
		return nil, goalsvc.ErrGoalConflict
	}
	s.current.Metadata = cloneExplicitGoalMetadata(s.current.Metadata)
	if s.current.Metadata == nil {
		s.current.Metadata = map[string]any{}
	}
	s.current.Metadata[protocol.GoalMetadataObjectiveTransition] = map[string]any{
		"transition_id":          command.TransitionID,
		"command_id":             command.CommandID,
		"phase":                  string(goalsvc.ObjectiveTransitionPrepared),
		"old_revision":           command.ExpectedObjectiveRevision,
		"new_revision":           command.ExpectedObjectiveRevision + 1,
		"old_execution_id":       protocol.GoalMetadataString(s.current.Metadata, protocol.GoalMetadataExecutionID),
		"old_execution_fenced":   false,
		"successor_execution_id": command.SuccessorExecutionID,
		"requested_objective":    command.RequestedObjective,
		"target_objective":       command.Objective,
		"reason":                 command.Reason,
		"source":                 string(command.Source),
	}
	s.current.Version++
	return cloneExplicitGoal(s.current), nil
}

func (s *stubExplicitGoalLifecycleService) FenceObjectiveRetargetPredecessor(
	_ context.Context,
	goalID string,
	transitionID string,
	executionID string,
) (*protocol.Goal, error) {
	if s.current == nil || s.current.ID != goalID {
		return nil, goalsvc.ErrGoalNotFound
	}
	transition, ok := goalsvc.ObjectiveTransitionFromGoal(*s.current)
	if !ok || transition.ID != transitionID ||
		transition.OldExecutionID != executionID ||
		transition.Phase != goalsvc.ObjectiveTransitionPrepared {
		return nil, goalsvc.ErrGoalRevisionStale
	}
	s.current.Metadata = cloneExplicitGoalMetadata(s.current.Metadata)
	metadata := cloneExplicitGoalMetadata(
		s.current.Metadata[protocol.GoalMetadataObjectiveTransition].(map[string]any),
	)
	metadata["old_execution_fenced"] = true
	s.current.Metadata[protocol.GoalMetadataObjectiveTransition] = metadata
	s.current.Version++
	return cloneExplicitGoal(s.current), nil
}

func (s *stubExplicitGoalLifecycleService) CommitObjectiveRetarget(
	_ context.Context,
	goalID string,
	transitionID string,
) (*protocol.Goal, error) {
	s.commitCalls++
	if s.commitErrOnce != nil {
		err := s.commitErrOnce
		s.commitErrOnce = nil
		return nil, err
	}
	if s.current == nil || s.current.ID != goalID {
		return nil, goalsvc.ErrGoalNotFound
	}
	transition, ok := goalsvc.ObjectiveTransitionFromGoal(*s.current)
	if !ok || transition.ID != transitionID {
		return nil, goalsvc.ErrGoalRevisionStale
	}
	if s.current.ObjectiveRevision() == transition.NewRevision &&
		s.current.Objective == transition.TargetObjective {
		return cloneExplicitGoal(s.current), nil
	}
	s.current.Objective = transition.TargetObjective
	s.current.Metadata = cloneExplicitGoalMetadata(s.current.Metadata)
	s.current.Metadata[protocol.GoalMetadataObjectiveRevision] = transition.NewRevision
	s.current.Metadata[protocol.GoalMetadataExecutionID] = transition.SuccessorExecutionID
	s.current.Metadata[protocol.GoalMetadataExecutionBindingState] =
		string(protocol.GoalExecutionBindingStateReserved)
	delete(s.current.Metadata, protocol.GoalMetadataCompletionCriteria)
	s.current.Metadata[protocol.GoalMetadataObjectiveTransition] = map[string]any{
		"transition_id":          transition.ID,
		"command_id":             transition.CommandID,
		"phase":                  string(goalsvc.ObjectiveTransitionAwaitingPlan),
		"old_revision":           transition.OldRevision,
		"new_revision":           transition.NewRevision,
		"old_execution_id":       transition.OldExecutionID,
		"old_execution_fenced":   transition.OldExecutionFenced,
		"successor_execution_id": transition.SuccessorExecutionID,
		"requested_objective":    transition.RequestedObjective,
		"target_objective":       transition.TargetObjective,
		"reason":                 transition.Reason,
		"source":                 string(transition.Source),
	}
	s.current.Version++
	return cloneExplicitGoal(s.current), nil
}

func (s *stubExplicitGoalLifecycleService) ConfirmObjectiveExecutionBinding(
	_ context.Context,
	goalID string,
	objectiveRevision int64,
	executionID string,
	completionCriteria []string,
) (*protocol.Goal, error) {
	s.confirmCalls++
	if s.current == nil || s.current.ID != goalID {
		return nil, goalsvc.ErrGoalNotFound
	}
	if s.current.ObjectiveRevision() != objectiveRevision ||
		protocol.GoalMetadataString(s.current.Metadata, protocol.GoalMetadataExecutionID) != executionID {
		return nil, goalsvc.ErrGoalExecutionBindingConflict
	}
	transition, ok := goalsvc.ObjectiveTransitionFromGoal(*s.current)
	s.current.Metadata = cloneExplicitGoalMetadata(s.current.Metadata)
	s.current.Metadata[protocol.GoalMetadataExecutionBindingState] =
		string(protocol.GoalExecutionBindingStateConfirmed)
	s.current.Metadata[protocol.GoalMetadataCompletionCriteria] = append(
		[]string(nil),
		completionCriteria...,
	)
	if ok {
		s.current.Metadata[protocol.GoalMetadataObjectiveTransition] = map[string]any{
			"transition_id":          transition.ID,
			"command_id":             transition.CommandID,
			"phase":                  string(goalsvc.ObjectiveTransitionBound),
			"old_revision":           transition.OldRevision,
			"new_revision":           transition.NewRevision,
			"old_execution_id":       transition.OldExecutionID,
			"old_execution_fenced":   transition.OldExecutionFenced,
			"successor_execution_id": transition.SuccessorExecutionID,
			"requested_objective":    transition.RequestedObjective,
			"target_objective":       transition.TargetObjective,
			"reason":                 transition.Reason,
			"source":                 string(transition.Source),
		}
	}
	s.current.Version++
	return cloneExplicitGoal(s.current), nil
}

type stubExplicitExecutionService struct {
	current          *protocol.ExecutionSnapshot
	lastBind         orchestrationsvc.BindExplicitGoalInput
	lastSupersede    orchestrationsvc.GoalRevisionSupersedeInput
	bindCalls        int
	supersedeCalls   int
	supersedeErrOnce error
}

func (s *stubExplicitExecutionService) GetCurrent(
	context.Context,
	orchestrationsvc.ActorContext,
) (*protocol.ExecutionSnapshot, error) {
	return s.current, nil
}

func (s *stubExplicitExecutionService) ValidateGoalRevisionOwner(
	_ context.Context,
	executionID string,
	goalID string,
	goalObjectiveRevision int64,
	expectedOwnerUserID string,
) (bool, error) {
	if s.current == nil || s.current.Execution.ID != executionID {
		return false, nil
	}
	if s.current.Execution.OwnerUserID != expectedOwnerUserID ||
		s.current.Execution.GoalID != goalID ||
		s.current.Execution.GoalObjectiveRevision != goalObjectiveRevision {
		return false, errors.New("Goal revision owner mismatch")
	}
	return true, nil
}

func (s *stubExplicitExecutionService) BindExplicitGoal(
	_ context.Context,
	_ orchestrationsvc.ActorContext,
	input orchestrationsvc.BindExplicitGoalInput,
) (orchestrationsvc.MutationResult, error) {
	s.bindCalls++
	s.lastBind = input
	if s.current == nil {
		return orchestrationsvc.MutationResult{}, errors.New("no current Execution")
	}
	if s.current.Execution.GoalID == input.GoalID {
		return orchestrationsvc.NoOpResult(s.current, "already bound"), nil
	}
	s.current.Execution.Version++
	s.current.Execution.GoalID = input.GoalID
	s.current.Execution.GoalObjectiveRevision = input.GoalObjectiveRevision
	s.current.Execution.GoalActivationOrigin = protocol.GoalActivationOriginUserExplicit
	s.current.Execution.GoalActivationReason = protocol.GoalActivationReasonPersistenceRequested
	return orchestrationsvc.AppliedResult(s.current, nil, nil), nil
}

func (s *stubExplicitExecutionService) SupersedeGoalRevision(
	_ context.Context,
	input orchestrationsvc.GoalRevisionSupersedeInput,
) (*protocol.ExecutionSnapshot, error) {
	s.supersedeCalls++
	s.lastSupersede = input
	if s.supersedeErrOnce != nil {
		err := s.supersedeErrOnce
		s.supersedeErrOnce = nil
		return nil, err
	}
	if s.current == nil {
		return nil, nil
	}
	if s.current.Execution.Status != protocol.ExecutionStatusSuperseded &&
		s.current.Execution.Status != protocol.ExecutionStatusCompleted &&
		s.current.Execution.Status != protocol.ExecutionStatusFailed &&
		s.current.Execution.Status != protocol.ExecutionStatusCancelled {
		s.current.Execution.Status = protocol.ExecutionStatusSuperseded
		s.current.Execution.Version++
	}
	return s.current, nil
}

func explicitGoalCreateRequest() protocol.CreateGoalRequest {
	return protocol.CreateGoalRequest{
		SessionKey:  "agent:nexus:ws:dm:explicit-chain",
		Objective:   "Ship the verified report",
		CreatedBy:   "model",
		RoundID:     "round-create",
		OwnerUserID: "owner-1",
		AgentID:     "agent-1",
		Metadata: map[string]any{
			"created_via": "goal_command",
			protocol.GoalMetadataExecutionBindingState: "confirmed",
		},
	}
}

func explicitExecutionSnapshot(
	sessionKey string,
	objective string,
) *protocol.ExecutionSnapshot {
	return &protocol.ExecutionSnapshot{Execution: protocol.Execution{
		ID:                 "execution-current",
		OwnerUserID:        "owner-1",
		SessionKey:         sessionKey,
		ScopeKind:          protocol.ExecutionScopeDM,
		CoordinatorAgentID: "agent-1",
		Origin:             protocol.ExecutionOriginUserRequest,
		Objective:          objective,
		CompletionCriteria: []string{"report accepted"},
		Status:             protocol.ExecutionStatusActive,
		Version:            1,
	}}
}

func cloneExplicitGoal(source *protocol.Goal) *protocol.Goal {
	if source == nil {
		return nil
	}
	clone := *source
	clone.Metadata = cloneExplicitGoalMetadata(source.Metadata)
	return &clone
}

func cloneExplicitGoalMetadata(source map[string]any) map[string]any {
	if source == nil {
		return nil
	}
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
