// INPUT: Agent 选择的 persistence reason 与通过权限/状态校验的 Execution snapshot。
// OUTPUT: Goal identity/revision 和激活原因，以及 SQL BindGoal 后的 durable Goal confirmation。
// POS: Orchestration 到 Goal 服务的消费侧端口；本包不直接修改 Goal 状态。
package orchestration

import (
	"context"
	"errors"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

const (
	// ExecutionMetadataScheduledRetryEvidence 只允许 scheduler/backend 写入。
	ExecutionMetadataScheduledRetryEvidence = "goal_promotion_scheduled_retry_evidence"
	// ExecutionMetadataContextBoundaryEvidence 只允许 runtime lifecycle 写入。
	ExecutionMetadataContextBoundaryEvidence = "goal_promotion_context_boundary_evidence"

	ErrorCodeGoalPromotionDisabled ErrorCode = "goal_promotion_disabled"
	ErrorCodeGoalConflict          ErrorCode = "goal_conflict"
	// ErrorCodeDurableSignalMissing 保留给历史客户端解析；新 promotion 不再以
	// 缺少推荐信号拒绝 Agent 的合法选择。
	ErrorCodeDurableSignalMissing ErrorCode = "durable_signal_missing"
)

var (
	// ErrGoalPromotionDisabled 由 gateway 在配置关闭自动 Goal 时返回。
	ErrGoalPromotionDisabled = errors.New("automatic Goal promotion is disabled")
	// ErrGoalPromotionConflict 由 gateway 在 scope 已存在不相关 Goal 时返回。
	ErrGoalPromotionConflict = errors.New("another Goal is active in this scope")
)

// GoalPromotionProposal 是模型基于任务语义选择的 Goal promotion 意图。
// 权限、current state、existing Goal 和配置开关仍由后端派生。
type GoalPromotionProposal struct {
	ObjectiveProposal string
	ActivationReason  protocol.GoalActivationReason
}

// GoalPromotionRequest 把权威 snapshot 与无授权能力的模型 proposal 交给 Goal gateway。
type GoalPromotionRequest struct {
	CommandID string
	Snapshot  *protocol.ExecutionSnapshot
	Actor     ActorContext
	Proposal  GoalPromotionProposal
}

// GoalPromotionBinding 是 Goal 服务创建或复用 persistence identity 后的结果。
type GoalPromotionBinding struct {
	GoalID                string
	GoalObjectiveRevision int64
	ActivationOrigin      protocol.GoalActivationOrigin
	ActivationReason      protocol.GoalActivationReason
}

// GoalPromotionGateway 隔离 Goal service；Orchestration 不自行创建 Goal。
//
// 实现必须以 Request.CommandID 幂等 find-or-create：若 Goal 已建立而 BindGoal 因
// CAS/基础设施失败，使用同一 CommandID 重试必须返回同一 Goal identity。
type GoalPromotionGateway interface {
	PromoteExecution(context.Context, GoalPromotionRequest) (GoalPromotionBinding, error)
}

// GoalPromotionAvailabilityRequest asks the application layer for policy facts
// that cannot be derived from an Execution snapshot alone.
type GoalPromotionAvailabilityRequest struct {
	Snapshot *protocol.ExecutionSnapshot
	Actor    ActorContext
}

// GoalPromotionAvailability is the current configuration and Goal-scope
// preflight used to keep the model-visible affordance aligned with execution.
type GoalPromotionAvailability struct {
	AutomaticGoalDisabled bool
	ConflictingGoalID     string
}

// GoalPromotionAvailabilityReader is optional on the mutation gateway but
// required before RuntimeContext may advertise automatic promotion.
type GoalPromotionAvailabilityReader interface {
	ReadGoalPromotionAvailability(
		context.Context,
		GoalPromotionAvailabilityRequest,
	) (GoalPromotionAvailability, error)
}

// PromoteExecutionToGoalInput 是模型能够提交的全部 promotion 意图。
type PromoteExecutionToGoalInput struct {
	ExecutionID       string
	SnapshotRevision  int64
	CommandID         string
	ObjectiveProposal string
	ActivationReason  protocol.GoalActivationReason
}

// SetGoalPromotionGateway 注入负责配置/持久证据 policy 与 Goal find-or-create 的 gateway。
func (s *Service) SetGoalPromotionGateway(gateway GoalPromotionGateway) {
	if s != nil {
		s.goalPromotionGateway = gateway
	}
}

// PromoteExecutionToGoal 先校验权威 WorkGraph，再经幂等 gateway 建立 Goal 并绑定 Execution。
func (s *Service) PromoteExecutionToGoal(
	ctx context.Context,
	actor ActorContext,
	input PromoteExecutionToGoalInput,
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
		return RejectedResult(snapshot, domainError(
			ErrorCodeInvalidInput,
			"command_id is required",
		), nil), nil
	}
	if snapshot.Execution.GoalID != "" {
		if confirmErr := s.confirmGoalExecutionBinding(ctx, snapshot); confirmErr != nil {
			return withPendingGoalConfirmation(
				NoOpResult(snapshot, ""),
				"Execution is already durably bound to the Goal; reverse binding confirmation is pending and will retry automatically.",
				NextAction{
					Tool:   "promote_execution_to_goal",
					Reason: "retry the same promotion intent now, or continue while durable background reconciliation confirms the Goal binding",
				},
			), nil
		}
		return withConfirmedGoalAuthority(
			NoOpResult(snapshot, "execution is already bound to a Goal"),
		), nil
	}
	if snapshot.Execution.Status != protocol.ExecutionStatusActive &&
		snapshot.Execution.Status != protocol.ExecutionStatusWaiting {
		return RejectedResult(snapshot, domainError(
			ErrorCodeInvalidInput,
			"only an active or waiting Execution can be promoted",
		), nil), nil
	}
	current, currentErr := s.repository.FindCurrent(
		ctx,
		snapshot.Execution.OwnerUserID,
		snapshot.Execution.SessionKey,
	)
	if currentErr != nil {
		return MutationResult{}, currentErr
	}
	if current == nil || current.ID != snapshot.Execution.ID {
		return RejectedResult(snapshot, domainError(
			ErrorCodeStaleExecution,
			"execution is no longer current for this session",
		), nil), nil
	}
	if strings.TrimSpace(snapshot.Execution.Objective) == "" ||
		len(normalizeNonEmptyValues(snapshot.Execution.CompletionCriteria)) == 0 {
		return RejectedResult(snapshot, domainError(
			ErrorCodeInvalidInput,
			"promotion requires an objective and completion criteria",
		), nil), nil
	}
	if !validAdaptiveActivationReason(input.ActivationReason) {
		return RejectedResult(snapshot, domainError(
			ErrorCodeInvalidInput,
			"activation_reason must identify a durable adaptive boundary",
		), nil), nil
	}
	if s.goalPromotionGateway == nil {
		return MutationResult{}, errors.New("Goal promotion gateway is not configured")
	}
	promotionCommandID := commandPart(input.CommandID, "promote-goal")
	binding, promoteErr := s.goalPromotionGateway.PromoteExecution(ctx, GoalPromotionRequest{
		CommandID: promotionCommandID,
		Snapshot:  snapshot,
		Actor:     actor,
		Proposal: GoalPromotionProposal{
			ObjectiveProposal: strings.TrimSpace(input.ObjectiveProposal),
			ActivationReason:  input.ActivationReason,
		},
	})
	if promoteErr != nil {
		switch {
		case errors.Is(promoteErr, ErrGoalPromotionDisabled):
			return RejectedResult(snapshot, domainError(
				ErrorCodeGoalPromotionDisabled,
				"automatic Goal promotion is disabled by policy",
			), nil), nil
		case errors.Is(promoteErr, ErrGoalPromotionConflict):
			return RejectedResult(snapshot, domainError(
				ErrorCodeGoalConflict,
				"another active Goal conflicts with this Execution",
			), nil), nil
		default:
			var domainErr *DomainError
			if errors.As(promoteErr, &domainErr) {
				return RejectedResult(snapshot, domainErr, nil), nil
			}
			return MutationResult{}, promoteErr
		}
	}
	if strings.TrimSpace(binding.GoalID) == "" ||
		binding.GoalObjectiveRevision <= 0 ||
		binding.ActivationOrigin != protocol.GoalActivationOriginAdaptivePromoted ||
		!validAdaptiveActivationReason(binding.ActivationReason) {
		return MutationResult{}, errors.New("Goal promotion gateway returned an invalid binding")
	}
	updated, bindErr := s.repository.BindGoal(ctx, orchestrationstore.BindGoalCommand{
		ExpectedExecutionVersion: snapshot.Execution.Version,
		Execution: protocol.Execution{
			ID:                    snapshot.Execution.ID,
			GoalID:                strings.TrimSpace(binding.GoalID),
			GoalObjectiveRevision: binding.GoalObjectiveRevision,
			GoalActivationOrigin:  binding.ActivationOrigin,
			GoalActivationReason:  binding.ActivationReason,
		},
		Meta: s.commandMeta(actor, input.CommandID, "bind-goal"),
	})
	if bindErr != nil {
		retry := []NextAction{{
			Tool:   "promote_execution_to_goal",
			Reason: "Goal identity exists; retry with the same semantic arguments so the backend can reuse it and finish binding",
		}}
		result, knownErr := s.storageMutationResult(snapshot, bindErr, retry)
		if knownErr != nil {
			return MutationResult{}, knownErr
		}
		result.Message = "Goal identity exists but Execution binding did not commit; retry with the same semantic arguments"
		return result, nil
	}
	if confirmErr := s.confirmGoalExecutionBinding(ctx, updated); confirmErr != nil {
		return withPendingGoalConfirmation(
			AppliedResult(updated, []string{
				"execution:" + updated.Execution.ID,
				"goal:" + binding.GoalID,
			}, nextActions(updated, actor)),
			"Execution and Goal identity are durable; reverse binding confirmation is pending and will retry automatically.",
			NextAction{
				Tool:   "promote_execution_to_goal",
				Reason: "retry the same promotion intent now, or continue while durable background reconciliation confirms the Goal binding",
			},
		), nil
	}
	return withConfirmedGoalAuthority(AppliedResult(updated, []string{
		"execution:" + updated.Execution.ID,
		"goal:" + binding.GoalID,
	}, nextActions(updated, actor))), nil
}

// GoalExecutionCompletionBlocker 返回 Goal 当前 objective revision 对应 WorkGraph 的完成阻塞。
// future reservation 仍属于 Goal-only；只有 confirmed binding 才审计 WorkGraph，
// pending/conflict 则 fail closed，避免半提交绑定被旁路。
func (s *Service) GoalExecutionCompletionBlocker(
	ctx context.Context,
	goal protocol.Goal,
) (string, error) {
	resolution, err := s.ResolveGoalExecutionBinding(ctx, goal)
	if err != nil {
		return "", err
	}
	switch resolution.State {
	case protocol.GoalExecutionBindingStateStandalone,
		protocol.GoalExecutionBindingStateReserved:
		return "", nil
	case protocol.GoalExecutionBindingStatePending:
		return "execution_binding_pending:" + firstNonEmptyExecutionBindingID(resolution), nil
	case protocol.GoalExecutionBindingStateConflict:
		return "execution_binding_conflict:" + firstNonEmptyExecutionBindingID(resolution), nil
	case protocol.GoalExecutionBindingStateConfirmed:
		snapshot, snapshotErr := s.repository.GetSnapshot(ctx, resolution.ExecutionID)
		if snapshotErr != nil {
			return "", snapshotErr
		}
		if snapshot == nil {
			return "execution_binding_missing:" + resolution.ExecutionID, nil
		}
		return goalExecutionSnapshotBlocker(goal, snapshot), nil
	default:
		return "execution_binding_conflict:unknown", nil
	}
}

func firstNonEmptyExecutionBindingID(
	resolution protocol.GoalExecutionBindingResolution,
) string {
	if value := strings.TrimSpace(resolution.ExecutionID); value != "" {
		return value
	}
	if value := strings.TrimSpace(resolution.ReservedExecutionID); value != "" {
		return value
	}
	return "unknown"
}

func goalExecutionSnapshotBlocker(
	goal protocol.Goal,
	snapshot *protocol.ExecutionSnapshot,
) string {
	if snapshot == nil {
		return "execution_binding_missing"
	}
	execution := snapshot.Execution
	if execution.GoalID != goal.ID ||
		execution.GoalObjectiveRevision != goal.ObjectiveRevision() ||
		execution.SessionKey != goal.SessionKey {
		return "execution_binding_conflict:" + execution.ID
	}
	if snapshot.Execution.Status == protocol.ExecutionStatusCompleted {
		return ""
	}
	switch snapshot.Execution.Status {
	case protocol.ExecutionStatusFailed,
		protocol.ExecutionStatusCancelled,
		protocol.ExecutionStatusSuperseded:
		return "execution_terminal_without_completion:" +
			snapshot.Execution.ID + ":" + string(snapshot.Execution.Status)
	}
	if len(snapshot.CompletionBlockers) == 0 {
		return "execution_completion_pending:" + snapshot.Execution.ID
	}
	return "execution_work_graph:" + snapshot.Execution.ID + ":" +
		snapshot.CompletionBlockers[0]
}

func requiredWorkRemaining(snapshot *protocol.ExecutionSnapshot) bool {
	if snapshot == nil || snapshot.Plan == nil {
		return true
	}
	accepted := make(map[string]bool, len(snapshot.Acceptances))
	for _, acceptance := range snapshot.Acceptances {
		if acceptance.Decision == protocol.WorkAcceptanceAccepted {
			accepted[acceptance.WorkItemID+"\x00"+acceptance.SpecID] = true
		}
	}
	for _, item := range snapshot.PlanItems {
		if (item.Required || item.Terminal) &&
			!accepted[item.WorkItemID+"\x00"+item.SpecID] {
			return true
		}
	}
	return false
}

func validAdaptiveActivationReason(reason protocol.GoalActivationReason) bool {
	switch reason {
	case protocol.GoalActivationReasonObservedBoundary,
		protocol.GoalActivationReasonRoomDependencyChain,
		protocol.GoalActivationReasonExternalWait,
		protocol.GoalActivationReasonScheduledRetry,
		protocol.GoalActivationReasonContextBoundary,
		protocol.GoalActivationReasonRecoveryRequired,
		protocol.GoalActivationReasonSubstantialComplexity:
		return true
	default:
		return false
	}
}

func adaptiveEvidenceFromSnapshot(
	snapshot *protocol.ExecutionSnapshot,
	actor ActorContext,
) AdaptiveGoalEvidence {
	return AdaptiveGoalEvidence{
		ExecutionID:                 snapshot.Execution.ID,
		ObjectiveClear:              strings.TrimSpace(snapshot.Execution.Objective) != "",
		CompletionCriteriaAvailable: len(normalizeNonEmptyValues(snapshot.Execution.CompletionCriteria)) > 0,
		// Promotion preserves this already-authorized Execution objective and
		// adds no capability. Future side effects still pass their own policy
		// checks, so persistence within the same owner/session is authorized.
		ScopeAuthorizesContinuation: true,
		PlanMode:                    actor.PlanMode,
		ExistingGoalID:              snapshot.Execution.GoalID,
		RequiredWorkRemaining:       requiredWorkRemaining(snapshot),

		ObservedBoundaryWithRequiredWork: strings.TrimSpace(snapshot.Execution.RootRoundID) != "" &&
			strings.TrimSpace(actor.RootRoundID) != "" &&
			snapshot.Execution.RootRoundID != strings.TrimSpace(actor.RootRoundID),
		BoundRoomDependency:  hasBoundCrossAgentRoomWork(snapshot),
		RequiredExternalWait: hasRequiredExternalWait(snapshot),
		ScheduledRetry:       metadataBool(snapshot.Execution.Metadata, ExecutionMetadataScheduledRetryEvidence),
		RecoveryRequired: snapshot.Execution.Origin == protocol.ExecutionOriginRecovery ||
			strings.TrimSpace(snapshot.Execution.RecoveryOfExecutionID) != "",
		PredictedContextBoundary: metadataBool(snapshot.Execution.Metadata, ExecutionMetadataContextBoundaryEvidence),
	}
}

func hasBoundCrossAgentRoomWork(snapshot *protocol.ExecutionSnapshot) bool {
	if snapshot == nil ||
		snapshot.Execution.ScopeKind != protocol.ExecutionScopeRoom ||
		snapshot.Plan == nil {
		return false
	}
	remaining := remainingRequiredWork(snapshot)
	if len(remaining) == 0 {
		return false
	}
	coordinator := strings.TrimSpace(snapshot.Execution.CoordinatorAgentID)
	for _, assignment := range snapshot.Assignments {
		specID, required := remaining[assignment.WorkItemID]
		if !required ||
			assignment.PlanID != snapshot.Plan.ID ||
			assignment.SpecID != specID ||
			strings.TrimSpace(assignment.OwnerAgentID) == "" ||
			strings.TrimSpace(assignment.OwnerAgentID) == coordinator {
			continue
		}
		switch assignment.Status {
		case protocol.WorkAssignmentStatusAssigned,
			protocol.WorkAssignmentStatusActive:
			return true
		}
	}
	for _, dispatch := range snapshot.Dispatches {
		specID, required := remaining[dispatch.WorkItemID]
		if !required ||
			dispatch.PlanID != snapshot.Plan.ID ||
			dispatch.SpecID != specID ||
			strings.TrimSpace(dispatch.TargetAgentID) == "" ||
			strings.TrimSpace(dispatch.TargetAgentID) == coordinator {
			continue
		}
		switch dispatch.Kind {
		case protocol.ExecutionDispatchRoomDirected,
			protocol.ExecutionDispatchRoomPublic:
		default:
			continue
		}
		switch dispatch.Status {
		case protocol.ExecutionDispatchStatusPending,
			protocol.ExecutionDispatchStatusClaimed,
			protocol.ExecutionDispatchStatusDelivered:
			return true
		}
	}
	return false
}

func hasRequiredExternalWait(snapshot *protocol.ExecutionSnapshot) bool {
	if snapshot == nil {
		return false
	}
	if snapshot.Execution.Status == protocol.ExecutionStatusWaiting {
		return true
	}
	remaining := remainingRequiredWork(snapshot)
	for _, state := range snapshot.WorkItemStates {
		specID, required := remaining[state.WorkItemID]
		if required &&
			state.CurrentSpecID == specID &&
			state.Status == protocol.WorkItemStatusWaitingInput {
			return true
		}
	}
	return false
}

func remainingRequiredWork(snapshot *protocol.ExecutionSnapshot) map[string]string {
	result := make(map[string]string)
	if snapshot == nil || snapshot.Plan == nil {
		return result
	}
	accepted := make(map[string]bool, len(snapshot.Acceptances))
	for _, acceptance := range snapshot.Acceptances {
		if acceptance.Decision == protocol.WorkAcceptanceAccepted {
			accepted[acceptance.WorkItemID+"\x00"+acceptance.SpecID] = true
		}
	}
	for _, item := range snapshot.PlanItems {
		if item.PlanID != snapshot.Plan.ID ||
			(!item.Required && !item.Terminal) ||
			accepted[item.WorkItemID+"\x00"+item.SpecID] {
			continue
		}
		result[item.WorkItemID] = item.SpecID
	}
	return result
}

func metadataBool(metadata map[string]any, key string) bool {
	value, ok := metadata[key]
	if !ok {
		return false
	}
	result, ok := value.(bool)
	return ok && result
}
