// INPUT: 共享命令授权、引用解析与结果投影。
// OUTPUT: 保持现有业务契约的领域处理结果。
// POS: orchestration 包内按职责聚合的实现。
package orchestration

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

// bindWorkReferenceFromTrustedResponsibility applies the same exact-binding
// locator semantics to block/resume as submit. A trusted WorkBinding is the
// canonical locator; model-supplied references may only confirm it.
func bindWorkReferenceFromTrustedResponsibility(
	actor ActorContext,
	snapshot *protocol.ExecutionSnapshot,
	workItemID string,
	logicalKey string,
) (string, string, error) {
	if actor.WorkBinding == nil {
		if strings.TrimSpace(workItemID) == "" && strings.TrimSpace(logicalKey) == "" {
			return "", "", domainError(
				ErrorCodeInvalidInput,
				"work_item_id or logical_key is required when no trusted WorkBinding is present",
			)
		}
		return workItemID, logicalKey, nil
	}
	binding := actor.WorkBinding.Normalized()
	if !binding.Complete() {
		return "", "", workBindingMismatch("trusted WorkBinding is incomplete")
	}
	if explicitWorkID := strings.TrimSpace(workItemID); explicitWorkID != "" &&
		explicitWorkID != binding.WorkItemID {
		return "", "", workBindingMismatch(
			"explicit work_item_id does not match the trusted WorkBinding",
		)
	}
	if err := validateWorkBindingLogicalKey(snapshot, binding, logicalKey); err != nil {
		return "", "", err
	}
	return binding.WorkItemID, logicalKey, nil
}

func validateWorkBindingLogicalKey(
	snapshot *protocol.ExecutionSnapshot,
	binding protocol.ExecutionWorkBinding,
	logicalKey string,
) error {
	logicalKey = strings.TrimSpace(logicalKey)
	if logicalKey == "" {
		return nil
	}
	boundWork := findWorkItemByID(snapshot, binding.WorkItemID)
	if boundWork == nil {
		return workBindingMismatch("trusted WorkBinding Work Item is outside the active Plan")
	}
	if logicalKey != strings.TrimSpace(boundWork.LogicalKey) {
		return workBindingMismatch("explicit logical_key does not match the trusted WorkBinding")
	}
	return nil
}

func (s *Service) mutableSnapshot(
	ctx context.Context,
	actor ActorContext,
	executionID string,
	expectedRevision int64,
	coordinatorOnly bool,
	allowPlanMode bool,
) (*protocol.ExecutionSnapshot, *MutationResult, error) {
	if err := validateActor(actor); err != nil {
		result := RejectedResult(nil, err, nil)
		return nil, &result, nil
	}
	if actor.PlanMode && !allowPlanMode {
		result := RejectedResult(nil, planModeError(), nil)
		return nil, &result, nil
	}
	snapshot, err := s.GetSnapshot(ctx, actor, executionID)
	if err != nil {
		var domainErr *DomainError
		if errors.As(err, &domainErr) {
			if domainErr.Code == ErrorCodeExecutionTerminal {
				result := SupersededResult(nil, err)
				return nil, &result, nil
			}
			result := RejectedResult(nil, err, nil)
			return nil, &result, nil
		}
		return nil, nil, err
	}
	if snapshot == nil {
		result := RejectedResult(nil, domainError(ErrorCodeInvalidInput, "execution was not found"), nil)
		return nil, &result, nil
	}
	if !isCurrentExecutionStatus(snapshot.Execution.Status) {
		result := RejectedResult(snapshot, terminalExecutionError(), nil)
		if snapshot.Execution.Status == protocol.ExecutionStatusSuperseded {
			result = SupersededResult(snapshot, terminalExecutionError())
		}
		return snapshot, &result, nil
	}
	if expectedErr := requireMutationRevision(snapshot, expectedRevision); expectedErr != nil {
		result := RejectedResult(snapshot, expectedErr, nextActions(snapshot, actor))
		return snapshot, &result, nil
	}
	if coordinatorOnly {
		if coordinationErr := s.requireRuntimeCoordination(actor, snapshot); coordinationErr != nil {
			result := RejectedResult(snapshot, coordinationErr, []NextAction{{
				Domain: "execution", Operation: "get_execution",
				Reason: "explicitly inspect and enter the current Room coordination scope",
			}})
			return snapshot, &result, nil
		}
		if authErr := requireCoordinator(actor, snapshot); authErr != nil {
			code := ErrorCodeWrongOwner
			if strings.Contains(authErr.Error(), "review") {
				code = ErrorCodeWrongReviewer
			}
			result := RejectedResult(snapshot, domainError(code, authErr.Error()), nil)
			return snapshot, &result, nil
		}
	}
	return snapshot, nil, nil
}

func (s *Service) storageMutationResult(
	snapshot *protocol.ExecutionSnapshot,
	err error,
	actions []NextAction,
) (MutationResult, error) {
	switch {
	case errors.Is(err, orchestrationstore.ErrVersionConflict):
		return RejectedResult(snapshot, domainError(
			ErrorCodeStaleExecution,
			"state changed concurrently; reload the execution before retrying",
		), actions), nil
	case errors.Is(err, orchestrationstore.ErrWorkNotReady):
		return RejectedResult(snapshot, domainError(
			ErrorCodeDependencyNotAccepted,
			"Work Item is not ready or already has a current Assignment",
		), actions), nil
	case errors.Is(err, orchestrationstore.ErrCompletionBlocked):
		return RejectedResult(snapshot, domainError(
			ErrorCodeCompletionBlocked,
			"execution still has completion blockers",
		), actions), nil
	case errors.Is(err, orchestrationstore.ErrProjectionLimitExceeded):
		return RejectedResult(snapshot, domainError(
			ErrorCodeProjectionLimitExceeded,
			err.Error(),
		), actions), nil
	case errors.Is(err, orchestrationstore.ErrCommandConflict),
		errors.Is(err, orchestrationstore.ErrInvariant):
		return RejectedResult(snapshot, domainError(
			ErrorCodeInvalidInput,
			"command violates the current execution state",
		), actions), nil
	default:
		return MutationResult{}, err
	}
}

func resolvePlanWork(
	snapshot *protocol.ExecutionSnapshot,
	workItemID string,
	logicalKey string,
) (protocol.WorkItem, protocol.WorkItemSpec, error) {
	workItemID = strings.TrimSpace(workItemID)
	logicalKey = strings.TrimSpace(logicalKey)
	var work *protocol.WorkItem
	for index := range snapshot.WorkItems {
		item := &snapshot.WorkItems[index]
		if workItemID != "" && item.ID == workItemID {
			work = item
			break
		}
		if workItemID == "" && logicalKey != "" && item.LogicalKey == logicalKey {
			work = item
			break
		}
	}
	if work == nil {
		return protocol.WorkItem{}, protocol.WorkItemSpec{}, newDomainError(
			ErrorCodeInvalidInput,
			"Work Item is not part of the active Plan",
			logicalKey,
			workItemID,
		)
	}
	if logicalKey != "" && work.LogicalKey != logicalKey {
		return protocol.WorkItem{}, protocol.WorkItemSpec{}, newDomainError(
			ErrorCodeInvalidInput,
			"work_item_id and logical_key identify different Work Items",
			logicalKey,
			workItemID,
		)
	}
	for _, item := range snapshot.PlanItems {
		if item.WorkItemID != work.ID {
			continue
		}
		for _, spec := range snapshot.WorkItemSpecs {
			if spec.ID == item.SpecID {
				return *work, spec, nil
			}
		}
	}
	return protocol.WorkItem{}, protocol.WorkItemSpec{}, newDomainError(
		ErrorCodeInvalidInput,
		"active Work Item spec is missing",
		work.LogicalKey,
		work.ID,
	)
}

func findWorkItemByID(
	snapshot *protocol.ExecutionSnapshot,
	workItemID string,
) *protocol.WorkItem {
	if snapshot == nil {
		return nil
	}
	workItemID = strings.TrimSpace(workItemID)
	for index := range snapshot.WorkItems {
		if snapshot.WorkItems[index].ID == workItemID {
			return &snapshot.WorkItems[index]
		}
	}
	return nil
}

func findStateByWorkID(
	snapshot *protocol.ExecutionSnapshot,
	workItemID string,
) *protocol.WorkItemState {
	for index := range snapshot.WorkItemStates {
		if snapshot.WorkItemStates[index].WorkItemID == workItemID {
			return &snapshot.WorkItemStates[index]
		}
	}
	return nil
}

func nextActions(
	snapshot *protocol.ExecutionSnapshot,
	actor ActorContext,
) []NextAction {
	if snapshot == nil {
		return nil
	}
	actorAgentID := strings.TrimSpace(actor.AgentID)
	isCoordinator := snapshot.Execution.CoordinatorAgentID == actorAgentID
	if isCoordinator &&
		snapshot.Execution.Status == protocol.ExecutionStatusCompleted &&
		strings.TrimSpace(snapshot.Execution.GoalID) != "" &&
		snapshot.Execution.GoalID == strings.TrimSpace(actor.GoalID) &&
		snapshot.Execution.GoalObjectiveRevision > 0 &&
		snapshot.Execution.GoalObjectiveRevision == actor.GoalObjectiveRevision {
		return []NextAction{{
			Domain:    "goal",
			Operation: "audit_objective_alignment",
			Reason: "the Goal-bound Execution is complete; audit the authoritative Goal criteria in this physical round, " +
				"then follow the Goal audit result—do not call audit_execution_alignment on the terminal Execution",
		}}
	}
	actions := make([]NextAction, 0)
	if isCoordinator {
		for _, workID := range snapshot.ReadyWorkItemIDs {
			work := logicalWork(snapshot, workID)
			actions = append(actions, NextAction{
				Domain: "execution", Operation: "assign_work",
				WorkItemID: workID,
				LogicalKey: work.LogicalKey,
				Reason:     "Work Item is ready and has no current Assignment",
			})
		}
	}
	for _, work := range snapshot.WorkItems {
		if submission := latestUnreviewedSubmission(snapshot, work.ID); submission != nil {
			assignment := findAssignmentByID(snapshot, submission.AssignmentID)
			if snapshot.Execution.ScopeKind == protocol.ExecutionScopeRoom &&
				(assignment == nil ||
					strings.TrimSpace(assignment.ReturnToAgentID) != actorAgentID) {
				continue
			}
			if snapshot.Execution.ScopeKind != protocol.ExecutionScopeRoom && !isCoordinator {
				continue
			}
			actions = append(actions, NextAction{
				Domain: "execution", Operation: "review_work",
				WorkItemID: work.ID,
				LogicalKey: work.LogicalKey,
				Reason:     "Submission is awaiting the selected review decision",
			})
		}
	}
	for _, work := range snapshot.WorkItems {
		state := findStateByWorkID(snapshot, work.ID)
		if state == nil || state.Status != protocol.WorkItemStatusWaitingInput {
			continue
		}
		assignment := latestAssignmentForCurrentSpec(
			snapshot,
			work.ID,
			state.CurrentSpecID,
		)
		if !isCoordinator &&
			(assignment == nil || assignment.OwnerAgentID != actorAgentID) {
			continue
		}
		actions = append(actions, NextAction{
			Domain: "execution", Operation: "resume_work",
			WorkItemID: work.ID,
			LogicalKey: work.LogicalKey,
			Reason:     "the exact waiting_input blocker must be resolved with evidence",
		})
	}
	for _, assignment := range snapshot.Assignments {
		if currentAssignment(assignment) &&
			assignment.OwnerAgentID == actorAgentID &&
			latestUnreviewedSubmission(snapshot, assignment.WorkItemID) == nil {
			state := findStateByWorkID(snapshot, assignment.WorkItemID)
			if state == nil ||
				state.CurrentSpecID != assignment.SpecID ||
				state.Status != protocol.WorkItemStatusOpen {
				continue
			}
			work := logicalWork(snapshot, assignment.WorkItemID)
			actions = append(actions, NextAction{
				Domain: "execution", Operation: "submit_work",
				WorkItemID: assignment.WorkItemID,
				LogicalKey: work.LogicalKey,
				Reason:     "current actor owns this Assignment",
			})
		}
	}
	return actions
}

func logicalWork(snapshot *protocol.ExecutionSnapshot, workID string) protocol.WorkItem {
	for _, work := range snapshot.WorkItems {
		if work.ID == workID {
			return work
		}
	}
	return protocol.WorkItem{}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

func resultOrZero(result *MutationResult) MutationResult {
	if result == nil {
		return MutationResult{}
	}
	return *result
}
