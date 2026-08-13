// INPUT: coordinator semantic abandon/replace intent and trusted current Execution identity.
// OUTPUT: transient-only objective-boundary validation, successor construction, stable mutation results, and post-commit session invalidation.
// POS: service authority boundary between ordinary replanning and whole-Execution lifecycle transitions.
package orchestration

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

// AbandonExecution cancels one transient Execution without creating a successor.
func (s *Service) AbandonExecution(
	ctx context.Context,
	actor ActorContext,
	input AbandonExecutionInput,
) (returned MutationResult, returnedErr error) {
	defer func() { s.invalidateMutationResult(ctx, returned, returnedErr) }()
	if err := validateActor(actor); err != nil {
		return RejectedResult(nil, err, nil), nil
	}
	if strings.TrimSpace(input.CommandID) == "" {
		return RejectedResult(nil, domainError(ErrorCodeInvalidInput, "command_id is required"), nil), nil
	}
	input.Reason = strings.TrimSpace(input.Reason)
	if input.Reason == "" {
		return RejectedResult(nil, domainError(ErrorCodeInvalidInput, "reason is required"), nil), nil
	}
	snapshot, err := s.GetSnapshot(ctx, actor, strings.TrimSpace(input.ExecutionID))
	if err != nil {
		var domainErr *DomainError
		if errors.As(err, &domainErr) {
			return RejectedResult(nil, err, nil), nil
		}
		return MutationResult{}, err
	}
	if snapshot == nil {
		return RejectedResult(nil, domainError(ErrorCodeNoCurrentExecution, "execution was not found"), nil), nil
	}
	if coordinationErr := s.requireRuntimeCoordination(actor, snapshot); coordinationErr != nil {
		return RejectedResult(snapshot, coordinationErr, []NextAction{{
			Tool:   "get_execution",
			Reason: "explicitly inspect and enter the current Room coordination scope",
		}}), nil
	}
	if authErr := requireCoordinator(actor, snapshot); authErr != nil {
		return RejectedResult(snapshot, authErr, nil), nil
	}
	if strings.TrimSpace(snapshot.Execution.GoalID) != "" {
		return RejectedResult(snapshot, goalRetargetRequiredError(), []NextAction{{
			Tool:   "retarget_goal",
			Reason: "Goal-bound objectives must advance through the Goal objective revision protocol",
		}}), nil
	}
	if actor.PlanMode {
		if !isCurrentExecutionStatus(snapshot.Execution.Status) {
			return RejectedResult(snapshot, terminalExecutionError(), nil), nil
		}
		if revisionErr := requireMutationRevision(snapshot, input.SnapshotRevision); revisionErr != nil {
			return RejectedResult(snapshot, revisionErr, nextActions(snapshot, actor)), nil
		}
		result := NoOpResult(
			snapshot,
			"Abandon proposal is valid; Plan Mode did not cancel the Execution or any execution chain.",
		)
		result.NextActions = []NextAction{{
			Tool:   "abandon_execution",
			Reason: "leave Plan Mode and resubmit to cancel this transient Execution",
		}}
		return result, nil
	}
	wasTerminal := !isCurrentExecutionStatus(snapshot.Execution.Status)
	if !wasTerminal {
		if revisionErr := requireMutationRevision(snapshot, input.SnapshotRevision); revisionErr != nil {
			return RejectedResult(snapshot, revisionErr, nextActions(snapshot, actor)), nil
		}
	}
	updated, abandonErr := s.repository.Abandon(ctx, orchestrationstore.AbandonCommand{
		ExecutionID:              snapshot.Execution.ID,
		ExpectedExecutionVersion: input.SnapshotRevision,
		Reason:                   input.Reason,
		Meta:                     s.commandMeta(actor, input.CommandID, "abandon"),
	})
	if abandonErr != nil {
		if wasTerminal {
			return RejectedResult(snapshot, terminalExecutionError(), nil), nil
		}
		return s.storageMutationResult(snapshot, abandonErr, nextActions(snapshot, actor))
	}
	if wasTerminal {
		return NoOpResult(updated, "Execution was already abandoned by this command"), nil
	}
	return AppliedResult(updated, []string{
		"execution_cancelled:" + updated.Execution.ID,
	}, nil), nil
}

func (s *Service) buildExecutionForPlan(
	ctx context.Context,
	actor ActorContext,
	objective string,
	completionCriteria []string,
	replacesExecutionID string,
	reservedExecutionID string,
	sealedGoalBinding *ExplicitGoalBinding,
	allowExplicitGoal bool,
) (protocol.Execution, error) {
	scope := actor.ScopeKind
	if scope == "" {
		scope = protocol.ExecutionScopeDM
	}
	executionID := strings.TrimSpace(reservedExecutionID)
	if executionID == "" {
		executionID = s.id("execution")
	}
	execution := protocol.Execution{
		ID:                  executionID,
		OwnerUserID:         strings.TrimSpace(actor.OwnerUserID),
		SessionKey:          strings.TrimSpace(actor.SessionKey),
		ScopeKind:           scope,
		CoordinatorAgentID:  strings.TrimSpace(actor.AgentID),
		Origin:              protocol.ExecutionOriginUserRequest,
		Objective:           strings.TrimSpace(objective),
		CompletionCriteria:  slices.Clone(completionCriteria),
		ReplacesExecutionID: strings.TrimSpace(replacesExecutionID),
		RootRoundID:         strings.TrimSpace(actor.RootRoundID),
		Status:              protocol.ExecutionStatusActive,
	}
	if sealedGoalBinding != nil && strings.TrimSpace(sealedGoalBinding.GoalID) != "" &&
		strings.TrimSpace(sealedGoalBinding.ExecutionID) == "" {
		sealedGoalBinding = &ExplicitGoalBinding{
			ExecutionID:           execution.ID,
			GoalID:                sealedGoalBinding.GoalID,
			GoalObjectiveRevision: sealedGoalBinding.GoalObjectiveRevision,
			ActivationOrigin:      sealedGoalBinding.ActivationOrigin,
			ActivationReason:      sealedGoalBinding.ActivationReason,
			ReplacesExecutionID:   sealedGoalBinding.ReplacesExecutionID,
		}
	}
	if scope == protocol.ExecutionScopeRoom {
		execution.RoomID = strings.TrimSpace(actor.RoomID)
		execution.ConversationID = strings.TrimSpace(actor.ConversationID)
	}
	if !allowExplicitGoal {
		return execution, nil
	}
	if sealedGoalBinding != nil && strings.TrimSpace(sealedGoalBinding.GoalID) == "" {
		return execution, nil
	}
	binding, err := s.prepareExplicitGoalBinding(ctx, actor, execution, false)
	if err != nil {
		return protocol.Execution{}, mapExplicitGoalGatewayError(err)
	}
	if binding == nil {
		if sealedGoalBinding != nil {
			return protocol.Execution{}, domainError(
				ErrorCodeGoalBindingConflict,
				"sealed Goal binding is no longer available",
			)
		}
		return execution, nil
	}
	if err = validateExplicitGoalBinding(*binding); err != nil {
		return protocol.Execution{}, err
	}
	if sealedGoalBinding != nil && !explicitGoalBindingsEqual(*binding, *sealedGoalBinding) {
		return protocol.Execution{}, domainError(
			ErrorCodeGoalBindingConflict,
			"Goal binding changed after the Plan proposal was sealed",
		)
	}
	execution.ID = strings.TrimSpace(binding.ExecutionID)
	execution.GoalID = strings.TrimSpace(binding.GoalID)
	execution.GoalObjectiveRevision = binding.GoalObjectiveRevision
	execution.GoalActivationOrigin = binding.ActivationOrigin
	execution.GoalActivationReason = binding.ActivationReason
	execution.ReplacesExecutionID = strings.TrimSpace(binding.ReplacesExecutionID)
	return execution, nil
}

func explicitGoalBindingsEqual(left, right ExplicitGoalBinding) bool {
	return strings.TrimSpace(left.ExecutionID) == strings.TrimSpace(right.ExecutionID) &&
		strings.TrimSpace(left.GoalID) == strings.TrimSpace(right.GoalID) &&
		left.GoalObjectiveRevision == right.GoalObjectiveRevision &&
		left.ActivationOrigin == right.ActivationOrigin &&
		left.ActivationReason == right.ActivationReason &&
		strings.TrimSpace(left.ReplacesExecutionID) == strings.TrimSpace(right.ReplacesExecutionID)
}

func validateExecutionBoundary(
	objective string,
	completionCriteria []string,
) (string, []string, error) {
	return validateNewExecutionProposal(EnsureInput{
		Objective:          objective,
		CompletionCriteria: completionCriteria,
	})
}

func validateOrdinaryReplanBoundary(
	snapshot *protocol.ExecutionSnapshot,
	objective string,
	completionCriteria []string,
) error {
	objective = strings.TrimSpace(objective)
	if objective != "" && objective != snapshot.Execution.Objective {
		return objectiveReplacementRequiredError()
	}
	if completionCriteria != nil {
		if err := newProjectionLimitError(
			"completion_criteria",
			len(completionCriteria),
			"",
		); err != nil {
			return err
		}
		criteria := normalizeNonEmptyValues(completionCriteria)
		if !slices.Equal(criteria, normalizeNonEmptyValues(snapshot.Execution.CompletionCriteria)) {
			return objectiveReplacementRequiredError()
		}
	}
	return nil
}

func validateReplacementBoundary(
	snapshot *protocol.ExecutionSnapshot,
	input PlanExecutionInput,
) (string, []string, error) {
	if strings.TrimSpace(snapshot.Execution.GoalID) != "" {
		return "", nil, goalRetargetRequiredError()
	}
	if strings.TrimSpace(input.ReplacementReason) == "" {
		return "", nil, domainError(
			ErrorCodeInvalidInput,
			"replacement_reason is required for operation: replace",
		)
	}
	if input.SupersedeActiveWork {
		return "", nil, domainError(
			ErrorCodeInvalidInput,
			"supersede_active_work belongs to same-Execution replanning and cannot be combined with Execution replacement",
		)
	}
	objective, criteria, err := validateExecutionBoundary(input.Objective, input.CompletionCriteria)
	if err != nil {
		return "", nil, err
	}
	if objective == snapshot.Execution.Objective {
		return "", nil, domainError(
			ErrorCodeInvalidInput,
			"operation: replace requires a different objective; prepare an operation: replan document for the same objective",
		)
	}
	for _, item := range input.Draft.Items {
		if strings.TrimSpace(item.ExistingWorkItemID) != "" {
			return "", nil, newDomainError(
				ErrorCodeInvalidInput,
				"replacement cannot carry a Work Item identity or Acceptance across Executions",
				item.LogicalKey,
				item.ExistingWorkItemID,
			)
		}
	}
	return objective, criteria, nil
}

func requireExecutionCoordinator(actor ActorContext) error {
	if actor.Role == ExecutionActorCoordinator {
		return nil
	}
	if actor.Role == "" && actor.ScopeKind != protocol.ExecutionScopeRoom {
		return nil
	}
	return domainError(ErrorCodeWrongOwner, "only the execution coordinator may perform this operation")
}

func isCurrentExecutionStatus(status protocol.ExecutionStatus) bool {
	return status == protocol.ExecutionStatusActive ||
		status == protocol.ExecutionStatusWaiting ||
		status == protocol.ExecutionStatusPaused
}

func terminalExecutionError() error {
	return domainError(
		ErrorCodeExecutionTerminal,
		"Execution is terminal and cannot accept further mutations",
	)
}

func objectiveReplacementRequiredError() error {
	return domainError(
		ErrorCodeObjectiveChangeReplace,
		"an existing Execution objective or completion criteria cannot be rewritten by a Plan revision; prepare an operation: replace document with a complete successor boundary",
	)
}

func goalRetargetRequiredError() error {
	return domainError(
		ErrorCodeGoalRetargetRequired,
		"Goal-bound Execution objective changes must use the Goal retarget and objective-revision protocol",
	)
}
