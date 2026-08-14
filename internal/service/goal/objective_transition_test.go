package goal

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalappserver "github.com/nexus-research-lab/nexus/internal/service/goal/appserver"
)

func TestGoalObjectiveTransitionPhasesAndProtectedBindingMetadata(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true, GoalAutoContinueEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	ctx := context.Background()
	created, err := service.Create(ctx, protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:goal-rebase",
		Objective:  "old objective",
		CreatedBy:  "model",
		Metadata: map[string]any{
			protocol.GoalMetadataExecutionID: "execution-old",
			protocol.GoalMetadataExecutionBindingState: string(
				protocol.GoalExecutionBindingStateConfirmed,
			),
			protocol.GoalMetadataActivationOrigin:   string(protocol.GoalActivationOriginAdaptivePromoted),
			protocol.GoalMetadataActivationReason:   string(protocol.GoalActivationReasonObservedBoundary),
			protocol.GoalMetadataCompletionCriteria: []string{"old accepted"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	command := ObjectiveRetargetCommand{
		Goal:                      *created,
		Objective:                 "new objective",
		Reason:                    "user changed scope",
		CommandID:                 "retarget-command",
		TransitionID:              "transition-1",
		SuccessorExecutionID:      "execution-new",
		ExpectedObjectiveRevision: 1,
		Source:                    protocol.GoalUpdateSourceModel,
	}
	prepared, err := service.PrepareObjectiveRetarget(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	transition, ok := ObjectiveTransitionFromGoal(*prepared)
	if !ok || transition.Phase != ObjectiveTransitionPrepared ||
		transition.OldExecutionID != "execution-old" ||
		prepared.Objective != "old objective" ||
		prepared.ObjectiveRevision() != 1 {
		t.Fatalf("prepared Goal = %#v transition=%#v", prepared, transition)
	}
	replayed, err := service.PrepareObjectiveRetarget(ctx, command)
	if err != nil || replayed.Version != prepared.Version {
		t.Fatalf("prepare replay = %#v, err=%v", replayed, err)
	}
	conflicting := command
	conflicting.Objective = "different concurrent target"
	conflicting.CommandID = "retarget-command-other"
	conflicting.TransitionID = "transition-other"
	conflicting.SuccessorExecutionID = "execution-other"
	if _, err = service.PrepareObjectiveRetarget(ctx, conflicting); !errors.Is(err, ErrGoalConflict) {
		t.Fatalf("concurrent objective transition error = %v, want ErrGoalConflict", err)
	}
	pendingMutations := []struct {
		name string
		call func() error
	}{
		{
			name: "block",
			call: func() error {
				_, callErr := service.BlockByModel(ctx, prepared.ID, protocol.BlockGoalRequest{
					BlockerID:                 "pending-retarget",
					Reason:                    "blocked",
					NeededInput:               "unblock",
					ExpectedObjectiveRevision: prepared.ObjectiveRevision(),
				})
				return callErr
			},
		},
		{
			name: "progress",
			call: func() error {
				_, callErr := service.RecordContinuationProgress(
					ctx,
					prepared.ID,
					"round-old",
					true,
					prepared.ObjectiveRevision(),
				)
				return callErr
			},
		},
		{
			name: "collaboration",
			call: func() error {
				_, callErr := service.RecordRoomGoalCollaborationEvidence(
					ctx,
					prepared.ID,
					"round-old",
					"agent-member",
					prepared.ObjectiveRevision(),
				)
				return callErr
			},
		},
		{
			name: "continuation claim",
			call: func() error {
				_, callErr := service.ClaimContinuationPlan(ctx, protocol.GoalContinuation{
					Goal:    *prepared,
					RoundID: "round-old",
				})
				return callErr
			},
		},
	}
	for _, mutation := range pendingMutations {
		if mutationErr := mutation.call(); !errors.Is(mutationErr, ErrGoalInvalidState) {
			t.Fatalf("%s during prepared transition error = %v", mutation.name, mutationErr)
		}
	}
	committed, err := service.CommitObjectiveRetarget(ctx, created.ID, transition.ID)
	if err != nil {
		t.Fatal(err)
	}
	transition, ok = ObjectiveTransitionFromGoal(*committed)
	if !ok || transition.Phase != ObjectiveTransitionAwaitingPlan ||
		committed.Objective != "new objective" ||
		committed.ObjectiveRevision() != 2 ||
		protocol.GoalMetadataString(committed.Metadata, protocol.GoalMetadataExecutionID) != "execution-new" {
		t.Fatalf("committed Goal = %#v transition=%#v", committed, transition)
	}
	if got := protocol.GoalExecutionBindingStateFromGoal(*committed); got !=
		protocol.GoalExecutionBindingStateReserved {
		t.Fatalf("committed binding state = %q, want reserved", got)
	}
	if _, exists := committed.Metadata[protocol.GoalMetadataCompletionCriteria]; exists {
		t.Fatalf("committed Goal retained old criteria: %#v", committed.Metadata)
	}
	if err = service.ensureExecutionGoalCompletionReady(ctx, *committed); !errors.Is(err, ErrGoalInvalidState) {
		t.Fatalf("pending transition completion error = %v", err)
	}
	if _, err = service.CompleteByModel(ctx, committed.ID, protocol.CompleteGoalRequest{
		ExpectedObjectiveRevision: committed.ObjectiveRevision(),
	}); !errors.Is(err, ErrGoalInvalidState) {
		t.Fatalf("model completion during rebase error = %v", err)
	}
	complete := goalappserver.ThreadGoalStatusComplete
	if _, err = service.SetFromThreadGoalParams(ctx, goalappserver.ThreadGoalSetParams{
		ThreadID: committed.SessionKey,
		Status:   &complete,
	}); !errors.Is(err, ErrGoalInvalidState) {
		t.Fatalf("app-server completion during rebase error = %v", err)
	}
	planning, planErr := service.PlanContinuationForSession(ctx, committed.SessionKey, "round-old")
	if planErr != nil || planning == nil ||
		planning.Purpose != goalObjectiveTransitionPlanningPurpose ||
		planning.ExecutionID != "" ||
		!strings.Contains(planning.Prompt, "goal_binding=current") ||
		planning.Metadata[goalTransitionContinuationIDMetadataKey] != transition.ID {
		t.Fatalf("transition planning continuation = %#v, err=%v", planning, planErr)
	}
	stillCurrent, err := service.GoalContinuationStillCurrent(ctx, *planning)
	if err != nil || !stillCurrent {
		t.Fatalf("transition planning continuation current=%t err=%v", stillCurrent, err)
	}
	if _, err = service.ClaimContinuationPlan(ctx, *planning); err != nil {
		t.Fatalf("claim transition planning continuation: %v", err)
	}
	failedPlanning, err := service.RecordContinuationFailure(
		ctx,
		committed.ID,
		planning.RoundID,
		"runtime start failed",
		committed.ObjectiveRevision(),
	)
	if err != nil || failedPlanning.EmptyProgressCount != goalContinuationSuppressionThreshold {
		t.Fatalf("record planning failure = %#v, err=%v", failedPlanning, err)
	}
	retryPlanning, err := service.PlanContinuationForSession(ctx, committed.SessionKey, planning.RoundID)
	if err != nil || retryPlanning == nil ||
		retryPlanning.Purpose != goalObjectiveTransitionPlanningPurpose {
		t.Fatalf("retry transition planning continuation = %#v, err=%v", retryPlanning, err)
	}
	if _, err = service.ClaimContinuationPlan(ctx, *retryPlanning); err != nil {
		t.Fatalf("claim retry transition planning continuation: %v", err)
	}
	reserved, err := service.BindExplicitExecution(ctx, ExplicitExecutionBinding{
		GoalID:                    committed.ID,
		ExpectedObjectiveRevision: 2,
		ExecutionID:               "execution-new",
		CompletionCriteria:        []string{"new accepted"},
	})
	if err != nil {
		t.Fatal(err)
	}
	transition, ok = ObjectiveTransitionFromGoal(*reserved)
	if !ok || transition.Phase != ObjectiveTransitionBindingReserved {
		t.Fatalf("reserved transition = %#v", transition)
	}
	if got := protocol.GoalExecutionBindingStateFromGoal(*reserved); got !=
		protocol.GoalExecutionBindingStatePending {
		t.Fatalf("prepared successor binding state = %q, want pending", got)
	}
	if plan, pendingErr := service.PlanContinuationForSession(ctx, reserved.SessionKey, "planning-round"); pendingErr != nil || plan != nil {
		t.Fatalf("binding-reserved transition continuation = %#v, err=%v", plan, pendingErr)
	}
	bound, err := service.ConfirmObjectiveExecutionBinding(
		ctx,
		reserved.ID,
		2,
		"execution-new",
		[]string{"new accepted"},
	)
	if err != nil {
		t.Fatal(err)
	}
	transition, ok = ObjectiveTransitionFromGoal(*bound)
	if !ok || transition.Phase != ObjectiveTransitionBound {
		t.Fatalf("bound transition = %#v", transition)
	}
	if got := protocol.GoalExecutionBindingStateFromGoal(*bound); got !=
		protocol.GoalExecutionBindingStateConfirmed {
		t.Fatalf("bound successor state = %q, want confirmed", got)
	}
	metadataUpdated, err := service.Update(ctx, bound.ID, protocol.UpdateGoalRequest{
		Metadata: map[string]any{"user_note": "keep"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if protocol.GoalMetadataString(metadataUpdated.Metadata, protocol.GoalMetadataExecutionID) != "execution-new" ||
		protocol.GoalExecutionBindingStateFromGoal(*metadataUpdated) !=
			protocol.GoalExecutionBindingStateConfirmed ||
		protocol.GoalMetadataString(metadataUpdated.Metadata, protocol.GoalMetadataActivationOrigin) !=
			string(protocol.GoalActivationOriginAdaptivePromoted) {
		t.Fatalf("user metadata update removed server binding: %#v", metadataUpdated.Metadata)
	}
	if currentTransition, exists := ObjectiveTransitionFromGoal(*metadataUpdated); !exists ||
		currentTransition.Phase != ObjectiveTransitionBound {
		t.Fatalf("user metadata update removed transition: %#v", metadataUpdated.Metadata)
	}
	malformed := *metadataUpdated
	malformed.Metadata = cloneMap(metadataUpdated.Metadata)
	malformed.Metadata[protocol.GoalMetadataObjectiveTransition] = "corrupted"
	if err = service.ensureExecutionGoalCompletionReady(ctx, malformed); !errors.Is(err, ErrGoalInvalidState) {
		t.Fatalf("malformed objective transition completion error = %v", err)
	}
}

func TestAllGoalObjectiveMutationSurfacesUseRetargetCoordinator(t *testing.T) {
	for _, test := range []struct {
		name       string
		wantSource protocol.GoalUpdateSource
		mutate     func(context.Context, *Service, protocol.Goal, string) (*protocol.Goal, error)
		assert     func(*testing.T, ObjectiveRetargetCommand)
	}{
		{
			name:       "model MCP",
			wantSource: protocol.GoalUpdateSourceModel,
			mutate: func(ctx context.Context, service *Service, item protocol.Goal, objective string) (*protocol.Goal, error) {
				return service.RetargetByModel(ctx, item.SessionKey, protocol.RetargetGoalRequest{
					Objective:                 objective,
					RoundID:                   "round-retarget",
					AgentID:                   "agent-lead",
					ExpectedObjectiveRevision: item.ObjectiveRevision(),
				})
			},
			assert: func(t *testing.T, command ObjectiveRetargetCommand) {
				t.Helper()
				if command.RoundID != "round-retarget" || command.AgentID != "agent-lead" {
					t.Fatalf("model retarget actor context = %#v", command)
				}
			},
		},
		{
			name:       "HTTP PATCH",
			wantSource: protocol.GoalUpdateSourceUser,
			mutate: func(ctx context.Context, service *Service, item protocol.Goal, objective string) (*protocol.Goal, error) {
				return service.Update(ctx, item.ID, protocol.UpdateGoalRequest{
					Objective:   &objective,
					OwnerUserID: "owner-http",
				})
			},
			assert: func(t *testing.T, command ObjectiveRetargetCommand) {
				t.Helper()
				if command.OwnerUserID != "owner-http" {
					t.Fatalf("HTTP retarget owner context = %#v", command)
				}
			},
		},
		{
			name:       "app-server thread goal set",
			wantSource: protocol.GoalUpdateSourceExternal,
			mutate: func(ctx context.Context, service *Service, item protocol.Goal, objective string) (*protocol.Goal, error) {
				return service.SetFromThreadGoalParams(ctx, goalappserver.ThreadGoalSetParams{
					ThreadID:  item.SessionKey,
					Objective: &objective,
				})
			},
			assert: func(_ *testing.T, _ ObjectiveRetargetCommand) {},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := newMemoryRepository()
			service := NewService(config.Config{GoalEnabled: true}, repo)
			service.nowFn = fixedClock()
			service.idFactory = sequentialID()
			ownerUserID := "__system__"
			if test.wantSource == protocol.GoalUpdateSourceUser {
				ownerUserID = "owner-http"
			}
			created, err := service.Create(context.Background(), protocol.CreateGoalRequest{
				SessionKey:  "agent:nexus:ws:dm:objective-hook",
				Objective:   "Original objective",
				OwnerUserID: ownerUserID,
				Metadata: map[string]any{
					protocol.GoalMetadataExecutionID: "execution-old",
					protocol.GoalMetadataExecutionBindingState: string(
						protocol.GoalExecutionBindingStateConfirmed,
					),
					protocol.GoalMetadataActivationOrigin:   string(protocol.GoalActivationOriginAdaptivePromoted),
					protocol.GoalMetadataActivationReason:   string(protocol.GoalActivationReasonObservedBoundary),
					protocol.GoalMetadataCompletionCriteria: []string{"old accepted"},
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			coordinator := &recordingObjectiveRetargetCoordinator{}
			service.SetObjectiveRetargetCoordinator(coordinator)
			revised := "Revised objective"
			updated, err := test.mutate(context.Background(), service, *created, revised)
			if err != nil {
				t.Fatal(err)
			}
			if updated.Objective != revised || len(coordinator.commands) != 1 {
				t.Fatalf("updated=%#v coordinator commands=%#v", updated, coordinator.commands)
			}
			command := coordinator.commands[0]
			if command.Goal.ID != created.ID ||
				command.RequestedObjective != revised ||
				command.ExpectedObjectiveRevision != created.ObjectiveRevision() ||
				command.Source != test.wantSource {
				t.Fatalf("retarget command = %#v", command)
			}
			test.assert(t, command)
		})
	}
}

func TestUserObjectiveRetargetEntrypointsDispatchSuccessorPlanning(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(context.Context, *Service, protocol.Goal, string) (*protocol.Goal, error)
	}{
		{
			name: "HTTP PATCH",
			mutate: func(ctx context.Context, service *Service, item protocol.Goal, objective string) (*protocol.Goal, error) {
				return service.Update(ctx, item.ID, protocol.UpdateGoalRequest{
					Objective:   &objective,
					OwnerUserID: "owner-1",
				})
			},
		},
		{
			name: "app-server set",
			mutate: func(ctx context.Context, service *Service, item protocol.Goal, objective string) (*protocol.Goal, error) {
				return service.SetFromThreadGoalParams(ctx, goalappserver.ThreadGoalSetParams{
					ThreadID:    item.SessionKey,
					Objective:   &objective,
					OwnerUserID: "owner-1",
				})
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := newMemoryRepository()
			service := NewService(config.Config{
				GoalEnabled:             true,
				GoalAutoContinueEnabled: true,
			}, repo)
			service.nowFn = fixedClock()
			service.idFactory = sequentialID()
			created, err := service.Create(context.Background(), protocol.CreateGoalRequest{
				SessionKey:  "agent:nexus:ws:dm:retarget-dispatch",
				Objective:   "Original objective",
				CreatedBy:   "model",
				OwnerUserID: "owner-1",
				Metadata: map[string]any{
					protocol.GoalMetadataExecutionID: "execution-old",
					protocol.GoalMetadataExecutionBindingState: string(
						protocol.GoalExecutionBindingStateConfirmed,
					),
					protocol.GoalMetadataActivationOrigin:   string(protocol.GoalActivationOriginAdaptivePromoted),
					protocol.GoalMetadataActivationReason:   string(protocol.GoalActivationReasonObservedBoundary),
					protocol.GoalMetadataCompletionCriteria: []string{"old accepted"},
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			service.SetObjectiveRetargetCoordinator(&awaitingPlanObjectiveRetargetCoordinator{
				service: service,
			})
			dispatcher := &fakeContinuationDispatcher{}
			service.SetContinuationDispatcher(dispatcher)

			updated, err := test.mutate(context.Background(), service, *created, "Replacement objective")
			if err != nil {
				t.Fatal(err)
			}
			transition, ok := ObjectiveTransitionFromGoal(*updated)
			if !ok || transition.Phase != ObjectiveTransitionAwaitingPlan ||
				len(dispatcher.plans) != 1 {
				t.Fatalf("updated=%#v transition=%#v plans=%#v", updated, transition, dispatcher.plans)
			}
			plan := dispatcher.plans[0]
			if plan.Purpose != goalObjectiveTransitionPlanningPurpose ||
				plan.ExecutionID != "" ||
				plan.Goal.ID != updated.ID ||
				plan.Goal.ObjectiveRevision() != updated.ObjectiveRevision() ||
				!strings.Contains(plan.Prompt, "goal_binding=current") {
				t.Fatalf("successor planning plan = %#v", plan)
			}
		})
	}
}

func TestReservedGoalObjectiveMutationSurfacesReviseInPlace(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(context.Context, *Service, protocol.Goal, string) (*protocol.Goal, error)
	}{
		{
			name: "model MCP",
			mutate: func(ctx context.Context, service *Service, item protocol.Goal, objective string) (*protocol.Goal, error) {
				return service.RetargetByModel(ctx, item.SessionKey, protocol.RetargetGoalRequest{
					Objective: objective, ExpectedGoalID: item.ID,
					ExpectedObjectiveRevision: item.ObjectiveRevision(),
				})
			},
		},
		{
			name: "HTTP PATCH",
			mutate: func(ctx context.Context, service *Service, item protocol.Goal, objective string) (*protocol.Goal, error) {
				return service.Update(ctx, item.ID, protocol.UpdateGoalRequest{
					Objective: &objective, OwnerUserID: "owner-routing",
				})
			},
		},
		{
			name: "app-server thread goal set",
			mutate: func(ctx context.Context, service *Service, item protocol.Goal, objective string) (*protocol.Goal, error) {
				return service.SetFromThreadGoalParams(ctx, goalappserver.ThreadGoalSetParams{
					ThreadID: item.SessionKey, Objective: &objective, OwnerUserID: "owner-routing",
				})
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := newMemoryRepository()
			service := NewService(config.Config{GoalEnabled: true}, repo)
			service.nowFn = fixedClock()
			service.idFactory = sequentialID()
			coordinator := &recordingObjectiveRetargetCoordinator{}
			service.SetObjectiveRetargetCoordinator(coordinator)
			created, err := service.Create(context.Background(), protocol.CreateGoalRequest{
				SessionKey: "agent:nexus:ws:dm:reserved-routing-" + strings.ReplaceAll(test.name, " ", "-"),
				Objective:  "Original objective", CreatedBy: "model", OwnerUserID: "owner-routing",
				Metadata: map[string]any{
					protocol.GoalMetadataExecutionID: "execution-reserved",
					protocol.GoalMetadataExecutionBindingState: string(
						protocol.GoalExecutionBindingStateReserved,
					),
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			updated, err := test.mutate(context.Background(), service, *created, "Revised objective")
			if err != nil {
				t.Fatal(err)
			}
			if updated.ID != created.ID || updated.Objective != "Revised objective" ||
				updated.ObjectiveRevision() != created.ObjectiveRevision()+1 {
				t.Fatalf("updated = %#v, want same Goal with next objective revision", updated)
			}
			if len(coordinator.commands) != 0 {
				t.Fatalf("coordinator commands = %#v, want none for reserved Goal", coordinator.commands)
			}
		})
	}
}

func TestObjectiveRetargetRetryUsesPersistedRequestedObjectiveBeforeRewritingAgain(t *testing.T) {
	repo := newMemoryRepository()
	service := NewService(config.Config{GoalEnabled: true}, repo)
	service.nowFn = fixedClock()
	service.idFactory = sequentialID()
	coordinator := &localObjectiveRetargetCoordinator{service: service}
	service.SetObjectiveRetargetCoordinator(coordinator)
	rewriter := &driftingObjectiveRewriter{
		outputs: []string{
			"Canonical objective from first rewrite",
			"Different canonical objective from a later rewrite",
		},
	}
	service.SetObjectiveRewriter(rewriter)
	created, err := service.Create(context.Background(), protocol.CreateGoalRequest{
		SessionKey: "agent:nexus:ws:dm:rewrite-retry",
		Objective:  "Original objective",
		CreatedBy:  "model",
		Metadata: map[string]any{
			protocol.GoalMetadataExecutionID: "execution-rewrite-retry",
			protocol.GoalMetadataExecutionBindingState: string(
				protocol.GoalExecutionBindingStateConfirmed,
			),
			protocol.GoalMetadataActivationOrigin:   string(protocol.GoalActivationOriginAdaptiveInitial),
			protocol.GoalMetadataActivationReason:   string(protocol.GoalActivationReasonContextBoundary),
			protocol.GoalMetadataCompletionCriteria: []string{"original objective accepted"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	requested := "Please revise the outcome"
	first, err := service.Update(context.Background(), created.ID, protocol.UpdateGoalRequest{
		Objective: &requested,
	})
	if err != nil {
		t.Fatal(err)
	}
	transition, ok := ObjectiveTransitionFromGoal(*first)
	if !ok || transition.RequestedObjective != requested ||
		transition.TargetObjective != rewriter.outputs[0] ||
		first.Objective != rewriter.outputs[0] {
		t.Fatalf("first retarget=%#v transition=%#v", first, transition)
	}

	replayed, err := service.Update(context.Background(), created.ID, protocol.UpdateGoalRequest{
		Objective: &requested,
	})
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Objective != first.Objective ||
		replayed.ObjectiveRevision() != first.ObjectiveRevision() ||
		replayed.Version != first.Version ||
		rewriter.calls != 1 ||
		coordinator.calls != 1 {
		t.Fatalf(
			"replayed=%#v rewriteCalls=%d coordinatorCalls=%d",
			replayed,
			rewriter.calls,
			coordinator.calls,
		)
	}
}

type recordingObjectiveRetargetCoordinator struct {
	commands []ObjectiveRetargetCommand
}

func (r *recordingObjectiveRetargetCoordinator) RetargetGoalObjective(
	_ context.Context,
	command ObjectiveRetargetCommand,
) (*protocol.Goal, error) {
	r.commands = append(r.commands, command)
	updated := command.Goal
	updated.Objective = command.Objective
	updated.Metadata = cloneMap(updated.Metadata)
	if updated.Metadata == nil {
		updated.Metadata = map[string]any{}
	}
	updated.Metadata[protocol.GoalMetadataObjectiveRevision] =
		command.ExpectedObjectiveRevision + 1
	updated.Version++
	return &updated, nil
}

type localObjectiveRetargetCoordinator struct {
	service *Service
	calls   int
}

type awaitingPlanObjectiveRetargetCoordinator struct {
	service *Service
}

func (c *awaitingPlanObjectiveRetargetCoordinator) RetargetGoalObjective(
	ctx context.Context,
	command ObjectiveRetargetCommand,
) (*protocol.Goal, error) {
	command.CommandID = "awaiting-plan-command"
	command.TransitionID = "awaiting-plan-transition"
	command.SuccessorExecutionID = "execution-successor"
	prepared, err := c.service.PrepareObjectiveRetarget(ctx, command)
	if err != nil {
		return nil, err
	}
	return c.service.CommitObjectiveRetarget(ctx, prepared.ID, command.TransitionID)
}

func (c *localObjectiveRetargetCoordinator) RetargetGoalObjective(
	ctx context.Context,
	command ObjectiveRetargetCommand,
) (*protocol.Goal, error) {
	c.calls++
	command.CommandID = "rewrite-retry-command"
	command.TransitionID = "rewrite-retry-transition"
	command.SuccessorExecutionID = "rewrite-retry-successor"
	prepared, err := c.service.PrepareObjectiveRetarget(ctx, command)
	if err != nil {
		return nil, err
	}
	committed, err := c.service.CommitObjectiveRetarget(ctx, prepared.ID, command.TransitionID)
	if err != nil {
		return nil, err
	}
	pending, err := c.service.BindExplicitExecution(ctx, ExplicitExecutionBinding{
		GoalID:                    committed.ID,
		ExpectedObjectiveRevision: committed.ObjectiveRevision(),
		ExecutionID:               command.SuccessorExecutionID,
		CompletionCriteria:        []string{"successor objective accepted"},
	})
	if err != nil {
		return nil, err
	}
	return c.service.ConfirmObjectiveExecutionBinding(
		ctx,
		pending.ID,
		pending.ObjectiveRevision(),
		command.SuccessorExecutionID,
		[]string{"successor objective accepted"},
	)
}

type driftingObjectiveRewriter struct {
	outputs []string
	calls   int
}

func (r *driftingObjectiveRewriter) RewriteGoalObjective(
	context.Context,
	string,
	string,
	string,
) (string, error) {
	index := r.calls
	r.calls++
	if index >= len(r.outputs) {
		index = len(r.outputs) - 1
	}
	return r.outputs[index], nil
}
