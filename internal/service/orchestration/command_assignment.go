// INPUT: 责任分配、接管与 Attempt 选择。
// OUTPUT: 原子命令结果与后续责任动作。
// POS: Orchestration 命令的业务阶段，复用同包授权、版本栅栏与结果投影。
package orchestration

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

// AssignWorkInput 把 Ready Work Item 交给一个责任 Agent。
type AssignWorkInput struct {
	ExecutionID      string
	SnapshotRevision int64
	CommandID        string
	WorkItemID       string
	LogicalKey       string
	TargetAgentID    string
	ReturnToAgentID  string
	Strategy         protocol.AssignmentStrategy
	Reason           string
	Instruction      string
	DispatchKind     protocol.ExecutionDispatchKind
}

// TakeOverWorkInput 由 coordinator 原子替换当前责任 Agent。
type TakeOverWorkInput struct {
	ExecutionID      string
	SnapshotRevision int64
	CommandID        string
	WorkItemID       string
	LogicalKey       string
	TargetAgentID    string
	ReturnToAgentID  string
	Strategy         protocol.AssignmentStrategy
	Reason           string
	Instruction      string
	DispatchKind     protocol.ExecutionDispatchKind
}

// AssignWork 创建 current Assignment、可选 Room dispatch 和 pending root Attempt。
func (s *Service) AssignWork(
	ctx context.Context,
	actor ActorContext,
	input AssignWorkInput,
) (returned MutationResult, returnedErr error) {
	defer func() { s.invalidateMutationResult(ctx, returned, returnedErr) }()
	snapshot, rejected, err := s.mutableSnapshot(
		ctx,
		actor,
		input.ExecutionID,
		input.SnapshotRevision,
		true,
		false,
	)
	if err != nil || rejected != nil {
		return resultOrZero(rejected), err
	}
	if strings.TrimSpace(input.CommandID) == "" {
		return RejectedResult(snapshot, domainError(ErrorCodeInvalidInput, "command_id is required"), nil), nil
	}
	work, spec, resolveErr := resolvePlanWork(snapshot, input.WorkItemID, input.LogicalKey)
	if resolveErr != nil {
		return RejectedResult(snapshot, resolveErr, nil), nil
	}
	if assignment := activeAssignmentForWork(snapshot, work.ID); assignment != nil {
		if matchingRoomSelfAssignmentRequest(actor, snapshot, assignment, input) {
			result := NoOpResult(snapshot, "work is already assigned to the current Room actor")
			return withRoomSelfWorkBindingReceipt(actor, result, work.ID), nil
		}
		return RejectedResult(snapshot, newDomainError(
			ErrorCodeDuplicateAssignment,
			"Work Item already has a current Assignment",
			work.LogicalKey,
			assignment.ID,
		), nextActions(snapshot, actor)), nil
	}
	if !slices.Contains(snapshot.ReadyWorkItemIDs, work.ID) {
		return RejectedResult(snapshot, newDomainError(
			ErrorCodeDependencyNotAccepted,
			"Work Item is not ready; inspect hard dependencies and lifecycle state",
			work.LogicalKey,
			"",
		), nextActions(snapshot, actor)), nil
	}
	target := strings.TrimSpace(input.TargetAgentID)
	if target == "" {
		return RejectedResult(snapshot, domainError(
			ErrorCodeInvalidInput,
			"target_agent_id is required",
		), nil), nil
	}
	assignment, dispatch, attempt, buildErr := s.buildAssignmentChain(
		actor,
		snapshot,
		work,
		spec,
		target,
		input.ReturnToAgentID,
		input.Strategy,
		input.Reason,
		"",
		input.Instruction,
		input.DispatchKind,
		input.CommandID,
	)
	if buildErr != nil {
		return RejectedResult(snapshot, buildErr, nil), nil
	}
	if targetErr := s.authorizeAssignmentTarget(ctx, actor, snapshot, assignment, dispatch); targetErr != nil {
		return RejectedResult(snapshot, targetErr, nextActions(snapshot, actor)), nil
	}
	updated, assignErr := s.repository.Assign(ctx, orchestrationstore.AssignCommand{
		ExpectedExecutionVersion: input.SnapshotRevision,
		Assignment:               assignment,
		Dispatch:                 dispatch,
		RootAttempt:              &attempt,
		Meta:                     s.commandMeta(actor, input.CommandID, "assign"),
	})
	if assignErr != nil {
		return s.storageMutationResult(snapshot, assignErr, nextActions(snapshot, actor))
	}
	changed := []string{
		"assignment:" + assignment.ID,
		"attempt:" + attempt.ID,
	}
	if dispatch != nil {
		changed = append(changed, "dispatch:"+dispatch.ID)
	}
	result := AppliedResult(updated, changed, nextActions(updated, actor))
	return withRoomSelfWorkBindingReceipt(actor, result, work.ID), nil
}

func matchingRoomSelfAssignmentRequest(
	actor ActorContext,
	snapshot *protocol.ExecutionSnapshot,
	assignment *protocol.WorkAssignment,
	input AssignWorkInput,
) bool {
	if snapshot == nil || assignment == nil ||
		snapshot.Execution.ScopeKind != protocol.ExecutionScopeRoom ||
		actor.ScopeKind != protocol.ExecutionScopeRoom ||
		actor.WorkBinding != nil || actor.ReviewBinding != nil ||
		assignment.Strategy != protocol.AssignmentStrategySelf ||
		strings.TrimSpace(assignment.OwnerAgentID) != strings.TrimSpace(actor.AgentID) ||
		strings.TrimSpace(input.TargetAgentID) != strings.TrimSpace(actor.AgentID) ||
		(input.Strategy != "" && input.Strategy != protocol.AssignmentStrategySelf) ||
		input.DispatchKind != "" {
		return false
	}
	returnTo := strings.TrimSpace(input.ReturnToAgentID)
	if returnTo == "" {
		returnTo = strings.TrimSpace(snapshot.Execution.CoordinatorAgentID)
	}
	return returnTo != "" &&
		returnTo == strings.TrimSpace(assignment.ReturnToAgentID)
}

func withRoomSelfWorkBindingReceipt(
	actor ActorContext,
	result MutationResult,
	workItemID string,
) MutationResult {
	if result.Snapshot == nil ||
		result.Snapshot.Execution.ScopeKind != protocol.ExecutionScopeRoom ||
		actor.ScopeKind != protocol.ExecutionScopeRoom ||
		actor.WorkBinding != nil ||
		actor.ReviewBinding != nil {
		return result
	}
	assignment := activeAssignmentForWork(result.Snapshot, strings.TrimSpace(workItemID))
	attempt := rootAttemptForAssignment(result.Snapshot, assignment)
	if assignment == nil || attempt == nil || result.Snapshot.Plan == nil ||
		assignment.Strategy != protocol.AssignmentStrategySelf ||
		strings.TrimSpace(assignment.OwnerAgentID) != strings.TrimSpace(actor.AgentID) ||
		assignment.ID != attempt.AssignmentID ||
		assignment.ExecutionID != result.Snapshot.Execution.ID ||
		assignment.PlanID != result.Snapshot.Plan.ID ||
		assignment.WorkItemID != attempt.WorkItemID ||
		assignment.SpecID != attempt.SpecID ||
		strings.TrimSpace(attempt.DispatchID) != "" ||
		attempt.ParentAttemptID != "" {
		return result
	}
	result.WorkBinding = &WorkBindingReceipt{Binding: &protocol.ExecutionWorkBinding{
		ExecutionID:  assignment.ExecutionID,
		PlanID:       assignment.PlanID,
		WorkItemID:   assignment.WorkItemID,
		SpecID:       assignment.SpecID,
		AssignmentID: assignment.ID,
		AttemptID:    attempt.ID,
	}}
	return result
}

func rootAttemptForAssignment(
	snapshot *protocol.ExecutionSnapshot,
	assignment *protocol.WorkAssignment,
) *protocol.WorkAttempt {
	if snapshot == nil || assignment == nil {
		return nil
	}
	var succeeded *protocol.WorkAttempt
	for index := range snapshot.Attempts {
		attempt := &snapshot.Attempts[index]
		if attempt.AssignmentID != assignment.ID || attempt.ParentAttemptID != "" {
			continue
		}
		if attempt.Status == protocol.WorkAttemptStatusPending ||
			attempt.Status == protocol.WorkAttemptStatusRunning {
			return attempt
		}
		if attempt.Status == protocol.WorkAttemptStatusSucceeded &&
			(succeeded == nil || attempt.CreatedAt.After(succeeded.CreatedAt)) {
			succeeded = attempt
		}
	}
	return succeeded
}

// TakeOverWork 原子释放旧责任链并建立 replacement Assignment。
func (s *Service) TakeOverWork(
	ctx context.Context,
	actor ActorContext,
	input TakeOverWorkInput,
) (returned MutationResult, returnedErr error) {
	defer func() { s.invalidateMutationResult(ctx, returned, returnedErr) }()
	snapshot, rejected, err := s.mutableSnapshot(
		ctx,
		actor,
		input.ExecutionID,
		input.SnapshotRevision,
		true,
		false,
	)
	if err != nil || rejected != nil {
		return resultOrZero(rejected), err
	}
	if strings.TrimSpace(input.CommandID) == "" || strings.TrimSpace(input.Reason) == "" {
		return RejectedResult(snapshot, domainError(
			ErrorCodeInvalidInput,
			"command_id and takeover reason are required",
		), nil), nil
	}
	work, spec, resolveErr := resolvePlanWork(snapshot, input.WorkItemID, input.LogicalKey)
	if resolveErr != nil {
		return RejectedResult(snapshot, resolveErr, nil), nil
	}
	current := activeAssignmentForWork(snapshot, work.ID)
	if current == nil {
		return RejectedResult(snapshot, newDomainError(
			ErrorCodeInvalidInput,
			"Work Item has no current Assignment to take over",
			work.LogicalKey,
			"",
		), nextActions(snapshot, actor)), nil
	}
	if submission := latestUnreviewedSubmissionForSpec(snapshot, work.ID, spec.ID); submission != nil {
		return RejectedResult(snapshot, newDomainError(
			ErrorCodeCompletionBlocked,
			"review the pending Submission before taking over this Work Item",
			work.LogicalKey,
			submission.ID,
		), nextActions(snapshot, actor)), nil
	}
	target := strings.TrimSpace(input.TargetAgentID)
	if target == "" {
		return RejectedResult(snapshot, domainError(ErrorCodeInvalidInput, "target_agent_id is required"), nil), nil
	}
	replacement, dispatch, attempt, buildErr := s.buildAssignmentChain(
		actor,
		snapshot,
		work,
		spec,
		target,
		input.ReturnToAgentID,
		input.Strategy,
		"",
		input.Reason,
		input.Instruction,
		input.DispatchKind,
		input.CommandID,
	)
	if buildErr != nil {
		return RejectedResult(snapshot, buildErr, nil), nil
	}
	if targetErr := s.authorizeAssignmentTarget(ctx, actor, snapshot, replacement, dispatch); targetErr != nil {
		return RejectedResult(snapshot, targetErr, nextActions(snapshot, actor)), nil
	}
	updated, takeoverErr := s.repository.Takeover(ctx, orchestrationstore.TakeoverCommand{
		ExpectedExecutionVersion:         snapshot.Execution.Version,
		ExpectedCurrentAssignmentVersion: current.Version,
		CurrentAssignmentID:              current.ID,
		Replacement:                      replacement,
		Dispatch:                         dispatch,
		RootAttempt:                      &attempt,
		Meta:                             s.commandMeta(actor, input.CommandID, "takeover"),
	})
	if takeoverErr != nil {
		return s.storageMutationResult(snapshot, takeoverErr, nextActions(snapshot, actor))
	}
	result := AppliedResult(updated, []string{
		"assignment:" + replacement.ID,
		"assignment_released:" + current.ID,
		"attempt:" + attempt.ID,
	}, nextActions(updated, actor))
	return withRoomSelfWorkBindingReceipt(actor, result, work.ID), nil
}

func (s *Service) buildAssignmentChain(
	actor ActorContext,
	snapshot *protocol.ExecutionSnapshot,
	work protocol.WorkItem,
	spec protocol.WorkItemSpec,
	target string,
	returnTo string,
	strategy protocol.AssignmentStrategy,
	assignmentReason string,
	takeoverReason string,
	instruction string,
	dispatchKind protocol.ExecutionDispatchKind,
	commandID string,
) (
	protocol.WorkAssignment,
	*protocol.ExecutionDispatch,
	protocol.WorkAttempt,
	error,
) {
	actorAgentID := strings.TrimSpace(actor.AgentID)
	if strategy == "" {
		strategy = protocol.AssignmentStrategyRoomMember
		if target == actorAgentID {
			strategy = protocol.AssignmentStrategySelf
		}
	}
	if strategy == protocol.AssignmentStrategyRoomMember &&
		snapshot.Execution.ScopeKind != protocol.ExecutionScopeRoom {
		return protocol.WorkAssignment{}, nil, protocol.WorkAttempt{}, domainError(
			ErrorCodeAssignmentTargetInvalid,
			"room_member Assignment requires a Room Execution",
		)
	}
	if strategy != protocol.AssignmentStrategySelf &&
		strategy != protocol.AssignmentStrategyRoomMember {
		return protocol.WorkAssignment{}, nil, protocol.WorkAttempt{}, domainError(
			ErrorCodeInvalidInput,
			"unknown assignment strategy",
		)
	}
	if strategy == protocol.AssignmentStrategySelf {
		if target != actorAgentID {
			return protocol.WorkAssignment{}, nil, protocol.WorkAttempt{}, domainError(
				ErrorCodeAssignmentTargetInvalid,
				"self Assignment target must be the current actor",
			)
		}
		if dispatchKind != "" {
			return protocol.WorkAssignment{}, nil, protocol.WorkAttempt{}, domainError(
				ErrorCodeAssignmentTargetInvalid,
				"self Assignment must not request a Room Dispatch",
			)
		}
	}
	if strategy == protocol.AssignmentStrategyRoomMember &&
		target == actorAgentID {
		return protocol.WorkAssignment{}, nil, protocol.WorkAttempt{}, domainError(
			ErrorCodeAssignmentTargetInvalid,
			"assign the current actor with strategy self",
		)
	}
	coordinatorAgentID := strings.TrimSpace(snapshot.Execution.CoordinatorAgentID)
	if returnTo = strings.TrimSpace(returnTo); returnTo == "" {
		returnTo = coordinatorAgentID
	}
	if returnTo == "" {
		return protocol.WorkAssignment{}, nil, protocol.WorkAttempt{}, domainError(
			ErrorCodeInvalidInput,
			"Assignment return target requires a coordinator",
		)
	}
	assignment := protocol.WorkAssignment{
		ID:                s.id("assignment"),
		ExecutionID:       snapshot.Execution.ID,
		PlanID:            snapshot.Plan.ID,
		WorkItemID:        work.ID,
		SpecID:            spec.ID,
		OwnerAgentID:      target,
		AssignedByAgentID: actorAgentID,
		ReturnToAgentID:   returnTo,
		Strategy:          strategy,
		Status:            protocol.WorkAssignmentStatusAssigned,
		AssignmentReason:  strings.TrimSpace(assignmentReason),
		TakeoverReason:    strings.TrimSpace(takeoverReason),
	}
	attempt := protocol.WorkAttempt{
		ID:              s.id("attempt"),
		ExecutionID:     snapshot.Execution.ID,
		PlanID:          snapshot.Plan.ID,
		WorkItemID:      work.ID,
		SpecID:          spec.ID,
		AssignmentID:    assignment.ID,
		ExecutorKind:    protocol.AttemptExecutorAgent,
		ExecutorAgentID: target,
		Status:          protocol.WorkAttemptStatusPending,
	}
	var dispatch *protocol.ExecutionDispatch
	if strategy == protocol.AssignmentStrategyRoomMember {
		if dispatchKind == "" {
			dispatchKind = protocol.ExecutionDispatchRoomDirected
		}
		if dispatchKind != protocol.ExecutionDispatchRoomDirected &&
			dispatchKind != protocol.ExecutionDispatchRoomPublic {
			return protocol.WorkAssignment{}, nil, protocol.WorkAttempt{}, domainError(
				ErrorCodeInvalidInput,
				"Room Assignment requires room_directed or room_public dispatch",
			)
		}
		if instruction = strings.TrimSpace(instruction); instruction == "" {
			instruction = fmt.Sprintf(
				"Deliver %s. Acceptance criteria: %s",
				spec.Deliverable,
				strings.Join(spec.AcceptanceCriteria, "; "),
			)
		}
		dispatch = &protocol.ExecutionDispatch{
			ID:            s.id("dispatch"),
			DedupeKey:     commandPart(commandID, "dispatch:"+work.ID+":"+target),
			TargetAgentID: target,
			Kind:          dispatchKind,
			Status:        protocol.ExecutionDispatchStatusPending,
			Instruction:   instruction,
		}
	}
	return assignment, dispatch, attempt, nil
}

func currentAssignment(assignment protocol.WorkAssignment) bool {
	return assignment.Status == protocol.WorkAssignmentStatusAssigned ||
		assignment.Status == protocol.WorkAssignmentStatusActive
}

func activeAssignmentForWork(
	snapshot *protocol.ExecutionSnapshot,
	workItemID string,
) *protocol.WorkAssignment {
	for index := range snapshot.Assignments {
		assignment := &snapshot.Assignments[index]
		if assignment.WorkItemID == workItemID && currentAssignment(*assignment) {
			return assignment
		}
	}
	return nil
}

func latestAssignmentForCurrentSpec(
	snapshot *protocol.ExecutionSnapshot,
	workItemID string,
	specID string,
) *protocol.WorkAssignment {
	if snapshot == nil {
		return nil
	}
	if !isCurrentExecutionStatus(snapshot.Execution.Status) {
		return nil
	}
	var latest *protocol.WorkAssignment
	for index := range snapshot.Assignments {
		assignment := &snapshot.Assignments[index]
		if assignment.WorkItemID != workItemID || assignment.SpecID != specID ||
			(snapshot.Plan != nil && assignment.PlanID != snapshot.Plan.ID) {
			continue
		}
		if latest == nil ||
			assignment.AssignedAt.After(latest.AssignedAt) ||
			(assignment.AssignedAt.Equal(latest.AssignedAt) && assignment.ID > latest.ID) {
			latest = assignment
		}
	}
	return latest
}

func selectAssignment(
	snapshot *protocol.ExecutionSnapshot,
	workItemID string,
	assignmentID string,
) *protocol.WorkAssignment {
	assignmentID = strings.TrimSpace(assignmentID)
	for index := range snapshot.Assignments {
		assignment := &snapshot.Assignments[index]
		if assignment.WorkItemID == workItemID &&
			(assignmentID == "" || assignment.ID == assignmentID) &&
			currentAssignment(*assignment) {
			return assignment
		}
	}
	return nil
}

func findAssignmentByID(
	snapshot *protocol.ExecutionSnapshot,
	assignmentID string,
) *protocol.WorkAssignment {
	if snapshot == nil {
		return nil
	}
	for index := range snapshot.Assignments {
		if snapshot.Assignments[index].ID == assignmentID {
			return &snapshot.Assignments[index]
		}
	}
	return nil
}

func findAttemptByID(
	snapshot *protocol.ExecutionSnapshot,
	attemptID string,
) *protocol.WorkAttempt {
	if snapshot == nil {
		return nil
	}
	for index := range snapshot.Attempts {
		if snapshot.Attempts[index].ID == attemptID {
			return &snapshot.Attempts[index]
		}
	}
	return nil
}

func currentOrSucceededAttempt(
	snapshot *protocol.ExecutionSnapshot,
	assignmentID string,
) *protocol.WorkAttempt {
	var succeeded *protocol.WorkAttempt
	for index := range snapshot.Attempts {
		attempt := &snapshot.Attempts[index]
		if attempt.AssignmentID != assignmentID {
			continue
		}
		if attempt.Status == protocol.WorkAttemptStatusPending ||
			attempt.Status == protocol.WorkAttemptStatusRunning {
			return attempt
		}
		if attempt.Status == protocol.WorkAttemptStatusSucceeded {
			if succeeded == nil || attempt.CreatedAt.After(succeeded.CreatedAt) {
				succeeded = attempt
			}
		}
	}
	return succeeded
}
