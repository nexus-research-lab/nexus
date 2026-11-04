// INPUT: Authenticated user, exact task identity and confirmed graph revision.
// OUTPUT: Dependency-aware retry preview and one atomic durable retry operation.
// POS: User-owned WorkGraph lifecycle; preserves prior owners, reviewers and result history.
package orchestration

import (
	"context"
	"errors"
	"fmt"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	store "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

type workRetryRepository interface {
	FindLatestManaged(context.Context, string, string) (*protocol.Execution, error)
	RetryWork(context.Context, store.RetryWorkCommand) (*protocol.ExecutionSnapshot, error)
}

func (s *Service) retrySnapshot(ctx context.Context, owner, executionID, session string) (*protocol.ExecutionSnapshot, error) {
	repository, ok := s.repository.(workRetryRepository)
	if !ok {
		return nil, errors.New("work retry unavailable")
	}
	latest, err := repository.FindLatestManaged(ctx, owner, session)
	if err != nil {
		return nil, err
	}
	if latest == nil || latest.ID != executionID || latest.OwnerUserID != owner {
		return nil, domainError(ErrorCodeWrongOwner, "retry requires the latest WorkGraph in this session")
	}
	current, err := s.repository.FindCurrent(ctx, owner, session)
	if err != nil {
		return nil, err
	}
	if current != nil && current.ID != executionID {
		return nil, domainError(ErrorCodeInvalidInput, "another execution is current")
	}
	return s.repository.GetSnapshot(ctx, executionID)
}

// retryWorkTarget checks current responsibility, including submitted and paused work.
func retryWorkTarget(snapshot *protocol.ExecutionSnapshot, workID, attemptID string) (bool, bool) {
	if snapshot == nil || snapshot.Plan == nil {
		return false, false
	}
	switch snapshot.Execution.Status {
	case protocol.ExecutionStatusActive, protocol.ExecutionStatusWaiting, protocol.ExecutionStatusPaused, protocol.ExecutionStatusCompleted, protocol.ExecutionStatusFailed, protocol.ExecutionStatusCancelled:
	default:
		return false, false
	}
	_, spec, err := resolvePlanWork(snapshot, workID, "")
	if err != nil {
		return false, false
	}
	assignment := retryAssignment(snapshot, workID, spec.ID)
	if assignment == nil {
		if attemptID != "" {
			return false, false
		}
		for _, policy := range snapshot.DispatchPolicies {
			if policy.WorkItemID == workID && policy.TargetAgentID != "" && policy.State == "pending" {
				return true, false
			}
		}
		return false, false
	}
	attempt := latestRootAttemptForAssignment(snapshot, assignment.ID)
	if attempt == nil || attempt.ID != attemptID {
		return false, false
	}

	submitted := false
	for _, submission := range snapshot.Submissions {
		if submission.WorkItemID == workID && submission.AttemptID == attempt.ID && !submission.Superseded {
			submitted = true
		}
	}
	if submitted {
		return true, true
	}
	state := findStateByWorkID(snapshot, workID)
	if attempt.Status == protocol.WorkAttemptStatusRunning && snapshot.Execution.Status != protocol.ExecutionStatusPaused && (state == nil || state.Status != protocol.WorkItemStatusWaitingInput) {
		return false, false
	}
	return true, false
}

func retryAffectedWorks(snapshot *protocol.ExecutionSnapshot, root string, cascade bool) []protocol.WorkRetryImpact {
	affected := map[string]bool{root: true}
	// A repeated request while a prior retry waits must retain that operation's cancellation scope.
	for _, policy := range snapshot.DispatchPolicies {
		if policy.WorkItemID == root && policy.UserRetry && (policy.State == "pending" || policy.State == "assigned") {
			for _, id := range policy.RetryWorkItemIDs {
				affected[id] = true
			}
		}
	}
	if cascade {
		for changed := true; changed; {
			changed = false
			for _, edge := range snapshot.Dependencies {
				if affected[edge.DependsOnWorkItemID] && !affected[edge.WorkItemID] {
					affected[edge.WorkItemID] = true
					changed = true
				}
			}
		}
	}
	result := make([]protocol.WorkRetryImpact, 0, len(affected))
	for _, item := range snapshot.PlanItems {
		if !affected[item.WorkItemID] {
			continue
		}
		_, spec, _ := resolvePlanWork(snapshot, item.WorkItemID, "")
		impact := protocol.WorkRetryImpact{WorkItemID: item.WorkItemID}
		impact.Subject = spec.Subject
		assignment := retryAssignment(snapshot, item.WorkItemID, item.SpecID)
		if assignment != nil {
			impact.AgentID = assignment.OwnerAgentID
		}
		for _, attempt := range snapshot.Attempts {
			if attempt.WorkItemID == item.WorkItemID && attempt.Status == protocol.WorkAttemptStatusRunning {
				impact.Running = true
			}
		}
		for _, submission := range snapshot.Submissions {
			if submission.WorkItemID == item.WorkItemID && !submission.Superseded {
				impact.Submitted = true
			}
		}
		result = append(result, impact)
	}
	return result
}

func (s *Service) PreviewWorkRetry(ctx context.Context, owner, executionID string, input protocol.RequestExecutionRecovery) (*protocol.WorkRetryPreview, error) {
	snapshot, err := s.retrySnapshot(ctx, owner, executionID, input.SessionKey)
	if err != nil {
		return nil, err
	}
	allowed, submitted := retryWorkTarget(snapshot, input.WorkItemID, input.AttemptID)
	if !allowed {
		return nil, domainError(ErrorCodeInvalidInput, "task is no longer retryable")
	}
	return &protocol.WorkRetryPreview{ExecutionRecoveryTarget: protocol.ExecutionRecoveryTarget{ExecutionID: executionID, WorkItemID: input.WorkItemID, AttemptID: input.AttemptID}, SnapshotRevision: snapshot.Execution.Version, Submitted: submitted, Affected: retryAffectedWorks(snapshot, input.WorkItemID, submitted)}, nil
}

func (s *Service) RequestWorkRetry(ctx context.Context, owner, executionID string, input protocol.RequestWorkRetry) (*protocol.ExecutionRecoveryReceipt, error) {
	if input.RequestID == "" || len(input.RequestID) > protocol.MaxClientMessageIDBytes || input.SnapshotRevision <= 0 {
		return nil, domainError(ErrorCodeInvalidInput, "request and snapshot revision required")
	}
	target := protocol.ExecutionRecoveryTarget{ExecutionID: executionID, WorkItemID: input.WorkItemID, AttemptID: input.AttemptID}
	receipt, err := s.GetWorkRecoveryReceipt(ctx, owner, executionID, input.SessionKey, input.RequestID)
	if err != nil {
		return nil, err
	}
	if receipt.Queued {
		if receipt.ExecutionRecoveryTarget != target {
			return nil, domainError(ErrorCodeInvalidInput, "request_id conflict")
		}
		return receipt, nil
	}
	snapshot, err := s.retrySnapshot(ctx, owner, executionID, input.SessionKey)
	if err != nil {
		return nil, err
	}
	allowed, submitted := retryWorkTarget(snapshot, input.WorkItemID, input.AttemptID)
	if !allowed || snapshot.Execution.Version != input.SnapshotRevision {
		return nil, domainError(ErrorCodeStaleExecution, "retry preview changed")
	}
	preview := protocol.WorkRetryPreview{ExecutionRecoveryTarget: target, SnapshotRevision: input.SnapshotRevision, Submitted: submitted, Affected: retryAffectedWorks(snapshot, input.WorkItemID, submitted)}
	admission, ok := s.scheduledWork.(interface {
		ContinuationPauseRevision(context.Context, protocol.Execution, string) (int64, error)
	})
	if !ok {
		return nil, errors.New("retry admission unavailable")
	}
	actor := ActorContext{OwnerUserID: owner, SessionKey: input.SessionKey, ExecutionID: executionID, AgentID: snapshot.Execution.CoordinatorAgentID, Role: ExecutionActorCoordinator, ActorKind: protocol.ExecutionActorSystem, ScopeKind: snapshot.Execution.ScopeKind, RoomID: snapshot.Execution.RoomID, ConversationID: snapshot.Execution.ConversationID}
	key := workRecoveryKey(executionID, input.RequestID)
	policies := make([]protocol.ExecutionDispatchPolicy, 0, len(preview.Affected))
	for _, affected := range preview.Affected {
		work, spec, resolveErr := resolvePlanWork(snapshot, affected.WorkItemID, "")
		if resolveErr != nil {
			return nil, resolveErr
		}
		assignment := retryAssignment(snapshot, work.ID, spec.ID)
		ownerAgent, reviewer := "", ""
		if assignment != nil {
			ownerAgent, reviewer = assignment.OwnerAgentID, assignment.ReturnToAgentID
		}
		if ownerAgent == "" {
			for _, policy := range snapshot.DispatchPolicies {
				if policy.WorkItemID == work.ID {
					ownerAgent, reviewer = policy.TargetAgentID, policy.ReturnToAgentID
				}
			}
		}
		// Untouched, unassigned descendants remain waiting for ordinary explicit assignment.
		if ownerAgent == "" {
			continue
		}
		instruction := fmt.Sprintf("The user requested retry of task %s and its affected dependents. Reconcile existing files, checkpoints and external receipts before repeating effects. Prior submissions for this task were superseded. Execute its current contract using newly accepted upstream results, then submit for the original reviewer.", input.WorkItemID)
		if _, _, _, err = s.authorizedPolicyChain(ctx, actor, snapshot, work, spec, DispatchPolicyInput{TargetAgentID: ownerAgent, ReturnToAgentID: reviewer, Instruction: instruction}, key); err != nil {
			return nil, err
		}
		revision, readErr := admission.ContinuationPauseRevision(ctx, snapshot.Execution, ownerAgent)
		if readErr != nil {
			return nil, readErr
		}
		policies = append(policies, protocol.ExecutionDispatchPolicy{ID: s.id("policy"), ExecutionID: executionID, PlanID: snapshot.Plan.ID, WorkItemID: work.ID, SpecID: spec.ID, TargetAgentID: ownerAgent, ReturnToAgentID: reviewer, Instruction: instruction, UserRetry: true, ContinuationPauseRevision: revision})
	}
	for index := range policies {
		for _, affected := range preview.Affected {
			policies[index].RetryWorkItemIDs = append(policies[index].RetryWorkItemIDs, affected.WorkItemID)
		}
		for _, dependency := range snapshot.Dependencies {
			if dependency.WorkItemID != policies[index].WorkItemID {
				continue
			}
			for _, affected := range preview.Affected {
				if affected.WorkItemID == dependency.DependsOnWorkItemID {
					policies[index].RetryDependencyIDs = append(policies[index].RetryDependencyIDs, affected.WorkItemID)
				}
			}
		}
	}
	for _, attempt := range snapshot.Attempts {
		for _, affected := range preview.Affected {
			if attempt.WorkItemID != affected.WorkItemID {
				continue
			}
			pending, checkErr := s.pendingDeliveryResult(ctx, executionID, attempt.ID)
			if checkErr != nil {
				return nil, checkErr
			}
			if pending {
				return nil, domainError(ErrorCodeCompletionBlocked, "delivery receipt requires reconciliation before retry")
			}
		}
	}
	meta := s.commandMeta(actor, key, "")
	meta.CommandID, meta.ActorKind, meta.ActorID = key, protocol.ExecutionActorUser, owner
	meta.Payload = map[string]any{"continued_attempt_id": input.AttemptID, "retry_preview": preview}
	updated, err := s.repository.(workRetryRepository).RetryWork(ctx, store.RetryWorkCommand{OwnerUserID: owner, Preview: preview, Policies: policies, Meta: meta})
	if err != nil {
		receipt, readErr := s.GetWorkRecoveryReceipt(ctx, owner, executionID, input.SessionKey, input.RequestID)
		if readErr == nil && receipt.Queued && receipt.ExecutionRecoveryTarget == target {
			return receipt, nil
		}
		if errors.Is(err, store.ErrVersionConflict) {
			return nil, domainError(ErrorCodeStaleExecution, "retry preview changed")
		}
		return nil, err
	}
	s.invalidateSnapshot(ctx, updated)
	s.WakeExecutionDispatch()
	s.WakeOrchestrationRecovery()
	return &protocol.ExecutionRecoveryReceipt{ExecutionRecoveryTarget: target, RequestID: input.RequestID, Queued: true}, nil
}

func retryAssignment(snapshot *protocol.ExecutionSnapshot, workID, specID string) *protocol.WorkAssignment {
	current := *snapshot
	current.Execution.Status = protocol.ExecutionStatusActive
	return latestAssignmentForCurrentSpec(&current, workID, specID)
}
