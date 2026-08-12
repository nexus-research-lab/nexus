// INPUT: Assignment target preflight、current Spec/accepted dependency、raw Room wake、SQL Dispatch lease 与 Room delivery receipt。
// OUTPUT: 先授权后持久化的目标边界、超限 fail-closed 的可执行 WorkContract、重复唤醒栅栏、区分永久/瞬时失败的 outbox 消费循环与状态变更后的 session 失效事实。
// POS: orchestration 不依赖 Room realtime 的消费侧端口，聊天 @ 不得绕过 structured Dispatch。
package orchestration

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

const roomAttemptActivationMutationAttempts = 3

// AssignmentTargetRequest 是 Assignment 持久化前必须通过的作用域校验。
type AssignmentTargetRequest struct {
	OwnerUserID    string
	SessionKey     string
	ExecutionID    string
	RoomID         string
	ConversationID string
	ActorAgentID   string
	TargetAgentID  string
	Strategy       protocol.AssignmentStrategy
}

// AssignmentTargetAuthorizer 验证 Room member 身份；实现不得依赖模型文本或 @。
type AssignmentTargetAuthorizer interface {
	AuthorizeAssignmentTarget(context.Context, AssignmentTargetRequest) error
}

// ExecutionDispatchDelivery 是交给 Room 数据面的完整结构化工作交接。
type ExecutionDispatchDelivery struct {
	OwnerUserID       string
	SessionKey        string
	RoomID            string
	ConversationID    string
	SourceAgentID     string
	TargetAgentID     string
	Kind              protocol.ExecutionDispatchKind
	Instruction       string
	WorkContract      ExecutionDispatchWorkContract
	Binding           protocol.ExecutionWorkBinding
	DispatchDedupeKey string
}

// ExecutionDispatchWorkContract 是 Room instruction 的有界可执行输入摘要。
// runtime 动态上下文仍是权威；这里只携带 current Spec 与已验收依赖。
type ExecutionDispatchWorkContract struct {
	InputRefs            []string
	OutputScopes         []protocol.WorkOutputScope
	AcceptedDependencies []ExecutionAcceptedDependency
}

// ExecutionAcceptedDependency 只投影通过 Acceptance 的上游交付。
type ExecutionAcceptedDependency struct {
	WorkItemID      string
	LogicalKey      string
	SpecID          string
	Kind            protocol.WorkDependencyKind
	SubmissionID    string
	ResultSummary   string
	ResultRefs      []string
	Evidence        []string
	AcceptanceID    string
	CriteriaResults []protocol.WorkAcceptanceCriterionResult
}

// ExecutionDispatchReceipt 表示 Room 已同步接受到 slot 或 durable queue。
type ExecutionDispatchReceipt struct {
	HandoffID   string
	QueueItemID string
}

// RoomAttemptActivationInput carries physical slot identity that the model cannot provide.
type RoomAttemptActivationInput struct {
	Binding           protocol.ExecutionWorkBinding
	RuntimeSessionKey string
	RoomSessionID     string
}

// ExecutionDispatchConsumer 是 Room realtime 在消费侧实现的投递端口。
type ExecutionDispatchConsumer interface {
	DeliverExecutionDispatch(context.Context, ExecutionDispatchDelivery) (ExecutionDispatchReceipt, error)
}

// PermanentDispatchDeliveryError 表示重试无法修复的 Room delivery contract/admission 失败。
// consumer 只应用它包装权威状态已经证明永久失效的错误，不得包装瞬时 I/O/runtime 错误。
type PermanentDispatchDeliveryError struct {
	Cause error
}

func (e *PermanentDispatchDeliveryError) Error() string {
	if e == nil || e.Cause == nil {
		return "permanent execution dispatch delivery failure"
	}
	return e.Cause.Error()
}

func (e *PermanentDispatchDeliveryError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// PermanentExecutionDispatchDelivery 标记 consumer 已权威确认的永久 delivery 失败。
func PermanentExecutionDispatchDelivery(cause error) error {
	if cause == nil {
		return nil
	}
	return &PermanentDispatchDeliveryError{Cause: cause}
}

func isPermanentExecutionDispatchDelivery(cause error) bool {
	var permanent *PermanentDispatchDeliveryError
	if errors.As(cause, &permanent) {
		return true
	}
	var domain *DomainError
	return errors.As(cause, &domain)
}

type dispatchOutboxRepository interface {
	ListAvailableRoomDispatches(context.Context, int) ([]protocol.ExecutionDispatch, error)
	ClaimDispatch(context.Context, string, int64, string, time.Duration) (*protocol.ExecutionDispatch, error)
	MarkDispatchDelivered(context.Context, string, int64, string, string, string) (*protocol.ExecutionDispatch, error)
	RetryDispatch(context.Context, string, int64, string, time.Time, string) (*protocol.ExecutionDispatch, error)
	CancelDispatch(context.Context, string, int64, string, string) (*protocol.ExecutionDispatch, error)
	GetSnapshot(context.Context, string) (*protocol.ExecutionSnapshot, error)
}

// DispatchRunResult 汇总一次有界 outbox drain。
type DispatchRunResult struct {
	Claimed   int
	Delivered int
	Retried   int
	Cancelled int
}

// SetAssignmentTargetAuthorizer 注入 Room membership 的权威读取器。
func (s *Service) SetAssignmentTargetAuthorizer(authorizer AssignmentTargetAuthorizer) {
	s.assignmentTargets = authorizer
}

// SetExecutionDispatchConsumer 注入 Room structured delivery 数据面。
func (s *Service) SetExecutionDispatchConsumer(consumer ExecutionDispatchConsumer) {
	s.dispatchConsumer = consumer
}

func (s *Service) authorizeAssignmentTarget(
	ctx context.Context,
	actor ActorContext,
	snapshot *protocol.ExecutionSnapshot,
	assignment protocol.WorkAssignment,
	dispatch *protocol.ExecutionDispatch,
) error {
	target := strings.TrimSpace(assignment.OwnerAgentID)
	reviewer := strings.TrimSpace(assignment.ReturnToAgentID)
	actorID := strings.TrimSpace(actor.AgentID)
	switch assignment.Strategy {
	case protocol.AssignmentStrategySelf:
		if target != actorID {
			return domainError(
				ErrorCodeAssignmentTargetInvalid,
				"self Assignment target must be the current actor",
			)
		}
		if dispatch != nil {
			return domainError(
				ErrorCodeAssignmentTargetInvalid,
				"self Assignment must not create a Room Dispatch",
			)
		}
		if snapshot == nil || snapshot.Execution.ScopeKind != protocol.ExecutionScopeRoom ||
			reviewer == target {
			return nil
		}
		return s.authorizeRoomMemberTarget(
			ctx,
			actorID,
			reviewer,
			snapshot,
			"reviewer",
		)
	case protocol.AssignmentStrategyRoomMember:
		if snapshot == nil || snapshot.Execution.ScopeKind != protocol.ExecutionScopeRoom {
			return domainError(
				ErrorCodeAssignmentTargetInvalid,
				"room_member Assignment requires a Room Execution",
			)
		}
		if target == actorID {
			return domainError(
				ErrorCodeAssignmentTargetInvalid,
				"assign the current actor with strategy self",
			)
		}
		if dispatch == nil {
			return domainError(
				ErrorCodeAssignmentTargetInvalid,
				"room_member Assignment requires a structured Room Dispatch",
			)
		}
		if err := s.authorizeRoomMemberTarget(
			ctx,
			actorID,
			target,
			snapshot,
			"target",
		); err != nil {
			return err
		}
		if reviewer == "" {
			return domainError(
				ErrorCodeRoomReviewerRequired,
				"Room Assignment requires an explicit review return target",
			)
		}
		if reviewer == target {
			return nil
		}
		if reviewer == strings.TrimSpace(snapshot.Execution.CoordinatorAgentID) {
			return nil
		}
		return s.authorizeRoomMemberTarget(
			ctx,
			actorID,
			reviewer,
			snapshot,
			"reviewer",
		)
	default:
		return domainError(ErrorCodeAssignmentTargetInvalid, "unknown Assignment strategy")
	}
}

func (s *Service) authorizeRoomMemberTarget(
	ctx context.Context,
	actorAgentID string,
	targetAgentID string,
	snapshot *protocol.ExecutionSnapshot,
	role string,
) error {
	if s.assignmentTargets == nil {
		return domainError(
			ErrorCodeAssignmentTargetInvalid,
			"Room assignment target authorizer is unavailable",
		)
	}
	if err := s.assignmentTargets.AuthorizeAssignmentTarget(ctx, AssignmentTargetRequest{
		OwnerUserID:    snapshot.Execution.OwnerUserID,
		SessionKey:     snapshot.Execution.SessionKey,
		ExecutionID:    snapshot.Execution.ID,
		RoomID:         snapshot.Execution.RoomID,
		ConversationID: snapshot.Execution.ConversationID,
		ActorAgentID:   actorAgentID,
		TargetAgentID:  targetAgentID,
		Strategy:       protocol.AssignmentStrategyRoomMember,
	}); err != nil {
		return newDomainError(
			ErrorCodeAssignmentTargetInvalid,
			role+" is not an authorized Room member: "+strings.TrimSpace(err.Error()),
			"",
			targetAgentID,
		)
	}
	return nil
}

// AuthorizeRoomRuntimeTarget 是逐 round 的 Execution capability admission。
//
// nil binding 永远表示 conversation transport，即使同一 Room 同时存在 active
// Execution；它不读取或激活 WorkGraph。只有结构化 Dispatch 携带的 binding
// 才校验并授予 Plan/Spec/Assignment/Attempt/Dispatch 责任链。
func (s *Service) AuthorizeRoomRuntimeTarget(
	ctx context.Context,
	actor ActorContext,
	binding *protocol.ExecutionWorkBinding,
) error {
	if binding == nil {
		return nil
	}
	if strings.TrimSpace(binding.ExecutionID) == "" {
		return domainError(ErrorCodeAssignmentTargetInvalid, "execution binding is incomplete")
	}
	actor.ExecutionID = binding.ExecutionID
	bindingCopy := *binding
	actor.WorkBinding = &bindingCopy
	snapshot, err := s.GetSnapshot(ctx, actor, binding.ExecutionID)
	if err != nil {
		return err
	}
	if snapshot == nil {
		return domainError(ErrorCodeAssignmentTargetInvalid, "bound Execution was not found")
	}
	if snapshot.Execution.Status != protocol.ExecutionStatusActive &&
		snapshot.Execution.Status != protocol.ExecutionStatusWaiting {
		return domainError(
			ErrorCodeExecutionTerminal,
			"Room target Execution is not active and cannot admit runtime work",
		)
	}
	if snapshot.Execution.ScopeKind != protocol.ExecutionScopeRoom ||
		strings.TrimSpace(snapshot.Execution.RoomID) != strings.TrimSpace(actor.RoomID) ||
		strings.TrimSpace(snapshot.Execution.ConversationID) != strings.TrimSpace(actor.ConversationID) {
		return domainError(ErrorCodeAssignmentTargetInvalid, "Room target is outside the managed Execution scope")
	}
	targetAgentID := strings.TrimSpace(actor.AgentID)
	activePlanID := ""
	if snapshot.Plan != nil &&
		snapshot.Plan.Status == protocol.PlanRevisionStatusActive {
		activePlanID = strings.TrimSpace(snapshot.Plan.ID)
	}
	var assignment *protocol.WorkAssignment
	for index := range snapshot.Assignments {
		candidate := &snapshot.Assignments[index]
		if candidate.OwnerAgentID != targetAgentID ||
			activePlanID == "" ||
			candidate.PlanID != activePlanID ||
			(candidate.Status != protocol.WorkAssignmentStatusAssigned &&
				candidate.Status != protocol.WorkAssignmentStatusActive) {
			continue
		}
		planMember := false
		for _, item := range snapshot.PlanItems {
			if item.PlanID == activePlanID &&
				item.WorkItemID == candidate.WorkItemID &&
				item.SpecID == candidate.SpecID {
				planMember = true
				break
			}
		}
		if !planMember {
			continue
		}
		if candidate.ID != strings.TrimSpace(binding.AssignmentID) ||
			candidate.PlanID != strings.TrimSpace(binding.PlanID) ||
			candidate.WorkItemID != strings.TrimSpace(binding.WorkItemID) ||
			candidate.SpecID != strings.TrimSpace(binding.SpecID) {
			continue
		}
		assignment = candidate
		break
	}
	if assignment == nil {
		return domainError(
			ErrorCodeAssignmentTargetInvalid,
			"target Agent has no matching current Assignment",
		)
	}
	if strings.TrimSpace(binding.DispatchID) == "" ||
		strings.TrimSpace(binding.AttemptID) == "" ||
		strings.TrimSpace(binding.PlanID) == "" ||
		strings.TrimSpace(binding.WorkItemID) == "" ||
		strings.TrimSpace(binding.SpecID) == "" ||
		strings.TrimSpace(binding.AssignmentID) == "" {
		return domainError(ErrorCodeAssignmentTargetInvalid, "execution binding is incomplete")
	}
	dispatchMatches := false
	for _, dispatch := range snapshot.Dispatches {
		if dispatch.ID == strings.TrimSpace(binding.DispatchID) &&
			dispatch.AssignmentID == assignment.ID &&
			dispatch.TargetAgentID == targetAgentID &&
			dispatch.PlanID == assignment.PlanID &&
			dispatch.WorkItemID == assignment.WorkItemID &&
			dispatch.SpecID == assignment.SpecID &&
			dispatch.Status != protocol.ExecutionDispatchStatusCancelled &&
			dispatch.Status != protocol.ExecutionDispatchStatusFailed {
			dispatchMatches = true
			break
		}
	}
	if !dispatchMatches {
		return domainError(ErrorCodeAssignmentTargetInvalid, "Dispatch binding is stale or does not match the target")
	}
	for _, attempt := range snapshot.Attempts {
		if attempt.ID == strings.TrimSpace(binding.AttemptID) &&
			attempt.ExecutionID == assignment.ExecutionID &&
			attempt.PlanID == assignment.PlanID &&
			attempt.WorkItemID == assignment.WorkItemID &&
			attempt.SpecID == assignment.SpecID &&
			attempt.DispatchID == strings.TrimSpace(binding.DispatchID) &&
			attempt.AssignmentID == assignment.ID &&
			attempt.ExecutorAgentID == targetAgentID &&
			(attempt.Status == protocol.WorkAttemptStatusPending ||
				attempt.Status == protocol.WorkAttemptStatusRunning) {
			return nil
		}
	}
	return domainError(ErrorCodeAssignmentTargetInvalid, "Attempt binding is stale or does not match the target")
}

// ActivateRoomAttempt marks a dispatch-bound root Attempt running only after the
// target Room slot has actually accepted its runtime query.
func (s *Service) ActivateRoomAttempt(
	ctx context.Context,
	actor ActorContext,
	input RoomAttemptActivationInput,
) error {
	binding := input.Binding
	if strings.TrimSpace(binding.ExecutionID) == "" ||
		strings.TrimSpace(binding.AssignmentID) == "" ||
		strings.TrimSpace(binding.AttemptID) == "" ||
		strings.TrimSpace(binding.DispatchID) == "" {
		return domainError(ErrorCodeAssignmentTargetInvalid, "Room Attempt binding is incomplete")
	}
	actor.ExecutionID = binding.ExecutionID
	bindingCopy := binding
	actor.WorkBinding = &bindingCopy
	for range roomAttemptActivationMutationAttempts {
		snapshot, err := s.GetSnapshot(ctx, actor, binding.ExecutionID)
		if err != nil {
			return err
		}
		if snapshot == nil {
			return domainError(ErrorCodeNoCurrentExecution, "bound Room Execution was not found")
		}
		if err = authorizeBoundRoomAttempt(snapshot, actor.AgentID, binding); err != nil {
			return err
		}
		assignment := findAssignmentByID(snapshot, binding.AssignmentID)
		attempt := findAttemptByID(snapshot, binding.AttemptID)
		if assignment == nil || attempt == nil {
			return domainError(ErrorCodeSubagentBindingMissing, "Room Assignment root Attempt is missing")
		}
		if attempt.Status == protocol.WorkAttemptStatusRunning {
			s.invalidateSnapshot(ctx, snapshot)
			return nil
		}
		if attempt.Status != protocol.WorkAttemptStatusPending {
			return domainError(
				ErrorCodeDuplicateAttempt,
				"Room Assignment root Attempt is already terminal",
			)
		}
		running := *attempt
		running.ExecutorKind = protocol.AttemptExecutorAgent
		running.ExecutorAgentID = strings.TrimSpace(actor.AgentID)
		running.RuntimeSessionKey = strings.TrimSpace(input.RuntimeSessionKey)
		running.RoomSessionID = strings.TrimSpace(input.RoomSessionID)
		running.RuntimeRoundID = strings.TrimSpace(actor.RuntimeRoundID)
		running.RootRoundID = strings.TrimSpace(actor.RootRoundID)
		running.AgentRoundID = strings.TrimSpace(actor.AgentRoundID)
		updated, startErr := s.repository.StartAttempt(ctx, orchestrationstore.StartAttemptCommand{
			ExpectedExecutionVersion:  snapshot.Execution.Version,
			ExpectedAssignmentVersion: assignment.Version,
			ExpectedAttemptVersion:    attempt.Version,
			Attempt:                   running,
			Meta: s.commandMeta(
				actor,
				"room-attempt-activate:"+binding.DispatchID,
				"room-attempt-activate",
			),
		})
		if errors.Is(startErr, orchestrationstore.ErrVersionConflict) ||
			errors.Is(startErr, orchestrationstore.ErrInvariant) {
			continue
		}
		if startErr == nil {
			s.invalidateSnapshot(ctx, updated)
		}
		return startErr
	}
	return domainError(
		ErrorCodeStaleExecution,
		"Room Attempt state changed concurrently; retry slot activation",
	)
}

func authorizeBoundRoomAttempt(
	snapshot *protocol.ExecutionSnapshot,
	targetAgentID string,
	binding protocol.ExecutionWorkBinding,
) error {
	if snapshot == nil || snapshot.Plan == nil ||
		snapshot.Execution.ScopeKind != protocol.ExecutionScopeRoom ||
		snapshot.Execution.ID != strings.TrimSpace(binding.ExecutionID) ||
		snapshot.Plan.ID != strings.TrimSpace(binding.PlanID) {
		return domainError(ErrorCodeAssignmentTargetInvalid, "Room Attempt is outside the active Execution Plan")
	}
	assignment := findAssignmentByID(snapshot, binding.AssignmentID)
	if assignment == nil ||
		assignment.OwnerAgentID != strings.TrimSpace(targetAgentID) ||
		assignment.PlanID != binding.PlanID ||
		assignment.WorkItemID != binding.WorkItemID ||
		assignment.SpecID != binding.SpecID ||
		!currentAssignment(*assignment) {
		return domainError(ErrorCodeAssignmentTargetInvalid, "Room Attempt Assignment binding is stale")
	}
	dispatchMatched := false
	for _, dispatch := range snapshot.Dispatches {
		if dispatch.ID == binding.DispatchID &&
			dispatch.AssignmentID == assignment.ID &&
			dispatch.TargetAgentID == strings.TrimSpace(targetAgentID) &&
			dispatch.PlanID == binding.PlanID &&
			dispatch.WorkItemID == binding.WorkItemID &&
			dispatch.SpecID == binding.SpecID &&
			dispatch.Status != protocol.ExecutionDispatchStatusCancelled &&
			dispatch.Status != protocol.ExecutionDispatchStatusFailed {
			dispatchMatched = true
			break
		}
	}
	if !dispatchMatched {
		return domainError(ErrorCodeAssignmentTargetInvalid, "Room Attempt Dispatch binding is stale")
	}
	attempt := findAttemptByID(snapshot, binding.AttemptID)
	if attempt == nil ||
		attempt.ParentAttemptID != "" ||
		attempt.DispatchID != binding.DispatchID ||
		attempt.AssignmentID != assignment.ID ||
		attempt.ExecutorKind != protocol.AttemptExecutorAgent ||
		attempt.ExecutorAgentID != strings.TrimSpace(targetAgentID) {
		return domainError(ErrorCodeAssignmentTargetInvalid, "Room root Attempt binding is stale")
	}
	return nil
}

// DispatchPending claims and delivers a bounded batch. A Room acceptance ACK means
// the work is either represented by a new slot or durably queued for that target.
func (s *Service) DispatchPending(
	ctx context.Context,
	workerID string,
	limit int,
) (DispatchRunResult, error) {
	var result DispatchRunResult
	repository, ok := s.repository.(dispatchOutboxRepository)
	if !ok {
		return result, errors.New("orchestration repository does not support dispatch outbox")
	}
	if s.dispatchConsumer == nil {
		return result, errors.New("execution dispatch consumer is unavailable")
	}
	workerID = strings.TrimSpace(workerID)
	if workerID == "" {
		return result, errors.New("dispatch worker id is required")
	}
	candidates, err := repository.ListAvailableRoomDispatches(ctx, limit)
	if err != nil {
		return result, err
	}
	for _, candidate := range candidates {
		claimed, claimErr := repository.ClaimDispatch(
			ctx,
			candidate.ID,
			candidate.Version,
			workerID,
			30*time.Second,
		)
		if errors.Is(claimErr, orchestrationstore.ErrDispatchLease) {
			continue
		}
		if claimErr != nil {
			return result, claimErr
		}
		result.Claimed++
		// Claim is itself durable and visible in the WorkGraph dispatch state.
		s.invalidateExecutionID(ctx, candidate.ExecutionID)
		delivered, deliveryErr := s.deliverClaimedDispatch(
			ctx,
			repository,
			workerID,
			claimed,
		)
		if deliveryErr != nil {
			result.Retried++
			s.invalidateExecutionID(ctx, candidate.ExecutionID)
			continue
		}
		if delivered {
			result.Delivered++
		} else {
			result.Cancelled++
		}
		s.invalidateExecutionID(ctx, candidate.ExecutionID)
	}
	return result, nil
}

func (s *Service) deliverClaimedDispatch(
	ctx context.Context,
	repository dispatchOutboxRepository,
	workerID string,
	dispatch *protocol.ExecutionDispatch,
) (bool, error) {
	if dispatch == nil {
		return false, errors.New("claimed dispatch is nil")
	}
	snapshot, err := repository.GetSnapshot(ctx, dispatch.ExecutionID)
	if err != nil {
		return false, s.retryClaimedDispatch(ctx, repository, workerID, dispatch, err)
	}
	if snapshot == nil {
		return false, s.cancelClaimedDispatch(
			ctx,
			repository,
			workerID,
			dispatch,
			errors.New("execution snapshot not found"),
		)
	}
	attemptID := ""
	for _, attempt := range snapshot.Attempts {
		if attempt.DispatchID == dispatch.ID &&
			attempt.AssignmentID == dispatch.AssignmentID &&
			(attempt.Status == protocol.WorkAttemptStatusPending ||
				attempt.Status == protocol.WorkAttemptStatusRunning) {
			attemptID = attempt.ID
			break
		}
	}
	if attemptID == "" {
		return false, s.cancelClaimedDispatch(
			ctx,
			repository,
			workerID,
			dispatch,
			errors.New("dispatch has no current root attempt"),
		)
	}
	workContract, contractErr := executionDispatchWorkContract(
		snapshot,
		dispatch.PlanID,
		dispatch.WorkItemID,
		dispatch.SpecID,
	)
	if contractErr != nil {
		return false, s.cancelClaimedDispatch(
			ctx,
			repository,
			workerID,
			dispatch,
			contractErr,
		)
	}
	receipt, err := s.dispatchConsumer.DeliverExecutionDispatch(ctx, ExecutionDispatchDelivery{
		OwnerUserID:       snapshot.Execution.OwnerUserID,
		SessionKey:        snapshot.Execution.SessionKey,
		RoomID:            snapshot.Execution.RoomID,
		ConversationID:    snapshot.Execution.ConversationID,
		SourceAgentID:     snapshot.Execution.CoordinatorAgentID,
		TargetAgentID:     dispatch.TargetAgentID,
		Kind:              dispatch.Kind,
		Instruction:       dispatch.Instruction,
		WorkContract:      workContract,
		DispatchDedupeKey: dispatch.DedupeKey,
		Binding: protocol.ExecutionWorkBinding{
			ExecutionID:  dispatch.ExecutionID,
			PlanID:       dispatch.PlanID,
			WorkItemID:   dispatch.WorkItemID,
			SpecID:       dispatch.SpecID,
			AssignmentID: dispatch.AssignmentID,
			AttemptID:    attemptID,
			DispatchID:   dispatch.ID,
		},
	})
	if err != nil {
		if isPermanentExecutionDispatchDelivery(err) {
			return false, s.cancelClaimedDispatch(
				ctx,
				repository,
				workerID,
				dispatch,
				err,
			)
		}
		return false, s.retryClaimedDispatch(ctx, repository, workerID, dispatch, err)
	}
	_, err = repository.MarkDispatchDelivered(
		ctx,
		dispatch.ID,
		dispatch.Version,
		workerID,
		receipt.HandoffID,
		receipt.QueueItemID,
	)
	return err == nil, err
}

func executionDispatchWorkContract(
	snapshot *protocol.ExecutionSnapshot,
	planID string,
	workItemID string,
	specID string,
) (ExecutionDispatchWorkContract, error) {
	var result ExecutionDispatchWorkContract
	if snapshot == nil || snapshot.Plan == nil {
		return result, errors.New("dispatch has no current Plan")
	}
	planID = strings.TrimSpace(planID)
	workItemID = strings.TrimSpace(workItemID)
	specID = strings.TrimSpace(specID)
	if snapshot.Plan.ID != planID ||
		snapshot.Plan.Status != protocol.PlanRevisionStatusActive {
		return result, errors.New("dispatch is outside the current active Plan")
	}
	planItemMatched := false
	for _, item := range snapshot.PlanItems {
		if item.PlanID == planID &&
			item.ExecutionID == snapshot.Execution.ID &&
			item.WorkItemID == workItemID &&
			item.SpecID == specID {
			planItemMatched = true
			break
		}
	}
	if !planItemMatched {
		return result, errors.New("dispatch Work Item is outside the current active Plan")
	}
	var spec *protocol.WorkItemSpec
	for index := range snapshot.WorkItemSpecs {
		candidate := &snapshot.WorkItemSpecs[index]
		if candidate.ID == specID &&
			candidate.WorkItemID == workItemID &&
			candidate.ExecutionID == snapshot.Execution.ID {
			spec = candidate
			break
		}
	}
	if spec == nil {
		return result, errors.New("dispatch current Work Item spec is missing")
	}
	if err := protocol.ValidateExecutionProjectionLimit(
		"acceptance_criteria",
		len(spec.AcceptanceCriteria),
	); err != nil {
		return result, err
	}
	if err := protocol.ValidateExecutionProjectionLimit(
		"input_refs",
		len(spec.InputRefs),
	); err != nil {
		return result, err
	}
	view := newExecutionContextView(snapshot)
	result.InputRefs = sortedUniqueValues(spec.InputRefs)
	result.OutputScopes = view.outputScopes(workItemID, specID)
	if err := protocol.ValidateExecutionProjectionLimit(
		"output_scopes",
		len(result.OutputScopes),
	); err != nil {
		return ExecutionDispatchWorkContract{}, err
	}
	resolvedDependencies := view.resolvedDependencies(workItemID)
	if err := protocol.ValidateExecutionProjectionLimit(
		"depends_on",
		len(resolvedDependencies),
	); err != nil {
		return ExecutionDispatchWorkContract{}, err
	}
	for _, resolved := range resolvedDependencies {
		if resolved.status != "accepted" ||
			resolved.submission == nil ||
			resolved.acceptance == nil {
			continue
		}
		for _, collection := range []struct {
			field string
			count int
		}{
			{field: "result_refs", count: len(resolved.submission.ResultRefs)},
			{field: "submission_evidence", count: len(resolved.submission.Evidence)},
			{field: "criteria_results", count: len(resolved.acceptance.CriteriaResults)},
		} {
			if err := protocol.ValidateExecutionProjectionLimit(
				collection.field,
				collection.count,
			); err != nil {
				return ExecutionDispatchWorkContract{}, err
			}
		}
		criteriaResults := append(
			[]protocol.WorkAcceptanceCriterionResult(nil),
			resolved.acceptance.CriteriaResults...,
		)
		slices.SortFunc(
			criteriaResults,
			func(left, right protocol.WorkAcceptanceCriterionResult) int {
				return strings.Compare(
					strings.TrimSpace(left.Criterion),
					strings.TrimSpace(right.Criterion),
				)
			},
		)
		for index := range criteriaResults {
			if err := protocol.ValidateExecutionProjectionLimit(
				"criteria_results.evidence",
				len(criteriaResults[index].Evidence),
			); err != nil {
				return ExecutionDispatchWorkContract{}, err
			}
			criteriaResults[index].Evidence = sortedUniqueValues(criteriaResults[index].Evidence)
		}
		result.AcceptedDependencies = append(
			result.AcceptedDependencies,
			ExecutionAcceptedDependency{
				WorkItemID:      resolved.workItem.ID,
				LogicalKey:      resolved.workItem.LogicalKey,
				SpecID:          resolved.planItem.SpecID,
				Kind:            resolved.dependency.Kind,
				SubmissionID:    resolved.submission.ID,
				ResultSummary:   strings.TrimSpace(resolved.submission.ResultSummary),
				ResultRefs:      sortedUniqueValues(resolved.submission.ResultRefs),
				Evidence:        sortedUniqueValues(resolved.submission.Evidence),
				AcceptanceID:    resolved.acceptance.ID,
				CriteriaResults: criteriaResults,
			},
		)
	}
	return result, nil
}

func (s *Service) retryClaimedDispatch(
	ctx context.Context,
	repository dispatchOutboxRepository,
	workerID string,
	dispatch *protocol.ExecutionDispatch,
	cause error,
) error {
	delay := time.Second << min(dispatch.DeliveryAttempts-1, 6)
	_, retryErr := repository.RetryDispatch(
		ctx,
		dispatch.ID,
		dispatch.Version,
		workerID,
		s.now().UTC().Add(delay),
		cause.Error(),
	)
	if retryErr != nil {
		return errors.Join(cause, retryErr)
	}
	return cause
}

func (s *Service) cancelClaimedDispatch(
	ctx context.Context,
	repository dispatchOutboxRepository,
	workerID string,
	dispatch *protocol.ExecutionDispatch,
	cause error,
) error {
	_, cancelErr := repository.CancelDispatch(
		ctx,
		dispatch.ID,
		dispatch.Version,
		workerID,
		cause.Error(),
	)
	return cancelErr
}
