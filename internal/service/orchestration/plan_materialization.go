// INPUT: 带 canonical root boundary 与显式/继承 Goal fence 的 sealed proposal id+digest、当前 trusted access identity 与 proposal recovery state。
// OUTPUT: sealed Goal ID/revision/objective/reservation exact-fence CAS、稳定 command/Execution identity、原子 Plan materialization 与 durable Goal confirmation receipt。
// POS: ExecutionPlanProposal saga 的唯一权威提交和重放边界；Goal-free seal 不重新发现 ambient Goal。
package orchestration

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

const planProposalRetryDelay = 15 * time.Second

// MaterializePlanExecutionInput 只引用已 seal 的 immutable proposal。
// 文档不能在 commit 调用中重传或被模型改写。
type MaterializePlanExecutionInput struct {
	ProposalID     string
	ProposalDigest string
}

// MaterializePlanExecution 把 sealed proposal 原子转为权威 Execution/Plan。
func (s *Service) MaterializePlanExecution(
	ctx context.Context,
	actor ActorContext,
	input MaterializePlanExecutionInput,
) (MutationResult, error) {
	defer s.WakeOrchestrationRecovery()
	if err := validateActor(actor); err != nil {
		return RejectedResult(nil, err, nil), nil
	}
	if err := requireExecutionCoordinator(actor); err != nil {
		return RejectedResult(nil, err, nil), nil
	}
	if actor.PlanMode {
		return RejectedResult(nil, planModeError(), []NextAction{{
			Tool:   "plan_execution",
			Reason: "leave Plan Mode, then commit the same proposal_id and proposal_digest",
		}}), nil
	}
	proposalID := strings.TrimSpace(input.ProposalID)
	digest := strings.TrimSpace(input.ProposalDigest)
	if proposalID == "" || digest == "" {
		return RejectedResult(nil, domainError(
			ErrorCodeInvalidInput,
			"proposal_id and proposal_digest are required",
		), []NextAction{{
			Tool:   "prepare_plan_execution",
			Reason: "submit one complete Nexus Plan Document before committing it",
		}}), nil
	}
	if s == nil || s.planProposals == nil {
		return MutationResult{}, errors.New("execution plan proposal repository is unavailable")
	}

	access := proposalAccess(actor, proposalID)
	proposal, err := s.planProposals.GetPlanProposal(
		ctx,
		orchestrationstore.GetPlanProposalQuery{Access: access},
	)
	if err != nil {
		if errors.Is(err, orchestrationstore.ErrPlanProposalAccess) {
			return RejectedResult(nil, domainError(
				ErrorCodeWrongOwner,
				"proposal is outside the current owner, session, scope, or coordinator",
			), nil), nil
		}
		return MutationResult{}, err
	}
	if proposal == nil {
		return RejectedResult(nil, domainError(
			ErrorCodePlanProposalNotFound,
			"sealed plan proposal was not found",
		), []NextAction{{
			Tool:   "prepare_plan_execution",
			Reason: "prepare the complete Plan again in the current scope",
		}}), nil
	}
	if !constantTimeDigestEqual(digest, proposal.ContentDigest) {
		return RejectedResult(nil, domainError(
			ErrorCodePlanProposalDigest,
			"proposal_digest does not match the sealed proposal and target fence",
		), []NextAction{{
			Tool:   "prepare_plan_execution",
			Reason: "use the exact proposal_id and proposal_digest returned together by preparation",
		}}), nil
	}
	return s.materializeLoadedPlanProposal(ctx, actor, proposal)
}

func (s *Service) materializeLoadedPlanProposal(
	ctx context.Context,
	actor ActorContext,
	proposal *protocol.ExecutionPlanProposal,
) (MutationResult, error) {
	for attempt := 0; attempt < 3; attempt++ {
		switch proposal.Status {
		case protocol.ExecutionPlanProposalStatusSealed:
			reservedExecutionID := strings.TrimSpace(proposal.TargetExecutionID)
			if proposal.Document.Operation != protocol.ExecutionPlanProposalReplan {
				reservedExecutionID = deterministicProposalExecutionID(
					proposal.ID,
					proposal.ContentDigest,
				)
				if strings.TrimSpace(proposal.GoalReservedExecutionID) != "" {
					reservedExecutionID = strings.TrimSpace(proposal.GoalReservedExecutionID)
				}
			}
			nextAttemptAt := s.currentTime().Add(planProposalRetryDelay)
			updated, err := s.planProposals.MarkPlanProposalMaterializing(
				ctx,
				orchestrationstore.MarkPlanProposalMaterializingCommand{
					Access:              proposalAccess(actor, proposal.ID),
					ExpectedVersion:     proposal.Version,
					ReservedExecutionID: reservedExecutionID,
					MaterializationCommandID: deterministicProposalCommandID(
						proposal.ID,
						proposal.ContentDigest,
					),
					GoalID:                proposal.GoalID,
					GoalObjectiveRevision: proposal.GoalObjectiveRevision,
					GoalActivationOrigin:  proposal.GoalActivationOrigin,
					GoalActivationReason:  proposal.GoalActivationReason,
					ReplacesExecutionID:   proposal.ReplacesExecutionID,
					NextAttemptAt:         &nextAttemptAt,
				},
			)
			if errors.Is(err, orchestrationstore.ErrVersionConflict) ||
				errors.Is(err, orchestrationstore.ErrPlanProposalNotDue) {
				proposal, err = s.reloadPlanProposal(ctx, actor, proposal.ID)
				if err != nil {
					return MutationResult{}, err
				}
				continue
			}
			if err != nil {
				return MutationResult{}, err
			}
			return s.materializeAuthoritativePlan(ctx, actor, updated, true)

		case protocol.ExecutionPlanProposalStatusMaterializing:
			return s.materializeAuthoritativePlan(ctx, actor, proposal, false)

		case protocol.ExecutionPlanProposalStatusMaterialized:
			return s.finishMaterializedPlanProposal(ctx, actor, proposal)

		case protocol.ExecutionPlanProposalStatusBlocked:
			recovered, recoverErr := s.recoverProposalMaterialization(ctx, actor, proposal)
			if recoverErr != nil {
				return MutationResult{}, recoverErr
			}
			if recovered != nil {
				result := NoOpResult(
					recovered.Snapshot,
					"blocked proposal converged through its exact authoritative command receipt",
				)
				result.NextActions = nextActions(recovered.Snapshot, actor)
				return s.recordMaterializedPlanProposal(
					ctx,
					actor,
					proposal,
					result,
					recovered.PlanID,
					false,
					nil,
				)
			}
			return RejectedResult(nil, domainError(
				ErrorCodePlanProposalBlocked,
				firstNonEmptyPlanProposalValue(
					proposal.LastError,
					"proposal can no longer be committed against its sealed target fence",
				),
			), []NextAction{{
				Tool:   "prepare_plan_execution",
				Reason: "refresh the current Execution and prepare a new complete proposal",
			}}), nil

		case protocol.ExecutionPlanProposalStatusDiscarded:
			return RejectedResult(nil, domainError(
				ErrorCodePlanProposalBlocked,
				"proposal was discarded and cannot be committed",
			), []NextAction{{
				Tool:   "prepare_plan_execution",
				Reason: "prepare a new complete proposal",
			}}), nil

		default:
			return MutationResult{}, fmt.Errorf(
				"unknown execution plan proposal status %q",
				proposal.Status,
			)
		}
	}
	return MutationResult{}, errors.New("execution plan proposal changed concurrently")
}

func (s *Service) materializeAuthoritativePlan(
	ctx context.Context,
	actor ActorContext,
	proposal *protocol.ExecutionPlanProposal,
	leaseOwned bool,
) (MutationResult, error) {
	recovered, err := s.recoverProposalMaterialization(ctx, actor, proposal)
	if err != nil {
		return s.deferPlanProposalRetry(ctx, actor, proposal, err)
	}
	if recovered != nil {
		result := NoOpResult(recovered.Snapshot, "sealed Plan proposal was already materialized")
		result.NextActions = nextActions(recovered.Snapshot, actor)
		return s.recordMaterializedPlanProposal(
			ctx,
			actor,
			proposal,
			result,
			recovered.PlanID,
			false,
			nil,
		)
	}
	if !leaseOwned {
		now := s.currentTime()
		claimed, claimErr := s.planProposals.ClaimPlanProposalMaterializing(
			ctx,
			orchestrationstore.ClaimPlanProposalMaterializingCommand{
				Access:          proposalAccess(actor, proposal.ID),
				ExpectedVersion: proposal.Version,
				ClaimAt:         now,
				LeaseUntil:      now.Add(planProposalRetryDelay),
			},
		)
		if errors.Is(claimErr, orchestrationstore.ErrPlanProposalNotDue) ||
			errors.Is(claimErr, orchestrationstore.ErrVersionConflict) {
			return NoOpResult(
				nil,
				"sealed Plan proposal materialization is already in progress or scheduled for recovery",
			), nil
		}
		if claimErr != nil {
			return MutationResult{}, claimErr
		}
		proposal = claimed
	}

	target, err := s.validateProposalTargetFence(ctx, actor, proposal)
	if err != nil {
		var domainErr *DomainError
		if errors.As(err, &domainErr) {
			return s.blockPlanProposal(ctx, actor, proposal, err)
		}
		return s.deferPlanProposalRetry(ctx, actor, proposal, err)
	}
	materializationActor := proposalActor(proposal)
	materializationActor.ActorKind = normalizeActorKind(actor.ActorKind)
	materializationActor.ExecutionID = strings.TrimSpace(proposal.TargetExecutionID)

	document := proposal.Document
	input := PlanExecutionInput{
		ExecutionID:         strings.TrimSpace(proposal.TargetExecutionID),
		SnapshotRevision:    proposal.TargetExecutionVersion,
		CommandID:           proposal.MaterializationCommandID,
		ReservedExecutionID: proposal.ReservedExecutionID,
		SealedGoalBinding:   sealedProposalGoalBinding(proposal),
		Objective:           document.Objective,
		CompletionCriteria:  slices.Clone(document.CompletionCriteria),
		ReplacementReason:   document.ReplacementReason,
		SupersedeActiveWork: document.SupersedeActiveWork,
		Draft:               canonicalDraftFromProposal(document),
	}
	switch document.Operation {
	case protocol.ExecutionPlanProposalCreate:
		input.ExecutionID = ""
		input.SnapshotRevision = 0
		materializationActor.ExecutionID = ""
	case protocol.ExecutionPlanProposalReplan:
		if target == nil {
			return s.blockPlanProposal(ctx, actor, proposal, domainError(
				ErrorCodePlanProposalStale,
				"sealed replan target no longer exists",
			))
		}
	case protocol.ExecutionPlanProposalReplace:
		input.ReplaceCurrentExecution = true
	default:
		return s.blockPlanProposal(ctx, actor, proposal, domainError(
			ErrorCodePlanDocumentInvalid,
			"sealed proposal operation is invalid",
		))
	}

	result, materializeErr := s.PlanExecution(ctx, materializationActor, input)
	if materializeErr != nil {
		var pending *GoalBindingConfirmationPendingError
		if errors.As(materializeErr, &pending) && pending.DurableMutation && pending.Snapshot != nil {
			matches, matchErr := proposalMatchesSnapshot(proposal, pending.Snapshot)
			if matchErr != nil {
				return s.deferPlanProposalRetry(ctx, actor, proposal, matchErr)
			}
			if !matches {
				return s.blockPlanProposal(ctx, actor, proposal, domainError(
					ErrorCodePlanProposalBlocked,
					"authoritative materialization receipt does not match the sealed Plan proposal",
				))
			}
			result = AppliedResult(
				pending.Snapshot,
				planChangedEntities(pending.Snapshot),
				nextActions(pending.Snapshot, materializationActor),
			)
			return s.recordMaterializedPlanProposal(
				ctx,
				actor,
				proposal,
				result,
				pending.Snapshot.Plan.ID,
				false,
				pending.Err,
			)
		}
		return s.deferPlanProposalRetry(ctx, actor, proposal, materializeErr)
	}
	if result.Outcome == MutationRejected {
		return s.blockPlanProposal(ctx, actor, proposal, domainError(
			firstNonEmptyErrorCode(result.ReasonCode, ErrorCodePlanProposalStale),
			firstNonEmptyPlanProposalValue(result.Message, "sealed proposal was rejected"),
		))
	}
	if result.Snapshot == nil || result.Snapshot.Plan == nil {
		return MutationResult{}, errors.New("plan materialization returned no authoritative Plan receipt")
	}
	matches, matchErr := proposalMatchesSnapshot(proposal, result.Snapshot)
	if matchErr != nil {
		return s.deferPlanProposalRetry(ctx, actor, proposal, matchErr)
	}
	if !matches {
		return s.blockPlanProposal(ctx, actor, proposal, domainError(
			ErrorCodePlanProposalBlocked,
			"authoritative materialization receipt does not match the sealed Plan proposal",
		))
	}
	return s.recordMaterializedPlanProposal(
		ctx,
		actor,
		proposal,
		result,
		result.Snapshot.Plan.ID,
		true,
		nil,
	)
}

func (s *Service) validateProposalTargetFence(
	ctx context.Context,
	actor ActorContext,
	proposal *protocol.ExecutionPlanProposal,
) (*protocol.ExecutionSnapshot, error) {
	lookupActor := actor
	lookupActor.ExecutionID = ""
	current, err := s.GetCurrent(ctx, lookupActor)
	if err != nil {
		return nil, err
	}
	if proposal.Document.Operation == protocol.ExecutionPlanProposalCreate {
		if current != nil {
			return nil, domainError(
				ErrorCodePlanProposalStale,
				"another current Execution exists after this create proposal was sealed",
			)
		}
		sealedActor := proposalActor(proposal)
		activation, resolveErr := s.resolveProposalGoalActivation(
			ctx,
			sealedActor,
		)
		if resolveErr != nil {
			return nil, resolveErr
		}
		if !proposalGoalActivationMatches(proposal, activation) {
			return nil, domainError(
				ErrorCodePlanProposalStale,
				"exact Goal binding changed after this create proposal was sealed",
			)
		}
		return nil, nil
	}
	if current == nil || current.Execution.ID != proposal.TargetExecutionID {
		return nil, domainError(
			ErrorCodePlanProposalStale,
			"current Execution no longer matches the sealed proposal target",
		)
	}
	if current.Execution.Version != proposal.TargetExecutionVersion {
		return nil, domainError(
			ErrorCodePlanProposalStale,
			"current Execution version changed after this proposal was sealed",
		)
	}
	basePlanID := ""
	if current.Plan != nil {
		basePlanID = strings.TrimSpace(current.Plan.ID)
	}
	if basePlanID != strings.TrimSpace(proposal.BasePlanID) {
		return nil, domainError(
			ErrorCodePlanProposalStale,
			"active Plan changed after this proposal was sealed",
		)
	}
	if strings.TrimSpace(current.Execution.GoalID) != strings.TrimSpace(proposal.GoalID) ||
		current.Execution.GoalObjectiveRevision != proposal.GoalObjectiveRevision {
		return nil, domainError(
			ErrorCodePlanProposalStale,
			"Goal binding changed after this proposal was sealed",
		)
	}
	return current, nil
}

type recoveredPlanMaterialization struct {
	Snapshot *protocol.ExecutionSnapshot
	PlanID   string
}

func (s *Service) recoverProposalMaterialization(
	ctx context.Context,
	actor ActorContext,
	proposal *protocol.ExecutionPlanProposal,
) (*recoveredPlanMaterialization, error) {
	reservedExecutionID := strings.TrimSpace(proposal.ReservedExecutionID)
	if reservedExecutionID == "" {
		return nil, errors.New("materializing proposal has no reserved Execution identity")
	}
	receiptReader, ok := s.repository.(PlanMaterializationReceiptReader)
	if !ok {
		return nil, errors.New("authoritative Plan materialization receipt reader is unavailable")
	}
	planID, err := receiptReader.FindPlanMaterializationReceipt(
		ctx,
		reservedExecutionID,
		proposal.MaterializationCommandID,
	)
	if err != nil || strings.TrimSpace(planID) == "" {
		return nil, err
	}
	snapshot, err := s.GetSnapshot(ctx, actor, reservedExecutionID)
	if err != nil {
		var domainErr *DomainError
		if errors.As(err, &domainErr) && domainErr.Code == ErrorCodeWrongOwner {
			return nil, err
		}
		return nil, err
	}
	if snapshot == nil {
		return nil, errors.New("materialization command receipt points to a missing Execution")
	}
	if snapshot.Plan != nil && snapshot.Plan.ID == strings.TrimSpace(planID) {
		matches, matchErr := proposalMatchesSnapshot(proposal, snapshot)
		if matchErr != nil {
			return nil, matchErr
		}
		if !matches {
			return nil, errors.New("materialization command receipt does not match its sealed Plan proposal")
		}
	}
	return &recoveredPlanMaterialization{
		Snapshot: snapshot,
		PlanID:   strings.TrimSpace(planID),
	}, nil
}

func proposalMatchesSnapshot(
	proposal *protocol.ExecutionPlanProposal,
	snapshot *protocol.ExecutionSnapshot,
) (bool, error) {
	if proposal == nil || snapshot == nil || snapshot.Plan == nil {
		return false, nil
	}
	switch proposal.Document.Operation {
	case protocol.ExecutionPlanProposalCreate:
		if snapshot.Execution.ID != proposal.ReservedExecutionID ||
			strings.TrimSpace(snapshot.Execution.ReplacesExecutionID) !=
				strings.TrimSpace(proposal.ReplacesExecutionID) {
			return false, nil
		}
	case protocol.ExecutionPlanProposalReplan:
		if snapshot.Execution.ID != proposal.TargetExecutionID {
			return false, nil
		}
	case protocol.ExecutionPlanProposalReplace:
		if snapshot.Execution.ID != proposal.ReservedExecutionID ||
			snapshot.Execution.ReplacesExecutionID != proposal.TargetExecutionID {
			return false, nil
		}
	default:
		return false, nil
	}
	if proposal.Document.Operation != protocol.ExecutionPlanProposalReplan {
		if strings.TrimSpace(snapshot.Execution.Objective) != strings.TrimSpace(proposal.Document.Objective) ||
			!slices.Equal(
				normalizeNonEmptyValues(snapshot.Execution.CompletionCriteria),
				normalizeNonEmptyValues(proposal.Document.CompletionCriteria),
			) {
			return false, nil
		}
	}
	if strings.TrimSpace(snapshot.Execution.GoalID) != strings.TrimSpace(proposal.GoalID) ||
		snapshot.Execution.GoalObjectiveRevision != proposal.GoalObjectiveRevision {
		return false, nil
	}
	return planDraftMatchesSnapshot(snapshot, canonicalDraftFromProposal(proposal.Document))
}

func sealedProposalGoalBinding(proposal *protocol.ExecutionPlanProposal) *ExplicitGoalBinding {
	if proposal == nil {
		return &ExplicitGoalBinding{}
	}
	return &ExplicitGoalBinding{
		ExecutionID:           strings.TrimSpace(proposal.ReservedExecutionID),
		GoalID:                strings.TrimSpace(proposal.GoalID),
		GoalObjectiveRevision: proposal.GoalObjectiveRevision,
		ActivationOrigin:      proposal.GoalActivationOrigin,
		ActivationReason:      proposal.GoalActivationReason,
		ReplacesExecutionID:   strings.TrimSpace(proposal.ReplacesExecutionID),
	}
}

func proposalGoalActivationMatches(
	proposal *protocol.ExecutionPlanProposal,
	activation *ExplicitGoalActivation,
) bool {
	if proposal == nil {
		return false
	}
	if activation == nil {
		return strings.TrimSpace(proposal.GoalID) == "" &&
			proposal.GoalObjectiveRevision == 0 &&
			proposal.GoalActivationOrigin == "" &&
			proposal.GoalActivationReason == "" &&
			strings.TrimSpace(proposal.GoalReservedExecutionID) == "" &&
			strings.TrimSpace(proposal.ReplacesExecutionID) == ""
	}
	reservationMatches := strings.TrimSpace(proposal.GoalReservedExecutionID) ==
		strings.TrimSpace(activation.ReservedExecutionID)
	if strings.TrimSpace(proposal.GoalReservedExecutionID) == "" &&
		strings.TrimSpace(activation.ReservedExecutionID) != "" {
		reservationMatches = true
	}
	return strings.TrimSpace(proposal.GoalID) == strings.TrimSpace(activation.GoalID) &&
		proposal.GoalObjectiveRevision == activation.GoalObjectiveRevision &&
		strings.TrimSpace(proposal.Document.Objective) == strings.TrimSpace(activation.Objective) &&
		proposal.GoalActivationOrigin == activation.ActivationOrigin &&
		proposal.GoalActivationReason == activation.ActivationReason &&
		reservationMatches &&
		strings.TrimSpace(proposal.ReplacesExecutionID) ==
			strings.TrimSpace(activation.ReplacesExecutionID)
}

func (s *Service) recordMaterializedPlanProposal(
	ctx context.Context,
	actor ActorContext,
	proposal *protocol.ExecutionPlanProposal,
	result MutationResult,
	materializedPlanID string,
	confirmationAlreadySucceeded bool,
	confirmationErr error,
) (MutationResult, error) {
	snapshot := result.Snapshot
	materializedPlanID = strings.TrimSpace(materializedPlanID)
	if snapshot == nil || materializedPlanID == "" {
		return MutationResult{}, errors.New("cannot record proposal without Execution and Plan receipt")
	}
	updated, err := s.markExactPlanProposalMaterialized(
		ctx,
		actor,
		proposal,
		snapshot.Execution.ID,
		materializedPlanID,
	)
	if err != nil {
		return s.deferPlanProposalRetry(ctx, actor, proposal, err)
	}
	if updated == nil {
		return MutationResult{}, errors.New("materialized proposal receipt disappeared")
	}
	if strings.TrimSpace(updated.GoalID) == "" {
		return result, nil
	}

	if confirmationAlreadySucceeded {
		result = withConfirmedGoalAuthority(result)
		confirmed, markErr := s.markPlanProposalConfirmation(
			ctx,
			actor,
			updated,
			protocol.ExecutionPlanProposalConfirmationConfirmed,
			"",
			nil,
		)
		if markErr == nil {
			_ = confirmed
			return result, nil
		}
		confirmationErr = markErr
	} else if confirmationErr == nil {
		confirmationErr = s.confirmGoalExecutionBinding(ctx, snapshot)
		if confirmationErr == nil {
			result = withConfirmedGoalAuthority(result)
			_, markErr := s.markPlanProposalConfirmation(
				ctx,
				actor,
				updated,
				protocol.ExecutionPlanProposalConfirmationConfirmed,
				"",
				nil,
			)
			if markErr == nil {
				return result, nil
			}
			confirmationErr = markErr
		}
	}

	nextAttemptAt := s.currentTime().Add(planProposalRetryDelay)
	_, _ = s.markPlanProposalConfirmation(
		ctx,
		actor,
		updated,
		protocol.ExecutionPlanProposalConfirmationPending,
		confirmationErr.Error(),
		&nextAttemptAt,
	)
	result.Message = "Execution and Plan are durable; Goal binding confirmation will retry automatically."
	result.GoalConfirmation = GoalConfirmationPending
	result.NextActions = appendUniqueNextAction(result.NextActions, NextAction{
		Tool:   "get_execution",
		Reason: "continue from the durable Execution while Goal confirmation recovers in the background",
	})
	return result, nil
}

func (s *Service) markExactPlanProposalMaterialized(
	ctx context.Context,
	actor ActorContext,
	proposal *protocol.ExecutionPlanProposal,
	executionID string,
	planID string,
) (*protocol.ExecutionPlanProposal, error) {
	current := proposal
	for attempt := 0; attempt < 3; attempt++ {
		updated, err := s.planProposals.MarkPlanProposalMaterialized(
			ctx,
			orchestrationstore.MarkPlanProposalMaterializedCommand{
				Access:                  proposalAccess(actor, proposal.ID),
				ExpectedVersion:         current.Version,
				MaterializedExecutionID: strings.TrimSpace(executionID),
				MaterializedPlanID:      strings.TrimSpace(planID),
			},
		)
		if err == nil {
			if exactMaterializedPlanProposalReceipt(updated, executionID, planID) {
				return updated, nil
			}
			return nil, errors.New("proposal repository returned a non-matching materialization receipt")
		}
		if !errors.Is(err, orchestrationstore.ErrVersionConflict) {
			return nil, err
		}
		current, err = s.reloadPlanProposal(ctx, actor, proposal.ID)
		if err != nil {
			return nil, err
		}
		if exactMaterializedPlanProposalReceipt(current, executionID, planID) {
			return current, nil
		}
		switch current.Status {
		case protocol.ExecutionPlanProposalStatusMaterializing:
			continue
		case protocol.ExecutionPlanProposalStatusBlocked:
			continue
		default:
			return nil, fmt.Errorf(
				"proposal changed to %q without the exact materialization receipt",
				current.Status,
			)
		}
	}
	return nil, errors.New("proposal receipt kept changing concurrently")
}

func exactMaterializedPlanProposalReceipt(
	proposal *protocol.ExecutionPlanProposal,
	executionID string,
	planID string,
) bool {
	return proposal != nil &&
		proposal.Status == protocol.ExecutionPlanProposalStatusMaterialized &&
		strings.TrimSpace(proposal.MaterializedExecutionID) == strings.TrimSpace(executionID) &&
		strings.TrimSpace(proposal.MaterializedPlanID) == strings.TrimSpace(planID)
}

func (s *Service) finishMaterializedPlanProposal(
	ctx context.Context,
	actor ActorContext,
	proposal *protocol.ExecutionPlanProposal,
) (MutationResult, error) {
	executionID := strings.TrimSpace(proposal.MaterializedExecutionID)
	snapshot, err := s.GetSnapshot(ctx, actor, executionID)
	if err != nil {
		return MutationResult{}, err
	}
	if snapshot == nil {
		return MutationResult{}, errors.New("materialized proposal receipt points to a missing Execution")
	}
	message := "sealed Plan proposal was already materialized"
	if snapshot.Plan == nil || snapshot.Plan.ID != proposal.MaterializedPlanID {
		message = "sealed Plan proposal was materialized; the Execution has since advanced to another Plan state"
	}
	goalBindingConfirmed := proposal.ConfirmationState ==
		protocol.ExecutionPlanProposalConfirmationConfirmed
	if proposal.ConfirmationState == protocol.ExecutionPlanProposalConfirmationPending {
		confirmErr := s.confirmGoalExecutionBinding(ctx, snapshot)
		if confirmErr == nil {
			goalBindingConfirmed = true
			_, err = s.markPlanProposalConfirmation(
				ctx,
				actor,
				proposal,
				protocol.ExecutionPlanProposalConfirmationConfirmed,
				"",
				nil,
			)
			if err != nil && !errors.Is(err, orchestrationstore.ErrVersionConflict) {
				return MutationResult{}, err
			}
			message = "sealed Plan proposal was already materialized and Goal confirmation is complete"
		} else {
			nextAttemptAt := s.currentTime().Add(planProposalRetryDelay)
			_, _ = s.markPlanProposalConfirmation(
				ctx,
				actor,
				proposal,
				protocol.ExecutionPlanProposalConfirmationPending,
				confirmErr.Error(),
				&nextAttemptAt,
			)
			message = "sealed Plan proposal is durable; Goal confirmation remains pending and will retry"
		}
	}
	result := NoOpResult(snapshot, message)
	result.NextActions = nextActions(snapshot, actor)
	if goalBindingConfirmed {
		result = withConfirmedGoalAuthority(result)
	} else if proposal.ConfirmationState == protocol.ExecutionPlanProposalConfirmationPending {
		result.GoalConfirmation = GoalConfirmationPending
	}
	return result, nil
}

func (s *Service) blockPlanProposal(
	ctx context.Context,
	actor ActorContext,
	proposal *protocol.ExecutionPlanProposal,
	cause error,
) (MutationResult, error) {
	message := "sealed proposal cannot be committed"
	var domainErr *DomainError
	code := ErrorCodePlanProposalBlocked
	if errors.As(cause, &domainErr) {
		message = domainErr.Message
		code = domainErr.Code
	} else if cause != nil {
		message = cause.Error()
	}
	updated, err := s.planProposals.MarkPlanProposalBlocked(
		ctx,
		orchestrationstore.MarkPlanProposalBlockedCommand{
			Access:          proposalAccess(actor, proposal.ID),
			ExpectedVersion: proposal.Version,
			LastError:       message,
		},
	)
	if errors.Is(err, orchestrationstore.ErrVersionConflict) {
		updated, err = s.reloadPlanProposal(ctx, actor, proposal.ID)
		if err != nil {
			return MutationResult{}, err
		}
		switch updated.Status {
		case protocol.ExecutionPlanProposalStatusMaterialized:
			return s.finishMaterializedPlanProposal(ctx, actor, updated)
		case protocol.ExecutionPlanProposalStatusMaterializing:
			return NoOpResult(
				nil,
				"sealed Plan proposal changed concurrently and remains under materialization",
			), nil
		case protocol.ExecutionPlanProposalStatusBlocked:
			message = firstNonEmptyPlanProposalValue(updated.LastError, message)
		default:
			return MutationResult{}, fmt.Errorf(
				"proposal changed to %q while blocking",
				updated.Status,
			)
		}
	}
	if err != nil {
		return MutationResult{}, err
	}
	return RejectedResult(nil, domainError(code, message), []NextAction{{
		Tool:   "prepare_plan_execution",
		Reason: "refresh current state and prepare a new complete proposal",
	}}), nil
}

func (s *Service) deferPlanProposalRetry(
	ctx context.Context,
	actor ActorContext,
	proposal *protocol.ExecutionPlanProposal,
	cause error,
) (MutationResult, error) {
	if cause == nil {
		cause = errors.New("plan proposal materialization failed transiently")
	}
	nextAttemptAt := s.currentTime().Add(planProposalRetryDelay)
	_, scheduleErr := s.planProposals.SchedulePlanProposalRetry(
		ctx,
		orchestrationstore.SchedulePlanProposalRetryCommand{
			Access:          proposalAccess(actor, proposal.ID),
			ExpectedVersion: proposal.Version,
			LastError:       cause.Error(),
			NextAttemptAt:   &nextAttemptAt,
		},
	)
	if scheduleErr != nil && !errors.Is(scheduleErr, orchestrationstore.ErrVersionConflict) {
		return MutationResult{}, errors.Join(cause, fmt.Errorf("schedule proposal retry: %w", scheduleErr))
	}
	return MutationResult{}, cause
}

func (s *Service) markPlanProposalConfirmation(
	ctx context.Context,
	actor ActorContext,
	proposal *protocol.ExecutionPlanProposal,
	state protocol.ExecutionPlanProposalConfirmationState,
	lastError string,
	nextAttemptAt *time.Time,
) (*protocol.ExecutionPlanProposal, error) {
	return s.planProposals.MarkPlanProposalConfirmation(
		ctx,
		orchestrationstore.MarkPlanProposalConfirmationCommand{
			Access:            proposalAccess(actor, proposal.ID),
			ExpectedVersion:   proposal.Version,
			ConfirmationState: state,
			LastError:         strings.TrimSpace(lastError),
			NextAttemptAt:     nextAttemptAt,
		},
	)
}

func (s *Service) reloadPlanProposal(
	ctx context.Context,
	actor ActorContext,
	proposalID string,
) (*protocol.ExecutionPlanProposal, error) {
	proposal, err := s.planProposals.GetPlanProposal(
		ctx,
		orchestrationstore.GetPlanProposalQuery{
			Access: proposalAccess(actor, proposalID),
		},
	)
	if err != nil {
		return nil, err
	}
	if proposal == nil {
		return nil, errors.New("execution plan proposal disappeared")
	}
	return proposal, nil
}

func proposalActor(proposal *protocol.ExecutionPlanProposal) ActorContext {
	return ActorContext{
		OwnerUserID:           strings.TrimSpace(proposal.OwnerUserID),
		SessionKey:            strings.TrimSpace(proposal.SessionKey),
		ExecutionID:           strings.TrimSpace(proposal.TargetExecutionID),
		GoalID:                strings.TrimSpace(proposal.GoalID),
		GoalObjectiveRevision: proposal.GoalObjectiveRevision,
		AgentID:               strings.TrimSpace(proposal.CoordinatorAgentID),
		Role:                  ExecutionActorCoordinator,
		ActorKind:             protocol.ExecutionActorAgent,
		ScopeKind:             proposal.ScopeKind,
		RoomID:                strings.TrimSpace(proposal.RoomID),
		ConversationID:        strings.TrimSpace(proposal.ConversationID),
		RootRoundID:           strings.TrimSpace(proposal.RootRoundID),
		RuntimeRoundID:        strings.TrimSpace(proposal.RuntimeRoundID),
		AgentRoundID:          strings.TrimSpace(proposal.AgentRoundID),
	}
}

func constantTimeDigestEqual(left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if len(left) != len(right) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func appendUniqueNextAction(actions []NextAction, candidate NextAction) []NextAction {
	for _, action := range actions {
		if action.Tool == candidate.Tool {
			return actions
		}
	}
	return append(actions, candidate)
}

func firstNonEmptyPlanProposalValue(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmptyErrorCode(values ...ErrorCode) ErrorCode {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ErrorCodeInvalidInput
}
