// INPUT: executions 与全部 Orchestration 子表 SQL rows。
// OUTPUT: 归一化 protocol domain objects。
// POS: Repository 的唯一 SQL row 解码边界。
package orchestration

import (
	"database/sql"

	"github.com/nexus-research-lab/nexus/internal/protocol"
)

func scanExecution(scanner interface{ Scan(...any) error }) (protocol.Execution, error) {
	var item protocol.Execution
	var roomID, conversationID, coordinatorID, goalID sql.NullString
	var goalOrigin, goalReason, recoveryID, replacesID, rootRoundID, triggerID sql.NullString
	var completedAt sql.NullTime
	var scopeKind, origin, status, criteriaJSON, metadataJSON string
	err := scanner.Scan(
		&item.ID, &item.OwnerUserID, &item.SessionKey, &scopeKind,
		&roomID, &conversationID, &coordinatorID, &origin, &item.Objective,
		&criteriaJSON, &goalID, &item.GoalObjectiveRevision, &goalOrigin, &goalReason,
		&recoveryID, &replacesID, &rootRoundID, &triggerID, &status, &item.Version,
		&item.CreatedAt, &item.UpdatedAt, &completedAt, &metadataJSON,
	)
	if err != nil {
		return protocol.Execution{}, err
	}
	item.ScopeKind = protocol.ExecutionScopeKind(scopeKind)
	item.RoomID = nullStringValue(roomID)
	item.ConversationID = nullStringValue(conversationID)
	item.CoordinatorAgentID = nullStringValue(coordinatorID)
	item.Origin = protocol.ExecutionOrigin(origin)
	item.GoalID = nullStringValue(goalID)
	item.GoalActivationOrigin = protocol.GoalActivationOrigin(nullStringValue(goalOrigin))
	item.GoalActivationReason = protocol.GoalActivationReason(nullStringValue(goalReason))
	item.RecoveryOfExecutionID = nullStringValue(recoveryID)
	item.ReplacesExecutionID = nullStringValue(replacesID)
	item.RootRoundID = nullStringValue(rootRoundID)
	item.TriggerMessageID = nullStringValue(triggerID)
	item.Status = protocol.ExecutionStatus(status)
	item.CompletedAt = nullTimePointer(completedAt)
	item.CompletionCriteria, err = parseSlice[string](criteriaJSON)
	if err != nil {
		return protocol.Execution{}, err
	}
	item.Metadata, err = parseMap(metadataJSON)
	return item, err
}

func scanPlan(scanner interface{ Scan(...any) error }) (protocol.ExecutionPlanRevision, error) {
	var item protocol.ExecutionPlanRevision
	var status string
	var baseID, createdBy, reason sql.NullString
	var activatedAt, supersededAt sql.NullTime
	var metadataJSON string
	err := scanner.Scan(
		&item.ID, &item.ExecutionID, &item.Revision, &status, &baseID,
		&createdBy, &reason, &item.Version, &item.CreatedAt, &activatedAt,
		&supersededAt, &metadataJSON,
	)
	if err != nil {
		return protocol.ExecutionPlanRevision{}, err
	}
	item.Status = protocol.PlanRevisionStatus(status)
	item.BasePlanID = nullStringValue(baseID)
	item.CreatedByAgentID = nullStringValue(createdBy)
	item.RevisionReason = nullStringValue(reason)
	item.ActivatedAt = nullTimePointer(activatedAt)
	item.SupersededAt = nullTimePointer(supersededAt)
	item.Metadata, err = parseMap(metadataJSON)
	return item, err
}

func scanWorkItem(scanner interface{ Scan(...any) error }) (protocol.WorkItem, error) {
	var item protocol.WorkItem
	var kind, metadataJSON string
	err := scanner.Scan(
		&item.ID, &item.ExecutionID, &item.LogicalKey, &kind, &item.CreatedAt, &metadataJSON,
	)
	if err != nil {
		return protocol.WorkItem{}, err
	}
	item.Kind = protocol.WorkItemKind(kind)
	item.Metadata, err = parseMap(metadataJSON)
	return item, err
}

func scanWorkItemState(scanner interface{ Scan(...any) error }) (protocol.WorkItemState, error) {
	var item protocol.WorkItemState
	var status string
	var reason, input sql.NullString
	var metadataJSON string
	err := scanner.Scan(
		&item.WorkItemID, &item.ExecutionID, &item.CurrentSpecID, &status,
		&reason, &input, &item.Version, &item.UpdatedAt, &metadataJSON,
	)
	if err != nil {
		return protocol.WorkItemState{}, err
	}
	item.Status = protocol.WorkItemStatus(status)
	item.BlockReason = nullStringValue(reason)
	item.NeededInput = nullStringValue(input)
	item.Metadata, err = parseMap(metadataJSON)
	return item, err
}

func scanSpec(scanner interface{ Scan(...any) error }) (protocol.WorkItemSpec, error) {
	var item protocol.WorkItemSpec
	var createdBy sql.NullString
	var criteriaJSON, inputRefsJSON, metadataJSON string
	err := scanner.Scan(
		&item.ID, &item.WorkItemID, &item.ExecutionID, &item.Version,
		&item.Subject, &item.Objective, &item.Deliverable, &criteriaJSON,
		&inputRefsJSON, &item.SpecHash, &createdBy, &item.CreatedAt, &metadataJSON,
	)
	if err != nil {
		return protocol.WorkItemSpec{}, err
	}
	item.CreatedByAgentID = nullStringValue(createdBy)
	item.AcceptanceCriteria, err = parseSlice[string](criteriaJSON)
	if err != nil {
		return protocol.WorkItemSpec{}, err
	}
	item.InputRefs, err = parseSlice[string](inputRefsJSON)
	if err != nil {
		return protocol.WorkItemSpec{}, err
	}
	item.Metadata, err = parseMap(metadataJSON)
	return item, err
}

func scanPlanItem(scanner interface{ Scan(...any) error }) (protocol.ExecutionPlanItem, error) {
	var item protocol.ExecutionPlanItem
	var parent sql.NullString
	err := scanner.Scan(
		&item.PlanID, &item.ExecutionID, &item.WorkItemID, &item.SpecID,
		&parent, &item.Required, &item.Terminal, &item.Position, &item.CreatedAt,
	)
	item.ParentWorkItemID = nullStringValue(parent)
	return item, err
}

func scanDependency(scanner interface{ Scan(...any) error }) (protocol.ExecutionPlanDependency, error) {
	var item protocol.ExecutionPlanDependency
	var kind string
	err := scanner.Scan(
		&item.PlanID, &item.ExecutionID, &item.WorkItemID,
		&item.DependsOnWorkItemID, &kind, &item.CreatedAt,
	)
	item.Kind = protocol.WorkDependencyKind(kind)
	return item, err
}

func scanOutputClaim(scanner interface{ Scan(...any) error }) (protocol.ExecutionPlanOutputClaim, error) {
	var item protocol.ExecutionPlanOutputClaim
	var mode string
	err := scanner.Scan(
		&item.PlanID, &item.ExecutionID, &item.WorkItemID,
		&item.SpecID, &item.Scope, &mode, &item.CreatedAt,
	)
	item.Mode = protocol.WorkOutputScopeMode(mode)
	return item, err
}

func scanAssignment(scanner interface{ Scan(...any) error }) (protocol.WorkAssignment, error) {
	var item protocol.WorkAssignment
	var strategy, status, metadataJSON string
	var assignedBy, returnTo, reason, takeover sql.NullString
	var activatedAt, releasedAt, completedAt sql.NullTime
	err := scanner.Scan(
		&item.ID, &item.ExecutionID, &item.PlanID, &item.WorkItemID, &item.SpecID,
		&item.OwnerAgentID, &assignedBy, &returnTo, &strategy, &status,
		&reason, &takeover, &item.Version, &item.AssignedAt, &activatedAt,
		&releasedAt, &completedAt, &metadataJSON,
	)
	if err != nil {
		return protocol.WorkAssignment{}, err
	}
	item.AssignedByAgentID = nullStringValue(assignedBy)
	item.ReturnToAgentID = nullStringValue(returnTo)
	item.Strategy = protocol.AssignmentStrategy(strategy)
	item.Status = protocol.WorkAssignmentStatus(status)
	item.AssignmentReason = nullStringValue(reason)
	item.TakeoverReason = nullStringValue(takeover)
	item.ActivatedAt = nullTimePointer(activatedAt)
	item.ReleasedAt = nullTimePointer(releasedAt)
	item.CompletedAt = nullTimePointer(completedAt)
	item.Metadata, err = parseMap(metadataJSON)
	return item, err
}

func scanDispatch(scanner interface{ Scan(...any) error }) (protocol.ExecutionDispatch, error) {
	var item protocol.ExecutionDispatch
	var kind, status, metadataJSON string
	var handoffID, queueID, leaseOwner, lastError sql.NullString
	var leaseExpires, claimedAt, deliveredAt sql.NullTime
	err := scanner.Scan(
		&item.ID, &item.ExecutionID, &item.PlanID, &item.WorkItemID, &item.SpecID,
		&item.AssignmentID, &item.CommandID, &item.DedupeKey, &item.TargetAgentID,
		&kind, &status, &item.Instruction, &handoffID, &queueID,
		&item.DeliveryAttempts, &item.Version, &item.AvailableAt, &leaseOwner,
		&leaseExpires, &item.CreatedAt, &item.UpdatedAt, &claimedAt,
		&deliveredAt, &lastError, &metadataJSON,
	)
	if err != nil {
		return protocol.ExecutionDispatch{}, err
	}
	item.Kind = protocol.ExecutionDispatchKind(kind)
	item.Status = protocol.ExecutionDispatchStatus(status)
	item.HandoffID = nullStringValue(handoffID)
	item.QueueItemID = nullStringValue(queueID)
	item.LeaseOwner = nullStringValue(leaseOwner)
	item.LeaseExpiresAt = nullTimePointer(leaseExpires)
	item.ClaimedAt = nullTimePointer(claimedAt)
	item.DeliveredAt = nullTimePointer(deliveredAt)
	item.LastError = nullStringValue(lastError)
	item.Metadata, err = parseMap(metadataJSON)
	return item, err
}

func scanAttempt(scanner interface{ Scan(...any) error }) (protocol.WorkAttempt, error) {
	var item protocol.WorkAttempt
	var executorKind, status, metadataJSON string
	var dispatchID, parentAttemptID, executorID, parentAgentID sql.NullString
	var runtimeKey, roomSessionID, sdkSessionID, runtimeRoundID sql.NullString
	var rootRoundID, agentRoundID, childSessionID, sdkTaskID, toolUseID sql.NullString
	var failureReason sql.NullString
	var startedAt, finishedAt, parentRoundExitedAt, reconcileAfter sql.NullTime
	err := scanner.Scan(
		&item.ID, &item.ExecutionID, &item.PlanID, &item.WorkItemID, &item.SpecID,
		&item.AssignmentID, &dispatchID, &parentAttemptID, &executorKind, &executorID,
		&parentAgentID, &runtimeKey, &roomSessionID, &sdkSessionID, &runtimeRoundID,
		&rootRoundID, &agentRoundID, &childSessionID, &sdkTaskID, &toolUseID,
		&status, &failureReason, &item.Version, &item.CreatedAt, &startedAt,
		&finishedAt, &parentRoundExitedAt, &reconcileAfter, &metadataJSON,
	)
	if err != nil {
		return protocol.WorkAttempt{}, err
	}
	item.DispatchID = nullStringValue(dispatchID)
	item.ParentAttemptID = nullStringValue(parentAttemptID)
	item.ExecutorKind = protocol.AttemptExecutorKind(executorKind)
	item.ExecutorAgentID = nullStringValue(executorID)
	item.ParentAgentID = nullStringValue(parentAgentID)
	item.RuntimeSessionKey = nullStringValue(runtimeKey)
	item.RoomSessionID = nullStringValue(roomSessionID)
	item.SDKSessionID = nullStringValue(sdkSessionID)
	item.RuntimeRoundID = nullStringValue(runtimeRoundID)
	item.RootRoundID = nullStringValue(rootRoundID)
	item.AgentRoundID = nullStringValue(agentRoundID)
	item.ChildSessionID = nullStringValue(childSessionID)
	item.SDKTaskID = nullStringValue(sdkTaskID)
	item.ToolUseID = nullStringValue(toolUseID)
	item.Status = protocol.WorkAttemptStatus(status)
	item.FailureReason = nullStringValue(failureReason)
	item.StartedAt = nullTimePointer(startedAt)
	item.FinishedAt = nullTimePointer(finishedAt)
	item.ParentRoundExitedAt = nullTimePointer(parentRoundExitedAt)
	item.ReconcileAfter = nullTimePointer(reconcileAfter)
	item.Metadata, err = parseMap(metadataJSON)
	return item, err
}

func scanSubmission(scanner interface{ Scan(...any) error }) (protocol.WorkSubmission, error) {
	var item protocol.WorkSubmission
	var resultRefsJSON, evidenceJSON, metadataJSON string
	err := scanner.Scan(
		&item.ID, &item.ExecutionID, &item.PlanID, &item.WorkItemID, &item.SpecID,
		&item.AssignmentID, &item.AttemptID, &item.Sequence, &item.SubmitterAgentID,
		&item.ResultSummary, &resultRefsJSON, &evidenceJSON, &item.CreatedAt, &metadataJSON,
	)
	if err != nil {
		return protocol.WorkSubmission{}, err
	}
	item.ResultRefs, err = parseSlice[string](resultRefsJSON)
	if err != nil {
		return protocol.WorkSubmission{}, err
	}
	item.Evidence, err = parseSlice[string](evidenceJSON)
	if err != nil {
		return protocol.WorkSubmission{}, err
	}
	item.Metadata, err = parseMap(metadataJSON)
	return item, err
}

func scanReviewDispatch(
	scanner interface{ Scan(...any) error },
) (protocol.ExecutionReviewDispatch, error) {
	var item protocol.ExecutionReviewDispatch
	var status, metadataJSON string
	var handoffID, queueID, leaseOwner, lastError sql.NullString
	var leaseExpires, claimedAt, deliveredAt sql.NullTime
	err := scanner.Scan(
		&item.ID, &item.ExecutionID, &item.PlanID, &item.WorkItemID, &item.SpecID,
		&item.AssignmentID, &item.SubmissionID, &item.CommandID, &item.DedupeKey,
		&item.TargetAgentID, &status, &item.Instruction, &handoffID, &queueID,
		&item.DeliveryAttempts, &item.Version, &item.AvailableAt, &leaseOwner,
		&leaseExpires, &item.CreatedAt, &item.UpdatedAt, &claimedAt,
		&deliveredAt, &lastError, &metadataJSON,
	)
	if err != nil {
		return protocol.ExecutionReviewDispatch{}, err
	}
	item.Status = protocol.ExecutionReviewDispatchStatus(status)
	item.HandoffID = nullStringValue(handoffID)
	item.QueueItemID = nullStringValue(queueID)
	item.LeaseOwner = nullStringValue(leaseOwner)
	item.LeaseExpiresAt = nullTimePointer(leaseExpires)
	item.ClaimedAt = nullTimePointer(claimedAt)
	item.DeliveredAt = nullTimePointer(deliveredAt)
	item.LastError = nullStringValue(lastError)
	item.Metadata, err = parseMap(metadataJSON)
	return item, err
}

func scanCancellationDispatch(
	scanner interface{ Scan(...any) error },
) (protocol.ExecutionCancellationDispatch, error) {
	var item protocol.ExecutionCancellationDispatch
	var scopeKind, executorKind, targetKind, status string
	var dispatchID, roomID, conversationID, targetAgentID sql.NullString
	var runtimeSessionKey, roomSessionID, sdkSessionID, runtimeRoundID sql.NullString
	var rootRoundID, agentRoundID, childSessionID, sdkTaskID, toolUseID sql.NullString
	var limitationCode, outcome, receipt, leaseOwner, lastError sql.NullString
	var leaseExpires, claimedAt, deliveredAt sql.NullTime
	var metadataJSON string
	err := scanner.Scan(
		&item.ID, &item.ExecutionID, &item.PlanID, &item.WorkItemID, &item.SpecID,
		&item.AssignmentID, &item.AttemptID, &item.RuntimeAttemptID, &dispatchID,
		&item.CommandID, &item.DedupeKey, &scopeKind, &item.ScopeSessionKey,
		&roomID, &conversationID, &executorKind, &targetKind, &targetAgentID,
		&runtimeSessionKey, &roomSessionID, &sdkSessionID, &runtimeRoundID,
		&rootRoundID, &agentRoundID, &childSessionID, &sdkTaskID, &toolUseID,
		&status, &item.Reason, &limitationCode, &outcome, &receipt,
		&item.DeliveryAttempts, &item.Version, &item.AvailableAt, &leaseOwner,
		&leaseExpires, &item.CreatedAt, &item.UpdatedAt, &claimedAt,
		&deliveredAt, &lastError, &metadataJSON,
	)
	if err != nil {
		return protocol.ExecutionCancellationDispatch{}, err
	}
	item.DispatchID = nullStringValue(dispatchID)
	item.ScopeKind = protocol.ExecutionScopeKind(scopeKind)
	item.RoomID = nullStringValue(roomID)
	item.ConversationID = nullStringValue(conversationID)
	item.ExecutorKind = protocol.AttemptExecutorKind(executorKind)
	item.TargetKind = protocol.ExecutionCancellationTargetKind(targetKind)
	item.TargetAgentID = nullStringValue(targetAgentID)
	item.RuntimeSessionKey = nullStringValue(runtimeSessionKey)
	item.RoomSessionID = nullStringValue(roomSessionID)
	item.SDKSessionID = nullStringValue(sdkSessionID)
	item.RuntimeRoundID = nullStringValue(runtimeRoundID)
	item.RootRoundID = nullStringValue(rootRoundID)
	item.AgentRoundID = nullStringValue(agentRoundID)
	item.ChildSessionID = nullStringValue(childSessionID)
	item.SDKTaskID = nullStringValue(sdkTaskID)
	item.ToolUseID = nullStringValue(toolUseID)
	item.Status = protocol.ExecutionCancellationDispatchStatus(status)
	item.LimitationCode = nullStringValue(limitationCode)
	item.Outcome = protocol.ExecutionCancellationOutcome(nullStringValue(outcome))
	item.Receipt = nullStringValue(receipt)
	item.LeaseOwner = nullStringValue(leaseOwner)
	item.LeaseExpiresAt = nullTimePointer(leaseExpires)
	item.ClaimedAt = nullTimePointer(claimedAt)
	item.DeliveredAt = nullTimePointer(deliveredAt)
	item.LastError = nullStringValue(lastError)
	item.Metadata, err = parseMap(metadataJSON)
	return item, err
}

func scanAcceptance(scanner interface{ Scan(...any) error }) (protocol.WorkAcceptance, error) {
	var item protocol.WorkAcceptance
	var decision, reviewerKind, criteriaJSON, metadataJSON string
	var feedback, decisionRoundID sql.NullString
	err := scanner.Scan(
		&item.ID, &item.ExecutionID, &item.PlanID, &item.WorkItemID, &item.SpecID,
		&item.AssignmentID, &item.SubmissionID, &decision, &reviewerKind,
		&item.ReviewerID, &criteriaJSON, &feedback, &decisionRoundID,
		&item.CreatedAt, &metadataJSON,
	)
	if err != nil {
		return protocol.WorkAcceptance{}, err
	}
	item.Decision = protocol.WorkAcceptanceDecision(decision)
	item.ReviewerKind = protocol.WorkReviewerKind(reviewerKind)
	item.Feedback = nullStringValue(feedback)
	item.DecisionRoundID = nullStringValue(decisionRoundID)
	item.CriteriaResults, err = parseSlice[protocol.WorkAcceptanceCriterionResult](criteriaJSON)
	if err != nil {
		return protocol.WorkAcceptance{}, err
	}
	item.Metadata, err = parseMap(metadataJSON)
	return item, err
}

func scanEvent(scanner interface{ Scan(...any) error }) (protocol.ExecutionEvent, error) {
	var item protocol.ExecutionEvent
	var eventType, entityType, actorKind, payloadJSON string
	var actorID, goalID, planID, workItemID, specID, assignmentID sql.NullString
	var dispatchID, attemptID, submissionID, reviewDispatchID, acceptanceID sql.NullString
	var rootRoundID, runtimeRoundID, agentRoundID sql.NullString
	err := scanner.Scan(
		&item.ID, &item.ExecutionID, &item.Sequence, &item.CommandID, &eventType,
		&entityType, &item.EntityID, &item.EntityVersion, &actorKind, &actorID,
		&goalID, &planID, &workItemID, &specID, &assignmentID, &dispatchID,
		&attemptID, &submissionID, &reviewDispatchID, &acceptanceID, &rootRoundID, &runtimeRoundID,
		&agentRoundID, &payloadJSON, &item.CreatedAt,
	)
	if err != nil {
		return protocol.ExecutionEvent{}, err
	}
	item.Type = protocol.ExecutionEventType(eventType)
	item.EntityType = protocol.ExecutionEntityType(entityType)
	item.ActorKind = protocol.ExecutionActorKind(actorKind)
	item.ActorID = nullStringValue(actorID)
	item.GoalID = nullStringValue(goalID)
	item.PlanID = nullStringValue(planID)
	item.WorkItemID = nullStringValue(workItemID)
	item.SpecID = nullStringValue(specID)
	item.AssignmentID = nullStringValue(assignmentID)
	item.DispatchID = nullStringValue(dispatchID)
	item.AttemptID = nullStringValue(attemptID)
	item.SubmissionID = nullStringValue(submissionID)
	item.ReviewDispatchID = nullStringValue(reviewDispatchID)
	item.AcceptanceID = nullStringValue(acceptanceID)
	item.RootRoundID = nullStringValue(rootRoundID)
	item.RuntimeRoundID = nullStringValue(runtimeRoundID)
	item.AgentRoundID = nullStringValue(agentRoundID)
	item.Payload, err = parseMap(payloadJSON)
	return item, err
}
