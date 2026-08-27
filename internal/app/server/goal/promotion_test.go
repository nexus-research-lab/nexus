package goal

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

type promotionGoalServiceStub struct {
	current       *protocol.Goal
	currentErr    error
	createResult  *protocol.Goal
	createErr     error
	createRequest protocol.CreateGoalRequest
	currentCalls  int
	createCalls   int
}

func (s *promotionGoalServiceStub) CurrentOptional(
	context.Context,
	string,
) (*protocol.Goal, error) {
	s.currentCalls++
	return s.current, s.currentErr
}

func (s *promotionGoalServiceStub) Create(
	_ context.Context,
	request protocol.CreateGoalRequest,
) (*protocol.Goal, error) {
	s.createCalls++
	s.createRequest = request
	if s.createResult != nil || s.createErr != nil {
		return s.createResult, s.createErr
	}
	return &protocol.Goal{
		ID:         "goal-created",
		SessionKey: request.SessionKey,
		Objective:  request.Objective,
		Status:     protocol.GoalStatusActive,
		Version:    1,
		Metadata:   request.Metadata,
	}, nil
}

func TestPromotionGatewayRespectsFeatureConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		config config.Config
	}{
		{
			name:   "Goal disabled",
			config: config.Config{GoalAutoContinueEnabled: true},
		},
		{
			name:   "automatic continuation disabled",
			config: config.Config{GoalEnabled: true},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			goals := &promotionGoalServiceStub{}
			gateway := NewExecutionPromotionGateway(test.config, goals)
			_, err := gateway.PromoteExecution(context.Background(), promotionRequest())
			if !errors.Is(err, orchestrationsvc.ErrGoalPromotionDisabled) {
				t.Fatalf("PromoteExecution() error = %v, want disabled", err)
			}
			if goals.currentCalls != 0 || goals.createCalls != 0 {
				t.Fatalf("disabled gateway called Goal service: current=%d create=%d", goals.currentCalls, goals.createCalls)
			}
			availability, availabilityErr := gateway.ReadGoalPromotionAvailability(
				context.Background(),
				orchestrationsvc.GoalPromotionAvailabilityRequest{
					Snapshot: promotionRequest().Snapshot,
				},
			)
			if availabilityErr != nil || !availability.AutomaticGoalDisabled {
				t.Fatalf("availability = %+v, err=%v", availability, availabilityErr)
			}
		})
	}
}

func TestPromotionGatewayCreatesFromCanonicalExecution(t *testing.T) {
	goals := &promotionGoalServiceStub{
		createResult: &protocol.Goal{
			ID:         "goal-1",
			SessionKey: "room:group:conversation-1",
			Objective:  "Canonical Execution objective",
			Status:     protocol.GoalStatusActive,
			Version:    1,
			Metadata: map[string]any{
				protocol.GoalMetadataObjectiveRevision: int64(4),
			},
		},
	}
	gateway := NewExecutionPromotionGateway(config.Config{
		GoalEnabled:             true,
		GoalAutoContinueEnabled: true,
	}, goals)
	request := promotionRequest()
	request.Proposal.ObjectiveProposal = "Broaden scope beyond the Execution"

	binding, err := gateway.PromoteExecution(context.Background(), request)
	if err != nil {
		t.Fatalf("PromoteExecution() error = %v", err)
	}
	if binding.GoalID != "goal-1" ||
		binding.GoalObjectiveRevision != 4 ||
		binding.ActivationOrigin != protocol.GoalActivationOriginAdaptivePromoted ||
		binding.ActivationReason != protocol.GoalActivationReasonRoomDependencyChain {
		t.Fatalf("binding = %+v", binding)
	}

	created := goals.createRequest
	if created.Objective != "Canonical Execution objective" ||
		created.Objective == request.Proposal.ObjectiveProposal ||
		created.SessionKey != "room:group:conversation-1" ||
		created.CreatedBy != "model" ||
		created.RoundID != "root-round-current" ||
		created.OwnerUserID != "owner-1" ||
		created.AgentID != "agent-lead" ||
		created.ReplaceExisting {
		t.Fatalf("CreateGoalRequest = %+v", created)
	}
	if got := protocol.GoalMetadataString(created.Metadata, protocol.GoalMetadataExecutionID); got != "execution-1" {
		t.Fatalf("execution metadata = %q", got)
	}
	if got := protocol.GoalMetadataString(
		created.Metadata,
		protocol.GoalMetadataExecutionBindingState,
	); got != string(protocol.GoalExecutionBindingStatePending) {
		t.Fatalf("execution binding state metadata = %q, want pending", got)
	}
	if got := protocol.GoalMetadataString(created.Metadata, protocol.GoalMetadataPromotionCommand); got != "command-1" {
		t.Fatalf("promotion command metadata = %q", got)
	}
	if got := protocol.GoalMetadataString(created.Metadata, protocol.GoalMetadataActivationOrigin); got != string(protocol.GoalActivationOriginAdaptivePromoted) {
		t.Fatalf("activation origin metadata = %q", got)
	}
	if got := protocol.GoalMetadataString(created.Metadata, protocol.GoalMetadataActivationReason); got != string(protocol.GoalActivationReasonRoomDependencyChain) {
		t.Fatalf("activation reason metadata = %q", got)
	}
	if got, ok := created.Metadata[protocol.GoalMetadataCompletionCriteria].([]string); !ok ||
		!reflect.DeepEqual(got, []string{"report accepted", "tests pass"}) {
		t.Fatalf("completion criteria metadata = %#v", created.Metadata[protocol.GoalMetadataCompletionCriteria])
	}
}

func TestPromotionGatewayReusesGoalForSameExecution(t *testing.T) {
	goals := &promotionGoalServiceStub{current: &protocol.Goal{
		ID:         "goal-existing",
		SessionKey: "room:group:conversation-1",
		Objective:  "Canonical Execution objective",
		Status:     protocol.GoalStatusActive,
		Metadata: map[string]any{
			protocol.GoalMetadataExecutionID:        "execution-1",
			protocol.GoalMetadataObjectiveRevision:  int64(7),
			protocol.GoalMetadataActivationReason:   string(protocol.GoalActivationReasonExternalWait),
			protocol.GoalMetadataPromotionCommand:   "original-command",
			protocol.GoalMetadataActivationOrigin:   string(protocol.GoalActivationOriginAdaptivePromoted),
			protocol.GoalMetadataCompletionCriteria: []string{"report accepted"},
		},
	}}
	gateway := NewExecutionPromotionGateway(config.Config{
		GoalEnabled:             true,
		GoalAutoContinueEnabled: true,
	}, goals)

	binding, err := gateway.PromoteExecution(context.Background(), promotionRequest())
	if err != nil {
		t.Fatalf("PromoteExecution() error = %v", err)
	}
	if binding.GoalID != "goal-existing" ||
		binding.GoalObjectiveRevision != 7 ||
		binding.ActivationReason != protocol.GoalActivationReasonExternalWait {
		t.Fatalf("reused binding = %+v", binding)
	}
	if goals.createCalls != 0 {
		t.Fatalf("idempotent reuse created %d Goals", goals.createCalls)
	}
}

func TestPromotionGatewayRejectsUnrelatedCurrentGoal(t *testing.T) {
	goals := &promotionGoalServiceStub{current: &protocol.Goal{
		ID:         "goal-unrelated",
		SessionKey: "room:group:conversation-1",
		Status:     protocol.GoalStatusActive,
		Metadata: map[string]any{
			protocol.GoalMetadataExecutionID: "execution-other",
		},
	}}
	gateway := NewExecutionPromotionGateway(config.Config{
		GoalEnabled:             true,
		GoalAutoContinueEnabled: true,
	}, goals)

	_, err := gateway.PromoteExecution(context.Background(), promotionRequest())
	if !errors.Is(err, orchestrationsvc.ErrGoalPromotionConflict) {
		t.Fatalf("PromoteExecution() error = %v, want conflict", err)
	}
	if goals.createCalls != 0 {
		t.Fatalf("conflicting active Goal triggered %d creates", goals.createCalls)
	}
	availability, availabilityErr := gateway.ReadGoalPromotionAvailability(
		context.Background(),
		orchestrationsvc.GoalPromotionAvailabilityRequest{
			Snapshot: promotionRequest().Snapshot,
		},
	)
	if availabilityErr != nil || availability.ConflictingGoalID != "goal-unrelated" {
		t.Fatalf("availability = %+v, err=%v", availability, availabilityErr)
	}
}

func TestPromotionGatewayRejectsSpoofedOrRetargetedBinding(t *testing.T) {
	tests := []struct {
		name     string
		goal     protocol.Goal
		metadata map[string]any
	}{
		{
			name: "missing adaptive origin",
			goal: protocol.Goal{
				ID:        "goal-spoofed",
				Objective: "Canonical Execution objective",
			},
			metadata: map[string]any{
				protocol.GoalMetadataExecutionID: "execution-1",
			},
		},
		{
			name: "retargeted objective",
			goal: protocol.Goal{
				ID:        "goal-retargeted",
				Objective: "A different objective",
			},
			metadata: map[string]any{
				protocol.GoalMetadataExecutionID:        "execution-1",
				protocol.GoalMetadataActivationOrigin:   string(protocol.GoalActivationOriginAdaptivePromoted),
				protocol.GoalMetadataActivationReason:   string(protocol.GoalActivationReasonRoomDependencyChain),
				protocol.GoalMetadataPromotionCommand:   "command-1",
				protocol.GoalMetadataCompletionCriteria: []string{"report accepted"},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.goal.SessionKey = "room:group:conversation-1"
			test.goal.Status = protocol.GoalStatusActive
			test.goal.Metadata = test.metadata
			goals := &promotionGoalServiceStub{current: &test.goal}
			gateway := NewExecutionPromotionGateway(config.Config{
				GoalEnabled:             true,
				GoalAutoContinueEnabled: true,
			}, goals)

			_, err := gateway.PromoteExecution(context.Background(), promotionRequest())
			if !errors.Is(err, orchestrationsvc.ErrGoalPromotionConflict) {
				t.Fatalf("PromoteExecution() error = %v, want conflict", err)
			}
		})
	}
}

func TestPromotionGatewayMapsConcurrentCreateConflict(t *testing.T) {
	goals := &promotionGoalServiceStub{createErr: goalsvc.ErrGoalConflict}
	gateway := NewExecutionPromotionGateway(config.Config{
		GoalEnabled:             true,
		GoalAutoContinueEnabled: true,
	}, goals)

	_, err := gateway.PromoteExecution(context.Background(), promotionRequest())
	if !errors.Is(err, orchestrationsvc.ErrGoalPromotionConflict) {
		t.Fatalf("PromoteExecution() error = %v, want conflict", err)
	}
}

func promotionRequest() orchestrationsvc.GoalPromotionRequest {
	return orchestrationsvc.GoalPromotionRequest{
		CommandID: "command-1",
		Snapshot: &protocol.ExecutionSnapshot{Execution: protocol.Execution{
			ID:                 "execution-1",
			OwnerUserID:        "owner-1",
			SessionKey:         "room:group:conversation-1",
			ScopeKind:          protocol.ExecutionScopeRoom,
			RoomID:             "room-1",
			ConversationID:     "conversation-1",
			CoordinatorAgentID: "agent-lead",
			Objective:          "Canonical Execution objective",
			CompletionCriteria: []string{"report accepted", "tests pass"},
			RootRoundID:        "root-round-origin",
			Status:             protocol.ExecutionStatusActive,
			Version:            3,
		}},
		Actor: orchestrationsvc.ActorContext{
			OwnerUserID:    "owner-1",
			SessionKey:     "room:group:conversation-1",
			AgentID:        "agent-lead",
			Role:           orchestrationsvc.ExecutionActorCoordinator,
			ActorKind:      protocol.ExecutionActorAgent,
			ScopeKind:      protocol.ExecutionScopeRoom,
			RoomID:         "room-1",
			ConversationID: "conversation-1",
			RootRoundID:    "root-round-current",
		},
		Proposal: orchestrationsvc.GoalPromotionProposal{
			ObjectiveProposal: "Canonical Execution objective",
			ActivationReason:  protocol.GoalActivationReasonRoomDependencyChain,
		},
	}
}
