// objective_test.go 覆盖 Goal 目标重写、状态迁移与执行绑定测试。
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

type ownerBindingReadResolver struct {
	calls      int
	resolution protocol.GoalExecutionBindingResolution
}

func (r *ownerBindingReadResolver) ResolveGoalExecutionBinding(
	context.Context,
	protocol.Goal,
) (protocol.GoalExecutionBindingResolution, error) {
	r.calls++
	return r.resolution, nil
}

func (*ownerBindingReadResolver) ExecutionGoalCompletionBlocker(
	context.Context,
	protocol.Goal,
) (string, error) {
	return "", nil
}

func TestExecutionBindingForOwnerRejectsForeignOwnerBeforeResolution(t *testing.T) {
	repository := newMemoryRepository()
	item := protocol.Goal{
		ID:         "goal-binding-private",
		SessionKey: "agent:nexus:ws:dm:binding-private",
		Objective:  "Keep binding private",
		Status:     protocol.GoalStatusActive,
		Version:    1,
		Metadata: map[string]any{
			protocol.GoalMetadataOwnerUserID: "owner-binding-private",
		},
	}
	repository.goals[item.ID] = item
	resolver := &ownerBindingReadResolver{}
	service := NewService(config.Config{GoalEnabled: true}, repository)
	service.SetExecutionGoalCompletionReadiness(resolver)

	_, err := service.ExecutionBindingForOwner(
		context.Background(),
		item.ID,
		"owner-attacker",
	)
	if !errors.Is(err, ErrGoalForbidden) {
		t.Fatalf("error = %v, want ErrGoalForbidden", err)
	}
	if resolver.calls != 0 {
		t.Fatalf("resolver calls = %d, want authorization before resolution", resolver.calls)
	}
}
