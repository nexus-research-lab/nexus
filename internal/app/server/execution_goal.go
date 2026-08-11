// INPUT: 已通过 Orchestration policy 的权威 Execution snapshot、当前 actor 与 Goal 配置。
// OUTPUT: 幂等创建或复用且在 SQL BindGoal 前保持 pending 的 adaptive Goal identity/revision。
// POS: Execution Orchestration 到 Goal 生命周期服务的应用层防腐适配器。
package server

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	goalsvc "github.com/nexus-research-lab/nexus/internal/service/goal"
	orchestrationsvc "github.com/nexus-research-lab/nexus/internal/service/orchestration"
)

type executionGoalService interface {
	CurrentOptional(context.Context, string) (*protocol.Goal, error)
	Create(context.Context, protocol.CreateGoalRequest) (*protocol.Goal, error)
}

type executionGoalPromotionGateway struct {
	config config.Config
	goals  executionGoalService
}

type executionGoalCompletionReadiness struct {
	orchestration *orchestrationsvc.Service
}

func (r executionGoalCompletionReadiness) ResolveGoalExecutionBinding(
	ctx context.Context,
	goal protocol.Goal,
) (protocol.GoalExecutionBindingResolution, error) {
	if r.orchestration == nil {
		return protocol.GoalExecutionBindingResolution{
			State: protocol.GoalExecutionBindingStateStandalone,
		}, nil
	}
	return r.orchestration.ResolveGoalExecutionBinding(ctx, goal)
}

func (r executionGoalCompletionReadiness) ValidateGoalRevisionOwner(
	ctx context.Context,
	executionID string,
	goalID string,
	goalObjectiveRevision int64,
	expectedOwnerUserID string,
) (bool, error) {
	if r.orchestration == nil {
		return false, errors.New("Execution Goal owner verifier is unavailable")
	}
	return r.orchestration.ValidateGoalRevisionOwner(
		ctx,
		executionID,
		goalID,
		goalObjectiveRevision,
		expectedOwnerUserID,
	)
}

func (r executionGoalCompletionReadiness) ExecutionGoalCompletionBlocker(
	ctx context.Context,
	goal protocol.Goal,
) (string, error) {
	if r.orchestration == nil {
		return "", nil
	}
	return r.orchestration.GoalExecutionCompletionBlocker(ctx, goal)
}

func newExecutionGoalPromotionGateway(
	cfg config.Config,
	goals executionGoalService,
) *executionGoalPromotionGateway {
	return &executionGoalPromotionGateway{config: cfg, goals: goals}
}

func (g *executionGoalPromotionGateway) ReadGoalPromotionAvailability(
	ctx context.Context,
	request orchestrationsvc.GoalPromotionAvailabilityRequest,
) (orchestrationsvc.GoalPromotionAvailability, error) {
	var availability orchestrationsvc.GoalPromotionAvailability
	if g == nil || g.goals == nil {
		return availability, errors.New("Goal promotion policy is unavailable")
	}
	if !g.config.GoalEnabled || !g.config.GoalAutoContinueEnabled {
		availability.AutomaticGoalDisabled = true
		return availability, nil
	}
	if request.Snapshot == nil {
		return availability, errors.New("Goal promotion availability requires an Execution snapshot")
	}
	execution := request.Snapshot.Execution
	if strings.TrimSpace(execution.ID) == "" ||
		strings.TrimSpace(execution.SessionKey) == "" ||
		strings.TrimSpace(execution.Objective) == "" {
		return availability, errors.New(
			"Goal promotion availability requires execution id, session key and objective",
		)
	}
	current, err := g.goals.CurrentOptional(
		goalsvc.WithActiveGoalContinuationSuppressed(ctx),
		execution.SessionKey,
	)
	if err != nil {
		return availability, mapExecutionGoalPromotionError(err)
	}
	if current == nil {
		return availability, nil
	}
	if _, err = bindingForExistingExecutionGoal(
		*current,
		execution.ID,
		execution.Objective,
		protocol.GoalActivationReasonObservedBoundary,
	); err == nil {
		// A compatible Goal may have been created before BindGoal hit a CAS
		// conflict. Advertising retry is safe and preserves idempotency.
		return availability, nil
	}
	if errors.Is(err, orchestrationsvc.ErrGoalPromotionConflict) {
		availability.ConflictingGoalID = strings.TrimSpace(current.ID)
		return availability, nil
	}
	return availability, err
}

func (g *executionGoalPromotionGateway) PromoteExecution(
	ctx context.Context,
	request orchestrationsvc.GoalPromotionRequest,
) (orchestrationsvc.GoalPromotionBinding, error) {
	if g == nil || g.goals == nil ||
		!g.config.GoalEnabled ||
		!g.config.GoalAutoContinueEnabled {
		return orchestrationsvc.GoalPromotionBinding{}, orchestrationsvc.ErrGoalPromotionDisabled
	}
	if request.Snapshot == nil {
		return orchestrationsvc.GoalPromotionBinding{}, errors.New("Goal promotion requires an Execution snapshot")
	}

	execution := request.Snapshot.Execution
	executionID := strings.TrimSpace(execution.ID)
	sessionKey := strings.TrimSpace(execution.SessionKey)
	objective := strings.TrimSpace(execution.Objective)
	commandID := strings.TrimSpace(request.CommandID)
	if executionID == "" || sessionKey == "" || objective == "" || commandID == "" {
		return orchestrationsvc.GoalPromotionBinding{}, errors.New(
			"Goal promotion requires execution id, session key, objective and command id",
		)
	}

	promotionContext := goalsvc.WithActiveGoalContinuationSuppressed(ctx)
	current, err := g.goals.CurrentOptional(promotionContext, sessionKey)
	if err != nil {
		return orchestrationsvc.GoalPromotionBinding{}, mapExecutionGoalPromotionError(err)
	}
	if current != nil {
		return bindingForExistingExecutionGoal(
			*current,
			executionID,
			objective,
			request.Proposal.ActivationReason,
		)
	}

	criteria := append([]string(nil), execution.CompletionCriteria...)
	roundID := strings.TrimSpace(request.Actor.RootRoundID)
	if roundID == "" {
		roundID = strings.TrimSpace(execution.RootRoundID)
	}
	created, err := g.goals.Create(promotionContext, protocol.CreateGoalRequest{
		SessionKey:      sessionKey,
		Objective:       objective,
		ReplaceExisting: false,
		CreatedBy:       "model",
		RoundID:         roundID,
		OwnerUserID:     strings.TrimSpace(execution.OwnerUserID),
		AgentID:         strings.TrimSpace(request.Actor.AgentID),
		Metadata: map[string]any{
			protocol.GoalMetadataExecutionID: executionID,
			protocol.GoalMetadataExecutionBindingState: string(
				protocol.GoalExecutionBindingStatePending,
			),
			protocol.GoalMetadataPromotionCommand:   commandID,
			protocol.GoalMetadataActivationOrigin:   string(protocol.GoalActivationOriginAdaptivePromoted),
			protocol.GoalMetadataActivationReason:   string(request.Proposal.ActivationReason),
			protocol.GoalMetadataCompletionCriteria: criteria,
		},
	})
	if errors.Is(err, goalsvc.ErrGoalConflict) {
		current, loadErr := g.goals.CurrentOptional(promotionContext, sessionKey)
		if loadErr != nil {
			return orchestrationsvc.GoalPromotionBinding{}, mapExecutionGoalPromotionError(loadErr)
		}
		if current != nil {
			return bindingForExistingExecutionGoal(
				*current,
				executionID,
				objective,
				request.Proposal.ActivationReason,
			)
		}
	}
	if err != nil {
		return orchestrationsvc.GoalPromotionBinding{}, mapExecutionGoalPromotionError(err)
	}
	if created == nil || strings.TrimSpace(created.ID) == "" {
		return orchestrationsvc.GoalPromotionBinding{}, errors.New("Goal service created no Goal identity")
	}
	return executionGoalPromotionBinding(*created, request.Proposal.ActivationReason), nil
}

func bindingForExistingExecutionGoal(
	goal protocol.Goal,
	executionID string,
	objective string,
	fallbackReason protocol.GoalActivationReason,
) (orchestrationsvc.GoalPromotionBinding, error) {
	bindingState := protocol.GoalExecutionBindingStateFromGoal(goal)
	if protocol.GoalMetadataString(goal.Metadata, protocol.GoalMetadataExecutionID) !=
		strings.TrimSpace(executionID) ||
		(bindingState != protocol.GoalExecutionBindingStateStandalone &&
			bindingState != protocol.GoalExecutionBindingStatePending &&
			bindingState != protocol.GoalExecutionBindingStateConfirmed) ||
		protocol.GoalMetadataString(goal.Metadata, protocol.GoalMetadataActivationOrigin) !=
			string(protocol.GoalActivationOriginAdaptivePromoted) ||
		strings.TrimSpace(goal.Objective) != strings.TrimSpace(objective) {
		return orchestrationsvc.GoalPromotionBinding{}, orchestrationsvc.ErrGoalPromotionConflict
	}
	reason := fallbackReason
	if stored := protocol.GoalMetadataString(goal.Metadata, protocol.GoalMetadataActivationReason); stored != "" {
		reason = protocol.GoalActivationReason(stored)
	}
	return executionGoalPromotionBinding(goal, reason), nil
}

func executionGoalPromotionBinding(
	goal protocol.Goal,
	reason protocol.GoalActivationReason,
) orchestrationsvc.GoalPromotionBinding {
	return orchestrationsvc.GoalPromotionBinding{
		GoalID:                strings.TrimSpace(goal.ID),
		GoalObjectiveRevision: goal.ObjectiveRevision(),
		ActivationOrigin:      protocol.GoalActivationOriginAdaptivePromoted,
		ActivationReason:      reason,
	}
}

func mapExecutionGoalPromotionError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, goalsvc.ErrGoalDisabled):
		return orchestrationsvc.ErrGoalPromotionDisabled
	case errors.Is(err, goalsvc.ErrGoalConflict):
		return orchestrationsvc.ErrGoalPromotionConflict
	default:
		return fmt.Errorf("promote Execution to Goal: %w", err)
	}
}
