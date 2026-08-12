// INPUT: 当前 active explicit Goal 的只读 authority fence、Execution snapshot 与幂等 command identity。
// OUTPUT: 带 canonical Goal objective 的 proposal activation、Goal -> Execution pending binding、transient Execution -> explicit Goal 的无损 CAS binding/confirmation，以及提交后的 session 失效事实。
// POS: 显式 Goal 与 Execution 共享状态链的领域协调边界；Plan transport 不拥有 Goal objective，Goal persistence 仍由应用层 gateway 负责。
package orchestration

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

var (
	// ErrExplicitGoalObjectiveConflict 表示同一 scope 的 Goal 与 Execution objective 不同。
	ErrExplicitGoalObjectiveConflict = errors.New("explicit Goal objective conflicts with Execution")
	// ErrExplicitGoalScopeConflict 表示 Goal session/scope 与 Execution scope 不同。
	ErrExplicitGoalScopeConflict = errors.New("explicit Goal scope conflicts with Execution")
	// ErrExplicitGoalBindingConflict 表示 Goal 或 Execution 已绑定另一条状态链。
	ErrExplicitGoalBindingConflict = errors.New("explicit Goal binding conflicts with Execution")
)

// ExplicitGoalBindingRequest 让应用层读取并预留当前 explicit Goal 的 Execution binding。
//
// ExistingExecution 区分“给已存在 transient Execution 补绑定”和“为尚未创建的
// Execution 预留 identity”；后者可以复用 Goal metadata 中由上次失败尝试留下的 ID。
type ExplicitGoalBindingRequest struct {
	CandidateExecutionID  string
	ExistingExecution     bool
	ExistingGoalID        string
	GoalObjectiveRevision int64
	OwnerUserID           string
	SessionKey            string
	ScopeKind             protocol.ExecutionScopeKind
	ConversationID        string
	Objective             string
	CompletionCriteria    []string
	AgentID               string
	RootRoundID           string
}

// ExplicitGoalActivationRequest 只携带 proposal sealing 所需的 trusted
// authority fence。Plan 文档中的 objective/criteria 不是 Goal activation 的
// 权威输入；fresh Goal-bound create 必须从 activation 继承 canonical objective。
type ExplicitGoalActivationRequest struct {
	ExistingGoalID        string
	GoalObjectiveRevision int64
	OwnerUserID           string
	SessionKey            string
	ScopeKind             protocol.ExecutionScopeKind
	ConversationID        string
	AgentID               string
}

// ExplicitGoalBinding 是可直接写入 Execution aggregate root 的完整 Goal identity。
type ExplicitGoalBinding struct {
	ExecutionID           string
	GoalID                string
	GoalObjectiveRevision int64
	ActivationOrigin      protocol.GoalActivationOrigin
	ActivationReason      protocol.GoalActivationReason
	ReplacesExecutionID   string
}

// ExplicitGoalActivation 是 proposal sealing 所需的只读 Goal provenance。
// ReservedExecutionID 只投影 Goal metadata 中已经持久化的 successor identity；
// resolver 本身不预留 identity，也不写 Goal metadata。
type ExplicitGoalActivation struct {
	GoalID                string
	GoalObjectiveRevision int64
	Objective             string
	ActivationOrigin      protocol.GoalActivationOrigin
	ActivationReason      protocol.GoalActivationReason
	ReservedExecutionID   string
	ReplacesExecutionID   string
}

// ExplicitGoalActivationResolver 在 Plan Mode 也必须保持只读。
type ExplicitGoalActivationResolver interface {
	ResolveExplicitGoalActivation(
		context.Context,
		ExplicitGoalActivationRequest,
	) (*ExplicitGoalActivation, error)
}

// ExplicitGoalBindingGateway 隔离 Goal service，并在创建 Execution 前持久化反向 metadata。
type ExplicitGoalBindingGateway interface {
	PrepareExplicitGoalBinding(
		context.Context,
		ExplicitGoalBindingRequest,
	) (*ExplicitGoalBinding, error)
}

type goalExecutionBindingConfirmer interface {
	ConfirmGoalExecutionBinding(
		context.Context,
		GoalExecutionBindingConfirmation,
	) error
}

// GoalExecutionBindingConfirmation is emitted only after the Goal-bound
// successor Execution and its first active Plan are durable.
type GoalExecutionBindingConfirmation struct {
	GoalID                string
	GoalObjectiveRevision int64
	ExecutionID           string
	CompletionCriteria    []string
}

// BindExplicitGoalInput 把已经创建的 explicit Goal 绑定到现有 transient Execution。
type BindExplicitGoalInput struct {
	ExecutionID           string
	SnapshotRevision      int64
	CommandID             string
	GoalID                string
	GoalObjectiveRevision int64
	Objective             string
}

// SetExplicitGoalBindingGateway 注入 explicit Goal lookup/metadata reservation gateway。
func (s *Service) SetExplicitGoalBindingGateway(gateway ExplicitGoalBindingGateway) {
	if s != nil {
		s.explicitGoalGateway = gateway
	}
}

// BindExplicitGoal 在 create_goal 已建立 durable Goal identity 后无损绑定当前 Execution。
func (s *Service) BindExplicitGoal(
	ctx context.Context,
	actor ActorContext,
	input BindExplicitGoalInput,
) (returned MutationResult, returnedErr error) {
	defer func() { s.invalidateMutationResult(ctx, returned, returnedErr) }()
	if actor.PlanMode {
		return RejectedResult(nil, planModeError(), nil), nil
	}
	snapshot, err := s.GetSnapshot(ctx, actor, input.ExecutionID)
	if err != nil {
		var domainErr *DomainError
		if errors.As(err, &domainErr) {
			return RejectedResult(nil, err, nil), nil
		}
		return MutationResult{}, err
	}
	if snapshot == nil {
		return RejectedResult(nil, domainError(
			ErrorCodeNoCurrentExecution,
			"explicit Goal binding requires a current Execution",
		), nil), nil
	}
	if err = requireCoordinator(actor, snapshot); err != nil {
		return RejectedResult(snapshot, err, nil), nil
	}
	if expectedErr := requireMutationRevision(snapshot, input.SnapshotRevision); expectedErr != nil {
		return RejectedResult(snapshot, expectedErr, nextActions(snapshot, actor)), nil
	}
	if strings.TrimSpace(input.CommandID) == "" {
		return RejectedResult(snapshot, domainError(
			ErrorCodeInvalidInput,
			"command_id is required",
		), nil), nil
	}
	if strings.TrimSpace(input.GoalID) == "" || input.GoalObjectiveRevision <= 0 {
		return RejectedResult(snapshot, domainError(
			ErrorCodeInvalidInput,
			"goal_id and goal_objective_revision are required",
		), nil), nil
	}
	if strings.TrimSpace(input.Objective) != strings.TrimSpace(snapshot.Execution.Objective) {
		return RejectedResult(snapshot, explicitGoalDomainError(
			ErrExplicitGoalObjectiveConflict,
			"explicit Goal objective must exactly match the current Execution objective",
		), nil), nil
	}
	if snapshot.Execution.Status != protocol.ExecutionStatusActive &&
		snapshot.Execution.Status != protocol.ExecutionStatusWaiting &&
		snapshot.Execution.Status != protocol.ExecutionStatusPaused {
		return RejectedResult(snapshot, domainError(
			ErrorCodeGoalBindingConflict,
			"only a current Execution can bind an explicit Goal",
		), nil), nil
	}

	binding := ExplicitGoalBinding{
		ExecutionID:           snapshot.Execution.ID,
		GoalID:                strings.TrimSpace(input.GoalID),
		GoalObjectiveRevision: input.GoalObjectiveRevision,
		ActivationOrigin:      protocol.GoalActivationOriginUserExplicit,
		ActivationReason:      protocol.GoalActivationReasonPersistenceRequested,
	}
	if snapshot.Execution.GoalID != "" {
		if executionHasExplicitGoalBinding(snapshot.Execution, binding) {
			if confirmErr := s.confirmGoalExecutionBinding(ctx, snapshot); confirmErr != nil {
				return withPendingGoalConfirmation(
					NoOpResult(snapshot, ""),
					"Explicit Goal and Execution are durably bound; reverse binding confirmation is pending and will retry automatically.",
					NextAction{
						Tool:   "get_execution",
						Reason: "continue from the durable Execution while background reconciliation confirms the Goal binding",
					},
				), nil
			}
			return s.activateRuntimeCoordinationResult(
				ctx,
				actor,
				NoOpResult(snapshot, "explicit Goal is already bound to this Execution"),
			), nil
		}
		return RejectedResult(snapshot, explicitGoalDomainError(
			ErrExplicitGoalBindingConflict,
			"Execution is already bound to another Goal or objective revision",
		), nil), nil
	}
	return s.persistExplicitGoalBinding(ctx, actor, input.CommandID, snapshot, binding)
}

func (s *Service) prepareExplicitGoalBinding(
	ctx context.Context,
	actor ActorContext,
	execution protocol.Execution,
	existing bool,
) (*ExplicitGoalBinding, error) {
	if s == nil || s.explicitGoalGateway == nil {
		return nil, nil
	}
	return s.explicitGoalGateway.PrepareExplicitGoalBinding(ctx, ExplicitGoalBindingRequest{
		CandidateExecutionID:  strings.TrimSpace(execution.ID),
		ExistingExecution:     existing,
		ExistingGoalID:        strings.TrimSpace(execution.GoalID),
		GoalObjectiveRevision: execution.GoalObjectiveRevision,
		OwnerUserID:           strings.TrimSpace(execution.OwnerUserID),
		SessionKey:            strings.TrimSpace(execution.SessionKey),
		ScopeKind:             execution.ScopeKind,
		ConversationID:        strings.TrimSpace(execution.ConversationID),
		Objective:             strings.TrimSpace(execution.Objective),
		CompletionCriteria:    append([]string(nil), execution.CompletionCriteria...),
		AgentID:               strings.TrimSpace(actor.AgentID),
		RootRoundID:           strings.TrimSpace(actor.RootRoundID),
	})
}

// GoalBindingConfirmationPendingError 区分“权威 mutation 已提交但 Goal 回执
// 尚未确认”和“mutation 前的既有 Goal 回执暂时不可用”。proposal materializer
// 依靠 DurableMutation 决定是否可以先保存 materialized receipt，再异步重试确认。
type GoalBindingConfirmationPendingError struct {
	Snapshot        *protocol.ExecutionSnapshot
	DurableMutation bool
	Err             error
}

func (e *GoalBindingConfirmationPendingError) Error() string {
	if e == nil || e.Err == nil {
		return "Goal binding confirmation is pending"
	}
	return "Goal binding confirmation is pending: " + e.Err.Error()
}

func (e *GoalBindingConfirmationPendingError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (s *Service) persistExplicitGoalBinding(
	ctx context.Context,
	actor ActorContext,
	commandID string,
	snapshot *protocol.ExecutionSnapshot,
	binding ExplicitGoalBinding,
) (MutationResult, error) {
	if err := validateExplicitGoalBinding(binding); err != nil {
		return RejectedResult(snapshot, err, nil), nil
	}
	updated, bindErr := s.repository.BindGoal(ctx, orchestrationstore.BindGoalCommand{
		ExpectedExecutionVersion: snapshot.Execution.Version,
		Execution: protocol.Execution{
			ID:                    snapshot.Execution.ID,
			GoalID:                binding.GoalID,
			GoalObjectiveRevision: binding.GoalObjectiveRevision,
			GoalActivationOrigin:  binding.ActivationOrigin,
			GoalActivationReason:  binding.ActivationReason,
		},
		Meta: s.commandMeta(actor, commandID, "bind-explicit-goal"),
	})
	if bindErr != nil {
		return s.storageMutationResult(snapshot, bindErr, nextActions(snapshot, actor))
	}
	if confirmErr := s.confirmGoalExecutionBinding(ctx, updated); confirmErr != nil {
		return withPendingGoalConfirmation(
			AppliedResult(updated, []string{
				"execution:" + updated.Execution.ID,
				"goal:" + binding.GoalID,
			}, nextActions(updated, actor)),
			"Explicit Goal and Execution are durable; reverse binding confirmation is pending and will retry automatically.",
			NextAction{
				Tool:   "get_execution",
				Reason: "continue from the durable Execution while background reconciliation confirms the Goal binding",
			},
		), nil
	}
	return s.activateRuntimeCoordinationResult(
		ctx,
		actor,
		AppliedResult(updated, []string{
			"execution:" + updated.Execution.ID,
			"goal:" + binding.GoalID,
		}, nextActions(updated, actor)),
	), nil
}

func validateExplicitGoalBinding(binding ExplicitGoalBinding) error {
	if strings.TrimSpace(binding.ExecutionID) == "" ||
		strings.TrimSpace(binding.GoalID) == "" ||
		binding.GoalObjectiveRevision <= 0 {
		return domainError(ErrorCodeGoalBindingConflict, "explicit Goal gateway returned an incomplete binding")
	}
	switch binding.ActivationOrigin {
	case protocol.GoalActivationOriginUserExplicit,
		protocol.GoalActivationOriginAdaptiveInitial,
		protocol.GoalActivationOriginAdaptivePromoted:
	default:
		return domainError(ErrorCodeGoalBindingConflict, "Goal gateway returned unsupported activation provenance")
	}
	if binding.ActivationReason == "" {
		return domainError(ErrorCodeGoalBindingConflict, "Goal gateway returned no activation reason")
	}
	return nil
}

func executionHasExplicitGoalBinding(
	execution protocol.Execution,
	binding ExplicitGoalBinding,
) bool {
	return strings.TrimSpace(execution.ID) == strings.TrimSpace(binding.ExecutionID) &&
		strings.TrimSpace(execution.GoalID) == strings.TrimSpace(binding.GoalID) &&
		execution.GoalObjectiveRevision == binding.GoalObjectiveRevision &&
		execution.GoalActivationOrigin == binding.ActivationOrigin &&
		execution.GoalActivationReason == binding.ActivationReason
}

func explicitGoalDomainError(cause error, message string) error {
	code := ErrorCodeGoalBindingConflict
	switch {
	case errors.Is(cause, ErrExplicitGoalObjectiveConflict):
		code = ErrorCodeGoalObjectiveConflict
	case errors.Is(cause, ErrExplicitGoalScopeConflict):
		code = ErrorCodeGoalScopeConflict
	}
	if strings.TrimSpace(message) == "" {
		message = cause.Error()
	}
	return newDomainError(code, strings.TrimSpace(message), "", "")
}

func mapExplicitGoalGatewayError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrExplicitGoalObjectiveConflict),
		errors.Is(err, ErrExplicitGoalScopeConflict),
		errors.Is(err, ErrExplicitGoalBindingConflict):
		return explicitGoalDomainError(err, err.Error())
	default:
		return fmt.Errorf("prepare explicit Goal binding: %w", err)
	}
}
