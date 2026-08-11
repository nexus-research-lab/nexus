package realtime

import (
	"context"
	"errors"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

type roomExecutionGoalAuthorityProvider struct {
	binding      orchestrationsvc.RuntimeGoalBinding
	resolution   protocol.GoalExecutionBindingResolution
	resolverErr  error
	resolveCalls int
	resolvedGoal protocol.Goal
}

func (p *roomExecutionGoalAuthorityProvider) RuntimeContext(
	context.Context,
	orchestrationsvc.ActorContext,
) (string, error) {
	return "", nil
}

func (p *roomExecutionGoalAuthorityProvider) RuntimeGoalBinding(
	context.Context,
	orchestrationsvc.ActorContext,
) (orchestrationsvc.RuntimeGoalBinding, error) {
	return p.binding, nil
}

func (p *roomExecutionGoalAuthorityProvider) ResolveGoalExecutionBinding(
	_ context.Context,
	goal protocol.Goal,
) (protocol.GoalExecutionBindingResolution, error) {
	p.resolveCalls++
	p.resolvedGoal = goal
	return p.resolution, p.resolverErr
}

func TestRoomWorkAndReviewGoalAuthorityRequiresConfirmedCentralBinding(t *testing.T) {
	states := []struct {
		name        string
		state       protocol.GoalExecutionBindingState
		wantGranted bool
		wantErr     error
	}{
		{name: "standalone remains unbound", state: protocol.GoalExecutionBindingStateStandalone},
		{name: "reserved remains unbound", state: protocol.GoalExecutionBindingStateReserved},
		{name: "pending fails closed", state: protocol.GoalExecutionBindingStatePending, wantErr: goalsvc.ErrGoalExecutionBindingConflict},
		{name: "conflict fails closed", state: protocol.GoalExecutionBindingStateConflict, wantErr: goalsvc.ErrGoalExecutionBindingConflict},
		// The central resolver also emits Confirmed for exact legacy bilateral
		// records; Room deliberately consumes the classification, not provenance.
		{name: "confirmed including legacy classification grants", state: protocol.GoalExecutionBindingStateConfirmed, wantGranted: true},
	}
	actors := []struct {
		name  string
		actor orchestrationsvc.ActorContext
	}{
		{
			name: "work",
			actor: orchestrationsvc.ActorContext{WorkBinding: &protocol.ExecutionWorkBinding{
				ExecutionID: "execution-1",
			}},
		},
		{
			name: "review",
			actor: orchestrationsvc.ActorContext{ReviewBinding: &protocol.ExecutionReviewBinding{
				ExecutionID: "execution-1",
			}},
		},
	}

	for _, actorCase := range actors {
		for _, stateCase := range states {
			t.Run(actorCase.name+"/"+stateCase.name, func(t *testing.T) {
				provider := &roomExecutionGoalAuthorityProvider{
					binding: orchestrationsvc.RuntimeGoalBinding{
						ExecutionID:           "execution-1",
						SessionKey:            "room:group:conversation-1",
						GoalID:                "goal-1",
						GoalObjectiveRevision: 2,
					},
					resolution: protocol.GoalExecutionBindingResolution{
						State:       stateCase.state,
						ExecutionID: "execution-1",
					},
				}
				service := &Service{
					executionContext: provider,
					goals: &fakeRoomGoalContextProvider{
						runtimeContexts: map[string]string{
							"room:group:conversation-1": "authoritative goal context",
						},
						runtimeGoals: map[string]*protocol.Goal{
							"room:group:conversation-1": {
								ID:         "goal-1",
								SessionKey: "room:group:conversation-1",
								Metadata: map[string]any{
									protocol.GoalMetadataObjectiveRevision: int64(2),
								},
							},
						},
					},
				}

				goalContext, authority, granted, err := service.resolveExecutionGoalMutationAuthority(
					context.Background(),
					actorCase.actor,
					"root-round-1",
				)
				if !errors.Is(err, stateCase.wantErr) {
					t.Fatalf("error = %v, want %v", err, stateCase.wantErr)
				}
				if granted != stateCase.wantGranted {
					t.Fatalf("granted = %v, want %v", granted, stateCase.wantGranted)
				}
				if provider.resolveCalls != 1 || provider.resolvedGoal.ID != "goal-1" || provider.resolvedGoal.ObjectiveRevision() != 2 {
					t.Fatalf("resolver calls = %d, goal = %#v", provider.resolveCalls, provider.resolvedGoal)
				}
				if !stateCase.wantGranted {
					if goalContext != "" || authority.valid() {
						t.Fatalf("unbound result context=%q authority=%+v", goalContext, authority)
					}
					return
				}
				if goalContext != "authoritative goal context" ||
					authority.SessionKey != "room:group:conversation-1" ||
					authority.GoalID != "goal-1" ||
					authority.ObjectiveRevision != 2 ||
					authority.ExecutionID != "execution-1" ||
					authority.RootRoundID != "root-round-1" ||
					authority.Source != roomGoalAuthorityExecutionBinding {
					t.Fatalf("confirmed result context=%q authority=%+v", goalContext, authority)
				}
			})
		}
	}
}

func TestRoomWorkGoalAuthorityRejectsConfirmedExecutionMismatch(t *testing.T) {
	provider := &roomExecutionGoalAuthorityProvider{
		binding: orchestrationsvc.RuntimeGoalBinding{
			ExecutionID:           "execution-1",
			SessionKey:            "room:group:conversation-1",
			GoalID:                "goal-1",
			GoalObjectiveRevision: 1,
		},
		resolution: protocol.GoalExecutionBindingResolution{
			State:       protocol.GoalExecutionBindingStateConfirmed,
			ExecutionID: "execution-other",
		},
	}
	service := &Service{
		executionContext: provider,
		goals: &fakeRoomGoalContextProvider{
			runtimeContexts: map[string]string{"room:group:conversation-1": "goal"},
			runtimeGoals: map[string]*protocol.Goal{
				"room:group:conversation-1": {
					ID:         "goal-1",
					SessionKey: "room:group:conversation-1",
				},
			},
		},
	}

	_, _, granted, err := service.resolveExecutionGoalMutationAuthority(
		context.Background(),
		orchestrationsvc.ActorContext{WorkBinding: &protocol.ExecutionWorkBinding{ExecutionID: "execution-1"}},
		"root-round-1",
	)
	if !errors.Is(err, goalsvc.ErrGoalExecutionBindingConflict) || granted {
		t.Fatalf("granted=%v error=%v", granted, err)
	}
}

func TestRoomWorkGoalAuthorityFailsClosedWithoutCentralResolver(t *testing.T) {
	provider := &roomExecutionGoalBindingOnlyProvider{
		binding: orchestrationsvc.RuntimeGoalBinding{
			ExecutionID:           "execution-1",
			SessionKey:            "room:group:conversation-1",
			GoalID:                "goal-1",
			GoalObjectiveRevision: 1,
		},
	}
	service := &Service{
		executionContext: provider,
		goals: &fakeRoomGoalContextProvider{
			runtimeContexts: map[string]string{"room:group:conversation-1": "goal"},
			runtimeGoals: map[string]*protocol.Goal{
				"room:group:conversation-1": {
					ID:         "goal-1",
					SessionKey: "room:group:conversation-1",
				},
			},
		},
	}

	_, _, granted, err := service.resolveExecutionGoalMutationAuthority(
		context.Background(),
		orchestrationsvc.ActorContext{WorkBinding: &protocol.ExecutionWorkBinding{ExecutionID: "execution-1"}},
		"root-round-1",
	)
	if !errors.Is(err, goalsvc.ErrGoalExecutionBindingConflict) || granted {
		t.Fatalf("granted=%v error=%v", granted, err)
	}
}

type roomExecutionGoalBindingOnlyProvider struct {
	binding orchestrationsvc.RuntimeGoalBinding
}

func (p *roomExecutionGoalBindingOnlyProvider) RuntimeContext(
	context.Context,
	orchestrationsvc.ActorContext,
) (string, error) {
	return "", nil
}

func (p *roomExecutionGoalBindingOnlyProvider) RuntimeGoalBinding(
	context.Context,
	orchestrationsvc.ActorContext,
) (orchestrationsvc.RuntimeGoalBinding, error) {
	return p.binding, nil
}
