// INPUT: 当前 owner/session/actor/structured WorkBinding/ReviewBinding/round coordination 身份、Execution ensure/read 请求、explicit Goal gateway 与 SQL Repository port。
// OUTPUT: 强制首次顶层完成标准、受 owner/session/scope/coordinator/Goal/Room Work/Review binding 保护并支持 Room 共享图只读观察、review-to-coordination/durable completion recovery 的 Execution snapshot，以及提交后的只读投影失效事实。
// POS: Execution Orchestration 应用服务入口；模型语义 command 见 commands.go。
package orchestration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/duework"
	"github.com/nexus-research-lab/nexus/internal/protocol"
	orchestrationstore "github.com/nexus-research-lab/nexus/internal/storage/orchestration"
)

// Repository 是应用服务在消费侧声明的持久化能力。
//
// storage/orchestration.Repository 满足该接口；测试和上层装配不依赖其具体类型。
type Repository interface {
	Create(context.Context, orchestrationstore.CreateCommand) (*protocol.ExecutionSnapshot, error)
	Get(context.Context, string) (*protocol.Execution, error)
	FindCurrent(context.Context, string, string) (*protocol.Execution, error)
	FindCurrentByGoal(context.Context, string, int64) (*protocol.Execution, error)
	GetSnapshot(context.Context, string) (*protocol.ExecutionSnapshot, error)
	CreateWithPlan(context.Context, orchestrationstore.CreateWithPlanCommand) (*protocol.ExecutionSnapshot, error)
	ReplaceWithPlan(context.Context, orchestrationstore.ReplaceWithPlanCommand) (*protocol.ExecutionSnapshot, error)
	Abandon(context.Context, orchestrationstore.AbandonCommand) (*protocol.ExecutionSnapshot, error)
	SupersedeGoalRevision(context.Context, orchestrationstore.SupersedeGoalRevisionCommand) (*protocol.ExecutionSnapshot, error)
	FenceGoalExecutionIdentity(context.Context, orchestrationstore.FenceGoalExecutionIdentityCommand) (bool, error)
	WritePlan(context.Context, orchestrationstore.WritePlanCommand) (*protocol.ExecutionSnapshot, error)
	Assign(context.Context, orchestrationstore.AssignCommand) (*protocol.ExecutionSnapshot, error)
	StartAttempt(context.Context, orchestrationstore.StartAttemptCommand) (*protocol.ExecutionSnapshot, error)
	FinishAttempt(context.Context, orchestrationstore.FinishAttemptCommand) (*protocol.ExecutionSnapshot, error)
	Submit(context.Context, orchestrationstore.SubmitCommand) (*protocol.ExecutionSnapshot, error)
	Review(context.Context, orchestrationstore.ReviewCommand) (*protocol.ExecutionSnapshot, error)
	Block(context.Context, orchestrationstore.BlockCommand) (*protocol.ExecutionSnapshot, error)
	Resume(context.Context, orchestrationstore.ResumeCommand) (*protocol.ExecutionSnapshot, error)
	Takeover(context.Context, orchestrationstore.TakeoverCommand) (*protocol.ExecutionSnapshot, error)
	BindGoal(context.Context, orchestrationstore.BindGoalCommand) (*protocol.ExecutionSnapshot, error)
	Complete(context.Context, orchestrationstore.CompleteCommand) (*protocol.ExecutionSnapshot, error)
}

// ActorContext 是每次应用调用的权威身份，而不是模型可自行填写的业务参数。
type ActorContext struct {
	OwnerUserID           string
	SessionKey            string
	ExecutionID           string
	WorkBinding           *protocol.ExecutionWorkBinding
	ReviewBinding         *protocol.ExecutionReviewBinding
	GoalID                string
	GoalObjectiveRevision int64
	AgentID               string
	Role                  ExecutionActorRole
	ActorKind             protocol.ExecutionActorKind
	ScopeKind             protocol.ExecutionScopeKind
	RoomID                string
	ConversationID        string
	RootRoundID           string
	RuntimeRoundID        string
	AgentRoundID          string
	PlanMode              bool
	ObservationOnly       bool
}

// RuntimeGoalBinding 是 runtime 能力层从 trusted Execution identity 派生出的
// Goal capability。聊天文本、Room 成员身份和 session 上的 ambient Goal 都不能
// 构造该绑定。
type RuntimeGoalBinding struct {
	ExecutionID           string
	SessionKey            string
	GoalID                string
	GoalObjectiveRevision int64
}

// EnsureInput 描述当前会话需要建立的 transient Execution。
type EnsureInput struct {
	CommandID             string
	Objective             string
	CompletionCriteria    []string
	Origin                protocol.ExecutionOrigin
	TriggerMessageID      string
	RecoveryOfExecutionID string
	Metadata              map[string]any
}

// Service 编排模型语义操作，并把机器状态转换交给 Repository。
type Service struct {
	repository             Repository
	planProposals          PlanProposalRepository
	goalConfirmations      GoalConfirmationRepository
	completionAudits       CompletionAuditRepository
	subagentToolHistory    RuntimeGraphSubagentToolHistoryProvider
	goalPromotionGateway   GoalPromotionGateway
	explicitGoalGateway    ExplicitGoalBindingGateway
	assignmentTargets      AssignmentTargetAuthorizer
	dispatchConsumer       ExecutionDispatchConsumer
	reviewDispatchConsumer ExecutionReviewDispatchConsumer
	cancellationConsumer   ExecutionCancellationConsumer
	invalidationMu         sync.RWMutex
	invalidationSink       ExecutionInvalidationSink
	coordinationMu         sync.RWMutex
	coordinationRounds     map[string]string
	dispatchLoop           *duework.Loop
	subagentLoop           *duework.Loop
	recoveryLoop           *duework.Loop
	now                    func() time.Time
	newID                  func(string) string
}

// NewService 创建 Execution Orchestration 应用服务。
func NewService(repository Repository) *Service {
	planProposals, _ := repository.(PlanProposalRepository)
	goalConfirmations, _ := repository.(GoalConfirmationRepository)
	completionAudits, _ := repository.(CompletionAuditRepository)
	service := &Service{
		repository:         repository,
		planProposals:      planProposals,
		goalConfirmations:  goalConfirmations,
		completionAudits:   completionAudits,
		coordinationRounds: make(map[string]string),
		now:                time.Now,
		newID:              newOrchestrationID,
	}
	service.dispatchLoop = duework.New(duework.Options{
		AuditInterval: 30 * time.Second,
		Now:           func() time.Time { return service.currentTime() },
	})
	service.subagentLoop = duework.New(duework.Options{
		AuditInterval: 30 * time.Second,
		Now:           func() time.Time { return service.currentTime() },
	})
	service.recoveryLoop = duework.New(duework.Options{
		AuditInterval: 30 * time.Second,
		Now:           func() time.Time { return service.currentTime() },
	})
	return service
}

// Ensure 返回当前未终结 Execution；不存在时由服务端创建。
func (s *Service) Ensure(
	ctx context.Context,
	actor ActorContext,
	input EnsureInput,
) (returned MutationResult, returnedErr error) {
	defer func() { s.invalidateMutationResult(ctx, returned, returnedErr) }()
	input.CommandID = strings.TrimSpace(input.CommandID)
	if err := validateActor(actor); err != nil {
		return RejectedResult(nil, err, nil), nil
	}
	if actor.PlanMode {
		if input.CommandID == "" {
			return RejectedResult(nil, domainError(
				ErrorCodeInvalidInput,
				"command_id is required",
			), nil), nil
		}
		objective, criteria, proposalErr := validateNewExecutionProposal(input)
		if proposalErr != nil {
			return RejectedResult(nil, proposalErr, nil), nil
		}
		result := NoOpResult(
			nil,
			fmt.Sprintf(
				"Execution proposal is valid with objective %q and %d top-level completion criterion(s); Plan Mode created no authoritative state.",
				objective,
				len(criteria),
			),
		)
		result.NextActions = []NextAction{{
			Domain: "execution", Operation: "prepare_plan_execution",
			Reason: "seal the complete WorkGraph proposal, then leave Plan Mode and commit its exact receipt",
		}}
		return result, nil
	}
	if s.repository == nil {
		return MutationResult{}, fmt.Errorf("orchestration repository is nil")
	}
	current, err := s.repository.FindCurrent(ctx, strings.TrimSpace(actor.OwnerUserID), strings.TrimSpace(actor.SessionKey))
	if err != nil {
		return MutationResult{}, err
	}
	if current != nil {
		snapshot, getErr := s.repository.GetSnapshot(ctx, current.ID)
		if getErr != nil {
			return MutationResult{}, getErr
		}
		if authErr := authorizeSnapshot(actor, snapshot); authErr != nil {
			return RejectedResult(snapshot, authErr, nil), nil
		}
		if boundaryErr := validateOrdinaryReplanBoundary(
			snapshot,
			input.Objective,
			input.CompletionCriteria,
		); boundaryErr != nil {
			if strings.TrimSpace(snapshot.Execution.GoalID) != "" {
				return RejectedResult(snapshot, goalRetargetRequiredError(), []NextAction{{
					Domain: "goal", Operation: "retarget_goal",
					Reason: "advance the Goal objective revision before replanning its bound Execution",
				}}), nil
			}
			return RejectedResult(snapshot, boundaryErr, []NextAction{{
				Domain: "execution", Operation: "prepare_plan_execution",
				Reason: "prepare an operation: replace document with a complete successor boundary",
			}}), nil
		}
		binding, bindingErr := s.prepareExplicitGoalBinding(ctx, actor, snapshot.Execution, true)
		if bindingErr != nil {
			mapped := mapExplicitGoalGatewayError(bindingErr)
			var domainErr *DomainError
			if errors.As(mapped, &domainErr) {
				return RejectedResult(snapshot, mapped, nil), nil
			}
			return MutationResult{}, mapped
		}
		if binding != nil {
			if validationErr := validateExplicitGoalBinding(*binding); validationErr != nil {
				return RejectedResult(snapshot, validationErr, nil), nil
			}
			if strings.TrimSpace(binding.ExecutionID) != snapshot.Execution.ID {
				return RejectedResult(snapshot, domainError(
					ErrorCodeGoalBindingConflict,
					"active explicit Goal is reserved for another Execution",
				), nil), nil
			}
			if snapshot.Execution.GoalID != "" {
				if executionHasExplicitGoalBinding(snapshot.Execution, *binding) {
					return NoOpResult(snapshot, "current Execution already carries the explicit Goal binding"), nil
				}
				return RejectedResult(snapshot, domainError(
					ErrorCodeGoalBindingConflict,
					"current Execution is already bound to another Goal or objective revision",
				), nil), nil
			}
			if input.CommandID == "" {
				return RejectedResult(snapshot, domainError(
					ErrorCodeInvalidInput,
					"command_id is required to bind the current Execution to an explicit Goal",
				), nil), nil
			}
			return s.persistExplicitGoalBinding(ctx, actor, input.CommandID, snapshot, *binding)
		}
		return NoOpResult(snapshot, "current execution already exists"), nil
	}

	if input.CommandID == "" {
		return RejectedResult(nil, domainError(
			ErrorCodeInvalidInput,
			"command_id is required",
		), nil), nil
	}
	objective, criteria, proposalErr := validateNewExecutionProposal(input)
	if proposalErr != nil {
		return RejectedResult(nil, proposalErr, nil), nil
	}
	scope := actor.ScopeKind
	if scope == "" {
		scope = protocol.ExecutionScopeDM
	}
	if scope == protocol.ExecutionScopeRoom &&
		(strings.TrimSpace(actor.RoomID) == "" || strings.TrimSpace(actor.ConversationID) == "") {
		return RejectedResult(nil, domainError(
			ErrorCodeInvalidInput,
			"Room execution requires room_id and conversation_id",
		), nil), nil
	}
	if scope != protocol.ExecutionScopeDM && scope != protocol.ExecutionScopeRoom {
		return RejectedResult(nil, domainError(
			ErrorCodeInvalidInput,
			"unknown execution scope",
		), nil), nil
	}
	origin := input.Origin
	if origin == "" {
		origin = protocol.ExecutionOriginUserRequest
	}
	execution := protocol.Execution{
		ID:                    s.id("execution"),
		OwnerUserID:           strings.TrimSpace(actor.OwnerUserID),
		SessionKey:            strings.TrimSpace(actor.SessionKey),
		ScopeKind:             scope,
		CoordinatorAgentID:    strings.TrimSpace(actor.AgentID),
		Origin:                origin,
		Objective:             objective,
		CompletionCriteria:    criteria,
		RecoveryOfExecutionID: strings.TrimSpace(input.RecoveryOfExecutionID),
		RootRoundID:           strings.TrimSpace(actor.RootRoundID),
		TriggerMessageID:      strings.TrimSpace(input.TriggerMessageID),
		Status:                protocol.ExecutionStatusActive,
		Metadata:              cloneMap(input.Metadata),
	}
	if scope == protocol.ExecutionScopeRoom {
		execution.RoomID = strings.TrimSpace(actor.RoomID)
		execution.ConversationID = strings.TrimSpace(actor.ConversationID)
	}
	binding, bindingErr := s.prepareExplicitGoalBinding(ctx, actor, execution, false)
	if bindingErr != nil {
		mapped := mapExplicitGoalGatewayError(bindingErr)
		var domainErr *DomainError
		if errors.As(mapped, &domainErr) {
			return RejectedResult(nil, mapped, nil), nil
		}
		return MutationResult{}, mapped
	}
	if binding != nil {
		if validationErr := validateExplicitGoalBinding(*binding); validationErr != nil {
			return RejectedResult(nil, validationErr, nil), nil
		}
		execution.ID = strings.TrimSpace(binding.ExecutionID)
		execution.GoalID = strings.TrimSpace(binding.GoalID)
		execution.GoalObjectiveRevision = binding.GoalObjectiveRevision
		execution.GoalActivationOrigin = binding.ActivationOrigin
		execution.GoalActivationReason = binding.ActivationReason
	}
	snapshot, err := s.repository.Create(ctx, orchestrationstore.CreateCommand{
		Execution: execution,
		Meta:      s.commandMeta(actor, input.CommandID, "ensure"),
	})
	if err != nil {
		return s.storageMutationResult(nil, err, nil)
	}
	result := AppliedResult(snapshot, []string{"execution:" + snapshot.Execution.ID}, []NextAction{{
		Domain: "execution", Operation: "prepare_plan_execution",
		Reason: "prepare and seal the complete WorkGraph when coordinated delivery is required",
	}})
	if strings.TrimSpace(snapshot.Execution.GoalID) == "" {
		return result, nil
	}
	if confirmErr := s.confirmGoalExecutionBinding(ctx, snapshot); confirmErr != nil {
		return withPendingGoalConfirmation(
			result,
			"Execution is durable; reverse Goal binding confirmation is pending and will retry automatically.",
			NextAction{
				Domain: "execution", Operation: "get_execution",
				Reason: "continue from the durable Execution while background reconciliation confirms the Goal binding",
			},
		), nil
	}
	return withConfirmedGoalAuthority(result), nil
}

func validateNewExecutionProposal(
	input EnsureInput,
) (string, []string, error) {
	objective := strings.TrimSpace(input.Objective)
	if objective == "" {
		return "", nil, domainError(
			ErrorCodeInvalidInput,
			"objective is required when creating an Execution",
		)
	}
	if err := newProjectionLimitError(
		"completion_criteria",
		len(input.CompletionCriteria),
		"",
	); err != nil {
		return "", nil, err
	}
	criteria := normalizeNonEmptyValues(input.CompletionCriteria)
	if len(criteria) == 0 {
		return "", nil, domainError(
			ErrorCodeCompletionCriteriaEmpty,
			"at least one non-empty top-level completion criterion is required when creating an Execution",
		)
	}
	return objective, criteria, nil
}

// GetCurrent 返回 actor 当前 session 的未终结 Execution snapshot。
func (s *Service) GetCurrent(
	ctx context.Context,
	actor ActorContext,
) (*protocol.ExecutionSnapshot, error) {
	if err := validateActor(actor); err != nil {
		return nil, err
	}
	if s.repository == nil {
		return nil, fmt.Errorf("orchestration repository is nil")
	}
	current, err := s.repository.FindCurrent(
		ctx,
		strings.TrimSpace(actor.OwnerUserID),
		strings.TrimSpace(actor.SessionKey),
	)
	if err != nil || current == nil {
		return nil, err
	}
	return s.GetSnapshot(ctx, actor, current.ID)
}

// ReadCurrent 返回当前 actor 在同一 verified Room/DM scope 中可观察的
// WorkGraph。它只服务显式只读投影，不建立 WorkBinding、ReviewBinding 或
// coordination capability。
func (s *Service) ReadCurrent(
	ctx context.Context,
	actor ActorContext,
) (*protocol.ExecutionSnapshot, error) {
	if err := validateActor(actor); err != nil {
		return nil, err
	}
	if s.repository == nil {
		return nil, fmt.Errorf("orchestration repository is nil")
	}
	current, err := s.repository.FindCurrent(
		ctx,
		strings.TrimSpace(actor.OwnerUserID),
		strings.TrimSpace(actor.SessionKey),
	)
	if err != nil || current == nil {
		return nil, err
	}
	return s.ReadSnapshot(ctx, actor, current.ID)
}

// ReadSnapshot 返回显式只读 WorkGraph 的权威数据源。Room 观察者必须携带
// exact Room 与 conversation identity；该入口不复用 mutation binding fence，
// 也不把完整 snapshot 直接投影给模型。
func (s *Service) ReadSnapshot(
	ctx context.Context,
	actor ActorContext,
	executionID string,
) (*protocol.ExecutionSnapshot, error) {
	if err := validateActor(actor); err != nil {
		return nil, err
	}
	if s.repository == nil {
		return nil, fmt.Errorf("orchestration repository is nil")
	}
	snapshot, err := s.repository.GetSnapshot(ctx, strings.TrimSpace(executionID))
	if err != nil || snapshot == nil {
		return snapshot, err
	}
	if err = authorizeSnapshot(actor, snapshot); err != nil {
		return nil, err
	}
	if snapshot.Execution.ScopeKind != protocol.ExecutionScopeRoom {
		return s.GetSnapshot(ctx, actor, executionID)
	}
	if actor.ScopeKind != protocol.ExecutionScopeRoom ||
		strings.TrimSpace(actor.RoomID) == "" ||
		strings.TrimSpace(actor.RoomID) != strings.TrimSpace(snapshot.Execution.RoomID) ||
		strings.TrimSpace(actor.ConversationID) == "" ||
		strings.TrimSpace(actor.ConversationID) != strings.TrimSpace(snapshot.Execution.ConversationID) {
		return nil, domainError(
			ErrorCodeWrongOwner,
			"execution is outside the current verified Room conversation",
		)
	}
	return snapshot, nil
}

// GetSnapshot 返回 actor 可访问的 Execution snapshot。
func (s *Service) GetSnapshot(
	ctx context.Context,
	actor ActorContext,
	executionID string,
) (*protocol.ExecutionSnapshot, error) {
	if err := validateActor(actor); err != nil {
		return nil, err
	}
	if s.repository == nil {
		return nil, fmt.Errorf("orchestration repository is nil")
	}
	snapshot, err := s.repository.GetSnapshot(ctx, strings.TrimSpace(executionID))
	if err != nil || snapshot == nil {
		return snapshot, err
	}
	if err = authorizeSnapshot(actor, snapshot); err != nil {
		return nil, err
	}
	return scopeSnapshotToTrustedWorkBinding(
		s.effectiveRuntimeCoordinationActor(actor, snapshot),
		executionID,
		snapshot,
	)
}

// runtimeContextSnapshot 允许 conversation-only Room round 看见“存在后台
// Execution”这一最小事实，但不会把完整 WorkGraph capability 交给该 round。
// 所有模型工具读取仍走 GetCurrent/GetSnapshot 的 binding fence。
func (s *Service) runtimeContextSnapshot(
	ctx context.Context,
	actor ActorContext,
) (*protocol.ExecutionSnapshot, error) {
	if err := validateActor(actor); err != nil {
		return nil, err
	}
	if s.repository == nil {
		return nil, fmt.Errorf("orchestration repository is nil")
	}
	executionID := strings.TrimSpace(actor.ExecutionID)
	if executionID == "" {
		current, err := s.repository.FindCurrent(
			ctx,
			strings.TrimSpace(actor.OwnerUserID),
			strings.TrimSpace(actor.SessionKey),
		)
		if err != nil || current == nil {
			return nil, err
		}
		executionID = strings.TrimSpace(current.ID)
	}
	snapshot, err := s.repository.GetSnapshot(ctx, executionID)
	if err != nil || snapshot == nil {
		return snapshot, err
	}
	if strings.TrimSpace(actor.ExecutionID) != "" &&
		strings.TrimSpace(snapshot.Execution.ID) != executionID {
		code := ErrorCodeStaleExecution
		message := "runtime Execution binding is stale"
		if strings.TrimSpace(actor.GoalID) != "" ||
			actor.GoalObjectiveRevision > 0 {
			code = ErrorCodeGoalBindingConflict
			message = "runtime Goal continuation does not match the exact Execution"
		}
		return nil, domainError(code, message)
	}
	if err = authorizeSnapshot(actor, snapshot); err != nil {
		return nil, err
	}
	if goalID := strings.TrimSpace(actor.GoalID); goalID != "" ||
		actor.GoalObjectiveRevision > 0 {
		unboundCoordinatorMismatch := actor.WorkBinding == nil &&
			actor.ReviewBinding == nil &&
			strings.TrimSpace(snapshot.Execution.CoordinatorAgentID) !=
				strings.TrimSpace(actor.AgentID)
		if goalID == "" ||
			actor.GoalObjectiveRevision <= 0 ||
			strings.TrimSpace(snapshot.Execution.GoalID) != goalID ||
			snapshot.Execution.GoalObjectiveRevision != actor.GoalObjectiveRevision ||
			unboundCoordinatorMismatch {
			return nil, domainError(
				ErrorCodeGoalBindingConflict,
				"runtime Goal binding is stale or does not match the current Execution capability",
			)
		}
	}
	if unboundRoomConversationActor(actor, snapshot) {
		return snapshot, nil
	}
	return scopeSnapshotToTrustedWorkBinding(
		s.effectiveRuntimeCoordinationActor(actor, snapshot),
		executionID,
		snapshot,
	)
}

// RuntimeContext 投影当前 actor 每轮需要的有界权威执行状态。
func (s *Service) RuntimeContext(ctx context.Context, actor ActorContext) (string, error) {
	var snapshot *protocol.ExecutionSnapshot
	var err error
	if actor.ObservationOnly {
		if executionID := strings.TrimSpace(actor.ExecutionID); executionID != "" {
			snapshot, err = s.ReadSnapshot(ctx, actor, executionID)
		} else {
			snapshot, err = s.ReadCurrent(ctx, actor)
		}
	} else {
		snapshot, err = s.runtimeContextSnapshot(ctx, actor)
	}
	if err != nil {
		return "", err
	}
	actor = s.effectiveRuntimeCoordinationActor(actor, snapshot)
	options := ExecutionContextOptions{
		ActorAgentID: strings.TrimSpace(actor.AgentID),
		Role:         actor.Role,
		ScopeKind:    actor.ScopeKind,
		WorkBound:    actor.WorkBinding != nil,
		ReviewBound:  actor.ReviewBinding != nil,
		PlanMode:     actor.PlanMode,
		ObserveOnly:  actor.ObservationOnly,
	}
	s.populateRuntimeGraphContext(ctx, actor, snapshot, &options)
	if snapshot == nil {
		return RenderUnmanagedExecutionContext(options), nil
	}
	options.ScopeKind = snapshot.Execution.ScopeKind
	if actor.ObservationOnly {
		options.Role = ExecutionActorMember
		return RenderExecutionContext(snapshot, options), nil
	}
	if strings.TrimSpace(actor.GoalID) != "" &&
		actor.GoalObjectiveRevision > 0 &&
		actor.WorkBinding == nil &&
		actor.ReviewBinding == nil {
		if err = s.ActivateRuntimeCoordination(ctx, actor, snapshot); err != nil {
			return "", err
		}
	}
	if unboundRoomConversationActor(actor, snapshot) {
		if s.runtimeCoordinationActive(actor, snapshot.Execution.ID) {
			options.Role = ExecutionActorCoordinator
			evidence := adaptiveEvidenceFromSnapshot(snapshot, actor)
			if strings.TrimSpace(snapshot.Execution.GoalID) == "" {
				reader, ok := s.goalPromotionGateway.(GoalPromotionAvailabilityReader)
				if !ok {
					evidence.PromotionPolicyUnavailable = true
				} else {
					availability, availabilityErr := reader.ReadGoalPromotionAvailability(
						ctx,
						GoalPromotionAvailabilityRequest{
							Snapshot: snapshot,
							Actor:    actor,
						},
					)
					if availabilityErr != nil {
						evidence.PromotionPolicyUnavailable = true
					} else {
						evidence.AutomaticGoalDisabled = availability.AutomaticGoalDisabled
						evidence.ConflictingGoalID = strings.TrimSpace(
							availability.ConflictingGoalID,
						)
					}
				}
			}
			promotion := EvaluateAdaptiveGoalPromotion(evidence)
			options.GoalPromotionReasons = activationReasonsForSignals(promotion.Signals)
			options.GoalPromotionBlockers = append(
				[]string(nil),
				promotion.Blockers...,
			)
			return RenderExecutionContext(snapshot, options), nil
		}
		options.Role = ExecutionActorMember
		if strings.TrimSpace(actor.AgentID) ==
			strings.TrimSpace(snapshot.Execution.CoordinatorAgentID) {
			options.Role = ExecutionActorCoordinator
		}
		return RenderConversationExecutionContext(snapshot, options), nil
	}
	// Managed Execution 的 coordinator 身份只取 snapshot；Room host/Goal lead
	// 仅用于尚未创建 Execution 时的 fail-closed bootstrap。
	options.Role = ""
	evidence := adaptiveEvidenceFromSnapshot(snapshot, actor)
	if strings.TrimSpace(snapshot.Execution.GoalID) == "" {
		reader, ok := s.goalPromotionGateway.(GoalPromotionAvailabilityReader)
		if !ok {
			evidence.PromotionPolicyUnavailable = true
		} else {
			availability, availabilityErr := reader.ReadGoalPromotionAvailability(
				ctx,
				GoalPromotionAvailabilityRequest{
					Snapshot: snapshot,
					Actor:    actor,
				},
			)
			if availabilityErr != nil {
				evidence.PromotionPolicyUnavailable = true
			} else {
				evidence.AutomaticGoalDisabled = availability.AutomaticGoalDisabled
				evidence.ConflictingGoalID = strings.TrimSpace(availability.ConflictingGoalID)
			}
		}
	}
	promotion := EvaluateAdaptiveGoalPromotion(evidence)
	options.GoalPromotionReasons = activationReasonsForSignals(promotion.Signals)
	options.GoalPromotionBlockers = append([]string(nil), promotion.Blockers...)
	return RenderExecutionContext(snapshot, options), nil
}

// RuntimeGoalBinding 返回当前 actor 的 exact Work/Review Execution 所绑定的
// Goal。未绑定的普通对话 round 永远返回空绑定。
func (s *Service) RuntimeGoalBinding(
	ctx context.Context,
	actor ActorContext,
) (RuntimeGoalBinding, error) {
	if actor.WorkBinding == nil && actor.ReviewBinding == nil {
		return RuntimeGoalBinding{}, nil
	}
	snapshot, err := s.runtimeContextSnapshot(ctx, actor)
	if err != nil || snapshot == nil {
		return RuntimeGoalBinding{}, err
	}
	execution := snapshot.Execution
	if strings.TrimSpace(execution.GoalID) == "" ||
		execution.GoalObjectiveRevision <= 0 {
		return RuntimeGoalBinding{}, nil
	}
	return RuntimeGoalBinding{
		ExecutionID:           strings.TrimSpace(execution.ID),
		SessionKey:            strings.TrimSpace(execution.SessionKey),
		GoalID:                strings.TrimSpace(execution.GoalID),
		GoalObjectiveRevision: execution.GoalObjectiveRevision,
	}, nil
}

func activationReasonsForSignals(signals []GoalPromotionSignal) []protocol.GoalActivationReason {
	result := make([]protocol.GoalActivationReason, 0, len(signals))
	for _, signal := range signals {
		var reason protocol.GoalActivationReason
		switch signal {
		case GoalPromotionSignalObservedBoundary:
			reason = protocol.GoalActivationReasonObservedBoundary
		case GoalPromotionSignalRoomDependency:
			reason = protocol.GoalActivationReasonRoomDependencyChain
		case GoalPromotionSignalExternalWait:
			reason = protocol.GoalActivationReasonExternalWait
		case GoalPromotionSignalScheduledRetry:
			reason = protocol.GoalActivationReasonScheduledRetry
		case GoalPromotionSignalRecovery:
			reason = protocol.GoalActivationReasonRecoveryRequired
		case GoalPromotionSignalContextBoundary:
			reason = protocol.GoalActivationReasonContextBoundary
		}
		if reason != "" && !slices.Contains(result, reason) {
			result = append(result, reason)
		}
	}
	slices.Sort(result)
	return result
}

func validateActor(actor ActorContext) error {
	if strings.TrimSpace(actor.OwnerUserID) == "" ||
		strings.TrimSpace(actor.SessionKey) == "" ||
		strings.TrimSpace(actor.AgentID) == "" {
		return domainError(
			ErrorCodeInvalidInput,
			"owner_user_id, session_key and agent_id are required",
		)
	}
	switch normalizeActorKind(actor.ActorKind) {
	case protocol.ExecutionActorAgent,
		protocol.ExecutionActorUser,
		protocol.ExecutionActorRuntime,
		protocol.ExecutionActorSystem:
		return nil
	default:
		return domainError(ErrorCodeInvalidInput, "unknown actor kind")
	}
}

func authorizeSnapshot(actor ActorContext, snapshot *protocol.ExecutionSnapshot) error {
	if snapshot == nil {
		return nil
	}
	execution := snapshot.Execution
	if execution.OwnerUserID != strings.TrimSpace(actor.OwnerUserID) ||
		execution.SessionKey != strings.TrimSpace(actor.SessionKey) {
		return domainError(
			ErrorCodeWrongOwner,
			"execution is outside the current owner or session",
		)
	}
	if execution.ScopeKind == protocol.ExecutionScopeRoom {
		if roomID := strings.TrimSpace(actor.RoomID); roomID != "" && roomID != execution.RoomID {
			return domainError(ErrorCodeWrongOwner, "execution is outside the current Room")
		}
		if conversationID := strings.TrimSpace(actor.ConversationID); conversationID != "" &&
			conversationID != execution.ConversationID {
			return domainError(ErrorCodeWrongOwner, "execution is outside the current Room conversation")
		}
	}
	return nil
}

func requireCoordinator(actor ActorContext, snapshot *protocol.ExecutionSnapshot) error {
	if err := authorizeSnapshot(actor, snapshot); err != nil {
		return err
	}
	if snapshot == nil ||
		strings.TrimSpace(snapshot.Execution.CoordinatorAgentID) == "" ||
		snapshot.Execution.CoordinatorAgentID != strings.TrimSpace(actor.AgentID) ||
		(actor.Role != "" && actor.Role != ExecutionActorCoordinator) {
		return domainError(
			ErrorCodeWrongOwner,
			"only the execution coordinator may perform this operation",
		)
	}
	return nil
}

func requireMutationRevision(snapshot *protocol.ExecutionSnapshot, expected int64) error {
	if snapshot == nil {
		return domainError(ErrorCodeInvalidInput, "execution was not found")
	}
	if expected <= 0 || snapshot.Execution.Version != expected {
		return domainError(
			ErrorCodeStaleExecution,
			"snapshot_revision is stale; reload the execution before retrying",
		)
	}
	return nil
}

func (s *Service) commandMeta(
	actor ActorContext,
	commandID string,
	suffix string,
) orchestrationstore.CommandMeta {
	return orchestrationstore.CommandMeta{
		CommandID:      commandPart(commandID, suffix),
		EventID:        s.id("event"),
		ActorKind:      normalizeActorKind(actor.ActorKind),
		ActorID:        strings.TrimSpace(actor.AgentID),
		RootRoundID:    strings.TrimSpace(actor.RootRoundID),
		RuntimeRoundID: strings.TrimSpace(actor.RuntimeRoundID),
		AgentRoundID:   strings.TrimSpace(actor.AgentRoundID),
	}
}

func normalizeActorKind(kind protocol.ExecutionActorKind) protocol.ExecutionActorKind {
	if kind == "" {
		return protocol.ExecutionActorAgent
	}
	return kind
}

func commandPart(commandID string, suffix string) string {
	return strings.TrimSpace(commandID) + ":" + strings.TrimSpace(suffix)
}

func (s *Service) id(kind string) string {
	if s != nil && s.newID != nil {
		return s.newID(kind)
	}
	return newOrchestrationID(kind)
}

func newOrchestrationID(kind string) string {
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err == nil {
		return strings.TrimSpace(kind) + "_" + hex.EncodeToString(buffer)
	}
	return fmt.Sprintf("%s_%d", strings.TrimSpace(kind), time.Now().UnixNano())
}

func domainError(code ErrorCode, message string) error {
	return newDomainError(code, strings.TrimSpace(message), "", "")
}

func planModeError() error {
	return domainError(
		ErrorCodePlanMode,
		"execution mutations are disabled while the runtime is in Plan Mode",
	)
}

func normalizeNonEmptyValues(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func cloneMap(source map[string]any) map[string]any {
	if len(source) == 0 {
		return nil
	}
	result := make(map[string]any, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
